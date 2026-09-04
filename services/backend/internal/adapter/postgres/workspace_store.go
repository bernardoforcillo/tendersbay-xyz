package postgres

import (
	"context"
	"errors"
	"time"

	dropsstore "github.com/bernardoforcillo/authlayer/store/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// workspaceTables names the three tables authlayer's scope store persists a
// workspace's containment to, and inviteTables the two the invitation store
// uses. Both default to organization-flavoured names in the library, so every
// construction site has to pass these — they are declared once, here.
func workspaceTables() dropsstore.Names {
	return dropsstore.Names{
		Containers: "workspaces",
		Members:    "workspace_members",
		Roles:      "workspace_roles",
	}
}

func inviteTables() dropsstore.InviteNames {
	return dropsstore.InviteNames{
		EmailInvites: "workspace_email_invitations",
		Links:        "workspace_invite_links",
	}
}

// NewWorkspaceScopeStore returns authlayer's scope store for workspaces: the
// containers, memberships and custom roles behind workspace.Service's RBAC.
func NewWorkspaceScopeStore(db *pg.DB) *dropsstore.Store[workspace.Workspace, workspace.Member] {
	return dropsstore.New[workspace.Workspace, workspace.Member](db, dropsstore.WithNames(workspaceTables()))
}

// NewWorkspaceInviteStore returns authlayer's invitation store for workspaces.
func NewWorkspaceInviteStore(db *pg.DB) *dropsstore.InviteStore {
	return dropsstore.NewInviteStore(db, dropsstore.WithInviteNames(inviteTables()))
}

// WorkspaceRepo covers the workspace queries authlayer's scope store has no
// opinion about, because they are about this product's own columns rather than
// about containment: resolving a slug, renaming, deleting.
type WorkspaceRepo struct{ db *pg.DB }

func NewWorkspaceRepo(db *pg.DB) *WorkspaceRepo { return &WorkspaceRepo{db: db} }

var _ workspace.Repository = (*WorkspaceRepo)(nil)

func (r *WorkspaceRepo) FindBySlug(ctx context.Context, slug string) (workspace.Workspace, error) {
	var ws workspace.Workspace
	err := r.db.Select().From(Workspaces).Where(WorkspaceSlug.Eq(slug)).One(ctx, &ws)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}
	if err != nil {
		return workspace.Workspace{}, err
	}
	return ws, nil
}

func (r *WorkspaceRepo) UpdateNameSlug(ctx context.Context, id, name, slug string) (workspace.Workspace, error) {
	var ws workspace.Workspace
	err := r.db.Update(Workspaces).
		Set(WorkspaceName.Val(name), WorkspaceSlug.Val(slug), WorkspaceUpdatedAt.Val(time.Now().UTC())).
		Where(WorkspaceID.Eq(id)).
		Returning(WorkspaceID, WorkspaceName, WorkspaceSlug, WorkspaceOwnerID, WorkspaceCreatedAt, WorkspaceUpdatedAt).
		One(ctx, &ws)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}
	if err != nil {
		return workspace.Workspace{}, err
	}
	return ws, nil
}

func (r *WorkspaceRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(Workspaces).Where(WorkspaceID.Eq(id)).Exec(ctx)
	return err
}
