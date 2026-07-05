package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type WorkspaceRepo struct{ db *pg.DB }

func NewWorkspaceRepo(db *pg.DB) *WorkspaceRepo { return &WorkspaceRepo{db: db} }

var _ workspace.WorkspaceRepository = (*WorkspaceRepo)(nil)

func (r *WorkspaceRepo) Create(ctx context.Context, w workspace.Workspace) (workspace.Workspace, error) {
	var row DBWorkspace
	err := r.db.Insert(Workspaces).
		Row(WorkspaceName.Val(w.Name), WorkspaceSlug.Val(w.Slug), WorkspaceOwnerID.Val(w.OwnerID)).
		Returning(WorkspaceID, WorkspaceName, WorkspaceSlug, WorkspaceOwnerID, WorkspaceCreatedAt, WorkspaceUpdatedAt).
		One(ctx, &row)
	if err != nil {
		return workspace.Workspace{}, err
	}
	return dbWorkspaceToDomain(row), nil
}

func (r *WorkspaceRepo) FindByID(ctx context.Context, id string) (workspace.Workspace, error) {
	var row DBWorkspace
	err := r.db.Select().From(Workspaces).Where(WorkspaceID.Eq(id)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}
	if err != nil {
		return workspace.Workspace{}, err
	}
	return dbWorkspaceToDomain(row), nil
}

func (r *WorkspaceRepo) FindBySlug(ctx context.Context, slug string) (workspace.Workspace, error) {
	var row DBWorkspace
	err := r.db.Select().From(Workspaces).Where(WorkspaceSlug.Eq(slug)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.Workspace{}, workspace.ErrWorkspaceNotFound
	}
	if err != nil {
		return workspace.Workspace{}, err
	}
	return dbWorkspaceToDomain(row), nil
}

func (r *WorkspaceRepo) ListByUserID(ctx context.Context, userID string) ([]workspace.Workspace, error) {
	var rows []DBWorkspace
	err := r.db.Select(WorkspaceID, WorkspaceName, WorkspaceSlug, WorkspaceOwnerID, WorkspaceCreatedAt, WorkspaceUpdatedAt).
		From(Workspaces).
		Join(WorkspaceMembers, WMemberWorkspaceID.EqCol(WorkspaceID)).
		Where(WMemberUserID.Eq(userID)).
		OrderBy(WorkspaceCreatedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]workspace.Workspace, len(rows))
	for i, row := range rows {
		out[i] = dbWorkspaceToDomain(row)
	}
	return out, nil
}

func (r *WorkspaceRepo) Update(ctx context.Context, id, name, slug string) (workspace.Workspace, error) {
	var row DBWorkspace
	err := r.db.Update(Workspaces).
		Set(WorkspaceName.Val(name), WorkspaceSlug.Val(slug), WorkspaceUpdatedAt.Val(time.Now())).
		Where(WorkspaceID.Eq(id)).
		Returning(WorkspaceID, WorkspaceName, WorkspaceSlug, WorkspaceOwnerID, WorkspaceCreatedAt, WorkspaceUpdatedAt).
		One(ctx, &row)
	if err != nil {
		return workspace.Workspace{}, err
	}
	return dbWorkspaceToDomain(row), nil
}

func (r *WorkspaceRepo) UpdateOwner(ctx context.Context, id, newOwnerID string) error {
	_, err := r.db.Update(Workspaces).
		Set(WorkspaceOwnerID.Val(newOwnerID), WorkspaceUpdatedAt.Val(time.Now())).
		Where(WorkspaceID.Eq(id)).
		Exec(ctx)
	return err
}

func (r *WorkspaceRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(Workspaces).Where(WorkspaceID.Eq(id)).Exec(ctx)
	return err
}

func dbWorkspaceToDomain(row DBWorkspace) workspace.Workspace {
	return workspace.Workspace{
		ID:        row.ID,
		Name:      row.Name,
		Slug:      row.Slug,
		OwnerID:   row.OwnerID,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}
