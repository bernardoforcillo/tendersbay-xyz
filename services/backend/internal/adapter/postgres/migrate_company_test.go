package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

func TestMigrateCompanyVersion(t *testing.T) {
	m := migrateCompany()
	if m.Version != "0010" {
		t.Errorf("migrateCompany version = %q, want %q", m.Version, "0010")
	}
	if m.Name != "company_dossier" {
		t.Errorf("migrateCompany name = %q, want %q", m.Name, "company_dossier")
	}
}

// TestCompanyIndexesRenderUnqualifiedColumns guards the 0010 indexes against
// the same regression class as 0002/0003/0009: an index column must render as a
// bare identifier or PostgreSQL rejects CREATE INDEX with SQLSTATE 42601.
func TestCompanyIndexesRenderUnqualifiedColumns(t *testing.T) {
	for _, idx := range companyIndexes() {
		sql, _ := drops.String(pg.CreateIndexIfNotExists(idx))
		if strings.Contains(sql, `"."`) {
			t.Errorf("index %q renders a table-qualified column, which PostgreSQL rejects in a CREATE INDEX column list: %s", idx.Name(), sql)
		}
	}
}

// TestCompanyIndexesCoverEveryChildTable: the dossier read loads all five child
// collections by workspace_id in one call, so a child table without that index
// turns one page load into five sequential scans.
func TestCompanyIndexesCoverEveryChildTable(t *testing.T) {
	want := map[string]bool{
		"idx_company_soa_ws":                false,
		"idx_company_cert_ws":               false,
		"idx_company_fy_ws":                 false,
		"idx_company_past_ws":               false,
		"idx_company_reg_ws":                false,
		"idx_tender_requirements_ws_tender": false,
	}
	for _, idx := range companyIndexes() {
		if _, ok := want[idx.Name()]; !ok {
			t.Errorf("unexpected index %q", idx.Name())
			continue
		}
		want[idx.Name()] = true
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("missing index %q", name)
		}
	}
}
