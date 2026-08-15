package knowledge_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/buildwithgo/berrygem/rag"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
)

func int64p(v int64) *int64 { return &v }

func mustTime(t *testing.T, s string) *time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return &parsed
}

// The prefix expansion is what makes "cpv starts with 45" expressible against
// a Qdrant keyword payload, which has no prefix operator — so the exact set of
// materialised lengths is a contract between Payload and SearchFilter, not an
// implementation detail.
func TestAttributes_Payload_ExpandsCPVPrefixes(t *testing.T) {
	attrs := knowledge.Attributes{
		CPV:          "45233120-9", // the check digit is not part of the code
		CPVSecondary: []string{"45233"},
	}
	got, _ := attrs.Payload()["cpv_prefixes"].([]string)
	sort.Strings(got)

	want := []string{
		"45", "452", "4523", "45233", "452331", "4523312", "45233120", // primary
		// the secondary code's own prefixes are all already covered above,
		// and must not appear twice
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cpv_prefixes = %v, want %v", got, want)
	}
}

func TestAttributes_Payload_ExpandsNUTSPrefixes(t *testing.T) {
	got, _ := knowledge.Attributes{NUTS: "ITC4C"}.Payload()["nuts_prefixes"].([]string)
	want := []string{"IT", "ITC", "ITC4", "ITC4C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("nuts_prefixes = %v, want %v", got, want)
	}
}

// "Unknown" and "zero" must stay distinguishable: a tender with no value must
// not answer a "value <= 100000" filter as if its value were 0.
func TestAttributes_Payload_OmitsUnsetFields(t *testing.T) {
	got := knowledge.Attributes{Title: "Solo titolo"}.Payload()
	for _, key := range []string{"country", "status", "cpv_prefixes", "nuts_prefixes", "value", "published_at", "deadline"} {
		if _, present := got[key]; present {
			t.Errorf("payload[%q] present for an all-unset Attributes: %v", key, got[key])
		}
	}
	if got["title"] != "Solo titolo" {
		t.Errorf("payload[title] = %v, want the title", got["title"])
	}
}

func TestAttributes_Payload_EncodesTimesAsUnixSeconds(t *testing.T) {
	deadline := mustTime(t, "2026-09-01T12:00:00Z")
	got := knowledge.Attributes{Deadline: deadline, Value: int64p(250000)}.Payload()
	if got["deadline"] != deadline.Unix() {
		t.Errorf("payload[deadline] = %v, want %d", got["deadline"], deadline.Unix())
	}
	if got["value"] != int64(250000) {
		t.Errorf("payload[value] = %v, want 250000", got["value"])
	}
}

func TestSearchFilter_IsZero(t *testing.T) {
	if !(knowledge.SearchFilter{}).IsZero() {
		t.Error("zero SearchFilter should report IsZero")
	}
	if (knowledge.SearchFilter{Countries: []string{"IT"}}).IsZero() {
		t.Error("SearchFilter with a country should not report IsZero")
	}
}

// searchFilterBody runs one filtered search against a stub Qdrant and returns
// the "filter" object it received, or nil when none was sent.
func searchFilterBody(t *testing.T, filter knowledge.SearchFilter) map[string]any {
	t.Helper()
	var body map[string]any
	kb := newTestKnowledgeBase(t,
		func(w http.ResponseWriter, r *http.Request) {
			switch {
			case r.Method == http.MethodGet && r.URL.Path == "/collections/tenders":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"result":{"status":"green"},"status":"ok","time":0.01}`))
			case r.Method == http.MethodPut && r.URL.Path == "/collections/tenders/index":
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"result":{"status":"acknowledged"},"status":"ok","time":0.01}`))
			case r.Method == http.MethodPost && r.URL.Path == "/collections/tenders/points/search":
				_ = json.NewDecoder(r.Body).Decode(&body)
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"result":[],"status":"ok","time":0.01}`))
			default:
				t.Errorf("unexpected qdrant request: %s %s", r.Method, r.URL.Path)
				w.WriteHeader(http.StatusInternalServerError)
			}
		},
		fakeOllamaEmbed([]float32{0.1, 0.2}),
	)
	if _, err := kb.SearchFiltered(context.Background(), "pulizie", 10, filter); err != nil {
		t.Fatalf("SearchFiltered: %v", err)
	}
	f, _ := body["filter"].(map[string]any)
	return f
}

func TestSearchFiltered_SendsNoFilterWhenUnconstrained(t *testing.T) {
	if got := searchFilterBody(t, knowledge.SearchFilter{}); got != nil {
		t.Errorf("filter = %v, want it omitted entirely for an unconstrained search", got)
	}
}

func TestSearchFiltered_PushesFacetsIntoQdrant(t *testing.T) {
	got := searchFilterBody(t, knowledge.SearchFilter{
		Countries:    []string{"IT", "DE"},
		Statuses:     []string{"open"},
		CPVPrefixes:  []string{"45"},
		DeadlineFrom: mustTime(t, "2026-08-01T00:00:00Z"),
		ValueMax:     int64p(100000),
	})
	must, _ := got["must"].([]any)
	if len(must) != 5 {
		t.Fatalf("must = %v, want one clause per constrained facet (5)", must)
	}

	byKey := map[string]map[string]any{}
	for _, c := range must {
		cond := c.(map[string]any)
		byKey[cond["key"].(string)] = cond
	}

	// Several values for one facet are an OR, matching how the SQL side reads
	// a multi-select.
	countryAny := byKey["country"]["match"].(map[string]any)["any"].([]any)
	if len(countryAny) != 2 || countryAny[0] != "IT" || countryAny[1] != "DE" {
		t.Errorf("country match.any = %v, want [IT DE]", countryAny)
	}
	if byKey["cpv_prefixes"]["match"].(map[string]any)["any"].([]any)[0] != "45" {
		t.Errorf("cpv_prefixes clause = %v, want a match on 45", byKey["cpv_prefixes"])
	}
	if byKey["value"]["range"].(map[string]any)["lte"] != float64(100000) {
		t.Errorf("value range = %v, want lte 100000", byKey["value"]["range"])
	}
	wantFrom := float64(mustTime(t, "2026-08-01T00:00:00Z").Unix())
	if byKey["deadline"]["range"].(map[string]any)["gte"] != wantFrom {
		t.Errorf("deadline range = %v, want gte %v", byKey["deadline"]["range"], wantFrom)
	}
}

// A prefix nobody materialised at index time can't be matched exactly, so it
// must be DROPPED rather than approximated — the Postgres pass still applies
// it, and a wrong pushdown would silently hide matching tenders instead of
// merely wasting candidates.
func TestSearchFiltered_DropsUnmaterialisedPrefixLengths(t *testing.T) {
	got := searchFilterBody(t, knowledge.SearchFilter{
		CPVPrefixes:  []string{"4"},           // shorter than the 2-digit minimum
		NUTSPrefixes: []string{"ITC4C1"},      // longer than the 5-char maximum
		Countries:    []string{"IT", "", " "}, // blank selections are not a `= ''` test
	})
	must, _ := got["must"].([]any)
	if len(must) != 1 {
		t.Fatalf("must = %v, want only the country clause to survive", must)
	}
	cond := must[0].(map[string]any)
	if cond["key"] != "country" {
		t.Fatalf("surviving clause = %v, want the country one", cond)
	}
	// The empty string is dropped; " " is a real (if useless) value and is not
	// this function's business to second-guess.
	if vals := cond["match"].(map[string]any)["any"].([]any); len(vals) != 2 {
		t.Errorf("country match.any = %v, want the empty selection dropped", vals)
	}
}

func TestIngestWithAttributes_StampsFacetsOnEveryPoint(t *testing.T) {
	var gotBody map[string]any
	kb := newTestKnowledgeBase(t,
		alwaysExistingCollectionHandler(t, func(body map[string]any) { gotBody = body }),
		fakeOllamaEmbed([]float32{0.1}),
	)

	doc := &rag.Document{
		ID:       "42",
		Metadata: map[string]string{"source": "ted"},
		Chunks: []rag.Chunk{
			{ID: "42_chunk_0", DocID: "42", Index: 0, Content: "riassunto"},
			{ID: "42_chunk_1", DocID: "42", Index: 1, Content: "allegato tecnico"},
		},
	}
	attrs := knowledge.Attributes{
		Title:    "Lavori stradali",
		Country:  "IT",
		Status:   "open",
		CPV:      "45233120",
		NUTS:     "ITC4",
		Value:    int64p(500000),
		Deadline: mustTime(t, "2026-09-01T12:00:00Z"),
	}

	if err := kb.IngestWithAttributes(context.Background(), doc, attrs); err != nil {
		t.Fatalf("IngestWithAttributes: %v", err)
	}

	points := gotBody["points"].([]any)
	if len(points) != 2 {
		t.Fatalf("points = %d, want 2", len(points))
	}
	// Every chunk carries the facets — a filter matches points, not documents,
	// so stamping only the summary chunk would make the rest unfilterable.
	for i, p := range points {
		payload := p.(map[string]any)["payload"].(map[string]any)
		if payload["country"] != "IT" || payload["status"] != "open" {
			t.Errorf("points[%d].payload = %+v, want country=IT status=open", i, payload)
		}
		if payload["value"] != float64(500000) {
			t.Errorf("points[%d].payload.value = %v, want 500000", i, payload["value"])
		}
		if payload["source"] != "ted" {
			t.Errorf("points[%d].payload.source = %v, want metadata passed through", i, payload["source"])
		}
	}
}

// The title slot of EmbeddingGemma's document prompt should come from
// Attributes.Title without the caller having to duplicate it into Metadata.
func TestIngestWithAttributes_UsesTitleForTheDocumentPrompt(t *testing.T) {
	var gotInput string
	kb := newTestKnowledgeBase(t,
		alwaysExistingCollectionHandler(t, nil),
		func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			gotInput = firstInput(body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(embeddingsFor(body, []float32{0.1}))
		},
	)

	doc := &rag.Document{ID: "42", Content: "riassunto"}
	if err := kb.IngestWithAttributes(context.Background(), doc, knowledge.Attributes{Title: "Lavori stradali"}); err != nil {
		t.Fatalf("IngestWithAttributes: %v", err)
	}
	if want := "title: Lavori stradali | text: riassunto"; gotInput != want {
		t.Errorf("embed input = %q, want %q", gotInput, want)
	}
}
