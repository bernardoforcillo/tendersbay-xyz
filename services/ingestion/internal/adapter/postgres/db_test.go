package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
)

func TestNew_CreatesTendersSchemaAndMigrates(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	db, sqlDB, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	_ = db
	defer sqlDB.Close()

	var exists bool
	row := sqlDB.QueryRowContext(ctx,
		`SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = 'tenders')`)
	if err := row.Scan(&exists); err != nil {
		t.Fatalf("query information_schema.schemata: %v", err)
	}
	if !exists {
		t.Fatal("tenders schema was not created")
	}

	var leakedIntoPublic int
	row = sqlDB.QueryRowContext(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'ingested_tenders'`)
	if err := row.Scan(&leakedIntoPublic); err != nil {
		t.Fatalf("query information_schema.tables: %v", err)
	}
	if leakedIntoPublic != 0 {
		t.Fatal("ingested_tenders was created in public, want it only in tenders")
	}

	// Running New again must be a no-op on the already-applied migration.
	if _, _, err := postgres.New(ctx, dsn); err != nil {
		t.Fatalf("second postgres.New: %v", err)
	}
}
