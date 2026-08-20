// Package config loads the backend service configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// minJWTSecretLen is the shortest JWT_SECRET the service will start with. HS256
// signing security depends entirely on the secret's entropy, so a too-short
// (guessable) key is treated as a misconfiguration, not a warning.
const minJWTSecretLen = 32

const (
	defaultPort           = "8080"
	defaultServiceName    = "tendersbay-backend"
	defaultPostHogHost    = "https://eu.i.posthog.com"
	defaultQdrantURL      = "http://localhost:6333"
	defaultOllamaBaseURL  = "http://localhost:11434"
	defaultEmbeddingModel = "embeddinggemma:latest"
	defaultRedisURL       = "redis://localhost:6379"
)

// Config holds the runtime configuration for the backend service.
type Config struct {
	Port          string
	ServiceName   string
	PostHogAPIKey string
	PostHogHost   string
	DatabaseURL   string
	JWTSecret     string
	JWTExpiry     time.Duration
	RefreshExpiry time.Duration
	ResendAPIKey  string
	// ReminderMailFrom is the From for deadline reminders, and it MUST be a
	// different sending subdomain from transactional mail — email.NewReminder
	// refuses to build otherwise. Empty disables reminder SENDING; it does not
	// disable unsubscribing, which has to keep working for anyone who already
	// received one.
	ReminderMailFrom string
	FireworksAPIKey  string
	AppBaseURL       string
	CORSOrigins      []string
	// WorkspaceInviteExpiry is how long an email workspace invitation stays valid.
	WorkspaceInviteExpiry time.Duration
	QdrantURL             string
	OllamaBaseURL         string
	EmbeddingModel        string
	RedisURL              string
}

// FromEnv builds a Config from environment variables, applying defaults for
// PORT (8080), SERVICE_NAME (tendersbay-backend) and POSTHOG_HOST (EU endpoint).
// POSTHOG_API_KEY has no default; an empty key disables telemetry export.
func FromEnv() Config {
	cfg := Config{
		Port:             getenv("PORT", defaultPort),
		ServiceName:      getenv("SERVICE_NAME", defaultServiceName),
		PostHogAPIKey:    os.Getenv("POSTHOG_API_KEY"),
		PostHogHost:      getenv("POSTHOG_HOST", defaultPostHogHost),
		DatabaseURL:      os.Getenv("DATABASE_URL"),
		JWTSecret:        os.Getenv("JWT_SECRET"),
		ResendAPIKey:     os.Getenv("RESEND_API_KEY"),
		ReminderMailFrom: os.Getenv("REMINDER_MAIL_FROM"),
		FireworksAPIKey:  os.Getenv("FIREWORKS_API_KEY"),
		AppBaseURL:       os.Getenv("APP_BASE_URL"),
		QdrantURL:        getenv("QDRANT_URL", defaultQdrantURL),
		OllamaBaseURL:    getenv("OLLAMA_BASE_URL", defaultOllamaBaseURL),
		EmbeddingModel:   getenv("EMBEDDING_MODEL", defaultEmbeddingModel),
		RedisURL:         getenv("REDIS_URL", defaultRedisURL),
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

// Validate reports a fatal misconfiguration in the required secrets. It fails
// closed on a missing or weak JWT_SECRET: with an empty secret the HS256 signer
// would happily sign and verify tokens with an empty key, letting anyone forge
// an access token for any user. Call this before starting the server.
func (c Config) Validate() error {
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}
	if c.JWTSecret == "" {
		return errors.New("JWT_SECRET is required")
	}
	if len(c.JWTSecret) < minJWTSecretLen {
		return fmt.Errorf("JWT_SECRET must be at least %d characters", minJWTSecretLen)
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
