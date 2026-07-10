// Package config loads the backend service configuration from the environment.
package config

import (
	"os"
	"strings"
	"time"
)

const (
	defaultPort        = "8080"
	defaultServiceName = "tendersbay-backend"
	defaultPostHogHost = "https://eu.i.posthog.com"
)

// Config holds the runtime configuration for the backend service.
type Config struct {
	Port            string
	ServiceName     string
	PostHogAPIKey   string
	PostHogHost     string
	DatabaseURL     string
	JWTSecret       string
	JWTExpiry       time.Duration
	RefreshExpiry   time.Duration
	ResendAPIKey    string
	FireworksAPIKey string
	AppBaseURL      string
	CORSOrigins     []string
	// WorkspaceInviteExpiry is how long an email workspace invitation stays valid.
	WorkspaceInviteExpiry time.Duration
}

// FromEnv builds a Config from environment variables, applying defaults for
// PORT (8080), SERVICE_NAME (tendersbay-backend) and POSTHOG_HOST (EU endpoint).
// POSTHOG_API_KEY has no default; an empty key disables telemetry export.
func FromEnv() Config {
	cfg := Config{
		Port:            getenv("PORT", defaultPort),
		ServiceName:     getenv("SERVICE_NAME", defaultServiceName),
		PostHogAPIKey:   os.Getenv("POSTHOG_API_KEY"),
		PostHogHost:     getenv("POSTHOG_HOST", defaultPostHogHost),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		ResendAPIKey:    os.Getenv("RESEND_API_KEY"),
		FireworksAPIKey: os.Getenv("FIREWORKS_API_KEY"),
		AppBaseURL:      os.Getenv("APP_BASE_URL"),
	}

	if raw := os.Getenv("CORS_ORIGINS"); raw != "" {
		cfg.CORSOrigins = strings.Split(raw, ",")
	} else {
		cfg.CORSOrigins = []string{"https://tendersbay.xyz", "https://dev.tendersbay.xyz"}
	}

	expiry := os.Getenv("JWT_EXPIRY")
	if expiry == "" {
		expiry = "15m"
	}
	cfg.JWTExpiry, _ = time.ParseDuration(expiry)

	refresh := os.Getenv("REFRESH_EXPIRY")
	if refresh == "" {
		refresh = "168h"
	}
	cfg.RefreshExpiry, _ = time.ParseDuration(refresh)

	inviteExpiry := os.Getenv("WORKSPACE_INVITE_EXPIRY")
	if inviteExpiry == "" {
		inviteExpiry = "168h"
	}
	cfg.WorkspaceInviteExpiry, _ = time.ParseDuration(inviteExpiry)

	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
