package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
)

// TransactionalFrom is the sending address for authentication and invitation
// mail. It is a constant here so the reminder sender's shared-domain check and
// the composition root compare against the same value rather than two string
// literals that can drift apart.
const TransactionalFrom = "noreply@tendersbay.xyz"

type ResendSender struct {
	apiKey  string
	from    string
	baseURL string
	client  *http.Client
}

func NewResend(apiKey, from string) *ResendSender {
	return &ResendSender{
		apiKey:  apiKey,
		from:    from,
		baseURL: "https://api.resend.com/emails",
		client:  &http.Client{},
	}
}

func NewResendWithURL(apiKey, from, url string) *ResendSender {
	return &ResendSender{apiKey: apiKey, from: from, baseURL: url, client: &http.Client{}}
}

var (
	verifyTmpl = template.Must(template.New("").Parse(
		`<p>Hi {{.Name}},</p><p>Click the link below to verify your email:</p><p><a href="{{.Link}}">Verify email</a></p>`,
	))
	resetTmpl = template.Must(template.New("").Parse(
		`<p>Hi {{.Name}},</p><p>Click the link below to reset your password. It expires in 1 hour:</p><p><a href="{{.Link}}">Reset password</a></p>`,
	))
	changeEmailTmpl = template.Must(template.New("").Parse(
		`<p>Hi {{.Name}},</p><p>Click the link below to confirm your new email address:</p><p><a href="{{.Link}}">Confirm email</a></p>`,
	))
	workspaceInviteTmpl = template.Must(template.New("").Parse(
		`<p>Hi,</p><p>{{.Inviter}} invited you to join the <strong>{{.Workspace}}</strong> workspace on Tendersbay.</p><p><a href="{{.Link}}">Accept invitation</a></p>`,
	))
	accountExistsTmpl = template.Must(template.New("").Parse(
		`<p>Hi {{.Name}},</p><p>Someone just tried to create a Tendersbay account with this email address, but you already have one. If this was you, simply sign in — you can reset your password if you've forgotten it. If it wasn't you, no action is needed.</p>`,
	))
)

func renderEmail(tmpl *template.Template, name, link string) (string, error) {
	var buf bytes.Buffer
	err := tmpl.Execute(&buf, struct{ Name, Link string }{name, link})
	return buf.String(), err
}

func (r *ResendSender) SendVerification(ctx context.Context, to, displayName, link string) error {
	body, err := renderEmail(verifyTmpl, displayName, link)
	if err != nil {
		return fmt.Errorf("render verification email: %w", err)
	}
	return r.send(ctx, to, "Verify your email — Tendersbay", body)
}

func (r *ResendSender) SendPasswordReset(ctx context.Context, to, displayName, link string) error {
	body, err := renderEmail(resetTmpl, displayName, link)
	if err != nil {
		return fmt.Errorf("render password reset email: %w", err)
	}
	return r.send(ctx, to, "Reset your password — Tendersbay", body)
}

func (r *ResendSender) SendEmailChangeVerification(ctx context.Context, to, displayName, link string) error {
	body, err := renderEmail(changeEmailTmpl, displayName, link)
	if err != nil {
		return fmt.Errorf("render email change email: %w", err)
	}
	return r.send(ctx, to, "Confirm your new email — Tendersbay", body)
}

func (r *ResendSender) SendAccountExists(ctx context.Context, to, displayName string) error {
	var buf bytes.Buffer
	if err := accountExistsTmpl.Execute(&buf, struct{ Name string }{displayName}); err != nil {
		return fmt.Errorf("render account exists email: %w", err)
	}
	return r.send(ctx, to, "You already have a Tendersbay account", buf.String())
}

func (r *ResendSender) SendWorkspaceInvite(ctx context.Context, to, workspaceName, inviterName, link string) error {
	var buf bytes.Buffer
	if err := workspaceInviteTmpl.Execute(&buf, struct{ Inviter, Workspace, Link string }{inviterName, workspaceName, link}); err != nil {
		return fmt.Errorf("render workspace invite email: %w", err)
	}
	return r.send(ctx, to, "You've been invited to "+workspaceName+" — Tendersbay", buf.String())
}

func (r *ResendSender) send(ctx context.Context, to, subject, html string) error {
	return postEmail(ctx, r.client, r.baseURL, r.apiKey, emailPayload{
		From: r.from, To: to, Subject: subject, HTML: html,
	})
}

// emailPayload is Resend's request body. Headers is a map rather than fixed
// fields because only the reminder path sets any, and it must stay omitted
// everywhere else: an empty headers object on a transactional mail is noise in
// a request that has worked untouched for months.
type emailPayload struct {
	From    string            `json:"from"`
	To      string            `json:"to"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html"`
	Headers map[string]string `json:"headers,omitempty"`
}

// postEmail is the one place this package talks to Resend. Extracted so the
// reminder sender can set headers and its own From without a second copy of the
// auth, status and transport handling drifting away from this one.
func postEmail(ctx context.Context, client *http.Client, url, apiKey string, p emailPayload) error {
	body, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("resend: encode payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend: unexpected status %d", resp.StatusCode)
	}
	return nil
}
