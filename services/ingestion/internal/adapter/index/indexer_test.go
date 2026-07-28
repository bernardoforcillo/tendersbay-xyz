package index_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/buildwithgo/berrygem/rag"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/knowledge"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/index"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/postgres"
)

type fakeRepo struct {
	unindexed      []postgres.UnindexedTender
	parts          map[int64][]string // documentID -> parts
	savedParts     map[int64][]string
	indexedIDs     []int64
	markIndexedErr error
}

func (f *fakeRepo) ListUnindexed(_ context.Context, limit int) ([]postgres.UnindexedTender, error) {
	if len(f.unindexed) > limit {
		return f.unindexed[:limit], nil
	}
	return f.unindexed, nil
}

func (f *fakeRepo) DocumentParts(_ context.Context, documentID int64) ([]string, error) {
	return f.parts[documentID], nil
}

func (f *fakeRepo) SaveDocumentParts(_ context.Context, documentID int64, parts []string) error {
	if f.savedParts == nil {
		f.savedParts = map[int64][]string{}
	}
	f.savedParts[documentID] = parts
	return nil
}

func (f *fakeRepo) MarkIndexed(_ context.Context, tenderID int64) error {
	if f.markIndexedErr != nil {
		return f.markIndexedErr
	}
	f.indexedIDs = append(f.indexedIDs, tenderID)
	return nil
}

type fakeKnowledgeBase struct {
	ingested []*rag.Document
	attrs    []knowledge.Attributes
	err      error
}

func (f *fakeKnowledgeBase) IngestWithAttributes(_ context.Context, doc *rag.Document, attrs knowledge.Attributes) error {
	if f.err != nil {
		return f.err
	}
	f.ingested = append(f.ingested, doc)
	f.attrs = append(f.attrs, attrs)
	return nil
}

type fakeFetcher struct {
	partsByURL map[string][]string
	err        error
}

func (f *fakeFetcher) FetchAndExtract(_ context.Context, url string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.partsByURL[url], nil
}

func TestRunOnce_IndexesTenderWithSummaryOnly(t *testing.T) {
	repo := &fakeRepo{unindexed: []postgres.UnindexedTender{
		{ID: 42, Title: "Lavori stradali", BuyerName: "Comune di Roma", CPV: "45233220",
			ProcedureType: "open", Country: "IT", Status: "open", Source: "ted", SourceRef: "proc-1"},
	}}
	kb := &fakeKnowledgeBase{}
	fetcher := &fakeFetcher{}

	idx := index.New(repo, kb, fetcher)
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(kb.ingested) != 1 {
		t.Fatalf("len(kb.ingested) = %d, want 1", len(kb.ingested))
	}
	doc := kb.ingested[0]
	if doc.ID != "42" {
		t.Errorf("doc.ID = %q, want %q", doc.ID, "42")
	}
	if len(doc.Chunks) != 1 {
		t.Fatalf("len(doc.Chunks) = %d, want 1 (summary only, no documents)", len(doc.Chunks))
	}
	if doc.Chunks[0].Index != 0 {
		t.Errorf("doc.Chunks[0].Index = %d, want 0", doc.Chunks[0].Index)
	}
	for _, want := range []string{"Lavori stradali", "Comune di Roma", "45233220", "open", "IT"} {
		if !contains(doc.Chunks[0].Content, want) {
			t.Errorf("summary chunk %q does not contain %q", doc.Chunks[0].Content, want)
		}
	}
	if doc.Metadata["source"] != "ted" || doc.Metadata["source_ref"] != "proc-1" {
		t.Errorf("doc.Metadata = %+v, want source=ted source_ref=proc-1", doc.Metadata)
	}
	if len(repo.indexedIDs) != 1 || repo.indexedIDs[0] != 42 {
		t.Errorf("repo.indexedIDs = %v, want [42]", repo.indexedIDs)
	}
}

// The facets travel alongside the embedded text, not inside it: they are what
// the search API filters the vector store on, so a column dropped here becomes
// a filter that silently can't be pushed down.
func TestRunOnce_ProjectsFilterableAttributes(t *testing.T) {
	value := int64(500000)
	deadline := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	published := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	repo := &fakeRepo{unindexed: []postgres.UnindexedTender{
		{
			ID: 42, Title: "Lavori stradali", BuyerName: "Comune di Roma",
			CPV: "45233220", CPVSecondary: []string{"45233120"}, NUTS: "ITI43",
			Country: "IT", Status: "open", Source: "ted", SourceRef: "proc-1",
			Value: &value, PublishedAt: &published, Deadline: &deadline,
		},
	}}
	kb := &fakeKnowledgeBase{}

	idx := index.New(repo, kb, &fakeFetcher{})
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if len(kb.attrs) != 1 {
		t.Fatalf("len(kb.attrs) = %d, want 1", len(kb.attrs))
	}
	got := kb.attrs[0]
	want := knowledge.Attributes{
		Title: "Lavori stradali", Country: "IT", Status: "open",
		CPV: "45233220", CPVSecondary: []string{"45233120"}, NUTS: "ITI43",
		Value: &value, PublishedAt: &published, Deadline: &deadline,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("attributes = %+v, want %+v", got, want)
	}
}

// A tender with no value/deadline must project nil, not a zero time or a zero
// value — a range filter has to exclude it rather than treat it as 0.
func TestRunOnce_LeavesUnknownFacetsNil(t *testing.T) {
	repo := &fakeRepo{unindexed: []postgres.UnindexedTender{
		{ID: 7, Title: "Fornitura arredi", Country: "IT", Status: "open", Source: "ted", SourceRef: "proc-2"},
	}}
	kb := &fakeKnowledgeBase{}

	idx := index.New(repo, kb, &fakeFetcher{})
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	got := kb.attrs[0]
	if got.Value != nil || got.Deadline != nil || got.PublishedAt != nil {
		t.Errorf("attributes = %+v, want Value/Deadline/PublishedAt all nil", got)
	}
}

func TestRunOnce_SummaryIncludesDescriptionWhenPresent(t *testing.T) {
	repo := &fakeRepo{unindexed: []postgres.UnindexedTender{
		{ID: 42, Title: "Lavori stradali", Description: "Riasfaltatura delle strade del centro storico.",
			BuyerName: "Comune di Roma", CPV: "45233220", Country: "IT", Status: "open", Source: "ted", SourceRef: "proc-1"},
	}}
	kb := &fakeKnowledgeBase{}
	fetcher := &fakeFetcher{}

	idx := index.New(repo, kb, fetcher)
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	summary := kb.ingested[0].Chunks[0].Content
	if !contains(summary, "Riasfaltatura delle strade del centro storico.") {
		t.Errorf("summary chunk %q does not contain the description", summary)
	}
	if !contains(summary, "Lavori stradali") {
		t.Errorf("summary chunk %q does not still contain the title", summary)
	}
}

func TestRunOnce_DownloadsAndPersistsDocumentPartsWhenNotAlreadySaved(t *testing.T) {
	repo := &fakeRepo{
		unindexed: []postgres.UnindexedTender{
			{ID: 1, Title: "T", Documents: []postgres.UnindexedDocument{{ID: 100, URL: "https://example.org/a.pdf"}}},
		},
		parts: map[int64][]string{}, // nothing saved yet for document 100
	}
	kb := &fakeKnowledgeBase{}
	fetcher := &fakeFetcher{partsByURL: map[string][]string{
		"https://example.org/a.pdf": {"extracted part one", "extracted part two"},
	}}

	idx := index.New(repo, kb, fetcher)
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if got := repo.savedParts[100]; len(got) != 2 || got[0] != "extracted part one" {
		t.Errorf("repo.savedParts[100] = %v, want the fetched parts persisted", got)
	}
	doc := kb.ingested[0]
	if len(doc.Chunks) != 3 { // summary + 2 document parts
		t.Fatalf("len(doc.Chunks) = %d, want 3 (summary + 2 parts)", len(doc.Chunks))
	}
	if doc.Chunks[1].Index != 1 || doc.Chunks[2].Index != 2 {
		t.Errorf("chunk indices = [%d, %d], want [1, 2] (globally unique per tender)",
			doc.Chunks[1].Index, doc.Chunks[2].Index)
	}
}

func TestRunOnce_SkipsRedownloadWhenPartsAlreadyExist(t *testing.T) {
	fetchCalled := false
	repo := &fakeRepo{
		unindexed: []postgres.UnindexedTender{
			{ID: 1, Title: "T", Documents: []postgres.UnindexedDocument{{ID: 100, URL: "https://example.org/a.pdf"}}},
		},
		parts: map[int64][]string{100: {"already extracted"}},
	}
	kb := &fakeKnowledgeBase{}
	fetcher := &fakeFetcher{partsByURL: map[string][]string{
		"https://example.org/a.pdf": {"should not be called"},
	}}

	idx := index.New(repo, kb, testFetcherFunc(func(ctx context.Context, url string) ([]string, error) {
		fetchCalled = true
		return fetcher.FetchAndExtract(ctx, url)
	}))
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if fetchCalled {
		t.Error("FetchAndExtract was called even though parts already existed in Postgres")
	}
	doc := kb.ingested[0]
	if len(doc.Chunks) != 2 || doc.Chunks[1].Content != "already extracted" {
		t.Errorf("doc.Chunks = %+v, want summary + the already-saved part", doc.Chunks)
	}
}

type testFetcherFunc func(ctx context.Context, url string) ([]string, error)

func (f testFetcherFunc) FetchAndExtract(ctx context.Context, url string) ([]string, error) {
	return f(ctx, url)
}

func TestRunOnce_LogsAndContinuesOnIngestFailure(t *testing.T) {
	repo := &fakeRepo{unindexed: []postgres.UnindexedTender{
		{ID: 1, Title: "Fails to ingest"},
		{ID: 2, Title: "Succeeds"},
	}}
	kb := &fakeKnowledgeBase{err: errors.New("qdrant unreachable")}
	fetcher := &fakeFetcher{}

	idx := index.New(repo, kb, fetcher)
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: want nil error even when individual tenders fail to ingest, got %v", err)
	}
	if len(repo.indexedIDs) != 0 {
		t.Errorf("repo.indexedIDs = %v, want none marked indexed (Ingest always failed)", repo.indexedIDs)
	}
}

func TestRunOnce_ChunkIndexContinuesAcrossMultipleDocuments(t *testing.T) {
	repo := &fakeRepo{
		unindexed: []postgres.UnindexedTender{
			{ID: 1, Title: "T", Documents: []postgres.UnindexedDocument{
				{ID: 100, URL: "https://example.org/a.pdf"},
				{ID: 200, URL: "https://example.org/b.pdf"},
			}},
		},
		parts: map[int64][]string{
			100: {"doc one part"},
			200: {"doc two part a", "doc two part b"},
		},
	}
	kb := &fakeKnowledgeBase{}
	fetcher := &fakeFetcher{}

	idx := index.New(repo, kb, fetcher)
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	doc := kb.ingested[0]
	if len(doc.Chunks) != 4 { // summary + 1 (doc 100) + 2 (doc 200)
		t.Fatalf("len(doc.Chunks) = %d, want 4 (summary + 1 + 2)", len(doc.Chunks))
	}
	for i, chunk := range doc.Chunks {
		if chunk.Index != i {
			t.Errorf("doc.Chunks[%d].Index = %d, want %d (continuous across documents, not reset per-document)",
				i, chunk.Index, i)
		}
	}
}

func TestRunOnce_LogsAndContinuesOnMarkIndexedFailure(t *testing.T) {
	repo := &fakeRepo{
		unindexed:      []postgres.UnindexedTender{{ID: 1, Title: "T"}},
		markIndexedErr: errors.New("db unreachable"),
	}
	kb := &fakeKnowledgeBase{}
	fetcher := &fakeFetcher{}

	idx := index.New(repo, kb, fetcher)
	if err := idx.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: want nil error even when MarkIndexed fails, got %v", err)
	}
	if len(kb.ingested) != 1 {
		t.Fatalf("len(kb.ingested) = %d, want 1 (ingest itself succeeded)", len(kb.ingested))
	}
	if len(repo.indexedIDs) != 0 {
		t.Errorf("repo.indexedIDs = %v, want none (MarkIndexed failed, so the fake never appended)", repo.indexedIDs)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
