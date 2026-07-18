package connectapi

import (
	"context"
	"time"

	"connectrpc.com/connect"

	tenderv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1/tenderv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// MemberRepository is the minimal membership-check port TenderHandler
// needs — satisfied by *postgres.MemberRepo unchanged, the same concrete
// type WorkspaceHandler and AgentHandler already depend on via their own
// narrow ports. Added by Task A-annotate (an amendment task) to gate
// SearchTenders' per-client fit annotation on workspace membership when a
// caller passes workspace_id. Task 9 (RecommendTendersForClient) — not yet
// implemented as of this task — reuses this same h.members field rather
// than adding its own port; that RPC also needs a membership check and
// this one is already shaped for it.
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
		results[i] = tenderResultToProto(t)
	}

	// workspace_id == "" is today's anonymous-safe behavior, left
	// completely unchanged: no membership check, no AnnotateForClient call.
	if req.Msg.WorkspaceId != "" {
		if _, err := h.members.LoadMembership(ctx, req.Msg.WorkspaceId, userID); err != nil {
			return nil, toConnectError(err)
		}
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
