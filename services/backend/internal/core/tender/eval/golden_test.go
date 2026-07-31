package eval

import (
	"os"
	"testing"
)

func TestLoadGolden_ReadsQueriesAndGrades(t *testing.T) {
	set, err := LoadGolden(os.DirFS("testdata"), "golden-tiny.json")
	if err != nil {
		t.Fatalf("LoadGolden: %v", err)
	}
	if len(set) != 2 {
		t.Fatalf("len(set) = %d, want 2 — a loader that silently drops or duplicates a query would still return no error, and the count is the only place that shows up before every aggregate downstream is wrong", len(set))
	}
	if set[0].ID != "cleaning-it-crosslang" || set[0].Language != "it" {
		t.Errorf("set[0] = %+v, want the Italian cross-language query first — a field decoded onto the wrong query would silently misattribute its judgements in the report", set[0])
	}
	if set[0].Judgements["ted:de-1"] != 2 {
		t.Errorf("grade = %d, want 2 — confirms judgements decode keyed by source:source_ref, the exact identity the metrics join against", set[0].Judgements["ted:de-1"])
	}
}

func TestLoadGolden_RejectsADuplicateID(t *testing.T) {
	// Two queries sharing an id would collide in the per-query report and one
	// would silently overwrite the other, so refuse the set outright.
	fsys := mapFS{"g.json": []byte(`[
	  {"id":"a","query":"x","language":"it","judgements":{"ted:1":2}},
	  {"id":"a","query":"y","language":"it","judgements":{"ted:2":2}}
	]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want a duplicate-id failure — a silently accepted duplicate id would collide in the per-query report and overwrite one query's score with the other's, looking exactly like a query that simply scored badly")
	}
}

func TestLoadGolden_RejectsAnOutOfRangeGrade(t *testing.T) {
	// nDCG's gain function is defined for 0/1/2. A stray 3 would silently
	// change what the metric means rather than fail.
	fsys := mapFS{"g.json": []byte(`[{"id":"a","query":"x","language":"it","judgements":{"ted:1":3}}]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want an out-of-range grade failure — a silently accepted grade outside 0/1/2 would change what nDCG's gain function means for that judgement, looking exactly like a query that simply scored badly")
	}
}

func TestLoadGolden_RejectsAQueryWithNothingRelevant(t *testing.T) {
	// Such a query contributes 0 to every metric no matter what the engine
	// does, so it can only dilute the report.
	fsys := mapFS{"g.json": []byte(`[{"id":"a","query":"x","language":"it","judgements":{"ted:1":0}}]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want a no-relevant-tenders failure — a silently accepted query with nothing relevant would score 0 no matter what the engine does, diluting the report while looking exactly like a query that simply scored badly")
	}
}

func TestLoadGolden_RejectsAnEmptyQueryText(t *testing.T) {
	// An empty query takes the filters-only browse path, where relevance is
	// undefined — it measures nothing about ranking.
	fsys := mapFS{"g.json": []byte(`[{"id":"a","query":"  ","language":"it","judgements":{"ted:1":2}}]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want an empty-query failure — a silently accepted empty query takes the undefined browse path, and its score would look exactly like a query that simply scored badly")
	}
}

func TestLoadGolden_RejectsAnEmptySet(t *testing.T) {
	// An empty set has nothing for the harness to report on; accepting it as
	// valid would let a broken export (an empty glob, a failed fetch) pass as
	// a clean run with zero findings instead of failing loudly.
	fsys := mapFS{"g.json": []byte(`[]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want an empty-set failure — a silently accepted empty set would let a broken export pass as a clean zero-query run instead of failing loudly")
	}
}

func TestLoadGolden_RejectsAMissingLanguage(t *testing.T) {
	// The report groups results by language; a query with none would either
	// silently vanish from every per-language breakdown or land in an
	// empty-string bucket, both wrong in ways that look like a clean report.
	fsys := mapFS{"g.json": []byte(`[{"id":"a","query":"x","judgements":{"ted:1":2}}]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want a missing-language failure — a silently accepted query with no language would vanish from every per-language breakdown, looking exactly like a query that simply scored badly")
	}
}
