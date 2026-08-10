package retrieve

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// batchSize bounds how many queued tenders one RunOnce call processes, so a
// large backlog drains over several batches (each of which commits as it goes)
// rather than making one pass an all-or-nothing unit of work.
//
// Twenty, not the enricher's two hundred, and the ratio is the point: a row
// there is ONE request to one upstream that publishes its own rate limit, where
// a row here is up to seventeen requests — one robots, one listing, up to
// fifteen files — against a different stranger's server per row, each of them
// serialised behind a two-second per-host gap. A batch is sized so that a run
// killed at its deadline loses a bounded amount of work, and the work per row
// is an order of magnitude larger here.
const batchSize = 20

// maxAttempts is how many failed attempts a row gets before it is parked as
// StatusUnreachable. Counted as attempts, not retries: the third failure is the
// one that parks it.
//
// Three, not the enricher's five, and the reduction is deliberate. There the
// budget is bounded by WORK — TED throttling clears in seconds and costs us
// nothing but our own time. Here it is bounded by POLITENESS: every attempt
// costs a stranger's server a robots fetch and a listing fetch, and five
// attempts against a portal that is down is ten requests for a result we
// already have.
const maxAttempts = 3

// maxConcurrentTenders bounds how many tenders are in flight at once. Combined
// with the fetching adapter's per-host request slot of one, it means at most
// four DISTINCT hosts are being talked to and never more than one request per
// host at a time.
//
// There is deliberately no dispatch pacer here, unlike the enricher's. That one
// exists because its rate limit is global to one upstream, so the orchestrator
// can enforce it without knowing anything about a URL. This pass's rate limit
// is PER HOST, and the host of a request is not knowable here — a listing link
// legitimately redirects to a file CDN, and it is the connection that has to be
// paced, not the dispatch. So the limit lives in the adapter, where a resolved
// host exists, and the orchestration bound is concurrency alone.
const maxConcurrentTenders = 4

// defaultPerTenderTimeout bounds one tender's whole retrieval — the listing and
// every file under it. Without it, a portal that answers every request in
// twenty-five seconds could consume a whole run on a handful of rows while
// looking perfectly healthy to both circuit breakers, because it is answering.
//
// Four minutes is roughly what fifteen files cost at the adapter's two-second
// per-host gap plus response time, so it binds on a slow portal rather than on
// a large tender.
const defaultPerTenderTimeout = 4 * time.Minute

// hostFailureLimit ends work against ONE host after this many consecutive
// failures to get any answer out of it.
//
// The enricher's single global breaker is the wrong shape here and would be
// actively harmful: its failures all belong to one upstream, where these belong
// to hosts, and one dead comune must not stop us reading the other nineteen
// tenders in the batch. Three is low because the fetcher has already retried
// each of those attempts internally, so three consecutive failures is already
// nine requests that produced nothing.
const hostFailureLimit = 3

// globalFailureLimit is the second breaker, for the one case a per-host breaker
// cannot see: the pod has no egress at all. Fifty consecutive failures across
// DISTINCT hosts is not a portal problem, and continuing would spend the rest
// of the window discovering the same thing.
const globalFailureLimit = 50

// minExtractedChars is the floor below which a file counts as not extracted.
//
// It exists because of what has no floor: a scanned PDF with no text layer.
// This service builds with CGO_ENABLED=0 and the OCR dependency needs cgo, so
// tabula returns zero or a handful of stray characters for those files rather
// than failing — and there will be many of them. Without this guard each one
// would be persisted as a document row with no usable parts, and the indexer
// treats a partless document row as "fetch this URL yourself", with the
// unguarded fetcher, on every cycle, forever. Two hundred characters is well
// below any real page of an Italian disciplinare and well above the noise a
// scan produces.
const minExtractedChars = 200

// Result reports what one RunOnce pass did. The counts partition Listed: every
// row is exactly one of succeeded, permanently parked, left queued, or skipped.
type Result struct {
	// Listed is how many queued rows the batch claimed.
	Listed int
	// Succeeded is how many tenders had at least one document's text persisted.
	Succeeded int
	// Permanent is how many rows were parked terminally — including the honest
	// refusals (robots_denied, auth_required, captcha), which are outcomes
	// rather than errors.
	Permanent int
	// Transient is how many rows failed but stay queued for a later pass.
	Transient int
	// Skipped is how many rows were not attempted at all: a breaker was
	// already open, the context was done, or the row already failed transiently
	// earlier in this drain. These rows are untouched — not even their attempt
	// counter moved.
	Skipped int
	// Tripped reports that the global circuit breaker ended this batch early.
	Tripped bool
}

// Advanced is how many rows left the queue this pass — the only measure of
// progress that is safe to terminate a drain on.
//
// Counting successes alone would be wrong for the same reason it is wrong in
// the enrichment pass, and more so here: this queue is full of rows that will
// never succeed. A portal that forbids automated clients is a permanent,
// expected, and genuinely useful outcome, and it still has to stop being
// re-listed.
func (r Result) Advanced() int { return r.Succeeded + r.Permanent }

// Retriever drains the portal-document retrieval queue.
type Retriever struct {
	repo      Repo
	fetcher   PortalFetcher
	enum      ListingEnumerator
	extractor TextExtractor

	perTenderTimeout time.Duration
}

// Option customises a Retriever.
type Option func(*Retriever)

// WithPerTenderTimeout overrides how long one tender's retrieval may take. It
// exists so tests can bound a fake portal in milliseconds rather than minutes;
// a non-positive value is rejected in favour of the default, because a tender
// with no bound is the failure this constant exists to prevent.
func WithPerTenderTimeout(d time.Duration) Option {
	return func(r *Retriever) {
		if d > 0 {
			r.perTenderTimeout = d
		}
	}
}

// New returns a Retriever over the given ports.
func New(repo Repo, fetcher PortalFetcher, enum ListingEnumerator, extractor TextExtractor, opts ...Option) *Retriever {
	r := &Retriever{
		repo:             repo,
		fetcher:          fetcher,
		enum:             enum,
		extractor:        extractor,
		perTenderTimeout: defaultPerTenderTimeout,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RunOnce retrieves documents for up to batchSize queued tenders, bounded by
// maxConcurrentTenders workers. Per-tender failures are recorded against their
// row and skipped, never fatal — one unreachable portal must not cost the rest
// of the batch. RunOnce itself only returns an error for a failure that
// prevents claiming a batch at all.
func (r *Retriever) RunOnce(ctx context.Context) (Result, error) {
	return r.runBatch(ctx, nil, newBreakers())
}

// runBatch is RunOnce with the drain's deferral set and circuit breakers
// threaded through. A row in deferred is left completely untouched — not
// fetched, not counted, not charged an attempt — because it already failed
// transiently earlier in this same run; see Drain. A nil map defers nothing,
// which is the right behaviour for a single pass.
func (r *Retriever) runBatch(ctx context.Context, deferred map[int64]bool, br *breakers) (Result, error) {
	pending, err := r.repo.ListPendingDocuments(ctx, batchSize)
	if err != nil {
		return Result{}, fmt.Errorf("retrieve: list pending documents: %w", err)
	}

	res := Result{Listed: len(pending)}
	if len(pending) == 0 {
		return res, nil
	}

	// Outcomes are collected by index rather than through a shared accumulator:
	// each worker owns exactly one slot, so the fan-out needs no lock and the
	// tally below cannot race.
	outcomes := make([]outcome, len(pending))

	var g errgroup.Group
	g.SetLimit(maxConcurrentTenders)
	for i, n := range pending {
		g.Go(func() error {
			// Checked here rather than only in the loop above because SetLimit
			// makes g.Go block: by the time a late worker starts, the global
			// breaker may have opened on tenders that were still in flight when
			// this one was queued. Skipping leaves the row completely
			// untouched, so it is simply attempted again on the next run.
			//
			// deferred is read-only for the whole fan-out — it is written only
			// after Wait below — so this needs no lock.
			if deferred[n.ID] || br.globalTripped() || ctx.Err() != nil {
				outcomes[i] = outcomeSkipped
				return nil
			}
			outcomes[i] = r.retrieveOne(ctx, n, br)
			return nil
		})
	}
	// The workers never return an error — a per-tender failure is recorded on
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
	res.Tripped = br.globalTripped()
	return res, nil
}

// Drain repeats a batch until a pass moves nothing out of the queue, so one
// invocation clears as much of the backlog as its time budget allows instead of
// stopping at batchSize. Backfill and steady state are therefore the same
// mechanism: the pre-existing corpus is simply the queue's state at t=0, which
// is why this pass ships with no backfill command.
//
// Cancellation is not an error: the caller's deadline expiring mid-drain is the
// normal way a large backlog ends a run, and every completed tender is already
// committed.
//
// # Why a row that failed transiently is held back for the rest of the run
//
// maxAttempts is three attempts spread over three SCHEDULED RUNS, and without
// the deferral below that is simply false whenever the backlog exceeds one
// batch — which is the entire initial drain, the case this loop exists for.
// ListPendingDocuments orders by id and a transiently-failed row keeps its place
// at the head of the queue, so it would be re-listed by the very next batch,
// seconds later, and again by the one after. Its whole budget would be spent
// inside one run against one portal's bad minute, and the row would then be
// parked as unreachable for good. Nothing ever restores an exhausted budget
// except a new publication number.
//
// Deferring costs nothing real: the row stays queued, and the next scheduled run
// starts with an empty deferral set and clean breakers.
func (r *Retriever) Drain(ctx context.Context) error {
	retrieved, parked := 0, 0
	deferred := make(map[int64]bool)
	br := newBreakers()
	for {
		if err := ctx.Err(); err != nil {
			slog.InfoContext(ctx, "portal retrieval pass stopped early",
				"reason", err, "retrieved_total", retrieved, "parked_total", parked)
			return nil
		}

		res, err := r.runBatch(ctx, deferred, br)
		retrieved += res.Succeeded
		parked += res.Permanent
		if err != nil {
			return err
		}
		if res.Tripped {
			slog.WarnContext(ctx, "portal retrieval pass stopped by circuit breaker",
				"consecutive_failures", globalFailureLimit,
				"retrieved_total", retrieved, "parked_total", parked)
			return nil
		}
		if res.Advanced() == 0 {
			slog.InfoContext(ctx, "portal retrieval pass complete",
				"retrieved_total", retrieved, "parked_total", parked, "left_queued", res.Transient)
			return nil
		}
		slog.InfoContext(ctx, "portal retrieval batch complete",
			"listed", res.Listed, "retrieved", res.Succeeded, "parked", res.Permanent,
			"left_queued", res.Transient, "retrieved_total", retrieved, "parked_total", parked)
	}
}

// outcome is how one tender counted toward the batch's progress.
type outcome int

const (
	outcomeSkipped outcome = iota
	outcomeSucceeded
	outcomePermanent
	outcomeTransient
)

// retrieveOne walks one tender's portal and records whatever happened against
// its row. It returns how the row counted, never an error: the caller's job is
// to keep going.
//
// The shape follows the failure taxonomy rather than the happy path, because
// the failures are the product here — a portal that refuses us is an answer
// this pass exists to deliver, not an error to be swallowed.
func (r *Retriever) retrieveOne(ctx context.Context, n PendingTender, br *breakers) outcome {
	host := hostOf(n.DocumentsURL)
	if !br.allows(host) {
		return outcomeSkipped
	}

	// A documents link that points back at the EU register is answered without
	// a request; see isTEDEntry for why following it would double-count this
	// corpus against itself.
	if isTEDEntry(n.DocumentsURL) {
		return r.finish(ctx, n, Outcome{Status: StatusNoFiles})
	}

	// The per-tender deadline bounds the FETCHING only. Every bookkeeping write
	// below takes the outer ctx, so a tender that ran out of its own time still
	// records what it learned instead of failing to write it on a context it
	// just cancelled.
	fetchCtx, cancel := context.WithTimeout(ctx, r.perTenderTimeout)
	defer cancel()

	page, err := r.fetcher.Fetch(fetchCtx, n.DocumentsURL)
	if err != nil {
		return r.recordFetchFailure(ctx, n, host, br, err)
	}
	br.responded(host)

	var budget Budget

	// The entry URL was itself a document rather than a listing. Not a failure
	// and not an edge case — a documents_url pointing straight at a capitolato
	// is the best case this pass has.
	if !page.IsHTML() {
		budget.Spend(len(page.Body))
		return r.finishEntryFile(ctx, n, page, &budget)
	}
	budget.SpendBytes(len(page.Body))

	listing, enumErr := r.enum.Enumerate(page)
	if enumErr != nil {
		// Deterministic: the same bytes fail the same way on every pass, so
		// retrying costs a download for an answer we already have. What changes
		// this row is a parser shipping, not a retry.
		slog.WarnContext(ctx, "portal listing could not be parsed",
			"tender_id", n.ID, "host", host, "error", enumErr)
		return r.finish(ctx, n, Outcome{Status: StatusTraversalFailed, BytesFetched: budget.Bytes()})
	}

	if len(listing.Files) == 0 {
		// THE INVARIANT, and the one line of this file that must not be
		// "simplified". An empty listing from a NAMED platform parser is a
		// positive claim that the buyer published nothing; an empty listing
		// from the generic fallback is a statement about the fallback, which
		// cannot read JavaScript-rendered tables, form-POST downloads or
		// per-document detail pages. Collapsing the two would reintroduce
		// "an inaccessible file mistaken for an absent one" one layer below
		// where anyone would look for it.
		status := StatusTraversalFailed
		if listing.Platform != PlatformGeneric && listing.Platform != "" {
			status = StatusNoFiles
		}
		return r.finish(ctx, n, Outcome{
			Status:       status,
			Platform:     listing.Platform,
			BytesFetched: budget.Bytes(),
		})
	}

	// Deduplicated ONCE, and the count is taken from the deduplicated set
	// rather than from the raw listing. A listing that links the same file
	// twice — as a table row and again as a sidebar icon, which dedupeFiles
	// exists because portals do routinely — published one document, not two,
	// and normalisation catches the copies that differ only by a session
	// parameter and therefore survived scanLinks' verbatim dedupe.
	//
	// Counting the raw slice inflated docs_files_found without ever inflating
	// docs_files_extracted, so the one ratio this whole pass is measured on
	// read as a coverage gap that did not exist: eight files published as
	// sixteen links became "16 found, 8 extracted", plus a spurious
	// files_capped on the observation. That is the same
	// unreadable-versus-absent conflation the statuses are built to prevent,
	// pointed the other way.
	//
	// The cap is still applied after this and still NOT reflected in the
	// count, which is the deliberate part: FilesFound is what the buyer
	// published, so a truncation stays a SQL fact (found > MaxFilesPerTender)
	// rather than an inference.
	files := dedupeFiles(listing.Files)

	out := Outcome{
		Platform:   listing.Platform,
		FilesFound: len(files),
	}

	for _, f := range rankFiles(files, MaxFilesPerTender) {
		if !budget.Allow() || fetchCtx.Err() != nil {
			break
		}
		r.retrieveFile(ctx, fetchCtx, n, f, host, br, &budget, &out)
	}
	out.BytesFetched = budget.Bytes()

	// The run's own deadline landing mid-tender is not a measurement. Files
	// already saved persist and docs_fetched_at stays NULL, so the tender is
	// re-listed and the surviving files are re-fetched idempotently — wasteful,
	// bounded, and correct. What must not happen is FinishRetrieval writing a
	// coverage count for a listing this pass never finished reading.
	if ctx.Err() != nil {
		return outcomeSkipped
	}

	// The TENDER's deadline is different from the run's: it is evidence about
	// this portal, not about this pass. Whenever it expires the listing was
	// only PARTLY read, so neither count is a measurement of it — and that is
	// true of a partial read exactly as much as of an empty one.
	//
	// Guarding only the all-or-nothing case was wrong in the more damaging
	// direction. A tender whose deadline expired after six of ten files were
	// read used to be finished as ok with 10/6 and docs_fetched_at stamped: it
	// left the queue permanently, and the four files nobody ever requested were
	// recorded as found-but-unextractable — which is precisely the population
	// that decides whether .p7m and .zip unwrapping is worth building (see
	// ports.go's StatusExtractFailed). A padded population is worse than a
	// missing one, because nothing about the row says it is padded.
	//
	// So the whole tender is charged as an ordinary transient failure. It keeps
	// the files it already persisted — those are third-party requests that must
	// not be spent twice — and stays queued, so the next pass re-reads it
	// idempotently, the same wasteful-but-correct trade the run-deadline branch
	// above already makes. A portal that never fits the budget spends its
	// attempts and parks as unreachable with both counts NULL, which reads as
	// "we could not finish reading this", and that is the true statement.
	if fetchCtx.Err() != nil {
		return r.markFailed(ctx, n, StatusUnreachable, n.Attempts+1 >= maxAttempts, host,
			fmt.Errorf("retrieve: tender exceeded %s after reading %d of %d documents",
				r.perTenderTimeout, out.FilesExtracted, out.FilesFound))
	}

	out.Status = StatusExtractFailed
	if out.FilesExtracted > 0 {
		out.Status = StatusOK
	}
	return r.finish(ctx, n, out)
}

// retrieveFile fetches, extracts and persists one of a tender's files, updating
// the budget and the outcome in place.
//
// Every failure here is a SKIP, never fatal to the tender: one 404 among
// fifteen attachments says nothing about the other fourteen, and the counts
// already carry the detail — FilesFound above FilesExtracted is exactly the
// measurement that decides what to build next, so it does not need a status of
// its own.
func (r *Retriever) retrieveFile(
	ctx, fetchCtx context.Context,
	n PendingTender, f FileLink, host string,
	br *breakers, budget *Budget, out *Outcome,
) {
	// Fetched with the address the page published, stored under the normalised
	// one. A download endpoint may well require the very session token
	// normalisation strips, so the request has to be verbatim; the record of it
	// has to be stable, or FinishRetrieval's delete-what-is-missing rule churns
	// every row on every run.
	doc, err := r.fetcher.Fetch(fetchCtx, f.URL)
	if err != nil {
		if Terminal(err) {
			br.responded(host)
		} else {
			br.failed(host)
		}
		slog.WarnContext(ctx, "portal document could not be fetched",
			"tender_id", n.ID, "host", host, "error", err)
		return
	}
	br.responded(host)
	budget.Spend(len(doc.Body))

	parts, err := r.extractor.Extract(doc)
	if err != nil {
		slog.WarnContext(ctx, "portal document could not be extracted",
			"tender_id", n.ID, "host", host, "error", err)
		return
	}
	// A format this phase cannot read returns no parts and no error, and a scan
	// with no text layer returns a handful of stray characters. Neither is
	// persisted: a document row without usable text is a portal URL handed to
	// the unguarded indexer fetcher forever. See the package comment.
	if !usableParts(parts) {
		return
	}

	stored := doc
	stored.URL = normaliseURL(f.URL)
	if err := r.repo.SaveRetrievedDocument(ctx, n.ID, stored, parts); err != nil {
		slog.ErrorContext(ctx, "failed to persist a retrieved portal document",
			"tender_id", n.ID, "host", host, "error", err)
		return
	}

	out.FilesExtracted++
	out.PartsWritten += len(parts)
	out.KeptURLs = append(out.KeptURLs, stored.URL)
}

// finishEntryFile handles the case where documents_url served a document rather
// than a listing. docs_platform stays empty, which is honest: no page was
// parsed, so no enumerator produced this file and attributing it to one would
// put a parser's name on work it did not do.
func (r *Retriever) finishEntryFile(ctx context.Context, n PendingTender, page Document, budget *Budget) outcome {
	out := Outcome{
		Status:       StatusExtractFailed,
		FilesFound:   1,
		BytesFetched: budget.Bytes(),
	}

	parts, err := r.extractor.Extract(page)
	switch {
	case err != nil:
		slog.WarnContext(ctx, "portal entry document could not be extracted",
			"tender_id", n.ID, "host", hostOf(n.DocumentsURL), "error", err)
	case !usableParts(parts):
	default:
		stored := page
		stored.URL = normaliseURL(n.DocumentsURL)
		if saveErr := r.repo.SaveRetrievedDocument(ctx, n.ID, stored, parts); saveErr != nil {
			slog.ErrorContext(ctx, "failed to persist a retrieved portal document",
				"tender_id", n.ID, "host", hostOf(n.DocumentsURL), "error", saveErr)
			break
		}
		out.Status = StatusOK
		out.FilesExtracted = 1
		out.PartsWritten = len(parts)
		out.KeptURLs = []string{stored.URL}
	}
	return r.finish(ctx, n, out)
}

// recordFetchFailure classifies a failure to retrieve the LISTING page — the
// point at which most of this pass's interesting outcomes are decided.
func (r *Retriever) recordFetchFailure(ctx context.Context, n PendingTender, host string, br *breakers, err error) outcome {
	// A cancelled outer context is the run's deadline arriving, not the portal
	// misbehaving. Charging an attempt for it would let a corpus larger than one
	// window park rows purely for being at the wrong end of the queue, and every
	// write from here on would fail on the same dead context anyway.
	//
	// It is checked BEFORE the breakers are touched, and that ordering is the
	// point: at the end of every full-window run there are up to
	// maxConcurrentTenders requests in flight that all fail at once when the
	// deadline lands. Counting those as portal failures would log "circuit
	// breaker tripped" against perfectly healthy hosts.
	if ctx.Err() != nil {
		return outcomeSkipped
	}

	// A refusal IS an answer. robots.txt denying us, a captcha, a login wall and
	// a 404 all prove the host is up and talking, so they reset the breaker
	// rather than counting toward it — the breakers measure whether a host is
	// REACHABLE, not whether rows are succeeding.
	if Terminal(err) {
		br.responded(host)
		return r.markFailed(ctx, n, StatusFor(err), true, host, err)
	}

	if br.failed(host) {
		slog.WarnContext(ctx, "portal retrieval circuit breaker tripped",
			"host", host, "host_failure_limit", hostFailureLimit,
			"global_failure_limit", globalFailureLimit, "error", err)
	}
	return r.markFailed(ctx, n, StatusUnreachable, n.Attempts+1 >= maxAttempts, host, err)
}

// finish takes the tender out of the queue and reports how it counted.
//
// A failed write is reported as transient rather than as the outcome it
// intended, because the row was not actually parked whatever we meant to do —
// otherwise a database rejecting every bookkeeping write would look like a
// draining queue. It is also what makes a database outage self-correcting
// everywhere in this file: when the store is down, both the per-file save and
// this call fail together, so nothing is recorded, no attempt is charged, and
// the row is simply re-listed.
func (r *Retriever) finish(ctx context.Context, n PendingTender, out Outcome) outcome {
	if err := r.repo.FinishRetrieval(ctx, n.ID, out); err != nil {
		slog.ErrorContext(ctx, "failed to record portal retrieval outcome",
			"tender_id", n.ID, "host", hostOf(n.DocumentsURL), "status", out.Status, "error", err)
		return outcomeTransient
	}
	observe(ctx, n, out)
	if out.Status == StatusOK {
		return outcomeSucceeded
	}
	return outcomePermanent
}

// markFailed records a failed attempt against the row and reports how it counts.
func (r *Retriever) markFailed(ctx context.Context, n PendingTender, status string, permanent bool, host string, cause error) outcome {
	// The host, never the URL. A documents link carries the buyer's own
	// reference for a procedure and sometimes a session token; the host is what
	// a human needs in order to open the same portal by hand, and it is all the
	// platform-backlog query groups by.
	slog.ErrorContext(ctx, "portal document retrieval failed",
		"tender_id", n.ID,
		"host", host,
		"status", status,
		"permanent", permanent,
		"attempts", n.Attempts+1,
		"error", cause)

	if err := r.repo.MarkDocumentsFailed(ctx, n.ID, status, permanent); err != nil {
		slog.ErrorContext(ctx, "failed to record portal retrieval failure",
			"tender_id", n.ID, "status", status, "error", err)
		return outcomeTransient
	}
	if permanent {
		// Observed only on a permanent park, because that is the attempt on
		// which the row leaves the queue. A transient failure will be retried
		// and would otherwise report the same tender once per attempt, turning
		// every count over these events into an attempt-weighted number —
		// weighted, specifically, by the portals that are slowest.
		observe(ctx, n, Outcome{Status: status})
		return outcomePermanent
	}
	return outcomeTransient
}

// usableParts reports whether an extraction produced enough text to be worth
// persisting. See minExtractedChars for what it is defending against.
func usableParts(parts []tender.DocumentPart) bool {
	if len(parts) == 0 {
		return false
	}
	total := 0
	for _, p := range parts {
		total += len(p.Text)
		if total >= minExtractedChars {
			return true
		}
	}
	return false
}

// breakers is the two-level circuit break: one counter per host, and one
// across all of them.
//
// Two levels rather than one because the two failures they detect are not the
// same shape. A per-host counter catches the ordinary case — one comune's
// server is down, and the other nineteen tenders in the batch are fine. It
// cannot catch the case where NOTHING is reachable, because that failure never
// accumulates against any single host; the global counter is for exactly that,
// and its limit is set well above any plausible run of unrelated bad portals.
//
// Once open a breaker stays open for the rest of the run. A late success from a
// host that just failed three times in a row is not evidence the outage is
// over, and the next scheduled run starts clean.
type breakers struct {
	mu sync.Mutex

	global     int
	globalOpen bool

	hosts    map[string]int
	hostOpen map[string]bool
}

func newBreakers() *breakers {
	return &breakers{
		hosts:    map[string]int{},
		hostOpen: map[string]bool{},
	}
}

// allows reports whether work against host may still be attempted.
func (b *breakers) allows(host string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.globalOpen && !b.hostOpen[host]
}

// failed records a failure to get any answer out of host and reports whether
// this failure is the one that opened a breaker — so the caller logs the trip
// exactly once.
func (b *breakers) failed(host string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	tripped := false
	if !b.hostOpen[host] {
		b.hosts[host]++
		if b.hosts[host] >= hostFailureLimit {
			b.hostOpen[host] = true
			tripped = true
		}
	}
	if !b.globalOpen {
		b.global++
		if b.global >= globalFailureLimit {
			b.globalOpen = true
			tripped = true
		}
	}
	return tripped
}

// responded records that host answered — including a refusal, which is an
// answer — resetting its streak and the global one.
func (b *breakers) responded(host string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.hosts[host] = 0
	b.global = 0
}

// globalTripped reports whether the run-wide breaker is open.
func (b *breakers) globalTripped() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.globalOpen
}
