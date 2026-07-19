// Package boampapi is the HTTP transport for France's BOAMP (Bulletin
// officiel des annonces des marchés publics), served through the Opendatasoft
// "records" search API (records/1.0/search over the `boamp` dataset) — verified
// live to require no authentication for read access. It knows nothing about how
// a BOAMP record maps onto a tender; see the sibling boamp package for that.
package boampapi

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

const defaultBaseURL = "https://boamp-datadila.opendatasoft.com/api/records/1.0/search/"

// dataset is the Opendatasoft dataset holding BOAMP notices.
const dataset = "boamp"

// pageSize is how many records each page requests. Opendatasoft's records/1.0
// API caps start+rows at 10000, so with maxPages this stays inside the window.
const pageSize = 100

// maxPages bounds pagination so a mis-sorted or endless feed can't spin
// forever: 100 pages x pageSize = start 10000, the API's hard ceiling.
const maxPages = 100

// maxErrorBodyBytes bounds how much of a non-2xx body is read into an error.
const maxErrorBodyBytes = 2048

// Record is one BOAMP notice, flattened from the record's `fields` object. The
// real 8-digit CPV is NOT in a flat field (fields.descripteur_code is BOAMP's
// own descripteur taxonomy, not CPV) — it lives inside the Donnees full-notice
// blob, which the boamp mapper digs out.
type Record struct {
	Idweb             string // fields.idweb — stable per-notice id
	Objet             string // fields.objet — title
	NomAcheteur       string // fields.nomacheteur — buyer name
	DateLimiteReponse string // fields.datelimitereponse — submission deadline
	DateParution      string // fields.dateparution — publication date
	Nature            string // fields.nature — e.g. APPEL_OFFRE, ATTRIBUTION
	NatureCategorise  string // fields.nature_categorise — e.g. appeloffre/standard
	Donnees           string // fields.donnees — nested EFORMS blob (JSON-in-a-string); carries real CPV

	Raw json.RawMessage // untouched record element from the `records` array
}

// Client talks to BOAMP's Opendatasoft search API.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client pointed at the real BOAMP dataset.
func New() *Client {
	return &Client{baseURL: defaultBaseURL, http: &http.Client{Timeout: 30 * time.Second}}
}

// NewWithURL returns a Client pointed at url — for tests.
func NewWithURL(url string) *Client {
	return &Client{baseURL: url, http: &http.Client{Timeout: 30 * time.Second}}
}

type searchResponse struct {
	NHits   int               `json:"nhits"`
	Records []json.RawMessage `json:"records"`
}

type recordEnvelope struct {
	RecordID string       `json:"recordid"`
	Fields   recordFields `json:"fields"`
}

type recordFields struct {
	Idweb             string `json:"idweb"`
	Objet             string `json:"objet"`
	NomAcheteur       string `json:"nomacheteur"`
	DateLimiteReponse string `json:"datelimitereponse"`
	DateParution      string `json:"dateparution"`
	Nature            string `json:"nature"`
	NatureCategorise  string `json:"nature_categorise"`
	Donnees           string `json:"donnees"`
}

// FetchSince returns BOAMP records published at or after since. It sorts newest
// first (sort=-dateparution) and pages until a record predates since (every
// later one is older), a short/empty page ends the results, or the page cap is
// hit. Records with an unparseable publication date are kept (never dropped)
// and don't trigger the cutoff.
func (c *Client) FetchSince(ctx context.Context, since time.Time) ([]Record, error) {
	var records []Record
	start := 0
	pages := 0
	for {
		pages++
		if pages > maxPages {
			return nil, fmt.Errorf("boampapi: exceeded %d pages without reaching the since cutoff — possible pagination loop", maxPages)
		}
		resp, err := c.do(ctx, start)
		if err != nil {
			return nil, err
		}

		stop := false
		for _, raw := range resp.Records {
			var env recordEnvelope
			if err := json.Unmarshal(raw, &env); err != nil {
				return nil, fmt.Errorf("boampapi: decode record: %w", err)
			}
			if pub, ok := parseDate(env.Fields.DateParution); ok && pub.Before(since) {
				stop = true
				break
			}
			records = append(records, Record{
				Idweb:             env.Fields.Idweb,
				Objet:             env.Fields.Objet,
				NomAcheteur:       env.Fields.NomAcheteur,
				DateLimiteReponse: env.Fields.DateLimiteReponse,
				DateParution:      env.Fields.DateParution,
				Nature:            env.Fields.Nature,
				NatureCategorise:  env.Fields.NatureCategorise,
				Donnees:           env.Fields.Donnees,
				Raw:               raw,
			})
		}

		start += len(resp.Records)
		// A short or empty page means the feed is exhausted; a stop means we
		// crossed the since cutoff. Either way, we're done.
		if stop || len(resp.Records) < pageSize || len(resp.Records) == 0 {
			break
		}
	}
	return records, nil
}

func (c *Client) do(ctx context.Context, start int) (*searchResponse, error) {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("dataset", dataset)
	q.Set("rows", strconv.Itoa(pageSize))
	q.Set("start", strconv.Itoa(start))
	q.Set("sort", "-dateparution")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("boampapi: unexpected status %d: %s", resp.StatusCode, snippet)
	}
	var out searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("boampapi: decode response: %w", err)
	}
	return &out, nil
}

// parseDate parses BOAMP's dateparution, which the feed emits as a bare date
// ("2026-07-19"). It also tolerates a full RFC3339 timestamp defensively.
func parseDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
