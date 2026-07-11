// Package tedapi is the HTTP transport for TED's public Search API
// (https://api.ted.europa.eu/v3/notices/search) — verified live to require
// no authentication for read access. It knows nothing about eForms notice
// semantics beyond decoding them via eforms.Decode; see eforms for that.
package tedapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/eforms"
)

const defaultBaseURL = "https://api.ted.europa.eu/v3/notices/search"

// pageLimit is the maximum notices per page the Search API allows,
// verified live.
const pageLimit = 250

// maxPages is the safety cap on pagination to guard against a stuck or
// repeating iterationNextToken. TED's documented limit is at most 15,000
// notices retrievable per query; 15,000 / 250-per-page = 60 pages, so
// 100 pages is comfortably above any legitimate response.
const maxPages = 100

// searchFields is the exact field set eforms.Notice's json tags decode —
// keep these in sync. Verified live against api.ted.europa.eu (requesting
// an unsupported field name returns a 400 listing every valid one, which
// is how each of these was confirmed).
var searchFields = []string{
	"publication-number", "procedure-identifier", "notice-type", "procedure-type",
	"notice-title", "buyer-name", "organisation-identifier-buyer", "official-language",
	"buyer-country", "classification-cpv", "estimated-value-proc", "estimated-value-cur-proc",
	"publication-date", "identifier-lot", "title-lot",
	"deadline-receipt-tender-date-lot", "deadline-receipt-tender-time-lot", "links",
}

// Client talks to TED's Search API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client pointed at the real TED Search API.
func New() *Client {
	return &Client{baseURL: defaultBaseURL, http: &http.Client{Timeout: 30 * time.Second}}
}

// NewWithURL returns a Client pointed at url — for tests.
func NewWithURL(url string) *Client {
	return &Client{baseURL: url, http: &http.Client{Timeout: 30 * time.Second}}
}

type searchRequest struct {
	Query              string   `json:"query"`
	Fields             []string `json:"fields"`
	Limit              int      `json:"limit"`
	PaginationMode     string   `json:"paginationMode"`
	IterationNextToken string   `json:"iterationNextToken,omitempty"`
}

type searchResponse struct {
	Notices            []json.RawMessage `json:"notices"`
	TotalNoticeCount    int               `json:"totalNoticeCount"`
	IterationNextToken  *string           `json:"iterationNextToken"`
}

// FetchSince returns every notice published at or after since, paging
// through TED's ITERATION mode until exhausted.
func (c *Client) FetchSince(ctx context.Context, since time.Time) ([]eforms.Notice, error) {
	query := fmt.Sprintf("publication-date>=%s", since.UTC().Format("20060102"))

	var notices []eforms.Notice
	token := ""
	pages := 0
	for {
		pages++
		if pages > maxPages {
			return nil, fmt.Errorf("tedapi: exceeded %d pages without exhausting iterationNextToken — possible pagination loop", maxPages)
		}
		resp, err := c.do(ctx, searchRequest{
			Query:              query,
			Fields:             searchFields,
			Limit:              pageLimit,
			PaginationMode:     "ITERATION",
			IterationNextToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, raw := range resp.Notices {
			n, decErr := eforms.Decode(raw)
			if decErr != nil {
				return nil, fmt.Errorf("tedapi: decode notice: %w", decErr)
			}
			notices = append(notices, n)
		}
		if resp.IterationNextToken == nil || *resp.IterationNextToken == "" {
			break
		}
		token = *resp.IterationNextToken
	}
	return notices, nil
}

func (c *Client) do(ctx context.Context, body searchRequest) (*searchResponse, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("tedapi: unexpected status %d", resp.StatusCode)
	}
	var out searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tedapi: decode response: %w", err)
	}
	return &out, nil
}
