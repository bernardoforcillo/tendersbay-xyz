package tender

import (
	"testing"
	"time"
)

// fusionService builds a Service with only the ranking config the fusion tests
// need — fuse touches no port.
func fusionService(r Ranking) *Service {
	return &Service{cfg: Config{Ranking: r}}
}

func scored(ids ...string) []ScoredTender {
	out := make([]ScoredTender, len(ids))
	for i, id := range ids {
		out[i] = ScoredTender{Tender: Tender{ID: id, Status: "open"}}
	}
	return out
}

func ids(results []ScoredTender) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.ID
	}
	return out
}

func equalIDs(got []ScoredTender, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i, id := range want {
		if got[i].ID != id {
			return false
		}
	}
	return true
}

// Fusion is what makes hybrid retrieval worth having: a tender both retrievers
// agree on must beat one that a single retriever ranked first and the other
// buried. Neither retriever alone can express that.
//
// Note the scale this needs: with RRFK at 60, positions near the top of a
// short list are deliberately almost indistinguishable. Agreement only
// outweighs a single first place once the disagreement is real — which is the
// damping constant doing its job, not the fusion failing.
func TestFuse_RewardsAgreementBetweenRetrievers(t *testing.T) {
	s := fusionService(DefaultRanking())
	now := time.Now()

	// "agreed" is third in both lists. "lexOnly" tops the lexical list but the
	// vector retriever ranks it 20th; "vecOnly" is the mirror image.
	lexical := scored("lexOnly", "x1", "agreed", "x2", "x3", "x4", "x5", "x6", "x7", "x8",
		"x9", "x10", "x11", "x12", "x13", "x14", "x15", "x16", "x17", "vecOnly")
	dense := scored("vecOnly", "y1", "agreed", "y2", "y3", "y4", "y5", "y6", "y7", "y8",
		"y9", "y10", "y11", "y12", "y13", "y14", "y15", "y16", "y17", "lexOnly")

	got := s.fuse(lexical, dense, nil, now)
	if got[0].ID != "agreed" {
		t.Errorf("order = %v, want \"agreed\" first — it is the only tender both retrievers rank highly", ids(got)[:3])
	}
}

// RRF works on ranks precisely so that two incomparable score scales (cosine
// similarity and ts_rank_cd) never have to be reconciled. A retriever's raw
// scores must therefore not influence the outcome at all.
func TestFuse_IgnoresRawScoresAndUsesRankOnly(t *testing.T) {
	s := fusionService(DefaultRanking())
	now := time.Now()

	lexical := scored("a", "b")
	dense := scored("a", "b")
	// Give "b" an enormous raw score. Its rank is still second, so it stays second.
	dense[1].RelevanceScore = 1000

	got := s.fuse(lexical, dense, nil, now)
	if !equalIDs(got, "a", "b") {
		t.Errorf("order = %v, want [a b] — a raw score must not override rank", ids(got))
	}
}

func TestFuse_WeightsShiftTheBalanceBetweenRetrievers(t *testing.T) {
	now := time.Now()
	lexical := scored("lex")
	dense := scored("vec")

	lexHeavy := fusionService(Ranking{LexicalWeight: 0.9, DenseWeight: 0.1}).fuse(lexical, dense, nil, now)
	if lexHeavy[0].ID != "lex" {
		t.Errorf("order = %v, want the lexical hit first when lexical is weighted up", ids(lexHeavy))
	}

	denseHeavy := fusionService(Ranking{LexicalWeight: 0.1, DenseWeight: 0.9}).fuse(lexical, dense, nil, now)
	if denseHeavy[0].ID != "vec" {
		t.Errorf("order = %v, want the vector hit first when dense is weighted up", ids(denseHeavy))
	}
}

// The top of both lists is the best attainable result, so it anchors the scale
// at 1.0. This matters beyond presentation: FitThresholds compares against
// RelevanceScore, so the scale is part of the contract.
func TestFuse_NormalisesTopOfBothListsToOne(t *testing.T) {
	s := fusionService(DefaultRanking())
	got := s.fuse(scored("a"), scored("a"), nil, time.Now())
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].RelevanceScore != 1 {
		t.Errorf("RelevanceScore = %v, want exactly 1 for rank 1 in both lists", got[0].RelevanceScore)
	}
}

// Text similarity alone ranks a tender awarded two years ago exactly like an
// open one — for someone looking for work to bid on, that is the wrong answer
// however well the words match.
func TestFuse_DemotesTendersThatCannotBeBidOn(t *testing.T) {
	s := fusionService(DefaultRanking())
	now := time.Now()

	closed := ScoredTender{Tender: Tender{ID: "closed", Status: "awarded"}}
	open := ScoredTender{Tender: Tender{ID: "open", Status: "open"}}

	// The closed tender is the better TEXT match: first in both lists.
	got := s.fuse([]ScoredTender{closed, open}, []ScoredTender{closed, open}, nil, now)
	if got[0].ID != "open" {
		t.Errorf("order = %v, want the open tender first despite ranking lower textually", ids(got))
	}
}

func TestFuse_DemotesExpiredDeadlines(t *testing.T) {
	s := fusionService(DefaultRanking())
	now := time.Now()
	past := now.Add(-24 * time.Hour)
	future := now.Add(30 * 24 * time.Hour)

	expired := ScoredTender{Tender: Tender{ID: "expired", Status: "open", Deadline: &past}}
	live := ScoredTender{Tender: Tender{ID: "live", Status: "open", Deadline: &future}}

	got := s.fuse([]ScoredTender{expired, live}, []ScoredTender{expired, live}, nil, now)
	if got[0].ID != "live" {
		t.Errorf("order = %v, want the tender that can still be bid on first", ids(got))
	}
}

// Demote, don't hide: a closed tender is legitimate research material, it just
// isn't what a bidder searching for work wants at the top.
func TestFuse_KeepsDemotedTendersInTheResults(t *testing.T) {
	s := fusionService(DefaultRanking())
	past := time.Now().Add(-24 * time.Hour)
	expired := ScoredTender{Tender: Tender{ID: "expired", Status: "cancelled", Deadline: &past}}

	got := s.fuse([]ScoredTender{expired}, nil, nil, time.Now())
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want the demoted tender still present", len(got))
	}
	if got[0].RelevanceScore <= 0 {
		t.Errorf("RelevanceScore = %v, want a positive but reduced score", got[0].RelevanceScore)
	}
}

func TestFuse_BoostsFreshlyPublishedTenders(t *testing.T) {
	s := fusionService(DefaultRanking())
	now := time.Now()
	fresh := now.Add(-24 * time.Hour)
	stale := now.Add(-365 * 24 * time.Hour)

	// Identical rank in both lists — only publication date differs.
	a := ScoredTender{Tender: Tender{ID: "stale", Status: "open", PublishedAt: &stale}}
	b := ScoredTender{Tender: Tender{ID: "fresh", Status: "open", PublishedAt: &fresh}}

	got := s.fuse([]ScoredTender{a, b}, []ScoredTender{b, a}, nil, now)
	if got[0].ID != "fresh" {
		t.Errorf("order = %v, want the freshly published tender to break the tie", ids(got))
	}
}

// Go map iteration is randomised, so an unsorted fusion would return a
// different order for the same query on every request.
func TestFuse_IsDeterministicForEqualScores(t *testing.T) {
	s := fusionService(DefaultRanking())
	now := time.Now()

	first := ids(s.fuse(scored("c", "a", "b"), scored("a", "b", "c"), nil, now))
	for i := 0; i < 20; i++ {
		if got := ids(s.fuse(scored("c", "a", "b"), scored("a", "b", "c"), nil, now)); !equalStrings(got, first) {
			t.Fatalf("order changed between identical calls: %v then %v", first, got)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A Config built without a Ranking must still rank, not score everything zero.
func TestRanking_ZeroValueFallsBackToDefaults(t *testing.T) {
	got := Ranking{}.withDefaults()
	// CPVWeight and CPVIndexExpanded are the one deliberate exception: their
	// zero value is a real "off" setting, not an absent one, so withDefaults
	// leaves them alone rather than inheriting DefaultRanking's — see
	// TestWithDefaults_LeavesTheCPVTogglesAlone. Every other knob must still
	// fall back to DefaultRanking's value, which this comparison still checks.
	want := DefaultRanking()
	want.CPVWeight = 0
	want.CPVIndexExpanded = false
	if got != want {
		t.Errorf("withDefaults() = %+v, want %+v", got, want)
	}
}

func TestRanking_WithDefaultsKeepsExplicitValues(t *testing.T) {
	got := Ranking{LexicalWeight: 0.8, DenseWeight: 0.2, RRFK: 10}.withDefaults()
	if got.LexicalWeight != 0.8 || got.DenseWeight != 0.2 || got.RRFK != 10 {
		t.Errorf("withDefaults() overwrote explicit weights: %+v", got)
	}
	if got.ClosedPenalty != DefaultRanking().ClosedPenalty {
		t.Errorf("ClosedPenalty = %v, want the default filled in", got.ClosedPenalty)
	}
}

func TestPage_ReportsHasMoreWhenARetrieverWasTruncated(t *testing.T) {
	ranked := scored("a", "b")

	// The fused set ends exactly at the page boundary, but a retriever hit its
	// own limit — so more matches exist and has_more must say so.
	out := page(ranked, 2, 0, true, ModeHybrid)
	if !out.HasMore {
		t.Error("HasMore = false, want true when a retriever was truncated")
	}

	out = page(ranked, 2, 0, false, ModeHybrid)
	if out.HasMore {
		t.Error("HasMore = true, want false when nothing was truncated and the page covers everything")
	}
}

func TestPage_PastTheEndReturnsEmptyNotNil(t *testing.T) {
	out := page(scored("a"), 10, 50, false, ModeHybrid)
	if out.Results == nil {
		t.Error("Results = nil, want an empty slice so JSON encodes [] rather than null")
	}
	if len(out.Results) != 0 || out.HasMore {
		t.Errorf("out = %+v, want an empty page with HasMore false", out)
	}
}

func TestRetrievalMode_Degraded(t *testing.T) {
	for mode, want := range map[RetrievalMode]bool{
		ModeHybrid:   false,
		ModeFilters:  false,
		ModeLexical:  true,
		ModeSemantic: true,
	} {
		if got := mode.Degraded(); got != want {
			t.Errorf("%q.Degraded() = %v, want %v", mode, got, want)
		}
	}
}

func TestDistinctByBestScore_KeepsBestChunkPerTenderInOrder(t *testing.T) {
	gotIDs, best := distinctByBestScore([]ScoredChunk{
		{DocID: "1", Score: 0.4},
		{DocID: "2", Score: 0.9},
		{DocID: "1", Score: 0.8}, // same tender, better chunk
		{DocID: "", Score: 1.0},  // malformed payload, must be skipped
	})
	if !equalStrings(gotIDs, []string{"1", "2"}) {
		t.Errorf("ids = %v, want [1 2] in first-seen order", gotIDs)
	}
	if best["1"] != 0.8 {
		t.Errorf("best[1] = %v, want 0.8 (the higher-scoring chunk)", best["1"])
	}
}

func TestParseSortOrder(t *testing.T) {
	for input, want := range map[string]SortOrder{
		"deadline":  SortDeadline,
		"PUBLISHED": SortPublished,
		" value ":   SortValue,
		"relevance": SortRelevance,
		// An unknown sort is a client bug; falling back beats failing the
		// whole search over the ordering of results it would still return.
		"nonsense": SortRelevance,
		"":         SortRelevance,
	} {
		if got := ParseSortOrder(input); got != want {
			t.Errorf("ParseSortOrder(%q) = %q, want %q", input, got, want)
		}
	}
}

// Sorting "soonest deadline first" naively puts the longest-expired notice at
// the very top, which is the opposite of useful.
func TestApplySort_DeadlinePushesExpiredAndUndatedLast(t *testing.T) {
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	past := now.AddDate(0, 0, -10)
	soon := now.AddDate(0, 0, 3)
	later := now.AddDate(0, 0, 40)

	results := []ScoredTender{
		{Tender: Tender{ID: "expired", Deadline: &past}},
		{Tender: Tender{ID: "none"}},
		{Tender: Tender{ID: "later", Deadline: &later}},
		{Tender: Tender{ID: "soon", Deadline: &soon}},
	}
	applySort(results, SortDeadline, now)

	if !equalIDs(results[:2], "soon", "later") {
		t.Errorf("order = %v, want the actionable deadlines first", ids(results))
	}
	if results[2].ID == "soon" || results[3].ID == "later" {
		t.Errorf("order = %v, want expired and undated tenders at the end", ids(results))
	}
}

// An unknown value is not a small one, but it can't outrank a known one either.
func TestApplySort_ValueAndPublishedPutUnknownsLast(t *testing.T) {
	now := time.Now()
	big, small := int64(1_000_000), int64(1_000)
	old := now.AddDate(-1, 0, 0)

	byValue := []ScoredTender{
		{Tender: Tender{ID: "unknown"}},
		{Tender: Tender{ID: "small", Value: &small}},
		{Tender: Tender{ID: "big", Value: &big}},
	}
	applySort(byValue, SortValue, now)
	if !equalIDs(byValue, "big", "small", "unknown") {
		t.Errorf("value order = %v, want [big small unknown]", ids(byValue))
	}

	byPublished := []ScoredTender{
		{Tender: Tender{ID: "undated"}},
		{Tender: Tender{ID: "old", PublishedAt: &old}},
		{Tender: Tender{ID: "new", PublishedAt: &now}},
	}
	applySort(byPublished, SortPublished, now)
	if !equalIDs(byPublished, "new", "old", "undated") {
		t.Errorf("published order = %v, want [new old undated]", ids(byPublished))
	}
}

// Relevance means "leave the fused order alone": re-sorting on the score would
// discard the deterministic tie-breaks fuse already applied.
func TestApplySort_RelevanceLeavesTheFusedOrderIntact(t *testing.T) {
	results := scored("c", "a", "b")
	applySort(results, SortRelevance, time.Now())
	if !equalIDs(results, "c", "a", "b") {
		t.Errorf("order = %v, want it untouched", ids(results))
	}
}

func TestFacetsOf_CountsByCountryStatusAndCPVDivision(t *testing.T) {
	got := facetsOf([]ScoredTender{
		{Tender: Tender{ID: "1", Country: "IT", Status: "open", CPV: "45233120"}},
		{Tender: Tender{ID: "2", Country: "IT", Status: "open", CPV: "45000000"}},
		{Tender: Tender{ID: "3", Country: "DE", Status: "awarded", CPV: "72000000"}},
		{Tender: Tender{ID: "4"}}, // no facets at all — must not become an "" bucket
	})

	if len(got.Countries) != 2 || got.Countries[0].Value != "IT" || got.Countries[0].Count != 2 {
		t.Errorf("Countries = %+v, want IT:2 first then DE:1", got.Countries)
	}
	if len(got.CPVDivisions) != 2 || got.CPVDivisions[0].Value != "45" || got.CPVDivisions[0].Count != 2 {
		t.Errorf("CPVDivisions = %+v, want division 45 counted twice", got.CPVDivisions)
	}
	for _, f := range got.Statuses {
		if f.Value == "" {
			t.Error("an empty status became a facet bucket")
		}
	}
}

// Map iteration is randomised, so unsorted facets would reorder the filter bar
// on every request.
func TestSortedFacets_IsDeterministic(t *testing.T) {
	counts := map[string]int{"a": 2, "b": 2, "c": 5, "d": 1}
	first := sortedFacets(counts)
	for i := 0; i < 20; i++ {
		got := sortedFacets(counts)
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("facet order changed between identical calls: %+v then %+v", first, got)
			}
		}
	}
	// Highest count first, ties broken by value.
	if first[0].Value != "c" || first[1].Value != "a" || first[2].Value != "b" {
		t.Errorf("order = %+v, want c, then a and b by value, then d", first)
	}
}

func TestFuse_CPVArmPromotesACodeMatchTheTextMissed(t *testing.T) {
	// The cross-language case in miniature: "de-1" shares no lexeme with the
	// query so the lexical arm never saw it, and it ranks poorly on the dense
	// arm — but it carries the CPV code the query resolved to.
	svc := fusionService(Ranking{LexicalWeight: 0.5, DenseWeight: 0.5, CPVWeight: 0.4, RRFK: 60})
	got := svc.fuse(scored("it-1"), scored("it-1", "de-1"), scored("de-1"), time.Now())

	if !equalIDs(got, "it-1", "de-1") {
		t.Fatalf("order = %v, want it-1 then de-1", ids(got))
	}
	// Without the CPV arm de-1 would sit at rank 2 of one list only; with it, it
	// also holds rank 1 of the CPV list.
	withoutCPV := fusionService(Ranking{LexicalWeight: 0.5, DenseWeight: 0.5, RRFK: 60}).
		fuse(scored("it-1"), scored("it-1", "de-1"), nil, time.Now())
	if got[1].RelevanceScore <= withoutCPV[1].RelevanceScore {
		t.Errorf("de-1 scored %v with the CPV arm and %v without; the arm must raise it",
			got[1].RelevanceScore, withoutCPV[1].RelevanceScore)
	}
}

func TestFuse_CPVArmNeverLowersAnyScore(t *testing.T) {
	// FitThresholds (RelevanceHigh 0.75, RelevanceLow 0.4) are calibrated against
	// this scale and read by computeFitTier. If enabling the arm could lower a
	// score, results would silently reclassify from "strong" to "possible" —
	// a regression in a feature this change does not touch.
	lexical, dense := scored("a", "b", "c"), scored("b", "a", "d")
	now := time.Now()

	base := fusionService(Ranking{LexicalWeight: 0.5, DenseWeight: 0.5, RRFK: 60}).
		fuse(lexical, dense, nil, now)
	withArm := fusionService(Ranking{LexicalWeight: 0.5, DenseWeight: 0.5, CPVWeight: 0.4, RRFK: 60}).
		fuse(lexical, dense, scored("d", "c"), now)

	baseScore := map[string]float64{}
	for _, r := range base {
		baseScore[r.ID] = r.RelevanceScore
	}
	for _, r := range withArm {
		if before, ok := baseScore[r.ID]; ok && r.RelevanceScore < before {
			t.Errorf("%s scored %v with the CPV arm, down from %v — the arm must only add", r.ID, r.RelevanceScore, before)
		}
	}
}

func TestFuse_ZeroCPVWeightReproducesTheTwoArmResultExactly(t *testing.T) {
	// The disable path has to be byte-identical, not merely similar: it is what
	// lets the harness measure the arm's effect as a controlled difference, and
	// what keeps every Ranking{} literal in the existing tests meaningful.
	lexical, dense := scored("a", "b"), scored("b", "c")
	now := time.Now()

	off := fusionService(Ranking{LexicalWeight: 0.5, DenseWeight: 0.5, CPVWeight: 0, RRFK: 60}).
		fuse(lexical, dense, scored("c"), now)
	twoArm := fusionService(Ranking{LexicalWeight: 0.5, DenseWeight: 0.5, RRFK: 60}).
		fuse(lexical, dense, nil, now)

	if len(off) != len(twoArm) {
		t.Fatalf("len = %d with CPVWeight 0, %d without the arm", len(off), len(twoArm))
	}
	for i := range off {
		if off[i].ID != twoArm[i].ID || off[i].RelevanceScore != twoArm[i].RelevanceScore {
			t.Errorf("position %d = %s/%v, want %s/%v", i, off[i].ID, off[i].RelevanceScore, twoArm[i].ID, twoArm[i].RelevanceScore)
		}
	}
}

func TestFuse_CPVOnlyResultStillScoresBelowAgreementOfBothRetrievers(t *testing.T) {
	// A taxonomy match is weaker evidence than a text match: the category says
	// what a notice is about, a title match says it is about exactly this.
	svc := fusionService(Ranking{LexicalWeight: 0.5, DenseWeight: 0.5, CPVWeight: 0.4, RRFK: 60})
	got := svc.fuse(scored("text"), scored("text"), scored("code"), time.Now())
	if !equalIDs(got, "text", "code") {
		t.Errorf("order = %v, want the lexical+dense agreement first", ids(got))
	}
}

func TestWithDefaults_LeavesTheCPVTogglesAlone(t *testing.T) {
	// Unlike every other knob, zero/false is MEANINGFUL here — it is how the arm
	// and the index expansion are switched off. Default-filling them would make
	// them impossible to disable and would silently change every existing test
	// that builds a bare Ranking{}.
	r := Ranking{}.withDefaults()
	if r.CPVWeight != 0 {
		t.Errorf("CPVWeight = %v, want 0 — a zero-value Ranking must leave the arm off", r.CPVWeight)
	}
	if r.CPVIndexExpanded {
		t.Error("CPVIndexExpanded = true, want false for a zero-value Ranking")
	}
	// …while DefaultRanking, which production uses, turns both on.
	d := DefaultRanking()
	if d.CPVWeight <= 0 || !d.CPVIndexExpanded {
		t.Errorf("DefaultRanking = %+v, want the CPV arm and index expansion enabled", d)
	}
}
