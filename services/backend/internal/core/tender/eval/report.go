package eval

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// Scores is one scope's averaged metrics. Queries is how many golden queries
// contributed, so a cell built from a single query is visibly thin rather than
// looking as authoritative as one built from twenty.
type Scores struct {
	Recall  float64 `json:"recall"`
	NDCG    float64 `json:"ndcg"`
	MRR     float64 `json:"mrr"`
	Queries int     `json:"queries"`
}

// QueryResult is one golden query's outcome, split by the language of the
// documents it actually retrieved.
type QueryResult struct {
	ID            string
	QueryLanguage string
	Scores        Scores
	ByDocLanguage map[string]Scores
}

// Report is a whole harness run.
//
// Matrix is the load-bearing view: rows are query language, columns are
// document language. The off-diagonal cells are cross-language retrieval,
// which is the requirement this whole project exists to satisfy — and which
// any aggregate number hides completely, since a healthy same-language score
// can sit right next to a total cross-language failure and average out to
// something that looks fine.
type Report struct {
	K               int                          `json:"k"`
	Overall         Scores                       `json:"overall"`
	ByQueryLanguage map[string]Scores            `json:"by_query_language"`
	Matrix          map[string]map[string]Scores `json:"matrix"`
	ByQuery         map[string]Scores            `json:"by_query"`
}

// accumulator sums scores so they can be averaged per query at the end,
// rather than per retrieved hit — see BuildReport.
type accumulator struct {
	recall, ndcg, mrr float64
	n                 int
}

func (a *accumulator) add(s Scores) {
	a.recall += s.Recall
	a.ndcg += s.NDCG
	a.mrr += s.MRR
	a.n++
}

// mean returns the zero Scores for an accumulator that never saw a query,
// rather than dividing by zero — a scope with no data must read as absent,
// not as a silent NaN spreading into whatever it's later mixed with.
func (a accumulator) mean() Scores {
	if a.n == 0 {
		return Scores{}
	}
	f := float64(a.n)
	return Scores{Recall: a.recall / f, NDCG: a.ndcg / f, MRR: a.mrr / f, Queries: a.n}
}

// BuildReport averages results per query — never per retrieved hit.
//
// Per-hit weighting would let one broadly-judged query dominate the whole
// report, so a single query with forty graded tenders could mask a dozen that
// return nothing. Grouping by query language and by query×document language
// happens in the same pass so every view is derived from the same per-query
// scores, never recomputed differently between views.
func BuildReport(k int, results []QueryResult) Report {
	overall := &accumulator{}
	byQL := map[string]*accumulator{}
	matrix := map[string]map[string]*accumulator{}
	byQuery := make(map[string]Scores, len(results))

	for _, r := range results {
		overall.add(r.Scores)
		byQuery[r.ID] = r.Scores

		if byQL[r.QueryLanguage] == nil {
			byQL[r.QueryLanguage] = &accumulator{}
		}
		byQL[r.QueryLanguage].add(r.Scores)

		for docLang, s := range r.ByDocLanguage {
			if matrix[r.QueryLanguage] == nil {
				matrix[r.QueryLanguage] = map[string]*accumulator{}
			}
			if matrix[r.QueryLanguage][docLang] == nil {
				matrix[r.QueryLanguage][docLang] = &accumulator{}
			}
			matrix[r.QueryLanguage][docLang].add(s)
		}
	}

	out := Report{
		K:               k,
		Overall:         overall.mean(),
		ByQueryLanguage: make(map[string]Scores, len(byQL)),
		Matrix:          make(map[string]map[string]Scores, len(matrix)),
		ByQuery:         byQuery,
	}
	for lang, acc := range byQL {
		out.ByQueryLanguage[lang] = acc.mean()
	}
	for qLang, row := range matrix {
		out.Matrix[qLang] = make(map[string]Scores, len(row))
		for dLang, acc := range row {
			out.Matrix[qLang][dLang] = acc.mean()
		}
	}
	return out
}

// Regression is one scope/metric pair that got worse. Metric is "missing"
// (rather than a metric name) when the whole scope disappeared from current —
// see Compare.
type Regression struct {
	Scope    string
	Metric   string
	Baseline float64
	Current  float64
}

// Compare reports every scope whose metric fell more than tolerance below the
// baseline, plus every baseline scope missing from the current run.
//
// It deliberately does NOT enforce an absolute threshold. Over ~50 hand-judged
// queries an absolute bar would be a number invented to be passed; a
// regression against a committed baseline is a claim that can actually be
// wrong.
//
// A tolerance exists because embedding output is not bit-stable across runs,
// so a hair's-width move is noise — and a harness that cries wolf on noise is
// a harness everyone learns to ignore.
//
// Note: metric names are formatted from baseline.K ("recall@%d"). If a caller
// ever compares reports built at different K, the labels would describe the
// baseline's window, not current's. That mismatch is a call-site precondition
// this function does not — and should not — guard itself.
func Compare(baseline, current Report, tolerance float64) []Regression {
	var out []Regression

	// check compares one scope's three metrics and appends a Regression for
	// each that dropped past tolerance. Recall/NDCG carry baseline.K in their
	// label so a reader can tell which window a regression was measured over
	// without cross-referencing the Report separately; MRR has no window.
	check := func(scope string, base, cur Scores) {
		metrics := []struct {
			name string
			base float64
			cur  float64
		}{
			{fmt.Sprintf("recall@%d", baseline.K), base.Recall, cur.Recall},
			{fmt.Sprintf("ndcg@%d", baseline.K), base.NDCG, cur.NDCG},
			{"mrr", base.MRR, cur.MRR},
		}
		for _, m := range metrics {
			if m.base-m.cur > tolerance {
				out = append(out, Regression{Scope: scope, Metric: m.name, Baseline: m.base, Current: m.cur})
			}
		}
	}

	check("overall", baseline.Overall, current.Overall)

	// sortedKeys drives every loop below so the emitted []Regression is in a
	// deterministic order — Compare's own output, not just Format's, would
	// otherwise reshuffle between runs and make two consecutive CI failures
	// on the same regression look like different diffs.
	for _, lang := range sortedKeys(baseline.ByQueryLanguage) {
		cur, ok := current.ByQueryLanguage[lang]
		if !ok {
			// The whole query language dropped out of the current run. There is
			// no current value to diff against, so this cannot go through check
			// — it must still be flagged, or losing an entire language reads as
			// "nothing to report" instead of the regression it is.
			out = append(out, Regression{Scope: lang, Metric: "missing", Baseline: baseline.ByQueryLanguage[lang].Recall})
			continue
		}
		check(lang, baseline.ByQueryLanguage[lang], cur)
	}

	for _, qLang := range sortedKeys(baseline.Matrix) {
		for _, dLang := range sortedKeys(baseline.Matrix[qLang]) {
			scope := qLang + "→" + dLang
			cur, ok := current.Matrix[qLang][dLang]
			if !ok {
				// The engine stopped retrieving this language pair entirely.
				// That is the most severe cross-language regression possible and
				// must never read as "unchanged".
				out = append(out, Regression{Scope: scope, Metric: "missing", Baseline: baseline.Matrix[qLang][dLang].Recall})
				continue
			}
			check(scope, baseline.Matrix[qLang][dLang], cur)
		}
	}

	// A second, explicit sort on top of the already-sorted-key iteration: the
	// three loops above append in "overall" / by-language / matrix order, not
	// in scope-alphabetical order, so without this the "en" missing-language
	// regression could sit either side of the "it→de" matrix regression
	// depending on which section ran first.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Metric < out[j].Metric
	})
	return out
}

// sortedKeys orders a map's keys so every report and comparison is byte-stable
// across runs — map iteration alone would reshuffle the table and make a
// baseline diff unreadable.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Format renders the report as a fixed-order text table.
func (r Report) Format() string {
	var b strings.Builder
	line := func(scope string, s Scores) {
		fmt.Fprintf(&b, "%-14s recall@%d=%.4f  ndcg@%d=%.4f  mrr=%.4f  (%d queries)\n",
			scope, r.K, s.Recall, r.K, s.NDCG, s.MRR, s.Queries)
	}

	line("overall", r.Overall)
	b.WriteString("\nby query language:\n")
	for _, lang := range sortedKeys(r.ByQueryLanguage) {
		line("  "+lang, r.ByQueryLanguage[lang])
	}
	b.WriteString("\nquery language → document language:\n")
	for _, qLang := range sortedKeys(r.Matrix) {
		for _, dLang := range sortedKeys(r.Matrix[qLang]) {
			line("  "+qLang+"→"+dLang, r.Matrix[qLang][dLang])
		}
	}
	return b.String()
}

// LoadBaseline reads a committed baseline report.
func LoadBaseline(fsys fs.FS, name string) (Report, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return Report{}, fmt.Errorf("eval: read baseline %s: %w", name, err)
	}
	var r Report
	if err := json.Unmarshal(raw, &r); err != nil {
		return Report{}, fmt.Errorf("eval: parse baseline %s: %w", name, err)
	}
	// K==0 is not a real window size BuildReport ever produces — it means the
	// file is hand-edited, from an older format, or otherwise not a genuine
	// prior run. Accepting it would make Compare label every metric
	// "recall@0" and diff today's run against a baseline that means nothing.
	if r.K == 0 {
		return Report{}, fmt.Errorf("eval: baseline %s has k=0; it was not written by BuildReport", name)
	}
	return r, nil
}

// MarshalBaseline renders a report as the JSON that belongs in
// testdata/baseline.json, indented so a baseline change is a readable diff.
func MarshalBaseline(r Report) ([]byte, error) {
	out, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("eval: marshal baseline: %w", err)
	}
	return append(out, '\n'), nil
}
