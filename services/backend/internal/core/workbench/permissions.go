package workbench

import (
	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/rbac"
)

// Resource names this scope declares. ResourceWorkbench is also the name the
// PARENT workspace declares (workspace.ResourceWorkbench) — it has to be, since
// creating a workbench is a permission check against the workspace and the two
// sides have to be asking about the same resource.
const (
	ResourceWorkbench = "workbench"
	ResourceMember    = scope.ResourceMember
	ResourceRole      = scope.ResourceRole
)

// ActionManage is the workspace-level action this scope inherits elevation
// from. It is declared by the PARENT (workspace.ActionManage) — repeated here
// only so NewService's inheritance reads without a cross-package import.
const ActionManage access.Action = "manage"

// Role keys, and why they are authlayer's three rather than this product's own
// words: scope.Service.ListRoles enumerates the code-defined roles by the fixed
// keys owner/admin/member, so a role registered under any other key would exist
// and resolve but never appear in a role list. The product's labels ride on top
// as display names (see defaultRoleNames) — "Manager" is the admin key and
// "Viewer" the member key, which is what the old seeded rows were called and
// what the client has always shown.
const (
	RoleOwner   = scope.RoleOwner  // the creator; full grants
	RoleManager = scope.RoleAdmin  // full grants, assignable
	RoleViewer  = scope.RoleMember // no grants — membership alone is the right to look
)

// Permission is a bitmask of workbench capabilities — the vocabulary the proto
// and the client speak. Like workspace.Permission it is a PROJECTION of what
// authlayer enforces, not the thing enforced; internal/rbac carries the
// argument and the machinery.
type Permission uint64

const (
	PermViewWorkbench   Permission = 1 << 0 // see the workbench (implied by membership)
	PermManageWorkbench Permission = 1 << 1 // rename / edit description / change visibility
	PermManageMembers   Permission = 1 << 2 // add/remove members, change member roles
	PermManageRoles     Permission = 1 << 3 // create / update / delete workbench roles
	PermAdministrator   Permission = 1 << 6 // bypass all non-owner-only checks
)

func (p Permission) Has(need Permission) bool { return p&need == need }

// permissionGrants is the mask-to-grant mapping. Deleting a workbench is
// absent, and again owner-only, so "workbench" declares only update — that
// keeps the manager role holding every grant and therefore elevated, which is
// what the old PermAdministrator bit meant.
var permissionGrants = []rbac.Grant[Permission]{
	{Bit: PermManageWorkbench, Grants: map[string][]access.Action{ResourceWorkbench: {scope.ActionUpdate}}},
	{Bit: PermManageMembers, Grants: map[string][]access.Action{ResourceMember: {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete}}},
	{Bit: PermManageRoles, Grants: map[string][]access.Action{ResourceRole: {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete}}},
}

// Statements is this scope's complete permission surface.
//
// It does NOT declare workbench:create or workbench:manage: those are the
// PARENT's grants (see workspace.Statements), asked of the workspace rather
// than of a workbench that does not exist yet.
func Statements() map[string][]access.Action {
	return map[string][]access.Action{
		ResourceWorkbench: {scope.ActionUpdate},
		ResourceMember:    {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete},
		ResourceRole:      {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete},
	}
}

// codec is the mask projection and the access engine behind it, built once.
// The viewer role carries no grants at all: membership alone is the right to
// see the workbench, which is exactly what the old default "Viewer" role was.
var codec = rbac.New(Statements(), PermViewWorkbench, PermAdministrator, permissionGrants,
	map[string][]access.Action{})

// NewAccess returns the access engine for workbenches — one shared value, for
// the reason workspace.NewAccess gives.
func NewAccess() *access.Access { return codec.Access() }

// defaultRoleNames are the labels the client shows for the code-defined roles.
var defaultRoleNames = map[string]string{
	RoleOwner:   "Owner",
	RoleManager: "Manager",
	RoleViewer:  "Viewer",
}

func grantsFor(p Permission) map[string][]access.Action { return codec.GrantsFor(p) }

func maskOf(perms access.Permission, elevated bool) Permission { return codec.Mask(perms, elevated) }

// EncodePermissions renders a wire mask in the form authlayer's role store
// keeps. It exists for the 0015 migration.
func EncodePermissions(p Permission) ([]byte, error) { return codec.Encode(p) }
