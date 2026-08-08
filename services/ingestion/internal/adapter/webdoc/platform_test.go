package webdoc

import "testing"

// The two named platforms against their fixtures.
//
// READ THE WARNING IN listing_test.go BEFORE TRUSTING THESE. The fixtures are
// synthetic, so what passes here is the shared machinery — container narrowing,
// row-aware labelling, the download-shape acceptance rule, deduplication of the
// icon column — and not the platform tables themselves.

func TestTuttogareReadsItsFileTable(t *testing.T) {
	page := loadPage(t, "tuttogare-listing.html",
		"https://gare.comune.esempio.it/gare/id4821-manutenzione")

	p := platformParser{spec: tuttogare}
	if !p.Detect(page) {
		t.Fatal("Detect declined a page carrying the platform's own fingerprint")
	}
	files, err := p.Parse(page)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	assertFiles(t, files, []FileLink{
		{
			URL:   "https://gare.comune.esempio.it/gare/id4821-manutenzione/documenti/download/1",
			Label: "Bando_di_gara.pdf — Bando — 412 KB",
			Ext:   "",
		},
		{
			URL:   "https://gare.comune.esempio.it/gare/id4821-manutenzione/documenti/download/2",
			Label: "Disciplinare_di_gara.pdf — Disciplinare — 1,2 MB",
			Ext:   "",
		},
		{
			URL:   "https://gare.comune.esempio.it/gare/id4821-manutenzione/documenti/download/3",
			Label: "Capitolato_speciale.pdf — Capitolato speciale d'appalto — 3,4 MB",
			Ext:   "",
		},
		{
			URL:   "https://gare.comune.esempio.it/allegati/dgue.zip",
			Label: "DGUE.zip — Modulistica — 88 KB",
			Ext:   "zip",
		},
		{
			URL:   "https://gare.comune.esempio.it/allegati/planimetrie.pdf.p7m",
			Label: "Planimetrie.pdf.p7m — Elaborati grafici — 9,1 MB",
			Ext:   "p7m",
		},
	})
}

// Every row in that fixture carries a second anchor — the ⭳ download icon —
// pointing at the same file. Deduplication by resolved URL is what keeps that
// from doubling the file count, and doubling it would be doubling the requests
// made to the buyer's server as well as the coverage number reported for the
// tender.
func TestTuttogareDeduplicatesTheIconColumn(t *testing.T) {
	page := loadPage(t, "tuttogare-listing.html",
		"https://gare.comune.esempio.it/gare/id4821-manutenzione")

	files, err := platformParser{spec: tuttogare}.Parse(page)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	seen := map[string]int{}
	for _, f := range files {
		seen[f.URL]++
	}
	for u, n := range seen {
		if n > 1 {
			t.Errorf("%s enumerated %d times", u, n)
		}
	}
	// And the icon's own glyph must not reach the label: it is a cell with no
	// letters and no digits, which is decoration rather than words.
	for _, f := range files {
		if !hasWordChar(f.Label) {
			t.Errorf("label %q carries no words", f.Label)
		}
	}
}

func TestSintelReadsItsFileTable(t *testing.T) {
	page := loadPage(t, "sintel-listing.html",
		"https://www.sintel.regione.lombardia.it/eprocdata/auctionDetail.xhtml?id=44120")

	p := platformParser{spec: ariaSintel}
	if !p.Detect(page) {
		t.Fatal("Detect declined a page carrying the platform's own fingerprint")
	}
	files, err := p.Parse(page)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	assertFiles(t, files, []FileLink{
		{
			URL:   "https://www.sintel.regione.lombardia.it/eprocdata/allegati/downloadDocument?id=55120",
			Label: "Disciplinare di gara — Documentazione di gara — 12/08/2026",
			Ext:   "",
		},
		{
			URL:   "https://www.sintel.regione.lombardia.it/eprocdata/allegati/downloadDocument?id=55121",
			Label: "Capitolato tecnico — Documentazione di gara — 12/08/2026",
			Ext:   "",
		},
		{
			URL:   "https://www.sintel.regione.lombardia.it/eprocdata/allegati/Modello_A_dichiarazioni.docx",
			Label: "Modello A — Modulistica — 12/08/2026",
			Ext:   "docx",
		},
	})
}

// Detection is the part that has to work on a buyer's own domain, because 33 of
// the 60 measured document URLs are on one — so a fingerprint that only ever
// fires on the operator's hostname is a fingerprint that fails on more than
// half its pages.
func TestDetectPrefersMarkupOverHostname(t *testing.T) {
	cases := []struct {
		name string
		spec platformSpec
		page Document
		want bool
	}{
		{
			name: "markup alone identifies the platform on a buyer's own domain",
			spec: tuttogare,
			page: Document{
				URL:  "https://appalti.comune.altrove.it/procedura/771",
				Body: []byte(`<html><footer>Powered by TuttoGare</footer></html>`),
			},
			want: true,
		},
		{
			name: "the URL shape identifies it when the markup does not",
			spec: tuttogare,
			page: Document{
				URL:  "https://appalti.comune.altrove.it/gare/id4821-manutenzione",
				Body: []byte(`<html><body>nulla di riconoscibile</body></html>`),
			},
			want: true,
		},
		{
			name: "the operator's own host identifies it as a last resort",
			spec: ariaSintel,
			page: Document{
				URL:  "https://www.sintel.regione.lombardia.it/qualcosa",
				Body: []byte(`<html><body>nulla di riconoscibile</body></html>`),
			},
			want: true,
		},
		{
			name: "an unrelated portal is declined by all three",
			spec: ariaSintel,
			page: Document{
				URL:  "https://appalti.comune.altrove.it/bandi/2026/refezione",
				Body: []byte(`<html><body>Bandi di gara e contratti</body></html>`),
			},
			want: false,
		},
		{
			name: "the platforms do not claim each other's pages",
			spec: tuttogare,
			page: Document{
				URL:  "https://www.sintel.regione.lombardia.it/eprocdata/auctionDetail.xhtml",
				Body: []byte(`<html><title>Sintel</title></html>`),
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := (platformParser{spec: tc.spec}).Detect(tc.page); got != tc.want {
				t.Errorf("Detect = %v, want %v", got, tc.want)
			}
		})
	}
}

// A container that matches nothing, or matches a region with no files in it,
// must widen the scan rather than end it. The direction matters: degrading
// toward finding more cannot invent a link — acceptance is still governed by
// the extension list and this platform's download shapes — whereas degrading
// toward silence would be reported as a listing that publishes nothing.
func TestPlatformParserWidensTheScanWhenItsContainerMisses(t *testing.T) {
	page := Document{
		URL:         "https://gare.comune.esempio.it/gare/id900-x",
		ContentType: "text/html",
		Body: []byte(`<html><body>
			<main><ul class="elenco-atti">
				<li><a href="/allegati/disciplinare.pdf">Disciplinare di gara</a></li>
			</ul></main>
		</body></html>`),
	}

	spec := tuttogare
	spec.container = nodeMatch{tags: []string{"table"}, tokens: []string{"questo-non-esiste"}}

	files, err := platformParser{spec: spec}.Parse(page)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertFiles(t, files, []FileLink{{
		URL:   "https://gare.comune.esempio.it/allegati/disciplinare.pdf",
		Label: "Disciplinare di gara",
		Ext:   "pdf",
	}})
}

// A file is not a page, and enumerating one would attribute a PDF's own bytes
// to a listing. The caller checks IsHTML before enumerating; this is the
// backstop for when it does not, and it mirrors the identical refusal in
// Extractor.Extract at the other end of the pipeline.
func TestEnumeratingANonHTMLDocumentIsRefused(t *testing.T) {
	page := Document{
		URL:         "https://appalti.esempio.it/allegati/disciplinare.pdf",
		ContentType: "application/pdf",
		Filename:    "disciplinare.pdf",
		Body:        []byte("%PDF-1.7"),
	}

	if _, err := NewRegistry().Enumerate(page); err == nil {
		t.Fatal("Enumerate accepted a PDF as a listing page")
	}
}
