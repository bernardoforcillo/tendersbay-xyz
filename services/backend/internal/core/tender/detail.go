package tender

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"
)

// ErrTenderNotFound is returned when no tender has the given id.
var ErrTenderNotFound = errors.New("tender: not found")

// Document is one attached notice/document link.
type Document struct {
	URL  string
	Type string
}

// Lot is one procurement lot.
type Lot struct {
	Ref      string
	Title    string
	CPV      string
	Value    *int64
	Currency string
	Deadline *time.Time
}

// TenderDetail is the full single-tender view (superset of Tender's fields).
type TenderDetail struct {
	ID            string
	Title         string
	BuyerName     string
	BuyerID       string
	Status        string
	ProcedureType string
	Country       string
	NUTS          string
	Language      string
	CPV           string
	CPVSecondary  []string
	Value         *int64
	Currency      string
	PublishedAt   *time.Time
	Deadline      *time.Time
	Source        string
	SourceRef     string
	SourceURL     string
	Documents     []Document
	Lots          []Lot
}

// TenderRef is one sitemap entry.
type TenderRef struct {
	ID      string
	Lastmod string
}

// resolveSourceURL builds a best-effort portal URL from (source, sourceRef).
// Today no source yields a stable per-record URL from the stored ref (TED's
// source_ref is the procedure-identifier, not a publication number), so this
// returns "" and the UI falls back to the tender's documents[]. The switch is
// the seam for adding a real pattern per source later.
func resolveSourceURL(source, sourceRef string) string {
	switch source {
	default:
		return ""
	}
}

// GetTenderParams is GetTender's input.
type GetTenderParams struct {
	ID           string
	RateLimitKey string
}

// RelatedParams is GetRelatedTenders's input.
type RelatedParams struct {
	ID           string
	Limit        int
	RateLimitKey string
}

// GetTender loads one tender's full detail. A non-numeric or absent id maps to
// ErrTenderNotFound. Rate-limited on the dedicated GetTenderTier.
func (s *Service) GetTender(ctx context.Context, p GetTenderParams) (TenderDetail, error) {
	allowed, err := s.rl.Allow(ctx, p.RateLimitKey, s.cfg.GetTenderTier.RateLimit, s.cfg.GetTenderTier.RateWindow)
	if err != nil {
		return TenderDetail{}, fmt.Errorf("%w: %v", ErrRateLimiterUnavailable, err)
	}
	if !allowed {
		return TenderDetail{}, ErrRateLimited
	}
	id, ok := parseTenderID(p.ID)
	if !ok {
		return TenderDetail{}, ErrTenderNotFound
	}
	detail, err := s.repo.FindDetailByID(ctx, id)
	if err != nil {
		return TenderDetail{}, err // includes ErrTenderNotFound
	}
	detail.SourceURL = resolveSourceURL(detail.Source, detail.SourceRef)
	return *detail, nil
}

// GetRelatedTenders returns tenders similar to id by vector recommendation,
// hydrated with card fields. A knowledge-base or enrich failure degrades to an
// empty list (never fails the request), mirroring Search's degrade-to-filters.
func (s *Service) GetRelatedTenders(ctx context.Context, p RelatedParams) ([]ScoredTender, error) {
	allowed, err := s.rl.Allow(ctx, p.RateLimitKey, s.cfg.GetTenderTier.RateLimit, s.cfg.GetTenderTier.RateWindow)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRateLimiterUnavailable, err)
	}
	if !allowed {
		return nil, ErrRateLimited
	}
	limit := p.Limit
	if limit <= 0 || limit > s.cfg.GetTenderTier.MaxResults {
		limit = s.cfg.GetTenderTier.MaxResults
	}
	hits, err := s.kb.RelatedByDocID(ctx, p.ID, limit)
	if err != nil {
		return []ScoredTender{}, nil // degrade
	}
	score := map[string]float32{}
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		if _, ok := score[h.DocID]; !ok {
			ids = append(ids, h.DocID)
		}
		score[h.DocID] = h.Score
	}
	tenders, err := s.repo.EnrichTenders(ctx, ids, Filters{})
	if err != nil {
		return []ScoredTender{}, nil // degrade
	}
	out := make([]ScoredTender, len(tenders))
	for i, t := range tenders {
		out[i] = ScoredTender{Tender: t, RelevanceScore: float64(score[t.ID])}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelevanceScore > out[j].RelevanceScore })
	return out, nil
}

// ListTenderSitemap returns recent tender refs for the dynamic sitemap.
func (s *Service) ListTenderSitemap(ctx context.Context, limit int) ([]TenderRef, error) {
	return s.repo.RecentTenderRefs(ctx, limit)
}

func parseTenderID(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
