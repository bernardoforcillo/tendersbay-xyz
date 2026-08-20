package alerting

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
)

type fakeRepo struct {
	candidates []Candidate
	recipients map[string][]Recipient
	marked     map[string]int
	optedOut   []string
	optOutHit  bool
	optOutErr  error
	listErr    error
	recErr     error
	markErr    error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{recipients: map[string][]Recipient{}, marked: map[string]int{}}
}

func (f *fakeRepo) ListDueCandidates(context.Context) ([]Candidate, error) {
	return f.candidates, f.listErr
}
func (f *fakeRepo) RecipientsFor(_ context.Context, wb string) ([]Recipient, error) {
	return f.recipients[wb], f.recErr
}
func (f *fakeRepo) OptOut(_ context.Context, token string) (bool, error) {
	if f.optOutErr != nil {
		return false, f.optOutErr
	}
	f.optedOut = append(f.optedOut, token)
	return f.optOutHit, nil
}

func (f *fakeRepo) MarkReminded(_ context.Context, bidID string, bucket int) error {
	if f.markErr != nil {
		return f.markErr
	}
	f.marked[bidID] = bucket
	return nil
}

type fakeMailer struct {
	sent []Recipient
	err  error
}

func (m *fakeMailer) SendReminder(_ context.Context, to Recipient, _ Reminder) error {
	if m.err != nil {
		return m.err
	}
	m.sent = append(m.sent, to)
	return nil
}

func testService(repo Repo, mailer Mailer) *Service {
	s := NewService(repo, mailer)
	s.now = func() time.Time { return now }
	return s
}

func dueCandidate() Candidate {
	d := now.Add(5 * 24 * time.Hour)
	return Candidate{BidID: "b1", WorkbenchID: "wb1", Deadline: &d,
		Decision: bid.GoNoGoGo, Stage: bid.StagePreparing}
}

func subscriber() Recipient {
	return Recipient{Email: "a@example.com", Locale: "it-it", UnsubscribeToken: "tok"}
}

func TestRunSendsAndAdvancesWatermark(t *testing.T) {
	repo := newFakeRepo()
	repo.candidates = []Candidate{dueCandidate()}
	repo.recipients["wb1"] = []Recipient{subscriber()}
	mailer := &fakeMailer{}

	rep, err := testService(repo, mailer).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Due != 1 || rep.Sent != 1 {
		t.Fatalf("report = %+v, want Due=1 Sent=1", rep)
	}
	if repo.marked["b1"] != 7 {
		t.Errorf("watermark = %d, want 7", repo.marked["b1"])
	}
}

// TestRunRefusesRecipientWithoutUnsubscribeToken is a compliance guard, not a
// nicety: a marketing-adjacent message with no way out is both unlawful in the
// EU and the fastest route to password-reset mail being filtered as spam.
func TestRunRefusesRecipientWithoutUnsubscribeToken(t *testing.T) {
	repo := newFakeRepo()
	repo.candidates = []Candidate{dueCandidate()}
	repo.recipients["wb1"] = []Recipient{{Email: "a@example.com"}} // no token
	mailer := &fakeMailer{}

	rep, _ := testService(repo, mailer).Run(context.Background())
	if len(mailer.sent) != 0 {
		t.Fatal("mailed a recipient who had no unsubscribe token")
	}
	// Nobody was mailable, so it counts as suppressed and the watermark still
	// advances — the job must not reconsider this bid hourly forever.
	if rep.Suppressed != 1 {
		t.Errorf("report = %+v, want Suppressed=1", rep)
	}
	if repo.marked["b1"] != 7 {
		t.Errorf("watermark = %d, want 7 even with nobody to mail", repo.marked["b1"])
	}
}

// TestRunDoesNotAdvanceWhenEverySendFailed is the one case where a duplicate is
// the lesser risk: the alternative is a bucket silently lost to an outage.
func TestRunDoesNotAdvanceWhenEverySendFailed(t *testing.T) {
	repo := newFakeRepo()
	repo.candidates = []Candidate{dueCandidate()}
	repo.recipients["wb1"] = []Recipient{subscriber()}
	mailer := &fakeMailer{err: errors.New("smtp down")}

	rep, _ := testService(repo, mailer).Run(context.Background())
	if rep.Failed != 1 || rep.Sent != 0 {
		t.Fatalf("report = %+v, want Failed=1 Sent=0", rep)
	}
	if _, advanced := repo.marked["b1"]; advanced {
		t.Error("watermark advanced despite every send failing — that bucket is lost")
	}
}

// TestRunContinuesPastOneFailure: a pass that aborts halfway leaves a partial
// send nobody can tell from a quiet day.
func TestRunContinuesPastOneFailure(t *testing.T) {
	repo := newFakeRepo()
	a, b := dueCandidate(), dueCandidate()
	b.BidID, b.WorkbenchID = "b2", "wb2"
	repo.candidates = []Candidate{a, b}
	repo.recipients["wb2"] = []Recipient{subscriber()}
	repo.recipients["wb1"] = nil // wb1 has nobody
	mailer := &fakeMailer{}

	rep, err := testService(repo, mailer).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Sent != 1 || rep.Suppressed != 1 {
		t.Errorf("report = %+v, want Sent=1 Suppressed=1 — the second bid must still be processed", rep)
	}
}

func TestRunPropagatesListFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.listErr = errors.New("db down")
	if _, err := testService(repo, &fakeMailer{}).Run(context.Background()); err == nil {
		t.Error("Run reported success when it could not read candidates at all")
	}
}

// TestUnsubscribeTreatsUnknownTokensLikeKnownOnes: the endpoint must not become
// a check for which tokens are live, so a miss is logged and not reported.
func TestUnsubscribeTreatsUnknownTokensLikeKnownOnes(t *testing.T) {
	repo := newFakeRepo()
	repo.optOutHit = false
	if err := testService(repo, &fakeMailer{}).Unsubscribe(context.Background(), "nope"); err != nil {
		t.Errorf("unknown token produced an error the caller would render differently: %v", err)
	}
	repo.optOutHit = true
	if err := testService(repo, &fakeMailer{}).Unsubscribe(context.Background(), "real"); err != nil {
		t.Errorf("known token: %v", err)
	}
}

func TestUnsubscribeIgnoresEmptyTokenWithoutTouchingTheStore(t *testing.T) {
	repo := newFakeRepo()
	if err := testService(repo, &fakeMailer{}).Unsubscribe(context.Background(), ""); err != nil {
		t.Fatalf("empty token: %v", err)
	}
	if len(repo.optedOut) != 0 {
		t.Error("an empty token reached the store")
	}
}

func TestUnsubscribePropagatesStoreFailure(t *testing.T) {
	repo := newFakeRepo()
	repo.optOutErr = errors.New("db down")
	if err := testService(repo, &fakeMailer{}).Unsubscribe(context.Background(), "t"); err == nil {
		t.Error("a store failure was swallowed — the reader would be told they are unsubscribed when they are not")
	}
}
