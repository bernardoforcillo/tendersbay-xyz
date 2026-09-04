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
		k, ok := criterionForTypeCode(code)
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

// criterionForTypeCode maps the ESPD-EDM criterion taxonomy code
// (cbc:CriterionTypeCode, e.g. CRITERION.EXCLUSION.CONVICTIONS.FRAUD) to our
// key. The taxonomy is the one identifier stable across EDM 2.x and 4.x code
// lists — the UUIDs are not — which is why the request keeps type codes and
// the serializers keep UUIDs.
//
// Matching is on the code with its CRITERION. prefix stripped and upper-cased,
// so a buyer tool that emits a lower-case variant still maps. Anything not in
// the table is reported, not dropped.
func criterionForTypeCode(code string) (CriterionKey, bool) {
	c := strings.ToUpper(strings.TrimSpace(code))
	c = strings.TrimPrefix(c, "CRITERION.")
	k, ok := taxonomy[c]
	return k, ok
}

var taxonomy = map[string]CriterionKey{
	"EXCLUSION.CONVICTIONS.PARTICIPATION_IN_CRIMINAL_ORGANISATION":                      CritParticipationCriminalOrg,
	"EXCLUSION.CONVICTIONS.CORRUPTION":                                                  CritCorruption,
	"EXCLUSION.CONVICTIONS.FRAUD":                                                       CritFraud,
	"EXCLUSION.CONVICTIONS.TERRORIST_OFFENCES":                                          CritTerroristOffences,
	"EXCLUSION.CONVICTIONS.MONEY_LAUNDERING":                                            CritMoneyLaundering,
	"EXCLUSION.CONVICTIONS.CHILD_LABOUR-HUMAN_TRAFFICKING":                              CritChildLabour,
	"EXCLUSION.CONTRIBUTIONS.PAYMENT_OF_TAXES":                                          CritPaymentTaxes,
	"EXCLUSION.CONTRIBUTIONS.PAYMENT_OF_SOCIAL_SECURITY":                                CritPaymentSocialSecurity,
	"EXCLUSION.SOCIAL.ENVIRONMENTAL_LAW":                                                CritEnvironmentalLaw,
	"EXCLUSION.SOCIAL.SOCIAL_LAW":                                                       CritSocialLaw,
	"EXCLUSION.SOCIAL.LABOUR_LAW":                                                       CritLabourLaw,
	"EXCLUSION.BUSINESS.BANKRUPTCY":                                                     CritBankruptcy,
	"EXCLUSION.BUSINESS.INSOLVENCY":                                                     CritInsolvency,
	"EXCLUSION.BUSINESS.CREDITORS_ARRANGEMENT":                                          CritCreditorsArrangement,
	"EXCLUSION.BUSINESS.ANALOGOUS_SITUATION":                                            CritAnalogousSituation,
	"EXCLUSION.BUSINESS.ASSETS_ADMINISTERED_BY_LIQUIDATOR":                              CritAssetsByLiquidator,
	"EXCLUSION.BUSINESS.BUSINESS_ACTIVITIES_SUSPENDED":                                  CritActivitiesSuspended,
	"EXCLUSION.MISCONDUCT.MC_PROFESSIONAL":                                              CritProfessionalMisconduct,
	"EXCLUSION.MISCONDUCT.MARKET_DISTORTION":                                            CritDistortingCompetition,
	"EXCLUSION.CONFLICT_OF_INTEREST.PROCEDURE_PARTICIPATION":                            CritConflictOfInterest,
	"EXCLUSION.CONFLICT_OF_INTEREST.PROCUREMENT_PROCEDURE_PREPARATION":                  CritInvolvementInPreparation,
	"EXCLUSION.CONFLICT_OF_INTEREST.EARLY_TERMINATION":                                  CritEarlyTermination,
	"EXCLUSION.CONFLICT_OF_INTEREST.MISINTERPRETATION":                                  CritMisrepresentation,
	"EXCLUSION.NATIONAL.OTHER":                                                          CritPurelyNationalGrounds,
	"SELECTION.SUITABILITY.PROFESSIONAL_REGISTER_ENROLMENT":                             CritEnrolmentProfessionalReg,
	"SELECTION.SUITABILITY.TRADE_REGISTER_ENROLMENT":                                    CritEnrolmentTradeReg,
	"SELECTION.ECONOMIC_FINANCIAL_STANDING.TURNOVER.GENERAL_YEARLY":                     CritGeneralTurnover,
	"SELECTION.ECONOMIC_FINANCIAL_STANDING.TURNOVER.SPECIFIC_YEARLY":                    CritSpecificTurnover,
	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.REFERENCES.WORKS_PERFORMANCE":             CritReferences,
	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.REFERENCES.SUPPLIES_DELIVERY_PERFORMANCE": CritReferences,
	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.REFERENCES.SERVICES_DELIVERY_PERFORMANCE": CritReferences,
	"SELECTION.TECHNICAL_PROFESSIONAL_ABILITY.MANAGEMENT.AVERAGE_ANNUAL_MANPOWER":       CritAverageAnnualManpower,
	"SELECTION.QUALITY_ASSURANCE.CERTIFICATE_INDEPENDENT_BODIES_ABOUT_QA":               CritQualityAssurance,
	"SELECTION.QUALITY_ASSURANCE.CERTIFICATE_INDEPENDENT_BODIES_ABOUT_ENVIRONMENTAL":    CritEnvironmentalManagement,
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
