package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buildwithgo/berrygem/rag"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
)

// newTestKnowledgeBase wires a KnowledgeBase against fake Qdrant and Ollama
// servers, both driven by the handler functions supplied by each test.
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

func alwaysExistingCollectionHandler(t *testing.T, onUpsert func(body map[string]any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"status":"green","vectors_count":0,"points_count":0,"segments_count":1,"config":{}},"status":"ok","time":0.01}`))
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/collections/tenders/points"):
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if onUpsert != nil {
				onUpsert(body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"operation_id":1,"status":"completed"},"status":"ok","time":0.01}`))
		default:
			t.Errorf("unexpected qdrant request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func fakeOllamaEmbed(vec []float32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp, _ := json.Marshal(map[string]any{"model": "embeddinggemma:latest", "embeddings": [][]float32{vec}})
		_, _ = w.Write(resp)
	}
}

func TestIngest_WithPreChunkedDocument(t *testing.T) {
	var gotBody map[string]any
	kb := newTestKnowledgeBase(t,
		alwaysExistingCollectionHandler(t, func(body map[string]any) { gotBody = body }),
		fakeOllamaEmbed([]float32{0.1, 0.2}),
	)

	doc := &rag.Document{
		ID:       "42",
		Metadata: map[string]string{"source": "ted", "source_ref": "proc-1"},
		Chunks: []rag.Chunk{
			{ID: "42_chunk_0", DocID: "42", Index: 0, Content: "Titolo: Lavori stradali"},
			{ID: "42_chunk_1", DocID: "42", Index: 1, Content: "Estratto del capitolato tecnico..."},
		},
	}

	if err := kb.Ingest(context.Background(), doc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	points, ok := gotBody["points"].([]any)
	if !ok || len(points) != 2 {
		t.Fatalf("points = %v, want 2 points", gotBody["points"])
	}
	p0 := points[0].(map[string]any)
	if p0["id"] != "42_chunk_0" {
		t.Errorf("points[0].id = %v, want 42_chunk_0", p0["id"])
	}
	payload := p0["payload"].(map[string]any)
	if payload["content"] != "Titolo: Lavori stradali" {
		t.Errorf("points[0].payload.content = %v, want the chunk text", payload["content"])
	}
	if payload["tender_id"] != "42" {
		t.Errorf("points[0].payload.tender_id = %v, want 42", payload["tender_id"])
	}
	if payload["source"] != "ted" || payload["source_ref"] != "proc-1" {
		t.Errorf("points[0].payload metadata = %+v, want source=ted source_ref=proc-1 passed through", payload)
	}

	// Embeddings computed from the chunks are written back onto doc.Chunks,
	// mirroring berrygem's own InMemoryKB.Ingest behavior. Compared back
	// through float32: the value round-trips a []float32 (Ollama's/qdrant's
	// native type) through float64 (rag.Chunk.Embedding's type), and widening
	// float32->float64 is exact but the float64 literal 0.1 is not bit-identical
	// to float64(float32(0.1)), so a direct float64 == 0.1 comparison would
	// spuriously fail.
	if len(doc.Chunks[0].Embedding) != 2 || float32(doc.Chunks[0].Embedding[0]) != 0.1 {
		t.Errorf("doc.Chunks[0].Embedding = %v, want [0.1 0.2]", doc.Chunks[0].Embedding)
	}
}

func TestIngest_WithNoPreChunkedContent_UsesSingleSummaryChunk(t *testing.T) {
	var gotBody map[string]any
	kb := newTestKnowledgeBase(t,
		alwaysExistingCollectionHandler(t, func(body map[string]any) { gotBody = body }),
		fakeOllamaEmbed([]float32{0.5}),
	)

	doc := &rag.Document{ID: "7", Content: "Comune di Roma — Appalto pulizie"}

	if err := kb.Ingest(context.Background(), doc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	points := gotBody["points"].([]any)
	if len(points) != 1 {
		t.Fatalf("points = %d, want 1 (Content used as a single chunk when Chunks is empty)", len(points))
	}
	p0 := points[0].(map[string]any)
	if p0["id"] != "7_chunk_0" {
		t.Errorf("points[0].id = %v, want 7_chunk_0", p0["id"])
	}
	payload := p0["payload"].(map[string]any)
	if payload["content"] != "Comune di Roma — Appalto pulizie" {
		t.Errorf("points[0].payload.content = %v, want doc.Content", payload["content"])
	}
}
