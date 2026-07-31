package tender

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
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

// SortOrder names how results should be ordered.
type SortOrder string

const (
	// SortRelevance is the default: the fused ranking. It is undefined without
	// a query, where it falls back to SortPublished rather than pretending
	// every result is equally relevant.
	SortRelevance SortOrder = "relevance"
	// SortDeadline puts the soonest still-open deadline first. Tenders whose
	// deadline has passed, or that have none, sort last — ordering by "soonest"
	// would otherwise surface the most thoroughly expired notice first.
	SortDeadline SortOrder = "deadline"
	// SortPublished is newest first.
	SortPublished SortOrder = "published"
	// SortValue is largest first; tenders with no known value sort last.
	SortValue SortOrder = "value"
)

// ParseSortOrder maps a wire value onto a SortOrder, falling back to
// SortRelevance for anything unrecognised — an unknown sort is a client bug,
// and failing the whole search over it would be a worse answer than the
// default ordering.
func ParseSortOrder(s string) SortOrder {
	switch SortOrder(strings.ToLower(strings.TrimSpace(s))) {
	case SortDeadline:
		return SortDeadline
	case SortPublished:
		return SortPublished
	case SortValue:
		return SortValue
	default:
		return SortRelevance
	}
}

// applySort reorders a ranked list. SortRelevance leaves it alone: it is
// already in fused order, and re-sorting on the score would discard the
// deterministic tie-breaks fuse applied.
func applySort(results []ScoredTender, order SortOrder, now time.Time) {
	switch order {
	case SortDeadline:
		sort.SliceStable(results, func(i, j int) bool {
			return deadlineSortKey(results[i], now) < deadlineSortKey(results[j], now)
		})
	case SortPublished:
		sort.SliceStable(results, func(i, j int) bool {
			return publishedSortKey(results[i]) > publishedSortKey(results[j])
		})
	case SortValue:
		sort.SliceStable(results, func(i, j int) bool {
			return valueSortKey(results[i]) > valueSortKey(results[j])
		})
	}
}

// deadlineSortKey ranks by how soon a tender closes, pushing expired and
// deadline-less tenders to the end. Sorting "soonest first" naively would put
// the longest-expired notice at the very top, which is the opposite of useful.
func deadlineSortKey(t ScoredTender, now time.Time) int64 {
	if t.Deadline == nil || t.Deadline.Before(now) {
		return math.MaxInt64
	}
	return t.Deadline.Unix()
}

func publishedSortKey(t ScoredTender) int64 {
	if t.PublishedAt == nil {
		return math.MinInt64
	}
	return t.PublishedAt.Unix()
}

func valueSortKey(t ScoredTender) int64 {
	if t.Value == nil {
		return math.MinInt64
	}
	return *t.Value
}

// FacetCount is one facet value and how many results carry it.
type FacetCount struct {
	Value string
	Count int
}

// Facets are per-field result counts, for filter-bar badges.
//
// What they count depends on the search, and the difference is reported rather
// than smoothed over: for a query-driven search they describe the ranked
// window this request considered, since that is the set that was actually
// scored; for a filters-only browse they are exact counts over the whole
// filtered corpus, which the database can aggregate directly. Presenting a
// window count as a corpus count would be a number that looks authoritative
// and isn't.
type Facets struct {
	Countries    []FacetCount
	Statuses     []FacetCount
	CPVDivisions []FacetCount // keyed by the 2-digit CPV division
}

// facetsOf counts a result window by country, status and CPV division.
func facetsOf(results []ScoredTender) Facets {
	countries, statuses, divisions := map[string]int{}, map[string]int{}, map[string]int{}
	for _, r := range results {
		if r.Country != "" {
			countries[r.Country]++
		}
		if r.Status != "" {
			statuses[r.Status]++
		}
		if len(r.CPV) >= 2 {
			divisions[r.CPV[:2]]++
		}
	}
	return BuildFacets(countries, statuses, divisions)
}

// BuildFacets assembles Facets from per-field count maps. Exported for the
// repository adapter, which aggregates the same three groupings in SQL for
// the browse path and needs to present them identically.
func BuildFacets(countries, statuses, cpvDivisions map[string]int) Facets {
	return Facets{
		Countries:    sortedFacets(countries),
		Statuses:     sortedFacets(statuses),
		CPVDivisions: sortedFacets(cpvDivisions),
	}
}

// sortedFacets orders counts descending, then by value, so the same result set
// always produces the same facet order.
func sortedFacets(counts map[string]int) []FacetCount {
	out := make([]FacetCount, 0, len(counts))
	for value, count := range counts {
		out = append(out, FacetCount{Value: value, Count: count})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Value < out[j].Value
	})
	return out
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

	// CPVWeight is the fusion weight of the CPV arm — tenders found because the
	// query resolved to a CPV code they carry (see cpv.go). It is what makes
	// cross-language retrieval work: a code is identical in all 24 EU languages.
	//
	// It is deliberately LOWER than the two text retrievers. A category label
	// says what a notice is about; a title match says it is about exactly this.
	//
	// Zero means the arm is off, and unlike every other knob in this struct that
	// is a MEANINGFUL value rather than "unset" — see withDefaults.
	CPVWeight float64
	// CPVIndexExpanded widens the lexical match to each tender's cpv_labels
	// weight class, so a query in one language can match a notice in another
	// through the label text denormalised onto it.
	//
	// This expresses the SAME signal as CPVWeight by a different route, so
	// running both counts it twice: the labels inflate the lexical arm's rank
	// AND the concept re-enters as its own arm, leaving the effective CPV weight
	// unknown. That is a real tuning hazard, which is why both are independently
	// switchable and why the evaluation harness measures all four combinations
	// before either is left on.
	//
	// False means off, and like CPVWeight that is meaningful, not unset.
	CPVIndexExpanded bool

	// CPVBoost is a STRUCTURALLY DIFFERENT way of using the same CPV signal:
	// instead of retrieving extra candidates (CPVWeight's fourth ranked list),
	// it multiplies the score of a candidate lexical or dense ALREADY found,
	// when that candidate's own CPV matches a resolved code's category. See
	// cpvBoost's doc comment in hybrid.go for the full reasoning — in short,
	// RRF fuses on rank, and rank inside one CPV category carries no relevance
	// information, so injecting a rank-ordered category list into fusion (what
	// CPVWeight does) adds noise; multiplying an already-earned score cannot.
	//
	// Like businessBoost's other multipliers this can only RAISE a score, so it
	// is applied the same way — after fusion, inside businessBoost — and never
	// touches maxScore.
	//
	// CPVBoost <= 1 means off — a multiplier's "off" is 1 (no-op), not 0 (which
	// would ZERO every score it touched), so unlike a weight, the meaningful
	// "unset" boundary here is <= 1, not <= 0. Like CPVWeight, this is
	// deliberately NOT filled in by withDefaults: a bare Ranking{} must leave
	// it off, not silently turn it on at whatever value DefaultRanking uses.
	CPVBoost float64
}

// DefaultRanking is the starting configuration, shared by main.go and the
// tests so the numbers live in exactly one place.
func DefaultRanking() Ranking {
	return Ranking{
		LexicalWeight: 0.5,
		DenseWeight:   0.5,
		// CPVWeight 0 — the RETRIEVAL ARM (cpv.go, FindByCPVPrefixes) is wired
		// but OFF, permanently, and Task 15 is why "permanently" is the right
		// word rather than "still".
		//
		// Task 14 measured the arm with three prefix-truncation depths and every
		// one scored BELOW this baseline (recall@20 0.6315/ndcg 0.4946/mrr
		// 0.4862): best was ~0.593/0.460/0.439, and the four cross-language
		// cells the arm exists to fix (it→de, fr→de, nl→de, pl→de) stayed at
		// exactly 0.0000 in all three. Root cause, confirmed against the eval
		// DB: FindByCPVPrefixes shared the same small per-page candidate window
		// every other arm uses (~21 rows for a 20-result page) and ordered its
		// matches by recency: for "pulizie uffici" → class 9091, 56 tenders in
		// that class exist and the one judged-relevant German tender ranked
		// 31st by publish date, so it never entered the candidate window at all.
		//
		// Task 15 tried the two structurally different fixes that finding
		// pointed at, and measured BOTH against the live harness:
		//
		//   (A) Minimal fix, keep the arm: order FindByCPVPrefixes by the
		//       caller's own code rank instead of recency (cpvPrefixRanking in
		//       tender_search.go), and give the arm its OWN wide budget
		//       (cpvArmWindow) instead of sharing the ~21-row page window.
		//       Measured at CPVWeight 0.4 (unchanged from Task 14, to isolate
		//       these two fixes from a third confound): recall@20 0.5852 / ndcg
		//       0.4404 / mrr 0.4349 — WORSE than baseline on every metric, and
		//       the four target cells stayed at 0.0000. This is the deeper
		//       reason CPVWeight stays 0 even with both fixes applied: RRF
		//       fuses on RANK, and rank carries no relevance information INSIDE
		//       one CPV category — reordering the arm's OWN list only fixes
		//       ties ACROSS the query's resolved codes, not the ties WITHIN one
		//       code, which is most of what a 56-tender class actually is. A
		//       widened window just floods fusion with more of that
		//       arbitrarily-ordered noise. The code fix (cpvPrefixRanking,
		//       cpvArmWindow) is left in place, switched off by CPVWeight 0 —
		//       see cpvBoost's doc comment for why a fourth ranked list is the
		//       wrong shape for this signal, not just an under-tuned one.
		//
		//   (B) Structural fix, drop the arm: apply the same resolved-code
		//       signal as CPVBoost — a post-fusion multiplier on candidates
		//       lexical/dense ALREADY found (see cpvBoost) — instead of a
		//       fourth retrieval. Measured at 1.15 and 1.5 to confirm the
		//       metric responds to the knob at all (it does, modestly: ndcg
		//       0.5004 at 1.15 vs 0.4988 at 1.5, both above baseline, so 1.15
		//       was kept — this was a choice between the two brief-specified
		//       values, not further tuning). At 1.15: recall@20 0.6315 (tied
		//       with baseline exactly — a pure re-ranking signal moves WHICH
		//       page-20 window membership rarely changes, so this is expected,
		//       not suspicious), ndcg 0.5004 (+0.0058), mrr 0.4983 (+0.0121).
		//       Only 4 regressions beyond tolerance, all confined to the single
		//       es/es→es scope (~0.02-0.04 each) — a real, small trade-off,
		//       disclosed rather than hidden. The four target cells (it→de,
		//       fr→de, nl→de, pl→de) stayed at 0.0000 under CPVBoost too: a
		//       multiplier can only re-rank a candidate lexical or dense
		//       already retrieved, and for those four specific cells the
		//       correct tender was never retrieved by either to begin with — a
		//       genuinely different, unsolved problem (index-side label
		//       expansion, Tasks 15/16) that this result does not condemn.
		//
		// (B) beat the baseline; (A) did not. CPVBoost 1.15 ships ENABLED.
		// CPVWeight stays 0 so the retrieval arm — proven twice now to inject
		// rank noise no amount of ordering or window-widening fixes — never
		// runs. See task-14-report.md (Task 14 run) and task-14-report.md's
		// Task 15 section (this decision) for the full harness output.
		CPVWeight:        0,
		CPVIndexExpanded: true,
		// CPVBoost 1.15 — see the CPVWeight comment above for the full
		// comparison against the retrieval-arm alternative. Shipped ENABLED:
		// this is the one CPV-signal knob in this struct whose zero-ish value
		// (<=1) is the OFF state and whose shipped default is switched on.
		CPVBoost:         1.15,
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
//
// CPVWeight, CPVIndexExpanded and CPVBoost are deliberately NOT filled. For
// them, the zero value is a real setting — "the CPV arm is off", "index
// expansion is off", "the boost multiplier is off" — not an absent one, so
// default-filling would make them impossible to disable and would silently
// switch the signal on in every test that builds a bare Ranking{}.
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
func (s *Service) searchHybrid(ctx context.Context, query string, filters Filters, sortBy SortOrder, limit, offset int) (SearchOutput, error) {
	// Retrieve one past the end of the requested page so has_more can be
	// answered without a second COUNT query.
	want := offset + limit + 1
	if want > maxWindow {
		want = maxWindow
	}

	lexical, lexErr := s.repo.LexicalSearch(ctx, LexicalQuery{
		Text:            query,
		ExpandCPVLabels: s.cfg.Ranking.CPVIndexExpanded,
	}, filters, want)
	dense, denseErr := s.denseCandidates(ctx, query, filters, want)
	// No error return: every CPV failure degrades to an absent signal — see
	// cpvCandidates. cpvArm is only non-nil when the retrieval arm (CPVWeight)
	// is on; cpvMatches is used both for AppliedCPV below and, inside fuse, for
	// the CPVBoost multiplier.
	cpvArm, cpvMatches := s.cpvCandidates(ctx, query, filters)

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

	now := time.Now()
	fused := s.fuse(lexical, dense, cpvArm, cpvMatches, now)

	// Facets are counted over the whole ranked window, before paging — they
	// describe the result set, not the page the caller happens to be on.
	facets := facetsOf(fused)
	applySort(fused, sortBy, now)

	// Either retriever coming back exactly full means it was truncated, so
	// more matches exist beyond this window even if the fused set doesn't
	// reach past the page.
	truncated := len(lexical) >= want || len(dense) >= want
	out := page(fused, limit, offset, truncated, mode)
	out.Facets = facets
	out.AppliedCPV = cpvMatches
	return out, nil
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

	// Embed once, outside the retry loop: the second round differs only in how
	// many chunks it asks for, and re-embedding for it would double the cost
	// of the most expensive step for no benefit.
	vec, err := s.embedQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("tender: embed query: %w", err)
	}

	var (
		ids       []string
		bestScore map[string]float32
	)
	for round := 0; round < 2; round++ {
		hits, err := s.kb.SearchByVector(ctx, vec, chunkLimit, filters)
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

	tenders, enrichErr := s.repo.EnrichTenders(ctx, ids, filters)
	if enrichErr != nil {
		return nil, fmt.Errorf("tender: enrich candidates: %w", enrichErr)
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

// fuse merges the ranked lists with Reciprocal Rank Fusion, then applies the
// business boost, and returns the result ordered best-first.
//
// The fused RelevanceScore is normalised so that 1.0 means "top of both TEXT
// retrievers". It is NOT a cosine similarity any more — FitThresholds compares
// against this field (see recommend.go), so those thresholds are calibrated
// against this scale, not against raw embedding distance.
//
// The CPV arm is deliberately absent from that normalisation. Adding its weight
// to the denominator would push every result found by lexical+dense but not CPV
// down from 1.0 to 0.667 of max, silently reclassifying "strong" fits as
// "possible" — a regression in a feature this arm has nothing to do with. Left
// out, the arm can only RAISE a score, so every existing threshold keeps exactly
// the meaning it had.
//
// cpvMatches carries the resolved CPV codes regardless of whether cpv (the
// retrieval arm's OWN candidate list) is populated — see cpvBoost, which reads
// cpvMatches directly against lexical/dense candidates that were never part of
// any CPV retrieval at all.
func (s *Service) fuse(lexical, dense, cpv []ScoredTender, cpvMatches []CPVMatch, now time.Time) []ScoredTender {
	r := s.cfg.Ranking.withDefaults()

	type entry struct {
		tender ScoredTender
		score  float64
	}
	merged := map[string]*entry{}

	contribute := func(list []ScoredTender, weight float64) {
		// A zero (or negative) weight means this arm is off. Skipping it here —
		// rather than just contributing 0 — keeps an off arm from inserting its
		// tenders into merged at all: without this guard, a CPV-only result
		// would still appear in the output ranked last instead of being absent,
		// and CPVWeight: 0 would stop being byte-identical to no arm at all.
		if weight <= 0 {
			return
		}
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
	contribute(cpv, r.CPVWeight)

	// The best attainable score from the two TEXT retrievers: rank 1 in both.
	// The CPV arm is deliberately excluded — see the fuse doc comment.
	maxScore := (r.LexicalWeight + r.DenseWeight) / (r.RRFK + 1)

	out := make([]ScoredTender, 0, len(merged))
	for _, e := range merged {
		t := e.tender
		normalised := e.score / maxScore
		t.RelevanceScore = normalised * businessBoost(t, r, now) * cpvBoost(t, cpvMatches, r)
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

// cpvBoost multiplies in the CPV signal directly against a candidate lexical
// or dense ALREADY found, instead of injecting it as a fourth ranked list the
// way CPVWeight's retrieval arm does.
//
// Why not a fourth arm: RRF fuses on RANK, and rank carries no relevance
// information INSIDE one CPV category — one office-cleaning tender is no more
// "about cleaning" than the next one sharing its class, so a rank-ordered CPV
// candidate list injects arbitrary noise into fusion. That is the most likely
// explanation for why CPVWeight regressed cells that used to work (es→de,
// en→nl) while its own four target cells (it→de, fr→de, nl→de, pl→de) stayed
// at 0.0000 regardless of ordering or window size — see DefaultRanking's
// CPVWeight comment and task-14-report.md. A tender lexical or dense already
// surfaced needs no separate retrieval at all to also carry the CPV signal:
// its score can only go up, never in by a borrowed, meaningless rank
// position.
//
// Matches on t.CPV (the primary code) only. ScoredTender.Tender does NOT
// carry CPVSecondary — only TenderDetail does (see detail.go) — so a tender
// found solely through a secondary CPV code is not boosted here. Widening the
// search projection to also carry cpv_secondary is a real, scoped follow-up;
// this is a known, stated limitation, not an oversight.
//
// The category prefix (cpvHierarchyPrefix), not the raw leaf, is what's
// compared — matching on the raw leaf would recreate the exact bug Task 14
// fixed: an Italian "pulizie uffici" resolves to 90919200, but its German
// sibling tender carries 90911200, agreeing only on the shared class "9091".
// Leaf equality would never fire for the cross-language cases this exists to
// help.
func cpvBoost(t ScoredTender, matches []CPVMatch, r Ranking) float64 {
	if r.CPVBoost <= 1 || t.CPV == "" {
		return 1
	}
	for _, m := range matches {
		if strings.HasPrefix(t.CPV, cpvHierarchyPrefix(m.Code)) {
			return r.CPVBoost
		}
	}
	return 1
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
