package postgres

import (
	"context"

	"github.com/bernardoforcillo/drops/pg"
)

// migrateWorkbenches is the 0003 schema migration for the workbench feature.
// It creates the three workbench tables, adds the composite constraints drops
// does not emit inline, indexes the FK columns, and grants the two baseline
// workbench bits (VIEW=1<<20, CREATE=1<<21) to every existing default workspace
// role so current members can immediately create and see workbenches.
func migrateWorkbenches() pg.Migration {
	tables := []*pg.Table{Workbenches, WorkbenchRoles, WorkbenchMembers}
	return pg.Migration{
		Version: "0003",
		Name:    "workbenches",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, t := range tables {
				if _, err := db.ExecExpr(ctx, pg.CreateTableIfNotExists(t)); err != nil {
					return err
				}
			}
			for _, s := range []string{
				`ALTER TABLE workbench_members ADD CONSTRAINT pk_workbench_members PRIMARY KEY (workbench_id, user_id)`,
				`ALTER TABLE workbench_roles ADD CONSTRAINT uq_workbench_roles_name UNIQUE (workbench_id, name)`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			for _, idx := range workbenchIndexes() {
				if _, err := db.ExecExpr(ctx, pg.CreateIndexIfNotExists(idx)); err != nil {
					return err
				}
			}
			// Backfill: grant VIEW_WORKBENCHES|CREATE_WORKBENCH (1<<20 | 1<<21 =
			// 3145728) to existing default workspace roles.
			if _, err := db.Exec(ctx, `UPDATE workspace_roles SET permissions = permissions | 3145728 WHERE is_default = true`); err != nil {
				return err
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

// workbenchIndexes declares the 0003 secondary indexes on the workbench
// foreign-key columns.
func workbenchIndexes() []*pg.Index {
	return []*pg.Index{
		pg.NewIndex("idx_workbenches_workspace", Workbenches, idxCol(WBWorkspaceID)),
		pg.NewIndex("idx_workbench_roles_workbench", WorkbenchRoles, idxCol(WBRoleWorkbenchID)),
		pg.NewIndex("idx_workbench_members_user", WorkbenchMembers, idxCol(WBMemberUserID)),
		pg.NewIndex("idx_workbench_members_role", WorkbenchMembers, idxCol(WBMemberRoleID)),
	}
}
