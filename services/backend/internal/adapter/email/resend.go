package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
)

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

func (r *ResendSender) SendWorkspaceInvite(ctx context.Context, to, workspaceName, inviterName, link string) error {
	var buf bytes.Buffer
	if err := workspaceInviteTmpl.Execute(&buf, struct{ Inviter, Workspace, Link string }{inviterName, workspaceName, link}); err != nil {
		return fmt.Errorf("render workspace invite email: %w", err)
	}
	return r.send(ctx, to, "You've been invited to "+workspaceName+" — Tendersbay", buf.String())
}

func (r *ResendSender) send(ctx context.Context, to, subject, html string) error {
	body, _ := json.Marshal(map[string]string{
		"from":    r.from,
		"to":      to,
		"subject": subject,
		"html":    html,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("resend: unexpected status %d", resp.StatusCode)
	}
	return nil
}
