// Package document downloads and extracts text from a tender's notice
// document (currently PDF only).
package document

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// Fetch downloads url to a temp file and returns its path. The caller must
// call the returned cleanup function (typically via defer) to remove the
// temp file once done with it.
func Fetch(ctx context.Context, url string) (path string, cleanup func(), err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", nil, fmt.Errorf("document: fetch %s: unexpected status %d", url, resp.StatusCode)
	}

	f, err := os.CreateTemp("", "ingestion-doc-*.pdf")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { _ = os.Remove(f.Name()) }

	if _, copyErr := io.Copy(f, resp.Body); copyErr != nil {
		_ = f.Close()
		cleanup()
		return "", nil, copyErr
	}
	if closeErr := f.Close(); closeErr != nil {
		cleanup()
		return "", nil, closeErr
	}
	return f.Name(), cleanup, nil
}
