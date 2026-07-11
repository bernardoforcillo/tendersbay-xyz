// Package knowledge is a Qdrant + Ollama-embeddings-backed implementation of
// berrygem/rag.KnowledgeBase, shared between services/ingestion (writes) and
// services/backend (reads) so both agree on one collection schema and one
// chunk-to-point mapping.
package knowledge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Embedder calls Ollama's native embeddings API — distinct from
// berrygem/providers/ollama.Provider, which wraps Ollama's OpenAI-compatible
// chat endpoint and has no embeddings method.
type Embedder struct {
	baseURL string
	model   string
	http    *http.Client
}

// NewEmbedder returns an Embedder pointed at baseURL (e.g.
// "http://localhost:11434") using model (e.g. "embeddinggemma:latest").
func NewEmbedder(baseURL, model string) *Embedder {
	return &Embedder{baseURL: baseURL, model: model, http: &http.Client{Timeout: 30 * time.Second}}
}

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed returns the embedding vector for text.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	payload, err := json.Marshal(embedRequest{Model: e.model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("knowledge: marshal embed request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("knowledge: build embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("knowledge: ollama embed: unexpected status %d: %s", resp.StatusCode, body)
	}
	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("knowledge: decode embed response: %w", err)
	}
	if len(out.Embeddings) == 0 {
		return nil, fmt.Errorf("knowledge: ollama embed: empty embeddings in response")
	}
	return out.Embeddings[0], nil
}
