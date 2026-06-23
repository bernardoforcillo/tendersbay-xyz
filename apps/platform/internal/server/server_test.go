package server

import (
	"io"
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
