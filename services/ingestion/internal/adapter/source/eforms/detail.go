package eforms

// eForms notice-document parsing: the structure that only the published XML
// carries, and that TED's Search API (the shape Notice/Map handle) does not
// expose at all — per-lot values, the award grid, and the parties a bidder
// needs in order to challenge an award.
//
// ATTRIBUTION. The XML struct-tag map below is forked from
// github.com/bernardoforcillo/tedmcp (internal/ted/dossier.go), where it was
// reverse-engineered from real notices. That tag map is the expensive part —
// eForms nests the same information at several depths depending on notice
// subtype, and the paths are not derivable from the schema alone. It is
// copied rather than imported because tedmcp keeps it under internal/, which
// makes it unimportable from this module by construction, and because its
// output types are wrong for persistence here in three specific ways. Each of
// those is a correctness bug for this codebase's purposes, not a matter of
// taste, so each is called out where it is fixed:
//
//  1. tedmcp's trimTime keeps the first five characters of an eForms time
//     ("16:00:00+02:00" becomes "16:00"), silently discarding the UTC offset.
//     Stored into a timestamptz that is a legally binding submission deadline,
//     that is an instant wrong by up to two hours. This file never truncates a
//     time: the full date and time, offset included, go into parseDeadline,
//     which already slices eForms' fixed-width offsets correctly.
//  2. tedmcp emits values and weights as display strings. Lot values here go
//     through parseMinorUnits, matching the bigint minor-unit column the rest
//     of this package already writes; weights are kept twice, parsed and
//     verbatim, because published weights are inconsistently formatted ("30",
//     "30%", "30,5") and a parse failure must cost comparability, not the
//     datum. See parseWeight.
//  3. tedmcp's sub-criterion type has no language field, so criterion language
//     is dropped entirely. Every text element here is decoded as a repeated,
//     language-tagged element and selected explicitly. See pickLangText.
//
// A fourth difference is not a deviation but the reason the fork targets
// dossier.go at all: tedmcp's other eForms parser dedupes criteria across
// lots, which deliberately destroys the per-lot association this package
// exists to keep.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"html"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/bernardoforcillo/tendersbay-xyz/go-services/tender"
)

// The roles a notice assigns to the organizations it names. A party can hold
// several: buyers routinely name themselves as the body to ask about appeal
// deadlines while naming a court as the body that hears them.
const (
	roleBuyer             = "buyer"
	roleAppealBody        = "appeal-body"
	roleAppealInformation = "appeal-information"
	roleMediation         = "mediation"
)

// ublSchemaNamespace prefixes the namespace of every UBL 2.3 notice root
// element eForms uses (ContractNotice, ContractAwardNotice,
// PriorInformationNotice, …).
//
// It is checked because encoding/xml decoding "succeeds" against almost any
// document: an HTML error page or a truncated response parses cleanly into a
// zero-valued struct, which would then be persisted as a notice that publishes
// no criteria and no lots. That is indistinguishable from the genuine — and
// frequent — case of a notice that really publishes neither, and it is exactly
// the outcome the criteria-coverage numbers exist to measure. Silently
// inflating "we looked and there was nothing" with "we never had the document"
// is worse than a loud parse failure, so the root element is required to be
// what it claims to be.
const ublSchemaNamespace = "urn:oasis:names:specification:ubl:schema:xsd:"

// DetailParser adapts ParseDetail to the NoticeDetailParser port that
// internal/core/enrich defines, so the enricher can be handed a parser without
// knowing that eForms — or XML — is involved at all.
//
// It deliberately does not import that package. A method set satisfies a Go
// interface structurally, so the port stays owned by core while this adapter
// keeps zero dependencies beyond the domain model, exactly as the dependency
// rule requires. The compile-time check that the two still line up lives in
// this package's tests, where a dependency on core is free.
type DetailParser struct{}

// Parse implements the notice-document parser port.
func (DetailParser) Parse(data []byte) (tender.NoticeDetail, error) { return ParseDetail(data) }

// ParseDetail turns one eForms XML document into domain detail. It is pure:
// no I/O, no context, no HTTP vocabulary, no notion of which tender the
// document belongs to — that is the caller's to know. Keeping the boundary
// this narrow is what makes swapping this implementation for an imported
// parser, should tedmcp ever promote its packages out of internal/, a
// one-file change.
//
// A decode error is returned as an error: it is deterministic, so retrying it
// against TED would only cost requests. Everything else is best-effort by
// design — a notice that publishes no lots and no criteria is a real and
// common outcome, and returning it as an empty NoticeDetail rather than an
// error is what lets the coverage measurement see it as a population instead
// of a per-row failure.
func ParseDetail(data []byte) (tender.NoticeDetail, error) {
	var root xmlDetailRoot

	// Strict = false and HTML entities mirror tedmcp: real notices carry
	// double-escaped entities and the occasional unbalanced construct that a
	// strict decoder rejects outright, and refusing a notice over an artefact
	// of the buyer's authoring tool loses the whole document to keep a
	// technicality.
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	if err := dec.Decode(&root); err != nil {
		return tender.NoticeDetail{}, fmt.Errorf("parse eForms notice: %w", err)
	}
	if !strings.HasPrefix(root.XMLName.Space, ublSchemaNamespace) {
		return tender.NoticeDetail{}, fmt.Errorf(
			"parse eForms notice: root element <%s> is not a UBL notice document", root.XMLName.Local)
	}

	// preferred drives every language selection below. NoticeLanguageCode is
	// the notice's own declaration of what it was written in, which is a
	// better answer than any heuristic over the languageID attributes.
	preferred := cleanText(root.NoticeLanguage)

	d := tender.NoticeDetail{
		Organizations: buildOrganizations(root, preferred),
		Language:      preferred,
	}
	if d.Language == "" {
		// A notice that declares no language still tags its text elements, so
		// fall back to whichever language the notice's own title resolved to.
		// Recording the language actually used matters more than recording the
		// one that was asked for.
		_, d.Language = pickLangText(root.Project.Name, "")
	}

	if len(root.Lots) <= 1 {
		fillSingleLot(&d, root, preferred)
	} else {
		fillMultiLot(&d, root, preferred)
	}
	return d, nil
}

// fillSingleLot handles the two shapes that carry one procurement scope: a
// notice with exactly one ProcurementProjectLot, and a notice with none at all
// (which puts its terms directly on the notice element).
//
// The single lot is deliberately collapsed onto the notice rather than emitted
// as a one-entry Lots slice. Two reasons, and the second is the load-bearing
// one:
//
//   - It matches the domain model, where a single-lot tender keeps its scope on
//     the tender itself and Lots is reserved for genuinely divisible
//     procurements — the same convention buildLots already applies on the
//     search-result path, so the two writers agree about which notices produce
//     lot rows at all.
//   - Collapsing moves the lot's criteria to notice level (LotRef ""), which is
//     what "these criteria apply to the whole procurement" means. Left on a
//     synthetic lot, every single-lot notice in the corpus would key its
//     criteria on a lot reference that exists nowhere else in storage.
//
// Nothing is lost by collapsing: the only lot-scoped fields with no
// notice-level home are value and CPV, and both are already carried on the
// tender from the search-result payload.
func fillSingleLot(d *tender.NoticeDetail, root xmlDetailRoot, preferred string) {
	var lotTerms xmlTerms
	var lotProcess xmlProcess
	if len(root.Lots) == 1 {
		lotTerms = root.Lots[0].Terms
		lotProcess = root.Lots[0].Process
	}

	// Real single-lot notices put essentially everything on the lot and leave
	// the notice-level TenderingTerms nearly empty (verified on 548186-2026,
	// whose notice-level terms carry only an exclusion-grounds code), so the
	// lot's value wins wherever both are present.
	d.DocumentsURL = firstNonEmpty(cleanText(lotTerms.Documents.URI), cleanText(root.Terms.Documents.URI))
	d.SubmissionURL = firstNonEmpty(cleanText(lotTerms.Recipient.EndpointID), cleanText(root.Terms.Recipient.EndpointID))

	d.Deadline = parseDeadline(cleanText(lotProcess.Deadline.EndDate), cleanText(lotProcess.Deadline.EndTime))
	if d.Deadline == nil {
		d.Deadline = parseDeadline(cleanText(root.Process.Deadline.EndDate), cleanText(root.Process.Deadline.EndTime))
	}

	// Criteria are never a union of the two levels. On every notice sampled
	// for this fork exactly one of the two is populated — they are alternative
	// encodings of one set, not two sets — so concatenating them would
	// double-count a grid the moment a buyer's tool emits both. The more
	// specific level wins.
	d.Criteria = buildCriteria(lotTerms, "", preferred)
	if len(d.Criteria) == 0 {
		d.Criteria = buildCriteria(root.Terms, "", preferred)
	}
}

// fillMultiLot handles a genuinely divisible procurement, where each lot nests
// its own ProcurementProject, TenderingProcess and TenderingTerms.
//
// That nesting is why per-lot enrichment is sound here and is not on the
// search-result path: the association is structural, not an index match
// between two independently-ordered arrays. Be honest about what that buys,
// though — on real multi-lot notices the lots routinely share one CPV, one
// documents URL, one submission URL and one criteria set, and differ only in
// value and title (verified on 545620-2026: 13 lots, 13 distinct values, one
// of everything else). Per-lot *value* is the real win; a per-lot CPV will
// frequently be degenerate. The shared-value hoisting below exists so that
// degeneracy is stored once rather than repeated per lot.
func fillMultiLot(d *tender.NoticeDetail, root xmlDetailRoot, preferred string) {
	d.Criteria = buildCriteria(root.Terms, "", preferred)
	d.DocumentsURL = cleanText(root.Terms.Documents.URI)
	d.SubmissionURL = cleanText(root.Terms.Recipient.EndpointID)
	d.Deadline = parseDeadline(cleanText(root.Process.Deadline.EndDate), cleanText(root.Process.Deadline.EndTime))

	d.Lots = make([]tender.Lot, 0, len(root.Lots))
	// A lot the document does not identify, or identifies with a reference it
	// has already used, is dropped. eForms requires that id, so either shape is
	// a malformed notice — but dropping it is a correctness requirement, not
	// hygiene: lot_ref is half the key every criterion is stored under, so an
	// empty ref collides with this notice's own notice-level criteria and a
	// repeated one collides with the earlier lot's. Either collision fails the
	// entire notice's write, which leaves the row queued and re-downloading its
	// document on every pass rather than losing one malformed lot.
	//
	// It also errs in the direction this pass is measured on: dropping a lot can
	// only ever undercount the published grid, and a missing criterion is the
	// cheaper mistake than a fabricated one.
	seenRefs := make(map[string]bool, len(root.Lots))
	for _, l := range root.Lots {
		ref := cleanText(l.ID)
		if ref == "" || seenRefs[ref] {
			continue
		}
		seenRefs[ref] = true
		title, _ := pickLangText(l.Project.Name, preferred)
		cpv, cpvSecondary := dedupCPV(l.Project.cpvCodes())

		d.Lots = append(d.Lots, tender.Lot{
			Ref:           ref,
			Title:         title,
			CPV:           cpv,
			CPVSecondary:  cpvSecondary,
			Value:         parseMinorUnits(cleanText(l.Project.Total.Amount.Value)),
			Currency:      cleanText(l.Project.Total.Amount.Currency),
			Deadline:      parseDeadline(cleanText(l.Process.Deadline.EndDate), cleanText(l.Process.Deadline.EndTime)),
			DocumentsURL:  cleanText(l.Terms.Documents.URI),
			SubmissionURL: cleanText(l.Terms.Recipient.EndpointID),
			Criteria:      buildCriteria(l.Terms, ref, preferred),
		})
	}

	// A notice-level deadline is optional in eForms; when it is absent the
	// lots carry the real one, and a tender with no deadline at all reads as
	// "no closing date published", which would be wrong.
	if d.Deadline == nil {
		for _, l := range d.Lots {
			if l.Deadline != nil {
				d.Deadline = l.Deadline
				break
			}
		}
	}

	// Hoist the URLs the lots agree on. Lot.DocumentsURL/SubmissionURL are
	// documented as overrides — "" means "use the notice's" — so clearing a
	// lot value that equals the notice's is the same fact stored once instead
	// of once per lot, and a lot that genuinely differs keeps its own.
	if d.DocumentsURL == "" {
		d.DocumentsURL = firstLotValue(d.Lots, func(l tender.Lot) string { return l.DocumentsURL })
	}
	if d.SubmissionURL == "" {
		d.SubmissionURL = firstLotValue(d.Lots, func(l tender.Lot) string { return l.SubmissionURL })
	}
	for i := range d.Lots {
		if d.Lots[i].DocumentsURL == d.DocumentsURL {
			d.Lots[i].DocumentsURL = ""
		}
		if d.Lots[i].SubmissionURL == d.SubmissionURL {
			d.Lots[i].SubmissionURL = ""
		}
	}
}

// firstLotValue returns the first non-empty value get yields across lots.
func firstLotValue(lots []tender.Lot, get func(tender.Lot) string) string {
	for _, l := range lots {
		if v := get(l); v != "" {
			return v
		}
	}
	return ""
}

// buildCriteria reads one award grid out of a terms block, tagging every entry
// with lotRef ("" for notice level).
//
// Ordinal counts the entries that are kept, so it stays contiguous from 1 and
// remains usable as the identifying key alongside lotRef. An entry carrying
// nothing at all — no type, no text, no weight — is dropped rather than
// persisted: it is an artefact of an authoring tool emitting an empty element,
// and storing it would inflate criteria_count, the very number that must not
// overstate coverage. Note that this is the *only* filtering done here; unlike
// tedmcp this does not dedupe across lots, because two lots publishing the
// same criterion is a fact about the notice, not a repetition to collapse.
func buildCriteria(terms xmlTerms, lotRef, preferred string) []tender.AwardCriterion {
	var out []tender.AwardCriterion
	for _, s := range terms.Awarding.Criterion.Subordinate {
		name, nameLang := pickLangText(s.Name, preferred)
		desc, descLang := pickLangText(s.Description, preferred)
		raw := s.weightRaw()

		c := tender.AwardCriterion{
			LotRef:      lotRef,
			Ordinal:     len(out) + 1,
			Type:        cleanText(s.TypeCode),
			Name:        name,
			Description: desc,
			Weight:      parseWeight(raw),
			WeightRaw:   raw,
			Lang:        firstNonEmpty(nameLang, descLang),
		}
		if c.Type == "" && c.Name == "" && c.Description == "" && c.WeightRaw == "" {
			continue
		}
		out = append(out, c)
	}
	return out
}

// buildOrganizations collects the parties the notice declares and then labels
// them.
//
// eForms declares each organization once, in the notice's extension block, and
// refers to it by an id from the places that give it a role — so the two passes
// are not a style choice, they are the document's shape. Roles are gathered
// from the notice-level terms *and* every lot's, because a single-lot notice
// puts its appeal terms on the lot and a multi-lot one on the notice.
func buildOrganizations(root xmlDetailRoot, preferred string) []tender.Organization {
	var orgs []tender.Organization
	// Index by position rather than by pointer: appending reallocates the
	// backing array, which would strand any pointer taken before it grew.
	indexByRef := make(map[string]int, len(root.Organizations))

	for _, o := range root.Organizations {
		name, _ := pickLangText(o.Company.Name, preferred)
		org := tender.Organization{
			Ref:       cleanText(o.Company.ID),
			Name:      name,
			CompanyID: cleanText(o.Company.CompanyID),
			Email:     cleanText(o.Company.Email),
			Phone:     cleanText(o.Company.Phone),
			Website:   cleanText(o.Company.WebsiteURI),
			City:      cleanText(o.Company.City),
			Country:   cleanText(o.Company.Country),
		}
		// A party the notice does not reference, or references with an id it
		// has already used, is dropped. Neither is tidiness:
		//
		//   - The ref is the whole point of the two-pass shape. Roles are
		//     assigned by pointing at an id, so a party without one can never
		//     be labelled and is stored as an anonymous row nothing links to.
		//   - Worse, it would swallow roles that belong to nobody. Most notices
		//     publish no AppealTerms at all, so addRole below is called with an
		//     empty ref several times per notice; a single kept party with an
		//     empty ref would match every one of them and be published as the
		//     buyer AND the appeal body AND the mediation body — fabricated
		//     data on exactly the field this parse exists to provide.
		//   - Both shapes also collide on the (tender_id, org_ref) uniqueness
		//     the store enforces, which fails the whole notice's write rather
		//     than just losing the malformed party.
		//
		// Keeping the FIRST occurrence of a repeated ref matches how the roles
		// below resolve one, so the party that gets labelled is the party that
		// gets stored.
		if org.Ref == "" {
			continue
		}
		if _, seen := indexByRef[org.Ref]; seen {
			continue
		}
		orgs = append(orgs, org)
		indexByRef[org.Ref] = len(orgs) - 1
	}

	addRole := func(ref, role string) {
		ref = cleanText(ref)
		// Belt and braces against the fabrication described above: an absent
		// reference must match nothing, and it arrives here routinely.
		if ref == "" {
			return
		}
		if i, ok := indexByRef[ref]; ok {
			orgs[i].Roles = appendUniqueRole(orgs[i].Roles, role)
		}
	}
	addRole(root.ContractingParty.PartyID, roleBuyer)
	for _, terms := range root.allTerms() {
		addRole(terms.Appeal.Receiver.ID, roleAppealBody)
		addRole(terms.Appeal.Information.ID, roleAppealInformation)
		addRole(terms.Appeal.Mediation.ID, roleMediation)
	}
	return orgs
}

// --- XML mapping ---
//
// Field tags name only the local element, so the eForms namespace prefixes
// (cac:, cbc:, efac:, efbc:) do not have to be spelled out — encoding/xml
// matches an untagged namespace against any.

type xmlDetailRoot struct {
	XMLName          xml.Name
	NoticeLanguage   string            `xml:"NoticeLanguageCode"`
	Organizations    []xmlOrganization `xml:"UBLExtensions>UBLExtension>ExtensionContent>EformsExtension>Organizations>Organization"`
	ContractingParty struct {
		PartyID string `xml:"Party>PartyIdentification>ID"`
	} `xml:"ContractingParty"`
	// Project is the notice's own procurement scope. Its value and CPV are
	// already ingested from the search payload, so it is decoded only for its
	// language-tagged title, which is the fallback when a notice declares no
	// NoticeLanguageCode.
	Project xmlProject `xml:"ProcurementProject"`
	Process xmlProcess `xml:"TenderingProcess"`
	Terms   xmlTerms   `xml:"TenderingTerms"`
	Lots    []xmlLot   `xml:"ProcurementProjectLot"`
}

// allTerms returns the notice-level terms plus every lot's, since a single-lot
// notice puts everything on the lot and a multi-lot one splits it.
func (r xmlDetailRoot) allTerms() []xmlTerms {
	terms := make([]xmlTerms, 0, 1+len(r.Lots))
	terms = append(terms, r.Terms)
	for _, l := range r.Lots {
		terms = append(terms, l.Terms)
	}
	return terms
}

// xmlLangText is one occurrence of a language-tagged text element.
//
// This type is why the elements below are decoded as slices. eForms documents
// are published as MUL: a notice available in two languages repeats
// <cbc:Name> once per language, each with its own languageID. Decoding such an
// element into a plain string makes encoding/xml keep whichever occurrence
// came first, so the language depends on the buyer's authoring tool and
// nothing records which one was taken. Repeated-and-selected is the only shape
// that makes the choice explicit and auditable.
type xmlLangText struct {
	Value string `xml:",chardata"`
	Lang  string `xml:"languageID,attr"`
}

type xmlOrganization struct {
	Company struct {
		WebsiteURI string        `xml:"WebsiteURI"`
		ID         string        `xml:"PartyIdentification>ID"`
		Name       []xmlLangText `xml:"PartyName>Name"`
		City       string        `xml:"PostalAddress>CityName"`
		Country    string        `xml:"PostalAddress>Country>IdentificationCode"`
		CompanyID  string        `xml:"PartyLegalEntity>CompanyID"`
		Phone      string        `xml:"Contact>Telephone"`
		Email      string        `xml:"Contact>ElectronicMail"`
	} `xml:"Company"`
}

type xmlProcess struct {
	ProcedureCode string `xml:"ProcedureCode"`
	Deadline      struct {
		// EndDate and EndTime are kept whole, offsets included
		// ("2026-09-10+02:00" and "12:30:00+02:00"), and are only ever combined
		// by parseDeadline. Truncating either to its wall-clock part is the
		// bug this fork exists in part to not inherit.
		EndDate string `xml:"EndDate"`
		EndTime string `xml:"EndTime"`
	} `xml:"TenderSubmissionDeadlinePeriod"`
}

type xmlTerms struct {
	Documents struct {
		ID  string `xml:"ID"`
		URI string `xml:"Attachment>ExternalReference>URI"`
	} `xml:"CallForTendersDocumentReference"`
	Recipient struct {
		EndpointID string `xml:"EndpointID"`
	} `xml:"TenderRecipientParty"`
	Appeal struct {
		Receiver    xmlPartyRef `xml:"AppealReceiverParty>PartyIdentification"`
		Information xmlPartyRef `xml:"AppealInformationParty>PartyIdentification"`
		Mediation   xmlPartyRef `xml:"MediationParty>PartyIdentification"`
	} `xml:"AppealTerms"`
	Awarding struct {
		Criterion xmlAwardingCriterion `xml:"AwardingCriterion"`
	} `xml:"AwardingTerms"`
}

type xmlPartyRef struct {
	ID string `xml:"ID"`
}

type xmlAwardingCriterion struct {
	Subordinate []xmlSubCriterion `xml:"SubordinateAwardingCriterion"`
}

type xmlSubCriterion struct {
	TypeCode    string         `xml:"AwardingCriterionTypeCode"`
	Name        []xmlLangText  `xml:"Name"`
	Description []xmlLangText  `xml:"Description"`
	Parameters  []xmlCritParam `xml:"UBLExtensions>UBLExtension>ExtensionContent>EformsExtension>AwardCriterionParameter"`
}

// weightRaw returns the criterion's weight exactly as published, or "" when it
// publishes none — which is the common case, and the reason Weight is a
// pointer downstream.
//
// The same parameter block also carries minimum scores, thresholds and fixed
// values. Those are not weights, so only the number-weight list is read; a
// looser match would invent a scoring grid out of a pass/fail threshold.
func (s xmlSubCriterion) weightRaw() string {
	for _, p := range s.Parameters {
		if p.Code.ListName == "number-weight" {
			return cleanText(p.Numeric)
		}
	}
	return ""
}

type xmlCritParam struct {
	Code struct {
		ListName string `xml:"listName,attr"`
	} `xml:"ParameterCode"`
	Numeric string `xml:"ParameterNumeric"`
}

type xmlLot struct {
	ID      string     `xml:"ID"`
	Project xmlProject `xml:"ProcurementProject"`
	Process xmlProcess `xml:"TenderingProcess"`
	Terms   xmlTerms   `xml:"TenderingTerms"`
}

type xmlProject struct {
	Name  []xmlLangText `xml:"Name"`
	Total struct {
		Amount struct {
			Value    string `xml:",chardata"`
			Currency string `xml:"currencyID,attr"`
		} `xml:"EstimatedOverallContractAmount"`
	} `xml:"RequestedTenderTotal"`
	Main       xmlClassification   `xml:"MainCommodityClassification"`
	Additional []xmlClassification `xml:"AdditionalCommodityClassification"`
}

// cpvCodes returns the project's CPV codes, main first, with empties dropped
// so that a project declaring only additional classifications does not end up
// with an empty primary code and a populated secondary list.
func (p xmlProject) cpvCodes() []string {
	codes := make([]string, 0, 1+len(p.Additional))
	if c := cleanText(p.Main.Code); c != "" {
		codes = append(codes, c)
	}
	for _, a := range p.Additional {
		if c := cleanText(a.Code); c != "" {
			codes = append(codes, c)
		}
	}
	return codes
}

type xmlClassification struct {
	Code string `xml:"ItemClassificationCode"`
}

// --- helpers ---

// pickLangText selects one occurrence of a repeated, language-tagged element
// and reports which language it took.
//
// The order is: the notice's own declared language, then English, then
// whatever is first available. It mirrors the fallback pickText already
// applies to the search payload's per-language maps, and it exists for the
// same reason — but here the stakes are higher, because the alternative is not
// a wrong-language title, it is a silent first-wins choice that looks
// deterministic and is not.
//
// Both the value and its language are returned so callers can persist the
// language alongside the text; a stored criterion whose language is unknown
// cannot be told apart from one whose language was never chosen.
func pickLangText(texts []xmlLangText, preferred string) (value, lang string) {
	if preferred != "" {
		if v, l, ok := findLangText(texts, func(t xmlLangText) bool { return t.Lang == preferred }); ok {
			return v, l
		}
	}
	if v, l, ok := findLangText(texts, func(t xmlLangText) bool { return t.Lang == "ENG" }); ok {
		return v, l
	}
	if v, l, ok := findLangText(texts, func(xmlLangText) bool { return true }); ok {
		return v, l
	}
	return "", ""
}

// findLangText returns the first occurrence matching match whose text is not
// blank. Blank occurrences are skipped rather than accepted, so a notice that
// emits an empty element in its own language still yields the translation
// instead of an empty string.
func findLangText(texts []xmlLangText, match func(xmlLangText) bool) (value, lang string, ok bool) {
	for _, t := range texts {
		if !match(t) {
			continue
		}
		if v := cleanText(t.Value); v != "" {
			return v, t.Lang, true
		}
	}
	return "", "", false
}

// parseWeight turns a published weight into a number, or nil when it publishes
// none or publishes something this function refuses to guess at.
//
// nil is a real answer here, not an error path: most notices attach no weight
// to their criteria at all, and the caller keeps the verbatim string
// regardless, so declining to parse costs comparability and nothing else. A
// fabricated weight is worse than an absent one, so the accepted grammar is
// exactly the two forms seen in the wild — a trailing percent sign and a comma
// decimal separator — and everything else is left to WeightRaw:
//
//   - Two separators ("1.234,5", "1,234.5") mean one of them is a grouping
//     separator, and which is which depends on a locale this element does not
//     declare.
//   - A comma followed by exactly three digits ("1,000") is the same ambiguity
//     with one separator: European notation reads 1, English notation reads a
//     thousand, and the naive comma-to-point substitution silently picks the
//     first. That substitution is what the previous version of this function
//     did, contradicting this very comment.
//   - Infinity and NaN are accepted by strconv.ParseFloat and rejected by a
//     numeric column, so letting one through would turn an odd string into a
//     write failure for the whole notice.
func parseWeight(raw string) *float64 {
	s := strings.TrimSpace(raw)
	s = strings.TrimSpace(strings.TrimSuffix(s, "%"))
	if s == "" {
		return nil
	}

	commas := strings.Count(s, ",")
	if commas > 1 || (commas == 1 && strings.Contains(s, ".")) {
		return nil
	}
	if i := strings.IndexByte(s, ','); i >= 0 {
		if len(s)-i-1 == 3 {
			return nil
		}
		s = s[:i] + "." + s[i+1:]
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(v, 0) || math.IsNaN(v) {
		return nil
	}
	return &v
}

// lineHyphen matches a hyphen used to break a word across lines. Leaving it in
// place turns one word into two fragments, which then read as words in their
// own right to anything that later indexes or embeds this text.
var lineHyphen = regexp.MustCompile(`(\p{L})-[ \t]*[\r\n]+[ \t]*(\p{L})`)

// cleanText normalises a text node so that what is stored is words rather than
// the layout of the buyer's authoring tool. TED double-escapes entities, so a
// node the XML decoder has already unescaped still holds things like
// "dell&#8217;infanzia"; whitespace wraps arbitrarily; and a word broken across
// lines keeps its hyphen.
func cleanText(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "­", "") // soft hyphen
	s = lineHyphen.ReplaceAllString(s, "$1$2")
	return strings.Join(strings.Fields(s), " ")
}

// firstNonEmpty returns the first of its arguments that is not "".
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// appendUniqueRole adds role to list unless it is already there — a notice can
// name the same party in the same capacity from both a lot and the notice.
func appendUniqueRole(list []string, role string) []string {
	for _, r := range list {
		if r == role {
			return list
		}
	}
	return append(list, role)
}
