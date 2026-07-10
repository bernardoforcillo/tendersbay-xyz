package config

import "testing"

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("POSTHOG_API_KEY", "")
	t.Setenv("POSTHOG_HOST", "")
	t.Setenv("DATABASE_URL", "")

	cfg := FromEnv()
	if cfg.ServiceName != "tendersbay-ingestion" {
		t.Errorf("ServiceName = %q, want tendersbay-ingestion", cfg.ServiceName)
	}
	if cfg.PostHogHost != "https://eu.i.posthog.com" {
		t.Errorf("PostHogHost = %q, want EU endpoint", cfg.PostHogHost)
	}
	if cfg.PostHogAPIKey != "" {
		t.Errorf("PostHogAPIKey = %q, want empty", cfg.PostHogAPIKey)
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("DatabaseURL = %q, want empty", cfg.DatabaseURL)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	t.Setenv("SERVICE_NAME", "custom")
	t.Setenv("POSTHOG_API_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://us.i.posthog.com")
	t.Setenv("DATABASE_URL", "postgres://example/test")

	cfg := FromEnv()
	if cfg.ServiceName != "custom" || cfg.PostHogAPIKey != "phc_test" ||
		cfg.PostHogHost != "https://us.i.posthog.com" || cfg.DatabaseURL != "postgres://example/test" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}
