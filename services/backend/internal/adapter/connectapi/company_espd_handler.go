package connectapi

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	companyv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/company/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
)

// The ESPD sections of the dossier: representatives (Part II.B), Part III
// declarations and national grounds. Same shape as the rest of
// company_handler.go — map the wire, delegate, map the error. The provenance
// wall on declarations is company.Service's, not this file's: whatever
// attribution the client sends is overwritten server-side.

// ── Wire → domain, and back ─────────────────────────────────────────────────

func toProtoRepresentative(r company.Representative) *companyv1.Representative {
	return &companyv1.Representative{
		Id:              r.ID,
		Role:            r.Role,
		GivenName:       r.GivenName,
		FamilyName:      r.FamilyName,
		BirthDate:       formatTime(r.BirthDate),
		BirthPlace:      r.BirthPlace,
		Address:         r.Address,
		Email:           r.Email,
		PowerOfAttorney: r.PowerOfAttorney,
		Attribution:     toProtoAttribution(r.Attribution),
	}
}

func fromProtoRepresentative(r *companyv1.Representative) (company.Representative, error) {
	if r == nil {
		return company.Representative{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("representative is required"))
	}
	birth, err := parseTime("representative.birth_date", r.BirthDate)
	if err != nil {
		return company.Representative{}, err
	}
	attr, err := fromProtoAttribution(r.Attribution)
	if err != nil {
		return company.Representative{}, err
	}
	return company.Representative{
		ID:              r.Id,
		Role:            r.Role,
		GivenName:       r.GivenName,
		FamilyName:      r.FamilyName,
		BirthDate:       birth,
		BirthPlace:      r.BirthPlace,
		Address:         r.Address,
		Email:           r.Email,
		PowerOfAttorney: r.PowerOfAttorney,
		Attribution:     attr,
	}, nil
}

func toProtoDeclaration(d company.Declaration) *companyv1.Declaration {
	return &companyv1.Declaration{
		Id:           d.ID,
		Criterion:    d.Criterion,
		Answer:       d.Answer,
		SelfCleaning: d.SelfCleaning,
		Attribution:  toProtoAttribution(d.Attribution),
	}
}

func fromProtoDeclaration(d *companyv1.Declaration) (company.Declaration, error) {
	if d == nil {
		return company.Declaration{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("declaration is required"))
	}
	attr, err := fromProtoAttribution(d.Attribution)
	if err != nil {
		return company.Declaration{}, err
	}
	return company.Declaration{ID: d.Id, Criterion: d.Criterion, Answer: d.Answer, SelfCleaning: d.SelfCleaning, Attribution: attr}, nil
}

func toProtoNationalGround(g company.NationalGround) *companyv1.NationalGround {
	return &companyv1.NationalGround{
		Id:          g.ID,
		Country:     g.Country,
		Criterion:   g.Criterion,
		Answer:      g.Answer,
		Note:        g.Note,
		Attribution: toProtoAttribution(g.Attribution),
	}
}

func fromProtoNationalGround(g *companyv1.NationalGround) (company.NationalGround, error) {
	if g == nil {
		return company.NationalGround{}, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("national_ground is required"))
	}
	attr, err := fromProtoAttribution(g.Attribution)
	if err != nil {
		return company.NationalGround{}, err
	}
	return company.NationalGround{ID: g.Id, Country: g.Country, Criterion: g.Criterion, Answer: g.Answer, Note: g.Note, Attribution: attr}, nil
}

// ── RPCs ────────────────────────────────────────────────────────────────────

func (h *CompanyHandler) PutRepresentative(ctx context.Context, req *connect.Request[companyv1.PutRepresentativeRequest]) (*connect.Response[companyv1.PutRepresentativeResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoRepresentative(req.Msg.Representative)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutRepresentative(ctx, uid, req.Msg.WorkspaceId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.PutRepresentativeResponse{Representative: toProtoRepresentative(out)}), nil
}

func (h *CompanyHandler) RemoveRepresentative(ctx context.Context, req *connect.Request[companyv1.RemoveRepresentativeRequest]) (*connect.Response[companyv1.RemoveRepresentativeResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteRepresentative(ctx, uid, req.Msg.WorkspaceId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemoveRepresentativeResponse{}), nil
}

func (h *CompanyHandler) PutDeclaration(ctx context.Context, req *connect.Request[companyv1.PutDeclarationRequest]) (*connect.Response[companyv1.PutDeclarationResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoDeclaration(req.Msg.Declaration)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutDeclaration(ctx, uid, req.Msg.WorkspaceId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.PutDeclarationResponse{Declaration: toProtoDeclaration(out)}), nil
}

func (h *CompanyHandler) RemoveDeclaration(ctx context.Context, req *connect.Request[companyv1.RemoveDeclarationRequest]) (*connect.Response[companyv1.RemoveDeclarationResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteDeclaration(ctx, uid, req.Msg.WorkspaceId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemoveDeclarationResponse{}), nil
}

func (h *CompanyHandler) PutNationalGround(ctx context.Context, req *connect.Request[companyv1.PutNationalGroundRequest]) (*connect.Response[companyv1.PutNationalGroundResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	in, err := fromProtoNationalGround(req.Msg.NationalGround)
	if err != nil {
		return nil, err
	}
	out, err := h.svc.PutNationalGround(ctx, uid, req.Msg.WorkspaceId, in)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.PutNationalGroundResponse{NationalGround: toProtoNationalGround(out)}), nil
}

func (h *CompanyHandler) RemoveNationalGround(ctx context.Context, req *connect.Request[companyv1.RemoveNationalGroundRequest]) (*connect.Response[companyv1.RemoveNationalGroundResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteNationalGround(ctx, uid, req.Msg.WorkspaceId, req.Msg.Id); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&companyv1.RemoveNationalGroundResponse{}), nil
}
