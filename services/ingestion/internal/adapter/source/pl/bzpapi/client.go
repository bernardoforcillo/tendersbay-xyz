// Package bzpapi is the HTTP transport for Poland's Biuletyn Zamówień
// Publicznych (BZP) board search API on ezamowienia.gov.pl — verified live
// to need no OAuth for read access. It knows nothing about how a notice maps
// onto tender.Tender; see the pl/bzp package for that.
//
// The endpoint (GET .../mo-board/api/v1/Board/Search?pageNumber=N&pageSize=M)
// returns a bare JSON array of notice objects — there is no envelope and no
// server-side date filter, so FetchSince paginates by incrementing
// pageNumber and applies the `since` cutoff client-side on publicationDate.
package bzpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://ezamowienia.gov.pl/mo-board/api/v1/Board/Search"

// pageSize is how many notices to request per page.
const pageSize = 100

// maxPages bounds pagination so a board that never returns an empty page (or
// notices we can never date-order past) can't spin forever.
const maxPages = 100

// maxErrorBodyBytes bounds how much of a non-2xx body is echoed into the error.
const maxErrorBodyBytes = 2048

// Notice is one BZP board-search record. Only the fields the mapper needs are
// pulled out with tags; the untouched element is kept in Raw so downstream
// code (Spec 2) can still read isTenderAmountBelowEU, bzpNumber, etc.
type Notice struct {
	ObjectID              string          `json:"objectId"`
	NoticeType            string          `json:"noticeType"`
	OrderObject           string          `json:"orderObject"`
	CpvCode               string          `json:"cpvCode"`
	SubmittingOffersDate  string          `json:"submittingOffersDate"`
	OrganizationName      string          `json:"organizationName"`
	IsTenderAmountBelowEU bool            `json:"isTenderAmountBelowEU"`
	PublicationDate       string          `json:"publicationDate"`
	BzpNumber             string          `json:"bzpNumber"`
	PdfURL                string          `json:"pdfUrl"`
	Raw                   json.RawMessage `json:"-"`
}

// Client talks to the BZP board search API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client pointed at the real BZP board search API.
func New() *Client {
	return &Client{baseURL: defaultBaseURL, http: &http.Client{Timeout: 30 * time.Second}}
}

// NewWithURL returns a Client pointed at url — for tests.
func NewWithURL(url string) *Client {
	return &Client{baseURL: url, http: &http.Client{Timeout: 30 * time.Second}}
}

// FetchSince returns every notice published at or after since. It pages by
// incrementing pageNumber; results are newest-first, so it stops as soon as
// it either exhausts the pages (an empty page) or reaches a notice that
// predates since (every later one is older). Each returned Notice carries its
// untouched JSON element in Raw.
func (c *Client) FetchSince(ctx context.Context, since time.Time) ([]Notice, error) {
	var notices []Notice
	for page := 1; ; page++ {
		if page > maxPages {
			return nil, fmt.Errorf("bzpapi: exceeded %d pages without reaching an empty page or the since cutoff — possible pagination loop", maxPages)
		}
		batch, err := c.do(ctx, page)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}
		stop := false
		for _, raw := range batch {
			var n Notice
			if err := json.Unmarshal(raw, &n); err != nil {
				return nil, fmt.Errorf("bzpapi: decode notice: %w", err)
			}
			n.Raw = raw
			if pub, ok := parsePublicationTime(n.PublicationDate); ok && pub.Before(since) {
				stop = true
				break
			}
			notices = append(notices, n)
		}
		if stop {
			break
		}
	}
	return notices, nil
}

func (c *Client) do(ctx context.Context, page int) ([]json.RawMessage, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("bzpapi: parse base url: %w", err)
	}
	q := u.Query()
	q.Set("pageNumber", strconv.Itoa(page))
	q.Set("pageSize", strconv.Itoa(pageSize))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("bzpapi: unexpected status %d: %s", resp.StatusCode, snippet)
	}
	var batch []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&batch); err != nil {
		return nil, fmt.Errorf("bzpapi: decode response: %w", err)
	}
	return batch, nil
}

// parsePublicationTime parses BZP's publicationDate, which comes as RFC3339
// with a Z zone and up to 7 fractional digits (e.g. 2024-02-14T08:02:05.25Z).
// ok is false for an empty or unparseable value, in which case the caller
// keeps the notice rather than dropping it on a date it can't read.
func parsePublicationTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
