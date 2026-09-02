package main

import (
	"testing"

	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// The workbench scope is nested inside the workspace scope, and the two halves
// of that nesting are declared in two packages that deliberately do not import
// each other. This is where they are wired together, so this is where the
// contract between them is checked.
//
// Nothing about a mismatch is loud: authlayer answers "not allowed" for an
// undeclared or misspelled grant, so a workspace that stopped declaring
// "workbench" would not fail to build or to start — every user would simply
// lose the ability to create a workbench, and every workspace administrator
// would quietly stop administering them.
func TestWorkbenchNestingMatchesTheWorkspaceSurface(t *testing.T) {
	ws := workspace.Statements()

	actions, ok := ws[workbench.ResourceWorkbench]
	if !ok {
		t.Fatalf("the workspace declares no %q resource; the nesting has nothing to ask about", workbench.ResourceWorkbench)
	}
	declared := map[access.Action]bool{}
	for _, a := range actions {
		declared[a] = true
	}

	// scope.WithContainerResource makes CreateContainer ask the parent for
	// <resource>:create — see workbench.NewService.
	if !declared[scope.ActionCreate] {
		t.Errorf("the workspace must declare %s:create, or nobody but a workspace administrator can create a workbench", workbench.ResourceWorkbench)
	}
	// scope.InheritWhen projects <resource>:manage onto elevation in every
	// workbench.
	if !declared[workbench.ActionManage] {
		t.Errorf("the workspace must declare %s:%s, or no workspace administrator is elevated in a workbench",
			workbench.ResourceWorkbench, workbench.ActionManage)
	}
	// The shared-visibility rule reads this one through workspace.Standing.
	if !declared[scope.ActionRead] {
		t.Errorf("the workspace must declare %s:read, or shared workbenches are invisible", workbench.ResourceWorkbench)
	}

	// The two packages spell the manage action independently; they have to
	// spell it the same.
	if string(workbench.ActionManage) != string(workspace.ActionManage) {
		t.Errorf("manage action mismatch: workbench %q, workspace %q", workbench.ActionManage, workspace.ActionManage)
	}
	if workbench.ResourceWorkbench != workspace.ResourceWorkbench {
		t.Errorf("resource mismatch: workbench %q, workspace %q", workbench.ResourceWorkbench, workspace.ResourceWorkbench)
	}

	// A workspace member holding PermManageWorkbenches must be exactly the
	// actor the inheritance elevates, so the mask the client speaks and the
	// grant the engine reads cannot drift apart.
	ac := workspace.NewAccess()
	perms, err := ac.Permission(map[string][]access.Action{
		workbench.ResourceWorkbench: {workbench.ActionManage},
	})
	if err != nil {
		t.Fatalf("workspace access refuses the grant the nesting inherits from: %v", err)
	}
	if !perms.Allows(workbench.ResourceWorkbench, workbench.ActionManage) {
		t.Fatal("the inherited grant does not resolve against the workspace's own access engine")
	}
}
