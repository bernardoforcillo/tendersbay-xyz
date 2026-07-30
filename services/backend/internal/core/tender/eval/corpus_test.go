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
		t.Fatalf("len(corpus) = %d, want 2", len(corpus))
	}
	if got := corpus[0].Key(); got != "ted:de-1" {
		t.Errorf("Key() = %q, want ted:de-1", got)
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
		t.Error("LoadCorpus = nil error, want a duplicate-key failure")
	}
}
