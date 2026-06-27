package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/bernardoforcillo/tendersbay-xyz/apps/platform/internal/server"
	"github.com/bernardoforcillo/tendersbay-xyz/apps/platform/internal/telemetry"
)

//go:embed all:dist
var distFS embed.FS

func main() {
	ctx := context.Background()

	shutdown, err := telemetry.Setup(ctx, telemetry.ConfigFromEnv())
	if err != nil {
		slog.Error("failed to set up telemetry", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

	dist, err := fs.Sub(distFS, "dist")
	if err != nil {
		slog.Error("failed to load embedded frontend", "error", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := ":" + port
	slog.Info("platform listening", "addr", "http://localhost"+addr)
	if err := http.ListenAndServe(addr, server.New(dist)); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
