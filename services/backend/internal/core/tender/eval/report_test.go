package eval

import (
	"strings"
	"testing"
)

// sample builds a three-query report spanning two query languages and three
// document languages, including one cross-language pair (it→de) and one
// same-language pair (it→it), so both Matrix and ByQueryLanguage have
// something non-trivial to assert against.
func sample() Report {
	return BuildReport(20, []QueryResult{
		{
			ID: "it-1", QueryLanguage: "it",
			Scores:        Scores{Recall: 1.0, NDCG: 0.9, MRR: 1.0, Queries: 1},
			ByDocLanguage: map[string]Scores{"de": {Recall: 1.0, NDCG: 0.9, MRR: 1.0, Queries: 1}},
		},
		{
			ID: "it-2", QueryLanguage: "it",
			Scores:        Scores{Recall: 0.5, NDCG: 0.5, MRR: 0.5, Queries: 1},
			ByDocLanguage: map[string]Scores{"it": {Recall: 0.5, NDCG: 0.5, MRR: 0.5, Queries: 1}},
		},
		{
			ID: "en-1", QueryLanguage: "en",
			Scores:        Scores{Recall: 0.0, NDCG: 0.0, MRR: 0.0, Queries: 1},
			ByDocLanguage: map[string]Scores{"fr": {Recall: 0.0, NDCG: 0.0, MRR: 0.0, Queries: 1}},
		},
	})
}

func TestBuildReport_AveragesPerQueryNotPerHit(t *testing.T) {
	// Every query weighs the same regardless of how many relevant tenders it
	// has, or a single broadly-judged query would dominate the whole report.
	r := sample()
	want := (1.0 + 0.5 + 0.0) / 3
	if !close(r.Overall.Recall, want) {
		t.Errorf("Overall.Recall = %v, want %v — per-hit weighting would let one broadly-judged query dominate the report instead of each query counting equally", r.Overall.Recall, want)
	}
	if r.Overall.Queries != 3 {
		t.Errorf("Overall.Queries = %d, want 3 — a wrong count would silently change the denominator every average downstream divides by", r.Overall.Queries)
	}
}

func TestBuildReport_GroupsByQueryLanguage(t *testing.T) {
	r := sample()
	it, ok := r.ByQueryLanguage["it"]
	if !ok {
		t.Fatalf("ByQueryLanguage has no it — got %v, so a whole query language would be invisible from the per-language breakdown", r.ByQueryLanguage)
	}
	if it.Queries != 2 || !close(it.Recall, 0.75) {
		t.Errorf("ByQueryLanguage[it] = %+v, want 2 queries averaging 0.75 recall — a wrong grouping would mix it-1/it-2 into en or merge only one of the two it queries", it)
	}
}

func TestBuildReport_MatrixSeparatesCrossLanguageFromSameLanguage(t *testing.T) {
	// This is the whole point of the matrix: an aggregate that mixes it→it with
	// it→de hides a total cross-language failure behind a healthy same-language
	// number.
	r := sample()
	if got := r.Matrix["it"]["de"]; !close(got.Recall, 1.0) {
		t.Errorf("Matrix[it][de].Recall = %v, want 1.0 — collapsing this cross-language cell into the it→it number would hide a total cross-language failure behind a healthy same-language score", got.Recall)
	}
	if got := r.Matrix["it"]["it"]; !close(got.Recall, 0.5) {
		t.Errorf("Matrix[it][it].Recall = %v, want 0.5 — the same-language cell must stay independent of the it→de cell above it, or the two would bleed into a single misleading average", got.Recall)
	}
	if _, ok := r.Matrix["en"]["de"]; ok {
		t.Error("Matrix[en][de] exists, want absent — no en query touched a de document, and a spurious cell would report a cross-language pair that was never actually evaluated")
	}
}

func TestCompare_ReportsAnOverallDropBeyondTolerance(t *testing.T) {
	base := sample()
	worse := BuildReport(20, []QueryResult{
		{ID: "it-1", QueryLanguage: "it", Scores: Scores{Recall: 0.1, NDCG: 0.1, MRR: 0.1, Queries: 1}},
	})
	regs := Compare(base, worse, 0.01)
	if len(regs) == 0 {
		t.Fatal("Compare = no regressions, want at least the overall recall drop — a comparison that misses a real 0.5→0.1 drop defeats the entire purpose of a regression gate")
	}
	var sawRecall bool
	for _, r := range regs {
		if r.Scope == "overall" && r.Metric == "recall@20" {
			sawRecall = true
		}
	}
	if !sawRecall {
		t.Errorf("regressions = %+v, want one for overall recall@20 — the drop from a 0.5 baseline mean to 0.1 is far past the 0.01 tolerance and must surface under the exact scope/metric callers filter on", regs)
	}
}

func TestCompare_IgnoresADropInsideTolerance(t *testing.T) {
	// Embedding output is not bit-stable across runs, so a hair's-width move is
	// noise. Failing on it would train everyone to ignore the harness.
	base := BuildReport(20, []QueryResult{
		{ID: "a", QueryLanguage: "it", Scores: Scores{Recall: 0.800, NDCG: 0.8, MRR: 0.8, Queries: 1}},
	})
	current := BuildReport(20, []QueryResult{
		{ID: "a", QueryLanguage: "it", Scores: Scores{Recall: 0.795, NDCG: 0.8, MRR: 0.8, Queries: 1}},
	})
	if regs := Compare(base, current, 0.01); len(regs) != 0 {
		t.Errorf("Compare = %+v, want none within tolerance — flagging a 0.005 move as a regression would train everyone to ignore the harness the first time it cries wolf", regs)
	}
}

func TestCompare_ReportsAScopeThatDisappeared(t *testing.T) {
	// A matrix cell present in the baseline and gone now means the engine
	// stopped retrieving that language pair at all — the most severe
	// cross-language regression there is, and it must not read as "unchanged".
	base := sample()
	current := BuildReport(20, []QueryResult{
		{ID: "it-2", QueryLanguage: "it", Scores: Scores{Recall: 0.5, NDCG: 0.5, MRR: 0.5, Queries: 1},
			ByDocLanguage: map[string]Scores{"it": {Recall: 0.5, NDCG: 0.5, MRR: 0.5, Queries: 1}}},
	})
	var sawMissing bool
	var missingMetric string
	for _, r := range Compare(base, current, 0.01) {
		if r.Scope == "it→de" {
			sawMissing = true
			missingMetric = r.Metric
		}
	}
	if !sawMissing {
		t.Error("Compare did not flag the vanished it→de cell — dropping a language pair entirely must not read as \"unchanged\" just because there is no current value to subtract from")
	}
	if missingMetric != "missing" {
		t.Errorf("Metric for the vanished it→de cell = %q, want %q — callers filter on this label to tell a value drop apart from a scope that stopped being retrieved at all", missingMetric, "missing")
	}
}

func TestCompare_ReportsAByQueryLanguageScopeThatDisappeared(t *testing.T) {
	// Same failure mode as the matrix case, one level up: an entire query
	// language dropping out of the current run (not just one document-language
	// cell within it) must also be flagged, not silently skipped because there
	// is nothing in `current` to compare against.
	base := sample() // ByQueryLanguage has both "it" and "en"
	current := BuildReport(20, []QueryResult{
		{ID: "it-1", QueryLanguage: "it", Scores: Scores{Recall: 1.0, NDCG: 0.9, MRR: 1.0, Queries: 1}},
		{ID: "it-2", QueryLanguage: "it", Scores: Scores{Recall: 0.5, NDCG: 0.5, MRR: 0.5, Queries: 1}},
	})
	var sawMissing bool
	var missingMetric string
	for _, r := range Compare(base, current, 0.01) {
		if r.Scope == "en" {
			sawMissing = true
			missingMetric = r.Metric
		}
	}
	if !sawMissing {
		t.Error("Compare did not flag the vanished en query-language scope — a whole language dropping out of the run is a regression, not silence")
	}
	if missingMetric != "missing" {
		t.Errorf("Metric for the vanished en scope = %q, want %q — the same missing-scope label the matrix arm uses, so callers filter both cases the same way", missingMetric, "missing")
	}
}

func TestFormat_ShowsTheMatrixAndIsStablyOrdered(t *testing.T) {
	// Ordered output so a baseline diff is readable; map iteration alone would
	// reshuffle the table on every run.
	got := sample().Format()
	for _, want := range []string{"overall", "recall@20", "it→de", "it→it", "en→fr"} {
		if !strings.Contains(got, want) {
			t.Errorf("Format() missing %q — a scope absent from the text report is invisible to anyone reading a baseline diff, however correct the underlying Report value is\n%s", want, got)
		}
	}
	if got != sample().Format() {
		t.Error("Format() is not deterministic — unordered map iteration would reshuffle the table between calls and make a baseline diff unreadable")
	}
}

func TestLoadBaseline_RejectsKZero(t *testing.T) {
	// A baseline JSON with k=0 was never produced by BuildReport — K is always
	// the caller's real window size — so it signals a hand-edited or corrupted
	// file. Accepting it would let Compare label every metric "recall@0" and
	// diff today's run against a baseline that was never a real prior run.
	fsys := mapFS{"baseline.json": []byte(
		`{"k":0,"overall":{"recall":0,"ndcg":0,"mrr":0,"queries":0},"by_query_language":{},"matrix":{},"by_query":{}}`,
	)}
	if _, err := LoadBaseline(fsys, "baseline.json"); err == nil {
		t.Error("LoadBaseline = nil error, want a k=0 rejection — a silently accepted k=0 baseline would mislabel every comparison metric as recall@0 and compare against a file BuildReport never produced")
	}
}

func TestLoadBaseline_ReadsAValidBaseline(t *testing.T) {
	// Round-trips a real report through MarshalBaseline and back, pinning the
	// two functions against each other: a broken encode or decode would corrupt
	// every future regression comparison while the load itself reports no error.
	want := sample()
	raw, err := MarshalBaseline(want)
	if err != nil {
		t.Fatalf("MarshalBaseline: %v", err)
	}
	got, err := LoadBaseline(mapFS{"baseline.json": raw}, "baseline.json")
	if err != nil {
		t.Fatalf("LoadBaseline: %v", err)
	}
	if got.K != want.K || !close(got.Overall.Recall, want.Overall.Recall) {
		t.Errorf("LoadBaseline round-trip = %+v, want K=%d and Overall.Recall=%v — a broken round trip would silently compare every future run against a corrupted baseline", got, want.K, want.Overall.Recall)
	}
}
