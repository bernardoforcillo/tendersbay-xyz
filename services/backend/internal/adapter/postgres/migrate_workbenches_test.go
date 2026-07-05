package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

// TestWorkbenchIndexesRenderUnqualifiedColumns guards against the same 0002
// regression class (see TestWorkspaceIndexesRenderUnqualifiedColumns) for the
// 0003 workbench indexes: every index column must render as a bare,
// unqualified identifier, or PostgreSQL rejects the CREATE INDEX statement
// with `syntax error at or near ")"` (SQLSTATE 42601).
func TestWorkbenchIndexesRenderUnqualifiedColumns(t *testing.T) {
	for _, idx := range workbenchIndexes() {
		sql, _ := drops.String(pg.CreateIndexIfNotExists(idx))
		if strings.Contains(sql, `"."`) {
			t.Errorf("index %q renders a table-qualified column, which PostgreSQL rejects in a CREATE INDEX column list: %s", idx.Name(), sql)
		}
	}
}
