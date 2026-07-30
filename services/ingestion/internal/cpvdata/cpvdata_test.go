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
	for _, r := range rows {
		langs[r.Lang]++
	}
	if len(langs) != wantLangs {
		t.Fatalf("languages = %d (%v), want %d", len(langs), langs, wantLangs)
	}
	// A language present but nearly empty is the subtler failure: the column
	// existed in the export but most of its cells were blank.
	for lang, n := range langs {
		if n < minCodes/2 {
			t.Errorf("language %s has only %d labels, want at least %d", lang, n, minCodes/2)
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
				t.Fatalf("code %q contains a non-digit", r.Code)
			}
		}
		codes[r.Code] = true
	}
	if len(codes) < minCodes || len(codes) > maxCodes {
		t.Errorf("distinct codes = %d, want between %d and %d", len(codes), minCodes, maxCodes)
	}
}

func TestRows_NoLabelIsBlank(t *testing.T) {
	rows, err := cpvdata.Rows()
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	for _, r := range rows {
		if r.Label == "" {
			t.Fatalf("code %s/%s has a blank label; the converter must skip empty cells", r.Code, r.Lang)
		}
	}
}
