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

func TestFromEnv_TenderSearchDefaults(t *testing.T) {
	t.Setenv("QDRANT_URL", "")
	t.Setenv("OLLAMA_BASE_URL", "")
	t.Setenv("EMBEDDING_MODEL", "")
	t.Setenv("REDIS_URL", "")

	cfg := FromEnv()
	if cfg.QdrantURL != "http://localhost:6333" {
		t.Errorf("QdrantURL = %q, want http://localhost:6333", cfg.QdrantURL)
	}
	if cfg.OllamaBaseURL != "http://localhost:11434" {
		t.Errorf("OllamaBaseURL = %q, want http://localhost:11434", cfg.OllamaBaseURL)
	}
	if cfg.EmbeddingModel != "embeddinggemma:latest" {
		t.Errorf("EmbeddingModel = %q, want embeddinggemma:latest", cfg.EmbeddingModel)
	}
	if cfg.RedisURL != "redis://localhost:6379" {
		t.Errorf("RedisURL = %q, want redis://localhost:6379", cfg.RedisURL)
	}
}

func TestFromEnv_TenderSearchOverrides(t *testing.T) {
	t.Setenv("QDRANT_URL", "http://qdrant.internal:6333")
	t.Setenv("OLLAMA_BASE_URL", "http://ollama.internal:11434")
	t.Setenv("EMBEDDING_MODEL", "custom-model")
	t.Setenv("REDIS_URL", "redis://redis.internal:6379")

	cfg := FromEnv()
	if cfg.QdrantURL != "http://qdrant.internal:6333" || cfg.OllamaBaseURL != "http://ollama.internal:11434" ||
		cfg.EmbeddingModel != "custom-model" || cfg.RedisURL != "redis://redis.internal:6379" {
		t.Errorf("overrides not applied: %+v", cfg)
	}
}
