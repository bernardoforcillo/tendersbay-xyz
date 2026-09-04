package postgres

import (
	"context"
	"database/sql"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/drops/stdlib"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/migrations"
	"github.com/jackc/pgx/v5"
	pgxstdlib "github.com/jackc/pgx/v5/stdlib"
)

// New opens a PostgreSQL connection, pings it, runs pending migrations, and
// returns both the drops DB and the underlying *sql.DB for health probes.
func New(ctx context.Context, dsn string) (*pg.DB, *sql.DB, error) {
	// search_path must include `tenders`: that's where services/ingestion
	// installs pg_trgm (see services/ingestion/internal/adapter/postgres/db.go),
	// and LexicalSearch's fuzzy arm (internal/adapter/postgres/tender_search.go)
	// uses the trigram `%` operator and similarity() unqualified — operators and
	// functions can't be schema-qualified in ordinary SQL, so without `tenders`
	// on the path Postgres can't resolve them at all. Every short query then
	// errors, hybrid.go's searchHybrid treats that as lexErr != nil, and
	// degrades silently to dense-only for essentially every real search.
	//
	// Unlike ingestion, `tenders` is deliberately placed AFTER `public`, not
	// before: this service owns its own unqualified tables (users, sessions,
	// workspaces, ...) that already live in `public`, and every tender row it
	// reads is already schema-qualified (`tenders.ingested_tenders`) in
	// tender_search.go — the schema is only needed to widen operator/function
	// resolution, not to relocate any table lookup. Putting `tenders` first,
	// mirroring ingestion verbatim, was tried and rejected: CREATE TABLE IF
	// NOT EXISTS only checks for an existing relation in the FIRST schema of
	// the search_path with CREATE privilege, not the whole path (verified
	// against a running instance) — so on the next migration run this
	// service's own migrator would silently create empty duplicate tables in
	// `tenders`, which would then shadow the real, data-filled `public` ones
	// on every subsequent unqualified read. `public,tenders` preserves
	// today's default resolution for this service's own schema unchanged and
	// only adds `tenders` for pg_trgm.
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, nil, err
	}
	cfg.RuntimeParams["search_path"] = "public,tenders"
	connStr := pgxstdlib.RegisterConnConfig(cfg)

	sqlDB, err := sql.Open("pgx", connStr)
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
	m.Add(migrateClientProfiles())
	m.Add(migrateAgentTenderResults())
	m.Add(migrateLowercaseEmails())
	m.Add(migrateBids())
	m.Add(migrateCompany())
	m.Add(migrateBidDecision())
	m.Add(migrateAuthlayer())
	m.Add(migrateWorkspaceScope())
	m.Add(migrateFeatures())
	m.Add(migrateWorkbenchScope())
	m.Add(migrateEspd())
	return db, sqlDB, m.Up(ctx)
}
