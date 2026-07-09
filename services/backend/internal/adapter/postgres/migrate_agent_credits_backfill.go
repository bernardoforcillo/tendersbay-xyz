package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateAgentCreditsBackfill is the 0005 migration. 0004 seeded a
// workspace_credits row for every workspace that existed at the time it
// ran, but workspaces created afterward through workspace.Service.CreateWorkspace
// never got one (that gap is now closed going forward by credits.Service.Seed,
// called from the CreateWorkspace RPC handler). Any workspace created between
// 0004 and that fix has no workspace_credits row, so credits.Service.Check
// falls back to its zero-value CheckResult — the UI shows credits as 0/0. This
// re-runs 0004's same idempotent seed to backfill those workspaces.
func migrateAgentCreditsBackfill() pg.Migration {
	return pg.Migration{
		Version: "0005",
		Name:    "agent_credits_backfill",
		Up: func(ctx context.Context, db *pg.DB) error {
			_, err := db.Exec(ctx, `
				INSERT INTO workspace_credits (workspace_id)
				SELECT id FROM workspaces
				ON CONFLICT (workspace_id) DO NOTHING
			`)
			return err
		},
		Down: func(ctx context.Context, db *pg.DB) error {
			// No-op: this migration only backfills missing rows for
			// pre-existing workspaces; reversing it would delete legitimate
			// credit ledgers, which 0004's Down already handles by dropping
			// the whole table.
			return nil
		},
	}
}
