package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// WorkbenchWorkspaceAccess implements workbench.WorkspaceAccess by reading the
// caller's standing in a workspace from the workspace tables.
type WorkbenchWorkspaceAccess struct {
	workspaces *WorkspaceRepo
	members    *MemberRepo
}

func NewWorkbenchWorkspaceAccess(db *pg.DB) *WorkbenchWorkspaceAccess {
	return &WorkbenchWorkspaceAccess{workspaces: NewWorkspaceRepo(db), members: NewMemberRepo(db)}
}

var _ workbench.WorkspaceAccess = (*WorkbenchWorkspaceAccess)(nil)

func (a *WorkbenchWorkspaceAccess) Lookup(ctx context.Context, workspaceID, userID string) (workbench.WorkspaceInfo, error) {
	ws, err := a.workspaces.FindByID(ctx, workspaceID)
	if err != nil {
		return workbench.WorkspaceInfo{}, err
	}
	info := workbench.WorkspaceInfo{Name: ws.Name}
	if ws.OwnerID == userID {
		info.IsOwner = true
		info.IsMember = true
		return info, nil
	}
	m, err := a.members.LoadMembership(ctx, workspaceID, userID)
	if errors.Is(err, workspace.ErrNotMember) {
		return info, nil // not a member; IsMember stays false
	}
	if err != nil {
		return workbench.WorkspaceInfo{}, err
	}
	info.IsMember = true
	info.Perms = uint64(m.Role.Permissions)
	return info, nil
}
