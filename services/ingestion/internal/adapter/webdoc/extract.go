package webdoc

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/document"
)

// Extractor turns one retrieved file's bytes into citable parts, over the PDF
// pipeline this service already has.
//
// It takes bytes rather than a path, and that is the storage decision made
// executable. The bytes arrive in a Document, are written to a temp file
// because tabula opens a path, are extracted, and the temp file is removed
// before this function returns — so a retrieved file exists on disk for the
// length of one extraction and nowhere else, ever. There is no object storage
// in this design, no blob column, and no presigned URL, and the reason this
// package can state that as a property rather than as a convention is that no
// caller is ever handed a path it could decide to keep.
//
// The extraction itself is document.Extract unchanged, which means retrieved
// portal documents carry the same page span, heading ancestry and has-table
// provenance as the notice PDFs already in the corpus. That is what lets an
// answer cite "disciplinare, §4.2, p. 17" instead of quoting text from nowhere,
// and it is why the retriever extracts here rather than merely recording a URL
// for the indexer to fetch later — the indexer's fetcher has no robots
// handling, no per-host pacing and no dial guard, and must never be pointed at
// a buyer portal.
type Extractor struct{}

// Extract returns the parts of doc, or none.
//
// A file this phase cannot read — .doc, .docx, .zip, .p7m — returns zero parts
// and NO error. That is the important half of the contract. Those formats are
// out of scope, their absence is a measurement rather than a per-row failure,
// and returning an error for each of them would log the same known limitation
// fifteen times per tender while making a tender of unreadable files
// indistinguishable from a tender whose portal was down. The caller counts what
// it found and what it could read; the gap between the two numbers is the
// answer to "what should we build next", and it only exists if this stays
// silent.
//
// An HTML document reaching here is a caller error, not a format gap: an HTML
// response is a page to enumerate, and extracting a listing's navigation as if
// it were tender text would attribute the site menu to the tender. It is
// reported rather than silently ignored, for that reason.
func (Extractor) Extract(doc Document) ([]tender.DocumentPart, error) {
	if doc.IsHTML() {
		return nil, fmt.Errorf("webdoc: extract: %s is HTML, which is a page to enumerate rather than a file to extract", doc.Filename)
	}
	if !isPDF(doc) {
		return nil, nil
	}

	f, err := os.CreateTemp("", "webdoc-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("webdoc: extract: create temp file: %w", err)
	}
	// Removed on every path out of this function, including the write and close
	// failures below — the bytes must not outlive the extraction even when the
	// extraction does not happen.
	defer func() { _ = os.Remove(f.Name()) }()

	if _, err := f.Write(doc.Body); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("webdoc: extract: write temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("webdoc: extract: close temp file: %w", err)
	}

	parts, err := document.Extract(f.Name())
	if err != nil {
		// The temp path is this process's own scratch name and says nothing
		// about the document, so it is replaced with the published filename —
		// which is what a human reading the log would need in order to open the
		// same file by hand.
		return nil, fmt.Errorf("webdoc: extract %s: %w", doc.Filename, err)
	}
	return parts, nil
}

// pdfExtensions and isPDF decide dispatch on the server's content type first
// and the filename second.
//
// Both are needed and neither is sufficient. Portals routinely serve every
// attachment as application/octet-stream, so the content type alone would find
// no PDFs at all; and just as routinely they serve a download endpoint with no
// extension in the path, so the filename alone would miss those. Taking either
// signal is the pragmatic read, and a false positive costs one failed tabula
// call on a file that was going to yield nothing anyway.
var pdfExtensions = map[string]bool{".pdf": true}

func isPDF(doc Document) bool {
	ct := strings.ToLower(doc.ContentType)
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if ct == "application/pdf" || ct == "application/x-pdf" {
		return true
	}
	if pdfExtensions[strings.ToLower(path.Ext(doc.Filename))] {
		return true
	}
	// The last resort is the file's own magic number, which is the only
	// statement about the format that nobody upstream can get wrong.
	return len(doc.Body) >= 5 && string(doc.Body[:5]) == "%PDF-"
}
