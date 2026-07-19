package tender

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
)

func i64(v int64) *int64 { return &v }

func TestValueFit(t *testing.T) {
	cases := []struct {
		name            string
		value, min, max *int64
		want            string
	}{
		{"no value", nil, i64(100), i64(200), "unknown"},
		{"no band at all", i64(150), nil, nil, "unknown"},
		{"below min", i64(50), i64(100), i64(200), "below"},
		{"above max", i64(250), i64(100), i64(200), "above"},
		{"in band", i64(150), i64(100), i64(200), "in_band"},
		{"at min boundary", i64(100), i64(100), i64(200), "in_band"},
		{"at max boundary", i64(200), i64(100), i64(200), "in_band"},
		{"only min set, above it", i64(150), i64(100), nil, "in_band"},
		{"only min set, below it", i64(50), i64(100), nil, "below"},
		{"only max set, below it", i64(150), nil, i64(200), "in_band"},
		{"only max set, above it", i64(250), nil, i64(200), "above"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := valueFit(tc.value, tc.min, tc.max); got != tc.want {
				t.Fatalf("valueFit(%v,%v,%v) = %q, want %q", tc.value, tc.min, tc.max, got, tc.want)
			}
		})
	}
}

func TestDeadlineDays(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)

	if got := deadlineDays(nil, now); got != nil {
		t.Fatalf("deadlineDays(nil) = %v, want nil", got)
	}

	past := now.Add(-24 * time.Hour)
	if got := deadlineDays(&past, now); got != nil {
		t.Fatalf("deadlineDays(past) = %v, want nil (already closed)", got)
	}

	in10Days := now.Add(10*24*time.Hour + time.Hour) // a hair over 10 whole days
	got := deadlineDays(&in10Days, now)
	if got == nil || *got != 10 {
		t.Fatalf("deadlineDays(+10d1h) = %v, want 10", got)
	}
}

func TestMatchesAnyPrefix(t *testing.T) {
	if !matchesAnyPrefix("45210000", []string{"45"}) {
		t.Fatal("want a match: 45210000 has prefix 45")
	}
	if matchesAnyPrefix("72000000", []string{"45", "80"}) {
		t.Fatal("want no match: 72000000 matches neither prefix")
	}
	if matchesAnyPrefix("45210000", nil) {
		t.Fatal("want no match against an empty prefix list")
	}
}

func TestContainsString(t *testing.T) {
	if !containsString([]string{"open", "restricted"}, "open") {
		t.Fatal("want a match: list contains open")
	}
	if containsString([]string{"open", "restricted"}, "negotiated") {
		t.Fatal("want no match: list does not contain negotiated")
	}
	if containsString(nil, "open") {
		t.Fatal("want no match against a nil list")
	}
}

func TestComputeFitTier(t *testing.T) {
	cfg := FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5}
	days := func(d int) *int { return &d }

	cases := []struct {
		name      string
		relevance float64
		reason    ReasonSignals
		want      FitTier
	}{
		{"high relevance, in band, no deadline", 0.9, ReasonSignals{ValueFit: "in_band"}, FitStrong},
		{"high relevance, unknown value, no deadline", 0.9, ReasonSignals{ValueFit: "unknown"}, FitStrong},
		{"high relevance but value below band", 0.9, ReasonSignals{ValueFit: "below"}, FitLongShot},
		{"high relevance but value above band", 0.9, ReasonSignals{ValueFit: "above"}, FitLongShot},
		{"high relevance but deadline too soon for strong", 0.9, ReasonSignals{ValueFit: "in_band", DeadlineDays: days(8)}, FitPossible},
		{"high relevance but deadline urgent (long-shot floor)", 0.9, ReasonSignals{ValueFit: "in_band", DeadlineDays: days(3)}, FitLongShot},
		{"high relevance, deadline exactly at MinDeadlineDays", 0.9, ReasonSignals{ValueFit: "in_band", DeadlineDays: days(10)}, FitStrong},
		{"mid relevance, in band", 0.6, ReasonSignals{ValueFit: "in_band"}, FitPossible},
		{"low relevance", 0.2, ReasonSignals{ValueFit: "in_band"}, FitLongShot},
		{"relevance exactly at RelevanceLow", 0.4, ReasonSignals{ValueFit: "in_band"}, FitPossible},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeFitTier(tc.relevance, tc.reason, cfg); got != tc.want {
				t.Fatalf("computeFitTier(%v, %+v) = %q, want %q", tc.relevance, tc.reason, got, tc.want)
			}
		})
	}
}

// TestComputeFitTierIgnoresRegionAndProcedureMatch is the delta's required
// invariance check: RegionMatch/ProcedureMatch are tie-breakers and reason
// enrichment only (see the design spec's honesty guardrail) — they must
// never move the tier. This proves it by holding relevance/ValueFit/
// DeadlineDays fixed across every combination of the two new booleans, at
// three different relevance bands (strong / possible / long_shot), and
// asserting the tier never changes.
func TestComputeFitTierIgnoresRegionAndProcedureMatch(t *testing.T) {
	cfg := FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5}
	days := func(d int) *int { return &d }

	base := ReasonSignals{ValueFit: "in_band", DeadlineDays: days(20)}
	regionProcedureCombos := []struct {
		region, procedure bool
	}{
		{false, false},
		{true, false},
		{false, true},
		{true, true},
	}

	for _, relevance := range []float64{0.9, 0.6, 0.2} {
		want := computeFitTier(relevance, base, cfg)
		for _, combo := range regionProcedureCombos {
			variant := base
			variant.RegionMatch = combo.region
			variant.ProcedureMatch = combo.procedure
			if got := computeFitTier(relevance, variant, cfg); got != want {
				t.Fatalf("computeFitTier(%v, %+v) = %q, want %q (relevance=%v baseline) — RegionMatch/ProcedureMatch must not move the tier",
					relevance, variant, got, want, relevance)
			}
		}
	}
}

func TestComputeReasonSignals(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	deadline := now.Add(5 * 24 * time.Hour)
	// NOTE: no CPVSecondary field here — Tender does not have one (Task A0
	// confirmed CPVSecondary is unreachable through the Postgres access
	// layer and was never added to the struct). SectorMatch below is
	// primary-CPV only.
	tn := Tender{
		CPV:           "45210000",
		Country:       "ITA",
		NUTS:          "ITC4",
		ProcedureType: "open",
		Value:         i64(150),
		Deadline:      &deadline,
	}

	got := computeReasonSignals(tn, []string{"45"}, []string{"ITA"}, []string{"ITC"}, []string{"open"}, i64(100), i64(200), now)
	if !got.SectorMatch {
		t.Fatal("SectorMatch = false, want true")
	}
	if !got.CountryMatch {
		t.Fatal("CountryMatch = false, want true")
	}
	if !got.RegionMatch {
		t.Fatal("RegionMatch = false, want true")
	}
	if !got.ProcedureMatch {
		t.Fatal("ProcedureMatch = false, want true")
	}
	if got.ValueFit != "in_band" {
		t.Fatalf("ValueFit = %q, want in_band", got.ValueFit)
	}
	if got.DeadlineDays == nil || *got.DeadlineDays != 5 {
		t.Fatalf("DeadlineDays = %v, want 5", got.DeadlineDays)
	}
}

// TestComputeReasonSignalsRegionAndProcedureMatch is the delta's required
// table coverage for the two new signals: an empty regions/procedureTypes
// list is an honest "not claimed" (never a penalty, never a false match),
// and a non-empty list matches on NUTS prefix / exact ProcedureType.
func TestComputeReasonSignalsRegionAndProcedureMatch(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	tn := Tender{CPV: "45210000", Country: "ITA", NUTS: "ITC4", ProcedureType: "open"}

	cases := []struct {
		name           string
		regions        []string
		procedureTypes []string
		wantRegion     bool
		wantProcedure  bool
	}{
		{"nothing claimed by the client", nil, nil, false, false},
		{"region prefix matches", []string{"ITC"}, nil, true, false},
		{"region prefix does not match", []string{"FRB"}, nil, false, false},
		{"procedure type matches, no region claimed", nil, []string{"open"}, false, true},
		{"procedure type claimed but does not match", nil, []string{"restricted"}, false, false},
		{"region and procedure both match", []string{"ITC"}, []string{"open"}, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeReasonSignals(tn, nil, nil, tc.regions, tc.procedureTypes, nil, nil, now)
			if got.RegionMatch != tc.wantRegion {
				t.Fatalf("RegionMatch = %v, want %v", got.RegionMatch, tc.wantRegion)
			}
			if got.ProcedureMatch != tc.wantProcedure {
				t.Fatalf("ProcedureMatch = %v, want %v", got.ProcedureMatch, tc.wantProcedure)
			}
		})
	}
}

type fakeProfileSource struct {
	profile clientprofile.Profile
	err     error
}

func (f *fakeProfileSource) Get(_ context.Context, _, _ string) (clientprofile.Profile, error) {
	if f.err != nil {
		return clientprofile.Profile{}, f.err
	}
	return f.profile, nil
}

type recommendFakeRepo struct {
	results []Tender
}

func (f *recommendFakeRepo) SearchTenders(_ context.Context, _ Filters, limit, _ int) ([]Tender, error) {
	end := limit
	if end > len(f.results) {
		end = len(f.results)
	}
	return f.results[:end], nil
}

func (f *recommendFakeRepo) EnrichTenders(context.Context, []string, Filters) ([]Tender, error) {
	return nil, nil
}

// recommendFakeRepo's tests exercise RecommendForClient only — the detail-page
// methods below (added to Repo by the tender-detail feature) are never called
// from this file, so trivial stubs satisfy the interface without adding
// meaningful behavior here.
func (f *recommendFakeRepo) FindDetailByID(context.Context, int64) (*TenderDetail, error) {
	return nil, nil
}

func (f *recommendFakeRepo) DocumentsByTenderID(context.Context, int64) ([]Document, error) {
	return nil, nil
}

func (f *recommendFakeRepo) LotsByTenderID(context.Context, int64) ([]Lot, error) {
	return nil, nil
}

func (f *recommendFakeRepo) RecentTenderRefs(context.Context, int) ([]TenderRef, error) {
	return nil, nil
}

type recommendFakeRateLimiter struct{}

func (recommendFakeRateLimiter) Allow(context.Context, string, int64, time.Duration) (bool, error) {
	return true, nil
}

func testFitConfig() Config {
	return Config{
		AnonTier:   Tier{MaxResults: 10, RateLimit: 30, RateWindow: time.Minute},
		AuthedTier: Tier{MaxResults: 50, RateLimit: 300, RateWindow: time.Minute},
		Fit:        FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5},
	}
}

func TestRecommendForClient_ReturnsErrProfileNotFoundUnwrapped(t *testing.T) {
	svc := NewService(&recommendFakeRepo{}, nil, recommendFakeRateLimiter{}, &fakeProfileSource{err: clientprofile.ErrProfileNotFound}, testFitConfig())

	_, err := svc.RecommendForClient(context.Background(), "user-1", "ws-1", 3)
	if !errors.Is(err, clientprofile.ErrProfileNotFound) {
		t.Fatalf("RecommendForClient error = %v, want ErrProfileNotFound", err)
	}
}

func TestRecommendForClient_ScoresAndSortsByTierThenRelevance(t *testing.T) {
	min, max := i64(100), i64(200)
	profile := clientprofile.Profile{
		WorkspaceID: "ws-1", Sectors: []string{"45"}, Countries: []string{"ITA"},
		ValueMin: min, ValueMax: max,
	}
	repo := &recommendFakeRepo{results: []Tender{
		{ID: "1", CPV: "45210000", Country: "ITA", Value: i64(150)}, // in-band, sector+country match
		{ID: "2", CPV: "99000000", Country: "FRA", Value: i64(999)}, // no match, value above band
	}}
	svc := NewService(repo, nil, recommendFakeRateLimiter{}, &fakeProfileSource{profile: profile}, testFitConfig())

	recs, err := svc.RecommendForClient(context.Background(), "user-1", "ws-1", 3)
	if err != nil {
		t.Fatalf("RecommendForClient: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("len(recs) = %d, want 2", len(recs))
	}
	if recs[0].ID != "1" {
		t.Fatalf("recs[0].ID = %q, want %q (the in-band, matching tender should sort first)", recs[0].ID, "1")
	}
	if !recs[0].Reason.SectorMatch || !recs[0].Reason.CountryMatch {
		t.Fatalf("recs[0].Reason = %+v, want sector+country match", recs[0].Reason)
	}
	if recs[1].Reason.ValueFit != "above" {
		t.Fatalf("recs[1].Reason.ValueFit = %q, want above", recs[1].Reason.ValueFit)
	}
}

func TestRecommendForClient_DefaultsLimitWhenNonPositive(t *testing.T) {
	repo := &recommendFakeRepo{results: []Tender{{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}}}
	svc := NewService(repo, nil, recommendFakeRateLimiter{}, &fakeProfileSource{profile: clientprofile.Profile{WorkspaceID: "ws-1"}}, testFitConfig())

	recs, err := svc.RecommendForClient(context.Background(), "user-1", "ws-1", 0)
	if err != nil {
		t.Fatalf("RecommendForClient: %v", err)
	}
	if len(recs) != defaultRecommendLimit {
		t.Fatalf("len(recs) = %d, want the default limit %d", len(recs), defaultRecommendLimit)
	}
}

// TestRecommendForClient_TieBreaksByRegionThenProcedureMatch is the delta's
// required coverage for the sort's extended tie-breakers: when tier and
// relevance are equal (all three tenders below RelevanceLow with no query,
// so all classify long_shot at RelevanceScore 0), RegionMatch is checked
// before ProcedureMatch, both descending. Fed in reverse of the wanted
// order to prove the sort — not fake-repo or slice order — decides it.
func TestRecommendForClient_TieBreaksByRegionThenProcedureMatch(t *testing.T) {
	profile := clientprofile.Profile{
		WorkspaceID:    "ws-1",
		Regions:        []string{"ITC"},
		ProcedureTypes: []string{"open"},
	}
	repo := &recommendFakeRepo{results: []Tender{
		{ID: "neither", NUTS: "FRB1", ProcedureType: "restricted"},
		{ID: "procedure-only", NUTS: "FRB1", ProcedureType: "open"},
		{ID: "region-only", NUTS: "ITC4", ProcedureType: "restricted"},
	}}
	svc := NewService(repo, nil, recommendFakeRateLimiter{}, &fakeProfileSource{profile: profile}, testFitConfig())

	recs, err := svc.RecommendForClient(context.Background(), "user-1", "ws-1", 3)
	if err != nil {
		t.Fatalf("RecommendForClient: %v", err)
	}
	if len(recs) != 3 {
		t.Fatalf("len(recs) = %d, want 3", len(recs))
	}
	wantOrder := []string{"region-only", "procedure-only", "neither"}
	for i, want := range wantOrder {
		if recs[i].ID != want {
			t.Fatalf("recs[%d].ID = %q, want %q (order: %v)", i, recs[i].ID, want, wantOrder)
		}
	}
}

// ── AnnotateForClient (Task A-annotate) ──────────────────────────────────

// TestAnnotateForClient_PreservesOrderAndAnnotates is the brief's Step 1
// test: search results in a fixed order, annotated, order preserved, each
// result carries the tier/reason a profile match implies.
func TestAnnotateForClient_PreservesOrderAndAnnotates(t *testing.T) {
	profile := clientprofile.Profile{WorkspaceID: "ws-1", Sectors: []string{"45"}}
	svc := NewService(&recommendFakeRepo{}, nil, recommendFakeRateLimiter{}, &fakeProfileSource{profile: profile}, testFitConfig())

	results := []ScoredTender{
		{Tender: Tender{ID: "A", CPV: "45210000"}}, // sector match
		{Tender: Tender{ID: "B", CPV: "72000000"}}, // no match
	}

	out, err := svc.AnnotateForClient(context.Background(), "u1", "ws1", results)
	if err != nil {
		t.Fatalf("AnnotateForClient: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].ScoredTender.ID != results[0].ID || out[1].ScoredTender.ID != results[1].ID {
		t.Fatal("order must be preserved (annotate, not re-rank)")
	}
	if !out[0].Reason.SectorMatch {
		t.Fatal("A should be a sector match")
	}
	if out[1].Reason.SectorMatch {
		t.Fatal("B should not be a sector match")
	}
	if out[0].Tier == "" || out[1].Tier == "" {
		t.Fatal("each result should carry a non-empty fit tier when a profile exists")
	}
}

// TestAnnotateForClient_NeverReorders feeds results in an order a
// tier-based sort would NOT produce (the long_shot tender first, the
// strong one second) and asserts AnnotateForClient leaves that order
// alone — the whole point of annotation vs. RecommendForClient's shortlist.
func TestAnnotateForClient_NeverReorders(t *testing.T) {
	profile := clientprofile.Profile{WorkspaceID: "ws-1", Sectors: []string{"45"}, ValueMin: i64(100), ValueMax: i64(200)}
	svc := NewService(&recommendFakeRepo{}, nil, recommendFakeRateLimiter{}, &fakeProfileSource{profile: profile}, testFitConfig())

	results := []ScoredTender{
		{Tender: Tender{ID: "long-shot", CPV: "99000000", Value: i64(999)}, RelevanceScore: 0.1},
		{Tender: Tender{ID: "strong", CPV: "45210000", Value: i64(150)}, RelevanceScore: 0.9},
	}

	out, err := svc.AnnotateForClient(context.Background(), "u1", "ws1", results)
	if err != nil {
		t.Fatalf("AnnotateForClient: %v", err)
	}
	if out[0].ID != "long-shot" || out[1].ID != "strong" {
		t.Fatalf("AnnotateForClient reordered results: got [%s, %s], want input order [long-shot, strong]", out[0].ID, out[1].ID)
	}
	if out[0].Tier != FitLongShot {
		t.Fatalf("out[0].Tier = %q, want %q", out[0].Tier, FitLongShot)
	}
	if out[1].Tier != FitStrong {
		t.Fatalf("out[1].Tier = %q, want %q", out[1].Tier, FitStrong)
	}
}

// TestAnnotateForClient_ProfileNotFoundReturnsUnannotatedPassthrough
// asserts the brief's required degradation: no profile yet ⇒ no
// annotation and no error, not a failure — order is still preserved.
func TestAnnotateForClient_ProfileNotFoundReturnsUnannotatedPassthrough(t *testing.T) {
	svc := NewService(&recommendFakeRepo{}, nil, recommendFakeRateLimiter{}, &fakeProfileSource{err: clientprofile.ErrProfileNotFound}, testFitConfig())

	results := []ScoredTender{
		{Tender: Tender{ID: "1"}},
		{Tender: Tender{ID: "2"}},
	}

	out, err := svc.AnnotateForClient(context.Background(), "u1", "ws1", results)
	if err != nil {
		t.Fatalf("AnnotateForClient: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	for i, r := range out {
		if r.ID != results[i].ID {
			t.Fatalf("out[%d].ID = %q, want %q (order preserved)", i, r.ID, results[i].ID)
		}
		if r.Tier != "" {
			t.Fatalf("out[%d].Tier = %q, want empty (no profile ⇒ no annotation)", i, r.Tier)
		}
	}
}
