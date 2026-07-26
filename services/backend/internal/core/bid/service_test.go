package bid

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
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
