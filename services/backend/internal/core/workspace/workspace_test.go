package workspace_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bernardoforcillo/authlayer/scope"
	memstore "github.com/bernardoforcillo/authlayer/store/memory"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// ── fakes ───────────────────────────────────────────────────────────────────

// testStore is authlayer's in-memory scope store plus the one thing it
// deliberately does not model: the slug UNIQUE constraint. In production that
// constraint lives in Postgres and is what turns a duplicate into
// scope.ErrConflict, so the mapping to ErrSlugTaken would otherwise be
// untestable without a database.
//
// It doubles as workspace.Repository, because the queries that port covers read
// and write the very same workspaces row this store does.
type testStore struct {
	workspace.Store

	mu     sync.Mutex
	bySlug map[string]string
	byID   map[string]workspace.Workspace
}

func newTestStore() *testStore {
	return &testStore{
		Store:  memstore.New[workspace.Workspace, workspace.Member](),
		bySlug: map[string]string{},
		byID:   map[string]workspace.Workspace{},
	}
}

func (s *testStore) CreateContainer(ctx context.Context, w workspace.Workspace) (workspace.Workspace, error) {
	s.mu.Lock()
	if id, taken := s.bySlug[w.Slug]; taken && id != w.ID {
		s.mu.Unlock()
		return workspace.Workspace{}, scope.ErrConflict
	}
	s.mu.Unlock()

	created, err := s.Store.CreateContainer(ctx, w)
	if err != nil {
		return workspace.Workspace{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bySlug[created.Slug] = created.ID
	s.byID[created.ID] = created
	return created, nil
}

// WithTx hands the WRAPPER to the closure, not the embedded memory store.
// authlayer's CreateContainer seeds the owner's membership in one transaction,
// so without this the inner CreateContainer would be the memory store's and the
// slug constraint above would never run.
func (s *testStore) WithTx(_ context.Context, fn func(workspace.Store) error) error {
	return fn(s)
}

func (s *testStore) FindBySlug(_ context.Context, slug string) (workspace.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.bySlug[slug]
	if !ok {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}
	return s.byID[id], nil
}

// UpdateNameSlug writes the workspaces row. The scope store's own copy of the
// container is not refreshed here because authlayer's memory store exposes no
// container update — in production both read the same row, so only the fake
// can see them diverge.
func (s *testStore) UpdateNameSlug(_ context.Context, id, name, slug string) (workspace.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws, ok := s.byID[id]
	if !ok {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}
	delete(s.bySlug, ws.Slug)
	ws.Name, ws.Slug = name, slug
	s.byID[id] = ws
	s.bySlug[slug] = id
	return ws, nil
}

func (s *testStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ws, ok := s.byID[id]; ok {
		delete(s.bySlug, ws.Slug)
		delete(s.byID, id)
	}
	return nil
}

type fakeUsers struct{ byID map[string]auth.User }

func newFakeUsers(ids ...string) *fakeUsers {
	u := &fakeUsers{byID: map[string]auth.User{}}
	for _, id := range ids {
		u.byID[id] = auth.User{ID: id, Email: id + "@example.com", DisplayName: strings.ToUpper(id)}
	}
	return u
}

func (f *fakeUsers) FindByID(_ context.Context, id string) (auth.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return u, nil
}

func (f *fakeUsers) FindByEmail(_ context.Context, email string) (auth.User, error) {
	for _, u := range f.byID {
		if u.Email == email {
			return u, nil
		}
	}
	return auth.User{}, auth.ErrNotFound
}

type sentInvite struct{ to, workspaceName, inviter, link string }

type fakeMailer struct{ sent []sentInvite }

func (m *fakeMailer) SendWorkspaceInvite(_ context.Context, to, workspaceName, inviter, link string) error {
	m.sent = append(m.sent, sentInvite{to, workspaceName, inviter, link})
	return nil
}

// ── fixture ─────────────────────────────────────────────────────────────────

type fixture struct {
	svc   *workspace.Service
	store *testStore
	users *fakeUsers
	mail  *fakeMailer
}

func newFixture(t *testing.T, inviteExpiry time.Duration, userIDs ...string) *fixture {
	t.Helper()
	store := newTestStore()
	users := newFakeUsers(userIDs...)
	mail := &fakeMailer{}
	svc := workspace.NewService(
		workspace.NewAccess(),
		store,
		memstore.NewInviteStore(),
		store,
		users,
		mail,
		workspace.Config{AppBaseURL: "https://app.test", InviteExpiry: inviteExpiry},
	)
	return &fixture{svc: svc, store: store, users: users, mail: mail}
}

func (f *fixture) workspace(t *testing.T, ownerID, name string) workspace.Workspace {
	t.Helper()
	ws, err := f.svc.CreateWorkspace(context.Background(), ownerID, name, "")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	return ws
}

// tokenOf pulls the invitation token out of the link the service mailed.
func tokenOf(t *testing.T, link string) string {
	t.Helper()
	_, tok, ok := strings.Cut(link, "token=")
	if !ok {
		t.Fatalf("no token in link %q", link)
	}
	return tok
}

// ── the permission codec ────────────────────────────────────────────────────

// Every bit the client can send must survive the round trip through authlayer's
// grant set, or a role editor would silently drop capabilities on save.
func TestPermissionMaskRoundTrip(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "member")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	for _, bit := range []workspace.Permission{
		workspace.PermManageWorkspace,
		workspace.PermManageMembers,
		workspace.PermManageRoles,
		workspace.PermCreateInvite,
		workspace.PermManageInvites,
		workspace.PermViewWorkbenches,
		workspace.PermCreateWorkbench,
		workspace.PermManageWorkbenches,
	} {
		role, err := f.svc.CreateRole(ctx, "owner", ws.ID, "Probe", bit)
		if err != nil {
			t.Fatalf("CreateRole(%d): %v", bit, err)
		}
		if !role.Permissions.Has(bit) {
			t.Fatalf("bit %d did not survive the round trip: got %d", bit, role.Permissions)
		}
		if role.Permissions.Has(workspace.PermAdministrator) {
			t.Fatalf("a single-capability role must not read as administrator: %d", role.Permissions)
		}
		if err := f.svc.DeleteRole(ctx, "owner", ws.ID, role.ID); err != nil {
			t.Fatalf("DeleteRole: %v", err)
		}
	}
}

// PermAdministrator is not a grant but a standing: a role holding every grant
// is what authlayer treats as elevated.
func TestCreateRole_AdministratorGrantsEverything(t *testing.T) {
	f := newFixture(t, time.Hour, "owner")
	ws := f.workspace(t, "owner", "Acme")
	role, err := f.svc.CreateRole(context.Background(), "owner", ws.ID, "Boss", workspace.PermAdministrator)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	for _, bit := range []workspace.Permission{
		workspace.PermAdministrator, workspace.PermManageWorkspace, workspace.PermManageMembers,
		workspace.PermManageRoles, workspace.PermCreateInvite, workspace.PermManageInvites,
		workspace.PermViewWorkbenches, workspace.PermCreateWorkbench, workspace.PermManageWorkbenches,
	} {
		if !role.Permissions.Has(bit) {
			t.Fatalf("administrator role is missing bit %d (got %d)", bit, role.Permissions)
		}
	}
}

// ── workspace lifecycle ─────────────────────────────────────────────────────

func TestCreateWorkspace_OwnerIsAnElevatedMember(t *testing.T) {
	f := newFixture(t, time.Hour, "owner")
	ws := f.workspace(t, "owner", "Acme")
	if ws.OwnerID != "owner" {
		t.Fatalf("owner = %q", ws.OwnerID)
	}
	got, perms, err := f.svc.GetWorkspace(context.Background(), "owner", ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if got.Name != "Acme" {
		t.Fatalf("name = %q", got.Name)
	}
	if !perms.Has(workspace.PermAdministrator) || !perms.Has(workspace.PermViewWorkspace) {
		t.Fatalf("owner permissions = %d, want administrator + view", perms)
	}
}

func TestCreateWorkspace_DerivesSlug(t *testing.T) {
	f := newFixture(t, time.Hour, "owner")
	ws := f.workspace(t, "owner", "Acme  Corp!")
	if ws.Slug != "acme-corp" {
		t.Fatalf("slug = %q, want acme-corp", ws.Slug)
	}
}

func TestCreateWorkspace_SlugTaken(t *testing.T) {
	f := newFixture(t, time.Hour, "owner")
	f.workspace(t, "owner", "Acme")
	_, err := f.svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	if !errors.Is(err, workspace.ErrSlugTaken) {
		t.Fatalf("err = %v, want ErrSlugTaken", err)
	}
}

func TestListMyWorkspaces(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "stranger")
	f.workspace(t, "owner", "Acme")
	f.workspace(t, "owner", "Beta")

	mine, err := f.svc.ListMyWorkspaces(context.Background(), "owner")
	if err != nil {
		t.Fatalf("ListMyWorkspaces: %v", err)
	}
	if len(mine) != 2 {
		t.Fatalf("got %d workspaces, want 2", len(mine))
	}
	theirs, err := f.svc.ListMyWorkspaces(context.Background(), "stranger")
	if err != nil {
		t.Fatalf("ListMyWorkspaces: %v", err)
	}
	if len(theirs) != 0 {
		t.Fatalf("a stranger sees %d workspaces, want 0", len(theirs))
	}
}

func TestUpdateWorkspace_OwnerBypassesTheGrantCheck(t *testing.T) {
	f := newFixture(t, time.Hour, "owner")
	ws := f.workspace(t, "owner", "Acme")
	got, err := f.svc.UpdateWorkspace(context.Background(), "owner", ws.ID, "Acme Two", "")
	if err != nil {
		t.Fatalf("UpdateWorkspace: %v", err)
	}
	if got.Name != "Acme Two" || got.Slug != "acme-two" {
		t.Fatalf("got %+v", got)
	}
}

// The baseline member role can see and create workbenches — that is what the
// seeded "Member" role granted before authlayer — and nothing else.
func TestUpdateWorkspace_PlainMemberForbidden(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	addMember(t, f, ws.ID, "bob", workspace.RoleMember)

	_, err := f.svc.UpdateWorkspace(ctx, "bob", ws.ID, "Hijacked", "")
	if !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	_, perms, err := f.svc.GetWorkspace(ctx, "bob", ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if !perms.Has(workspace.PermViewWorkbenches) || !perms.Has(workspace.PermCreateWorkbench) {
		t.Fatalf("member permissions = %d, want the workbench baseline", perms)
	}
	if perms.Has(workspace.PermManageWorkspace) || perms.Has(workspace.PermAdministrator) {
		t.Fatalf("member permissions = %d, want no management bits", perms)
	}
}

func TestGetWorkspace_NotMember(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "stranger")
	ws := f.workspace(t, "owner", "Acme")
	if _, _, err := f.svc.GetWorkspace(context.Background(), "stranger", ws.ID); !errors.Is(err, workspace.ErrNotMember) {
		t.Fatalf("err = %v, want ErrNotMember", err)
	}
}

func TestDeleteWorkspace_OwnerOnly(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "admin")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	addMember(t, f, ws.ID, "admin", workspace.RoleAdmin)

	// An admin is elevated for every grant, and still may not delete: deletion
	// is not a grant at all.
	if err := f.svc.DeleteWorkspace(ctx, "admin", ws.ID); !errors.Is(err, workspace.ErrOwnerOnly) {
		t.Fatalf("err = %v, want ErrOwnerOnly", err)
	}
	if err := f.svc.DeleteWorkspace(ctx, "owner", ws.ID); err != nil {
		t.Fatalf("DeleteWorkspace: %v", err)
	}
}

func TestLeaveWorkspace_OwnerBlocked(t *testing.T) {
	f := newFixture(t, time.Hour, "owner")
	ws := f.workspace(t, "owner", "Acme")
	if err := f.svc.LeaveWorkspace(context.Background(), "owner", ws.ID); !errors.Is(err, workspace.ErrLastOwner) {
		t.Fatalf("err = %v, want ErrLastOwner", err)
	}
}

func TestTransferOwnership_RequiresMember(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob", "stranger")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	if err := f.svc.TransferOwnership(ctx, "owner", ws.ID, "stranger"); err == nil {
		t.Fatal("transferring to a non-member must fail")
	}
	addMember(t, f, ws.ID, "bob", workspace.RoleMember)
	if err := f.svc.TransferOwnership(ctx, "owner", ws.ID, "bob"); err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}
	got, _, err := f.svc.GetWorkspace(ctx, "bob", ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if got.OwnerID != "bob" {
		t.Fatalf("owner = %q, want bob", got.OwnerID)
	}
}

// ── members ─────────────────────────────────────────────────────────────────

// addMember admits a user through an invite link, which is the only way in
// besides creating the workspace.
func addMember(t *testing.T, f *fixture, workspaceID, userID, roleKey string) {
	t.Helper()
	ctx := context.Background()
	owner, _, err := f.svc.GetWorkspace(ctx, ownerOf(t, f, workspaceID), workspaceID)
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	link, err := f.svc.CreateInviteLink(ctx, owner.OwnerID, workspaceID, roleKey, 0, nil)
	if err != nil {
		t.Fatalf("CreateInviteLink: %v", err)
	}
	if _, err := f.svc.JoinViaInviteLink(ctx, userID, link.Code); err != nil {
		t.Fatalf("JoinViaInviteLink: %v", err)
	}
}

func ownerOf(t *testing.T, f *fixture, workspaceID string) string {
	t.Helper()
	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	ws, ok := f.store.byID[workspaceID]
	if !ok {
		t.Fatalf("unknown workspace %q", workspaceID)
	}
	return ws.OwnerID
}

func TestListMembers(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	addMember(t, f, ws.ID, "bob", workspace.RoleMember)

	views, err := f.svc.ListMembers(ctx, "owner", ws.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("got %d members, want 2", len(views))
	}
	for _, v := range views {
		if v.User.DisplayName == "" {
			t.Fatalf("member %q has no profile joined in", v.Member.UserID)
		}
		if v.Member.ContainerID != ws.ID {
			t.Fatalf("member carries container %q, want %q", v.Member.ContainerID, ws.ID)
		}
	}
}

func TestChangeMemberRole_OneRolePerMember(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	addMember(t, f, ws.ID, "bob", workspace.RoleMember)

	view, err := f.svc.ChangeMemberRole(ctx, "owner", ws.ID, "bob", workspace.RoleAdmin)
	if err != nil {
		t.Fatalf("ChangeMemberRole: %v", err)
	}
	if view.Member.RoleKey != workspace.RoleAdmin {
		t.Fatalf("role = %q, want admin", view.Member.RoleKey)
	}
	_, perms, err := f.svc.GetWorkspace(ctx, "bob", ws.ID)
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if !perms.Has(workspace.PermManageMembers) {
		t.Fatalf("promoted member permissions = %d", perms)
	}
}

// A member manager may not hand out more than they hold themselves.
func TestChangeMemberRole_PrivilegeEscalationBlocked(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "mod", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	mod, err := f.svc.CreateRole(ctx, "owner", ws.ID, "Moderator",
		workspace.PermManageMembers|workspace.PermViewWorkbenches)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	addMember(t, f, ws.ID, "mod", mod.ID)
	addMember(t, f, ws.ID, "bob", workspace.RoleMember)

	if _, err := f.svc.ChangeMemberRole(ctx, "mod", ws.ID, "bob", workspace.RoleAdmin); !errors.Is(err, workspace.ErrPrivilegeEscalation) {
		t.Fatalf("err = %v, want ErrPrivilegeEscalation", err)
	}
}

func TestRemoveMember_CannotRemoveOwner(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "admin")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	addMember(t, f, ws.ID, "admin", workspace.RoleAdmin)

	if err := f.svc.RemoveMember(ctx, "admin", ws.ID, "owner"); !errors.Is(err, workspace.ErrLastOwner) {
		t.Fatalf("err = %v, want ErrLastOwner", err)
	}
}

func TestLoadMembership(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob", "stranger")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	addMember(t, f, ws.ID, "bob", workspace.RoleMember)

	m, err := f.svc.LoadMembership(ctx, ws.ID, "bob")
	if err != nil {
		t.Fatalf("LoadMembership: %v", err)
	}
	if m.Member.UserID != "bob" || m.Member.ContainerID != ws.ID {
		t.Fatalf("membership = %+v", m.Member)
	}
	if !m.Role.Permissions.Has(workspace.PermViewWorkspace) {
		t.Fatalf("permissions = %d, want at least view", m.Role.Permissions)
	}
	if _, err := f.svc.LoadMembership(ctx, ws.ID, "stranger"); !errors.Is(err, workspace.ErrNotMember) {
		t.Fatalf("err = %v, want ErrNotMember", err)
	}
}

func TestStanding_ReportsOwnershipAndNonMembership(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "stranger")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	own, err := f.svc.Standing(ctx, ws.ID, "owner")
	if err != nil {
		t.Fatalf("Standing: %v", err)
	}
	if !own.IsOwner || !own.IsMember || own.WorkspaceName != "Acme" {
		t.Fatalf("owner standing = %+v", own)
	}
	out, err := f.svc.Standing(ctx, ws.ID, "stranger")
	if err != nil {
		t.Fatalf("Standing for a non-member must not be an error: %v", err)
	}
	if out.IsMember || out.IsOwner {
		t.Fatalf("stranger standing = %+v", out)
	}
	if out.WorkspaceName != "Acme" {
		t.Fatalf("a non-member still needs the workspace name, got %q", out.WorkspaceName)
	}
}

// ── roles ───────────────────────────────────────────────────────────────────

func TestListRoles_IncludesTheCodeDefinedOnes(t *testing.T) {
	f := newFixture(t, time.Hour, "owner")
	ws := f.workspace(t, "owner", "Acme")
	roles, err := f.svc.ListRoles(context.Background(), "owner", ws.ID)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	seen := map[string]workspace.Role{}
	for _, r := range roles {
		seen[r.ID] = r
	}
	for _, key := range []string{workspace.RoleOwner, workspace.RoleAdmin, workspace.RoleMember} {
		r, ok := seen[key]
		if !ok {
			t.Fatalf("role %q missing from %v", key, seen)
		}
		if !r.IsDefault {
			t.Fatalf("role %q should be marked default", key)
		}
	}
}

func TestDeleteRole_DefaultRefusedAndInUseRefused(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	if err := f.svc.DeleteRole(ctx, "owner", ws.ID, workspace.RoleMember); !errors.Is(err, workspace.ErrDefaultRole) {
		t.Fatalf("err = %v, want ErrDefaultRole", err)
	}

	custom, err := f.svc.CreateRole(ctx, "owner", ws.ID, "Reviewer", workspace.PermViewWorkbenches)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	addMember(t, f, ws.ID, "bob", custom.ID)
	if err := f.svc.DeleteRole(ctx, "owner", ws.ID, custom.ID); !errors.Is(err, workspace.ErrRoleInUse) {
		t.Fatalf("err = %v, want ErrRoleInUse", err)
	}
}

func TestCreateRole_KeyDerivedFromName(t *testing.T) {
	f := newFixture(t, time.Hour, "owner")
	role, err := f.svc.CreateRole(context.Background(), "owner", f.workspace(t, "owner", "Acme").ID,
		"Bid Reviewer", workspace.PermViewWorkbenches)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if role.ID != "bid-reviewer" {
		t.Fatalf("role key = %q, want bid-reviewer", role.ID)
	}
}

// ── email invitations ───────────────────────────────────────────────────────

func TestInviteByEmail_SendsMailAndAdmits(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	inv, err := f.svc.InviteByEmail(ctx, "owner", ws.ID, "Bob@Example.com", workspace.RoleMember, "it-it")
	if err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if inv.Email != "bob@example.com" {
		t.Fatalf("invitation address = %q, want it normalized", inv.Email)
	}
	if len(f.mail.sent) != 1 {
		t.Fatalf("sent %d emails, want 1", len(f.mail.sent))
	}
	sent := f.mail.sent[0]
	if sent.workspaceName != "Acme" || !strings.HasPrefix(sent.link, "https://app.test/it-it/workspace/accept-invite?token=") {
		t.Fatalf("mail = %+v", sent)
	}

	joined, err := f.svc.AcceptEmailInvite(ctx, "bob", tokenOf(t, sent.link))
	if err != nil {
		t.Fatalf("AcceptEmailInvite: %v", err)
	}
	if joined.ID != ws.ID {
		t.Fatalf("joined %q, want %q", joined.ID, ws.ID)
	}
	if _, err := f.svc.LoadMembership(ctx, ws.ID, "bob"); err != nil {
		t.Fatalf("invitee is not a member: %v", err)
	}
	// The token is spent.
	if _, err := f.svc.AcceptEmailInvite(ctx, "bob", tokenOf(t, sent.link)); !errors.Is(err, workspace.ErrInviteInvalid) {
		t.Fatalf("replayed invite err = %v, want ErrInviteInvalid", err)
	}
}

func TestInviteByEmail_AlreadyMember(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	addMember(t, f, ws.ID, "bob", workspace.RoleMember)

	_, err := f.svc.InviteByEmail(ctx, "owner", ws.ID, "bob@example.com", workspace.RoleMember, "en-ie")
	if !errors.Is(err, workspace.ErrAlreadyMember) {
		t.Fatalf("err = %v, want ErrAlreadyMember", err)
	}
}

func TestAcceptEmailInvite_Expired(t *testing.T) {
	// 1ns, not a negative duration: authlayer ignores a non-positive
	// expiry rather than minting an already-dead invite.
	f := newFixture(t, time.Nanosecond, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	if _, err := f.svc.InviteByEmail(ctx, "owner", ws.ID, "bob@example.com", workspace.RoleMember, "en-ie"); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	_, err := f.svc.AcceptEmailInvite(ctx, "bob", tokenOf(t, f.mail.sent[0].link))
	if !errors.Is(err, workspace.ErrInviteExpired) {
		t.Fatalf("err = %v, want ErrInviteExpired", err)
	}
}

func TestPreviewEmailInvite(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	if _, err := f.svc.InviteByEmail(ctx, "owner", ws.ID, "bob@example.com", workspace.RoleMember, "en-ie"); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	p, err := f.svc.PreviewEmailInvite(ctx, tokenOf(t, f.mail.sent[0].link))
	if err != nil {
		t.Fatalf("PreviewEmailInvite: %v", err)
	}
	if !p.Valid || p.WorkspaceName != "Acme" || p.Email != "bob@example.com" {
		t.Fatalf("preview = %+v", p)
	}

	unknown, err := f.svc.PreviewEmailInvite(ctx, "never-issued")
	if err != nil {
		t.Fatalf("an unknown token is not an error, it is an invalid preview: %v", err)
	}
	if unknown.Valid {
		t.Fatal("unknown token previewed as valid")
	}
}

// ── invite links ────────────────────────────────────────────────────────────

func TestInviteLink_MaxUsesExhausted(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob", "carol")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	link, err := f.svc.CreateInviteLink(ctx, "owner", ws.ID, workspace.RoleMember, 1, nil)
	if err != nil {
		t.Fatalf("CreateInviteLink: %v", err)
	}
	if _, err := f.svc.JoinViaInviteLink(ctx, "bob", link.Code); err != nil {
		t.Fatalf("first join: %v", err)
	}
	if _, err := f.svc.JoinViaInviteLink(ctx, "carol", link.Code); !errors.Is(err, workspace.ErrLinkExhausted) {
		t.Fatalf("err = %v, want ErrLinkExhausted", err)
	}
}

func TestInviteLink_Revoked(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	link, err := f.svc.CreateInviteLink(ctx, "owner", ws.ID, workspace.RoleMember, 0, nil)
	if err != nil {
		t.Fatalf("CreateInviteLink: %v", err)
	}
	if err := f.svc.RevokeInviteLink(ctx, "owner", ws.ID, link.ID); err != nil {
		t.Fatalf("RevokeInviteLink: %v", err)
	}
	if _, err := f.svc.JoinViaInviteLink(ctx, "bob", link.Code); !errors.Is(err, workspace.ErrLinkInvalid) {
		t.Fatalf("err = %v, want ErrLinkInvalid", err)
	}
	links, err := f.svc.ListInviteLinks(ctx, "owner", ws.ID)
	if err != nil {
		t.Fatalf("ListInviteLinks: %v", err)
	}
	if len(links) != 1 || !links[0].Revoked {
		t.Fatalf("links = %+v, want one revoked", links)
	}
}

func TestInviteLink_Expired(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	past := time.Now().Add(-time.Hour)

	link, err := f.svc.CreateInviteLink(ctx, "owner", ws.ID, workspace.RoleMember, 0, &past)
	if err != nil {
		t.Fatalf("CreateInviteLink: %v", err)
	}
	if _, err := f.svc.JoinViaInviteLink(ctx, "bob", link.Code); !errors.Is(err, workspace.ErrLinkExpired) {
		t.Fatalf("err = %v, want ErrLinkExpired", err)
	}
}

// Re-following a link you already used must not burn one of its uses.
func TestJoinViaInviteLink_AlreadyMemberDoesNotConsumeAUse(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")

	link, err := f.svc.CreateInviteLink(ctx, "owner", ws.ID, workspace.RoleMember, 2, nil)
	if err != nil {
		t.Fatalf("CreateInviteLink: %v", err)
	}
	for range 2 {
		if _, err := f.svc.JoinViaInviteLink(ctx, "bob", link.Code); err != nil {
			t.Fatalf("JoinViaInviteLink: %v", err)
		}
	}
	links, err := f.svc.ListInviteLinks(ctx, "owner", ws.ID)
	if err != nil {
		t.Fatalf("ListInviteLinks: %v", err)
	}
	if links[0].UseCount != 1 {
		t.Fatalf("use count = %d, want 1", links[0].UseCount)
	}
}

func TestInviteLink_PlainMemberCannotCreate(t *testing.T) {
	f := newFixture(t, time.Hour, "owner", "bob")
	ctx := context.Background()
	ws := f.workspace(t, "owner", "Acme")
	addMember(t, f, ws.ID, "bob", workspace.RoleMember)

	if _, err := f.svc.CreateInviteLink(ctx, "bob", ws.ID, workspace.RoleMember, 0, nil); !errors.Is(err, workspace.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}
