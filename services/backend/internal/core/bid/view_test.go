package bid

import (
	"context"
	"errors"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

// viewFakeRepo satisfies bid.Repository for the read/aggregation paths; the
// mutating methods are unused stubs here.
type viewFakeRepo struct {
	bids  []Bid
	byID  map[string]Bid
	items map[string][]ChecklistItem
}

func (f *viewFakeRepo) CreateBid(context.Context, Bid) (Bid, error) { return Bid{}, nil }
func (f *viewFakeRepo) FindBidByID(_ context.Context, _, bidID string) (Bid, error) {
	b, ok := f.byID[bidID]
	if !ok {
		return Bid{}, ErrBidNotFound
	}
	return b, nil
}
func (f *viewFakeRepo) ListBidsByWorkbench(_ context.Context, _ string) ([]Bid, error) {
	return f.bids, nil
}
func (f *viewFakeRepo) UpdateGoNoGo(context.Context, string, GoNoGo, DecisionRecord) (Bid, error) {
	return Bid{}, nil
}
func (f *viewFakeRepo) UpdateStage(context.Context, string, Stage) (Bid, error) { return Bid{}, nil }
func (f *viewFakeRepo) UpdateOutcome(context.Context, string, Outcome) (Bid, error) {
	return Bid{}, nil
}
func (f *viewFakeRepo) DeleteBid(context.Context, string) error { return nil }
func (f *viewFakeRepo) SeedChecklist(context.Context, string, []ChecklistItemSeed) error {
	return nil
}
func (f *viewFakeRepo) ListChecklistItems(_ context.Context, bidID string) ([]ChecklistItem, error) {
	return f.items[bidID], nil
}
func (f *viewFakeRepo) UpsertChecklistItem(context.Context, string, string, string, string) (ChecklistItem, error) {
	return ChecklistItem{}, nil
}

type viewFakeAccess struct {
	accessErr   error
	manageErr   error
	workspaceID string
}

func (f *viewFakeAccess) CanAccessWorkbench(context.Context, string, string) error {
	return f.accessErr
}
func (f *viewFakeAccess) CanManageWorkbench(context.Context, string, string) error {
	return f.manageErr
}
func (f *viewFakeAccess) WorkspaceOf(context.Context, string) (string, error) {
	return f.workspaceID, nil
}

// viewFakeEligibility is a never-called stand-in: the read paths under test in
// this file never record a decision, so no assessment is ever requested.
type viewFakeEligibility struct{}

func (viewFakeEligibility) CheckEligibility(context.Context, string, string, int64, string) (company.Assessment, error) {
	return company.Assessment{}, nil
}

type viewFakeTenders struct {
	summaries map[int64]tender.TenderSummary
	fits      map[int64]tender.TenderFitResult
}

func (f *viewFakeTenders) SummariesByIDs(_ context.Context, ids []int64) (map[int64]tender.TenderSummary, error) {
	out := make(map[int64]tender.TenderSummary, len(ids))
	for _, id := range ids {
		if s, ok := f.summaries[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}
func (f *viewFakeTenders) FitForTenders(_ context.Context, _, _ string, ids []int64) (map[int64]tender.TenderFitResult, error) {
	out := make(map[int64]tender.TenderFitResult, len(ids))
	for _, id := range ids {
		if r, ok := f.fits[id]; ok {
			out[id] = r
		}
	}
	return out, nil
}

func TestListBids_Aggregates(t *testing.T) {
	repo := &viewFakeRepo{
		bids: []Bid{
			{ID: "bid-10", WorkbenchID: "wb1", TenderID: 10, GoNoGo: GoNoGoUndecided, Stage: StageShortlisted},
			{ID: "bid-20", WorkbenchID: "wb1", TenderID: 20, GoNoGo: GoNoGoUndecided, Stage: StageShortlisted},
		},
		items: map[string][]ChecklistItem{
			"bid-10": {
				{ID: "i1", BidID: "bid-10", ItemCode: "a", Status: "done"},
				{ID: "i2", BidID: "bid-10", ItemCode: "b", Status: "pending"},
			},
			"bid-20": {},
		},
	}
	access := &viewFakeAccess{workspaceID: "ws1"}
	tenders := &viewFakeTenders{
		summaries: map[int64]tender.TenderSummary{
			10: {ID: 10, Title: "Road works", BuyerName: "City", Country: "IT", CPV: "45000000", Status: "open"},
		},
		fits: map[int64]tender.TenderFitResult{
			10: {Tier: tender.FitStrong, HasProfile: true, Available: true},
		},
	}
	svc := NewService(repo, access, tenders, &viewFakeEligibility{})

	views, err := svc.ListBids(context.Background(), "u1", "wb1")
	if err != nil {
		t.Fatalf("ListBids: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("len(views) = %d, want 2", len(views))
	}

	v10 := views[0]
	if !v10.TenderAvailable || v10.Summary.Title != "Road works" || v10.Fit.Tier != tender.FitStrong {
		t.Fatalf("bid-10 view = %+v, want available strong 'Road works'", v10)
	}
	if v10.ChecklistDone != 1 || v10.ChecklistTotal != 2 {
		t.Fatalf("bid-10 checklist = %d/%d, want 1/2", v10.ChecklistDone, v10.ChecklistTotal)
	}

	v20 := views[1]
	if v20.TenderAvailable {
		t.Fatal("bid-20 tender is dangling -> TenderAvailable must be false")
	}
	if v20.Summary.Title != "" || v20.Fit.Tier != "" {
		t.Fatalf("dangling bid must carry empty summary/fit, got %+v / %+v", v20.Summary, v20.Fit)
	}
	if v20.ChecklistTotal != 0 {
		t.Fatalf("bid-20 checklist total = %d, want 0", v20.ChecklistTotal)
	}
}

func TestListBids_SortsByDeadlineDanglingLast(t *testing.T) {
	repo := &viewFakeRepo{
		bids: []Bid{
			{ID: "late", WorkbenchID: "wb1", TenderID: 1, Stage: StageShortlisted},
			{ID: "gone", WorkbenchID: "wb1", TenderID: 2, Stage: StageShortlisted},
			{ID: "soon", WorkbenchID: "wb1", TenderID: 3, Stage: StageShortlisted},
		},
		items: map[string][]ChecklistItem{},
	}
	access := &viewFakeAccess{workspaceID: "ws1"}
	tenders := &viewFakeTenders{
		summaries: map[int64]tender.TenderSummary{
			1: {ID: 1, Title: "Late", Deadline: "2026-09-01T00:00:00Z"},
			3: {ID: 3, Title: "Soon", Deadline: "2026-07-15T00:00:00Z"},
			// tender 2 absent -> dangling -> sorts last
		},
		fits: map[int64]tender.TenderFitResult{
			1: {Available: true}, 3: {Available: true},
		},
	}
	svc := NewService(repo, access, tenders, &viewFakeEligibility{})

	views, err := svc.ListBids(context.Background(), "u1", "wb1")
	if err != nil {
		t.Fatalf("ListBids: %v", err)
	}
	order := []string{views[0].Bid.ID, views[1].Bid.ID, views[2].Bid.ID}
	want := []string{"soon", "late", "gone"}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v (deadline asc, dangling last)", order, want)
		}
	}
}

func TestListBids_ReadRequiresAccess(t *testing.T) {
	svc := NewService(&viewFakeRepo{}, &viewFakeAccess{accessErr: workbench.ErrForbidden}, &viewFakeTenders{}, &viewFakeEligibility{})
	if _, err := svc.ListBids(context.Background(), "u1", "wb1"); !errors.Is(err, workbench.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestGetBid_AggregatesSingle(t *testing.T) {
	repo := &viewFakeRepo{
		byID: map[string]Bid{
			"bid-10": {ID: "bid-10", WorkbenchID: "wb1", TenderID: 10, GoNoGo: GoNoGoGo, Stage: StagePreparing},
		},
		items: map[string][]ChecklistItem{
			"bid-10": {
				{ID: "i1", BidID: "bid-10", ItemCode: "a", Status: "na"},
				{ID: "i2", BidID: "bid-10", ItemCode: "b", Status: "done"},
				{ID: "i3", BidID: "bid-10", ItemCode: "c", Status: "pending"},
			},
		},
	}
	access := &viewFakeAccess{workspaceID: "ws1"}
	tenders := &viewFakeTenders{
		summaries: map[int64]tender.TenderSummary{10: {ID: 10, Title: "Bridge"}},
		fits:      map[int64]tender.TenderFitResult{10: {Tier: tender.FitPossible, HasProfile: true, Available: true}},
	}
	svc := NewService(repo, access, tenders, &viewFakeEligibility{})

	v, err := svc.GetBid(context.Background(), "u1", "wb1", "bid-10")
	if err != nil {
		t.Fatalf("GetBid: %v", err)
	}
	if !v.TenderAvailable || v.Summary.Title != "Bridge" || v.Fit.Tier != tender.FitPossible {
		t.Fatalf("view = %+v", v)
	}
	if v.ChecklistDone != 2 || v.ChecklistTotal != 3 {
		t.Fatalf("checklist = %d/%d, want 2/3 (done+na counted)", v.ChecklistDone, v.ChecklistTotal)
	}

	if _, err := svc.GetBid(context.Background(), "u1", "wb1", "missing"); !errors.Is(err, ErrBidNotFound) {
		t.Fatalf("want ErrBidNotFound, got %v", err)
	}
}
