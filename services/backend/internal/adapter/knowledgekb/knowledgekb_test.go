package knowledgekb_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/knowledgekb"
)

// newTestKnowledgeBase wires a *knowledge.KnowledgeBase against fake Qdrant
// and Ollama servers, mirroring go-services/knowledge's own test helper
// (knowledgebase_test.go). New takes a concrete *knowledge.KnowledgeBase, so
// exercising the real HTTP round trip is the only way to test its mapping
// against knowledge.SearchWithScores's actual return shape.
func newTestKnowledgeBase(t *testing.T, qdrantHandler, ollamaHandler http.HandlerFunc) *knowledge.KnowledgeBase {
	t.Helper()
	qdrantSrv := httptest.NewServer(qdrantHandler)
	t.Cleanup(qdrantSrv.Close)
	ollamaSrv := httptest.NewServer(ollamaHandler)
	t.Cleanup(ollamaSrv.Close)

	kb, err := knowledge.NewKnowledgeBase(context.Background(), qdrantSrv.URL, ollamaSrv.URL, "embeddinggemma:latest")
	if err != nil {
		t.Fatalf("NewKnowledgeBase: %v", err)
	}
	return kb
}

func fakeOllamaEmbed(vec []float32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp, _ := json.Marshal(map[string]any{"model": "embeddinggemma:latest", "embeddings": [][]float32{vec}})
		_, _ = w.Write(resp)
	}
}

// fakeQdrantSearch serves the collection-existence check NewKnowledgeBase
// issues plus the points/search request SearchWithScores issues, returning
// hits wrapped in Qdrant's {"result": ...} envelope.
func fakeQdrantSearch(t *testing.T, hits []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"status":"green"},"status":"ok","time":0.01}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/tenders/points/search":
			w.Header().Set("Content-Type", "application/json")
			body, _ := json.Marshal(map[string]any{"result": hits, "status": "ok", "time": 0.01})
			_, _ = w.Write(body)
		default:
			t.Errorf("unexpected qdrant request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func TestKB_SearchWithScores_MapsDocIDAndScore(t *testing.T) {
	hits := []map[string]any{
		{
			"id":      "point-1",
			"score":   0.87,
			"payload": map[string]any{"content": "chunk text", "tender_id": "42", "chunk_index": float64(0)},
		},
	}
	kb := newTestKnowledgeBase(t, fakeQdrantSearch(t, hits), fakeOllamaEmbed([]float32{0.1, 0.2, 0.3}))
	adapter := knowledgekb.New(kb)

	out, err := adapter.SearchWithScores(context.Background(), "roadworks", 5)
	if err != nil {
		t.Fatalf("SearchWithScores: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].DocID != "42" {
		t.Errorf("DocID = %q, want %q", out[0].DocID, "42")
	}
	if out[0].Score != float32(0.87) {
		t.Errorf("Score = %v, want 0.87", out[0].Score)
	}
}

func TestKB_SearchWithScores_MapsMultipleHits(t *testing.T) {
	hits := []map[string]any{
		{"id": "point-1", "score": 0.4, "payload": map[string]any{"content": "a", "tender_id": "1", "chunk_index": float64(0)}},
		{"id": "point-2", "score": 0.9, "payload": map[string]any{"content": "b", "tender_id": "2", "chunk_index": float64(0)}},
	}
	kb := newTestKnowledgeBase(t, fakeQdrantSearch(t, hits), fakeOllamaEmbed([]float32{0.1, 0.2, 0.3}))
	adapter := knowledgekb.New(kb)

	out, err := adapter.SearchWithScores(context.Background(), "lavori stradali", 5)
	if err != nil {
		t.Fatalf("SearchWithScores: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].DocID != "1" || out[0].Score != float32(0.4) {
		t.Errorf("out[0] = %+v, want {DocID: 1, Score: 0.4}", out[0])
	}
	if out[1].DocID != "2" || out[1].Score != float32(0.9) {
		t.Errorf("out[1] = %+v, want {DocID: 2, Score: 0.9}", out[1])
	}
}

func TestUnavailable_SearchWithScoresAlwaysErrors(t *testing.T) {
	var kb knowledgekb.Unavailable
	out, err := kb.SearchWithScores(context.Background(), "q", 5)
	if err == nil {
		t.Fatal("SearchWithScores: want error, got nil")
	}
	if out != nil {
		t.Errorf("out = %v, want nil", out)
	}
}
