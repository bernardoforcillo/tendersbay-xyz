// Package tender orchestrates the direct search API: combining Qdrant
// semantic search with structured Postgres filters, applying auth-tier
// result caps, and enforcing a Redis-backed rate limit. It has no
// knowledge of HTTP, ConnectRPC, or SQL — see
// internal/adapter/connectapi/tender_handler.go and
// internal/adapter/postgres/tender_repo.go for those. It also has no
// direct dependency on go-services/knowledge — KnowledgeBase's interface
// uses this package's own minimal ScoredChunk type, so main.go supplies a
// small adapter around the real *knowledge.KnowledgeBase (see Task 8).
package tender

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ── Sentinel errors ──
var (
	ErrRateLimited    = errors.New("tender: rate limit exceeded")
	ErrInvalidFilters = errors.New("tender: deadline_from is after deadline_to")
	// ErrRateLimiterUnavailable wraps a failure of the rate-limit CHECK
	// itself (e.g. Redis unreachable), as opposed to ErrRateLimited, which
	// means the check succeeded and reported "over limit". Distinguished so
	// callers can map it to a retryable status instead of a generic
	// internal error.
	ErrRateLimiterUnavailable = errors.New("tender: rate limiter unavailable")
)

// candidateMultiplier over-fetches Qdrant candidates so a restrictive
// filter doesn't starve the result page below the requested limit.
// maxCandidates bounds worst-case Qdrant/Postgres load regardless of the
// requested limit.
const (
	candidateMultiplier = 5
	maxCandidates       = 250
)

// Tender is a search result's structured fields.
type Tender struct {
	ID            string
	Title         string
	BuyerName     string
	Status        string
	ProcedureType string
	Country       string
	CPV           string
	Value         *int64
	Currency      string
	PublishedAt   *time.Time
	Deadline      *time.Time
	Source        string
	SourceRef     string
	NUTS          string
	SourceURL     string // the notice document's URL; "" if none is ingested
}

// Filters narrows a search. Zero-value fields are unset.
type Filters struct {
	Country      string
	CPV          string
	Status       string
	DeadlineFrom *time.Time
	DeadlineTo   *time.Time
}

// ScoredTender is a Tender plus its semantic-search relevance score (0
// for filters-only results, where relevance doesn't apply).
type ScoredTender struct {
	Tender
	RelevanceScore float64
}

// Repo is the subset of postgres.TenderRepo the service needs.
type Repo interface {
	SearchTenders(ctx context.Context, filters Filters, limit, offset int) ([]Tender, error)
	EnrichTenders(ctx context.Context, ids []string, filters Filters) ([]Tender, error)
}

// ScoredChunk is the minimal shape Search needs from a semantic search
// hit: which tender it belongs to, and how well it matched.
type ScoredChunk struct {
	DocID string
	Score float32
}

// KnowledgeBase is the subset of knowledge.KnowledgeBase the service
// needs, expressed in this package's own ScoredChunk type rather than
// go-services/knowledge's — see the package doc comment.
type KnowledgeBase interface {
	SearchWithScores(ctx context.Context, query string, limit int) ([]ScoredChunk, error)
}

// RateLimiter is the subset of redis.RateLimiter the service needs.
type RateLimiter interface {
	Allow(ctx context.Context, key string, limit int64, window time.Duration) (bool, error)
}

// Tier bounds one auth class's search behavior.
type Tier struct {
	MaxResults int
	RateLimit  int64
	RateWindow time.Duration
}

// Config holds the two tiers' limits.
type Config struct {
	AnonTier   Tier
	AuthedTier Tier
}

// Service runs tender searches.
type Service struct {
	repo Repo
	kb   KnowledgeBase
	rl   RateLimiter
	cfg  Config
}

// NewService returns a Service.
func NewService(repo Repo, kb KnowledgeBase, rl RateLimiter, cfg Config) *Service {
	return &Service{repo: repo, kb: kb, rl: rl, cfg: cfg}
}

// SearchParams is Search's input. RateLimitKey is the client IP for
// anonymous callers or the user ID for authenticated ones — the caller
// (the ConnectRPC handler) decides which, Search just uses whatever key
// it's given.
type SearchParams struct {
	Query         string
	Filters       Filters
	Limit         int
	Offset        int
	Authenticated bool
	RateLimitKey  string
}

// SearchOutput is Search's result.
type SearchOutput struct {
	Results []ScoredTender
	HasMore bool
}

// Search runs one tender search: rate-limits, clamps the result count to
// the caller's auth tier, then either a structured filter query
// (no Query text) or a semantic search enriched with structured filters
// (Query present). A Qdrant/Ollama failure during the semantic path
// degrades to the filters-only path rather than failing the request.
func (s *Service) Search(ctx context.Context, p SearchParams) (SearchOutput, error) {
	if p.Filters.DeadlineFrom != nil && p.Filters.DeadlineTo != nil && p.Filters.DeadlineFrom.After(*p.Filters.DeadlineTo) {
		return SearchOutput{}, ErrInvalidFilters
	}

	tier := s.cfg.AnonTier
	if p.Authenticated {
		tier = s.cfg.AuthedTier
	}

	allowed, err := s.rl.Allow(ctx, p.RateLimitKey, tier.RateLimit, tier.RateWindow)
	if err != nil {
		return SearchOutput{}, fmt.Errorf("%w: %v", ErrRateLimiterUnavailable, err)
	}
	if !allowed {
		return SearchOutput{}, ErrRateLimited
	}

	limit := p.Limit
	if limit <= 0 || limit > tier.MaxResults {
		limit = tier.MaxResults
	}

	offset := p.Offset
	if offset < 0 {
		offset = 0
	}

	if p.Query == "" {
		return s.searchByFiltersOnly(ctx, p.Filters, limit, offset)
	}

	out, err := s.searchSemantic(ctx, p.Query, p.Filters, limit, offset)
	if err != nil {
		return s.searchByFiltersOnly(ctx, p.Filters, limit, offset)
	}
	return out, nil
}

func (s *Service) searchByFiltersOnly(ctx context.Context, filters Filters, limit, offset int) (SearchOutput, error) {
	tenders, err := s.repo.SearchTenders(ctx, filters, limit+1, offset)
	if err != nil {
		return SearchOutput{}, fmt.Errorf("tender: search by filters: %w", err)
	}
	hasMore := len(tenders) > limit
	if hasMore {
		tenders = tenders[:limit]
	}
	results := make([]ScoredTender, len(tenders))
	for i, t := range tenders {
		results[i] = ScoredTender{Tender: t}
	}
	return SearchOutput{Results: results, HasMore: hasMore}, nil
}

func (s *Service) searchSemantic(ctx context.Context, query string, filters Filters, limit, offset int) (SearchOutput, error) {
	candidateLimit := limit * candidateMultiplier
	if candidateLimit > maxCandidates {
		candidateLimit = maxCandidates
	}

	hits, err := s.kb.SearchWithScores(ctx, query, candidateLimit)
	if err != nil {
		return SearchOutput{}, fmt.Errorf("tender: semantic search: %w", err)
	}

	bestScore := map[string]float32{}
	var ids []string
	for _, h := range hits {
		if existing, ok := bestScore[h.DocID]; !ok || h.Score > existing {
			if !ok {
				ids = append(ids, h.DocID)
			}
			bestScore[h.DocID] = h.Score
		}
	}

	tenders, err := s.repo.EnrichTenders(ctx, ids, filters)
	if err != nil {
		return SearchOutput{}, fmt.Errorf("tender: enrich candidates: %w", err)
	}

	scored := make([]ScoredTender, len(tenders))
	for i, t := range tenders {
		scored[i] = ScoredTender{Tender: t, RelevanceScore: float64(bestScore[t.ID])}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].RelevanceScore > scored[j].RelevanceScore })

	if offset >= len(scored) {
		return SearchOutput{Results: []ScoredTender{}, HasMore: false}, nil
	}
	end := offset + limit
	hasMore := len(scored) > end
	if end > len(scored) {
		end = len(scored)
	}
	return SearchOutput{Results: scored[offset:end], HasMore: hasMore}, nil
}
