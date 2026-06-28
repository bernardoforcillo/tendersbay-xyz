package config

import "testing"

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("POSTHOG_API_KEY", "")
	t.Setenv("POSTHOG_HOST", "")

	cfg := FromEnv()
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if cfg.ServiceName != "tendersbay-backend" {
		t.Errorf("ServiceName = %q, want tendersbay-backend", cfg.ServiceName)
	}
	if cfg.PostHogHost != "https://eu.i.posthog.com" {
		t.Errorf("PostHogHost = %q, want EU endpoint", cfg.PostHogHost)
	}
	if cfg.PostHogAPIKey != "" {
		t.Errorf("PostHogAPIKey = %q, want empty", cfg.PostHogAPIKey)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("SERVICE_NAME", "custom")
	t.Setenv("POSTHOG_API_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://us.i.posthog.com")

	cfg := FromEnv()
	if cfg.Port != "9090" || cfg.ServiceName != "custom" ||
		cfg.PostHogAPIKey != "phc_test" || cfg.PostHogHost != "https://us.i.posthog.com" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}
