package enrich_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/enrich"
)

// maxConcurrentFetches mirrors the unexported production constant. Duplicated
// rather than exported: the cap is an upstream constraint the package owns, and
// a test that reads it from the package under test cannot notice it changing.
const maxConcurrentFetches = 4

// batchSize mirrors the unexported production constant, for the same reason.
// Tests that need a backlog spanning more than one batch have to know where the
// batch ends.
const batchSize = 200

// markCall records one MarkXMLFailed invocation, which is the only externally
// visible statement of what the enricher decided about a failed notice.
type markCall struct {
	tenderID  int64
	status    string
	permanent bool
}

// fakeRepo models the enrichment queue rather than just recording calls: a row
// leaves `pending` when it is saved or permanently marked, and only has its
// attempt counter bumped when it is marked transiently. Drain's termination is
// a property of that state machine, so a fake that merely replayed a fixed list
// would make every drain test pass for the wrong reason.
type fakeRepo struct {
	mu sync.Mutex

	pending    []enrich.PendingNotice
	listErr    error
	listCalls  int
	listLimits []int
	// maxLists fails the run once exceeded, so a drain that fails to terminate
	// surfaces as a clear error instead of hanging the test binary.
	maxLists int

	saved   map[int64]tender.NoticeDetail
	saveErr error

	marks   []markCall
	markErr error
}

func (f *fakeRepo) ListPendingXML(_ context.Context, limit int) ([]enrich.PendingNotice, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.listCalls++
	f.listLimits = append(f.listLimits, limit)
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.maxLists > 0 && f.listCalls > f.maxLists {
		return nil, fmt.Errorf("ListPendingXML called %d times: drain is not terminating", f.listCalls)
	}

	out := f.pending
	if len(out) > limit {
		out = out[:limit]
	}
	return append([]enrich.PendingNotice(nil), out...), nil
}

func (f *fakeRepo) SaveDetail(_ context.Context, tenderID int64, d tender.NoticeDetail) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.saveErr != nil {
		return f.saveErr
	}
	if f.saved == nil {
		f.saved = map[int64]tender.NoticeDetail{}
	}
	f.saved[tenderID] = d
	f.dequeue(tenderID)
	return nil
}

func (f *fakeRepo) MarkXMLFailed(_ context.Context, tenderID int64, status string, permanent bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.marks = append(f.marks, markCall{tenderID: tenderID, status: status, permanent: permanent})
	if f.markErr != nil {
		return f.markErr
	}
	if permanent {
		f.dequeue(tenderID)
		return nil
	}
	for i := range f.pending {
		if f.pending[i].ID == tenderID {
			f.pending[i].Attempts++
			return nil
		}
	}
	return nil
}

// dequeue drops a row from the queue. Callers hold f.mu.
func (f *fakeRepo) dequeue(tenderID int64) {
	for i, n := range f.pending {
		if n.ID == tenderID {
			f.pending = append(f.pending[:i:i], f.pending[i+1:]...)
			return
		}
	}
}

// attempts reports the queued row's stored attempt counter, which is the only
// evidence of whether a failure actually cost the row part of its budget —
// MarkXMLFailed having been CALLED is not the same as it having succeeded.
func (f *fakeRepo) attempts(tenderID int64) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, n := range f.pending {
		if n.ID == tenderID {
			return n.Attempts
		}
	}
	return -1
}

func (f *fakeRepo) snapshot() ([]markCall, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]markCall(nil), f.marks...), len(f.saved), len(f.pending)
}

// fakeFetcher answers from respond and records how the calls were spaced and
// how many overlapped, which is how the pacing and concurrency limits are
// observed from outside the package.
type fakeFetcher struct {
	mu sync.Mutex

	respond func(url string) ([]byte, error)

	urls        []string
	starts      []time.Time
	inFlight    int
	maxInFlight int
}

func (f *fakeFetcher) FetchXML(_ context.Context, url string) ([]byte, error) {
	f.mu.Lock()
	f.urls = append(f.urls, url)
	f.starts = append(f.starts, time.Now())
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	respond := f.respond
	f.mu.Unlock()

	var (
		body []byte
		err  error
	)
	if respond != nil {
		body, err = respond(url)
	}

	f.mu.Lock()
	f.inFlight--
	f.mu.Unlock()
	return body, err
}

func (f *fakeFetcher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.urls)
}

// fakeParser returns one fixed result, which is all the orchestrator cares
// about — the mapping from bytes to domain detail is the adapter's test.
type fakeParser struct {
	detail tender.NoticeDetail
	err    error
}

func (f *fakeParser) Parse([]byte) (tender.NoticeDetail, error) {
	return f.detail, f.err
}

// newEnricher builds an Enricher with pacing disabled, so tests exercise the
// drain logic without spending a quarter-second per notice. The pacer itself is
// covered by TestRunOnce_PacesFetchStarts.
func newEnricher(repo enrich.Repo, fetcher enrich.NoticeFetcher, parser enrich.NoticeDetailParser) *enrich.Enricher {
	return enrich.New(repo, fetcher, parser, enrich.WithPaceInterval(0))
}

func notice(id int64, attempts int) enrich.PendingNotice {
	return enrich.PendingNotice{
		ID:                id,
		PublicationNumber: fmt.Sprintf("%d-2026", id),
		XMLURL:            fmt.Sprintf("https://ted.europa.eu/notice/%d/xml", id),
		Attempts:          attempts,
	}
}

// TestRunOnce_ClassifiesOutcomes pins the mapping from what went wrong to the
// status written and to whether the row leaves the queue. Getting a single row
// of this table wrong is either an infinite retry loop against TED (a permanent
// failure treated as transient) or silent data loss (the reverse).
func TestRunOnce_ClassifiesOutcomes(t *testing.T) {
	detail := tender.NoticeDetail{Language: "ITA"}

	tests := []struct {
		name       string
		pending    enrich.PendingNotice
		fetchErr   error
		parseErr   error
		saveErr    error
		markErr    error
		wantFetch  int
		wantSaves  int
		wantMarks  []markCall
		wantResult enrich.Result
	}{
		{
			name:       "success persists detail and leaves the queue",
			pending:    notice(1, 0),
			wantFetch:  1,
			wantSaves:  1,
			wantResult: enrich.Result{Listed: 1, Succeeded: 1},
		},
		{
			name:       "absent notice is parked after one attempt",
			pending:    notice(2, 0),
			fetchErr:   fmt.Errorf("status 404: %w", enrich.ErrNoticeAbsent),
			wantFetch:  1,
			wantMarks:  []markCall{{tenderID: 2, status: enrich.StatusAbsent, permanent: true}},
			wantResult: enrich.Result{Listed: 1, Permanent: 1},
		},
		{
			name:       "unusable document is parked as a parse error",
			pending:    notice(3, 0),
			fetchErr:   fmt.Errorf("document exceeds 8 MiB: %w", enrich.ErrNoticeUnusable),
			wantFetch:  1,
			wantMarks:  []markCall{{tenderID: 3, status: enrich.StatusParseError, permanent: true}},
			wantResult: enrich.Result{Listed: 1, Permanent: 1},
		},
		{
			name:       "parse failure is deterministic and therefore terminal",
			pending:    notice(4, 0),
			parseErr:   errors.New("XML syntax error"),
			wantFetch:  1,
			wantMarks:  []markCall{{tenderID: 4, status: enrich.StatusParseError, permanent: true}},
			wantResult: enrich.Result{Listed: 1, Permanent: 1},
		},
		{
			name:       "transient failure keeps the row queued",
			pending:    notice(5, 0),
			fetchErr:   errors.New("status 503"),
			wantFetch:  1,
			wantMarks:  []markCall{{tenderID: 5, status: enrich.StatusUnavailable, permanent: false}},
			wantResult: enrich.Result{Listed: 1, Transient: 1},
		},
		{
			name:       "row with no document URL is absent, and is not fetched",
			pending:    enrich.PendingNotice{ID: 6, PublicationNumber: "6-2026"},
			wantFetch:  0,
			wantMarks:  []markCall{{tenderID: 6, status: enrich.StatusAbsent, permanent: true}},
			wantResult: enrich.Result{Listed: 1, Permanent: 1},
		},
		{
			// A rejected write charges an attempt like any other failure. It
			// looks like it should not — the notice was fetched and parsed
			// fine — but a store that rejects THIS row's detail and no other
			// (two parties the document identifies identically, say) rejects
			// it on every pass forever, and with an id-ordered queue that row
			// also pins the head and re-downloads its document every time. The
			// counter is the only thing that ever gets it out.
			name:       "rejected write charges an attempt so a poisoned row can eventually park",
			pending:    notice(7, 0),
			saveErr:    errors.New("duplicate key value violates unique constraint"),
			wantFetch:  1,
			wantMarks:  []markCall{{tenderID: 7, status: enrich.StatusUnavailable, permanent: false}},
			wantResult: enrich.Result{Listed: 1, Transient: 1},
		},
		{
			// The budget is a budget on this path too: the fifth rejected
			// write parks the row, exactly as the fifth failed fetch does.
			name:       "a write that keeps failing parks the row at the attempt budget",
			pending:    notice(10, 4),
			saveErr:    errors.New("duplicate key value violates unique constraint"),
			wantFetch:  1,
			wantMarks:  []markCall{{tenderID: 10, status: enrich.StatusUnavailable, permanent: true}},
			wantResult: enrich.Result{Listed: 1, Permanent: 1},
		},
		{
			name:       "a failed bookkeeping write is not progress",
			pending:    notice(8, 0),
			fetchErr:   fmt.Errorf("gone: %w", enrich.ErrNoticeAbsent),
			markErr:    errors.New("connection refused"),
			wantFetch:  1,
			wantMarks:  []markCall{{tenderID: 8, status: enrich.StatusAbsent, permanent: true}},
			wantResult: enrich.Result{Listed: 1, Transient: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{
				pending: []enrich.PendingNotice{tt.pending},
				saveErr: tt.saveErr,
				markErr: tt.markErr,
			}
			fetcher := &fakeFetcher{respond: func(string) ([]byte, error) {
				return []byte("<notice/>"), tt.fetchErr
			}}
			parser := &fakeParser{detail: detail, err: tt.parseErr}

			res, err := newEnricher(repo, fetcher, parser).RunOnce(t.Context())
			if err != nil {
				t.Fatalf("RunOnce: %v", err)
			}
			if res != tt.wantResult {
				t.Errorf("result = %+v, want %+v", res, tt.wantResult)
			}
			if got := fetcher.callCount(); got != tt.wantFetch {
				t.Errorf("fetch calls = %d, want %d", got, tt.wantFetch)
			}

			marks, saves, _ := repo.snapshot()
			if !reflect.DeepEqual(marks, tt.wantMarks) {
				t.Errorf("marks = %+v, want %+v", marks, tt.wantMarks)
			}
			if saves != tt.wantSaves {
				t.Errorf("saved details = %d, want %d", saves, tt.wantSaves)
			}
			if tt.wantSaves > 0 && !reflect.DeepEqual(repo.saved[tt.pending.ID], detail) {
				t.Errorf("saved detail = %+v, want %+v", repo.saved[tt.pending.ID], detail)
			}
		})
	}
}

// TestRunOnce_TransientFailureParksAtAttemptBudget covers the transition that
// stops the queue re-listing a row forever: the counter carried on the row is
// what the enricher adds to, and the failure that reaches the budget is the one
// that parks it. An off-by-one here either wastes an attempt or, worse, parks a
// row that was only briefly unreachable.
func TestRunOnce_TransientFailureParksAtAttemptBudget(t *testing.T) {
	tests := []struct {
		name          string
		attempts      int
		wantPermanent bool
	}{
		{name: "first failure", attempts: 0, wantPermanent: false},
		{name: "second failure", attempts: 1, wantPermanent: false},
		{name: "fourth failure", attempts: 3, wantPermanent: false},
		{name: "fifth failure exhausts the budget", attempts: 4, wantPermanent: true},
		{name: "beyond the budget stays parked", attempts: 5, wantPermanent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{pending: []enrich.PendingNotice{notice(1, tt.attempts)}}
			fetcher := &fakeFetcher{respond: func(string) ([]byte, error) {
				return nil, errors.New("status 429")
			}}

			res, err := newEnricher(repo, fetcher, &fakeParser{}).RunOnce(t.Context())
			if err != nil {
				t.Fatalf("RunOnce: %v", err)
			}

			marks, _, stillQueued := repo.snapshot()
			want := []markCall{{tenderID: 1, status: enrich.StatusUnavailable, permanent: tt.wantPermanent}}
			if !reflect.DeepEqual(marks, want) {
				t.Fatalf("marks = %+v, want %+v", marks, want)
			}
			if tt.wantPermanent {
				if res.Permanent != 1 || stillQueued != 0 {
					t.Errorf("permanent=%d queued=%d, want the row parked", res.Permanent, stillQueued)
				}
			} else {
				if res.Transient != 1 || stillQueued != 1 {
					t.Errorf("transient=%d queued=%d, want the row still queued", res.Transient, stillQueued)
				}
			}
		})
	}
}

// TestRunOnce_DatabaseOutageSpendsNoAttemptBudget is the other half of
// charging an attempt for a rejected write, and the reason charging is safe at
// all.
//
// A store that rejects one row's detail and a store that is simply down are
// indistinguishable from this layer — it cannot see a constraint name — so the
// two are told apart structurally instead: an outage fails the bookkeeping
// write on the same connection, so no attempt is recorded and the row's budget
// survives. Were that not so, a five-minute database outage would park every
// row it touched, permanently, and nothing but a new publication number ever
// restores a budget.
func TestRunOnce_DatabaseOutageSpendsNoAttemptBudget(t *testing.T) {
	down := errors.New("connection refused")
	repo := &fakeRepo{
		pending: []enrich.PendingNotice{notice(1, 0)},
		saveErr: down,
		markErr: down,
	}
	fetcher := &fakeFetcher{respond: func(string) ([]byte, error) { return []byte("<notice/>"), nil }}

	res, err := newEnricher(repo, fetcher, &fakeParser{}).RunOnce(t.Context())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if want := (enrich.Result{Listed: 1, Transient: 1}); res != want {
		t.Errorf("result = %+v, want %+v", res, want)
	}
	if got := repo.attempts(1); got != 0 {
		t.Errorf("attempts = %d, want 0: an outage must not consume the row's budget", got)
	}
	if _, _, queued := repo.snapshot(); queued != 1 {
		t.Errorf("queued = %d, want the row retained for the next run", queued)
	}
}

// TestDrain_DoesNotRetryATransientlyFailedRowWithinTheSameRun protects the
// attempt budget's stated unit.
//
// maxAttempts is documented as five attempts across five SCHEDULED RUNS, and
// with a backlog larger than one batch that is only true because the drain
// holds a failed row back. ListPendingXML is ordered by id, so a row that
// failed keeps the head of the queue and would otherwise be re-listed by the
// very next batch — burning its whole budget in seconds on an upstream blip too
// brief to trip the circuit breaker, and then parking it terminally.
//
// The backlog here is deliberately larger than one batch so that later batches
// exist at all; the first row fails every time it is asked, everything else
// succeeds.
func TestDrain_DoesNotRetryATransientlyFailedRowWithinTheSameRun(t *testing.T) {
	const total = batchSize + 50

	repo := &fakeRepo{maxLists: 10}
	for i := range total {
		repo.pending = append(repo.pending, notice(int64(i+1), 0))
	}
	failing := repo.pending[0].XMLURL
	fetcher := &fakeFetcher{respond: func(url string) ([]byte, error) {
		if url == failing {
			return nil, errors.New("status 503")
		}
		return []byte("<notice/>"), nil
	}}

	if err := newEnricher(repo, fetcher, &fakeParser{}).Drain(t.Context()); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	marks, saves, queued := repo.snapshot()
	if len(marks) != 1 {
		t.Errorf("marks = %+v, want exactly one attempt charged in this run", marks)
	}
	if got := repo.attempts(1); got != 1 {
		t.Errorf("attempts = %d, want 1: the row's remaining budget belongs to later runs", got)
	}
	if saves != total-1 {
		t.Errorf("saved = %d, want %d: deferring one row must not stall the drain", saves, total-1)
	}
	if queued != 1 {
		t.Errorf("queued = %d, want only the failed row left", queued)
	}
}

// TestRunOnce_EmptyQueueDoesNothing — the steady state, run every 15 minutes.
func TestRunOnce_EmptyQueueDoesNothing(t *testing.T) {
	repo := &fakeRepo{}
	fetcher := &fakeFetcher{}

	res, err := newEnricher(repo, fetcher, &fakeParser{}).RunOnce(t.Context())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if (res != enrich.Result{}) {
		t.Errorf("result = %+v, want zero", res)
	}
	if fetcher.callCount() != 0 {
		t.Errorf("fetch calls = %d, want 0", fetcher.callCount())
	}
	if got := repo.listLimits; len(got) != 1 || got[0] <= 0 {
		t.Errorf("list limits = %v, want one positive batch size", got)
	}
}

// TestRunOnce_ListFailureIsFatalToThePass — failing to claim a batch is the one
// error RunOnce propagates, because it is not attributable to any single row.
func TestRunOnce_ListFailureIsFatalToThePass(t *testing.T) {
	listErr := errors.New("connection refused")
	repo := &fakeRepo{listErr: listErr}

	if _, err := newEnricher(repo, &fakeFetcher{}, &fakeParser{}).RunOnce(t.Context()); !errors.Is(err, listErr) {
		t.Fatalf("RunOnce error = %v, want it to wrap %v", err, listErr)
	}
	if err := newEnricher(repo, &fakeFetcher{}, &fakeParser{}).Drain(t.Context()); !errors.Is(err, listErr) {
		t.Fatalf("Drain error = %v, want it to wrap %v", err, listErr)
	}
}

// TestDrain_ParkedRowsCountAsProgress is the regression test for this pass's
// one deliberate departure from the indexing pass it is modelled on. The first
// batch here is entirely notices that will never succeed; only rows past that
// batch can be enriched. Terminating on successes — as the indexer does — would
// stop after batch one and leave the tail unreachable for as long as those rows
// keep being listed first.
func TestDrain_ParkedRowsCountAsProgress(t *testing.T) {
	const (
		unfetchable = 200 // one full batch of notices with no XML document
		fetchable   = 50
	)

	var pending []enrich.PendingNotice
	for i := range unfetchable {
		pending = append(pending, enrich.PendingNotice{ID: int64(i + 1), PublicationNumber: "x"})
	}
	for i := range fetchable {
		pending = append(pending, notice(int64(unfetchable+i+1), 0))
	}

	repo := &fakeRepo{pending: pending, maxLists: 10}
	fetcher := &fakeFetcher{respond: func(string) ([]byte, error) { return []byte("<notice/>"), nil }}

	if err := newEnricher(repo, fetcher, &fakeParser{}).Drain(t.Context()); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	marks, saves, queued := repo.snapshot()
	if queued != 0 {
		t.Errorf("%d rows left queued, want the whole backlog drained", queued)
	}
	if saves != fetchable {
		t.Errorf("saved = %d, want %d", saves, fetchable)
	}
	if len(marks) != unfetchable {
		t.Errorf("marks = %d, want %d", len(marks), unfetchable)
	}
	for _, m := range marks {
		if m.status != enrich.StatusAbsent || !m.permanent {
			t.Fatalf("mark = %+v, want a permanent absent", m)
		}
	}
}

// TestDrain_StopsWhenNothingAdvances — a batch that fails every row transiently
// leaves the queue exactly as it found it, so listing again would hand back the
// same rows forever. One pass, then stop and let the next scheduled run retry.
func TestDrain_StopsWhenNothingAdvances(t *testing.T) {
	repo := &fakeRepo{maxLists: 3}
	for i := range 10 {
		repo.pending = append(repo.pending, notice(int64(i+1), 0))
	}
	fetcher := &fakeFetcher{respond: func(string) ([]byte, error) {
		return nil, errors.New("status 503")
	}}

	if err := newEnricher(repo, fetcher, &fakeParser{}).Drain(t.Context()); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if repo.listCalls != 1 {
		t.Errorf("list calls = %d, want 1: a pass that advanced nothing must not list again", repo.listCalls)
	}
	if got := fetcher.callCount(); got != 10 {
		t.Errorf("fetch calls = %d, want 10", got)
	}
	_, _, queued := repo.snapshot()
	if queued != 10 {
		t.Errorf("queued = %d, want all 10 rows retained for the next run", queued)
	}
}

// TestRunOnce_CircuitBreakEndsThePass — a fully-down upstream must not cost the
// whole window. The break lands within a few notices of the threshold: fetches
// already in flight when it trips still complete, so the tolerance below is the
// concurrency cap, not slack.
func TestRunOnce_CircuitBreakEndsThePass(t *testing.T) {
	const (
		queued    = 120
		threshold = 25
	)

	repo := &fakeRepo{}
	for i := range queued {
		repo.pending = append(repo.pending, notice(int64(i+1), 0))
	}
	fetcher := &fakeFetcher{respond: func(string) ([]byte, error) {
		return nil, errors.New("dial tcp: connection refused")
	}}

	res, err := newEnricher(repo, fetcher, &fakeParser{}).RunOnce(t.Context())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if !res.Tripped {
		t.Fatal("result not marked as tripped")
	}
	if got := fetcher.callCount(); got < threshold || got > threshold+maxConcurrentFetches-1 {
		t.Errorf("fetch calls = %d, want between %d and %d", got, threshold, threshold+maxConcurrentFetches-1)
	}
	if res.Skipped == 0 || res.Listed != queued {
		t.Errorf("result = %+v, want most of the batch skipped", res)
	}
	if res.Succeeded+res.Permanent != 0 {
		t.Errorf("result = %+v, want no progress against a dead upstream", res)
	}
}

// TestDrain_StopsOnCircuitBreak — the break has to end the whole drain, not
// just the batch, or the next RunOnce simply starts hammering again with a
// fresh breaker.
func TestDrain_StopsOnCircuitBreak(t *testing.T) {
	repo := &fakeRepo{maxLists: 2}
	// Half the batch succeeds, so the pass DOES advance and would otherwise keep
	// draining — the break is the only reason it stops.
	for i := range 60 {
		repo.pending = append(repo.pending, notice(int64(i+1), 0))
	}

	var mu sync.Mutex
	served := 0
	fetcher := &fakeFetcher{respond: func(string) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		served++
		if served <= 20 {
			return []byte("<notice/>"), nil
		}
		return nil, errors.New("dial tcp: connection refused")
	}}

	if err := newEnricher(repo, fetcher, &fakeParser{}).Drain(t.Context()); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if repo.listCalls != 1 {
		t.Errorf("list calls = %d, want 1: the drain must not start another batch after tripping", repo.listCalls)
	}
}

// TestRunOnce_UpstreamRepliesResetTheBreak — 404s are the upstream answering,
// not failing. A corpus stretch of legacy notices with no XML document must
// drain, not be mistaken for an outage.
func TestRunOnce_UpstreamRepliesResetTheBreak(t *testing.T) {
	const queued = 80

	repo := &fakeRepo{}
	for i := range queued {
		repo.pending = append(repo.pending, notice(int64(i+1), 0))
	}
	fetcher := &fakeFetcher{respond: func(string) ([]byte, error) {
		return nil, fmt.Errorf("status 404: %w", enrich.ErrNoticeAbsent)
	}}

	res, err := newEnricher(repo, fetcher, &fakeParser{}).RunOnce(t.Context())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Tripped {
		t.Error("breaker tripped on notices the upstream answered for")
	}
	if res.Permanent != queued {
		t.Errorf("permanent = %d, want %d", res.Permanent, queued)
	}
}

// TestRunOnce_LimitsConcurrency — the cap is an upstream constraint, so the
// test asserts the exact ceiling rather than a bound: too few workers wastes the
// window, too many is what TED rate-limits.
func TestRunOnce_LimitsConcurrency(t *testing.T) {
	const queued = 16

	repo := &fakeRepo{}
	for i := range queued {
		repo.pending = append(repo.pending, notice(int64(i+1), 0))
	}

	// Each call blocks until the cap is saturated, so the observed maximum is
	// the real limit rather than a scheduling accident. The timeout keeps a
	// lower-than-expected cap a failure instead of a hang.
	saturated := make(chan struct{})
	var once sync.Once
	fetcher := &fakeFetcher{}
	fetcher.respond = func(string) ([]byte, error) {
		fetcher.mu.Lock()
		reached := fetcher.inFlight >= maxConcurrentFetches
		fetcher.mu.Unlock()
		if reached {
			once.Do(func() { close(saturated) })
		}
		select {
		case <-saturated:
		case <-time.After(time.Second):
		}
		return []byte("<notice/>"), nil
	}

	if _, err := newEnricher(repo, fetcher, &fakeParser{}).RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if fetcher.maxInFlight != maxConcurrentFetches {
		t.Errorf("max in-flight fetches = %d, want %d", fetcher.maxInFlight, maxConcurrentFetches)
	}
}

// TestRunOnce_PacesFetchStarts — the pacer, not the worker count, is what
// actually caps the request rate against TED. Asserted on the interval between
// consecutive starts because that is the property the upstream enforces.
func TestRunOnce_PacesFetchStarts(t *testing.T) {
	const (
		queued   = 8
		interval = 10 * time.Millisecond
	)

	repo := &fakeRepo{}
	for i := range queued {
		repo.pending = append(repo.pending, notice(int64(i+1), 0))
	}
	fetcher := &fakeFetcher{respond: func(string) ([]byte, error) { return []byte("<notice/>"), nil }}

	e := enrich.New(repo, fetcher, &fakeParser{}, enrich.WithPaceInterval(interval))
	started := time.Now()
	if _, err := e.RunOnce(t.Context()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	// The first start is free, so the pass cannot finish sooner than the
	// remaining slots take.
	if elapsed, floor := time.Since(started), time.Duration(queued-1)*interval; elapsed < floor {
		t.Errorf("pass took %s, want at least %s", elapsed, floor)
	}
	if got := fetcher.callCount(); got != queued {
		t.Errorf("fetch calls = %d, want %d", got, queued)
	}
}

// TestRunOnce_CancelledContextLeavesRowsUntouched — the run's deadline expiring
// is the normal end of a large backlog, not a notice failing. Rows caught by it
// must keep their attempt budget, or a corpus bigger than one window would park
// its tail purely for being at the wrong end of the queue.
func TestRunOnce_CancelledContextLeavesRowsUntouched(t *testing.T) {
	repo := &fakeRepo{}
	for i := range 5 {
		repo.pending = append(repo.pending, notice(int64(i+1), 0))
	}
	fetcher := &fakeFetcher{respond: func(string) ([]byte, error) { return []byte("<notice/>"), nil }}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	res, err := newEnricher(repo, fetcher, &fakeParser{}).RunOnce(ctx)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Skipped != 5 || res.Advanced() != 0 {
		t.Errorf("result = %+v, want every row skipped", res)
	}
	if fetcher.callCount() != 0 {
		t.Errorf("fetch calls = %d, want 0", fetcher.callCount())
	}

	marks, saves, queued := repo.snapshot()
	if len(marks) != 0 || saves != 0 || queued != 5 {
		t.Errorf("marks=%d saves=%d queued=%d, want the queue untouched", len(marks), saves, queued)
	}
}
