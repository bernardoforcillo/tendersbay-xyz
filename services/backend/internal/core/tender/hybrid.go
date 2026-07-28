package tender

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// RetrievalMode records which retrieval paths actually produced the results of
// one search. It is reported back to the caller because a search that silently
// answered a different question than the one asked is worse than a visible
// failure: before this existed, a Qdrant outage turned every query into "the
// most recently published tenders", presented as if they matched.
type RetrievalMode string

const (
	// ModeFilters — no query text, so results are a filtered browse ordered by
	// publication date. Relevance is undefined and reported as zero.
	ModeFilters RetrievalMode = "filters"
	// ModeHybrid — both retrievers ran and were fused. The normal path.
	ModeHybrid RetrievalMode = "hybrid"
	// ModeLexical — keyword retrieval only; the vector store was unavailable.
	ModeLexical RetrievalMode = "lexical"
	// ModeSemantic — vector retrieval only; the lexical query failed.
	ModeSemantic RetrievalMode = "semantic"
)

// Degraded reports whether the mode is missing a retriever that a query-driven
// search would normally use.
func (m RetrievalMode) Degraded() bool {
	return m == ModeLexical || m == ModeSemantic
}

// Retrieval-window bounds. They exist because a chunk is not a tender: one
// tender with a long attached PDF occupies dozens of vector points, so a
// candidate budget counted in chunks silently collapses to a handful of
// distinct tenders. Everything here is therefore counted in TENDERS, with the
// chunk fan-out as an explicit multiplier on top.
const (
	// denseChunkFanout is how many vector chunks to request per distinct
	// tender wanted, as a first guess before the dedupe.
	denseChunkFanout = 8
	// maxDenseChunks bounds one Qdrant round trip regardless of the fan-out.
	maxDenseChunks = 600
	// maxWindow bounds how deep into the ranking one request may reach. Paging
	// past it returns empty rather than an ever-growing retrieval.
	maxWindow = 300
)

// Ranking tunes fusion and the post-fusion business boost. Like FitThresholds,
// these are deliberately data, not code: they are an initial, uncalibrated
// guess (there is no click data yet) and are meant to be retuned from
// main.go without touching the ranking logic.
type Ranking struct {
	// LexicalWeight and DenseWeight split the fused score between the two
	// retrievers. Equal weights say "neither retriever is trusted more"; raise
	// LexicalWeight if users search mostly by code and buyer name.
	LexicalWeight float64
	DenseWeight   float64
	// RRFK is Reciprocal Rank Fusion's damping constant. Fusion works on RANKS,
	// not raw scores, precisely because a cosine similarity and a ts_rank_cd
	// value are not on a comparable scale and never will be — normalising them
	// against each other would be inventing a relationship that doesn't exist.
	// Larger K flattens the difference between top positions.
	RRFK float64

	// ClosedPenalty multiplies a tender that can no longer be bid on
	// (awarded/cancelled/closed). It demotes rather than hides: a closed tender
	// is legitimate research material, it just isn't what a bidder searching
	// for work wants at the top.
	ClosedPenalty float64
	// ExpiredPenalty multiplies a tender whose deadline has already passed.
	ExpiredPenalty float64
	// UrgentPenalty multiplies a tender whose deadline is closer than
	// UrgentWithinDays — technically open, realistically too late to prepare a
	// serious bid.
	UrgentPenalty    float64
	UrgentWithinDays int
	// FreshBoost multiplies a tender published within FreshWithinDays.
	FreshBoost      float64
	FreshWithinDays int
}

// DefaultRanking is the starting configuration, shared by main.go and the
// tests so the numbers live in exactly one place.
func DefaultRanking() Ranking {
	return Ranking{
		LexicalWeight:    0.5,
		DenseWeight:      0.5,
		RRFK:             60, // the value the RRF paper uses and everyone since
		ClosedPenalty:    0.5,
		ExpiredPenalty:   0.35,
		UrgentPenalty:    0.9,
		UrgentWithinDays: 5,
		FreshBoost:       1.05,
		FreshWithinDays:  30,
	}
}

// withDefaults fills in any unset knob, so a Config built without a Ranking
// (every existing caller and test) gets sane behaviour rather than a
// zero-weight ranking that scores everything 0.
func (r Ranking) withDefaults() Ranking {
	d := DefaultRanking()
	if r.LexicalWeight == 0 && r.DenseWeight == 0 {
		r.LexicalWeight, r.DenseWeight = d.LexicalWeight, d.DenseWeight
	}
	if r.RRFK <= 0 {
		r.RRFK = d.RRFK
	}
	if r.ClosedPenalty <= 0 {
		r.ClosedPenalty = d.ClosedPenalty
	}
	if r.ExpiredPenalty <= 0 {
		r.ExpiredPenalty = d.ExpiredPenalty
	}
	if r.UrgentPenalty <= 0 {
		r.UrgentPenalty = d.UrgentPenalty
	}
	if r.UrgentWithinDays <= 0 {
		r.UrgentWithinDays = d.UrgentWithinDays
	}
	if r.FreshBoost <= 0 {
		r.FreshBoost = d.FreshBoost
	}
	if r.FreshWithinDays <= 0 {
		r.FreshWithinDays = d.FreshWithinDays
	}
	return r
}

// searchHybrid runs both retrievers over the same query and filters, fuses
// their rankings, applies the business boost, and returns the requested page.
//
// Both retrievers return tenders that have already passed the authoritative
// Postgres filter — the vector store's own pre-filter narrows the search but
// never decides what is allowed through.
func (s *Service) searchHybrid(ctx context.Context, query string, filters Filters, limit, offset int) (SearchOutput, error) {
	// Retrieve one past the end of the requested page so has_more can be
	// answered without a second COUNT query.
	want := offset + limit + 1
	if want > maxWindow {
		want = maxWindow
	}

	lexical, lexErr := s.repo.LexicalSearch(ctx, query, filters, want)
	dense, denseErr := s.denseCandidates(ctx, query, filters, want)

	mode := ModeHybrid
	switch {
	case lexErr != nil && denseErr != nil:
		// Both retrievers are down. Returning date-ordered rows here would be
		// the exact dishonesty this function exists to remove, so fail instead
		// and let the caller surface it.
		return SearchOutput{}, fmt.Errorf("tender: hybrid search: lexical: %w; semantic: %v", lexErr, denseErr)
	case denseErr != nil:
		mode = ModeLexical
	case lexErr != nil:
		mode = ModeSemantic
	}

	fused := s.fuse(lexical, dense, time.Now())

	// Either retriever coming back exactly full means it was truncated, so
	// more matches exist beyond this window even if the fused set doesn't
	// reach past the page.
	truncated := len(lexical) >= want || len(dense) >= want
	return page(fused, limit, offset, truncated, mode), nil
}

// denseCandidates runs the vector retriever and resolves its hits to tenders,
// ordered by similarity.
//
// The two-round shape exists because vector hits are chunks: a first request
// sized by fan-out usually yields enough distinct tenders, but a corpus whose
// tenders carry many document chunks can collapse a large chunk budget into
// very few tenders. When that happens, one wider retry is worth it; a third
// would not be, so the loop is deliberately bounded rather than adaptive.
func (s *Service) denseCandidates(ctx context.Context, query string, filters Filters, want int) ([]ScoredTender, error) {
	chunkLimit := want * denseChunkFanout
	if chunkLimit > maxDenseChunks {
		chunkLimit = maxDenseChunks
	}

	var (
		ids       []string
		bestScore map[string]float32
	)
	for round := 0; round < 2; round++ {
		hits, err := s.kb.SearchFiltered(ctx, query, chunkLimit, filters)
		if err != nil {
			return nil, fmt.Errorf("tender: semantic search: %w", err)
		}
		ids, bestScore = distinctByBestScore(hits)

		// Enough distinct tenders, or the store had nothing more to give.
		if len(ids) >= want || len(hits) < chunkLimit || chunkLimit >= maxDenseChunks {
			break
		}
		chunkLimit = maxDenseChunks
	}

	tenders, err := s.repo.EnrichTenders(ctx, ids, filters)
	if err != nil {
		return nil, fmt.Errorf("tender: enrich candidates: %w", err)
	}

	scored := make([]ScoredTender, len(tenders))
	for i, t := range tenders {
		scored[i] = ScoredTender{Tender: t, RelevanceScore: float64(bestScore[t.ID])}
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].RelevanceScore > scored[j].RelevanceScore })
	if len(scored) > want {
		scored = scored[:want]
	}
	return scored, nil
}

// distinctByBestScore collapses chunk hits to one entry per tender, keeping
// each tender's best-scoring chunk and preserving first-seen (i.e. best-first)
// order.
func distinctByBestScore(hits []ScoredChunk) ([]string, map[string]float32) {
	best := make(map[string]float32, len(hits))
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		if h.DocID == "" {
			continue
		}
		if existing, ok := best[h.DocID]; !ok {
			ids = append(ids, h.DocID)
			best[h.DocID] = h.Score
		} else if h.Score > existing {
			best[h.DocID] = h.Score
		}
	}
	return ids, best
}

// fuse merges two ranked lists with Reciprocal Rank Fusion, then applies the
// business boost, and returns the result ordered best-first.
//
// The fused RelevanceScore is normalised so that 1.0 means "top of both
// lists". It is NOT a cosine similarity any more — FitThresholds compares
// against this field (see recommend.go), so those thresholds are calibrated
// against this scale, not against raw embedding distance.
func (s *Service) fuse(lexical, dense []ScoredTender, now time.Time) []ScoredTender {
	r := s.cfg.Ranking.withDefaults()

	type entry struct {
		tender ScoredTender
		score  float64
	}
	merged := map[string]*entry{}

	contribute := func(list []ScoredTender, weight float64) {
		for i, t := range list {
			rrf := weight / (r.RRFK + float64(i+1))
			e, ok := merged[t.ID]
			if !ok {
				e = &entry{tender: t}
				merged[t.ID] = e
			}
			e.score += rrf
		}
	}
	contribute(lexical, r.LexicalWeight)
	contribute(dense, r.DenseWeight)

	// The best attainable fused score: rank 1 in both lists.
	maxScore := (r.LexicalWeight + r.DenseWeight) / (r.RRFK + 1)

	out := make([]ScoredTender, 0, len(merged))
	for _, e := range merged {
		t := e.tender
		normalised := e.score / maxScore
		t.RelevanceScore = normalised * businessBoost(t, r, now)
		out = append(out, t)
	}

	// Ties broken by deadline (soonest actionable first), then id, so the same
	// query always returns the same order — map iteration alone would not.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RelevanceScore != out[j].RelevanceScore {
			return out[i].RelevanceScore > out[j].RelevanceScore
		}
		if di, dj := out[i].Deadline, out[j].Deadline; di != nil && dj != nil && !di.Equal(*dj) {
			return di.Before(*dj)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// businessBoost is the multiplier that turns textual relevance into useful
// relevance. Text similarity alone ranks a tender awarded two years ago
// exactly like an open one closing in three weeks — for someone looking for
// work to bid on, that is the wrong answer however well the words match.
func businessBoost(t ScoredTender, r Ranking, now time.Time) float64 {
	boost := 1.0

	if t.Status != "" && t.Status != "open" && t.Status != "unknown" {
		boost *= r.ClosedPenalty
	}

	if t.Deadline != nil {
		days := t.Deadline.Sub(now).Hours() / 24
		switch {
		case days < 0:
			boost *= r.ExpiredPenalty
		case days < float64(r.UrgentWithinDays):
			boost *= r.UrgentPenalty
		}
	}

	if t.PublishedAt != nil && now.Sub(*t.PublishedAt).Hours()/24 <= float64(r.FreshWithinDays) {
		boost *= r.FreshBoost
	}

	return boost
}

// page slices one page out of a ranked list. truncated carries whether a
// retriever hit its own limit, so has_more stays honest when the fused set
// happens to end exactly at the page boundary.
func page(ranked []ScoredTender, limit, offset int, truncated bool, mode RetrievalMode) SearchOutput {
	if offset >= len(ranked) {
		return SearchOutput{Results: []ScoredTender{}, HasMore: false, Mode: mode}
	}
	end := offset + limit
	hasMore := len(ranked) > end || truncated
	if end > len(ranked) {
		end = len(ranked)
	}
	return SearchOutput{Results: ranked[offset:end], HasMore: hasMore, Mode: mode}
}
