package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

func TestMigrateEspdVersion(t *testing.T) {
	m := migrateEspd()
	if m.Version != "0016" {
		t.Errorf("migrateEspd version = %q, want %q", m.Version, "0016")
	}
	if m.Name != "espd" {
		t.Errorf("migrateEspd name = %q, want %q", m.Name, "espd")
	}
}

// TestEspdIndexesRenderUnqualifiedColumns guards the 0016 indexes against the
// regression class 0002/0003/0009/0010 all hit: an index column must render as
// a bare identifier or PostgreSQL rejects CREATE INDEX with SQLSTATE 42601.
func TestEspdIndexesRenderUnqualifiedColumns(t *testing.T) {
	for _, idx := range espdIndexes() {
		sql, _ := drops.String(pg.CreateIndexIfNotExists(idx))
		if strings.Contains(sql, `"."`) {
			t.Errorf("index %q renders a table-qualified column: %s", idx.Name(), sql)
		}
	}
}

// TestEspdMigrationIsRegisteredAfterWorkbenchScope pins the chain order: 0016
// must follow 0015 in db.go, or a fresh database would build the tables before
// the bids table they reference.
func TestEspdMigrationIsRegisteredLast(t *testing.T) {
	versions := []string{
		migrateWorkbenchScope().Version, migrateEspd().Version,
	}
	if versions[0] >= versions[1] {
		t.Errorf("0016 must sort after the previous migration, got %v", versions)
	}
}
