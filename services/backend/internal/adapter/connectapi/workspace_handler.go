package connectapi

import (
	"context"
	"time"

	"connectrpc.com/connect"
	workspacev1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workspace/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workspace/v1/workspacev1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type WorkspaceHandler struct{ svc *workspace.Service }

func NewWorkspaceHandler(svc *workspace.Service) *WorkspaceHandler {
	return &WorkspaceHandler{svc: svc}
}

var _ workspacev1connect.WorkspaceServiceHandler = (*WorkspaceHandler)(nil)

func requireUser(ctx context.Context) (string, error) {
	id, ok := UserIDFromContext(ctx)
	if !ok {
		return "", connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return id, nil
}

// ── Workspace lifecycle ─────────────────────────────────────────────────────

func (h *WorkspaceHandler) CreateWorkspace(ctx context.Context, req *connect.Request[workspacev1.CreateWorkspaceRequest]) (*connect.Response[workspacev1.CreateWorkspaceResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := h.svc.CreateWorkspace(ctx, uid, req.Msg.Name, req.Msg.Slug)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.CreateWorkspaceResponse{Workspace: toProtoWorkspace(ws)}), nil
}

func (h *WorkspaceHandler) ListMyWorkspaces(ctx context.Context, _ *connect.Request[workspacev1.ListMyWorkspacesRequest]) (*connect.Response[workspacev1.ListMyWorkspacesResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	wss, err := h.svc.ListMyWorkspaces(ctx, uid)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*workspacev1.Workspace, len(wss))
	for i, w := range wss {
		out[i] = toProtoWorkspace(w)
	}
	return connect.NewResponse(&workspacev1.ListMyWorkspacesResponse{Workspaces: out}), nil
}

func (h *WorkspaceHandler) GetWorkspace(ctx context.Context, req *connect.Request[workspacev1.GetWorkspaceRequest]) (*connect.Response[workspacev1.GetWorkspaceResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	ws, perms, err := h.svc.GetWorkspace(ctx, uid, req.Msg.WorkspaceId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.GetWorkspaceResponse{
		Workspace:     toProtoWorkspace(ws),
		MyPermissions: uint64(perms),
	}), nil
}

func (h *WorkspaceHandler) UpdateWorkspace(ctx context.Context, req *connect.Request[workspacev1.UpdateWorkspaceRequest]) (*connect.Response[workspacev1.UpdateWorkspaceResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := h.svc.UpdateWorkspace(ctx, uid, req.Msg.WorkspaceId, req.Msg.Name, req.Msg.Slug)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.UpdateWorkspaceResponse{Workspace: toProtoWorkspace(ws)}), nil
}

func (h *WorkspaceHandler) DeleteWorkspace(ctx context.Context, req *connect.Request[workspacev1.DeleteWorkspaceRequest]) (*connect.Response[workspacev1.DeleteWorkspaceResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteWorkspace(ctx, uid, req.Msg.WorkspaceId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.DeleteWorkspaceResponse{}), nil
}

func (h *WorkspaceHandler) TransferOwnership(ctx context.Context, req *connect.Request[workspacev1.TransferOwnershipRequest]) (*connect.Response[workspacev1.TransferOwnershipResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.TransferOwnership(ctx, uid, req.Msg.WorkspaceId, req.Msg.NewOwnerUserId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.TransferOwnershipResponse{}), nil
}

func (h *WorkspaceHandler) LeaveWorkspace(ctx context.Context, req *connect.Request[workspacev1.LeaveWorkspaceRequest]) (*connect.Response[workspacev1.LeaveWorkspaceResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.LeaveWorkspace(ctx, uid, req.Msg.WorkspaceId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.LeaveWorkspaceResponse{}), nil
}

// ── Members ─────────────────────────────────────────────────────────────────

func (h *WorkspaceHandler) ListMembers(ctx context.Context, req *connect.Request[workspacev1.ListMembersRequest]) (*connect.Response[workspacev1.ListMembersResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	members, err := h.svc.ListMembers(ctx, uid, req.Msg.WorkspaceId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*workspacev1.Member, len(members))
	for i, m := range members {
		out[i] = toProtoMember(m)
	}
	return connect.NewResponse(&workspacev1.ListMembersResponse{Members: out}), nil
}

func (h *WorkspaceHandler) ChangeMemberRole(ctx context.Context, req *connect.Request[workspacev1.ChangeMemberRoleRequest]) (*connect.Response[workspacev1.ChangeMemberRoleResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	mv, err := h.svc.ChangeMemberRole(ctx, uid, req.Msg.WorkspaceId, req.Msg.UserId, req.Msg.RoleId)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.ChangeMemberRoleResponse{Member: toProtoMember(mv)}), nil
}

func (h *WorkspaceHandler) RemoveMember(ctx context.Context, req *connect.Request[workspacev1.RemoveMemberRequest]) (*connect.Response[workspacev1.RemoveMemberResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RemoveMember(ctx, uid, req.Msg.WorkspaceId, req.Msg.UserId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.RemoveMemberResponse{}), nil
}

// ── Roles ───────────────────────────────────────────────────────────────────

func (h *WorkspaceHandler) ListRoles(ctx context.Context, req *connect.Request[workspacev1.ListRolesRequest]) (*connect.Response[workspacev1.ListRolesResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := h.svc.ListRoles(ctx, uid, req.Msg.WorkspaceId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*workspacev1.Role, len(roles))
	for i, r := range roles {
		out[i] = toProtoRole(r)
	}
	return connect.NewResponse(&workspacev1.ListRolesResponse{Roles: out}), nil
}

func (h *WorkspaceHandler) CreateRole(ctx context.Context, req *connect.Request[workspacev1.CreateRoleRequest]) (*connect.Response[workspacev1.CreateRoleResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	role, err := h.svc.CreateRole(ctx, uid, req.Msg.WorkspaceId, req.Msg.Name, workspace.Permission(req.Msg.Permissions))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.CreateRoleResponse{Role: toProtoRole(role)}), nil
}

func (h *WorkspaceHandler) UpdateRole(ctx context.Context, req *connect.Request[workspacev1.UpdateRoleRequest]) (*connect.Response[workspacev1.UpdateRoleResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	role, err := h.svc.UpdateRole(ctx, uid, req.Msg.WorkspaceId, req.Msg.RoleId, req.Msg.Name, workspace.Permission(req.Msg.Permissions))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.UpdateRoleResponse{Role: toProtoRole(role)}), nil
}

func (h *WorkspaceHandler) DeleteRole(ctx context.Context, req *connect.Request[workspacev1.DeleteRoleRequest]) (*connect.Response[workspacev1.DeleteRoleResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteRole(ctx, uid, req.Msg.WorkspaceId, req.Msg.RoleId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.DeleteRoleResponse{}), nil
}

// ── Email invitations ───────────────────────────────────────────────────────

func (h *WorkspaceHandler) InviteByEmail(ctx context.Context, req *connect.Request[workspacev1.InviteByEmailRequest]) (*connect.Response[workspacev1.InviteByEmailResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	inv, err := h.svc.InviteByEmail(ctx, uid, req.Msg.WorkspaceId, req.Msg.Email, req.Msg.RoleId, req.Msg.Locale)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.InviteByEmailResponse{Invitation: toProtoEmailInvitation(inv)}), nil
}

func (h *WorkspaceHandler) ListEmailInvitations(ctx context.Context, req *connect.Request[workspacev1.ListEmailInvitationsRequest]) (*connect.Response[workspacev1.ListEmailInvitationsResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	invs, err := h.svc.ListEmailInvitations(ctx, uid, req.Msg.WorkspaceId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*workspacev1.EmailInvitation, len(invs))
	for i, inv := range invs {
		out[i] = toProtoEmailInvitation(inv)
	}
	return connect.NewResponse(&workspacev1.ListEmailInvitationsResponse{Invitations: out}), nil
}

func (h *WorkspaceHandler) RevokeEmailInvitation(ctx context.Context, req *connect.Request[workspacev1.RevokeEmailInvitationRequest]) (*connect.Response[workspacev1.RevokeEmailInvitationResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RevokeEmailInvitation(ctx, uid, req.Msg.WorkspaceId, req.Msg.InvitationId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.RevokeEmailInvitationResponse{}), nil
}

func (h *WorkspaceHandler) AcceptEmailInvite(ctx context.Context, req *connect.Request[workspacev1.AcceptEmailInviteRequest]) (*connect.Response[workspacev1.AcceptEmailInviteResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := h.svc.AcceptEmailInvite(ctx, uid, req.Msg.Token)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.AcceptEmailInviteResponse{Workspace: toProtoWorkspace(ws)}), nil
}

func (h *WorkspaceHandler) PreviewEmailInvite(ctx context.Context, req *connect.Request[workspacev1.PreviewEmailInviteRequest]) (*connect.Response[workspacev1.PreviewEmailInviteResponse], error) {
	p, err := h.svc.PreviewEmailInvite(ctx, req.Msg.Token)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.PreviewEmailInviteResponse{
		WorkspaceName: p.WorkspaceName,
		RoleName:      p.RoleName,
		Email:         p.Email,
		Valid:         p.Valid,
	}), nil
}

// ── Invite links ────────────────────────────────────────────────────────────

func (h *WorkspaceHandler) CreateInviteLink(ctx context.Context, req *connect.Request[workspacev1.CreateInviteLinkRequest]) (*connect.Response[workspacev1.CreateInviteLinkResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	var expiresAt *time.Time
	if s := req.Msg.ExpiresAt; s != "" {
		t, perr := time.Parse(time.RFC3339, s)
		if perr != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, perr)
		}
		expiresAt = &t
	}
	link, err := h.svc.CreateInviteLink(ctx, uid, req.Msg.WorkspaceId, req.Msg.RoleId, req.Msg.MaxUses, expiresAt)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.CreateInviteLinkResponse{Link: toProtoInviteLink(link)}), nil
}

func (h *WorkspaceHandler) ListInviteLinks(ctx context.Context, req *connect.Request[workspacev1.ListInviteLinksRequest]) (*connect.Response[workspacev1.ListInviteLinksResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	links, err := h.svc.ListInviteLinks(ctx, uid, req.Msg.WorkspaceId)
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*workspacev1.InviteLink, len(links))
	for i, l := range links {
		out[i] = toProtoInviteLink(l)
	}
	return connect.NewResponse(&workspacev1.ListInviteLinksResponse{Links: out}), nil
}

func (h *WorkspaceHandler) RevokeInviteLink(ctx context.Context, req *connect.Request[workspacev1.RevokeInviteLinkRequest]) (*connect.Response[workspacev1.RevokeInviteLinkResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RevokeInviteLink(ctx, uid, req.Msg.WorkspaceId, req.Msg.LinkId); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.RevokeInviteLinkResponse{}), nil
}

func (h *WorkspaceHandler) PreviewInviteLink(ctx context.Context, req *connect.Request[workspacev1.PreviewInviteLinkRequest]) (*connect.Response[workspacev1.PreviewInviteLinkResponse], error) {
	p, err := h.svc.PreviewInviteLink(ctx, req.Msg.Code)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.PreviewInviteLinkResponse{
		WorkspaceName: p.WorkspaceName,
		RoleName:      p.RoleName,
		Valid:         p.Valid,
	}), nil
}

func (h *WorkspaceHandler) JoinViaInviteLink(ctx context.Context, req *connect.Request[workspacev1.JoinViaInviteLinkRequest]) (*connect.Response[workspacev1.JoinViaInviteLinkResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	ws, err := h.svc.JoinViaInviteLink(ctx, uid, req.Msg.Code)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&workspacev1.JoinViaInviteLinkResponse{Workspace: toProtoWorkspace(ws)}), nil
}

// ── proto mappers ───────────────────────────────────────────────────────────

func toProtoWorkspace(w workspace.Workspace) *workspacev1.Workspace {
	return &workspacev1.Workspace{
		Id:        w.ID,
		Name:      w.Name,
		Slug:      w.Slug,
		OwnerId:   w.OwnerID,
		CreatedAt: w.CreatedAt.Format(time.RFC3339),
		UpdatedAt: w.UpdatedAt.Format(time.RFC3339),
	}
}

func toProtoRole(r workspace.Role) *workspacev1.Role {
	return &workspacev1.Role{
		Id:          r.ID,
		WorkspaceId: r.WorkspaceID,
		Name:        r.Name,
		Permissions: uint64(r.Permissions),
		IsDefault:   r.IsDefault,
		CreatedAt:   r.CreatedAt.Format(time.RFC3339),
	}
}

func toProtoMember(mv workspace.MemberView) *workspacev1.Member {
	return &workspacev1.Member{
		UserId:      mv.Member.UserID,
		WorkspaceId: mv.Member.WorkspaceID,
		RoleId:      mv.Member.RoleID,
		RoleName:    mv.Role.Name,
		Permissions: uint64(mv.Role.Permissions),
		User:        toProtoUser(mv.User),
		JoinedAt:    mv.Member.JoinedAt.Format(time.RFC3339),
	}
}

func toProtoEmailInvitation(inv workspace.EmailInvitation) *workspacev1.EmailInvitation {
	return &workspacev1.EmailInvitation{
		Id:          inv.ID,
		WorkspaceId: inv.WorkspaceID,
		Email:       inv.Email,
		RoleId:      inv.RoleID,
		InvitedBy:   inv.InvitedBy,
		ExpiresAt:   inv.ExpiresAt.Format(time.RFC3339),
		CreatedAt:   inv.CreatedAt.Format(time.RFC3339),
	}
}

func toProtoInviteLink(l workspace.InviteLink) *workspacev1.InviteLink {
	expiresAt := ""
	if l.ExpiresAt != nil {
		expiresAt = l.ExpiresAt.Format(time.RFC3339)
	}
	return &workspacev1.InviteLink{
		Id:          l.ID,
		WorkspaceId: l.WorkspaceID,
		Code:        l.Code,
		RoleId:      l.RoleID,
		CreatedBy:   l.CreatedBy,
		MaxUses:     l.MaxUses,
		UseCount:    l.UseCount,
		ExpiresAt:   expiresAt,
		Revoked:     l.Revoked,
		CreatedAt:   l.CreatedAt.Format(time.RFC3339),
	}
}
