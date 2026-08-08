// Package document is this service's outbound-HTTP capability: it downloads
// what a tender points at, and extracts text from the attachments it can read
// (currently PDF only).
//
// Two fetchers live here, and they share exactly one thing — the retry,
// backoff and Retry-After machinery at the bottom of this file. That machinery
// encodes what is worth retrying against a throttling upstream and, more
// importantly, what is not: a 404 is never retried, because treating absence
// as transient is the shortest path to an infinite loop against a service that
// has already given a final answer. Anything fetching over HTTP from this
// service should reuse it rather than grow its own.
//
// In every other respect the two are deliberately different:
//
//   - Fetch downloads a tender attachment (a PDF) from wherever the notice
//     said it lives, to be extracted for the search index.
//   - NoticeXMLClient (notice_xml.go) downloads one eForms notice document
//     from the EU's own register, to be parsed into structured detail.
//
// # The scope fence on NoticeXMLClient
//
// NoticeXMLClient fetches ted.europa.eu and nothing else, and enforces that
// rather than merely documenting it: any URL whose scheme+host is not
// https://ted.europa.eu is rejected before a request is made, and again on
// every redirect hop. The second half is why that client carries its own
// *http.Client instead of the one below — Fetch has to follow redirects
// wherever a buyer's portal sends it, which is the exact policy the fence
// cannot allow.
//
// This matters because the enrichment pass stores buyer-portal links
// (documents_url, submission_url) that it deliberately does not follow.
// Following them is a later phase's work and carries obligations this path
// does not have and is not audited for: this service has no robots.txt
// handling anywhere in it (the user agent below is politeness, not
// compliance), and an arbitrary-URL fetcher driven by addresses that arrive
// from an external feed is an SSRF primitive pointed at whatever network the
// pod sits in. The allowlist is one comparison today and is the whole
// difference between a scoped fetcher and an unaudited egress primitive later,
// so do not relax it to "make the fetcher reusable" — a second scope gets a
// second fetcher, with its own robots and SSRF story.
//
// That second fetcher now exists: internal/adapter/webdoc reads buyer portals
// under robots.txt, with per-host pacing and a dial guard. It reuses the retry
// machinery below through the exported wrappers in retry.go, and nothing else
// from this package. The obligation runs the other way too, and it is the
// sharpest constraint either package carries: FETCH MUST NEVER BE POINTED AT A
// BUYER PORTAL. The indexer reaches for it whenever a document row has no
// extracted parts (internal/adapter/index), so a portal document persisted
// without its text would hand this unguarded fetcher a portal URL on every
// indexing cycle, forever — which is why webdoc extracts text itself rather
// than merely recording URLs for the indexer to fetch later.
package document

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// userAgent identifies the ingestion crawler to origin servers. TED's public
// site (ted.europa.eu) throttles the default Go-http-client user agent more
// aggressively, so we present a descriptive one carrying a contact URL.
const userAgent = "tendersbay-ingestion/1.0 (+https://tendersbay.xyz)"

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Retry tunables are package-level vars (not consts) so tests can shrink the
// delays; production keeps the defaults below.
var (
	// maxAttempts bounds total tries — one initial attempt plus retries.
	// TED throttling clears quickly, so a handful of attempts is plenty.
	maxAttempts = 4
	// baseBackoff is the first retry delay; it doubles on each further attempt.
	baseBackoff = time.Second
	// maxBackoff caps any single wait — including a server-sent Retry-After —
	// so a large or hostile value can't stall the whole indexing run.
	maxBackoff = 30 * time.Second
)

// Fetch downloads url to a temp file and returns its path. The caller must
// call the returned cleanup function (typically via defer) to remove the
// temp file once done with it.
//
// Transient failures are retried with exponential backoff: TED throttling
// (HTTP 429), transient gateway/upstream errors (502/503/504), and
// transport-level errors. A server-sent Retry-After header (delta-seconds or
// HTTP-date) overrides the computed delay, clamped to maxBackoff. Permanent
// responses (e.g. 404) fail immediately, and ctx cancellation aborts any
// pending backoff wait.
func Fetch(ctx context.Context, url string) (path string, cleanup func(), err error) {
	for attempt := 0; ; attempt++ {
		last := attempt >= maxAttempts-1

		req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if reqErr != nil {
			return "", nil, reqErr
		}
		req.Header.Set("User-Agent", userAgent)

		resp, doErr := httpClient.Do(req)
		if doErr != nil {
			// Transport error (connection reset, timeout). A GET is
			// idempotent, so retry — unless the context is done (in which
			// case doErr wraps ctx.Err()) or we're out of attempts.
			if last || ctx.Err() != nil {
				return "", nil, doErr
			}
			if waitErr := wait(ctx, backoff(attempt, 0)); waitErr != nil {
				return "", nil, waitErr
			}
			continue
		}

		if resp.StatusCode >= 400 {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			retryable := retryableStatus(resp.StatusCode)
			resp.Body.Close()
			if !retryable || last {
				return "", nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
			}
			if waitErr := wait(ctx, backoff(attempt, retryAfter)); waitErr != nil {
				return "", nil, waitErr
			}
			continue
		}

		path, cleanup, err = save(resp.Body)
		resp.Body.Close()
		return path, cleanup, err
	}
}

// save streams body to a temp file and returns its path plus a cleanup
// function that removes it.
func save(body io.Reader) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "ingestion-doc-*.pdf")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.Remove(f.Name()) }

	if _, copyErr := io.Copy(f, body); copyErr != nil {
		_ = f.Close()
		cleanup()
		return "", nil, copyErr
	}
	if closeErr := f.Close(); closeErr != nil {
		cleanup()
		return "", nil, closeErr
	}
	return f.Name(), cleanup, nil
}

// retryableStatus reports whether an HTTP status is worth retrying: TED
// throttling (429) and transient gateway/upstream failures (502/503/504).
// Permanent 4xx such as 404 are not retried.
func retryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// parseRetryAfter reads a Retry-After header value in either supported form —
// delta-seconds ("120") or an HTTP-date — and returns the delay. It returns 0
// when the header is absent, malformed, non-positive, or already in the past.
func parseRetryAfter(v string) time.Duration {
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// backoff returns the wait before the next attempt: a positive Retry-After
// wins, otherwise an exponential delay from baseBackoff doubling per attempt.
// The result is clamped to maxBackoff (a shift overflow also lands on the cap).
func backoff(attempt int, retryAfter time.Duration) time.Duration {
	d := retryAfter
	if d <= 0 {
		d = baseBackoff << attempt
	}
	if d <= 0 || d > maxBackoff {
		d = maxBackoff
	}
	return d
}

// wait sleeps for d, returning early with ctx.Err() if ctx is cancelled first.
func wait(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
