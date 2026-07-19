// Package placspapi is the HTTP transport for Spain's PLACSP
// (Plataforma de Contratación del Sector Público) open-data syndication — the
// "licitaciones" ATOM feed at contrataciondelsectorpublico.gob.es, verified to
// need no authentication for read access. Each ATOM <entry> carries its CODICE
// contract folder inline, so this client fetches the feed, pages backwards via
// <link rel="next">, and delegates each entry's folder to codice.Parse. It
// knows CODICE semantics only through that package.
package placspapi

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/codice"
)

// defaultFeedURL is PLACSP's "licitacionesPerfilesContratanteCompleto3" ATOM
// feed — the richest syndication stream (full CODICE inline per entry).
const defaultFeedURL = "https://contrataciondelsectorpublico.gob.es/sindicacion/sindicacion_643/licitacionesPerfilesContratanteCompleto3.atom"

// maxPages caps how many rel=next hops one FetchSince makes, guarding against a
// feed that never stops linking to a next page.
const maxPages = 100

// maxErrorBodyBytes bounds how much of a non-2xx body is read into an error.
const maxErrorBodyBytes = 2048

// Client fetches the PLACSP ATOM feed.
type Client struct {
	feedURL string
	http    *http.Client
}

// New returns a Client pointed at the real PLACSP syndication feed.
func New() *Client {
	return &Client{feedURL: defaultFeedURL, http: &http.Client{Timeout: 60 * time.Second}}
}

// NewWithURL returns a Client pointed at url — for tests.
func NewWithURL(url string) *Client {
	return &Client{feedURL: url, http: &http.Client{Timeout: 60 * time.Second}}
}

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	Updated string    `xml:"updated"`
	Folder  rawFolder `xml:"ContractFolderStatus"`
}

// rawFolder captures a ContractFolderStatus element's inner XML verbatim, so it
// can be re-rooted and handed to codice.Parse without the client having to know
// any CODICE field.
type rawFolder struct {
	Inner []byte `xml:",innerxml"`
}

// FetchSince returns every entry's parsed CODICE document with an <updated> at
// or after since. The PLACSP feed is a rolling, newest-first stream with no
// server-side date filter, so the cutoff is applied client-side: FetchSince
// pages backwards via rel=next until an entry predates since (or there is no
// next link). A malformed or folderless entry is skipped, never fatal to the
// batch.
func (c *Client) FetchSince(ctx context.Context, since time.Time) ([]codice.Document, error) {
	var docs []codice.Document
	next := c.feedURL
	pages := 0
	for next != "" {
		pages++
		if pages > maxPages {
			return nil, fmt.Errorf("placspapi: exceeded %d pages without exhausting rel=next — possible pagination loop", maxPages)
		}
		feed, err := c.fetch(ctx, next)
		if err != nil {
			return nil, err
		}
		reachedCutoff := false
		for _, e := range feed.Entries {
			if !entryAfter(e.Updated, since) {
				reachedCutoff = true
				continue
			}
			payload := e.payload()
			if len(payload) == 0 {
				continue
			}
			doc, perr := codice.Parse(payload)
			if perr != nil {
				slog.WarnContext(ctx, "placspapi: skipping unparseable CODICE entry", "error", perr)
				continue
			}
			docs = append(docs, doc)
		}
		if reachedCutoff {
			break
		}
		next = feed.nextLink()
	}
	return docs, nil
}

func (c *Client) fetch(ctx context.Context, url string) (atomFeed, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return atomFeed{}, err
	}
	req.Header.Set("Accept", "application/atom+xml")
	resp, err := c.http.Do(req)
	if err != nil {
		return atomFeed{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return atomFeed{}, fmt.Errorf("placspapi: unexpected status %d: %s", resp.StatusCode, snippet)
	}
	var feed atomFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return atomFeed{}, fmt.Errorf("placspapi: decode atom feed: %w", err)
	}
	return feed, nil
}

// payload re-roots the captured contract-folder inner XML into a standalone
// document for codice.Parse. The synthetic <ContractFolderStatus> root is
// matched by local name; the inner elements keep their (now-unbound) CODICE
// prefixes, which codice.Parse tolerates.
func (e atomEntry) payload() []byte {
	if len(strings.TrimSpace(string(e.Folder.Inner))) == 0 {
		return nil
	}
	return []byte("<ContractFolderStatus>" + string(e.Folder.Inner) + "</ContractFolderStatus>")
}

func (f atomFeed) nextLink() string {
	for _, l := range f.Links {
		if strings.EqualFold(strings.TrimSpace(l.Rel), "next") {
			return strings.TrimSpace(l.Href)
		}
	}
	return ""
}

// entryAfter reports whether an entry's <updated> is at or after since. An
// unparseable timestamp is kept (treated as after) rather than silently
// dropped — better to over-fetch than to lose a notice to a format surprise.
func entryAfter(updated string, since time.Time) bool {
	t, err := time.Parse(time.RFC3339, strings.TrimSpace(updated))
	if err != nil {
		return true
	}
	return !t.Before(since)
}
