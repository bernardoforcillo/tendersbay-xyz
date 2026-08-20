// Command digest runs one deadline-reminder pass and exits.
//
// It is a command in the BACKEND's module, shipped in the backend image and
// invoked by a CronJob, rather than a service of its own or a ticker inside the
// API process. Each of those alternatives was rejected for a specific reason:
//
//   - Not a new services/<name>: the microservices threshold in
//     .claude/rules/system-design.md is not met. There is no proven isolated
//     bottleneck and no separate ownership boundary, and a new service costs an
//     own go.mod, Dockerfile, CI workflow, k8s folder and image automation.
//
//   - Not in services/ingestion, which already runs CronJobs: the pass needs
//     core/alerting, core/bid's vocabulary and the backend's repositories, all
//     of which are internal/ to this module and unreachable from there.
//
//   - Not a ticker inside the API process: that breaks the statelessness rule.
//     N replicas would send N copies of every reminder, and the send cadence
//     would couple to pod lifecycle. concurrencyPolicy: Forbid on a CronJob is
//     the existing, correct answer.
//
// One pass, then exit. Reminder buckets make the pass idempotent, so a missed
// run costs at most a delayed mail and a double run costs nothing.
package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/email"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/alerting"
)

// passTimeout bounds one run well inside the CronJob's own deadline, so a
// stalled database or mail provider fails the job cleanly rather than being
// killed mid-send with no log line explaining why.
const passTimeout = 10 * time.Minute

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), passTimeout)
	defer cancel()

	cfg := config.FromEnv()

	if cfg.ResendAPIKey == "" || cfg.ReminderMailFrom == "" {
		// Exit 0, not 1. Reminders being switched off is a configuration
		// choice, and a CronJob that reports failure for it would page someone
		// every hour about a decision they made on purpose.
		slog.Warn("reminders are not configured; nothing to do",
			"have_api_key", cfg.ResendAPIKey != "", "have_from", cfg.ReminderMailFrom != "")
		return
	}
	if cfg.AppBaseURL == "" {
		slog.Error("APP_BASE_URL is required: without it a reminder cannot link to the bid it is about")
		os.Exit(1)
	}

	db, sqlDB, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	mailer, err := email.NewReminder(
		cfg.ResendAPIKey, cfg.ReminderMailFrom, email.TransactionalFrom,
		cfg.AppBaseURL+"/unsubscribe", cfg.AppBaseURL,
	)
	if err != nil {
		// This is where the shared-sending-domain check lands. Failing startup
		// is the point: the alternative is discovering the mistake when
		// password-reset mail starts being filtered as spam.
		slog.Error("reminder mailer refused to start", "error", err)
		os.Exit(1)
	}

	rep, err := alerting.NewService(postgres.NewAlertingRepo(db), mailer).Run(ctx)
	if err != nil {
		slog.Error("reminder pass failed", "error", err)
		os.Exit(1)
	}
	// A pass that could not deliver anything it tried to is a failure even
	// though it completed: exiting 0 would make a broken mail provider look
	// like a quiet day for as long as it stayed broken.
	if rep.Failed > 0 && rep.Sent == 0 {
		slog.Error("every reminder in this pass failed", "failed", rep.Failed)
		os.Exit(1)
	}
}
