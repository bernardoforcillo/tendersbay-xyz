package tender_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

type fakeRepo struct {
	byFilters    []tender.Tender
	byFiltersErr error
	byIDs        map[string]tender.Tender // keyed by ID
	byIDsErr     error
	gotLimit     int
}

func (f *fakeRepo) SearchTenders(_ context.Context, _ tender.Filters, limit, offset int) ([]tender.Tender, error) {
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

type fakeKnowledgeBase struct {
	results  []tender.ScoredChunk
	err      error
	gotLimit int
}

func (f *fakeKnowledgeBase) SearchWithScores(_ context.Context, _ string, limit int) ([]tender.ScoredChunk, error) {
	f.gotLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
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
	svc := tender.NewService(repo, kb, rl, testConfig())

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
	svc := tender.NewService(repo, kb, rl, testConfig())

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

func TestSearch_KeepsBestScorePerTenderWhenMultipleChunksMatch(t *testing.T) {
	repo := &fakeRepo{byIDs: map[string]tender.Tender{"1": {ID: "1", Title: "T"}}}
	kb := &fakeKnowledgeBase{results: []tender.ScoredChunk{
		{DocID: "1", Score: 0.3},
		{DocID: "1", Score: 0.95}, // same tender, different chunk, higher score
	}}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, testConfig())

	out, err := svc.Search(context.Background(), tender.SearchParams{Query: "q", Limit: 10, RateLimitKey: "k"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(out.Results) != 1 {
		t.Fatalf("len(out.Results) = %d, want 1 (deduped by tender id)", len(out.Results))
	}
	if out.Results[0].RelevanceScore != float64(float32(0.95)) {
		t.Errorf("RelevanceScore = %v, want the higher of the two chunk scores (0.95)", out.Results[0].RelevanceScore)
	}
}

func TestSearch_FallsBackToFiltersOnlyWhenKnowledgeBaseErrors(t *testing.T) {
	repo := &fakeRepo{byFilters: []tender.Tender{{ID: "1", Title: "Fallback result"}}}
	kb := &fakeKnowledgeBase{err: errors.New("qdrant unreachable")}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, testConfig())

	out, err := svc.Search(context.Background(), tender.SearchParams{Query: "q", Limit: 10, RateLimitKey: "k"})
	if err != nil {
		t.Fatalf("Search: want nil error (should degrade to filters-only), got %v", err)
	}
	if len(out.Results) != 1 || out.Results[0].ID != "1" {
		t.Errorf("out.Results = %+v, want the filters-only fallback result", out.Results)
	}
}

func TestSearch_RejectsRequestOverRateLimit(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: false}
	svc := tender.NewService(repo, kb, rl, testConfig())

	_, err := svc.Search(context.Background(), tender.SearchParams{RateLimitKey: "k"})
	if !errors.Is(err, tender.ErrRateLimited) {
		t.Errorf("Search error = %v, want ErrRateLimited", err)
	}
}

func TestSearch_ClampsLimitToAuthTier(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, testConfig())

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
	svc := tender.NewService(repo, kb, rl, testConfig())

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

func TestSearch_OverFetchesCandidatesByFiveX(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, testConfig())

	// Authenticated tier max is 50; Limit: 20 stays under that, so effective
	// limit is 20, and candidateLimit should be 20*5 = 100.
	_, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "q", Limit: 20, RateLimitKey: "k", Authenticated: true,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if kb.gotLimit != 100 {
		t.Errorf("SearchWithScores called with limit=%d, want 100 (20*5)", kb.gotLimit)
	}
}

func TestSearch_CapsCandidatesAt250(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, testConfig())

	// Authenticated tier max is 50; even at the max, 50*5 = 250 exactly hits
	// the cap. Confirm it's capped, not left uncapped past 250.
	_, err := svc.Search(context.Background(), tender.SearchParams{
		Query: "q", Limit: 50, RateLimitKey: "k", Authenticated: true,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if kb.gotLimit != 250 {
		t.Errorf("SearchWithScores called with limit=%d, want 250 (capped)", kb.gotLimit)
	}
}

func TestSearch_RejectsInvalidDeadlineRange(t *testing.T) {
	repo := &fakeRepo{}
	kb := &fakeKnowledgeBase{}
	rl := &fakeRateLimiter{allow: true}
	svc := tender.NewService(repo, kb, rl, testConfig())

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
