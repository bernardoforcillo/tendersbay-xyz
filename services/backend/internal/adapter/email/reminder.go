package email

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/alerting"
)

// ReminderSender implements alerting.Mailer over Resend.
//
// It is a separate type from ResendSender, with its own From, and that is the
// whole reason it exists rather than being five more methods on the existing
// sender. A reminder is direct-marketing-adjacent; the five transactional
// templates are not. Sent from one domain they share a reputation, and a single
// spam complaint about a deadline reminder degrades deliverability for password
// resets — the mail a locked-out user cannot do without. Separate sending
// subdomain, separate reputation, separate type.
type ReminderSender struct {
	apiKey         string
	from           string
	baseURL        string
	unsubscribeURL string
	appBaseURL     string
	client         *http.Client
}

// Compile-time proof that this type satisfies the port. The interface is
// declared in core and satisfied here, never the other way round — the same
// mechanical statement of the dependency rule the postgres adapters make.
var _ alerting.Mailer = (*ReminderSender)(nil)

// NewReminder builds the sender, and REFUSES to build one that would share a
// sending domain with transactional mail.
//
// The check is here rather than in a comment because a convention nobody
// enforces is a convention that erodes: someone reusing MAIL_FROM for both is
// the single likeliest way this protection is lost, and it would be lost
// silently, discovered only when password-reset mail started bouncing.
func NewReminder(apiKey, from, transactionalFrom, unsubscribeURL, appBaseURL string) (*ReminderSender, error) {
	if apiKey == "" || from == "" || unsubscribeURL == "" || appBaseURL == "" {
		return nil, fmt.Errorf("email: reminder sender needs an api key, a from address, an unsubscribe url and an app base url")
	}
	if d := domainOf(from); d != "" && d == domainOf(transactionalFrom) {
		return nil, fmt.Errorf(
			"email: reminder From (%s) shares a sending domain with transactional mail (%s); "+
				"use a separate subdomain so a spam complaint cannot degrade password-reset delivery",
			from, transactionalFrom)
	}
	return &ReminderSender{
		apiKey: apiKey, from: from, unsubscribeURL: unsubscribeURL,
		appBaseURL: strings.TrimSuffix(appBaseURL, "/"),
		baseURL:    "https://api.resend.com/emails", client: &http.Client{},
	}, nil
}

// NewReminderWithURL is NewReminder against an arbitrary endpoint, for tests.
// It applies the same domain check — a test helper that skipped it would let
// the invariant rot behind green tests.
func NewReminderWithURL(apiKey, from, transactionalFrom, unsubscribeURL, appBaseURL, endpoint string) (*ReminderSender, error) {
	s, err := NewReminder(apiKey, from, transactionalFrom, unsubscribeURL, appBaseURL)
	if err != nil {
		return nil, err
	}
	s.baseURL = endpoint
	return s, nil
}

// domainOf extracts the domain from "Name <box@domain>" or a bare address.
// Returns "" when there is no "@" to split on, which makes an unparseable
// address fail the equality check open rather than blocking startup on a
// format this function does not understand.
func domainOf(addr string) string {
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		return strings.ToLower(strings.Trim(addr[i+1:], "> "))
	}
	return ""
}

var reminderTmpl = template.Must(template.New("").Parse(
	`<p>{{.Greeting}}</p>` +
		`<p>{{.Lead}}</p>` +
		`<p><a href="{{.Link}}">{{.CTA}}</a></p>` +
		`<hr><p style="font-size:12px;color:#666">{{.UnsubLead}} <a href="{{.UnsubLink}}">{{.UnsubCTA}}</a></p>`,
))

// SendReminder renders and sends one reminder.
func (r *ReminderSender) SendReminder(ctx context.Context, to alerting.Recipient, rem alerting.Reminder) error {
	// Defence in depth: alerting.Service already refuses a tokenless recipient,
	// but this is the layer that would actually put an inescapable message on
	// the wire, so it checks too.
	if to.UnsubscribeToken == "" {
		return fmt.Errorf("email: refusing to send a reminder with no unsubscribe token")
	}
	c := copyFor(to.Locale, rem)
	c.link = r.bidLink(rem.Candidate)
	unsub := r.unsubscribeLink(to.UnsubscribeToken)

	var buf bytes.Buffer
	if err := reminderTmpl.Execute(&buf, struct{ Greeting, Lead, Link, CTA, UnsubLead, UnsubLink, UnsubCTA string }{
		Greeting: c.greeting, Lead: c.lead, Link: c.link, CTA: c.cta,
		UnsubLead: c.unsubLead, UnsubLink: unsub, UnsubCTA: c.unsubCTA,
	}); err != nil {
		return fmt.Errorf("email: render reminder: %w", err)
	}

	return postEmail(ctx, r.client, r.baseURL, r.apiKey, emailPayload{
		From: r.from, To: to.Email, Subject: c.subject, HTML: buf.String(),
		// One-click unsubscribe. List-Unsubscribe alone is a link a mail client
		// may show; the -Post pair is what makes the client's own "unsubscribe"
		// button work without the reader ever opening the message, which is
		// what the bulk-sender rules require and what keeps complaints from
		// becoming spam reports.
		Headers: map[string]string{
			"List-Unsubscribe":      "<" + unsub + ">",
			"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
		},
	})
}

// bidLink deep-links to the bid's scheda gara, mirroring the app's own route
// (/workspaces/:ws/workbench/:wb/bids/:bid). It is built here rather than
// carried on the Reminder because a URL shape is a delivery concern: the domain
// decides that someone should be told, not where the button points.
func (r *ReminderSender) bidLink(c alerting.Candidate) string {
	return fmt.Sprintf("%s/workspaces/%s/workbench/%s/bids/%s",
		r.appBaseURL, c.WorkspaceID, c.WorkbenchID, c.BidID)
}

func (r *ReminderSender) unsubscribeLink(token string) string {
	sep := "?"
	if strings.Contains(r.unsubscribeURL, "?") {
		sep = "&"
	}
	return r.unsubscribeURL + sep + "t=" + url.QueryEscape(token)
}

type reminderCopy struct {
	subject, greeting, lead, link, cta, unsubLead, unsubCTA string
}

// copyFor localises the reminder. Italian and English only, and that is a
// deliberate scope statement rather than an oversight: the frontend carries 24
// locales, but this is the first message this product ever sends unprompted and
// the beachhead is Italian. Every other locale falls back to English, which is
// honest; a machine-translated reminder about a legal deadline is not.
func copyFor(locale string, rem alerting.Reminder) reminderCopy {
	title := rem.Candidate.TenderTitle
	if strings.HasPrefix(strings.ToLower(locale), "it") {
		return italianCopy(rem, title)
	}
	return englishCopy(rem, title)
}

func italianCopy(rem alerting.Reminder, title string) reminderCopy {
	when := fmt.Sprintf("tra %d giorni", rem.DaysLeft)
	if rem.DaysLeft == 0 {
		when = "oggi"
	} else if rem.DaysLeft == 1 {
		when = "domani"
	}
	c := reminderCopy{
		greeting:  "Ciao,",
		link:      "",
		cta:       "Apri la gara",
		unsubLead: "Ricevi questo promemoria perché segui questa gara.",
		unsubCTA:  "Non voglio più questi promemoria",
	}
	if rem.Reason == alerting.ReasonUnconfirmedRequirements {
		n := rem.Candidate.UnconfirmedRequirements
		c.subject = fmt.Sprintf("%d requisiti da confermare — scadenza %s: %s", n, when, title)
		c.lead = fmt.Sprintf(
			"La scadenza di «%s» è %s, e ci sono ancora %d requisiti non confermati. "+
				"Confermarli ora è ciò che rende affidabile il verdetto di ammissibilità.", title, when, n)
		return c
	}
	c.subject = fmt.Sprintf("Scadenza %s: %s", when, title)
	c.lead = fmt.Sprintf("La scadenza per presentare l'offerta su «%s» è %s.", title, when)
	return c
}

func englishCopy(rem alerting.Reminder, title string) reminderCopy {
	when := fmt.Sprintf("in %d days", rem.DaysLeft)
	if rem.DaysLeft == 0 {
		when = "today"
	} else if rem.DaysLeft == 1 {
		when = "tomorrow"
	}
	c := reminderCopy{
		greeting:  "Hi,",
		cta:       "Open the tender",
		unsubLead: "You get this reminder because you are tracking this tender.",
		unsubCTA:  "Stop these reminders",
	}
	if rem.Reason == alerting.ReasonUnconfirmedRequirements {
		n := rem.Candidate.UnconfirmedRequirements
		c.subject = fmt.Sprintf("%d requirements still unconfirmed — deadline %s: %s", n, when, title)
		c.lead = fmt.Sprintf(
			"The deadline for %q is %s, and %d requirements are still unconfirmed. "+
				"Confirming them is what makes the eligibility verdict worth trusting.", title, when, n)
		return c
	}
	c.subject = fmt.Sprintf("Deadline %s: %s", when, title)
	c.lead = fmt.Sprintf("The submission deadline for %q is %s.", title, when)
	return c
}
