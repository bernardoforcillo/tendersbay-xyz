package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
)

func testClientProfileRepo(t *testing.T) (*postgres.ClientProfileRepo, *sql.DB) {
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
	return postgres.NewClientProfileRepo(db), sqlDB
}

func seedWorkspaceForClientProfile(t *testing.T, sqlDB *sql.DB) string {
	t.Helper()
	ctx := context.Background()

	var ownerID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ('client-profile-test@example.com', 'x', 'Client Profile Test User')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		 RETURNING id`,
	).Scan(&ownerID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM users WHERE id = $1`, ownerID) })

	var workspaceID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO workspaces (name, slug, owner_id) VALUES ('Client Profile Test WS', 'cp-test-ws-1', $1) RETURNING id`,
		ownerID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() { _, _ = sqlDB.Exec(`DELETE FROM workspaces WHERE id = $1`, workspaceID) })
	return workspaceID
}

func TestClientProfileRepo_Get_ReturnsErrProfileNotFoundWhenNoRow(t *testing.T) {
	repo, sqlDB := testClientProfileRepo(t)
	workspaceID := seedWorkspaceForClientProfile(t, sqlDB)

	_, err := repo.Get(context.Background(), workspaceID)
	if !errors.Is(err, clientprofile.ErrProfileNotFound) {
		t.Fatalf("Get error = %v, want ErrProfileNotFound", err)
	}
}

func TestClientProfileRepo_Upsert_CreatesThenUpdatesAndClearsValueBand(t *testing.T) {
	repo, sqlDB := testClientProfileRepo(t)
	workspaceID := seedWorkspaceForClientProfile(t, sqlDB)
	ctx := context.Background()

	min, max := int64(100_000), int64(500_000)
	created, err := repo.Upsert(ctx, clientprofile.Profile{
		WorkspaceID: workspaceID,
		Sectors:     []string{"45", "72"},
		Countries:   []string{"IT"},
		ValueMin:    &min,
		ValueMax:    &max,
		Notes:       "first pass",
	})
	if err != nil {
		t.Fatalf("Upsert (create): %v", err)
	}
	if len(created.Sectors) != 2 || created.Sectors[1] != "72" {
		t.Fatalf("created.Sectors = %v", created.Sectors)
	}

	got, err := repo.Get(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Get after create: %v", err)
	}
	if got.ValueMin == nil || *got.ValueMin != min {
		t.Fatalf("got.ValueMin = %v, want %d", got.ValueMin, min)
	}

	// Second Upsert must fully replace the row, including clearing ValueMax
	// back to NULL (nil) — an Update is a full replace, not a partial patch.
	updated, err := repo.Upsert(ctx, clientprofile.Profile{
		WorkspaceID: workspaceID,
		Sectors:     []string{"80"},
		Countries:   []string{"DE", "FR"},
		ValueMin:    &min,
		ValueMax:    nil,
		Notes:       "second pass",
	})
	if err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}
	if len(updated.Sectors) != 1 || updated.Sectors[0] != "80" {
		t.Fatalf("updated.Sectors = %v", updated.Sectors)
	}
	if updated.ValueMax != nil {
		t.Fatalf("updated.ValueMax = %v, want nil (cleared)", updated.ValueMax)
	}
	if updated.Notes != "second pass" {
		t.Fatalf("updated.Notes = %q", updated.Notes)
	}
}

// TestClientProfileRepo_Upsert_RoundTripsRegionsAndProcedureTypes covers the
// Task 3 delta: Regions and ProcedureTypes must be handled identically to
// Sectors/Countries — round-tripped through Upsert then Get, not silently
// dropped or left as an empty slice.
func TestClientProfileRepo_Upsert_RoundTripsRegionsAndProcedureTypes(t *testing.T) {
	repo, sqlDB := testClientProfileRepo(t)
	workspaceID := seedWorkspaceForClientProfile(t, sqlDB)
	ctx := context.Background()

	created, err := repo.Upsert(ctx, clientprofile.Profile{
		WorkspaceID:    workspaceID,
		Sectors:        []string{"45"},
		Countries:      []string{"IT"},
		Regions:        []string{"ITC", "DE3"},
		ProcedureTypes: []string{"open", "restricted"},
		Notes:          "regions and procedure types",
	})
	if err != nil {
		t.Fatalf("Upsert (create): %v", err)
	}
	if len(created.Regions) != 2 || created.Regions[0] != "ITC" || created.Regions[1] != "DE3" {
		t.Fatalf("created.Regions = %v", created.Regions)
	}
	if len(created.ProcedureTypes) != 2 || created.ProcedureTypes[0] != "open" || created.ProcedureTypes[1] != "restricted" {
		t.Fatalf("created.ProcedureTypes = %v", created.ProcedureTypes)
	}

	got, err := repo.Get(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Get after create: %v", err)
	}
	if len(got.Regions) != 2 || got.Regions[0] != "ITC" || got.Regions[1] != "DE3" {
		t.Fatalf("got.Regions = %v", got.Regions)
	}
	if len(got.ProcedureTypes) != 2 || got.ProcedureTypes[0] != "open" || got.ProcedureTypes[1] != "restricted" {
		t.Fatalf("got.ProcedureTypes = %v", got.ProcedureTypes)
	}

	// A second Upsert with different Regions/ProcedureTypes must fully
	// replace them too — same full-replace discipline as Sectors/Countries.
	updated, err := repo.Upsert(ctx, clientprofile.Profile{
		WorkspaceID:    workspaceID,
		Sectors:        []string{"45"},
		Countries:      []string{"IT"},
		Regions:        []string{"FR1"},
		ProcedureTypes: []string{"negotiated"},
		Notes:          "regions and procedure types updated",
	})
	if err != nil {
		t.Fatalf("Upsert (update): %v", err)
	}
	if len(updated.Regions) != 1 || updated.Regions[0] != "FR1" {
		t.Fatalf("updated.Regions = %v", updated.Regions)
	}
	if len(updated.ProcedureTypes) != 1 || updated.ProcedureTypes[0] != "negotiated" {
		t.Fatalf("updated.ProcedureTypes = %v", updated.ProcedureTypes)
	}
}
