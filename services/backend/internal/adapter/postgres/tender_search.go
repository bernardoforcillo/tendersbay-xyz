// Package postgres — this file holds the tender search queries. They are
// written as raw, parameterised SQL rather than through the drops query
// builder because lexical retrieval needs constructs the builder has no DSL
// for: websearch_to_tsquery, ts_rank_cd, trigram similarity, and prefix
// matching over the cpv_secondary array. Keeping all three read paths
// (lexical, filters-only, by-ids) in one file lets them share exactly one
// filter-clause builder, so the predicates can't drift apart between them.
package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

// tenderSelectColumns is the projection every search query returns, in the
// order scanTenderRows reads them.
//
// The notice URL comes from a LATERAL subquery rather than a plain LEFT JOIN:
// a tender with more than one document of type "notice" would duplicate its
// row under a join, silently inflating result counts and consuming candidate
// slots with copies of itself.
const tenderSelectColumns = `
	t.id, t.title, t.buyer_name, t.status, t.procedure_type, t.country, t.cpv,
	t.value, t.currency, t.published_at, t.deadline, t.source, t.source_ref,
	t.nuts, doc.url`

const tenderFromClause = `
FROM tenders.ingested_tenders t
LEFT JOIN LATERAL (
	SELECT d.url FROM tenders.ingested_tender_documents d
	WHERE d.tender_id = t.id AND d.type = 'notice'
	ORDER BY d.id
	LIMIT 1
) doc ON true`

// trigramQueryMaxLen bounds when the fuzzy arm of the lexical query is worth
// adding. Trigram similarity earns its keep on short, name-shaped input
// ("comune bergam"); on a long sentence it matches nothing useful and only
// costs a second index probe.
const trigramQueryMaxLen = 60

// argBuilder accumulates positional query arguments and hands out their
// placeholders, so filter clauses can be composed without any caller having
// to track $-numbering by hand — the classic source of off-by-one injection
// bugs in hand-built SQL.
type argBuilder struct{ args []any }

// next records v and returns its placeholder ("$1", "$2", …).
func (b *argBuilder) next(v any) string {
	b.args = append(b.args, v)
	return "$" + strconv.Itoa(len(b.args))
}

// filterClauses renders f as SQL predicates over the aliased table t. Values
// within one facet are OR-ed, facets are AND-ed by the caller joining the
// returned clauses with AND.
//
// An unset facet contributes no clause at all — it must not become a
// `= ”` test, which would exclude every row instead of none.
func filterClauses(f tender.Filters, b *argBuilder) []string {
	var out []string

	if vals := nonEmpty(f.Countries); len(vals) > 0 {
		out = append(out, "t.country IN ("+placeholders(vals, b)+")")
	}
	if vals := nonEmpty(f.Statuses); len(vals) > 0 {
		out = append(out, "t.status IN ("+placeholders(vals, b)+")")
	}
	// A CPV prefix matches the primary code or any secondary one, mirroring
	// what the vector payload indexes (knowledge.Attributes) so both retrieval
	// paths answer the same question. The prefix is escaped before the trailing
	// wildcard is appended, so a user-supplied "4%" filters for a literal
	// percent sign rather than quietly widening to all of division 4.
	if vals := nonEmpty(f.CPVPrefixes); len(vals) > 0 {
		var arms []string
		for _, v := range vals {
			p := b.next(escapeLike(v) + "%")
			arms = append(arms, "t.cpv LIKE "+p,
				"EXISTS (SELECT 1 FROM unnest(t.cpv_secondary) AS sec WHERE sec LIKE "+p+")")
		}
		out = append(out, "("+strings.Join(arms, " OR ")+")")
	}
	if vals := nonEmpty(f.NUTSPrefixes); len(vals) > 0 {
		var arms []string
		for _, v := range vals {
			arms = append(arms, "t.nuts LIKE "+b.next(escapeLike(v)+"%"))
		}
		out = append(out, "("+strings.Join(arms, " OR ")+")")
	}
	if f.Buyer != "" {
		out = append(out, "t.buyer_name ILIKE "+b.next("%"+escapeLike(f.Buyer)+"%"))
	}
	if f.ValueMin != nil {
		out = append(out, "t.value >= "+b.next(*f.ValueMin))
	}
	if f.ValueMax != nil {
		out = append(out, "t.value <= "+b.next(*f.ValueMax))
	}
	if f.DeadlineFrom != nil {
		out = append(out, "t.deadline >= "+b.next(*f.DeadlineFrom))
	}
	if f.DeadlineTo != nil {
		out = append(out, "t.deadline <= "+b.next(*f.DeadlineTo))
	}
	return out
}

// escapeLike neutralises the wildcards in a user-supplied substring, so a
// buyer name containing % or _ searches for those characters rather than
// matching everything. Callers pair it with the default backslash escape.
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// placeholders records each value and returns their comma-joined placeholders.
func placeholders(vals []string, b *argBuilder) string {
	out := make([]string, len(vals))
	for i, v := range vals {
		out[i] = b.next(v)
	}
	return strings.Join(out, ", ")
}

func nonEmpty(vals []string) []string {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// whereClause joins clauses into a WHERE, or returns "" when there are none.
func whereClause(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	return "\nWHERE " + strings.Join(clauses, "\n  AND ")
}

// LexicalSearch runs the keyword half of hybrid retrieval: a full-text match
// over the generated search_vector, ranked by ts_rank_cd, optionally widened
// by a trigram similarity arm for short misspelled input.
//
// This is the half that finds what vector search structurally cannot — an
// exact CPV code, a notice reference, a buyer's name, a rare acronym. Dense
// embeddings map those onto whatever they most resemble; a lexical index
// matches them.
//
// The 'simple' text-search configuration matches the one the generated column
// was built with in the ingestion migration; using a different one here would
// silently stop matching.
func (r *TenderRepo) LexicalSearch(ctx context.Context, q tender.LexicalQuery, filters tender.Filters, limit int) ([]tender.ScoredTender, error) {
	query := strings.TrimSpace(q.Text)
	if query == "" || limit <= 0 {
		return nil, nil
	}

	b := &argBuilder{}
	tsq := b.next(query)

	// ts_rank_cd's normalisation flag 32 divides the raw rank by rank+1,
	// mapping an unbounded score into (0,1) — comparable across queries, which
	// a raw ts_rank_cd value is not.
	rank := "ts_rank_cd(t.search_vector, websearch_to_tsquery('simple', " + tsq + "), 32)"
	match := "t.search_vector @@ websearch_to_tsquery('simple', " + tsq + ")"

	if len([]rune(query)) <= trigramQueryMaxLen {
		trg := b.next(query)
		// `%` is pg_trgm's similarity operator, backed by the GIN trigram
		// indexes on title and buyer_name.
		match = "(" + match + " OR t.title % " + trg + " OR t.buyer_name % " + trg + ")"
		rank = "GREATEST(" + rank + ", similarity(t.title, " + trg + "), similarity(t.buyer_name, " + trg + "))"
	}

	// A snippet of the matched text with the query terms marked, so a result
	// can show WHY it matched. The source text is truncated first: ts_headline
	// cost grows with document length, and no useful fragment comes from
	// beyond the first few thousand characters.
	snippet := "ts_headline('simple', left(coalesce(NULLIF(t.description, ''), t.title), 4000), " +
		"websearch_to_tsquery('simple', " + tsq + "), " +
		"'StartSel=<mark>, StopSel=</mark>, MaxWords=32, MinWords=12, MaxFragments=1, FragmentDelimiter= … ')"

	clauses := append([]string{match}, filterClauses(filters, b)...)
	sql := "SELECT" + tenderSelectColumns + ",\n\t" + rank + " AS relevance,\n\t" + snippet + " AS snippet" +
		tenderFromClause + whereClause(clauses) +
		// t.id last so the order is total: without it, two equally-ranked
		// tenders published at the same instant could swap places between
		// pages and make a result appear twice or not at all.
		"\nORDER BY relevance DESC, t.published_at DESC NULLS LAST, t.id DESC" +
		"\nLIMIT " + b.next(limit)

	return r.queryScoredTenders(ctx, "lexical search", sql, b.args)
}

// FindByCPVPrefixes returns tenders whose primary or any secondary CPV is
// prefixed by one of codes, newest first.
//
// It reuses filterClauses' CPV predicate shape rather than inventing a second
// one, so the arm and the CPVPrefixes filter can never disagree about what "a
// tender in this category" means — including the escapeLike guard, without which
// a code containing a wildcard would silently widen the match.
//
// Ordering is by recency, not by a score: the codes carry the relevance (they
// came out of the lexicon ranked), and within one code every tender is equally
// "in that category". Fusion works on RANKS, so recency is the only defensible
// tie-break here — and t.id makes the order total so paging cannot repeat a row.
func (r *TenderRepo) FindByCPVPrefixes(ctx context.Context, codes []string, filters tender.Filters, limit int) ([]tender.ScoredTender, error) {
	vals := nonEmpty(codes)
	if len(vals) == 0 || limit <= 0 {
		return nil, nil
	}

	b := &argBuilder{}
	var arms []string
	for _, v := range vals {
		p := b.next(escapeLike(v) + "%")
		arms = append(arms, "t.cpv LIKE "+p,
			"EXISTS (SELECT 1 FROM unnest(t.cpv_secondary) AS sec WHERE sec LIKE "+p+")")
	}
	clauses := append([]string{"(" + strings.Join(arms, " OR ") + ")"}, filterClauses(filters, b)...)

	sql := "SELECT" + tenderSelectColumns + ",\n\t0::float8 AS relevance,\n\t'' AS snippet" +
		tenderFromClause + whereClause(clauses) +
		"\nORDER BY t.published_at DESC NULLS LAST, t.id DESC" +
		"\nLIMIT " + b.next(limit)

	return r.queryScoredTenders(ctx, "find tenders by cpv prefixes", sql, b.args)
}

// browseOrderBy renders a sort order as SQL for the browse path.
//
// Every branch ends in t.id so the order is total: without a final
// tie-break, rows that compare equal can be returned in a different order on
// each page, making a tender appear twice or not at all as the user pages.
// NULLs sort last throughout — an unknown value is not a small one.
func browseOrderBy(sortBy tender.SortOrder) string {
	switch sortBy {
	case tender.SortDeadline:
		// Only deadlines still ahead of us are useful "soonest first"; expired
		// and missing ones go to the end rather than the top.
		return "\nORDER BY (t.deadline IS NULL OR t.deadline < now()), t.deadline ASC, t.id DESC"
	case tender.SortValue:
		return "\nORDER BY t.value DESC NULLS LAST, t.id DESC"
	default:
		return "\nORDER BY t.published_at DESC NULLS LAST, t.id DESC"
	}
}

// SearchByFiltersRanked returns tenders matching filters in the requested
// order — the browse path, used when there is no query text to rank by.
// Scores are left at zero: with no query, relevance is undefined, and
// inventing one would be a lie the UI would then display.
func (r *TenderRepo) SearchByFiltersRanked(ctx context.Context, filters tender.Filters, sortBy tender.SortOrder, limit, offset int) ([]tender.ScoredTender, error) {
	b := &argBuilder{}
	clauses := filterClauses(filters, b)
	sql := "SELECT" + tenderSelectColumns + ",\n\t0::float8 AS relevance,\n\t'' AS snippet" +
		tenderFromClause + whereClause(clauses) +
		browseOrderBy(sortBy) +
		"\nLIMIT " + b.next(limit) + " OFFSET " + b.next(offset)

	return r.queryScoredTenders(ctx, "search by filters", sql, b.args)
}

// FacetCounts aggregates the filtered corpus by country, status and CPV
// division in one round trip.
//
// These are exact counts over everything matching the filters, not over a
// page — which is what makes them useful on a filter control ("Germany: 412").
// The three aggregates are UNION ALL-ed rather than issued separately so the
// filter predicates are evaluated once per grouping instead of three times
// across three round trips.
func (r *TenderRepo) FacetCounts(ctx context.Context, filters tender.Filters) (tender.Facets, error) {
	b := &argBuilder{}
	// Each arm builds its own clauses so the placeholders stay sequential
	// across the whole statement.
	sql := facetArm("country", "t.country", filters, b) +
		"\nUNION ALL" + facetArm("status", "t.status", filters, b) +
		"\nUNION ALL" + facetArm("cpv", "left(t.cpv, 2)", filters, b)

	rows, err := r.db.Query(ctx, sql, b.args...)
	if err != nil {
		return tender.Facets{}, fmt.Errorf("postgres: facet counts: %w", err)
	}
	defer rows.Close()

	byKind := map[string]map[string]int{"country": {}, "status": {}, "cpv": {}}
	for rows.Next() {
		var (
			kind  string
			value string
			count int
		)
		if err := rows.Scan(&kind, &value, &count); err != nil {
			return tender.Facets{}, fmt.Errorf("postgres: scan facet count: %w", err)
		}
		if bucket, ok := byKind[kind]; ok && value != "" {
			bucket[value] = count
		}
	}
	if err := rows.Err(); err != nil {
		return tender.Facets{}, fmt.Errorf("postgres: facet counts: %w", err)
	}
	return tender.BuildFacets(byKind["country"], byKind["status"], byKind["cpv"]), nil
}

// facetArm renders one GROUP BY arm of the facet query.
func facetArm(kind, expr string, filters tender.Filters, b *argBuilder) string {
	clauses := filterClauses(filters, b)
	return "\nSELECT " + b.next(kind) + " AS kind, coalesce(" + expr + ", '') AS value, count(*)::int AS n" +
		"\nFROM tenders.ingested_tenders t" + whereClause(clauses) +
		"\nGROUP BY 1, 2"
}

// FindByIDsFiltered resolves ids to full tenders, keeping only those that also
// satisfy filters. Order is unspecified — callers that need one (by fused
// relevance, say) re-sort in Go.
//
// This is the authoritative filter pass. Whatever the vector store pre-filtered
// is an optimisation on top of this, never a substitute for it.
func (r *TenderRepo) FindByIDsFiltered(ctx context.Context, ids []string, filters tender.Filters) ([]tender.Tender, error) {
	numeric := make([]string, 0, len(ids))
	for _, id := range ids {
		if _, ok := tenderIDFromString(id); ok {
			numeric = append(numeric, id)
		}
	}
	if len(numeric) == 0 {
		return nil, nil
	}

	b := &argBuilder{}
	// The ids are re-serialised as strings and cast, rather than passed as a
	// bigint[]: database/sql has no portable slice binding, and this keeps one
	// placeholder per id under the same escaping as every other argument.
	clauses := append([]string{"t.id IN (" + placeholders(numeric, b) + ")"}, filterClauses(filters, b)...)
	sql := "SELECT" + tenderSelectColumns + ",\n\t0::float8 AS relevance,\n\t'' AS snippet" +
		tenderFromClause + whereClause(clauses)

	scored, err := r.queryScoredTenders(ctx, "find tenders by ids", sql, b.args)
	if err != nil {
		return nil, err
	}
	out := make([]tender.Tender, len(scored))
	for i, s := range scored {
		out[i] = s.Tender
	}
	return out, nil
}

// queryScoredTenders runs sql and scans it into ScoredTenders. Every query in
// this file shares tenderSelectColumns' projection, so they all scan here.
func (r *TenderRepo) queryScoredTenders(ctx context.Context, what, sql string, args []any) ([]tender.ScoredTender, error) {
	rows, err := r.db.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: %s: %w", what, err)
	}
	defer rows.Close()

	var out []tender.ScoredTender
	for rows.Next() {
		var (
			row       TenderResultRow
			id        int64
			sourceURL *string
			relevance float64
			snippet   string
		)
		if err := rows.Scan(&id, &row.Title, &row.BuyerName, &row.Status, &row.ProcedureType,
			&row.Country, &row.CPV, &row.Value, &row.Currency, &row.PublishedAt, &row.Deadline,
			&row.Source, &row.SourceRef, &row.NUTS, &sourceURL, &relevance, &snippet); err != nil {
			return nil, fmt.Errorf("postgres: scan %s row: %w", what, err)
		}
		t := tender.Tender{
			ID: strconv.FormatInt(id, 10), Title: row.Title, BuyerName: row.BuyerName,
			Status: row.Status, ProcedureType: row.ProcedureType, Country: row.Country,
			CPV: row.CPV, Value: row.Value, Currency: row.Currency,
			PublishedAt: row.PublishedAt, Deadline: row.Deadline,
			Source: row.Source, SourceRef: row.SourceRef, NUTS: row.NUTS,
		}
		if sourceURL != nil {
			t.SourceURL = *sourceURL
		}
		out = append(out, tender.ScoredTender{Tender: t, RelevanceScore: relevance, Snippet: snippet})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: %s: %w", what, err)
	}
	return out, nil
}
