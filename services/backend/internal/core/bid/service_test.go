package bid

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

// ── fakeRepo ──────────────────────────────────────────────────────────────────

type fakeRepo struct {
	bids      map[string]Bid
	checklist map[string][]ChecklistItem
	seq       int
	createErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{bids: map[string]Bid{}, checklist: map[string][]ChecklistItem{}}
}

var _ Repository = (*fakeRepo)(nil)

func (r *fakeRepo) CreateBid(_ context.Context, b Bid) (Bid, error) {
	if r.createErr != nil {
		return Bid{}, r.createErr
	}
	for _, e := range r.bids { // simulate the (workbench_id, tender_id) unique
		if e.WorkbenchID == b.WorkbenchID && e.TenderID == b.TenderID {
			return Bid{}, ErrBidExists
		}
	}
	r.seq++
	b.ID = fmt.Sprintf("bid-%d", r.seq)
	now := time.Now()
	b.CreatedAt, b.UpdatedAt = now, now
	r.bids[b.ID] = b
	return b, nil
}

func (r *fakeRepo) FindBidByID(_ context.Context, workbenchID, bidID string) (Bid, error) {
	b, ok := r.bids[bidID]
	if !ok || b.WorkbenchID != workbenchID { // workbench-scoped: guards cross-workbench access
		return Bid{}, ErrBidNotFound
	}
	return b, nil
}

func (r *fakeRepo) ListBidsByWorkbench(_ context.Context, workbenchID string) ([]Bid, error) {
	var out []Bid
	for _, b := range r.bids {
		if b.WorkbenchID == workbenchID {
			out = append(out, b)
		}
	}
	return out, nil
}

func (r *fakeRepo) UpdateGoNoGo(_ context.Context, bidID string, d GoNoGo) (Bid, error) {
	b, ok := r.bids[bidID]
	if !ok {
		return Bid{}, ErrBidNotFound
	}
	b.GoNoGo, b.UpdatedAt = d, time.Now()
	r.bids[bidID] = b
	return b, nil
}

func (r *fakeRepo) UpdateStage(_ context.Context, bidID string, s Stage) (Bid, error) {
	b, ok := r.bids[bidID]
	if !ok {
		return Bid{}, ErrBidNotFound
	}
	b.Stage, b.UpdatedAt = s, time.Now()
	r.bids[bidID] = b
	return b, nil
}

func (r *fakeRepo) UpdateOutcome(_ context.Context, bidID string, o Outcome) (Bid, error) {
	b, ok := r.bids[bidID]
	if !ok {
		return Bid{}, ErrBidNotFound
	}
	b.Outcome, b.UpdatedAt = o, time.Now()
	r.bids[bidID] = b
	return b, nil
}

func (r *fakeRepo) DeleteBid(_ context.Context, bidID string) error {
	if _, ok := r.bids[bidID]; !ok {
		return ErrBidNotFound
	}
	delete(r.bids, bidID)
	delete(r.checklist, bidID)
	return nil
}

func (r *fakeRepo) SeedChecklist(_ context.Context, bidID string, seeds []ChecklistItemSeed) error {
	items := make([]ChecklistItem, len(seeds))
	for i, s := range seeds {
		items[i] = ChecklistItem{
			ID:          fmt.Sprintf("%s-%s", bidID, s.ItemCode),
			BidID:       bidID,
			SectionCode: s.SectionCode,
			ItemCode:    s.ItemCode,
			Status:      "pending",
			Required:    s.Required,
			Position:    s.Position,
		}
	}
	r.checklist[bidID] = items
	return nil
}

func (r *fakeRepo) ListChecklistItems(_ context.Context, bidID string) ([]ChecklistItem, error) {
	return r.checklist[bidID], nil
}

func (r *fakeRepo) UpsertChecklistItem(_ context.Context, bidID, itemCode, status, note string) (ChecklistItem, error) {
	items := r.checklist[bidID]
	for i, it := range items {
		if it.ItemCode == itemCode {
			it.Status, it.Note, it.UpdatedAt = status, note, time.Now()
			items[i] = it
			r.checklist[bidID] = items
			return it, nil
		}
	}
	it := ChecklistItem{ID: fmt.Sprintf("%s-%s", bidID, itemCode), BidID: bidID, ItemCode: itemCode, Status: status, Note: note}
	r.checklist[bidID] = append(items, it)
	return it, nil
}

// ── fakeAccess ────────────────────────────────────────────────────────────────

type fakeAccess struct {
	accessErr map[string]error  // workbenchID -> err for CanAccessWorkbench (nil = allowed)
	manageErr map[string]error  // workbenchID -> err for CanManageWorkbench (nil = allowed)
	workspace map[string]string // workbenchID -> workspaceID for WorkspaceOf
}

func newFakeAccess() *fakeAccess {
	return &fakeAccess{accessErr: map[string]error{}, manageErr: map[string]error{}, workspace: map[string]string{}}
}

var _ WorkbenchAccess = (*fakeAccess)(nil)

func (a *fakeAccess) CanAccessWorkbench(_ context.Context, _, workbenchID string) error {
	return a.accessErr[workbenchID]
}

func (a *fakeAccess) CanManageWorkbench(_ context.Context, _, workbenchID string) error {
	return a.manageErr[workbenchID]
}

func (a *fakeAccess) WorkspaceOf(_ context.Context, workbenchID string) (string, error) {
	return a.workspace[workbenchID], nil
}

// ── fakeTenders ───────────────────────────────────────────────────────────────

type fakeTenders struct {
	summaries map[int64]tender.TenderSummary
	fits      map[int64]tender.TenderFitResult
}

func newFakeTenders() *fakeTenders {
	return &fakeTenders{summaries: map[int64]tender.TenderSummary{}, fits: map[int64]tender.TenderFitResult{}}
}

var _ Tenders = (*fakeTenders)(nil)

func (t *fakeTenders) SummariesByIDs(_ context.Context, ids []int64) (map[int64]tender.TenderSummary, error) {
	out := map[int64]tender.TenderSummary{}
	for _, id := range ids {
		if s, ok := t.summaries[id]; ok {
			out[id] = s
		}
	}
	return out, nil
}

func (t *fakeTenders) FitForTenders(_ context.Context, _, _ string, ids []int64) (map[int64]tender.TenderFitResult, error) {
	out := map[int64]tender.TenderFitResult{}
	for _, id := range ids {
		if f, ok := t.fits[id]; ok {
			out[id] = f
		}
	}
	return out, nil
}

// newBidTestService wires a Service over fresh fakes and returns all four.
func newBidTestService() (*Service, *fakeRepo, *fakeAccess, *fakeTenders) {
	repo := newFakeRepo()
	access := newFakeAccess()
	tenders := newFakeTenders()
	return NewService(repo, access, tenders), repo, access, tenders
}

func TestNewService_WiresPorts(t *testing.T) {
	svc, repo, access, tenders := newBidTestService()
	if svc.repo != Repository(repo) {
		t.Fatal("repo not wired")
	}
	if svc.access != WorkbenchAccess(access) {
		t.Fatal("access not wired")
	}
	// fit AND summaries must both come from the single tenders arg.
	if svc.fit != TenderFit(tenders) {
		t.Fatal("fit not wired from tenders")
	}
	if svc.summaries != TenderSummaries(tenders) {
		t.Fatal("summaries not wired from tenders")
	}
}

func TestAddBid_Defaults(t *testing.T) {
	svc, repo, _, tenders := newBidTestService()
	tenders.summaries[42] = tender.TenderSummary{ID: 42, Title: "Road works", CPV: "45000000"}
	b, err := svc.AddBid(context.Background(), "u1", "wb1", 42)
	if err != nil {
		t.Fatalf("AddBid: %v", err)
	}
	if b.GoNoGo != GoNoGoUndecided || b.Stage != StageShortlisted || b.Outcome != "" {
		t.Fatalf("defaults wrong: %+v", b)
	}
	if b.WorkbenchID != "wb1" || b.TenderID != 42 || b.CreatedBy != "u1" {
		t.Fatalf("fields wrong: %+v", b)
	}
	if len(repo.checklist[b.ID]) == 0 {
		t.Fatal("AddBid must seed the checklist")
	}
}

func TestAddBid_Duplicate(t *testing.T) {
	svc, _, _, tenders := newBidTestService()
	tenders.summaries[42] = tender.TenderSummary{ID: 42}
	if _, err := svc.AddBid(context.Background(), "u1", "wb1", 42); err != nil {
		t.Fatalf("first AddBid: %v", err)
	}
	if _, err := svc.AddBid(context.Background(), "u1", "wb1", 42); !errors.Is(err, ErrBidExists) {
		t.Fatalf("want ErrBidExists, got %v", err)
	}
}

func TestAddBid_Forbidden(t *testing.T) {
	svc, _, access, tenders := newBidTestService()
	tenders.summaries[42] = tender.TenderSummary{ID: 42}
	access.manageErr["wb1"] = workbench.ErrForbidden
	if _, err := svc.AddBid(context.Background(), "u1", "wb1", 42); !errors.Is(err, workbench.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestAddBid_UnknownTender(t *testing.T) {
	svc, _, _, _ := newBidTestService()
	if _, err := svc.AddBid(context.Background(), "u1", "wb1", 999); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("want ErrInvalidArgument, got %v", err)
	}
}

func TestSetGoNoGo_Records(t *testing.T) {
	svc, repo, _, _ := newBidTestService()
	repo.bids["b1"] = Bid{ID: "b1", WorkbenchID: "wb1", TenderID: 1, GoNoGo: GoNoGoUndecided, Stage: StageShortlisted}
	b, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", "b1", GoNoGoGo)
	if err != nil || b.GoNoGo != GoNoGoGo {
		t.Fatalf("set go: gng=%q err=%v", b.GoNoGo, err)
	}
}

func TestSetGoNoGo_InvalidValue(t *testing.T) {
	svc, repo, _, _ := newBidTestService()
	repo.bids["b1"] = Bid{ID: "b1", WorkbenchID: "wb1"}
	for _, d := range []GoNoGo{GoNoGoUndecided, "banana"} {
		if _, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", "b1", d); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("value %q: want ErrInvalidArgument, got %v", d, err)
		}
	}
}

func TestSetGoNoGo_Forbidden(t *testing.T) {
	svc, repo, access, _ := newBidTestService()
	repo.bids["b1"] = Bid{ID: "b1", WorkbenchID: "wb1"}
	access.manageErr["wb1"] = workbench.ErrForbidden
	if _, err := svc.SetGoNoGo(context.Background(), "u1", "wb1", "b1", GoNoGoGo); !errors.Is(err, workbench.ErrForbidden) {
		t.Fatalf("want ErrForbidden, got %v", err)
	}
}

func TestSetGoNoGo_WrongWorkbench(t *testing.T) {
	svc, repo, _, _ := newBidTestService()
	repo.bids["b1"] = Bid{ID: "b1", WorkbenchID: "wb1"}
	// Manage on wb2 is allowed (default nil), but b1 belongs to wb1 -> not found.
	if _, err := svc.SetGoNoGo(context.Background(), "u1", "wb2", "b1", GoNoGoGo); !errors.Is(err, ErrBidNotFound) {
		t.Fatalf("want ErrBidNotFound, got %v", err)
	}
}
