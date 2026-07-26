package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateBids is the 0009 schema migration for the workbench bando hub. It
// creates the bids + bid_checklist_items tables, adds the two composite uniques
// drops does not emit inline ((workbench_id, tender_id) so a tender is taken
// into a workbench at most once; (bid_id, item_code) which backs the checklist
// upsert), and indexes the FK columns (workbench_id for portfolio listing,
// bid_id for checklist fetch).
func migrateBids() pg.Migration {
	tables := []*pg.Table{Bids, BidChecklistItems}
	return pg.Migration{
		Version: "0009",
		Name:    "bids",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, t := range tables {
				if _, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(t)); err != nil {
					return err
				}
			}
			for _, s := range []string{
				`ALTER TABLE bids ADD CONSTRAINT uq_bids_workbench_tender UNIQUE (workbench_id, tender_id)`,
				`ALTER TABLE bid_checklist_items ADD CONSTRAINT uq_bid_checklist_items_bid_item UNIQUE (bid_id, item_code)`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			for _, idx := range bidIndexes() {
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

// bidIndexes declares the 0009 secondary indexes on the bid foreign-key columns.
func bidIndexes() []*pg.Index {
	return []*pg.Index{
		pg.NewIndex("idx_bids_workbench", Bids, idxCol(BidWorkbenchID)),
		pg.NewIndex("idx_bid_checklist_items_bid", BidChecklistItems, idxCol(BCIBidID)),
	}
}
