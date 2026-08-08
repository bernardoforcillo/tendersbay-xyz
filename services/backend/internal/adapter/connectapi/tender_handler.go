package connectapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"

	tenderv1 "github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/gen/tender/v1/tenderv1connect"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/clientprofile"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/document"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/workspace"
)

// MemberRepository is the minimal membership-check port TenderHandler
// needs — satisfied by *postgres.MemberRepo unchanged, the same concrete
// type WorkspaceHandler and AgentHandler already depend on via their own
// narrow ports. Added by Task A-annotate (an amendment task); SearchTenders
// itself does not call it — its per-client fit annotation trusts
// AnnotateForClient's own internal membership check instead (see
// SearchTenders' doc comment), so this port is currently unused by that RPC.
// Task 9's RecommendTendersForClient (not yet implemented as of this task)
// won't need it either: RecommendForClient is membership-checked the same
// way AnnotateForClient is — via ProfileSource.Get itself (see both
// methods' doc comments in core/tender/recommend.go) — so it has no more
// need for a redundant handler-level LoadMembership call than
// AnnotateForClient does. This port is kept on TenderHandler for now in
// case a future handler needs a membership check the service layer doesn't
// already provide — none of the current or currently-planned uses require
// it.
type MemberRepository interface {
	LoadMembership(ctx context.Context, workspaceID, userID string) (workspace.Membership, error)
}

// DocumentReader is the minimal document port GetTenderPassages needs,
// satisfied by *document.Service unchanged.
//
// One method, not two: core/document answers availability through the SAME call
// when the question is blank (see document.Service.Excerpts), and that pairing is
// deliberate there — it is what stops a caller ever obtaining passages without
// the coverage that qualifies them. Declaring a separate Availability method
// here would hand this handler exactly the seam that pairing exists to close.
type DocumentReader interface {
	Excerpts(ctx context.Context, q document.ExcerptQuery) (document.ExcerptResult, error)
}

// TenderHandler serves TenderService. Unlike every other handler in this
// package, SearchTenders works for unauthenticated callers by design —
// see UserIDFromContext below, used directly instead of requireUser.
type TenderHandler struct {
	svc     *tender.Service
	members MemberRepository
	docs    DocumentReader
}

var _ tenderv1connect.TenderServiceHandler = (*TenderHandler)(nil)

// NewTenderHandler builds a TenderHandler.
func NewTenderHandler(svc *tender.Service, members MemberRepository, docs DocumentReader) *TenderHandler {
	return &TenderHandler{svc: svc, members: members, docs: docs}
}

func (h *TenderHandler) SearchTenders(ctx context.Context, req *connect.Request[tenderv1.SearchTendersRequest]) (*connect.Response[tenderv1.SearchTendersResponse], error) {
	userID, authed := UserIDFromContext(ctx)
	rateLimitKey := userID
	if !authed {
		rateLimitKey = ClientIPFromContext(ctx)
	}

	filters, err := filtersFromProto(req.Msg.Filters)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	out, err := h.svc.Search(ctx, tender.SearchParams{
		Query:   req.Msg.Query,
		Filters: filters,
		// This query is typed by a person into a search box, so the
		// constraints buried in it are meant as constraints.
		ParseQuery: true,
		Sort:       tender.ParseSortOrder(req.Msg.Sort),
		Limit:      int(req.Msg.Limit),
		// SuppressedCpv is honoured now, unlike req.Msg.Language (a forward
		// declaration Phase 2 wires up — deliberately not read here yet):
		// Task 18 needs removing an inferred CPV chip to actually stick on the
		// next request, since the server resolves the same code from the same
		// text every time otherwise.
		SuppressedCPV: req.Msg.SuppressedCpv,
		Offset:        int(req.Msg.Offset),
		Authenticated: authed,
		RateLimitKey:  rateLimitKey,
	})
	if err != nil {
		return nil, toConnectError(err)
	}

	results := make([]*tenderv1.TenderResult, len(out.Results))
	for i, t := range out.Results {
		results[i] = h.tenderResultToProtoWithThreshold(t)
	}

	// workspace_id == "" is today's anonymous-safe behavior, left
	// completely unchanged: no membership check, no AnnotateForClient call.
	//
	// When workspace_id is set, this trusts AnnotateForClient's own
	// membership check (it calls ProfileSource.Get, which is
	// membership-checked by clientprofile.Service.Get → requireMember) and
	// does not re-check membership itself here — the same trust-the-callee
	// shape agent.Service.ChatStream uses for its analogous case, to avoid a
	// redundant LoadMembership round trip on this path. toConnectError maps
	// the resulting workspace.ErrNotMember to PermissionDenied the same way
	// regardless of which call site produced it.
	if req.Msg.WorkspaceId != "" {
		recs, err := h.svc.AnnotateForClient(ctx, userID, req.Msg.WorkspaceId, out.Results)
		if err != nil {
			return nil, toConnectError(err)
		}
		// AnnotateForClient preserves input order, so recs[i] always
		// corresponds to results[i] built from the same out.Results above.
		for i, r := range recs {
			if r.Tier == "" {
				// ErrProfileNotFound passthrough — leave fit_tier/reason unset.
				continue
			}
			results[i].FitTier = string(r.Tier)
			results[i].Reason = reasonSignalsToProto(r.Reason)
		}
	}

	return connect.NewResponse(&tenderv1.SearchTendersResponse{
		Results:           results,
		HasMore:           out.HasMore,
		RetrievalMode:     string(out.Mode),
		Degraded:          out.Mode.Degraded(),
		AppliedFilters:    filtersToProto(out.AppliedFilters),
		AppliedQuery:      out.AppliedQuery,
		AppliedCpv:        cpvMatchesToProto(out.AppliedCPV),
		CountryFacets:     facetsToProto(out.Facets.Countries),
		StatusFacets:      facetsToProto(out.Facets.Statuses),
		CpvDivisionFacets: facetsToProto(out.Facets.CPVDivisions),
	}), nil
}

// facetsToProto maps facet counts onto the wire type.
func facetsToProto(counts []tender.FacetCount) []*tenderv1.FacetCount {
	out := make([]*tenderv1.FacetCount, len(counts))
	for i, c := range counts {
		out[i] = &tenderv1.FacetCount{Value: c.Value, Count: int32(c.Count)}
	}
	return out
}

// cpvMatchesToProto maps the inferred CPV codes for the wire.
//
// Always a non-nil slice: the field reaches JSON, where nil and [] read
// differently to a client, and the house rule on those paths is
// empty-slice-not-nil (see tender.page).
func cpvMatchesToProto(matches []tender.CPVMatch) []*tenderv1.CpvMatch {
	out := make([]*tenderv1.CpvMatch, len(matches))
	for i, m := range matches {
		out[i] = &tenderv1.CpvMatch{
			Code:     m.Code,
			Label:    m.Label,
			Language: m.Lang,
			Score:    m.Score,
		}
	}
	return out
}

// filtersToProto reports back the filters the search actually ran with,
// including any lifted out of the query text. Only the plural fields are
// filled: the singular ones are deprecated inputs, and echoing a value into
// both shapes would leave the client guessing which one is authoritative.
func filtersToProto(f tender.Filters) *tenderv1.TenderFilters {
	out := &tenderv1.TenderFilters{
		Countries:    f.Countries,
		CpvPrefixes:  f.CPVPrefixes,
		Statuses:     f.Statuses,
		NutsPrefixes: f.NUTSPrefixes,
		Buyer:        f.Buyer,
		ValueMin:     f.ValueMin,
		ValueMax:     f.ValueMax,
	}
	if f.DeadlineFrom != nil {
		out.DeadlineFrom = f.DeadlineFrom.Format(time.RFC3339)
	}
	if f.DeadlineTo != nil {
		out.DeadlineTo = f.DeadlineTo.Format(time.RFC3339)
	}
	return out
}

// RecommendTendersForClient requires auth like every other handler in this
// package except SearchTenders — no handler-level membership check is added
// here: h.svc.RecommendForClient is already membership-checked internally
// via ProfileSource.Get → clientprofile.Service.Get → requireMember, the
// same trust-the-callee shape SearchTenders' workspace_id branch uses for
// AnnotateForClient (see its doc comment). A redundant h.members.
// LoadMembership call here would duplicate that check for no benefit.
func (h *TenderHandler) RecommendTendersForClient(ctx context.Context, req *connect.Request[tenderv1.RecommendTendersForClientRequest]) (*connect.Response[tenderv1.RecommendTendersForClientResponse], error) {
	uid, err := requireUser(ctx)
	if err != nil {
		return nil, err
	}
	recs, err := h.svc.RecommendForClient(ctx, uid, req.Msg.WorkspaceId, int(req.Msg.Limit))
	if errors.Is(err, clientprofile.ErrProfileNotFound) {
		return connect.NewResponse(&tenderv1.RecommendTendersForClientResponse{NeedsProfile: true}), nil
	}
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*tenderv1.RecommendedTenderResult, len(recs))
	for i, r := range recs {
		out[i] = h.recommendedTenderToProto(r)
	}
	return connect.NewResponse(&tenderv1.RecommendTendersForClientResponse{Results: out}), nil
}

// GetCoverage is anonymous-safe like SearchTenders — no auth, no membership.
// It reports which countries we currently hold tenders for so the landing
// coverage marquee can light real flags.
func (h *TenderHandler) GetCoverage(ctx context.Context, _ *connect.Request[tenderv1.GetCoverageRequest]) (*connect.Response[tenderv1.GetCoverageResponse], error) {
	countries, err := h.svc.Coverage(ctx)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&tenderv1.GetCoverageResponse{Countries: countries}), nil
}

// filtersFromProto maps the wire filters onto the domain's.
//
// The deprecated singular fields are merged into their plural counterparts
// rather than ignored: they are a live wire contract, and a client still
// sending country="IT" must keep getting Italian tenders. Since values within
// one facet are OR-ed, appending is the correct merge — never a replacement.
func filtersFromProto(f *tenderv1.TenderFilters) (tender.Filters, error) {
	if f == nil {
		return tender.Filters{}, nil
	}
	out := tender.Filters{
		Countries:    append(tender.SingleFilter(f.Country), f.Countries...),
		CPVPrefixes:  append(tender.SingleFilter(f.Cpv), f.CpvPrefixes...),
		Statuses:     append(tender.SingleFilter(f.Status), f.Statuses...),
		NUTSPrefixes: f.NutsPrefixes,
		Buyer:        f.Buyer,
		ValueMin:     f.ValueMin,
		ValueMax:     f.ValueMax,
	}
	if f.DeadlineFrom != "" {
		t, err := time.Parse(time.RFC3339, f.DeadlineFrom)
		if err != nil {
			return tender.Filters{}, err
		}
		out.DeadlineFrom = &t
	}
	if f.DeadlineTo != "" {
		t, err := time.Parse(time.RFC3339, f.DeadlineTo)
		if err != nil {
			return tender.Filters{}, err
		}
		out.DeadlineTo = &t
	}
	return out, nil
}

// tenderResultToProto converts the shared, EU-threshold-independent result
// fields — reused by every caller in this package that needs a
// *tenderv1.TenderResult, including agent_handler.go's chat tender_results
// event (which has no *tender.Service to compute a threshold band from, and
// doesn't need one — TenderResultCard already renders no badge at all when
// EuThreshold is empty). Callers that DO have the band (every TenderHandler
// method) stamp it via tenderResultToProtoWithThreshold instead.
func tenderResultToProto(t tender.ScoredTender) *tenderv1.TenderResult {
	var value int64
	if t.Value != nil {
		value = *t.Value
	}
	var publishedAt, deadline string
	if t.PublishedAt != nil {
		publishedAt = t.PublishedAt.Format(time.RFC3339)
	}
	if t.Deadline != nil {
		deadline = t.Deadline.Format(time.RFC3339)
	}
	return &tenderv1.TenderResult{
		Id: t.ID, Title: t.Title, BuyerName: t.BuyerName, Status: t.Status,
		ProcedureType: t.ProcedureType, Country: t.Country, Cpv: t.CPV,
		Value: value, Currency: t.Currency, PublishedAt: publishedAt, Deadline: deadline,
		RelevanceScore: t.RelevanceScore, Source: t.Source, SourceRef: t.SourceRef,
		SourceUrl: t.SourceURL, Snippet: t.Snippet,
	}
}

// tenderResultToProtoWithThreshold is a method so it can reach h.svc for the
// eu_threshold band — every TenderHandler result path (SearchTenders,
// GetRelatedTenders, and RecommendTendersForClient via
// recommendedTenderToProto) routes through it, so the coarse below/above-EU-
// threshold band is stamped on all of them.
func (h *TenderHandler) tenderResultToProtoWithThreshold(t tender.ScoredTender) *tenderv1.TenderResult {
	p := tenderResultToProto(t)
	p.EuThreshold = h.svc.EUThresholdBand(t.Value, t.CPV)
	return p
}

// reasonSignalsToProto maps tender.ReasonSignals onto the wire type. Only
// called from SearchTenders' workspace_id-set branch, once a result has
// actually been annotated (see AnnotateForClient's empty-Tier passthrough
// contract) — so DeadlineDays/HasDeadline follow ReasonSignals.DeadlineDays'
// own nil-means-"no deadline or already closed" convention (see
// tender.deadlineDays), not a fresh assumption made here.
func reasonSignalsToProto(r tender.ReasonSignals) *tenderv1.ReasonSignals {
	out := &tenderv1.ReasonSignals{
		SectorMatch:    r.SectorMatch,
		CountryMatch:   r.CountryMatch,
		ValueFit:       r.ValueFit,
		RegionMatch:    r.RegionMatch,
		ProcedureMatch: r.ProcedureMatch,
	}
	if r.DeadlineDays != nil {
		out.DeadlineDays = int32(*r.DeadlineDays)
		out.HasDeadline = true
	}
	return out
}

// recommendedTenderToProto maps one tender.RecommendedTender (Task 8) onto
// the wire RecommendedTenderResult, reusing reasonSignalsToProto above for
// the Reason mapping so all six tender.ReasonSignals fields — including
// RegionMatch/ProcedureMatch — stay in sync across both RPCs that emit
// ReasonSignals (SearchTenders' annotation branch and this one), rather than
// two independently-maintained copies of the same mapping.
func (h *TenderHandler) recommendedTenderToProto(r tender.RecommendedTender) *tenderv1.RecommendedTenderResult {
	return &tenderv1.RecommendedTenderResult{
		Tender:  h.tenderResultToProtoWithThreshold(r.ScoredTender),
		FitTier: string(r.Tier),
		Reason:  reasonSignalsToProto(r.Reason),
	}
}

func (h *TenderHandler) GetTender(ctx context.Context, req *connect.Request[tenderv1.GetTenderRequest]) (*connect.Response[tenderv1.GetTenderResponse], error) {
	detail, err := h.svc.GetTender(ctx, tender.GetTenderParams{
		ID:           req.Msg.Id,
		RateLimitKey: rateLimitKey(ctx),
	})
	if err != nil {
		if errors.Is(err, tender.ErrTenderNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&tenderv1.GetTenderResponse{Tender: tenderDetailToProto(detail)}), nil
}

func (h *TenderHandler) GetRelatedTenders(ctx context.Context, req *connect.Request[tenderv1.GetRelatedTendersRequest]) (*connect.Response[tenderv1.GetRelatedTendersResponse], error) {
	out, err := h.svc.GetRelatedTenders(ctx, tender.RelatedParams{
		ID:           req.Msg.Id,
		Limit:        int(req.Msg.Limit),
		RateLimitKey: rateLimitKey(ctx),
	})
	if err != nil {
		return nil, toConnectError(err)
	}
	results := make([]*tenderv1.TenderResult, len(out))
	for i, t := range out {
		results[i] = h.tenderResultToProtoWithThreshold(t)
	}
	return connect.NewResponse(&tenderv1.GetRelatedTendersResponse{Results: results}), nil
}

func (h *TenderHandler) ListTenderSitemap(ctx context.Context, req *connect.Request[tenderv1.ListTenderSitemapRequest]) (*connect.Response[tenderv1.ListTenderSitemapResponse], error) {
	refs, err := h.svc.ListTenderSitemap(ctx, int(req.Msg.Limit))
	if err != nil {
		return nil, toConnectError(err)
	}
	out := make([]*tenderv1.TenderRef, len(refs))
	for i, r := range refs {
		out[i] = &tenderv1.TenderRef{Id: r.ID, Lastmod: r.Lastmod}
	}
	return connect.NewResponse(&tenderv1.ListTenderSitemapResponse{Refs: out}), nil
}

// GetTenderPassages answers one question against one tender's extracted text,
// always alongside what we hold of that tender at all.
//
// Authenticated, unlike the GetTender it sits beside. Retrieval runs a full-text
// query per call, which is a materially more expensive read than serving an
// already-materialised notice, and the surface that consumes it — the scheda
// gara — is behind auth anyway. The public tender page renders its coverage
// strip only when a route hands it one, and the public route deliberately does
// not.
//
// The retrieval bound is NOT enforced here: core/document clamps both the
// passage count and each passage's length itself, precisely so that adding a
// second transport cannot widen a budget the transport does not own — so
// req.Msg.Limit is passed through as the hint it is documented to be.
func (h *TenderHandler) GetTenderPassages(ctx context.Context, req *connect.Request[tenderv1.GetTenderPassagesRequest]) (*connect.Response[tenderv1.GetTenderPassagesResponse], error) {
	if _, err := requireUser(ctx); err != nil {
		return nil, err
	}
	id, err := strconv.ParseInt(req.Msg.Id, 10, 64)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("tender id must be an integer: %w", err))
	}
	out, err := h.docs.Excerpts(ctx, document.ExcerptQuery{
		TenderID: id,
		Question: req.Msg.Question,
		Limit:    int(req.Msg.Limit),
	})
	if err != nil {
		return nil, toConnectError(err)
	}
	passages := make([]*tenderv1.Passage, len(out.Excerpts))
	for i, e := range out.Excerpts {
		passages[i] = &tenderv1.Passage{
			Text:           e.Text,
			Truncated:      e.Truncated,
			RelevanceScore: e.RelevanceScore,
			Citation:       citationToProto(e.Citation),
		}
	}
	return connect.NewResponse(&tenderv1.GetTenderPassagesResponse{
		Availability: availabilityToProto(out.Availability),
		Passages:     passages,
	}), nil
}

// availabilityToProto carries the evidence counts across, not just the verdict:
// core/document.Availability.Consistent is only checkable because they travel
// with the answer, and an availability that has forgotten why it says what it
// says cannot be audited by anything except the code that built it.
func availabilityToProto(a document.Availability) *tenderv1.DocumentAvailability {
	return &tenderv1.DocumentAvailability{
		Coverage:           string(a.Coverage),
		Reason:             string(a.Reason),
		NoticeRead:         a.NoticeRead,
		KnownDocumentLinks: int32(a.KnownDocumentLinks),
		ExtractedDocuments: int32(a.ExtractedDocuments),
		ExtractedParts:     int32(a.ExtractedParts),
	}
}

// citationToProto maps provenance onto the wire, keeping the page bounds'
// present/absent distinction as explicit *_set flags — a nil PageStart and a
// page zero are both 0 on the wire, and only one of them may be rendered.
func citationToProto(c document.Citation) *tenderv1.DocumentCitation {
	out := &tenderv1.DocumentCitation{
		DocumentUrl:  c.DocumentURL,
		DocumentType: c.DocumentType,
		PartIndex:    int32(c.PartIndex),
		SectionPath:  c.SectionPath,
		SectionTitle: c.SectionTitle,
		HasTable:     c.HasTable,
	}
	if c.PageStart != nil {
		out.PageStart = int32(*c.PageStart)
		out.PageStartSet = true
	}
	if c.PageEnd != nil {
		out.PageEnd = int32(*c.PageEnd)
		out.PageEndSet = true
	}
	return out
}

// rateLimitKey is the user ID for authenticated callers, else the client IP —
// the same rule SearchTenders uses.
func rateLimitKey(ctx context.Context) string {
	if userID, authed := UserIDFromContext(ctx); authed {
		return userID
	}
	return ClientIPFromContext(ctx)
}

func tenderDetailToProto(d tender.TenderDetail) *tenderv1.TenderDetail {
	var value int64
	if d.Value != nil {
		value = *d.Value
	}
	var publishedAt, deadline string
	if d.PublishedAt != nil {
		publishedAt = d.PublishedAt.Format(time.RFC3339)
	}
	if d.Deadline != nil {
		deadline = d.Deadline.Format(time.RFC3339)
	}
	docs := make([]*tenderv1.TenderDocument, len(d.Documents))
	for i, doc := range d.Documents {
		docs[i] = &tenderv1.TenderDocument{Url: doc.URL, Type: doc.Type}
	}
	lots := make([]*tenderv1.TenderLot, len(d.Lots))
	for i, lot := range d.Lots {
		lots[i] = tenderLotToProto(lot)
	}
	criteria := make([]*tenderv1.AwardCriterion, len(d.Criteria))
	for i, c := range d.Criteria {
		criteria[i] = awardCriterionToProto(c)
	}
	out := &tenderv1.TenderDetail{
		Id: d.ID, Title: d.Title, BuyerName: d.BuyerName, BuyerId: d.BuyerID, Status: d.Status,
		ProcedureType: d.ProcedureType, Country: d.Country, Nuts: d.NUTS, Language: d.Language,
		Cpv: d.CPV, CpvSecondary: d.CPVSecondary, Value: value, Currency: d.Currency,
		PublishedAt: publishedAt, Deadline: deadline, Source: d.Source, SourceRef: d.SourceRef,
		SourceUrl: d.SourceURL, Documents: docs, Lots: lots,
		Criteria: criteria, DocumentsUrl: d.DocumentsURL,
	}
	// Both three-valued fields keep "never enriched" expressible. Coalescing
	// either to its zero would report ignorance as a finding: a grid_usable
	// false means "this notice publishes no usable grid", which is a claim about
	// the buyer, and it is indefensible for a notice nobody has read.
	if d.GridUsable != nil {
		out.GridUsable = *d.GridUsable
		out.GridUsableSet = true
	}
	if d.EnrichedAt != nil {
		out.EnrichedAt = d.EnrichedAt.Format(time.RFC3339)
	}
	return out
}

// awardCriterionToProto maps one published criterion. Weight travels with its
// own presence flag for the same reason core/tender models it as a pointer: a
// zero weight and an absent weight are different published facts, and a notice
// naming an award method with nothing attached to it is the COMMON case.
func awardCriterionToProto(c tender.AwardCriterion) *tenderv1.AwardCriterion {
	out := &tenderv1.AwardCriterion{
		LotRef:      c.LotRef,
		Ordinal:     int32(c.Ordinal),
		Type:        c.Type,
		Name:        c.Name,
		Description: c.Description,
		WeightRaw:   c.WeightRaw,
		Lang:        c.Lang,
	}
	if c.Weight != nil {
		out.Weight = *c.Weight
		out.WeightSet = true
	}
	return out
}

func tenderLotToProto(l tender.Lot) *tenderv1.TenderLot {
	var value int64
	if l.Value != nil {
		value = *l.Value
	}
	var deadline string
	if l.Deadline != nil {
		deadline = l.Deadline.Format(time.RFC3339)
	}
	return &tenderv1.TenderLot{
		Ref: l.Ref, Title: l.Title, Cpv: l.CPV, Value: value, Currency: l.Currency, Deadline: deadline,
	}
}
