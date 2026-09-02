package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
)

// UserRepo is the profile half of the users table: the columns authlayer's
// auth.Store has no opinion about (display_name) plus the ones a caller needs
// alongside a name to render a member row. Everything credential-shaped —
// creating the row, moving the address, rotating the hash, stamping
// verification, anonymizing — belongs to authlayer's store and is deliberately
// absent here, so there is exactly one writer per column.
type UserRepo struct{ db *pg.DB }

func NewUserRepo(db *pg.DB) *UserRepo { return &UserRepo{db: db} }

var _ auth.UserRepository = (*UserRepo)(nil)

func (r *UserRepo) FindByEmail(ctx context.Context, email string) (auth.User, error) {
	var row DBUser
	err := r.db.Select().From(Users).Where(UserEmail.Eq(auth.NormalizeEmail(email))).One(ctx, &row)
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

// UpdateDisplayName is the only write this repository still owns. It does not
// touch updated_at's credential meaning — authlayer stamps that column on every
// write of its own — but it does bump it, because a renamed profile is a
// changed row and leaving the stamp behind would make the two writers disagree
// about when the row last moved.
func (r *UserRepo) UpdateDisplayName(ctx context.Context, id, displayName string) error {
	res, err := r.db.Update(Users).
		Set(UserDisplayName.Val(displayName), UserUpdatedAt.Val(time.Now().UTC())).
		Where(UserID.Eq(id)).
		Exec(ctx)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return auth.ErrNotFound
	}
	return nil
}

func dbUserToDomain(row DBUser) auth.User {
	return auth.User{
		ID:              row.ID,
		Email:           row.Email,
		DisplayName:     row.DisplayName,
		EmailVerifiedAt: row.EmailVerifiedAt,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		DeletedAt:       row.DeletedAt,
	}
}
