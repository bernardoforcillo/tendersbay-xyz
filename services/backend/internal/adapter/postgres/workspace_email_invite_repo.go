package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

type EmailInviteRepo struct{ db *pg.DB }

func NewEmailInviteRepo(db *pg.DB) *EmailInviteRepo { return &EmailInviteRepo{db: db} }

var _ workspace.EmailInvitationRepository = (*EmailInviteRepo)(nil)

func (r *EmailInviteRepo) Create(ctx context.Context, inv workspace.EmailInvitation) (workspace.EmailInvitation, error) {
	var row DBWorkspaceEmailInvite
	err := r.db.Insert(WorkspaceEmailInvites).
		Row(
			WEInviteWorkspaceID.Val(inv.WorkspaceID),
			WEInviteEmail.Val(inv.Email),
			WEInviteRoleID.Val(inv.RoleID),
			WEInviteTokenHash.Val(inv.TokenHash),
			WEInviteInvitedBy.Val(inv.InvitedBy),
			WEInviteExpiresAt.Val(inv.ExpiresAt),
		).
		Returning(WEInviteID, WEInviteWorkspaceID, WEInviteEmail, WEInviteRoleID, WEInviteTokenHash, WEInviteInvitedBy, WEInviteExpiresAt, WEInviteCreatedAt).
		One(ctx, &row)
	if err != nil {
		return workspace.EmailInvitation{}, err
	}
	return dbEmailInviteToDomain(row), nil
}

func (r *EmailInviteRepo) FindByTokenHash(ctx context.Context, hash string) (workspace.EmailInvitation, error) {
	var row DBWorkspaceEmailInvite
	err := r.db.Select().From(WorkspaceEmailInvites).Where(WEInviteTokenHash.Eq(hash)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.EmailInvitation{}, workspace.ErrInviteInvalid
	}
	if err != nil {
		return workspace.EmailInvitation{}, err
	}
	return dbEmailInviteToDomain(row), nil
}

func (r *EmailInviteRepo) FindByID(ctx context.Context, id string) (workspace.EmailInvitation, error) {
	var row DBWorkspaceEmailInvite
	err := r.db.Select().From(WorkspaceEmailInvites).Where(WEInviteID.Eq(id)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return workspace.EmailInvitation{}, workspace.ErrInviteInvalid
	}
	if err != nil {
		return workspace.EmailInvitation{}, err
	}
	return dbEmailInviteToDomain(row), nil
}

func (r *EmailInviteRepo) ListByWorkspace(ctx context.Context, workspaceID string) ([]workspace.EmailInvitation, error) {
	var rows []DBWorkspaceEmailInvite
	err := r.db.Select().From(WorkspaceEmailInvites).
		Where(WEInviteWorkspaceID.Eq(workspaceID)).
		OrderBy(WEInviteCreatedAt.Asc()).
		All(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]workspace.EmailInvitation, len(rows))
	for i, row := range rows {
		out[i] = dbEmailInviteToDomain(row)
	}
	return out, nil
}

func (r *EmailInviteRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(WorkspaceEmailInvites).Where(WEInviteID.Eq(id)).Exec(ctx)
	return err
}

func (r *EmailInviteRepo) DeleteByWorkspaceEmail(ctx context.Context, workspaceID, email string) error {
	_, err := r.db.Delete(WorkspaceEmailInvites).
		Where(WEInviteWorkspaceID.Eq(workspaceID), WEInviteEmail.Eq(email)).
		Exec(ctx)
	return err
}

func dbEmailInviteToDomain(row DBWorkspaceEmailInvite) workspace.EmailInvitation {
	return workspace.EmailInvitation{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Email:       row.Email,
		RoleID:      row.RoleID,
		TokenHash:   row.TokenHash,
		InvitedBy:   row.InvitedBy,
		ExpiresAt:   row.ExpiresAt,
		CreatedAt:   row.CreatedAt,
	}
}
