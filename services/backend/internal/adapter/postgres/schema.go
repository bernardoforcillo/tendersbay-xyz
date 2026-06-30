package postgres

import (
	"time"

	"github.com/bernardoforcillo/drops/pg"
)

// Table and column definitions — single source of truth for all repositories.
var (
	Users               = pg.NewTable("users")
	UserID              = pg.Add(Users, pg.Text("id").PrimaryKey())
	UserEmail           = pg.Add(Users, pg.Text("email").NotNull())
	UserPasswordHash    = pg.Add(Users, pg.Text("password_hash").NotNull())
	UserDisplayName     = pg.Add(Users, pg.Text("display_name").NotNull())
	UserEmailVerifiedAt = pg.Add(Users, pg.Timestamp("email_verified_at", true))
	UserCreatedAt       = pg.Add(Users, pg.Timestamp("created_at", true).NotNull())
	UserUpdatedAt       = pg.Add(Users, pg.Timestamp("updated_at", true).NotNull())

	Sessions         = pg.NewTable("sessions")
	SessionID        = pg.Add(Sessions, pg.Text("id").PrimaryKey())
	SessionUserID    = pg.Add(Sessions, pg.Text("user_id").NotNull())
	SessionTokenHash = pg.Add(Sessions, pg.Text("token_hash").NotNull())
	SessionExpiresAt = pg.Add(Sessions, pg.Timestamp("expires_at", true).NotNull())
	SessionCreatedAt = pg.Add(Sessions, pg.Timestamp("created_at", true).NotNull())

	EmailVerifications = pg.NewTable("email_verifications")
	EVID               = pg.Add(EmailVerifications, pg.Text("id").PrimaryKey())
	EVUserID           = pg.Add(EmailVerifications, pg.Text("user_id").NotNull())
	EVNewEmail         = pg.Add(EmailVerifications, pg.Text("new_email").NotNull())
	EVTokenHash        = pg.Add(EmailVerifications, pg.Text("token_hash").NotNull())
	EVExpiresAt        = pg.Add(EmailVerifications, pg.Timestamp("expires_at", true).NotNull())
	EVCreatedAt        = pg.Add(EmailVerifications, pg.Timestamp("created_at", true).NotNull())

	PasswordResets = pg.NewTable("password_resets")
	PRID           = pg.Add(PasswordResets, pg.Text("id").PrimaryKey())
	PRUserID       = pg.Add(PasswordResets, pg.Text("user_id").NotNull())
	PRTokenHash    = pg.Add(PasswordResets, pg.Text("token_hash").NotNull())
	PRExpiresAt    = pg.Add(PasswordResets, pg.Timestamp("expires_at", true).NotNull())
	PRCreatedAt    = pg.Add(PasswordResets, pg.Timestamp("created_at", true).NotNull())
)

// DB scan targets — drops maps fields by `drop` tag.

type DBUser struct {
	ID              string     `drop:"id"`
	Email           string     `drop:"email"`
	PasswordHash    string     `drop:"password_hash"`
	DisplayName     string     `drop:"display_name"`
	EmailVerifiedAt *time.Time `drop:"email_verified_at"`
	CreatedAt       time.Time  `drop:"created_at"`
	UpdatedAt       time.Time  `drop:"updated_at"`
}

type DBSession struct {
	ID        string    `drop:"id"`
	UserID    string    `drop:"user_id"`
	TokenHash string    `drop:"token_hash"`
	ExpiresAt time.Time `drop:"expires_at"`
	CreatedAt time.Time `drop:"created_at"`
}

type DBEmailVerification struct {
	ID        string    `drop:"id"`
	UserID    string    `drop:"user_id"`
	NewEmail  string    `drop:"new_email"`
	TokenHash string    `drop:"token_hash"`
	ExpiresAt time.Time `drop:"expires_at"`
	CreatedAt time.Time `drop:"created_at"`
}

type DBPasswordReset struct {
	ID        string    `drop:"id"`
	UserID    string    `drop:"user_id"`
	TokenHash string    `drop:"token_hash"`
	ExpiresAt time.Time `drop:"expires_at"`
	CreatedAt time.Time `drop:"created_at"`
}
