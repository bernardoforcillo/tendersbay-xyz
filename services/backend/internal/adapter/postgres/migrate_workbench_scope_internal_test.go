package postgres

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

// The two rows every old CreateWorkbench seeded are what authlayer's registry
// defines now, and their keys are NOT derived from their names: the registry
// calls them admin and member, so the members holding them have to land there.
func TestPlanWorkbenchRoles_MapsTheSeededRowsOntoTheRegistryKeys(t *testing.T) {
	plans := planWorkbenchRolePlans([]legacyRole{
		{id: "r1", containerID: "wb1", name: "Manager", mask: legacySeededManagerMask},
		{id: "r2", containerID: "wb1", name: "Viewer", mask: legacySeededViewerMask, isDefault: true},
	})
	want := map[string]string{"r1": workbench.RoleManager, "r2": workbench.RoleViewer}
	for _, p := range plans {
		if p.key != want[p.id] {
			t.Fatalf("role %s planned as %q, want %q", p.id, p.key, want[p.id])
		}
		if p.keep {
			t.Fatalf("role %s should be dropped in favour of the code-defined one", p.id)
		}
	}
}

// A role somebody created and named "Manager" is not the seeded one — it has
// its own permissions, and promoting it to the registry's full-grant role would
// hand out access nobody granted.
func TestPlanWorkbenchRoles_CustomRoleKeepsItsOwnPermissions(t *testing.T) {
	plans := planWorkbenchRolePlans([]legacyRole{
		{id: "r1", containerID: "wb1", name: "Manager", mask: int64(workbench.PermManageMembers)},
	})
	if len(plans) != 1 || !plans[0].keep {
		t.Fatalf("plans = %+v, want one kept", plans)
	}
	if plans[0].key != "manager" {
		t.Fatalf("key = %q, want manager — it is not a reserved key", plans[0].key)
	}
}

// A custom role whose name derives to one of authlayer's three reserved keys is
// renamed rather than merged into the registry role.
func TestPlanWorkbenchRoles_ReservedKeyCollisionIsRenamed(t *testing.T) {
	plans := planWorkbenchRolePlans([]legacyRole{
		{id: "r1", containerID: "wb1", name: "Admin", mask: int64(workbench.PermManageRoles)},
		{id: "r2", containerID: "wb1", name: "Reviewer", mask: 0},
		{id: "r3", containerID: "wb1", name: "reviewer", mask: 0},
	})
	got := map[string]string{}
	for _, p := range plans {
		got[p.id] = p.key
	}
	for id, want := range map[string]string{"r1": "admin-custom", "r2": "reviewer", "r3": "reviewer-2"} {
		if got[id] != want {
			t.Fatalf("role %s got key %q, want %q (all: %v)", id, got[id], want, got)
		}
	}
}

func TestPlanWorkbenchRoles_MasksEncode(t *testing.T) {
	plans := planWorkbenchRolePlans([]legacyRole{
		{id: "r1", containerID: "wb1", name: "Reviewer",
			mask: int64(workbench.PermManageWorkbench | workbench.PermManageMembers)},
	})
	encoded, err := workbench.EncodePermissions(workbench.Permission(plans[0].mask))
	if err != nil {
		t.Fatalf("EncodePermissions: %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("a role with two capabilities encoded to nothing")
	}
}
