package config

import "testing"

func TestFromEnvDefaults(t *testing.T) {
	t.Setenv("SERVICE_NAME", "")
	t.Setenv("POSTHOG_API_KEY", "")
	t.Setenv("POSTHOG_HOST", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("QDRANT_URL", "")
	t.Setenv("OLLAMA_BASE_URL", "")
	t.Setenv("EMBEDDING_MODEL", "")

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
	if cfg.QdrantURL != "http://localhost:6333" {
		t.Errorf("QdrantURL = %q, want http://localhost:6333", cfg.QdrantURL)
	}
	if cfg.OllamaBaseURL != "http://localhost:11434" {
		t.Errorf("OllamaBaseURL = %q, want http://localhost:11434", cfg.OllamaBaseURL)
	}
	if cfg.EmbeddingModel != "embeddinggemma:latest" {
		t.Errorf("EmbeddingModel = %q, want embeddinggemma:latest", cfg.EmbeddingModel)
	}
}

func TestFromEnvOverrides(t *testing.T) {
	t.Setenv("SERVICE_NAME", "custom")
	t.Setenv("POSTHOG_API_KEY", "phc_test")
	t.Setenv("POSTHOG_HOST", "https://us.i.posthog.com")
	t.Setenv("DATABASE_URL", "postgres://example/test")
	t.Setenv("QDRANT_URL", "http://qdrant.internal:6333")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama.internal:11434")
	t.Setenv("EMBEDDING_MODEL", "custom-embed-model")

	cfg := FromEnv()
	if cfg.ServiceName != "custom" || cfg.PostHogAPIKey != "phc_test" ||
		cfg.PostHogHost != "https://us.i.posthog.com" || cfg.DatabaseURL != "postgres://example/test" ||
		cfg.QdrantURL != "http://qdrant.internal:6333" || cfg.OllamaBaseURL != "http://ollama.internal:11434" ||
		cfg.EmbeddingModel != "custom-embed-model" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}
