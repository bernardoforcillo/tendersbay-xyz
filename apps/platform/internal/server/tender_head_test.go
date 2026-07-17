package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

const testShell = `<html><head>` +
	`<title>tendersbay — generic</title>` +
	`<meta name="description" content="generic description">` +
	`<meta property="og:title" content="generic og title">` +
	`<meta property="og:description" content="generic og description">` +
	`<meta name="twitter:title" content="generic tw title">` +
	`<meta name="twitter:description" content="generic tw description">` +
	`<link rel="canonical" href="https://tendersbay.xyz/en-ie/">` +
	`<script>window.__ENV__ = {};</script>` +
	`<!--tender-head--></head><body></body></html>`

func TestInjectTenderHead_OverridesInPlaceAndInjectsJSONLD(t *testing.T) {
	out := string(injectTenderHead([]byte(testShell), tenderMeta{
		ID: "5", Title: "Road works", BuyerName: "City", Country: "ITA",
		CanonicalURL: "https://tendersbay.xyz/en-ie/tenders/5",
	}))
	if !strings.Contains(out, "<title>Road works — tendersbay</title>") {
		t.Errorf("title not overridden: %s", out)
	}
	// og:title / twitter:title overridden in place, generic gone, not duplicated.
	if !strings.Contains(out, `<meta property="og:title" content="Road works — tendersbay">`) {
		t.Errorf("og:title not overridden: %s", out)
	}
	if !strings.Contains(out, `<meta name="twitter:title" content="Road works — tendersbay">`) {
		t.Errorf("twitter:title not overridden: %s", out)
	}
	if strings.Contains(out, "generic og title") || strings.Contains(out, "generic tw title") {
		t.Errorf("generic social titles still present: %s", out)
	}
	if strings.Count(out, `property="og:title"`) != 1 || strings.Count(out, `name="twitter:title"`) != 1 {
		t.Errorf("social tags duplicated: %s", out)
	}
	if !strings.Contains(out, `<link rel="canonical" href="https://tendersbay.xyz/en-ie/tenders/5">`) {
		t.Errorf("canonical not overridden: %s", out)
	}
	if strings.Count(out, `rel="canonical"`) != 1 {
		t.Errorf("canonical duplicated: %s", out)
	}
	if !strings.Contains(out, "application/ld+json") || !strings.Contains(out, "Road works") {
		t.Errorf("JSON-LD missing: %s", out)
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

	fsys := fstest.MapFS{
		"index.html":       {Data: []byte(testShell)},
		"en-ie/index.html": {Data: []byte(testShell)},
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

	srv := New(fstest.MapFS{"index.html": {Data: []byte(testShell)}, "en-ie/index.html": {Data: []byte(testShell)}})

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

func TestTenderSitemapXML_OneURLPerTenderWithHreflang(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tender.v1.TenderService/ListTenderSitemap") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"refs":[{"id":"5","lastmod":"2026-01-02T00:00:00Z"},{"id":"6","lastmod":""}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	xml, err := tenderSitemapXML(context.Background(), backend.URL, "https://tendersbay.xyz", []string{"en-ie", "it-it", "de-de"})
	if err != nil {
		t.Fatalf("tenderSitemapXML: %v", err)
	}
	s := string(xml)
	// One <url> per tender (2), not per locale (would be 6).
	if strings.Count(s, "<url>") != 2 {
		t.Errorf("want 2 <url> blocks (one per tender), got %d:\n%s", strings.Count(s, "<url>"), s)
	}
	// Primary loc at the default locale; hreflang alternates present incl. x-default and BCP-47 casing.
	if !strings.Contains(s, "<loc>https://tendersbay.xyz/en-ie/tenders/5</loc>") {
		t.Errorf("missing default-locale <loc> for tender 5:\n%s", s)
	}
	if !strings.Contains(s, `hreflang="it-IT" href="https://tendersbay.xyz/it-it/tenders/5"`) {
		t.Errorf("missing it-IT hreflang alternate:\n%s", s)
	}
	if !strings.Contains(s, `hreflang="x-default"`) {
		t.Errorf("missing x-default alternate:\n%s", s)
	}
	if !strings.Contains(s, `xmlns:xhtml=`) {
		t.Errorf("missing xhtml namespace:\n%s", s)
	}
}

func TestServer_AuthedShellIsNoindexButLocaleIsNot(t *testing.T) {
	shell := "<html><head><title>t</title><meta name=\"description\" content=\"x\"><script>window.__ENV__ = {};</script><!--tender-head--></head><body></body></html>"
	srv := New(fstest.MapFS{"index.html": {Data: []byte(shell)}, "en-ie/index.html": {Data: []byte(shell)}})

	// Authed app route (non-locale) → served the root shell → noindex.
	recAuthed := httptest.NewRecorder()
	srv.ServeHTTP(recAuthed, httptest.NewRequest(http.MethodGet, "/explore/tenders/5", nil))
	if recAuthed.Header().Get("X-Robots-Tag") != "noindex" {
		t.Errorf("authed shell X-Robots-Tag = %q, want noindex", recAuthed.Header().Get("X-Robots-Tag"))
	}

	// Locale landing page → indexable (no noindex).
	recLocale := httptest.NewRecorder()
	srv.ServeHTTP(recLocale, httptest.NewRequest(http.MethodGet, "/en-ie", nil))
	if recLocale.Header().Get("X-Robots-Tag") == "noindex" {
		t.Errorf("locale page must NOT be noindex")
	}
}

func TestServer_TenderBackendErrorServesPlainShell(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()
	t.Setenv("API_URL", backend.URL)

	srv := New(fstest.MapFS{"index.html": {Data: []byte(testShell)}, "en-ie/index.html": {Data: []byte(testShell)}})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/en-ie/tenders/5", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (degrade to plain shell)", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if strings.Contains(string(body), "— tendersbay") || strings.Contains(string(body), "application/ld+json") {
		t.Errorf("backend-error page should be the untouched shell: %s", body)
	}
	if strings.Contains(string(body), `content="noindex"`) {
		t.Errorf("backend-error (not 404) must not be noindex: %s", body)
	}
}
