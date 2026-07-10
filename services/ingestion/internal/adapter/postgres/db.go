// Package postgres is the driven adapter storing ingested tenders in the
// dedicated `tenders` Postgres schema.
package postgres

import (
	"context"
	"database/sql"

	"github.com/bernardoforcillo/drops/pg"
	dropsstdlib "github.com/bernardoforcillo/drops/stdlib"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/migrations"
	"github.com/jackc/pgx/v5"
	pgxstdlib "github.com/jackc/pgx/v5/stdlib"
)

// New opens a PostgreSQL connection scoped to the `tenders` schema, creates
// the schema if missing, runs pending migrations, and returns both the drops
// DB and the underlying *sql.DB.
//
// Schema placement can't go through drops' Migrator.WithTable (it can only
// rename the ledger table within whatever schema the connection defaults to
// — quoteIdent wraps a "schema.table" string as one literal identifier, not
// a qualified name; see the design doc). Instead, `tenders` is baked into
// the connection's search_path at the pgx config level, so every physical
// connection the pool opens — including ones opened concurrently by
// Service.RunOnce's provider fan-out — resolves unqualified names into
// `tenders` from the start. A one-off `SET search_path` after connecting
// would not be safe here: database/sql may open more than one physical
// connection for concurrent queries, and a session-level SET only affects
// the connection it ran on.
func New(ctx context.Context, dsn string) (*pg.DB, *sql.DB, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, nil, err
	}
	cfg.RuntimeParams["search_path"] = "tenders,public"
	connStr := pgxstdlib.RegisterConnConfig(cfg)

	sqlDB, err := sql.Open("pgx", connStr)
	if err != nil {
		return nil, nil, err
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, nil, err
	}
	if _, err := sqlDB.ExecContext(ctx, `CREATE SCHEMA IF NOT EXISTS tenders`); err != nil {
		return nil, nil, err
	}

	db := pg.New(dropsstdlib.New(sqlDB))
	m := pg.NewMigrator(db) // default ledger table name — isolated by schema, not by name
	if err := m.AddFS(migrations.Files, "."); err != nil {
		return nil, nil, err
	}
	return db, sqlDB, m.Up(ctx)
}
