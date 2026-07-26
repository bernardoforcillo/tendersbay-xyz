package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

func TestMigrateBidsVersion(t *testing.T) {
	m := migrateBids()
	if m.Version != "0009" {
		t.Errorf("migrateBids version = %q, want %q", m.Version, "0009")
	}
	if m.Name != "bids" {
		t.Errorf("migrateBids name = %q, want %q", m.Name, "bids")
	}
}

// TestBidIndexesRenderUnqualifiedColumns guards the 0009 indexes against the
// same regression class as 0002/0003: an index column must render as a bare
// identifier or PostgreSQL rejects CREATE INDEX with SQLSTATE 42601.
func TestBidIndexesRenderUnqualifiedColumns(t *testing.T) {
	for _, idx := range bidIndexes() {
		sql, _ := drops.String(pg.CreateIndexIfNotExists(idx))
		if strings.Contains(sql, `"."`) {
			t.Errorf("index %q renders a table-qualified column, which PostgreSQL rejects in a CREATE INDEX column list: %s", idx.Name(), sql)
		}
	}
}
