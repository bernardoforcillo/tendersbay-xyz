package server

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>app</title>")},
		"assets/app.js": {Data: []byte("console.log('hi')")},
	}
}

func get(t *testing.T, h http.Handler, target string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result()
}

func TestServesStaticAsset(t *testing.T) {
	res := get(t, New(testFS()), "/assets/app.js")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "console.log('hi')") {
		t.Fatalf("expected app.js body, got %q", body)
	}
}

func TestServesIndexAtRoot(t *testing.T) {
	res := get(t, New(testFS()), "/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "<title>app</title>") {
		t.Fatalf("expected index.html body, got %q", body)
	}
}

func TestSPAFallbackForUnknownRoute(t *testing.T) {
	res := get(t, New(testFS()), "/dashboard/settings")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "<title>app</title>") {
		t.Fatalf("expected index.html body, got %q", body)
	}
}

func TestSPAFallbackForLocalePrefix(t *testing.T) {
	res := get(t, New(testFS()), "/en-ie/")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("got %d, want 200", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "<title>app</title>") {
		t.Fatalf("expected index.html body, got %q", body)
	}
}

func TestMissingAssetReturns404(t *testing.T) {
	res := get(t, New(testFS()), "/assets/missing.js")
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("got %d, want 404", res.StatusCode)
	}
}

func TestLogsEachRequest(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	get(t, New(testFS()), "/assets/app.js")

	out := buf.String()
	if !strings.Contains(out, "method=GET") || !strings.Contains(out, "status=200") ||
		!strings.Contains(out, "path=/assets/app.js") || !strings.Contains(out, "duration_ms=") {
		t.Fatalf("expected a request log with method+path+status+duration_ms, got %q", out)
	}
}

// envFS carries the window.__ENV__ placeholder the real index.html ships with.
func envFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html": {Data: []byte(
			"<!doctype html><head><script>window.__ENV__ = {};</script></head><title>app</title>",
		)},
	}
}

func TestInjectsRuntimeEnvIntoIndex(t *testing.T) {
	t.Setenv("API_URL", "https://api.example.com")
	t.Setenv("POSTHOG_API_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://eu.i.posthog.com")

	// New reads the environment once, so it must be constructed after Setenv.
	body, _ := io.ReadAll(get(t, New(envFS()), "/").Body)
	got := string(body)

	for _, want := range []string{
		`"API_URL":"https://api.example.com"`,
		`"POSTHOG_KEY":"phc_test"`,
		`"POSTHOG_HOST":"https://eu.i.posthog.com"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("served index missing %q\n got: %s", want, got)
		}
	}
	if strings.Contains(got, "window.__ENV__ = {}") {
		t.Errorf("placeholder was not replaced: %s", got)
	}
}

func TestLeavesPlaceholderWhenNoRuntimeEnv(t *testing.T) {
	t.Setenv("API_URL", "")
	t.Setenv("POSTHOG_API_KEY", "")
	t.Setenv("POSTHOG_HOST", "")

	// SPA fallback route serves the same (un-injected) index.
	body, _ := io.ReadAll(get(t, New(envFS()), "/en-ie/dashboard").Body)
	if !strings.Contains(string(body), "window.__ENV__ = {}") {
		t.Errorf("expected empty placeholder, got: %s", body)
	}
}

func TestInjectedConfigCannotBreakOutOfScript(t *testing.T) {
	t.Setenv("API_URL", "https://x/</script><script>alert(1)</script>")
	t.Setenv("POSTHOG_API_KEY", "")
	t.Setenv("POSTHOG_HOST", "")

	body, _ := io.ReadAll(get(t, New(envFS()), "/").Body)
	// json.Marshal escapes <, > and &, so the value cannot open a second script
	// element. The document must still contain exactly the one placeholder script.
	if n := strings.Count(string(body), "<script"); n != 1 {
		t.Fatalf("expected exactly 1 <script tag, got %d — value broke out: %s", n, body)
	}
}
