// Command ingestion runs one tender-ingestion cycle and exits. Scheduling is
// external — a Kubernetes CronJob fires this on the hour (see the design
// doc at docs/superpowers/specs/2026-07-06-ingestion-service-design.md).
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/telemetry"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
)

func main() { os.Exit(run()) }

func run() int {
	cfg := config.FromEnv()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := telemetry.Setup(ctx, telemetry.Config{
		APIKey:      cfg.PostHogAPIKey,
		Host:        cfg.PostHogHost,
		ServiceName: cfg.ServiceName,
	})
	if err != nil {
		slog.Error("failed to set up telemetry", "error", err)
		return 1
	}
	defer func() { _ = shutdown(context.Background()) }()

	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		return 1
	}

	db, sqlDB, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		return 1
	}
	defer sqlDB.Close()

	sink := postgres.NewTenderRepo(db)
	sources := source.NewRegistry()
	svc := ingestion.NewService(sources, sink)

	report := svc.RunOnce(ctx)
	slog.Info("ingestion complete", "providers", len(sources), "summary", report.Summary())
	if report.Failed() {
		return 1
	}
	return 0
}
