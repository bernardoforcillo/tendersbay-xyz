package connectapi

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	tenderv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1/tenderv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

type fakeTenderSearcher struct {
	called    bool
	gotParams tender.SearchParams
	out       tender.SearchOutput
	err       error
}

func (f *fakeTenderSearcher) Search(_ context.Context, p tender.SearchParams) (tender.SearchOutput, error) {
	f.called = true
	f.gotParams = p
	return f.out, f.err
}

func connectErrorCode(t *testing.T, err error) connect.Code {
	t.Helper()
	var connectErr *connect.Error
	if !errors.As(err, &connectErr) {
		t.Fatalf("error = %v, want a *connect.Error", err)
	}
	return connectErr.Code()
}

func TestSearchTenders_AuthenticatedUsesUserIDAsRateLimitKey(t *testing.T) {
	fake := &fakeTenderSearcher{}
	h := NewTenderHandler(fake)
	ctx := context.WithValue(context.Background(), userIDKey, "user-123")

	_, err := h.SearchTenders(ctx, connect.NewRequest(&tenderv1.SearchTendersRequest{}))
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if !fake.gotParams.Authenticated {
		t.Error("Authenticated = false, want true")
	}
	if fake.gotParams.RateLimitKey != "user-123" {
		t.Errorf("RateLimitKey = %q, want %q", fake.gotParams.RateLimitKey, "user-123")
	}
}

func TestSearchTenders_AnonymousUsesXFFLastHopAsRateLimitKey(t *testing.T) {
	fake := &fakeTenderSearcher{}
	h := NewTenderHandler(fake)

	req := connect.NewRequest(&tenderv1.SearchTendersRequest{})
	req.Header().Set("X-Forwarded-For", "203.0.113.5, 10.0.0.1")

	_, err := h.SearchTenders(context.Background(), req)
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if fake.gotParams.Authenticated {
		t.Error("Authenticated = true, want false")
	}
	if fake.gotParams.RateLimitKey != "10.0.0.1" {
		t.Errorf("RateLimitKey = %q, want %q", fake.gotParams.RateLimitKey, "10.0.0.1")
	}
}

// TestSearchTenders_AnonymousIgnoresClientForgedFirstXFFHop is the spoof
// regression: a client that sends its own X-Forwarded-For prefix must not be
// able to pick its own rate-limit key (bypass) or a victim's (bucket
// griefing). Traefik appends the true peer as the last hop; only that hop is
// trustworthy.
func TestSearchTenders_AnonymousIgnoresClientForgedFirstXFFHop(t *testing.T) {
	fake := &fakeTenderSearcher{}
	h := NewTenderHandler(fake)

	req := connect.NewRequest(&tenderv1.SearchTendersRequest{})
	req.Header().Set("X-Forwarded-For", "6.6.6.6, 203.0.113.9")

	_, err := h.SearchTenders(context.Background(), req)
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if fake.gotParams.RateLimitKey != "203.0.113.9" {
		t.Errorf("RateLimitKey = %q, want the proxy-appended hop %q, not the client-forged first hop", fake.gotParams.RateLimitKey, "203.0.113.9")
	}
}

// TestSearchTenders_EndToEndAnonymousWithNoXFFUsesPeerAddr drives a request
// through the real handler entry point over an actual HTTP connection (not
// just the clientKey unit or a hand-built connect.Request), so connect
// populates Peer() from the genuine RemoteAddr the way it does in
// production. With no X-Forwarded-For header at all, the RateLimitKey the
// handler passes to Search must come from that peer address — this
// end-to-end path was previously untested.
func TestSearchTenders_EndToEndAnonymousWithNoXFFUsesPeerAddr(t *testing.T) {
	fake := &fakeTenderSearcher{}
	h := NewTenderHandler(fake)
	path, connectHandler := tenderv1connect.NewTenderServiceHandler(h)

	var capturedRemoteAddr string
	mux := http.NewServeMux()
	mux.Handle(path, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedRemoteAddr = r.RemoteAddr
		connectHandler.ServeHTTP(w, r)
	}))
	server := httptest.NewServer(mux)
	defer server.Close()

	client := tenderv1connect.NewTenderServiceClient(server.Client(), server.URL)
	_, err := client.SearchTenders(context.Background(), connect.NewRequest(&tenderv1.SearchTendersRequest{}))
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}

	wantHost, _, err := net.SplitHostPort(capturedRemoteAddr)
	if err != nil {
		t.Fatalf("net.SplitHostPort(%q): %v", capturedRemoteAddr, err)
	}
	if fake.gotParams.Authenticated {
		t.Error("Authenticated = true, want false")
	}
	if fake.gotParams.RateLimitKey != wantHost {
		t.Errorf("RateLimitKey = %q, want the peer address host %q", fake.gotParams.RateLimitKey, wantHost)
	}
}

func TestSearchTenders_InvalidDeadlineFromReturnsInvalidArgumentWithoutCallingSearcher(t *testing.T) {
	fake := &fakeTenderSearcher{}
	h := NewTenderHandler(fake)

	req := connect.NewRequest(&tenderv1.SearchTendersRequest{
		Filters: &tenderv1.TenderFilters{DeadlineFrom: "not-a-date"},
	})
	_, err := h.SearchTenders(context.Background(), req)
	if err == nil {
		t.Fatal("SearchTenders: want error, got nil")
	}
	if code := connectErrorCode(t, err); code != connect.CodeInvalidArgument {
		t.Errorf("code = %v, want CodeInvalidArgument", code)
	}
	if fake.called {
		t.Error("searcher was called, want it skipped on a filter parse failure")
	}
}

func TestSearchTenders_InvalidDeadlineToReturnsInvalidArgumentWithoutCallingSearcher(t *testing.T) {
	fake := &fakeTenderSearcher{}
	h := NewTenderHandler(fake)

	req := connect.NewRequest(&tenderv1.SearchTendersRequest{
		Filters: &tenderv1.TenderFilters{DeadlineTo: "not-a-date"},
	})
	_, err := h.SearchTenders(context.Background(), req)
	if code := connectErrorCode(t, err); code != connect.CodeInvalidArgument {
		t.Errorf("code = %v, want CodeInvalidArgument", code)
	}
	if fake.called {
		t.Error("searcher was called, want it skipped on a filter parse failure")
	}
}

func TestSearchTenders_RateLimitedMapsToResourceExhausted(t *testing.T) {
	fake := &fakeTenderSearcher{err: tender.ErrRateLimited}
	h := NewTenderHandler(fake)

	_, err := h.SearchTenders(context.Background(), connect.NewRequest(&tenderv1.SearchTendersRequest{}))
	if code := connectErrorCode(t, err); code != connect.CodeResourceExhausted {
		t.Errorf("code = %v, want CodeResourceExhausted", code)
	}
}

func TestSearchTenders_MapsResultsAndHasMore(t *testing.T) {
	value := int64(50000)
	published := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	fake := &fakeTenderSearcher{out: tender.SearchOutput{
		Results: []tender.ScoredTender{{
			Tender: tender.Tender{
				ID: "1", Title: "Road works", BuyerName: "City of Rome", Status: "open",
				ProcedureType: "open", Country: "ITA", CPV: "45000000", Value: &value,
				Currency: "EUR", PublishedAt: &published, Deadline: &deadline,
				Source: "TED", SourceRef: "ref-1",
			},
			RelevanceScore: 0.83,
		}},
		HasMore: true,
	}}
	h := NewTenderHandler(fake)

	resp, err := h.SearchTenders(context.Background(), connect.NewRequest(&tenderv1.SearchTendersRequest{}))
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	if !resp.Msg.HasMore {
		t.Error("HasMore = false, want true")
	}
	if len(resp.Msg.Results) != 1 {
		t.Fatalf("len(Results) = %d, want 1", len(resp.Msg.Results))
	}
	got := resp.Msg.Results[0]
	if got.Id != "1" || got.Title != "Road works" || got.BuyerName != "City of Rome" ||
		got.Status != "open" || got.ProcedureType != "open" || got.Country != "ITA" ||
		got.Cpv != "45000000" || got.Value != 50000 || got.Currency != "EUR" ||
		got.Source != "TED" || got.SourceRef != "ref-1" {
		t.Errorf("Results[0] = %+v, want the mapped fields from the core Tender", got)
	}
	if got.RelevanceScore != 0.83 {
		t.Errorf("RelevanceScore = %v, want 0.83", got.RelevanceScore)
	}
	if got.PublishedAt != published.Format(time.RFC3339) {
		t.Errorf("PublishedAt = %q, want %q", got.PublishedAt, published.Format(time.RFC3339))
	}
	if got.Deadline != deadline.Format(time.RFC3339) {
		t.Errorf("Deadline = %q, want %q", got.Deadline, deadline.Format(time.RFC3339))
	}
}

func TestSearchTenders_NilValueAndUnsetTimesMapToZeroValues(t *testing.T) {
	fake := &fakeTenderSearcher{out: tender.SearchOutput{
		Results: []tender.ScoredTender{{Tender: tender.Tender{ID: "1", Title: "T"}}},
	}}
	h := NewTenderHandler(fake)

	resp, err := h.SearchTenders(context.Background(), connect.NewRequest(&tenderv1.SearchTendersRequest{}))
	if err != nil {
		t.Fatalf("SearchTenders: %v", err)
	}
	got := resp.Msg.Results[0]
	if got.Value != 0 {
		t.Errorf("Value = %d, want 0 for a nil core Value", got.Value)
	}
	if got.PublishedAt != "" || got.Deadline != "" {
		t.Errorf("PublishedAt = %q, Deadline = %q, want both empty for unset core times", got.PublishedAt, got.Deadline)
	}
}

// ── clientKey ────────────────────────────────────────────────────────────

type fakeRequestPeer struct {
	header http.Header
	peer   connect.Peer
}

func (f fakeRequestPeer) Header() http.Header { return f.header }
func (f fakeRequestPeer) Peer() connect.Peer  { return f.peer }

func TestClientKey_PrefersXFFLastHopTrimmed(t *testing.T) {
	req := fakeRequestPeer{header: http.Header{"X-Forwarded-For": []string{" 198.51.100.7 , 10.0.0.2 "}}}
	if got := clientKey(req); got != "10.0.0.2" {
		t.Errorf("clientKey = %q, want %q", got, "10.0.0.2")
	}
}

// TestClientKey_IgnoresClientForgedFirstHop is the spoof regression: the
// first XFF entry is attacker-controlled (a client can send any prefix it
// likes), so it must never be used as the rate-limit key. Only the last hop
// — the one Traefik appends — is trustworthy.
func TestClientKey_IgnoresClientForgedFirstHop(t *testing.T) {
	req := fakeRequestPeer{header: http.Header{"X-Forwarded-For": []string{"6.6.6.6, 203.0.113.9"}}}
	if got := clientKey(req); got != "203.0.113.9" {
		t.Errorf("clientKey = %q, want the proxy-appended hop %q, not the client-forged first hop", got, "203.0.113.9")
	}
}

// TestClientKey_SkipsTrailingEmptyHop covers a trailing comma (an empty
// final segment, e.g. "203.0.113.9,") — the last non-empty hop must still
// win, not the empty string.
func TestClientKey_SkipsTrailingEmptyHop(t *testing.T) {
	req := fakeRequestPeer{header: http.Header{"X-Forwarded-For": []string{"6.6.6.6, 203.0.113.9, "}}}
	if got := clientKey(req); got != "203.0.113.9" {
		t.Errorf("clientKey = %q, want %q", got, "203.0.113.9")
	}
}

func TestClientKey_FallsBackToPeerAddrHostWhenNoXFF(t *testing.T) {
	req := fakeRequestPeer{header: http.Header{}, peer: connect.Peer{Addr: "192.0.2.10:54321"}}
	if got := clientKey(req); got != "192.0.2.10" {
		t.Errorf("clientKey = %q, want %q", got, "192.0.2.10")
	}
}

func TestClientKey_FallsBackToRawPeerAddrWhenNoPort(t *testing.T) {
	req := fakeRequestPeer{header: http.Header{}, peer: connect.Peer{Addr: "192.0.2.10"}}
	if got := clientKey(req); got != "192.0.2.10" {
		t.Errorf("clientKey = %q, want %q", got, "192.0.2.10")
	}
}
