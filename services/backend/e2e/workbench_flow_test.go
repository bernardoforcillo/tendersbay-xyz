package e2e

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	workbenchv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workbench/v1"
	workspacev1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workspace/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

func (a *account) createWorkbench(t *testing.T, workspaceID, name string, v workbench.Visibility) *workbenchv1.Workbench {
	t.Helper()
	resp, err := a.workbench.CreateWorkbench(context.Background(),
		authed(&workbenchv1.CreateWorkbenchRequest{
			WorkspaceId: workspaceID, Name: name, Visibility: string(v),
		}, a.access))
	if err != nil {
		t.Fatalf("CreateWorkbench: %v", err)
	}
	return resp.Msg.Workbench
}

// Creating a workbench is a permission check against the WORKSPACE — that is
// what the nested scope buys, and it is invisible from inside the workbench
// domain because there is nothing to check yet. Over the wire it is the
// difference between a member who may open one and a stranger who may not.
func TestCreateWorkbenchIsAWorkspaceGrant(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "wb-owner")
	member := s.signUp(t, "wb-member")
	stranger := s.signUp(t, "wb-stranger")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Workbenches")
	s.invite(t, owner, ws.Id, workspace.RoleMember, member)

	// A member holds workspace.PermCreateWorkbench, so they may.
	member.createWorkbench(t, ws.Id, "Member's bid", workbench.VisibilityPrivate)

	// Somebody who is not in the workspace at all may not.
	if got := codeOf(mustErr(stranger.workbench.CreateWorkbench(ctx,
		authed(&workbenchv1.CreateWorkbenchRequest{
			WorkspaceId: ws.Id, Name: "Trespass", Visibility: string(workbench.VisibilityPrivate),
		}, stranger.access)))); got == connect.CodeUnknown {
		t.Fatal("a stranger created a workbench in somebody else's workspace")
	}
}

// A workspace administrator administers every workbench in it, including
// private ones they were never added to. That is scope.InheritWhen, and it has
// to survive the projection back into the mask the client reads.
func TestWorkspaceAdminInheritsEveryWorkbench(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "inh-owner")
	admin := s.signUp(t, "inh-admin")
	member := s.signUp(t, "inh-member")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Inheritance")
	s.invite(t, owner, ws.Id, workspace.RoleAdmin, admin)
	s.invite(t, owner, ws.Id, workspace.RoleMember, member)

	// The member's own private workbench: the admin is not a member of it.
	wb := member.createWorkbench(t, ws.Id, "Private bid", workbench.VisibilityPrivate)

	got, err := admin.workbench.GetWorkbench(ctx,
		authed(&workbenchv1.GetWorkbenchRequest{WorkbenchId: wb.Id}, admin.access))
	if err != nil {
		t.Fatalf("a workspace admin cannot reach a private workbench: %v", err)
	}
	// Inherited elevation must report as full permissions, not as view-only.
	// Reporting less would tell the client it may only look while every write
	// it attempted would succeed — the mismatch internal/rbac's Mask exists to
	// prevent.
	perms := workbench.Permission(got.Msg.MyPermissions)
	for _, bit := range []workbench.Permission{
		workbench.PermViewWorkbench, workbench.PermManageWorkbench, workbench.PermManageMembers,
	} {
		if !perms.Has(bit) {
			t.Fatalf("an inheriting admin's mask %#x is missing %#x", perms, bit)
		}
	}
	// And it is not merely reported: the write actually goes through.
	if _, err := admin.workbench.UpdateWorkbench(ctx, authed(&workbenchv1.UpdateWorkbenchRequest{
		WorkbenchId: wb.Id, Name: "Renamed by the admin",
	}, admin.access)); err != nil {
		t.Fatalf("the mask promised manage and the write was refused: %v", err)
	}
}

// A shared workbench is visible to workspace members who may see shared
// workbenches; a private one is not. This is the domain's own rule — the engine
// cannot express it, because it turns on a column the engine does not know —
// and it runs after authlayer has had its say.
func TestSharedVisibility(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "vis-owner")
	member := s.signUp(t, "vis-member")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Visibility")
	s.invite(t, owner, ws.Id, workspace.RoleMember, member)
	wb := owner.createWorkbench(t, ws.Id, "Owner's bid", workbench.VisibilityPrivate)

	if got := codeOf(mustErr(member.workbench.GetWorkbench(ctx,
		authed(&workbenchv1.GetWorkbenchRequest{WorkbenchId: wb.Id}, member.access)))); got != connect.CodeNotFound {
		t.Fatalf("a private workbench answered %v to a colleague, want not_found", got)
	}

	if _, err := owner.workbench.ChangeVisibility(ctx, authed(&workbenchv1.ChangeVisibilityRequest{
		WorkbenchId: wb.Id, Visibility: string(workbench.VisibilityShared),
	}, owner.access)); err != nil {
		t.Fatalf("ChangeVisibility: %v", err)
	}

	seen, err := member.workbench.GetWorkbench(ctx,
		authed(&workbenchv1.GetWorkbenchRequest{WorkbenchId: wb.Id}, member.access))
	if err != nil {
		t.Fatalf("a shared workbench is still hidden from a colleague: %v", err)
	}
	// Shared means look, not touch.
	perms := workbench.Permission(seen.Msg.MyPermissions)
	if !perms.Has(workbench.PermViewWorkbench) {
		t.Fatalf("a shared viewer's mask %#x cannot even view", perms)
	}
	if perms.Has(workbench.PermManageWorkbench) {
		t.Fatalf("a shared viewer's mask %#x carries manage", perms)
	}
	if got := codeOf(mustErr(member.workbench.UpdateWorkbench(ctx, authed(&workbenchv1.UpdateWorkbenchRequest{
		WorkbenchId: wb.Id, Name: "Not yours",
	}, member.access)))); got != connect.CodePermissionDenied {
		t.Fatalf("a shared viewer could rename the workbench: %v", got)
	}
}

// The regression this branch shipped and then fixed, end to end and over the
// wire: nothing cascades workbench_members when a workspace membership ends, so
// removing somebody from the workspace has to revoke the workbenches they were
// explicitly added to — reads AND writes, because the two are gated separately.
//
// Run for both roles. The manager is the sharp end: scope reports elevated for
// any member whose permissions are full, so a removed manager's standing is
// indistinguishable from an inherited elevation unless the parent is checked.
func TestRemovingAWorkspaceMemberRevokesTheirWorkbenches(t *testing.T) {
	for _, role := range []struct{ name, key string }{
		{"a viewer", workbench.RoleViewer},
		{"a manager", workbench.RoleManager},
	} {
		t.Run(role.name, func(t *testing.T) {
			s := newStack(t)
			owner := s.signUp(t, "rev-owner")
			member := s.signUp(t, "rev-member")
			ctx := context.Background()

			ws := owner.createWorkspace(t, "Revocation")
			s.invite(t, owner, ws.Id, workspace.RoleMember, member)
			wb := owner.createWorkbench(t, ws.Id, "Shared bid", workbench.VisibilityPrivate)

			if _, err := owner.workbench.AddWorkbenchMember(ctx, authed(&workbenchv1.AddWorkbenchMemberRequest{
				WorkbenchId: wb.Id, UserId: member.userID, RoleId: role.key,
			}, owner.access)); err != nil {
				t.Fatalf("AddWorkbenchMember: %v", err)
			}
			if _, err := member.workbench.GetWorkbench(ctx,
				authed(&workbenchv1.GetWorkbenchRequest{WorkbenchId: wb.Id}, member.access)); err != nil {
				t.Fatalf("the member cannot see the workbench they were added to: %v", err)
			}

			if _, err := owner.workspace.RemoveMember(ctx, authed(&workspacev1.RemoveMemberRequest{
				WorkspaceId: ws.Id, UserId: member.userID,
			}, owner.access)); err != nil {
				t.Fatalf("RemoveMember: %v", err)
			}

			for _, call := range []struct {
				name string
				run  func() error
			}{
				{"GetWorkbench", func() error {
					return mustErr(member.workbench.GetWorkbench(ctx,
						authed(&workbenchv1.GetWorkbenchRequest{WorkbenchId: wb.Id}, member.access)))
				}},
				{"ListWorkbenchMembers", func() error {
					return mustErr(member.workbench.ListWorkbenchMembers(ctx,
						authed(&workbenchv1.ListWorkbenchMembersRequest{WorkbenchId: wb.Id}, member.access)))
				}},
				{"UpdateWorkbench", func() error {
					return mustErr(member.workbench.UpdateWorkbench(ctx,
						authed(&workbenchv1.UpdateWorkbenchRequest{WorkbenchId: wb.Id, Name: "Mine now"}, member.access)))
				}},
				{"CreateWorkbenchRole", func() error {
					return mustErr(member.workbench.CreateWorkbenchRole(ctx,
						authed(&workbenchv1.CreateWorkbenchRoleRequest{
							WorkbenchId: wb.Id, Name: "Sneaky",
							Permissions: uint64(workbench.PermManageWorkbench),
						}, member.access)))
				}},
				{"AddWorkbenchMember", func() error {
					return mustErr(member.workbench.AddWorkbenchMember(ctx,
						authed(&workbenchv1.AddWorkbenchMemberRequest{
							WorkbenchId: wb.Id, UserId: member.userID, RoleId: workbench.RoleManager,
						}, member.access)))
				}},
				{"RemoveWorkbenchMember", func() error {
					return mustErr(member.workbench.RemoveWorkbenchMember(ctx,
						authed(&workbenchv1.RemoveWorkbenchMemberRequest{
							WorkbenchId: wb.Id, UserId: owner.userID,
						}, member.access)))
				}},
			} {
				if got := codeOf(call.run()); got != connect.CodeNotFound {
					t.Errorf("%s after removal = %v, want not_found", call.name, got)
				}
			}

			// And it is gone from their list, which is the other way it would
			// have shown up.
			list, err := member.workbench.ListWorkbenches(ctx,
				authed(&workbenchv1.ListWorkbenchesRequest{WorkspaceId: ws.Id}, member.access))
			if err == nil {
				for _, w := range list.Msg.Workbenches {
					if w.Id == wb.Id {
						t.Error("a removed member still lists the workbench")
					}
				}
			}
		})
	}
}

// The workbench's own owner is the deliberate exception: locking them out of
// what they own would leave a workbench nobody can transfer or delete.
func TestRemovedWorkspaceMemberKeepsAWorkbenchTheyOwn(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "own-owner")
	member := s.signUp(t, "own-member")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Ownership")
	s.invite(t, owner, ws.Id, workspace.RoleMember, member)
	wb := member.createWorkbench(t, ws.Id, "Their own bid", workbench.VisibilityPrivate)

	if _, err := owner.workspace.RemoveMember(ctx, authed(&workspacev1.RemoveMemberRequest{
		WorkspaceId: ws.Id, UserId: member.userID,
	}, owner.access)); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}

	if _, err := member.workbench.GetWorkbench(ctx,
		authed(&workbenchv1.GetWorkbenchRequest{WorkbenchId: wb.Id}, member.access)); err != nil {
		t.Fatalf("the workbench's owner lost their own workbench: %v", err)
	}
}
