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
	"strings"
	"time"
)

// Embedder calls Ollama's native embeddings API — distinct from
// berrygem/providers/ollama.Provider, which wraps Ollama's OpenAI-compatible
// chat endpoint and has no embeddings method.
type Embedder struct {
	baseURL string
	model   string
	prompt  promptStyle
	http    *http.Client
}

// NewEmbedder returns an Embedder pointed at baseURL (e.g.
// "http://localhost:11434") using model (e.g. "embeddinggemma:latest"). The
// model name also selects the task-prompt style — see promptStyleFor.
func NewEmbedder(baseURL, model string) *Embedder {
	return &Embedder{
		baseURL: baseURL,
		model:   model,
		prompt:  promptStyleFor(model),
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// promptStyle names the asymmetric task-prompt convention an embedding model
// was trained with. Retrieval models are trained on (query, document) pairs
// where each side carries a distinct instruction prefix; feeding both sides
// bare text — as this package did before — projects them into different
// regions of the vector space and measurably degrades recall. The prefix is
// not cosmetic, it is part of the model's input contract.
type promptStyle int

const (
	// promptNone sends text through unchanged, for models with no documented
	// task prefix (and as the safe default for an unrecognised model: a
	// wrong prefix is worse than none).
	promptNone promptStyle = iota
	// promptGemma is EmbeddingGemma's convention.
	promptGemma
	// promptNomic is the nomic-embed-text family's convention.
	promptNomic
)

// promptStyleFor picks the prompt convention for an Ollama model tag
// ("embeddinggemma:latest", "nomic-embed-text:v1.5", …). An unrecognised
// model deliberately gets promptNone rather than a guess.
func promptStyleFor(model string) promptStyle {
	name := strings.ToLower(model)
	switch {
	case strings.Contains(name, "embeddinggemma"):
		return promptGemma
	case strings.Contains(name, "nomic-embed"):
		return promptNomic
	default:
		return promptNone
	}
}

// queryPrompt wraps a user's search text in the model's query-side prefix.
func (e *Embedder) queryPrompt(query string) string {
	switch e.prompt {
	case promptGemma:
		return "task: search result | query: " + query
	case promptNomic:
		return "search_query: " + query
	default:
		return query
	}
}

// documentPrompt wraps indexed text in the model's document-side prefix.
// EmbeddingGemma's document format carries a title slot, and expects the
// literal "none" when there isn't one — an empty title is not the same input.
func (e *Embedder) documentPrompt(title, text string) string {
	switch e.prompt {
	case promptGemma:
		if title == "" {
			title = "none"
		}
		return "title: " + title + " | text: " + text
	case promptNomic:
		return "search_document: " + text
	default:
		return text
	}
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// maxEmbedBatch bounds how many texts ride in one /api/embed call. Ollama
// accepts an arbitrarily long array, but the whole batch is marshalled and
// held in memory on both sides, and a failure costs the entire batch — 32
// keeps the round-trip saving while leaving a retry cheap.
const maxEmbedBatch = 32

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// EmbedQuery returns the embedding vector for a user's search text, wrapped
// in the model's query-side task prompt. Use this for anything typed by a
// person; use EmbedDocument for anything being indexed.
func (e *Embedder) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	return e.Embed(ctx, e.queryPrompt(query))
}

// EmbedDocument returns the embedding vector for indexed content, wrapped in
// the model's document-side task prompt. Pass an empty title when the chunk
// has none.
func (e *Embedder) EmbedDocument(ctx context.Context, title, text string) ([]float32, error) {
	return e.Embed(ctx, e.documentPrompt(title, text))
}

// Embed returns the embedding vector for text exactly as given, applying no
// task prompt. It is the raw transport primitive behind EmbedQuery and
// EmbedDocument — prefer those, so query and document sides stay on the
// convention the model was trained with.
func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := e.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

// EmbedDocuments embeds many chunks of one document in as few round-trips as
// the batch size allows, applying the document-side task prompt to each.
//
// This exists because embedding was the indexing pass's throughput ceiling:
// one HTTP request per chunk against a CPU-only Ollama measured 0.4-0.9s
// each, so a 200-tender batch spent minutes in per-request overhead for work
// the model can do in groups.
func (e *Embedder) EmbedDocuments(ctx context.Context, title string, texts []string) ([][]float32, error) {
	prompts := make([]string, len(texts))
	for i, t := range texts {
		prompts[i] = e.documentPrompt(title, t)
	}
	return e.EmbedBatch(ctx, prompts)
}

// EmbedBatch returns one vector per input text, in the same order, applying
// no task prompt. Inputs are split into requests of at most maxEmbedBatch.
func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += maxEmbedBatch {
		end := min(start+maxEmbedBatch, len(texts))
		vecs, err := e.embedOnce(ctx, texts[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, vecs...)
	}
	return out, nil
}

func (e *Embedder) embedOnce(ctx context.Context, texts []string) ([][]float32, error) {
	payload, err := json.Marshal(embedRequest{Model: e.model, Input: texts})
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
	// Positional, so a short or long reply is not recoverable by guessing:
	// vector i belongs to text i, and a mismatch would silently attach the
	// wrong embedding to a chunk rather than fail.
	if len(out.Embeddings) != len(texts) {
		return nil, fmt.Errorf("knowledge: ollama embed: got %d embeddings for %d inputs", len(out.Embeddings), len(texts))
	}
	return out.Embeddings, nil
}
