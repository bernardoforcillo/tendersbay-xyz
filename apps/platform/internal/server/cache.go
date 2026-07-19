package server

import (
	"context"
	"sync"
	"time"
)

const (
	metaCacheTTL     = 60 * time.Second
	metaCacheMaxSize = 20000
	sitemapCacheTTL  = 10 * time.Minute
)

type metaEntry struct {
	meta     *tenderMeta // nil when notFound
	notFound bool
	expiry   time.Time
}

// metaCache caches GetTender results (found and not-found) by tender id.
type metaCache struct {
	mu      sync.Mutex
	entries map[string]metaEntry
}

func newMetaCache() *metaCache { return &metaCache{entries: map[string]metaEntry{}} }

// get returns (meta, notFound, hit). hit=false means "not cached / expired".
func (c *metaCache) get(id string) (*tenderMeta, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[id]
	if !ok || time.Now().After(e.expiry) {
		return nil, false, false
	}
	return e.meta, e.notFound, true
}

func (c *metaCache) put(id string, meta *tenderMeta, notFound bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= metaCacheMaxSize {
		c.entries = map[string]metaEntry{} // crude bound; short TTL makes this cheap
	}
	c.entries[id] = metaEntry{meta: meta, notFound: notFound, expiry: time.Now().Add(metaCacheTTL)}
}

// fetch returns the cached meta or fetches + caches it.
// Returns (nil, nil) for a not-found tender (cached), or an error for a backend failure
// (NOT cached — a transient backend error should be retried).
func (c *metaCache) fetch(ctx context.Context, apiURL, id string) (*tenderMeta, error) {
	if meta, notFound, hit := c.get(id); hit {
		if notFound {
			return nil, nil
		}
		return meta, nil
	}
	meta, err := fetchTenderMeta(ctx, apiURL, id)
	if err != nil {
		return nil, err // do not cache transient errors
	}
	c.put(id, meta, meta == nil)
	return meta, nil
}

// sitemapCache caches the generated tenders sitemap XML.
type sitemapCache struct {
	mu     sync.Mutex
	xml    []byte
	expiry time.Time
}

func newSitemapCache() *sitemapCache { return &sitemapCache{} }

// get returns the cached xml if fresh, else (nil, false).
func (c *sitemapCache) get() ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.xml == nil || time.Now().After(c.expiry) {
		return nil, false
	}
	return c.xml, true
}

func (c *sitemapCache) put(xml []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.xml = xml
	c.expiry = time.Now().Add(sitemapCacheTTL)
}
