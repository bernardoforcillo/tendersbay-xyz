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

	authv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/auth/v1/authv1connect"
	userv1connect "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/user/v1/userv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/connectapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/email"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/httpapi"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/postgres"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/probe"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/config"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/auth"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/health"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/user"
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

	if cfg.DatabaseURL == "" {
		slog.Error("DATABASE_URL is required")
		os.Exit(1)
	}

	db, sqlDB, err := postgres.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	userRepo := postgres.NewUserRepo(db)
	sessionRepo := postgres.NewSessionRepo(db)
	evRepo := postgres.NewEVRepo(db)
	prRepo := postgres.NewPRRepo(db)

	mailer := email.NewResend(cfg.ResendAPIKey, "noreply@tendersbay.xyz")

	authCfg := auth.Config{
		JWTSecret:     cfg.JWTSecret,
		JWTExpiry:     cfg.JWTExpiry,
		RefreshExpiry: cfg.RefreshExpiry,
		AppBaseURL:    cfg.AppBaseURL,
	}

	authSvc := auth.NewService(userRepo, sessionRepo, evRepo, prRepo, mailer, authCfg)
	userSvc := user.NewService(userRepo, sessionRepo, evRepo, mailer, authCfg)

	authHandler := connectapi.NewAuthHandler(authSvc, int(cfg.RefreshExpiry.Seconds()))
	userHandler := connectapi.NewUserHandler(userSvc)

	authPath, authRPC := authv1connect.NewAuthServiceHandler(authHandler)
	userPath, userRPC := userv1connect.NewUserServiceHandler(userHandler)

	healthSvc := health.New(probe.NewReady(), probe.NewDB(sqlDB))

	mux := http.NewServeMux()
	mux.Handle(authPath, authRPC)
	mux.Handle(userPath, userRPC)
	mux.Handle("/", httpapi.New(healthSvc))

	handler := connectapi.NewCORS(cfg.CORSOrigins)(connectapi.JWTMiddleware(cfg.JWTSecret)(mux))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

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
