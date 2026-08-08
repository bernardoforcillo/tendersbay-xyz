package bid

import (
	"context"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// WorkbenchAccess is the workbench RBAC slice the bid domain needs. Satisfied
// by *workbench.Service (CanAccessWorkbench exists; CanManageWorkbench +
// WorkspaceOf are added by the workbench-additions part).
type WorkbenchAccess interface {
	CanAccessWorkbench(ctx context.Context, userID, workbenchID string) error
	CanManageWorkbench(ctx context.Context, userID, workbenchID string) error
	WorkspaceOf(ctx context.Context, workbenchID string) (string, error)
}

// TenderFit computes fresh fit for a batch of tenders. Satisfied by
// *tender.Service (FitForTenders, tender-additions part).
type TenderFit interface {
	FitForTenders(ctx context.Context, userID, workspaceID string, tenderIDs []int64) (map[int64]tender.TenderFitResult, error)
}

// TenderSummaries batches tender card data and reports per-id found/not-found
// (the dangling-tender case). Satisfied by *tender.Service (SummariesByIDs).
type TenderSummaries interface {
	SummariesByIDs(ctx context.Context, tenderIDs []int64) (map[int64]tender.TenderSummary, error)
}

// Tenders is the combined tender-domain port; *tender.Service satisfies it.
type Tenders interface {
	TenderFit
	TenderSummaries
}

// Eligibility is the narrow company-domain port the decision needs: the
// recommendation a go/no-go is recorded against. Satisfied by *company.Service
// unchanged.
//
// The dependency direction is one-way and stays that way: bid consumes the
// assessment, company knows nothing about bids. Eligibility is a pure function
// of (requirements × dossier); the DECISION is GoNoGo and lives here. Inverting
// this — letting company reach for the decision — would let a recommendation
// depend on what the user already chose, which is how an "assistant" quietly
// becomes a mirror.
//
// The service calls this rather than accepting a recommendation as a parameter
// so that the recorded baseline is the one the system actually computed. A
// transport layer that could pass its own would be asserting what we
// recommended, and the override rate would then measure the client's memory
// rather than our advice.
type Eligibility interface {
	CheckEligibility(ctx context.Context, userID, workspaceID string, tenderID int64, lotRef string) (company.Assessment, error)
}

// Repository persists bids and their checklist items. Implemented by the
// postgres adapter (Phase 1); faked in tests.
type Repository interface {
	CreateBid(ctx context.Context, b Bid) (Bid, error)
	FindBidByID(ctx context.Context, workbenchID, bidID string) (Bid, error)
	ListBidsByWorkbench(ctx context.Context, workbenchID string) ([]Bid, error)
	// UpdateGoNoGo writes the decision AND the recommendation it was taken
	// against in one statement. They are one write and not two because a
	// decision stored without its baseline is an override rate nobody can
	// reconstruct — the assessment is never persisted, so a second failed
	// statement loses the baseline permanently.
	UpdateGoNoGo(ctx context.Context, bidID string, d GoNoGo, rec DecisionRecord) (Bid, error)
	UpdateStage(ctx context.Context, bidID string, s Stage) (Bid, error)
	UpdateOutcome(ctx context.Context, bidID string, o Outcome) (Bid, error)
	DeleteBid(ctx context.Context, bidID string) error
	SeedChecklist(ctx context.Context, bidID string, seeds []ChecklistItemSeed) error
	ListChecklistItems(ctx context.Context, bidID string) ([]ChecklistItem, error)
	UpsertChecklistItem(ctx context.Context, bidID, itemCode, status, note string) (ChecklistItem, error)
}
