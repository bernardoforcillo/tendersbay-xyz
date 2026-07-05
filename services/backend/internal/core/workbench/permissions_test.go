package workbench

import "testing"

func TestPermissionHasAndSubset(t *testing.T) {
	p := PermViewWorkbench | PermManageMembers
	if !p.Has(PermViewWorkbench) {
		t.Fatal("expected Has(View)")
	}
	if p.Has(PermManageRoles) {
		t.Fatal("did not expect Has(ManageRoles)")
	}
	if !p.subsetOf(permAdminRole) {
		t.Fatal("expected subset of admin role")
	}
	if permAdminRole.subsetOf(PermViewWorkbench) {
		t.Fatal("admin role is not a subset of view-only")
	}
}

func TestVisibilityConstants(t *testing.T) {
	if VisibilityPrivate != "private" || VisibilityShared != "shared" {
		t.Fatalf("unexpected visibility values: %q %q", VisibilityPrivate, VisibilityShared)
	}
}
