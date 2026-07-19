// Package bzp maps a Poland BZP board-search notice (as decoded by bzpapi)
// onto the shared tender.Tender model. It is the protocol half of the pl-bzp
// source and knows nothing about HTTP or paging — mirroring how eforms is the
// protocol half of ted.
package bzp

import (
	"regexp"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/pl/bzpapi"
)

// Map normalizes one bzpapi.Notice onto tender.Tender. SourceRef is the stable
// objectId GUID (not the human "2024/BZP …" bzpNumber, which stays in Raw).
// Value is always nil: the board search gateway exposes only the
// isTenderAmountBelowEU boolean, no numeric estimate — that flag stays in Raw
// for Spec 2's threshold badge.
func Map(n bzpapi.Notice, source string) tender.Tender {
	cpv, secondary := splitCPV(n.CpvCode)

	var documents []tender.Document
	if n.PdfURL != "" {
		documents = []tender.Document{{URL: n.PdfURL, Type: "notice"}}
	}

	return tender.Tender{
		Source:       source,
		SourceRef:    n.ObjectID,
		Title:        n.OrderObject,
		Buyer:        tender.Buyer{Name: n.OrganizationName},
		Status:       statusFromNoticeType(n.NoticeType),
		Country:      "PL",
		Language:     "pl",
		CPV:          cpv,
		CPVSecondary: secondary,
		Value:        nil,
		Deadline:     parseTime(n.SubmittingOffersDate),
		PublishedAt:  parseTime(n.PublicationDate),
		Documents:    documents,
		Raw:          n.Raw,
	}
}

// statusFromNoticeType maps BZP's noticeType onto the fixed tender.Status enum.
// Only the two unambiguous types are mapped; everything else (e.g.
// ContractPerformingNotice) becomes StatusUnknown rather than a guess.
func statusFromNoticeType(noticeType string) tender.Status {
	switch noticeType {
	case "ContractNotice":
		return tender.StatusOpen
	case "ContractAwardNotice":
		return tender.StatusAwarded
	default:
		return tender.StatusUnknown
	}
}

// cpvCodePattern matches a CPV code: 8 digits with an optional "-checkDigit"
// suffix. Extracting by pattern (rather than splitting on commas) is what makes
// splitCPV robust to BZP's cpvCode format — a comma-joined list of
// "CODE (Polish description)" whose descriptions themselves contain commas.
var cpvCodePattern = regexp.MustCompile(`\d{8}(?:-\d)?`)

// splitCPV pulls the CPV codes out of BZP's cpvCode string and returns the
// first as the primary and the (de-duplicated) remainder as secondary, nil
// when there is at most one code. A plain "45000000" yields ("45000000", nil).
func splitCPV(raw string) (string, []string) {
	matches := cpvCodePattern.FindAllString(raw, -1)
	if len(matches) == 0 {
		return "", nil
	}
	primary := matches[0]
	var secondary []string
	seen := map[string]bool{primary: true}
	for _, code := range matches[1:] {
		if seen[code] {
			continue
		}
		seen[code] = true
		secondary = append(secondary, code)
	}
	return primary, secondary
}

// parseTime parses BZP timestamps (RFC3339 with a Z zone, optional fractional
// seconds), tolerating a bare no-zone datetime as UTC. It returns nil for an
// empty or unparseable value rather than a zero time.
func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}
