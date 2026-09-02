// Package workbench implements workspace-scoped workbenches: personal-or-shared
// containers whose members are drawn from the parent workspace.
//
// The RBAC engine is github.com/bernardoforcillo/authlayer/scope, configured as
// a NESTED scope with the workspace as its parent. That is what the two-layer
// access model used to be: the workspace half is no longer a bitmask copied
// across a bridge but an inheritance the engine resolves — anyone who may
// administer workbenches in the workspace is elevated in every workbench in it,
// and creating one is a permission check against the workspace rather than a
// hand-written precondition.
//
// What stays this package's own is the one rule authlayer cannot express,
// because it depends on a column authlayer does not know about: a SHARED
// workbench is visible to any workspace member who may see shared workbenches,
// with no membership of its own. See Service.authorize.
package workbench

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/authlayer/scope"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityShared  Visibility = "shared"
)

// Workbench is the scope container, nested in a workspace.
//
// It satisfies scope.Nested through its own WorkspaceID field rather than by
// embedding scope.NestedBase, whose column is named parent_id: the column here
// has always been workspace_id, the name reads better everywhere it is used,
// and the interface asks for a method, not a field name.
type Workbench struct {
	scope.ContainerBase
	WorkspaceID string     `drop:"workspace_id"`
	Name        string     `drop:"name"`
	Description string     `drop:"description"`
	Visibility  Visibility `drop:"visibility"`
}

func (w Workbench) ContainerParent() string { return w.WorkspaceID }
func (w *Workbench) SetParent(id string)    { w.WorkspaceID = id }

// Member is a workbench membership: the workbench id, the user id, the role
// KEY, and when they joined.
type Member struct {
	scope.MemberBase
}

// Role is the product's view of a workbench role. ID is authlayer's role key —
// what the wire calls role_id.
type Role struct {
	ID          string
	WorkbenchID string
	Name        string
	Permissions Permission
	// IsDefault marks a code-defined role (owner/manager/viewer): it exists in
	// every workbench without a stored row and cannot be edited or deleted.
	IsDefault bool
}

// Membership is a member together with the permissions it holds.
type Membership struct {
	Member Member
	Role   Role
}

// MemberView is a member enriched with its role and user profile for the API.
type MemberView struct {
	Member Member
	Role   Role
	User   auth.User
}

// Sentinel errors — the vocabulary connectapi.toConnectError maps. mapErr
// translates every scope sentinel into one of them.
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
	ErrRoleKeyTaken        = errors.New("a role with this name already exists")
)

// ── Ports ───────────────────────────────────────────────────────────────────

// Store is authlayer's scope persistence port, typed for this scope.
type Store = scope.Store[Workbench, Member]

// Repository is the handful of workbench queries authlayer's scope store does
// not cover, because they are about this product's own columns — the name, the
// description, the visibility — rather than about containment.
type Repository interface {
	ListByWorkspace(ctx context.Context, workspaceID string) ([]Workbench, error)
	UpdateDetails(ctx context.Context, id, name, description string) (Workbench, error)
	UpdateVisibility(ctx context.Context, id string, v Visibility) (Workbench, error)
	Delete(ctx context.Context, id string) error
}

// UserLookup is the narrow slice of the user profile store used to enrich
// members. FindByIDs is what a member listing uses: one query for the whole
// page rather than one per row.
type UserLookup interface {
	FindByID(ctx context.Context, id string) (auth.User, error)
	FindByIDs(ctx context.Context, ids []string) (map[string]auth.User, error)
}

// WorkspaceInfo is the parent-workspace context this package still needs after
// nesting took over the permission half: the breadcrumb name, and the two
// questions the engine cannot answer because they are about workbenches in
// GENERAL rather than about one workbench.
//
// It carries decisions, not a bitmask. The previous version passed the raw
// workspace permission mask across and this package re-derived the bits from
// three hand-copied `1 << 20` constants that a comment asked future readers to
// keep in sync with core/workspace — a defect waiting for the first person who
// added a bit. The workspace domain now answers the questions instead.
type WorkspaceInfo struct {
	// IsMember is whether the caller belongs to the workspace at all. A
	// non-member must not learn that a workbench exists.
	IsMember bool
	// MayViewShared is the workspace-level right to see shared workbenches.
	MayViewShared bool
	// MayManageAll is the workspace-level right to administer every workbench
	// in the workspace, which is also what the nesting projects as elevation.
	MayManageAll bool
}

// WorkspaceAccess bridges to the workspace domain without importing it.
// Implemented by an adapter in the composition root over the workspace service.
//
// Two methods, because they are two questions with two costs: Lookup resolves a
// caller's standing, which means reading their membership and their role;
// WorkspaceName reads one column. GetWorkbench needs only the second, for a
// breadcrumb, and asking the first for it made a page load pay for a permission
// check nothing consulted.
type WorkspaceAccess interface {
	Lookup(ctx context.Context, workspaceID, userID string) (WorkspaceInfo, error)
	WorkspaceName(ctx context.Context, workspaceID string) (string, error)
}
