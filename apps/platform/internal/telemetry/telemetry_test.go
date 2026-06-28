package telemetry

import (
	"context"
	"testing"
)

func TestSetupDisabledWithoutKey(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{ServiceName: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("expected a non-nil no-op shutdown")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown returned error: %v", err)
	}
}

func TestConfigFromEnvDefaults(t *testing.T) {
	t.Setenv("POSTHOG_API_KEY", "")
	t.Setenv("POSTHOG_HOST", "")
	cfg := ConfigFromEnv()
	if cfg.Host != "https://eu.i.posthog.com" {
		t.Fatalf("Host = %q, want default EU host", cfg.Host)
	}
	if cfg.ServiceName != "tendersbay-platform" {
		t.Fatalf("ServiceName = %q, want tendersbay-platform", cfg.ServiceName)
	}
	if cfg.APIKey != "" {
		t.Fatalf("APIKey = %q, want empty", cfg.APIKey)
	}
}
