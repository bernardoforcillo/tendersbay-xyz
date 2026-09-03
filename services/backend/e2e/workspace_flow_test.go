package e2e

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	workspacev1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/workspace/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// createWorkspace is the owner's first act, and the one that seeds the
// code-defined roles and the free subscription behind it.
func (a *account) createWorkspace(t *testing.T, name string) *workspacev1.Workspace {
	t.Helper()
	resp, err := a.workspace.CreateWorkspace(context.Background(),
		authed(&workspacev1.CreateWorkspaceRequest{Name: name, Slug: uniq("ws")}, a.access))
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	return resp.Msg.Workspace
}

// invite carries a colleague in by mail, all the way through: the owner sends,
// the invitee reads the token out of their own mail and redeems it.
func (s *stack) invite(t *testing.T, owner *account, workspaceID, roleID string, invitee *account) {
	t.Helper()
	ctx := context.Background()
	if _, err := owner.workspace.InviteByEmail(ctx, authed(&workspacev1.InviteByEmailRequest{
		WorkspaceId: workspaceID, Email: invitee.email, RoleId: roleID, Locale: "en-ie",
	}, owner.access)); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if _, err := invitee.workspace.AcceptEmailInvite(ctx, authed(&workspacev1.AcceptEmailInviteRequest{
		Token: s.mail.lastTo(t, "workspace_invite", invitee.email),
	}, invitee.access)); err != nil {
		t.Fatalf("AcceptEmailInvite: %v", err)
	}
}

// The whole invitation journey, and the permission mask the client is handed at
// the end of it. That mask is a projection of authlayer's grants computed in
// internal/rbac, stored as encoded grants by the migration, and re-derived on
// every read — a lot of moving parts between the role the owner picked and the
// number the client branches on.
func TestInviteAcceptAndRoleProjection(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "ws-owner")
	member := s.signUp(t, "ws-member")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Acme Bids")
	s.invite(t, owner, ws.Id, workspace.RoleMember, member)

	got, err := member.workspace.GetWorkspace(ctx,
		authed(&workspacev1.GetWorkspaceRequest{WorkspaceId: ws.Id}, member.access))
	if err != nil {
		t.Fatalf("the invitee cannot see the workspace they joined: %v", err)
	}
	perms := workspace.Permission(got.Msg.MyPermissions)

	// What a member is: they belong, they can see and create workbenches, and
	// they administer nothing.
	for _, want := range []struct {
		name string
		bit  workspace.Permission
	}{
		{"view the workspace", workspace.PermViewWorkspace},
		{"see workbenches", workspace.PermViewWorkbenches},
		{"create a workbench", workspace.PermCreateWorkbench},
	} {
		if !perms.Has(want.bit) {
			t.Errorf("a member cannot %s (mask %#x)", want.name, perms)
		}
	}
	for _, forbidden := range []struct {
		name string
		bit  workspace.Permission
	}{
		{"manage members", workspace.PermManageMembers},
		{"manage roles", workspace.PermManageRoles},
		{"administer the workspace", workspace.PermAdministrator},
	} {
		if perms.Has(forbidden.bit) {
			t.Errorf("a plain member may %s (mask %#x)", forbidden.name, perms)
		}
	}

	// And the owner sees both of them on the roster, with profiles resolved.
	members, err := owner.workspace.ListMembers(ctx,
		authed(&workspacev1.ListMembersRequest{WorkspaceId: ws.Id}, owner.access))
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members.Msg.Members) != 2 {
		t.Fatalf("roster has %d members, want 2", len(members.Msg.Members))
	}
	for _, m := range members.Msg.Members {
		if m.User == nil || m.User.Email == "" {
			t.Fatalf("member %s came back with no profile resolved", m.UserId)
		}
		if m.RoleName == "" || m.RoleName == m.RoleId {
			t.Fatalf("member %s shows the raw role key %q instead of a label", m.UserId, m.RoleName)
		}
	}
}

// A plain member must not be able to administer anything, and must be told so
// with permission_denied rather than with a 500 or a silent success.
func TestMemberCannotAdminister(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "guard-owner")
	member := s.signUp(t, "guard-member")
	victim := s.signUp(t, "guard-victim")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Guarded")
	s.invite(t, owner, ws.Id, workspace.RoleMember, member)
	s.invite(t, owner, ws.Id, workspace.RoleMember, victim)

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"remove a colleague", func() error {
			_, err := member.workspace.RemoveMember(ctx, authed(&workspacev1.RemoveMemberRequest{
				WorkspaceId: ws.Id, UserId: victim.userID,
			}, member.access))
			return err
		}},
		{"promote a colleague", func() error {
			_, err := member.workspace.ChangeMemberRole(ctx, authed(&workspacev1.ChangeMemberRoleRequest{
				WorkspaceId: ws.Id, UserId: victim.userID, RoleId: workspace.RoleAdmin,
			}, member.access))
			return err
		}},
		{"mint a role", func() error {
			_, err := member.workspace.CreateRole(ctx, authed(&workspacev1.CreateRoleRequest{
				WorkspaceId: ws.Id, Name: "Sneaky", Permissions: uint64(workspace.PermManageMembers),
			}, member.access))
			return err
		}},
		{"invite somebody", func() error {
			_, err := member.workspace.InviteByEmail(ctx, authed(&workspacev1.InviteByEmailRequest{
				WorkspaceId: ws.Id, Email: "nobody@e2e.test", RoleId: workspace.RoleMember, Locale: "en-ie",
			}, member.access))
			return err
		}},
	} {
		if got := codeOf(tc.call()); got != connect.CodePermissionDenied {
			t.Errorf("a member could %s: code %v", tc.name, got)
		}
	}
}

// The privilege-escalation guard, over the wire. An admin may promote people,
// but not past themselves — authlayer enforces it and the transport has to
// carry the refusal through as something other than a generic failure.
func TestAdminCannotEscalateBeyondThemselves(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "esc-owner")
	admin := s.signUp(t, "esc-admin")
	target := s.signUp(t, "esc-target")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Escalation")
	s.invite(t, owner, ws.Id, workspace.RoleAdmin, admin)
	s.invite(t, owner, ws.Id, workspace.RoleMember, target)

	// An admin can do the ordinary thing.
	if _, err := admin.workspace.ChangeMemberRole(ctx, authed(&workspacev1.ChangeMemberRoleRequest{
		WorkspaceId: ws.Id, UserId: target.userID, RoleId: workspace.RoleAdmin,
	}, admin.access)); err != nil {
		t.Fatalf("an admin could not promote a member to admin: %v", err)
	}

	// But not hand out ownership — that is the owner's alone.
	if got := codeOf(mustErr(admin.workspace.TransferOwnership(ctx, authed(
		&workspacev1.TransferOwnershipRequest{WorkspaceId: ws.Id, NewOwnerUserId: target.userID},
		admin.access)))); got == connect.CodeUnknown {
		t.Fatal("an admin transferred ownership of a workspace they do not own")
	}
}

// What a stranger holding a workspace id can and cannot learn.
//
// The answer is permission_denied, which DOES confirm the workspace exists —
// unlike the workbench domain, which answers not_found precisely so that it
// does not. The difference is long-standing rather than introduced here: the
// pre-authlayer code answered the same way, through ErrNotMember, and the
// migration preserved it. It is pinned here because it is wire-visible: the
// client branches on the code, so changing it is a deliberate API decision
// somebody should make on purpose, not a detail that drifts.
//
// The leak it allows is narrow — workspace ids are UUIDs, so an id is something
// you were given rather than something you can guess — and the disclosure that
// would actually matter is closed below: a stranger's own list never mentions
// it.
func TestStrangerLearnsNothingUseful(t *testing.T) {
	s := newStack(t)
	owner := s.signUp(t, "priv-owner")
	stranger := s.signUp(t, "priv-stranger")
	ctx := context.Background()

	ws := owner.createWorkspace(t, "Private")

	if got := codeOf(mustErr(stranger.workspace.GetWorkspace(ctx,
		authed(&workspacev1.GetWorkspaceRequest{WorkspaceId: ws.Id}, stranger.access)))); got != connect.CodePermissionDenied {
		t.Fatalf("GetWorkspace as a stranger = %v, want permission_denied (see this test's doc)", got)
	}
	mine, err := stranger.workspace.ListMyWorkspaces(ctx,
		authed(&workspacev1.ListMyWorkspacesRequest{}, stranger.access))
	if err != nil {
		t.Fatalf("ListMyWorkspaces: %v", err)
	}
	for _, w := range mine.Msg.Workspaces {
		if w.Id == ws.Id {
			t.Fatal("a stranger's workspace list includes one they were never invited to")
		}
	}
}

// mustErr discards a successful response so a test can assert on the error
// alone. A nil error reports CodeUnknown through codeOf, which no handler
// returns on purpose.
func mustErr[T any](_ *connect.Response[T], err error) error { return err }
