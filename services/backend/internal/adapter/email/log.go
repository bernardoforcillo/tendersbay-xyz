package email

import (
	"context"
	"log/slog"
)

// LogSender is a no-op mailer for local development: it logs the email
// details to stdout instead of calling an external API. Use it when no
// RESEND_API_KEY is configured.
type LogSender struct{}

func NewLog() *LogSender { return &LogSender{} }

func (l *LogSender) SendVerification(_ context.Context, to, displayName, link string) error {
	slog.Info("[dev-email] verify email", "to", to, "name", displayName, "link", link)
	return nil
}

func (l *LogSender) SendPasswordReset(_ context.Context, to, displayName, link string) error {
	slog.Info("[dev-email] password reset", "to", to, "name", displayName, "link", link)
	return nil
}

func (l *LogSender) SendEmailChangeVerification(_ context.Context, to, displayName, link string) error {
	slog.Info("[dev-email] email change verification", "to", to, "name", displayName, "link", link)
	return nil
}

func (l *LogSender) SendWorkspaceInvite(_ context.Context, to, workspaceName, inviterName, link string) error {
	slog.Info("[dev-email] workspace invite", "to", to, "workspace", workspaceName, "inviter", inviterName, "link", link)
	return nil
}
