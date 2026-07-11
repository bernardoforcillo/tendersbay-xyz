package knowledge_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/buildwithgo/berrygem/rag"
	"github.com/google/uuid"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
)

// pointNamespace mirrors the unexported namespace UUID knowledgebase.go uses
// to derive deterministic Qdrant point IDs (see Ingest). knowledgebase_test.go
// is package knowledge_test (external), so it can't call the unexported
// pointID helper directly — recomputing the UUID here with the same fixed
// namespace is safe since the namespace value isn't itself sensitive.
var pointNamespace = uuid.MustParse("99753bb2-77f9-4686-a470-c3cbfc566fa6")

// wantPointID computes the expected deterministic Qdrant point ID for one
// chunk of one document, matching knowledgebase.go's pointID helper.
func wantPointID(docID string, index int) string {
	key := fmt.Sprintf("%s_chunk_%d", docID, index)
	return uuid.NewSHA1(pointNamespace, []byte(key)).String()
}

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
	wantID := wantPointID("42", 0)
	if p0["id"] != wantID {
		t.Errorf("points[0].id = %v, want %v (deterministic UUIDv5 of 42_chunk_0)", p0["id"], wantID)
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
	wantID := wantPointID("7", 0)
	if p0["id"] != wantID {
		t.Errorf("points[0].id = %v, want %v (deterministic UUIDv5 of 7_chunk_0)", p0["id"], wantID)
	}
	payload := p0["payload"].(map[string]any)
	if payload["content"] != "Comune di Roma — Appalto pulizie" {
		t.Errorf("points[0].payload.content = %v, want doc.Content", payload["content"])
	}
}

func TestIngest_WithEmptyNonNilChunks_DoesNotPanic(t *testing.T) {
	kb := newTestKnowledgeBase(t,
		alwaysExistingCollectionHandler(t, nil),
		fakeOllamaEmbed([]float32{0.3}),
	)

	// doc.Chunks is non-nil but zero-length: Ingest falls back to a single
	// chunk built from doc.Content, so the write-back guard must not index
	// into the original empty slice.
	doc := &rag.Document{ID: "9", Content: "Comune di Milano — Fornitura arredi", Chunks: []rag.Chunk{}}

	if err := kb.Ingest(context.Background(), doc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
}

func TestSearch_ReconstructsChunksFromPayload(t *testing.T) {
	var gotSearchBody map[string]any
	qdrantHandler := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"status":"green","vectors_count":0,"points_count":0,"segments_count":1,"config":{}},"status":"ok","time":0.01}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/tenders/points/search":
			_ = json.NewDecoder(r.Body).Decode(&gotSearchBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":[{"id":"42_chunk_0","score":0.87,"payload":{"content":"Lavori stradali","tender_id":"42","chunk_index":0,"source":"ted"}}],"status":"ok","time":0.01}`))
		default:
			t.Errorf("unexpected qdrant request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
	kb := newTestKnowledgeBase(t, qdrantHandler, fakeOllamaEmbed([]float32{0.9, 0.1}))

	chunks, err := kb.Search(context.Background(), "lavori stradali Blaj", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("len(chunks) = %d, want 1", len(chunks))
	}
	if chunks[0].Content != "Lavori stradali" {
		t.Errorf("chunks[0].Content = %q, want %q", chunks[0].Content, "Lavori stradali")
	}
	if chunks[0].DocID != "42" {
		t.Errorf("chunks[0].DocID = %q, want %q", chunks[0].DocID, "42")
	}
	if chunks[0].Index != 0 {
		t.Errorf("chunks[0].Index = %d, want 0", chunks[0].Index)
	}

	if gotSearchBody["limit"] != float64(5) {
		t.Errorf("search request limit = %v, want 5", gotSearchBody["limit"])
	}
	vec, _ := gotSearchBody["vector"].([]any)
	if len(vec) != 2 {
		t.Errorf("search request vector = %v, want the 2-element embedded query vector", gotSearchBody["vector"])
	}
}

func TestSearch_DefaultsLimitWhenZero(t *testing.T) {
	var gotSearchBody map[string]any
	qdrantHandler := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"status":"green","vectors_count":0,"points_count":0,"segments_count":1,"config":{}},"status":"ok","time":0.01}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/tenders/points/search":
			_ = json.NewDecoder(r.Body).Decode(&gotSearchBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":[],"status":"ok","time":0.01}`))
		default:
			t.Errorf("unexpected qdrant request: %s %s", r.Method, r.URL.Path)
		}
	}
	kb := newTestKnowledgeBase(t, qdrantHandler, fakeOllamaEmbed([]float32{0.1}))

	if _, err := kb.Search(context.Background(), "query", 0); err != nil {
		t.Fatalf("Search: %v", err)
	}
	// drops/qdrant.Client.Search defaults Limit to 10 when <= 0 — confirm
	// that default reaches the wire request via this KnowledgeBase.
	if gotSearchBody["limit"] != float64(10) {
		t.Errorf("search request limit = %v, want 10 (drops/qdrant's own default)", gotSearchBody["limit"])
	}
}

func TestDelete_FiltersByTenderID(t *testing.T) {
	var gotDeleteBody map[string]any
	qdrantHandler := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"status":"green","vectors_count":0,"points_count":0,"segments_count":1,"config":{}},"status":"ok","time":0.01}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/tenders/points/delete":
			_ = json.NewDecoder(r.Body).Decode(&gotDeleteBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"operation_id":1,"status":"completed"},"status":"ok","time":0.01}`))
		default:
			t.Errorf("unexpected qdrant request: %s %s", r.Method, r.URL.Path)
		}
	}
	kb := newTestKnowledgeBase(t, qdrantHandler, fakeOllamaEmbed([]float32{0}))

	if err := kb.Delete(context.Background(), "42"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	filter, ok := gotDeleteBody["filter"].(map[string]any)
	if !ok {
		t.Fatalf("delete request has no filter: %v", gotDeleteBody)
	}
	must, _ := filter["must"].([]any)
	if len(must) != 1 {
		t.Fatalf("filter.must = %v, want one condition", must)
	}
	cond := must[0].(map[string]any)
	if cond["key"] != "tender_id" {
		t.Errorf("filter condition key = %v, want tender_id", cond["key"])
	}
	match := cond["match"].(map[string]any)
	if match["value"] != "42" {
		t.Errorf("filter condition match.value = %v, want 42", match["value"])
	}
}

func TestList_GroupsChunksByTenderID(t *testing.T) {
	qdrantHandler := func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"status":"green","vectors_count":0,"points_count":0,"segments_count":1,"config":{}},"status":"ok","time":0.01}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/tenders/points/scroll":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"points":[
				{"id":"42_chunk_0","payload":{"content":"parte 1","tender_id":"42"}},
				{"id":"42_chunk_1","payload":{"content":"parte 2","tender_id":"42"}},
				{"id":"7_chunk_0","payload":{"content":"altro tender","tender_id":"7"}}
			],"next_page_offset":null},"status":"ok","time":0.01}`))
		default:
			t.Errorf("unexpected qdrant request: %s %s", r.Method, r.URL.Path)
		}
	}
	kb := newTestKnowledgeBase(t, qdrantHandler, fakeOllamaEmbed([]float32{0}))

	docs, err := kb.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(docs) != 2 {
		t.Fatalf("len(docs) = %d, want 2 (grouped by tender_id)", len(docs))
	}
	byID := map[string]int{}
	for _, d := range docs {
		byID[d.ID] = len(d.Chunks)
	}
	if byID["42"] != 2 {
		t.Errorf("doc 42 has %d chunks, want 2", byID["42"])
	}
	if byID["7"] != 1 {
		t.Errorf("doc 7 has %d chunks, want 1", byID["7"])
	}
}
