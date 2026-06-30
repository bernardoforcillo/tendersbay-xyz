package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

type UserRepo struct{ db *pg.DB }

func NewUserRepo(db *pg.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) Create(ctx context.Context, u auth.User) (auth.User, error) {
	var row DBUser
	err := r.db.Insert(Users).
		Row(
			UserEmail.Val(u.Email),
			UserPasswordHash.Val(u.PasswordHash),
			UserDisplayName.Val(u.DisplayName),
		).
		Returning(UserID, UserEmail, UserPasswordHash, UserDisplayName, UserEmailVerifiedAt, UserCreatedAt, UserUpdatedAt).
		One(ctx, &row)
	if err != nil {
		return auth.User{}, err
	}
	return dbUserToDomain(row), nil
}

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (auth.User, error) {
	var row DBUser
	err := r.db.Select().From(Users).Where(UserEmail.Eq(email)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	return dbUserToDomain(row), nil
}

func (r *UserRepo) FindByID(ctx context.Context, id string) (auth.User, error) {
	var row DBUser
	err := r.db.Select().From(Users).Where(UserID.Eq(id)).One(ctx, &row)
	if errors.Is(err, pg.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, err
	}
	return dbUserToDomain(row), nil
}

func (r *UserRepo) UpdatePassword(ctx context.Context, id, hash string) error {
	_, err := r.db.Update(Users).
		Set(UserPasswordHash.Val(hash), UserUpdatedAt.Val(time.Now())).
		Where(UserID.Eq(id)).
		Exec(ctx)
	return err
}

func (r *UserRepo) UpdateEmail(ctx context.Context, id, email string) error {
	_, err := r.db.Update(Users).
		Set(UserEmail.Val(email), UserUpdatedAt.Val(time.Now())).
		Where(UserID.Eq(id)).
		Exec(ctx)
	return err
}

func (r *UserRepo) UpdateDisplayName(ctx context.Context, id, displayName string) error {
	_, err := r.db.Update(Users).
		Set(UserDisplayName.Val(displayName), UserUpdatedAt.Val(time.Now())).
		Where(UserID.Eq(id)).
		Exec(ctx)
	return err
}

func (r *UserRepo) MarkEmailVerified(ctx context.Context, id string, at time.Time) error {
	_, err := r.db.Update(Users).
		Set(UserEmailVerifiedAt.Val(at), UserUpdatedAt.Val(time.Now())).
		Where(UserID.Eq(id)).
		Exec(ctx)
	return err
}

func (r *UserRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.Delete(Users).Where(UserID.Eq(id)).Exec(ctx)
	return err
}

func dbUserToDomain(row DBUser) auth.User {
	return auth.User{
		ID:              row.ID,
		Email:           row.Email,
		PasswordHash:    row.PasswordHash,
		DisplayName:     row.DisplayName,
		EmailVerifiedAt: row.EmailVerifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}
