package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func (r *ResendSender) SendVerification(ctx context.Context, to, displayName, link string) error {
	return r.send(ctx, to, "Verify your email — Tendersbay",
		fmt.Sprintf("<p>Hi %s,</p><p>Click the link below to verify your email:</p><p><a href=%q>Verify email</a></p>", displayName, link))
}

func (r *ResendSender) SendPasswordReset(ctx context.Context, to, displayName, link string) error {
	return r.send(ctx, to, "Reset your password — Tendersbay",
		fmt.Sprintf("<p>Hi %s,</p><p>Click the link below to reset your password. It expires in 1 hour:</p><p><a href=%q>Reset password</a></p>", displayName, link))
}

func (r *ResendSender) SendEmailChangeVerification(ctx context.Context, to, displayName, link string) error {
	return r.send(ctx, to, "Confirm your new email — Tendersbay",
		fmt.Sprintf("<p>Hi %s,</p><p>Click the link below to confirm your new email address:</p><p><a href=%q>Confirm email</a></p>", displayName, link))
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
