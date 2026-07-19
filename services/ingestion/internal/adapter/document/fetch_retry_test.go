// White-box tests for Fetch's retry/backoff behavior. They live in
// package document (not document_test) so they can shrink the retry delays
// and exercise the unexported helpers directly, keeping the suite fast and
// deterministic instead of sleeping real backoff durations.
package document

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// fastRetries shrinks the package-level retry tunables for the duration of a
// test and restores them afterward, so retry tests don't sleep whole seconds.
func fastRetries(t *testing.T, attempts int) {
	t.Helper()
	oa, ob, om := maxAttempts, baseBackoff, maxBackoff
	maxAttempts, baseBackoff, maxBackoff = attempts, time.Millisecond, 5*time.Millisecond
	t.Cleanup(func() { maxAttempts, baseBackoff, maxBackoff = oa, ob, om })
}

func TestFetch_RetriesThenSucceeds(t *testing.T) {
	fastRetries(t, 4)
	want := []byte("%PDF-1.4 recovered after throttling")
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 { // 429 on the first two calls, then succeed
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write(want)
	}))
	defer srv.Close()

	path, cleanup, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	defer cleanup()

	if got := calls.Load(); got != 3 {
		t.Errorf("server calls = %d, want 3 (two 429s then success)", got)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	if string(got) != string(want) {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestFetch_ExhaustsRetriesOn429(t *testing.T) {
	fastRetries(t, 3)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	_, _, err := Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("Fetch: want error after exhausting retries, got nil")
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("server calls = %d, want 3 (maxAttempts)", got)
	}
}

func TestFetch_NoRetryOnPermanentStatus(t *testing.T) {
	fastRetries(t, 4)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, _, err := Fetch(context.Background(), srv.URL)
	if err == nil {
		t.Fatal("Fetch: want error on 404, got nil")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("server calls = %d, want 1 (404 must not be retried)", got)
	}
}

func TestFetch_SetsUserAgent(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("User-Agent")
		_, _ = w.Write([]byte("%PDF-1.4"))
	}))
	defer srv.Close()

	_, cleanup, err := Fetch(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	defer cleanup()
	if seen != userAgent {
		t.Errorf("User-Agent = %q, want %q", seen, userAgent)
	}
}

func TestFetch_ContextCancelAbortsBackoff(t *testing.T) {
	// Long backoff so the test controls timing via cancellation, not the timer.
	oa, ob, om := maxAttempts, baseBackoff, maxBackoff
	maxAttempts, baseBackoff, maxBackoff = 4, time.Hour, time.Hour
	t.Cleanup(func() { maxAttempts, baseBackoff, maxBackoff = oa, ob, om })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()

	start := time.Now()
	_, _, err := Fetch(ctx, srv.URL)
	if err == nil {
		t.Fatal("Fetch: want error on context cancel, got nil")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("Fetch took %v; cancel should abort the backoff wait promptly", elapsed)
	}
}

func TestParseRetryAfter(t *testing.T) {
	future := time.Now().Add(90 * time.Second)
	cases := []struct {
		name string
		in   string
		min  time.Duration
		max  time.Duration
	}{
		{"empty", "", 0, 0},
		{"seconds", "120", 120 * time.Second, 120 * time.Second},
		{"zero", "0", 0, 0},
		{"negative", "-5", 0, 0},
		{"garbage", "soon", 0, 0},
		{"http-date-future", future.UTC().Format(http.TimeFormat), 80 * time.Second, 90 * time.Second},
		{"http-date-past", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat), 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseRetryAfter(c.in)
			if got < c.min || got > c.max {
				t.Errorf("parseRetryAfter(%q) = %v, want within [%v, %v]", c.in, got, c.min, c.max)
			}
		})
	}
}

func TestBackoff(t *testing.T) {
	ob, om := baseBackoff, maxBackoff
	baseBackoff, maxBackoff = time.Second, 30*time.Second
	t.Cleanup(func() { baseBackoff, maxBackoff = ob, om })

	if got := backoff(0, 0); got != time.Second {
		t.Errorf("backoff(0,0) = %v, want 1s", got)
	}
	if got := backoff(2, 0); got != 4*time.Second { // 1s << 2
		t.Errorf("backoff(2,0) = %v, want 4s", got)
	}
	if got := backoff(0, 5*time.Second); got != 5*time.Second { // Retry-After wins
		t.Errorf("backoff(0,5s) = %v, want 5s", got)
	}
	if got := backoff(0, time.Hour); got != 30*time.Second { // clamped to maxBackoff
		t.Errorf("backoff(0,1h) = %v, want 30s (clamped)", got)
	}
	if got := backoff(10, 0); got != 30*time.Second { // exponential clamped to maxBackoff
		t.Errorf("backoff(10,0) = %v, want 30s (clamped)", got)
	}
}
