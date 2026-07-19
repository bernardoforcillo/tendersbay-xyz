package connectapi

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"

	tenderv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1/tenderv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// MemberRepository is the minimal membership-check port TenderHandler
// needs — satisfied by *postgres.MemberRepo unchanged, the same concrete
// type WorkspaceHandler and AgentHandler already depend on via their own
// narrow ports. Added by Task A-annotate (an amendment task); SearchTenders
// itself does not call it — its per-client fit annotation trusts
// AnnotateForClient's own internal membership check instead (see
// SearchTenders' doc comment), so this port is currently unused by that RPC.
// Task 9's RecommendTendersForClient (not yet implemented as of this task)
// won't need it either: RecommendForClient is membership-checked the same
// way AnnotateForClient is — via ProfileSource.Get itself (see both
// methods' doc comments in core/tender/recommend.go) — so it has no more
// need for a redundant handler-level LoadMembership call than
// AnnotateForClient does. This port is kept on TenderHandler for now in
// case a future handler needs a membership check the service layer doesn't
// already provide — none of the current or currently-planned uses require
// it.
type MemberRepository interface {
	LoadMembership(ctx context.Context, workspaceID, userID string) (workspace.Membership, error)
}

// TenderHandler serves TenderService. Unlike every other handler in this
// package, SearchTenders works for unauthenticated callers by design —
// see UserIDFromContext below, used directly instead of requireUser.
type TenderHandler struct {
	svc     *tender.Service
	members MemberRepository
}

var _ tenderv1connect.TenderServiceHandler = (*TenderHandler)(nil)

// NewTenderHandler builds a TenderHandler.
func NewTenderHandler(svc *tender.Service, members MemberRepository) *TenderHandler {
	return &TenderHandler{svc: svc, members: members}
}

func (h *TenderHandler) SearchTenders(ctx context.Context, req *connect.Request[tenderv1.SearchTendersRequest]) (*connect.Response[tenderv1.SearchTendersResponse], error) {
	userID, authed := UserIDFromContext(ctx)
	rateLimitKey := userID
	if !authed {
		rateLimitKey = ClientIPFromContext(ctx)
	}

	filters, err := filtersFromProto(req.Msg.Filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	out, err := h.svc.Search(ctx, tender.SearchParams{
		Query:         req.Msg.Query,
		Filters:       filters,
		Limit:         int(req.Msg.Limit),
		Offset:        int(req.Msg.Offset),
		Authenticated: authed,
		RateLimitKey:  rateLimitKey,
	})
	if err != nil {
		return nil, toConnectError(err)
	}

	results := make([]*tenderv1.TenderResult, len(out.Results))
	for i, t := range out.Results {
		results[i] = h.tenderResultToProtoWithThreshold(t)
	}

	// workspace_id == "" is today's anonymous-safe behavior, left
	// completely unchanged: no membership check, no AnnotateForClient call.
	//
	// When workspace_id is set, this trusts AnnotateForClient's own
	// membership check (it calls ProfileSource.Get, which is
	// membership-checked by clientprofile.Service.Get → requireMember) and
	// does not re-check membership itself here — the same trust-the-callee
	// shape agent.Service.ChatStream uses for its analogous case, to avoid a
	// redundant LoadMembership round trip on this path. toConnectError maps
	// the resulting workspace.ErrNotMember to PermissionDenied the same way
	// regardless of which call site produced it.
	if req.Msg.WorkspaceId != "" {
		recs, err := h.svc.AnnotateForClient(ctx, userID, req.Msg.WorkspaceId, out.Results)
		if err != nil {
			return nil, toConnectError(err)
		}
		// AnnotateForClient preserves input order, so recs[i] always
		// corresponds to results[i] built from the same out.Results above.
		for i, r := range recs {
			if r.Tier == "" {
				// ErrProfileNotFound passthrough — leave fit_tier/reason unset.
				continue
			}
			results[i].FitTier = string(r.Tier)
			results[i].Reason = reasonSignalsToProto(r.Reason)
		}
	}

	return connect.NewResponse(&tenderv1.SearchTendersResponse{
		Results: results,
		HasMore: out.HasMore,
	}), nil
}

// RecommendTendersForClient requires auth like every other handler in this
// package except SearchTenders — no handler-level membership check is added
// here: h.svc.RecommendForClient is already membership-checked internally
// via ProfileSource.Get → clientprofile.Service.Get → requireMember, the
// same trust-the-callee shape SearchTenders' workspace_id branch uses for
// AnnotateForClient (see its doc comment). A redundant h.members.
// LoadMembership call here would duplicate that check for no benefit.
func (h *TenderHandler) RecommendTendersForClient(ctx context.Context, req *connect.Request[tenderv1.RecommendTendersForClientRequest]) (*connect.Response[tenderv1.RecommendTendersForClientResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	recs, err := h.svc.RecommendForClient(ctx, uid, req.Msg.WorkspaceId, int(req.Msg.Limit))
	if errors.Is(err, clientprofile.ErrProfileNotFound) {
		return connect.NewResponse(&tenderv1.RecommendTendersForClientResponse{NeedsProfile: true}), nil
	}
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*tenderv1.RecommendedTenderResult, len(recs))
	for i, r := range recs {
		out[i] = h.recommendedTenderToProto(r)
	}
	return connect.NewResponse(&tenderv1.RecommendTendersForClientResponse{Results: out}), nil
}

// GetCoverage is anonymous-safe like SearchTenders — no auth, no membership.
// It reports which countries we currently hold tenders for so the landing
// coverage marquee can light real flags.
func (h *TenderHandler) GetCoverage(ctx context.Context, _ *connect.Request[tenderv1.GetCoverageRequest]) (*connect.Response[tenderv1.GetCoverageResponse], error) {
	countries, err := h.svc.Coverage(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&tenderv1.GetCoverageResponse{Countries: countries}), nil
}

func filtersFromProto(f *tenderv1.TenderFilters) (tender.Filters, error) {
	if f == nil {
		return tender.Filters{}, nil
	}
	out := tender.Filters{Country: f.Country, CPV: f.Cpv, Status: f.Status}
	if f.DeadlineFrom != "" {
		t, err := time.Parse(time.RFC3339, f.DeadlineFrom)
		if err != nil {
			return tender.Filters{}, err
		}
		out.DeadlineFrom = &t
	}
	if f.DeadlineTo != "" {
		t, err := time.Parse(time.RFC3339, f.DeadlineTo)
		if err != nil {
			return tender.Filters{}, err
		}
		out.DeadlineTo = &t
	}
	return out, nil
}

// tenderResultToProto converts the shared, EU-threshold-independent result
// fields — reused by every caller in this package that needs a
// *tenderv1.TenderResult, including agent_handler.go's chat tender_results
// event (which has no *tender.Service to compute a threshold band from, and
// doesn't need one — TenderResultCard already renders no badge at all when
// EuThreshold is empty). Callers that DO have the band (every TenderHandler
// method) stamp it via tenderResultToProtoWithThreshold instead.
func tenderResultToProto(t tender.ScoredTender) *tenderv1.TenderResult {
	var value int64
	if t.Value != nil {
		value = *t.Value
	}
	var publishedAt, deadline string
	if t.PublishedAt != nil {
		publishedAt = t.PublishedAt.Format(time.RFC3339)
	}
	if t.Deadline != nil {
		deadline = t.Deadline.Format(time.RFC3339)
	}
	return &tenderv1.TenderResult{
		Id: t.ID, Title: t.Title, BuyerName: t.BuyerName, Status: t.Status,
		ProcedureType: t.ProcedureType, Country: t.Country, Cpv: t.CPV,
		Value: value, Currency: t.Currency, PublishedAt: publishedAt, Deadline: deadline,
		RelevanceScore: t.RelevanceScore, Source: t.Source, SourceRef: t.SourceRef,
		SourceUrl: t.SourceURL,
	}
}

// tenderResultToProtoWithThreshold is a method so it can reach h.svc for the
// eu_threshold band — every TenderHandler result path (SearchTenders,
// GetRelatedTenders, and RecommendTendersForClient via
// recommendedTenderToProto) routes through it, so the coarse below/above-EU-
// threshold band is stamped on all of them.
func (h *TenderHandler) tenderResultToProtoWithThreshold(t tender.ScoredTender) *tenderv1.TenderResult {
	p := tenderResultToProto(t)
	p.EuThreshold = h.svc.EUThresholdBand(t.Value, t.CPV)
	return p
}

// reasonSignalsToProto maps tender.ReasonSignals onto the wire type. Only
// called from SearchTenders' workspace_id-set branch, once a result has
// actually been annotated (see AnnotateForClient's empty-Tier passthrough
// contract) — so DeadlineDays/HasDeadline follow ReasonSignals.DeadlineDays'
// own nil-means-"no deadline or already closed" convention (see
// tender.deadlineDays), not a fresh assumption made here.
func reasonSignalsToProto(r tender.ReasonSignals) *tenderv1.ReasonSignals {
	out := &tenderv1.ReasonSignals{
		SectorMatch:    r.SectorMatch,
		CountryMatch:   r.CountryMatch,
		ValueFit:       r.ValueFit,
		RegionMatch:    r.RegionMatch,
		ProcedureMatch: r.ProcedureMatch,
	}
	if r.DeadlineDays != nil {
		out.DeadlineDays = int32(*r.DeadlineDays)
		out.HasDeadline = true
	}
	return out
}

// recommendedTenderToProto maps one tender.RecommendedTender (Task 8) onto
// the wire RecommendedTenderResult, reusing reasonSignalsToProto above for
// the Reason mapping so all six tender.ReasonSignals fields — including
// RegionMatch/ProcedureMatch — stay in sync across both RPCs that emit
// ReasonSignals (SearchTenders' annotation branch and this one), rather than
// two independently-maintained copies of the same mapping.
func (h *TenderHandler) recommendedTenderToProto(r tender.RecommendedTender) *tenderv1.RecommendedTenderResult {
	return &tenderv1.RecommendedTenderResult{
		Tender:  h.tenderResultToProtoWithThreshold(r.ScoredTender),
		FitTier: string(r.Tier),
		Reason:  reasonSignalsToProto(r.Reason),
	}
}

func (h *TenderHandler) GetTender(ctx context.Context, req *connect.Request[tenderv1.GetTenderRequest]) (*connect.Response[tenderv1.GetTenderResponse], error) {
	detail, err := h.svc.GetTender(ctx, tender.GetTenderParams{
		ID:           req.Msg.Id,
		RateLimitKey: rateLimitKey(ctx),
	})
	if err != nil {
		if errors.Is(err, tender.ErrTenderNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&tenderv1.GetTenderResponse{Tender: tenderDetailToProto(detail)}), nil
}

func (h *TenderHandler) GetRelatedTenders(ctx context.Context, req *connect.Request[tenderv1.GetRelatedTendersRequest]) (*connect.Response[tenderv1.GetRelatedTendersResponse], error) {
	out, err := h.svc.GetRelatedTenders(ctx, tender.RelatedParams{
		ID:           req.Msg.Id,
		Limit:        int(req.Msg.Limit),
		RateLimitKey: rateLimitKey(ctx),
	})
	if err != nil {
		return nil, toConnectError(err)
	}
	results := make([]*tenderv1.TenderResult, len(out))
	for i, t := range out {
		results[i] = h.tenderResultToProtoWithThreshold(t)
	}
	return connect.NewResponse(&tenderv1.GetRelatedTendersResponse{Results: results}), nil
}

func (h *TenderHandler) ListTenderSitemap(ctx context.Context, req *connect.Request[tenderv1.ListTenderSitemapRequest]) (*connect.Response[tenderv1.ListTenderSitemapResponse], error) {
	refs, err := h.svc.ListTenderSitemap(ctx, int(req.Msg.Limit))
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*tenderv1.TenderRef, len(refs))
	for i, r := range refs {
		out[i] = &tenderv1.TenderRef{Id: r.ID, Lastmod: r.Lastmod}
	}
	return connect.NewResponse(&tenderv1.ListTenderSitemapResponse{Refs: out}), nil
}

// rateLimitKey is the user ID for authenticated callers, else the client IP —
// the same rule SearchTenders uses.
func rateLimitKey(ctx context.Context) string {
	if userID, authed := UserIDFromContext(ctx); authed {
		return userID
	}
	return ClientIPFromContext(ctx)
}

func tenderDetailToProto(d tender.TenderDetail) *tenderv1.TenderDetail {
	var value int64
	if d.Value != nil {
		value = *d.Value
	}
	var publishedAt, deadline string
	if d.PublishedAt != nil {
		publishedAt = d.PublishedAt.Format(time.RFC3339)
	}
	if d.Deadline != nil {
		deadline = d.Deadline.Format(time.RFC3339)
	}
	docs := make([]*tenderv1.TenderDocument, len(d.Documents))
	for i, doc := range d.Documents {
		docs[i] = &tenderv1.TenderDocument{Url: doc.URL, Type: doc.Type}
	}
	lots := make([]*tenderv1.TenderLot, len(d.Lots))
	for i, lot := range d.Lots {
		lots[i] = tenderLotToProto(lot)
	}
	return &tenderv1.TenderDetail{
		Id: d.ID, Title: d.Title, BuyerName: d.BuyerName, BuyerId: d.BuyerID, Status: d.Status,
		ProcedureType: d.ProcedureType, Country: d.Country, Nuts: d.NUTS, Language: d.Language,
		Cpv: d.CPV, CpvSecondary: d.CPVSecondary, Value: value, Currency: d.Currency,
		PublishedAt: publishedAt, Deadline: deadline, Source: d.Source, SourceRef: d.SourceRef,
		SourceUrl: d.SourceURL, Documents: docs, Lots: lots,
	}
}

func tenderLotToProto(l tender.Lot) *tenderv1.TenderLot {
	var value int64
	if l.Value != nil {
		value = *l.Value
	}
	var deadline string
	if l.Deadline != nil {
		deadline = l.Deadline.Format(time.RFC3339)
	}
	return &tenderv1.TenderLot{
		Ref: l.Ref, Title: l.Title, Cpv: l.CPV, Value: value, Currency: l.Currency, Deadline: deadline,
	}
}
