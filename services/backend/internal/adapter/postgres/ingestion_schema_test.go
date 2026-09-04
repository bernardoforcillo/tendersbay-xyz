package postgres_test

import (
	"context"
	"database/sql"
	"testing"
)

// The tenders schema is owned and migrated by services/ingestion, not by this
// service. postgres.New runs the BACKEND's migrations only, so a database that
// has never had the ingestion service pointed at it has every public table this
// module needs and none of the tenders.* ones the search, document, detail and
// eForms repositories read.
//
// Without this check that shows up as eight or so tests failing with a raw
// SQLSTATE 42P01, which reads like the repository is broken. It is not: the
// database is half-migrated, and the fix is one command. Say so instead.
//
// It is a skip rather than a failure because the missing half belongs to
// another service: this suite has no business failing over a schema it does not
// own and does not create. CI never reaches either branch — no database is
// provisioned there, so every test in this file's package skips one step
// earlier, at TEST_DATABASE_URL.
const ingestionSchemaHint = `the tenders schema is missing from TEST_DATABASE_URL.
It belongs to services/ingestion, which migrates it on start-up; the quickest way
to create it against this database is:

    cd services/ingestion && TEST_DATABASE_URL="$TEST_DATABASE_URL" go test ./internal/adapter/postgres/ -run TestNew

(cpv_lexicon_test.go additionally needs the vocabulary seeded: go run ./cmd/seed-cpv)`

// requireIngestionSchema skips the calling test unless the tenders schema this
// service reads from has been migrated into the database under test.
func requireIngestionSchema(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	var present bool
	if err := sqlDB.QueryRowContext(context.Background(),
		`SELECT to_regclass('tenders.ingested_tenders') IS NOT NULL`).Scan(&present); err != nil {
		t.Fatalf("checking for the ingestion schema: %v", err)
	}
	if !present {
		t.Skip(ingestionSchemaHint)
	}
}
