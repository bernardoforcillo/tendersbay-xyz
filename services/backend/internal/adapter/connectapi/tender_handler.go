package connectapi

import (
	"context"
	"net"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	tenderv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1/tenderv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// TenderSearcher is the narrow slice of tender.Service the handler needs
// (testability precedent: WorkspaceHandler's CreditSeeder).
type TenderSearcher interface {
	Search(ctx context.Context, p tender.SearchParams) (tender.SearchOutput, error)
}

type TenderHandler struct {
	svc TenderSearcher
}

func NewTenderHandler(svc TenderSearcher) *TenderHandler {
	return &TenderHandler{svc: svc}
}

var _ tenderv1connect.TenderServiceHandler = (*TenderHandler)(nil)

// SearchTenders deliberately has NO requireUser — it serves both
// authenticated and anonymous callers, at different auth tiers (see
// tender.Config). An authenticated caller's rate limit is keyed by user ID;
// an anonymous caller's is keyed by client IP (see clientKey).
func (h *TenderHandler) SearchTenders(ctx context.Context, req *connect.Request[tenderv1.SearchTendersRequest]) (*connect.Response[tenderv1.SearchTendersResponse], error) {
	uid, authenticated := UserIDFromContext(ctx)
	rateLimitKey := uid
	if !authenticated {
		rateLimitKey = clientKey(req)
	}

	filters, err := toTenderFilters(req.Msg.Filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	out, err := h.svc.Search(ctx, tender.SearchParams{
		Query:         req.Msg.Query,
		Filters:       filters,
		Limit:         int(req.Msg.Limit),
		Offset:        int(req.Msg.Offset),
		Authenticated: authenticated,
		RateLimitKey:  rateLimitKey,
	})
	if err != nil {
		return nil, toConnectError(err)
	}

	return connect.NewResponse(&tenderv1.SearchTendersResponse{
		Results: toProtoTenderResults(out.Results),
		HasMore: out.HasMore,
	}), nil
}

// requestPeer is the subset of connect.AnyRequest clientKey needs. It's a
// narrow interface rather than connect.AnyRequest itself — connect.AnyRequest
// can only be implemented by types in the connect package (its internalOnly
// method is unexported), so a fake couldn't satisfy it in tests.
type requestPeer interface {
	Header() http.Header
	Peer() connect.Peer
}

// clientKey resolves an anonymous caller's rate-limit key: the LAST hop of
// X-Forwarded-For if present, else the host part of the RPC peer's address.
// The last hop is the one Traefik appends (it doesn't strip client-sent XFF,
// it appends the true peer) — see the ipStrategy.depth: 1 (rightmost hop)
// convention on the `rate-limit` middleware in
// infrastructure/kubernetes/tendersbay-xyz/commons.yaml. Earlier hops are
// client-controlled and must not be trusted: trusting them lets a caller
// rotate its own key to dodge the anonymous rate limit, or forge a victim's
// IP to burn their bucket.
func clientKey(req requestPeer) string {
	if xff := req.Header().Get("X-Forwarded-For"); xff != "" {
		hops := strings.Split(xff, ",")
		for i := len(hops) - 1; i >= 0; i-- {
			if hop := strings.TrimSpace(hops[i]); hop != "" {
				return hop
			}
		}
	}
	addr := req.Peer().Addr
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

func toTenderFilters(f *tenderv1.TenderFilters) (tender.Filters, error) {
	if f == nil {
		return tender.Filters{}, nil
	}
	filters := tender.Filters{Country: f.Country, CPV: f.Cpv, Status: f.Status}
	if f.DeadlineFrom != "" {
		t, err := time.Parse(time.RFC3339, f.DeadlineFrom)
		if err != nil {
			return tender.Filters{}, err
		}
		filters.DeadlineFrom = &t
	}
	if f.DeadlineTo != "" {
		t, err := time.Parse(time.RFC3339, f.DeadlineTo)
		if err != nil {
			return tender.Filters{}, err
		}
		filters.DeadlineTo = &t
	}
	return filters, nil
}

func toProtoTenderResults(results []tender.ScoredTender) []*tenderv1.TenderResult {
	out := make([]*tenderv1.TenderResult, len(results))
	for i, r := range results {
		out[i] = toProtoTenderResult(r)
	}
	return out
}

func toProtoTenderResult(r tender.ScoredTender) *tenderv1.TenderResult {
	var value int64
	if r.Value != nil {
		value = *r.Value
	}
	var publishedAt, deadline string
	if r.PublishedAt != nil {
		publishedAt = r.PublishedAt.Format(time.RFC3339)
	}
	if r.Deadline != nil {
		deadline = r.Deadline.Format(time.RFC3339)
	}
	return &tenderv1.TenderResult{
		Id:             r.ID,
		Title:          r.Title,
		BuyerName:      r.BuyerName,
		Status:         r.Status,
		ProcedureType:  r.ProcedureType,
		Country:        r.Country,
		Cpv:            r.CPV,
		Value:          value,
		Currency:       r.Currency,
		PublishedAt:    publishedAt,
		Deadline:       deadline,
		RelevanceScore: r.RelevanceScore,
		Source:         r.Source,
		SourceRef:      r.SourceRef,
	}
}
