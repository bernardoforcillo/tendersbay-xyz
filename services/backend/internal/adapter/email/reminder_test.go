package email

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/alerting"
)

func capture(t *testing.T) (*httptest.Server, *emailPayload) {
	t.Helper()
	var got emailPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func reminderFixture(reason alerting.Reason, days, unconfirmed int) alerting.Reminder {
	d := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	return alerting.Reminder{
		Candidate: alerting.Candidate{
			BidID: "bid-1", WorkspaceID: "ws-1", WorkbenchID: "wb-1",
			TenderTitle: "Servizio di pulizia", Deadline: &d, UnconfirmedRequirements: unconfirmed,
		},
		Reason: reason, Bucket: 7, DaysLeft: days,
	}
}

// TestNewReminderRefusesSharedSendingDomain is the enforced half of the
// separate-subdomain rule. A convention nobody checks is one that erodes, and
// this one erodes silently — discovered when password resets start bouncing.
func TestNewReminderRefusesSharedSendingDomain(t *testing.T) {
	_, err := NewReminder("k", "avvisi@tendersbay.xyz", "noreply@tendersbay.xyz", "https://u", "https://app")
	if err == nil {
		t.Fatal("built a reminder sender sharing a domain with transactional mail")
	}
	if _, err := NewReminder("k", "avvisi@mail.tendersbay.xyz", "noreply@tendersbay.xyz", "https://u", "https://app"); err != nil {
		t.Fatalf("refused a legitimately separate subdomain: %v", err)
	}
	// Case and display-name forms must not slip past the check.
	if _, err := NewReminder("k", "A <a@TENDERSBAY.xyz>", "b@tendersbay.xyz", "https://u", "https://app"); err == nil {
		t.Error("case/display-name form slipped past the shared-domain check")
	}
}

func TestSendReminderCarriesOneClickUnsubscribe(t *testing.T) {
	srv, got := capture(t)
	s, err := NewReminderWithURL("k", "avvisi@mail.tendersbay.xyz", "noreply@tendersbay.xyz",
		"https://tendersbay.xyz/unsubscribe", "https://tendersbay.xyz", srv.URL)
	if err != nil {
		t.Fatalf("NewReminderWithURL: %v", err)
	}
	to := alerting.Recipient{Email: "a@example.com", Locale: "it-it", UnsubscribeToken: "tok 1"}

	if err := s.SendReminder(context.Background(), to, reminderFixture(alerting.ReasonDeadline, 5, 0)); err != nil {
		t.Fatalf("SendReminder: %v", err)
	}

	if got.From != "avvisi@mail.tendersbay.xyz" {
		t.Errorf("From = %q — reminders must not send from the transactional domain", got.From)
	}
	if h := got.Headers["List-Unsubscribe-Post"]; h != "List-Unsubscribe=One-Click" {
		t.Errorf("List-Unsubscribe-Post = %q; without it the client's own button does nothing", h)
	}
	// The token must be escaped: it lands in a query string and a raw space
	// would truncate the link.
	wantLink := "https://tendersbay.xyz/unsubscribe?t=tok+1"
	if h := got.Headers["List-Unsubscribe"]; h != "<"+wantLink+">" {
		t.Errorf("List-Unsubscribe = %q, want <%s>", h, wantLink)
	}
	// The body is HTML, so the same URL appears entity-encoded there ("+"
	// becomes "&#43;"). That is correct — the parser decodes it before the URL
	// is used — so the assertion decodes rather than comparing raw, which would
	// have failed on a link that works perfectly.
	decoded := html.UnescapeString(got.HTML)
	if !strings.Contains(decoded, wantLink) {
		t.Errorf("the body carries no visible unsubscribe link: %s", decoded)
	}
	// And a deep link to the scheda gara, not a dashboard.
	if !strings.Contains(got.HTML, "https://tendersbay.xyz/workspaces/ws-1/workbench/wb-1/bids/bid-1") {
		t.Errorf("body has no deep link to the bid: %s", got.HTML)
	}
}

// TestSendReminderRefusesTokenlessRecipient — defence in depth. alerting.Service
// already refuses, but this is the layer that would put the message on the wire.
func TestSendReminderRefusesTokenlessRecipient(t *testing.T) {
	srv, got := capture(t)
	s, _ := NewReminderWithURL("k", "avvisi@mail.tendersbay.xyz", "noreply@tendersbay.xyz",
		"https://u", "https://app", srv.URL)
	err := s.SendReminder(context.Background(),
		alerting.Recipient{Email: "a@example.com"}, reminderFixture(alerting.ReasonDeadline, 5, 0))
	if err == nil {
		t.Fatal("sent a reminder with no unsubscribe token")
	}
	if got.To != "" {
		t.Error("a request reached the transport despite the missing token")
	}
}

func TestReminderCopy(t *testing.T) {
	tests := []struct {
		name, locale string
		rem          alerting.Reminder
		wantSubject  []string
	}{
		{"italian leads with the blocker", "it-it",
			reminderFixture(alerting.ReasonUnconfirmedRequirements, 5, 3),
			[]string{"3 requisiti da confermare", "tra 5 giorni"}},
		{"italian deadline only", "it-it",
			reminderFixture(alerting.ReasonDeadline, 5, 0),
			[]string{"Scadenza tra 5 giorni"}},
		{"tomorrow is named, not counted", "it-it",
			reminderFixture(alerting.ReasonDeadline, 1, 0), []string{"domani"}},
		{"today is named, not zero days", "it-it",
			reminderFixture(alerting.ReasonDeadline, 0, 0), []string{"oggi"}},
		{"unknown locale falls back to english", "de-de",
			reminderFixture(alerting.ReasonDeadline, 5, 0), []string{"Deadline in 5 days"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := copyFor(tt.locale, tt.rem)
			for _, want := range tt.wantSubject {
				if !strings.Contains(c.subject, want) {
					t.Errorf("subject %q does not contain %q", c.subject, want)
				}
			}
		})
	}
}
