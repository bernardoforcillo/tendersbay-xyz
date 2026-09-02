package workbench

import (
	"strings"

	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
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
// authlayer enforces, not the thing enforced; see that package's permissions.go
// for the full argument, which applies here unchanged.
type Permission uint64

const (
	PermViewWorkbench   Permission = 1 << 0 // see the workbench (implied by membership)
	PermManageWorkbench Permission = 1 << 1 // rename / edit description / change visibility
	PermManageMembers   Permission = 1 << 2 // add/remove members, change member roles
	PermManageRoles     Permission = 1 << 3 // create / update / delete workbench roles
	PermAdministrator   Permission = 1 << 6 // bypass all non-owner-only checks
)

func (p Permission) Has(need Permission) bool { return p&need == need }

// permissionGrants is the mask-to-grant mapping. PermViewWorkbench is
// membership and PermAdministrator is elevation, so neither has grants — the
// same two exceptions workspace.permissionGrants documents.
//
// Deleting a workbench is again absent and again owner-only, so "workbench"
// declares only update. That keeps the manager role at IsFull and therefore
// elevated, which is what the old PermAdministrator bit meant.
var permissionGrants = []struct {
	bit    Permission
	grants map[string][]access.Action
}{
	{PermManageWorkbench, map[string][]access.Action{ResourceWorkbench: {scope.ActionUpdate}}},
	{PermManageMembers, map[string][]access.Action{ResourceMember: {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete}}},
	{PermManageRoles, map[string][]access.Action{ResourceRole: {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete}}},
}

// Statements is this scope's complete permission surface.
//
// It does NOT declare workbench:create or workbench:manage: those are the
// PARENT's grants (see workspace.Statements), asked of the workspace rather
// than of a workbench that does not exist yet — creation is checked against the
// workspace, and "administer every workbench" is a workspace-level capability
// projected down by the inheritance below.
func Statements() map[string][]access.Action {
	return map[string][]access.Action{
		ResourceWorkbench: {scope.ActionUpdate},
		ResourceMember:    {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete},
		ResourceRole:      {scope.ActionCreate, scope.ActionUpdate, scope.ActionDelete},
	}
}

// NewAccess builds the workbench access engine. owner and manager both hold
// everything — matching the old seeded "Manager" role, which carried
// PermAdministrator — and viewer holds nothing, which is exactly the old
// default "Viewer" role: membership alone is the right to see the workbench.
func NewAccess() *access.Access {
	ac := access.New(access.NewStatements(Statements()))
	ac.NewRole(RoleOwner, Statements())
	ac.NewRole(RoleManager, Statements())
	ac.NewRole(RoleViewer, map[string][]access.Action{})
	return ac
}

// defaultRoleNames are the labels the client shows for the code-defined roles.
var defaultRoleNames = map[string]string{
	RoleOwner:   "Owner",
	RoleManager: "Manager",
	RoleViewer:  "Viewer",
}

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

// maskOf projects an authlayer permission back onto the wire mask.
//
// An elevated caller is reported as holding EVERY bit, not just the
// administrator one. That is not cosmetic: elevation here can be INHERITED —
// a workspace administrator has no membership in the workbench and therefore no
// grants of their own, so a mask derived from grants alone would tell the client
// they may only look, while every write they attempted would in fact succeed.
func maskOf(perms access.Permission, elevated bool) Permission {
	if elevated {
		out := PermViewWorkbench | PermAdministrator
		for _, pg := range permissionGrants {
			out |= pg.bit
		}
		return out
	}
	out := PermViewWorkbench
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

// roleKey derives a role's key from its display name, the same way
// workspace.roleKey does — the key is what travels the wire as Role.ID.
func roleKey(name string) string {
	key := slugify(name)
	if key == "" {
		key = "role"
	}
	return key
}

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

// EncodePermissions renders a wire mask in the form authlayer's role store
// keeps. It exists for the 0015 migration; see workspace.EncodePermissions.
func EncodePermissions(p Permission) ([]byte, error) {
	perm, err := NewAccess().Permission(grantsFor(p))
	if err != nil {
		return nil, err
	}
	return perm.Encode(), nil
}

// RoleKeyFor is roleKey, exported for the 0015 migration.
func RoleKeyFor(name string) string { return roleKey(name) }
