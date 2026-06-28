// Package config loads the backend service configuration from the environment.
package config

import "os"

const (
	defaultPort        = "8080"
	defaultServiceName = "tendersbay-backend"
	defaultPostHogHost = "https://eu.i.posthog.com"
)

// Config holds the runtime configuration for the backend service.
type Config struct {
	Port          string
	ServiceName   string
	PostHogAPIKey string
	PostHogHost   string
}

// FromEnv builds a Config from environment variables, applying defaults for
// PORT (8080), SERVICE_NAME (tendersbay-backend) and POSTHOG_HOST (EU endpoint).
// POSTHOG_API_KEY has no default; an empty key disables telemetry export.
func FromEnv() Config {
	return Config{
		Port:          getenv("PORT", defaultPort),
		ServiceName:   getenv("SERVICE_NAME", defaultServiceName),
		PostHogAPIKey: os.Getenv("POSTHOG_API_KEY"),
		PostHogHost:   getenv("POSTHOG_HOST", defaultPostHogHost),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
