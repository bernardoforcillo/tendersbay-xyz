package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

type EVRepo struct{ db *pg.DB }

func NewEVRepo(db *pg.DB) *EVRepo { return &EVRepo{db: db} }

func (r *EVRepo) Create(ctx context.Context, ev auth.EmailVerification) (auth.EmailVerification, error) {
	var row DBEmailVerification
	err := r.db.Insert(EmailVerifications).
		Row(
			EVUserID.Val(ev.UserID),
			EVNewEmail.Val(ev.NewEmail),
			EVTokenHash.Val(ev.TokenHash),
			EVExpiresAt.Val(ev.ExpiresAt),
		).
		Returning(EVID, EVUserID, EVNewEmail, EVTokenHash, EVExpiresAt, EVCreatedAt).
		One(ctx, &row)
	if err != nil {
		return auth.EmailVerification{}, err
	}
	return auth.EmailVerification{
		ID: row.ID, UserID: row.UserID, NewEmail: row.NewEmail,
		TokenHash: row.TokenHash, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (r *EVRepo) FindByTokenHash(ctx context.Context, hash string) (auth.EmailVerification, error) {
	var row DBEmailVerification
	err := r.db.Select().From(EmailVerifications).Where(EVTokenHash.Eq(hash)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return auth.EmailVerification{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.EmailVerification{}, err
	}
	return auth.EmailVerification{
		ID: row.ID, UserID: row.UserID, NewEmail: row.NewEmail,
		TokenHash: row.TokenHash, ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (r *EVRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(EmailVerifications).Where(EVID.Eq(id)).Exec(ctx)
	return err
}

func (r *EVRepo) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.Delete(EmailVerifications).Where(EVUserID.Eq(userID)).Exec(ctx)
	return err
}
