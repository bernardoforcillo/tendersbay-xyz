package workbench_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/bernardoforcillo/authlayer/access"
	"github.com/bernardoforcillo/authlayer/scope"
	memstore "github.com/bernardoforcillo/authlayer/store/memory"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

const workspaceID = "ws-1"

// ── the parent scope ────────────────────────────────────────────────────────

// parentStatements is the slice of the WORKSPACE's permission surface this
// scope depends on. It is declared here rather than imported so the test states
// the contract explicitly: the parent must declare "workbench" with these
// actions, or the nesting silently resolves to "denied". The real declaration
// lives in workspace.Statements, and TestWorkbenchNestingMatchesWorkspace in
// the main package holds the two together.
func parentStatements() map[string][]access.Action {
	return map[string][]access.Action{
		workbench.ResourceWorkbench: {scope.ActionRead, scope.ActionCreate, workbench.ActionManage},
	}
}

// fakeParent is a scope.ParentScope: it answers what a user's standing in the
// workspace is, which is everything the nesting consults.
type fakeParent struct {
	ac       *access.Access
	standing map[string]access.Permission
	elevated map[string]bool
}

func newFakeParent() *fakeParent {
	return &fakeParent{
		ac:       access.New(access.NewStatements(parentStatements())),
		standing: map[string]access.Permission{},
		elevated: map[string]bool{},
	}
}

// grant gives userID the named workspace-level actions on workbenches.
func (f *fakeParent) grant(t *testing.T, userID string, actions ...access.Action) {
	t.Helper()
	p, err := f.ac.Permission(map[string][]access.Action{workbench.ResourceWorkbench: actions})
	if err != nil {
		t.Fatalf("parent permission: %v", err)
	}
	f.standing[userID] = p
}

func (f *fakeParent) Standing(_ context.Context, containerID, userID string) (access.Permission, bool, error) {
	if containerID != workspaceID {
		return access.Permission{}, false, scope.ErrContainerNotFound
	}
	p, ok := f.standing[userID]
	if !ok && !f.elevated[userID] {
		return access.Permission{}, false, scope.ErrNotMember
	}
	return p, f.elevated[userID], nil
}

// ── the store ───────────────────────────────────────────────────────────────

// testStore is authlayer's in-memory scope store, indexed so it can also serve
// as workbench.Repository — in production both read and write the same
// workbenches row.
type testStore struct {
	workbench.Store

	mu   sync.Mutex
	byID map[string]workbench.Workbench
}

func newTestStore() *testStore {
	return &testStore{
		Store: memstore.New[workbench.Workbench, workbench.Member](),
		byID:  map[string]workbench.Workbench{},
	}
}

func (s *testStore) CreateContainer(ctx context.Context, w workbench.Workbench) (workbench.Workbench, error) {
	created, err := s.Store.CreateContainer(ctx, w)
	if err != nil {
		return workbench.Workbench{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[created.ID] = created
	return created, nil
}

// WithTx hands the wrapper to the closure so CreateContainer's own indexing
// runs — authlayer creates the container and seeds the owner's membership in
// one transaction, and the memory store would otherwise pass itself.
func (s *testStore) WithTx(_ context.Context, fn func(workbench.Store) error) error {
	return fn(s)
}

func (s *testStore) ListByWorkspace(_ context.Context, wsID string) ([]workbench.Workbench, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []workbench.Workbench
	for _, wb := range s.byID {
		if wb.WorkspaceID == wsID {
			out = append(out, wb)
		}
	}
	return out, nil
}

func (s *testStore) UpdateDetails(_ context.Context, id, name, description string) (workbench.Workbench, error) {
	return s.mutate(id, func(wb *workbench.Workbench) { wb.Name, wb.Description = name, description })
}

func (s *testStore) UpdateVisibility(_ context.Context, id string, v workbench.Visibility) (workbench.Workbench, error) {
	return s.mutate(id, func(wb *workbench.Workbench) { wb.Visibility = v })
}

func (s *testStore) mutate(id string, fn func(*workbench.Workbench)) (workbench.Workbench, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	wb, ok := s.byID[id]
	if !ok {
		return workbench.Workbench{}, workbench.ErrWorkbenchNotFound
	}
	fn(&wb)
	s.byID[id] = wb
	return wb, nil
}

func (s *testStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.byID, id)
	return nil
}

// ── the workspace bridge ────────────────────────────────────────────────────

type fakeWorkspace struct {
	infos map[string]workbench.WorkspaceInfo
}

func (f *fakeWorkspace) Lookup(_ context.Context, wsID, userID string) (workbench.WorkspaceInfo, error) {
	info, ok := f.infos[wsID+"|"+userID]
	if !ok {
		return workbench.WorkspaceInfo{Name: "Acme"}, nil
	}
	info.Name = "Acme"
	return info, nil
}

type fakeUsers struct{}

func (fakeUsers) FindByID(_ context.Context, id string) (auth.User, error) {
	return auth.User{ID: id, Email: id + "@example.com", DisplayName: strings.ToUpper(id)}, nil
}

// ── fixture ─────────────────────────────────────────────────────────────────

type fixture struct {
	svc    *workbench.Service
	store  *testStore
	parent *fakeParent
	ws     *fakeWorkspace
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	store := newTestStore()
	parent := newFakeParent()
	ws := &fakeWorkspace{infos: map[string]workbench.WorkspaceInfo{}}
	svc := workbench.NewService(workbench.NewAccess(), parent, store, store, fakeUsers{}, ws)
	return &fixture{svc: svc, store: store, parent: parent, ws: ws}
}

// member makes userID a workspace member with the given workbench-related
// workspace rights, on both halves of the bridge: the parent scope (which the
// nesting consults) and the WorkspaceInfo lookup (which the shared-visibility
// rule consults).
func (f *fixture) member(t *testing.T, userID string, info workbench.WorkspaceInfo, actions ...access.Action) {
	t.Helper()
	info.IsMember = true
	f.ws.infos[workspaceID+"|"+userID] = info
	f.parent.grant(t, userID, actions...)
}

func (f *fixture) create(t *testing.T, userID, name string, v workbench.Visibility) workbench.Workbench {
	t.Helper()
	wb, err := f.svc.CreateWorkbench(context.Background(), userID, workspaceID, name, "", v)
	if err != nil {
		t.Fatalf("CreateWorkbench: %v", err)
	}
	return wb
}

// ── creation is a permission check against the workspace ────────────────────

func TestCreateWorkbench_RequiresTheWorkspaceGrant(t *testing.T) {
	f := newFixture(t)
	f.member(t, "bob", workbench.WorkspaceInfo{}, scope.ActionRead)

	_, err := f.svc.CreateWorkbench(context.Background(), "bob", workspaceID, "Bid A", "", workbench.VisibilityPrivate)
	if !errors.Is(err, workbench.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}

	f.member(t, "carol", workbench.WorkspaceInfo{}, scope.ActionRead, scope.ActionCreate)
	if _, err := f.svc.CreateWorkbench(context.Background(), "carol", workspaceID, "Bid A", "", workbench.VisibilityPrivate); err != nil {
		t.Fatalf("CreateWorkbench: %v", err)
	}
}

func TestCreateWorkbench_NonWorkspaceMemberRefused(t *testing.T) {
	f := newFixture(t)
	_, err := f.svc.CreateWorkbench(context.Background(), "stranger", workspaceID, "Bid A", "", workbench.VisibilityPrivate)
	if !errors.Is(err, workbench.ErrNotMember) {
		t.Fatalf("err = %v, want ErrNotMember", err)
	}
}

func TestCreateWorkbench_CreatorIsOwnerAndElevated(t *testing.T) {
	f := newFixture(t)
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	if wb.OwnerID != "owner" || wb.WorkspaceID != workspaceID {
		t.Fatalf("workbench = %+v", wb)
	}
	got, perms, wsName, err := f.svc.GetWorkbench(context.Background(), "owner", wb.ID)
	if err != nil {
		t.Fatalf("GetWorkbench: %v", err)
	}
	if got.Name != "Bid A" || wsName != "Acme" {
		t.Fatalf("got %+v / %q", got, wsName)
	}
	if !perms.Has(workbench.PermAdministrator) || !perms.Has(workbench.PermManageMembers) {
		t.Fatalf("owner permissions = %d", perms)
	}
}

// ── the inheritance ─────────────────────────────────────────────────────────

// A workspace administrator administers every workbench in it, with no
// membership of their own. This is the rule that used to be a bitmask copied
// across a bridge and is now scope.InheritWhen.
func TestWorkspaceManagerIsElevatedInEveryWorkbench(t *testing.T) {
	f := newFixture(t)
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	f.member(t, "boss", workbench.WorkspaceInfo{MayManageAll: true}, workbench.ActionManage)

	_, perms, _, err := f.svc.GetWorkbench(context.Background(), "boss", wb.ID)
	if err != nil {
		t.Fatalf("GetWorkbench: %v", err)
	}
	if !perms.Has(workbench.PermAdministrator) {
		t.Fatalf("permissions = %d, want administrator", perms)
	}
	// An elevated caller must be reported as holding every bit, or the client
	// hides actions the server would in fact allow.
	if !perms.Has(workbench.PermManageWorkbench) || !perms.Has(workbench.PermManageRoles) {
		t.Fatalf("permissions = %d, want the full mask for an elevated caller", perms)
	}
	if err := f.svc.CanManageWorkbench(context.Background(), "boss", wb.ID); err != nil {
		t.Fatalf("CanManageWorkbench: %v", err)
	}
}

// ── visibility ──────────────────────────────────────────────────────────────

func TestSharedWorkbenchIsVisibleToWorkspaceViewers(t *testing.T) {
	f := newFixture(t)
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	shared := f.create(t, "owner", "Shared", workbench.VisibilityShared)
	private := f.create(t, "owner", "Private", workbench.VisibilityPrivate)

	f.member(t, "viewer", workbench.WorkspaceInfo{MayViewShared: true}, scope.ActionRead)
	ctx := context.Background()

	if err := f.svc.CanAccessWorkbench(ctx, "viewer", shared.ID); err != nil {
		t.Fatalf("a shared workbench must be visible: %v", err)
	}
	// A viewer may look, not manage.
	if err := f.svc.CanManageWorkbench(ctx, "viewer", shared.ID); !errors.Is(err, workbench.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
	// A private one they are not a member of must not even be admitted to exist.
	if err := f.svc.CanAccessWorkbench(ctx, "viewer", private.ID); !errors.Is(err, workbench.ErrWorkbenchNotFound) {
		t.Fatalf("err = %v, want ErrWorkbenchNotFound", err)
	}
}

// Whether a workbench exists is itself something a non-member must not learn.
func TestNonWorkspaceMemberSeesNothing(t *testing.T) {
	f := newFixture(t)
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Shared", workbench.VisibilityShared)

	err := f.svc.CanAccessWorkbench(context.Background(), "stranger", wb.ID)
	if !errors.Is(err, workbench.ErrWorkbenchNotFound) {
		t.Fatalf("err = %v, want ErrWorkbenchNotFound", err)
	}
}

func TestListWorkbenches_AppliesTheSameLadder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	shared := f.create(t, "owner", "Shared", workbench.VisibilityShared)
	private := f.create(t, "owner", "Private", workbench.VisibilityPrivate)

	f.member(t, "viewer", workbench.WorkspaceInfo{MayViewShared: true}, scope.ActionRead)
	f.member(t, "boss", workbench.WorkspaceInfo{MayManageAll: true, MayViewShared: true}, workbench.ActionManage)
	f.member(t, "plain", workbench.WorkspaceInfo{}, scope.ActionRead)

	for _, tc := range []struct {
		user string
		want []string
	}{
		{"owner", []string{shared.ID, private.ID}},
		{"boss", []string{shared.ID, private.ID}},
		{"viewer", []string{shared.ID}},
		{"plain", nil},
	} {
		got, err := f.svc.ListWorkbenches(ctx, tc.user, workspaceID)
		if err != nil {
			t.Fatalf("ListWorkbenches(%s): %v", tc.user, err)
		}
		seen := map[string]bool{}
		for _, wb := range got {
			seen[wb.ID] = true
		}
		if len(seen) != len(tc.want) {
			t.Fatalf("%s sees %d workbenches, want %d", tc.user, len(seen), len(tc.want))
		}
		for _, id := range tc.want {
			if !seen[id] {
				t.Fatalf("%s cannot see workbench %s", tc.user, id)
			}
		}
	}
}

func TestListWorkbenches_NonWorkspaceMember(t *testing.T) {
	f := newFixture(t)
	f.ws.infos[workspaceID+"|stranger"] = workbench.WorkspaceInfo{IsMember: false}
	if _, err := f.svc.ListWorkbenches(context.Background(), "stranger", workspaceID); !errors.Is(err, workbench.ErrNotMember) {
		t.Fatalf("err = %v, want ErrNotMember", err)
	}
}

// ── members and roles ───────────────────────────────────────────────────────

func TestAddMember_MustBeAWorkspaceMember(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	if _, err := f.svc.AddMember(ctx, "owner", wb.ID, "stranger", workbench.RoleViewer); !errors.Is(err, workbench.ErrNotWorkspaceMember) {
		t.Fatalf("err = %v, want ErrNotWorkspaceMember", err)
	}

	f.member(t, "bob", workbench.WorkspaceInfo{}, scope.ActionRead)
	view, err := f.svc.AddMember(ctx, "owner", wb.ID, "bob", workbench.RoleViewer)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if view.Member.RoleKey != workbench.RoleViewer || view.Role.Name != "Viewer" {
		t.Fatalf("view = %+v", view)
	}
	if err := f.svc.CanAccessWorkbench(ctx, "bob", wb.ID); err != nil {
		t.Fatalf("the new member cannot see the workbench: %v", err)
	}
}

func TestPrivilegeEscalationBlocked(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	f.member(t, "mod", workbench.WorkspaceInfo{}, scope.ActionRead)
	f.member(t, "bob", workbench.WorkspaceInfo{}, scope.ActionRead)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	modRole, err := f.svc.CreateRole(ctx, "owner", wb.ID, "Moderator", workbench.PermManageMembers)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if _, err := f.svc.AddMember(ctx, "owner", wb.ID, "mod", modRole.ID); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if _, err := f.svc.AddMember(ctx, "owner", wb.ID, "bob", workbench.RoleViewer); err != nil {
		t.Fatalf("AddMember: %v", err)
	}

	// mod may manage members but does not hold manage-roles, so it may not
	// hand out the manager role.
	if _, err := f.svc.ChangeMemberRole(ctx, "mod", wb.ID, "bob", workbench.RoleManager); !errors.Is(err, workbench.ErrPrivilegeEscalation) {
		t.Fatalf("err = %v, want ErrPrivilegeEscalation", err)
	}
}

func TestOwnerIsProtected(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	f.member(t, "boss", workbench.WorkspaceInfo{MayManageAll: true}, workbench.ActionManage)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	if err := f.svc.LeaveWorkbench(ctx, "owner", wb.ID); !errors.Is(err, workbench.ErrLastOwner) {
		t.Fatalf("LeaveWorkbench err = %v, want ErrLastOwner", err)
	}
	if err := f.svc.RemoveMember(ctx, "boss", wb.ID, "owner"); !errors.Is(err, workbench.ErrLastOwner) {
		t.Fatalf("RemoveMember err = %v, want ErrLastOwner", err)
	}
}

func TestListRoles_IncludesTheCodeDefinedOnes(t *testing.T) {
	f := newFixture(t)
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	roles, err := f.svc.ListRoles(context.Background(), "owner", wb.ID)
	if err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	byKey := map[string]workbench.Role{}
	for _, r := range roles {
		byKey[r.ID] = r
	}
	for key, name := range map[string]string{
		workbench.RoleOwner: "Owner", workbench.RoleManager: "Manager", workbench.RoleViewer: "Viewer",
	} {
		r, ok := byKey[key]
		if !ok {
			t.Fatalf("role %q missing from %v", key, byKey)
		}
		if r.Name != name || !r.IsDefault {
			t.Fatalf("role %q = %+v, want the %q label and IsDefault", key, r, name)
		}
	}
	if !byKey[workbench.RoleManager].Permissions.Has(workbench.PermAdministrator) {
		t.Fatal("the manager role must be elevated — it is what the old Manager row was")
	}
	if byKey[workbench.RoleViewer].Permissions.Has(workbench.PermManageWorkbench) {
		t.Fatal("the viewer role must grant nothing but the right to look")
	}
}

// A shared workbench's viewers could always see its roles and members; access
// through the visibility column is not standing authlayer knows about, so this
// is the path that would silently regress.
func TestSharedViewerCanListRolesAndMembers(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Shared", workbench.VisibilityShared)
	f.member(t, "viewer", workbench.WorkspaceInfo{MayViewShared: true}, scope.ActionRead)

	if _, err := f.svc.ListRoles(ctx, "viewer", wb.ID); err != nil {
		t.Fatalf("ListRoles: %v", err)
	}
	members, err := f.svc.ListMembers(ctx, "viewer", wb.ID)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 || members[0].Member.UserID != "owner" {
		t.Fatalf("members = %+v", members)
	}
	if members[0].User.DisplayName == "" {
		t.Fatal("member has no profile joined in")
	}
}

func TestDeleteRole_DefaultRefusedAndInUseRefused(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	f.member(t, "bob", workbench.WorkspaceInfo{}, scope.ActionRead)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	if err := f.svc.DeleteRole(ctx, "owner", wb.ID, workbench.RoleViewer); !errors.Is(err, workbench.ErrDefaultRole) {
		t.Fatalf("err = %v, want ErrDefaultRole", err)
	}
	custom, err := f.svc.CreateRole(ctx, "owner", wb.ID, "Reviewer", workbench.PermManageWorkbench)
	if err != nil {
		t.Fatalf("CreateRole: %v", err)
	}
	if custom.ID != "reviewer" {
		t.Fatalf("role key = %q, want reviewer", custom.ID)
	}
	if _, err := f.svc.AddMember(ctx, "owner", wb.ID, "bob", custom.ID); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if err := f.svc.DeleteRole(ctx, "owner", wb.ID, custom.ID); !errors.Is(err, workbench.ErrRoleInUse) {
		t.Fatalf("err = %v, want ErrRoleInUse", err)
	}
}

// ── ownership and lifecycle ─────────────────────────────────────────────────

// The new owner has to belong to the WORKSPACE, not to the workbench — which is
// why this does not use scope's own TransferOwnership.
func TestTransferOwnership_ToAWorkspaceMember(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	if err := f.svc.TransferOwnership(ctx, "owner", wb.ID, "stranger"); !errors.Is(err, workbench.ErrNotWorkspaceMember) {
		t.Fatalf("err = %v, want ErrNotWorkspaceMember", err)
	}
	f.member(t, "bob", workbench.WorkspaceInfo{}, scope.ActionRead)
	if err := f.svc.TransferOwnership(ctx, "owner", wb.ID, "bob"); err != nil {
		t.Fatalf("TransferOwnership: %v", err)
	}
	if err := f.svc.CanManageWorkbench(ctx, "bob", wb.ID); err != nil {
		t.Fatalf("the new owner cannot manage: %v", err)
	}
}

func TestDeleteWorkbench_OwnerOrWorkspaceAdmin(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	f.member(t, "bob", workbench.WorkspaceInfo{}, scope.ActionRead)
	f.member(t, "boss", workbench.WorkspaceInfo{MayManageAll: true}, workbench.ActionManage)

	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)
	if _, err := f.svc.AddMember(ctx, "owner", wb.ID, "bob", workbench.RoleManager); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	// A manager is not the owner: deleting is owner-only, and it is not a grant.
	if err := f.svc.DeleteWorkbench(ctx, "bob", wb.ID); !errors.Is(err, workbench.ErrOwnerOnly) {
		t.Fatalf("err = %v, want ErrOwnerOnly", err)
	}
	if err := f.svc.DeleteWorkbench(ctx, "boss", wb.ID); err != nil {
		t.Fatalf("a workspace administrator may delete: %v", err)
	}
}

func TestUpdateAndChangeVisibility(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	got, err := f.svc.UpdateWorkbench(ctx, "owner", wb.ID, "Bid B", "notes")
	if err != nil {
		t.Fatalf("UpdateWorkbench: %v", err)
	}
	if got.Name != "Bid B" || got.Description != "notes" {
		t.Fatalf("got %+v", got)
	}
	got, err = f.svc.ChangeVisibility(ctx, "owner", wb.ID, workbench.VisibilityShared)
	if err != nil {
		t.Fatalf("ChangeVisibility: %v", err)
	}
	if got.Visibility != workbench.VisibilityShared {
		t.Fatalf("visibility = %q", got.Visibility)
	}
	if _, err := f.svc.ChangeVisibility(ctx, "owner", wb.ID, "nonsense"); !errors.Is(err, workbench.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

func TestWorkspaceOf(t *testing.T) {
	f := newFixture(t)
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	got, err := f.svc.WorkspaceOf(context.Background(), wb.ID)
	if err != nil {
		t.Fatalf("WorkspaceOf: %v", err)
	}
	if got != workspaceID {
		t.Fatalf("WorkspaceOf = %q, want %q", got, workspaceID)
	}
	if _, err := f.svc.WorkspaceOf(context.Background(), "nope"); !errors.Is(err, workbench.ErrWorkbenchNotFound) {
		t.Fatalf("err = %v, want ErrWorkbenchNotFound", err)
	}
}

// Every bit the client can send has to survive the round trip through
// authlayer's grant set, or the role editor silently drops capabilities.
func TestPermissionMaskRoundTrip(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.member(t, "owner", workbench.WorkspaceInfo{}, scope.ActionCreate)
	wb := f.create(t, "owner", "Bid A", workbench.VisibilityPrivate)

	for _, bit := range []workbench.Permission{
		workbench.PermManageWorkbench,
		workbench.PermManageMembers,
		workbench.PermManageRoles,
	} {
		role, err := f.svc.CreateRole(ctx, "owner", wb.ID, "Probe", bit)
		if err != nil {
			t.Fatalf("CreateRole(%d): %v", bit, err)
		}
		if !role.Permissions.Has(bit) {
			t.Fatalf("bit %d did not survive: got %d", bit, role.Permissions)
		}
		if role.Permissions.Has(workbench.PermAdministrator) {
			t.Fatalf("a single-capability role must not read as administrator: %d", role.Permissions)
		}
		if err := f.svc.DeleteRole(ctx, "owner", wb.ID, role.ID); err != nil {
			t.Fatalf("DeleteRole: %v", err)
		}
	}
}
