package retrieve_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/retrieve"
)

// maxAttempts and maxConcurrentTenders mirror the unexported production
// constants. Duplicated rather than exported, for the reason the enrichment
// tests duplicate theirs: the budget is a policy the package owns, and a test
// that read it from the package under test could not notice it changing.
const (
	maxAttempts          = 3
	maxConcurrentTenders = 4
	hostFailureLimit     = 3
)

// longText is text long enough to clear the minimum-extracted-characters floor,
// which is what separates a real extraction from the handful of stray glyphs a
// scanned PDF produces.
var longText = strings.Repeat("criteri di valutazione dell'offerta tecnica. ", 20)

// markCall records one MarkDocumentsFailed invocation, and finishCall one
// FinishRetrieval — between them they are the entire externally visible
// statement of what the retriever decided about a tender.
type markCall struct {
	tenderID  int64
	status    string
	permanent bool
}

type finishCall struct {
	tenderID int64
	out      retrieve.Outcome
}

// savedDocument is one persisted file. The test asserts on these rather than on
// extractor calls, because "extracted" and "persisted" are deliberately
// different events: a file that yields no usable text is extracted and NOT
// persisted, and that gap is the rule keeping buyer-portal URLs away from the
// indexer's unguarded fetcher.
type savedDocument struct {
	tenderID int64
	url      string
	parts    int
}

// fakeRepo models the retrieval queue rather than merely recording calls: a row
// leaves `pending` when it is finished or permanently marked, and only has its
// attempt counter bumped when it is marked transiently. Drain's termination is a
// property of that state machine.
type fakeRepo struct {
	mu sync.Mutex

	pending   []retrieve.PendingTender
	listCalls int
	// maxLists fails the run once exceeded, so a drain that fails to terminate
	// surfaces as a clear error instead of hanging the test binary.
	maxLists int

	saved     []savedDocument
	saveErr   error
	finishes  []finishCall
	finishErr error
	marks     []markCall
}

func (f *fakeRepo) ListPendingDocuments(_ context.Context, limit int) ([]retrieve.PendingTender, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.listCalls++
	if f.maxLists > 0 && f.listCalls > f.maxLists {
		return nil, fmt.Errorf("ListPendingDocuments called %d times: drain is not terminating", f.listCalls)
	}
	out := f.pending
	if len(out) > limit {
		out = out[:limit]
	}
	return append([]retrieve.PendingTender(nil), out...), nil
}

func (f *fakeRepo) SaveRetrievedDocument(
	_ context.Context, tenderID int64, doc retrieve.Document, parts []tender.DocumentPart,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, savedDocument{tenderID: tenderID, url: doc.URL, parts: len(parts)})
	return nil
}

func (f *fakeRepo) FinishRetrieval(_ context.Context, tenderID int64, o retrieve.Outcome) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.finishes = append(f.finishes, finishCall{tenderID: tenderID, out: o})
	if f.finishErr != nil {
		return f.finishErr
	}
	f.dequeue(tenderID)
	return nil
}

func (f *fakeRepo) MarkDocumentsFailed(_ context.Context, tenderID int64, status string, permanent bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.marks = append(f.marks, markCall{tenderID: tenderID, status: status, permanent: permanent})
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
	for i, t := range f.pending {
		if t.ID == tenderID {
			f.pending = append(f.pending[:i:i], f.pending[i+1:]...)
			return
		}
	}
}

func (f *fakeRepo) finish(t *testing.T, tenderID int64) retrieve.Outcome {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.finishes {
		if c.tenderID == tenderID {
			return c.out
		}
	}
	t.Fatalf("FinishRetrieval was never called for tender %d (marks: %+v)", tenderID, f.marks)
	return retrieve.Outcome{}
}

func (f *fakeRepo) attempts(tenderID int64) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range f.pending {
		if t.ID == tenderID {
			return t.Attempts
		}
	}
	return -1
}

// response is one canned answer from the fake portal.
type response struct {
	doc retrieve.Document
	err error
}

// fakeFetcher answers from a fixed map and records what was asked for, which is
// how the tests assert that a refused path costs no request and that a capped
// listing stops at the cap.
type fakeFetcher struct {
	mu        sync.Mutex
	responses map[string]response
	requested []string
}

func (f *fakeFetcher) Fetch(_ context.Context, url string) (retrieve.Document, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.requested = append(f.requested, url)
	r, ok := f.responses[url]
	if !ok {
		return retrieve.Document{}, retrieve.ErrNotFound
	}
	return r.doc, r.err
}

func (f *fakeFetcher) calls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.requested)
}

// fakeEnumerator returns one canned listing for every page.
type fakeEnumerator struct {
	listing retrieve.Listing
	err     error
}

func (f fakeEnumerator) Enumerate(retrieve.Document) (retrieve.Listing, error) {
	return f.listing, f.err
}

// fakeExtractor returns parts keyed by the document's URL, so a test can make
// one file of a listing readable and the rest not.
type fakeExtractor struct {
	byURL map[string][]tender.DocumentPart
	err   error
}

func (f fakeExtractor) Extract(doc retrieve.Document) ([]tender.DocumentPart, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.byURL[doc.URL], nil
}

func htmlPage(url string) retrieve.Document {
	return retrieve.Document{URL: url, ContentType: "text/html; charset=utf-8", Body: []byte("<html></html>")}
}

func pdf(url string) retrieve.Document {
	return retrieve.Document{URL: url, ContentType: "application/pdf", Filename: "d.pdf", Body: []byte("%PDF-1.4")}
}

func parts(n int) []tender.DocumentPart {
	out := make([]tender.DocumentPart, n)
	for i := range out {
		out[i] = tender.DocumentPart{Text: longText, PageStart: i + 1, PageEnd: i + 1}
	}
	return out
}

func pending(id int64, url string) retrieve.PendingTender {
	return retrieve.PendingTender{ID: id, DocumentsURL: url, Country: "IT"}
}

// TestNoFilesIsOnlyProducibleByANamedPlatformParser is THE invariant of this
// pass, asserted mechanically rather than described.
//
// An empty listing means two completely different things depending on which
// enumerator produced it. From a named platform parser it is a positive claim —
// this buyer published no documents. From the generic fallback it is an
// admission: the fallback cannot read JavaScript-rendered tables, form-POST
// downloads or per-document detail pages, so finding nothing is evidence about
// the parser and none at all about the page.
//
// Collapsing the two would tell a bid manager "no documents" about a tender
// whose disciplinare is sitting there behind an XHR — the exact
// inaccessible-mistaken-for-absent lie the whole feature exists to remove.
func TestNoFilesIsOnlyProducibleByANamedPlatformParser(t *testing.T) {
	for _, tc := range []struct {
		name       string
		platform   string
		wantStatus string
		wantCounts bool
	}{
		{"a named platform parser read an empty listing", "tuttogare", retrieve.StatusNoFiles, true},
		{"the generic fallback found nothing", retrieve.PlatformGeneric, retrieve.StatusTraversalFailed, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeRepo{pending: []retrieve.PendingTender{pending(1, "https://portal.example/gare/1")}}
			fetcher := &fakeFetcher{responses: map[string]response{
				"https://portal.example/gare/1": {doc: htmlPage("https://portal.example/gare/1")},
			}}
			r := retrieve.New(repo, fetcher, fakeEnumerator{listing: retrieve.Listing{Platform: tc.platform}}, fakeExtractor{})

			if _, err := r.RunOnce(context.Background()); err != nil {
				t.Fatalf("RunOnce: %v", err)
			}

			out := repo.finish(t, 1)
			if out.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", out.Status, tc.wantStatus)
			}
			if out.Platform != tc.platform {
				t.Errorf("Platform = %q, want %q", out.Platform, tc.platform)
			}
			found, _ := out.Counts()
			if gotCounts := found != nil; gotCounts != tc.wantCounts {
				t.Errorf("counts written = %v, want %v — a count from the fallback would claim the buyer published nothing", gotCounts, tc.wantCounts)
			}
		})
	}
}

// TestCountsAreWrittenOnlyWhenAListingWasRead pins the other half of the
// coverage-versus-cause rule, at the level where it is decided.
//
// A tender whose portal refused us has no file count, because we never saw its
// listing. Writing 0 there would say the buyer published nothing — the same lie
// as above, arriving through a different column.
func TestCountsAreWrittenOnlyWhenAListingWasRead(t *testing.T) {
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{retrieve.StatusOK, true},
		{retrieve.StatusNoFiles, true},
		{retrieve.StatusExtractFailed, true},
		{retrieve.StatusRobotsDenied, false},
		{retrieve.StatusCaptcha, false},
		{retrieve.StatusAuthRequired, false},
		{retrieve.StatusTraversalFailed, false},
		{retrieve.StatusUnreachable, false},
	} {
		t.Run(tc.status, func(t *testing.T) {
			out := retrieve.Outcome{Status: tc.status, FilesFound: 7, FilesExtracted: 2}
			found, extracted := out.Counts()
			if got := found != nil; got != tc.want {
				t.Fatalf("counts written = %v, want %v", got, tc.want)
			}
			if found == nil {
				return
			}
			if *found != 7 || *extracted != 2 {
				t.Errorf("counts = %d/%d, want 7/2", *found, *extracted)
			}
		})
	}
}

// TestARefusalACaptchaAndAnEmptyListingProduceThreeDifferentRows is the
// invariant stated as its consequence: three tenders nobody could read produce
// three DIFFERENT persisted rows, because they are three different sentences to
// a human and three different people can act on them.
func TestARefusalACaptchaAndAnEmptyListingProduceThreeDifferentRows(t *testing.T) {
	repo := &fakeRepo{pending: []retrieve.PendingTender{
		pending(1, "https://a.example/gare"),
		pending(2, "https://b.example/gare"),
		pending(3, "https://c.example/gare"),
	}}
	fetcher := &fakeFetcher{responses: map[string]response{
		"https://a.example/gare": {err: fmt.Errorf("a.example: %w", retrieve.ErrRobotsDenied)},
		"https://b.example/gare": {err: fmt.Errorf("b.example: %w", retrieve.ErrCaptcha)},
		"https://c.example/gare": {doc: htmlPage("https://c.example/gare")},
	}}
	r := retrieve.New(repo, fetcher,
		fakeEnumerator{listing: retrieve.Listing{Platform: "tuttogare"}}, fakeExtractor{})

	res, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Permanent != 3 {
		t.Fatalf("Permanent = %d, want 3 (%+v)", res.Permanent, res)
	}

	got := map[int64]string{}
	repo.mu.Lock()
	for _, m := range repo.marks {
		got[m.tenderID] = m.status
		if !m.permanent {
			t.Errorf("tender %d marked transient; a refusal is a final answer", m.tenderID)
		}
	}
	for _, f := range repo.finishes {
		got[f.tenderID] = f.out.Status
	}
	repo.mu.Unlock()

	want := map[int64]string{1: retrieve.StatusRobotsDenied, 2: retrieve.StatusCaptcha, 3: retrieve.StatusNoFiles}
	for id, wantStatus := range want {
		if got[id] != wantStatus {
			t.Errorf("tender %d status = %q, want %q", id, got[id], wantStatus)
		}
	}
}

// TestAFileWithNoUsableTextIsNeverPersisted is the sharpest constraint in this
// feature, in its executable form.
//
// The indexer treats a document row with no extracted parts as "fetch this URL
// yourself", using a fetcher with no robots handling, no per-host pacing and no
// dial guard. A scanned PDF — of which there will be many, since this service
// builds without cgo and therefore without OCR — extracts to nothing. If that
// produced a row, the unguarded fetcher would be pointed at a buyer portal on
// every indexing cycle, forever.
func TestAFileWithNoUsableTextIsNeverPersisted(t *testing.T) {
	repo := &fakeRepo{pending: []retrieve.PendingTender{pending(1, "https://portal.example/gare/1")}}
	fetcher := &fakeFetcher{responses: map[string]response{
		"https://portal.example/gare/1":    {doc: htmlPage("https://portal.example/gare/1")},
		"https://portal.example/scan.pdf":  {doc: pdf("https://portal.example/scan.pdf")},
		"https://portal.example/empty.pdf": {doc: pdf("https://portal.example/empty.pdf")},
	}}
	r := retrieve.New(repo, fetcher,
		fakeEnumerator{listing: retrieve.Listing{Platform: "tuttogare", Files: []retrieve.FileLink{
			{URL: "https://portal.example/scan.pdf", Label: "Disciplinare"},
			{URL: "https://portal.example/empty.pdf", Label: "Capitolato"},
		}}},
		// A scan yields a few stray glyphs rather than an error, which is why
		// the floor is a character count and not a nil check.
		fakeExtractor{byURL: map[string][]tender.DocumentPart{
			"https://portal.example/scan.pdf": {{Text: "Pag. 1"}},
		}},
	)

	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	repo.mu.Lock()
	saved := len(repo.saved)
	repo.mu.Unlock()
	if saved != 0 {
		t.Fatalf("persisted %d documents; a document row without usable text hands a portal URL to the indexer forever", saved)
	}

	out := repo.finish(t, 1)
	if out.Status != retrieve.StatusExtractFailed {
		t.Errorf("Status = %q, want %q", out.Status, retrieve.StatusExtractFailed)
	}
	if out.FilesFound != 2 || out.FilesExtracted != 0 {
		t.Errorf("counts = %d/%d, want 2/0 — the gap is what decides whether .p7m unwrapping is worth building",
			out.FilesFound, out.FilesExtracted)
	}
}

// TestFilesFoundRecordsTheListingBeforeTheCap keeps a truncation visible as a
// SQL fact rather than as an inference. A listing of forty files that we read
// fifteen of must not report fifteen, or "we chose not to look" becomes
// indistinguishable from "there was nothing more".
func TestFilesFoundRecordsTheListingBeforeTheCap(t *testing.T) {
	const listed = 40

	files := make([]retrieve.FileLink, listed)
	responses := map[string]response{
		"https://portal.example/gare/1": {doc: htmlPage("https://portal.example/gare/1")},
	}
	extractable := map[string][]tender.DocumentPart{}
	for i := range files {
		url := fmt.Sprintf("https://portal.example/f%02d.pdf", i)
		files[i] = retrieve.FileLink{URL: url, Label: fmt.Sprintf("Allegato %d", i)}
		responses[url] = response{doc: pdf(url)}
		extractable[url] = parts(2)
	}

	repo := &fakeRepo{pending: []retrieve.PendingTender{pending(1, "https://portal.example/gare/1")}}
	fetcher := &fakeFetcher{responses: responses}
	r := retrieve.New(repo, fetcher,
		fakeEnumerator{listing: retrieve.Listing{Platform: "tuttogare", Files: files}},
		fakeExtractor{byURL: extractable})

	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	out := repo.finish(t, 1)
	if out.FilesFound != listed {
		t.Errorf("FilesFound = %d, want %d (the TRUE listing size)", out.FilesFound, listed)
	}
	if out.FilesExtracted != retrieve.MaxFilesPerTender {
		t.Errorf("FilesExtracted = %d, want %d", out.FilesExtracted, retrieve.MaxFilesPerTender)
	}
	// One listing fetch plus the cap, and not one request more: the cap is a
	// promise to the buyer's server, not a post-hoc filter.
	if want := 1 + retrieve.MaxFilesPerTender; fetcher.calls() != want {
		t.Errorf("made %d requests, want %d", fetcher.calls(), want)
	}
}

// TestKeptURLsAreNormalised guards the churn that would otherwise make
// FinishRetrieval's delete-what-is-missing rule destroy and recreate every
// document row on every run: a portal that stamps a session id on its links
// publishes a different URL every visit, so the persisted address has to be the
// stable one even though the request has to be the published one.
func TestKeptURLsAreNormalised(t *testing.T) {
	const published = "https://portal.example/download?id=7&sid=abc123"

	repo := &fakeRepo{pending: []retrieve.PendingTender{pending(1, "https://portal.example/gare/1")}}
	fetcher := &fakeFetcher{responses: map[string]response{
		"https://portal.example/gare/1": {doc: htmlPage("https://portal.example/gare/1")},
		published:                       {doc: pdf(published)},
	}}
	r := retrieve.New(repo, fetcher,
		fakeEnumerator{listing: retrieve.Listing{Platform: "tuttogare", Files: []retrieve.FileLink{
			{URL: published, Label: "Disciplinare di gara"},
		}}},
		fakeExtractor{byURL: map[string][]tender.DocumentPart{published: parts(3)}})

	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	const want = "https://portal.example/download?id=7"
	out := repo.finish(t, 1)
	if len(out.KeptURLs) != 1 || out.KeptURLs[0] != want {
		t.Errorf("KeptURLs = %v, want [%q]", out.KeptURLs, want)
	}
	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.saved) != 1 || repo.saved[0].url != want {
		t.Fatalf("saved = %+v, want one row at %q", repo.saved, want)
	}
	// The REQUEST kept the session parameter: a download endpoint may well
	// require the very token normalisation strips.
	if fetcher.requested[1] != published {
		t.Errorf("requested %q, want the published address %q", fetcher.requested[1], published)
	}
}

// TestEntryURLThatIsItselfAFileIsExtractedDirectly covers the best case rather
// than an edge one: a documents_url pointing straight at a capitolato is a
// tender we can read with one request.
func TestEntryURLThatIsItselfAFileIsExtractedDirectly(t *testing.T) {
	const entry = "https://portal.example/allegati/capitolato.pdf"

	repo := &fakeRepo{pending: []retrieve.PendingTender{pending(1, entry)}}
	fetcher := &fakeFetcher{responses: map[string]response{entry: {doc: pdf(entry)}}}
	r := retrieve.New(repo, fetcher,
		fakeEnumerator{err: errors.New("Enumerate must not be called for a document")},
		fakeExtractor{byURL: map[string][]tender.DocumentPart{entry: parts(4)}})

	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	out := repo.finish(t, 1)
	if out.Status != retrieve.StatusOK || out.FilesFound != 1 || out.FilesExtracted != 1 {
		t.Errorf("outcome = %+v, want ok with 1/1", out)
	}
	if out.Platform != "" {
		t.Errorf("Platform = %q, want empty — no page was parsed, so no enumerator produced this file", out.Platform)
	}
	if out.PartsWritten != 4 {
		t.Errorf("PartsWritten = %d, want 4", out.PartsWritten)
	}
}

// TestTEDDocumentsLinkCostsNoRequest keeps this pass from re-ingesting the
// notice PDF that is already in the corpus under type='notice', which would
// double-count the corpus against itself and spend requests on a host it has no
// business reading.
func TestTEDDocumentsLinkCostsNoRequest(t *testing.T) {
	repo := &fakeRepo{pending: []retrieve.PendingTender{
		pending(1, "https://ted.europa.eu/en/notice/-/detail/545620-2026"),
	}}
	fetcher := &fakeFetcher{responses: map[string]response{}}
	r := retrieve.New(repo, fetcher, fakeEnumerator{}, fakeExtractor{})

	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if fetcher.calls() != 0 {
		t.Errorf("made %d requests to ted.europa.eu, want 0", fetcher.calls())
	}
	if out := repo.finish(t, 1); out.Status != retrieve.StatusNoFiles {
		t.Errorf("Status = %q, want %q", out.Status, retrieve.StatusNoFiles)
	}
}

// TestTransientFailuresSpendTheAttemptBudgetAndThenPark pins the retry policy:
// a row that cannot be reached stays queued while it has budget, and parks once
// it does not — which is what stops "again next pass" from meaning "forever".
func TestTransientFailuresSpendTheAttemptBudgetAndThenPark(t *testing.T) {
	repo := &fakeRepo{pending: []retrieve.PendingTender{pending(1, "https://slow.example/gare")}}
	fetcher := &fakeFetcher{responses: map[string]response{
		"https://slow.example/gare": {err: fmt.Errorf("slow.example: %w", retrieve.ErrRobotsUnknown)},
	}}
	r := retrieve.New(repo, fetcher, fakeEnumerator{}, fakeExtractor{})

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		res, err := r.RunOnce(context.Background())
		if err != nil {
			t.Fatalf("attempt %d: RunOnce: %v", attempt, err)
		}
		wantPermanent := attempt == maxAttempts
		if (res.Permanent == 1) != wantPermanent {
			t.Fatalf("attempt %d: Permanent = %d, want permanent=%v", attempt, res.Permanent, wantPermanent)
		}
		if !wantPermanent && repo.attempts(1) != attempt {
			t.Fatalf("attempt %d: stored attempts = %d, want %d", attempt, repo.attempts(1), attempt)
		}
	}

	repo.mu.Lock()
	defer repo.mu.Unlock()
	if len(repo.marks) != maxAttempts {
		t.Fatalf("marks = %+v, want %d", repo.marks, maxAttempts)
	}
	for i, m := range repo.marks {
		if m.status != retrieve.StatusUnreachable {
			t.Errorf("mark %d status = %q, want %q", i, m.status, retrieve.StatusUnreachable)
		}
	}
	if repo.marks[maxAttempts-1].permanent != true {
		t.Error("the last attempt did not park the row; it would be re-listed forever")
	}
	// ErrRobotsUnknown must never become ErrRobotsDenied: a robots.txt we could
	// not read is an absence of information, not a refusal.
	for _, m := range repo.marks {
		if m.status == retrieve.StatusRobotsDenied {
			t.Fatal("an unreadable robots.txt was reported as a refusal")
		}
	}
}

// TestOneDeadHostDoesNotStopTheOthers is why the breaker is per host rather
// than global. One comune's server being down must cost that comune's tenders
// and nothing else — and it must stop costing requests once it is established.
func TestOneDeadHostDoesNotStopTheOthers(t *testing.T) {
	const deadTenders = 12

	repo := &fakeRepo{}
	responses := map[string]response{}
	for i := range deadTenders {
		url := fmt.Sprintf("https://dead.example/gare/%d", i)
		repo.pending = append(repo.pending, pending(int64(i+1), url))
		responses[url] = response{err: errors.New("connection refused")}
	}
	const healthy = "https://healthy.example/allegato.pdf"
	repo.pending = append(repo.pending, pending(99, healthy))
	responses[healthy] = response{doc: pdf(healthy)}

	fetcher := &fakeFetcher{responses: responses}
	r := retrieve.New(repo, fetcher, fakeEnumerator{},
		fakeExtractor{byURL: map[string][]tender.DocumentPart{healthy: parts(2)}})

	res, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if out := repo.finish(t, 99); out.Status != retrieve.StatusOK {
		t.Errorf("the healthy host's tender ended %q, want %q", out.Status, retrieve.StatusOK)
	}
	if res.Skipped == 0 {
		t.Error("no tender was skipped; the dead host was asked for all 12")
	}
	// Once the breaker is established the remaining rows cost nothing. The
	// bound is loose on purpose: workers already in flight when it opens are
	// not recalled, so the waste is at most one full concurrency window plus
	// the evidence the breaker needed.
	if max := maxConcurrentTenders + hostFailureLimit; res.Transient > max {
		t.Errorf("attempted %d rows against a dead host, want at most %d", res.Transient, max)
	}
}

// TestDrainHoldsBackTransientRowsForTheRestOfTheRun protects the attempt budget
// from being spent inside a single run.
//
// The queue is ordered by id and a transiently-failed row keeps its place at the
// head, so without the deferral it is re-listed by the very next batch seconds
// later, and its whole three-attempt budget goes on one portal's bad minute.
// Nothing ever restores an exhausted budget except a new publication number.
func TestDrainHoldsBackTransientRowsForTheRestOfTheRun(t *testing.T) {
	repo := &fakeRepo{maxLists: 10, pending: []retrieve.PendingTender{
		pending(1, "https://flaky.example/gare"),
		pending(2, "https://good.example/allegato.pdf"),
	}}
	fetcher := &fakeFetcher{responses: map[string]response{
		"https://flaky.example/gare":        {err: errors.New("i/o timeout")},
		"https://good.example/allegato.pdf": {doc: pdf("https://good.example/allegato.pdf")},
	}}
	r := retrieve.New(repo, fetcher, fakeEnumerator{},
		fakeExtractor{byURL: map[string][]tender.DocumentPart{
			"https://good.example/allegato.pdf": parts(2),
		}})

	if err := r.Drain(context.Background()); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	if got := repo.attempts(1); got != 1 {
		t.Errorf("stored attempts = %d, want 1 — the drain spent more than one attempt in a single run", got)
	}
	if out := repo.finish(t, 2); out.Status != retrieve.StatusOK {
		t.Errorf("the healthy tender ended %q, want %q", out.Status, retrieve.StatusOK)
	}
}

// TestAFailedBookkeepingWriteIsNotProgress keeps a database outage from looking
// like a draining queue. If FinishRetrieval fails, the row was not actually
// taken out of the queue whatever the pass intended, so it must not count as
// advanced — otherwise Drain terminates on a queue it never moved.
func TestAFailedBookkeepingWriteIsNotProgress(t *testing.T) {
	repo := &fakeRepo{
		pending:   []retrieve.PendingTender{pending(1, "https://portal.example/gare/1")},
		finishErr: errors.New("connection reset by peer"),
	}
	fetcher := &fakeFetcher{responses: map[string]response{
		"https://portal.example/gare/1": {doc: htmlPage("https://portal.example/gare/1")},
	}}
	r := retrieve.New(repo, fetcher,
		fakeEnumerator{listing: retrieve.Listing{Platform: "tuttogare"}}, fakeExtractor{})

	res, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Advanced() != 0 {
		t.Fatalf("Advanced = %d, want 0", res.Advanced())
	}
	if res.Transient != 1 {
		t.Errorf("Transient = %d, want 1", res.Transient)
	}
}

// TestFilesFoundCountsDocumentsNotLinks pins the coverage ratio against the
// most common shape of listing markup there is.
//
// Portals link the same file more than once as a matter of routine — a row in
// the attachments table and again as a download icon beside it — and the two
// copies frequently differ by a session parameter, so the enumerator's own
// verbatim dedupe does not see them as the same link. Counting the raw slice
// therefore reported a coverage gap that did not exist: eight documents
// published as sixteen links read as "16 found, 8 extracted", and the
// observation additionally claimed the tender had hit the per-tender cap.
//
// docs_files_found over docs_files_extracted is the single number the whole
// pass is measured on and the input to deciding whether .p7m and .zip
// unwrapping is worth building. Inflating its numerator is the same
// unreadable-versus-absent conflation the statuses exist to prevent, pointed
// the other way.
func TestFilesFoundCountsDocumentsNotLinks(t *testing.T) {
	const listingURL = "https://portal.example/gare/1"

	responses := map[string]response{listingURL: {doc: htmlPage(listingURL)}}
	extractable := map[string][]tender.DocumentPart{}

	// Three documents, each published twice under a different session id — the
	// duplicate is a distinct string, so nothing upstream of dedupeFiles can
	// tell it is the same file.
	var files []retrieve.FileLink
	for i := range 3 {
		first := fmt.Sprintf("https://portal.example/d%d.pdf?sid=aaa", i)
		second := fmt.Sprintf("https://portal.example/d%d.pdf?sid=bbb", i)
		files = append(files,
			retrieve.FileLink{URL: first, Label: fmt.Sprintf("Allegato %d", i)},
			retrieve.FileLink{URL: second, Label: fmt.Sprintf("Allegato %d", i)},
		)
		// Only the FIRST occurrence is ever requested: dedupeFiles keeps the
		// published order, so a response for the second would mean the
		// deduplication did not happen at all.
		responses[first] = response{doc: pdf(first)}
		extractable[first] = parts(2)
	}

	repo := &fakeRepo{pending: []retrieve.PendingTender{pending(1, listingURL)}}
	fetcher := &fakeFetcher{responses: responses}
	r := retrieve.New(repo, fetcher,
		fakeEnumerator{listing: retrieve.Listing{Platform: "tuttogare", Files: files}},
		fakeExtractor{byURL: extractable})

	if _, err := r.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	out := repo.finish(t, 1)
	if out.FilesFound != 3 {
		t.Errorf("FilesFound = %d, want 3 — the buyer published three documents, not six links", out.FilesFound)
	}
	if out.FilesExtracted != 3 {
		t.Errorf("FilesExtracted = %d, want 3", out.FilesExtracted)
	}
	if want := 1 + 3; fetcher.calls() != want {
		t.Errorf("made %d requests, want %d — a duplicate link is a request spent twice", fetcher.calls(), want)
	}
}

// blockingFetcher answers some URLs immediately and parks on the rest until the
// caller's context expires. It is how a portal that is merely slow is expressed
// deterministically, with no sleeps and no timing assumptions.
type blockingFetcher struct {
	responses map[string]response
	block     map[string]bool

	mu       sync.Mutex
	requests int
}

func (f *blockingFetcher) Fetch(ctx context.Context, url string) (retrieve.Document, error) {
	f.mu.Lock()
	f.requests++
	f.mu.Unlock()

	if f.block[url] {
		<-ctx.Done()
		return retrieve.Document{}, ctx.Err()
	}
	r, ok := f.responses[url]
	if !ok {
		return retrieve.Document{}, retrieve.ErrNotFound
	}
	return r.doc, r.err
}

// TestAPartlyReadTenderIsNotAFinishedMeasurement covers the per-tender deadline
// expiring AFTER some documents were already read.
//
// The all-or-nothing case was always handled; this is the one that was not, and
// it fails in the more expensive direction. A tender whose four-minute budget
// ran out after two of four files used to be finished as ok with 4/2 and
// docs_fetched_at stamped: it left the queue permanently, and the two files
// nobody ever requested were recorded as found-but-unextractable. That is
// exactly the population extract_failed exists to size, so the record does not
// merely lose information, it manufactures the wrong kind.
//
// A partial read is not a measurement of the listing, so the whole tender is
// charged as an ordinary transient failure: it keeps the files it already
// persisted, stays queued, and is re-read next pass.
func TestAPartlyReadTenderIsNotAFinishedMeasurement(t *testing.T) {
	const listingURL = "https://portal.example/gare/1"

	files := []retrieve.FileLink{
		{URL: "https://portal.example/fast-1.pdf", Label: "Disciplinare"},
		{URL: "https://portal.example/fast-2.pdf", Label: "Capitolato"},
		{URL: "https://portal.example/slow-1.pdf", Label: "Allegato 1"},
		{URL: "https://portal.example/slow-2.pdf", Label: "Allegato 2"},
	}
	responses := map[string]response{listingURL: {doc: htmlPage(listingURL)}}
	extractable := map[string][]tender.DocumentPart{}
	for _, f := range files[:2] {
		responses[f.URL] = response{doc: pdf(f.URL)}
		extractable[f.URL] = parts(2)
	}

	repo := &fakeRepo{pending: []retrieve.PendingTender{pending(1, listingURL)}}
	fetcher := &blockingFetcher{
		responses: responses,
		block: map[string]bool{
			files[2].URL: true,
			files[3].URL: true,
		},
	}
	r := retrieve.New(repo, fetcher,
		fakeEnumerator{listing: retrieve.Listing{Platform: "tuttogare", Files: files}},
		fakeExtractor{byURL: extractable},
		retrieve.WithPerTenderTimeout(100*time.Millisecond))

	res, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Transient != 1 {
		t.Fatalf("Transient = %d, want 1 (%+v)", res.Transient, res)
	}

	repo.mu.Lock()
	finishes := len(repo.finishes)
	marks := append([]markCall(nil), repo.marks...)
	repo.mu.Unlock()

	if finishes != 0 {
		t.Fatalf("FinishRetrieval was called %d times; a partly-read listing must not be stamped as read", finishes)
	}
	if len(marks) != 1 {
		t.Fatalf("MarkDocumentsFailed calls = %d, want 1", len(marks))
	}
	if marks[0].status != retrieve.StatusUnreachable {
		t.Errorf("status = %q, want %q", marks[0].status, retrieve.StatusUnreachable)
	}
	if marks[0].permanent {
		t.Errorf("marked permanent; a tender that ran out of time has attempts left to spend")
	}

	// The two files that WERE read stay persisted. They are third-party
	// requests, and re-spending them is the cost this asymmetry avoids.
	repo.mu.Lock()
	saved := len(repo.saved)
	repo.mu.Unlock()
	if saved != 2 {
		t.Errorf("persisted %d documents, want 2 — files already read must survive the transient outcome", saved)
	}
}
