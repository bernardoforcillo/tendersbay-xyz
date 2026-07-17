package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"strings"
	"time"
)

// tenderMeta is the subset of a tender the head needs (Connect JSON camelCase).
type tenderMeta struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	BuyerName    string `json:"buyerName"`
	Country      string `json:"country"`
	Status       string `json:"status"`
	Deadline     string `json:"deadline"`
	CanonicalURL string `json:"-"`
}

type getTenderResponse struct {
	Tender *tenderMeta `json:"tender"`
}

const tenderHeadTimeout = 800 * time.Millisecond

func apiURLFromEnv() string { return os.Getenv("API_URL") }

// fetchTenderMeta calls the backend GetTender over Connect's JSON protocol.
// Returns (nil, nil) when not found (HTTP 404).
func fetchTenderMeta(ctx context.Context, apiURL, id string) (*tenderMeta, error) {
	if apiURL == "" {
		return nil, fmt.Errorf("API_URL unset")
	}
	body, _ := json.Marshal(map[string]string{"id": id})
	url := strings.TrimRight(apiURL, "/") + "/tender.v1.TenderService/GetTender"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend GetTender: status %d", resp.StatusCode)
	}
	var out getTenderResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Tender, nil
}

// injectTenderHead overrides the shell's <title> + description and injects
// OG/Twitter/canonical + JSON-LD at the <!--tender-head--> sentinel.
func injectTenderHead(shell []byte, m tenderMeta) []byte {
	title := m.Title + " — tendersbay"
	desc := "Tender details, deadline, buyer and related opportunities for " + m.Title + "."
	out := replaceFirstTag(shell, "<title>", "</title>", html.EscapeString(title))
	out = replaceMetaDescription(out, html.EscapeString(desc))

	ld := map[string]any{
		"@context": "https://schema.org", "@type": "GovernmentService",
		"name": m.Title, "areaServed": m.Country, "url": m.CanonicalURL,
	}
	if m.BuyerName != "" {
		ld["provider"] = map[string]any{"@type": "GovernmentOrganization", "name": m.BuyerName}
	}
	ldBytes, _ := json.Marshal(ld)

	var b strings.Builder
	fmt.Fprintf(&b, `<meta property="og:title" content="%s">`, html.EscapeString(title))
	fmt.Fprintf(&b, `<meta property="og:description" content="%s">`, html.EscapeString(desc))
	fmt.Fprintf(&b, `<meta property="og:type" content="website">`)
	if m.CanonicalURL != "" {
		fmt.Fprintf(&b, `<link rel="canonical" href="%s">`, html.EscapeString(m.CanonicalURL))
	}
	fmt.Fprintf(&b, `<meta name="twitter:card" content="summary">`)
	fmt.Fprintf(&b, `<script type="application/ld+json">%s</script>`, ldBytes)
	return bytes.Replace(out, []byte("<!--tender-head-->"), []byte(b.String()), 1)
}

func replaceFirstTag(src []byte, open, closeTag, content string) []byte {
	i := bytes.Index(src, []byte(open))
	if i < 0 {
		return src
	}
	j := bytes.Index(src[i:], []byte(closeTag))
	if j < 0 {
		return src
	}
	var out bytes.Buffer
	out.Write(src[:i])
	out.WriteString(open)
	out.WriteString(content)
	out.WriteString(closeTag)
	out.Write(src[i+j+len(closeTag):])
	return out.Bytes()
}

func replaceMetaDescription(src []byte, content string) []byte {
	marker := []byte(`<meta name="description" content="`)
	i := bytes.Index(src, marker)
	if i < 0 {
		return src
	}
	start := i + len(marker)
	end := bytes.IndexByte(src[start:], '"')
	if end < 0 {
		return src
	}
	var out bytes.Buffer
	out.Write(src[:start])
	out.WriteString(content)
	out.Write(src[start+end:])
	return out.Bytes()
}

// noindexHead injects a robots noindex at the sentinel (for not-found tenders).
func noindexHead(shell []byte) []byte {
	return bytes.Replace(shell, []byte("<!--tender-head-->"), []byte(`<meta name="robots" content="noindex">`), 1)
}

// tenderSitemapXML builds sitemap XML from the backend ListTenderSitemap RPC.
func tenderSitemapXML(ctx context.Context, apiURL, hostname string, locales []string) ([]byte, error) {
	body, _ := json.Marshal(map[string]int{"limit": 50000})
	url := strings.TrimRight(apiURL, "/") + "/tender.v1.TenderService/ListTenderSitemap"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("backend ListTenderSitemap: status %d", resp.StatusCode)
	}
	var out struct {
		Refs []struct {
			ID      string `json:"id"`
			Lastmod string `json:"lastmod"`
		} `json:"refs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	for _, r := range out.Refs {
		for _, loc := range locales {
			fmt.Fprintf(&b, "<url><loc>%s/%s/tenders/%s</loc>", hostname, loc, r.ID)
			if len(r.Lastmod) >= 10 {
				fmt.Fprintf(&b, "<lastmod>%s</lastmod>", r.Lastmod[:10])
			}
			b.WriteString("</url>\n")
		}
	}
	b.WriteString(`</urlset>`)
	return []byte(b.String()), nil
}

// tenderIDFromPath returns the id for a locale-relative "tenders/<id>" path.
func tenderIDFromPath(rest string) (string, bool) {
	after, ok := strings.CutPrefix(rest, "tenders/")
	if !ok || after == "" || strings.Contains(after, "/") {
		return "", false
	}
	return after, true
}

func serveTenderPage(w http.ResponseWriter, r *http.Request, shell []byte, locale, id string) {
	ctx, cancel := context.WithTimeout(r.Context(), tenderHeadTimeout)
	defer cancel()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	meta, err := fetchTenderMeta(ctx, apiURLFromEnv(), id)
	if err != nil {
		// Backend slow/unreachable: serve the plain SPA shell (page still works client-side).
		_, _ = w.Write(shell)
		return
	}
	if meta == nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(noindexHead(shell))
		return
	}
	meta.CanonicalURL = canonicalURL(r, locale, id)
	_, _ = w.Write(injectTenderHead(shell, *meta))
}

func canonicalURL(r *http.Request, locale, id string) string {
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") == "" {
		scheme = "http"
	}
	return scheme + "://" + r.Host + "/" + locale + "/tenders/" + id
}

func localeNames(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
