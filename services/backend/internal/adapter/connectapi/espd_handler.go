package connectapi

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	espdv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/espd/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/espd/v1/espdv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// EspdHandler is the transport for the ESPD/DGUE document: preview, Part III
// re-confirmation, export, and the import of a buyer's request.
//
// It validates the wire shapes it cannot delegate (an unknown version or
// format) and maps between the wire and the domain. Everything else —
// authorization through the workbench, the espd.export entitlement, whether a
// document is ready to export — belongs to espd.Service, and a handler that
// re-decided any of it would be the fat controller the code-organization rule
// names as the failure mode.
type EspdHandler struct{ svc *espd.Service }

// NewEspdHandler wires the handler.
func NewEspdHandler(svc *espd.Service) *EspdHandler { return &EspdHandler{svc: svc} }

var _ espdv1connect.EspdServiceHandler = (*EspdHandler)(nil)

// ── Domain → wire ───────────────────────────────────────────────────────────

func toProtoValue(v espd.Value) *espdv1.Value {
	out := &espdv1.Value{Kind: string(v.Kind)}
	switch v.Kind {
	case espd.KindBool:
		out.BoolValue = v.Bool
	case espd.KindInt:
		out.IntValue = v.Int
	case espd.KindAmount:
		out.IntValue, out.Currency = v.Int, v.Currency
	case espd.KindDate:
		out.Date = v.Date.Format(time.RFC3339)
	default:
		out.Text = v.Text
	}
	return out
}

func toProtoLeaf(l espd.Leaf) *espdv1.Leaf {
	return &espdv1.Leaf{
		Part:        string(l.Part),
		Criterion:   string(l.Criterion),
		Field:       l.Field,
		Value:       toProtoValue(l.Value),
		Attribution: toProtoAttribution(l.Attribution),
		SourceKind:  string(l.Source.Kind),
		SourceId:    l.Source.ID,
	}
}

// toProtoEspdGap is named for its domain because company_handler.go already
// owns a toProtoGap: the two Gaps answer different questions — an eligibility
// gap is "you may not qualify", an ESPD gap is "this field is empty" — and
// sharing a name would invite sharing a mapping.
func toProtoEspdGap(g espd.Gap) *espdv1.Gap {
	return &espdv1.Gap{
		Part:      string(g.Part),
		Criterion: string(g.Criterion),
		Field:     g.Field,
		Scope:     string(g.Scope),
		Reason:    string(g.Reason),
	}
}

func toProtoDeclarations(s espd.DeclarationSet) *espdv1.DeclarationState {
	out := &espdv1.DeclarationState{
		Complete:         s.Complete(),
		Confirmed:        s.Confirmed(),
		DeclarationsHash: s.Hash,
	}
	for _, a := range s.Answers {
		out.Answers = append(out.Answers, &espdv1.DeclarationAnswer{
			Criterion:    string(a.Criterion),
			Answered:     a.Answered,
			Applies:      a.Applies,
			SelfCleaning: a.SelfCleaning,
			Attribution:  toProtoAttribution(a.Attribution),
		})
	}
	// The confirmation is reported even when it is STALE — its timestamp is
	// what a client shows next to "these answers were confirmed on …, and one
	// has changed since". Collapsing it to nothing would lose the sentence.
	if c := s.Confirmation; c != nil {
		out.ConfirmedAt = c.ConfirmedAt.Format(time.RFC3339)
		out.ConfirmedBy = c.UserID
	}
	return out
}

func toProtoRequestSummary(r *espd.Request) *espdv1.RequestSummary {
	if r == nil {
		return nil
	}
	out := &espdv1.RequestSummary{
		Version:            string(r.Version),
		BuyerName:          r.BuyerName,
		ProcedureTitle:     r.ProcedureTitle,
		ProcedureReference: r.ProcedureReference,
		NoticeRef:          r.NoticeRef,
		Country:            r.Country,
		Lots:               r.Lots,
		UnmappedCriteria:   r.UnmappedCriteria,
		Sha256:             r.SHA256,
		ImportedBy:         r.ImportedBy,
	}
	if !r.ImportedAt.IsZero() {
		out.ImportedAt = r.ImportedAt.Format(time.RFC3339)
	}
	for _, k := range r.Criteria {
		out.Criteria = append(out.Criteria, string(k))
	}
	return out
}

func toProtoExport(e espd.Export) *espdv1.Export {
	return &espdv1.Export{
		Id:                      e.ID,
		BidId:                   e.BidID,
		UserId:                  e.UserID,
		Version:                 string(e.Version),
		Format:                  string(e.Format),
		ContentSha256:           e.ContentSHA256,
		DeclarationsConfirmedAt: e.DeclarationsConfirmedAt.Format(time.RFC3339),
		ExportedAt:              e.ExportedAt.Format(time.RFC3339),
	}
}

func toProtoPreview(r espd.Response) *espdv1.GetResponsePreviewResponse {
	ready, missing := r.Readiness()
	out := &espdv1.GetResponsePreviewResponse{
		Ready:        r.Ready(),
		ReadyCount:   int32(ready),
		MissingCount: int32(missing),
		RequestKnown: r.Request != nil,
		Request:      toProtoRequestSummary(r.Request),
		Declarations: toProtoDeclarations(r.Declarations),
		ComposedAt:   r.ComposedAt.Format(time.RFC3339),
	}
	for _, l := range r.Leaves {
		out.Leaves = append(out.Leaves, toProtoLeaf(l))
	}
	for _, g := range r.Gaps {
		out.Gaps = append(out.Gaps, toProtoEspdGap(g))
	}
	return out
}

// ── RPCs ────────────────────────────────────────────────────────────────────

// GetResponsePreview composes the document and reports what is filled, what is
// open and whether it can be exported. Free: the entitlement gates the
// artefact, not the knowledge.
func (h *EspdHandler) GetResponsePreview(ctx context.Context, req *connect.Request[espdv1.GetResponsePreviewRequest]) (*connect.Response[espdv1.GetResponsePreviewResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := h.svc.Preview(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(toProtoPreview(resp)), nil
}

// ConfirmDeclarations re-confirms the Part III answers for this bid.
func (h *EspdHandler) ConfirmDeclarations(ctx context.Context, req *connect.Request[espdv1.ConfirmDeclarationsRequest]) (*connect.Response[espdv1.ConfirmDeclarationsResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := h.svc.ConfirmDeclarations(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&espdv1.ConfirmDeclarationsResponse{
		Declarations: toProtoDeclarations(resp.Declarations),
	}), nil
}

// ExportResponse serializes or renders the document. The version and format are
// validated here because they are wire values with a closed set; everything
// else about the refusal — the plan, the open gaps — is the service's.
func (h *EspdHandler) ExportResponse(ctx context.Context, req *connect.Request[espdv1.ExportResponseRequest]) (*connect.Response[espdv1.ExportResponseResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	version := espd.Version(req.Msg.Version)
	if !version.Valid() {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("version must be %q or %q, got %q", espd.EDM211, espd.EDM4, req.Msg.Version))
	}
	format := espd.Format(req.Msg.Format)
	if !format.Valid() {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("format must be %q or %q, got %q", espd.FormatXML, espd.FormatPDF, req.Msg.Format))
	}
	art, err := h.svc.Export(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, version, format, req.Msg.Locale)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&espdv1.ExportResponseResponse{
		Content:  art.Content,
		Filename: art.Filename,
		MimeType: art.MIMEType,
		Export:   toProtoExport(art.Export),
	}), nil
}

// ImportRequest attaches the buyer's ESPD request XML to the bid.
func (h *EspdHandler) ImportRequest(ctx context.Context, req *connect.Request[espdv1.ImportRequestRequest]) (*connect.Response[espdv1.ImportRequestResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if len(req.Msg.Xml) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("xml is required"))
	}
	parsed, err := h.svc.ImportRequest(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, req.Msg.Xml)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&espdv1.ImportRequestResponse{Request: toProtoRequestSummary(&parsed)}), nil
}

// ListExports returns the audit trail: the fact of each export, never its bytes.
func (h *EspdHandler) ListExports(ctx context.Context, req *connect.Request[espdv1.ListExportsRequest]) (*connect.Response[espdv1.ListExportsResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	exports, err := h.svc.ListExports(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := &espdv1.ListExportsResponse{}
	for _, e := range exports {
		out.Exports = append(out.Exports, toProtoExport(e))
	}
	return connect.NewResponse(out), nil
}
