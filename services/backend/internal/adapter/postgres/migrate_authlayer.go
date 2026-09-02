package postgres

import (
	"context"

	dropsstore "github.com/bernardoforcillo/authlayer/store/drops"
	"github.com/bernardoforcillo/drops/pg"
)

// migrateAuthlayer is the 0012 schema migration that hands authentication over
// to github.com/bernardoforcillo/authlayer.
//
// Three shapes change:
//
//   - users gains deleted_at (authlayer's anonymization stamp) and a DEFAULT ”
//     on display_name. The default is load-bearing, not cosmetic: authlayer's
//     store writes only the columns it owns, so a NOT NULL display_name with no
//     default would fail every INSERT it makes. core/auth sets the real name
//     immediately after signup.
//   - sessions gains the refresh-rotation columns (family_id, rotated_at) plus
//     the audit and step-up ones (user_agent, ip, mfa_at). family_id is
//     backfilled from id — a pre-migration session is a family of one, which is
//     exactly what it was — before it is made NOT NULL.
//   - email_verifications and password_resets are replaced by one verifications
//     table carrying a purpose column, which is how authlayer distinguishes the
//     signup, email-change, password-reset, magic-link and MFA-challenge flows.
//
// The verifications table itself is created by authlayer's own CreateSchema
// rather than declared here, so its columns, constraints and indexes stay
// derived from the library's structs instead of a copy that can drift. That
// call is idempotent and never alters an existing table, which is why the
// users and sessions ALTERs above it are written out by hand — see
// dropsstore.AuthStore.CreateSchema's own doc on exactly that limitation.
func migrateAuthlayer() pg.Migration {
	return pg.Migration{
		Version: "0012",
		Name:    "authlayer_auth",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, s := range []string{
				`ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`,
				`ALTER TABLE users ALTER COLUMN display_name SET DEFAULT ''`,

				`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS family_id UUID`,
				`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS rotated_at TIMESTAMPTZ`,
				`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS ip TEXT NOT NULL DEFAULT ''`,
				`ALTER TABLE sessions ADD COLUMN IF NOT EXISTS mfa_at TIMESTAMPTZ`,
				// A session minted before rotation existed is the only member of
				// its own family, so its id is the correct family id.
				`UPDATE sessions SET family_id = id WHERE family_id IS NULL`,
				`ALTER TABLE sessions ALTER COLUMN family_id SET NOT NULL`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}

			// Creates verifications and self-heals the constraints and indexes
			// authlayer needs on all three tables. Safe against users and
			// sessions, which already exist: every statement it issues is
			// idempotent.
			if err := dropsstore.NewAuthStore(db).CreateSchema(ctx); err != nil {
				return err
			}

			for _, s := range []string{
				// Carry live tokens across so a link already in someone's inbox
				// keeps working. Expired rows are left behind — they are dead
				// either way, and copying them would only give PurgeExpired more
				// to sweep.
				//
				// An email_verifications row records no purpose, so every one of
				// them is copied as "signup". An email-CHANGE token among them
				// therefore lands with a purpose whose redemption compares its
				// address against the account's current one and refuses on the
				// mismatch — the safe direction: an in-flight address change has
				// to be re-requested, and none is silently certified.
				`INSERT INTO verifications (id, user_id, token_hash, purpose, email, expires_at, created_at)
				 SELECT id, user_id, token_hash, 'signup', lower(trim(new_email)), expires_at, created_at
				 FROM email_verifications WHERE expires_at > NOW()
				 ON CONFLICT DO NOTHING`,
				// A password_resets row records no address at all, so it is taken
				// from the account the token belongs to. That is the address the
				// mail was delivered to, which is precisely what authlayer
				// compares on redemption.
				`INSERT INTO verifications (id, user_id, token_hash, purpose, email, expires_at, created_at)
				 SELECT pr.id, pr.user_id, pr.token_hash, 'password_reset', u.email, pr.expires_at, pr.created_at
				 FROM password_resets pr JOIN users u ON u.id = pr.user_id
				 WHERE pr.expires_at > NOW()
				 ON CONFLICT DO NOTHING`,

				`ALTER TABLE verifications
				 ADD CONSTRAINT fk_verifications_user
				 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE`,

				`DROP TABLE IF EXISTS email_verifications`,
				`DROP TABLE IF EXISTS password_resets`,
				// 0001 declared UNIQUE(token_hash) inline, which PostgreSQL named
				// sessions_token_hash_key; authlayer registers the same constraint
				// under its own name and CreateSchema has just added it. Two
				// identical unique indexes on one column is pure write
				// amplification, so the older one goes.
				`ALTER TABLE sessions DROP CONSTRAINT IF EXISTS sessions_token_hash_key`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			return nil
		},
		// Down restores the two dropped tables' shapes so a rollback leaves a
		// schema the pre-authlayer code can run against. It does NOT copy the
		// verification rows back: purpose has no column to land in over there,
		// and a token whose flow cannot be recorded is not worth restoring.
		Down: func(ctx context.Context, db *pg.DB) error {
			for _, s := range []string{
				`CREATE TABLE IF NOT EXISTS email_verifications (
				    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
				    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				    new_email  TEXT        NOT NULL,
				    token_hash TEXT        NOT NULL UNIQUE,
				    expires_at TIMESTAMPTZ NOT NULL,
				    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				)`,
				`CREATE TABLE IF NOT EXISTS password_resets (
				    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
				    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
				    token_hash TEXT        NOT NULL UNIQUE,
				    expires_at TIMESTAMPTZ NOT NULL,
				    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				)`,
				`DROP TABLE IF EXISTS verifications`,
				`ALTER TABLE sessions DROP COLUMN IF EXISTS family_id`,
				`ALTER TABLE sessions DROP COLUMN IF EXISTS rotated_at`,
				`ALTER TABLE sessions DROP COLUMN IF EXISTS user_agent`,
				`ALTER TABLE sessions DROP COLUMN IF EXISTS ip`,
				`ALTER TABLE sessions DROP COLUMN IF EXISTS mfa_at`,
				`ALTER TABLE sessions ADD CONSTRAINT sessions_token_hash_key UNIQUE (token_hash)`,
				`ALTER TABLE users DROP COLUMN IF EXISTS deleted_at`,
				`ALTER TABLE users ALTER COLUMN display_name DROP DEFAULT`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
