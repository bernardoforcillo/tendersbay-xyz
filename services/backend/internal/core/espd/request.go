package espd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

// Request is what this package keeps of a buyer's ESPD request: the Part I
// facts, the lot structure and the criteria the buyer asks for, mapped to our
// keys where the taxonomy allows and preserved verbatim where it does not.
type Request struct {
	Version            Version
	BuyerName          string
	ProcedureTitle     string
	ProcedureReference string // ContractFolderID — the CIG on Italian procedures
	NoticeRef          string // TED publication number when the request cites one
	Country            string // alpha-2 of the contracting authority
	Lots               []string
	Criteria           []CriterionKey
	// UnmappedCriteria are the buyer's criterion type codes our taxonomy has no
	// key for. They are kept so an unmapped criterion is visible in the
	// preview rather than silently absent from the document.
	UnmappedCriteria []string
	SHA256           string // of the raw XML, the identity of this import
	ImportedBy       string
	ImportedAt       time.Time
}

// maxRequestBytes bounds what ParseRequest will read. A real request is tens
// of kilobytes; a megabyte is already an anomaly and 8 MiB is the ceiling
// before the parser is a denial-of-service surface.
const maxRequestBytes = 8 << 20

// ParseRequest reads an ESPD-EDM 2.x or 3.x/4.x QualificationApplicationRequest.
// It is namespace-tolerant (element local names only), which is what lets the
// same reader serve both generations of the schema: the elements it needs have
// kept their names across UBL 2.2 and 2.3.
//
// An EDM 1.x ESPDRequest — the pre-2018 format — is refused with
// ErrUnsupportedRequest rather than half-read.
func ParseRequest(raw []byte) (Request, error) {
	if len(raw) == 0 {
		return Request{}, fmt.Errorf("%w: empty request", ErrInvalidArgument)
	}
	if len(raw) > maxRequestBytes {
		return Request{}, fmt.Errorf("%w: request exceeds %d bytes", ErrInvalidArgument, maxRequestBytes)
	}

	var doc qualificationApplicationRequest
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	if err := dec.Decode(&doc); err != nil {
		return Request{}, fmt.Errorf("%w: %v", ErrInvalidArgument, err)
	}
	switch doc.XMLName.Local {
	case "QualificationApplicationRequest":
	case "ESPDRequest":
		return Request{}, fmt.Errorf("%w: ESPD-EDM 1.x requests are not supported; ask the buyer for a 2.x or later export", ErrUnsupportedRequest)
	default:
		return Request{}, fmt.Errorf("%w: root element %q is not a QualificationApplicationRequest", ErrUnsupportedRequest, doc.XMLName.Local)
	}

	version, err := versionOf(doc.VersionID)
	if err != nil {
		return Request{}, err
	}
	sum := sha256.Sum256(raw)
	req := Request{
		Version:            version,
		BuyerName:          strings.TrimSpace(doc.ContractingParty.Party.PartyName.Name),
		ProcedureTitle:     strings.TrimSpace(doc.ProcurementProject.Name),
		ProcedureReference: strings.TrimSpace(doc.ContractFolderID),
		Country:            strings.ToUpper(strings.TrimSpace(doc.ContractingParty.Party.PostalAddress.Country.IdentificationCode)),
		SHA256:             hex.EncodeToString(sum[:]),
	}
	for _, ref := range doc.AdditionalDocumentReference {
		if strings.EqualFold(ref.DocumentTypeCode, "TED_CN") || strings.Contains(strings.ToUpper(ref.DocumentTypeCode), "TED") {
			req.NoticeRef = strings.TrimSpace(ref.ID)
			break
		}
	}
	for _, lot := range doc.ProcurementProjectLot {
		if id := strings.TrimSpace(lot.ID); id != "" {
			req.Lots = append(req.Lots, id)
		}
	}
	seen := map[CriterionKey]bool{}
	for _, crit := range doc.TenderingCriterion {
		code := strings.TrimSpace(crit.CriterionTypeCode)
		if code == "" {
			continue
		}
		k, ok := CriterionForTypeCode(code)
		if !ok {
			req.UnmappedCriteria = append(req.UnmappedCriteria, code)
			continue
		}
		if !seen[k] {
			seen[k] = true
			req.Criteria = append(req.Criteria, k)
		}
	}
	return req, nil
}

func versionOf(versionID string) (Version, error) {
	v := strings.TrimSpace(versionID)
	switch {
	case strings.HasPrefix(v, "2."):
		return EDM211, nil
	case strings.HasPrefix(v, "3."), strings.HasPrefix(v, "4."):
		return EDM4, nil
	case v == "":
		return "", fmt.Errorf("%w: request carries no VersionID", ErrUnsupportedRequest)
	}
	return "", fmt.Errorf("%w: ESPD-EDM version %q", ErrUnsupportedRequest, v)
}

// CriterionForTypeCode maps a buyer's cbc:CriterionTypeCode onto our key.
//
// BOTH generations are recognised, because a request can arrive in either and a
// criterion this parser does not recognise is a criterion the preview cannot
// tell the operator about:
//
//   - ESPD-EDM 2.x spells the taxonomy out —
//     "CRITERION.EXCLUSION.CONVICTIONS.FRAUD";
//   - 4.x shortened every one of them — "fraud".
//
// Matching ignores case and the optional CRITERION. prefix, so a tool that
// emits a lower-case variant still maps. Anything absent from both tables is
// REPORTED as unmapped, never dropped: a buyer asking for something we do not
// model has to be visible.
//
// It is exported because the codelist package cross-checks it against the
// vendored tables — the tables and this map are two transcriptions of one
// taxonomy, and only a test can keep them honest.
func CriterionForTypeCode(code string) (CriterionKey, bool) {
	c := strings.TrimSpace(code)
	if k, ok := shortTaxonomy[strings.ToLower(c)]; ok {
		return k, true
	}
	upper := strings.TrimPrefix(strings.ToUpper(c), "CRITERION.")
	k, ok := taxonomy[upper]
	return k, ok
}

// taxonomy is the ESPD-EDM 2.x criteria taxonomy, transcribed from
// ESPD-CriteriaTaxonomy_V2.1.1.gc in the official release. The codes are not
// guessable — the release spells "analogous situation" as BANKRUPTCY_ANALOGOUS
// and files the quality-assurance certificates under
// TECHNICAL_PROFESSIONAL_ABILITY rather than under QUALITY_ASSURANCE — so this
// table is checked against the vendored code list by a test in
// internal/adapter/espd/codelist.
var taxonomy = map[string]CriterionKey{
	"EXCLUSION.CONVICTIONS.PARTICIPATION_IN_CRIMINAL_ORGANISATION": CritParticipationCriminalOrg,
	"EXCLUSION.CONVICTIONS.CORRUPTION":                             CritCorruption,
	"EXCLUSION.CONVICTIONS.FRAUD":                                  CritFraud,
	"EXCLUSION.CONVICTIONS.TERRORIST_OFFENCES":                     CritTerroristOffences,
	"EXCLUSION.CONVICTIONS.MONEY_LAUNDERING":                       CritMoneyLaundering,
	"EXCLUSION.CONVICTIONS.CHILD_LABOUR-HUMAN_TRAFFICKING":         CritChildLabour,
	"EXCLUSION.CONTRIBUTIONS.PAYMENT_OF_TAXES":                     CritPaymentTaxes,
	"EXCLUSION.CONTRIBUTIONS.PAYMENT_OF_SOCIAL_SECURITY":           CritPaymentSocialSecurity,
	"EXCLUSION.SOCIAL.ENVIRONMENTAL_LAW":                           CritEnvironmentalLaw,
	"EXCLUSION.SOCIAL.SOCIAL_LAW":                                  CritSocialLaw,
	"EXCLUSION.SOCIAL.LABOUR_LAW":                                  CritLabourLaw,
	"EXCLUSION.BUSINESS.BANKRUPTCY":                                CritBankruptcy,
	"EXCLUSION.BUSINESS.INSOLVENCY":                                CritInsolvency,
	"EXCLUSION.BUSINESS.CREDITORS_ARRANGEMENT":                     CritCreditorsArrangement,
	"EXCLUSION.BUSINESS.BANKRUPTCY_ANALOGOUS":                      CritAnalogousSituation,
	"EXCLUSION.BUSINESS.LIQUIDATOR_ADMINISTERED":                   CritAssetsByLiquidator,
	"EXCLUSION.BUSINESS.ACTIVITIES_SUSPENDED":                      CritActivitiesSuspended,
	"EXCLUSION.MISCONDUCT.MC_PROFESSIONAL":                         CritProfessionalMisconduct,
	"EXCLUSION.MISCONDUCT.MARKET_DISTORTION":                       CritDistortingCompetition,
	"EXCLUSION.CONFLICT_OF_INTEREST.PROCEDURE_PARTICIPATION":       CritConflictOfInterest,
	"EXCLUSION.CONFLICT_OF_INTEREST.PROCEDURE_PREPARATION":         CritInvolvementInPreparation,
	"EXCLUSION.CONFLICT_OF_INTEREST.EARLY_TERMINATION":             CritEarlyTermination,
	"EXCLUSION.CONFLICT_OF_INTEREST.MISINTERPRETATION":             CritMisrepresentation,
	"EXCLUSION.NATIONAL.OTHER":                                     CritPurelyNationalGrounds,

	"SELECTION.SUITABILITY.PROFESSIONAL_REGISTER_ENROLMENT": CritEnrolmentProfessionalReg,
	"SELECTION.SUITABILITY.TRADE_REGISTER_ENROLMENT":        CritEnrolmentTradeReg,
	"SELECTION.SUITABILITY.AUTHORISATION":                   CritOtherRegistration,

	"SELECTION.ECONOMIC_FINANCIAL_STANDING.TURNOVER.GENERAL_YEARLY":  CritGeneralTurnover,
	"SELECTION.ECONOMIC_FINANCIAL_STANDING.TURNOVER.SPECIFIC_YEARLY": CritSpecificTurnover,

	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.REFERENCES.WORKS_PERFORMANCE":             CritReferences,
	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.REFERENCES.SUPPLIES_DELIVERY_PERFORMANCE": CritReferences,
	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.REFERENCES.SERVICES_DELIVERY_PERFORMANCE": CritReferences,
	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.MANAGEMENT.AVERAGE_ANNUAL_MANPOWER":       CritAverageAnnualManpower,

	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.CERTIFICATES.QUALITY_ASSURANCE.QA_INDEPENDENT_CERTIFICATE":         CritQualityAssurance,
	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.CERTIFICATES.ENVIRONMENTAL_MANAGEMENT.ENV_INDEPENDENT_CERTIFICATE": CritEnvironmentalManagement,

	"OTHER.EO_DATA.REGISTERED_IN_OFFICIAL_LIST":     CritSOAAttestation,
	"OTHER.EO_DATA.RELIES_ON_OTHER_CAPACITIES":      CritReliance,
	"OTHER.EO_DATA.SUBCONTRACTS_WITH_THIRD_PARTIES": CritSubcontracting,
	"OTHER.EO_DATA.LOTS_TENDERED":                   CritLots,
}

// shortTaxonomy is the same set of questions as 4.1.0 spells them. The codes
// are lower-case in the release, and matched lower-cased here.
var shortTaxonomy = map[string]CriterionKey{
	"crime-org":       CritParticipationCriminalOrg,
	"corruption":      CritCorruption,
	"fraud":           CritFraud,
	"terr-offence":    CritTerroristOffences,
	"finan-laund":     CritMoneyLaundering,
	"human-traffic":   CritChildLabour,
	"tax-pay":         CritPaymentTaxes,
	"socsec-pay":      CritPaymentSocialSecurity,
	"envir-law":       CritEnvironmentalLaw,
	"socsec-law":      CritSocialLaw,
	"labour-law":      CritLabourLaw,
	"bankruptcy":      CritBankruptcy,
	"insolvency":      CritInsolvency,
	"cred-arran":      CritCreditorsArrangement,
	"bankr-nat":       CritAnalogousSituation,
	"liq-admin":       CritAssetsByLiquidator,
	"susp-act":        CritActivitiesSuspended,
	"prof-misconduct": CritProfessionalMisconduct,
	"distorsion":      CritDistortingCompetition,
	"partic-confl":    CritConflictOfInterest,
	"prep-confl":      CritInvolvementInPreparation,
	"sanction":        CritEarlyTermination,
	"misrepresent":    CritMisrepresentation,
	"nati-ground":     CritPurelyNationalGrounds,

	"prof-regist":   CritEnrolmentProfessionalReg,
	"trade-regist":  CritEnrolmentTradeReg,
	"authorisation": CritOtherRegistration,

	"gen-year-to":  CritGeneralTurnover,
	"spec-year-to": CritSpecificTurnover,

	"work-perform":    CritReferences,
	"supply-perform":  CritReferences,
	"service-perform": CritReferences,
	"year-manpower":   CritAverageAnnualManpower,

	"qu-certif-indep":    CritQualityAssurance,
	"envir-certif-indep": CritEnvironmentalManagement,

	"registered": CritSOAAttestation,
	"relied":     CritReliance,
	"subco-ent":  CritSubcontracting,
}

// ── XML shape (local names only; namespaces deliberately ignored) ───────────

type qualificationApplicationRequest struct {
	XMLName                     xml.Name
	VersionID                   string               `xml:"VersionID"`
	ContractFolderID            string               `xml:"ContractFolderID"`
	ContractingParty            contractingParty     `xml:"ContractingParty"`
	ProcurementProject          procurementProject   `xml:"ProcurementProject"`
	ProcurementProjectLot       []procurementLot     `xml:"ProcurementProjectLot"`
	TenderingCriterion          []tenderingCriterion `xml:"TenderingCriterion"`
	AdditionalDocumentReference []documentReference  `xml:"AdditionalDocumentReference"`
}

type contractingParty struct {
	Party struct {
		PartyName struct {
			Name string `xml:"Name"`
		} `xml:"PartyName"`
		PostalAddress struct {
			Country struct {
				IdentificationCode string `xml:"IdentificationCode"`
			} `xml:"Country"`
		} `xml:"PostalAddress"`
	} `xml:"Party"`
}

type procurementProject struct {
	Name string `xml:"Name"`
}

type procurementLot struct {
	ID string `xml:"ID"`
}

type tenderingCriterion struct {
	ID                string `xml:"ID"`
	CriterionTypeCode string `xml:"CriterionTypeCode"`
}

type documentReference struct {
	ID               string `xml:"ID"`
	DocumentTypeCode string `xml:"DocumentTypeCode"`
}
