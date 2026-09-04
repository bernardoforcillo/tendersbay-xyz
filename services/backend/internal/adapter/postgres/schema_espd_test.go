package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

// TestEspdTablesRenderColumns renders the CREATE TABLE DDL for the nine 0016
// tables straight from the drops handles (no DB needed) and asserts the columns
// the repositories rely on are present.
func TestEspdTablesRenderColumns(t *testing.T) {
	tests := []struct {
		table   *pg.Table
		columns []string
	}{
		{CompanyRepresentatives, []string{"company_representatives", "workspace_id", "role", "given_name", "family_name", "birth_date", "birth_place", "address", "email", "power_of_attorney"}},
		{CompanyDeclarations, []string{"company_declarations", "workspace_id", "criterion", "answer", "self_cleaning"}},
		{CompanyNationalGrounds, []string{"company_national_grounds", "workspace_id", "country", "criterion", "answer", "note"}},
		{BidLots, []string{"bid_lots", "bid_id", "lot_ref", "position"}},
		{BidSubcontractors, []string{"bid_subcontractors", "bid_id", "name", "vat", "country", "share"}},
		{BidReliances, []string{"bid_reliances", "bid_id", "entity_name", "vat", "criterion"}},
		{BidDeclarationConfirmations, []string{"bid_declaration_confirmations", "bid_id", "user_id", "confirmed_at", "declarations_hash"}},
		{BidEspdRequests, []string{"bid_espd_requests", "bid_id", "version", "xml", "sha256", "imported_by", "imported_at"}},
		{BidEspdExports, []string{"bid_espd_exports", "bid_id", "exported_by", "version", "format", "content_sha256", "declarations_confirmed_at", "exported_at"}},
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

// TestEspdDossierTablesCarryAttribution: the three new dossier sections are
// facts about the company like the five before them, so they carry the same
// per-fact provenance columns with provenance NOT NULL.
func TestEspdDossierTablesCarryAttribution(t *testing.T) {
	attributionColumns := []string{"provenance", "confidence", "stated_by", "stated_at", "prompted_by", "prompted_by_tender_id", "source_note"}
	for _, table := range []*pg.Table{CompanyRepresentatives, CompanyDeclarations, CompanyNationalGrounds} {
		sql, _ := drops.String(pg.CreateTableIfNotExists(table))
		for _, col := range attributionColumns {
			if !strings.Contains(sql, col) {
				t.Errorf("%s is missing the attribution column %q", table.Name(), col)
			}
		}
		if !strings.Contains(sql, "provenance text NOT NULL") && !strings.Contains(sql, `"provenance" text NOT NULL`) {
			t.Errorf("%s must declare provenance NOT NULL: %s", table.Name(), sql)
		}
	}
}

// TestEspdBidTablesCarryNoProvenance pins the opposite decision for per-bid
// data: lots, parties and confirmations are the operator's choices for one
// gara, not facts a buyer can demand a certificate for, and a provenance column
// there would invite the engine to treat them as evidence.
func TestEspdBidTablesCarryNoProvenance(t *testing.T) {
	for _, table := range []*pg.Table{BidLots, BidSubcontractors, BidReliances, BidDeclarationConfirmations} {
		sql, _ := drops.String(pg.CreateTableIfNotExists(table))
		if strings.Contains(sql, "provenance") {
			t.Errorf("%s must not carry a provenance column: %s", table.Name(), sql)
		}
	}
}

// TestEspdChildTablesCascade: every 0016 table hangs off a workspace or a bid
// with ON DELETE CASCADE, which is what makes "gone with the workspace" (the
// PII rule) and "gone with the bid" schema facts rather than code paths.
func TestEspdChildTablesCascade(t *testing.T) {
	parents := map[*pg.Table]string{
		CompanyRepresentatives: "workspaces", CompanyDeclarations: "workspaces", CompanyNationalGrounds: "workspaces",
		BidLots: "bids", BidSubcontractors: "bids", BidReliances: "bids",
		BidDeclarationConfirmations: "bids", BidEspdRequests: "bids", BidEspdExports: "bids",
	}
	for table, parent := range parents {
		sql, _ := drops.String(pg.CreateTableIfNotExists(table))
		if !strings.Contains(sql, "REFERENCES") || !strings.Contains(sql, parent) || !strings.Contains(sql, "ON DELETE CASCADE") {
			t.Errorf("%s must reference %s ON DELETE CASCADE: %s", table.Name(), parent, sql)
		}
	}
}

// TestEspdSingletonsKeyOnBid: the confirmation and the request are one-per-bid
// by primary key, so a second row is unrepresentable rather than disallowed.
func TestEspdSingletonsKeyOnBid(t *testing.T) {
	for _, table := range []*pg.Table{BidDeclarationConfirmations, BidEspdRequests} {
		sql, _ := drops.String(pg.CreateTableIfNotExists(table))
		pk := strings.Index(sql, "PRIMARY KEY")
		if pk < 0 || !strings.Contains(sql[:pk], "bid_id") {
			t.Errorf("%s must have bid_id as its primary key: %s", table.Name(), sql)
		}
	}
}

// TestEspdExportsCarryNoBytes: the audit table stores a content hash, never
// the artefact.
func TestEspdExportsCarryNoBytes(t *testing.T) {
	sql, _ := drops.String(pg.CreateTableIfNotExists(BidEspdExports))
	for _, forbidden := range []string{"bytea", "content text", "xml", "pdf"} {
		if strings.Contains(sql, forbidden) {
			t.Errorf("bid_espd_exports must not store the artefact (%q): %s", forbidden, sql)
		}
	}
}

// TestWorkspaceCompaniesHasIsSME: the 0016 column is on the drops handle, so
// UpsertIdentity's full replace writes it and RETURNING reads it back.
func TestWorkspaceCompaniesHasIsSME(t *testing.T) {
	sql, _ := drops.String(pg.CreateTableIfNotExists(WorkspaceCompanies))
	if !strings.Contains(sql, "is_sme boolean NOT NULL DEFAULT false") && !strings.Contains(sql, `"is_sme" boolean NOT NULL DEFAULT false`) {
		t.Errorf("workspace_companies must declare is_sme boolean NOT NULL DEFAULT false: %s", sql)
	}
}
