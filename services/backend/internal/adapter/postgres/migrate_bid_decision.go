package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateBidDecision is the 0011 migration. It adds the four decision-record
// columns to `bids`: the eligibility recommendation a go/no-go was taken
// against, whether the decision contradicted it, how many blocking gaps stood at
// the time, and when the human decided.
//
// It is an ALTER and not a table declaration because schema.go's Bids handle
// already carries these columns as of this change: a FRESH database gets them at
// 0009-creation time via CreateTableIfNotExists, and this migration only matters
// for a database that already ran 0009 before they existed. That is the same
// shape — and the same reason — as migrateAgentTenderResults (0007).
//
// Every column is NOT NULL with a default, so the ALTER is safe on a populated
// table and every pre-existing bid reads back as "decided before we recorded a
// baseline": an empty recommendation (none existed), not overridden, and
// a NULL decision_recorded_at that says plainly we do not know when.
func migrateBidDecision() pg.Migration {
	columns := []string{
		`ADD COLUMN IF NOT EXISTS decision_recommendation TEXT NOT NULL DEFAULT ''`,
		`ADD COLUMN IF NOT EXISTS decision_overridden BOOLEAN NOT NULL DEFAULT false`,
		`ADD COLUMN IF NOT EXISTS decision_blocking_gaps BIGINT NOT NULL DEFAULT 0`,
		`ADD COLUMN IF NOT EXISTS decision_recorded_at TIMESTAMPTZ`,
	}
	dropped := []string{
		`DROP COLUMN IF EXISTS decision_recorded_at`,
		`DROP COLUMN IF EXISTS decision_blocking_gaps`,
		`DROP COLUMN IF EXISTS decision_overridden`,
		`DROP COLUMN IF EXISTS decision_recommendation`,
	}
	return pg.Migration{
		Version: "0011",
		Name:    "bid_decision_record",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, c := range columns {
				if _, err := db.Exec(ctx, `ALTER TABLE bids `+c); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(ctx context.Context, db *pg.DB) error {
			for _, c := range dropped {
				if _, err := db.Exec(ctx, `ALTER TABLE bids `+c); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
