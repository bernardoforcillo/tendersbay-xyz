package knowledge

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/bernardoforcillo/drops/qdrant"
)

// Payload keys carrying a tender's structured facets on every one of its
// points. They exist so a filtered search can constrain the vector search
// itself instead of retrieving unfiltered candidates and discarding most of
// them afterwards — with a restrictive filter (one country out of 27), post-
// filtering throws away almost the entire candidate window and starves the
// result page.
const (
	fieldTenderID     = "tender_id"
	fieldTitle        = "title"
	fieldCountry      = "country"
	fieldStatus       = "status"
	fieldCPVPrefixes  = "cpv_prefixes"
	fieldNUTSPrefixes = "nuts_prefixes"
	fieldValue        = "value"
	fieldPublishedAt  = "published_at" // unix seconds
	fieldDeadline     = "deadline"     // unix seconds
)

// Attributes are the structured, filterable facets of an indexed tender,
// stamped onto every point belonging to it. Nil pointer fields mean "unknown"
// and are left out of the payload entirely, so a range filter over them
// excludes the tender — matching how a SQL predicate on a NULL column behaves.
type Attributes struct {
	Title        string
	Country      string
	Status       string
	CPV          string
	CPVSecondary []string
	NUTS         string
	Value        *int64
	PublishedAt  *time.Time
	Deadline     *time.Time
}

// prefix lengths materialised for hierarchical-code filtering. Qdrant has no
// prefix operator on keyword payloads, so instead of matching "cpv starts
// with 45" we store every meaningful prefix of the code and match one exactly.
//
// CPV's hierarchy is division(2) > group(3) > class(4) > category(5), then
// digits 6-8 refine further; NUTS runs country(2) > NUTS1(3) > NUTS2(4) >
// NUTS3(5).
const (
	cpvMinPrefix  = 2
	cpvMaxPrefix  = 8
	nutsMinPrefix = 2
	nutsMaxPrefix = 5
)

// Payload renders the attributes as Qdrant payload entries. Empty strings and
// nil pointers are omitted rather than written as zero values, so "unset" and
// "set to zero" stay distinguishable in a filter.
func (a Attributes) Payload() map[string]any {
	out := map[string]any{}
	if a.Title != "" {
		out[fieldTitle] = a.Title
	}
	if a.Country != "" {
		out[fieldCountry] = a.Country
	}
	if a.Status != "" {
		out[fieldStatus] = a.Status
	}
	if p := cpvPrefixes(append([]string{a.CPV}, a.CPVSecondary...)); len(p) > 0 {
		out[fieldCPVPrefixes] = p
	}
	if p := codePrefixes(a.NUTS, nutsMinPrefix, nutsMaxPrefix); len(p) > 0 {
		out[fieldNUTSPrefixes] = p
	}
	if a.Value != nil {
		out[fieldValue] = *a.Value
	}
	if a.PublishedAt != nil {
		out[fieldPublishedAt] = a.PublishedAt.Unix()
	}
	if a.Deadline != nil {
		out[fieldDeadline] = a.Deadline.Unix()
	}
	return out
}

// cpvPrefixes expands every code into its filterable prefixes, deduplicated
// across the primary and secondary codes. A CPV may arrive with the optional
// check digit ("45233120-9") — only the significant part is expanded.
func cpvPrefixes(codes []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, code := range codes {
		code, _, _ = strings.Cut(code, "-")
		for _, p := range codePrefixes(code, cpvMinPrefix, cpvMaxPrefix) {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	return out
}

// codePrefixes returns code's prefixes from min to max runes long, capped at
// the code's own length. Runes, not bytes: NUTS and CPV are ASCII in practice,
// but slicing a non-ASCII value by byte would emit invalid fragments.
func codePrefixes(code string, minLen, maxLen int) []string {
	code = strings.TrimSpace(code)
	r := []rune(code)
	if len(r) < minLen {
		return nil
	}
	if maxLen > len(r) {
		maxLen = len(r)
	}
	out := make([]string, 0, maxLen-minLen+1)
	for n := minLen; n <= maxLen; n++ {
		out = append(out, string(r[:n]))
	}
	return out
}

// SearchFilter narrows a vector search to the tenders matching these facets.
// Every field is optional; the zero value filters nothing.
//
// This is an OPTIMISATION, never the authority on correctness: whatever
// survives here is filtered again in Postgres by the caller. A constraint that
// can't be expressed exactly against the stored payload is therefore dropped
// here rather than approximated — dropping it costs some wasted candidates,
// approximating it would silently hide matching tenders.
type SearchFilter struct {
	Countries    []string
	Statuses     []string
	CPVPrefixes  []string
	NUTSPrefixes []string
	ValueMin     *int64
	ValueMax     *int64
	DeadlineFrom *time.Time
	DeadlineTo   *time.Time
}

// IsZero reports whether the filter constrains nothing.
func (f SearchFilter) IsZero() bool {
	return len(f.Countries) == 0 && len(f.Statuses) == 0 &&
		len(f.CPVPrefixes) == 0 && len(f.NUTSPrefixes) == 0 &&
		f.ValueMin == nil && f.ValueMax == nil &&
		f.DeadlineFrom == nil && f.DeadlineTo == nil
}

// qdrantFilter renders the filter as a Qdrant predicate, or nil when nothing
// is expressible. Multiple values for one facet are an OR (In), separate
// facets are an AND (Must) — the same semantics the SQL side applies.
func (f SearchFilter) qdrantFilter() *qdrant.Filter {
	var conds []qdrant.Condition
	if vals := anySlice(f.Countries); len(vals) > 0 {
		conds = append(conds, qdrant.In(fieldCountry, vals...))
	}
	if vals := anySlice(f.Statuses); len(vals) > 0 {
		conds = append(conds, qdrant.In(fieldStatus, vals...))
	}
	// Only prefixes whose length was actually materialised at index time can
	// be matched; anything else is left to the Postgres pass.
	if vals := anySlice(filterLen(f.CPVPrefixes, cpvMinPrefix, cpvMaxPrefix)); len(vals) > 0 {
		conds = append(conds, qdrant.In(fieldCPVPrefixes, vals...))
	}
	if vals := anySlice(filterLen(f.NUTSPrefixes, nutsMinPrefix, nutsMaxPrefix)); len(vals) > 0 {
		conds = append(conds, qdrant.In(fieldNUTSPrefixes, vals...))
	}
	if f.ValueMin != nil || f.ValueMax != nil {
		opts := qdrant.RangeOpts{}
		if f.ValueMin != nil {
			opts.Gte = qdrant.F(float64(*f.ValueMin))
		}
		if f.ValueMax != nil {
			opts.Lte = qdrant.F(float64(*f.ValueMax))
		}
		conds = append(conds, qdrant.Range(fieldValue, opts))
	}
	if f.DeadlineFrom != nil || f.DeadlineTo != nil {
		opts := qdrant.RangeOpts{}
		if f.DeadlineFrom != nil {
			opts.Gte = qdrant.F(float64(f.DeadlineFrom.Unix()))
		}
		if f.DeadlineTo != nil {
			opts.Lte = qdrant.F(float64(f.DeadlineTo.Unix()))
		}
		conds = append(conds, qdrant.Range(fieldDeadline, opts))
	}
	if len(conds) == 0 {
		return nil
	}
	return qdrant.Must(conds...)
}

// filterLen keeps only the values whose rune length falls in [minLen, maxLen]
// — i.e. the ones actually present in the stored prefix arrays.
func filterLen(vals []string, minLen, maxLen int) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if n := len([]rune(v)); n >= minLen && n <= maxLen {
			out = append(out, v)
		}
	}
	return out
}

// anySlice widens a non-empty []string for the variadic filter constructors,
// skipping empty entries so a blank UI selection never becomes a `= ”` test.
func anySlice(vals []string) []any {
	out := make([]any, 0, len(vals))
	for _, v := range vals {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// payloadIndexFields are the payload keys Qdrant needs a field index on for
// filtering to stay fast as the collection grows. Without one, a filtered
// search degrades to a full scan of the payload.
var payloadIndexFields = map[string]string{
	fieldTenderID:     "keyword",
	fieldCountry:      "keyword",
	fieldStatus:       "keyword",
	fieldCPVPrefixes:  "keyword",
	fieldNUTSPrefixes: "keyword",
	fieldValue:        "integer",
	fieldPublishedAt:  "integer",
	fieldDeadline:     "integer",
}

// ensurePayloadIndexes creates the payload field indexes if they're missing.
// Qdrant treats a repeat create as a no-op, so this is safe on every boot.
//
// Best-effort on purpose: a missing payload index makes filtered search slower,
// not wrong, and that is not worth failing service startup over — both
// services/ingestion and services/backend call this on the way up. Failures
// are logged rather than swallowed.
func (kb *KnowledgeBase) ensurePayloadIndexes(ctx context.Context) {
	for field, schema := range payloadIndexFields {
		body := map[string]any{"field_name": field, "field_schema": schema}
		path := "/collections/" + kb.collection + "/index?wait=true"
		if err := kb.qdrant.Do(ctx, "PUT", path, body, nil); err != nil {
			slog.WarnContext(ctx, "could not create qdrant payload index; filtered search will be slower",
				"collection", kb.collection, "field", field, "error", err)
		}
	}
}
