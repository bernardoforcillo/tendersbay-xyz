package enrich

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// batchSize bounds how many queued notices one RunOnce call processes, so a
// large backlog drains over several batches (each of which commits as it goes)
// instead of making one pass balloon into an all-or-nothing unit of work.
//
// It is sized against the pacer below rather than against memory: at one fetch
// start every paceInterval, 200 notices is roughly 50 seconds of upstream
// traffic, which keeps a batch small enough that a run killed at its deadline
// loses at most that much progress.
const batchSize = 200

// maxAttempts is how many failed attempts a row gets before it is parked as
// StatusUnavailable. Counted as attempts, not retries: the fifth failure is the
// one that parks it.
//
// The number exists to bound work, not to be optimal. TED throttling clears in
// seconds and the fetcher already retries within a single attempt, so five
// attempts spread over five SCHEDULED RUNS is generous for anything genuinely
// transient; past that, a row is not "slow to succeed", it is broken, and
// re-listing it every pass costs the batch slots that working rows need.
//
// "Five scheduled runs" is a property Drain has to defend, not one it gets for
// free — see the deferral it carries, without which a backlog larger than one
// batch spends the whole budget inside a single run.
const maxAttempts = 5

// maxConcurrentFetches bounds how many notice documents are in flight at once.
// Four is tedmcp's default and its documented ceiling, with the note that TED
// rate-limits aggressively above it — an upstream constraint measured against
// the real service, not a guess about our own capacity, which is why it is not
// tuned up to "use the pod better".
const maxConcurrentFetches = 4

// defaultPaceInterval is the minimum gap between two outbound fetch STARTS,
// across all workers. It is the actual throughput ceiling of this pass:
// concurrency only decides how much of the 250 ms is spent waiting on responses
// in parallel, so the pass tops out near four notices a second regardless of
// how many workers run.
//
// That arithmetic (~3,000 notices in a 840-second window at perfect
// utilisation) is arithmetic, not a measurement — the real rate is worth
// instrumenting on the first production run before anyone plans capacity on it.
const defaultPaceInterval = 250 * time.Millisecond

// consecutiveFailureLimit is the circuit break: this many transient fetch
// failures in a row with no intervening response ends the pass.
//
// Without it, a TED outage turns every scheduled run into the whole window
// spent hammering a service that is already struggling — the pass would fail
// every row, mark none of them permanently, and still keep listing more rows
// until it was killed. Twenty-five is deliberately well above any plausible
// unlucky streak (the fetcher has already retried each of those attempts
// internally) and well below a batch, so a genuinely-down upstream is detected
// within seconds and a merely flaky one is not.
const consecutiveFailureLimit = 25

// Result reports what one RunOnce pass did. The counts partition Listed:
// every row is exactly one of succeeded, permanently parked, left queued, or
// skipped.
type Result struct {
	// Listed is how many queued rows the batch claimed.
	Listed int
	// Succeeded is how many notices were fetched, parsed and persisted.
	Succeeded int
	// Permanent is how many rows were parked terminally (absent, unusable,
	// unparseable, or out of attempts).
	Permanent int
	// Transient is how many rows failed but stay queued for a later pass.
	Transient int
	// Skipped is how many rows were not attempted at all: the circuit breaker
	// had already tripped, the context was done, or the row already failed
	// transiently earlier in this drain. These rows are untouched — not even
	// their attempt counter moved.
	Skipped int
	// Tripped reports that the circuit breaker ended this batch early.
	Tripped bool
}

// Advanced is how many rows left the queue this pass — the only measure of
// progress that is safe to terminate a drain on.
//
// Counting successes alone (as the indexing pass does, correctly for its own
// queue) would be wrong here, because this queue can contain rows that will
// never succeed: a notice with no XML document is a permanent, expected outcome
// rather than an error, and it still has to stop being re-listed. Counting
// listed rows instead would be worse — a batch that fails every row transiently
// would look like progress and spin against the upstream for the whole window,
// which is exactly what the circuit breaker exists to catch.
func (r Result) Advanced() int { return r.Succeeded + r.Permanent }

// Enricher drains the notice-enrichment queue.
type Enricher struct {
	repo    Repo
	fetcher NoticeFetcher
	parser  NoticeDetailParser
	pacer   *pacer
}

// Option customises an Enricher.
type Option func(*Enricher)

// WithPaceInterval overrides the minimum gap between outbound fetch starts.
//
// Production uses defaultPaceInterval; this exists so a run can be paced from
// configuration if TED's real tolerance is ever measured to differ from the
// value tedmcp settled on, and so tests can exercise the drain without spending
// real seconds per row. A non-positive interval disables pacing entirely, which
// is only ever appropriate against a fake.
func WithPaceInterval(d time.Duration) Option {
	return func(e *Enricher) { e.pacer.interval = d }
}

// New returns an Enricher over the given ports.
func New(repo Repo, fetcher NoticeFetcher, parser NoticeDetailParser, opts ...Option) *Enricher {
	e := &Enricher{
		repo:    repo,
		fetcher: fetcher,
		parser:  parser,
		pacer:   &pacer{interval: defaultPaceInterval},
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// RunOnce enriches up to batchSize queued notices, bounded by
// maxConcurrentFetches workers whose fetch starts are paced. Per-notice
// failures are recorded against their row and skipped, never fatal — a failed
// notice must not cost the rest of the batch. RunOnce itself only returns an
// error for a failure that prevents claiming a batch at all.
func (e *Enricher) RunOnce(ctx context.Context) (Result, error) {
	return e.runBatch(ctx, nil)
}

// runBatch is RunOnce with the drain's deferral set threaded through. A row in
// deferred is left completely untouched — not fetched, not counted, not charged
// an attempt — because it already failed transiently earlier in this same run;
// see Drain for why re-attempting it immediately would be worse than waiting.
// A nil map (the RunOnce entry point) defers nothing, which is the right
// behaviour for a single pass: there is no earlier batch to have failed in.
func (e *Enricher) runBatch(ctx context.Context, deferred map[int64]bool) (Result, error) {
	pending, err := e.repo.ListPendingXML(ctx, batchSize)
	if err != nil {
		return Result{}, fmt.Errorf("enrich: list pending xml: %w", err)
	}

	res := Result{Listed: len(pending)}
	if len(pending) == 0 {
		return res, nil
	}

	br := &breaker{limit: consecutiveFailureLimit}
	// Outcomes are collected by index rather than through a shared accumulator:
	// each worker owns exactly one slot, so the fan-out needs no lock and the
	// tally below cannot race.
	outcomes := make([]outcome, len(pending))

	var g errgroup.Group
	g.SetLimit(maxConcurrentFetches)
	for i, n := range pending {
		g.Go(func() error {
			// Checked here rather than only in the loop above because SetLimit
			// makes g.Go block: by the time a late worker starts, the breaker
			// may have tripped on notices that were still in flight when it was
			// queued. Skipping leaves the row completely untouched, so it is
			// simply attempted again on the next run.
			//
			// deferred is read-only for the whole fan-out — it is only written
			// after Wait below — so this needs no lock. A nil map reads as
			// "nothing deferred", which is exactly what RunOnce wants.
			if deferred[n.ID] || br.tripped() || ctx.Err() != nil {
				outcomes[i] = outcomeSkipped
				return nil
			}
			outcomes[i] = e.enrichOne(ctx, n, br)
			return nil
		})
	}
	// The workers never return an error — a per-notice failure is recorded on
	// its row, not propagated — so Wait is only a join.
	_ = g.Wait()

	for i, o := range outcomes {
		switch o {
		case outcomeSucceeded:
			res.Succeeded++
		case outcomePermanent:
			res.Permanent++
		case outcomeTransient:
			res.Transient++
			// Recorded after the fan-out has joined, so the map is written by
			// exactly one goroutine and the workers above only ever read it.
			if deferred != nil {
				deferred[pending[i].ID] = true
			}
		case outcomeSkipped:
			res.Skipped++
		}
	}
	res.Tripped = br.tripped()
	return res, nil
}

// Drain repeats RunOnce until a pass moves nothing out of the queue, so one
// invocation clears as much of the backlog as its time budget allows instead of
// stopping at batchSize. Backfill and steady state are therefore the same
// mechanism: the pre-existing corpus is just the queue's state at t=0.
//
// Terminating on rows ADVANCED — not on rows indexed, and not on rows listed —
// is the correction this pass makes to the indexing pass it is otherwise
// modelled on. That one terminates when a batch succeeds at nothing, which is
// right for a queue whose every row can eventually succeed. This queue's rows
// cannot: a notice with no XML document will never succeed, and if "parked
// forever" did not count as progress, the same permanently-failing batch would
// be re-listed and re-attempted until the deadline killed the run. Counting a
// permanent park as progress is what lets those rows drain out; the attempt
// counter behind MarkXMLFailed is what guarantees every row eventually reaches
// one of the two exits. Do not "simplify" this back to counting successes.
//
// Cancellation is not an error: the caller's deadline expiring mid-drain is the
// normal way a large backlog ends a run, and every completed batch is already
// committed.
//
// # Why a row that failed transiently is held back for the rest of the run
//
// maxAttempts is sized as five attempts spread over five scheduled runs, and
// without the deferral below that is simply false whenever the backlog exceeds
// one batch — which is the entire initial drain, the case this loop exists for.
// ListPendingXML orders by id and a transiently-failed row keeps its place at
// the head of the queue, so it would be re-listed by the very next batch,
// seconds later, and again by the one after that. Its whole budget would be
// spent inside one run, on one upstream blip too brief to trip the circuit
// breaker (successes from the rest of the batch keep resetting it), and the row
// would then be stamped terminally unavailable. Nothing ever restores an
// exhausted budget except a new publication number, so those notices would be
// lost for good — a permanent outcome bought with about thirty-five seconds of
// evidence.
//
// Deferring costs nothing real: the row stays queued, and the next scheduled
// run starts with an empty deferral set.
func (e *Enricher) Drain(ctx context.Context) error {
	enriched, parked := 0, 0
	deferred := make(map[int64]bool)
	for {
		if err := ctx.Err(); err != nil {
			slog.InfoContext(ctx, "enrichment pass stopped early",
				"reason", err, "enriched_total", enriched, "parked_total", parked)
			return nil
		}

		res, err := e.runBatch(ctx, deferred)
		enriched += res.Succeeded
		parked += res.Permanent
		if err != nil {
			return err
		}
		if res.Tripped {
			slog.WarnContext(ctx, "enrichment pass stopped by circuit breaker",
				"consecutive_failures", consecutiveFailureLimit,
				"enriched_total", enriched, "parked_total", parked)
			return nil
		}
		if res.Advanced() == 0 {
			slog.InfoContext(ctx, "enrichment pass complete",
				"enriched_total", enriched, "parked_total", parked, "left_queued", res.Transient)
			return nil
		}
		slog.InfoContext(ctx, "enrichment batch complete",
			"listed", res.Listed, "enriched", res.Succeeded, "parked", res.Permanent,
			"left_queued", res.Transient, "enriched_total", enriched, "parked_total", parked)
	}
}

// outcome is how one notice counted toward the batch's progress.
type outcome int

const (
	outcomeSkipped outcome = iota
	outcomeSucceeded
	outcomePermanent
	outcomeTransient
)

// enrichOne fetches, parses and persists one notice, recording whatever
// happened against its row. It returns how the row counted, never an error:
// the caller's job is to keep going.
func (e *Enricher) enrichOne(ctx context.Context, n PendingNotice, br *breaker) outcome {
	// A queued row with nothing to fetch is not a transient failure and must
	// not consume five attempts pretending to be one. It happens for real:
	// rows ingested from a payload that carried no link to a notice document
	// have no URL to recover, so the honest answer is the same as a 404 —
	// there is no document, stop asking.
	if n.XMLURL == "" {
		return e.markFailed(ctx, n, StatusAbsent, true, noXMLBytes, errors.New("no notice document URL on the row"))
	}

	if err := e.pacer.wait(ctx); err != nil {
		return outcomeSkipped
	}

	raw, fetchErr := e.fetcher.FetchXML(ctx, n.XMLURL)

	// A cancelled context is the run's deadline arriving, not the notice
	// misbehaving. Charging an attempt for it would let a corpus larger than one
	// window park rows purely for being at the wrong end of the queue, and every
	// write from here on would fail on the same dead context anyway.
	//
	// It is checked BEFORE the breaker is touched, and that ordering is the
	// point: at the end of every full-window run there are up to
	// maxConcurrentFetches requests in flight that all fail at once when the
	// deadline lands. Counting those as upstream failures would log "circuit
	// breaker tripped" on a perfectly healthy TED, poisoning the one line whose
	// whole job is to mean that TED is down.
	if ctx.Err() != nil {
		return outcomeSkipped
	}

	// The breaker tracks whether the UPSTREAM is answering, not whether rows are
	// succeeding. A 404 and an oversized document are both replies — TED is
	// healthy, this notice is simply not usable — so they reset the counter.
	// Only a failure to get an answer at all counts toward the break.
	if fetchErr == nil || errors.Is(fetchErr, ErrNoticeAbsent) || errors.Is(fetchErr, ErrNoticeUnusable) {
		br.responded()
	} else if br.failed() {
		slog.WarnContext(ctx, "notice fetching circuit breaker tripped",
			"consecutive_failures", consecutiveFailureLimit, "error", fetchErr)
	}

	if fetchErr != nil {
		// No document arrived in any of these cases — including the unusable
		// one, where the fetcher refused it precisely because reading it fully
		// was not safe — so the observation reports its size as absent rather
		// than as zero.
		switch {
		case errors.Is(fetchErr, ErrNoticeAbsent):
			return e.markFailed(ctx, n, StatusAbsent, true, noXMLBytes, fetchErr)
		case errors.Is(fetchErr, ErrNoticeUnusable):
			return e.markFailed(ctx, n, StatusParseError, true, noXMLBytes, fetchErr)
		default:
			// n.Attempts counts the failures already recorded, so this one is
			// the (n.Attempts+1)-th and the budget is spent when it reaches
			// maxAttempts.
			return e.markFailed(ctx, n, StatusUnavailable, n.Attempts+1 >= maxAttempts, noXMLBytes, fetchErr)
		}
	}

	detail, parseErr := e.parser.Parse(raw)
	if parseErr != nil {
		return e.markFailed(ctx, n, StatusParseError, true, len(raw), parseErr)
	}

	if err := e.repo.SaveDetail(ctx, n.ID, detail); err != nil {
		// A persistence failure is USUALLY ours rather than the notice's — a
		// database that is down fails every row alike — but it is not always,
		// and the difference decides whether this row can ever leave the queue.
		// One notice whose detail the store rejects deterministically (two
		// parties the document identifies identically, a weight the numeric
		// column will not take) fails this write and only this write, on every
		// pass, forever. Since ListPendingXML is ordered by id, such a row also
		// pins the head of the queue and re-downloads its document from TED on
		// every batch of every run.
		//
		// Charging the attempt is what tells the two apart WITHOUT having to
		// classify the error, which is not something this layer can do — it
		// cannot see a constraint name. When the database is genuinely down the
		// bookkeeping write inside markFailed fails on the same outage, so
		// nothing is recorded and the budget is preserved exactly as before;
		// when only this row is poison, the counter advances and the row parks
		// after maxAttempts like any other row that cannot be made to work.
		// StatusUnavailable is the honest status for both: its detail could not
		// be made available, for a reason that might not persist.
		return e.markFailed(ctx, n, StatusUnavailable, n.Attempts+1 >= maxAttempts, len(raw), err)
	}
	observeNotice(ctx, n, StatusOK, detail, len(raw))
	return outcomeSucceeded
}

// markFailed records a failed attempt against the row and reports how it
// counts. xmlBytes is the size of the document that was read before the
// failure, or noXMLBytes when none arrived — it feeds the observation and
// nothing else.
func (e *Enricher) markFailed(ctx context.Context, n PendingNotice, status string, permanent bool, xmlBytes int, cause error) outcome {
	slog.ErrorContext(ctx, "notice enrichment failed",
		"tender_id", n.ID,
		"publication_number", n.PublicationNumber,
		"status", status,
		"permanent", permanent,
		"attempts", n.Attempts+1,
		"error", cause)

	if err := e.repo.MarkXMLFailed(ctx, n.ID, status, permanent); err != nil {
		slog.ErrorContext(ctx, "failed to record notice enrichment failure",
			"tender_id", n.ID, "status", status, "error", err)
		// The row was not actually parked, whatever we intended, so it must not
		// be counted as progress — otherwise a database that rejects every
		// bookkeeping write would look like a draining queue.
		return outcomeTransient
	}
	if permanent {
		// Observed only on a permanent park, because that is the attempt on
		// which the row leaves the queue. A transient failure will be retried
		// and would otherwise report the same notice once per attempt, turning
		// every count over these events into an attempt-weighted number.
		observeNotice(ctx, n, status, tender.NoticeDetail{}, xmlBytes)
		return outcomePermanent
	}
	return outcomeTransient
}

// pacer serialises the START of every outbound fetch so that no more than one
// begins per interval, no matter how many workers are running. It is the rate
// limit; the concurrency cap only bounds how many responses are outstanding.
//
// It reserves a slot under the lock and then sleeps outside it, so waiting
// workers queue on the reservation rather than on each other's sleep — the
// difference between four workers spaced 250 ms apart and four workers taking
// turns being slow.
//
// This is per-process and per-run state, and it is not a violation of the
// stateless-service rule despite looking like one. It holds nothing about any
// tender, nothing a second replica would need to agree with, and nothing that
// has to survive a restart: it paces one process's own egress for the length of
// one batch job. A shared/distributed limiter would be the wrong tool — there
// is exactly one enricher process per scheduled run by construction.
//
// The fetcher adapter paces its own request starts as well, and the two are not
// redundant. This one paces DISPATCH: it is what makes the pass's throughput
// ceiling a property of the orchestration rather than of whichever fetcher is
// plugged into it, so a fetcher swap cannot silently remove the rate limit. The
// adapter's covers what this one cannot see — its internal retries are requests
// that never pass through here. Composing them does not halve the rate either,
// since by the time a worker reaches the adapter its slot there is normally
// already due.
type pacer struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

// wait blocks until this caller's paced slot arrives, returning ctx.Err() if
// the context is done first.
func (p *pacer) wait(ctx context.Context) error {
	if p.interval <= 0 {
		return ctx.Err()
	}

	p.mu.Lock()
	now := time.Now()
	if p.next.Before(now) {
		p.next = now
	}
	slot := p.next
	p.next = slot.Add(p.interval)
	p.mu.Unlock()

	d := time.Until(slot)
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// breaker counts consecutive failures to get an answer out of the upstream and
// trips once there have been limit of them.
//
// "Consecutive" is approximate under concurrency — with several fetches in
// flight the interleaving of failures and replies is not a strict sequence —
// and that is fine for what it is measuring. It is a health signal about the
// upstream, not an exact count of anything; being a few notices early or late
// changes nothing about the decision it drives.
type breaker struct {
	mu          sync.Mutex
	limit       int
	consecutive int
	open        bool
}

// failed records a failure to reach the upstream and reports whether that
// failure is the one that tripped the breaker — so the caller can log the trip
// exactly once.
func (b *breaker) failed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.open {
		return false
	}
	b.consecutive++
	if b.consecutive >= b.limit {
		b.open = true
		return true
	}
	return false
}

// responded records that the upstream answered, which resets the streak.
func (b *breaker) responded() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutive = 0
}

// tripped reports whether the breaker is open. It stays open for the rest of
// the batch once tripped: a late reply from an upstream that just failed 25
// times in a row is not evidence the outage is over, and the next scheduled run
// starts with a clean breaker anyway.
func (b *breaker) tripped() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.open
}
