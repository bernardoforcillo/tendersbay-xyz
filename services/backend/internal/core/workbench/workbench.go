// Package workbench implements workspace-scoped workbenches: personal-or-shared
// containers with Discord-like bitwise RBAC (one role per member) whose members
// are drawn from the parent workspace. Access resolves through two layers — the
// coarse workspace RBAC bits (reserved 1<<20+) OR the fine per-workbench role —
// combined in Service.authorize. The workbench owner, a role bearing the
// ADMINISTRATOR bit, and any workspace owner/admin bypass per-workbench checks.
package workbench

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

type Permission uint64

const (
	PermViewWorkbench   Permission = 1 << 0 // see the workbench (the default role)
	PermManageWorkbench Permission = 1 << 1 // rename / edit description / change visibility
	PermManageMembers   Permission = 1 << 2 // add/remove members, change member roles
	PermManageRoles     Permission = 1 << 3 // create / update / delete workbench roles
	PermAdministrator   Permission = 1 << 6 // bypass all non-owner-only checks
)

const permAdminRole = PermViewWorkbench | PermManageWorkbench | PermManageMembers |
	PermManageRoles | PermAdministrator

// Workspace-level bit VALUES (mirrors workspace.Permission) read from a
// WorkspaceInfo.Perms bitmask. Kept as local constants so this package need not
// import core/workspace. Must stay in sync with workspace.PermView/Create/ManageWorkbenches.
const (
	wsPermAdministrator     uint64 = 1 << 6
	wsPermViewWorkbenches   uint64 = 1 << 20
	wsPermCreateWorkbench   uint64 = 1 << 21
	wsPermManageWorkbenches uint64 = 1 << 22
)

func (p Permission) Has(need Permission) bool       { return p&need == need }
func (p Permission) subsetOf(other Permission) bool { return p&^other == 0 }

type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityShared  Visibility = "shared"
)

type Workbench struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	Visibility  Visibility
	OwnerID     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Role struct {
	ID          string
	WorkbenchID string
	Name        string
	Permissions Permission
	IsDefault   bool
	CreatedAt   time.Time
}

type Member struct {
	WorkbenchID string
	UserID      string
	RoleID      string
	AddedAt     time.Time
}

type Membership struct {
	Member Member
	Role   Role
}

type MemberView struct {
	Member Member
	Role   Role
	User   auth.User
}

var (
	ErrWorkbenchNotFound   = errors.New("workbench not found")
	ErrNotMember           = errors.New("not a member of this workbench")
	ErrForbidden           = errors.New("insufficient permissions")
	ErrPrivilegeEscalation = errors.New("cannot grant permissions you do not have")
	ErrRoleNotFound        = errors.New("role not found")
	ErrRoleInUse           = errors.New("role is assigned to members")
	ErrDefaultRole         = errors.New("cannot delete the default role")
	ErrLastOwner           = errors.New("cannot remove or demote the workbench owner")
	ErrOwnerOnly           = errors.New("only the workbench owner may do this")
	ErrAlreadyMember       = errors.New("user is already a member")
	ErrNotWorkspaceMember  = errors.New("user is not a member of the workspace")
)

type WorkbenchRepository interface {
	Create(ctx context.Context, w Workbench) (Workbench, error)
	FindByID(ctx context.Context, id string) (Workbench, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Workbench, error)
	Update(ctx context.Context, id, name, description string) (Workbench, error)
	UpdateVisibility(ctx context.Context, id string, v Visibility) (Workbench, error)
	UpdateOwner(ctx context.Context, id, newOwnerID string) error
	Delete(ctx context.Context, id string) error
}

type WorkbenchRoleRepository interface {
	Create(ctx context.Context, r Role) (Role, error)
	FindByID(ctx context.Context, id string) (Role, error)
	ListByWorkbench(ctx context.Context, workbenchID string) ([]Role, error)
	Update(ctx context.Context, id, name string, perms Permission) (Role, error)
	Delete(ctx context.Context, id string) error
	CountMembersUsing(ctx context.Context, roleID string) (int64, error)
}

type WorkbenchMemberRepository interface {
	Add(ctx context.Context, m Member) (Member, error)
	Find(ctx context.Context, workbenchID, userID string) (Member, error)
	LoadMembership(ctx context.Context, workbenchID, userID string) (Membership, error)
	ListByWorkbench(ctx context.Context, workbenchID string) ([]Member, error)
	UpdateRole(ctx context.Context, workbenchID, userID, roleID string) error
	Remove(ctx context.Context, workbenchID, userID string) error
	CountByWorkbench(ctx context.Context, workbenchID string) (int64, error)
}

// UserLookup is the narrow slice of the auth user store used to enrich members.
type UserLookup interface {
	FindByID(ctx context.Context, id string) (auth.User, error)
}

// WorkspaceInfo is the parent-workspace context needed to resolve the coarse
// (workspace) access layer for a workbench.
type WorkspaceInfo struct {
	Name     string
	Perms    uint64 // the caller's workspace permission bitmask
	IsOwner  bool   // caller owns the workspace
	IsMember bool   // caller is a member of the workspace at all
}

// WorkspaceAccess bridges to the workspace domain without importing it: it
// returns the caller's standing in a workspace. Implemented by a postgres
// adapter over the workspace member/repo.
type WorkspaceAccess interface {
	Lookup(ctx context.Context, workspaceID, userID string) (WorkspaceInfo, error)
}

// Repos is the set of tx-scoped repositories handed to a UnitOfWork closure.
type Repos struct {
	Workbenches WorkbenchRepository
	Roles       WorkbenchRoleRepository
	Members     WorkbenchMemberRepository
}

// UnitOfWork runs fn inside a single database transaction (seeding).
type UnitOfWork interface {
	Do(ctx context.Context, fn func(Repos) error) error
}
