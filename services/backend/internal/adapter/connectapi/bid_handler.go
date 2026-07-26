package connectapi

import (
	"context"
	"strconv"
	"time"

	"connectrpc.com/connect"

	bidv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/bid/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/bid/v1/bidv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

type BidHandler struct{ svc *bid.Service }

func NewBidHandler(svc *bid.Service) *BidHandler {
	return &BidHandler{svc: svc}
}

// toProtoBidEntity maps a bare bid.Bid — the shape the write RPCs return —
// onto the wire Bid. Only the lifecycle fields are populated; the embedded
// tender summary, fit, and checklist counts stay zero (clients refetch
// ListBids/GetBid for the enriched view).
func toProtoBidEntity(b bid.Bid) *bidv1.Bid {
	return &bidv1.Bid{
		Id:          b.ID,
		WorkbenchId: b.WorkbenchID,
		TenderId:    strconv.FormatInt(b.TenderID, 10),
		GoNoGo:      string(b.GoNoGo),
		Stage:       string(b.Stage),
		Outcome:     string(b.Outcome),
		CreatedAt:   b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   b.UpdatedAt.Format(time.RFC3339),
	}
}

// toProtoBid maps a fully-enriched bid.BidView (the read RPCs' shape) onto the
// wire Bid: lifecycle fields via toProtoBidEntity, plus the embedded tender
// summary, fresh fit, and checklist progress. reasonSignalsToProto is reused
// from tender_handler.go (same package), so the reason mapping stays identical
// to SearchTenders/RecommendTendersForClient.
func toProtoBid(v bid.BidView) *bidv1.Bid {
	b := toProtoBidEntity(v.Bid)
	b.TenderAvailable = v.TenderAvailable
	b.NeedsProfile = !v.Fit.HasProfile
	b.FitTier = string(v.Fit.Tier)
	b.Reason = reasonSignalsToProto(v.Fit.Reason)
	b.ChecklistDone = int32(v.ChecklistDone)
	b.ChecklistTotal = int32(v.ChecklistTotal)
	if v.TenderAvailable {
		b.TenderTitle = v.Summary.Title
		b.TenderBuyerName = v.Summary.BuyerName
		b.TenderCountry = v.Summary.Country
		b.TenderCpv = v.Summary.CPV
		if v.Summary.Value != nil {
			b.TenderValue = *v.Summary.Value
		}
		b.TenderCurrency = v.Summary.Currency
		b.TenderDeadline = v.Summary.Deadline
		b.TenderStatus = v.Summary.Status
	}
	return b
}

func toProtoChecklistItem(c bid.ChecklistItem) *bidv1.ChecklistItem {
	return &bidv1.ChecklistItem{
		Id:          c.ID,
		SectionCode: c.SectionCode,
		ItemCode:    c.ItemCode,
		Status:      c.Status,
		Note:        c.Note,
		Required:    c.Required,
		Position:    int32(c.Position),
	}
}

func (h *BidHandler) ListBids(ctx context.Context, req *connect.Request[bidv1.ListBidsRequest]) (*connect.Response[bidv1.ListBidsResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	views, err := h.svc.ListBids(ctx, uid, req.Msg.WorkbenchId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*bidv1.Bid, len(views))
	for i, v := range views {
		out[i] = toProtoBid(v)
	}
	return connect.NewResponse(&bidv1.ListBidsResponse{Bids: out}), nil
}

func (h *BidHandler) GetBid(ctx context.Context, req *connect.Request[bidv1.GetBidRequest]) (*connect.Response[bidv1.GetBidResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	v, err := h.svc.GetBid(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.GetBidResponse{Bid: toProtoBid(v)}), nil
}

func (h *BidHandler) ListChecklistItems(ctx context.Context, req *connect.Request[bidv1.ListChecklistItemsRequest]) (*connect.Response[bidv1.ListChecklistItemsResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	items, err := h.svc.ListChecklistItems(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*bidv1.ChecklistItem, len(items))
	for i, it := range items {
		out[i] = toProtoChecklistItem(it)
	}
	return connect.NewResponse(&bidv1.ListChecklistItemsResponse{Items: out}), nil
}

var _ bidv1connect.BidServiceHandler = (*BidHandler)(nil)

func (h *BidHandler) AddBid(ctx context.Context, req *connect.Request[bidv1.AddBidRequest]) (*connect.Response[bidv1.AddBidResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	tenderID, err := strconv.ParseInt(req.Msg.TenderId, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	b, err := h.svc.AddBid(ctx, uid, req.Msg.WorkbenchId, tenderID)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.AddBidResponse{Bid: toProtoBidEntity(b)}), nil
}

func (h *BidHandler) SetGoNoGo(ctx context.Context, req *connect.Request[bidv1.SetGoNoGoRequest]) (*connect.Response[bidv1.SetGoNoGoResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	b, err := h.svc.SetGoNoGo(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, bid.GoNoGo(req.Msg.GoNoGo))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.SetGoNoGoResponse{Bid: toProtoBidEntity(b)}), nil
}

func (h *BidHandler) AdvanceStage(ctx context.Context, req *connect.Request[bidv1.AdvanceStageRequest]) (*connect.Response[bidv1.AdvanceStageResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	b, err := h.svc.AdvanceStage(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.AdvanceStageResponse{Bid: toProtoBidEntity(b)}), nil
}

func (h *BidHandler) RecordOutcome(ctx context.Context, req *connect.Request[bidv1.RecordOutcomeRequest]) (*connect.Response[bidv1.RecordOutcomeResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	b, err := h.svc.RecordOutcome(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, bid.Outcome(req.Msg.Outcome))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.RecordOutcomeResponse{Bid: toProtoBidEntity(b)}), nil
}

func (h *BidHandler) UpsertChecklistAnswer(ctx context.Context, req *connect.Request[bidv1.UpsertChecklistAnswerRequest]) (*connect.Response[bidv1.UpsertChecklistAnswerResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	item, err := h.svc.UpsertChecklistAnswer(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, req.Msg.ItemCode, req.Msg.Status, req.Msg.Note)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.UpsertChecklistAnswerResponse{Item: toProtoChecklistItem(item)}), nil
}

func (h *BidHandler) RemoveBid(ctx context.Context, req *connect.Request[bidv1.RemoveBidRequest]) (*connect.Response[bidv1.RemoveBidResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RemoveBid(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.RemoveBidResponse{}), nil
}
