package postgres

import (
	"context"
	"errors"
	"time"

	dropsstore "github.com/bernardoforcillo/authlayer/store/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

// NewWorkbenchScopeStore returns authlayer's scope store for workbenches — the
// nested half of the RBAC model, with the workspace as its parent.
func NewWorkbenchScopeStore(db *pg.DB) *dropsstore.Store[workbench.Workbench, workbench.Member] {
	return dropsstore.New[workbench.Workbench, workbench.Member](db, dropsstore.WithNames(dropsstore.Names{
		Containers: "workbenches",
		Members:    "workbench_members",
		Roles:      "workbench_roles",
	}))
}

// WorkbenchRepo covers the workbench queries authlayer's scope store has no
// opinion about: the product's own columns and the by-workspace listing.
type WorkbenchRepo struct{ db *pg.DB }

func NewWorkbenchRepo(db *pg.DB) *WorkbenchRepo { return &WorkbenchRepo{db: db} }

var _ workbench.Repository = (*WorkbenchRepo)(nil)

func (r *WorkbenchRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]workbench.Workbench, error) {
	var rows []workbench.Workbench
	err := r.db.Select().From(Workbenches).
		Where(WBWorkspaceID.Eq(workspaceID)).
		OrderBy(WBCreatedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *WorkbenchRepo) UpdateDetails(ctx context.Context, id, name, description string) (workbench.Workbench, error) {
	return r.update(ctx, id, WBName.Val(name), WBDescription.Val(description))
}

func (r *WorkbenchRepo) UpdateVisibility(ctx context.Context, id string, v workbench.Visibility) (workbench.Workbench, error) {
	return r.update(ctx, id, WBVisibility.Val(string(v)))
}

func (r *WorkbenchRepo) update(ctx context.Context, id string, sets ...pg.ColumnValue) (workbench.Workbench, error) {
	var wb workbench.Workbench
	sets = append(sets, WBUpdatedAt.Val(time.Now().UTC()))
	err := r.db.Update(Workbenches).
		Set(sets...).
		Where(WBID.Eq(id)).
		Returning(WBID, WBWorkspaceID, WBName, WBDescription, WBVisibility, WBOwnerID, WBCreatedAt, WBUpdatedAt).
		One(ctx, &wb)
	if errors.Is(err, pg.ErrNoRows) {
		return workbench.Workbench{}, workbench.ErrWorkbenchNotFound
	}
	if err != nil {
		return workbench.Workbench{}, err
	}
	return wb, nil
}

func (r *WorkbenchRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(Workbenches).Where(WBID.Eq(id)).Exec(ctx)
	return err
}
