// Package rbac is the mechanism the scopes tendersbay runs on authlayer share.
// It holds no product decisions: which capabilities exist, what they are called
// and who gets them are each domain's business, declared in that domain's own
// permissions.go. What lives here is the machinery those declarations are fed
// to, and it lives here because there is more than one of them — the workspace
// and the workbench scopes had the same 150 lines twice, differing only in the
// table at the top.
//
// The one thing it does decide is the pair of rules every projection has to
// obey, because they are the two that would silently drift if each domain kept
// its own copy: the baseline bit is set for anyone with standing at all, and an
// elevated caller holds every bit. See Codec.Mask.
package rbac

import (
	"strings"

	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
)

// Grant pairs one bit of a domain's wire mask with the authlayer grants it
// stands for. A bit with no grants is not listed at all — see Codec.Mask for
// the two that never are.
type Grant[M ~uint64] struct {
	Bit    M
	Grants map[string][]access.Action
}

// Codec projects between a domain's published permission bitmask and
// authlayer's grant set.
//
// The bitmask is what the proto and the client speak; the grant set is what the
// library enforces. Neither is derived from the other at runtime — the mapping
// is the Grant table the domain hands to New, and everything else follows from
// it.
type Codec[M ~uint64] struct {
	ac         *access.Access
	statements map[string][]access.Action
	grants     []Grant[M]
	baseline   M
	admin      M
	labels     map[string]string
}

// Config is everything a scope declares about its own permission surface.
// Grouped into a struct rather than passed positionally: six arguments of which
// three are maps is a call nobody can read.
type Config[M ~uint64] struct {
	// Statements is the complete set of (resource, action) pairs the scope
	// declares. Nothing outside it can be granted or checked.
	Statements map[string][]access.Action
	// Baseline is the bit that means "is a member", and Admin the bit that
	// means "bypasses every check". Neither is a grant, so neither appears in
	// Grants.
	Baseline M
	Admin    M
	// Grants maps each remaining bit onto what it stands for.
	Grants []Grant[M]
	// MemberGrants is what the baseline role carries — empty for a scope where
	// membership alone is the whole of it.
	MemberGrants map[string][]access.Action
	// Labels are the display names for the code-defined roles, keyed by
	// scope.RoleOwner / RoleAdmin / RoleMember. authlayer reports a
	// code-defined role's name as its key, because a role built in code has no
	// separate label; these are what the client shows instead.
	Labels map[string]string
}

// New builds a codec and, with it, the access engine the scope runs on.
//
// The three code-defined roles are seeded here rather than through
// scope.NewAccess, which declares <container>:delete and then withholds it from
// admin. That would leave the admin role short of access.Permission.IsFull and
// therefore un-elevated, which is not what either of this product's scopes
// means by an administrator. Owner and admin both hold everything; deletion is
// gated by ownership instead of by a grant, in the domain that owns the rule.
//
// It panics on a grant the statements do not declare, at package
// initialization: a capability nobody can hold is a programming error, not a
// runtime condition, and authlayer refuses it for the same reason.
func New[M ~uint64](cfg Config[M]) *Codec[M] {
	ac := access.New(access.NewStatements(cfg.Statements))
	ac.NewRole(scope.RoleOwner, cfg.Statements)
	ac.NewRole(scope.RoleAdmin, cfg.Statements)
	ac.NewRole(scope.RoleMember, cfg.MemberGrants)
	return &Codec[M]{
		ac:         ac,
		statements: cfg.Statements,
		grants:     cfg.Grants,
		baseline:   cfg.Baseline,
		admin:      cfg.Admin,
		labels:     cfg.Labels,
	}
}

// Access is the engine to hand scope.New. One per scope, built once: an
// access.Permission is a bitset over one Statements instance, so two engines
// over the same declarations still produce permissions that cannot be compared
// with each other.
func (c *Codec[M]) Access() *access.Access { return c.ac }

// GrantsFor expands a wire mask into the grants authlayer stores on a role.
// The admin bit expands to every declared grant, which is what makes the
// resulting role elevated.
func (c *Codec[M]) GrantsFor(p M) map[string][]access.Action {
	out := map[string][]access.Action{}
	if p&c.admin == c.admin {
		// A copy, not c.statements itself: this map is handed to a caller that
		// passes it on to authlayer, and the codec's own declarations must not
		// be reachable for mutation from there.
		for resource, actions := range c.statements {
			out[resource] = append([]access.Action(nil), actions...)
		}
		return out
	}
	for _, g := range c.grants {
		if p&g.Bit != g.Bit {
			continue
		}
		for resource, actions := range g.Grants {
			out[resource] = append(out[resource], actions...)
		}
	}
	return out
}

// Mask projects an authlayer permission back onto the wire mask.
//
// Two rules, and both matter:
//
//   - The baseline bit is always set. authlayer has no "read the container"
//     grant because being a member IS that right, so a caller with standing
//     holds it by definition.
//   - An elevated caller holds EVERY bit, not just the admin one. Elevation can
//     be INHERITED from a parent scope, where the caller has no membership and
//     therefore no grants of their own; a mask derived from grants alone would
//     tell such a caller they may only look, while every write they attempted
//     would in fact succeed.
func (c *Codec[M]) Mask(perms access.Permission, elevated bool) M {
	out := c.baseline
	if elevated {
		out |= c.admin
		for _, g := range c.grants {
			out |= g.Bit
		}
		return out
	}
	for _, g := range c.grants {
		granted := true
		for resource, actions := range g.Grants {
			if !perms.Allows(resource, actions...) {
				granted = false
				break
			}
		}
		if granted {
			out |= g.Bit
		}
	}
	return out
}

// Encode renders a wire mask in the form authlayer's role store keeps: the
// encoded grant names its access engine reads back. The migrations that rewrite
// a stored bitmask into that form go through here, so the conversion uses the
// same table every other conversion does.
func (c *Codec[M]) Encode(p M) ([]byte, error) {
	perm, err := c.ac.Permission(c.GrantsFor(p))
	if err != nil {
		return nil, err
	}
	return perm.Encode(), nil
}

// RoleKey derives a stable, URL-safe key from a role's display name.
//
// authlayer keys a role by a string unique within its container, where this
// product used to key it by a UUID, and that key is what travels the wire as a
// role's id. It is derived from something a person chose rather than generated
// so a renamed role stays the same role.
func RoleKey(name string) string {
	if key := Slug(name); key != "" {
		return key
	}
	return "role"
}

// Slug lowercases, keeps [a-z0-9], and collapses everything else into single
// dashes. Role keys and workspace slugs share it so the two cannot drift.
func Slug(s string) string {
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

// RoleView is one role as the product publishes it: authlayer's key, the label
// to show for it, its permissions already projected onto the wire mask, and
// whether it is one of the code-defined three.
type RoleView[M ~uint64] struct {
	Key         string
	Name        string
	Permissions M
	IsDefault   bool
}

// Roles composes the code-defined roles with a container's own stored ones.
//
// It is scope.Service.ListRoles without the authorization: that method gates on
// membership, and a caller can be entitled to see a container's roles by a rule
// the engine does not know about — a shared workbench's viewers, whose access
// comes from a visibility column. The gate is the domain's to apply before
// calling this.
func (c *Codec[M]) Roles(records []scope.RoleRecord) ([]RoleView[M], error) {
	out := make([]RoleView[M], 0, len(records)+3)
	for _, key := range []string{scope.RoleOwner, scope.RoleAdmin, scope.RoleMember} {
		r, ok := c.ac.Role(key)
		if !ok {
			continue
		}
		out = append(out, RoleView[M]{
			Key:         r.Key,
			Name:        c.Label(r.Key),
			Permissions: c.Mask(r.Permissions, r.Permissions.IsFull()),
			IsDefault:   true,
		})
	}
	for _, rec := range records {
		perms, err := c.ac.Decode(rec.Permissions)
		if err != nil {
			return nil, err
		}
		out = append(out, RoleView[M]{
			Key:         rec.Key,
			Name:        rec.Name,
			Permissions: c.Mask(perms, perms.IsFull()),
		})
	}
	return out, nil
}

// View projects one of authlayer's role views onto the product's. Use it for
// the single role scope.Service.CreateRole and UpdateRole hand back; Roles is
// the same projection over a whole container.
func (c *Codec[M]) View(v scope.RoleView) RoleView[M] {
	name := v.Name
	if v.IsDefault {
		name = c.Label(v.Key)
	}
	return RoleView[M]{
		Key:         v.Key,
		Name:        name,
		Permissions: c.Mask(v.Permissions, v.Permissions.IsFull()),
		IsDefault:   v.IsDefault,
	}
}

// Label is the display name for a code-defined role key, and the key itself for
// anything else — which is the right answer for a stored role only when its own
// name is unavailable, so callers holding the row should prefer that.
func (c *Codec[M]) Label(key string) string {
	if name, ok := c.labels[key]; ok {
		return name
	}
	return key
}
