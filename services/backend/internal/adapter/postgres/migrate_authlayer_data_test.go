package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/drops/stdlib"
	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workbench"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/migrations"
	"github.com/jackc/pgx/v5"
	pgxstdlib "github.com/jackc/pgx/v5/stdlib"
)

// Running 0012–0015 against an empty database only proves their DDL parses.
// Every interesting line in them is a data-carrying one — a token copied into
// verifications, a member moved from a role_id to a role_key, a credits row
// turned into a subscription and a usage counter — and none of that executes
// with nothing in the tables.
//
// So this stops the chain at 0011, writes the shapes a live deployment actually
// holds, and then runs the four migrations over them.
//
// It needs a database it may reshape from scratch, which is the same scratch
// database the store contracts use, and it drops the whole public schema on the
// way in so the partial chain starts from nothing.
func TestAuthlayerMigrationsCarryLegacyData(t *testing.T) {
	dsn := os.Getenv("TEST_AUTH_CONTRACT_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_AUTH_CONTRACT_DATABASE_URL not set (a SCRATCH database: this test drops the public schema)")
	}
	ctx := context.Background()
	db, sqlDB := freshDatabase(t, dsn)

	// ── up to 0011: the schema as it stood before authlayer ──────────────
	if err := migratorThrough0011(db).Up(ctx); err != nil {
		t.Fatalf("migrate to 0011: %v", err)
	}
	seed := seedLegacyRows(t, sqlDB)

	// ── 0012–0015 ────────────────────────────────────────────────────────
	m := migratorThrough0011(db)
	m.Add(migrateAuthlayer())
	m.Add(migrateWorkspaceScope())
	m.Add(migrateFeatures())
	m.Add(migrateWorkbenchScope())
	if err := m.Up(ctx); err != nil {
		t.Fatalf("migrate to 0015: %v", err)
	}

	t.Run("0012 carries live tokens into verifications", func(t *testing.T) {
		var purpose, email string
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT purpose, email FROM verifications WHERE id = $1`, seed.emailVerificationID,
		).Scan(&purpose, &email); err != nil {
			t.Fatalf("the pending email verification did not survive: %v", err)
		}
		// An email_verifications row records no purpose, so every one of them
		// lands as signup — the safe direction, per the migration's own doc.
		if purpose != "signup" || email != "new@example.com" {
			t.Fatalf("carried verification = (%q, %q)", purpose, email)
		}

		if err := sqlDB.QueryRowContext(ctx,
			`SELECT purpose, email FROM verifications WHERE id = $1`, seed.passwordResetID,
		).Scan(&purpose, &email); err != nil {
			t.Fatalf("the pending password reset did not survive: %v", err)
		}
		// A password_resets row records no address, so it comes from the
		// account — which is the address the mail went to.
		if purpose != "password_reset" || email != "owner@example.com" {
			t.Fatalf("carried reset = (%q, %q)", purpose, email)
		}

		// An expired token is dead either way, and copying it would only give
		// PurgeExpired more to sweep.
		var expired int
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT count(*) FROM verifications WHERE id = $1`, seed.expiredVerificationID,
		).Scan(&expired); err != nil || expired != 0 {
			t.Fatalf("an expired token was carried across (count=%d, err=%v)", expired, err)
		}
	})

	t.Run("0012 gives every session a family of its own", func(t *testing.T) {
		var familyID string
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT family_id FROM sessions WHERE id = $1`, seed.sessionID).Scan(&familyID); err != nil {
			t.Fatalf("session lost: %v", err)
		}
		// A session minted before rotation existed is the only member of its
		// own family, so its id is the correct family id.
		if familyID != seed.sessionID {
			t.Fatalf("family_id = %q, want the session's own id %q", familyID, seed.sessionID)
		}
	})

	t.Run("0013 moves members onto the code-defined keys", func(t *testing.T) {
		assertRoleKey(t, sqlDB, "workspace_members", seed.adminUserID, workspace.RoleAdmin)
		assertRoleKey(t, sqlDB, "workspace_members", seed.memberUserID, workspace.RoleMember)
		// The custom role is somebody's own: it keeps its permissions and gets
		// a key derived from its name.
		assertRoleKey(t, sqlDB, "workspace_members", seed.customUserID, "bid-team")
		// A role that merely READS like a reserved one is renamed, never
		// promoted — its holder must not land on the registry's admin.
		assertRoleKey(t, sqlDB, "workspace_members", seed.reservedNameUserID, "admin-custom")
		// And two names deriving to one key are separated, or the new
		// (container_id, key) unique would reject the second row outright.
		assertRoleKey(t, sqlDB, "workspace_members", seed.collidingUserID, "bid-team-2")
	})

	t.Run("0013 drops the seeded rows and keeps the custom one", func(t *testing.T) {
		var names []string
		rows, err := sqlDB.QueryContext(ctx, `SELECT name FROM workspace_roles WHERE container_id = $1`, seed.workspaceID)
		if err != nil {
			t.Fatalf("list roles: %v", err)
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var n string
			if err := rows.Scan(&n); err != nil {
				t.Fatalf("scan: %v", err)
			}
			names = append(names, n)
		}
		// "Admin" and "Member" are exactly what authlayer's registry defines
		// now; leaving the rows would list every workspace's roles twice.
		sort.Strings(names)
		want := []string{"ADMIN", "Bid Team", "bid  team"}
		if fmt.Sprint(names) != fmt.Sprint(want) {
			t.Fatalf("workspace roles after 0013 = %v, want only the custom ones %v", names, want)
		}

		// The kept role's mask has to come back as grants authlayer can read.
		var encoded []byte
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT permissions FROM workspace_roles WHERE container_id = $1 AND key = 'bid-team'`,
			seed.workspaceID,
		).Scan(&encoded); err != nil {
			t.Fatalf("read permissions: %v", err)
		}
		if len(encoded) == 0 {
			t.Fatal("the custom role's permissions encoded to nothing")
		}
	})

	t.Run("0013 moves the invitations and turns revoked into a timestamp", func(t *testing.T) {
		var roleKey string
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT role_key FROM workspace_email_invitations WHERE id = $1`, seed.inviteID,
		).Scan(&roleKey); err != nil {
			t.Fatalf("invitation lost: %v", err)
		}
		if roleKey != workspace.RoleMember {
			t.Fatalf("invitation role_key = %q", roleKey)
		}

		var revokedAt *time.Time
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT revoked_at FROM workspace_invite_links WHERE id = $1`, seed.revokedLinkID,
		).Scan(&revokedAt); err != nil {
			t.Fatalf("link lost: %v", err)
		}
		if revokedAt == nil {
			t.Fatal("a link that was revoked came back with no revoked_at")
		}
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT revoked_at FROM workspace_invite_links WHERE id = $1`, seed.liveLinkID,
		).Scan(&revokedAt); err != nil {
			t.Fatalf("live link lost: %v", err)
		}
		if revokedAt != nil {
			t.Fatalf("a live link came back revoked at %s", revokedAt)
		}
	})

	t.Run("0014 turns a credits row into a subscription and a counter", func(t *testing.T) {
		repo := NewSubscriptionRepo(db)
		sub, err := repo.Subscription(ctx, seed.workspaceID)
		if err != nil {
			t.Fatalf("Subscription: %v", err)
		}
		if sub.Plan != features.PlanFree {
			t.Fatalf("plan = %q", sub.Plan)
		}
		// The billing anchor is the cycle the workspace was already in, so
		// nobody gains or loses a month.
		if !sub.BillingAnchor.UTC().Equal(seed.cycleStart) {
			t.Fatalf("billing anchor = %s, want the cycle it was on, %s", sub.BillingAnchor.UTC(), seed.cycleStart)
		}
		// An allowance moved off the default is somebody's negotiated number:
		// dropping it would silently re-price their workspace.
		if len(sub.Grants) != 1 || sub.Grants[0].Limit == nil || sub.Grants[0].Limit.Max != seed.allowance {
			t.Fatalf("grants = %+v, want an override at %d", sub.Grants, seed.allowance)
		}

		period, ok := features.MeterPeriod(features.AgentTokens)
		if !ok {
			t.Fatal("the agent budget declares no metered period")
		}
		if sub.Grants[0].Limit.Period != period {
			t.Fatalf("grant period = %q, want the catalog's %q", sub.Grants[0].Limit.Period, period)
		}

		// Its spend so far becomes the counter for the current period, keyed
		// the way the engine will read it back.
		used, err := NewFeatureUsageRepo(db).Get(ctx, entitlement.UsageKey{
			Tenant:  seed.workspaceID,
			Feature: features.AgentTokens,
			Period:  entitlement.PeriodKey(period, seed.cycleStart, time.Now().UTC()),
		})
		if err != nil {
			t.Fatalf("usage Get: %v", err)
		}
		if used != seed.spent {
			t.Fatalf("carried usage = %d, want %d", used, seed.spent)
		}
	})

	t.Run("0014 subscribes a workspace that never had a credits row", func(t *testing.T) {
		// Such a workspace could not run an agent turn before and would still
		// be unable to; leaving it out is a regression nobody asked for.
		sub, err := NewSubscriptionRepo(db).Subscription(ctx, seed.creditlessWorkspaceID)
		if err != nil {
			t.Fatalf("Subscription: %v", err)
		}
		if sub.Plan != features.PlanFree {
			t.Fatalf("plan = %q", sub.Plan)
		}
		if len(sub.Grants) != 0 {
			t.Fatalf("a workspace on the default allowance got grants %+v", sub.Grants)
		}
	})

	t.Run("0014 leaves a default allowance ungranted but keeps its anchor", func(t *testing.T) {
		sub, err := NewSubscriptionRepo(db).Subscription(ctx, seed.defaultWorkspaceID)
		if err != nil {
			t.Fatalf("Subscription: %v", err)
		}
		// The plan already says this number; recording it again as a grant
		// would freeze the workspace at today's free allowance and quietly
		// exclude it from any future change to the plan.
		if len(sub.Grants) != 0 {
			t.Fatalf("a workspace on the default allowance got grants %+v", sub.Grants)
		}
		if !sub.BillingAnchor.UTC().Equal(seed.cycleStart) {
			t.Fatalf("billing anchor = %s, want the cycle it was on, %s", sub.BillingAnchor.UTC(), seed.cycleStart)
		}

		// Nothing spent, so there is no counter to carry — and writing a zero
		// row would only be a row the engine has to read past.
		var rows int
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT count(*) FROM feature_usage WHERE workspace_id = $1`, seed.defaultWorkspaceID,
		).Scan(&rows); err != nil {
			t.Fatalf("count usage: %v", err)
		}
		if rows != 0 {
			t.Fatalf("a workspace that had spent nothing got %d usage rows", rows)
		}
	})

	t.Run("0014 drops the credits table", func(t *testing.T) {
		var exists bool
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT to_regclass('public.workspace_credits') IS NOT NULL`).Scan(&exists); err != nil {
			t.Fatalf("to_regclass: %v", err)
		}
		if exists {
			t.Fatal("workspace_credits is still there")
		}
	})

	t.Run("0015 moves workbench members and renames added_at", func(t *testing.T) {
		assertRoleKey(t, sqlDB, "workbench_members", seed.adminUserID, workbench.RoleManager)
		assertRoleKey(t, sqlDB, "workbench_members", seed.memberUserID, workbench.RoleViewer)

		var joinedAt time.Time
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT joined_at FROM workbench_members WHERE container_id = $1 AND user_id = $2`,
			seed.workbenchID, seed.adminUserID).Scan(&joinedAt); err != nil {
			t.Fatalf("joined_at: %v", err)
		}
		if joinedAt.IsZero() {
			t.Fatal("added_at did not carry into joined_at")
		}

		// workspace_id keeps its name: the parent link is an interface, not a
		// column authlayer owns.
		var workspaceID string
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT workspace_id FROM workbenches WHERE id = $1`, seed.workbenchID).Scan(&workspaceID); err != nil {
			t.Fatalf("workbenches.workspace_id: %v", err)
		}
		if workspaceID != seed.workspaceID {
			t.Fatalf("workbench parent = %q, want %q", workspaceID, seed.workspaceID)
		}
	})

	// The migrated tables have to be readable through the stores the service
	// actually runs on, not only through SELECTs written here.
	t.Run("the scope stores read what the migrations wrote", func(t *testing.T) {
		standings, err := NewWorkspaceScopeStore(db).ListUserStandings(ctx, seed.adminUserID)
		if err != nil {
			t.Fatalf("ListUserStandings: %v", err)
		}
		if len(standings) != 1 || standings[0].ContainerID != seed.workspaceID || standings[0].RoleKey != workspace.RoleAdmin {
			t.Fatalf("workspace standings = %+v", standings)
		}
		members, err := NewWorkbenchScopeStore(db).ListMembers(ctx, seed.workbenchID)
		if err != nil {
			t.Fatalf("ListMembers: %v", err)
		}
		if len(members) != 2 {
			t.Fatalf("workbench members = %+v", members)
		}
	})

	// The last link in the chain, and the one nothing else exercises: 0013
	// rewrote every kept role's permission column with
	// workspace.EncodePermissions, and until now nobody has read one back out
	// of Postgres. This runs the whole round trip — old bitmask, encoded
	// grants, a column, authlayer's decode, the projection the proto speaks —
	// through the same Service the handlers call.
	t.Run("a migrated role projects back to the mask the client is told", func(t *testing.T) {
		svc := workspace.NewService(
			workspace.NewAccess(),
			NewWorkspaceScopeStore(db),
			NewWorkspaceInviteStore(db),
			NewWorkspaceRepo(db),
			NewUserRepo(db),
			nil, // Standing sends no mail
			workspace.Config{AppBaseURL: "https://example.test", InviteExpiry: 7 * 24 * time.Hour},
		)

		for _, tc := range []struct {
			name     string
			userID   string
			holds    workspace.Permission
			withheld workspace.Permission
		}{
			// An elevated caller holds every bit, which is the rule that makes
			// an inherited elevation readable — see internal/rbac.
			{"the seeded admin", seed.adminUserID,
				workspace.PermManageMembers | workspace.PermManageWorkbenches | workspace.PermAdministrator, 0},
			// The baseline member can see and create workbenches, which is what
			// the seeded "Member" role granted before authlayer.
			{"the seeded member", seed.memberUserID,
				workspace.PermViewWorkbenches | workspace.PermCreateWorkbench,
				workspace.PermManageMembers | workspace.PermAdministrator},
			// The custom role's own two capabilities, and nothing it never had.
			{"a custom role", seed.customUserID,
				workspace.PermViewWorkbenches | workspace.PermManageInvites,
				workspace.PermManageMembers | workspace.PermAdministrator},
			// Renamed, not promoted: this is the check that would catch a
			// migration handing "ADMIN" the registry's admin.
			{"a role renamed off a reserved key", seed.reservedNameUserID,
				workspace.PermViewWorkbenches,
				workspace.PermManageMembers | workspace.PermManageInvites | workspace.PermAdministrator},
			{"the role that lost the key race", seed.collidingUserID,
				workspace.PermCreateWorkbench,
				workspace.PermManageInvites | workspace.PermAdministrator},
		} {
			t.Run(tc.name, func(t *testing.T) {
				st, err := svc.Standing(ctx, seed.workspaceID, tc.userID)
				if err != nil {
					t.Fatalf("Standing: %v", err)
				}
				if !st.IsMember {
					t.Fatal("a migrated member is not a member")
				}
				// Membership itself, which is never a stored grant.
				if !st.Permissions.Has(workspace.PermViewWorkspace) {
					t.Fatalf("permissions %#x carry no baseline", st.Permissions)
				}
				if !st.Permissions.Has(tc.holds) {
					t.Fatalf("permissions %#x are missing %#x", st.Permissions, tc.holds)
				}
				if tc.withheld != 0 && st.Permissions&tc.withheld != 0 {
					t.Fatalf("permissions %#x include %#x, which this role never had",
						st.Permissions, st.Permissions&tc.withheld)
				}
			})
		}
	})

	// A Down that has never been executed is not a rollback plan, it is a
	// hope. This walks all four back and asserts the schema the pre-authlayer
	// code needs is there again — the shapes, not the data: 0013 and 0015 say
	// plainly that reconstructing a bitmask from encoded grants is lossy, and
	// 0012 that a token whose flow has no column is not worth restoring.
	t.Run("the rollbacks leave a schema the old code can run against", func(t *testing.T) {
		for i := 0; i < 4; i++ {
			if err := m.Down(ctx); err != nil {
				t.Fatalf("rollback %d: %v", i, err)
			}
		}
		for _, want := range []struct{ table, column string }{
			{"users", "display_name"},
			{"sessions", "token_hash"},
			{"email_verifications", "new_email"},
			{"password_resets", "token_hash"},
			{"workspace_members", "workspace_id"},
			{"workspace_members", "role_id"},
			{"workspace_roles", "is_default"},
			{"workspace_roles", "permissions"},
			{"workspace_email_invitations", "role_id"},
			{"workspace_invite_links", "revoked"},
			{"workbench_members", "workbench_id"},
			{"workbench_members", "added_at"},
			{"workbench_roles", "is_default"},
			{"workspace_credits", "monthly_token_allowance"},
		} {
			var exists bool
			if err := sqlDB.QueryRowContext(ctx, `
				SELECT EXISTS (SELECT 1 FROM information_schema.columns
				               WHERE table_name = $1 AND column_name = $2)`,
				want.table, want.column).Scan(&exists); err != nil {
				t.Fatalf("information_schema: %v", err)
			}
			if !exists {
				t.Fatalf("after rolling back, %s.%s is missing", want.table, want.column)
			}
		}
		// 0014 pours the counters back into the credits row it recreates, so a
		// rolled-back deployment finds the spend where it left it.
		var spent int64
		if err := sqlDB.QueryRowContext(ctx,
			`SELECT current_cycle_tokens FROM workspace_credits WHERE workspace_id = $1`,
			seed.workspaceID).Scan(&spent); err != nil {
			t.Fatalf("workspace_credits after rollback: %v", err)
		}
		if spent != seed.spent {
			t.Fatalf("rolled-back spend = %d, want %d", spent, seed.spent)
		}
		for _, gone := range []string{"verifications", "workspace_subscriptions", "feature_usage"} {
			var exists bool
			if err := sqlDB.QueryRowContext(ctx,
				`SELECT to_regclass('public.' || $1) IS NOT NULL`, gone).Scan(&exists); err != nil {
				t.Fatalf("to_regclass(%s): %v", gone, err)
			}
			if exists {
				t.Fatalf("%s survived the rollback", gone)
			}
		}

		// Rolling 0013 or 0015 back is a ONE-WAY door for role assignments,
		// and running Up again here is how that was established: Down restores
		// role_id as an empty column (it says so, and says why — reconstructing
		// a bitmask from encoded grants is lossy), so the second Up finds
		// members it cannot map to a key and stops at role_key's NOT NULL.
		// That is the documented trade rather than a defect, so this asserts
		// the refusal instead of pretending the round trip works.
		if err := m.Up(ctx); err == nil {
			t.Fatal("0013 re-applied over rolled-back members; either Down now restores " +
				"the role mapping or Up stopped requiring one — the migrations' docs say neither")
		}

		// Which leaves the database mid-rollback, and the store contracts share
		// it. Hand it back empty so whatever runs next migrates from scratch.
		resetSchema(t, sqlDB)
	})

}

// migratorThrough0011 is the chain as it stood before authlayer: the FS-based
// 0001 plus the ten programmatic migrations, in version order. Building it
// here rather than calling postgres.New is the whole point — New runs
// everything, and this test needs the schema to stop where the data was
// written.
func migratorThrough0011(db *pg.DB) *pg.Migrator {
	m := pg.NewMigrator(db)
	if err := m.AddFS(migrations.Files, "."); err != nil {
		panic(err)
	}
	m.Add(migrateWorkspaces())
	m.Add(migrateWorkbenches())
	m.Add(migrateAgent())
	m.Add(migrateAgentCreditsBackfill())
	m.Add(migrateClientProfiles())
	m.Add(migrateAgentTenderResults())
	m.Add(migrateLowercaseEmails())
	m.Add(migrateBids())
	m.Add(migrateCompany())
	m.Add(migrateBidDecision())
	return m
}

type legacySeed struct {
	workspaceID           string
	creditlessWorkspaceID string
	defaultWorkspaceID    string
	workbenchID           string
	adminUserID           string
	memberUserID          string
	customUserID          string
	reservedNameUserID    string
	collidingUserID       string
	sessionID             string
	emailVerificationID   string
	passwordResetID       string
	expiredVerificationID string
	inviteID              string
	revokedLinkID         string
	liveLinkID            string
	cycleStart            time.Time
	allowance             int64
	spent                 int64
}

// seedLegacyRows writes one of everything the four migrations have to carry:
// the two roles CreateWorkspace seeded plus a custom one, a member on each, a
// live and an expired token of both kinds, a revoked and a live invite link,
// and a credits row whose allowance had been moved off the default.
func seedLegacyRows(t *testing.T, sqlDB *sql.DB) legacySeed {
	t.Helper()
	ctx := context.Background()
	s := legacySeed{
		cycleStart: time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		allowance:  5_000_000,
		spent:      1_234_567,
	}

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := sqlDB.ExecContext(ctx, query, args...); err != nil {
			t.Fatalf("seed %q: %v", query, err)
		}
	}
	scan := func(dest *string, query string, args ...any) {
		t.Helper()
		if err := sqlDB.QueryRowContext(ctx, query, args...).Scan(dest); err != nil {
			t.Fatalf("seed %q: %v", query, err)
		}
	}

	scan(&s.adminUserID, `INSERT INTO users (email, password_hash, display_name)
		VALUES ('owner@example.com', 'x', 'Owner') RETURNING id`)
	scan(&s.memberUserID, `INSERT INTO users (email, password_hash, display_name)
		VALUES ('member@example.com', 'x', 'Member') RETURNING id`)
	scan(&s.customUserID, `INSERT INTO users (email, password_hash, display_name)
		VALUES ('custom@example.com', 'x', 'Custom') RETURNING id`)
	scan(&s.reservedNameUserID, `INSERT INTO users (email, password_hash, display_name)
		VALUES ('reserved@example.com', 'x', 'Reserved') RETURNING id`)
	scan(&s.collidingUserID, `INSERT INTO users (email, password_hash, display_name)
		VALUES ('colliding@example.com', 'x', 'Colliding') RETURNING id`)

	scan(&s.sessionID, `INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, 'live-session', now() + interval '1 day') RETURNING id`, s.adminUserID)
	scan(&s.emailVerificationID, `INSERT INTO email_verifications (user_id, new_email, token_hash, expires_at)
		VALUES ($1, 'New@Example.com ', 'live-verification', now() + interval '1 hour') RETURNING id`, s.adminUserID)
	scan(&s.expiredVerificationID, `INSERT INTO email_verifications (user_id, new_email, token_hash, expires_at)
		VALUES ($1, 'stale@example.com', 'dead-verification', now() - interval '1 hour') RETURNING id`, s.adminUserID)
	scan(&s.passwordResetID, `INSERT INTO password_resets (user_id, token_hash, expires_at)
		VALUES ($1, 'live-reset', now() + interval '1 hour') RETURNING id`, s.adminUserID)

	scan(&s.workspaceID, `INSERT INTO workspaces (name, slug, owner_id)
		VALUES ('Acme', 'acme', $1) RETURNING id`, s.adminUserID)
	scan(&s.creditlessWorkspaceID, `INSERT INTO workspaces (name, slug, owner_id)
		VALUES ('No Credits', 'no-credits', $1) RETURNING id`, s.adminUserID)
	scan(&s.defaultWorkspaceID, `INSERT INTO workspaces (name, slug, owner_id)
		VALUES ('Default Allowance', 'default-allowance', $1) RETURNING id`, s.adminUserID)

	// The two rows the old CreateWorkspace seeded, at exactly the masks it
	// wrote — that is what identifies them as seeded rather than custom.
	var adminRoleID, memberRoleID, customRoleID string
	scan(&adminRoleID, `INSERT INTO workspace_roles (workspace_id, name, permissions, is_default)
		VALUES ($1, 'Admin', $2, false) RETURNING id`, s.workspaceID, legacySeededAdminMask)
	scan(&memberRoleID, `INSERT INTO workspace_roles (workspace_id, name, permissions, is_default)
		VALUES ($1, 'Member', $2, true) RETURNING id`, s.workspaceID, legacySeededMemberMask)
	scan(&customRoleID, `INSERT INTO workspace_roles (workspace_id, name, permissions, is_default)
		VALUES ($1, 'Bid Team', $2, false) RETURNING id`,
		s.workspaceID, int64(workspace.PermViewWorkbenches|workspace.PermManageInvites))
	// A role somebody named "ADMIN" that is NOT the seeded one: it carries its
	// own permissions, and promoting it to the registry's admin would hand out
	// access nobody granted. The spelling differs from the seeded "Admin"
	// because it has to: 0002's UNIQUE (workspace_id, name) makes an exact
	// duplicate unreachable, so a name that merely DERIVES to a reserved key is
	// the only shape this branch can ever see.
	var reservedNameRoleID, collidingRoleID string
	scan(&reservedNameRoleID, `INSERT INTO workspace_roles (workspace_id, name, permissions, is_default)
		VALUES ($1, 'ADMIN', $2, false) RETURNING id`,
		s.workspaceID, int64(workspace.PermViewWorkbenches))
	// A second name that derives to the key "Bid Team" already claimed. The new
	// (container_id, key) unique is what turns a double-insert into
	// ErrRoleKeyTaken, so two rows must not both claim it.
	scan(&collidingRoleID, `INSERT INTO workspace_roles (workspace_id, name, permissions, is_default)
		VALUES ($1, 'bid  team', $2, false) RETURNING id`,
		s.workspaceID, int64(workspace.PermCreateWorkbench))

	exec(`INSERT INTO workspace_members (workspace_id, user_id, role_id) VALUES ($1, $2, $3)`,
		s.workspaceID, s.adminUserID, adminRoleID)
	exec(`INSERT INTO workspace_members (workspace_id, user_id, role_id) VALUES ($1, $2, $3)`,
		s.workspaceID, s.memberUserID, memberRoleID)
	exec(`INSERT INTO workspace_members (workspace_id, user_id, role_id) VALUES ($1, $2, $3)`,
		s.workspaceID, s.customUserID, customRoleID)
	exec(`INSERT INTO workspace_members (workspace_id, user_id, role_id) VALUES ($1, $2, $3)`,
		s.workspaceID, s.reservedNameUserID, reservedNameRoleID)
	exec(`INSERT INTO workspace_members (workspace_id, user_id, role_id) VALUES ($1, $2, $3)`,
		s.workspaceID, s.collidingUserID, collidingRoleID)

	scan(&s.inviteID, `INSERT INTO workspace_email_invitations
		(workspace_id, email, role_id, token_hash, invited_by, expires_at)
		VALUES ($1, 'invitee@example.com', $2, 'invite-token', $3, now() + interval '7 days') RETURNING id`,
		s.workspaceID, memberRoleID, s.adminUserID)
	scan(&s.revokedLinkID, `INSERT INTO workspace_invite_links
		(workspace_id, code, role_id, created_by, revoked)
		VALUES ($1, 'revoked-code', $2, $3, true) RETURNING id`,
		s.workspaceID, memberRoleID, s.adminUserID)
	scan(&s.liveLinkID, `INSERT INTO workspace_invite_links
		(workspace_id, code, role_id, created_by, revoked)
		VALUES ($1, 'live-code', $2, $3, false) RETURNING id`,
		s.workspaceID, memberRoleID, s.adminUserID)

	scan(&s.workbenchID, `INSERT INTO workbenches (workspace_id, name, owner_id)
		VALUES ($1, 'Q3 bids', $2) RETURNING id`, s.workspaceID, s.adminUserID)
	var managerRoleID, viewerRoleID string
	scan(&managerRoleID, `INSERT INTO workbench_roles (workbench_id, name, permissions, is_default)
		VALUES ($1, 'Manager', $2, false) RETURNING id`, s.workbenchID, legacySeededManagerMask)
	scan(&viewerRoleID, `INSERT INTO workbench_roles (workbench_id, name, permissions, is_default)
		VALUES ($1, 'Viewer', $2, true) RETURNING id`, s.workbenchID, legacySeededViewerMask)
	exec(`INSERT INTO workbench_members (workbench_id, user_id, role_id) VALUES ($1, $2, $3)`,
		s.workbenchID, s.adminUserID, managerRoleID)
	exec(`INSERT INTO workbench_members (workbench_id, user_id, role_id) VALUES ($1, $2, $3)`,
		s.workbenchID, s.memberUserID, viewerRoleID)

	// One workspace with an allowance moved off the default and a month
	// half spent; the other with no credits row at all, which is what a
	// workspace created before 0004's backfill (or after a failed one) looks
	// like. Both are written by hand rather than left to 0004's seed: these
	// workspaces are inserted after that migration has already run.
	exec(`INSERT INTO workspace_credits
		(workspace_id, monthly_token_allowance, current_cycle_start, current_cycle_tokens)
		VALUES ($1, $2, $3, $4)`, s.workspaceID, s.allowance, s.cycleStart, s.spent)
	// The third shape: a credits row at exactly the default allowance with
	// nothing spent. It must keep its anchor like the others and pick up
	// neither a grant (its allowance is the plan's) nor a usage row (there is
	// no spend to carry).
	exec(`INSERT INTO workspace_credits
		(workspace_id, monthly_token_allowance, current_cycle_start, current_cycle_tokens)
		VALUES ($1, $2, $3, 0)`, s.defaultWorkspaceID, features.FreeMonthlyTokenAllowance, s.cycleStart)

	return s
}

func assertRoleKey(t *testing.T, sqlDB *sql.DB, table, userID, want string) {
	t.Helper()
	var got string
	if err := sqlDB.QueryRowContext(context.Background(),
		`SELECT role_key FROM `+table+` WHERE user_id = $1`, userID).Scan(&got); err != nil {
		t.Fatalf("%s role_key for %s: %v", table, userID, err)
	}
	if got != want {
		t.Fatalf("%s role_key = %q, want %q", table, got, want)
	}
}

// freshDatabase opens dsn and resets the public schema, so the partial chain
// above starts from nothing rather than from whatever a previous run left.
func freshDatabase(t *testing.T, dsn string) (*pg.DB, *sql.DB) {
	t.Helper()
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	cfg.RuntimeParams["search_path"] = "public,tenders"
	sqlDB, err := sql.Open("pgx", pgxstdlib.RegisterConnConfig(cfg))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.PingContext(context.Background()); err != nil {
		t.Fatalf("ping: %v", err)
	}
	resetSchema(t, sqlDB)
	return pg.New(stdlib.New(sqlDB)), sqlDB
}

// resetSchema empties the database. Every caller of it is a test that owns the
// scratch database outright — see the DSN's own doc — and none of them can
// share a schema with rows a previous run left.
func resetSchema(t *testing.T, sqlDB *sql.DB) {
	t.Helper()
	for _, s := range []string{`DROP SCHEMA public CASCADE`, `CREATE SCHEMA public`} {
		if _, err := sqlDB.ExecContext(context.Background(), s); err != nil {
			t.Fatalf("reset schema (%s): %v", s, err)
		}
	}
}
