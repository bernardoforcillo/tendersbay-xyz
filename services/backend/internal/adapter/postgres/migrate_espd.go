package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateEspd is the 0016 schema migration for the ESPD/DGUE generator: the
// three dossier sections the eligibility engine never needed (representatives,
// Part III declarations, national grounds), the per-bid data (lots,
// subcontractors, reliances, the declaration confirmation), the buyer's request
// and the export audit — plus the one column the existing identity row gains,
// is_sme.
//
// Everything lives in `public` and belongs to this service. The composite
// uniques are raw ALTER TABLE for the reason migrate_company.go states (drops
// does not emit composites inline), and each is a modelling claim:
//
//   - (workspace_id, criterion) on declarations and (workspace_id, country,
//     criterion) on national grounds — an operator has exactly ONE current
//     answer to a Part III question; a re-answer is a correction.
//   - (bid_id, lot_ref) — a lot is tendered for once.
//   - (bid_id, vat) on subcontractors and (bid_id, vat, criterion) on
//     reliances — a portal listing the same party twice describes one
//     relationship.
//
// Representatives carry no composite unique on purpose: two people can share a
// name and a birth date is optional, so a natural key would either be nullable
// (and not unique) or would refuse a legitimate second row.
//
// Down drops the new tables and the is_sme column. Nothing to reconstruct — the
// tables are new, and is_sme was unrepresentable before 0016.
func migrateEspd() pg.Migration {
	tables := []*pg.Table{
		CompanyRepresentatives,
		CompanyDeclarations,
		CompanyNationalGrounds,
		BidLots,
		BidSubcontractors,
		BidReliances,
		BidDeclarationConfirmations,
		BidEspdRequests,
		BidEspdExports,
	}
	return pg.Migration{
		Version: "0016",
		Name:    "espd",
		Up: func(ctx context.Context, db *pg.DB) error {
			if _, err := db.Exec(ctx, `ALTER TABLE workspace_companies ADD COLUMN IF NOT EXISTS is_sme boolean NOT NULL DEFAULT false`); err != nil {
				return err
			}
			for _, t := range tables {
				if _, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(t)); err != nil {
					return err
				}
			}
			for _, s := range []string{
				`ALTER TABLE company_declarations ADD CONSTRAINT uq_company_decl_ws_criterion UNIQUE (workspace_id, criterion)`,
				`ALTER TABLE company_national_grounds ADD CONSTRAINT uq_company_ng_ws_country_criterion UNIQUE (workspace_id, country, criterion)`,
				`ALTER TABLE bid_lots ADD CONSTRAINT uq_bid_lots_bid_ref UNIQUE (bid_id, lot_ref)`,
				`ALTER TABLE bid_subcontractors ADD CONSTRAINT uq_bid_sub_bid_vat UNIQUE (bid_id, vat)`,
				`ALTER TABLE bid_reliances ADD CONSTRAINT uq_bid_rel_bid_vat_criterion UNIQUE (bid_id, vat, criterion)`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			for _, idx := range espdIndexes() {
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
			_, err := db.Exec(ctx, `ALTER TABLE workspace_companies DROP COLUMN IF EXISTS is_sme`)
			return err
		},
	}
}

// espdIndexes declares the 0016 secondary indexes: every dossier child is read
// by workspace_id (the dossier fetch), every bid child by bid_id (the ESPD data
// fetch and the export listing). The composite uniques above already index
// their leading column, so declarations, national grounds, lots,
// subcontractors and reliances need no separate FK index; the tables without a
// composite unique get one.
func espdIndexes() []*pg.Index {
	return []*pg.Index{
		pg.NewIndex("idx_company_rep_ws", CompanyRepresentatives, idxCol(RepWorkspaceID)),
		pg.NewIndex("idx_bid_espd_exports_bid", BidEspdExports, idxCol(EXBidID)),
	}
}
