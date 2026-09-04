package codelist_test

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/codelist"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// TestTablesAreConsistent guards the generated files against a hand edit: every
// Go entry needs its XML definition and the other way round.
func TestTablesAreConsistent(t *testing.T) {
	for _, tbl := range []*codelist.Table{codelist.EDM211(), codelist.EDM4()} {
		if err := tbl.Validate(); err != nil {
			t.Errorf("%s: %v", tbl.Version, err)
		}
	}
}

// TestBothVersionsCoverTheSameCriteria: the two tables must answer the same
// questions, or a customer would get a fuller document from one version than
// from the other without being told why.
func TestBothVersionsCoverTheSameCriteria(t *testing.T) {
	a, b := codelist.EDM211(), codelist.EDM4()
	inB := map[espd.CriterionKey]bool{}
	for _, k := range b.Keys() {
		inB[k] = true
	}
	for _, k := range a.Keys() {
		if !inB[k] {
			t.Errorf("%s is in %s but not in %s", k, a.Version, b.Version)
		}
		delete(inB, k)
	}
	for k := range inB {
		t.Errorf("%s is in %s but not in %s", k, b.Version, a.Version)
	}
}

// TestEveryExclusionGroundIsExpressible is the one that would actually bite: a
// Part III ground with no code-list entry makes every export fail for any
// operator who answered it.
func TestEveryExclusionGroundIsExpressible(t *testing.T) {
	for _, tbl := range []*codelist.Table{codelist.EDM211(), codelist.EDM4()} {
		for _, k := range espd.ExclusionCriteria() {
			c, err := tbl.Lookup(k)
			if err != nil {
				t.Errorf("%s: %v", tbl.Version, err)
				continue
			}
			if c.UUID == "" || c.TypeCode == "" || c.AnswerPropertyID == "" {
				t.Errorf("%s: %s is incompletely mapped: %+v", tbl.Version, k, c)
			}
			if c.SelfCleaningIndicatorID != "" && c.SelfCleaningTextID == "" {
				t.Errorf("%s: %s has a self-cleaning indicator with nowhere to put the text", tbl.Version, k)
			}
		}
	}
}

// TestCriterionUUIDsAgreeAcrossVersions pins the fact the whole design rests
// on: the UUIDs are stable and the TYPE CODES are not. If a future release
// changes the UUIDs too, the tables stop being interchangeable and this test is
// where that shows up.
func TestCriterionUUIDsAgreeAcrossVersions(t *testing.T) {
	a, b := codelist.EDM211(), codelist.EDM4()
	sameCode := 0
	for _, k := range a.Keys() {
		ca, err := a.Lookup(k)
		if err != nil {
			t.Fatal(err)
		}
		cb, err := b.Lookup(k)
		if err != nil {
			t.Fatal(err)
		}
		// The "other data" criteria were renumbered in 4.x; the Directive's own
		// criteria kept their UUIDs.
		if espd.IsExclusionCriterion(k) && ca.UUID != cb.UUID {
			t.Errorf("%s: UUID differs between versions (%s vs %s)", k, ca.UUID, cb.UUID)
		}
		if ca.TypeCode == cb.TypeCode {
			sameCode++
		}
		if ca.AnswerPropertyID == cb.AnswerPropertyID {
			t.Errorf("%s: the two versions share an answer property UUID (%s), which the releases do not",
				k, ca.AnswerPropertyID)
		}
	}
	if sameCode != 0 {
		t.Errorf("%d criteria share a type code across versions; 4.1.0 shortened all of them", sameCode)
	}
}

// TestUnknownCriterionIsAnError: a criterion with no entry must fail loudly.
func TestUnknownCriterionIsAnError(t *testing.T) {
	_, err := codelist.EDM211().Lookup(espd.CriterionKey("iv.z.invented"))
	if err == nil {
		t.Fatal("an unknown criterion must not resolve")
	}
	var unsupported *espd.CriterionUnsupportedError
	if !asError(err, &unsupported) || unsupported.Key != "iv.z.invented" {
		t.Errorf("err = %v, want a CriterionUnsupportedError naming the key", err)
	}
}

// TestTheParserRecognisesEveryVendoredTypeCode is the drift guard between the
// two transcriptions of one taxonomy.
//
// espd.CriterionForTypeCode is hand-written from the release's own taxonomy
// code list; these tables are generated from the release's sample documents.
// Both describe the same criteria, and nothing but a test connects them — which
// is how six of those codes were wrong when they were first written from
// memory. A buyer's request naming a code the parser does not know is reported
// as unmapped, so the failure mode is silent: the preview simply never mentions
// a criterion the buyer asked about.
func TestTheParserRecognisesEveryVendoredTypeCode(t *testing.T) {
	for _, tbl := range []*codelist.Table{codelist.EDM211(), codelist.EDM4()} {
		for _, k := range tbl.Keys() {
			c, err := tbl.Lookup(k)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := espd.CriterionForTypeCode(c.TypeCode)
			if !ok {
				t.Errorf("%s: the parser does not recognise %q (%s)", tbl.Version, c.TypeCode, k)
				continue
			}
			if got != k {
				t.Errorf("%s: %q maps to %s, but the code list files it under %s",
					tbl.Version, c.TypeCode, got, k)
			}
		}
	}
}

// TestTheParserIsCaseAndPrefixTolerant: buyer tools are not consistent about
// either, and a criterion lost to capitalisation is a criterion the operator is
// never asked about.
func TestTheParserIsCaseAndPrefixTolerant(t *testing.T) {
	for _, code := range []string{
		"CRITERION.EXCLUSION.CONVICTIONS.FRAUD",
		"criterion.exclusion.convictions.fraud",
		"EXCLUSION.CONVICTIONS.FRAUD",
		"  CRITERION.EXCLUSION.CONVICTIONS.FRAUD  ",
		"fraud",
		"FRAUD",
	} {
		if got, ok := espd.CriterionForTypeCode(code); !ok || got != espd.CritFraud {
			t.Errorf("%q -> %s, %v; want the fraud criterion", code, got, ok)
		}
	}
	if _, ok := espd.CriterionForTypeCode("CRITERION.SELECTION.SOMETHING.NEW"); ok {
		t.Error("an unknown code must be reported, not mapped to something")
	}
}
