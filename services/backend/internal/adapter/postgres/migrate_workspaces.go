package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateWorkspaces is the 0002 schema migration for the workspace feature.
//
// Tables are generated from the drops schema DSL declared in schema.go, so the
// column definitions are the single source of truth for both DDL and queries.
// Composite constraints drops does not emit inline (the members composite PK and
// the two composite UNIQUEs) are added with raw ALTER TABLE. The whole Up runs
// inside one transaction (the migrator wraps it), so any failure rolls back the
// entire migration.
func migrateWorkspaces() pg.Migration {
	// Dependency order: parents before children (creation), reversed for drop.
	tables := []*pg.Table{
		Workspaces,
		WorkspaceRoles,
		WorkspaceMembers,
		WorkspaceEmailInvites,
		WorkspaceInviteLinks,
	}
	return pg.Migration{
		Version: "0002",
		Name:    "workspaces",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, t := range tables {
				if _, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(t)); err != nil {
					return err
				}
			}
			// Composite constraints — drops' CreateTable only emits single-column
			// PK/UNIQUE, so these are raw. The members composite PK is what
			// physically enforces one role per (workspace, user).
			for _, s := range []string{
				`ALTER TABLE workspace_members ADD CONSTRAINT pk_workspace_members PRIMARY KEY (workspace_id, user_id)`,
				`ALTER TABLE workspace_roles ADD CONSTRAINT uq_workspace_roles_name UNIQUE (workspace_id, name)`,
				`ALTER TABLE workspace_email_invitations ADD CONSTRAINT uq_workspace_email_invites UNIQUE (workspace_id, email)`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			// Indexes on foreign-key columns (the members.workspace_id lookup is
			// already covered by the leading column of the composite PK).
			for _, idx := range []*pg.Index{
				pg.NewIndex("idx_workspace_roles_workspace", WorkspaceRoles, WRoleWorkspaceID),
				pg.NewIndex("idx_workspace_members_user", WorkspaceMembers, WMemberUserID),
				pg.NewIndex("idx_workspace_members_role", WorkspaceMembers, WMemberRoleID),
				pg.NewIndex("idx_ws_email_invites_workspace", WorkspaceEmailInvites, WEInviteWorkspaceID),
				pg.NewIndex("idx_ws_invite_links_workspace", WorkspaceInviteLinks, WLinkWorkspaceID),
			} {
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
