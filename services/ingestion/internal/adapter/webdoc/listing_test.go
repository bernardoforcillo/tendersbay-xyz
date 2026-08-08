package webdoc

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", raw, err)
	}
	return u
}

// EVERY FIXTURE UNDER testdata/ IS SYNTHETIC.
//
// None of them is a captured page. They were hand-written from the URL shapes
// this repository records, and they are here to validate the SHARED machinery —
// chrome skipping, cross-host refusal, row-aware labelling, deduplication, the
// registry's routing and its guard — all of which is portal-independent and
// stays correct whatever the real markup turns out to be.
//
// They validate NOTHING about the two platform tables in sintel.go and
// tuttogare.go. A listing parser checked only against invented HTML is not a
// validated parser, and the fixtures for the named platforms must be replaced
// with real captures — from the robots-reachability probe the design puts
// before any of this is trusted — before either platform is believed in
// production.
func loadPage(t *testing.T, name, pageURL string) Document {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return Document{
		URL:         pageURL,
		ContentType: "text/html; charset=utf-8",
		Filename:    name,
		Body:        body,
	}
}

// assertFiles compares an enumeration against the exact expected set.
//
// Exact rather than "contains", per the golden-corpus gate: a parser that
// starts returning one extra link is the failure that matters most here, and a
// subset assertion is precisely the one that would not catch it.
func assertFiles(t *testing.T, got, want []FileLink) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("len(files) = %d, want %d", len(got), len(want))
		for i, f := range got {
			t.Logf("got[%d] = %+v", i, f)
		}
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("files[%d] =\n  %+v\nwant\n  %+v", i, got[i], want[i])
		}
	}
}

func TestExtensionOf(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"a plain document", "https://x.it/a/disciplinare.pdf", "pdf"},
		{"uppercase is folded", "https://x.it/a/BANDO.PDF", "pdf"},
		{"a signed PDF reports its OUTER format, which is the one we would have to unwrap", "https://x.it/a/planimetrie.pdf.p7m", "p7m"},
		{"a query string is not part of the path", "https://x.it/a/capitolato.pdf?sid=99", "pdf"},
		{"an extension-less download endpoint has none, and that is normal here", "https://x.it/download/8842", ""},
		{"a dotted procedure slug is not an extension", "https://x.it/gare/id42-comune.di.esempio", ""},
		{"a percent-escaped name still resolves", "https://x.it/a/disciplinare%20di%20gara.pdf", "pdf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := mustParse(t, tc.url)
			if got := extensionOf(u); got != tc.want {
				t.Errorf("extensionOf(%s) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestResolveLinkRefusals(t *testing.T) {
	base := mustParse(t, "https://appalti.esempio.it/gare/id1-x/")

	cases := []struct {
		name string
		href string
		want string // "" means the link must be refused
	}{
		{"a relative path resolves against the page it was found on",
			"allegati/bando.pdf", "https://appalti.esempio.it/gare/id1-x/allegati/bando.pdf"},
		{"an absolute path resolves against the host",
			"/allegati/bando.pdf", "https://appalti.esempio.it/allegati/bando.pdf"},
		{"a fragment is stripped, so #top and no fragment are one document",
			"/allegati/bando.pdf#pagina2", "https://appalti.esempio.it/allegati/bando.pdf"},
		{"an in-page anchor is not a document", "#allegati", ""},
		{"an empty href is not a document", "", ""},
		{"javascript: is refused by the scheme allowlist without being named",
			"javascript:stampa()", ""},
		{"mailto: likewise", "mailto:rup@esempio.it", ""},
		{"another host is refused — on these pages that is the transparency portal, not a tender document",
			"https://cdn.altro.it/x/bando.pdf", ""},
		{"a different port is a different host", "https://appalti.esempio.it:8443/a/bando.pdf", ""},
		{"the same host in another case is the same host",
			"https://APPALTI.ESEMPIO.IT/a/bando.pdf", "https://APPALTI.ESEMPIO.IT/a/bando.pdf"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, ok := resolveLink(base, tc.href)
			if tc.want == "" {
				if ok {
					t.Fatalf("resolveLink(%q) = %s, want refusal", tc.href, u)
				}
				return
			}
			if !ok {
				t.Fatalf("resolveLink(%q) was refused, want %s", tc.href, tc.want)
			}
			if got := u.String(); got != tc.want {
				t.Errorf("resolveLink(%q) = %s, want %s", tc.href, got, tc.want)
			}
		})
	}
}

func TestContainsFold(t *testing.T) {
	body := []byte(`<footer><p>Powered by TuttoGare</p></footer>`)

	if !containsFold(body, "tuttogare") {
		t.Error("containsFold did not fold the case of a capitalised product name")
	}
	if containsFold(body, "sintel") {
		t.Error("containsFold matched a fingerprint that is not on the page")
	}
	if !containsFold(body, "powered by tuttogare") {
		t.Error("containsFold failed on a multi-word fingerprint")
	}
	if containsFold([]byte("tutto"), "tuttogare") {
		t.Error("containsFold matched a needle longer than the haystack")
	}
}
