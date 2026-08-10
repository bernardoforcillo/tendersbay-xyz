package eforms

import (
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// Map normalizes one eForms Notice onto tender.Tender. SourceRef is the
// notice's procedure-identifier, not its publication-number: a Contract
// Notice and its later Contract Award Notice for the same procurement each
// get their own publication-number but share one procedure-identifier —
// using it as SourceRef is what makes the CN→CAN status transition land as
// an update to the same row instead of two unrelated ones (see the design
// doc's "Why SourceRef = procedure-identifier" section).
//
// PublicationNumber is carried through alongside it precisely because
// SourceRef deliberately is not it. Publication numbers are immutable — TED
// republishes a corrected notice under a new one rather than editing the old
// — which makes the field the change key for notice-document enrichment: an
// unchanged number means the eForms XML we already read cannot have changed,
// so it is never re-fetched. XMLURL is the address of that document, and it
// costs nothing to recover here because tedapi already requests the `links`
// field for the notice PDF.
func Map(n Notice, source string) tender.Tender {
	officialLang := first(n.OfficialLanguage) // e.g. "RON" — uppercase, as TED returns it
	langKey := strings.ToLower(officialLang)  // lowercase key into NoticeTitle/BuyerName/TitleLot

	country := ""
	if c := first(n.BuyerCountry); c != "" {
		country = alpha3ToAlpha2(c)
	}

	cpv, cpvSecondary := dedupCPV(n.ClassificationCPV)
	lots := buildLots(n, langKey)

	var deadline *time.Time
	if len(lots) == 0 {
		deadline = parseDeadline(first(n.DeadlineReceiptTenderDateLot), first(n.DeadlineReceiptTenderTimeLot))
	}

	var documents []tender.Document
	if url := pickLink(n.Links.PDF, officialLang); url != "" {
		documents = []tender.Document{{URL: url, Type: "notice"}}
	}

	return tender.Tender{
		Source:            source,
		SourceRef:         n.ProcedureIdentifier,
		PublicationNumber: n.PublicationNumber,
		XMLURL:            pickXMLLink(n.Links.XML, officialLang),
		Title:             pickText(n.NoticeTitle, langKey),
		Description:       pickText(n.DescriptionProc, langKey),
		Buyer:             tender.Buyer{Name: first(pickTextArray(n.BuyerName, langKey)), ID: first(n.OrganisationIdentifierBuyer)},
		Status:            statusFromNoticeType(n.NoticeType),
		ProcedureType:     n.ProcedureType,
		Language:          lang3To1(officialLang),
		Country:           country,
		CPV:               cpv,
		CPVSecondary:      cpvSecondary,
		Value:             parseMinorUnits(n.EstimatedValueProc),
		Currency:          n.EstimatedValueCurProc,
		PublishedAt:       parseDeadline(n.PublicationDate, ""),
		Deadline:          deadline,
		Documents:         documents,
		Lots:              lots,
		Raw:               n.Raw,
	}
}

// statusFromNoticeType maps eForms notice-type prefixes onto the fixed
// tender.Status enum. Anything not recognized falls to StatusUnknown
// rather than guessing — the full eForms notice-type catalog (corrigenda,
// cancellations, etc.) is a documented non-goal for this cut.
func statusFromNoticeType(noticeType string) tender.Status {
	switch {
	case strings.HasPrefix(noticeType, "cn-"):
		return tender.StatusOpen
	case strings.HasPrefix(noticeType, "can-"), strings.HasPrefix(noticeType, "car-"):
		return tender.StatusAwarded
	default:
		return tender.StatusUnknown
	}
}

// buildLots returns one tender.Lot per identifier-lot entry, with only
// Ref/Title populated. CPV/Value/Deadline are intentionally left at their
// zero value — classification-cpv does not align 1:1 with identifier-lot
// (verified live: 84 lots, 114 CPV entries on one real notice), so a naive
// index match would silently assign the wrong CPV to the wrong lot. A
// single- (or zero-) lot notice returns nil: per the domain model, a
// single-lot tender keeps its scope directly on Tender, not in Lots.
func buildLots(n Notice, langKey string) []tender.Lot {
	if len(n.IdentifierLot) <= 1 {
		return nil
	}
	titles := pickTextArray(n.TitleLot, langKey)
	lots := make([]tender.Lot, len(n.IdentifierLot))
	for i, ref := range n.IdentifierLot {
		lot := tender.Lot{Ref: ref}
		if i < len(titles) {
			lot.Title = titles[i]
		}
		lots[i] = lot
	}
	return lots
}

// pickText looks up langKey in m, falling back to "eng" — the convention
// NoticeTitle/BuyerName/TitleLot use (lowercase keys).
func pickText(m map[string]string, langKey string) string {
	if v, ok := m[langKey]; ok {
		return v
	}
	return m["eng"]
}

// pickTextArray is pickText for the map[string][]string shape BuyerName
// and TitleLot use.
func pickTextArray(m map[string][]string, langKey string) []string {
	if v, ok := m[langKey]; ok {
		return v
	}
	return m["eng"]
}

// pickLink looks up officialLang in m, falling back to "ENG" — Links.PDF
// uses UPPERCASE keys, verified to be a different casing convention than
// pickText/pickTextArray's maps (TED's own inconsistency).
func pickLink(m map[string]string, officialLang string) string {
	if v, ok := m[officialLang]; ok {
		return v
	}
	return m["ENG"]
}

// pickXMLLink returns the machine-readable eForms document's URL. Unlike the
// PDF map — one rendering per published language — the XML is a single file
// containing every language at once, which TED files under the pseudo-code
// "MUL" (verified live: `links.xml` has exactly one key, "MUL"). That key is
// therefore tried first; the per-language fallback exists only so a notice
// keyed some other way still yields a URL instead of silently dropping the
// one field the enrichment queue is resolvable by.
func pickXMLLink(m map[string]string, officialLang string) string {
	if v, ok := m["MUL"]; ok {
		return v
	}
	return pickLink(m, officialLang)
}
