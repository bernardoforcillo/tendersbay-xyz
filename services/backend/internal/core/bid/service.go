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

// SetGoNoGo records the pursue/skip decision. Only "go"/"no_go" are settable.
func (s *Service) SetGoNoGo(ctx context.Context, userID, workbenchID, bidID string, d GoNoGo) (Bid, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Bid{}, err
	}
	if d != GoNoGoGo && d != GoNoGoNoGo {
		return Bid{}, ErrInvalidArgument
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return Bid{}, err
	}
	return s.repo.UpdateGoNoGo(ctx, bidID, d)
}

// AdvanceStage moves the bid one step forward (shortlisted->preparing->submitted).
// A no_go bid cannot advance; there is no step past submitted.
func (s *Service) AdvanceStage(ctx context.Context, userID, workbenchID, bidID string) (Bid, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Bid{}, err
	}
	current, err := s.repo.FindBidByID(ctx, workbenchID, bidID)
	if err != nil {
		return Bid{}, err
	}
	if current.GoNoGo == GoNoGoNoGo {
		return Bid{}, ErrBidNotGo
	}
	var next Stage
	switch current.Stage {
	case StageShortlisted:
		next = StagePreparing
	case StagePreparing:
		next = StageSubmitted
	default:
		return Bid{}, ErrInvalidTransition
	}
	return s.repo.UpdateStage(ctx, bidID, next)
}

// RecordOutcome closes a pursued bid with a terminal result. Requires a prior
// "go" decision; the outcome is kept (non-destructive) for win-rate history.
func (s *Service) RecordOutcome(ctx context.Context, userID, workbenchID, bidID string, o Outcome) (Bid, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Bid{}, err
	}
	if o != OutcomeWon && o != OutcomeLost && o != OutcomeWithdrawn {
		return Bid{}, ErrInvalidArgument
	}
	current, err := s.repo.FindBidByID(ctx, workbenchID, bidID)
	if err != nil {
		return Bid{}, err
	}
	if current.GoNoGo != GoNoGoGo {
		return Bid{}, ErrBidNotGo
	}
	return s.repo.UpdateOutcome(ctx, bidID, o)
}

// RemoveBid hard-deletes a mistaken add, freeing the (workbench_id, tender_id)
// unique for re-adding. The normal close is RecordOutcome, not this.
func (s *Service) RemoveBid(ctx context.Context, userID, workbenchID, bidID string) error {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return err
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return err
	}
	return s.repo.DeleteBid(ctx, bidID)
}

// ListChecklistItems returns a bid's ESPD checklist (a read — shared-workbench
// viewers may see it).
func (s *Service) ListChecklistItems(ctx context.Context, userID, workbenchID, bidID string) ([]ChecklistItem, error) {
	if err := s.access.CanAccessWorkbench(ctx, userID, workbenchID); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return nil, err
	}
	return s.repo.ListChecklistItems(ctx, bidID)
}

// UpsertChecklistAnswer sets one checklist line's status/note (a write).
func (s *Service) UpsertChecklistAnswer(ctx context.Context, userID, workbenchID, bidID, itemCode, status, note string) (ChecklistItem, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return ChecklistItem{}, err
	}
	if itemCode == "" || (status != "pending" && status != "done" && status != "na") {
		return ChecklistItem{}, ErrInvalidArgument
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return ChecklistItem{}, err
	}
	return s.repo.UpsertChecklistItem(ctx, bidID, itemCode, status, note)
}
