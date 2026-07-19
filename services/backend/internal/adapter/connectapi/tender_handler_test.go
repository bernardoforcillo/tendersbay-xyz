package connectapi_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	tenderv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type fakeRepo struct{ results []tender.Tender }

func (f *fakeRepo) SearchTenders(context.Context, tender.Filters, int, int) ([]tender.Tender, error) {
	return f.results, nil
}
func (f *fakeRepo) EnrichTenders(context.Context, []string, tender.Filters) ([]tender.Tender, error) {
	return nil, nil
}
func (f *fakeRepo) FindDetailByID(context.Context, int64) (*tender.TenderDetail, error) {
	return &tender.TenderDetail{ID: "1", Title: "Lavori stradali", Source: "ted", SourceRef: "P1",
		Documents: []tender.Document{{URL: "https://x/notice.pdf", Type: "notice"}}}, nil
}
func (f *fakeRepo) DocumentsByTenderID(context.Context, int64) ([]tender.Document, error) {
	return nil, nil
}
func (f *fakeRepo) LotsByTenderID(context.Context, int64) ([]tender.Lot, error) { return nil, nil }
func (f *fakeRepo) RecentTenderRefs(context.Context, int) ([]tender.TenderRef, error) {
	return []tender.TenderRef{{ID: "1", Lastmod: "2026-01-01T00:00:00Z"}}, nil
}

type fakeKB struct{}

func (fakeKB) SearchWithScores(context.Context, string, int) ([]tender.ScoredChunk, error) {
	return nil, nil
}
func (fakeKB) RelatedByDocID(context.Context, string, int) ([]tender.ScoredChunk, error) {
	return nil, nil
}

type fakeRL struct{}

func (fakeRL) Allow(context.Context, string, int64, time.Duration) (bool, error) {
	return true, nil
}

type fakeProfileSource struct{}

func (fakeProfileSource) Get(context.Context, string, string) (clientprofile.Profile, error) {
	return clientprofile.Profile{}, nil
}

// fakeProfileSourceWithProfile is a settable double for the annotation
// tests below — unlike fakeProfileSource (always an empty profile, no
// error), it lets a test configure the exact profile or error AnnotateForClient sees.
type fakeProfileSourceWithProfile struct {
	profile clientprofile.Profile
	err     error
}

func (f fakeProfileSourceWithProfile) Get(context.Context, string, string) (clientprofile.Profile, error) {
	if f.err != nil {
		return clientprofile.Profile{}, f.err
	}
	return f.profile, nil
}

// fakeMemberRepo is the connectapi_test double for the members port added
// by Task A-annotate (TenderHandler.members) — same shape and allow-list
// pattern as agent.Service's own test double. Deny-all by default; call
// allow to register a membership.
type fakeMemberRepo struct {
	members map[string]bool // "workspaceID|userID" -> is a member
}

func newFakeMemberRepo() *fakeMemberRepo { return &fakeMemberRepo{members: map[string]bool{}} }

func (f *fakeMemberRepo) allow(workspaceID, userID string) {
	f.members[workspaceID+"|"+userID] = true
}

func (f *fakeMemberRepo) LoadMembership(_ context.Context, workspaceID, userID string) (workspace.Membership, error) {
	if f.members[workspaceID+"|"+userID] {
		return workspace.Membership{}, nil
	}
	return workspace.Membership{}, workspace.ErrNotMember
}

func testTenderHandler(t *testing.T) *connectapi.TenderHandler {
	t.Helper()
	repo := &fakeRepo{results: []tender.Tender{{ID: "1", Title: "Lavori stradali"}}}
	cfg := tender.Config{
		AnonTier:      tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier:    tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
		GetTenderTier: tender.Tier{MaxResults: 20, RateLimit: 600, RateWindow: time.Minute},
	}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSource{}, cfg)
	return connectapi.NewTenderHandler(svc, newFakeMemberRepo())
}

func TestSearchTenders_WorksWithoutAuth(t *testing.T) {
	h := testTenderHandler(t)
	// No UserIDFromContext value set on this context — simulates an
	// unauthenticated request. Must not error.
	req := connect.NewRequest(&tenderv1.SearchTendersRequest{Query: "", Limit: 5})
	resp, err := h.SearchTenders(context.Background(), req)
	if err != nil {
		t.Fatalf("SearchTenders (anonymous): %v", err)
	}
	if len(resp.Msg.Results) != 1 {
		t.Fatalf("len(resp.Msg.Results) = %d, want 1", len(resp.Msg.Results))
	}
	if resp.Msg.Results[0].Id != "1" {
		t.Errorf("resp.Msg.Results[0].Id = %q, want %q", resp.Msg.Results[0].Id, "1")
	}
}

func TestSearchTenders_RejectsInvalidDeadlineRangeAsInvalidArgument(t *testing.T) {
	h := testTenderHandler(t)
	req := connect.NewRequest(&tenderv1.SearchTendersRequest{
		Filters: &tenderv1.TenderFilters{DeadlineFrom: "2030-01-01T00:00:00Z", DeadlineTo: "2020-01-01T00:00:00Z"},
	})
	_, err := h.SearchTenders(context.Background(), req)
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("error = %v, want a connect.Error with CodeInvalidArgument", err)
	}
}

func TestGetTender_ReturnsDetailProto(t *testing.T) {
	h := testTenderHandler(t)
	resp, err := h.GetTender(context.Background(), connect.NewRequest(&tenderv1.GetTenderRequest{Id: "1"}))
	if err != nil {
		t.Fatalf("GetTender: %v", err)
	}
	if resp.Msg.Tender.GetId() != "1" || len(resp.Msg.Tender.GetDocuments()) != 1 {
		t.Errorf("tender = %+v, want id 1 with one document", resp.Msg.Tender)
	}
}

func TestGetTender_NotFoundMapsToCodeNotFound(t *testing.T) {
	h := testTenderHandler(t)
	// A non-numeric id makes the service return ErrTenderNotFound before touching the repo.
	_, err := h.GetTender(context.Background(), connect.NewRequest(&tenderv1.GetTenderRequest{Id: "not-a-number"}))
	var ce *connect.Error
	if !errors.As(err, &ce) || ce.Code() != connect.CodeNotFound {
		t.Errorf("err = %v, want CodeNotFound", err)
	}
}

func TestListTenderSitemap_ReturnsRefs(t *testing.T) {
	h := testTenderHandler(t)
	resp, err := h.ListTenderSitemap(context.Background(), connect.NewRequest(&tenderv1.ListTenderSitemapRequest{Limit: 10}))
	if err != nil {
		t.Fatalf("ListTenderSitemap: %v", err)
	}
	if len(resp.Msg.Refs) != 1 || resp.Msg.Refs[0].GetId() != "1" {
		t.Errorf("refs = %+v, want one ref id 1", resp.Msg.Refs)
	}
}

// TestSearchTenders_AnonymousPathLeavesFitFieldsUnset guards Task
// A-annotate's core requirement alongside TestSearchTenders_WorksWithoutAuth
// (left byte-for-byte untouched by this task — its unmodified PASS is the
// proof the anonymous/no-workspace_id path is unchanged): an empty
// workspace_id must never populate fit_tier/reason, even though the handler
// now has a members port and an AnnotateForClient call available to it.
func TestSearchTenders_AnonymousPathLeavesFitFieldsUnset(t *testing.T) {
	h := testTenderHandler(t)
	req := connect.NewRequest(&tenderv1.SearchTendersRequest{Query: "", Limit: 5})
	resp, err := h.SearchTenders(context.Background(), req)
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if len(resp.Msg.Results) != 1 {
		t.Fatalf("len(resp.Msg.Results) = %d, want 1", len(resp.Msg.Results))
	}
	if got := resp.Msg.Results[0]; got.FitTier != "" || got.Reason != nil {
		t.Fatalf("empty workspace_id must never annotate: fit_tier=%q reason=%v", got.FitTier, got.Reason)
	}
}

func testAnnotatedTenderConfig() tender.Config {
	return tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
		Fit:        tender.FitThresholds{RelevanceHigh: 0.75, RelevanceLow: 0.4, MinDeadlineDays: 10, UrgentDeadlineDays: 5},
	}
}

func TestSearchTenders_AnnotatesWhenWorkspaceIdSetAndMember(t *testing.T) {
	repo := &fakeRepo{results: []tender.Tender{{ID: "1", Title: "Lavori stradali", CPV: "45210000"}}}
	profile := clientprofile.Profile{WorkspaceID: "ws-1", Sectors: []string{"45"}}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSourceWithProfile{profile: profile}, testAnnotatedTenderConfig())
	members := newFakeMemberRepo()
	members.allow("ws-1", "user-1")
	h := connectapi.NewTenderHandler(svc, members)

	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	req := connect.NewRequest(&tenderv1.SearchTendersRequest{Limit: 5, WorkspaceId: "ws-1"})
	resp, err := h.SearchTenders(ctx, req)
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if len(resp.Msg.Results) != 1 {
		t.Fatalf("len(resp.Msg.Results) = %d, want 1", len(resp.Msg.Results))
	}
	got := resp.Msg.Results[0]
	if got.FitTier == "" {
		t.Fatal("FitTier not set on the annotated (workspace_id set, member) path")
	}
	if got.Reason == nil {
		t.Fatal("Reason not set on the annotated path")
	}
	if !got.Reason.SectorMatch {
		t.Fatal("Reason.SectorMatch = false, want true (tender CPV 45210000 matches profile sector 45)")
	}
}

// TestSearchTenders_NoProfileYetLeavesFitFieldsUnset covers
// AnnotateForClient's ErrProfileNotFound degradation end-to-end through the
// handler: a member of a workspace with no ClientProfile still gets search
// results back, just unannotated — not a failure.
func TestSearchTenders_NoProfileYetLeavesFitFieldsUnset(t *testing.T) {
	repo := &fakeRepo{results: []tender.Tender{{ID: "1"}}}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSourceWithProfile{err: clientprofile.ErrProfileNotFound}, testAnnotatedTenderConfig())
	members := newFakeMemberRepo()
	members.allow("ws-1", "user-1")
	h := connectapi.NewTenderHandler(svc, members)

	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	req := connect.NewRequest(&tenderv1.SearchTendersRequest{Limit: 5, WorkspaceId: "ws-1"})
	resp, err := h.SearchTenders(ctx, req)
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if got := resp.Msg.Results[0]; got.FitTier != "" || got.Reason != nil {
		t.Fatalf("no ClientProfile yet must not annotate: fit_tier=%q reason=%v", got.FitTier, got.Reason)
	}
}

// TestSearchTenders_NonMemberWorkspaceIdReturnsPermissionDenied proves the
// non-member rejection now via AnnotateForClient's own internal membership
// check, not a handler-level one: SearchTenders no longer calls
// h.members.LoadMembership itself (see its doc comment), so the profile
// source fake stands in for clientprofile.Service.Get → requireMember by
// returning workspace.ErrNotMember, exactly what that call chain produces
// in production for a non-member. h.members is still passed (deny-all,
// unused by this RPC) only because the handler's constructor requires it —
// Task 9 will exercise it directly.
func TestSearchTenders_NonMemberWorkspaceIdReturnsPermissionDenied(t *testing.T) {
	repo := &fakeRepo{results: []tender.Tender{{ID: "1"}}}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSourceWithProfile{err: workspace.ErrNotMember}, testAnnotatedTenderConfig())
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo()) // deny-all, unused by SearchTenders now

	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")
	req := connect.NewRequest(&tenderv1.SearchTendersRequest{Limit: 5, WorkspaceId: "ws-1"})
	_, err := h.SearchTenders(ctx, req)

	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodePermissionDenied {
		t.Fatalf("error = %v, want a connect.Error with CodePermissionDenied", err)
	}
}

// i64 is a small test-local helper for building *int64 literals (e.g.
// tender.Tender.Value, clientprofile.Profile.ValueMin/ValueMax).
func i64(v int64) *int64 { return &v }

// TestRecommendTendersForClient_ReturnsNeedsProfileWhenNoneStored proves the
// handler turns clientprofile.ErrProfileNotFound (surfaced unwrapped by
// RecommendForClient, unlike AnnotateForClient's silent passthrough) into an
// honest needs_profile response rather than an error.
func TestRecommendTendersForClient_ReturnsNeedsProfileWhenNoneStored(t *testing.T) {
	repo := &fakeRepo{results: []tender.Tender{{ID: "1", Title: "Lavori stradali"}}}
	cfg := tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
	}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSourceWithProfile{err: clientprofile.ErrProfileNotFound}, cfg)
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo())
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")

	resp, err := h.RecommendTendersForClient(ctx, connect.NewRequest(&tenderv1.RecommendTendersForClientRequest{WorkspaceId: "ws-1"}))
	if err != nil {
		t.Fatalf("RecommendTendersForClient: %v", err)
	}
	if !resp.Msg.NeedsProfile {
		t.Fatal("NeedsProfile = false, want true")
	}
	if len(resp.Msg.Results) != 0 {
		t.Fatalf("len(Results) = %d, want 0", len(resp.Msg.Results))
	}
}

func TestRecommendTendersForClient_RejectsUnauthenticated(t *testing.T) {
	h := testTenderHandler(t)
	_, err := h.RecommendTendersForClient(context.Background(), connect.NewRequest(&tenderv1.RecommendTendersForClientRequest{WorkspaceId: "ws-1"}))
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeUnauthenticated {
		t.Fatalf("err = %v, want CodeUnauthenticated", err)
	}
}

// TestRecommendTendersForClient_MapsFitTierAndReason covers all six
// tender.ReasonSignals fields making it onto the wire ReasonSignals message
// — including region_match/procedure_match, which Task 8 explicitly
// deferred to this task (see recommend.go's RecommendForClient doc comment
// history / task-9-brief correction #4).
func TestRecommendTendersForClient_MapsFitTierAndReason(t *testing.T) {
	repo := &fakeRepo{results: []tender.Tender{{
		ID: "1", Title: "Lavori stradali", CPV: "45210000", Country: "ITA",
		NUTS: "ITC4C", ProcedureType: "open", Value: i64(150),
	}}}
	cfg := tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
		Fit:        tender.FitThresholds{RelevanceHigh: 0.0, RelevanceLow: -1, MinDeadlineDays: 10, UrgentDeadlineDays: 5}, // RelevanceHigh=0 so a 0-relevance filters-only result still qualifies as "strong"
	}
	profile := clientprofile.Profile{
		WorkspaceID: "ws-1", Sectors: []string{"45"}, Countries: []string{"ITA"},
		Regions: []string{"ITC"}, ProcedureTypes: []string{"open"},
		ValueMin: i64(100), ValueMax: i64(200),
	}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSourceWithProfile{profile: profile}, cfg)
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo())
	ctx := connectapi.ContextWithUserID(context.Background(), "user-1")

	resp, err := h.RecommendTendersForClient(ctx, connect.NewRequest(&tenderv1.RecommendTendersForClientRequest{WorkspaceId: "ws-1", Limit: 3}))
	if err != nil {
		t.Fatalf("RecommendTendersForClient: %v", err)
	}
	if resp.Msg.NeedsProfile {
		t.Fatal("NeedsProfile = true, want false")
	}
	if len(resp.Msg.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(resp.Msg.Results))
	}
	got := resp.Msg.Results[0]
	if got.FitTier != "strong" {
		t.Fatalf("FitTier = %q, want strong", got.FitTier)
	}
	if !got.Reason.SectorMatch || !got.Reason.CountryMatch {
		t.Fatalf("Reason = %+v, want sector+country match", got.Reason)
	}
	if !got.Reason.RegionMatch {
		t.Fatal("Reason.RegionMatch = false, want true (tender NUTS ITC4C matches profile region ITC)")
	}
	if !got.Reason.ProcedureMatch {
		t.Fatal("Reason.ProcedureMatch = false, want true (tender procedure_type open matches profile procedure_types [open])")
	}
	if got.Reason.ValueFit != "in_band" {
		t.Fatalf("Reason.ValueFit = %q, want in_band", got.Reason.ValueFit)
	}
	if got.Reason.HasDeadline {
		t.Fatal("Reason.HasDeadline = true, want false (tender has no deadline)")
	}
	if got.Tender.Id != "1" {
		t.Fatalf("Tender.Id = %q, want 1", got.Tender.Id)
	}
}
