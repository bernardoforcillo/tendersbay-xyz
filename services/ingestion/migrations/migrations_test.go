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

func TestFilesEmbedsRevertCPVLabelExpansionMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0009_revert_cpv_label_expansion.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0009_revert_cpv_label_expansion.up.sql not found in embedded migrations: %v", entries)
	}
}

// sqlStatementsOnly strips '--' line comments, so a migration's own
// explanatory header — which legitimately has to NAME the thing it removed,
// in prose, to explain why — cannot trip a check for that name's absence from
// the executable SQL itself. Comment stripping is line-based (matching this
// repo's migrations, which only ever use '--' line comments, never /* */).
func sqlStatementsOnly(sql string) string {
	lines := strings.Split(sql, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// TestRevertCPVLabelExpansionRestoresThe0005VectorImmutably guards 0009's
// rebuilt search_vector the same way the 0005/0008 tests guard theirs: a
// STORED generated column may only call IMMUTABLE functions. It also pins
// cpv_labels OUT of the expression — the whole point of this migration is
// that the labels no longer participate in the indexed vector (see 0009's
// header comment: the toggle that let a caller narrow them back out wrapped
// the vector in ts_filter(), which is not the indexed expression and turned
// every lexical search into a sequential scan in production).
func TestRevertCPVLabelExpansionRestoresThe0005VectorImmutably(t *testing.T) {
	body, err := fs.ReadFile(migrations.Files, "0009_revert_cpv_label_expansion.up.sql")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	code := sqlStatementsOnly(string(body))

	for _, forbidden := range []string{"array_to_string", "unaccent", "cpv_labels", "ts_filter"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("0009's executable SQL references %q, want it gone — the whole point of this migration is that cpv_labels (and the ts_filter toggle built on it) no longer touch search_vector", forbidden)
		}
	}
	for _, want := range []string{
		"to_tsvector('simple', coalesce(title, ''))",
		"to_tsvector('simple', coalesce(buyer_name, ''))",
		"to_tsvector('simple', coalesce(cpv, ''))",
		"to_tsvector('simple', coalesce(description, ''))",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("0009 missing immutable tsvector expression %q — the vector must exactly match 0005's original shape", want)
		}
	}
	// Dropping the column drops its index too; forgetting to recreate it turns
	// every lexical search into a sequential scan with no error to notice —
	// the exact defect this migration exists to fix.
	if !strings.Contains(code, "idx_ingested_tenders_search_vector") {
		t.Error("0009 drops and rebuilds search_vector but never recreates its GIN index")
	}
	if !strings.Contains(code, "setweight(to_tsvector('simple', coalesce(cpv, '')), 'C')") {
		t.Error("0009 does not restore cpv alone (without cpv_labels) in weight class C")
	}
}

// TestRevertCPVLabelExpansionKeepsTheColumn guards the other half of the
// revert: cpv_labels the COLUMN must survive even though it is no longer
// indexed. It is cheap, already populated, and a future re-attempt at
// cross-language lexical matching would want it still sitting on the row —
// see 0009's header comment. Checked against the literal DROP COLUMN phrase
// (not a bare "cpv_labels" substring search) so the migration's own
// explanatory prose — which has to name the column to explain it is being
// kept — cannot trip this assertion.
func TestRevertCPVLabelExpansionKeepsTheColumn(t *testing.T) {
	body, err := fs.ReadFile(migrations.Files, "0009_revert_cpv_label_expansion.up.sql")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	code := sqlStatementsOnly(string(body))
	for _, dropped := range []string{"DROP COLUMN IF EXISTS cpv_labels", "DROP COLUMN cpv_labels"} {
		if strings.Contains(code, dropped) {
			t.Errorf("0009's executable SQL contains %q, want the cpv_labels column kept — only search_vector should be rebuilt", dropped)
		}
	}
}

func TestFilesEmbedsEFormsXMLDetailMigration(t *testing.T) {
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	var found bool
	for _, e := range entries {
		if e.Name() == "0010_eforms_xml_detail.up.sql" {
			found = true
		}
	}
	if !found {
		t.Fatalf("0010_eforms_xml_detail.up.sql not found in embedded migrations: %v", entries)
	}
}

// columnDefinition returns the line of a CREATE TABLE / ALTER TABLE body that
// defines the named column, matched on the column name as the first token so
// that "weight" does not also match "weight_raw". Returns "" when no such line
// exists, which the callers treat as the column being missing.
func columnDefinition(code, column string) string {
	for _, line := range strings.Split(code, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, column+" ") || strings.HasPrefix(trimmed, column+"\t") {
			return trimmed
		}
		// ALTER TABLE ... ADD COLUMN IF NOT EXISTS <column> <type>
		if i := strings.Index(trimmed, "IF NOT EXISTS "+column+" "); i >= 0 {
			return trimmed
		}
	}
	return ""
}

// TestEFormsXMLDetailKeepsAbsenceDistinguishable guards the single decision the
// rest of Phase 1a is built on: absence must stay distinguishable from zero.
//
// grid_usable is three-valued on purpose (NULL = not yet enriched or not a TED
// row, false = enriched with no weighted criterion, true = at least one weight)
// and award_criteria.weight is nullable because NULL is the COMMON case — only
// 18 of 50 sampled Italian cn-standard notices carry any weight at all, and
// only 63 of 87 criterion entries. A NOT NULL or a DEFAULT false on either
// column collapses "we have not looked" into "there is nothing there", which is
// the coverage-vs-cause conflation this phase exists to remove, and it would do
// so silently — every downstream query keeps returning rows, just the wrong
// ones. Asserted here because no runtime error would ever reveal it.
func TestEFormsXMLDetailKeepsAbsenceDistinguishable(t *testing.T) {
	body, err := fs.ReadFile(migrations.Files, "0010_eforms_xml_detail.up.sql")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	code := sqlStatementsOnly(string(body))

	for _, column := range []string{"grid_usable", "weight"} {
		def := columnDefinition(code, column)
		if def == "" {
			t.Errorf("0010's executable SQL never defines %q", column)
			continue
		}
		if strings.Contains(def, "NOT NULL") {
			t.Errorf("0010 declares %s NOT NULL (%q) — absence must stay representable", column, def)
		}
		if strings.Contains(def, "DEFAULT") {
			t.Errorf("0010 gives %s a DEFAULT (%q) — a default makes an unenriched row indistinguishable from an enriched one with nothing to report", column, def)
		}
	}

	// weight_raw is the other half of the pair: when the numeric parse gives up
	// on "30%" or "30,5" the published string must still survive.
	if def := columnDefinition(code, "weight_raw"); def == "" {
		t.Error("0010 defines weight without weight_raw — a parse failure would lose the published datum entirely")
	}

	// The queue index has to stay partial. A plain index on xml_fetched_at would
	// carry an entry per enriched row forever, when the set it serves is the one
	// that shrinks toward empty as the corpus drains.
	if !strings.Contains(code, "WHERE xml_fetched_at IS NULL") {
		t.Error("0010's idx_ingested_tenders_xml_pending is not partial on xml_fetched_at IS NULL")
	}
}

// TestEFormsXMLDetailBackfillsOnlyTEDRowsWithRaw guards the one-shot UPDATE at
// the bottom of 0010. It is what makes the enricher's queue resolvable at all —
// without it every pre-existing TED row lists as pending with no URL to fetch —
// but it reads `raw` with a TED-shaped path, and the other three sources
// (pl-bzp, fr-boamp, es-placsp) store an entirely different payload under the
// same column. Dropping either half of the WHERE clause would quietly write
// NULL over the whole table instead of touching only the rows the path applies
// to.
func TestEFormsXMLDetailBackfillsOnlyTEDRowsWithRaw(t *testing.T) {
	body, err := fs.ReadFile(migrations.Files, "0010_eforms_xml_detail.up.sql")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	code := sqlStatementsOnly(string(body))

	for _, want := range []string{
		"raw->>'publication-number'",
		"raw->'links'->'xml'->>'MUL'",
		"source = 'ted'",
		"raw IS NOT NULL",
	} {
		if !strings.Contains(code, want) {
			t.Errorf("0010's backfill missing %q — without it the pre-existing corpus is either unreachable or overwritten across every source", want)
		}
	}
}

// TestEFormsXMLDetailLeavesVersionAndHistoryAlone pins the decision recorded at
// length in 0010's header: version and history track status transitions only,
// and XML enrichment must not extend them. history is an unbounded jsonb on a
// row read on every fetch, and "the criteria changed between a call and its
// award" is the expected consequence of keying on procedure-identifier, not a
// notable event. xml_fetched_at, xml_status and publication_number already
// record when we looked, what we got, and which notice it came from. Asserted
// against the executable SQL only, so the header's own prose — which has to
// name both columns to explain why they are excluded — cannot trip it.
func TestEFormsXMLDetailLeavesVersionAndHistoryAlone(t *testing.T) {
	body, err := fs.ReadFile(migrations.Files, "0010_eforms_xml_detail.up.sql")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	code := sqlStatementsOnly(string(body))

	for _, forbidden := range []string{"version", "history"} {
		if strings.Contains(code, forbidden) {
			t.Errorf("0010's executable SQL touches %q — XML enrichment must not bump version or append to history", forbidden)
		}
	}
}
