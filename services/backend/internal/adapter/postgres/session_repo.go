package postgres

import (
	"context"
	"errors"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

type SessionRepo struct{ db *pg.DB }

func NewSessionRepo(db *pg.DB) *SessionRepo { return &SessionRepo{db: db} }

func (r *SessionRepo) Create(ctx context.Context, s auth.Session) (auth.Session, error) {
	var row DBSession
	err := r.db.Insert(Sessions).
		Row(
			SessionUserID.Val(s.UserID),
			SessionTokenHash.Val(s.TokenHash),
			SessionExpiresAt.Val(s.ExpiresAt),
		).
		Returning(SessionID, SessionUserID, SessionTokenHash, SessionExpiresAt, SessionCreatedAt).
		One(ctx, &row)
	if err != nil {
		return auth.Session{}, err
	}
	return auth.Session{
		ID: row.ID, UserID: row.UserID, TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (r *SessionRepo) FindByTokenHash(ctx context.Context, hash string) (auth.Session, error) {
	var row DBSession
	err := r.db.Select().From(Sessions).Where(SessionTokenHash.Eq(hash)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return auth.Session{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.Session{}, err
	}
	return auth.Session{
		ID: row.ID, UserID: row.UserID, TokenHash: row.TokenHash,
		ExpiresAt: row.ExpiresAt, CreatedAt: row.CreatedAt,
	}, nil
}

func (r *SessionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(Sessions).Where(SessionID.Eq(id)).Exec(ctx)
	return err
}

func (r *SessionRepo) DeleteByUserID(ctx context.Context, userID string) error {
	_, err := r.db.Delete(Sessions).Where(SessionUserID.Eq(userID)).Exec(ctx)
	return err
}
