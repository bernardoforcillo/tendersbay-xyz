package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateUserLocale is the 0013 migration: the user's chosen interface and
// email language.
//
// It defaults to ” and NOT to a language. Every existing user reads back as
// "never told us", which is exactly true — and it is what lets the reminder
// path tell an explicit English choice apart from a person nobody has asked
// yet. Defaulting to 'en-ie' would have been one line shorter and would have
// erased that distinction for the whole existing user base permanently.
func migrateUserLocale() pg.Migration {
	return pg.Migration{
		Version: "0013",
		Name:    "user_locale",
		Up: func(ctx context.Context, db *pg.DB) error {
			_, err := db.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS locale TEXT NOT NULL DEFAULT ''`)
			return err
		},
		Down: func(ctx context.Context, db *pg.DB) error {
			_, err := db.Exec(ctx, `ALTER TABLE users DROP COLUMN IF EXISTS locale`)
			return err
		},
	}
}
