package tender_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

type fakeProfiles struct{}

func (fakeProfiles) Get(context.Context, string, string) (clientprofile.Profile, error) {
	return clientprofile.Profile{}, nil
}

type fakeRepo struct {
	byFilters    []tender.Tender
	byFiltersErr error
	byIDs        map[string]tender.Tender // keyed by ID
	byIDsErr     error
	gotLimit     int
	detail       *tender.TenderDetail
	detailErr    error
	refs         []tender.TenderRef
	lexical      []tender.ScoredTender
	lexicalErr   error
	gotLexLimit  int
	gotFilters   tender.Filters
	gotSort      tender.SortOrder
	facets       tender.Facets
}

func (f *fakeRepo) LexicalSearch(_ context.Context, _ tender.LexicalQuery, filters tender.Filters, limit int) ([]tender.ScoredTender, error) {
	f.gotLexLimit = limit
	f.gotFilters = filters
	if f.lexicalErr != nil {
		return nil, f.lexicalErr
	}
	if len(f.lexical) > limit {
		return f.lexical[:limit], nil
	}
	return f.lexical, nil
}

func (f *fakeRepo) FacetCounts(context.Context, tender.Filters) (tender.Facets, error) {
	return f.facets, nil
}

func (f *fakeRepo) SearchTenders(_ context.Context, _ tender.Filters, sortBy tender.SortOrder, limit, offset int) ([]tender.Tender, error) {
	f.gotSort = sortBy
	f.gotLimit = limit
	if f.byFiltersErr != nil {
		return nil, f.byFiltersErr
	}
	end := offset + limit
	if end > len(f.byFilters) {
		end = len(f.byFilters)
	}
	if offset >= len(f.byFilters) {
		return nil, nil
	}
	return f.byFilters[offset:end], nil
}

func (f *fakeRepo) EnrichTenders(_ context.Context, ids []string, _ tender.Filters) ([]tender.Tender, error) {
	if f.byIDsErr != nil {
		return nil, f.byIDsErr
	}
	var out []tender.Tender
	for _, id := range ids {
		if t, ok := f.byIDs[id]; ok {
			out = append(out, t)
		}
	}
	return out, nil
}

func (f *fakeRepo) FindDetailByID(_ context.Context, _ int64) (*tender.TenderDetail, error) {
	return f.detail, f.detailErr
}
func (f *fakeRepo) DocumentsByTenderID(context.Context, int64) ([]tender.Document, error) {
	return nil, nil
}
func (f *fakeRepo) LotsByTenderID(context.Context, int64) ([]tender.Lot, error) {
	return nil, nil
}
func (f *fakeRepo) RecentTenderRefs(context.Context, int) ([]tender.TenderRef, error) {
	return f.refs, nil
}
func (f *fakeRepo) DistinctCountries(context.Context) ([]string, error) {
	return nil, nil
}

type fakeKnowledgeBase struct {
	results    []tender.ScoredChunk
	err        error
	gotLimit   int
	related    []tender.ScoredChunk
	relatedErr error
	gotFilters tender.Filters
	gotVector  []float32
	gotQuery   string
	embedCalls int
	embedErr   error
}

func (f *fakeKnowledgeBase) EmbedQuery(_ context.Context, query string) ([]float32, error) {
	f.embedCalls++
	f.gotQuery = query
	if f.embedErr != nil {
		return nil, f.embedErr
	}
	return []float32{0.1, 0.2, 0.3}, nil
}

func (f *fakeKnowledgeBase) SearchByVector(_ context.Context, vec []float32, limit int, filters tender.Filters) ([]tender.ScoredChunk, error) {
	f.gotVector = vec
	f.gotLimit = limit
	f.gotFilters = filters
	if f.err != nil {
		return nil, f.err
	}
	if len(f.results) > limit {
		return f.results[:limit], nil
	}
	return f.results, nil
}

func (f *fakeKnowledgeBase) RelatedByDocID(_ context.Context, _ string, _ int) ([]tender.ScoredChunk, error) {
	return f.related, f.relatedErr
}

type fakeRateLimiter struct {
	allow bool
	err   error
}

func (f *fakeRateLimiter) Allow(_ context.Context, _ string, _ int64, _ time.Duration) (bool, error) {
	return f.allow, f.err
}

func testConfig() tender.Config {
	return tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
	}
}

func TestSearch_FiltersOnlyWhenQueryEmpty(t *testing.T) {
	repo := &fakeRepo{byFilters: []tender.Tender{{ID: "1", Title: "A"}, {ID: "2", Title: "B"}}}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	out, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "", Limit: 10, RateLimitKey: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Results) != 2 {
		t.Fatalf("len(out.Results) = %d, want 2", len(out.Results))
	}
	if out.Results[0].RelevanceScore != 0 {
		t.Errorf("filters-only result RelevanceScore = %v, want 0", out.Results[0].RelevanceScore)
	}
}

func TestSearch_SemanticMergesScoresAndSortsDescending(t *testing.T) {
	repo := &fakeRepo{byIDs: map[string]tender.Tender{
		"1": {ID: "1", Title: "Low match"},
		"2": {ID: "2", Title: "High match"},
	}}
	kb := &fakeKnowledgeBase{results: []tender.ScoredChunk{
		{DocID: "1", Score: 0.4},
		{DocID: "2", Score: 0.9},
	}}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	out, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "lavori stradali", Limit: 10, RateLimitKey: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Results) != 2 {
		t.Fatalf("len(out.Results) = %d, want 2", len(out.Results))
	}
	if out.Results[0].ID != "2" || out.Results[1].ID != "1" {
		t.Errorf("results = [%s, %s], want [2, 1] (sorted by score descending)", out.Results[0].ID, out.Results[1].ID)
	}
}

// A tender is one result however many of its chunks matched — otherwise a
// tender with a long attached PDF would crowd the page with copies of itself
// and consume the candidate budget several times over.
func TestSearch_DedupesChunksOfTheSameTender(t *testing.T) {
	repo := &fakeRepo{byIDs: map[string]tender.Tender{
		"1": {ID: "1", Title: "T1"},
		"2": {ID: "2", Title: "T2"},
	}}
	kb := &fakeKnowledgeBase{results: []tender.ScoredChunk{
		{DocID: "1", Score: 0.3},
		{DocID: "2", Score: 0.5},
		{DocID: "1", Score: 0.95}, // same tender, different chunk, higher score
	}}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	out, err := svc.Search(context.Background(), tender.SearchParams{Query: "q", Limit: 10, RateLimitKey: "k"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Results) != 2 {
		t.Fatalf("len(out.Results) = %d, want 2 (deduped by tender id)", len(out.Results))
	}
	// Tender 1's best chunk (0.95) beats tender 2's only chunk (0.5), so the
	// dedupe must keep the best score, not the first-seen one.
	if out.Results[0].ID != "1" {
		t.Errorf("results = [%s, %s], want tender 1 first (its best chunk outscores tender 2)",
			out.Results[0].ID, out.Results[1].ID)
	}
}

// A dead vector store must not turn a query into a date-ordered browse: the
// lexical retriever still answers the question that was actually asked, and
// the mode says the search ran degraded.
func TestSearch_DegradesToLexicalWhenKnowledgeBaseErrors(t *testing.T) {
	repo := &fakeRepo{
		byFilters: []tender.Tender{{ID: "99", Title: "Merely the most recent tender"}},
		lexical:   []tender.ScoredTender{{Tender: tender.Tender{ID: "1", Title: "Actual keyword match"}}},
	}
	kb := &fakeKnowledgeBase{err: errors.New("qdrant unreachable")}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	out, err := svc.Search(context.Background(), tender.SearchParams{Query: "q", Limit: 10, RateLimitKey: "k"})
	if err != nil {
		t.Fatalf("Search: want nil error (lexical retrieval still works), got %v", err)
	}
	if len(out.Results) != 1 || out.Results[0].ID != "1" {
		t.Errorf("out.Results = %+v, want the lexical match, never the filters-only browse", out.Results)
	}
	if out.Mode != tender.ModeLexical {
		t.Errorf("out.Mode = %q, want %q", out.Mode, tender.ModeLexical)
	}
	if !out.Mode.Degraded() {
		t.Error("out.Mode.Degraded() = false, want true so callers can surface the degradation")
	}
}

// The symmetric case: a lexical failure leaves the semantic retriever answering.
func TestSearch_DegradesToSemanticWhenLexicalErrors(t *testing.T) {
	repo := &fakeRepo{
		byIDs:      map[string]tender.Tender{"1": {ID: "1", Title: "Semantic match"}},
		lexicalErr: errors.New("full-text index unavailable"),
	}
	kb := &fakeKnowledgeBase{results: []tender.ScoredChunk{{DocID: "1", Score: 0.8}}}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	out, err := svc.Search(context.Background(), tender.SearchParams{Query: "q", Limit: 10, RateLimitKey: "k"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Results) != 1 || out.Results[0].ID != "1" {
		t.Errorf("out.Results = %+v, want the semantic match", out.Results)
	}
	if out.Mode != tender.ModeSemantic {
		t.Errorf("out.Mode = %q, want %q", out.Mode, tender.ModeSemantic)
	}
}

// With neither retriever available the request must FAIL. Answering it with
// the most recently published tenders would present unrelated rows as search
// results, which is worse than a visible error.
func TestSearch_FailsWhenBothRetrieversError(t *testing.T) {
	repo := &fakeRepo{
		byFilters:  []tender.Tender{{ID: "99", Title: "Merely the most recent tender"}},
		lexicalErr: errors.New("full-text index unavailable"),
	}
	kb := &fakeKnowledgeBase{err: errors.New("qdrant unreachable")}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	_, err := svc.Search(context.Background(), tender.SearchParams{Query: "q", Limit: 10, RateLimitKey: "k"})
	if err == nil {
		t.Fatal("Search: want an error when no retriever can answer the query, got nil")
	}
}

// An empty query is a browse, not a failed search — it legitimately orders by
// publication date and says so.
func TestSearch_EmptyQueryBrowsesByFilters(t *testing.T) {
	repo := &fakeRepo{byFilters: []tender.Tender{{ID: "1", Title: "Most recent"}}}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, &fakeKnowledgeBase{}, rl, &fakeProfiles{}, testConfig())

	out, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "   ", Limit: 10, RateLimitKey: "k",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if out.Mode != tender.ModeFilters {
		t.Errorf("out.Mode = %q, want %q", out.Mode, tender.ModeFilters)
	}
	if len(out.Results) != 1 {
		t.Fatalf("len(out.Results) = %d, want 1", len(out.Results))
	}
	if out.Results[0].RelevanceScore != 0 {
		t.Errorf("RelevanceScore = %v, want 0 — with no query, relevance is undefined",
			out.Results[0].RelevanceScore)
	}
}

func TestSearch_RejectsRequestOverRateLimit(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: false}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	_, err := svc.Search(context.Background(), tender.SearchParams{RateLimitKey: "k"})
	if !errors.Is(err, tender.ErrRateLimited) {
		t.Errorf("Search error = %v, want ErrRateLimited", err)
	}
}

func TestSearch_ClampsLimitToAuthTier(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	// Anonymous (Authenticated: false) requests 100; anon tier max is 10, so
	// SearchTenders should be called with limit+1 = 11 (10 clamped + 1 for has_more).
	_, err := svc.Search(context.Background(), tender.SearchParams{Limit: 100, RateLimitKey: "k", Authenticated: false})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if repo.gotLimit != 11 {
		t.Errorf("SearchTenders called with limit=%d, want 11 (10 clamped + 1)", repo.gotLimit)
	}
}

func TestSearch_ClampsNegativeOffsetToZero(t *testing.T) {
	repo := &fakeRepo{byIDs: map[string]tender.Tender{"1": {ID: "1", Title: "T"}}}
	kb := &fakeKnowledgeBase{results: []tender.ScoredChunk{{DocID: "1", Score: 0.5}}}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	// Must not panic, and must behave as if offset were 0.
	out, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "q", Limit: 10, Offset: -5, RateLimitKey: "k",
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Results) != 1 || out.Results[0].ID != "1" {
		t.Errorf("out.Results = %+v, want the single result (negative offset treated as 0)", out.Results)
	}
}

// The vector store is asked for CHUNKS, but the budget is expressed in
// TENDERS: one tender with a long attached PDF holds dozens of chunks, so a
// budget counted in chunks silently collapses to a handful of distinct
// tenders. The fan-out multiplier is what keeps the two straight.
func TestSearch_RequestsChunkBudgetScaledFromTheTenderWindow(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	// Authenticated tier max is 50, so Limit 20 stands. The window is
	// offset+limit+1 = 21 tenders, and the chunk request is that times the
	// fan-out.
	_, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "q", Limit: 20, RateLimitKey: "k", Authenticated: true,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if want := 21 * 8; kb.gotLimit != want {
		t.Errorf("SearchFiltered called with limit=%d, want %d (21 tenders x fan-out 8)", kb.gotLimit, want)
	}
	// The lexical side is counted in tenders directly — no fan-out.
	if repo.gotLexLimit != 21 {
		t.Errorf("LexicalSearch called with limit=%d, want 21", repo.gotLexLimit)
	}
}

// Deep paging must not turn into an unbounded retrieval: the window is capped,
// and past it the page is honestly empty rather than ever more expensive.
func TestSearch_CapsTheRetrievalWindow(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	_, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "q", Limit: 50, Offset: 5000, RateLimitKey: "k", Authenticated: true,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if repo.gotLexLimit != 300 {
		t.Errorf("LexicalSearch called with limit=%d, want the 300-tender window cap", repo.gotLexLimit)
	}
	if kb.gotLimit != 600 {
		t.Errorf("SearchFiltered called with limit=%d, want the 600-chunk cap", kb.gotLimit)
	}
}

// The filters the caller asked for must reach the vector store, not just the
// SQL side — that is the whole point of pre-filtering.
func TestSearch_PushesFiltersIntoBothRetrievers(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	filters := tender.Filters{Countries: []string{"IT"}, CPVPrefixes: []string{"45"}}
	if _, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "q", Filters: filters, Limit: 10, RateLimitKey: "k",
	}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(kb.gotFilters.Countries) != 1 || kb.gotFilters.Countries[0] != "IT" {
		t.Errorf("vector filters = %+v, want the country pushed down", kb.gotFilters)
	}
	if len(repo.gotFilters.CPVPrefixes) != 1 || repo.gotFilters.CPVPrefixes[0] != "45" {
		t.Errorf("lexical filters = %+v, want the CPV prefix applied", repo.gotFilters)
	}
}

func TestSearch_WrapsRateLimiterUnavailableWhenAllowErrors(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{err: errors.New("redis: connection refused")}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	_, err := svc.Search(context.Background(), tender.SearchParams{RateLimitKey: "k"})
	if !errors.Is(err, tender.ErrRateLimiterUnavailable) {
		t.Errorf("Search error = %v, want ErrRateLimiterUnavailable", err)
	}
}

func TestSearch_RejectsInvalidDeadlineRange(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, &fakeProfiles{}, testConfig())

	from := time.Now()
	to := from.Add(-time.Hour) // before from — invalid
	_, err := svc.Search(context.Background(), tender.SearchParams{
		RateLimitKey: "k",
		Filters:      tender.Filters{DeadlineFrom: &from, DeadlineTo: &to},
	})
	if !errors.Is(err, tender.ErrInvalidFilters) {
		t.Errorf("Search error = %v, want ErrInvalidFilters", err)
	}
}
