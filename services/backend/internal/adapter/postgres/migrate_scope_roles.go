package postgres

import (
	"fmt"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/rbac"
)

// legacyRole is one pre-authlayer role row, from either the workspace or the
// workbench table: both had the same shape, a UUID identifying a row whose
// permissions were a bitmask.
type legacyRole struct {
	id          string
	containerID string
	name        string
	mask        int64
	isDefault   bool
}

// rolePlan is what a migration decided about one legacy role row.
type rolePlan struct {
	id   string
	key  string
	mask int64
	keep bool // false: the registry now defines this role, so the row goes
}

// planLegacyRoles turns legacy role rows into the keys authlayer will know them
// by. Migrations 0013 and 0015 are the same problem one level apart, so this is
// the decision both of them make:
//
//   - A row the old code SEEDED collapses into the code-defined role that
//     replaced it, and the row goes. Its key is whatever seeded reports, not
//     what the name derives to: authlayer's registry calls a workbench's
//     "Manager" role admin, and the members holding it have to land there.
//   - Any other row keeps its permissions and takes the key its name derives
//     to. If that key collides with one of the reserved ones it is renamed
//     rather than merged — a custom role that merely READS like a reserved one
//     has its own permissions, and promoting it would hand out access nobody
//     granted.
//   - Keys are then deduplicated within their container, because the new
//     (container_id, key) unique is what turns a concurrent double-insert into
//     ErrRoleKeyTaken.
//
// The whole snapshot is decided at once, so the deduplication sees every row.
func planLegacyRoles(legacy []legacyRole, reserved []string, seeded func(legacyRole) (string, bool)) []rolePlan {
	isReserved := make(map[string]bool, len(reserved))
	for _, k := range reserved {
		isReserved[k] = true
	}
	// taken tracks the keys already claimed in each container, so two names
	// that derive to the same key ("Bid Team" and "bid team") do not collide.
	taken := map[string]map[string]bool{}

	plans := make([]rolePlan, 0, len(legacy))
	for _, r := range legacy {
		if key, isSeeded := seeded(r); isSeeded {
			plans = append(plans, rolePlan{id: r.id, key: key, mask: r.mask})
			continue
		}

		key := rbac.RoleKey(r.name)
		if isReserved[key] {
			key += "-custom"
		}
		if taken[r.containerID] == nil {
			taken[r.containerID] = map[string]bool{}
		}
		base := key
		for n := 2; taken[r.containerID][key] || isReserved[key]; n++ {
			key = fmt.Sprintf("%s-%d", base, n)
		}
		taken[r.containerID][key] = true

		plans = append(plans, rolePlan{id: r.id, key: key, mask: r.mask, keep: true})
	}
	return plans
}
