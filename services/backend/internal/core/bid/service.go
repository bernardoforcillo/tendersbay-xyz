package bid

import "context"

// Service owns the bid lifecycle and the ESPD checklist. It authorizes every
// call against the workbench RBAC layer, then delegates persistence to its
// Repository — no business rules leak into the transport or adapter layers.
type Service struct {
	repo      Repository
	access    WorkbenchAccess
	fit       TenderFit
	summaries TenderSummaries
}

// NewService wires the Service. fit and summaries are BOTH taken from the single
// tenders arg (*tender.Service satisfies both ports).
func NewService(repo Repository, access WorkbenchAccess, tenders Tenders) *Service {
	return &Service{repo: repo, access: access, fit: tenders, summaries: tenders}
}

// AddBid tracks tenderID as a new bid in workbenchID: authorize, confirm the
// tender exists, insert with lifecycle defaults, then seed the checklist.
func (s *Service) AddBid(ctx context.Context, userID, workbenchID string, tenderID int64) (Bid, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Bid{}, err
	}
	summaries, err := s.summaries.SummariesByIDs(ctx, []int64{tenderID})
	if err != nil {
		return Bid{}, err
	}
	summary, ok := summaries[tenderID]
	if !ok {
		return Bid{}, ErrInvalidArgument // no such tender to track
	}
	created, err := s.repo.CreateBid(ctx, Bid{
		WorkbenchID: workbenchID,
		TenderID:    tenderID,
		GoNoGo:      GoNoGoUndecided,
		Stage:       StageShortlisted,
		Outcome:     "",
		CreatedBy:   userID,
	})
	if err != nil {
		return Bid{}, err // ErrBidExists propagates from the unique violation
	}
	if err := s.repo.SeedChecklist(ctx, created.ID, checklistTemplate("", summary.CPV)); err != nil {
		return Bid{}, err
	}
	return created, nil
}
