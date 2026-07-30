package eval

import (
	"bytes"
	"compress/gzip"
	"io/fs"
	"os"
	"testing"
	"testing/fstest"
)

// mapFS is a tiny in-memory fs.FS so the validation tests don't need fixture
// files on disk for inputs that are meant to be rejected.
type mapFS map[string][]byte

func (m mapFS) Open(name string) (fs.File, error) {
	files := make(fstest.MapFS, len(m))
	for n, data := range m {
		files[n] = &fstest.MapFile{Data: data}
	}
	return files.Open(name)
}

func TestLoadCorpus_ReadsGzippedJSONL(t *testing.T) {
	corpus, err := LoadCorpus(os.DirFS("testdata"), "corpus-tiny.jsonl.gz")
	if err != nil {
		t.Fatalf("LoadCorpus: %v", err)
	}
	if len(corpus) != 2 {
		t.Fatalf("len(corpus) = %d, want 2 — a loader that silently drops or duplicates a record would still return no error, and the count is the only place that shows up", len(corpus))
	}
	if got := corpus[0].Key(); got != "ted:de-1" {
		t.Errorf("Key() = %q, want ted:de-1 — Key() is the join key golden-set judgements are written against; a wrong composition would silently detach every judgement from its tender", got)
	}
	if corpus[0].Language != "de" {
		t.Errorf("Language = %q, want de — the harness reports by document language", corpus[0].Language)
	}
}

func TestLoadCorpus_RejectsADuplicateKey(t *testing.T) {
	// Two records with the same source:source_ref would both claim the same
	// judgements and inflate whichever one the retriever happened to return.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	line := `{"source":"ted","source_ref":"a","title":"t","language":"it","country":"IT"}` + "\n"
	if _, err := zw.Write([]byte(line + line)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := LoadCorpus(mapFS{"c.jsonl.gz": buf.Bytes()}, "c.jsonl.gz"); err == nil {
		t.Error("LoadCorpus = nil error, want a duplicate-key failure — a silently accepted duplicate key would let two records claim the same judgements, inflating whichever one the retriever happened to return")
	}
}

func TestLoadCorpus_RejectsARecordMissingSourceOrSourceRef(t *testing.T) {
	// Judgements key on source:source_ref. A record missing either half of
	// that key can never be matched by any golden query's judgements, and
	// would silently sit in the corpus as unreachable, unscoreable noise.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(`{"source":"ted","title":"t","language":"it","country":"IT"}` + "\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := LoadCorpus(mapFS{"c.jsonl.gz": buf.Bytes()}, "c.jsonl.gz"); err == nil {
		t.Error("LoadCorpus = nil error, want a missing-source-ref failure — a silently accepted record with no source_ref can never be matched by any judgement key, sitting in the corpus as unreachable, unscoreable noise")
	}
}

func TestLoadCorpus_RejectsAnEmptyCorpus(t *testing.T) {
	// An empty corpus means every golden query's judgements reference tenders
	// that do not exist; the harness would silently compute every metric
	// against nothing while looking like a clean, if uneventful, run.
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := zw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := LoadCorpus(mapFS{"c.jsonl.gz": buf.Bytes()}, "c.jsonl.gz"); err == nil {
		t.Error("LoadCorpus = nil error, want an empty-corpus failure — a silently accepted empty corpus would let every metric compute against nothing while looking like a clean run")
	}
}
