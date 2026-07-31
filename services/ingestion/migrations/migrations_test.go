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

func TestFilesEmbedsCPVVocabularyMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0007_cpv_vocabulary.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0007_cpv_vocabulary.up.sql not found in embedded migrations: %v", entries)
	}
}

// TestCPVVocabularyMigrationUsesImmutableTSVector guards 0007's generated
// column the same way TestHybridSearchMigrationUsesImmutableTSVector guards
// 0005's: a STORED generated column may only call IMMUTABLE functions, and
// to_tsvector is immutable only in its two-argument form. unaccent() is STABLE
// and must not appear — the usual workaround of wrapping it in a function
// falsely declared IMMUTABLE silently corrupts the index when the unaccent
// dictionary changes.
func TestCPVVocabularyMigrationUsesImmutableTSVector(t *testing.T) {
	body, err := fs.ReadFile(migrations.Files, "0007_cpv_vocabulary.up.sql")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	sql := string(body)
	if strings.Contains(sql, "unaccent") {
		t.Error("0007 calls unaccent, which is STABLE and cannot appear in a STORED generated column — the migration would fail to apply, or silently freeze stale accents if wrapped in a falsely-IMMUTABLE wrapper")
	}
	if !strings.Contains(sql, "to_tsvector('simple', label)") {
		t.Error("0007 missing the immutable two-argument tsvector expression over label — its absence means the generated column would fail to create or silently use a mutable configuration")
	}
	if !strings.Contains(sql, "PRIMARY KEY (code, lang)") {
		t.Error("0007 missing the (code, lang) primary key that makes the seed idempotent — without it, ON CONFLICT DO UPDATE in the seeder has no target and re-seeding duplicates rows")
	}
}

func TestFilesEmbedsCPVLabelExpansionMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0008_cpv_label_expansion.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0008_cpv_label_expansion.up.sql not found in embedded migrations: %v", entries)
	}
}

// TestCPVLabelExpansionRebuildsTheVectorImmutably guards the rebuilt
// search_vector the same way TestHybridSearchMigrationUsesImmutableTSVector
// guards the original: a STORED generated column may only call IMMUTABLE
// functions. It also pins the weight class the labels land in, because moving
// them to A or B would let a category label outrank a title match.
func TestCPVLabelExpansionRebuildsTheVectorImmutably(t *testing.T) {
	body, err := fs.ReadFile(migrations.Files, "0008_cpv_label_expansion.up.sql")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	sql := string(body)

	for _, forbidden := range []string{"array_to_string", "unaccent"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("0008 calls %s, which is STABLE and cannot appear in a STORED generated column", forbidden)
		}
	}
	for _, want := range []string{
		"to_tsvector('simple', coalesce(title, ''))",
		"to_tsvector('simple', coalesce(buyer_name, ''))",
		"to_tsvector('simple', coalesce(cpv, '') || ' ' || coalesce(cpv_labels, ''))",
		"to_tsvector('simple', coalesce(description, ''))",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("0008 missing immutable tsvector expression %q", want)
		}
	}
	// Dropping the column drops its index too; forgetting to recreate it turns
	// every lexical search into a sequential scan with no error to notice.
	if !strings.Contains(sql, "idx_ingested_tenders_search_vector") {
		t.Error("0008 drops and rebuilds search_vector but never recreates its GIN index")
	}
	if !strings.Contains(sql, "setweight(to_tsvector('simple', coalesce(cpv, '') || ' ' || coalesce(cpv_labels, '')), 'C')") {
		t.Error("0008 does not put cpv_labels in weight class C alongside the code")
	}
}
