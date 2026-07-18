package tender

import (
	"context"
	"sort"
	"strings"
	"time"
)

// FitTier is a per-client shortlist result's qualitative fit — never a
// numeric percentage (no false precision; see the design spec's honesty
// guardrail).
type FitTier string

const (
	FitStrong   FitTier = "strong"
	FitPossible FitTier = "possible"
	FitLongShot FitTier = "long_shot"
)

// ReasonSignals are the localizable FACTS behind a fit tier, not a prebuilt
// sentence — the caller (the proto handler, then the frontend) renders the
// sentence in the user's locale from these fields.
//
// RegionMatch and ProcedureMatch are tie-breakers and reason enrichment
// only: they never move computeFitTier's output, which stays gated on
// relevance/value/deadline (see computeFitTier's doc comment).
type ReasonSignals struct {
	SectorMatch    bool
	CountryMatch   bool
	RegionMatch    bool
	ProcedureMatch bool
	ValueFit       string // "in_band" | "below" | "above" | "unknown"
	DeadlineDays   *int   // nil = no deadline on the tender
}

// valueFit classifies a tender's value against a client's value band.
// Either bound may be unset (nil); a nil tender value or a fully-unset band
// both report "unknown" rather than a false "below"/"above".
func valueFit(value, min, max *int64) string {
	if value == nil || (min == nil && max == nil) {
		return "unknown"
	}
	if min != nil && *value < *min {
		return "below"
	}
	if max != nil && *value > *max {
		return "above"
	}
	return "in_band"
}

// deadlineDays returns the whole days remaining until deadline, or nil when
// there is no deadline or it has already passed (a past deadline reads the
// same as "no deadline" to the caller — neither should influence the tier).
func deadlineDays(deadline *time.Time, now time.Time) *int {
	if deadline == nil {
		return nil
	}
	d := int(deadline.Sub(now).Hours() / 24)
	if d < 0 {
		return nil
	}
	return &d
}

func matchesAnyPrefix(cpv string, prefixes []string) bool {
	for _, p := range prefixes {
		if p != "" && strings.HasPrefix(cpv, p) {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// computeReasonSignals derives the localizable facts for one tender against
// a client's profile fields (passed individually, not as a
// clientprofile.Profile, so this package stays free of a hard dependency on
// core/clientprofile — RecommendForClient in Task 8 is what bridges the
// two).
//
// SectorMatch is primary-CPV only (matchesAnyPrefix(t.CPV, sectors)). The
// original design allowed for a secondary-CPV union too
// (matchesAnyPrefix(t.CPV, sectors) || anySecondaryMatches(t.CPVSecondary,
// sectors)), but Task A0 investigated plumbing CPVSecondary into the tender
// read path and found it genuinely unreachable through this codebase's
// Postgres access layer (drops has no array-column type; pgx's
// database/sql scanning rejects the array OID for this driver setup) — so
// Tender.CPVSecondary was never added to the struct. This is the
// documented degradation applying: "if A0 shipped NUTS-only, t.CPVSecondary
// is empty and this degrades to primary-CPV — safe."
//
// RegionMatch and ProcedureMatch follow the same "empty claim ⇒ false"
// honesty rule as SectorMatch/CountryMatch: an empty regions or
// procedureTypes list is an honest "not claimed" and never counts as a
// match or a penalty.
func computeReasonSignals(t Tender, sectors, countries, regions, procedureTypes []string, valueMin, valueMax *int64, now time.Time) ReasonSignals {
	return ReasonSignals{
		SectorMatch:    matchesAnyPrefix(t.CPV, sectors),
		CountryMatch:   containsString(countries, t.Country),
		RegionMatch:    matchesAnyPrefix(t.NUTS, regions),
		ProcedureMatch: len(procedureTypes) > 0 && containsString(procedureTypes, t.ProcedureType),
		ValueFit:       valueFit(t.Value, valueMin, valueMax),
		DeadlineDays:   deadlineDays(t.Deadline, now),
	}
}

// computeFitTier is a pure, deterministic classification over one search
// result's relevance score plus its ReasonSignals:
//
//	long_shot if relevance < RelevanceLow, OR value is below/above the band,
//	          OR the deadline is inside UrgentDeadlineDays
//	strong    if relevance >= RelevanceHigh AND (no deadline OR deadline >= MinDeadlineDays)
//	possible  otherwise
//
// The long_shot check runs first, so by the time the strong check runs,
// ValueFit is already guaranteed in_band/unknown and the deadline (if any)
// is already guaranteed >= UrgentDeadlineDays.
//
// RegionMatch and ProcedureMatch are deliberately not read here — they are
// tie-breakers and reason enrichment only (per the delta amendment and the
// design spec's honesty guardrail), never inputs to the tier itself.
// TestComputeFitTierIgnoresRegionAndProcedureMatch asserts this.
func computeFitTier(relevance float64, r ReasonSignals, cfg FitThresholds) FitTier {
	badValue := r.ValueFit == "below" || r.ValueFit == "above"
	urgent := r.DeadlineDays != nil && *r.DeadlineDays < cfg.UrgentDeadlineDays
	if relevance < cfg.RelevanceLow || badValue || urgent {
		return FitLongShot
	}
	tooSoonForStrong := r.DeadlineDays != nil && *r.DeadlineDays < cfg.MinDeadlineDays
	if relevance >= cfg.RelevanceHigh && !tooSoonForStrong {
		return FitStrong
	}
	return FitPossible
}

// RecommendedTender is one shortlist entry: the scored search result plus
// its deterministic fit classification.
type RecommendedTender struct {
	ScoredTender
	Tier   FitTier
	Reason ReasonSignals
}

// defaultRecommendLimit is used when the caller passes limit <= 0.
const defaultRecommendLimit = 3

// RecommendForClient turns workspaceID's ClientProfile into a ranked
// best-fit shortlist: it loads the profile (membership-checked by
// ProfileSource.Get itself), searches on the profile's notes/sectors/
// countries, classifies every result's fit, and sorts by (tier desc,
// relevance desc, RegionMatch desc, ProcedureMatch desc). Returns
// clientprofile.ErrProfileNotFound, unwrapped, when the client has no
// profile yet — callers (the RPC handler) turn that into an honest "set up
// a profile" response rather than a hard error.
func (s *Service) RecommendForClient(ctx context.Context, userID, workspaceID string, limit int) ([]RecommendedTender, error) {
	profile, err := s.profiles.Get(ctx, userID, workspaceID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultRecommendLimit
	}

	filters := Filters{Status: "open"}
	if len(profile.Countries) > 0 {
		// profile.Countries is alpha-2 (clientprofile.Profile's Countries
		// field comment: "matches ingested_tenders.country (alpha3ToAlpha2
		// at ingestion)" — Task 1's delta). Before that, Countries was
		// alpha-3, so this filter silently matched zero rows against the
		// alpha-2 ingested_tenders.country column — a real bug, now fixed
		// by the type change alone; this line's logic did not need to
		// change.
		filters.Country = profile.Countries[0]
	}
	if len(profile.Sectors) > 0 {
		filters.CPV = profile.Sectors[0]
	}

	out, err := s.Search(ctx, SearchParams{
		Query:         profile.Notes,
		Filters:       filters,
		Limit:         limit,
		Authenticated: true,
		RateLimitKey:  userID,
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	recs := make([]RecommendedTender, len(out.Results))
	for i, t := range out.Results {
		reason := computeReasonSignals(t.Tender, profile.Sectors, profile.Countries, profile.Regions, profile.ProcedureTypes, profile.ValueMin, profile.ValueMax, now)
		recs[i] = RecommendedTender{
			ScoredTender: t,
			Tier:         computeFitTier(t.RelevanceScore, reason, s.cfg.Fit),
			Reason:       reason,
		}
	}
	sort.SliceStable(recs, func(i, j int) bool {
		if recs[i].Tier != recs[j].Tier {
			return tierRank(recs[i].Tier) > tierRank(recs[j].Tier)
		}
		if recs[i].RelevanceScore != recs[j].RelevanceScore {
			return recs[i].RelevanceScore > recs[j].RelevanceScore
		}
		if recs[i].Reason.RegionMatch != recs[j].Reason.RegionMatch {
			return recs[i].Reason.RegionMatch
		}
		if recs[i].Reason.ProcedureMatch != recs[j].Reason.ProcedureMatch {
			return recs[i].Reason.ProcedureMatch
		}
		return false
	})
	if len(recs) > limit {
		recs = recs[:limit]
	}
	return recs, nil
}

func tierRank(t FitTier) int {
	switch t {
	case FitStrong:
		return 2
	case FitPossible:
		return 1
	default:
		return 0
	}
}
