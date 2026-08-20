package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateReminders is the 0012 migration for deadline reminders. It adds the
// per-bid watermark column and creates the per-user reminder_preferences table.
//
// The column is an ALTER and not a table declaration for the same reason
// migrateBidDecision (0011) is: schema.go's Bids handle already carries it, so a
// FRESH database gets it at 0009-creation time via CreateTableIfNotExists and
// this migration only matters for a database that already ran 0009.
//
// last_reminded_bucket is NOT NULL DEFAULT 0, so every bid that existed before
// reminders did reads back as "never reminded" — which is true, and which means
// the first pass after deploy treats the whole existing portfolio as fresh.
// That is deliberate but it is not free: every open bid inside 14 days of its
// deadline earns a mail on that first run. It is the correct behaviour (those
// deadlines are real and the user has not been told), but it is a burst, and
// whoever deploys this should expect it rather than discover it.
func migrateReminders() pg.Migration {
	return pg.Migration{
		Version: "0012",
		Name:    "reminders",
		Up: func(ctx context.Context, db *pg.DB) error {
			if _, err := db.Exec(ctx,
				`ALTER TABLE bids ADD COLUMN IF NOT EXISTS last_reminded_bucket BIGINT NOT NULL DEFAULT 0`,
			); err != nil {
				return err
			}
			if _, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(ReminderPrefs)); err != nil {
				return err
			}
			// No explicit index on unsubscribe_token. The unsubscribe endpoint
			// queries by it, unauthenticated, so it must never be a sequential
			// scan — but the column is declared .Unique() in schema.go and
			// Postgres backs a UNIQUE constraint with an index already. Adding
			// one here produced a verified duplicate
			// (reminder_preferences_unsubscribe_token_key AND uq_reminder_prefs_token),
			// paying for two index writes per row to answer one query.
			return nil
		},
		Down: func(ctx context.Context, db *pg.DB) error {
			if _, err := db.Exec(ctx, `DROP TABLE IF EXISTS reminder_preferences`); err != nil {
				return err
			}
			_, err := db.Exec(ctx, `ALTER TABLE bids DROP COLUMN IF EXISTS last_reminded_bucket`)
			return err
		},
	}
}
