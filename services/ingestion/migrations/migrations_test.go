package migrations_test

import (
	"io/fs"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/migrations"
)

func TestFilesEmbedsInitMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0001_init.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0001_init.up.sql not found in embedded migrations: %v", entries)
	}
}
