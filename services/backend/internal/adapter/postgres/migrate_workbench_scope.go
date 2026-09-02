package postgres

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
)

// Permission masks the seeded workbench roles carried before authlayer, written
// out for the same reason their workspace counterparts are: they are historical
// facts about rows in the database, not part of the live vocabulary.
const (
	legacySeededManagerMask int64 = 79 // view + manage workbench/members/roles + administrator
	legacySeededViewerMask  int64 = 1  // view only
)

// migrateWorkbenchScope is the 0015 schema migration that puts workbench RBAC
// on authlayer's nested scope, with the workspace as its parent.
//
// It is 0013 again, one level down, and the same three shapes move: a role
// stops being a UUID row with a bitmask and becomes a key with encoded grants,
// memberships point at the key, and the containment columns take the names
// scope.MemberBase and scope.RoleRecord fix.
//
// One column deliberately does NOT move: workbenches.workspace_id keeps its
// name. scope.NestedBase would have called it parent_id, but the parent link is
// an INTERFACE (scope.Nested), so core/workbench satisfies it with a method
// over its own field and the column, and everything that reads it, stays put.
//
// The two seeded roles are dropped in favour of the code-defined ones, exactly
// as 0013 does: "Manager" becomes the admin key and "Viewer" the member key,
// after their members are moved across. A custom role that merely reads like a
// reserved one is renamed rather than promoted.
func migrateWorkbenchScope() pg.Migration {
	return pg.Migration{
		Version: "0015",
		Name:    "workbench_scope",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, s := range []string{
				`ALTER TABLE workbench_roles ADD COLUMN IF NOT EXISTS key TEXT`,
				`ALTER TABLE workbench_roles ADD COLUMN IF NOT EXISTS permissions_enc BYTEA`,
				`ALTER TABLE workbench_members ADD COLUMN IF NOT EXISTS role_key TEXT`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}

			plans, err := planWorkbenchRoles(ctx, db)
			if err != nil {
				return err
			}
			for _, p := range plans {
				if _, err := db.Exec(ctx,
					`UPDATE workbench_members SET role_key = $1 WHERE role_id = $2`, p.key, p.id,
				); err != nil {
					return err
				}
			}
			// Dropping role_id takes its foreign key with it, which is what
			// lets the seeded rows be deleted below.
			if _, err := db.Exec(ctx, `ALTER TABLE workbench_members DROP COLUMN role_id`); err != nil {
				return err
			}
			for _, p := range plans {
				if !p.keep {
					if _, err := db.Exec(ctx, `DELETE FROM workbench_roles WHERE id = $1`, p.id); err != nil {
						return err
					}
					continue
				}
				encoded, err := workbench.EncodePermissions(workbench.Permission(p.mask))
				if err != nil {
					return fmt.Errorf("encode permissions for workbench role %s: %w", p.id, err)
				}
				if _, err := db.Exec(ctx,
					`UPDATE workbench_roles SET key = $1, permissions_enc = $2 WHERE id = $3`,
					p.key, encoded, p.id,
				); err != nil {
					return err
				}
			}

			for _, s := range []string{
				`ALTER TABLE workbench_members ALTER COLUMN role_key SET NOT NULL`,
				`ALTER TABLE workbench_members RENAME COLUMN workbench_id TO container_id`,
				`ALTER TABLE workbench_members RENAME COLUMN added_at TO joined_at`,

				`ALTER TABLE workbench_roles DROP COLUMN permissions`,
				`ALTER TABLE workbench_roles RENAME COLUMN permissions_enc TO permissions`,
				`ALTER TABLE workbench_roles ALTER COLUMN permissions SET DEFAULT ''::bytea`,
				`UPDATE workbench_roles SET permissions = ''::bytea WHERE permissions IS NULL`,
				`ALTER TABLE workbench_roles ALTER COLUMN permissions SET NOT NULL`,
				`ALTER TABLE workbench_roles ALTER COLUMN key SET NOT NULL`,
				`ALTER TABLE workbench_roles DROP COLUMN is_default`,
				`ALTER TABLE workbench_roles RENAME COLUMN workbench_id TO container_id`,
				`ALTER TABLE workbench_roles ADD CONSTRAINT workbench_roles_container_key UNIQUE (container_id, key)`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			return nil
		},
		// Down restores the column shapes, not the data: reconstructing a
		// bitmask from encoded grants is lossy in that direction, and a
		// workbench whose roles came back wrong is worse than one whose roles
		// have to be recreated. Same posture as 0013.
		Down: func(ctx context.Context, db *pg.DB) error {
			for _, s := range []string{
				`ALTER TABLE workbench_roles DROP CONSTRAINT IF EXISTS workbench_roles_container_key`,
				`ALTER TABLE workbench_roles RENAME COLUMN container_id TO workbench_id`,
				`ALTER TABLE workbench_roles DROP COLUMN IF EXISTS key`,
				`ALTER TABLE workbench_roles DROP COLUMN IF EXISTS permissions`,
				`ALTER TABLE workbench_roles ADD COLUMN IF NOT EXISTS permissions BIGINT NOT NULL DEFAULT 0`,
				`ALTER TABLE workbench_roles ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false`,

				`ALTER TABLE workbench_members RENAME COLUMN joined_at TO added_at`,
				`ALTER TABLE workbench_members RENAME COLUMN container_id TO workbench_id`,
				`ALTER TABLE workbench_members DROP COLUMN IF EXISTS role_key`,
				`ALTER TABLE workbench_members ADD COLUMN IF NOT EXISTS role_id UUID`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// planWorkbenchRoles reads every stored workbench role and decides its key and
// fate, from one consistent snapshot taken before anything is written.
func planWorkbenchRoles(ctx context.Context, db *pg.DB) ([]rolePlan, error) {
	rows, err := db.Query(ctx, `SELECT id, workbench_id, name, permissions, is_default FROM workbench_roles`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var legacy []legacyRole
	for rows.Next() {
		var r legacyRole
		if err := rows.Scan(&r.id, &r.containerID, &r.name, &r.mask, &r.isDefault); err != nil {
			return nil, err
		}
		legacy = append(legacy, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return planWorkbenchRolePlans(legacy), nil
}

// planWorkbenchRolePlans is the decision itself, separated from the reading so
// it can be exercised without a database. Unlike the workspace's, the seeded
// rows here do NOT collapse into the key their names derive to: authlayer's
// registry calls them admin and member, and that is where their members land.
func planWorkbenchRolePlans(legacy []legacyRole) []rolePlan {
	reserved := []string{workbench.RoleOwner, workbench.RoleManager, workbench.RoleViewer}
	return planLegacyRoles(legacy, reserved, func(r legacyRole) (string, bool) {
		switch {
		case r.name == "Manager" && r.mask == legacySeededManagerMask:
			return workbench.RoleManager, true
		case r.name == "Viewer" && r.isDefault && r.mask == legacySeededViewerMask:
			return workbench.RoleViewer, true
		default:
			return "", false
		}
	})
}
