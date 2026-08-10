// White-box tests for the notice-document fetcher. They live in package
// document (not document_test) so they can point the client's origin fence at
// a loopback server and shrink the size bound, which is the only way to
// exercise the truncation boundary without moving 8 MiB through a test server.
package document

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/enrich"
)

// smallNoticeBound shrinks the document size bound for the duration of a test
// and restores it afterwards, mirroring fastRetries.
func smallNoticeBound(t *testing.T, n int) {
	t.Helper()
	old := maxNoticeXMLBytes
	maxNoticeXMLBytes = n
	t.Cleanup(func() { maxNoticeXMLBytes = old })
}

// noticeClient returns a client fenced to srv and with pacing disabled, so the
// retry and bounds tests don't spend a quarter-second per request. Pacing is
// covered on its own by TestFetchXML_PacesRequestStarts.
func noticeClient(srv *httptest.Server) *NoticeXMLClient {
	c := NewNoticeXMLClient()
	c.origin = srv.URL // httptest URLs are exactly scheme://host
	c.pacer.interval = 0
	return c
}

// The fence is the difference between a scoped fetcher and an unaudited egress
// primitive, so it has to hold before a request is made — not merely produce a
// nicer error afterwards. The assertion that matters is the call count.
func TestFetchXML_RefusesAnOriginOutsideTheAllowlist(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte("<ContractNotice/>"))
	}))
	defer srv.Close()

	// A production client, deliberately: the fence is only meaningful if the
	// exported constructor installs it.
	c := NewNoticeXMLClient()
	c.pacer.interval = 0

	for _, url := range []string{
		srv.URL + "/notice.xml",                 // a buyer portal stand-in
		"http://ted.europa.eu/notice.xml",       // scheme downgrade of the allowed host
		"https://ted.europa.eu.evil.test/x.xml", // suffix that merely looks like the host
		"https://ted.europa.eu:8443/notice.xml", // allowed host, unexpected port
		"//ted.europa.eu/notice.xml",            // no scheme at all
		"notaurl",                               // not an absolute URL
	} {
		_, err := c.FetchXML(context.Background(), url)
		if !errors.Is(err, enrich.ErrNoticeUnusable) {
			t.Errorf("FetchXML(%q) error = %v, want enrich.ErrNoticeUnusable", url, err)
		}
	}
	if got := calls.Load(); got != 0 {
		t.Errorf("server calls = %d, want 0 — the fence must reject before dialling", got)
	}
}

// Checking the URL we asked for proves nothing about the server we read, and
// the gap between the two is one 302 wide. Go's default redirect policy follows
// up to ten hops to any host, so without a CheckRedirect the fence would be
// enforced against our own intent while an off-allowlist origin's bytes reached
// the parser regardless — which is exactly the unaudited egress primitive the
// package comment says this must never become.
//
// The assertion that matters is that the off-origin server is never READ from,
// not merely that an error comes back.
func TestFetchXML_RefusesARedirectOutsideTheAllowlist(t *testing.T) {
	var offOrigin atomic.Int32
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		offOrigin.Add(1)
		_, _ = w.Write([]byte("<ContractNotice/>"))
	}))
	defer elsewhere.Close()

	ted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+"/notice.xml", http.StatusFound)
	}))
	defer ted.Close()

	body, err := noticeClient(ted).FetchXML(context.Background(), ted.URL+"/notice.xml")
	if !errors.Is(err, enrich.ErrNoticeUnusable) {
		t.Errorf("FetchXML error = %v, want enrich.ErrNoticeUnusable", err)
	}
	if body != nil {
		t.Errorf("body = %q, want nothing read from an off-allowlist origin", body)
	}
	if got := offOrigin.Load(); got != 0 {
		t.Errorf("off-origin server calls = %d, want 0 — the fence must survive the redirect", got)
	}
	// The refusal must not leak the address it refused: the enrichment pass
	// logs these errors verbatim and has a standing no-URLs rule.
	if strings.Contains(err.Error(), elsewhere.URL) || strings.Contains(err.Error(), ted.URL) {
		t.Errorf("error %q quotes a URL", err)
	}
}

// A redirect that stays inside the allowlist is ordinary traffic — TED does
// redirect within its own site — so the fence must not turn every hop into a
// failure.
func TestFetchXML_FollowsARedirectWithinTheAllowlist(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/notice.xml" {
			http.Redirect(w, r, srv.URL+"/final.xml", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte("<ContractNotice/>"))
	}))
	defer srv.Close()

	body, err := noticeClient(srv).FetchXML(context.Background(), srv.URL+"/notice.xml")
	if err != nil {
		t.Fatalf("FetchXML: %v", err)
	}
	if string(body) != "<ContractNotice/>" {
		t.Errorf("body = %q, want the redirected document", body)
	}
}

// A redirect loop inside the allowlist passes every origin check, so the hop
// cap is the only thing that ends it.
func TestFetchXML_RefusesAnEndlessRedirectChain(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/again.xml", http.StatusFound)
	}))
	defer srv.Close()

	if _, err := noticeClient(srv).FetchXML(context.Background(), srv.URL+"/notice.xml"); !errors.Is(err, enrich.ErrNoticeUnusable) {
		t.Errorf("FetchXML error = %v, want enrich.ErrNoticeUnusable", err)
	}
}

// The fence must not accidentally reject the one origin it exists to allow.
func TestFetchXML_AllowsTheTEDOrigin(t *testing.T) {
	if err := NewNoticeXMLClient().checkOrigin("https://ted.europa.eu/en/notice/-/detail/545620-2026"); err != nil {
		t.Errorf("checkOrigin on the allowed origin = %v, want nil", err)
	}
}

// A 404 is TED's final answer that a notice has no XML document — a legacy
// pre-eForms notice, or a type it publishes no XML for. Retrying it would turn
// a corpus full of such notices into four times the traffic for the same
// answer, so the call count is as much the assertion as the error is.
func TestFetchXML_TreatsNotFoundAsAbsentAndDoesNotRetry(t *testing.T) {
	fastRetries(t, 4)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := noticeClient(srv).FetchXML(context.Background(), srv.URL+"/notice.xml")
	if !errors.Is(err, enrich.ErrNoticeAbsent) {
		t.Errorf("FetchXML error = %v, want enrich.ErrNoticeAbsent", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("server calls = %d, want 1 — a 404 must never be retried", got)
	}
}

// Throttling is the case the shared retry machinery exists for; this pins that
// the notice fetcher actually goes through it rather than failing on the first
// 429.
func TestFetchXML_RetriesThrottlingThenSucceeds(t *testing.T) {
	fastRetries(t, 4)
	want := []byte("<ContractNotice>recovered</ContractNotice>")
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	got, err := noticeClient(srv).FetchXML(context.Background(), srv.URL+"/notice.xml")
	if err != nil {
		t.Fatalf("FetchXML: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("body = %q, want %q", got, want)
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("server calls = %d, want 3 (two 429s then success)", n)
	}
}

// A retryable status that never clears must give up rather than loop, and must
// stay transient — an unclassified failure is retried on a later pass, not
// parked.
func TestFetchXML_GivesUpOnPersistentThrottlingWithATransientError(t *testing.T) {
	fastRetries(t, 3)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := noticeClient(srv).FetchXML(context.Background(), srv.URL+"/notice.xml")
	if err == nil {
		t.Fatal("FetchXML error = nil, want a failure after the attempts are spent")
	}
	if errors.Is(err, enrich.ErrNoticeAbsent) || errors.Is(err, enrich.ErrNoticeUnusable) {
		t.Errorf("FetchXML error = %v, want a transient (unclassified) error, not a terminal one", err)
	}
	if n := calls.Load(); n != 3 {
		t.Errorf("server calls = %d, want 3 (one attempt plus two retries)", n)
	}
}

// The subtle failure this fetcher exists to prevent: a document cut off at the
// bound still parses, because the notice decoder is non-strict, so a truncated
// notice would be persisted as a complete one that happens to publish fewer
// lots. One byte over the bound must therefore be a hard, terminal error and
// not a shortened body.
func TestFetchXML_RefusesADocumentOverTheBoundInsteadOfTruncatingIt(t *testing.T) {
	smallNoticeBound(t, 1024)
	oversized := strings.Repeat("x", maxNoticeXMLBytes+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(oversized))
	}))
	defer srv.Close()

	body, err := noticeClient(srv).FetchXML(context.Background(), srv.URL+"/notice.xml")
	if !errors.Is(err, enrich.ErrNoticeUnusable) {
		t.Errorf("FetchXML error = %v, want enrich.ErrNoticeUnusable", err)
	}
	if body != nil {
		t.Errorf("FetchXML returned %d bytes alongside the error; a truncated body must never reach the parser", len(body))
	}
}

// The boundary itself: a document of exactly the bound is legitimate, so the
// cap+1 read must not turn "exactly at the limit" into a refusal.
func TestFetchXML_AcceptsADocumentExactlyAtTheBound(t *testing.T) {
	smallNoticeBound(t, 1024)
	exact := strings.Repeat("y", maxNoticeXMLBytes)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(exact))
	}))
	defer srv.Close()

	body, err := noticeClient(srv).FetchXML(context.Background(), srv.URL+"/notice.xml")
	if err != nil {
		t.Fatalf("FetchXML: %v", err)
	}
	if len(body) != maxNoticeXMLBytes {
		t.Errorf("body = %d bytes, want exactly the bound (%d)", len(body), maxNoticeXMLBytes)
	}
}

// The pacer is the actual throughput ceiling against TED, and it has to hold
// across concurrent callers of one client — a per-goroutine gap would let four
// workers issue four simultaneous requests and still look correct.
func TestFetchXML_PacesRequestStarts(t *testing.T) {
	const (
		interval = 20 * time.Millisecond
		requests = 4
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<ContractNotice/>"))
	}))
	defer srv.Close()

	c := noticeClient(srv)
	c.pacer.interval = interval

	start := time.Now()
	done := make(chan error, requests)
	for range requests {
		go func() {
			_, err := c.FetchXML(context.Background(), srv.URL+"/notice.xml")
			done <- err
		}()
	}
	for range requests {
		if err := <-done; err != nil {
			t.Fatalf("FetchXML: %v", err)
		}
	}

	// The first request takes its slot immediately, so N requests cost at least
	// (N-1) intervals. Asserting the lower bound only: an upper bound would be
	// a scheduler-timing test, not a pacing one.
	if want := time.Duration(requests-1) * interval; time.Since(start) < want {
		t.Errorf("%d paced requests took %v, want at least %v", requests, time.Since(start), want)
	}
}
