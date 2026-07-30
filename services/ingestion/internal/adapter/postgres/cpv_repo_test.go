package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/cpvdata"
)

func testCPVRepo(t *testing.T) (*postgres.CPVRepo, *sql.DB) {
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
	return postgres.NewCPVRepo(db), sqlDB
}

// cleanupTerms removes only the rows a test inserted, matching the house
// convention: tests share one long-lived database and must never truncate.
func cleanupTerms(t *testing.T, sqlDB *sql.DB, codes ...string) {
	t.Helper()
	t.Cleanup(func() {
		for _, code := range codes {
			_, _ = sqlDB.Exec(`DELETE FROM tenders.cpv_terms WHERE code = $1`, code)
		}
	})
}

func TestUpsertTerms_InsertsAndIsIdempotent(t *testing.T) {
	repo, sqlDB := testCPVRepo(t)
	ctx := context.Background()
	cleanupTerms(t, sqlDB, "99999901")

	rows := []cpvdata.Row{
		{Code: "99999901", Lang: "it", Label: "Servizi di prova"},
		{Code: "99999901", Lang: "de", Label: "Testdienstleistungen"},
	}
	if n, err := repo.UpsertTerms(ctx, rows); err != nil || n != 2 {
		t.Fatalf("UpsertTerms(rows) = (%d, %v), want (2, nil) — a batch write must report one row sent per input row with no error", n, err)
	}

	// Re-seeding must not duplicate: the (code, lang) primary key plus
	// ON CONFLICT DO UPDATE is what makes the seeder safe to re-run.
	if _, err := repo.UpsertTerms(ctx, rows); err != nil {
		t.Fatalf("second UpsertTerms: %v", err)
	}
	var n int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM tenders.cpv_terms WHERE code = $1`, "99999901").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("rows for 99999901 = %d, want 2 — the upsert duplicated instead of updating", n)
	}
}

func TestUpsertTerms_UpdatesAChangedLabelAndItsVector(t *testing.T) {
	repo, sqlDB := testCPVRepo(t)
	ctx := context.Background()
	cleanupTerms(t, sqlDB, "99999902")

	if _, err := repo.UpsertTerms(ctx, []cpvdata.Row{{Code: "99999902", Lang: "it", Label: "vecchia etichetta"}}); err != nil {
		t.Fatalf("UpsertTerms: %v", err)
	}
	if _, err := repo.UpsertTerms(ctx, []cpvdata.Row{{Code: "99999902", Lang: "it", Label: "nuova etichetta"}}); err != nil {
		t.Fatalf("UpsertTerms: %v", err)
	}

	// The generated column must follow the label — that is the whole reason it
	// is generated rather than written.
	var matches bool
	if err := sqlDB.QueryRow(
		`SELECT label_vector @@ to_tsquery('simple', 'nuova') FROM tenders.cpv_terms WHERE code = $1 AND lang = $2`,
		"99999902", "it",
	).Scan(&matches); err != nil {
		t.Fatalf("query vector: %v", err)
	}
	if !matches {
		t.Errorf("label_vector @@ to_tsquery('nuova') = %v, want true — the generated column must follow an updated label, not stay frozen on the stale one", matches)
	}
}

func TestCountTerms_ReportsTheWholeVocabulary(t *testing.T) {
	repo, sqlDB := testCPVRepo(t)
	ctx := context.Background()
	cleanupTerms(t, sqlDB, "99999903")

	before, err := repo.CountTerms(ctx)
	if err != nil {
		t.Fatalf("CountTerms: %v", err)
	}
	if _, err := repo.UpsertTerms(ctx, []cpvdata.Row{{Code: "99999903", Lang: "it", Label: "x"}}); err != nil {
		t.Fatalf("UpsertTerms: %v", err)
	}
	after, err := repo.CountTerms(ctx)
	if err != nil {
		t.Fatalf("CountTerms: %v", err)
	}
	if after != before+1 {
		t.Errorf("CountTerms after insert = %d, want %d — inserting one new row must advance the count by exactly one", after, before+1)
	}
}
