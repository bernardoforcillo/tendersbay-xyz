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

	got := s.fuse(lexical, dense, now)
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

	got := s.fuse(lexical, dense, now)
	if !equalIDs(got, "a", "b") {
		t.Errorf("order = %v, want [a b] — a raw score must not override rank", ids(got))
	}
}

func TestFuse_WeightsShiftTheBalanceBetweenRetrievers(t *testing.T) {
	now := time.Now()
	lexical := scored("lex")
	dense := scored("vec")

	lexHeavy := fusionService(Ranking{LexicalWeight: 0.9, DenseWeight: 0.1}).fuse(lexical, dense, now)
	if lexHeavy[0].ID != "lex" {
		t.Errorf("order = %v, want the lexical hit first when lexical is weighted up", ids(lexHeavy))
	}

	denseHeavy := fusionService(Ranking{LexicalWeight: 0.1, DenseWeight: 0.9}).fuse(lexical, dense, now)
	if denseHeavy[0].ID != "vec" {
		t.Errorf("order = %v, want the vector hit first when dense is weighted up", ids(denseHeavy))
	}
}

// The top of both lists is the best attainable result, so it anchors the scale
// at 1.0. This matters beyond presentation: FitThresholds compares against
// RelevanceScore, so the scale is part of the contract.
func TestFuse_NormalisesTopOfBothListsToOne(t *testing.T) {
	s := fusionService(DefaultRanking())
	got := s.fuse(scored("a"), scored("a"), time.Now())
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
	got := s.fuse([]ScoredTender{closed, open}, []ScoredTender{closed, open}, now)
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

	got := s.fuse([]ScoredTender{expired, live}, []ScoredTender{expired, live}, now)
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

	got := s.fuse([]ScoredTender{expired}, nil, time.Now())
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

	got := s.fuse([]ScoredTender{a, b}, []ScoredTender{b, a}, now)
	if got[0].ID != "fresh" {
		t.Errorf("order = %v, want the freshly published tender to break the tie", ids(got))
	}
}

// Go map iteration is randomised, so an unsorted fusion would return a
// different order for the same query on every request.
func TestFuse_IsDeterministicForEqualScores(t *testing.T) {
	s := fusionService(DefaultRanking())
	now := time.Now()

	first := ids(s.fuse(scored("c", "a", "b"), scored("a", "b", "c"), now))
	for i := 0; i < 20; i++ {
		if got := ids(s.fuse(scored("c", "a", "b"), scored("a", "b", "c"), now)); !equalStrings(got, first) {
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
	if got != DefaultRanking() {
		t.Errorf("withDefaults() = %+v, want %+v", got, DefaultRanking())
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
