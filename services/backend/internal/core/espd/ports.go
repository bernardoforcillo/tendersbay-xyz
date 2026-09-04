package espd

import (
	"context"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// CompanyReader is the one dossier read Compose needs. Satisfied by
// *company.Service's GetDossier through a thin adapter in the wiring (the
// service method also takes the caller, which this port deliberately does not:
// authorization for a preview is the WORKBENCH's, resolved once by the espd
// service, and the dossier read that follows must not re-ask the workspace).
type CompanyReader interface {
	GetDossier(ctx context.Context, workspaceID string) (company.Dossier, error)
}

// BidReader is the per-bid slice. Satisfied by the bid repository directly:
// the espd service authorizes through WorkbenchAccess first, exactly as
// bid.Service does, and then reads by id.
type BidReader interface {
	FindBidByID(ctx context.Context, workbenchID, bidID string) (bid.Bid, error)
	ListEspdData(ctx context.Context, bidID string) (bid.EspdData, error)
	GetDeclarationConfirmation(ctx context.Context, bidID string) (bid.DeclarationConfirmation, error)
	PutDeclarationConfirmation(ctx context.Context, c bid.DeclarationConfirmation) (bid.DeclarationConfirmation, error)
}

// RequestStore keeps the buyer's ESPD request attached to a bid (Approach C).
// Get returns ErrRequestNotFound when none was imported.
//
// Put takes the RAW bytes alongside the parsed Request because the bytes are
// the source of truth and the struct is derived: storing only the struct would
// freeze the criterion taxonomy at import time, so a request imported before we
// learned a new criterion code could never be re-read with the newer mapping.
type RequestStore interface {
	Get(ctx context.Context, bidID string) (Request, error)
	Put(ctx context.Context, bidID string, r Request, raw []byte) error
}

// WorkbenchAccess is the workbench RBAC slice this domain needs — the same
// shape and the same rationale as bid.WorkbenchAccess, satisfied by
// *workbench.Service unchanged. Authorization for an ESPD read or write is the
// WORKBENCH's, never re-derived here: the document belongs to a bid, and a bid
// lives in a workbench.
type WorkbenchAccess interface {
	CanAccessWorkbench(ctx context.Context, userID, workbenchID string) error
	CanManageWorkbench(ctx context.Context, userID, workbenchID string) error
	WorkspaceOf(ctx context.Context, workbenchID string) (string, error)
}

// Tenders supplies the Part I facts a bid knows only through its tender: who is
// buying and under which reference. Satisfied by *tender.Service unchanged.
//
// A failure here is NOT fatal to a preview: a tender that cannot be read leaves
// Part I to the gap list, which is a truthful "we do not know the buyer yet"
// rather than a refused document.
type Tenders interface {
	GetTender(ctx context.Context, p tender.GetTenderParams) (tender.TenderDetail, error)
}

// Serializer turns a Response into one XML version. Each implementation owns
// exactly one code list and one schema; nothing about a version leaks into
// Response.
type Serializer interface {
	Version() Version
	Serialize(r Response) ([]byte, error)
}

// RenderOptions are the human-facing knobs of the PDF: which locale to label
// in and which version's field order to follow.
type RenderOptions struct {
	Locale  string
	Version Version
}

// Renderer produces the human-readable artefact.
type Renderer interface {
	Render(r Response, opts RenderOptions) ([]byte, error)
}

// ExportLog is the audit trail of exports — the fact, never the bytes.
type ExportLog interface {
	Record(ctx context.Context, e Export) error
	List(ctx context.Context, bidID string) ([]Export, error)
}
