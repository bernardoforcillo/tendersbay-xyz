package postgres

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bernardoforcillo/drops"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/tender"
)

var tenderDetailColumns = []drops.Expression{
	TenderID, TenderSource, TenderSourceRef, TenderTitle, TenderBuyerName, TenderBuyerID,
	TenderStatus, TenderProcedureType, TenderCountry, TenderNUTS, TenderLanguage,
	TenderCPV, TenderValue, TenderCurrency, TenderPublishedAt, TenderDeadline,
	TenderDescription, TenderPublicationNumber, TenderDocumentsURL, TenderSubmissionURL,
	TenderGridUsable, TenderXMLFetchedAt, TenderXMLStatus,
}

// FindDetailByID returns the full detail row for id, or tender.ErrTenderNotFound.
//
// It issues six queries: the scalar detail row, its cpv_secondary array, then
// documents, lots, criteria and organizations. That is six round trips for a
// FIXED shape, not an N+1 — N is one tender, and each child read is a single
// statement over an indexed tender_id regardless of how many rows it returns.
// The N+1 that would have been easy to write here is a criteria query per lot;
// CriteriaByTenderID exists precisely so that a 13-lot notice still costs one.
func (r *TenderRepo) FindDetailByID(ctx context.Context, id int64) (*tender.TenderDetail, error) {
	var rows []DBTenderDetail
	if err := r.db.Select(tenderDetailColumns...).From(Tenders).Where(TenderID.Eq(id)).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: find tender detail: %w", err)
	}
	if len(rows) == 0 {
		return nil, tender.ErrTenderNotFound
	}
	cpvSecondary, err := r.cpvSecondary(ctx, id)
	if err != nil {
		return nil, err
	}
	docs, err := r.DocumentsByTenderID(ctx, id)
	if err != nil {
		return nil, err
	}
	lots, err := r.LotsByTenderID(ctx, id)
	if err != nil {
		return nil, err
	}
	criteria, err := r.CriteriaByTenderID(ctx, id)
	if err != nil {
		return nil, err
	}
	orgs, err := r.OrganizationsByTenderID(ctx, id)
	if err != nil {
		return nil, err
	}
	d := detailRowToDomain(rows[0])
	d.CPVSecondary = cpvSecondary
	d.Documents = docs
	d.Lots = lots
	d.Criteria = criteria
	d.Organizations = orgs
	return &d, nil
}

// cpvSecondary reads the text[] column via array_to_string (drops' typed
// builder can't scan a Postgres array), returning nil for an empty array.
func (r *TenderRepo) cpvSecondary(ctx context.Context, id int64) ([]string, error) {
	rows, err := r.db.Query(ctx,
		"SELECT array_to_string(cpv_secondary, ',') FROM tenders.ingested_tenders WHERE id = $1", id)
	if err != nil {
		return nil, fmt.Errorf("postgres: cpv_secondary: %w", err)
	}
	defer rows.Close()
	var csv string
	if rows.Next() {
		if err := rows.Scan(&csv); err != nil {
			return nil, fmt.Errorf("postgres: scan cpv_secondary: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: cpv_secondary rows: %w", err)
	}
	if csv == "" {
		return nil, nil
	}
	return strings.Split(csv, ","), nil
}

func (r *TenderRepo) DocumentsByTenderID(ctx context.Context, id int64) ([]tender.Document, error) {
	var rows []DBTenderDocument
	if err := r.db.Select(TenderDocURL, TenderDocType).From(TenderDocuments).
		Where(TenderDocTenderID.Eq(id)).OrderBy(TenderDocURL.Asc()).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: tender documents: %w", err)
	}
	out := make([]tender.Document, len(rows))
	for i, row := range rows {
		out[i] = tender.Document{URL: row.URL, Type: row.Type}
	}
	return out, nil
}

// lotsSQL reads one tender's lots.
//
// Raw SQL rather than the typed builder because of cpv_secondary: it is
// text[], which drops has neither a column type nor a scan path for, so it
// crosses the boundary comma-joined exactly the way the tender-level
// cpv_secondary already does. Once one column in the projection has to be an
// expression, keeping the rest in the builder would only split one read across
// two statements.
//
// The nullable eForms columns are coalesced in SQL rather than scanned into
// pointers: unlike grid_usable, "no documents_url on this lot" and "an empty
// documents_url on this lot" are the same fact, so there is no three-valued
// distinction to preserve and a plain string is the honest shape.
const lotsSQL = `
SELECT ref, title, cpv, value, currency, deadline,
       array_to_string(cpv_secondary, ','),
       coalesce(documents_url, ''), coalesce(submission_url, '')
FROM tenders.ingested_tender_lots
WHERE tender_id = $1
ORDER BY ref ASC`

func (r *TenderRepo) LotsByTenderID(ctx context.Context, id int64) ([]tender.Lot, error) {
	rows, err := r.db.Query(ctx, lotsSQL, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: tender lots: %w", err)
	}
	defer rows.Close()

	var out []tender.Lot
	for rows.Next() {
		var (
			lot          tender.Lot
			cpvSecondary string
		)
		if err := rows.Scan(&lot.Ref, &lot.Title, &lot.CPV, &lot.Value, &lot.Currency, &lot.Deadline,
			&cpvSecondary, &lot.DocumentsURL, &lot.SubmissionURL); err != nil {
			return nil, fmt.Errorf("postgres: scan tender lot: %w", err)
		}
		if cpvSecondary != "" {
			lot.CPVSecondary = strings.Split(cpvSecondary, ",")
		}
		out = append(out, lot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: tender lots: %w", err)
	}
	return out, nil
}

// criteriaSQL reads every criterion of one tender — notice-level and every
// lot's — in one statement.
//
// weight is projected as ::float8 on purpose: the column is `numeric`, and the
// driver hands a numeric back as a string or a decimal type that will not scan
// into a *float64. The cast is what makes the nullable weight round-trip as a
// nullable Go float, which is the whole point of keeping it a pointer.
//
// The ordering is (lot_ref, ordinal), which is what lets TenderDetail's
// CriteriaForLot group in a single pass and return entries in published order.
// lot_ref sorts ” (notice-level) first, so the inherited grid always leads.
const criteriaSQL = `
SELECT lot_ref, ordinal, type, name, description,
       weight::float8, weight_raw, lang
FROM tenders.ingested_tender_award_criteria
WHERE tender_id = $1
ORDER BY lot_ref ASC, ordinal ASC`

// CriteriaByTenderID satisfies tender.Repo.
func (r *TenderRepo) CriteriaByTenderID(ctx context.Context, id int64) ([]tender.AwardCriterion, error) {
	rows, err := r.db.Query(ctx, criteriaSQL, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: tender award criteria: %w", err)
	}
	defer rows.Close()

	var out []tender.AwardCriterion
	for rows.Next() {
		var c tender.AwardCriterion
		// c.Weight stays a *float64 all the way through the scan: a NULL
		// weight — the common case — must arrive as nil, not as 0.
		if err := rows.Scan(&c.LotRef, &c.Ordinal, &c.Type, &c.Name, &c.Description,
			&c.Weight, &c.WeightRaw, &c.Lang); err != nil {
			return nil, fmt.Errorf("postgres: scan award criterion: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: tender award criteria: %w", err)
	}
	return out, nil
}

// organizationsSQL reads the parties one notice names.
//
// roles is text[], so it crosses the boundary comma-joined for the same reason
// cpv_secondary does. A comma is safe as the separator here because the values
// are a closed vocabulary of hyphenated slugs ("buyer", "appeal-body",
// "appeal-information", "mediation") that cannot themselves contain one.
const organizationsSQL = `
SELECT org_ref, name, company_id, array_to_string(roles, ','),
       email, phone, website, city, country
FROM tenders.ingested_tender_organizations
WHERE tender_id = $1
ORDER BY org_ref ASC`

// OrganizationsByTenderID satisfies tender.Repo.
func (r *TenderRepo) OrganizationsByTenderID(ctx context.Context, id int64) ([]tender.Organization, error) {
	rows, err := r.db.Query(ctx, organizationsSQL, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: tender organizations: %w", err)
	}
	defer rows.Close()

	var out []tender.Organization
	for rows.Next() {
		var (
			org   tender.Organization
			roles string
		)
		if err := rows.Scan(&org.Ref, &org.Name, &org.CompanyID, &roles,
			&org.Email, &org.Phone, &org.Website, &org.City, &org.Country); err != nil {
			return nil, fmt.Errorf("postgres: scan tender organization: %w", err)
		}
		if roles != "" {
			org.Roles = strings.Split(roles, ",")
		}
		out = append(out, org)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: tender organizations: %w", err)
	}
	return out, nil
}

// RecentTenderRefs returns up to limit tenders (id + published_at) newest first,
// for the dynamic sitemap. limit is clamped to [1, 50000].
func (r *TenderRepo) RecentTenderRefs(ctx context.Context, limit int) ([]tender.TenderRef, error) {
	if limit <= 0 || limit > 50000 {
		limit = 50000
	}
	type refRow struct {
		ID          int64      `drop:"id"`
		PublishedAt *time.Time `drop:"published_at"`
	}
	var rows []refRow
	if err := r.db.Select(TenderID, TenderPublishedAt).From(Tenders).
		OrderBy(TenderPublishedAt.Desc()).Limit(int64(limit)).All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("postgres: recent tender refs: %w", err)
	}
	out := make([]tender.TenderRef, len(rows))
	for i, row := range rows {
		lastmod := ""
		if row.PublishedAt != nil {
			lastmod = row.PublishedAt.Format(time.RFC3339)
		}
		out[i] = tender.TenderRef{ID: strconv.FormatInt(row.ID, 10), Lastmod: lastmod}
	}
	return out, nil
}

func detailRowToDomain(row DBTenderDetail) tender.TenderDetail {
	return tender.TenderDetail{
		ID: strconv.FormatInt(row.ID, 10), Source: row.Source, SourceRef: row.SourceRef,
		Title: row.Title, BuyerName: row.BuyerName, BuyerID: row.BuyerID, Status: row.Status,
		ProcedureType: row.ProcedureType, Country: row.Country, NUTS: row.NUTS, Language: row.Language,
		CPV: row.CPV, Value: row.Value, Currency: row.Currency,
		PublishedAt: row.PublishedAt, Deadline: row.Deadline,
		Description: row.Description,
		// The nullable text columns flatten to "" — an absent URL and an empty
		// one mean the same thing. GridUsable does NOT flatten: its NULL is a
		// third value ("not yet read") that no bool can carry, so the pointer
		// is passed through untouched.
		PublicationNumber: derefString(row.PublicationNumber),
		DocumentsURL:      derefString(row.DocumentsURL),
		SubmissionURL:     derefString(row.SubmissionURL),
		GridUsable:        row.GridUsable,
		EnrichedAt:        enrichedAt(row.XMLFetchedAt, row.XMLStatus),
	}
}

// enrichedAt maps the fetcher's two persisted columns onto the one domain
// question a reader actually asks: did we successfully read this notice, and
// when.
//
// xml_fetched_at is stamped on permanent FAILURE as well as on success — a
// notice with no XML at all ('absent') and one whose XML would not parse
// ('parse_error') are both terminal, so both get a timestamp so the enricher's
// queue stops re-listing them. Reporting that timestamp as "enriched" would
// tell a reader we have read a notice we demonstrably could not, which is the
// inaccessible-versus-absent conflation in its smallest form.
func enrichedAt(fetchedAt *time.Time, status *string) *time.Time {
	if fetchedAt == nil || status == nil || *status != xmlStatusOK {
		return nil
	}
	return fetchedAt
}

// derefString flattens a nullable text column to "". Used only where NULL and
// the empty string are genuinely the same fact — never for grid_usable.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
