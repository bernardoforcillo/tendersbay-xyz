package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

type WorkbenchRepo struct{ db *pg.DB }

func NewWorkbenchRepo(db *pg.DB) *WorkbenchRepo { return &WorkbenchRepo{db: db} }

var _ workbench.WorkbenchRepository = (*WorkbenchRepo)(nil)

func (r *WorkbenchRepo) Create(ctx context.Context, w workbench.Workbench) (workbench.Workbench, error) {
	var row DBWorkbench
	err := r.db.Insert(Workbenches).
		Row(
			WBWorkspaceID.Val(w.WorkspaceID), WBName.Val(w.Name), WBDescription.Val(w.Description),
			WBVisibility.Val(string(w.Visibility)), WBOwnerID.Val(w.OwnerID),
		).
		Returning(WBID, WBWorkspaceID, WBName, WBDescription, WBVisibility, WBOwnerID, WBCreatedAt, WBUpdatedAt).
		One(ctx, &row)
	if err != nil {
		return workbench.Workbench{}, err
	}
	return dbWorkbenchToDomain(row), nil
}

func (r *WorkbenchRepo) FindByID(ctx context.Context, id string) (workbench.Workbench, error) {
	var row DBWorkbench
	err := r.db.Select().From(Workbenches).Where(WBID.Eq(id)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workbench.Workbench{}, workbench.ErrWorkbenchNotFound
	}
	if err != nil {
		return workbench.Workbench{}, err
	}
	return dbWorkbenchToDomain(row), nil
}

func (r *WorkbenchRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]workbench.Workbench, error) {
	var rows []DBWorkbench
	err := r.db.Select().From(Workbenches).
		Where(WBWorkspaceID.Eq(workspaceID)).
		OrderBy(WBCreatedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]workbench.Workbench, len(rows))
	for i, row := range rows {
		out[i] = dbWorkbenchToDomain(row)
	}
	return out, nil
}

func (r *WorkbenchRepo) Update(ctx context.Context, id, name, description string) (workbench.Workbench, error) {
	var row DBWorkbench
	err := r.db.Update(Workbenches).
		Set(WBName.Val(name), WBDescription.Val(description), WBUpdatedAt.Val(time.Now())).
		Where(WBID.Eq(id)).
		Returning(WBID, WBWorkspaceID, WBName, WBDescription, WBVisibility, WBOwnerID, WBCreatedAt, WBUpdatedAt).
		One(ctx, &row)
	if err != nil {
		return workbench.Workbench{}, err
	}
	return dbWorkbenchToDomain(row), nil
}

func (r *WorkbenchRepo) UpdateVisibility(ctx context.Context, id string, v workbench.Visibility) (workbench.Workbench, error) {
	var row DBWorkbench
	err := r.db.Update(Workbenches).
		Set(WBVisibility.Val(string(v)), WBUpdatedAt.Val(time.Now())).
		Where(WBID.Eq(id)).
		Returning(WBID, WBWorkspaceID, WBName, WBDescription, WBVisibility, WBOwnerID, WBCreatedAt, WBUpdatedAt).
		One(ctx, &row)
	if err != nil {
		return workbench.Workbench{}, err
	}
	return dbWorkbenchToDomain(row), nil
}

func (r *WorkbenchRepo) UpdateOwner(ctx context.Context, id, newOwnerID string) error {
	_, err := r.db.Update(Workbenches).
		Set(WBOwnerID.Val(newOwnerID), WBUpdatedAt.Val(time.Now())).
		Where(WBID.Eq(id)).Exec(ctx)
	return err
}

func (r *WorkbenchRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(Workbenches).Where(WBID.Eq(id)).Exec(ctx)
	return err
}

func dbWorkbenchToDomain(row DBWorkbench) workbench.Workbench {
	return workbench.Workbench{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
		Description: row.Description,
		Visibility:  workbench.Visibility(row.Visibility),
		OwnerID:     row.OwnerID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
