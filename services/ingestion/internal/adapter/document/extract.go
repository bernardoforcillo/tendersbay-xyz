package document

import (
	"context"
	"fmt"

	"github.com/tsawler/tabula"
)

// Extract reads path (a downloaded PDF file) and returns its text content
// split into chunks via tabula's built-in layout-aware chunker, using its
// default sizing (~1000 target / 2000 max characters per chunk).
func Extract(path string) ([]string, error) {
	chunks, _, err := tabula.Open(path).Chunks()
	if err != nil {
		return nil, fmt.Errorf("document: extract %s: %w", path, err)
	}
	parts := make([]string, len(chunks.Chunks))
	for i, c := range chunks.Chunks {
		parts[i] = c.Text
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
func (c *Client) FetchAndExtract(ctx context.Context, url string) ([]string, error) {
	path, cleanup, err := Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	return Extract(path)
}
