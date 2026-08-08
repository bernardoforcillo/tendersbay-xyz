package webdoc

import "testing"

func TestRegistryRoutesEachPageToItsPlatform(t *testing.T) {
	cases := []struct {
		name     string
		fixture  string
		pageURL  string
		platform string
		files    int
	}{
		{
			name:     "a TuttoGare procedure page",
			fixture:  "tuttogare-listing.html",
			pageURL:  "https://gare.comune.esempio.it/gare/id4821-manutenzione",
			platform: "tuttogare",
			files:    5,
		},
		{
			name:     "a Sintel procedure page",
			fixture:  "sintel-listing.html",
			pageURL:  "https://www.sintel.regione.lombardia.it/eprocdata/auctionDetail.xhtml?id=44120",
			platform: "aria-sintel",
			files:    3,
		},
		{
			// The value that makes the platform column a backlog: this is a
			// portal nobody has integrated, the fallback read it anyway, and
			// "generic + ok" is exactly the row that says no parser is needed
			// here.
			name:     "an unrecognised portal falls back, and says so",
			fixture:  "generic-trasparente.html",
			pageURL:  "https://www.comune.esempio.it/trasparenza/bandi/refezione/",
			platform: PlatformGeneric,
			files:    3,
		},
		{
			name:     "an unrecognised portal the fallback cannot read is still reported as generic",
			fixture:  "generic-script-rendered.html",
			pageURL:  "https://appalti.esempio.it/procedura/44120",
			platform: PlatformGeneric,
			files:    0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			listing, err := NewRegistry().Enumerate(loadPage(t, tc.fixture, tc.pageURL))
			if err != nil {
				t.Fatalf("Enumerate: %v", err)
			}
			if listing.Platform != tc.platform {
				t.Errorf("Platform = %q, want %q", listing.Platform, tc.platform)
			}
			if len(listing.Files) != tc.files {
				t.Errorf("len(Files) = %d, want %d", len(listing.Files), tc.files)
			}
		})
	}
}

// THE INVARIANT, AS AN ASSERTION.
//
// An inaccessible file is never mistaken for an absent one. Downstream, an
// empty listing from a NAMED platform parser becomes a positive claim that the
// buyer published nothing, while an empty listing from the fallback becomes an
// admission that no parser could read the page. Those are two different
// sentences to a bid manager and only one of them may ever be wrong in the
// direction of "there is nothing here".
//
// So the registry has to keep the two apart, and the two cases below are that
// distinction end to end: same empty result, different Platform, different
// meaning.
func TestRegistryDistinguishesAnEmptyListingFromAnUnreadableOne(t *testing.T) {
	registry := NewRegistry()

	// A recognised platform whose listing is genuinely empty. The named parser
	// read the page and the page has no files on it — the one case that may
	// become a claim of absence.
	empty, err := registry.Enumerate(loadPage(t, "tuttogare-no-documents.html",
		"https://gare.comune.esempio.it/gare/id4999-cimiteriali"))
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	if empty.Platform != "tuttogare" {
		t.Errorf("Platform = %q, want the named platform — only a named parser may claim absence", empty.Platform)
	}
	if len(empty.Files) != 0 {
		t.Errorf("len(Files) = %d, want 0", len(empty.Files))
	}

	// An unrecognised portal the fallback could not read. Identical file count,
	// and it must NOT be attributable to a platform, because nothing here
	// justifies a claim about what the buyer published.
	unreadable, err := registry.Enumerate(loadPage(t, "generic-script-rendered.html",
		"https://appalti.esempio.it/procedura/44120"))
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	if unreadable.Platform != PlatformGeneric {
		t.Errorf("Platform = %q, want %q", unreadable.Platform, PlatformGeneric)
	}
	if len(unreadable.Files) != 0 {
		t.Errorf("len(Files) = %d, want 0", len(unreadable.Files))
	}
}

// The guard, and it is the reason Enumerate is not four lines.
//
// A named parser's fingerprints decide whether an empty result becomes a claim,
// and the fingerprints in sintel.go and tuttogare.go have never been checked
// against a captured page. This fixture is what a wrong one looks like: the
// TuttoGare marker is there, the download endpoints are not the ones that file
// describes, and the named parser comes back with nothing.
//
// Believing it would report a tender with three published documents as
// publishing none. Instead the fallback re-reads the page, finds them, and the
// listing is returned under PlatformGeneric — because the platform column
// records which enumerator produced the links, and crediting a parser that
// produced none would hide the defect in the one query written to find it.
func TestRegistryNeverLetsAMisdetectionClaimAbsence(t *testing.T) {
	page := loadPage(t, "tuttogare-moved-markup.html",
		"https://gare.unione.esempio.it/gare/id7702-verde")

	if !(platformParser{spec: tuttogare}).Detect(page) {
		t.Fatal("the fixture no longer trips the platform fingerprint, so it no longer tests the guard")
	}
	direct, err := platformParser{spec: tuttogare}.Parse(page)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(direct) != 0 {
		t.Fatalf("the named parser found %d files, so this fixture no longer tests the guard", len(direct))
	}

	listing, err := NewRegistry().Enumerate(page)
	if err != nil {
		t.Fatalf("Enumerate: %v", err)
	}
	if listing.Platform != PlatformGeneric {
		t.Errorf("Platform = %q, want %q — the fallback produced these links, not the named parser",
			listing.Platform, PlatformGeneric)
	}
	assertFiles(t, listing.Files, []FileLink{
		{
			URL:   "https://gare.unione.esempio.it/asset/estraiFile.do?k=91021",
			Label: "Scarica — Bando di gara",
			Ext:   "",
		},
		{
			URL:   "https://gare.unione.esempio.it/asset/estraiFile.do?k=91022",
			Label: "Scarica — Disciplinare di gara",
			Ext:   "",
		},
		{
			URL:   "https://gare.unione.esempio.it/asset/estraiFile.do?k=91023",
			Label: "Scarica — Capitolato speciale",
			Ext:   "",
		},
	})
}

// Adding a platform is one entry here and one file beside it. The check is that
// the fallback stays OUT of the parser list: it is not "the parser that matches
// everything", it is the one whose empty result means something different, and
// a loop that treated it as an ordinary entry would lose exactly that.
func TestRegistryKeepsTheFallbackOutOfTheParserList(t *testing.T) {
	r := NewRegistry()

	if r.fallback == nil {
		t.Fatal("no fallback registered")
	}
	names := map[string]bool{}
	for _, p := range r.parsers {
		if p.Name() == PlatformGeneric {
			t.Errorf("the fallback is registered as a named platform parser")
		}
		if names[p.Name()] {
			t.Errorf("two parsers share the name %q, which is a grouping key in every backlog query", p.Name())
		}
		names[p.Name()] = true
	}
	for _, want := range []string{"aria-sintel", "tuttogare"} {
		if !names[want] {
			t.Errorf("platform %q is not registered", want)
		}
	}
}
