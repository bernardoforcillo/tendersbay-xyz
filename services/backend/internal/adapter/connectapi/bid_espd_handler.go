package connectapi

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	bidv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/bid/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

// The ESPD per-bid data: lots, subcontractors, reliances. Map, delegate, map
// the error — bid.Service owns validation and the workbench authorization.

func toProtoLot(l bid.Lot) *bidv1.Lot {
	return &bidv1.Lot{Id: l.ID, LotRef: l.LotRef, Position: l.Position}
}

func fromProtoLot(l *bidv1.Lot) (bid.Lot, error) {
	if l == nil {
		return bid.Lot{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("lot is required"))
	}
	return bid.Lot{ID: l.Id, LotRef: l.LotRef, Position: l.Position}, nil
}

func toProtoSubcontractor(s bid.Subcontractor) *bidv1.Subcontractor {
	out := &bidv1.Subcontractor{Id: s.ID, Name: s.Name, Vat: s.VAT, Country: s.Country}
	if s.Share != nil {
		out.SharePct, out.SharePctSet = *s.Share, true
	}
	return out
}

func fromProtoSubcontractor(s *bidv1.Subcontractor) (bid.Subcontractor, error) {
	if s == nil {
		return bid.Subcontractor{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("subcontractor is required"))
	}
	out := bid.Subcontractor{ID: s.Id, Name: s.Name, VAT: s.Vat, Country: s.Country}
	if s.SharePctSet {
		share := s.SharePct
		out.Share = &share
	}
	return out, nil
}

func toProtoReliance(r bid.Reliance) *bidv1.Reliance {
	return &bidv1.Reliance{Id: r.ID, EntityName: r.EntityName, Vat: r.VAT, Criterion: r.Criterion}
}

func fromProtoReliance(r *bidv1.Reliance) (bid.Reliance, error) {
	if r == nil {
		return bid.Reliance{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("reliance is required"))
	}
	return bid.Reliance{ID: r.Id, EntityName: r.EntityName, VAT: r.Vat, Criterion: r.Criterion}, nil
}

func (h *BidHandler) ListEspdData(ctx context.Context, req *connect.Request[bidv1.ListEspdDataRequest]) (*connect.Response[bidv1.ListEspdDataResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	data, err := h.svc.ListEspdData(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := &bidv1.ListEspdDataResponse{}
	for _, l := range data.Lots {
		out.Lots = append(out.Lots, toProtoLot(l))
	}
	for _, s := range data.Subcontractors {
		out.Subcontractors = append(out.Subcontractors, toProtoSubcontractor(s))
	}
	for _, r := range data.Reliances {
		out.Reliances = append(out.Reliances, toProtoReliance(r))
	}
	return connect.NewResponse(out), nil
}

func (h *BidHandler) PutLot(ctx context.Context, req *connect.Request[bidv1.PutLotRequest]) (*connect.Response[bidv1.PutLotResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoLot(req.Msg.Lot)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutLot(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.PutLotResponse{Lot: toProtoLot(out)}), nil
}

func (h *BidHandler) RemoveLot(ctx context.Context, req *connect.Request[bidv1.RemoveLotRequest]) (*connect.Response[bidv1.RemoveLotResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteLot(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.RemoveLotResponse{}), nil
}

func (h *BidHandler) PutSubcontractor(ctx context.Context, req *connect.Request[bidv1.PutSubcontractorRequest]) (*connect.Response[bidv1.PutSubcontractorResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoSubcontractor(req.Msg.Subcontractor)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutSubcontractor(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.PutSubcontractorResponse{Subcontractor: toProtoSubcontractor(out)}), nil
}

func (h *BidHandler) RemoveSubcontractor(ctx context.Context, req *connect.Request[bidv1.RemoveSubcontractorRequest]) (*connect.Response[bidv1.RemoveSubcontractorResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteSubcontractor(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.RemoveSubcontractorResponse{}), nil
}

func (h *BidHandler) PutReliance(ctx context.Context, req *connect.Request[bidv1.PutRelianceRequest]) (*connect.Response[bidv1.PutRelianceResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoReliance(req.Msg.Reliance)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutReliance(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.PutRelianceResponse{Reliance: toProtoReliance(out)}), nil
}

func (h *BidHandler) RemoveReliance(ctx context.Context, req *connect.Request[bidv1.RemoveRelianceRequest]) (*connect.Response[bidv1.RemoveRelianceResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteReliance(ctx, uid, req.Msg.WorkbenchId, req.Msg.BidId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&bidv1.RemoveRelianceResponse{}), nil
}
