// Package edmxml builds an ESPD-EDM QualificationApplicationResponse from a
// composed espd.Response. It is shared by the two version packages
// (internal/adapter/espd/edm21 and .../edm4), which differ only in their
// Profile: the header identifiers, the namespaces and the vendored code list.
//
// # What this serializer does and does not claim
//
// It emits the document skeleton, the economic operator (identity, SME status,
// representatives as cac:PowerOfAttorney), the lots tendered for, the vendored
// criterion definitions, and ONE response per criterion: the criterion's own
// "Your answer?" property, plus the Art. 57(6) self-cleaning pair when a Part
// III ground applies and the operator described the measures taken.
//
// It deliberately does NOT fill a criterion's DETAIL properties — the year and
// amount under a turnover criterion, the description under a reference. Those
// slots exist, and the generator could hand us their UUIDs, but choosing which
// value goes in which slot by shape ("the amount property takes the amount
// leaf") is a guess, and a guess in this document is a false declaration
// carrying a signature. The PDF carries every value in full; the XML carries
// the criterion-level declaration. Filling the detail slots needs a per-
// criterion mapping reviewed against the specification, which is named work,
// not a heuristic — see README.md.
//
// # Determinism
//
// Every generated identifier is derived from the document's own content, so
// re-exporting an unchanged response produces byte-identical bytes. That is
// what makes the content_sha256 in the export audit worth storing: two exports
// with the same hash really are the same document, and a different hash really
// means something changed.
package edmxml

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/codelist"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// Profile is everything that differs between ESPD-EDM versions.
type Profile struct {
	Version espd.Version
	// UBLVersionID is the UBL release the ESPD version is built on: 2.2 for
	// ESPD-EDM 2.1.1, 2.3 for 4.1.0.
	UBLVersionID string
	// VersionID is the ESPD-EDM version itself, as it appears in cbc:VersionID.
	VersionID string
	// CustomizationID and ProfileID are 2.1.1's CEN-BII identifiers; 4.1.0
	// replaced both with a single ProfileExecutionID.
	CustomizationID    string
	ProfileID          string
	ProfileExecutionID string
	// SchemeAgencyID stamps the generated identifiers.
	SchemeAgencyID string
	// Table is the vendored code list for this version.
	Table *codelist.Table
}

// Serializer implements espd.Serializer for one profile.
type Serializer struct{ p Profile }

// New wires a serializer for a profile.
func New(p Profile) *Serializer { return &Serializer{p: p} }

// Version reports which ESPD-EDM version this serializer writes.
func (s *Serializer) Version() espd.Version { return s.p.Version }

var _ espd.Serializer = (*Serializer)(nil)

// Serialize renders the response.
func (s *Serializer) Serialize(r espd.Response) ([]byte, error) {
	doc, err := s.build(r)
	if err != nil {
		return nil, err
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(out, '\n')...), nil
}

// ── The document ────────────────────────────────────────────────────────────

type document struct {
	XMLName xml.Name `xml:"QualificationApplicationResponse"`
	XMLNS   string   `xml:"xmlns,attr"`
	CAC     string   `xml:"xmlns:cac,attr"`
	CBC     string   `xml:"xmlns:cbc,attr"`

	UBLVersionID       string `xml:"cbc:UBLVersionID"`
	CustomizationID    string `xml:"cbc:CustomizationID,omitempty"`
	ProfileID          string `xml:"cbc:ProfileID,omitempty"`
	ProfileExecutionID string `xml:"cbc:ProfileExecutionID,omitempty"`
	ID                 string `xml:"cbc:ID"`
	CopyIndicator      bool   `xml:"cbc:CopyIndicator"`
	UUID               string `xml:"cbc:UUID"`
	ContractFolderID   string `xml:"cbc:ContractFolderID"`
	IssueDate          string `xml:"cbc:IssueDate"`
	IssueTime          string `xml:"cbc:IssueTime"`
	VersionID          string `xml:"cbc:VersionID"`

	ContractingParty      contractingParty      `xml:"cac:ContractingParty"`
	EconomicOperatorParty economicOperatorParty `xml:"cac:EconomicOperatorParty"`
	ProcurementProject    *procurementProject   `xml:"cac:ProcurementProject,omitempty"`
	Lots                  []lot                 `xml:"cac:ProcurementProjectLot,omitempty"`

	// Criteria are the vendored definitions, injected verbatim.
	Criteria  []rawXML   `xml:",omitempty"`
	Responses []response `xml:"cac:TenderingCriterionResponse,omitempty"`
}

// rawXML injects a pre-rendered element without re-encoding it, which is what
// keeps the vendored criterion definitions byte-for-byte what the EU published.
type rawXML struct {
	Value string `xml:",innerxml"`
	// XMLName is set to the element name the fragment already carries, so the
	// encoder writes the fragment and nothing around it.
	XMLName xml.Name
}

type contractingParty struct {
	Party party `xml:"cac:Party"`
}

type party struct {
	IndustryClassificationCode string            `xml:"cbc:IndustryClassificationCode,omitempty"`
	PartyIdentification        *partyID          `xml:"cac:PartyIdentification,omitempty"`
	PartyName                  *partyName        `xml:"cac:PartyName,omitempty"`
	PostalAddress              *postalAddress    `xml:"cac:PostalAddress,omitempty"`
	PowerOfAttorney            []powerOfAttorney `xml:"cac:PowerOfAttorney,omitempty"`
}

type partyID struct {
	ID string `xml:"cbc:ID"`
}

type partyName struct {
	Name string `xml:"cbc:Name"`
}

type postalAddress struct {
	Country *country `xml:"cac:Country,omitempty"`
}

type country struct {
	IdentificationCode string `xml:"cbc:IdentificationCode"`
}

type powerOfAttorney struct {
	Description string      `xml:"cbc:Description,omitempty"`
	AgentParty  *agentParty `xml:"cac:AgentParty,omitempty"`
}

type agentParty struct {
	Person person `xml:"cac:Person"`
}

type person struct {
	FirstName      string   `xml:"cbc:FirstName,omitempty"`
	FamilyName     string   `xml:"cbc:FamilyName,omitempty"`
	BirthDate      string   `xml:"cbc:BirthDate,omitempty"`
	BirthplaceName string   `xml:"cbc:BirthplaceName,omitempty"`
	Contact        *contact `xml:"cac:Contact,omitempty"`
}

type contact struct {
	ElectronicMail string `xml:"cbc:ElectronicMail,omitempty"`
}

type economicOperatorParty struct {
	Party party `xml:"cac:Party"`
}

type procurementProject struct {
	Name string `xml:"cbc:Name"`
}

type lot struct {
	ID string `xml:"cbc:ID"`
}

type response struct {
	ID                           string        `xml:"cbc:ID"`
	ValidatedCriterionPropertyID string        `xml:"cbc:ValidatedCriterionPropertyID"`
	ResponseValue                responseValue `xml:"cac:ResponseValue"`
}

type responseValue struct {
	ID                string `xml:"cbc:ID"`
	ResponseIndicator *bool  `xml:"cbc:ResponseIndicator,omitempty"`
	Description       string `xml:"cbc:Description,omitempty"`
	ResponseID        string `xml:"cbc:ResponseID,omitempty"`
}

// ── Building ────────────────────────────────────────────────────────────────

const (
	nsResponse = "urn:oasis:names:specification:ubl:schema:xsd:QualificationApplicationResponse-2"
	nsCAC      = "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
	nsCBC      = "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2"
)

func (s *Serializer) build(r espd.Response) (*document, error) {
	if err := s.p.Table.Validate(); err != nil {
		return nil, err
	}
	seed := documentSeed(r)
	doc := &document{
		XMLNS: nsResponse, CAC: nsCAC, CBC: nsCBC,
		UBLVersionID:       s.p.UBLVersionID,
		CustomizationID:    s.p.CustomizationID,
		ProfileID:          s.p.ProfileID,
		ProfileExecutionID: s.p.ProfileExecutionID,
		ID:                 "DGUE-" + seed[:10],
		CopyIndicator:      false,
		UUID:               deterministicUUID(seed, "document"),
		ContractFolderID:   leafText(r, espd.CritProcedure, "reference"),
		IssueDate:          r.ComposedAt.UTC().Format("2006-01-02"),
		IssueTime:          r.ComposedAt.UTC().Format("15:04:05Z"),
		VersionID:          s.p.VersionID,
	}

	if buyer := leafText(r, espd.CritProcedure, "buyer_name"); buyer != "" {
		doc.ContractingParty.Party.PartyName = &partyName{Name: buyer}
	}
	if title := leafText(r, espd.CritProcedure, "title"); title != "" {
		doc.ProcurementProject = &procurementProject{Name: title}
	}
	doc.EconomicOperatorParty.Party = s.operator(r)
	for _, l := range leaves(r, espd.CritLots, "lot_ref") {
		doc.Lots = append(doc.Lots, lot{ID: l.Value.String()})
	}

	if err := s.criteriaAndResponses(r, seed, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// operator builds Part II.A and II.B off the identity and representative
// leaves. Everything is read from LEAVES, never from the dossier: a fact that
// did not survive Compose's authoritative-only rule must not reappear here.
func (s *Serializer) operator(r espd.Response) party {
	p := party{}
	if name := leafText(r, espd.CritIdentity, "legal_name"); name != "" {
		p.PartyName = &partyName{Name: name}
	}
	if vat := leafText(r, espd.CritIdentity, "vat_number"); vat != "" {
		p.PartyIdentification = &partyID{ID: vat}
	}
	if c := leafText(r, espd.CritIdentity, "country"); c != "" {
		p.PostalAddress = &postalAddress{Country: &country{IdentificationCode: c}}
	}
	// The ESPD models SME status as an industry classification code, not as a
	// boolean: "SME" or "LARGE", and nothing when never answered.
	for _, l := range leaves(r, espd.CritIdentity, "is_sme") {
		if l.Value.Bool {
			p.IndustryClassificationCode = "SME"
		} else {
			p.IndustryClassificationCode = "LARGE"
		}
	}

	// Representatives: the leaves arrive field by field, grouped by the record
	// they came from, so they are regrouped by SourceRef here.
	for _, id := range representativeIDs(r) {
		fields := map[string]espd.Leaf{}
		for _, l := range r.Leaves {
			if l.Criterion == espd.CritRepresentatives && l.Source.ID == id {
				fields[l.Field] = l
			}
		}
		poa := powerOfAttorney{Description: fields["role"].Value.String()}
		per := person{
			FirstName:      fields["given_name"].Value.String(),
			FamilyName:     fields["family_name"].Value.String(),
			BirthplaceName: fields["birth_place"].Value.String(),
		}
		if bd, ok := fields["birth_date"]; ok {
			per.BirthDate = bd.Value.Date.Format("2006-01-02")
		}
		if mail, ok := fields["email"]; ok && mail.Value.String() != "" {
			per.Contact = &contact{ElectronicMail: mail.Value.String()}
		}
		poa.AgentParty = &agentParty{Person: per}
		p.PowerOfAttorney = append(p.PowerOfAttorney, poa)
	}
	return p
}

// representativeIDs lists the representative records in leaf order, without
// duplicates, so the output order follows the document rather than a map.
func representativeIDs(r espd.Response) []string {
	var out []string
	seen := map[string]bool{}
	for _, l := range r.Leaves {
		if l.Criterion == espd.CritRepresentatives && !seen[l.Source.ID] {
			seen[l.Source.ID] = true
			out = append(out, l.Source.ID)
		}
	}
	return out
}

// criteriaAndResponses emits, for every criterion the document covers, the
// vendored definition and the response to its answer property.
//
// A criterion the table cannot express is an ERROR, never an omission: a DGUE
// that looks complete while silently missing a ground the buyer asked about is
// worse than no DGUE at all.
func (s *Serializer) criteriaAndResponses(r espd.Response, seed string, doc *document) error {
	for _, k := range answeredCriteria(r) {
		crit, err := s.p.Table.Lookup(k)
		if err != nil {
			return err
		}
		def, err := s.p.Table.Definition(k)
		if err != nil {
			return err
		}
		doc.Criteria = append(doc.Criteria, rawXML{
			XMLName: xml.Name{Local: "cac:TenderingCriterion"},
			Value:   innerOf(def),
		})

		answers, err := s.answersFor(r, k, crit)
		if err != nil {
			return err
		}
		for _, a := range answers {
			doc.Responses = append(doc.Responses, response{
				ID:                           deterministicUUID(seed, "response:"+string(k)+":"+a.propertyID),
				ValidatedCriterionPropertyID: a.propertyID,
				ResponseValue: responseValue{
					ID:                deterministicUUID(seed, "value:"+string(k)+":"+a.propertyID),
					ResponseIndicator: a.indicator,
					Description:       a.description,
					ResponseID:        a.responseID,
				},
			})
		}
	}
	return nil
}

type answer struct {
	propertyID  string
	indicator   *bool
	description string
	responseID  string
}

// answersFor turns one criterion's state into the properties it answers.
func (s *Serializer) answersFor(r espd.Response, k espd.CriterionKey, crit codelist.Criterion) ([]answer, error) {
	// Part III.A–C: the declaration's own yes/no, plus self-cleaning.
	if espd.IsExclusionCriterion(k) {
		for _, a := range r.Declarations.Answers {
			if a.Criterion != k || !a.Answered {
				continue
			}
			applies := a.Applies
			out := []answer{{propertyID: crit.AnswerPropertyID, indicator: &applies}}
			if applies && strings.TrimSpace(a.SelfCleaning) != "" && crit.SelfCleaningIndicatorID != "" {
				taken := true
				out = append(out,
					answer{propertyID: crit.SelfCleaningIndicatorID, indicator: &taken},
					answer{propertyID: crit.SelfCleaningTextID, description: a.SelfCleaning},
				)
			}
			return out, nil
		}
		return nil, nil
	}

	// Part III.D: one yes/no for the whole national catalogue. It is true when
	// ANY national ground the operator declared applies — the conservative
	// reading, and the only one that cannot understate an exclusion.
	if k == espd.CritPurelyNationalGrounds {
		applies := false
		for _, l := range r.Leaves {
			if nationalGroundKey(l.Criterion) && l.Field == "answer" && l.Value.Bool {
				applies = true
			}
		}
		return []answer{{propertyID: crit.AnswerPropertyID, indicator: &applies}}, nil
	}

	// Everything else: the operator declares it holds what the criterion asks
	// about, which is exactly what a leaf under that criterion means — Compose
	// only produced one from an authoritative fact.
	switch crit.AnswerDataType {
	case "INDICATOR":
		yes := true
		return []answer{{propertyID: crit.AnswerPropertyID, indicator: &yes}}, nil
	case "DESCRIPTION":
		return []answer{{propertyID: crit.AnswerPropertyID, description: leafSummary(r, k)}}, nil
	default:
		// A data type this serializer has no rule for is a bug report, not a
		// value to invent.
		return nil, fmt.Errorf("%w: criterion %q answers with %s, which this serializer cannot fill",
			espd.ErrCriterionUnsupported, k, crit.AnswerDataType)
	}
}

// nationalGroundKey reports whether k is one of the per-country Part III.D
// answers, which Compose keys as "iii.d.<country>.<national code>".
//
// The ESPD has exactly ONE slot for these — CRITERION.EXCLUSION.NATIONAL.OTHER,
// a single yes/no — because national grounds are defined by national law and
// the EU model refuses to enumerate twenty-seven catalogues. So every national
// answer collapses onto that criterion here, and the PDF, which has no such
// constraint, prints them one by one.
func nationalGroundKey(k espd.CriterionKey) bool {
	return strings.HasPrefix(string(k), "iii.d.") && k != espd.CritPurelyNationalGrounds
}

// answeredCriteria lists the criteria the document actually covers, in a stable
// order: every answered Part III ground, plus every criterion a leaf sits
// under. Identity, representatives, the procedure and the lots are document
// STRUCTURE rather than criteria, so they are excluded.
func answeredCriteria(r espd.Response) []espd.CriterionKey {
	structural := map[espd.CriterionKey]bool{
		espd.CritIdentity: true, espd.CritRepresentatives: true,
		espd.CritProcedure: true, espd.CritLots: true,
	}
	seen := map[espd.CriterionKey]bool{}
	for _, a := range r.Declarations.Answers {
		if a.Answered {
			seen[a.Criterion] = true
		}
	}
	for _, l := range r.Leaves {
		switch {
		case structural[l.Criterion]:
		case nationalGroundKey(l.Criterion):
			seen[espd.CritPurelyNationalGrounds] = true
		default:
			seen[l.Criterion] = true
		}
	}
	out := make([]espd.CriterionKey, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func leaves(r espd.Response, k espd.CriterionKey, field string) []espd.Leaf {
	var out []espd.Leaf
	for _, l := range r.Leaves {
		if l.Criterion == k && l.Field == field {
			out = append(out, l)
		}
	}
	return out
}

func leafText(r espd.Response, k espd.CriterionKey, field string) string {
	if ls := leaves(r, k, field); len(ls) > 0 {
		return ls[0].Value.String()
	}
	return ""
}

// leafSummary renders every leaf of a criterion as "field: value" lines, for
// the few criteria whose answer property is free text.
func leafSummary(r espd.Response, k espd.CriterionKey) string {
	var lines []string
	for _, l := range r.Leaves {
		if l.Criterion == k {
			lines = append(lines, l.Field+": "+l.Value.String())
		}
	}
	return strings.Join(lines, "; ")
}

// innerOf strips the outer <cac:TenderingCriterion> wrapper the vendored
// fragment carries, since the encoder writes that element itself.
func innerOf(def string) string {
	const open, close = "<cac:TenderingCriterion>", "</cac:TenderingCriterion>"
	def = strings.TrimSpace(def)
	def = strings.TrimPrefix(def, open)
	return strings.TrimSuffix(def, close)
}

// documentSeed is the content the generated identifiers derive from: what the
// document says, not when it was written. Two exports of the same composed
// response are byte-identical; a changed answer produces different ids.
func documentSeed(r espd.Response) string {
	var b strings.Builder
	for _, l := range r.Leaves {
		fmt.Fprintf(&b, "%s|%s|%s\n", l.Criterion, l.Field, l.Value.String())
	}
	for _, a := range r.Declarations.Answers {
		fmt.Fprintf(&b, "%s|%t|%t|%s\n", a.Criterion, a.Answered, a.Applies, a.SelfCleaning)
	}
	sum := sha1.Sum([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// deterministicUUID derives an RFC 4122 version-5 style identifier from the
// document seed and a name. Version 5 is the name-based one, which is exactly
// what this is: the same document and the same slot always produce the same id.
func deterministicUUID(seed, name string) string {
	sum := sha1.Sum([]byte(seed + "\x00" + name))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50 // version 5
	b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// IssueTimeOf is exported for the tests that pin determinism: the clock only
// ever reaches the document through ComposedAt.
func IssueTimeOf(t time.Time) string { return t.UTC().Format("15:04:05Z") }
