package bid

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// Service owns the bid lifecycle and the ESPD checklist. It authorizes every
// call against the workbench RBAC layer, then delegates persistence to its
// Repository — no business rules leak into the transport or adapter layers.
type Service struct {
	repo        Repository
	access      WorkbenchAccess
	fit         TenderFit
	summaries   TenderSummaries
	eligibility Eligibility

	// now is injected so the decision timestamp is reproducible in tests. The
	// lifecycle's other timestamps are written by the database (created_at /
	// updated_at defaults); this one is a domain fact about when a human
	// decided, so the domain owns it.
	now func() time.Time
}

// NewService wires the Service. fit and summaries are BOTH taken from the single
// tenders arg (*tender.Service satisfies both ports); eligibility is
// *company.Service.
func NewService(repo Repository, access WorkbenchAccess, tenders Tenders, eligibility Eligibility) *Service {
	return &Service{
		repo:        repo,
		access:      access,
		fit:         tenders,
		summaries:   tenders,
		eligibility: eligibility,
		now:         time.Now,
	}
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

// SetGoNoGo records the pursue/skip decision together with the eligibility
// recommendation it was taken against. Only "go"/"no_go" are settable.
//
// The recommendation NEVER gates the decision. A user may record a go on a
// no_go recommendation and the call succeeds — they carry the liability, they
// decide, and an override that the product refuses to represent is an override
// the product cannot learn from.
func (s *Service) SetGoNoGo(ctx context.Context, userID, workbenchID, bidID string, d GoNoGo) (Bid, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Bid{}, err
	}
	if d != GoNoGoGo && d != GoNoGoNoGo {
		return Bid{}, ErrInvalidArgument
	}
	current, err := s.repo.FindBidByID(ctx, workbenchID, bidID)
	if err != nil {
		return Bid{}, err
	}
	return s.repo.UpdateGoNoGo(ctx, bidID, d, s.recommendationFor(ctx, userID, workbenchID, current, d))
}

// recommendationFor resolves the eligibility recommendation this decision is
// being taken against.
//
// It never returns an error, and that is deliberate: a decision the user has
// already made must be recordable even when we cannot say what we would have
// advised. Failing the write instead would turn a missing assessment into an
// outage in the lifecycle, and it would do so precisely in the cases where the
// engine is weakest — a tender we hold no documents for. An unresolvable check
// records Recommendation "" ("no recommendation existed"), which is a distinct,
// queryable fact rather than a silent zero pretending to be agreement.
//
// The check is notice-level (lotRef ""): a bid tracks a tender, not a lot, so
// the requirements that apply are the notice's own — which is also the set a
// multi-lot notice states once and every lot inherits.
func (s *Service) recommendationFor(ctx context.Context, userID, workbenchID string, current Bid, d GoNoGo) DecisionRecord {
	at := s.now()
	rec := DecisionRecord{RecordedAt: &at}

	workspaceID, err := s.access.WorkspaceOf(ctx, workbenchID)
	if err != nil {
		slog.WarnContext(ctx, "bid decision recorded without an eligibility recommendation",
			"stage", "workspace_lookup", "error", err)
		return rec
	}
	a, err := s.eligibility.CheckEligibility(ctx, userID, workspaceID, current.TenderID, "")
	if err != nil {
		slog.WarnContext(ctx, "bid decision recorded without an eligibility recommendation",
			"stage", "eligibility_check", "error", err)
		return rec
	}

	rec.Recommendation = a.Verdict
	rec.BlockingGapCount = a.BlockingGapCount
	rec.Overridden = Overrides(a.Verdict, d)

	// Server-side product metric — the trust signal. slog → OTLP, since Go
	// carries no product-analytics SDK, and deliberately no workspace id, no
	// tender id and no company identifier travels on this line.
	slog.InfoContext(ctx, "bid decision recorded",
		"decision", string(d), "recommendation", string(a.Verdict), "overridden", rec.Overridden,
		"blocking_gaps", a.BlockingGapCount, "unknown_gaps", a.UnknownGapCount,
		"requirement_count", a.RequirementCount,
		"authoritative_requirements", a.AuthoritativeRequirements,
		"document_coverage", string(a.DocumentCoverage))
	return rec
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

// ── ESPD per-bid data ───────────────────────────────────────────────────────

// ListEspdData returns the lots, subcontractors and reliances declared for a
// bid (a read — shared-workbench viewers may see it).
func (s *Service) ListEspdData(ctx context.Context, userID, workbenchID, bidID string) (EspdData, error) {
	if err := s.access.CanAccessWorkbench(ctx, userID, workbenchID); err != nil {
		return EspdData{}, err
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return EspdData{}, err
	}
	return s.repo.ListEspdData(ctx, bidID)
}

// PutLot records a lot this bid tenders for, upserting on its reference.
func (s *Service) PutLot(ctx context.Context, userID, workbenchID, bidID string, l Lot) (Lot, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Lot{}, err
	}
	l.LotRef = strings.TrimSpace(l.LotRef)
	if err := l.validate(); err != nil {
		return Lot{}, err
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return Lot{}, err
	}
	return s.repo.PutLot(ctx, bidID, l)
}

// DeleteLot removes one lot.
func (s *Service) DeleteLot(ctx context.Context, userID, workbenchID, bidID, id string) error {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return ErrInvalidArgument
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return err
	}
	return s.repo.DeleteLot(ctx, bidID, id)
}

// PutSubcontractor records a Part II.D entry, upserting on the VAT number.
func (s *Service) PutSubcontractor(ctx context.Context, userID, workbenchID, bidID string, sub Subcontractor) (Subcontractor, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Subcontractor{}, err
	}
	sub.Name, sub.VAT = strings.TrimSpace(sub.Name), strings.TrimSpace(sub.VAT)
	sub.Country = strings.ToUpper(strings.TrimSpace(sub.Country))
	if err := sub.validate(); err != nil {
		return Subcontractor{}, err
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return Subcontractor{}, err
	}
	return s.repo.PutSubcontractor(ctx, bidID, sub)
}

// DeleteSubcontractor removes one Part II.D entry.
func (s *Service) DeleteSubcontractor(ctx context.Context, userID, workbenchID, bidID, id string) error {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return ErrInvalidArgument
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return err
	}
	return s.repo.DeleteSubcontractor(ctx, bidID, id)
}

// PutReliance records a Part II.C avvalimento, upserting on (vat, criterion).
func (s *Service) PutReliance(ctx context.Context, userID, workbenchID, bidID string, r Reliance) (Reliance, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Reliance{}, err
	}
	r.EntityName, r.VAT, r.Criterion = strings.TrimSpace(r.EntityName), strings.TrimSpace(r.VAT), strings.TrimSpace(r.Criterion)
	if err := r.validate(); err != nil {
		return Reliance{}, err
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return Reliance{}, err
	}
	return s.repo.PutReliance(ctx, bidID, r)
}

// DeleteReliance removes one Part II.C entry.
func (s *Service) DeleteReliance(ctx context.Context, userID, workbenchID, bidID, id string) error {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return ErrInvalidArgument
	}
	if _, err := s.repo.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return err
	}
	return s.repo.DeleteReliance(ctx, bidID, id)
}
