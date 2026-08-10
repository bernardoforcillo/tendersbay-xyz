package webdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture.pdf is a copy of internal/adapter/document/testdata/fixture.pdf — a
// minimal one-page PDF reading "Comune di Roma appalto lavori stradali". It is
// copied rather than referenced across the package boundary so this package's
// tests keep working if the document package reorganises its own testdata.
func fixturePDF(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "fixture.pdf"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return body
}

func TestExtractorReadsAPDFAndCarriesProvenance(t *testing.T) {
	parts, err := Extractor{}.Extract(Document{
		URL:         "https://appalti.example.it/gare/123/disciplinare.pdf",
		ContentType: "application/pdf",
		Filename:    "disciplinare.pdf",
		Body:        fixturePDF(t),
	})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	if !strings.Contains(parts[0].Text, "Comune di Roma") {
		t.Errorf("parts[0].Text = %q", parts[0].Text)
	}
	// The page span is the half of a citation that text alone cannot supply,
	// and it is unrecoverable once the bytes are gone — which, in this package,
	// is the moment Extract returns.
	if parts[0].PageStart != 1 || parts[0].PageEnd != 1 {
		t.Errorf("page span = %d-%d, want 1-1", parts[0].PageStart, parts[0].PageEnd)
	}
}

// TestExtractorLeavesNothingOnDisk is the storage decision as an assertion.
// There is no object storage in this design and no blob column anywhere; the
// only place a retrieved file has ever existed on disk is the temp file this
// function writes and removes, and a leak here would quietly turn a batch pod's
// /tmp into unbounded state.
func TestExtractorLeavesNothingOnDisk(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)

	if _, err := (Extractor{}).Extract(Document{
		ContentType: "application/pdf",
		Filename:    "disciplinare.pdf",
		Body:        fixturePDF(t),
	}); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp dir still holds %d entries after extraction: %v", len(entries), entries)
	}
}

// TestExtractorLeavesNothingOnDiskWhenExtractionFails covers the path that
// matters more, because it is the one a scanned or corrupt file takes — and
// there will be many of those.
func TestExtractorLeavesNothingOnDiskWhenExtractionFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)

	if _, err := (Extractor{}).Extract(Document{
		ContentType: "application/pdf",
		Filename:    "broken.pdf",
		Body:        []byte("%PDF-1.4 this is not a PDF past its first line"),
	}); err == nil {
		t.Fatal("Extract: want an error for a corrupt PDF")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp dir still holds %d entries after a failed extraction: %v", len(entries), entries)
	}
}

// TestExtractorIsSilentAboutFormatsThisPhaseCannotRead is the contract that
// keeps the coverage measurement honest. .p7m is ubiquitous in Italy and .zip is
// common; both are out of scope here, and the gap between "files found" and
// "files extracted" is what will decide whether they are worth building. An
// error per file would log the same known limitation fifteen times a tender and
// make a tender of unreadable files look like a tender whose portal was down.
func TestExtractorIsSilentAboutFormatsThisPhaseCannotRead(t *testing.T) {
	unreadable := []Document{
		{ContentType: "application/zip", Filename: "allegati.zip", Body: []byte("PK\x03\x04")},
		{ContentType: "application/pkcs7-mime", Filename: "disciplinare.pdf.p7m", Body: []byte("\x30\x82")},
		{ContentType: "application/msword", Filename: "modello-a.doc", Body: []byte("\xd0\xcf\x11\xe0")},
		{ContentType: "application/octet-stream", Filename: "planimetria.dwg", Body: []byte("AC1024")},
	}
	for _, doc := range unreadable {
		parts, err := Extractor{}.Extract(doc)
		if err != nil {
			t.Errorf("Extract(%s) = %v, want no error — an unhandled format is a measurement, not a failure", doc.Filename, err)
		}
		if len(parts) != 0 {
			t.Errorf("Extract(%s) returned %d parts, want none", doc.Filename, len(parts))
		}
	}
}

// TestExtractorRefusesHTML guards against the caller error that would be worst
// if it were silent: extracting a listing page attributes the site's navigation
// menu to the tender as if it were the tender's own text.
func TestExtractorRefusesHTML(t *testing.T) {
	_, err := Extractor{}.Extract(Document{
		ContentType: "text/html; charset=utf-8",
		Filename:    "lista",
		Body:        []byte("<html><body>Elenco gare</body></html>"),
	})
	if err == nil {
		t.Fatal("Extract: want an error for an HTML document")
	}
}

// TestIsPDFTakesEitherSignal pins the dispatch. Portals routinely serve every
// attachment as application/octet-stream, and just as routinely serve a download
// endpoint with no extension in the path — so requiring both signals would find
// almost nothing.
func TestIsPDFTakesEitherSignal(t *testing.T) {
	tests := []struct {
		name string
		doc  Document
		want bool
	}{
		{"content type alone", Document{ContentType: "application/pdf", Filename: "download"}, true},
		{"content type with parameters", Document{ContentType: "application/pdf; charset=binary"}, true},
		{"extension alone", Document{ContentType: "application/octet-stream", Filename: "disciplinare.PDF"}, true},
		{"magic number alone", Document{ContentType: "application/octet-stream", Filename: "download", Body: []byte("%PDF-1.7 x")}, true},
		{"none of the three", Document{ContentType: "application/zip", Filename: "allegati.zip", Body: []byte("PK\x03\x04")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPDF(tt.doc); got != tt.want {
				t.Errorf("isPDF = %v, want %v", got, tt.want)
			}
		})
	}
}
