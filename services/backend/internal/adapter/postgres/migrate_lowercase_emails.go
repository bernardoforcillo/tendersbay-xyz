package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateLowercaseEmails is the 0008 data migration. The auth service now
// canonicalizes every email to trimmed-lowercase on write and read
// (auth.NormalizeEmail), so the existing UNIQUE(email) constraint reliably
// blocks case-variant duplicate accounts. This backfills any rows written
// before that normalization existed so they line up with the new invariant —
// safe pre-launch, where there are no case-variant collisions to reconcile.
//
// Only users.email is uniqueness-critical; email_verifications.new_email is
// normalized too so a pending verification confirms to the same canonical
// address the app would now store.
func migrateLowercaseEmails() pg.Migration {
	return pg.Migration{
		Version: "0008",
		Name:    "lowercase_emails",
		Up: func(ctx context.Context, db *pg.DB) error {
			if _, err := db.Exec(ctx, `UPDATE users SET email = lower(trim(email)) WHERE email <> lower(trim(email))`); err != nil {
				return err
			}
			_, err := db.Exec(ctx, `UPDATE email_verifications SET new_email = lower(trim(new_email)) WHERE new_email <> lower(trim(new_email))`)
			return err
		},
		// Down is a no-op: lowercasing is not reversible (the original casing is
		// gone) and there is nothing to undo structurally.
		Down: func(context.Context, *pg.DB) error { return nil },
	}
}
