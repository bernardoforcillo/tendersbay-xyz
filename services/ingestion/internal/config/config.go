// Package config loads the ingestion service configuration from the
// environment.
package config

import "os"

const (
	defaultServiceName = "tendersbay-ingestion"
	defaultPostHogHost = "https://eu.i.posthog.com"
)

// Config holds the runtime configuration for the ingestion service. There is
// no PORT (no HTTP server) and no provider-selection or timeout setting —
// every registered provider runs every cycle, and the run's time cap is the
// CronJob's activeDeadlineSeconds, not an app-level setting.
type Config struct {
	ServiceName   string
	PostHogAPIKey string
	PostHogHost   string
	DatabaseURL   string
}

// FromEnv builds a Config from environment variables, applying defaults for
// SERVICE_NAME (tendersbay-ingestion) and POSTHOG_HOST (EU endpoint).
// POSTHOG_API_KEY has no default; an empty key disables telemetry export.
// DATABASE_URL has no default; main treats an empty value as fatal.
func FromEnv() Config {
	return Config{
		ServiceName:   getenv("SERVICE_NAME", defaultServiceName),
		PostHogAPIKey: os.Getenv("POSTHOG_API_KEY"),
		PostHogHost:   getenv("POSTHOG_HOST", defaultPostHogHost),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
