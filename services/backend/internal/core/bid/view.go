package bid

import (
	"context"
	"sort"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// ListBids returns every bid in a workbench as an aggregated BidView: the bid
// row, a fresh tender summary + fit (both batched — one SummariesByIDs and one
// FitForTenders for the whole portfolio), and checklist progress. Read access
// only. Returns an empty slice (never nil) for an empty portfolio.
func (s *Service) ListBids(ctx context.Context, userID, workbenchID string) ([]BidView, error) {
	if err := s.access.CanAccessWorkbench(ctx, userID, workbenchID); err != nil {
		return nil, err
	}
	bids, err := s.repo.ListBidsByWorkbench(ctx, workbenchID)
	if err != nil {
		return nil, err
	}
	if len(bids) == 0 {
		return []BidView{}, nil
	}
	workspaceID, err := s.access.WorkspaceOf(ctx, workbenchID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(bids))
	for i, b := range bids {
		ids[i] = b.TenderID
	}
	summaries, err := s.summaries.SummariesByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	fits, err := s.fit.FitForTenders(ctx, userID, workspaceID, ids)
	if err != nil {
		return nil, err
	}
	views := make([]BidView, len(bids))
	for i, b := range bids {
		v, err := s.buildView(ctx, b, summaries, fits)
		if err != nil {
			return nil, err
		}
		views[i] = v
	}
	// Spec §5/§6: order by live tender deadline ascending, with dangling tenders
	// and bids whose tender has no deadline last. A STABLE sort preserves the
	// repo's created_at order among those trailing "no deadline" entries.
	sort.SliceStable(views, func(i, j int) bool {
		di, dj := deadlineSortKey(views[i]), deadlineSortKey(views[j])
		switch {
		case di == "" && dj == "":
			return false // both undated/dangling — preserve input order
		case di == "":
			return false // i undated -> sorts after j
		case dj == "":
			return true // j undated -> i sorts before j
		default:
			return di < dj // RFC3339 sorts lexicographically = chronologically
		}
	})
	return views, nil
}

// deadlineSortKey returns a view's live tender deadline for sorting, or "" when
// the tender is unavailable (dangling) or carries no deadline — both sort last.
func deadlineSortKey(v BidView) string {
	if !v.TenderAvailable {
		return ""
	}
	return v.Summary.Deadline
}

// GetBid returns one bid as an aggregated BidView. Read access only.
func (s *Service) GetBid(ctx context.Context, userID, workbenchID, bidID string) (BidView, error) {
	if err := s.access.CanAccessWorkbench(ctx, userID, workbenchID); err != nil {
		return BidView{}, err
	}
	b, err := s.repo.FindBidByID(ctx, workbenchID, bidID)
	if err != nil {
		return BidView{}, err
	}
	workspaceID, err := s.access.WorkspaceOf(ctx, workbenchID)
	if err != nil {
		return BidView{}, err
	}
	ids := []int64{b.TenderID}
	summaries, err := s.summaries.SummariesByIDs(ctx, ids)
	if err != nil {
		return BidView{}, err
	}
	fits, err := s.fit.FitForTenders(ctx, userID, workspaceID, ids)
	if err != nil {
		return BidView{}, err
	}
	return s.buildView(ctx, b, summaries, fits)
}

// buildView assembles one BidView from a bid, the batched summary/fit maps, and
// the bid's checklist. A checklist item counts as done when its status is
// "done" or "na". When the tender no longer resolves (dangling), the view is
// marked TenderAvailable=false and Summary/Fit are left zero-valued — the
// user's checklist and outcome are still preserved.
func (s *Service) buildView(ctx context.Context, b Bid, summaries map[int64]tender.TenderSummary, fits map[int64]tender.TenderFitResult) (BidView, error) {
	items, err := s.repo.ListChecklistItems(ctx, b.ID)
	if err != nil {
		return BidView{}, err
	}
	done := 0
	for _, it := range items {
		if it.Status == "done" || it.Status == "na" {
			done++
		}
	}
	view := BidView{Bid: b, ChecklistDone: done, ChecklistTotal: len(items)}
	if summary, available := summaries[b.TenderID]; available {
		view.TenderAvailable = true
		view.Summary = summary
		view.Fit = fits[b.TenderID]
	}
	return view, nil
}
