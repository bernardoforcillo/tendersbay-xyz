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
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type fakeRepo struct {
	results   []tender.Tender
	countries []string
	// detail overrides FindDetailByID's canned answer, for the tests that
	// exercise the eForms fields the criteria surface reads.
	detail *tender.TenderDetail
}

func (f *fakeRepo) FacetCounts(context.Context, tender.Filters) (tender.Facets, error) {
	return tender.Facets{}, nil
}

func (f *fakeRepo) SearchTenders(context.Context, tender.Filters, tender.SortOrder, int, int) ([]tender.Tender, error) {
	return f.results, nil
}
func (f *fakeRepo) DistinctCountries(context.Context) ([]string, error) {
	return f.countries, nil
}
func (f *fakeRepo) LexicalSearch(context.Context, tender.LexicalQuery, tender.Filters, int) ([]tender.ScoredTender, error) {
	return nil, nil
}
func (f *fakeRepo) FindByCPVPrefixes(context.Context, []string, tender.Filters, int) ([]tender.ScoredTender, error) {
	return nil, nil
}

func (f *fakeRepo) EnrichTenders(context.Context, []string, tender.Filters) ([]tender.Tender, error) {
	return nil, nil
}
func (f *fakeRepo) FindDetailByID(context.Context, int64) (*tender.TenderDetail, error) {
	if f.detail != nil {
		return f.detail, nil
	}
	return &tender.TenderDetail{ID: "1", Title: "Lavori stradali", Source: "ted", SourceRef: "P1",
		Documents: []tender.Document{{URL: "https://x/notice.pdf", Type: "notice"}}}, nil
}
func (f *fakeRepo) DocumentsByTenderID(context.Context, int64) ([]tender.Document, error) {
	return nil, nil
}
func (f *fakeRepo) LotsByTenderID(context.Context, int64) ([]tender.Lot, error) { return nil, nil }
func (f *fakeRepo) CriteriaByTenderID(context.Context, int64) ([]tender.AwardCriterion, error) {
	return nil, nil
}
func (f *fakeRepo) OrganizationsByTenderID(context.Context, int64) ([]tender.Organization, error) {
	return nil, nil
}
func (f *fakeRepo) RecentTenderRefs(context.Context, int) ([]tender.TenderRef, error) {
	return []tender.TenderRef{{ID: "1", Lastmod: "2026-01-01T00:00:00Z"}}, nil
}

// fakeLexicon is a settable double for tender.CPVLexicon — it always
// returns its canned matches and no error, mirroring the always-succeeds
// contract every other fake in this file gives its own narrow port.
type fakeLexicon struct{ matches []tender.CPVMatch }

func (f fakeLexicon) MatchCodes(context.Context, string, int) ([]tender.CPVMatch, error) {
	return f.matches, nil
}

type fakeKB struct{}

func (fakeKB) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{0.1}, nil
}

func (fakeKB) SearchByVector(context.Context, []float32, int, tender.Filters) ([]tender.ScoredChunk, error) {
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
	return connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{})
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

// TestSearchTenders_ReportsAppliedCPVToTheClient proves cpvMatchesToProto's
// mapping end to end: a query the lexicon resolves to a code must reach the
// wire response with that code, its label and its language — a signal the
// user can't see is one the UI can't offer to undo.
func TestSearchTenders_ReportsAppliedCPVToTheClient(t *testing.T) {
	repo := &fakeRepo{}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, nil, tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 100, RateWindow: time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 100, RateWindow: time.Minute},
		Ranking:    tender.DefaultRanking(),
	}).WithCPVLexicon(fakeLexicon{matches: []tender.CPVMatch{
		{Code: "90919200", Lang: "it", Label: "Servizi di pulizia di uffici", Score: 0.9},
	}})
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{})

	res, err := h.SearchTenders(context.Background(), connect.NewRequest(&tenderv1.SearchTendersRequest{
		Query: "pulizie uffici", Limit: 10,
	}))
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}

	if len(res.Msg.AppliedCpv) != 1 {
		t.Fatalf("AppliedCpv = %+v, want one match — the UI cannot let the user undo what it cannot see", res.Msg.AppliedCpv)
	}
	got := res.Msg.AppliedCpv[0]
	if got.Code != "90919200" || got.Label != "Servizi di pulizia di uffici" || got.Language != "it" {
		t.Errorf("AppliedCpv[0] = %+v, want the code, its label and its language", got)
	}
}

// TestSearchTenders_AppliedCPVIsEmptyNotNilWithoutALexicon proves the
// empty-slice-not-nil house rule (see tender.page) also holds for AppliedCpv:
// the field reaches JSON, where nil and [] read differently to a client.
func TestSearchTenders_AppliedCPVIsEmptyNotNilWithoutALexicon(t *testing.T) {
	repo := &fakeRepo{}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, nil, tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 100, RateWindow: time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 100, RateWindow: time.Minute},
	})
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{})

	res, err := h.SearchTenders(context.Background(), connect.NewRequest(&tenderv1.SearchTendersRequest{
		Query: "pulizie", Limit: 10,
	}))
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if res.Msg.AppliedCpv == nil {
		t.Error("AppliedCpv is nil, want an empty slice")
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
	h := connectapi.NewTenderHandler(svc, members, &fakeDocReader{})

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
	h := connectapi.NewTenderHandler(svc, members, &fakeDocReader{})

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
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{}) // deny-all, unused by SearchTenders now

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
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{})
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

// TestGetCoverage_ReturnsCountriesAnonymously proves GetCoverage needs no
// auth (like SearchTenders) and passes the service's countries through.
func TestGetCoverage_ReturnsCountriesAnonymously(t *testing.T) {
	repo := &fakeRepo{countries: []string{"IT", "PL"}}
	cfg := tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
	}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSource{}, cfg)
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{})

	// No UserIDFromContext value — an anonymous request must still succeed.
	resp, err := h.GetCoverage(context.Background(), connect.NewRequest(&tenderv1.GetCoverageRequest{}))
	if err != nil {
		t.Fatalf("GetCoverage: %v", err)
	}
	if len(resp.Msg.Countries) != 2 {
		t.Fatalf("countries = %v, want [IT PL]", resp.Msg.Countries)
	}
	if resp.Msg.Countries[0] != "IT" || resp.Msg.Countries[1] != "PL" {
		t.Fatalf("countries = %v, want [IT PL]", resp.Msg.Countries)
	}
}

// euThresholdTenderConfig defaults the 2026-2027 EU thresholds so the handler
// maps eu_threshold onto results the same way production main.go does.
func euThresholdTenderConfig() tender.Config {
	return tender.Config{
		AnonTier:   tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier: tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
		EU: tender.EUThreshold{
			WorksMinor:              540400000, // €5,404,000
			SuppliesCentralMinor:    14000000,  // €140,000
			SuppliesSubCentralMinor: 21600000,  // €216,000
		},
	}
}

// TestTenderResultToProto_SetsEuThreshold proves every SearchTenders result
// carries the coarse EU-threshold band: a works tender under €5.404M is
// below_eu, one over is above_eu, and an unknown (nil) value stays "".
func TestTenderResultToProto_SetsEuThreshold(t *testing.T) {
	cases := []struct {
		name  string
		value *int64
		want  string
	}{
		{"works below threshold", i64(100000_00), "below_eu"},  // €100k < €5.404M
		{"works above threshold", i64(6000000_00), "above_eu"}, // €6M > €5.404M
		{"unknown value", nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &fakeRepo{results: []tender.Tender{{ID: "1", CPV: "45210000", Value: c.value}}}
			svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSource{}, euThresholdTenderConfig())
			h := connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{})

			resp, err := h.SearchTenders(context.Background(), connect.NewRequest(&tenderv1.SearchTendersRequest{Limit: 5}))
			if err != nil {
				t.Fatalf("SearchTenders: %v", err)
			}
			if len(resp.Msg.Results) != 1 {
				t.Fatalf("len(Results) = %d, want 1", len(resp.Msg.Results))
			}
			if got := resp.Msg.Results[0].EuThreshold; got != c.want {
				t.Fatalf("EuThreshold = %q, want %q", got, c.want)
			}
		})
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
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{})
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

// ── GetTenderPassages ───────────────────────────────────────────────────────

// fakeDocReader stands in for *document.Service. It records the query it was
// handed so the tests can assert the handler passes the caller's question and
// limit through UNCHANGED — clamping is core/document's job, and a transport
// that pre-clamped would be owning a bound it does not own.
type fakeDocReader struct {
	got    document.ExcerptQuery
	result document.ExcerptResult
	err    error
}

func (f *fakeDocReader) Excerpts(_ context.Context, q document.ExcerptQuery) (document.ExcerptResult, error) {
	f.got = q
	if f.err != nil {
		return document.ExcerptResult{}, f.err
	}
	return f.result, nil
}

func passagesHandler(t *testing.T, docs *fakeDocReader) *connectapi.TenderHandler {
	t.Helper()
	svc := tender.NewService(&fakeRepo{}, fakeKB{}, fakeRL{}, fakeProfileSource{}, tender.Config{
		AnonTier:      tender.Tier{MaxResults: 10, RateLimit: 30, RateWindow: 5 * time.Minute},
		AuthedTier:    tender.Tier{MaxResults: 50, RateLimit: 300, RateWindow: 5 * time.Minute},
		GetTenderTier: tender.Tier{MaxResults: 20, RateLimit: 600, RateWindow: time.Minute},
	})
	return connectapi.NewTenderHandler(svc, newFakeMemberRepo(), docs)
}

func TestGetTenderPassages_RequiresAuth(t *testing.T) {
	h := passagesHandler(t, &fakeDocReader{})
	_, err := h.GetTenderPassages(context.Background(),
		connect.NewRequest(&tenderv1.GetTenderPassagesRequest{Id: "1"}))
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeUnauthenticated {
		t.Errorf("error = %v, want Unauthenticated — retrieval is a metered read", err)
	}
}

func TestGetTenderPassages_RejectsANonNumericID(t *testing.T) {
	h := passagesHandler(t, &fakeDocReader{})
	ctx := connectapi.ContextWithUserID(context.Background(), "u1")
	_, err := h.GetTenderPassages(ctx,
		connect.NewRequest(&tenderv1.GetTenderPassagesRequest{Id: "not-a-number"}))
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) || connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("error = %v, want InvalidArgument", err)
	}
}

// TestGetTenderPassages_AnswersAvailabilityWithoutAQuestion pins the pairing the
// whole RPC exists for: an empty question is availability alone, and the cause
// travels with it. A client that learned "zero passages" without learning why
// cannot tell "nothing matched" from "we hold nothing but the notice".
func TestGetTenderPassages_AnswersAvailabilityWithoutAQuestion(t *testing.T) {
	docs := &fakeDocReader{result: document.ExcerptResult{Availability: document.Availability{
		TenderID:           7,
		Coverage:           document.CoverageNotice,
		Reason:             document.ReasonBodyNotRetrieved,
		NoticeRead:         true,
		KnownDocumentLinks: 3,
	}}}
	h := passagesHandler(t, docs)
	ctx := connectapi.ContextWithUserID(context.Background(), "u1")

	res, err := h.GetTenderPassages(ctx,
		connect.NewRequest(&tenderv1.GetTenderPassagesRequest{Id: "7", Question: "", Limit: 0}))
	if err != nil {
		t.Fatalf("GetTenderPassages: %v", err)
	}
	if docs.got.TenderID != 7 || docs.got.Question != "" || docs.got.Limit != 0 {
		t.Errorf("query = %+v, want the caller's id/question/limit passed through unchanged", docs.got)
	}
	av := res.Msg.Availability
	if av == nil {
		t.Fatal("availability is nil — an empty passage list with no cause is the conflation this RPC exists to prevent")
	}
	if av.Coverage != "notice_only" || av.Reason != "body_not_retrieved" {
		t.Errorf("availability = %+v, want notice_only/body_not_retrieved", av)
	}
	if av.KnownDocumentLinks != 3 || !av.NoticeRead {
		t.Errorf("availability evidence = %+v, want the counts that make the answer auditable", av)
	}
	if len(res.Msg.Passages) != 0 {
		t.Errorf("passages = %+v, want none for an empty question", res.Msg.Passages)
	}
}

// TestGetTenderPassages_KeepsAbsentPageBoundsAbsent is the one mapping detail
// that cannot be got wrong quietly: page 0 and "we never learned the page" are
// both 0 on the wire, and rendering "p. 0" under a verbatim legal quote would
// undermine the affordance the citation exists for.
func TestGetTenderPassages_KeepsAbsentPageBoundsAbsent(t *testing.T) {
	page := 14
	docs := &fakeDocReader{result: document.ExcerptResult{
		Availability: document.Availability{Coverage: document.CoverageFull},
		Excerpts: []document.Excerpt{
			{Text: "con pagina", Truncated: true, Citation: document.Citation{
				DocumentURL: "https://x/disciplinare.pdf", DocumentType: "spec",
				PartIndex: 4, PageStart: &page, SectionPath: []string{"7", "7.2"},
			}},
			{Text: "senza pagina", Citation: document.Citation{DocumentURL: "https://x/notice.pdf"}},
		},
	}}
	h := passagesHandler(t, docs)
	ctx := connectapi.ContextWithUserID(context.Background(), "u1")

	res, err := h.GetTenderPassages(ctx,
		connect.NewRequest(&tenderv1.GetTenderPassagesRequest{Id: "7", Question: "penali", Limit: 3}))
	if err != nil {
		t.Fatalf("GetTenderPassages: %v", err)
	}
	if docs.got.Question != "penali" || docs.got.Limit != 3 {
		t.Errorf("query = %+v, want the question and limit passed through", docs.got)
	}
	if len(res.Msg.Passages) != 2 {
		t.Fatalf("len(passages) = %d, want 2", len(res.Msg.Passages))
	}

	withPage := res.Msg.Passages[0]
	if !withPage.Truncated {
		t.Error("truncated = false, want true — a passage that stops mid-clause must say so")
	}
	if c := withPage.Citation; c == nil || !c.PageStartSet || c.PageStart != 14 {
		t.Errorf("citation = %+v, want page 14 marked present", c)
	}
	if c := withPage.Citation; c.PageEndSet {
		t.Error("page_end_set = true, want false — only page_start was published")
	}
	if got := withPage.Citation.SectionPath; len(got) != 2 || got[1] != "7.2" {
		t.Errorf("section_path = %v, want the hierarchical numbering", got)
	}

	bare := res.Msg.Passages[1].Citation
	if bare == nil || bare.PageStartSet || bare.PageEndSet {
		t.Errorf("citation = %+v, want both page bounds absent — a renderer must degrade to the URL alone", bare)
	}
}

// TestGetTenderDetail_CarriesTheAwardGridAndItsThirdState pins the two
// three-valued fields the criteria surface reads. Absent is a state, and a
// notice nobody has read must not report "publishes no usable grid" — that is a
// claim about the buyer.
func TestGetTenderDetail_CarriesTheAwardGridAndItsThirdState(t *testing.T) {
	weight := 70.0
	enriched := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	usable := true
	repo := &fakeRepo{detail: &tender.TenderDetail{
		ID: "5", Title: "Lavori", DocumentsURL: "https://buyer/docs",
		GridUsable: &usable, EnrichedAt: &enriched,
		Criteria: []tender.AwardCriterion{
			{LotRef: "", Ordinal: 1, Type: "quality", Name: "Offerta tecnica", Weight: &weight, WeightRaw: "70", Lang: "ITA"},
			{LotRef: "", Ordinal: 2, Type: "price", Name: "Offerta economica", WeightRaw: "OEPV"},
		},
	}}
	svc := tender.NewService(repo, fakeKB{}, fakeRL{}, fakeProfileSource{}, tender.Config{
		GetTenderTier: tender.Tier{MaxResults: 20, RateLimit: 600, RateWindow: time.Minute},
	})
	h := connectapi.NewTenderHandler(svc, newFakeMemberRepo(), &fakeDocReader{})

	res, err := h.GetTender(context.Background(), connect.NewRequest(&tenderv1.GetTenderRequest{Id: "5"}))
	if err != nil {
		t.Fatalf("GetTender: %v", err)
	}
	got := res.Msg.Tender
	if got.DocumentsUrl != "https://buyer/docs" {
		t.Errorf("documents_url = %q — it is the only affordance a body_not_retrieved coverage can offer", got.DocumentsUrl)
	}
	if !got.GridUsableSet || !got.GridUsable {
		t.Errorf("grid_usable = %v/%v, want true marked present", got.GridUsable, got.GridUsableSet)
	}
	if got.EnrichedAt == "" {
		t.Error("enriched_at is empty — the client cannot date an answer it was never given")
	}
	if len(got.Criteria) != 2 {
		t.Fatalf("len(criteria) = %d, want 2", len(got.Criteria))
	}
	if !got.Criteria[0].WeightSet || got.Criteria[0].Weight != 70 {
		t.Errorf("criteria[0] weight = %v/%v, want 70 marked present", got.Criteria[0].Weight, got.Criteria[0].WeightSet)
	}
	if got.Criteria[1].WeightSet {
		t.Error("criteria[1] weight_set = true, want false — an absent weight is not a zero weight")
	}
	if got.Criteria[1].WeightRaw != "OEPV" {
		t.Errorf("criteria[1] weight_raw = %q, want the published text kept even when it does not parse", got.Criteria[1].WeightRaw)
	}
}
