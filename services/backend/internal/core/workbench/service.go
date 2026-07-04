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

// CreateWorkbench creates a workbench, seeds a "Manager" (all bits) and default
// "Viewer" role, and adds the creator as a Manager member — all atomically.
// Requires the caller be a workspace member with CREATE_WORKBENCH (or a
// workspace owner/admin/manage-workbenches).
func (s *Service) CreateWorkbench(ctx context.Context, userID, workspaceID, name, description string, visibility Visibility) (Workbench, error) {
	info, err := s.wsAccess.Lookup(ctx, workspaceID, userID)
	if err != nil {
		return Workbench{}, err
	}
	if !info.IsMember && !info.IsOwner {
		return Workbench{}, ErrNotWorkspaceMember
	}
	allowed := info.IsOwner ||
		info.Perms&wsPermAdministrator != 0 ||
		info.Perms&wsPermManageWorkbenches != 0 ||
		info.Perms&wsPermCreateWorkbench != 0
	if !allowed {
		return Workbench{}, ErrForbidden
	}
	if visibility != VisibilityShared {
		visibility = VisibilityPrivate
	}

	var created Workbench
	err = s.uow.Do(ctx, func(r Repos) error {
		wb, err := r.Workbenches.Create(ctx, Workbench{
			WorkspaceID: workspaceID, Name: name, Description: description,
			Visibility: visibility, OwnerID: userID,
		})
		if err != nil {
			return err
		}
		mgr, err := r.Roles.Create(ctx, Role{WorkbenchID: wb.ID, Name: "Manager", Permissions: permAdminRole})
		if err != nil {
			return err
		}
		if _, err := r.Roles.Create(ctx, Role{WorkbenchID: wb.ID, Name: "Viewer", Permissions: PermViewWorkbench, IsDefault: true}); err != nil {
			return err
		}
		if _, err := r.Members.Add(ctx, Member{WorkbenchID: wb.ID, UserID: userID, RoleID: mgr.ID}); err != nil {
			return err
		}
		created = wb
		return nil
	})
	if err != nil {
		return Workbench{}, err
	}
	return created, nil
}

// ListWorkbenches returns the workbenches in a workspace the caller may see:
// owner / explicit member / shared (with VIEW_WORKBENCHES) / all when the caller
// is a workspace owner/admin.
func (s *Service) ListWorkbenches(ctx context.Context, userID, workspaceID string) ([]Workbench, error) {
	info, err := s.wsAccess.Lookup(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !info.IsMember && !info.IsOwner {
		return nil, ErrNotMember
	}
	all, err := s.workbenches.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	admin := info.IsOwner || info.Perms&wsPermAdministrator != 0 || info.Perms&wsPermManageWorkbenches != 0
	canView := info.Perms&wsPermViewWorkbenches != 0
	out := make([]Workbench, 0, len(all))
	for _, wb := range all {
		switch {
		case admin, wb.OwnerID == userID:
			out = append(out, wb)
			continue
		case wb.Visibility == VisibilityShared && canView:
			out = append(out, wb)
			continue
		}
		if _, err := s.members.Find(ctx, wb.ID, userID); err == nil {
			out = append(out, wb)
		} else if !errors.Is(err, ErrNotMember) {
			return nil, err
		}
	}
	return out, nil
}

// GetWorkbench returns the workbench, the caller's effective permissions, and
// the parent workspace name (for the breadcrumb).
func (s *Service) GetWorkbench(ctx context.Context, userID, workbenchID string) (Workbench, Permission, string, error) {
	a, err := s.authorize(ctx, workbenchID, userID, PermViewWorkbench)
	if err != nil {
		return Workbench{}, 0, "", err
	}
	return a.wb, a.perms, a.wsName, nil
}

func (s *Service) UpdateWorkbench(ctx context.Context, userID, workbenchID, name, description string) (Workbench, error) {
	if _, err := s.authorize(ctx, workbenchID, userID, PermManageWorkbench); err != nil {
		return Workbench{}, err
	}
	return s.workbenches.Update(ctx, workbenchID, name, description)
}

func (s *Service) ChangeVisibility(ctx context.Context, userID, workbenchID string, v Visibility) (Workbench, error) {
	if v != VisibilityShared && v != VisibilityPrivate {
		return Workbench{}, ErrForbidden
	}
	if _, err := s.authorize(ctx, workbenchID, userID, PermManageWorkbench); err != nil {
		return Workbench{}, err
	}
	return s.workbenches.UpdateVisibility(ctx, workbenchID, v)
}

func (s *Service) DeleteWorkbench(ctx context.Context, userID, workbenchID string) error {
	wb, err := s.requireWorkbenchOwner(ctx, workbenchID, userID)
	if err != nil {
		return err
	}
	return s.workbenches.Delete(ctx, wb.ID)
}

func (s *Service) TransferOwnership(ctx context.Context, userID, workbenchID, newOwnerID string) error {
	wb, err := s.requireWorkbenchOwner(ctx, workbenchID, userID)
	if err != nil {
		return err
	}
	// The new owner must be a member of the parent workspace.
	info, err := s.wsAccess.Lookup(ctx, wb.WorkspaceID, newOwnerID)
	if err != nil {
		return err
	}
	if !info.IsMember && !info.IsOwner {
		return ErrNotWorkspaceMember
	}
	return s.workbenches.UpdateOwner(ctx, workbenchID, newOwnerID)
}

// LeaveWorkbench removes the caller's own membership. The owner must transfer
// ownership first.
func (s *Service) LeaveWorkbench(ctx context.Context, userID, workbenchID string) error {
	wb, err := s.workbenches.FindByID(ctx, workbenchID)
	if err != nil {
		return err
	}
	if wb.OwnerID == userID {
		return ErrLastOwner
	}
	if _, err := s.members.Find(ctx, workbenchID, userID); err != nil {
		return err
	}
	return s.members.Remove(ctx, workbenchID, userID)
}
