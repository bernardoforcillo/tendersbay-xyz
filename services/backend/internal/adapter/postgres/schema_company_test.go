package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

// TestCompanyTablesRenderColumns renders the CREATE TABLE DDL for all seven
// company tables straight from the drops handles (no DB needed) and asserts the
// columns the repositories and the migration rely on are present.
func TestCompanyTablesRenderColumns(t *testing.T) {
	tests := []struct {
		table   *pg.Table
		columns []string
	}{
		{WorkspaceCompanies, []string{
			"workspace_companies", "workspace_id", "legal_name", "vat_number", "fiscal_code",
			"legal_form", "cciaa_office", "cciaa_number", "country", "nuts", "founded_year",
			"attribution", "updated_at",
		}},
		{CompanySOACategories, []string{
			"company_soa_categories", "workspace_id", "category", "classifica", "issued_by",
			"valid_until", "verified_until",
		}},
		{CompanyCertifications, []string{
			"company_certifications", "standard", "standard_raw", "scope", "issued_by",
			"valid_from", "valid_until",
		}},
		{CompanyFinancialYears, []string{
			"company_financial_years", "year", "turnover_minor", "specific_turnover_minor",
			"currency", "headcount",
		}},
		{CompanyPastContracts, []string{
			"company_past_contracts", "description", "buyer_name", "cpv", "value_minor",
			"currency", "started_on", "ended_on", "role", "share_pct",
		}},
		{CompanyRegistrations, []string{
			"company_registrations", "kind", "authority", "identifier", "section",
			"valid_from", "valid_until",
		}},
		{TenderRequirements, []string{
			"tender_requirements", "workspace_id", "tender_id", "lot_ref", "kind", "payload",
			"requirement_text", "blocking", "source", "excerpt", "citation", "dedupe_key",
			"confirmed_by", "confirmed_at", "created_at", "updated_at",
		}},
	}
	for _, tt := range tests {
		sql, _ := drops.String(pg.CreateTableIfNotExists(tt.table))
		for _, col := range tt.columns {
			if !strings.Contains(sql, col) {
				t.Errorf("%s DDL missing %q: %s", tt.table.Name(), col, sql)
			}
		}
	}
}

// TestCompanyChildTablesCarryAttribution asserts every repeated record can say
// WHO asserted it. A child table that shipped without these columns would make
// provenance unrepresentable for that record kind, and the engine's whole
// authoritative/inferred asymmetry silently inapplicable to it.
func TestCompanyChildTablesCarryAttribution(t *testing.T) {
	attributionColumns := []string{
		"provenance", "confidence", "stated_by", "stated_at",
		"prompted_by", "prompted_by_tender_id", "source_note",
	}
	for _, table := range []*pg.Table{
		CompanySOACategories, CompanyCertifications, CompanyFinancialYears,
		CompanyPastContracts, CompanyRegistrations,
	} {
		sql, _ := drops.String(pg.CreateTableIfNotExists(table))
		for _, col := range attributionColumns {
			if !strings.Contains(sql, col) {
				t.Errorf("%s is missing the attribution column %q", table.Name(), col)
			}
		}
		// provenance must be NOT NULL: an un-attributed fact has to be
		// impossible to insert, which is precisely what a JSON key could not
		// enforce.
		if !strings.Contains(sql, "provenance text NOT NULL") && !strings.Contains(sql, `"provenance" text NOT NULL`) {
			t.Errorf("%s must declare provenance NOT NULL: %s", table.Name(), sql)
		}
	}
}

// TestTenderRequirementsLotRefNotNull guards the reason lot_ref is NOT NULL
// DEFAULT the empty string: PostgreSQL NULLs do not conflict in a unique key, so a nullable
// column would permit unlimited duplicate notice-level rows despite the
// (workspace_id, tender_id, lot_ref, dedupe_key) unique.
func TestTenderRequirementsLotRefNotNull(t *testing.T) {
	sql, _ := drops.String(pg.CreateTableIfNotExists(TenderRequirements))
	idx := strings.Index(sql, "lot_ref")
	if idx < 0 {
		t.Fatalf("lot_ref missing from the DDL: %s", sql)
	}
	segment := sql[idx:]
	if end := strings.Index(segment, ","); end > 0 {
		segment = segment[:end]
	}
	if !strings.Contains(segment, "NOT NULL") {
		t.Errorf("lot_ref must be NOT NULL: %q", segment)
	}
}

// TestWorkspaceCompaniesIsOnePerWorkspace asserts the one-company-per-workspace
// rule is enforced by the SCHEMA and not merely by code: the primary key IS the
// foreign key to workspaces.id, so a second company row in one workspace is
// unrepresentable rather than just disallowed.
func TestWorkspaceCompaniesIsOnePerWorkspace(t *testing.T) {
	sql, _ := drops.String(pg.CreateTableIfNotExists(WorkspaceCompanies))
	if !strings.Contains(sql, "PRIMARY KEY") {
		t.Fatalf("workspace_companies must declare a primary key: %s", sql)
	}
	pkIdx := strings.Index(sql, "PRIMARY KEY")
	// The primary key clause must sit on the workspace_id column, which is the
	// first column declared.
	head := sql[:pkIdx]
	if !strings.Contains(head, "workspace_id") {
		t.Fatalf("the primary key must be workspace_id: %s", sql)
	}
	if !strings.Contains(sql, "REFERENCES") || !strings.Contains(sql, "workspaces") {
		t.Fatalf("workspace_companies.workspace_id must reference workspaces: %s", sql)
	}
	// A separate company id would make a second company per workspace
	// representable, which is exactly what the PRD settled against.
	if strings.Contains(sql, "company_id") {
		t.Errorf("workspace_companies must not carry a company id: %s", sql)
	}
}

// TestTenderRequirementsHasNoTenderFK: tenders.ingested_tenders is owned and
// migrated by services/ingestion, so tender_id is a plain bigint — the same rule
// bids.tender_id follows. A foreign key here would make this service's migration
// depend on another service's schema.
func TestTenderRequirementsHasNoTenderFK(t *testing.T) {
	sql, _ := drops.String(pg.CreateTableIfNotExists(TenderRequirements))
	idx := strings.Index(sql, "tender_id bigint")
	if idx < 0 {
		idx = strings.Index(sql, `"tender_id" bigint`)
	}
	if idx < 0 {
		t.Fatalf("tender_id column missing: %s", sql)
	}
	segment := sql[idx:]
	if end := strings.Index(segment, ","); end > 0 {
		segment = segment[:end]
	}
	if strings.Contains(segment, "REFERENCES") {
		t.Errorf("tender_id must carry no foreign key: %q", segment)
	}
}
