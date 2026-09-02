package workspace

import (
	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/rbac"
)

// ── The permission surface ──────────────────────────────────────────────────

// Resource names this product declares to authlayer. Three of them are
// authlayer's own control resources, re-exported so a call site reads in one
// vocabulary; workbench is tendersbay's.
const (
	ResourceWorkspace = "workspace"
	ResourceMember    = scope.ResourceMember
	ResourceRole      = scope.ResourceRole
	ResourceInvite    = scope.ResourceInvite
	ResourceWorkbench = "workbench"
)

// ActionManage is this product's own action, on top of authlayer's
// create/read/update/delete: "administer every workbench in the workspace,
// including ones you are not a member of".
const ActionManage access.Action = "manage"

// Code-defined role keys. They exist in every workspace without a stored row,
// cannot be edited or deleted, and are what CreateWorkspace and the seeded
// invitations refer to.
const (
	RoleOwner  = scope.RoleOwner
	RoleAdmin  = scope.RoleAdmin
	RoleMember = scope.RoleMember
)

// Permission is a bitmask of workspace capabilities — the vocabulary the proto
// and the client speak (workspacev1 carries it as a uint64, mirrored in
// apps/platform/src/features/workspace/permissions.ts).
//
// It is NOT what the backend enforces. Enforcement is authlayer's
// access.Permission, a bitset over declared (resource, action) grants, and this
// mask is the published projection of it — see internal/rbac for the machinery
// and for the two rules every projection obeys. Keeping the mask is what let
// the migration to authlayer change no API and no client code.
//
// Adding a capability means adding a bit here, its grants to permissionGrants,
// and the resource/action to Statements — nothing else.
type Permission uint64

const (
	PermViewWorkspace   Permission = 1 << 0 // see the workspace (implied by membership)
	PermManageWorkspace Permission = 1 << 1 // rename / change slug
	PermManageMembers   Permission = 1 << 2 // change member roles, remove members
	PermManageRoles     Permission = 1 << 3 // create / update / delete roles
	PermCreateInvite    Permission = 1 << 4 // create email invites and invite links
	PermManageInvites   Permission = 1 << 5 // list / revoke invites and links
	PermAdministrator   Permission = 1 << 6 // bypass all non-owner-only checks

	// 1<<20.. — workbench feature bits (see internal/core/workbench).
	PermViewWorkbenches   Permission = 1 << 20 // see shared workbenches in the workspace
	PermCreateWorkbench   Permission = 1 << 21 // create new workbenches
	PermManageWorkbenches Permission = 1 << 22 // admin over all workbenches (bypass per-workbench ACL)
)

// Has reports whether p contains every bit in need.
func (p Permission) Has(need Permission) bool { return p&need == need }

// permissionGrants is the single source of truth for the mask-to-grant mapping.
//
// PermViewWorkspace and PermAdministrator are absent because neither is a
// grant: the first is membership, the second is standing. rbac.Codec.Mask
// documents both.
//
// Deleting a workspace is absent too, and that is why "workspace" declares only
// update: deletion is owner-only, enforced by requireOwner rather than by a
// grant. Were it declared, the admin role would either hold it (admins could
// delete the workspace — a behaviour change) or not hold it (admins would stop
// being elevated at all).
var permissionGrants = []rbac.Grant[Permission]{
	{Bit: PermManageWorkspace, Grants: map[string][]access.Action{ResourceWorkspace: {scope.ActionUpdate}}},
	{Bit: PermManageMembers, Grants: map[string][]access.Action{ResourceMember: {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete}}},
	{Bit: PermManageRoles, Grants: map[string][]access.Action{ResourceRole: {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete}}},
	{Bit: PermCreateInvite, Grants: map[string][]access.Action{ResourceInvite: {scope.ActionCreate}}},
	{Bit: PermManageInvites, Grants: map[string][]access.Action{ResourceInvite: {scope.ActionRead, scope.ActionDelete}}},
	{Bit: PermViewWorkbenches, Grants: map[string][]access.Action{ResourceWorkbench: {scope.ActionRead}}},
	{Bit: PermCreateWorkbench, Grants: map[string][]access.Action{ResourceWorkbench: {scope.ActionCreate}}},
	{Bit: PermManageWorkbenches, Grants: map[string][]access.Action{ResourceWorkbench: {ActionManage}}},
}

// Statements is the complete permission surface handed to authlayer. Nothing
// outside it can be granted or checked — access.Access refuses an undeclared
// pair rather than silently denying it.
//
// The workbench entries are declared HERE, on the parent, because that is where
// they are asked: creating a workbench is a check against the workspace, and
// "administers every workbench" is a workspace-level capability the nested
// scope inherits. See internal/core/workbench.
func Statements() map[string][]access.Action {
	return map[string][]access.Action{
		ResourceWorkspace: {scope.ActionUpdate},
		ResourceMember:    {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete},
		ResourceRole:      {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete},
		ResourceInvite:    {scope.ActionCreate, scope.ActionRead, scope.ActionDelete},
		ResourceWorkbench: {scope.ActionRead, scope.ActionCreate, ActionManage},
	}
}

// codec is the mask projection and the access engine behind it, built once.
//
// The member role is redefined over authlayer's empty default because this
// product's baseline member can already see and create workbenches; that was
// the seeded "Member" role's permission set before authlayer.
var codec = rbac.New(Statements(), PermViewWorkspace, PermAdministrator, permissionGrants,
	map[string][]access.Action{
		ResourceWorkbench: {scope.ActionRead, scope.ActionCreate},
	})

// NewAccess returns the access engine for workspaces. It is one shared value,
// not a fresh build per call: permissions from two engines over the same
// declarations still cannot be compared with each other.
func NewAccess() *access.Access { return codec.Access() }

func grantsFor(p Permission) map[string][]access.Action { return codec.GrantsFor(p) }

func maskOf(perms access.Permission, elevated bool) Permission { return codec.Mask(perms, elevated) }

// EncodePermissions renders a wire mask in the form authlayer's role store
// keeps. It exists for the 0013 migration, which rewrites every stored role's
// permission column from the old bitmask into that form.
func EncodePermissions(p Permission) ([]byte, error) { return codec.Encode(p) }

// normalizeSlug turns a requested slug (or, when empty, the workspace's name)
// into the canonical form the unique constraint is enforced on.
func normalizeSlug(slug, name string) string {
	s := slug
	if s == "" {
		s = name
	}
	if out := rbac.Slug(s); out != "" {
		return out
	}
	return "workspace"
}
