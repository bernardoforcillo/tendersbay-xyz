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
		t.Fatalf("len(set) = %d, want 2", len(set))
	}
	if set[0].ID != "cleaning-it-crosslang" || set[0].Language != "it" {
		t.Errorf("set[0] = %+v, want the Italian cross-language query first", set[0])
	}
	if set[0].Judgements["ted:de-1"] != 2 {
		t.Errorf("grade = %d, want 2", set[0].Judgements["ted:de-1"])
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
		t.Error("LoadGolden = nil error, want a duplicate-id failure")
	}
}

func TestLoadGolden_RejectsAnOutOfRangeGrade(t *testing.T) {
	// nDCG's gain function is defined for 0/1/2. A stray 3 would silently
	// change what the metric means rather than fail.
	fsys := mapFS{"g.json": []byte(`[{"id":"a","query":"x","language":"it","judgements":{"ted:1":3}}]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want an out-of-range grade failure")
	}
}

func TestLoadGolden_RejectsAQueryWithNothingRelevant(t *testing.T) {
	// Such a query contributes 0 to every metric no matter what the engine
	// does, so it can only dilute the report.
	fsys := mapFS{"g.json": []byte(`[{"id":"a","query":"x","language":"it","judgements":{"ted:1":0}}]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want a no-relevant-tenders failure")
	}
}

func TestLoadGolden_RejectsAnEmptyQueryText(t *testing.T) {
	// An empty query takes the filters-only browse path, where relevance is
	// undefined — it measures nothing about ranking.
	fsys := mapFS{"g.json": []byte(`[{"id":"a","query":"  ","language":"it","judgements":{"ted:1":2}}]`)}
	if _, err := LoadGolden(fsys, "g.json"); err == nil {
		t.Error("LoadGolden = nil error, want an empty-query failure")
	}
}
