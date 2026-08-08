package document

import (
	"context"
	"time"
)

// This file is the retry, backoff and Retry-After machinery of fetch.go,
// exported for the second outbound-HTTP scope in this service.
//
// The package comment states the rule: "Anything fetching over HTTP from this
// service should reuse it rather than grow its own." The second scope it
// anticipated now exists — internal/adapter/webdoc fetches buyer portals under
// robots.txt — and it lives in another package, so it cannot reach the
// unexported helpers below their own definitions. Copying them across the
// package boundary is precisely what the rule forbids: what is worth retrying
// against a throttling upstream, and (far more important) what is not, must
// have exactly one definition. A 404 that stays terminal here and quietly
// becomes retryable in a copy is an infinite loop pointed at a stranger's
// server.
//
// The exported surface is deliberately these four pieces and no policy. Retry
// BUDGETS are not shared and must not be: maxAttempts here is calibrated for
// TED, which throttles hard and recovers fast, while webdoc's is bounded by
// politeness toward small municipal portals where every extra attempt is a
// request someone else pays for. A caller that needs a different budget, or a
// different reading of a hostile Retry-After, writes its own loop around these
// four functions rather than widening this file into a framework.

// RetryableStatus reports whether an HTTP status is worth retrying. See
// retryableStatus for the classification and why 404 is not in it.
func RetryableStatus(code int) bool { return retryableStatus(code) }

// ParseRetryAfter reads a Retry-After header value in either supported form —
// delta-seconds or an HTTP-date — and returns the delay, or 0 when the header
// is absent, malformed, non-positive or already past.
//
// It returns the server's stated delay UNCLAMPED, so a caller can tell a
// throttle worth waiting out from a server asking for five minutes. Backoff
// applies this package's clamp; a caller that wants to abandon rather than
// sleep has to see the raw value first.
func ParseRetryAfter(v string) time.Duration { return parseRetryAfter(v) }

// Backoff returns the wait before the next attempt: a positive retryAfter
// wins, otherwise an exponential delay doubling per attempt, clamped to this
// package's maximum single wait.
func Backoff(attempt int, retryAfter time.Duration) time.Duration {
	return backoff(attempt, retryAfter)
}

// Wait sleeps for d, returning early with ctx.Err() if ctx is cancelled first.
func Wait(ctx context.Context, d time.Duration) error { return wait(ctx, d) }
