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

// minorUnitScale converts whole currency units to the minor units (cents) that
// ingested_tenders.value is stored in — the house rule stated in
// company.proto's header, "money travels as int64 minor units".
//
// It exists as a named constant because the mismatch it guards against is not
// hypothetical: a bare 100 read as €100 where the column holds €1.00 is
// invisible in review, wrong by two orders of magnitude, and was shipped in
// both the display and the query-bound paths. Anywhere a whole-euro figure
// meets Tender.Value, it should meet this constant too.
const minorUnitScale = 100

// Tender is a search result's structured fields.
type Tender struct {
	ID            string
	Title         string
	BuyerName     string
	Status        string
	ProcedureType string
	Country       string
	CPV           string
	// Value is in MINOR units (cents), as ingested by parseMinorUnits — a
	// €6,297,240.05 notice is 629_724_005 here. nil when the notice publishes
	// no estimate, which is common.
	Value       *int64
	Currency    string
	PublishedAt *time.Time
	Deadline    *time.Time
	Source      string
	SourceRef   string
	NUTS        string
	SourceURL   string // the notice document's URL; "" if none is ingested

	// ── The eForms detail, as same-row scalars ──
	//
	// Every field below is a scalar column on tenders.ingested_tenders, so it
	// costs zero extra queries: it rides the one projection every search
	// already runs (see tenderSelectColumns in the postgres adapter).
	//
	// Award criteria, organizations and lots are deliberately NOT here. They
	// are child tables, and this struct is materialised across a candidate
	// window bounded at maxWindow (hybrid.go) — one join per search would be
	// an N+1 on the hot path for data no result card shows. They live on
	// TenderDetail, whose fetch is already per-tender.

	// Description is the notice's free-text scope of work; "" when the source
	// exposes none. It is carried in full rather than truncated: the search
	// projection already reads the head of this column to build its snippet,
	// so the page is detoasted and the I/O is paid either way, and a silently
	// truncated field is worse than either extreme — a reader cannot tell a
	// short description from a clipped one.
	Description string

	// PublicationNumber, DocumentsURL, SubmissionURL and GridUsable are
	// DELIBERATELY ABSENT from this type. They live on TenderDetail only.
	//
	// They were briefly here, populated from the shared search projection, and
	// that took every search down in production: those columns are created by
	// services/ingestion's migration 0010, this service does not migrate the
	// tenders schema, and the two deploy on independent image tags — so the
	// backend asked for a column that did not exist yet and every query failed
	// with SQLSTATE 42703, on both the lexical and the semantic arm.
	//
	// Nothing had ever read them from a search result; the proto carries them
	// on TenderDetail alone. Two rules follow, and postgres.TenderSelectColumns'
	// guard tests enforce them: a field only the detail view consumes never
	// joins the hot projection, and this type never names a column from a
	// migration younger than one full release cycle.
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
	Buyer string
	// ValueMin/ValueMax are in MINOR units, the same scale as Tender.Value —
	// they become "t.value >= $n" / "t.value <= $n" verbatim, so any other unit
	// silently shifts the band by a factor of 100. nil = unbounded on that side,
	// which is distinct from a zero bound.
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
	// Snippet is a fragment of the matched text with the query terms wrapped
	// in <mark>…</mark>, so a result can show WHY it matched. Only the keyword
	// retriever can produce one, so it is empty for results found by vector
	// search alone and for filters-only browses.
	//
	// It contains raw tender text around those markers: renderers must split
	// on the markers and escape everything else, never inject it as HTML.
	Snippet string
}

// LexicalQuery is what the keyword retriever is asked for.
//
// It is a struct rather than a bare string because Repo has nine methods and
// five fakes implement it: every scalar parameter added later would break all
// six call sites at once, whereas a new FIELD breaks nothing. The fields that
// follow Text are all "how should this text be matched", not "what to match".
type LexicalQuery struct {
	// Text is the query with structured constraints already lifted out — see
	// ParseQuery. Empty means there is nothing to retrieve on.
	Text string
}

// Repo is the subset of postgres.TenderRepo the service needs.
type Repo interface {
	SearchTenders(ctx context.Context, filters Filters, sortBy SortOrder, limit, offset int) ([]Tender, error)
	// FacetCounts aggregates the whole filtered corpus by country, status and
	// CPV division — the browse path's facets are exact, not windowed.
	FacetCounts(ctx context.Context, filters Filters) (Facets, error)
	// LexicalSearch is the keyword half of hybrid retrieval — full-text and
	// trigram matching, already filtered and ranked, best-first. It answers
	// the queries dense embeddings structurally can't: exact codes, notice
	// references, buyer names, rare acronyms.
	LexicalSearch(ctx context.Context, q LexicalQuery, filters Filters, limit int) ([]ScoredTender, error)
	// FindByCPVPrefixes returns tenders whose primary or secondary CPV is
	// prefixed by any of codes, newest first. It backs the CPV retrieval arm —
	// the language-independent half of cross-language search.
	//
	// Prefix rather than equality so a resolved division ("45") behaves the same
	// way the CPVPrefixes filter already does, and so a resolved 8-digit code
	// still matches a tender that records it with a trailing refinement.
	FindByCPVPrefixes(ctx context.Context, codes []string, filters Filters, limit int) ([]ScoredTender, error)
	EnrichTenders(ctx context.Context, ids []string, filters Filters) ([]Tender, error)
	FindDetailByID(ctx context.Context, id int64) (*TenderDetail, error)
	DocumentsByTenderID(ctx context.Context, id int64) ([]Document, error)
	LotsByTenderID(ctx context.Context, id int64) ([]Lot, error)
	// CriteriaByTenderID returns every criterion for one tender — the
	// notice-level ones and every lot's — in ONE query, ordered by
	// (lot_ref, ordinal). Grouping by lot happens in Go (see
	// TenderDetail.CriteriaForLot): a query per lot would be an N+1 against a
	// 13-lot notice, for a set small enough to sort in memory.
	CriteriaByTenderID(ctx context.Context, id int64) ([]AwardCriterion, error)
	// OrganizationsByTenderID returns the parties the notice names — the
	// buyer, and the bodies a bidder needs in order to challenge an award.
	OrganizationsByTenderID(ctx context.Context, id int64) ([]Organization, error)
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
	// EmbedQuery and SearchByVector are separate because embedding is a
	// network round trip to the model server and searching is not: splitting
	// them is what lets this package cache the expensive half (see
	// EmbeddingCache) instead of paying for it again on every page of the
	// same search.
	EmbedQuery(ctx context.Context, query string) ([]float32, error)
	// SearchByVector narrows the vector search by the same facets the SQL
	// side filters on. Pushing the filter into the vector store is what keeps
	// a narrow filter (one country out of 27) from consuming the whole
	// candidate window with rows that will be discarded moments later. It is
	// an optimisation only: results are filtered authoritatively in Postgres
	// afterwards either way.
	SearchByVector(ctx context.Context, vec []float32, limit int, filters Filters) ([]ScoredChunk, error)
	RelatedByDocID(ctx context.Context, docID string, limit int) ([]ScoredChunk, error)
}

// EmbeddingCache memoises query embeddings across requests. Optional: a nil
// cache simply means every search embeds its own query.
//
// Correctness never depends on it — a miss, a corrupt entry, or an unreachable
// cache all just mean "compute it", which is always available and always
// right. Cache errors are therefore treated as misses rather than propagated.
type EmbeddingCache interface {
	// GetEmbedding reports ok=false for a miss, with a nil error.
	GetEmbedding(ctx context.Context, query string) ([]float32, bool, error)
	SetEmbedding(ctx context.Context, query string, vec []float32) error
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
	cache    EmbeddingCache // optional; nil disables embedding reuse
	lexicon  CPVLexicon     // optional; nil disables the CPV arm
	cfg      Config
}

// NewService returns a Service.
func NewService(repo Repo, kb KnowledgeBase, rl RateLimiter, profiles ProfileSource, cfg Config) *Service {
	return &Service{repo: repo, kb: kb, rl: rl, profiles: profiles, cfg: cfg}
}

// WithEmbeddingCache attaches a query-embedding cache and returns s. It is a
// separate step rather than a constructor argument because the cache is
// genuinely optional — the service is fully correct without one, just slower.
func (s *Service) WithEmbeddingCache(c EmbeddingCache) *Service {
	s.cache = c
	return s
}

// WithCPVLexicon attaches a CPV vocabulary and returns s.
//
// A separate step rather than a constructor argument for the same reason
// WithEmbeddingCache is: the service is fully correct without one — the CPV arm
// simply contributes nothing — and a database whose cpv_terms table has not been
// seeded must not be able to stop the service from starting.
func (s *Service) WithCPVLexicon(l CPVLexicon) *Service {
	s.lexicon = l
	return s
}

// embedQuery returns query's embedding, reusing a cached one when available.
//
// Every cache failure degrades to computing the embedding: a cache that is
// down, slow to parse, or holding garbage must never be able to fail a search
// that could otherwise have succeeded.
func (s *Service) embedQuery(ctx context.Context, query string) ([]float32, error) {
	if s.cache != nil {
		if vec, ok, err := s.cache.GetEmbedding(ctx, query); err == nil && ok {
			return vec, nil
		}
	}
	vec, err := s.kb.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		// A failed write is not worth failing the request over — the next
		// search simply recomputes.
		_ = s.cache.SetEmbedding(ctx, query, vec)
	}
	return vec, nil
}

// SearchParams is Search's input. RateLimitKey is the client IP for
// anonymous callers or the user ID for authenticated ones — the caller
// (the ConnectRPC handler) decides which, Search just uses whatever key
// it's given.
type SearchParams struct {
	Query   string
	Filters Filters
	// ParseQuery lifts constraints out of Query text ("sotto 100k" becomes a
	// value bound — see ParseQuery). Opt-in per call site, because it is only
	// right for text a PERSON typed into a search box.
	//
	// Feeding it arbitrary domain prose misreads it: a client profile
	// describing a company as "fatturato oltre 5 milioni" would become a
	// "value >= 5,000,000" filter and quietly exclude almost every tender that
	// client could actually bid on.
	ParseQuery bool
	// Sort selects the result ordering; the zero value means relevance.
	Sort  SortOrder
	Limit int
	// SuppressedCPV are codes the caller does not want inferred, because the
	// user removed the chip for them. Client-supplied because the server
	// would otherwise resolve the same code from the same text on the next
	// page — see cpvCandidates.
	SuppressedCPV []string
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
	// AppliedFilters and AppliedQuery are what the search actually ran with
	// after constraints were lifted out of the query text (see ParseQuery).
	// Returned so the UI can show what it understood and let the user undo it
	// — a filter the user can't see is one they can't correct.
	AppliedFilters Filters
	AppliedQuery   string
	// AppliedCPV are the CPV codes the search inferred from the query text, with
	// the label that matched. Reported for the same reason AppliedFilters is: the
	// user must be able to see that "pulizie uffici" was read as
	// "90919200 — Servizi di pulizia di uffici", and remove it.
	//
	// Unlike AppliedFilters these did not NARROW anything — the CPV arm is a
	// boost, so a wrong match reorders the page rather than emptying it.
	AppliedCPV []CPVMatch
	// Facets are per-field counts for filter-bar badges. See Facets for what
	// they count, which differs between a query-driven search and a browse.
	Facets Facets
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
		out, err := s.searchByFiltersOnly(ctx, p.Filters, p.Sort, limit, offset)
		return withApplied(out, err, p.Filters, "")
	}

	// Lift the constraints hiding in the prose ("sotto 100k", "entro 30
	// giorni") into real filters before retrieving — see ParseQuery. Filters
	// the caller set explicitly always win over what was inferred.
	parsed := ParsedQuery{Text: p.Query}
	if p.ParseQuery {
		parsed = ParseQuery(p.Query, time.Now())
	}
	filters := mergeParsedFilters(p.Filters, parsed.Filters)

	// If the query was nothing BUT constraints ("bandi aperti sotto 100k"),
	// there is no text left to retrieve on and this is really a browse.
	if parsed.Text == "" {
		out, err := s.searchByFiltersOnly(ctx, filters, p.Sort, limit, offset)
		return withApplied(out, err, filters, "")
	}
	out, err := s.searchHybrid(ctx, parsed.Text, filters, p.Sort, limit, offset, p.SuppressedCPV)
	return withApplied(out, err, filters, parsed.Text)
}

// withApplied stamps the filters and query the search actually ran with onto
// its output, so every return path reports them the same way.
func withApplied(out SearchOutput, err error, filters Filters, query string) (SearchOutput, error) {
	if err != nil {
		return SearchOutput{}, err
	}
	out.AppliedFilters = filters
	out.AppliedQuery = query
	return out, nil
}

// searchByFiltersOnly is the browse path: no query text, so results are
// ordered by the requested sort (publication date by default) rather than by a
// relevance that doesn't exist.
func (s *Service) searchByFiltersOnly(ctx context.Context, filters Filters, sortBy SortOrder, limit, offset int) (SearchOutput, error) {
	tenders, err := s.repo.SearchTenders(ctx, filters, browseSort(sortBy), limit+1, offset)
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

	// Browse facets are exact counts over the whole filtered corpus, which the
	// database can aggregate directly — unlike a query-driven search, there is
	// no ranked window here to count instead. A facet failure is not worth
	// failing the search over; the badges simply don't render.
	out := SearchOutput{Results: results, HasMore: hasMore, Mode: ModeFilters}
	if facets, facetErr := s.repo.FacetCounts(ctx, filters); facetErr == nil {
		out.Facets = facets
	}
	return out, nil
}

// browseSort maps a requested order onto one the browse path can serve.
// Relevance is undefined with no query, so it becomes "newest first" rather
// than an arbitrary order dressed up as ranking.
func browseSort(sortBy SortOrder) SortOrder {
	if sortBy == SortRelevance || sortBy == "" {
		return SortPublished
	}
	return sortBy
}
