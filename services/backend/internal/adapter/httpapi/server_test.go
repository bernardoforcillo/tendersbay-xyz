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
	res := get(t, New(health.New()), "/healthz")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("body = %q, want status ok", body)
	}
}

func TestReadyzOK(t *testing.T) {
	res := get(t, New(health.New(stubProbe{name: "a"})), "/readyz")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
}

func TestReadyzUnavailable(t *testing.T) {
	svc := health.New(stubProbe{name: "db", err: errors.New("down")})
	res := get(t, New(svc), "/readyz")
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

	get(t, New(health.New()), "/healthz")

	out := buf.String()
	if !strings.Contains(out, "method=GET") || !strings.Contains(out, "status=200") ||
		!strings.Contains(out, "path=/healthz") || !strings.Contains(out, "duration_ms=") {
		t.Fatalf("expected request log, got %q", out)
	}
}
