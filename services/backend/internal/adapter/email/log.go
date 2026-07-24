package email

import (
	"context"
	"log/slog"
	"net/url"
)

// LogSender is a no-op mailer for local development: it logs the email
// details to stdout instead of calling an external API. Use it when no
// RESEND_API_KEY is configured.
type LogSender struct{}

func NewLog() *LogSender { return &LogSender{} }

// redactLink strips the single-use token from a verification/reset/invite link
// before it is logged. The plaintext token grants account access (email
// verification, password reset, workspace join), so even in the dev logger it
// must never be written to stdout where it could be shoulder-surfed or shipped
// to a log aggregator. The path is kept so the log still shows which flow ran.
func redactLink(link string) string {
	u, err := url.Parse(link)
	if err != nil {
		return "[unparseable link redacted]"
	}
	q := u.Query()
	if q.Has("token") {
		q.Set("token", "REDACTED")
		u.RawQuery = q.Encode()
	}
	return u.String()
}

func (l *LogSender) SendVerification(_ context.Context, to, displayName, link string) error {
	slog.Info("[dev-email] verify email", "to", to, "name", displayName, "link", redactLink(link))
	return nil
}

func (l *LogSender) SendPasswordReset(_ context.Context, to, displayName, link string) error {
	slog.Info("[dev-email] password reset", "to", to, "name", displayName, "link", redactLink(link))
	return nil
}

func (l *LogSender) SendEmailChangeVerification(_ context.Context, to, displayName, link string) error {
	slog.Info("[dev-email] email change verification", "to", to, "name", displayName, "link", redactLink(link))
	return nil
}

func (l *LogSender) SendAccountExists(_ context.Context, to, displayName string) error {
	slog.Info("[dev-email] account already exists", "to", to, "name", displayName)
	return nil
}

func (l *LogSender) SendWorkspaceInvite(_ context.Context, to, workspaceName, inviterName, link string) error {
	slog.Info("[dev-email] workspace invite", "to", to, "workspace", workspaceName, "inviter", inviterName, "link", redactLink(link))
	return nil
}
