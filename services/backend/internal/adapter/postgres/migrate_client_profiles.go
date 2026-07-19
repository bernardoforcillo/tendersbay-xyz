package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateClientProfiles is the 0006 schema migration for the per-client
// bid-qualification agent (v1.0). One table, PK = FK to workspaces.id — no
// extra index needed (the primary key already indexes the only lookup path,
// unlike the child tables in earlier migrations that index a plain FK column).
func migrateClientProfiles() pg.Migration {
	return pg.Migration{
		Version: "0006",
		Name:    "client_profiles",
		Up: func(ctx context.Context, db *pg.DB) error {
			_, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(ClientProfiles))
			return err
		},
		Down: func(ctx context.Context, db *pg.DB) error {
			_, err := db.ExecExpr(ctx, pg.DropTableIfExists(ClientProfiles))
			return err
		},
	}
}
