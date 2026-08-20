package httpapi

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/health"
)

type stubProbe struct {
	name string
	err  error
}

func (p stubProbe) Name() string                { return p.name }
func (p stubProbe) Check(context.Context) error { return p.err }

func get(t *testing.T, h http.Handler, target string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestHealthzAlwaysOK(t *testing.T) {
	res := get(t, New(health.New(), nil), "/healthz")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("body = %q, want status ok", body)
	}
}

func TestReadyzOK(t *testing.T) {
	res := get(t, New(health.New(stubProbe{name: "a"}), nil), "/readyz")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
}

func TestReadyzUnavailable(t *testing.T) {
	svc := health.New(stubProbe{name: "db", err: errors.New("down")})
	res := get(t, New(svc, nil), "/readyz")
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `"db":"down"`) {
		t.Fatalf("body = %q, want db check", body)
	}
}

func TestLogsEachRequest(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	get(t, New(health.New(), nil), "/healthz")

	out := buf.String()
	if !strings.Contains(out, "method=GET") || !strings.Contains(out, "status=200") ||
		!strings.Contains(out, "path=/healthz") || !strings.Contains(out, "duration_ms=") {
		t.Fatalf("expected request log, got %q", out)
	}
}

type fakeUnsub struct {
	got  []string
	err  error
	hits int
}

func (f *fakeUnsub) Unsubscribe(_ context.Context, token string) error {
	f.hits++
	if f.err != nil {
		return f.err
	}
	f.got = append(f.got, token)
	return nil
}

// TestUnsubscribeGetChangesNothing is the guard against mail scanners. They
// follow every URL in a message, so a GET that acted would opt out people who
// never clicked — and the symptom would be reminders silently stopping.
func TestUnsubscribeGetChangesNothing(t *testing.T) {
	u := &fakeUnsub{}
	res := get(t, New(health.New(), u), "/unsubscribe?t=abc")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
	if u.hits != 0 {
		t.Error("a GET reached the unsubscribe use case")
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `method="post"`) {
		t.Error("the confirmation page has no POST form, so there is no way to actually unsubscribe")
	}
}

func TestUnsubscribePostOptsOut(t *testing.T) {
	u := &fakeUnsub{}
	rec := httptest.NewRecorder()
	New(health.New(), u).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/unsubscribe?t=abc", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if len(u.got) != 1 || u.got[0] != "abc" {
		t.Fatalf("use case saw %v, want [abc]", u.got)
	}
}

// TestUnsubscribeFailureIsNotReportedAsSuccess: telling a reader they are
// unsubscribed when nothing was recorded is the one outcome this endpoint must
// never produce — they would stop looking for a way out while mail kept coming.
func TestUnsubscribeFailureIsNotReportedAsSuccess(t *testing.T) {
	for _, tt := range []struct {
		name string
		h    http.Handler
	}{
		{"use case errors", New(health.New(), &fakeUnsub{err: errors.New("db down")})},
		{"not wired at all", New(health.New(), nil)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tt.h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/unsubscribe?t=abc", nil))
			if rec.Code < 500 {
				t.Errorf("status %d — a failure must not read as success", rec.Code)
			}
			if strings.Contains(rec.Body.String(), "Done.") {
				t.Error("the page claims success on a failed unsubscribe")
			}
		})
	}
}
