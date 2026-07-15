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

func TestFilesEmbedsIndexCountryMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0002_index_country.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0002_index_country.up.sql not found in embedded migrations: %v", entries)
	}
}

func TestFilesEmbedsSearchIndexingMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0003_search_indexing.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0003_search_indexing.up.sql not found in embedded migrations: %v", entries)
	}
}
