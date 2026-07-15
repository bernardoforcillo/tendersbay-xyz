// Package document downloads and extracts text from a tender's notice
// document (currently PDF only).
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
