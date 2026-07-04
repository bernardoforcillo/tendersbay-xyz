package workspace_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// ── in-memory state shared by the fake repos and the fake unit of work ───────

type state struct {
	ws    map[string]workspace.Workspace
	roles map[string]workspace.Role
	mem   map[string]workspace.Member // key: workspaceID + "|" + userID
	einv  map[string]workspace.EmailInvitation
	links map[string]workspace.InviteLink
	users map[string]auth.User // by id
	n     int
}

func newState() *state {
	return &state{
		ws:    map[string]workspace.Workspace{},
		roles: map[string]workspace.Role{},
		mem:   map[string]workspace.Member{},
		einv:  map[string]workspace.EmailInvitation{},
		links: map[string]workspace.InviteLink{},
		users: map[string]auth.User{},
	}
}

func (s *state) id() string { s.n++; return fmt.Sprintf("id-%d", s.n) }

func memKey(ws, user string) string { return ws + "|" + user }

func (s *state) addUser(id, email, name string) {
	s.users[id] = auth.User{ID: id, Email: email, DisplayName: name}
}

// ── fakes ────────────────────────────────────────────────────────────────────

type wsRepo struct{ s *state }

func (r wsRepo) Create(_ context.Context, w workspace.Workspace) (workspace.Workspace, error) {
	w.ID = r.s.id()
	w.CreatedAt, w.UpdatedAt = time.Now(), time.Now()
	r.s.ws[w.ID] = w
	return w, nil
}
func (r wsRepo) FindByID(_ context.Context, id string) (workspace.Workspace, error) {
	w, ok := r.s.ws[id]
	if !ok {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}
	return w, nil
}
func (r wsRepo) FindBySlug(_ context.Context, slug string) (workspace.Workspace, error) {
	for _, w := range r.s.ws {
		if w.Slug == slug {
			return w, nil
		}
	}
	return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
}
func (r wsRepo) ListByUserID(_ context.Context, userID string) ([]workspace.Workspace, error) {
	var out []workspace.Workspace
	for _, m := range r.s.mem {
		if m.UserID == userID {
			out = append(out, r.s.ws[m.WorkspaceID])
		}
	}
	return out, nil
}
func (r wsRepo) Update(_ context.Context, id, name, slug string) (workspace.Workspace, error) {
	w := r.s.ws[id]
	w.Name, w.Slug, w.UpdatedAt = name, slug, time.Now()
	r.s.ws[id] = w
	return w, nil
}
func (r wsRepo) UpdateOwner(_ context.Context, id, newOwnerID string) error {
	w := r.s.ws[id]
	w.OwnerID = newOwnerID
	r.s.ws[id] = w
	return nil
}
func (r wsRepo) Delete(_ context.Context, id string) error { delete(r.s.ws, id); return nil }

type roleRepo struct{ s *state }

func (r roleRepo) Create(_ context.Context, role workspace.Role) (workspace.Role, error) {
	role.ID = r.s.id()
	role.CreatedAt = time.Now()
	r.s.roles[role.ID] = role
	return role, nil
}
func (r roleRepo) FindByID(_ context.Context, id string) (workspace.Role, error) {
	role, ok := r.s.roles[id]
	if !ok {
		return workspace.Role{}, workspace.ErrRoleNotFound
	}
	return role, nil
}
func (r roleRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]workspace.Role, error) {
	var out []workspace.Role
	for _, role := range r.s.roles {
		if role.WorkspaceID == workspaceID {
			out = append(out, role)
		}
	}
	return out, nil
}
func (r roleRepo) Update(_ context.Context, id, name string, perms workspace.Permission) (workspace.Role, error) {
	role := r.s.roles[id]
	role.Name, role.Permissions = name, perms
	r.s.roles[id] = role
	return role, nil
}
func (r roleRepo) Delete(_ context.Context, id string) error { delete(r.s.roles, id); return nil }
func (r roleRepo) CountMembersUsing(_ context.Context, roleID string) (int64, error) {
	var n int64
	for _, m := range r.s.mem {
		if m.RoleID == roleID {
			n++
		}
	}
	return n, nil
}

type memRepo struct{ s *state }

func (r memRepo) Add(_ context.Context, m workspace.Member) (workspace.Member, error) {
	m.JoinedAt = time.Now()
	r.s.mem[memKey(m.WorkspaceID, m.UserID)] = m
	return m, nil
}
func (r memRepo) Find(_ context.Context, workspaceID, userID string) (workspace.Member, error) {
	m, ok := r.s.mem[memKey(workspaceID, userID)]
	if !ok {
		return workspace.Member{}, workspace.ErrNotMember
	}
	return m, nil
}
func (r memRepo) LoadMembership(_ context.Context, workspaceID, userID string) (workspace.Membership, error) {
	m, ok := r.s.mem[memKey(workspaceID, userID)]
	if !ok {
		return workspace.Membership{}, workspace.ErrNotMember
	}
	return workspace.Membership{Member: m, Role: r.s.roles[m.RoleID]}, nil
}
func (r memRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]workspace.Member, error) {
	var out []workspace.Member
	for _, m := range r.s.mem {
		if m.WorkspaceID == workspaceID {
			out = append(out, m)
		}
	}
	return out, nil
}
func (r memRepo) UpdateRole(_ context.Context, workspaceID, userID, roleID string) error {
	k := memKey(workspaceID, userID)
	m := r.s.mem[k]
	m.RoleID = roleID
	r.s.mem[k] = m
	return nil
}
func (r memRepo) Remove(_ context.Context, workspaceID, userID string) error {
	delete(r.s.mem, memKey(workspaceID, userID))
	return nil
}
func (r memRepo) CountByWorkspace(_ context.Context, workspaceID string) (int64, error) {
	var n int64
	for _, m := range r.s.mem {
		if m.WorkspaceID == workspaceID {
			n++
		}
	}
	return n, nil
}

type einvRepo struct{ s *state }

func (r einvRepo) Create(_ context.Context, inv workspace.EmailInvitation) (workspace.EmailInvitation, error) {
	inv.ID = r.s.id()
	inv.CreatedAt = time.Now()
	r.s.einv[inv.ID] = inv
	return inv, nil
}
func (r einvRepo) FindByTokenHash(_ context.Context, hash string) (workspace.EmailInvitation, error) {
	for _, inv := range r.s.einv {
		if inv.TokenHash == hash {
			return inv, nil
		}
	}
	return workspace.EmailInvitation{}, workspace.ErrInviteInvalid
}
func (r einvRepo) FindByID(_ context.Context, id string) (workspace.EmailInvitation, error) {
	inv, ok := r.s.einv[id]
	if !ok {
		return workspace.EmailInvitation{}, workspace.ErrInviteInvalid
	}
	return inv, nil
}
func (r einvRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]workspace.EmailInvitation, error) {
	var out []workspace.EmailInvitation
	for _, inv := range r.s.einv {
		if inv.WorkspaceID == workspaceID {
			out = append(out, inv)
		}
	}
	return out, nil
}
func (r einvRepo) Delete(_ context.Context, id string) error { delete(r.s.einv, id); return nil }
func (r einvRepo) DeleteByWorkspaceEmail(_ context.Context, workspaceID, email string) error {
	for id, inv := range r.s.einv {
		if inv.WorkspaceID == workspaceID && inv.Email == email {
			delete(r.s.einv, id)
		}
	}
	return nil
}

type linkRepo struct{ s *state }

func (r linkRepo) Create(_ context.Context, l workspace.InviteLink) (workspace.InviteLink, error) {
	l.ID = r.s.id()
	l.CreatedAt = time.Now()
	r.s.links[l.ID] = l
	return l, nil
}
func (r linkRepo) FindByCode(_ context.Context, code string) (workspace.InviteLink, error) {
	for _, l := range r.s.links {
		if l.Code == code {
			return l, nil
		}
	}
	return workspace.InviteLink{}, workspace.ErrLinkInvalid
}
func (r linkRepo) FindByID(_ context.Context, id string) (workspace.InviteLink, error) {
	l, ok := r.s.links[id]
	if !ok {
		return workspace.InviteLink{}, workspace.ErrLinkInvalid
	}
	return l, nil
}
func (r linkRepo) ListByWorkspace(_ context.Context, workspaceID string) ([]workspace.InviteLink, error) {
	var out []workspace.InviteLink
	for _, l := range r.s.links {
		if l.WorkspaceID == workspaceID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (r linkRepo) IncrementUse(_ context.Context, id string) error {
	l := r.s.links[id]
	l.UseCount++
	r.s.links[id] = l
	return nil
}
func (r linkRepo) Revoke(_ context.Context, id string) error {
	l := r.s.links[id]
	l.Revoked = true
	r.s.links[id] = l
	return nil
}

type userLookup struct{ s *state }

func (u userLookup) FindByID(_ context.Context, id string) (auth.User, error) {
	usr, ok := u.s.users[id]
	if !ok {
		return auth.User{}, auth.ErrNotFound
	}
	return usr, nil
}
func (u userLookup) FindByEmail(_ context.Context, email string) (auth.User, error) {
	for _, usr := range u.s.users {
		if usr.Email == email {
			return usr, nil
		}
	}
	return auth.User{}, auth.ErrNotFound
}

type fakeEmail struct {
	sent     []string
	lastLink string
}

func (e *fakeEmail) SendWorkspaceInvite(_ context.Context, to, _, _, link string) error {
	e.sent = append(e.sent, to)
	e.lastLink = link
	return nil
}

type fakeUow struct{ s *state }

func (u fakeUow) Do(_ context.Context, fn func(workspace.Repos) error) error {
	return fn(workspace.Repos{
		Workspaces: wsRepo{u.s},
		Roles:      roleRepo{u.s},
		Members:    memRepo{u.s},
		EmailInvs:  einvRepo{u.s},
		Links:      linkRepo{u.s},
	})
}

// ── helpers ──────────────────────────────────────────────────────────────────

func newService(inviteExpiry time.Duration) (*workspace.Service, *state, *fakeEmail) {
	s := newState()
	em := &fakeEmail{}
	svc := workspace.NewService(
		wsRepo{s}, roleRepo{s}, memRepo{s}, einvRepo{s}, linkRepo{s},
		userLookup{s}, em, fakeUow{s},
		workspace.Config{AppBaseURL: "https://app.test", InviteExpiry: inviteExpiry},
	)
	return svc, s, em
}

// rolesOf returns the seeded admin (non-default) and member (default) roles.
func rolesOf(s *state, wsID string) (admin, member workspace.Role) {
	for _, r := range s.roles {
		if r.WorkspaceID != wsID {
			continue
		}
		if r.IsDefault {
			member = r
		} else {
			admin = r
		}
	}
	return
}

func tokenFromLink(link string) string {
	_, tok, _ := strings.Cut(link, "token=")
	return tok
}

// ── tests ────────────────────────────────────────────────────────────────────

func TestCreateWorkspace_SeedsRolesAndOwnerMember(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	s.addUser("owner", "owner@test", "Owner")

	ws, err := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if ws.OwnerID != "owner" {
		t.Errorf("owner_id = %q, want owner", ws.OwnerID)
	}
	admin, member := rolesOf(s, ws.ID)
	if admin.ID == "" || member.ID == "" {
		t.Fatalf("expected seeded Admin + Member roles, got admin=%q member=%q", admin.Name, member.Name)
	}
	if !member.IsDefault {
		t.Errorf("Member role should be default")
	}
	if admin.Permissions != workspace.PermViewWorkspace|workspace.PermManageWorkspace|workspace.PermManageMembers|
		workspace.PermManageRoles|workspace.PermCreateInvite|workspace.PermManageInvites|workspace.PermAdministrator {
		t.Errorf("Admin role missing permissions: %b", admin.Permissions)
	}
	m, err := memRepo{s}.Find(context.Background(), ws.ID, "owner")
	if err != nil || m.RoleID != admin.ID {
		t.Errorf("owner should be a member with the Admin role, got %+v err=%v", m, err)
	}
}

func TestCreateWorkspace_DerivesSlug(t *testing.T) {
	svc, _, _ := newService(time.Hour)
	ws, err := svc.CreateWorkspace(context.Background(), "owner", "My Cool Team!", "")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if ws.Slug != "my-cool-team" {
		t.Errorf("slug = %q, want my-cool-team", ws.Slug)
	}
}

func TestCreateWorkspace_SlugTaken(t *testing.T) {
	svc, _, _ := newService(time.Hour)
	if _, err := svc.CreateWorkspace(context.Background(), "owner", "Acme", "acme"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := svc.CreateWorkspace(context.Background(), "owner2", "Acme Two", "acme"); err != workspace.ErrSlugTaken {
		t.Errorf("err = %v, want ErrSlugTaken", err)
	}
}

func TestAuthorize_OwnerBypassesBitCheck(t *testing.T) {
	svc, _, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	// Owner can manage the workspace even though bit checks are irrelevant to them.
	if _, err := svc.UpdateWorkspace(context.Background(), "owner", ws.ID, "Renamed", ""); err != nil {
		t.Errorf("owner UpdateWorkspace: %v", err)
	}
}

func TestAuthorize_MissingBitForbidden(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	_, member := rolesOf(s, ws.ID)
	// Bob joins with the default (VIEW-only) role.
	memRepo{s}.Add(context.Background(), workspace.Member{WorkspaceID: ws.ID, UserID: "bob", RoleID: member.ID})

	if _, err := svc.CreateRole(context.Background(), "bob", ws.ID, "Hacker", workspace.PermViewWorkspace); err != workspace.ErrForbidden {
		t.Errorf("err = %v, want ErrForbidden", err)
	}
}

func TestAuthorize_AdministratorBypass(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	adminRole, _ := rolesOf(s, ws.ID)
	// Bob gets the Admin role (has ADMINISTRATOR) — bypasses the ManageRoles check.
	memRepo{s}.Add(context.Background(), workspace.Member{WorkspaceID: ws.ID, UserID: "bob", RoleID: adminRole.ID})

	if _, err := svc.CreateRole(context.Background(), "bob", ws.ID, "Editor", workspace.PermViewWorkspace); err != nil {
		t.Errorf("administrator CreateRole: %v", err)
	}
}

func TestAuthorize_NotMember(t *testing.T) {
	svc, _, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	if _, _, err := svc.GetWorkspace(context.Background(), "stranger", ws.ID); err != workspace.ErrNotMember {
		t.Errorf("err = %v, want ErrNotMember", err)
	}
}

func TestChangeMemberRole_OneRolePerMember(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	admin, member := rolesOf(s, ws.ID)
	memRepo{s}.Add(context.Background(), workspace.Member{WorkspaceID: ws.ID, UserID: "bob", RoleID: member.ID})
	s.addUser("bob", "bob@test", "Bob")

	mv, err := svc.ChangeMemberRole(context.Background(), "owner", ws.ID, "bob", admin.ID)
	if err != nil {
		t.Fatalf("ChangeMemberRole: %v", err)
	}
	if mv.Role.ID != admin.ID {
		t.Errorf("view role = %q, want admin", mv.Role.ID)
	}
	// Exactly one membership row, now pointing at the admin role.
	got, _ := memRepo{s}.Find(context.Background(), ws.ID, "bob")
	if got.RoleID != admin.ID {
		t.Errorf("stored role = %q, want admin (role replaced, not appended)", got.RoleID)
	}
}

func TestChangeMemberRole_PrivilegeEscalationBlocked(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	_, member := rolesOf(s, ws.ID)
	// Manager has ManageMembers + View but NOT ManageRoles.
	mgrRole, _ := roleRepo{s}.Create(context.Background(), workspace.Role{
		WorkspaceID: ws.ID, Name: "Manager",
		Permissions: workspace.PermViewWorkspace | workspace.PermManageMembers,
	})
	// A powerful role that carries a bit the manager lacks.
	powerful, _ := roleRepo{s}.Create(context.Background(), workspace.Role{
		WorkspaceID: ws.ID, Name: "Power",
		Permissions: workspace.PermViewWorkspace | workspace.PermManageRoles,
	})
	memRepo{s}.Add(context.Background(), workspace.Member{WorkspaceID: ws.ID, UserID: "mgr", RoleID: mgrRole.ID})
	memRepo{s}.Add(context.Background(), workspace.Member{WorkspaceID: ws.ID, UserID: "bob", RoleID: member.ID})
	s.addUser("bob", "bob@test", "Bob")

	if _, err := svc.ChangeMemberRole(context.Background(), "mgr", ws.ID, "bob", powerful.ID); err != workspace.ErrPrivilegeEscalation {
		t.Errorf("err = %v, want ErrPrivilegeEscalation", err)
	}
}

func TestRemoveMember_CannotRemoveOwner(t *testing.T) {
	svc, _, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	if err := svc.RemoveMember(context.Background(), "owner", ws.ID, "owner"); err != workspace.ErrLastOwner {
		t.Errorf("err = %v, want ErrLastOwner", err)
	}
}

func TestLeaveWorkspace_OwnerBlocked(t *testing.T) {
	svc, _, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	if err := svc.LeaveWorkspace(context.Background(), "owner", ws.ID); err != workspace.ErrLastOwner {
		t.Errorf("err = %v, want ErrLastOwner", err)
	}
}

func TestDeleteRole_DefaultAndInUse(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	admin, member := rolesOf(s, ws.ID)

	if err := svc.DeleteRole(context.Background(), "owner", ws.ID, member.ID); err != workspace.ErrDefaultRole {
		t.Errorf("delete default: err = %v, want ErrDefaultRole", err)
	}
	// Admin role is assigned to the owner member -> in use.
	if err := svc.DeleteRole(context.Background(), "owner", ws.ID, admin.ID); err != workspace.ErrRoleInUse {
		t.Errorf("delete in-use: err = %v, want ErrRoleInUse", err)
	}
}

func TestInviteByEmail_SendsEmailAndAccept(t *testing.T) {
	svc, s, em := newService(time.Hour)
	s.addUser("owner", "owner@test", "Owner")
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	_, member := rolesOf(s, ws.ID)

	if _, err := svc.InviteByEmail(context.Background(), "owner", ws.ID, "new@test", member.ID, "en-ie"); err != nil {
		t.Fatalf("InviteByEmail: %v", err)
	}
	if len(em.sent) != 1 || em.sent[0] != "new@test" {
		t.Fatalf("expected one invite email to new@test, got %v", em.sent)
	}
	token := tokenFromLink(em.lastLink)
	if token == "" {
		t.Fatalf("no token in link %q", em.lastLink)
	}
	// A signed-up user accepts.
	got, err := svc.AcceptEmailInvite(context.Background(), "newuser", token)
	if err != nil {
		t.Fatalf("AcceptEmailInvite: %v", err)
	}
	if got.ID != ws.ID {
		t.Errorf("accepted into %q, want %q", got.ID, ws.ID)
	}
	if _, err := (memRepo{s}).Find(context.Background(), ws.ID, "newuser"); err != nil {
		t.Errorf("new user should be a member: %v", err)
	}
}

func TestInviteByEmail_AlreadyMember(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	s.addUser("owner", "owner@test", "Owner")
	_, member := rolesOf(s, ws.ID)
	if _, err := svc.InviteByEmail(context.Background(), "owner", ws.ID, "owner@test", member.ID, "en-ie"); err != workspace.ErrAlreadyMember {
		t.Errorf("err = %v, want ErrAlreadyMember", err)
	}
}

func TestAcceptEmailInvite_Expired(t *testing.T) {
	svc, s, em := newService(-time.Hour) // invites are born expired
	s.addUser("owner", "owner@test", "Owner")
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	_, member := rolesOf(s, ws.ID)
	svc.InviteByEmail(context.Background(), "owner", ws.ID, "new@test", member.ID, "en-ie")
	token := tokenFromLink(em.lastLink)
	if _, err := svc.AcceptEmailInvite(context.Background(), "newuser", token); err != workspace.ErrInviteExpired {
		t.Errorf("err = %v, want ErrInviteExpired", err)
	}
}

func TestInviteLink_MaxUsesExhausted(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	_, member := rolesOf(s, ws.ID)
	link, err := svc.CreateInviteLink(context.Background(), "owner", ws.ID, member.ID, 1, nil)
	if err != nil {
		t.Fatalf("CreateInviteLink: %v", err)
	}
	if _, err := svc.JoinViaInviteLink(context.Background(), "bob", link.Code); err != nil {
		t.Fatalf("first join: %v", err)
	}
	if _, err := svc.JoinViaInviteLink(context.Background(), "carol", link.Code); err != workspace.ErrLinkExhausted {
		t.Errorf("second join err = %v, want ErrLinkExhausted", err)
	}
}

func TestInviteLink_Revoked(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	_, member := rolesOf(s, ws.ID)
	link, _ := svc.CreateInviteLink(context.Background(), "owner", ws.ID, member.ID, 0, nil)
	if err := svc.RevokeInviteLink(context.Background(), "owner", ws.ID, link.ID); err != nil {
		t.Fatalf("RevokeInviteLink: %v", err)
	}
	if _, err := svc.JoinViaInviteLink(context.Background(), "bob", link.Code); err != workspace.ErrLinkInvalid {
		t.Errorf("err = %v, want ErrLinkInvalid", err)
	}
}

func TestJoinViaInviteLink_AlreadyMemberNoIncrement(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	_, member := rolesOf(s, ws.ID)
	link, _ := svc.CreateInviteLink(context.Background(), "owner", ws.ID, member.ID, 5, nil)
	// The owner is already a member; joining must not consume a use.
	if _, err := svc.JoinViaInviteLink(context.Background(), "owner", link.Code); err != nil {
		t.Fatalf("owner join: %v", err)
	}
	got, _ := linkRepo{s}.FindByID(context.Background(), link.ID)
	if got.UseCount != 0 {
		t.Errorf("use_count = %d, want 0 (already a member)", got.UseCount)
	}
}

func TestTransferOwnership_RequiresMember(t *testing.T) {
	svc, s, _ := newService(time.Hour)
	ws, _ := svc.CreateWorkspace(context.Background(), "owner", "Acme", "")
	if err := svc.TransferOwnership(context.Background(), "owner", ws.ID, "stranger"); err != workspace.ErrNotMember {
		t.Fatalf("transfer to non-member err = %v, want ErrNotMember", err)
	}
	_, member := rolesOf(s, ws.ID)
	memRepo{s}.Add(context.Background(), workspace.Member{WorkspaceID: ws.ID, UserID: "bob", RoleID: member.ID})
	if err := svc.TransferOwnership(context.Background(), "owner", ws.ID, "bob"); err != nil {
		t.Fatalf("transfer to member: %v", err)
	}
	got, _ := wsRepo{s}.FindByID(context.Background(), ws.ID)
	if got.OwnerID != "bob" {
		t.Errorf("owner = %q, want bob", got.OwnerID)
	}
}
