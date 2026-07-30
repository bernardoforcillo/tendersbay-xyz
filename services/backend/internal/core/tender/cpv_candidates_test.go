package tender

import (
	"context"
	"errors"
	"testing"
)

// cpvArmFakeLexicon records what it was asked and returns canned matches.
type cpvArmFakeLexicon struct {
	gotQuery string
	gotLimit int
	matches  []CPVMatch
	err      error
}

func (f *cpvArmFakeLexicon) MatchCodes(_ context.Context, query string, limit int) ([]CPVMatch, error) {
	f.gotQuery, f.gotLimit = query, limit
	return f.matches, f.err
}

// cpvArmFakeRepo answers only the two methods the arm touches.
type cpvArmFakeRepo struct {
	parseFakeRepo // reuse the nine-method stub already in this package
	byCPV         []ScoredTender
	byCPVErr      error
	gotCodes      []string
}

func (f *cpvArmFakeRepo) FindByCPVPrefixes(_ context.Context, codes []string, _ Filters, _ int) ([]ScoredTender, error) {
	f.gotCodes = codes
	return f.byCPV, f.byCPVErr
}

// cpvArmTestRanking turns the CPV arm ON for tests that exercise it directly.
//
// DefaultRanking ships with CPVWeight 0 — Task 14 measured the arm against the
// live eval corpus and it made every metric worse (see hybrid.go's
// DefaultRanking doc comment and task-14-report.md), so it ships disabled.
// These tests are specifically about what the arm DOES when it runs, so they
// build their own enabled Ranking rather than depending on a default that
// deliberately turns it off.
func cpvArmTestRanking() Ranking {
	r := DefaultRanking()
	r.CPVWeight = 0.4
	return r
}

func TestCPVCandidates_ResolvesCodesThenFetchesTendersCarryingThem(t *testing.T) {
	lex := &cpvArmFakeLexicon{matches: []CPVMatch{
		{Code: "90919200", Lang: "it", Label: "Servizi di pulizia di uffici", Score: 0.9},
		{Code: "90911200", Lang: "it", Label: "Servizi di pulizia di edifici", Score: 0.4},
	}}
	repo := &cpvArmFakeRepo{byCPV: scored("de-1", "fr-1")}
	svc := &Service{repo: repo, lexicon: lex, cfg: Config{Ranking: cpvArmTestRanking()}}

	got, matches := svc.cpvCandidates(context.Background(), "pulizie uffici", Filters{}, 20)

	if !equalIDs(got, "de-1", "fr-1") {
		t.Errorf("arm = %v, want de-1 and fr-1", ids(got))
	}
	if len(matches) != 2 || matches[0].Code != "90919200" {
		t.Errorf("matches = %+v, want both RESOLVED codes with the strongest first — AppliedCPV must show what the query was actually read as, not the widened prefix it was searched with", matches)
	}
	// Both leaves truncate to the same 4-digit class "9091" — this is the
	// exact pair the live corpus surfaced (90919200 IT vs 90911200 DE), and is
	// why matching the untruncated leaf found neither: the repo must be asked
	// for the class, not the two distinct 8-digit leaves.
	if !equalStrings(repo.gotCodes, []string{"9091", "9091"}) {
		t.Errorf("repo asked for %v, want both codes truncated to their shared class prefix, not the raw 8-digit leaves", repo.gotCodes)
	}
	if lex.gotQuery != "pulizie uffici" {
		t.Errorf("lexicon got %q, want the query text", lex.gotQuery)
	}
}

func TestCPVCandidates_NilLexiconContributesNothing(t *testing.T) {
	// The lexicon is optional in exactly the way EmbeddingCache is: a Service
	// without one must behave as it did before the arm existed.
	svc := &Service{repo: &cpvArmFakeRepo{}, cfg: Config{Ranking: cpvArmTestRanking()}}
	got, matches := svc.cpvCandidates(context.Background(), "pulizie", Filters{}, 20)
	if got != nil || matches != nil {
		t.Errorf("arm = %v / %v, want nil for a Service with no lexicon", ids(got), matches)
	}
}

func TestCPVCandidates_ALexiconFailureIsSwallowed(t *testing.T) {
	// A vocabulary table that is missing, unseeded or unreachable must not fail a
	// search that lexical and dense can answer between them.
	lex := &cpvArmFakeLexicon{err: errors.New("relation cpv_terms does not exist")}
	svc := &Service{repo: &cpvArmFakeRepo{}, lexicon: lex, cfg: Config{Ranking: cpvArmTestRanking()}}
	got, matches := svc.cpvCandidates(context.Background(), "pulizie", Filters{}, 20)
	if got != nil || matches != nil {
		t.Errorf("arm = %v / %v, want nil after a lexicon error", ids(got), matches)
	}
}

func TestCPVCandidates_ARepoFailureStillReportsWhatWasUnderstood(t *testing.T) {
	// The codes were resolved successfully; only the fetch failed. The UI should
	// still be able to show "read as: 90919200" — the inference is real even when
	// the arm contributed no candidates.
	lex := &cpvArmFakeLexicon{matches: []CPVMatch{{Code: "90919200", Label: "x"}}}
	repo := &cpvArmFakeRepo{byCPVErr: errors.New("boom")}
	svc := &Service{repo: repo, lexicon: lex, cfg: Config{Ranking: cpvArmTestRanking()}}

	got, matches := svc.cpvCandidates(context.Background(), "pulizie", Filters{}, 20)
	if got != nil {
		t.Errorf("arm = %v, want nil after a repo error", ids(got))
	}
	if len(matches) != 1 {
		t.Errorf("matches = %+v, want the resolved code reported anyway", matches)
	}
}

func TestCPVCandidates_SkippedEntirelyWhenTheArmIsDisabled(t *testing.T) {
	// CPVWeight 0 must not even reach the lexicon: the arm would contribute
	// nothing to fusion, so resolving codes would be a wasted round trip on
	// every single search.
	lex := &cpvArmFakeLexicon{matches: []CPVMatch{{Code: "90919200"}}}
	svc := &Service{repo: &cpvArmFakeRepo{}, lexicon: lex, cfg: Config{Ranking: Ranking{
		LexicalWeight: 0.5, DenseWeight: 0.5, RRFK: 60, CPVWeight: 0,
	}}}

	got, matches := svc.cpvCandidates(context.Background(), "pulizie", Filters{}, 20)
	if got != nil || matches != nil {
		t.Errorf("arm = %v / %v, want nil with CPVWeight 0", ids(got), matches)
	}
	if lex.gotQuery != "" {
		t.Errorf("lexicon was called with %q, want not called at all", lex.gotQuery)
	}
}

func TestSearchHybrid_ReportsAppliedCPVAndFusesTheArm(t *testing.T) {
	lex := &cpvArmFakeLexicon{matches: []CPVMatch{{Code: "90919200", Lang: "it", Label: "Servizi di pulizia di uffici"}}}
	repo := &cpvArmFakeRepo{byCPV: scored("de-1")}
	svc := &Service{repo: repo, kb: parseFakeKB{}, lexicon: lex, cfg: Config{Ranking: cpvArmTestRanking()}}

	out, err := svc.searchHybrid(context.Background(), "pulizie uffici", Filters{}, SortRelevance, 20, 0)
	if err != nil {
		t.Fatalf("searchHybrid: %v", err)
	}
	if len(out.AppliedCPV) != 1 || out.AppliedCPV[0].Code != "90919200" {
		t.Errorf("AppliedCPV = %+v, want the resolved code — a signal the user can't see is one they can't correct", out.AppliedCPV)
	}
	var sawDE bool
	for _, r := range out.Results {
		if r.ID == "de-1" {
			sawDE = true
		}
	}
	if !sawDE {
		t.Errorf("results = %v, want the CPV-only match de-1 fused in", ids(out.Results))
	}
}

func TestSearchHybrid_DefaultRankingLeavesTheArmOff(t *testing.T) {
	// Task 14's measured verdict: the arm made every eval metric worse and
	// left its four target cells at 0.0000 regardless of prefix depth (see
	// task-14-report.md), so DefaultRanking ships it disabled. A search built
	// with the real default must not resolve any CPV codes or fuse in a
	// CPV-only result — this is the "shipped switched off" contract, checked
	// end to end rather than just at the CPVWeight field.
	lex := &cpvArmFakeLexicon{matches: []CPVMatch{{Code: "90919200", Lang: "it", Label: "Servizi di pulizia di uffici"}}}
	repo := &cpvArmFakeRepo{byCPV: scored("de-1")}
	svc := &Service{repo: repo, kb: parseFakeKB{}, lexicon: lex, cfg: Config{Ranking: DefaultRanking()}}

	out, err := svc.searchHybrid(context.Background(), "pulizie uffici", Filters{}, SortRelevance, 20, 0)
	if err != nil {
		t.Fatalf("searchHybrid: %v", err)
	}
	if len(out.AppliedCPV) != 0 {
		t.Errorf("AppliedCPV = %+v, want empty — DefaultRanking's CPVWeight is 0", out.AppliedCPV)
	}
	for _, r := range out.Results {
		if r.ID == "de-1" {
			t.Errorf("results = %v, want de-1 (a CPV-only match) absent — the arm must not fuse in candidates when it is off", ids(out.Results))
		}
	}
	if lex.gotQuery != "" {
		t.Errorf("lexicon was called with %q, want not called at all — resolving codes for an arm that contributes nothing is a wasted round trip", lex.gotQuery)
	}
}

// cpvHierarchyPrefixCases are the falsifiable behaviours cpvHierarchyPrefix
// must satisfy. Table-driven because every case is the same shape: one input
// code, one expected prefix, one reason the case matters.
var cpvHierarchyPrefixCases = []struct {
	name string
	code string
	want string
}{
	{
		// The worked example from the live corpus: an Italian query resolves to
		// this exact 8-digit leaf, and the fix must back it off to the 4-digit
		// class the sibling German/French/Spanish tenders actually share.
		name: "fully specified leaf backs off to its class",
		code: "90919200",
		want: "9091",
	},
	{
		// A leaf with no trailing-zero padding at all: nothing to strip first,
		// so the "drop one level" step alone must still produce a shorter,
		// even-length prefix rather than leaving the leaf untouched.
		name: "leaf with no trailing zeros still drops one level",
		code: "71317210",
		want: "713172",
	},
	{
		// Already at division level (2 significant digits, fully padded).
		// There is no coarser level below "division" without matching every
		// CPV code in the database, so this must NOT collapse to "".
		name: "division-only code is left alone, not emptied",
		code: "45000000",
		want: "45",
	},
	{
		// A single significant digit sitting at an ODD position. Rounding the
		// significant length DOWN here would cut the '9' itself and leave an
		// empty division ("" from code[:0]) — silently matching everything.
		name: "single significant digit rounds up, not down, to an even boundary",
		code: "90000000",
		want: "90",
	},
	{
		// Defensive: a code shorter than the normal 8-digit leaf (should never
		// happen from the lexicon, but the function must not panic or slice
		// out of range on it) is returned unchanged once at/below 2 digits.
		name: "already-short input is returned as-is",
		code: "45",
		want: "45",
	},
}

func TestCPVHierarchyPrefix_MatchesTheWorkedExamplesAndEdgeCases(t *testing.T) {
	for _, c := range cpvHierarchyPrefixCases {
		t.Run(c.name, func(t *testing.T) {
			got := cpvHierarchyPrefix(c.code)
			if got != c.want {
				t.Errorf("cpvHierarchyPrefix(%q) = %q, want %q — %s", c.code, got, c.want, c.name)
			}
		})
	}
}

func TestCPVHierarchyPrefix_NeverReturnsEmpty(t *testing.T) {
	// An empty prefix is not "coarser", it is "match everything" — the LIKE
	// clause would degenerate into no filter at all. No input in
	// cpvHierarchyPrefixCases (or the pathological "00000000") may produce one.
	for _, c := range append(cpvHierarchyPrefixCases, struct{ name, code, want string }{"all zero digits", "00000000", ""}) {
		if got := cpvHierarchyPrefix(c.code); got == "" {
			t.Errorf("cpvHierarchyPrefix(%q) = %q, want a non-empty prefix — an empty LIKE prefix matches every tender in the database", c.code, got)
		}
	}
}
