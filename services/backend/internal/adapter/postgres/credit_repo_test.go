package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
)

func testCreditRepo(t *testing.T) (*postgres.WorkspaceCreditRepo, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, sqlDB, err := postgres.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return postgres.NewWorkspaceCreditRepo(db), sqlDB
}

func TestDeduct_RejectsOverCeilingAndLeavesRowUnchanged(t *testing.T) {
	repo, sqlDB := testCreditRepo(t)
	ctx := context.Background()

	// workspaces.owner_id has a real FK to users.id (ON DELETE RESTRICT) —
	// seed a user first, then the workspace referencing it.
	var ownerID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ('credit-test@example.com', 'x', 'Credit Test User')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM users WHERE id = $1`, ownerID) })

	var workspaceID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO workspaces (name, slug, owner_id) VALUES ('Credit Test WS', 'credit-test-ws-1', $1) RETURNING id`,
		ownerID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM workspaces WHERE id = $1`, workspaceID) })

	if _, err := repo.Upsert(ctx, workspaceID, 100); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if _, _, err := repo.Deduct(ctx, workspaceID, 95); err != nil {
		t.Fatalf("seed Deduct to 95/100: %v", err)
	}

	row, applied, err := repo.Deduct(ctx, workspaceID, 10)
	if err != nil {
		t.Fatalf("Deduct(10) over ceiling: %v", err)
	}
	if applied {
		t.Fatal("applied = true, want false (95+10 > 100 ceiling)")
	}
	if row.CurrentCycleTokens != 95 {
		t.Fatalf("CurrentCycleTokens = %d, want unchanged 95 (not partially applied)", row.CurrentCycleTokens)
	}

	row, applied, err = repo.Deduct(ctx, workspaceID, 5)
	if err != nil {
		t.Fatalf("Deduct(5) at ceiling: %v", err)
	}
	if !applied {
		t.Fatal("applied = false, want true (95+5 == 100 ceiling)")
	}
	if row.CurrentCycleTokens != 100 {
		t.Fatalf("CurrentCycleTokens = %d, want 100", row.CurrentCycleTokens)
	}
}
