package workbench

import (
	"context"
	"errors"
)

type Service struct {
	workbenches WorkbenchRepository
	roles       WorkbenchRoleRepository
	members     WorkbenchMemberRepository
	users       UserLookup
	wsAccess    WorkspaceAccess
	uow         UnitOfWork
}

func NewService(
	workbenches WorkbenchRepository,
	roles WorkbenchRoleRepository,
	members WorkbenchMemberRepository,
	users UserLookup,
	wsAccess WorkspaceAccess,
	uow UnitOfWork,
) *Service {
	return &Service{
		workbenches: workbenches,
		roles:       roles,
		members:     members,
		users:       users,
		wsAccess:    wsAccess,
		uow:         uow,
	}
}

// authz is the outcome of an authorization check.
type authz struct {
	wb       Workbench
	wsName   string
	perms    Permission // effective per-workbench permissions
	elevated bool       // owner/administrator — bypasses the subset guard
}

// authorize resolves the two-layer access model for a workbench action needing
// bit `need`. Resolution order: workbench owner → workspace owner/admin override
// → not-a-workspace-member (hidden) → explicit workbench membership → shared
// baseline viewer → private/non-member (hidden).
func (s *Service) authorize(ctx context.Context, workbenchID, userID string, need Permission) (authz, error) {
	wb, err := s.workbenches.FindByID(ctx, workbenchID)
	if err != nil {
		return authz{}, err
	}
	info, err := s.wsAccess.Lookup(ctx, wb.WorkspaceID, userID)
	if err != nil {
		return authz{}, err
	}
	a := authz{wb: wb, wsName: info.Name}

	// Elevated: workbench owner, workspace owner, workspace ADMINISTRATOR, or
	// workspace MANAGE_WORKBENCHES — all bypass per-workbench checks.
	if wb.OwnerID == userID || info.IsOwner ||
		info.Perms&wsPermAdministrator != 0 || info.Perms&wsPermManageWorkbenches != 0 {
		a.perms = permAdminRole
		a.elevated = true
		return a, nil
	}
	// Must at least be a member of the parent workspace; otherwise hide existence.
	if !info.IsMember {
		return authz{}, ErrWorkbenchNotFound
	}
	// Explicit per-workbench membership.
	m, err := s.members.LoadMembership(ctx, workbenchID, userID)
	if err == nil {
		a.perms = m.Role.Permissions
		a.elevated = a.perms.Has(PermAdministrator)
		if !a.elevated && !a.perms.Has(need) {
			return authz{}, ErrForbidden
		}
		return a, nil
	}
	if !errors.Is(err, ErrNotMember) {
		return authz{}, err
	}
	// Shared workbench: workspace members with VIEW_WORKBENCHES get baseline view.
	if wb.Visibility == VisibilityShared && info.Perms&wsPermViewWorkbenches != 0 {
		a.perms = PermViewWorkbench
		if !a.perms.Has(need) {
			return authz{}, ErrForbidden
		}
		return a, nil
	}
	// Private workbench, not a member → indistinguishable from not-found.
	return authz{}, ErrWorkbenchNotFound
}

// requireWorkbenchOwner asserts the caller owns the workbench, allowing the
// workspace owner / administrator / manage-workbenches override.
func (s *Service) requireWorkbenchOwner(ctx context.Context, workbenchID, userID string) (Workbench, error) {
	wb, err := s.workbenches.FindByID(ctx, workbenchID)
	if err != nil {
		return Workbench{}, err
	}
	if wb.OwnerID == userID {
		return wb, nil
	}
	info, err := s.wsAccess.Lookup(ctx, wb.WorkspaceID, userID)
	if err != nil {
		return Workbench{}, err
	}
	if info.IsOwner || info.Perms&wsPermAdministrator != 0 || info.Perms&wsPermManageWorkbenches != 0 {
		return wb, nil
	}
	return Workbench{}, ErrOwnerOnly
}
