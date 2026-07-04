package workbench

import (
	"context"
	"errors"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

// ── fakes ────────────────────────────────────────────────────────────────────

type fakeWB struct {
	items map[string]Workbench
}

func (f *fakeWB) Create(_ context.Context, w Workbench) (Workbench, error) {
	if w.ID == "" {
		w.ID = "wb-" + w.Name
	}
	f.items[w.ID] = w
	return w, nil
}
func (f *fakeWB) FindByID(_ context.Context, id string) (Workbench, error) {
	w, ok := f.items[id]
	if !ok {
		return Workbench{}, ErrWorkbenchNotFound
	}
	return w, nil
}
func (f *fakeWB) ListByWorkspace(_ context.Context, wsID string) ([]Workbench, error) {
	var out []Workbench
	for _, w := range f.items {
		if w.WorkspaceID == wsID {
			out = append(out, w)
		}
	}
	return out, nil
}
func (f *fakeWB) Update(_ context.Context, id, name, desc string) (Workbench, error) {
	w := f.items[id]
	w.Name, w.Description = name, desc
	f.items[id] = w
	return w, nil
}
func (f *fakeWB) UpdateVisibility(_ context.Context, id string, v Visibility) (Workbench, error) {
	w := f.items[id]
	w.Visibility = v
	f.items[id] = w
	return w, nil
}
func (f *fakeWB) UpdateOwner(_ context.Context, id, newOwner string) error {
	w := f.items[id]
	w.OwnerID = newOwner
	f.items[id] = w
	return nil
}
func (f *fakeWB) Delete(_ context.Context, id string) error { delete(f.items, id); return nil }

type fakeRoles struct{ items map[string]Role }

func (f *fakeRoles) Create(_ context.Context, r Role) (Role, error) {
	if r.ID == "" {
		r.ID = "role-" + r.Name + "-" + r.WorkbenchID
	}
	f.items[r.ID] = r
	return r, nil
}
func (f *fakeRoles) FindByID(_ context.Context, id string) (Role, error) {
	r, ok := f.items[id]
	if !ok {
		return Role{}, ErrRoleNotFound
	}
	return r, nil
}
func (f *fakeRoles) ListByWorkbench(_ context.Context, wbID string) ([]Role, error) {
	var out []Role
	for _, r := range f.items {
		if r.WorkbenchID == wbID {
			out = append(out, r)
		}
	}
	return out, nil
}
func (f *fakeRoles) Update(_ context.Context, id, name string, p Permission) (Role, error) {
	r := f.items[id]
	r.Name, r.Permissions = name, p
	f.items[id] = r
	return r, nil
}
func (f *fakeRoles) Delete(_ context.Context, id string) error { delete(f.items, id); return nil }
func (f *fakeRoles) CountMembersUsing(_ context.Context, roleID string) (int64, error) {
	return 0, nil
}

type fakeMembers struct{ items map[string]Membership } // key: wbID+"|"+userID

func mkey(wb, u string) string { return wb + "|" + u }
func (f *fakeMembers) Add(_ context.Context, m Member) (Member, error) {
	if _, ok := f.items[mkey(m.WorkbenchID, m.UserID)]; ok {
		return Member{}, ErrAlreadyMember
	}
	f.items[mkey(m.WorkbenchID, m.UserID)] = Membership{Member: m}
	return m, nil
}
func (f *fakeMembers) Find(_ context.Context, wb, u string) (Member, error) {
	ms, ok := f.items[mkey(wb, u)]
	if !ok {
		return Member{}, ErrNotMember
	}
	return ms.Member, nil
}
func (f *fakeMembers) LoadMembership(_ context.Context, wb, u string) (Membership, error) {
	ms, ok := f.items[mkey(wb, u)]
	if !ok {
		return Membership{}, ErrNotMember
	}
	return ms, nil
}
func (f *fakeMembers) ListByWorkbench(_ context.Context, wb string) ([]Member, error) {
	var out []Member
	for _, ms := range f.items {
		if ms.Member.WorkbenchID == wb {
			out = append(out, ms.Member)
		}
	}
	return out, nil
}
func (f *fakeMembers) UpdateRole(_ context.Context, wb, u, roleID string) error {
	ms := f.items[mkey(wb, u)]
	ms.Member.RoleID = roleID
	f.items[mkey(wb, u)] = ms
	return nil
}
func (f *fakeMembers) Remove(_ context.Context, wb, u string) error {
	delete(f.items, mkey(wb, u))
	return nil
}
func (f *fakeMembers) CountByWorkbench(_ context.Context, wb string) (int64, error) {
	var n int64
	for _, ms := range f.items {
		if ms.Member.WorkbenchID == wb {
			n++
		}
	}
	return n, nil
}

type fakeUsers struct{}

func (fakeUsers) FindByID(_ context.Context, id string) (auth.User, error) {
	return auth.User{ID: id, Email: id + "@x.io"}, nil
}

type fakeWSAccess struct{ infos map[string]WorkspaceInfo } // key: wsID+"|"+userID

func (f *fakeWSAccess) Lookup(_ context.Context, wsID, userID string) (WorkspaceInfo, error) {
	if info, ok := f.infos[wsID+"|"+userID]; ok {
		return info, nil
	}
	return WorkspaceInfo{Name: "WS " + wsID}, nil // non-member by default
}

type fakeUoW struct{ r Repos }

func (f fakeUoW) Do(ctx context.Context, fn func(Repos) error) error { return fn(f.r) }

// newTestService wires a Service over fresh fakes and returns both.
type testFakes struct {
	wb    *fakeWB
	roles *fakeRoles
	mem   *fakeMembers
	wsa   *fakeWSAccess
}

func newTestService() (*Service, *testFakes) {
	wb := &fakeWB{items: map[string]Workbench{}}
	roles := &fakeRoles{items: map[string]Role{}}
	mem := &fakeMembers{items: map[string]Membership{}}
	wsa := &fakeWSAccess{infos: map[string]WorkspaceInfo{}}
	uow := fakeUoW{r: Repos{Workbenches: wb, Roles: roles, Members: mem}}
	svc := NewService(wb, roles, mem, fakeUsers{}, wsa, uow)
	return svc, &testFakes{wb: wb, roles: roles, mem: mem, wsa: wsa}
}

// ── authorize table test ──────────────────────────────────────────────────────

func TestAuthorize(t *testing.T) {
	const wsID, wbID = "ws1", "wb1"
	base := Workbench{ID: wbID, WorkspaceID: wsID, OwnerID: "owner", Visibility: VisibilityPrivate}

	cases := []struct {
		name       string
		vis        Visibility
		user       string
		wsInfo     WorkspaceInfo
		wbRolePerm *Permission // explicit workbench membership role perms, nil = none
		need       Permission
		wantErr    error
	}{
		{name: "workbench owner bypasses", user: "owner", wsInfo: WorkspaceInfo{IsMember: true}, need: PermManageRoles},
		{name: "workspace owner override", user: "u2", wsInfo: WorkspaceInfo{IsMember: true, IsOwner: true}, need: PermManageRoles},
		{name: "workspace admin override", user: "u2", wsInfo: WorkspaceInfo{IsMember: true, Perms: wsPermAdministrator}, need: PermManageRoles},
		{name: "manage-workbenches override", user: "u2", wsInfo: WorkspaceInfo{IsMember: true, Perms: wsPermManageWorkbenches}, need: PermDeleteSentinel()},
		{name: "non-workspace-member hidden", user: "stranger", wsInfo: WorkspaceInfo{IsMember: false}, need: PermViewWorkbench, wantErr: ErrWorkbenchNotFound},
		{name: "explicit member with bit", user: "u3", wsInfo: WorkspaceInfo{IsMember: true}, wbRolePerm: permPtr(PermViewWorkbench | PermManageMembers), need: PermManageMembers},
		{name: "explicit member missing bit", user: "u3", wsInfo: WorkspaceInfo{IsMember: true}, wbRolePerm: permPtr(PermViewWorkbench), need: PermManageMembers, wantErr: ErrForbidden},
		{name: "shared baseline viewer can view", vis: VisibilityShared, user: "u4", wsInfo: WorkspaceInfo{IsMember: true, Perms: wsPermViewWorkbenches}, need: PermViewWorkbench},
		{name: "shared baseline viewer cannot manage", vis: VisibilityShared, user: "u4", wsInfo: WorkspaceInfo{IsMember: true, Perms: wsPermViewWorkbenches}, need: PermManageWorkbench, wantErr: ErrForbidden},
		{name: "private non-member hidden", user: "u5", wsInfo: WorkspaceInfo{IsMember: true}, need: PermViewWorkbench, wantErr: ErrWorkbenchNotFound},
		{name: "shared without view bit hidden", vis: VisibilityShared, user: "u6", wsInfo: WorkspaceInfo{IsMember: true}, need: PermViewWorkbench, wantErr: ErrWorkbenchNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, f := newTestService()
			wb := base
			if tc.vis != "" {
				wb.Visibility = tc.vis
			}
			f.wb.items[wbID] = wb
			f.wsa.infos[wsID+"|"+tc.user] = tc.wsInfo
			if tc.wbRolePerm != nil {
				r, _ := f.roles.Create(context.Background(), Role{WorkbenchID: wbID, Name: "r", Permissions: *tc.wbRolePerm})
				f.mem.items[mkey(wbID, tc.user)] = Membership{Member: Member{WorkbenchID: wbID, UserID: tc.user, RoleID: r.ID}, Role: r}
			}
			_, err := svc.authorize(context.Background(), wbID, tc.user, tc.need)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("authorize: got %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func permPtr(p Permission) *Permission { return &p }

// PermDeleteSentinel is a stand-in bit for "needs more than view"; use ManageRoles.
func PermDeleteSentinel() Permission { return PermManageRoles }
