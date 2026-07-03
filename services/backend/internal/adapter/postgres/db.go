package postgres

import (
	"context"
	"database/sql"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/drops/stdlib"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/migrations"
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
	return db, sqlDB, m.Up(ctx)
}
