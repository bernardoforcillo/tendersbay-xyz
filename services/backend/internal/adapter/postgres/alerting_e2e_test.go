package postgres_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/email"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/alerting"
)

// captureMailer records what would have been sent. It stands in for the Resend
// sender, which has its own tests; what this file is for is the SEAMS between
// the repository, the domain rule and the watermark — none of which any other
// test crosses.
type captureMailer struct {
	mu   sync.Mutex
	sent []alerting.Reminder
	to   []alerting.Recipient
}

func (m *captureMailer) SendReminder(_ context.Context, to alerting.Recipient, r alerting.Reminder) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, r)
	m.to = append(m.to, to)
	return nil
}

// TestReminderPassEndToEnd runs the real service over the real repository
// against a real database, and asserts the property that no unit test can: that
// a SECOND pass sends nothing.
//
// That is the one behaviour most likely to be wrong and most damaging if it is.
// The CronJob fires daily, the domain buckets, the repository stores a
// watermark, and a mistake anywhere in that chain means a user gets the same
// reminder every single day until their deadline passes.
func TestReminderPassEndToEnd(t *testing.T) {
	repo, db := testAlertingRepo(t)
	ctx := context.Background()
	bidID, userID := seedReminderFixture(t, db, time.Now().Add(5*24*time.Hour))

	// Give the user a locale, so this also proves the column added for
	// reminders reaches the mailer rather than stopping at the query.
	if _, err := db.ExecContext(ctx, `UPDATE users SET locale = 'it-it' WHERE id = $1`, userID); err != nil {
		t.Fatalf("set locale: %v", err)
	}

	mailer := &captureMailer{}
	svc := alerting.NewService(repo, mailer)

	rep, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if rep.Sent < 1 {
		t.Fatalf("first pass sent nothing: %+v", rep)
	}

	var mine *alerting.Reminder
	var to alerting.Recipient
	for i, r := range mailer.sent {
		if r.Candidate.BidID == bidID {
			mine, to = &mailer.sent[i], mailer.to[i]
		}
	}
	if mine == nil {
		t.Fatal("the seeded bid produced no reminder — the chain is broken somewhere between the query and the rule")
	}
	if mine.Bucket != 7 {
		t.Errorf("bucket = %d, want 7 for a deadline five days out", mine.Bucket)
	}
	if to.Locale != "it-it" {
		t.Errorf("recipient locale = %q, want it-it — the stored locale is not reaching the mailer", to.Locale)
	}
	if to.UnsubscribeToken == "" {
		t.Error("no unsubscribe token reached the mailer, so the send would be refused")
	}
	// The candidate must carry what the mail needs to deep-link. A missing
	// workspace id renders a URL that 404s, which no unit test would catch
	// because both halves are individually valid.
	if mine.Candidate.WorkspaceID == "" || mine.Candidate.WorkbenchID == "" {
		t.Errorf("candidate cannot build a deep link: %+v", mine.Candidate)
	}
	if mine.Candidate.TenderTitle == "" {
		t.Error("candidate has no tender title, so the subject line would be empty")
	}

	// THE ASSERTION THIS FILE EXISTS FOR.
	before := len(mailer.sent)
	if _, err := svc.Run(ctx); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	for _, r := range mailer.sent[before:] {
		if r.Candidate.BidID == bidID {
			t.Fatal("the same bid was reminded twice in a row — the watermark is not stopping a re-run, " +
				"which on a daily CronJob means one mail per day until the deadline")
		}
	}

	// And the watermark is what did it, not luck.
	if got := watermarkOf(t, db, bidID); got != 7 {
		t.Errorf("watermark = %d, want 7", got)
	}
}

// TestReminderPassRespectsUnsubscribe walks the whole opt-out path: a reminder
// goes out, the user unsubscribes through the same token the mail carried, and
// the next pass leaves them alone.
func TestReminderPassRespectsUnsubscribe(t *testing.T) {
	repo, db := testAlertingRepo(t)
	ctx := context.Background()
	bidID, _ := seedReminderFixture(t, db, time.Now().Add(5*24*time.Hour))

	mailer := &captureMailer{}
	svc := alerting.NewService(repo, mailer)
	if _, err := svc.Run(ctx); err != nil {
		t.Fatalf("first pass: %v", err)
	}

	var token string
	for i, r := range mailer.sent {
		if r.Candidate.BidID == bidID {
			token = mailer.to[i].UnsubscribeToken
		}
	}
	if token == "" {
		t.Fatal("no reminder for the seeded bid")
	}

	// Unsubscribe with exactly the token the mail carried — the same value the
	// endpoint would receive from the link.
	if err := svc.Unsubscribe(ctx, token); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}

	// Re-arm the bid so the ONLY thing that can stop the next reminder is the
	// opt-out, not the watermark. Otherwise this test would pass even if
	// unsubscribing did nothing at all.
	if _, err := db.ExecContext(ctx, `UPDATE bids SET last_reminded_bucket = 0 WHERE id = $1`, bidID); err != nil {
		t.Fatalf("re-arm: %v", err)
	}

	before := len(mailer.sent)
	rep, err := svc.Run(ctx)
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	for _, r := range mailer.sent[before:] {
		if r.Candidate.BidID == bidID {
			t.Fatal("an unsubscribed user was mailed again")
		}
	}
	if rep.Suppressed < 1 {
		t.Errorf("report = %+v, want the bid counted as suppressed rather than skipped silently", rep)
	}
}

// TestReminderRendersThroughTheRealSender joins the last seam: the candidate
// the repository produced actually renders in the real Resend sender. It posts
// to a local server instead of Resend, so nothing leaves the machine.
func TestReminderRendersThroughTheRealSender(t *testing.T) {
	repo, db := testAlertingRepo(t)
	ctx := context.Background()
	_, userID := seedReminderFixture(t, db, time.Now().Add(5*24*time.Hour))
	if _, err := db.ExecContext(ctx, `UPDATE users SET locale = 'it-it' WHERE id = $1`, userID); err != nil {
		t.Fatalf("set locale: %v", err)
	}

	srv, payload := capturePost(t)
	sender, err := email.NewReminderWithURL("k", "avvisi@mail.tendersbay.xyz", email.TransactionalFrom,
		"https://tendersbay.xyz/unsubscribe", "https://tendersbay.xyz", srv.URL)
	if err != nil {
		t.Fatalf("NewReminderWithURL: %v", err)
	}

	if _, err := alerting.NewService(repo, sender).Run(ctx); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if payload.To == "" {
		t.Fatal("nothing reached the sender")
	}
	if !strings.Contains(payload.Subject, "Scadenza") {
		t.Errorf("subject %q is not Italian — the stored locale did not survive the whole chain", payload.Subject)
	}
	if !strings.Contains(payload.HTML, "/workspaces/") {
		t.Errorf("body has no deep link: %s", payload.HTML)
	}
}

// sentMail mirrors Resend's request body. It is declared here rather than
// reusing the email package's own type because that one is unexported — and
// deliberately so: an external test asserting on the shape of the request is
// exactly the coupling that would stop that package from changing it.
type sentMail struct {
	From    string            `json:"from"`
	To      string            `json:"to"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html"`
	Headers map[string]string `json:"headers"`
}

func capturePost(t *testing.T) (*httptest.Server, *sentMail) {
	t.Helper()
	var got sentMail
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}
