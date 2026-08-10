// White-box tests for the robots-aware fetcher. They live in package webdoc
// (not webdoc_test) for two reasons that are both structural rather than
// convenient: the dial guard refuses 127.0.0.1, which is the only address an
// httptest server ever listens on, and the byte bounds are sized for production
// rather than for a loopback test. Both escape hatches are unexported so that
// no caller outside this package can reach them.
package webdoc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// portal is a stand-in buyer portal: a robots.txt of our choosing plus a
// handler for everything else.
type portal struct {
	*httptest.Server
	requests atomic.Int32
}

// newPortal starts a server that answers /robots.txt with robotsBody (or 404
// when it is empty, which is the "no rules published" case) and everything else
// with handler.
func newPortal(t *testing.T, robotsBody string, handler http.HandlerFunc) *portal {
	t.Helper()
	p := &portal{}
	p.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.requests.Add(1)
		if r.URL.Path == "/robots.txt" {
			if robotsBody == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte(robotsBody))
			return
		}
		handler(w, r)
	}))
	t.Cleanup(p.Close)
	return p
}

// newTestClient builds a client that can reach a loopback server and does not
// sleep production delays. Everything else — the robots handling, the redirect
// policy, the bounds, the classification — is the production path.
func newTestClient(t *testing.T, opts ...Option) *Client {
	t.Helper()
	base := []Option{
		allowPrivateAddresses(),
		WithHostGap(0),
		withBackoff(func(int, time.Duration) time.Duration { return time.Millisecond }),
	}
	return NewClient(append(base, opts...)...)
}

func servePDF(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="disciplinare.pdf"`)
	_, _ = w.Write([]byte("%PDF-1.4 disciplinare di gara"))
}

func TestFetchAllowedByRobots(t *testing.T) {
	p := newPortal(t, "User-agent: *\nDisallow: /admin\n", servePDF)
	c := newTestClient(t)

	doc, err := c.Fetch(context.Background(), p.URL+"/gare/123/disciplinare.pdf")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := string(doc.Body); got != "%PDF-1.4 disciplinare di gara" {
		t.Errorf("Body = %q", got)
	}
	if doc.ContentType != "application/pdf" {
		t.Errorf("ContentType = %q, want application/pdf", doc.ContentType)
	}
	// Content-Disposition wins over the path, which is what makes a download
	// endpoint with no extension nameable at all.
	if doc.Filename != "disciplinare.pdf" {
		t.Errorf("Filename = %q, want disciplinare.pdf", doc.Filename)
	}
	if doc.IsHTML() {
		t.Error("IsHTML = true for application/pdf")
	}
}

// TestFetchDeniedByRobotsCostsNoRequestToTheResource is the politeness contract
// in one assertion. A denial that still fetched the document would be a
// compliance theatre: we would have taken the thing we were told not to take
// and then discarded it.
func TestFetchDeniedByRobotsCostsNoRequestToTheResource(t *testing.T) {
	p := newPortal(t, portaleAppalti, func(http.ResponseWriter, *http.Request) {
		t.Error("the resource was requested despite a robots.txt denial")
	})
	c := newTestClient(t)

	_, err := c.Fetch(context.Background(), p.URL+"/PortaleAppalti/it/procedure/codice/G00749")
	if !errors.Is(err, ErrRobotsDenied) {
		t.Fatalf("Fetch = %v, want ErrRobotsDenied", err)
	}
	if !Terminal(err) {
		t.Error("a robots denial must be terminal — asking again is itself the impoliteness")
	}
	if got := p.requests.Load(); got != 1 {
		t.Errorf("server saw %d requests, want 1 (robots.txt and nothing else)", got)
	}
}

// TestFetchRobotsIsCachedPerHost pins the cache. Re-reading robots.txt before
// each of a tender's fifteen files would double this package's request count
// against every portal it touches.
func TestFetchRobotsIsCachedPerHost(t *testing.T) {
	var robotsHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			robotsHits.Add(1)
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /admin\n"))
			return
		}
		servePDF(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t)
	for i := range 4 {
		if _, err := c.Fetch(context.Background(), fmt.Sprintf("%s/gare/%d.pdf", srv.URL, i)); err != nil {
			t.Fatalf("Fetch %d: %v", i, err)
		}
	}
	if got := robotsHits.Load(); got != 1 {
		t.Errorf("robots.txt fetched %d times, want 1", got)
	}
}

// TestFetchRobots404IsPermission and TestFetchRobots503IsNotPermission are the
// two halves of deviation 2, and they are the pair that matters most in this
// file. tedmcp maps every non-200 to "no rules published"; that is right for the
// first and catastrophic for the second, because it would file a struggling
// server under "this portal forbids us" — or worse, crawl a portal that had
// simply failed to serve its own refusal.
func TestFetchRobots404IsPermission(t *testing.T) {
	p := newPortal(t, "", servePDF)
	c := newTestClient(t)

	if _, err := c.Fetch(context.Background(), p.URL+"/gare/123.pdf"); err != nil {
		t.Fatalf("Fetch = %v, want success — a 404 robots.txt means no rules are published", err)
	}
}

func TestFetchRobots503IsNotPermission(t *testing.T) {
	var resourceHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		resourceHits.Add(1)
		servePDF(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t)
	_, err := c.Fetch(context.Background(), srv.URL+"/gare/123.pdf")
	if !errors.Is(err, ErrRobotsUnknown) {
		t.Fatalf("Fetch = %v, want ErrRobotsUnknown", err)
	}
	if errors.Is(err, ErrRobotsDenied) {
		t.Error("an unreadable robots.txt was reported as a denial — the two must never merge")
	}
	if Terminal(err) {
		t.Error("ErrRobotsUnknown must be transient: it is an absence of information, not a decision")
	}
	if got := resourceHits.Load(); got != 0 {
		t.Errorf("the resource was fetched %d times without knowing the rules", got)
	}
}

// TestFetchRobotsFailureIsNotCached follows from ErrRobotsUnknown being
// transient: caching it would turn one bad minute on one server into a whole
// run of refusals for every tender behind that host.
func TestFetchRobotsFailureIsNotCached(t *testing.T) {
	var robotsHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			if robotsHits.Add(1) == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /admin\n"))
			return
		}
		servePDF(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t)
	if _, err := c.Fetch(context.Background(), srv.URL+"/gare/1.pdf"); !errors.Is(err, ErrRobotsUnknown) {
		t.Fatalf("first Fetch = %v, want ErrRobotsUnknown", err)
	}
	if _, err := c.Fetch(context.Background(), srv.URL+"/gare/2.pdf"); err != nil {
		t.Fatalf("second Fetch = %v, want success once robots.txt became readable", err)
	}
}

// TestFetchOversizedRobotsIsNotPermission applies the truncation lesson one
// layer up from the document read. A robots.txt cut short parses into a SHORTER
// rule list, so the byte that got cut could be the Disallow governing the path
// we were about to take: truncation here fails open, which is the one direction
// it must never fail.
func TestFetchOversizedRobotsIsNotPermission(t *testing.T) {
	oldLimit := maxRobotsBytes
	maxRobotsBytes = 64
	t.Cleanup(func() { maxRobotsBytes = oldLimit })

	body := "User-agent: *\nAllow: /\n" + strings.Repeat("# padding\n", 32) + "Disallow: /gare\n"
	p := newPortal(t, body, func(http.ResponseWriter, *http.Request) {
		t.Error("the resource was requested with a robots.txt we could not read in full")
	})
	c := newTestClient(t)

	_, err := c.Fetch(context.Background(), p.URL+"/gare/123.pdf")
	if !errors.Is(err, ErrRobotsUnknown) {
		t.Fatalf("Fetch = %v, want ErrRobotsUnknown for an oversized robots.txt", err)
	}
}

// TestFetchCrawlDelayIsHonoured is the other half of deviation 1: parsing the
// directive is worthless unless the pacer actually slows down for it.
func TestFetchCrawlDelayIsHonoured(t *testing.T) {
	p := newPortal(t, "User-agent: *\nCrawl-delay: 0.12\n", servePDF)
	c := newTestClient(t) // host gap 0 — every wait below comes from the site's own directive

	start := time.Now()
	for i := range 2 {
		if _, err := c.Fetch(context.Background(), fmt.Sprintf("%s/gare/%d.pdf", p.URL, i)); err != nil {
			t.Fatalf("Fetch %d: %v", i, err)
		}
	}
	// The delay only exists once robots.txt has been read, so the robots load
	// and the first document take slots that were reserved while the gap was
	// still zero; the second document is the first request the published delay
	// governs. Asserting only that one lower bound keeps this from being a
	// timing test of the machine it runs on.
	if elapsed := time.Since(start); elapsed < 100*time.Millisecond {
		t.Errorf("two fetches took %s, want at least the published 120ms Crawl-delay between them", elapsed)
	}
}

// TestFetchPacesPerHost covers the floor that applies when a site publishes no
// Crawl-delay at all, which is the overwhelming majority of them.
func TestFetchPacesPerHost(t *testing.T) {
	p := newPortal(t, "", servePDF)
	c := newTestClient(t, WithHostGap(80*time.Millisecond))

	start := time.Now()
	if _, err := c.Fetch(context.Background(), p.URL+"/gare/1.pdf"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// The robots load takes the first slot; the document waits one gap behind it.
	if elapsed := time.Since(start); elapsed < 60*time.Millisecond {
		t.Errorf("robots + document took %s, want at least one 80ms gap between them", elapsed)
	}
}

// TestFetchIsSerialPerHost is the control that matters for the largest
// platforms, which are one host serving hundreds of buyers: two tenders in the
// same batch routinely share a host, and without this they would double that
// host's rate exactly when the corpus is densest.
func TestFetchIsSerialPerHost(t *testing.T) {
	var inFlight, peak atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := inFlight.Add(1)
		for {
			old := peak.Load()
			if n <= old || peak.CompareAndSwap(old, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
		if r.URL.Path == "/robots.txt" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		servePDF(w, r)
	}))
	defer srv.Close()

	c := newTestClient(t)
	done := make(chan error, 6)
	for i := range 6 {
		go func() {
			_, err := c.Fetch(context.Background(), fmt.Sprintf("%s/gare/%d.pdf", srv.URL, i))
			done <- err
		}()
	}
	for range 6 {
		if err := <-done; err != nil {
			t.Fatalf("Fetch: %v", err)
		}
	}
	if got := peak.Load(); got != 1 {
		t.Errorf("peak concurrent requests to one host = %d, want 1", got)
	}
}

// TestFetchStatusClassification is the vocabulary, in a table. Every row is a
// different sentence to a human, and the whole package exists so that they stay
// apart rather than collapsing into "could not fetch".
func TestFetchStatusClassification(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		want     error
		terminal bool
	}{
		{"not found", http.StatusNotFound, ErrNotFound, true},
		{"gone", http.StatusGone, ErrNotFound, true},
		{"unauthorized", http.StatusUnauthorized, ErrAuthRequired, true},
		{"forbidden is the login wall, not a rate limit", http.StatusForbidden, ErrAuthRequired, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			})
			c := newTestClient(t)

			_, err := c.Fetch(context.Background(), p.URL+"/gare/123")
			if !errors.Is(err, tt.want) {
				t.Fatalf("Fetch = %v, want %v", err, tt.want)
			}
			if Terminal(err) != tt.terminal {
				t.Errorf("Terminal(%v) = %v, want %v", err, Terminal(err), tt.terminal)
			}
		})
	}
}

// TestFetchServerErrorIsTransient pins the default direction of the contract: an
// unclassified failure costs a bounded number of retries, where guessing
// "terminal" would park rows that were only briefly unreachable.
func TestFetchServerErrorIsTransient(t *testing.T) {
	p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	c := newTestClient(t)

	_, err := c.Fetch(context.Background(), p.URL+"/gare/123")
	if err == nil {
		t.Fatal("Fetch: want an error")
	}
	if Terminal(err) {
		t.Errorf("Terminal(%v) = true, want false", err)
	}
}

func TestFetchRetriesThrottlingThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	p := newPortal(t, "", func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		servePDF(w, r)
	})
	c := newTestClient(t)

	if _, err := c.Fetch(context.Background(), p.URL+"/gare/123.pdf"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("resource requested %d times, want 3 (two 429s then success)", got)
	}
}

// TestFetchStopsAtTheAttemptBudget pins the budget itself. It is three rather
// than document.Fetch's four on purpose: there the budget is bounded by work,
// here by politeness, and every extra attempt is a request a small municipal
// server pays for.
func TestFetchStopsAtTheAttemptBudget(t *testing.T) {
	var calls atomic.Int32
	p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	})
	c := newTestClient(t)

	if _, err := c.Fetch(context.Background(), p.URL+"/gare/123"); err == nil {
		t.Fatal("Fetch: want an error once the budget is spent")
	}
	if got := calls.Load(); got != maxFetchAttempts {
		t.Errorf("resource requested %d times, want %d", got, maxFetchAttempts)
	}
}

// TestFetchAbandonsALongRetryAfter is the one deviation from document's retry
// machinery. A small server asking us to come back in five minutes is telling us
// something we should obey by LEAVING, not by holding a worker and a connection
// slot through a fifth of the run window.
func TestFetchAbandonsALongRetryAfter(t *testing.T) {
	var calls atomic.Int32
	p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	c := newTestClient(t)

	start := time.Now()
	_, err := c.Fetch(context.Background(), p.URL+"/gare/123")
	if err == nil {
		t.Fatal("Fetch: want an error")
	}
	if Terminal(err) {
		t.Errorf("Terminal(%v) = true, want false — the server asked us to come back", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("resource requested %d times, want 1 — the attempt is abandoned, not retried", got)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Fetch slept %s waiting out a 600s Retry-After", elapsed)
	}
}

// TestFetchRetryAfterWithinBoundIsObeyed is the other side of that rule: a
// short, honest throttle is still waited out rather than abandoned.
func TestFetchRetryAfterWithinBoundIsObeyed(t *testing.T) {
	var calls atomic.Int32
	p := newPortal(t, "", func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		servePDF(w, r)
	})
	c := newTestClient(t)

	if _, err := c.Fetch(context.Background(), p.URL+"/gare/123.pdf"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("resource requested %d times, want 2", got)
	}
}

// TestFetchDetectsHumanChecks covers the failure mode this package is named
// after: a 200 that is not the document. Reporting one of these as the document
// is the exact confusion the whole feature exists to remove.
func TestFetchDetectsHumanChecks(t *testing.T) {
	bodies := map[string]string{
		"friendly captcha": `<html><body><div class="frc-captcha"></div></body></html>`,
		"recaptcha":        `<html><body><div class="g-recaptcha" data-sitekey="x"></div></body></html>`,
		"hcaptcha":         `<html><body><div class="h-captcha"></div></body></html>`,
		"cloudflare":       `<html><head><title>Just a moment</title></head><body id="cf-challenge"></body></html>`,
		"waf":              `<html><body>Request Rejected</body></html>`,
		"js interstitial":  `<html><body>Please enable JavaScript and cookies to continue</body></html>`,
		"incapsula":        `<html><body><iframe src="/_Incapsula_Resource?SWKMTFSR=1"></iframe></body></html>`,
		"datadome":         `<html><body><script src="https://js.datadome.co/tags.js"></script></body></html>`,
		"aws waf":          `<html><body><script src="https://x.awswaf.com/challenge.js"></script></body></html>`,
	}
	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				_, _ = w.Write([]byte(body))
			})
			c := newTestClient(t)

			_, err := c.Fetch(context.Background(), p.URL+"/gare/123")
			if !errors.Is(err, ErrCaptcha) {
				t.Fatalf("Fetch = %v, want ErrCaptcha", err)
			}
			if !Terminal(err) {
				t.Error("a human check is terminal — we cannot solve it on a later pass either")
			}
		})
	}
}

// TestFetchDetectsALoginWallServedAs200 is the half of the auth detection that
// status codes cannot reach. The marketplace-shaped portals answer 200 with a
// login form, and a caller that only looked at status codes would enumerate that
// form, find no documents, and record the tender as publishing none.
func TestFetchDetectsALoginWallServedAs200(t *testing.T) {
	const login = `<html><body><form action="/login" method="post">
	<input type="text" name="user"><input type="password" name="pass">
	<button>Accedi</button></form></body></html>`

	p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(login))
	})
	c := newTestClient(t)

	_, err := c.Fetch(context.Background(), p.URL+"/gare/123")
	if !errors.Is(err, ErrAuthRequired) {
		t.Fatalf("Fetch = %v, want ErrAuthRequired", err)
	}
}

// TestFetchDoesNotCallAListingWithALoginHeaderALoginWall is the guard on the
// heuristic above, and it is the one that keeps it honest. Half the portals in
// this corpus render a site-wide login box in their header; claiming
// auth_required for a page that is visibly publishing its documents would be
// precisely the wrong answer, in precisely the direction that hurts.
func TestFetchDoesNotCallAListingWithALoginHeaderALoginWall(t *testing.T) {
	const listing = `<html><body>
	<header><form><input type="password" name="pass"></form></header>
	<main><a href="/files/disciplinare.pdf">Disciplinare di gara</a></main>
	</body></html>`

	p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(listing))
	})
	c := newTestClient(t)

	doc, err := c.Fetch(context.Background(), p.URL+"/gare/123")
	if err != nil {
		t.Fatalf("Fetch = %v, want the listing", err)
	}
	if !doc.IsHTML() {
		t.Errorf("IsHTML = false for %q", doc.ContentType)
	}
}

// TestFetchRefusesAnOversizedBody is the truncation lesson at the document
// layer. io.LimitReader truncates silently, and a listing page cut short parses
// cleanly into a shorter file list — a silent undercount of exactly the number
// this feature exists to measure.
func TestFetchRefusesAnOversizedBody(t *testing.T) {
	oldLimit := maxDocumentBytes
	maxDocumentBytes = 1024
	t.Cleanup(func() { maxDocumentBytes = oldLimit })

	p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(make([]byte, maxDocumentBytes+1))
	})
	c := newTestClient(t)

	_, err := c.Fetch(context.Background(), p.URL+"/gare/big.pdf")
	if !errors.Is(err, ErrUnusable) {
		t.Fatalf("Fetch = %v, want ErrUnusable", err)
	}
	if !Terminal(err) {
		t.Error("an oversized response is terminal — it will be the same size next pass")
	}
}

// TestFetchAcceptsABodyExactlyAtTheBound is the other side of the cap+1 read: a
// document of exactly the bound is legal, and an off-by-one here would reject
// the largest thing we said we would accept.
func TestFetchAcceptsABodyExactlyAtTheBound(t *testing.T) {
	oldLimit := maxDocumentBytes
	maxDocumentBytes = 1024
	t.Cleanup(func() { maxDocumentBytes = oldLimit })

	p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(make([]byte, maxDocumentBytes))
	})
	c := newTestClient(t)

	doc, err := c.Fetch(context.Background(), p.URL+"/gare/big.pdf")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(doc.Body) != maxDocumentBytes {
		t.Errorf("len(Body) = %d, want %d", len(doc.Body), maxDocumentBytes)
	}
}

func TestFetchFollowsARedirectAndReportsTheFinalURL(t *testing.T) {
	files := newPortal(t, "", servePDF)
	listing := newPortal(t, "", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, files.URL+"/cdn/disciplinare.pdf", http.StatusFound)
	})
	c := newTestClient(t)

	doc, err := c.Fetch(context.Background(), listing.URL+"/gare/123/download")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// Enumerated links are resolved against this, so reporting the address we
	// asked for rather than the one that answered would shift a whole listing's
	// links by a path segment.
	if !strings.HasPrefix(doc.URL, files.URL) {
		t.Errorf("doc.URL = %q, want the host that actually served the bytes (%s)", doc.URL, files.URL)
	}
}

// TestFetchRechecksRobotsOnEveryRedirectHop is what stops a redirect from
// quietly turning a permitted fetch into a forbidden one. Checking only the
// address we chose would enforce robots against our own intent rather than
// against the server that answered.
func TestFetchRechecksRobotsOnEveryRedirectHop(t *testing.T) {
	closed := newPortal(t, portaleAppalti, func(http.ResponseWriter, *http.Request) {
		t.Error("a redirect target that robots.txt disallows was fetched")
	})
	open := newPortal(t, "", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, closed.URL+"/PortaleAppalti/it/procedure/codice/G00749", http.StatusFound)
	})
	c := newTestClient(t)

	_, err := c.Fetch(context.Background(), open.URL+"/gare/123")
	if !errors.Is(err, ErrRobotsDenied) {
		t.Fatalf("Fetch = %v, want ErrRobotsDenied", err)
	}
}

func TestFetchRefusesARedirectLoop(t *testing.T) {
	var p *portal
	p = newPortal(t, "", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, p.URL+"/gare/123", http.StatusFound)
	})
	c := newTestClient(t)

	_, err := c.Fetch(context.Background(), p.URL+"/gare/123")
	if !errors.Is(err, ErrUnusable) {
		t.Fatalf("Fetch = %v, want ErrUnusable", err)
	}
}

// TestFetchRefusesARedirectToANonHTTPScheme covers the hop that a scheme check
// done only at parse time would miss entirely.
func TestFetchRefusesARedirectToANonHTTPScheme(t *testing.T) {
	p := newPortal(t, "", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "file:///etc/passwd", http.StatusFound)
	})
	c := newTestClient(t)

	_, err := c.Fetch(context.Background(), p.URL+"/gare/123")
	if !errors.Is(err, ErrAddressRefused) {
		t.Fatalf("Fetch = %v, want ErrAddressRefused", err)
	}
}

// TestFetchWithTheGuardOnRefusesLoopback is the end-to-end proof that the dial
// guard is actually installed on the transport rather than merely written. It is
// the only test in this file that builds a production client, and it is exactly
// because an httptest server is an address the guard must refuse.
func TestFetchWithTheGuardOnRefusesLoopback(t *testing.T) {
	p := newPortal(t, "", servePDF)
	c := NewClient(WithHostGap(0))

	_, err := c.Fetch(context.Background(), p.URL+"/gare/123.pdf")
	if !errors.Is(err, ErrAddressRefused) {
		t.Fatalf("Fetch = %v, want ErrAddressRefused for a loopback address", err)
	}
	// And it must not be misfiled as a portal problem: an address we will never
	// dial is terminal, where an unreadable robots.txt is transient.
	if errors.Is(err, ErrRobotsUnknown) {
		t.Error("a refused dial was reported as an unreadable robots.txt")
	}
	if !Terminal(err) {
		t.Error("a refused address is terminal — it will not become public on a later pass")
	}
	if got := p.requests.Load(); got != 0 {
		t.Errorf("the server saw %d requests through a guard that should have refused the dial", got)
	}
}

// TestFetchHonoursContextCancellation matters because the whole run is
// deadline-bounded: a worker blocked on a slow host has to be releasable when
// the deadline arrives, not when that host finally answers.
func TestFetchHonoursContextCancellation(t *testing.T) {
	p := newPortal(t, "", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		servePDF(w, r)
	})
	c := newTestClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	if _, err := c.Fetch(ctx, p.URL+"/gare/123.pdf"); err == nil {
		t.Fatal("Fetch: want an error once the context is done")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Fetch took %s to notice a cancelled context", elapsed)
	}
}

// TestFetchSendsTheHonestUserAgent pins the token robots.txt rules match on. It
// must never impersonate a browser or a search engine — the Maggioli finding
// behind this whole feature is that claiming to be Googlebot is the difference
// between a platform we cannot serve and fraud.
func TestFetchSendsTheHonestUserAgent(t *testing.T) {
	seen := make(chan string, 4)
	p := newPortal(t, "", func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("User-Agent")
		servePDF(w, r)
	})
	c := newTestClient(t)

	if _, err := c.Fetch(context.Background(), p.URL+"/gare/123.pdf"); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	got := <-seen
	if got != UserAgent {
		t.Fatalf("User-Agent = %q, want %q", got, UserAgent)
	}
	for _, forbidden := range []string{"googlebot", "bingbot", "mozilla", "chrome", "safari"} {
		if strings.Contains(strings.ToLower(got), forbidden) {
			t.Errorf("User-Agent %q impersonates %q", got, forbidden)
		}
	}
}

// TestFilenameFallsBackToThePathSegment covers the majority case: portals mostly
// do not send Content-Disposition, and the filename is what ranking reads to
// tell a disciplinare from a planimetria.
func TestFilenameFallsBackToThePathSegment(t *testing.T) {
	p := newPortal(t, "", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("%PDF-1.4 x"))
	})
	c := newTestClient(t)

	doc, err := c.Fetch(context.Background(), p.URL+"/gare/123/capitolato%20speciale.pdf")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if doc.Filename != "capitolato speciale.pdf" {
		t.Errorf("Filename = %q, want %q", doc.Filename, "capitolato speciale.pdf")
	}
}
