package company

import (
	"context"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// Repository persists the dossier. Every method is workspace-scoped as its
// FIRST argument — no method may be reachable without one, which is what makes
// tenant isolation an argument-level property rather than a caller discipline.
//
// The Put* methods are upserts on the record's natural key, not blind inserts:
// a re-answered question is a correction, and a dossier that grew a second row
// for every re-answer would report both readings as facts the company holds.
type Repository interface {
	// GetDossier returns everything asserted about workspaceID's company, or
	// ErrDossierNotFound when nothing at all has been.
	GetDossier(ctx context.Context, workspaceID string) (Dossier, error)
	// UpsertIdentity is a FULL REPLACE of the identity scalars, mirroring
	// clientprofile.Repository.Upsert: a cleared field must overwrite the stored
	// value, not leave it standing.
	UpsertIdentity(ctx context.Context, workspaceID string, id Identity) (Identity, error)

	PutSOACategory(ctx context.Context, workspaceID string, c SOACategory) (SOACategory, error)
	DeleteSOACategory(ctx context.Context, workspaceID, id string) error
	PutCertification(ctx context.Context, workspaceID string, c Certification) (Certification, error)
	DeleteCertification(ctx context.Context, workspaceID, id string) error
	PutFinancialYear(ctx context.Context, workspaceID string, f FinancialYear) (FinancialYear, error)
	DeleteFinancialYear(ctx context.Context, workspaceID string, year int32) error
	PutPastContract(ctx context.Context, workspaceID string, p PastContract) (PastContract, error)
	DeletePastContract(ctx context.Context, workspaceID, id string) error
	PutRegistration(ctx context.Context, workspaceID string, r Registration) (Registration, error)
	DeleteRegistration(ctx context.Context, workspaceID, id string) error
}

// RequirementRepository is separate from Repository and not folded into it,
// because a requirement is a fact about a TENDER captured by a workspace, not a
// fact about the company. Two lifetimes, two ports.
type RequirementRepository interface {
	ListRequirements(ctx context.Context, workspaceID string, tenderID int64) ([]Requirement, error)
	// PutRequirements upserts on (workspace_id, tender_id, lot_ref, dedupe_key)
	// in ONE transaction — an agent that re-extracts on every turn must update,
	// not multiply, and a half-written batch would leave a tender's requirement
	// set describing neither the old extraction nor the new one.
	PutRequirements(ctx context.Context, workspaceID string, tenderID int64, reqs []Requirement) ([]Requirement, error)
	ConfirmRequirement(ctx context.Context, workspaceID, requirementID, userID string) (Requirement, error)
	DeleteRequirement(ctx context.Context, workspaceID, requirementID string) error
}

// MemberRepository is the minimal membership-check port this service needs —
// identical shape and identical rationale to clientprofile.MemberRepository, and
// satisfied by *postgres.MemberRepo unchanged.
type MemberRepository interface {
	LoadMembership(ctx context.Context, workspaceID, userID string) (workspace.Membership, error)
}

// Tenders is the narrow tender port: the engine needs the DEADLINE (the expiry
// comparison that separates "expiring" from "fine") and nothing else about the
// notice. Satisfied by *tender.Service unchanged — the same shape
// agent.TenderInspector already uses.
type Tenders interface {
	GetTender(ctx context.Context, p tender.GetTenderParams) (tender.TenderDetail, error)
}

// Documents reports how much of the tender we can actually read, so an
// assessment can carry coverage/reason rather than presenting ignorance as a
// clean bill of health. Satisfied by *document.Service unchanged (the precedent
// is agent.Documents).
type Documents interface {
	Availability(ctx context.Context, tenderID int64) (document.Availability, error)
}
