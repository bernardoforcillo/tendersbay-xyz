package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

// TestBidTablesRenderColumns renders the CREATE TABLE DDL for both bid tables
// straight from the drops handles (no DB needed) and asserts each expected
// column stem is present — a fast guard that the schema declarations exist and
// name the columns the repo + migration rely on.
func TestBidTablesRenderColumns(t *testing.T) {
	bidsSQL, _ := drops.String(pg.CreateTableIfNotExists(Bids))
	for _, col := range []string{"bids", "workbench_id", "tender_id", "go_no_go", "stage", "outcome", "created_by", "created_at", "updated_at"} {
		if !strings.Contains(bidsSQL, col) {
			t.Errorf("bids DDL missing %q: %s", col, bidsSQL)
		}
	}
	itemsSQL, _ := drops.String(pg.CreateTableIfNotExists(BidChecklistItems))
	for _, col := range []string{"bid_checklist_items", "bid_id", "section_code", "item_code", "status", "note", "required", "position"} {
		if !strings.Contains(itemsSQL, col) {
			t.Errorf("bid_checklist_items DDL missing %q: %s", col, itemsSQL)
		}
	}
}
