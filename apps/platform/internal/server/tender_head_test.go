package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestInjectTenderHead_OverridesTitleAndInjectsJSONLD(t *testing.T) {
	shell := []byte(`<html><head><title>tendersbay</title><meta name="description" content="landing"><!--tender-head--></head><body></body></html>`)
	out := string(injectTenderHead(shell, tenderMeta{ID: "5", Title: "Road works", BuyerName: "City", Country: "ITA", CanonicalURL: "https://tendersbay.xyz/en-ie/tenders/5"}))
	if !strings.Contains(out, "<title>Road works — tendersbay</title>") {
		t.Errorf("title not overridden: %s", out)
	}
	if !strings.Contains(out, "application/ld+json") || !strings.Contains(out, "Road works") {
		t.Errorf("JSON-LD not injected: %s", out)
	}
	if !strings.Contains(out, `property="og:title"`) || !strings.Contains(out, `rel="canonical"`) {
		t.Errorf("OG/canonical missing: %s", out)
	}
	if strings.Contains(out, "<!--tender-head-->") {
		t.Errorf("sentinel not consumed: %s", out)
	}
}

func TestServer_ServesTenderPageFromBackend(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tender.v1.TenderService/GetTender") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tender":{"id":"5","title":"Road works","buyerName":"City","country":"ITA"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()
	t.Setenv("API_URL", backend.URL)

	shell := "<html><head><title>tendersbay</title><meta name=\"description\" content=\"x\"><script>window.__ENV__ = {};</script><!--tender-head--></head><body></body></html>"
	fsys := fstest.MapFS{
		"index.html":       {Data: []byte(shell)},
		"en-ie/index.html": {Data: []byte(shell)},
	}
	srv := New(fsys)

	req := httptest.NewRequest(http.MethodGet, "/en-ie/tenders/5", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "Road works — tendersbay") {
		t.Errorf("body missing per-tender title: %s", body)
	}
}

func TestServer_TenderNotFoundServesNoindex(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()
	t.Setenv("API_URL", backend.URL)

	shell := "<html><head><title>t</title><meta name=\"description\" content=\"x\"><script>window.__ENV__ = {};</script><!--tender-head--></head><body></body></html>"
	srv := New(fstest.MapFS{"index.html": {Data: []byte(shell)}, "en-ie/index.html": {Data: []byte(shell)}})

	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/en-ie/tenders/999", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), `name="robots" content="noindex"`) {
		t.Errorf("not-found page should be noindex: %s", body)
	}
}
