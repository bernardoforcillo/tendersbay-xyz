package postgres

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// The two rows the old CreateWorkspace seeded in every workspace are exactly
// what authlayer's registry defines now, so they are dropped and their members
// moved onto the code-defined keys.
func TestPlanRoles_DropsTheSeededRows(t *testing.T) {
	plans := planRoles([]legacyRole{
		{id: "r1", containerID: "w1", name: "Admin", mask: legacySeededAdminMask},
		{id: "r2", containerID: "w1", name: "Member", mask: legacySeededMemberMask, isDefault: true},
	})
	want := map[string]struct {
		key  string
		keep bool
	}{
		"r1": {workspace.RoleAdmin, false},
		"r2": {workspace.RoleMember, false},
	}
	for _, p := range plans {
		w := want[p.id]
		if p.key != w.key || p.keep != w.keep {
			t.Fatalf("role %s planned as (%q, keep=%v), want (%q, keep=%v)", p.id, p.key, p.keep, w.key, w.keep)
		}
	}
}

// A role someone created and named "Admin" is NOT the seeded one: it carries
// its own permissions, and promoting it to the registry's admin would hand out
// access nobody granted.
func TestPlanRoles_CustomRoleNamedLikeAReservedOneIsRenamed(t *testing.T) {
	plans := planRoles([]legacyRole{
		{id: "r1", containerID: "w1", name: "Admin", mask: int64(workspace.PermViewWorkbenches)},
	})
	if len(plans) != 1 {
		t.Fatalf("got %d plans", len(plans))
	}
	if plans[0].key != "admin-custom" || !plans[0].keep {
		t.Fatalf("plan = %+v, want admin-custom kept", plans[0])
	}
}

// The new (container_id, key) unique is what makes ErrRoleKeyTaken work, so two
// names that slugify alike must not both claim the same key.
func TestPlanRoles_DeduplicatesWithinAWorkspace(t *testing.T) {
	plans := planRoles([]legacyRole{
		{id: "r1", containerID: "w1", name: "Bid Team", mask: 0},
		{id: "r2", containerID: "w1", name: "bid  team", mask: 0},
		{id: "r3", containerID: "w1", name: "BID-TEAM", mask: 0},
		// A different workspace is a different namespace.
		{id: "r4", containerID: "w2", name: "Bid Team", mask: 0},
	})
	got := map[string]string{}
	for _, p := range plans {
		got[p.id] = p.key
	}
	want := map[string]string{"r1": "bid-team", "r2": "bid-team-2", "r3": "bid-team-3", "r4": "bid-team"}
	for id, key := range want {
		if got[id] != key {
			t.Fatalf("role %s got key %q, want %q (all: %v)", id, got[id], key, got)
		}
	}
}

// Every kept role's mask has to survive encoding, or the migration would write
// a permission column authlayer cannot read back.
func TestPlanRoles_MasksEncode(t *testing.T) {
	plans := planRoles([]legacyRole{
		{id: "r1", containerID: "w1", name: "Reviewer",
			mask: int64(workspace.PermViewWorkbenches | workspace.PermManageInvites)},
	})
	encoded, err := workspace.EncodePermissions(workspace.Permission(plans[0].mask))
	if err != nil {
		t.Fatalf("EncodePermissions: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("a role with two capabilities encoded to nothing")
	}
}
