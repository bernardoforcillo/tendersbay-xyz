package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

type WorkbenchMemberRepo struct{ db *pg.DB }

func NewWorkbenchMemberRepo(db *pg.DB) *WorkbenchMemberRepo { return &WorkbenchMemberRepo{db: db} }

var _ workbench.WorkbenchMemberRepository = (*WorkbenchMemberRepo)(nil)

func (r *WorkbenchMemberRepo) Add(ctx context.Context, m workbench.Member) (workbench.Member, error) {
	var row DBWorkbenchMember
	err := r.db.Insert(WorkbenchMembers).
		Row(WBMemberWorkbenchID.Val(m.WorkbenchID), WBMemberUserID.Val(m.UserID), WBMemberRoleID.Val(m.RoleID)).
		Returning(WBMemberWorkbenchID, WBMemberUserID, WBMemberRoleID, WBMemberAddedAt).
		One(ctx, &row)
	if err != nil {
		return workbench.Member{}, err
	}
	return dbWorkbenchMemberToDomain(row), nil
}

func (r *WorkbenchMemberRepo) Find(ctx context.Context, workbenchID, userID string) (workbench.Member, error) {
	var row DBWorkbenchMember
	err := r.db.Select().From(WorkbenchMembers).
		Where(WBMemberWorkbenchID.Eq(workbenchID), WBMemberUserID.Eq(userID)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workbench.Member{}, workbench.ErrNotMember
	}
	if err != nil {
		return workbench.Member{}, err
	}
	return dbWorkbenchMemberToDomain(row), nil
}

func (r *WorkbenchMemberRepo) LoadMembership(ctx context.Context, workbenchID, userID string) (workbench.Membership, error) {
	var row DBWorkbenchMembership
	err := r.db.Select(WBMemberWorkbenchID, WBMemberUserID, WBMemberRoleID, WBMemberAddedAt, WBRoleName, WBRolePermissions).
		From(WorkbenchMembers).
		Join(WorkbenchRoles, WBMemberRoleID.EqCol(WBRoleID)).
		Where(WBMemberWorkbenchID.Eq(workbenchID), WBMemberUserID.Eq(userID)).
		One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workbench.Membership{}, workbench.ErrNotMember
	}
	if err != nil {
		return workbench.Membership{}, err
	}
	return workbench.Membership{
		Member: workbench.Member{WorkbenchID: row.WorkbenchID, UserID: row.UserID, RoleID: row.RoleID, AddedAt: row.AddedAt},
		Role:   workbench.Role{ID: row.RoleID, WorkbenchID: row.WorkbenchID, Name: row.RoleName, Permissions: workbench.Permission(row.Permissions)},
	}, nil
}

func (r *WorkbenchMemberRepo) ListByWorkbench(ctx context.Context, workbenchID string) ([]workbench.Member, error) {
	var rows []DBWorkbenchMember
	err := r.db.Select().From(WorkbenchMembers).
		Where(WBMemberWorkbenchID.Eq(workbenchID)).
		OrderBy(WBMemberAddedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]workbench.Member, len(rows))
	for i, row := range rows {
		out[i] = dbWorkbenchMemberToDomain(row)
	}
	return out, nil
}

func (r *WorkbenchMemberRepo) UpdateRole(ctx context.Context, workbenchID, userID, roleID string) error {
	_, err := r.db.Update(WorkbenchMembers).
		Set(WBMemberRoleID.Val(roleID)).
		Where(WBMemberWorkbenchID.Eq(workbenchID), WBMemberUserID.Eq(userID)).Exec(ctx)
	return err
}

func (r *WorkbenchMemberRepo) Remove(ctx context.Context, workbenchID, userID string) error {
	_, err := r.db.Delete(WorkbenchMembers).
		Where(WBMemberWorkbenchID.Eq(workbenchID), WBMemberUserID.Eq(userID)).Exec(ctx)
	return err
}

func (r *WorkbenchMemberRepo) CountByWorkbench(ctx context.Context, workbenchID string) (int64, error) {
	return r.db.Select().From(WorkbenchMembers).Where(WBMemberWorkbenchID.Eq(workbenchID)).Count(ctx)
}

func dbWorkbenchMemberToDomain(row DBWorkbenchMember) workbench.Member {
	return workbench.Member{WorkbenchID: row.WorkbenchID, UserID: row.UserID, RoleID: row.RoleID, AddedAt: row.AddedAt}
}
