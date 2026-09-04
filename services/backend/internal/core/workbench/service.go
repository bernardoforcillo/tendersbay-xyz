package workbench

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/rbac"
)

type scopeService = scope.Service[Workbench, Member, *Workbench, *Member]

type Service struct {
	sc       *scopeService
	store    Store
	repo     Repository
	users    UserLookup
	wsAccess WorkspaceAccess
}

// NewService wires the nested scope. parent is the workspace's own scope
// service, which supplies the standing this one inherits from.
//
// Two options make the nesting work, and both are load-bearing:
//
//   - WithContainerResource turns CreateContainer into a "workbench:create"
//     check against the WORKSPACE. That grant is declared on the workspace's
//     surface (workspace.Statements), which is what makes
//     workspace.PermCreateWorkbench mean something. Without it, creating a
//     workbench would silently require elevated standing in the workspace.
//   - InheritWhen projects the workspace's "workbench:manage" onto elevation
//     here, so a workspace administrator administers every workbench in it.
//     scope's own default (InheritElevation) would carry only workspace
//     OWNERS across, which is narrower than the rule this product has.
func NewService(
	ac *access.Access,
	parent scope.ParentScope,
	store Store,
	repo Repository,
	users UserLookup,
	wsAccess WorkspaceAccess,
) *Service {
	sc := scope.New[Workbench, Member](ac, store,
		scope.WithContainerResource(ResourceWorkbench),
		scope.WithParent(parent, scope.InheritWhen(ResourceWorkbench, ActionManage)),
	)
	return &Service{sc: sc, store: store, repo: repo, users: users, wsAccess: wsAccess}
}

func actor(ctx context.Context, userID, containerID string) context.Context {
	return scope.WithScope(scope.WithSubject(ctx, userID), containerID)
}

// scopeErrors is this domain's vocabulary for authlayer's scope sentinels.
// Conflict has no entry: a workbench declares no unique constraint, so there is
// no conflict of its own to report — see rbac.Errors on a nil field.
var scopeErrors = rbac.Errors{
	NotFound:            ErrWorkbenchNotFound,
	NotMember:           ErrNotMember,
	NotParentMember:     ErrNotWorkspaceMember,
	Forbidden:           ErrForbidden,
	PrivilegeEscalation: ErrPrivilegeEscalation,
	RoleNotFound:        ErrRoleNotFound,
	RoleInUse:           ErrRoleInUse,
	DefaultRole:         ErrDefaultRole,
	LastOwner:           ErrLastOwner,
	OwnerOnly:           ErrOwnerOnly,
	AlreadyMember:       ErrAlreadyMember,
	RoleKeyTaken:        ErrRoleKeyTaken,
}

func mapErr(err error) error { return scopeErrors.Translate(err) }

// ── Authorization ───────────────────────────────────────────────────────────

// authorize resolves a read or a gated action on one workbench, and reports the
// caller's effective permissions.
//
// The first rung is the whole of authlayer's ladder: workbench owner, inherited
// elevation from the workspace, explicit membership, the member's role. Only
// when that reports no standing at all does this package's own rule apply — a
// SHARED workbench is visible to a workspace member who may see shared
// workbenches. That rule cannot live in the engine because it turns on the
// visibility column, so it lives here, after the engine has had its say.
//
// Standing resolved that way is then re-checked against the parent workspace,
// and that check is load-bearing rather than defensive. A workbench membership
// row OUTLIVES the workspace membership that justified it: nothing cascades
// workbench_members when workspace.RemoveMember runs (its foreign key is to
// users), and authlayer's MembersFromParent policy says in its own doc that it
// constrains AddMember only, never Standing. Without the re-check, a colleague
// removed from the workspace keeps full access to every workbench they were
// explicitly added to. Keying it on the standing being unelevated would not do
// either: scope sets elevated for any member whose permissions are full, so a
// removed workbench MANAGER — the most privileged orphan there is — would be
// exactly the one waved through.
//
// The workbench's own owner is the deliberate exception, as it was before the
// nested scope: an owner locked out of their workbench by losing workspace
// membership can no longer transfer it to anyone.
//
// A caller with no way in gets ErrWorkbenchNotFound rather than ErrForbidden,
// on both the not-a-workspace-member and the private-workbench paths: whether a
// particular workbench exists is itself something they must not learn.
func (s *Service) authorize(ctx context.Context, workbenchID, userID string, need Permission) (Permission, error) {
	perms, elevated, err := s.sc.Standing(ctx, workbenchID, userID)
	switch {
	case err == nil:
		if err := s.requireParentStanding(ctx, workbenchID, userID); err != nil {
			return 0, err
		}
		mask := maskOf(perms, elevated)
		if !elevated && !mask.Has(need) {
			return 0, ErrForbidden
		}
		return mask, nil
	case errors.Is(err, scope.ErrNotMember):
		// No standing of its own and nothing inherited — fall through.
	default:
		return 0, mapErr(err)
	}

	wb, err := s.sc.Container(ctx, workbenchID)
	if err != nil {
		return 0, mapErr(err)
	}
	info, err := s.wsAccess.Lookup(ctx, wb.WorkspaceID, userID)
	if err != nil {
		return 0, err
	}
	if !info.IsMember {
		return 0, ErrWorkbenchNotFound
	}
	if wb.Visibility == VisibilityShared && info.MayViewShared {
		if !PermViewWorkbench.Has(need) {
			return 0, ErrForbidden
		}
		return PermViewWorkbench, nil
	}
	return 0, ErrWorkbenchNotFound
}

// requireParentStanding refuses a caller who no longer belongs to the workspace
// that holds the workbench. See authorize for why standing on the workbench
// alone is not enough to answer with.
//
// It costs the container read and one workspace lookup, which is the query the
// nested-scope rewrite had removed from this path. Correctness buys it back:
// the alternative is honouring a membership row whose justification is gone.
func (s *Service) requireParentStanding(ctx context.Context, workbenchID, userID string) error {
	wb, err := s.sc.Container(ctx, workbenchID)
	if err != nil {
		return mapErr(err)
	}
	if wb.OwnerID == userID {
		return nil
	}
	info, err := s.wsAccess.Lookup(ctx, wb.WorkspaceID, userID)
	if err != nil {
		return err
	}
	if !info.IsMember {
		return ErrWorkbenchNotFound
	}
	return nil
}

// requireWorkbenchOwner gates the two owner-only actions. It is not expressed
// as a grant — see permissionGrants — so the workspace-level override has to be
// asked for explicitly here rather than falling out of the engine.
func (s *Service) requireWorkbenchOwner(ctx context.Context, workbenchID, userID string) (Workbench, error) {
	wb, err := s.sc.Container(ctx, workbenchID)
	if err != nil {
		return Workbench{}, mapErr(err)
	}
	if wb.OwnerID == userID {
		return wb, nil
	}
	info, err := s.wsAccess.Lookup(ctx, wb.WorkspaceID, userID)
	if err != nil {
		return Workbench{}, err
	}
	if info.MayManageAll {
		return wb, nil
	}
	return Workbench{}, ErrOwnerOnly
}

// ── Workbench lifecycle ─────────────────────────────────────────────────────

// CreateWorkbench creates a workbench owned by userID, who becomes its first
// member with the owner role. The permission check is the nesting's: the engine
// asks the workspace whether the caller holds workbench:create.
func (s *Service) CreateWorkbench(ctx context.Context, userID, workspaceID, name, description string, visibility Visibility) (Workbench, error) {
	if visibility != VisibilityShared {
		visibility = VisibilityPrivate
	}
	wb, err := s.sc.CreateContainer(actor(ctx, userID, workspaceID), Workbench{
		Name: name, Description: description, Visibility: visibility,
	})
	if err != nil {
		return Workbench{}, mapErr(err)
	}
	return wb, nil
}

// ListWorkbenches returns the workbenches in a workspace the caller may see.
// The filter mirrors authorize's ladder, applied to a list: administrators see
// everything, an owner sees their own, a shared workbench needs only the
// workspace-level right, and anything else needs membership.
func (s *Service) ListWorkbenches(ctx context.Context, userID, workspaceID string) ([]Workbench, error) {
	info, err := s.wsAccess.Lookup(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !info.IsMember {
		return nil, ErrNotMember
	}
	all, err := s.repo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// The memberships are read once, not once per workbench: the previous
	// version asked FindMember for every row it could not decide from the
	// workspace alone, which is a query per workbench in the workspace on the
	// commonest listing in the product.
	joined := map[string]bool{}
	if !info.MayManageAll {
		mine, err := s.store.ListUserContainers(ctx, userID)
		if err != nil {
			return nil, mapErr(err)
		}
		for _, wb := range mine {
			joined[wb.ID] = true
		}
	}

	out := make([]Workbench, 0, len(all))
	for _, wb := range all {
		switch {
		case info.MayManageAll,
			wb.OwnerID == userID,
			joined[wb.ID],
			wb.Visibility == VisibilityShared && info.MayViewShared:
			out = append(out, wb)
		}
	}
	return out, nil
}

// CanAccessWorkbench returns nil when userID may view workbenchID. It reuses
// the exact authorize path GetWorkbench uses, so an external caller (the agent
// chat service gating a workbench-scoped chat) gets an identical decision.
func (s *Service) CanAccessWorkbench(ctx context.Context, userID, workbenchID string) error {
	_, err := s.authorize(ctx, workbenchID, userID, PermViewWorkbench)
	return err
}

// CanManageWorkbench mirrors CanAccessWorkbench against PermManageWorkbench, so
// the bid domain's write RPCs get the same decision UpdateWorkbench uses.
func (s *Service) CanManageWorkbench(ctx context.Context, userID, workbenchID string) error {
	_, err := s.authorize(ctx, workbenchID, userID, PermManageWorkbench)
	return err
}

// WorkspaceOf resolves a workbench's parent workspace id. It performs no
// authorization of its own — callers use it only to scope a downstream lookup
// after their own access check.
func (s *Service) WorkspaceOf(ctx context.Context, workbenchID string) (string, error) {
	wb, err := s.sc.Container(ctx, workbenchID)
	if err != nil {
		return "", mapErr(err)
	}
	return wb.WorkspaceID, nil
}

// AccessibleWorkbenchIDs returns the workbench IDs in workspaceID that userID
// may view, indexed for an O(1) membership test.
func (s *Service) AccessibleWorkbenchIDs(ctx context.Context, userID, workspaceID string) (map[string]struct{}, error) {
	wbs, err := s.ListWorkbenches(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	ids := make(map[string]struct{}, len(wbs))
	for _, wb := range wbs {
		ids[wb.ID] = struct{}{}
	}
	return ids, nil
}

// GetWorkbench returns the workbench, the caller's effective permissions, and
// the parent workspace name for the breadcrumb.
func (s *Service) GetWorkbench(ctx context.Context, userID, workbenchID string) (Workbench, Permission, string, error) {
	perms, err := s.authorize(ctx, workbenchID, userID, PermViewWorkbench)
	if err != nil {
		return Workbench{}, 0, "", err
	}
	wb, err := s.sc.Container(ctx, workbenchID)
	if err != nil {
		return Workbench{}, 0, "", mapErr(err)
	}
	name, err := s.wsAccess.WorkspaceName(ctx, wb.WorkspaceID)
	if err != nil {
		return Workbench{}, 0, "", err
	}
	return wb, perms, name, nil
}

func (s *Service) UpdateWorkbench(ctx context.Context, userID, workbenchID, name, description string) (Workbench, error) {
	if _, err := s.authorize(ctx, workbenchID, userID, PermManageWorkbench); err != nil {
		return Workbench{}, err
	}
	return s.repo.UpdateDetails(ctx, workbenchID, name, description)
}

func (s *Service) ChangeVisibility(ctx context.Context, userID, workbenchID string, v Visibility) (Workbench, error) {
	if v != VisibilityShared && v != VisibilityPrivate {
		return Workbench{}, ErrForbidden
	}
	if _, err := s.authorize(ctx, workbenchID, userID, PermManageWorkbench); err != nil {
		return Workbench{}, err
	}
	return s.repo.UpdateVisibility(ctx, workbenchID, v)
}

func (s *Service) DeleteWorkbench(ctx context.Context, userID, workbenchID string) error {
	wb, err := s.requireWorkbenchOwner(ctx, workbenchID, userID)
	if err != nil {
		return err
	}
	return s.repo.Delete(ctx, wb.ID)
}

// TransferOwnership hands a workbench to another WORKSPACE member — not
// necessarily one of its own members, which is why it does not use scope's own
// TransferOwnership: that one is owner-only and requires the target to already
// belong to the container. Both differences are product rules, so both are
// enforced here.
func (s *Service) TransferOwnership(ctx context.Context, userID, workbenchID, newOwnerID string) error {
	wb, err := s.requireWorkbenchOwner(ctx, workbenchID, userID)
	if err != nil {
		return err
	}
	info, err := s.wsAccess.Lookup(ctx, wb.WorkspaceID, newOwnerID)
	if err != nil {
		return err
	}
	if !info.IsMember {
		return ErrNotWorkspaceMember
	}
	return mapErr(s.store.UpdateContainerOwner(ctx, workbenchID, newOwnerID))
}

// LeaveWorkbench removes the caller's own membership. The owner must transfer
// ownership first — authlayer's last-owner lock, not a check of our own.
//
// This is the one membership call that does NOT need requireParentStanding:
// dropping your own row cannot grant anything, and somebody already removed
// from the workspace should still be able to clean themselves out of what it
// left behind.
func (s *Service) LeaveWorkbench(ctx context.Context, userID, workbenchID string) error {
	return mapErr(s.sc.LeaveContainer(actor(ctx, userID, workbenchID)))
}

// ── Roles ───────────────────────────────────────────────────────────────────

// The six mutating calls below delegate authorization to scope, which resolves
// it from the caller's standing on the WORKBENCH. That is the standing
// authorize refuses to take at face value — see its doc — so each one asks
// requireParentStanding first. Before the nested scope every one of them went
// through authorize and inherited the parent check from it; delegating without
// it would let a colleague removed from the workspace go on minting roles and
// admitting members to a workbench they were once added to.

// ListRoles is gated by authorize rather than by scope's own ListRoles, so a
// shared workbench's viewers can see its roles the way they always could —
// scope has no standing to grant them, since their access comes from the
// visibility column it does not know about.
func (s *Service) ListRoles(ctx context.Context, userID, workbenchID string) ([]Role, error) {
	if _, err := s.authorize(ctx, workbenchID, userID, PermViewWorkbench); err != nil {
		return nil, err
	}
	return s.roleViews(ctx, workbenchID)
}

func (s *Service) CreateRole(ctx context.Context, userID, workbenchID, name string, perms Permission) (Role, error) {
	if err := s.requireParentStanding(ctx, workbenchID, userID); err != nil {
		return Role{}, err
	}
	view, err := s.sc.CreateRole(actor(ctx, userID, workbenchID), rbac.RoleKey(name), name, grantsFor(perms))
	if err != nil {
		return Role{}, mapErr(err)
	}
	return roleFromView(workbenchID, codec.View(view)), nil
}

func (s *Service) UpdateRole(ctx context.Context, userID, workbenchID, roleID, name string, perms Permission) (Role, error) {
	if err := s.requireParentStanding(ctx, workbenchID, userID); err != nil {
		return Role{}, err
	}
	view, err := s.sc.UpdateRole(actor(ctx, userID, workbenchID), roleID, name, grantsFor(perms))
	if err != nil {
		return Role{}, mapErr(err)
	}
	return roleFromView(workbenchID, codec.View(view)), nil
}

func (s *Service) DeleteRole(ctx context.Context, userID, workbenchID, roleID string) error {
	if err := s.requireParentStanding(ctx, workbenchID, userID); err != nil {
		return err
	}
	return mapErr(s.sc.DeleteRole(actor(ctx, userID, workbenchID), roleID))
}

// roleViews lists the workbench's roles. It reads the store directly rather
// than calling scope.ListRoles because that method authorizes on membership,
// which the shared-viewer path does not have; the gate has already been applied
// by the caller.
func (s *Service) roleViews(ctx context.Context, workbenchID string) ([]Role, error) {
	records, err := s.store.ListRoles(ctx, workbenchID)
	if err != nil {
		return nil, mapErr(err)
	}
	views, err := codec.Roles(records)
	if err != nil {
		return nil, err
	}
	out := make([]Role, len(views))
	for i, v := range views {
		out[i] = roleFromView(workbenchID, v)
	}
	return out, nil
}

func (s *Service) role(ctx context.Context, workbenchID, key string) (Role, error) {
	roles, err := s.roleViews(ctx, workbenchID)
	if err != nil {
		return Role{}, err
	}
	for _, r := range roles {
		if r.ID == key {
			return r, nil
		}
	}
	return Role{}, ErrRoleNotFound
}

func roleFromView(workbenchID string, v rbac.RoleView[Permission]) Role {
	return Role{
		ID:          v.Key,
		WorkbenchID: workbenchID,
		Name:        v.Name,
		Permissions: v.Permissions,
		IsDefault:   v.IsDefault,
	}
}

// ── Members ─────────────────────────────────────────────────────────────────

func (s *Service) ListMembers(ctx context.Context, userID, workbenchID string) ([]MemberView, error) {
	if _, err := s.authorize(ctx, workbenchID, userID, PermViewWorkbench); err != nil {
		return nil, err
	}
	members, err := s.store.ListMembers(ctx, workbenchID)
	if err != nil {
		return nil, mapErr(err)
	}
	roles, err := s.roleViews(ctx, workbenchID)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]Role, len(roles))
	for _, r := range roles {
		byKey[r.ID] = r
	}

	ids := make([]string, len(members))
	for i, m := range members {
		ids[i] = m.UserID
	}
	profiles, err := s.users.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	out := make([]MemberView, 0, len(members))
	for _, m := range members {
		u, ok := profiles[m.UserID]
		if !ok {
			return nil, auth.ErrNotFound
		}
		out = append(out, MemberView{Member: m, Role: byKey[m.RoleKey], User: u})
	}
	return out, nil
}

// AddMember admits an existing workspace member to the workbench. That the
// target already belongs to the parent is authlayer's MembersFromParent policy,
// not a check of ours — it surfaces as ErrNotWorkspaceMember.
func (s *Service) AddMember(ctx context.Context, userID, workbenchID, targetUserID, roleID string) (MemberView, error) {
	if err := s.requireParentStanding(ctx, workbenchID, userID); err != nil {
		return MemberView{}, err
	}
	if _, err := s.sc.AddMember(actor(ctx, userID, workbenchID), targetUserID, roleID); err != nil {
		return MemberView{}, mapErr(err)
	}
	return s.memberView(ctx, workbenchID, targetUserID, roleID)
}

func (s *Service) ChangeMemberRole(ctx context.Context, userID, workbenchID, targetUserID, roleID string) (MemberView, error) {
	if err := s.requireParentStanding(ctx, workbenchID, userID); err != nil {
		return MemberView{}, err
	}
	if err := s.sc.ChangeMemberRole(actor(ctx, userID, workbenchID), targetUserID, roleID); err != nil {
		return MemberView{}, mapErr(err)
	}
	return s.memberView(ctx, workbenchID, targetUserID, roleID)
}

func (s *Service) RemoveMember(ctx context.Context, userID, workbenchID, targetUserID string) error {
	if err := s.requireParentStanding(ctx, workbenchID, userID); err != nil {
		return err
	}
	return mapErr(s.sc.RemoveMember(actor(ctx, userID, workbenchID), targetUserID))
}

func (s *Service) memberView(ctx context.Context, workbenchID, userID, roleKey string) (MemberView, error) {
	role, err := s.role(ctx, workbenchID, roleKey)
	if err != nil {
		return MemberView{}, err
	}
	u, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return MemberView{}, err
	}
	view := MemberView{Role: role, User: u}
	view.Member.ContainerID = workbenchID
	view.Member.UserID = userID
	view.Member.RoleKey = roleKey
	return view, nil
}
