package postgres

import (
	"context"
	"database/sql"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/drops/stdlib"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// New opens a PostgreSQL connection, pings it, runs pending migrations, and
// returns both the drops DB and the underlying *sql.DB for health probes.
func New(ctx context.Context, dsn string) (*pg.DB, *sql.DB, error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, nil, err
	}
	db := pg.New(stdlib.New(sqlDB))
	m := pg.NewMigrator(db)
	if err := m.AddFS(migrations.Files, "."); err != nil {
		return nil, nil, err
	}
	// 0002+ are managed programmatically via the drops schema DSL (see
	// migrate_workspaces.go), mixed with the FS-based 0001 by version order.
	m.Add(migrateWorkspaces())
	m.Add(migrateWorkbenches())
	m.Add(migrateAgent())
	m.Add(migrateAgentCreditsBackfill())
	return db, sqlDB, m.Up(ctx)
}
