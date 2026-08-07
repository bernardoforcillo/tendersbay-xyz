package document

import (
	"context"
	"fmt"

	"github.com/tsawler/tabula"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// Extract reads path (a downloaded PDF file) and returns its text content
// split into chunks via tabula's built-in layout-aware chunker, using its
// default sizing (~1000 target / 2000 max characters per chunk).
//
// Each chunk keeps the provenance the chunker already computed on its way to
// deciding where the chunk ends: the page span it covers, the heading ancestry
// it sits under, and whether it holds a table. That metadata is free here and
// unrecoverable later — page geometry and heading structure exist only while
// the PDF is open, so re-deriving them for a chunk already in the database
// means re-fetching and re-extracting the whole file. Carrying it out of this
// function is what lets a retrieved answer cite "capitolato, §4.2, p. 17"
// instead of quoting text from nowhere, and an unverifiable claim about award
// requirements is worse than no claim.
//
// HasTable is worth keeping for a second, blunter reason: requirement and
// scoring grids in Italian tender documents are almost always laid out as
// tables, so it doubles as a cheap locator for the parts most likely to answer
// "where are the points won".
func Extract(path string) ([]tender.DocumentPart, error) {
	chunks, _, err := tabula.Open(path).Chunks()
	if err != nil {
		return nil, fmt.Errorf("document: extract %s: %w", path, err)
	}
	parts := make([]tender.DocumentPart, len(chunks.Chunks))
	for i, c := range chunks.Chunks {
		parts[i] = tender.DocumentPart{
			Text:         c.Text,
			PageStart:    c.Metadata.PageStart,
			PageEnd:      c.Metadata.PageEnd,
			SectionPath:  c.Metadata.SectionPath,
			SectionTitle: c.Metadata.SectionTitle,
			HasTable:     c.Metadata.HasTable,
		}
	}
	return parts, nil
}

// Client combines Fetch and Extract behind a single method, so it can be
// wired as the concrete implementation of index.Fetcher.
type Client struct{}

// NewClient returns a Client.
func NewClient() *Client { return &Client{} }

// FetchAndExtract downloads url to a temp file, extracts its text, and
// removes the temp file regardless of outcome.
func (c *Client) FetchAndExtract(ctx context.Context, url string) ([]tender.DocumentPart, error) {
	path, cleanup, err := Fetch(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("document: fetch %s: %w", url, err)
	}
	defer cleanup()
	return Extract(path)
}
