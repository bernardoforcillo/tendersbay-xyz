// Package config loads the ingestion service configuration from the
// environment.
package config

import "os"

const (
	defaultServiceName    = "tendersbay-ingestion"
	defaultPostHogHost    = "https://eu.i.posthog.com"
	defaultQdrantURL      = "http://localhost:6333"
	defaultOllamaBaseURL  = "http://localhost:11434"
	defaultEmbeddingModel = "embeddinggemma:latest"
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
	QdrantURL     string
	OllamaBaseURL string
	// EmbeddingModel selects the Ollama model used to embed document text.
	// It must produce 768-dimensional embeddings: go-services/knowledge's
	// Qdrant collection is created with a hard-coded vectorSize of 768
	// (sized for the default embeddinggemma:latest), and Qdrant rejects any
	// vector of a different dimension. Because indexing failures are only
	// logged, not fatal (see index.Indexer.RunOnce), overriding
	// EMBEDDING_MODEL to a model with a different output dimension makes
	// every Ingest call fail silently — indexing stops advancing with no
	// obvious error surfaced.
	EmbeddingModel string
}

// FromEnv builds a Config from environment variables, applying defaults for
// SERVICE_NAME (tendersbay-ingestion), POSTHOG_HOST (EU endpoint), and the
// local-dev defaults for QDRANT_URL/OLLAMA_BASE_URL/EMBEDDING_MODEL.
// POSTHOG_API_KEY has no default; an empty key disables telemetry export.
// DATABASE_URL has no default; main treats an empty value as fatal.
func FromEnv() Config {
	return Config{
		ServiceName:    getenv("SERVICE_NAME", defaultServiceName),
		PostHogAPIKey:  os.Getenv("POSTHOG_API_KEY"),
		PostHogHost:    getenv("POSTHOG_HOST", defaultPostHogHost),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		QdrantURL:      getenv("QDRANT_URL", defaultQdrantURL),
		OllamaBaseURL:  getenv("OLLAMA_BASE_URL", defaultOllamaBaseURL),
		EmbeddingModel: getenv("EMBEDDING_MODEL", defaultEmbeddingModel),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
