package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type MemberRepo struct{ db *pg.DB }

func NewMemberRepo(db *pg.DB) *MemberRepo { return &MemberRepo{db: db} }

var _ workspace.MemberRepository = (*MemberRepo)(nil)

func (r *MemberRepo) Add(ctx context.Context, m workspace.Member) (workspace.Member, error) {
	var row DBWorkspaceMember
	err := r.db.Insert(WorkspaceMembers).
		Row(WMemberWorkspaceID.Val(m.WorkspaceID), WMemberUserID.Val(m.UserID), WMemberRoleID.Val(m.RoleID)).
		Returning(WMemberWorkspaceID, WMemberUserID, WMemberRoleID, WMemberJoinedAt).
		One(ctx, &row)
	if err != nil {
		return workspace.Member{}, err
	}
	return dbMemberToDomain(row), nil
}

func (r *MemberRepo) Find(ctx context.Context, workspaceID, userID string) (workspace.Member, error) {
	var row DBWorkspaceMember
	err := r.db.Select().From(WorkspaceMembers).
		Where(WMemberWorkspaceID.Eq(workspaceID), WMemberUserID.Eq(userID)).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.Member{}, workspace.ErrNotMember
	}
	if err != nil {
		return workspace.Member{}, err
	}
	return dbMemberToDomain(row), nil
}

func (r *MemberRepo) LoadMembership(ctx context.Context, workspaceID, userID string) (workspace.Membership, error) {
	var row DBMembership
	err := r.db.Select(WMemberWorkspaceID, WMemberUserID, WMemberRoleID, WMemberJoinedAt, WRoleName, WRolePermissions).
		From(WorkspaceMembers).
		Join(WorkspaceRoles, WMemberRoleID.EqCol(WRoleID)).
		Where(WMemberWorkspaceID.Eq(workspaceID), WMemberUserID.Eq(userID)).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.Membership{}, workspace.ErrNotMember
	}
	if err != nil {
		return workspace.Membership{}, err
	}
	return workspace.Membership{
		Member: workspace.Member{
			WorkspaceID: row.WorkspaceID,
			UserID:      row.UserID,
			RoleID:      row.RoleID,
			JoinedAt:    row.JoinedAt,
		},
		Role: workspace.Role{
			ID:          row.RoleID,
			WorkspaceID: row.WorkspaceID,
			Name:        row.RoleName,
			Permissions: workspace.Permission(row.Permissions),
		},
	}, nil
}

func (r *MemberRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]workspace.Member, error) {
	var rows []DBWorkspaceMember
	err := r.db.Select().From(WorkspaceMembers).
		Where(WMemberWorkspaceID.Eq(workspaceID)).
		OrderBy(WMemberJoinedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]workspace.Member, len(rows))
	for i, row := range rows {
		out[i] = dbMemberToDomain(row)
	}
	return out, nil
}

func (r *MemberRepo) UpdateRole(ctx context.Context, workspaceID, userID, roleID string) error {
	_, err := r.db.Update(WorkspaceMembers).
		Set(WMemberRoleID.Val(roleID)).
		Where(WMemberWorkspaceID.Eq(workspaceID), WMemberUserID.Eq(userID)).
		Exec(ctx)
	return err
}

func (r *MemberRepo) Remove(ctx context.Context, workspaceID, userID string) error {
	_, err := r.db.Delete(WorkspaceMembers).
		Where(WMemberWorkspaceID.Eq(workspaceID), WMemberUserID.Eq(userID)).
		Exec(ctx)
	return err
}

func (r *MemberRepo) CountByWorkspace(ctx context.Context, workspaceID string) (int64, error) {
	return r.db.Select().From(WorkspaceMembers).Where(WMemberWorkspaceID.Eq(workspaceID)).Count(ctx)
}

func dbMemberToDomain(row DBWorkspaceMember) workspace.Member {
	return workspace.Member{
		WorkspaceID: row.WorkspaceID,
		UserID:      row.UserID,
		RoleID:      row.RoleID,
		JoinedAt:    row.JoinedAt,
	}
}
