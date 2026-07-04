package postgres

import (
	"strings"
	"testing"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/drops/pg"
)

// TestWorkspaceIndexesRenderUnqualifiedColumns guards against the 0002
// regression where an index column was rendered table-qualified
// (`"workspace_roles"."workspace_id"`). PostgreSQL rejects a qualified name
// inside a CREATE INDEX column list with `syntax error at or near ")"`
// (SQLSTATE 42601), which broke the whole 0002 migration. Every index
// column must render as a bare, unqualified identifier.
func TestWorkspaceIndexesRenderUnqualifiedColumns(t *testing.T) {
	for _, idx := range workspaceIndexes() {
		sql, _ := drops.String(pg.CreateIndexIfNotExists(idx))
		// A qualified column reference renders `"table"."column"`, whose
		// telltale substring is `"."`. An unqualified column is just
		// `"column"`, so this must not appear anywhere in the statement.
		if strings.Contains(sql, `"."`) {
			t.Errorf("index %q renders a table-qualified column, which PostgreSQL rejects in a CREATE INDEX column list: %s", idx.Name(), sql)
		}
	}
}
