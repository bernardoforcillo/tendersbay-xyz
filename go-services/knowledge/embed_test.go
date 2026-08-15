package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
)

func TestEmbedder_Embed(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %q, want /api/embed", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"embeddinggemma:latest","embeddings":[[0.1,0.2,0.3]]}`))
	}))
	defer srv.Close()

	e := knowledge.NewEmbedder(srv.URL, "embeddinggemma:latest")
	vec, err := e.Embed(context.Background(), "appalto pubblico")
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if len(vec) != 3 || vec[0] != 0.1 || vec[1] != 0.2 || vec[2] != 0.3 {
		t.Fatalf("vec = %v, want [0.1 0.2 0.3]", vec)
	}
	if gotBody["model"] != "embeddinggemma:latest" {
		t.Errorf("model = %v, want embeddinggemma:latest", gotBody["model"])
	}
	// input is an array now: one request carries a batch, and a single
	// Embed is just a batch of one.
	if got := firstInput(gotBody); got != "appalto pubblico" {
		t.Errorf("input = %v, want %q", gotBody["input"], "appalto pubblico")
	}
}

// firstInput reads the first element of a decoded /api/embed "input" array.
func firstInput(body map[string]any) string {
	arr, _ := body["input"].([]any)
	if len(arr) == 0 {
		return ""
	}
	s, _ := arr[0].(string)
	return s
}

// embeddingsFor builds an /api/embed reply carrying one copy of vec per
// input in the request. The real endpoint answers positionally, and the
// client rejects a count mismatch rather than misalign vectors to chunks —
// so a stub returning a fixed single vector fails any multi-chunk test.
func embeddingsFor(body map[string]any, vec []float32) []byte {
	arr, _ := body["input"].([]any)
	n := len(arr)
	if n == 0 {
		n = 1
	}
	vecs := make([][]float32, n)
	for i := range vecs {
		vecs[i] = vec
	}
	resp, _ := json.Marshal(map[string]any{"model": "embeddinggemma:latest", "embeddings": vecs})
	return resp
}

// captureInput starts a stub Ollama that records the "input" field of the
// last /api/embed request and always answers with a fixed vector.
func captureInput(t *testing.T) (*httptest.Server, func() string) {
	t.Helper()
	var last string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		last = firstInput(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"x","embeddings":[[0.1]]}`))
	}))
	t.Cleanup(srv.Close)
	return srv, func() string { return last }
}

// The task prompts are part of each model's input contract, not decoration:
// sending bare text puts queries and documents in different regions of the
// vector space. These assert the exact documented wire format per model
// family, including EmbeddingGemma's literal "none" title placeholder.
func TestEmbedder_TaskPrompts(t *testing.T) {
	tests := []struct {
		name         string
		model        string
		title        string
		wantQuery    string
		wantDocument string
	}{
		{
			name:         "embeddinggemma with title",
			model:        "embeddinggemma:latest",
			title:        "Servizi di pulizia",
			wantQuery:    "task: search result | query: appalto pulizie",
			wantDocument: "title: Servizi di pulizia | text: contenuto del bando",
		},
		{
			name:         "embeddinggemma without title uses the none placeholder",
			model:        "embeddinggemma:300m",
			wantQuery:    "task: search result | query: appalto pulizie",
			wantDocument: "title: none | text: contenuto del bando",
		},
		{
			name:         "nomic-embed",
			model:        "nomic-embed-text:v1.5",
			title:        "ignored by this family",
			wantQuery:    "search_query: appalto pulizie",
			wantDocument: "search_document: contenuto del bando",
		},
		{
			name:         "unknown model gets no prefix at all",
			model:        "some-other-embedder:latest",
			title:        "Servizi di pulizia",
			wantQuery:    "appalto pulizie",
			wantDocument: "contenuto del bando",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, lastInput := captureInput(t)
			e := knowledge.NewEmbedder(srv.URL, tt.model)

			if _, err := e.EmbedQuery(context.Background(), "appalto pulizie"); err != nil {
				t.Fatalf("EmbedQuery: %v", err)
			}
			if got := lastInput(); got != tt.wantQuery {
				t.Errorf("query input = %q, want %q", got, tt.wantQuery)
			}

			if _, err := e.EmbedDocument(context.Background(), tt.title, "contenuto del bando"); err != nil {
				t.Fatalf("EmbedDocument: %v", err)
			}
			if got := lastInput(); got != tt.wantDocument {
				t.Errorf("document input = %q, want %q", got, tt.wantDocument)
			}
		})
	}
}

// Embed stays the raw primitive — it must not acquire a prefix, or callers
// that already formatted their own input would get it applied twice.
func TestEmbedder_Embed_AppliesNoPrompt(t *testing.T) {
	srv, lastInput := captureInput(t)
	e := knowledge.NewEmbedder(srv.URL, "embeddinggemma:latest")
	if _, err := e.Embed(context.Background(), "testo grezzo"); err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if got := lastInput(); got != "testo grezzo" {
		t.Errorf("input = %q, want %q", got, "testo grezzo")
	}
}

func TestEmbedder_Embed_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("model not found"))
	}))
	defer srv.Close()

	e := knowledge.NewEmbedder(srv.URL, "missing-model")
	_, err := e.Embed(context.Background(), "text")
	if err == nil {
		t.Fatal("Embed: want error on 500 response, got nil")
	}
}

func TestEmbedder_Embed_EmptyEmbeddings(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"x","embeddings":[]}`))
	}))
	defer srv.Close()

	e := knowledge.NewEmbedder(srv.URL, "x")
	_, err := e.Embed(context.Background(), "text")
	if err == nil {
		t.Fatal("Embed: want error when embeddings array is empty, got nil")
	}
}
