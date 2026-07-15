package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
)

func TestNewKnowledgeBase_CreatesCollectionIfMissing(t *testing.T) {
	var gotCreateBody map[string]any
	created := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("{\"status\":{\"error\":\"not found: Collection `tenders` doesn't exist!\"}}"))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/tenders":
			created = true
			_ = json.NewDecoder(r.Body).Decode(&gotCreateBody)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":true,"status":"ok","time":0.01}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	_, err := knowledge.NewKnowledgeBase(context.Background(), srv.URL, "http://localhost:11434", "embeddinggemma:latest")
	if err != nil {
		t.Fatalf("NewKnowledgeBase: %v", err)
	}
	if !created {
		t.Fatal("collection was not created")
	}
	vectors, _ := gotCreateBody["vectors"].(map[string]any)
	if vectors["size"] != float64(768) {
		t.Errorf("vectors.size = %v, want 768", vectors["size"])
	}
	if vectors["distance"] != "Cosine" {
		t.Errorf("vectors.distance = %v, want Cosine", vectors["distance"])
	}
}

func TestNewKnowledgeBase_SkipsCreateIfCollectionExists(t *testing.T) {
	putCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"status":"green","vectors_count":0,"points_count":0,"segments_count":1,"config":{}},"status":"ok","time":0.01}`))
		case r.Method == http.MethodPut && r.URL.Path == "/collections/tenders":
			putCalled = true
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	_, err := knowledge.NewKnowledgeBase(context.Background(), srv.URL, "http://localhost:11434", "embeddinggemma:latest")
	if err != nil {
		t.Fatalf("NewKnowledgeBase: %v", err)
	}
	if putCalled {
		t.Fatal("CreateCollection was called even though the collection already exists")
	}
}
