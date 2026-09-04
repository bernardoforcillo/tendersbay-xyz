package espd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// Service orchestrates the ESPD document: preview, re-confirmation, export and
// the import of a buyer's request. It authorizes every call through the
// workbench, composes with the pure Compose above it, and delegates the bytes
// to a serializer or the renderer.
//
// It holds *features.Engine concretely, as credits.Service does: the
// entitlement check and the catalog entry it names belong together, and a port
// here would put the feature key in whatever wired the closure.
type Service struct {
	company     CompanyReader
	bids        BidReader
	requests    RequestStore
	exports     ExportLog
	access      WorkbenchAccess
	tenders     Tenders
	features    *features.Engine
	serializers map[Version]Serializer
	renderer    Renderer

	// now is injected so an export's timestamps and a composed document's
	// ComposedAt are reproducible in tests.
	now func() time.Time
}

// NewService wires the domain. Serializers are indexed by the version they
// declare, so registering two for one version is a programming error the
// constructor reports rather than a silent last-one-wins.
func NewService(
	companies CompanyReader,
	bids BidReader,
	requests RequestStore,
	exports ExportLog,
	access WorkbenchAccess,
	tenders Tenders,
	engine *features.Engine,
	serializers []Serializer,
	renderer Renderer,
) (*Service, error) {
	byVersion := make(map[Version]Serializer, len(serializers))
	for _, s := range serializers {
		v := s.Version()
		if !v.Valid() {
			return nil, fmt.Errorf("espd: serializer declares unknown version %q", v)
		}
		if _, dup := byVersion[v]; dup {
			return nil, fmt.Errorf("espd: two serializers registered for version %q", v)
		}
		byVersion[v] = s
	}
	return &Service{
		company: companies, bids: bids, requests: requests, exports: exports,
		access: access, tenders: tenders, features: engine,
		serializers: byVersion, renderer: renderer, now: time.Now,
	}, nil
}

// Artifact is one exported file plus the audit row the export wrote.
type Artifact struct {
	Content  []byte
	Filename string
	MIMEType string
	Export   Export
}

// ── Preview ─────────────────────────────────────────────────────────────────

// Preview composes the document for a bid. It is a READ: any member who can see
// the workbench can see how ready the DGUE is, and it costs nothing — the
// entitlement gates the artefact, not the knowledge.
func (s *Service) Preview(ctx context.Context, userID, workbenchID, bidID string) (Response, error) {
	if err := s.access.CanAccessWorkbench(ctx, userID, workbenchID); err != nil {
		return Response{}, err
	}
	return s.compose(ctx, workbenchID, bidID)
}

// compose gathers the inputs and runs the pure Compose. Everything it reads is
// scoped by ids the caller has already been authorized for.
func (s *Service) compose(ctx context.Context, workbenchID, bidID string) (Response, error) {
	workspaceID, err := s.access.WorkspaceOf(ctx, workbenchID)
	if err != nil {
		return Response{}, err
	}
	b, err := s.bids.FindBidByID(ctx, workbenchID, bidID)
	if err != nil {
		return Response{}, err
	}

	dossier, err := s.company.GetDossier(ctx, workspaceID)
	if err != nil && !errors.Is(err, company.ErrDossierNotFound) {
		return Response{}, err
	}
	dossier.WorkspaceID = workspaceID

	data, err := s.bids.ListEspdData(ctx, bidID)
	if err != nil {
		return Response{}, err
	}

	in := BidInput{Bid: b, Data: data, Procedure: s.procedureOf(ctx, b)}
	if c, err := s.bids.GetDeclarationConfirmation(ctx, bidID); err == nil {
		in.Confirmation = &c
	} else if !errors.Is(err, bid.ErrConfirmationNotFound) {
		return Response{}, err
	}

	var req *Request
	if r, err := s.requests.Get(ctx, bidID); err == nil {
		req = &r
	} else if !errors.Is(err, ErrRequestNotFound) {
		return Response{}, err
	}

	return Compose(dossier, in, req, s.now()), nil
}

// procedureOf reads Part I off the tender the bid tracks. A tender that cannot
// be read yields an empty Procedure, and Compose then reports the Part I fields
// as gaps — which is the honest answer, and strictly better than refusing a
// preview because a notice went missing from the corpus.
func (s *Service) procedureOf(ctx context.Context, b bid.Bid) Procedure {
	detail, err := s.tenders.GetTender(ctx, tender.GetTenderParams{ID: strconv.FormatInt(b.TenderID, 10)})
	if err != nil {
		return Procedure{}
	}
	return Procedure{
		BuyerName: detail.BuyerName,
		Title:     detail.Title,
		Reference: detail.SourceRef,
		NoticeRef: detail.PublicationNumber,
		Country:   detail.Country,
	}
}

// ── Declarations ────────────────────────────────────────────────────────────

// ConfirmDeclarations records that this user re-confirmed, for this bid, the
// Part III declarations the dossier holds right now. It binds to the content
// hash: any later change to a declaration leaves the confirmation behind, and
// the next preview says so without anything having to notice the change.
//
// It refuses an incomplete set. Confirming answers that do not exist yet would
// produce a signed statement about questions nobody answered — the one thing
// this whole domain exists to prevent.
func (s *Service) ConfirmDeclarations(ctx context.Context, userID, workbenchID, bidID string) (Response, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Response{}, err
	}
	resp, err := s.compose(ctx, workbenchID, bidID)
	if err != nil {
		return Response{}, err
	}
	if !resp.Declarations.Complete() {
		return Response{}, fmt.Errorf("%w: every Part III exclusion ground must be answered before confirming", ErrInvalidArgument)
	}
	confirmation := bid.DeclarationConfirmation{
		BidID:            bidID,
		UserID:           userID,
		ConfirmedAt:      s.now(),
		DeclarationsHash: resp.Declarations.Hash,
	}
	stored, err := s.bids.PutDeclarationConfirmation(ctx, confirmation)
	if err != nil {
		return Response{}, err
	}
	resp.Declarations.Confirmation = &stored
	return resp, nil
}

// ── Import ──────────────────────────────────────────────────────────────────

// ImportRequest attaches the buyer's ESPD request to the bid. The raw bytes are
// stored, not the parsed struct: see RequestStore.
func (s *Service) ImportRequest(ctx context.Context, userID, workbenchID, bidID string, raw []byte) (Request, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Request{}, err
	}
	if _, err := s.bids.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return Request{}, err
	}
	req, err := ParseRequest(raw)
	if err != nil {
		return Request{}, err
	}
	req.ImportedBy = userID
	req.ImportedAt = s.now()
	if err := s.requests.Put(ctx, bidID, req, raw); err != nil {
		return Request{}, err
	}
	return req, nil
}

// ── Export ──────────────────────────────────────────────────────────────────

// Export serializes or renders the composed document and records the fact of
// the export.
//
// Three gates, in this order, and the order is the point:
//
//  1. May this user write to this workbench? (404/403 before anything else.)
//  2. Does the workspace's plan carry espd.export? An unentitled caller must
//     not learn whether the document happens to be ready.
//  3. Is the document ready — no gaps, Part III confirmed for this bid? An
//     ESPD with a blank field is not a draft, it is an incomplete legal
//     declaration, so this is a refusal and not a warning.
func (s *Service) Export(ctx context.Context, userID, workbenchID, bidID string, version Version, format Format, locale string) (Artifact, error) {
	if err := s.access.CanManageWorkbench(ctx, userID, workbenchID); err != nil {
		return Artifact{}, err
	}
	if !version.Valid() {
		return Artifact{}, fmt.Errorf("%w: unknown ESPD version %q", ErrInvalidArgument, version)
	}
	if !format.Valid() {
		return Artifact{}, fmt.Errorf("%w: unknown export format %q", ErrInvalidArgument, format)
	}
	workspaceID, err := s.access.WorkspaceOf(ctx, workbenchID)
	if err != nil {
		return Artifact{}, err
	}
	if d := s.features.Evaluate(ctx, features.EspdExport, workspaceID, userID); !d.Enabled {
		return Artifact{}, &NotEntitledError{Reason: string(d.Reason), Detail: d.Detail}
	}

	resp, err := s.compose(ctx, workbenchID, bidID)
	if err != nil {
		return Artifact{}, err
	}
	if !resp.Ready() {
		return Artifact{}, &NotReadyError{Gaps: resp.Gaps, DeclarationsConfirmed: resp.Declarations.Confirmed()}
	}

	var content []byte
	switch format {
	case FormatXML:
		ser, ok := s.serializers[version]
		if !ok {
			return Artifact{}, fmt.Errorf("%w: no serializer registered for version %q", ErrInvalidArgument, version)
		}
		content, err = ser.Serialize(resp)
	case FormatPDF:
		content, err = s.renderer.Render(resp, RenderOptions{Locale: locale, Version: version})
	}
	if err != nil {
		return Artifact{}, err
	}

	sum := sha256.Sum256(content)
	export := Export{
		BidID:                   bidID,
		UserID:                  userID,
		Version:                 version,
		Format:                  format,
		ContentSHA256:           hex.EncodeToString(sum[:]),
		DeclarationsConfirmedAt: resp.Declarations.Confirmation.ConfirmedAt,
		ExportedAt:              s.now(),
	}
	if err := s.exports.Record(ctx, export); err != nil {
		return Artifact{}, err
	}
	return Artifact{
		Content:  content,
		Filename: filename(resp, bidID, version, format),
		MIMEType: mimeType(format),
		Export:   export,
	}, nil
}

// ListExports returns a bid's export history — a read, like the preview.
func (s *Service) ListExports(ctx context.Context, userID, workbenchID, bidID string) ([]Export, error) {
	if err := s.access.CanAccessWorkbench(ctx, userID, workbenchID); err != nil {
		return nil, err
	}
	if _, err := s.bids.FindBidByID(ctx, workbenchID, bidID); err != nil {
		return nil, err
	}
	return s.exports.List(ctx, bidID)
}

// ── Naming ──────────────────────────────────────────────────────────────────

// filename names the downloaded file after the PROCEDURE rather than after our
// bid id: the person saving it files it next to the rest of that gara's
// paperwork, and "dgue-CIG9876543210.xml" is findable there where a uuid is
// not. The bid id is the fallback when no reference is known.
func filename(r Response, bidID string, v Version, f Format) string {
	ref := ""
	for _, l := range r.Leaves {
		if l.Criterion == CritProcedure && l.Field == "reference" {
			ref = l.Value.String()
			break
		}
	}
	if ref == "" {
		ref = bidID
	}
	suffix := "2.1.1"
	if v == EDM4 {
		suffix = "4"
	}
	ext := "xml"
	if f == FormatPDF {
		ext = "pdf"
	}
	return fmt.Sprintf("dgue-%s-edm%s.%s", slug(ref), suffix, ext)
}

// slug keeps a reference safe for a Content-Disposition header and a file
// system: ASCII alphanumerics and dashes, nothing else.
func slug(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		case r == '-' || r == '_':
			b.WriteRune('-')
		default:
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteRune('-')
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func mimeType(f Format) string {
	if f == FormatPDF {
		return "application/pdf"
	}
	return "application/xml"
}
