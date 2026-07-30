package postgres

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/cpvdata"
)

// buildUpsertValues is UpsertTerms' riskiest logic and, unlike everything
// else in this file, needs no database — code, lang and label are all
// `text`, so a misaligned bind would not error, it would silently seed a
// label under the wrong language, and CI never provisions a database to
// catch it via the DB-gated tests. Pinning the exact placeholder string
// (not just its length) catches a stride error; pinning the exact arg order
// catches a swapped column. Three rows exercise the n = i*3 stride past
// i == 0, with the third row landing on the last placeholder in the batch.
func TestBuildUpsertValues_PlacesPlaceholdersAndArgsInOrder(t *testing.T) {
	batch := []cpvdata.Row{
		{Code: "45000000", Lang: "en", Label: "Construction work"},
		{Code: "45000000", Lang: "it", Label: "Lavori di costruzione"},
		{Code: "45000000", Lang: "de", Label: "Bauarbeiten"},
	}

	placeholders, args := buildUpsertValues(batch)

	const want = "($1, $2, $3), ($4, $5, $6), ($7, $8, $9)"
	if placeholders != want {
		t.Errorf("placeholders = %q, want %q — a stride error would misalign every row after the first", placeholders, want)
	}

	wantArgs := []any{
		"45000000", "en", "Construction work",
		"45000000", "it", "Lavori di costruzione",
		"45000000", "de", "Bauarbeiten",
	}
	if len(args) != len(wantArgs) {
		t.Fatalf("len(args) = %d, want %d — one (code, lang, label) triple per row", len(args), len(wantArgs))
	}
	for i, want := range wantArgs {
		if args[i] != want {
			t.Errorf("args[%d] = %v, want %v — a swapped bind would seed a label under the wrong language and nothing would error", i, args[i], want)
		}
	}
}
