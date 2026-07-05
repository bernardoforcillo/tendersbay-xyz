package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type RoleRepo struct{ db *pg.DB }

func NewRoleRepo(db *pg.DB) *RoleRepo { return &RoleRepo{db: db} }

var _ workspace.RoleRepository = (*RoleRepo)(nil)

func (r *RoleRepo) Create(ctx context.Context, role workspace.Role) (workspace.Role, error) {
	var row DBWorkspaceRole
	err := r.db.Insert(WorkspaceRoles).
		Row(
			WRoleWorkspaceID.Val(role.WorkspaceID),
			WRoleName.Val(role.Name),
			WRolePermissions.Val(int64(role.Permissions)),
			WRoleIsDefault.Val(role.IsDefault),
		).
		Returning(WRoleID, WRoleWorkspaceID, WRoleName, WRolePermissions, WRoleIsDefault, WRoleCreatedAt).
		One(ctx, &row)
	if err != nil {
		return workspace.Role{}, err
	}
	return dbRoleToDomain(row), nil
}

func (r *RoleRepo) FindByID(ctx context.Context, id string) (workspace.Role, error) {
	var row DBWorkspaceRole
	err := r.db.Select().From(WorkspaceRoles).Where(WRoleID.Eq(id)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.Role{}, workspace.ErrRoleNotFound
	}
	if err != nil {
		return workspace.Role{}, err
	}
	return dbRoleToDomain(row), nil
}

func (r *RoleRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]workspace.Role, error) {
	var rows []DBWorkspaceRole
	err := r.db.Select().From(WorkspaceRoles).
		Where(WRoleWorkspaceID.Eq(workspaceID)).
		OrderBy(WRoleCreatedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]workspace.Role, len(rows))
	for i, row := range rows {
		out[i] = dbRoleToDomain(row)
	}
	return out, nil
}

func (r *RoleRepo) Update(ctx context.Context, id, name string, perms workspace.Permission) (workspace.Role, error) {
	var row DBWorkspaceRole
	err := r.db.Update(WorkspaceRoles).
		Set(WRoleName.Val(name), WRolePermissions.Val(int64(perms))).
		Where(WRoleID.Eq(id)).
		Returning(WRoleID, WRoleWorkspaceID, WRoleName, WRolePermissions, WRoleIsDefault, WRoleCreatedAt).
		One(ctx, &row)
	if err != nil {
		return workspace.Role{}, err
	}
	return dbRoleToDomain(row), nil
}

func (r *RoleRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(WorkspaceRoles).Where(WRoleID.Eq(id)).Exec(ctx)
	return err
}

func (r *RoleRepo) CountMembersUsing(ctx context.Context, roleID string) (int64, error) {
	return r.db.Select().From(WorkspaceMembers).Where(WMemberRoleID.Eq(roleID)).Count(ctx)
}

func dbRoleToDomain(row DBWorkspaceRole) workspace.Role {
	return workspace.Role{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
		Permissions: workspace.Permission(row.Permissions),
		IsDefault:   row.IsDefault,
		CreatedAt:   row.CreatedAt,
	}
}
