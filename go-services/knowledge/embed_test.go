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
	if gotBody["input"] != "appalto pubblico" {
		t.Errorf("input = %v, want %q", gotBody["input"], "appalto pubblico")
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
