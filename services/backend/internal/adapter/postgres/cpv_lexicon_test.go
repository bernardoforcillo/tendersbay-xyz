package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
)

func testCPVLexicon(t *testing.T) *postgres.CPVLexicon {
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
	requireIngestionSchema(t, sqlDB)
	return postgres.NewCPVLexicon(db)
}

func TestMatchCodes_ResolvesAnItalianQueryToACodeSharedWithGerman(t *testing.T) {
	// This is the premise of the whole phase: the code an Italian phrase
	// resolves to is the same code a German notice carries.
	lex := testCPVLexicon(t)
	matches, err := lex.MatchCodes(context.Background(), "pulizie uffici", 8)
	if err != nil {
		t.Fatalf("MatchCodes: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("MatchCodes returned nothing — is cpv_terms seeded? (go run ./cmd/seed-cpv)")
	}
	var sawCleaning bool
	for _, m := range matches {
		if len(m.Code) == 8 && m.Code[:4] == "9091" {
			sawCleaning = true
		}
		if m.Label == "" || m.Lang == "" {
			t.Errorf("match %+v has label=%q lang=%q, want both non-empty — the UI needs both to show the user what their query was understood as, and a signal they can't see is one they can't correct", m, m.Label, m.Lang)
		}
	}
	if !sawCleaning {
		t.Errorf("matches = %+v, want a 9091xxxx cleaning-services code among them — this is the premise of the whole phase: the Italian phrase must resolve to the same code a German (or any other language's) notice carries", matches)
	}
}

func TestMatchCodes_ReturnsNothingForNonsense(t *testing.T) {
	// A query that resolves to no code must contribute no arm at all rather than
	// falling back to something arbitrary.
	lex := testCPVLexicon(t)
	matches, err := lex.MatchCodes(context.Background(), "zzqxwvk", 8)
	if err != nil {
		t.Fatalf("MatchCodes: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("matches = %+v, want none — a query that resolves to no code must contribute no arm at all rather than falling back to something arbitrary", matches)
	}
}

func TestMatchCodes_HonoursTheLimit(t *testing.T) {
	lex := testCPVLexicon(t)
	matches, err := lex.MatchCodes(context.Background(), "servizi", 3)
	if err != nil {
		t.Fatalf("MatchCodes: %v", err)
	}
	if len(matches) > 3 {
		t.Errorf("len(matches) = %d, want at most 3 — exceeding the caller's requested limit means LIMIT was dropped or miscounted in the SQL, and callers rely on it to cap what they render", len(matches))
	}
}

func TestMatchCodes_IsDeterministic(t *testing.T) {
	lex := testCPVLexicon(t)
	ctx := context.Background()
	first, err := lex.MatchCodes(ctx, "costruzione strade", 8)
	if err != nil {
		t.Fatalf("MatchCodes: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := lex.MatchCodes(ctx, "costruzione strade", 8)
		if err != nil {
			t.Fatalf("MatchCodes: %v", err)
		}
		if len(again) != len(first) {
			t.Fatalf("run %d returned %d matches, first returned %d — repeated identical queries must return the same result SET, or the per-position comparison below is meaningless", i, len(again), len(first))
		}
		for j := range first {
			if again[j].Code != first[j].Code {
				t.Fatalf("run %d position %d = %s, first = %s — the ordering is not total", i, j, again[j].Code, first[j].Code)
			}
		}
	}
}
