package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestMetaCache_SecondFetchHitsCacheNotBackend(t *testing.T) {
	var calls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/tender.v1.TenderService/GetTender") {
			calls.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tender":{"id":"5","title":"Road works"}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	c := newMetaCache()
	m1, err := c.fetch(context.Background(), backend.URL, "5")
	if err != nil || m1 == nil || m1.Title != "Road works" {
		t.Fatalf("first fetch: meta=%v err=%v", m1, err)
	}
	m2, err := c.fetch(context.Background(), backend.URL, "5")
	if err != nil || m2 == nil || m2.Title != "Road works" {
		t.Fatalf("second fetch: meta=%v err=%v", m2, err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("backend GetTender called %d times, want 1 (second served from cache)", got)
	}
}

func TestMetaCache_CachesNotFound(t *testing.T) {
	var calls atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer backend.Close()

	c := newMetaCache()
	for i := 0; i < 3; i++ {
		m, err := c.fetch(context.Background(), backend.URL, "999")
		if err != nil || m != nil {
			t.Fatalf("fetch %d: want (nil,nil) for not-found, got (%v,%v)", i, m, err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("backend called %d times for a not-found id, want 1 (not-found cached)", got)
	}
}

func TestSitemapCache_ServesCachedOnSecondRequest(t *testing.T) {
	c := newSitemapCache()
	if _, ok := c.get(); ok {
		t.Fatal("empty cache should miss")
	}
	c.put([]byte("<urlset/>"))
	xml, ok := c.get()
	if !ok || string(xml) != "<urlset/>" {
		t.Errorf("cache get = (%q,%v), want the stored xml", xml, ok)
	}
}
