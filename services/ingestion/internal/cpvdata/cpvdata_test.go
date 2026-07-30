package cpvdata_test

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/cpvdata"
)

// These bounds are the gate on the conversion step, which happens outside this
// repo and cannot be reproduced by a test. A half-complete export would
// otherwise seed a vocabulary that silently fails to bridge most languages.
const (
	minCodes  = 9000
	maxCodes  = 10000
	wantLangs = 24

	// The real export has exactly one documented gap: code 03117140 is missing
	// its Hungarian and Italian labels in the European Commission's own data
	// (see scripts/cpv/convert.mjs's KNOWN_GAPS). That accounts for at most one
	// missing label per language, so the per-language floor below allows
	// exactly that much slack — anything looser would let a bad re-seed lose
	// most of a language's labels and still pass.
	maxMissingPerLang = 1
)

func TestRows_DecodesTheEmbeddedVocabulary(t *testing.T) {
	rows, err := cpvdata.Rows()
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if len(rows) < minCodes*wantLangs/2 {
		t.Errorf("len(rows) = %d, want at least %d — the export looks truncated", len(rows), minCodes*wantLangs/2)
	}
}

func TestRows_CoversAllTwentyFourLanguages(t *testing.T) {
	rows, err := cpvdata.Rows()
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	langs := map[string]int{}
	codes := map[string]bool{}
	for _, r := range rows {
		langs[r.Lang]++
		codes[r.Code] = true
	}
	if len(langs) != wantLangs {
		t.Fatalf("len(langs) = %d, want %d — a vocabulary missing a language can't bridge search queries written in it, which is this artifact's entire reason to exist", len(langs), wantLangs)
	}
	// The floor is the total code count minus the one documented gap, not half
	// of it: with ~9450 codes, a floor of minCodes/2 would let a language lose
	// more than half its labels and still pass this test.
	floor := len(codes) - maxMissingPerLang
	for lang, n := range langs {
		if n < floor {
			t.Errorf("labels for %s = %d, want at least %d — a floor this loose would let a bad re-seed silently drop most of a language's labels", lang, n, floor)
		}
	}
}

func TestRows_EveryCodeIsEightDigits(t *testing.T) {
	rows, err := cpvdata.Rows()
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	codes := map[string]bool{}
	for _, r := range rows {
		if len(r.Code) != 8 {
			t.Fatalf("code %q is %d chars, want 8 — the check digit was not stripped", r.Code, len(r.Code))
		}
		for _, c := range r.Code {
			if c < '0' || c > '9' {
				t.Fatalf("code %q contains non-digit rune %q, want digits only — a non-digit surviving normaliseCode means a section heading or malformed row leaked into the data", r.Code, c)
			}
		}
		codes[r.Code] = true
	}
	if len(codes) < minCodes || len(codes) > maxCodes {
		t.Errorf("distinct codes = %d, want between %d and %d — a count outside this band means the export gained or lost a systematic chunk of the vocabulary (wrong sheet, truncated download, or a merge with the supplementary codes)", len(codes), minCodes, maxCodes)
	}
}

func TestRows_NoLabelIsBlank(t *testing.T) {
	rows, err := cpvdata.Rows()
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	for _, r := range rows {
		if r.Label == "" {
			t.Fatalf("label for %s/%s = %q, want non-empty — a blank label reaching Rows() means convert.mjs's blank-cell skip regressed and wrote an empty cell as data", r.Code, r.Lang, r.Label)
		}
	}
}
