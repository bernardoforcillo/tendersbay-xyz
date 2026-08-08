package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateCompany is the 0010 schema migration for the company dossier and the
// per-workspace tender requirements it is checked against.
//
// Every table it creates lives in `public` and belongs to THIS service. Nothing
// in Phase 2 writes to `tenders.*` — that schema is owned and migrated
// exclusively by services/ingestion (see the note at the foot of schema.go) —
// so no ingestion migration is needed, and none would be permitted.
//
// The four composite uniques below are raw ALTER TABLE because drops does not
// emit composites inline, the same reason migrate_bids.go states. Each is a
// modelling claim, not a defensive index:
//
//   - (workspace_id, category) on SOA — a company holds ONE attestation per
//     category at a time, so a re-answered question is an UPDATE, not a second
//     row asserting a contradictory classifica.
//   - (workspace_id, standard, scope) on certifications — a company genuinely
//     can hold ISO 9001 under two scopes, so scope is part of the identity.
//   - (workspace_id, year) on financial years — one esercizio per year.
//   - (workspace_id, tender_id, lot_ref, dedupe_key) on requirements — an agent
//     that re-extracts on every turn must update rows rather than multiply them.
func migrateCompany() pg.Migration {
	tables := []*pg.Table{
		WorkspaceCompanies,
		CompanySOACategories,
		CompanyCertifications,
		CompanyFinancialYears,
		CompanyPastContracts,
		CompanyRegistrations,
		TenderRequirements,
	}
	return pg.Migration{
		Version: "0010",
		Name:    "company_dossier",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, t := range tables {
				if _, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(t)); err != nil {
					return err
				}
			}
			for _, s := range []string{
				`ALTER TABLE company_soa_categories ADD CONSTRAINT uq_company_soa_ws_category UNIQUE (workspace_id, category)`,
				`ALTER TABLE company_certifications ADD CONSTRAINT uq_company_cert_ws_standard_scope UNIQUE (workspace_id, standard, scope)`,
				`ALTER TABLE company_financial_years ADD CONSTRAINT uq_company_fy_ws_year UNIQUE (workspace_id, year)`,
				`ALTER TABLE tender_requirements ADD CONSTRAINT uq_tender_req_ws_tender_lot_key UNIQUE (workspace_id, tender_id, lot_ref, dedupe_key)`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			for _, idx := range companyIndexes() {
				if _, err := db.ExecExpr(ctx, pg.CreateIndexIfNotExists(idx)); err != nil {
					return err
				}
			}
			return nil
		},
		Down: func(ctx context.Context, db *pg.DB) error {
			for i := len(tables) - 1; i >= 0; i-- {
				if _, err := db.ExecExpr(ctx, pg.DropTableIfExists(tables[i])); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// companyIndexes declares the 0010 secondary indexes. Every child table is read
// by workspace_id (the dossier fetch loads all six at once), and requirements
// are read by (workspace_id, tender_id).
//
// workspace_companies gets none: its primary key IS the only lookup path, the
// same reasoning migrate_client_profiles.go records for
// workspace_client_profiles.
func companyIndexes() []*pg.Index {
	return []*pg.Index{
		pg.NewIndex("idx_company_soa_ws", CompanySOACategories, idxCol(SOAWorkspaceID)),
		pg.NewIndex("idx_company_cert_ws", CompanyCertifications, idxCol(CertWorkspaceID)),
		pg.NewIndex("idx_company_fy_ws", CompanyFinancialYears, idxCol(FYWorkspaceID)),
		pg.NewIndex("idx_company_past_ws", CompanyPastContracts, idxCol(PCWorkspaceID)),
		pg.NewIndex("idx_company_reg_ws", CompanyRegistrations, idxCol(RegWorkspaceID)),
		pg.NewIndex("idx_tender_requirements_ws_tender", TenderRequirements, idxCol(TReqWorkspaceID), idxCol(TReqTenderID)),
	}
}
