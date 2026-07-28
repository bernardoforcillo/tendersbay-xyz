package migrations_test

import (
	"io/fs"
	"strings"
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

func TestFilesEmbedsAddDescriptionMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0004_add_description.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0004_add_description.up.sql not found in embedded migrations: %v", entries)
	}
}

func TestFilesEmbedsHybridSearchMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0005_hybrid_search.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0005_hybrid_search.up.sql not found in embedded migrations: %v", entries)
	}
}

func TestFilesEmbedsReindexEmbeddingsMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0006_reindex_embeddings.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0006_reindex_embeddings.up.sql not found in embedded migrations: %v", entries)
	}
}

// TestHybridSearchMigrationUsesImmutableTSVector guards the generated column
// in 0005: a STORED generated column may only call IMMUTABLE functions, and
// to_tsvector is only immutable in its two-argument (explicit regconfig)
// form. A refactor that drops the 'simple' argument, or reaches for
// array_to_string over cpv_secondary (STABLE, not IMMUTABLE), makes the
// migration fail at deploy time rather than in review — so assert the shape
// here instead.
func TestHybridSearchMigrationUsesImmutableTSVector(t *testing.T) {
	body, err := fs.ReadFile(migrations.Files, "0005_hybrid_search.up.sql")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	sql := string(body)
	if strings.Contains(sql, "array_to_string") {
		t.Error("0005 calls array_to_string, which is STABLE and cannot appear in a STORED generated column")
	}
	for _, want := range []string{
		"to_tsvector('simple', coalesce(title, ''))",
		"to_tsvector('simple', coalesce(buyer_name, ''))",
		"to_tsvector('simple', coalesce(cpv, ''))",
		"to_tsvector('simple', coalesce(description, ''))",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("0005 missing immutable tsvector expression %q", want)
		}
	}
}
