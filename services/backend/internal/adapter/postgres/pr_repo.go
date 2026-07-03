package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

type PRRepo struct{ db *pg.DB }

func NewPRRepo(db *pg.DB) *PRRepo { return &PRRepo{db: db} }

func (r *PRRepo) Create(ctx context.Context, pr auth.PasswordReset) (auth.PasswordReset, error) {
	var row DBPasswordReset
	err := r.db.Insert(PasswordResets).
		Row(
			PRUserID.Val(pr.UserID),
			PRTokenHash.Val(pr.TokenHash),
			PRExpiresAt.Val(pr.ExpiresAt),
		).
		Returning(PRID, PRUserID, PRTokenHash, PRExpiresAt, PRCreatedAt).
		One(ctx, &row)
	if err != nil {
		return auth.PasswordReset{}, err
	}
	return auth.PasswordReset{
		ID: row.ID, UserID: row.UserID, TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (r *PRRepo) FindByTokenHash(ctx context.Context, hash string) (auth.PasswordReset, error) {
	var row DBPasswordReset
	err := r.db.Select().From(PasswordResets).Where(PRTokenHash.Eq(hash)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return auth.PasswordReset{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.PasswordReset{}, err
	}
	return auth.PasswordReset{
		ID: row.ID, UserID: row.UserID, TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (r *PRRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(PasswordResets).Where(PRID.Eq(id)).Exec(ctx)
	return err
}

func (r *PRRepo) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.Delete(PasswordResets).Where(PRUserID.Eq(userID)).Exec(ctx)
	return err
}
