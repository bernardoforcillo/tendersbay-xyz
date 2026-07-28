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
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
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

// Filters narrows a search. Zero-value fields are unset, and every list field
// is an OR within itself while separate fields AND together — "Italy or
// Germany, and construction".
//
// The code lists are PREFIX matches against a hierarchical code, not equality:
// CPV "45" selects all of division 45, NUTS "ITC" all of north-west Italy. A
// CPV prefix matches a tender's secondary CPVs as well as its primary one, so
// a tender only incidentally in a sector is still findable there — and so the
// SQL and vector paths agree, since the vector payload indexes both (see
// knowledge.Attributes).
type Filters struct {
	Countries    []string
	CPVPrefixes  []string
	Statuses     []string
	NUTSPrefixes []string
	// Buyer is a case-insensitive substring of the contracting authority's
	// name, not a prefix — buyers are written inconsistently across sources
	// ("Comune di Roma" / "Roma Capitale — Comune di Roma").
	Buyer        string
	ValueMin     *int64
	ValueMax     *int64
	DeadlineFrom *time.Time
	DeadlineTo   *time.Time
}

// SingleFilter wraps one optional facet value as a Filters list field,
// returning nil for the empty string so an unset value contributes no
// predicate. For callers whose input shape is one-value-per-facet — the
// agent's search tool, a single-select UI control.
func SingleFilter(v string) []string {
	if v == "" {
		return nil
	}
	return []string{v}
}

// IsZero reports whether the filters constrain nothing.
func (f Filters) IsZero() bool {
	return len(f.Countries) == 0 && len(f.CPVPrefixes) == 0 && len(f.Statuses) == 0 &&
		len(f.NUTSPrefixes) == 0 && f.Buyer == "" &&
		f.ValueMin == nil && f.ValueMax == nil &&
		f.DeadlineFrom == nil && f.DeadlineTo == nil
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
	// LexicalSearch is the keyword half of hybrid retrieval — full-text and
	// trigram matching, already filtered and ranked, best-first. It answers
	// the queries dense embeddings structurally can't: exact codes, notice
	// references, buyer names, rare acronyms.
	LexicalSearch(ctx context.Context, query string, filters Filters, limit int) ([]ScoredTender, error)
	EnrichTenders(ctx context.Context, ids []string, filters Filters) ([]Tender, error)
	FindDetailByID(ctx context.Context, id int64) (*TenderDetail, error)
	DocumentsByTenderID(ctx context.Context, id int64) ([]Document, error)
	LotsByTenderID(ctx context.Context, id int64) ([]Lot, error)
	RecentTenderRefs(ctx context.Context, limit int) ([]TenderRef, error)
	DistinctCountries(ctx context.Context) ([]string, error)
}

// ScoredChunk is the minimal shape Search needs from a semantic search
// hit: which tender it belongs to, and how well it matched.
type ScoredChunk struct {
	DocID string
	Score float32
}

// KnowledgeBase is the subset of knowledge.KnowledgeBase the service
// needs, expressed in this package's own ScoredChunk and Filters types
// rather than go-services/knowledge's — see the package doc comment.
type KnowledgeBase interface {
	// SearchFiltered narrows the vector search by the same facets the SQL
	// side filters on. Pushing the filter into the vector store is what keeps
	// a narrow filter (one country out of 27) from consuming the whole
	// candidate window with rows that will be discarded moments later. It is
	// an optimisation only: results are filtered authoritatively in Postgres
	// afterwards either way.
	SearchFiltered(ctx context.Context, query string, limit int, filters Filters) ([]ScoredChunk, error)
	RelatedByDocID(ctx context.Context, docID string, limit int) ([]ScoredChunk, error)
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

// FitThresholds tunes RecommendForClient's tier classification (see
// recommend.go). These are an initial, uncalibrated guess — there is no
// conversion data pre-launch (see the design spec's Risks section) — safe to
// retune here without touching the classification logic itself.
type FitThresholds struct {
	RelevanceHigh      float64 // relevance_score at/above which a result can be "strong"
	RelevanceLow       float64 // relevance_score below which a result is always "long_shot"
	MinDeadlineDays    int     // fewer days than this blocks "strong" (too tight to call it a sure thing)
	UrgentDeadlineDays int     // fewer days than this forces "long_shot" regardless of relevance
}

// Config holds the search tiers' limits, the GetTender tier's limits, the
// fit-tier thresholds, and the statutory EU-threshold band.
type Config struct {
	AnonTier      Tier
	AuthedTier    Tier
	GetTenderTier Tier // generous; GetTender is cheap and called server-side per crawl
	Fit           FitThresholds
	EU            EUThreshold
	// Ranking tunes hybrid fusion and the post-fusion business boost. An
	// unset (zero) Ranking falls back to DefaultRanking — see hybrid.go.
	Ranking Ranking
}

// ProfileSource is the subset of clientprofile.Service this package needs to
// scope a recommendation to one client (workspace) — defined here, the
// consumer, mirroring the KnowledgeBase port above. Its Get is already
// membership-checked (clientprofile.Service.Get requires membership before
// touching its repo), so RecommendForClient does not re-check membership
// itself — see recommend.go.
type ProfileSource interface {
	Get(ctx context.Context, userID, workspaceID string) (clientprofile.Profile, error)
}

// Service runs tender searches.
type Service struct {
	repo     Repo
	kb       KnowledgeBase
	rl       RateLimiter
	profiles ProfileSource
	cfg      Config
}

// NewService returns a Service.
func NewService(repo Repo, kb KnowledgeBase, rl RateLimiter, profiles ProfileSource, cfg Config) *Service {
	return &Service{repo: repo, kb: kb, rl: rl, profiles: profiles, cfg: cfg}
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
	// Mode says which retrievers produced these results. Callers surface it
	// so a degraded search is visibly degraded rather than quietly answering
	// a different question — see RetrievalMode.
	Mode RetrievalMode
}

// Search runs one tender search: rate-limits, clamps the result count to
// the caller's auth tier, then either a filtered browse (no Query text) or
// hybrid retrieval — lexical and vector search fused by Reciprocal Rank
// Fusion, then adjusted by deadline/status/freshness (see hybrid.go).
//
// If one retriever fails the other still answers the query, and Mode reports
// the degradation. If both fail, the search fails: it deliberately does NOT
// fall back to the filters-only path, which ignores the query entirely and
// would return recently-published tenders dressed up as search results.
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

	if strings.TrimSpace(p.Query) == "" {
		return s.searchByFiltersOnly(ctx, p.Filters, limit, offset)
	}
	return s.searchHybrid(ctx, p.Query, p.Filters, limit, offset)
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
	return SearchOutput{Results: results, HasMore: hasMore, Mode: ModeFilters}, nil
}
