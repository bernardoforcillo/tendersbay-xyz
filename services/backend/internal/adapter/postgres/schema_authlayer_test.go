package postgres

import (
	"fmt"
	"sort"
	"testing"

	dropsstore "github.com/bernardoforcillo/authlayer/store/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// authlayer derives its tables from the drop: tags on the structs it persists,
// so what those structs say IS the schema its stores read and write. Migrations
// 0012, 0013 and 0015 reshape tables that predate the library, which means they
// spell those column names out by hand — in an ALTER … RENAME, an ADD COLUMN,
// or a constraint name — with nothing connecting the two spellings.
//
// The tests below are that connection. They fail when an authlayer upgrade
// moves a column, adds one, or adds a constraint the migrations do not create,
// which is the one class of drift the rest of the suite cannot see: every unit
// test here runs against store/memory, and the mismatch would first appear as a
// failing query in production.
//
// A pinned set is not a duplicate of the migration: the migration says how the
// table got its shape, this says which shape the library will look for.

// columnNames is the set of columns a derived table declares, sorted so the
// comparison does not depend on field order in the struct.
func columnNames(t *pg.Table) []string {
	names := make([]string, 0, len(t.Columns()))
	for _, c := range t.Columns() {
		names = append(names, c.Name())
	}
	sort.Strings(names)
	return names
}

func assertColumns(t *testing.T, tbl *pg.Table, migration string, want ...string) {
	t.Helper()
	sort.Strings(want)
	got := columnNames(tbl)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("authlayer derives %s as %v; migration %s produces %v.\n"+
			"The library moved: update the migration (and this expectation) together.",
			tbl.Name(), got, migration, want)
	}
}

func assertCompositeUniques(t *testing.T, tbl *pg.Table, want map[string][]string) {
	t.Helper()
	got := map[string][]string{}
	for name, cols := range tbl.CompositeUniques() {
		for _, c := range cols {
			got[name] = append(got[name], c.Name())
		}
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("%s composite uniques = %v, want %v.\n"+
			"These are added by hand — the migrations do not call CreateSchema on this table — "+
			"so a new one the library expects is a constraint nobody creates.",
			tbl.Name(), got, want)
	}
}

// The scope stores do not call CreateSchema on these tables, so an index
// authlayer starts relying on is one no migration creates.
func assertNoIndexes(t *testing.T, tbl *pg.Table) {
	t.Helper()
	if n := len(tbl.Indexes()); n != 0 {
		var names []string
		for _, i := range tbl.Indexes() {
			names = append(names, i.Name())
		}
		t.Fatalf("%s now declares %d index(es) %v that no migration creates", tbl.Name(), n, names)
	}
}

func workspaceScopeSchema() *dropsstore.Schema[workspace.Workspace, workspace.Member] {
	return dropsstore.NewSchema[workspace.Workspace, workspace.Member](dropsstore.WithNames(workspaceTables()))
}

func workbenchScopeSchema() *dropsstore.Schema[workbench.Workbench, workbench.Member] {
	return dropsstore.NewSchema[workbench.Workbench, workbench.Member](dropsstore.WithNames(workbenchTables()))
}

// The two container tables are read through two independent handles: the ones
// in schema.go, which WorkspaceRepo and WorkbenchRepo use for this product's own
// columns, and the one authlayer derives from the same struct for containment.
// They describe one physical table, so they have to agree — and unlike the
// pinned sets below, this needs no expectation written down at all.
func TestContainerTablesAgreeWithTheProductSchema(t *testing.T) {
	for _, tc := range []struct {
		name             string
		product, derived *pg.Table
	}{
		{"workspaces", Workspaces, workspaceScopeSchema().Containers},
		{"workbenches", Workbenches, workbenchScopeSchema().Containers},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.product.Name() != tc.derived.Name() {
				t.Fatalf("table name %q vs %q", tc.product.Name(), tc.derived.Name())
			}
			p, d := fmt.Sprint(columnNames(tc.product)), fmt.Sprint(columnNames(tc.derived))
			if p != d {
				t.Fatalf("schema.go declares %s as %s; authlayer derives %s from the domain struct",
					tc.name, p, d)
			}
		})
	}
}

// 0013 renames workspace_members.workspace_id to container_id and role_id to
// role_key, and rebuilds workspace_roles around (container_id, key).
func TestWorkspaceScopeSchemaMatchesMigration0013(t *testing.T) {
	s := workspaceScopeSchema()

	assertColumns(t, s.Members, "0013", "container_id", "user_id", "role_key", "joined_at")
	assertColumns(t, s.Roles, "0013", "id", "container_id", "key", "name", "permissions", "created_at")
	assertNoIndexes(t, s.Members)
	assertNoIndexes(t, s.Roles)

	// 0002 created pk_workspace_members on (workspace_id, user_id); the rename
	// carries the constraint with the column, which is why 0013 does not touch
	// it. That only holds while the library keys membership the same way.
	var pk []string
	for _, c := range s.Members.CompositePrimaryKey() {
		pk = append(pk, c.Name())
	}
	if fmt.Sprint(pk) != fmt.Sprint([]string{"container_id", "user_id"}) {
		t.Fatalf("membership primary key = %v; 0002's pk_workspace_members no longer covers it", pk)
	}

	// The one constraint 0013 adds by hand, by this literal name.
	assertCompositeUniques(t, s.Roles, map[string][]string{
		"workspace_roles_container_key": {"container_id", "key"},
	})
}

// 0015 is 0013 one level down, with one deliberate exception: workspace_id
// keeps its name, because the parent link is an interface rather than a column
// authlayer owns.
func TestWorkbenchScopeSchemaMatchesMigration0015(t *testing.T) {
	s := workbenchScopeSchema()

	assertColumns(t, s.Members, "0015", "container_id", "user_id", "role_key", "joined_at")
	assertColumns(t, s.Roles, "0015", "id", "container_id", "key", "name", "permissions", "created_at")
	assertNoIndexes(t, s.Members)
	assertNoIndexes(t, s.Roles)

	if s.Containers.Col("workspace_id") == nil {
		t.Fatal("the workbench container no longer carries workspace_id: " +
			"scope.Nested is being satisfied by a column named something else, and " +
			"every query in workbench_store.go reads the old name")
	}

	assertCompositeUniques(t, s.Roles, map[string][]string{
		"workbench_roles_container_key": {"container_id", "key"},
	})
}

// 0012 hands the three auth tables over. Unlike the scope migrations it DOES
// call the library's CreateSchema, so constraints and indexes are self-healing
// there and only the columns 0012 adds by hand are pinned here.
func TestAuthSchemaMatchesMigration0012(t *testing.T) {
	s := dropsstore.NewAuthSchema()

	assertColumns(t, s.Users, "0012",
		"id", "email", "email_verified_at", "password_hash", "created_at", "updated_at", "deleted_at")
	assertColumns(t, s.Sessions, "0012",
		"id", "user_id", "token_hash", "family_id", "expires_at", "created_at",
		"rotated_at", "user_agent", "ip", "mfa_at")
	// verifications is created by CreateSchema, so its columns are the
	// library's by construction — what 0012 spells out is the INSERT that
	// carries live tokens into it.
	assertColumns(t, s.Verifications, "0012",
		"id", "user_id", "token_hash", "purpose", "email", "expires_at", "created_at")
}

// 0013 moves the two invitation tables onto authlayer's columns. Like the scope
// tables they are never handed to CreateSchema, so their constraints come from
// 0002 (under names of its own) plus what 0013 adds.
func TestInviteSchemaMatchesMigration0013(t *testing.T) {
	s := dropsstore.NewInviteSchema(dropsstore.WithInviteNames(inviteTables()))

	assertColumns(t, s.EmailInvites, "0013",
		"id", "container_id", "email", "role_key", "token_hash", "invited_by", "expires_at", "created_at")
	assertColumns(t, s.Links, "0013",
		"id", "container_id", "code", "role_key", "created_by",
		"max_uses", "use_count", "expires_at", "revoked_at", "created_at")
	assertNoIndexes(t, s.EmailInvites)
	assertNoIndexes(t, s.Links)

	// (container_id, email) is 0002's uq_workspace_email_invites, carried
	// across by the rename; token_hash and code are the inline uniques 0002
	// declared. All three are load-bearing — the library's doc calls them what
	// makes a lookup resolve to exactly one row — so a fourth appearing here
	// is a constraint no migration creates.
	assertCompositeUniques(t, s.EmailInvites, map[string][]string{
		"workspace_email_invitations_container_email": {"container_id", "email"},
		"workspace_email_invitations_token_hash":      {"token_hash"},
	})
	assertCompositeUniques(t, s.Links, map[string][]string{
		"workspace_invite_links_code": {"code"},
	})
}
