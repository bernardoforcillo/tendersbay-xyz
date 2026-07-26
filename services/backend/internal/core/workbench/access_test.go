package workbench

import (
	"context"
	"errors"
	"testing"
)

func TestCanManageWorkbench_OwnerAllowed(t *testing.T) {
	svc, f := newTestService()
	f.wb.items["wb1"] = Workbench{ID: "wb1", WorkspaceID: "ws1", OwnerID: "owner", Visibility: VisibilityPrivate}
	f.wsa.infos["ws1|owner"] = WorkspaceInfo{IsMember: true}
	if err := svc.CanManageWorkbench(context.Background(), "owner", "wb1"); err != nil {
		t.Fatalf("owner should manage: %v", err)
	}
}

func TestCanManageWorkbench_ViewerDenied(t *testing.T) {
	svc, f := newTestService()
	f.wb.items["wb1"] = Workbench{ID: "wb1", WorkspaceID: "ws1", OwnerID: "owner", Visibility: VisibilityShared}
	f.wsa.infos["ws1|viewer"] = WorkspaceInfo{IsMember: true, Perms: wsPermViewWorkbenches}
	if err := svc.CanManageWorkbench(context.Background(), "viewer", "wb1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("shared viewer cannot manage: want ErrForbidden, got %v", err)
	}
}

func TestCanManageWorkbench_NonMemberHidden(t *testing.T) {
	svc, f := newTestService()
	f.wb.items["wb1"] = Workbench{ID: "wb1", WorkspaceID: "ws1", OwnerID: "owner", Visibility: VisibilityPrivate}
	f.wsa.infos["ws1|stranger"] = WorkspaceInfo{IsMember: false}
	if err := svc.CanManageWorkbench(context.Background(), "stranger", "wb1"); !errors.Is(err, ErrWorkbenchNotFound) {
		t.Fatalf("non-member: want ErrWorkbenchNotFound, got %v", err)
	}
}

func TestWorkspaceOf_ReturnsParent(t *testing.T) {
	svc, f := newTestService()
	f.wb.items["wb1"] = Workbench{ID: "wb1", WorkspaceID: "ws-parent", OwnerID: "owner"}
	got, err := svc.WorkspaceOf(context.Background(), "wb1")
	if err != nil {
		t.Fatalf("WorkspaceOf: %v", err)
	}
	if got != "ws-parent" {
		t.Fatalf("WorkspaceOf = %q, want ws-parent", got)
	}
}

func TestWorkspaceOf_NotFound(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.WorkspaceOf(context.Background(), "nope"); !errors.Is(err, ErrWorkbenchNotFound) {
		t.Fatalf("want ErrWorkbenchNotFound, got %v", err)
	}
}
