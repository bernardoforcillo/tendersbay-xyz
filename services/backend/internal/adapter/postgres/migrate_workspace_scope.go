package postgres

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// Permission masks the seeded roles carried before authlayer. They are written
// out rather than referenced from core/workspace because they are historical
// facts about rows already in the database, not part of the live vocabulary:
// if the mask ever changes meaning, these two numbers must not follow it.
const (
	legacySeededAdminMask  int64 = 7340159 // every bit the old permAdminRole held
	legacySeededMemberMask int64 = 3145729 // view workspace + view/create workbenches
)

// migrateWorkspaceScope is the 0013 schema migration that hands workspace RBAC
// and invitations to authlayer's scope and invite engines.
//
// The shape change is one idea: a role stops being a row identified by a UUID
// carrying a permission bitmask, and becomes a KEY carrying encoded grant
// names, with owner/admin/member defined in code rather than stored at all.
// Everything below follows from that:
//
//   - workspace_members.role_id (uuid, FK) becomes role_key (text), and
//     workspace_id becomes container_id — the column names authlayer's
//     scope.MemberBase fixes.
//   - workspace_roles.permissions (bigint mask) becomes the encoded grant set
//     access.Permission.Encode produces; is_default disappears, because a
//     code-defined role is now the thing that is "default"; workspace_id
//     becomes container_id and a key column is added.
//   - The two seeded roles are DELETED, not converted: "Admin" and "Member"
//     are exactly what authlayer's registry now defines, and leaving the rows
//     would list every workspace's roles twice. Their members keep working
//     because they are moved onto the matching code-defined key first.
//   - A CUSTOM role whose name happens to derive to a reserved key is renamed
//     to <key>-custom rather than deleted: it is somebody's own role with its
//     own permissions, and silently promoting it to the registry's admin would
//     hand out permissions nobody granted.
//   - Invitations move to role_key + container_id, and an invite link's
//     revoked flag becomes the revoked_at timestamp authlayer records.
//
// The constraints authlayer relies on are added by hand rather than by calling
// its CreateSchema: these tables predate it and already carry equivalents under
// different names (the members composite PK, the token_hash and code uniques),
// and asking CreateSchema to add its own would try to give workspace_members a
// second PRIMARY KEY. The one genuinely missing constraint is the roles'
// (container_id, key) unique, which is what turns a concurrent double-insert
// into ErrRoleKeyTaken — that one is added below.
func migrateWorkspaceScope() pg.Migration {
	return pg.Migration{
		Version: "0013",
		Name:    "workspace_scope",
		Up: func(ctx context.Context, db *pg.DB) error {
			for _, s := range []string{
				`ALTER TABLE workspace_roles ADD COLUMN IF NOT EXISTS key TEXT`,
				`ALTER TABLE workspace_roles ADD COLUMN IF NOT EXISTS permissions_enc BYTEA`,
				`ALTER TABLE workspace_members ADD COLUMN IF NOT EXISTS role_key TEXT`,
				`ALTER TABLE workspace_email_invitations ADD COLUMN IF NOT EXISTS role_key TEXT`,
				`ALTER TABLE workspace_invite_links ADD COLUMN IF NOT EXISTS role_key TEXT`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}

			plans, err := planRoleMigration(ctx, db)
			if err != nil {
				return err
			}

			// Point every reference at the new key while role_id still exists.
			for _, p := range plans {
				for _, s := range []string{
					`UPDATE workspace_members SET role_key = $1 WHERE role_id = $2`,
					`UPDATE workspace_email_invitations SET role_key = $1 WHERE role_id = $2`,
					`UPDATE workspace_invite_links SET role_key = $1 WHERE role_id = $2`,
				} {
					if _, err := db.Exec(ctx, s, p.key, p.id); err != nil {
						return err
					}
				}
			}

			// Dropping role_id takes its foreign keys with it, which is what
			// lets the seeded role rows be deleted below.
			for _, s := range []string{
				`ALTER TABLE workspace_members DROP COLUMN role_id`,
				`ALTER TABLE workspace_email_invitations DROP COLUMN role_id`,
				`ALTER TABLE workspace_invite_links DROP COLUMN role_id`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}

			for _, p := range plans {
				if !p.keep {
					if _, err := db.Exec(ctx, `DELETE FROM workspace_roles WHERE id = $1`, p.id); err != nil {
						return err
					}
					continue
				}
				encoded, err := workspace.EncodePermissions(workspace.Permission(p.mask))
				if err != nil {
					return fmt.Errorf("encode permissions for role %s: %w", p.id, err)
				}
				if _, err := db.Exec(ctx,
					`UPDATE workspace_roles SET key = $1, permissions_enc = $2 WHERE id = $3`,
					p.key, encoded, p.id,
				); err != nil {
					return err
				}
			}

			for _, s := range []string{
				`ALTER TABLE workspace_members ALTER COLUMN role_key SET NOT NULL`,
				`ALTER TABLE workspace_members RENAME COLUMN workspace_id TO container_id`,

				`ALTER TABLE workspace_roles DROP COLUMN permissions`,
				`ALTER TABLE workspace_roles RENAME COLUMN permissions_enc TO permissions`,
				`ALTER TABLE workspace_roles ALTER COLUMN permissions SET DEFAULT ''::bytea`,
				`UPDATE workspace_roles SET permissions = ''::bytea WHERE permissions IS NULL`,
				`ALTER TABLE workspace_roles ALTER COLUMN permissions SET NOT NULL`,
				`ALTER TABLE workspace_roles ALTER COLUMN key SET NOT NULL`,
				`ALTER TABLE workspace_roles DROP COLUMN is_default`,
				`ALTER TABLE workspace_roles RENAME COLUMN workspace_id TO container_id`,
				`ALTER TABLE workspace_roles ADD CONSTRAINT workspace_roles_container_key UNIQUE (container_id, key)`,

				`ALTER TABLE workspace_email_invitations ALTER COLUMN role_key SET NOT NULL`,
				`ALTER TABLE workspace_email_invitations RENAME COLUMN workspace_id TO container_id`,

				`ALTER TABLE workspace_invite_links ALTER COLUMN role_key SET NOT NULL`,
				`ALTER TABLE workspace_invite_links RENAME COLUMN workspace_id TO container_id`,
				`ALTER TABLE workspace_invite_links ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ`,
				`UPDATE workspace_invite_links SET revoked_at = NOW() WHERE revoked`,
				`ALTER TABLE workspace_invite_links DROP COLUMN revoked`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			return nil
		},
		// Down restores the column shapes so the pre-authlayer code can run
		// again. It does NOT restore the seeded role rows or reconstruct a
		// bitmask from encoded grants: the mapping is lossy in that direction
		// (an encoded set that is not exactly one of the mask's bit groups has
		// no bit to become), and a workspace whose roles came back wrong is
		// worse than one whose roles have to be recreated.
		Down: func(ctx context.Context, db *pg.DB) error {
			for _, s := range []string{
				`ALTER TABLE workspace_invite_links ADD COLUMN IF NOT EXISTS revoked BOOLEAN NOT NULL DEFAULT false`,
				`UPDATE workspace_invite_links SET revoked = true WHERE revoked_at IS NOT NULL`,
				`ALTER TABLE workspace_invite_links DROP COLUMN IF EXISTS revoked_at`,
				`ALTER TABLE workspace_invite_links RENAME COLUMN container_id TO workspace_id`,
				`ALTER TABLE workspace_invite_links DROP COLUMN IF EXISTS role_key`,
				`ALTER TABLE workspace_invite_links ADD COLUMN IF NOT EXISTS role_id UUID`,

				`ALTER TABLE workspace_email_invitations RENAME COLUMN container_id TO workspace_id`,
				`ALTER TABLE workspace_email_invitations DROP COLUMN IF EXISTS role_key`,
				`ALTER TABLE workspace_email_invitations ADD COLUMN IF NOT EXISTS role_id UUID`,

				`ALTER TABLE workspace_roles DROP CONSTRAINT IF EXISTS workspace_roles_container_key`,
				`ALTER TABLE workspace_roles RENAME COLUMN container_id TO workspace_id`,
				`ALTER TABLE workspace_roles DROP COLUMN IF EXISTS key`,
				`ALTER TABLE workspace_roles DROP COLUMN IF EXISTS permissions`,
				`ALTER TABLE workspace_roles ADD COLUMN IF NOT EXISTS permissions BIGINT NOT NULL DEFAULT 0`,
				`ALTER TABLE workspace_roles ADD COLUMN IF NOT EXISTS is_default BOOLEAN NOT NULL DEFAULT false`,

				`ALTER TABLE workspace_members RENAME COLUMN container_id TO workspace_id`,
				`ALTER TABLE workspace_members DROP COLUMN IF EXISTS role_key`,
				`ALTER TABLE workspace_members ADD COLUMN IF NOT EXISTS role_id UUID`,
			} {
				if _, err := db.Exec(ctx, s); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

// planRoleMigration reads every stored role and decides its key and fate.
func planRoleMigration(ctx context.Context, db *pg.DB) ([]rolePlan, error) {
	rows, err := db.Query(ctx, `SELECT id, workspace_id, name, permissions, is_default FROM workspace_roles`)
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
	return planRoles(legacy), nil
}

// planRoles is the decision itself, separated from the reading so it can be
// exercised without a database. The shared rules live in planLegacyRoles; what
// this adds is which rows the old CreateWorkspace seeded.
func planRoles(legacy []legacyRole) []rolePlan {
	reserved := []string{workspace.RoleOwner, workspace.RoleAdmin, workspace.RoleMember}
	return planLegacyRoles(legacy, reserved, func(r legacyRole) (string, bool) {
		switch {
		case r.name == "Admin" && r.mask == legacySeededAdminMask:
			return workspace.RoleAdmin, true
		case r.name == "Member" && r.isDefault && r.mask == legacySeededMemberMask:
			return workspace.RoleMember, true
		default:
			return "", false
		}
	})
}
