package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

func TestMigrateBidDecisionVersion(t *testing.T) {
	m := migrateBidDecision()
	if m.Version != "0011" {
		t.Errorf("migrateBidDecision version = %q, want %q", m.Version, "0011")
	}
	if m.Name != "bid_decision_record" {
		t.Errorf("migrateBidDecision name = %q, want %q", m.Name, "bid_decision_record")
	}
}

// TestBidsDDLCarriesTheDecisionRecord: the 0011 ALTER only reaches a database
// that already ran 0009. A FRESH database gets these columns from the Bids
// handle at CREATE TABLE time, so the handle and the ALTER have to agree — and
// the failure mode if they do not is silent: the repo's RETURNING scans a column
// that does not exist on one of the two paths.
func TestBidsDDLCarriesTheDecisionRecord(t *testing.T) {
	sql, _ := drops.String(pg.CreateTableIfNotExists(Bids))
	for _, col := range []string{
		"decision_recommendation", "decision_overridden",
		"decision_blocking_gaps", "decision_recorded_at",
	} {
		if !strings.Contains(sql, col) {
			t.Errorf("bids DDL missing %q: %s", col, sql)
		}
	}
	// decision_recorded_at is the only nullable one: NULL means "we do not know
	// when this was decided", which is the truth for every bid decided before
	// the record existed.
	if strings.Contains(sql, `"decision_recorded_at" TIMESTAMPTZ NOT NULL`) {
		t.Errorf("decision_recorded_at must stay nullable: %s", sql)
	}
}

// TestBidColumnsProjectsEveryScannedColumn: DBBid is scanned from bidColumns()
// on all four write paths, and a column present on the struct but absent from
// the projection scans as its zero value — which for the decision record is
// indistinguishable from a real "no recommendation existed, not overridden".
func TestBidColumnsProjectsEveryScannedColumn(t *testing.T) {
	if got, want := len(bidColumns()), 13; got != want {
		t.Fatalf("bidColumns() projects %d columns, want %d (one per DBBid field)", got, want)
	}
}
