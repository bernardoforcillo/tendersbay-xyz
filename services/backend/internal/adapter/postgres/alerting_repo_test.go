package postgres_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

func testAlertingRepo(t *testing.T) (*postgres.AlertingRepo, *sql.DB) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, sqlDB, err := postgres.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	return postgres.NewAlertingRepo(db), sqlDB
}

// seedReminderFixture builds the whole chain a candidate needs: a verified
// user, a workspace, a workbench with that user as a member, an ingested tender
// with a deadline, and a bid on it. Returns the bid id and the user id.
func seedReminderFixture(t *testing.T, db *sql.DB, deadline time.Time) (bidID, userID string) {
	t.Helper()
	ctx := context.Background()
	must := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("seed %q: %v", q, err)
		}
	}
	row := func(q string, args ...any) string {
		t.Helper()
		var id string
		if err := db.QueryRowContext(ctx, q, args...).Scan(&id); err != nil {
			t.Fatalf("seed scan %q: %v", q, err)
		}
		return id
	}

	userID = row(`INSERT INTO users (id, email, password_hash, display_name, email_verified_at, created_at, updated_at)
	              VALUES (gen_random_uuid(), 'rem-'||gen_random_uuid()||'@example.com', 'x', 'Tester', now(), now(), now())
	              RETURNING id`)
	wsID := row(`INSERT INTO workspaces (name, slug, owner_id)
	             VALUES ('ws', 'ws-'||gen_random_uuid(), $1) RETURNING id`, userID)
	wbID := row(`INSERT INTO workbenches (workspace_id, name, owner_id)
	             VALUES ($1, 'wb', $2) RETURNING id`, wsID, userID)
	roleID := row(`INSERT INTO workbench_roles (workbench_id, name)
	               VALUES ($1, 'owner') RETURNING id`, wbID)
	must(`INSERT INTO workbench_members (workbench_id, user_id, role_id) VALUES ($1,$2,$3)`, wbID, userID, roleID)

	tenderID := row(`INSERT INTO tenders.ingested_tenders (source, source_ref, title, status, deadline)
	                 VALUES ('test','rem-'||gen_random_uuid(),'Servizio di pulizia','open',$1) RETURNING id`, deadline)
	bidID = row(`INSERT INTO bids (workbench_id, tender_id, created_by)
	             VALUES ($1, $2, $3) RETURNING id`, wbID, tenderID, userID)

	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		_, _ = db.ExecContext(ctx, `DELETE FROM tenders.ingested_tenders WHERE id = $1`, tenderID)
	})
	return bidID, userID
}

func TestAlertingRepo_CandidatesRecipientsAndWatermark(t *testing.T) {
	repo, db := testAlertingRepo(t)
	ctx := context.Background()
	bidID, userID := seedReminderFixture(t, db, time.Now().Add(5*24*time.Hour))

	listed := func() bool {
		t.Helper()
		all, err := repo.ListDueCandidates(ctx)
		if err != nil {
			t.Fatalf("ListDueCandidates: %v", err)
		}
		for _, x := range all {
			if x.BidID == bidID {
				return true
			}
		}
		return false
	}

	if !listed() {
		t.Fatal("a bid five days from its deadline was not returned as a candidate")
	}

	// A member with no preferences row is a SUBSCRIBER — they have never been
	// asked, and the first reminder is where they get the chance to say no.
	// The token must be minted on the way out.
	recips, err := repo.RecipientsFor(ctx, workbenchOf(t, db, bidID))
	if err != nil {
		t.Fatalf("RecipientsFor: %v", err)
	}
	if len(recips) != 1 {
		t.Fatalf("got %d recipients, want 1", len(recips))
	}
	if recips[0].UnsubscribeToken == "" {
		t.Fatal("no unsubscribe token was minted — the reminder would be refused")
	}
	first := recips[0].UnsubscribeToken

	// Minting is idempotent: a second pass must not rotate the token, or the
	// link in an already-delivered mail would stop working.
	again, _ := repo.RecipientsFor(ctx, workbenchOf(t, db, bidID))
	if len(again) != 1 || again[0].UnsubscribeToken != first {
		t.Errorf("token rotated between passes (%q -> %+v); old mail links would break", first, again)
	}

	// Opting out removes them entirely.
	if _, err := db.ExecContext(ctx,
		`UPDATE reminder_preferences SET opted_out_at = now() WHERE user_id = $1`, userID); err != nil {
		t.Fatalf("opt out: %v", err)
	}
	after, _ := repo.RecipientsFor(ctx, workbenchOf(t, db, bidID))
	if len(after) != 0 {
		t.Errorf("an opted-out member is still a recipient: %+v", after)
	}

	// The watermark only moves toward the deadline.
	if err := repo.MarkReminded(ctx, bidID, 7); err != nil {
		t.Fatalf("MarkReminded: %v", err)
	}
	if got := watermarkOf(t, db, bidID); got != 7 {
		t.Fatalf("watermark = %d, want 7", got)
	}
	if err := repo.MarkReminded(ctx, bidID, 14); err != nil {
		t.Fatalf("MarkReminded (widen): %v", err)
	}
	if got := watermarkOf(t, db, bidID); got != 7 {
		t.Errorf("watermark widened back to %d — an out-of-order pass re-armed a sent bucket", got)
	}
	if err := repo.MarkReminded(ctx, bidID, 3); err != nil {
		t.Fatalf("MarkReminded (narrow): %v", err)
	}
	if got := watermarkOf(t, db, bidID); got != 3 {
		t.Errorf("watermark = %d, want 3", got)
	}
}

// TestAlertingRepo_SuppressesClosedAndSubmitted pins the coarse filter: the two
// states that can never become due again must not reach the domain at all.
func TestAlertingRepo_SuppressesClosedAndSubmitted(t *testing.T) {
	repo, db := testAlertingRepo(t)
	ctx := context.Background()
	bidID, _ := seedReminderFixture(t, db, time.Now().Add(5*24*time.Hour))

	present := func() bool {
		all, err := repo.ListDueCandidates(ctx)
		if err != nil {
			t.Fatalf("ListDueCandidates: %v", err)
		}
		for _, c := range all {
			if c.BidID == bidID {
				return true
			}
		}
		return false
	}

	if _, err := db.ExecContext(ctx, `UPDATE bids SET stage = $2 WHERE id = $1`, bidID, string(bid.StageSubmitted)); err != nil {
		t.Fatalf("set stage: %v", err)
	}
	if present() {
		t.Error("a submitted bid is still a candidate")
	}
	if _, err := db.ExecContext(ctx,
		`UPDATE bids SET stage = 'preparing', outcome = $2 WHERE id = $1`, bidID, string(bid.OutcomeWon)); err != nil {
		t.Fatalf("set outcome: %v", err)
	}
	if present() {
		t.Error("a closed bid is still a candidate")
	}
}

func workbenchOf(t *testing.T, db *sql.DB, bidID string) string {
	t.Helper()
	var wb string
	if err := db.QueryRow(`SELECT workbench_id FROM bids WHERE id = $1`, bidID).Scan(&wb); err != nil {
		t.Fatalf("workbench of bid: %v", err)
	}
	return wb
}

func watermarkOf(t *testing.T, db *sql.DB, bidID string) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRow(`SELECT last_reminded_bucket FROM bids WHERE id = $1`, bidID).Scan(&n); err != nil {
		t.Fatalf("watermark: %v", err)
	}
	return n
}
