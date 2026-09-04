package postgres_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/authlayer/auth"
	"github.com/bernardoforcillo/authlayer/auth/authtest"
	"github.com/bernardoforcillo/authlayer/invite"
	"github.com/bernardoforcillo/authlayer/invite/invitetest"
	dropsstore "github.com/bernardoforcillo/authlayer/store/drops"
	"github.com/bernardoforcillo/drops/pg"
	"github.com/bernardoforcillo/featurelayer/entitlement"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/features"
)

// This file checks the migrated schema against the libraries' own definition of
// correct, rather than against a second opinion written here.
//
// The unit tests elsewhere in the tree run authlayer and featurelayer over
// their in-memory stores, which proves the domain layer's rules and nothing at
// all about migrations 0012–0015: those reshape tables by hand, and a column
// they name wrongly fails first in production. What follows only runs with a
// database.

// authContractDSN is deliberately NOT TEST_DATABASE_URL. authtest's factory
// contract requires a store whose users, sessions and verifications are EMPTY
// before every sub-check, which here means truncating them — and users cascades
// to workspaces, workbenches, bids and everything else in the tree. Pointing
// that at the database the rest of the suite seeds into would delete its
// fixtures, and pointing it at a developer's own database would delete their
// work. So it takes a scratch database, named separately and on purpose.
const authContractDSN = "TEST_AUTH_CONTRACT_DATABASE_URL"

// The migrated auth schema has to satisfy authlayer's port, not merely resemble
// it: eleven of auth.Store's methods carry an explicit MUST, several of them
// because violating one reopens a named security hole (an enumeration oracle at
// sign-up, two successful rotations of one refresh token, a revoked family
// resurrected by a racing rotation). authtest is the executable form of those
// obligations, and running it here is the only check that 0012's hand-written
// ALTERs left a schema they hold on.
func TestMigratedAuthSchemaSatisfiesTheAuthlayerContract(t *testing.T) {
	db, sqlDB := scratchDB(t)
	authtest.RunStoreContract(t, func(t *testing.T) auth.Store {
		skipCascadingDeleteCheck(t)
		// The factory is called once per sub-check with that check's own *T,
		// so this runs between checks rather than once for the suite.
		truncateAll(t, sqlDB, "verifications, sessions, users")
		return referencedUserStore{Store: dropsstore.NewAuthStore(db), sqlDB: sqlDB}
	})
}

// The migrated invitation tables get the same treatment, and for the same
// reason: 0013 renamed their columns by hand, and invite.Store's obligations —
// one emailed token paying out at most once, a MaxUses:1 link admitting exactly
// one redeemer, three uniqueness constraints each keeping a lookup resolved to
// one row — are what those columns are for.
func TestMigratedInviteTablesSatisfyTheAuthlayerContract(t *testing.T) {
	db, sqlDB := scratchDB(t)
	invitetest.RunStoreContract(t, func(t *testing.T) invite.Store {
		truncateAll(t, sqlDB, "workspace_email_invitations, workspace_invite_links, workspaces, users")
		return referencedWorkspaceStore{Store: postgres.NewWorkspaceInviteStore(db), sqlDB: sqlDB}
	})
}

// Neither suite creates the rows our foreign keys point at, and that is not an
// oversight on their part: authlayer's own CreateSchema declares no foreign key
// to a users table at all, so its ports treat a user id (and a container id) as
// an opaque string. This product's schema is stricter — sessions, verifications
// and both invitation tables have referenced users since 0001, and the
// invitation tables reference workspaces — so a suite that mints fresh ids has
// every insert rejected by a constraint doing exactly its job.
//
// The two wrappers below close that gap the honest way: they supply the
// referenced row and change nothing else. Every obligation the contract asserts
// is still asserted against the real migrated tables, with the real
// constraints, through the real store. Only the rows our schema requires and
// the library's does not are added, and only on the three writes that carry a
// user id and the two that carry a container id.

// cascadingDeleteCheck is the one obligation this schema deliberately does not
// meet. The port says DeleteUser removes the user row ONLY and leaves the
// cascade to the caller, so a store that quietly takes the sessions with it is
// doing more than it was asked. Ours does: sessions.user_id and
// verifications.user_id have carried ON DELETE CASCADE since 0001, and 0012
// kept them.
//
// That is the safer direction for this product and it is staying. auth.Service
// already deletes the sessions and verifications itself before the user, so the
// outcome is identical on the path that runs in production; what the cascade
// adds is that a future delete path which forgets cannot leave a live refresh
// token behind for an account that no longer exists. Dropping the constraint to
// pass one check would trade a real guarantee for a green line.
//
// The suite's own doc invites this: a factory is free to skip. Skipping it here
// keeps the other checks — every MUST that IS about the migrated columns —
// running and honest.
const cascadingDeleteCheck = "DeleteUser/RemovesTheUserRowOnly"

func skipCascadingDeleteCheck(t *testing.T) {
	t.Helper()
	if strings.HasSuffix(t.Name(), cascadingDeleteCheck) {
		t.Skip("this schema cascades sessions and verifications on DeleteUser by design; see cascadingDeleteCheck")
	}
}

type referencedUserStore struct {
	auth.Store
	sqlDB *sql.DB
}

func (s referencedUserStore) CreateSession(ctx context.Context, sess auth.Session) (auth.Session, error) {
	if err := ensureUser(ctx, s.sqlDB, sess.UserID); err != nil {
		return auth.Session{}, err
	}
	return s.Store.CreateSession(ctx, sess)
}

func (s referencedUserStore) CreateSuccessorSession(ctx context.Context, predecessorID string, sess auth.Session) (auth.Session, bool, error) {
	if err := ensureUser(ctx, s.sqlDB, sess.UserID); err != nil {
		return auth.Session{}, false, err
	}
	return s.Store.CreateSuccessorSession(ctx, predecessorID, sess)
}

func (s referencedUserStore) CreateVerification(ctx context.Context, v auth.Verification) (auth.Verification, error) {
	if err := ensureUser(ctx, s.sqlDB, v.UserID); err != nil {
		return auth.Verification{}, err
	}
	return s.Store.CreateVerification(ctx, v)
}

type referencedWorkspaceStore struct {
	invite.Store
	sqlDB *sql.DB
}

func (s referencedWorkspaceStore) CreateEmailInvite(ctx context.Context, inv invite.EmailInvite) (invite.EmailInvite, error) {
	if err := ensureUser(ctx, s.sqlDB, inv.InvitedBy); err != nil {
		return invite.EmailInvite{}, err
	}
	if err := ensureWorkspace(ctx, s.sqlDB, inv.ContainerID); err != nil {
		return invite.EmailInvite{}, err
	}
	return s.Store.CreateEmailInvite(ctx, inv)
}

func (s referencedWorkspaceStore) CreateLink(ctx context.Context, l invite.Link) (invite.Link, error) {
	if err := ensureUser(ctx, s.sqlDB, l.CreatedBy); err != nil {
		return invite.Link{}, err
	}
	if err := ensureWorkspace(ctx, s.sqlDB, l.ContainerID); err != nil {
		return invite.Link{}, err
	}
	return s.Store.CreateLink(ctx, l)
}

// ensureUser inserts the users row a foreign key needs, keyed by the id the
// contract chose. The address is derived from that id because users.email is
// unique and the suite has no opinion about it.
func ensureUser(ctx context.Context, sqlDB *sql.DB, id string) error {
	if id == "" {
		return nil
	}
	_, err := sqlDB.ExecContext(ctx,
		`INSERT INTO users (id, email, password_hash, display_name)
		 VALUES ($1, $2, 'x', 'Contract Fixture') ON CONFLICT DO NOTHING`,
		id, "contract-"+id+"@example.invalid")
	return err
}

// ensureWorkspace does the same for a container id, minting the owner the
// workspace itself requires. name and slug are the id cast to text for the same
// reason the address is derived from it: slug is unique and the suite does not
// care what it says.
func ensureWorkspace(ctx context.Context, sqlDB *sql.DB, id string) error {
	if id == "" {
		return nil
	}
	var ownerID string
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ($1, 'x', 'Contract Fixture Owner')
		 ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING id`,
		"contract-owner@example.invalid").Scan(&ownerID); err != nil {
		return err
	}
	_, err := sqlDB.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, slug, owner_id) VALUES ($1::uuid, $1, $1, $2)
		 ON CONFLICT DO NOTHING`,
		id, ownerID)
	return err
}

// scratchDB opens the contract database and runs the migrations into it.
func scratchDB(t *testing.T) (*pg.DB, *sql.DB) {
	t.Helper()
	dsn := os.Getenv(authContractDSN)
	if dsn == "" {
		t.Skipf("%s not set (a SCRATCH database: these tests truncate users and everything referencing it)", authContractDSN)
	}
	db, sqlDB, err := postgres.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, sqlDB
}

// truncateAll empties the tables a contract owns before each of its checks.
// CASCADE is load-bearing and is why these tests insist on their own database:
// users is the root of nearly every foreign key in the schema.
func truncateAll(t *testing.T, sqlDB *sql.DB, tables string) {
	t.Helper()
	if _, err := sqlDB.ExecContext(context.Background(),
		"TRUNCATE "+tables+" RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// A walk through the invitation store over real seeded ids. The packaged
// contract above covers strictly more, but it needs a database it may truncate;
// this one runs in the shared TEST_DATABASE_URL alongside the rest of the
// suite, which is the setup CI actually has. It stays focused on what 0013
// touched: the normalization the (container_id, email) unique depends on, the
// rows-affected gate that pays an emailed token out once, and revoked_at, which
// replaced a boolean.
func TestMigratedInviteTablesRoundTrip(t *testing.T) {
	db, sqlDB := testDB(t)
	store := postgres.NewWorkspaceInviteStore(db)
	ctx := context.Background()

	workspaceID, userID := seedWorkspace(t, sqlDB, "invite-roundtrip-"+randomSuffix(t))

	// An emailed invitation, found by its hash and then paid out once.
	inv, err := store.CreateEmailInvite(ctx, invite.EmailInvite{
		ID:          newUUID(t, sqlDB),
		ContainerID: workspaceID,
		Email:       "Someone@Example.COM",
		RoleKey:     "member",
		TokenHash:   randomSuffix(t),
		InvitedBy:   userID,
		ExpiresAt:   time.Now().UTC().Add(time.Hour),
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateEmailInvite: %v", err)
	}
	// The store normalizes before writing, which is what makes the
	// (container_id, email) unique a constraint on a person rather than on a
	// spelling — and it only holds if the migration kept the column.
	if inv.Email != "someone@example.com" {
		t.Fatalf("stored email = %q, want it normalized", inv.Email)
	}

	found, err := store.FindEmailInviteByTokenHash(ctx, inv.TokenHash)
	if err != nil {
		t.Fatalf("FindEmailInviteByTokenHash: %v", err)
	}
	if found.ID != inv.ID || found.RoleKey != "member" || found.ContainerID != workspaceID {
		t.Fatalf("round-tripped invite = %+v", found)
	}

	list, err := store.ListEmailInvites(ctx, workspaceID)
	if err != nil {
		t.Fatalf("ListEmailInvites: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListEmailInvites returned %d rows, want 1", len(list))
	}

	// One emailed token pays out at most once: the second delete has no row
	// left to affect and must say so.
	if err := store.DeleteEmailInvite(ctx, inv.ID); err != nil {
		t.Fatalf("DeleteEmailInvite: %v", err)
	}
	if err := store.DeleteEmailInvite(ctx, inv.ID); err == nil {
		t.Fatal("deleting a spent invitation twice succeeded twice")
	}

	// A link, consumed up to its ceiling and then revoked.
	link, err := store.CreateLink(ctx, invite.Link{
		ID:          newUUID(t, sqlDB),
		ContainerID: workspaceID,
		Code:        randomSuffix(t),
		RoleKey:     "member",
		CreatedBy:   userID,
		MaxUses:     2,
		CreatedAt:   time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	for i := 1; i <= 2; i++ {
		ok, err := store.ConsumeLink(ctx, link.ID, time.Now().UTC())
		if err != nil || !ok {
			t.Fatalf("consume %d: ok=%v err=%v", i, ok, err)
		}
	}
	if ok, err := store.ConsumeLink(ctx, link.ID, time.Now().UTC()); err != nil || ok {
		t.Fatalf("a MaxUses:2 link admitted a third redeemer: ok=%v err=%v", ok, err)
	}

	// revoked_at replaced the old boolean in 0013; RevokeLink is what writes it.
	if err := store.RevokeLink(ctx, link.ID, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeLink: %v", err)
	}
	revoked, err := store.FindLinkByCode(ctx, link.Code)
	if err != nil {
		t.Fatalf("FindLinkByCode: %v", err)
	}
	if revoked.RevokedAt == nil {
		t.Fatal("revoked_at is still null after RevokeLink")
	}
	if revoked.UseCount != 2 {
		t.Fatalf("use_count = %d, want 2", revoked.UseCount)
	}
}

// featurelayer ships entitlement.MemUsage as the reference implementation of
// its UsageStore, including the exact behaviour its doc calls for: an increment
// is applied whole or not at all, and a refusal leaves the total where it was.
// FeatureUsageRepo has to answer the same way over SQL, and it reaches that
// answer differently — one ON CONFLICT statement with a guard, so two agent
// turns cannot both spend the last of a budget.
//
// So the reference is the oracle: the same operations run against both, and
// every step must agree. A ceiling the SQL enforces one token more generously
// than the library does is a real overspend, and nothing else in the suite
// would see it.
func TestFeatureUsageRepoAgreesWithTheReferenceStore(t *testing.T) {
	db, sqlDB := testDB(t)
	repo := postgres.NewFeatureUsageRepo(db)
	mem := entitlement.NewMemUsage()
	ctx := context.Background()

	workspaceID, _ := seedWorkspace(t, sqlDB, "feature-usage-"+randomSuffix(t))

	period, ok := features.MeterPeriod(features.AgentTokens)
	if !ok {
		t.Fatal("the agent budget declares no metered period")
	}
	key := entitlement.UsageKey{
		Tenant:  workspaceID,
		Feature: features.AgentTokens,
		Period:  entitlement.PeriodKey(period, time.Now().UTC(), time.Now().UTC()),
	}

	const max = 100
	steps := []struct {
		name        string
		delta, ceil int64
	}{
		{"first spend on an untouched counter", 30, max},
		{"a second that still fits", 60, max},
		{"one that would cross the ceiling", 20, max},
		{"the exact remainder", 10, max},
		{"anything at all once the budget is gone", 1, max},
		{"a single request larger than the whole allowance", max + 1, max},
		{"an unlimited feature ignores the ceiling", 5_000, -1},
	}
	for _, s := range steps {
		gotTotal, gotOK, err := repo.Increment(ctx, key, s.delta, s.ceil)
		if err != nil {
			t.Fatalf("%s: repo.Increment: %v", s.name, err)
		}
		wantTotal, wantOK, err := mem.Increment(ctx, key, s.delta, s.ceil)
		if err != nil {
			t.Fatalf("%s: reference.Increment: %v", s.name, err)
		}
		if gotTotal != wantTotal || gotOK != wantOK {
			t.Fatalf("%s: repo = (%d, %v), featurelayer's reference = (%d, %v)",
				s.name, gotTotal, gotOK, wantTotal, wantOK)
		}
	}

	got, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("repo.Get: %v", err)
	}
	want, err := mem.Get(ctx, key)
	if err != nil {
		t.Fatalf("reference.Get: %v", err)
	}
	if got != want {
		t.Fatalf("counter settled at %d, reference at %d", got, want)
	}
}

// A subscription is written as JSON columns and read back as featurelayer's own
// types, so the round trip is where a plan, an add-on or a per-tenant grant
// would quietly go missing — which is exactly what the 0014 backfill writes for
// a workspace whose allowance had been moved off the default.
func TestSubscriptionRepoRoundTripsAGrant(t *testing.T) {
	db, sqlDB := testDB(t)
	repo := postgres.NewSubscriptionRepo(db)
	ctx := context.Background()

	workspaceID, _ := seedWorkspace(t, sqlDB, "subscription-"+randomSuffix(t))

	period, ok := features.MeterPeriod(features.AgentTokens)
	if !ok {
		t.Fatal("the agent budget declares no metered period")
	}
	anchor := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	want := entitlement.Subscription{
		TenantID:      workspaceID,
		Plan:          features.PlanPro,
		AddOns:        []entitlement.AddOnID{features.AddOnExtraTokens},
		BillingAnchor: anchor,
		Grants: []entitlement.Grant{entitlement.Override(
			features.AgentTokens,
			&entitlement.Limit{Max: 3_000_000, Period: period},
			"allowance carried over from workspace_credits",
		)},
	}
	if err := repo.Upsert(ctx, want); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := repo.Subscription(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Subscription: %v", err)
	}
	if got.Plan != want.Plan || len(got.AddOns) != 1 || got.AddOns[0] != features.AddOnExtraTokens {
		t.Fatalf("plan/add-ons came back as %q / %v", got.Plan, got.AddOns)
	}
	if !got.BillingAnchor.Equal(anchor) {
		t.Fatalf("billing anchor = %s, want %s", got.BillingAnchor, anchor)
	}
	if len(got.Grants) != 1 || got.Grants[0].Limit == nil {
		t.Fatalf("grants came back as %+v", got.Grants)
	}
	if g := got.Grants[0]; g.Feature != features.AgentTokens || g.Limit.Max != 3_000_000 || g.Limit.Period != period {
		t.Fatalf("grant came back as %+v (limit %+v)", g, g.Limit)
	}

	// Upsert is called on every workspace-creation attempt, so a second one
	// must not hand the workspace a fresh period.
	want.Plan = features.PlanFree
	want.BillingAnchor = time.Now().UTC()
	if err := repo.Upsert(ctx, want); err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	got, err = repo.Subscription(ctx, workspaceID)
	if err != nil {
		t.Fatalf("Subscription after re-upsert: %v", err)
	}
	if !got.BillingAnchor.Equal(anchor) {
		t.Fatalf("re-upserting moved the billing anchor to %s", got.BillingAnchor)
	}
	if got.Plan != features.PlanFree {
		t.Fatalf("re-upserting left the plan at %q", got.Plan)
	}

	// An unknown workspace is entitled to nothing, and says so with the
	// sentinel featurelayer fails closed on.
	if _, err := repo.Subscription(ctx, newUUID(t, sqlDB)); !errors.Is(err, entitlement.ErrNoSubscription) {
		t.Fatalf("unknown workspace returned %v, want ErrNoSubscription", err)
	}
}

// testDB opens the shared test database. Unlike the auth contract above these
// tests only ever add rows of their own, so they run against TEST_DATABASE_URL
// alongside the rest of the suite.
func testDB(t *testing.T) (*pg.DB, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, sqlDB, err := postgres.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, sqlDB
}

// seedWorkspace creates a throwaway user and workspace and returns both ids.
// The invitation and usage tables all carry foreign keys to them.
func seedWorkspace(t *testing.T, sqlDB *sql.DB, slug string) (workspaceID, userID string) {
	t.Helper()
	ctx := context.Background()

	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash, display_name)
		 VALUES ($1, 'x', 'Authlayer Contract Test') RETURNING id`,
		slug+"@example.com",
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := sqlDB.QueryRowContext(ctx,
		`INSERT INTO workspaces (name, slug, owner_id) VALUES ($1, $1, $2) RETURNING id`,
		slug, userID,
	).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sqlDB.ExecContext(context.Background(), `DELETE FROM workspaces WHERE id = $1`, workspaceID)
		_, _ = sqlDB.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})
	return workspaceID, userID
}

// newUUID borrows the database's own generator so the ids these tests hand to
// authlayer's stores are the shape the uuid columns accept.
func newUUID(t *testing.T, sqlDB *sql.DB) string {
	t.Helper()
	var id string
	if err := sqlDB.QueryRowContext(context.Background(), `SELECT gen_random_uuid()`).Scan(&id); err != nil {
		t.Fatalf("gen_random_uuid: %v", err)
	}
	return id
}

// randomSuffix keeps slugs, addresses, token hashes and link codes unique
// across runs against a database the suite never truncates.
func randomSuffix(t *testing.T) string {
	t.Helper()
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("rand: %v", err)
	}
	return hex.EncodeToString(b[:])
}
