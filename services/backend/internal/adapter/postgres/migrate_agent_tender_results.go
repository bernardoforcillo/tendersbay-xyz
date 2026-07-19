package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateAgentTenderResults is the 0007 migration. 0004 created chat_messages
// with choices/metadata JSONB columns; this adds a tenders JSONB column so a
// "tender_results" role message (the agent chat search-cards feature) can
// persist its structured tender list the same way choice_prompt persists its
// choices. schema.go's ChatMessages declaration already includes
// ChatMessageTenders as of this change, so a FRESH database gets the column
// at 0004-creation time via CreateTableIfNotExists — this migration only
// matters for a database that already ran 0004 before this column existed.
func migrateAgentTenderResults() pg.Migration {
	return pg.Migration{
		Version: "0007",
		Name:    "agent_tender_results",
		Up: func(ctx context.Context, db *pg.DB) error {
			_, err := db.Exec(ctx, `ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS tenders JSONB`)
			return err
		},
		Down: func(ctx context.Context, db *pg.DB) error {
			_, err := db.Exec(ctx, `ALTER TABLE chat_messages DROP COLUMN IF EXISTS tenders`)
			return err
		},
	}
}
