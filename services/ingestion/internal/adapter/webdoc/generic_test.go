package webdoc

import "testing"

// The generic fallback on the page it exists for: a buyer's own
// amministrazione-trasparente listing, running no platform anybody has
// integrated. Exact set, because a fabricated link is worse in kind than a
// missing one — its text ends up persisted against the tender and quoted back
// as that tender's requirements.
func TestGenericParserReadsATrasparenteListing(t *testing.T) {
	page := loadPage(t, "generic-trasparente.html",
		"https://www.comune.esempio.it/trasparenza/bandi/refezione/")

	files, err := genericParser{}.Parse(page)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	assertFiles(t, files, []FileLink{
		{
			URL:   "https://www.comune.esempio.it/trasparenza/bandi/refezione/atti/bando-refezione.pdf",
			Label: "Bando di gara — Bando",
			Ext:   "pdf",
		},
		{
			URL:   "https://www.comune.esempio.it/trasparenza/bandi/refezione/atti/disciplinare-refezione.pdf",
			Label: "Disciplinare di gara — Disciplinare",
			Ext:   "pdf",
		},
		// The two-signal link, and the reason labels are assembled from the
		// whole row rather than the anchor. The anchor says "Scarica" and
		// nothing else; every word that identifies this as the capitolato is in
		// the next cell.
		{
			URL:   "https://www.comune.esempio.it/scarica/8842",
			Label: "Scarica — Capitolato speciale descrittivo e prestazionale",
			Ext:   "",
		},
	})
}

// The refusals in that same fixture, asserted as refusals rather than inferred
// from a count. Each of these is a link the parser SAW and declined, and each
// declining rule earns its place from a different failure: chrome links produce
// the site menu as tender documents, cross-host links produce the AgID banner,
// and a one-signal page link produces a request for a page we are already on.
func TestGenericParserRefusesEverythingThatIsNotAFile(t *testing.T) {
	page := loadPage(t, "generic-trasparente.html",
		"https://www.comune.esempio.it/trasparenza/bandi/refezione/")

	files, err := genericParser{}.Parse(page)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	refused := []struct {
		name string
		url  string
	}{
		{"a PDF in the <header>", "https://www.comune.esempio.it/allegati/logo-comune.pdf"},
		{"a PDF in a role=navigation region", "https://www.comune.esempio.it/trasparenza/piano-anticorruzione.pdf"},
		{"a PDF in a role=search region", "https://www.comune.esempio.it/trasparenza/guida-ricerca.pdf"},
		{"a PDF in the <footer>", "https://www.comune.esempio.it/trasparenza/note-legali.pdf"},
		{"a PDF on another host", "https://cdn.altro-host.it/banner/agid-linee-guida.pdf"},
		{"a page link whose label alone looks document-shaped", "https://www.comune.esempio.it/pagine/chiarimenti-refezione"},
	}
	for _, r := range refused {
		t.Run(r.name, func(t *testing.T) {
			for _, f := range files {
				if f.URL == r.url {
					t.Errorf("enumerated %s", r.url)
				}
			}
		})
	}
}

// The fallback's honest limit, asserted so it stays a known limit rather than a
// surprise. A page whose file table arrives by XHR yields nothing here, and
// nothing is what it must yield: there is no headless browser in this design.
//
// What this test really protects is the SIGN of that result. Zero files from
// the fallback is a statement about the fallback, and TestRegistryNeverLetsThe
// FallbackClaimAbsence is where that is kept from becoming a claim about the
// page.
func TestGenericParserFindsNothingOnAScriptRenderedListing(t *testing.T) {
	page := loadPage(t, "generic-script-rendered.html",
		"https://appalti.esempio.it/procedura/44120")

	files, err := genericParser{}.Parse(page)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	assertFiles(t, files, nil)
}

func TestGenericAcceptanceRuleNeedsTwoSignalsWithoutAnExtension(t *testing.T) {
	cases := []struct {
		name  string
		url   string
		label string
		want  bool
	}{
		{"an extension alone is enough", "https://x.it/a/qualunque-nome.pdf", "", true},
		{"an archive counts, because not extracting it is a measurement",
			"https://x.it/a/allegati.zip", "", true},
		{"a signed PDF counts for the same reason", "https://x.it/a/bando.pdf.p7m", "", true},
		{"both signals together", "https://x.it/download/8842", "Disciplinare di gara", true},
		{"the download shape alone is not enough", "https://x.it/download/8842", "Torna all'elenco", false},
		{"the words alone are not enough", "https://x.it/pagine/disciplinare", "Disciplinare di gara", false},
		{"neither", "https://x.it/pagine/contatti", "Contatti", false},
		// "documenti" is deliberately not a download token: on several of these
		// portals it is the LISTING's own path, so taking it would spend a
		// request per page re-fetching the page we are standing on.
		{"the listing's own path is not a download shape",
			"https://x.it/gare/id42/documenti", "Documenti di gara", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := mustParse(t, tc.url)
			if got := (genericParser{}).accept(u, tc.label); got != tc.want {
				t.Errorf("accept(%s, %q) = %v, want %v", tc.url, tc.label, got, tc.want)
			}
		})
	}
}
