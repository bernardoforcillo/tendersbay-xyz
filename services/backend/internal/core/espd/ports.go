package espd

import (
	"context"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
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
type RequestStore interface {
	Get(ctx context.Context, bidID string) (Request, error)
	Put(ctx context.Context, bidID string, r Request) error
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
