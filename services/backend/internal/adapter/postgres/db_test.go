package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
)

func TestNew_RunsMigrations(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	db, _, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	_ = db
}
