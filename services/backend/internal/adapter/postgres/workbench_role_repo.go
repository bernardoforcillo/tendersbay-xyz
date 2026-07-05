package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

type WorkbenchRoleRepo struct{ db *pg.DB }

func NewWorkbenchRoleRepo(db *pg.DB) *WorkbenchRoleRepo { return &WorkbenchRoleRepo{db: db} }

var _ workbench.WorkbenchRoleRepository = (*WorkbenchRoleRepo)(nil)

func (r *WorkbenchRoleRepo) Create(ctx context.Context, role workbench.Role) (workbench.Role, error) {
	var row DBWorkbenchRole
	err := r.db.Insert(WorkbenchRoles).
		Row(
			WBRoleWorkbenchID.Val(role.WorkbenchID), WBRoleName.Val(role.Name),
			WBRolePermissions.Val(int64(role.Permissions)), WBRoleIsDefault.Val(role.IsDefault),
		).
		Returning(WBRoleID, WBRoleWorkbenchID, WBRoleName, WBRolePermissions, WBRoleIsDefault, WBRoleCreatedAt).
		One(ctx, &row)
	if err != nil {
		return workbench.Role{}, err
	}
	return dbWorkbenchRoleToDomain(row), nil
}

func (r *WorkbenchRoleRepo) FindByID(ctx context.Context, id string) (workbench.Role, error) {
	var row DBWorkbenchRole
	err := r.db.Select().From(WorkbenchRoles).Where(WBRoleID.Eq(id)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workbench.Role{}, workbench.ErrRoleNotFound
	}
	if err != nil {
		return workbench.Role{}, err
	}
	return dbWorkbenchRoleToDomain(row), nil
}

func (r *WorkbenchRoleRepo) ListByWorkbench(ctx context.Context, workbenchID string) ([]workbench.Role, error) {
	var rows []DBWorkbenchRole
	err := r.db.Select().From(WorkbenchRoles).
		Where(WBRoleWorkbenchID.Eq(workbenchID)).
		OrderBy(WBRoleCreatedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]workbench.Role, len(rows))
	for i, row := range rows {
		out[i] = dbWorkbenchRoleToDomain(row)
	}
	return out, nil
}

func (r *WorkbenchRoleRepo) Update(ctx context.Context, id, name string, perms workbench.Permission) (workbench.Role, error) {
	var row DBWorkbenchRole
	err := r.db.Update(WorkbenchRoles).
		Set(WBRoleName.Val(name), WBRolePermissions.Val(int64(perms))).
		Where(WBRoleID.Eq(id)).
		Returning(WBRoleID, WBRoleWorkbenchID, WBRoleName, WBRolePermissions, WBRoleIsDefault, WBRoleCreatedAt).
		One(ctx, &row)
	if err != nil {
		return workbench.Role{}, err
	}
	return dbWorkbenchRoleToDomain(row), nil
}

func (r *WorkbenchRoleRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(WorkbenchRoles).Where(WBRoleID.Eq(id)).Exec(ctx)
	return err
}

func (r *WorkbenchRoleRepo) CountMembersUsing(ctx context.Context, roleID string) (int64, error) {
	return r.db.Select().From(WorkbenchMembers).Where(WBMemberRoleID.Eq(roleID)).Count(ctx)
}

func dbWorkbenchRoleToDomain(row DBWorkbenchRole) workbench.Role {
	return workbench.Role{
		ID:          row.ID,
		WorkbenchID: row.WorkbenchID,
		Name:        row.Name,
		Permissions: workbench.Permission(row.Permissions),
		IsDefault:   row.IsDefault,
		CreatedAt:   row.CreatedAt,
	}
}
