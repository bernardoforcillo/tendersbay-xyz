package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
)

func TestRelatedByDocID_ExcludesSelfAndDedupesByTender(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":{"status":"green"},"status":"ok","time":0.01}`))
		case r.Method == http.MethodPost && r.URL.Path == "/collections/tenders/points/recommend":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"result":[
				{"id":"a","score":0.91,"payload":{"tender_id":"7","content":"c","chunk_index":0}},
				{"id":"b","score":0.80,"payload":{"tender_id":"7","content":"c","chunk_index":1}},
				{"id":"c","score":0.75,"payload":{"tender_id":"42","content":"c","chunk_index":0}},
				{"id":"d","score":0.60,"payload":{"tender_id":"9","content":"c","chunk_index":0}}
			],"status":"ok","time":0.01}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	kb, err := knowledge.NewKnowledgeBase(context.Background(), srv.URL, "http://localhost:11434", "embeddinggemma:latest")
	if err != nil {
		t.Fatalf("NewKnowledgeBase: %v", err)
	}
	got, err := kb.RelatedByDocID(context.Background(), "42", 10)
	if err != nil {
		t.Fatalf("RelatedByDocID: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2 (self 42 excluded, tender 7 deduped)", len(got))
	}
	if got[0].DocID != "7" || got[1].DocID != "9" {
		t.Errorf("docIDs = [%s, %s], want [7, 9]", got[0].DocID, got[1].DocID)
	}
	if got[0].Score != float32(0.91) {
		t.Errorf("tender 7 score = %v, want the higher chunk's 0.91", got[0].Score)
	}
}
