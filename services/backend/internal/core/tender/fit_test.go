package tender

import (
	"context"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
)

// fitFakeRepo satisfies tender.Repo for the fit/summary methods: EnrichTenders
// returns configured cards, FindDetailByID returns one detail (or not-found).
type fitFakeRepo struct {
	enrich []Tender
	detail *TenderDetail
}

func (f *fitFakeRepo) SearchTenders(context.Context, Filters, SortOrder, int, int) ([]Tender, error) {
	return nil, nil
}

func (f *fitFakeRepo) FacetCounts(context.Context, Filters) (Facets, error) {
	return Facets{}, nil
}
func (f *fitFakeRepo) LexicalSearch(context.Context, LexicalQuery, Filters, int) ([]ScoredTender, error) {
	return nil, nil
}
func (f *fitFakeRepo) FindByCPVPrefixes(context.Context, []string, Filters, int) ([]ScoredTender, error) {
	return nil, nil
}

func (f *fitFakeRepo) EnrichTenders(_ context.Context, _ []string, _ Filters) ([]Tender, error) {
	return f.enrich, nil
}
func (f *fitFakeRepo) FindDetailByID(_ context.Context, _ int64) (*TenderDetail, error) {
	if f.detail == nil {
		return nil, ErrTenderNotFound
	}
	return f.detail, nil
}
func (f *fitFakeRepo) DocumentsByTenderID(context.Context, int64) ([]Document, error) {
	return nil, nil
}
func (f *fitFakeRepo) LotsByTenderID(context.Context, int64) ([]Lot, error) { return nil, nil }
func (f *fitFakeRepo) CriteriaByTenderID(context.Context, int64) ([]AwardCriterion, error) {
	return nil, nil
}
func (f *fitFakeRepo) OrganizationsByTenderID(context.Context, int64) ([]Organization, error) {
	return nil, nil
}

func (f *fitFakeRepo) RecentTenderRefs(context.Context, int) ([]TenderRef, error) { return nil, nil }
func (f *fitFakeRepo) DistinctCountries(context.Context) ([]string, error)        { return nil, nil }

func TestFitForTenders_ClassifiesAndFlagsAvailability(t *testing.T) {
	profile := clientprofile.Profile{WorkspaceID: "ws1", Sectors: []string{"45"}, Countries: []string{"IT"}}
	repo := &fitFakeRepo{enrich: []Tender{
		{ID: "10", CPV: "45210000", Country: "IT"}, // sector+country match, no deadline -> strong
	}}
	svc := NewService(repo, nil, recommendFakeRateLimiter{}, &fakeProfileSource{profile: profile}, testFitConfig())

	fits, err := svc.FitForTenders(context.Background(), "u1", "ws1", []int64{10, 20})
	if err != nil {
		t.Fatalf("FitForTenders: %v", err)
	}
	if len(fits) != 2 {
		t.Fatalf("len(fits) = %d, want 2 (one entry per requested id)", len(fits))
	}
	if !fits[10].Available || !fits[10].HasProfile || fits[10].Tier != FitStrong {
		t.Fatalf("id 10 = %+v, want available+hasProfile+strong", fits[10])
	}
	if fits[20].Available {
		t.Fatal("id 20 was not returned by EnrichTenders -> Available must be false")
	}
}

func TestFitForTenders_NoProfileReturnsEmptyTier(t *testing.T) {
	repo := &fitFakeRepo{enrich: []Tender{{ID: "10", CPV: "45210000", Country: "IT"}}}
	svc := NewService(repo, nil, recommendFakeRateLimiter{}, &fakeProfileSource{err: clientprofile.ErrProfileNotFound}, testFitConfig())

	fits, err := svc.FitForTenders(context.Background(), "u1", "ws1", []int64{10})
	if err != nil {
		t.Fatalf("FitForTenders: %v", err)
	}
	if fits[10].HasProfile {
		t.Fatal("HasProfile must be false with no profile")
	}
	if fits[10].Tier != "" {
		t.Fatalf("tier = %q, want empty (never a fabricated tier)", fits[10].Tier)
	}
	if !fits[10].Available {
		t.Fatal("id 10 resolved -> Available true even without a profile")
	}
}

func TestFitForTender_Single(t *testing.T) {
	profile := clientprofile.Profile{WorkspaceID: "ws1", Sectors: []string{"45"}, Countries: []string{"IT"}}
	repo := &fitFakeRepo{detail: &TenderDetail{ID: "10", CPV: "45210000", Country: "IT"}}
	svc := NewService(repo, nil, recommendFakeRateLimiter{}, &fakeProfileSource{profile: profile}, testFitConfig())

	res, err := svc.FitForTender(context.Background(), "u1", "ws1", "10")
	if err != nil {
		t.Fatalf("FitForTender: %v", err)
	}
	if !res.Available || !res.HasProfile || res.Tier != FitStrong {
		t.Fatalf("res = %+v, want available+hasProfile+strong", res)
	}

	repo.detail = nil
	res, err = svc.FitForTender(context.Background(), "u1", "ws1", "999")
	if err != nil {
		t.Fatalf("FitForTender(missing): %v", err)
	}
	if res.Available {
		t.Fatal("missing tender -> Available false, no error")
	}
}

func TestSummariesByIDs_MapsAndSkipsMissing(t *testing.T) {
	deadline := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	repo := &fitFakeRepo{enrich: []Tender{
		{ID: "10", Title: "Road works", BuyerName: "City", Country: "IT", CPV: "45000000", Currency: "EUR", Status: "open", Value: i64(500000), Deadline: &deadline},
	}}
	svc := NewService(repo, nil, recommendFakeRateLimiter{}, &fakeProfileSource{}, testFitConfig())

	sums, err := svc.SummariesByIDs(context.Background(), []int64{10, 20})
	if err != nil {
		t.Fatalf("SummariesByIDs: %v", err)
	}
	if len(sums) != 1 {
		t.Fatalf("len(sums) = %d, want 1 (id 20 not found -> omitted)", len(sums))
	}
	s := sums[10]
	if s.ID != 10 || s.Title != "Road works" || s.Country != "IT" || s.Currency != "EUR" || s.Status != "open" {
		t.Fatalf("summary = %+v", s)
	}
	if s.Value == nil || *s.Value != 500000 {
		t.Fatalf("value = %v, want 500000", s.Value)
	}
	if s.Deadline != deadline.Format(time.RFC3339) {
		t.Fatalf("deadline = %q, want %q", s.Deadline, deadline.Format(time.RFC3339))
	}
}
