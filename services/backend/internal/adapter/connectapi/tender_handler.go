package connectapi

import (
	"context"
	"errors"
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
		results[i] = tenderResultToProto(t)
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
