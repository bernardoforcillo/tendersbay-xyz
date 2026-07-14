package connectapi

import (
	"context"
	"time"

	"connectrpc.com/connect"

	tenderv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1/tenderv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// TenderHandler serves TenderService. Unlike every other handler in this
// package, SearchTenders works for unauthenticated callers by design —
// see UserIDFromContext below, used directly instead of requireUser.
type TenderHandler struct {
	svc *tender.Service
}

var _ tenderv1connect.TenderServiceHandler = (*TenderHandler)(nil)

// NewTenderHandler builds a TenderHandler.
func NewTenderHandler(svc *tender.Service) *TenderHandler {
	return &TenderHandler{svc: svc}
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
	}
}
