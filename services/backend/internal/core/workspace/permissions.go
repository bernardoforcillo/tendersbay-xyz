package workspace

import (
	"strings"

	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
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
// It is NOT what the backend enforces any more. Enforcement is authlayer's
// access.Permission, a bitset over declared (resource, action) grants, and this
// mask is the published projection of it: grantsFor turns a mask from the wire
// into grants for the library, maskOf turns the library's answer back into a
// mask for the wire. Keeping the mask is what let the migration to authlayer
// change no API and no client code.
//
// The mapping is declared once, in permissionGrants below. Adding a capability
// means adding a bit here, its grants there, and the resource/action to
// Statements — nothing else.
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

// permissionGrants is the single source of truth for the mask to grant mapping.
//
// Two bits deliberately have no grants:
//
//   - PermViewWorkspace is membership itself. authlayer has no "read the
//     container" grant because being a member IS that right, so the bit is
//     reported for every member and never asked for.
//   - PermAdministrator is not a grant but a STANDING: authlayer treats a role
//     holding every declared grant as elevated (access.Permission.IsFull), and
//     that is what bypasses the privilege-escalation guard. maskOf reports the
//     bit for an elevated caller; grantsFor expands it to every grant.
//
// Deleting a workspace is likewise absent, and that is why "workspace" declares
// only update: deletion is owner-only, enforced by requireOwner rather than by
// a grant. Were it declared, the admin role would either hold it (admins could
// delete the workspace — a behaviour change) or not hold it (admins would stop
// being IsFull, and so stop being elevated at all).
var permissionGrants = []struct {
	bit    Permission
	grants map[string][]access.Action
}{
	{PermManageWorkspace, map[string][]access.Action{ResourceWorkspace: {scope.ActionUpdate}}},
	{PermManageMembers, map[string][]access.Action{ResourceMember: {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete}}},
	{PermManageRoles, map[string][]access.Action{ResourceRole: {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete}}},
	{PermCreateInvite, map[string][]access.Action{ResourceInvite: {scope.ActionCreate}}},
	{PermManageInvites, map[string][]access.Action{ResourceInvite: {scope.ActionRead, scope.ActionDelete}}},
	{PermViewWorkbenches, map[string][]access.Action{ResourceWorkbench: {scope.ActionRead}}},
	{PermCreateWorkbench, map[string][]access.Action{ResourceWorkbench: {scope.ActionCreate}}},
	{PermManageWorkbenches, map[string][]access.Action{ResourceWorkbench: {ActionManage}}},
}

// Statements is the complete permission surface handed to authlayer. Nothing
// outside it can be granted or checked — access.Access refuses an undeclared
// pair rather than silently denying it.
func Statements() map[string][]access.Action {
	return map[string][]access.Action{
		ResourceWorkspace: {scope.ActionUpdate},
		ResourceMember:    {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete},
		ResourceRole:      {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete},
		ResourceInvite:    {scope.ActionCreate, scope.ActionRead, scope.ActionDelete},
		ResourceWorkbench: {scope.ActionRead, scope.ActionCreate, ActionManage},
	}
}

// NewAccess builds the access engine with this product's statements and its
// three code-defined roles. Call it once at startup and share the result.
//
// It does not use scope.NewAccess: that helper declares <container>:delete and
// then withholds it from admin, which would leave the admin role short of
// IsFull and therefore un-elevated. Here owner and admin both hold everything —
// matching the old Admin role, which carried PermAdministrator — and workspace
// deletion is gated by ownership instead of by a grant.
//
// member is redefined rather than left at scope's empty default because this
// product's baseline member can already see and create workbenches; that was
// the seeded "Member" role's permission set before authlayer.
func NewAccess() *access.Access {
	ac := access.New(access.NewStatements(Statements()))
	ac.NewRole(RoleOwner, Statements())
	ac.NewRole(RoleAdmin, Statements())
	ac.NewRole(RoleMember, map[string][]access.Action{
		ResourceWorkbench: {scope.ActionRead, scope.ActionCreate},
	})
	return ac
}

// grantsFor expands a wire mask into the grants authlayer stores on a role.
// PermAdministrator expands to every declared grant, which is what makes the
// resulting role elevated.
func grantsFor(p Permission) map[string][]access.Action {
	if p.Has(PermAdministrator) {
		return Statements()
	}
	out := map[string][]access.Action{}
	for _, pg := range permissionGrants {
		if !p.Has(pg.bit) {
			continue
		}
		for resource, actions := range pg.grants {
			out[resource] = append(out[resource], actions...)
		}
	}
	return out
}

// maskOf projects an authlayer permission back onto the wire mask. elevated
// comes from scope.Standing (owner, or a role holding every grant) and is what
// PermAdministrator means to the client.
func maskOf(perms access.Permission, elevated bool) Permission {
	// Anyone with standing at all is a member, and membership is what
	// PermViewWorkspace names.
	//
	// An elevated caller is reported as holding every bit rather than just the
	// administrator one: elevation bypasses each check individually, so a mask
	// that showed less would be telling the client something untrue about what
	// it may do.
	if elevated {
		out := PermViewWorkspace | PermAdministrator
		for _, pg := range permissionGrants {
			out |= pg.bit
		}
		return out
	}
	out := PermViewWorkspace
	for _, pg := range permissionGrants {
		granted := true
		for resource, actions := range pg.grants {
			if !perms.Allows(resource, actions...) {
				granted = false
				break
			}
		}
		if granted {
			out |= pg.bit
		}
	}
	return out
}

// roleKey derives a stable, URL-safe key from a role's display name. authlayer
// keys a role by a string unique within its container, where this service used
// to key it by a UUID; the key is what travels the wire as Role.ID, so it is
// derived from something the user chose rather than generated — otherwise a
// renamed role would silently become a different role.
func roleKey(name string) string {
	key := slugify(name)
	if key == "" {
		key = "role"
	}
	return key
}

// slugify lowercases, keeps [a-z0-9], and collapses everything else into single
// dashes. Shared by role keys and workspace slugs so the two cannot drift.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func normalizeSlug(slug, name string) string {
	s := slug
	if s == "" {
		s = name
	}
	out := slugify(s)
	if out == "" {
		out = "workspace"
	}
	return out
}

// EncodePermissions renders a wire mask in the form authlayer's role store
// keeps: the encoded grant names its access engine reads back. It exists for
// the 0013 migration, which has to rewrite every stored role's permission
// column from the old bitmask into that form, and it lives here so the
// conversion uses the same mapping table every other conversion does.
func EncodePermissions(p Permission) ([]byte, error) {
	perm, err := NewAccess().Permission(grantsFor(p))
	if err != nil {
		return nil, err
	}
	return perm.Encode(), nil
}

// RoleKeyFor is roleKey, exported for the 0013 migration, which has to derive
// the same key from a stored role's name that CreateRole would.
func RoleKeyFor(name string) string { return roleKey(name) }
