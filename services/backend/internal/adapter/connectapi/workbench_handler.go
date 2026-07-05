package connectapi

import (
	"context"
	"time"

	"connectrpc.com/connect"
	workbenchv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workbench/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workbench/v1/workbenchv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

type WorkbenchHandler struct{ svc *workbench.Service }

func NewWorkbenchHandler(svc *workbench.Service) *WorkbenchHandler {
	return &WorkbenchHandler{svc: svc}
}

var _ workbenchv1connect.WorkbenchServiceHandler = (*WorkbenchHandler)(nil)

func toProtoWorkbench(w workbench.Workbench) *workbenchv1.Workbench {
	return &workbenchv1.Workbench{
		Id: w.ID, WorkspaceId: w.WorkspaceID, Name: w.Name, Description: w.Description,
		Visibility: string(w.Visibility), OwnerId: w.OwnerID,
		CreatedAt: w.CreatedAt.Format(time.RFC3339), UpdatedAt: w.UpdatedAt.Format(time.RFC3339),
	}
}

func toProtoWorkbenchRole(r workbench.Role) *workbenchv1.Role {
	return &workbenchv1.Role{
		Id: r.ID, WorkbenchId: r.WorkbenchID, Name: r.Name,
		Permissions: uint64(r.Permissions), IsDefault: r.IsDefault,
		CreatedAt: r.CreatedAt.Format(time.RFC3339),
	}
}

func toProtoWorkbenchMember(m workbench.MemberView) *workbenchv1.Member {
	return &workbenchv1.Member{
		UserId: m.Member.UserID, WorkbenchId: m.Member.WorkbenchID, RoleId: m.Member.RoleID,
		RoleName: m.Role.Name, Permissions: uint64(m.Role.Permissions),
		User:    toProtoUser(m.User), // reuse the existing workspace-handler mapper
		AddedAt: m.Member.AddedAt.Format(time.RFC3339),
	}
}

func (h *WorkbenchHandler) CreateWorkbench(ctx context.Context, req *connect.Request[workbenchv1.CreateWorkbenchRequest]) (*connect.Response[workbenchv1.CreateWorkbenchResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	wb, err := h.svc.CreateWorkbench(ctx, uid, req.Msg.WorkspaceId, req.Msg.Name, req.Msg.Description, workbench.Visibility(req.Msg.Visibility))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.CreateWorkbenchResponse{Workbench: toProtoWorkbench(wb)}), nil
}

func (h *WorkbenchHandler) ListWorkbenches(ctx context.Context, req *connect.Request[workbenchv1.ListWorkbenchesRequest]) (*connect.Response[workbenchv1.ListWorkbenchesResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.svc.ListWorkbenches(ctx, uid, req.Msg.WorkspaceId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*workbenchv1.Workbench, len(list))
	for i, w := range list {
		out[i] = toProtoWorkbench(w)
	}
	return connect.NewResponse(&workbenchv1.ListWorkbenchesResponse{Workbenches: out}), nil
}

func (h *WorkbenchHandler) GetWorkbench(ctx context.Context, req *connect.Request[workbenchv1.GetWorkbenchRequest]) (*connect.Response[workbenchv1.GetWorkbenchResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	wb, perms, wsName, err := h.svc.GetWorkbench(ctx, uid, req.Msg.WorkbenchId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.GetWorkbenchResponse{
		Workbench: toProtoWorkbench(wb), MyPermissions: uint64(perms), WorkspaceName: wsName,
	}), nil
}

func (h *WorkbenchHandler) UpdateWorkbench(ctx context.Context, req *connect.Request[workbenchv1.UpdateWorkbenchRequest]) (*connect.Response[workbenchv1.UpdateWorkbenchResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	wb, err := h.svc.UpdateWorkbench(ctx, uid, req.Msg.WorkbenchId, req.Msg.Name, req.Msg.Description)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.UpdateWorkbenchResponse{Workbench: toProtoWorkbench(wb)}), nil
}

func (h *WorkbenchHandler) ChangeVisibility(ctx context.Context, req *connect.Request[workbenchv1.ChangeVisibilityRequest]) (*connect.Response[workbenchv1.ChangeVisibilityResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	wb, err := h.svc.ChangeVisibility(ctx, uid, req.Msg.WorkbenchId, workbench.Visibility(req.Msg.Visibility))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.ChangeVisibilityResponse{Workbench: toProtoWorkbench(wb)}), nil
}

func (h *WorkbenchHandler) DeleteWorkbench(ctx context.Context, req *connect.Request[workbenchv1.DeleteWorkbenchRequest]) (*connect.Response[workbenchv1.DeleteWorkbenchResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteWorkbench(ctx, uid, req.Msg.WorkbenchId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.DeleteWorkbenchResponse{}), nil
}

func (h *WorkbenchHandler) TransferWorkbenchOwnership(ctx context.Context, req *connect.Request[workbenchv1.TransferWorkbenchOwnershipRequest]) (*connect.Response[workbenchv1.TransferWorkbenchOwnershipResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.TransferOwnership(ctx, uid, req.Msg.WorkbenchId, req.Msg.NewOwnerUserId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.TransferWorkbenchOwnershipResponse{}), nil
}

func (h *WorkbenchHandler) LeaveWorkbench(ctx context.Context, req *connect.Request[workbenchv1.LeaveWorkbenchRequest]) (*connect.Response[workbenchv1.LeaveWorkbenchResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.LeaveWorkbench(ctx, uid, req.Msg.WorkbenchId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.LeaveWorkbenchResponse{}), nil
}

func (h *WorkbenchHandler) ListWorkbenchMembers(ctx context.Context, req *connect.Request[workbenchv1.ListWorkbenchMembersRequest]) (*connect.Response[workbenchv1.ListWorkbenchMembersResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	members, err := h.svc.ListMembers(ctx, uid, req.Msg.WorkbenchId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*workbenchv1.Member, len(members))
	for i, m := range members {
		out[i] = toProtoWorkbenchMember(m)
	}
	return connect.NewResponse(&workbenchv1.ListWorkbenchMembersResponse{Members: out}), nil
}

func (h *WorkbenchHandler) AddWorkbenchMember(ctx context.Context, req *connect.Request[workbenchv1.AddWorkbenchMemberRequest]) (*connect.Response[workbenchv1.AddWorkbenchMemberResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.AddMember(ctx, uid, req.Msg.WorkbenchId, req.Msg.UserId, req.Msg.RoleId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.AddWorkbenchMemberResponse{Member: toProtoWorkbenchMember(m)}), nil
}

func (h *WorkbenchHandler) ChangeWorkbenchMemberRole(ctx context.Context, req *connect.Request[workbenchv1.ChangeWorkbenchMemberRoleRequest]) (*connect.Response[workbenchv1.ChangeWorkbenchMemberRoleResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.ChangeMemberRole(ctx, uid, req.Msg.WorkbenchId, req.Msg.UserId, req.Msg.RoleId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.ChangeWorkbenchMemberRoleResponse{Member: toProtoWorkbenchMember(m)}), nil
}

func (h *WorkbenchHandler) RemoveWorkbenchMember(ctx context.Context, req *connect.Request[workbenchv1.RemoveWorkbenchMemberRequest]) (*connect.Response[workbenchv1.RemoveWorkbenchMemberResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RemoveMember(ctx, uid, req.Msg.WorkbenchId, req.Msg.UserId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.RemoveWorkbenchMemberResponse{}), nil
}

func (h *WorkbenchHandler) ListWorkbenchRoles(ctx context.Context, req *connect.Request[workbenchv1.ListWorkbenchRolesRequest]) (*connect.Response[workbenchv1.ListWorkbenchRolesResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := h.svc.ListRoles(ctx, uid, req.Msg.WorkbenchId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*workbenchv1.Role, len(roles))
	for i, r := range roles {
		out[i] = toProtoWorkbenchRole(r)
	}
	return connect.NewResponse(&workbenchv1.ListWorkbenchRolesResponse{Roles: out}), nil
}

func (h *WorkbenchHandler) CreateWorkbenchRole(ctx context.Context, req *connect.Request[workbenchv1.CreateWorkbenchRoleRequest]) (*connect.Response[workbenchv1.CreateWorkbenchRoleResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.svc.CreateRole(ctx, uid, req.Msg.WorkbenchId, req.Msg.Name, workbench.Permission(req.Msg.Permissions))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.CreateWorkbenchRoleResponse{Role: toProtoWorkbenchRole(r)}), nil
}

func (h *WorkbenchHandler) UpdateWorkbenchRole(ctx context.Context, req *connect.Request[workbenchv1.UpdateWorkbenchRoleRequest]) (*connect.Response[workbenchv1.UpdateWorkbenchRoleResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.svc.UpdateRole(ctx, uid, req.Msg.WorkbenchId, req.Msg.RoleId, req.Msg.Name, workbench.Permission(req.Msg.Permissions))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.UpdateWorkbenchRoleResponse{Role: toProtoWorkbenchRole(r)}), nil
}

func (h *WorkbenchHandler) DeleteWorkbenchRole(ctx context.Context, req *connect.Request[workbenchv1.DeleteWorkbenchRoleRequest]) (*connect.Response[workbenchv1.DeleteWorkbenchRoleResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteRole(ctx, uid, req.Msg.WorkbenchId, req.Msg.RoleId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workbenchv1.DeleteWorkbenchRoleResponse{}), nil
}
