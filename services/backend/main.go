package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/httpapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/probe"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/health"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/telemetry"
)

func main() {
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
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

	svc := health.New(probe.NewReady())
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: httpapi.New(svc)}

	srvErr := make(chan error, 1)
	go func() {
		slog.Info("backend listening", "addr", "http://localhost:"+cfg.Port)
		srvErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-srvErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
			os.Exit(1)
		}
	}
}
