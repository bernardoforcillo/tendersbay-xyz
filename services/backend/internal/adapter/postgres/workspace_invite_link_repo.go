package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type InviteLinkRepo struct{ db *pg.DB }

func NewInviteLinkRepo(db *pg.DB) *InviteLinkRepo { return &InviteLinkRepo{db: db} }

var _ workspace.InviteLinkRepository = (*InviteLinkRepo)(nil)

func (r *InviteLinkRepo) Create(ctx context.Context, l workspace.InviteLink) (workspace.InviteLink, error) {
	vals := []pg.ColumnValue{
		WLinkWorkspaceID.Val(l.WorkspaceID),
		WLinkCode.Val(l.Code),
		WLinkRoleID.Val(l.RoleID),
		WLinkCreatedBy.Val(l.CreatedBy),
		WLinkMaxUses.Val(l.MaxUses),
	}
	if l.ExpiresAt != nil {
		vals = append(vals, WLinkExpiresAt.Val(*l.ExpiresAt))
	}
	var row DBWorkspaceInviteLink
	err := r.db.Insert(WorkspaceInviteLinks).
		Row(vals...).
		Returning(WLinkID, WLinkWorkspaceID, WLinkCode, WLinkRoleID, WLinkCreatedBy, WLinkMaxUses, WLinkUseCount, WLinkExpiresAt, WLinkRevoked, WLinkCreatedAt).
		One(ctx, &row)
	if err != nil {
		return workspace.InviteLink{}, err
	}
	return dbInviteLinkToDomain(row), nil
}

func (r *InviteLinkRepo) FindByCode(ctx context.Context, code string) (workspace.InviteLink, error) {
	var row DBWorkspaceInviteLink
	err := r.db.Select().From(WorkspaceInviteLinks).Where(WLinkCode.Eq(code)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.InviteLink{}, workspace.ErrLinkInvalid
	}
	if err != nil {
		return workspace.InviteLink{}, err
	}
	return dbInviteLinkToDomain(row), nil
}

func (r *InviteLinkRepo) FindByID(ctx context.Context, id string) (workspace.InviteLink, error) {
	var row DBWorkspaceInviteLink
	err := r.db.Select().From(WorkspaceInviteLinks).Where(WLinkID.Eq(id)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.InviteLink{}, workspace.ErrLinkInvalid
	}
	if err != nil {
		return workspace.InviteLink{}, err
	}
	return dbInviteLinkToDomain(row), nil
}

func (r *InviteLinkRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]workspace.InviteLink, error) {
	var rows []DBWorkspaceInviteLink
	err := r.db.Select().From(WorkspaceInviteLinks).
		Where(WLinkWorkspaceID.Eq(workspaceID)).
		OrderBy(WLinkCreatedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]workspace.InviteLink, len(rows))
	for i, row := range rows {
		out[i] = dbInviteLinkToDomain(row)
	}
	return out, nil
}

func (r *InviteLinkRepo) IncrementUse(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE workspace_invite_links SET use_count = use_count + 1 WHERE id = $1`, id)
	return err
}

func (r *InviteLinkRepo) Revoke(ctx context.Context, id string) error {
	_, err := r.db.Update(WorkspaceInviteLinks).
		Set(WLinkRevoked.Val(true)).
		Where(WLinkID.Eq(id)).
		Exec(ctx)
	return err
}

func dbInviteLinkToDomain(row DBWorkspaceInviteLink) workspace.InviteLink {
	return workspace.InviteLink{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Code:        row.Code,
		RoleID:      row.RoleID,
		CreatedBy:   row.CreatedBy,
		MaxUses:     row.MaxUses,
		UseCount:    row.UseCount,
		ExpiresAt:   row.ExpiresAt,
		Revoked:     row.Revoked,
		CreatedAt:   row.CreatedAt,
	}
}
