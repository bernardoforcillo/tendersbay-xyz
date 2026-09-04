package espd

import (
	"errors"
	"strings"
	"testing"
)

const sampleRequest = `<?xml version="1.0" encoding="UTF-8"?>
<QualificationApplicationRequest xmlns="urn:oasis:names:specification:ubl:schema:xsd:QualificationApplicationRequest-2"
  xmlns:cac="urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2"
  xmlns:cbc="urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2">
  <cbc:UBLVersionID>2.2</cbc:UBLVersionID>
  <cbc:CustomizationID>urn:www.cenbii.eu:transaction:biitrdm070:ver3.0</cbc:CustomizationID>
  <cbc:VersionID>%VERSION%</cbc:VersionID>
  <cbc:ContractFolderID>A0B1C2D3E4</cbc:ContractFolderID>
  <cac:AdditionalDocumentReference>
    <cbc:ID>2026/S 100-123456</cbc:ID>
    <cbc:DocumentTypeCode listID="ReferencesType">TED_CN</cbc:DocumentTypeCode>
  </cac:AdditionalDocumentReference>
  <cac:ContractingParty>
    <cac:Party>
      <cac:PartyName><cbc:Name>Comune di Milano</cbc:Name></cac:PartyName>
      <cac:PostalAddress><cac:Country><cbc:IdentificationCode listID="CountryCodeIdentifier">it</cbc:IdentificationCode></cac:Country></cac:PostalAddress>
    </cac:Party>
  </cac:ContractingParty>
  <cac:ProcurementProject><cbc:Name>Manutenzione strade 2027</cbc:Name></cac:ProcurementProject>
  <cac:ProcurementProjectLot><cbc:ID schemeID="Criterion">LOT-0001</cbc:ID></cac:ProcurementProjectLot>
  <cac:ProcurementProjectLot><cbc:ID schemeID="Criterion">LOT-0002</cbc:ID></cac:ProcurementProjectLot>
  <cac:TenderingCriterion>
    <cbc:ID schemeID="CriteriaID">005eb9ed-1347-4ca3-bb29-9bc0db64e1ab</cbc:ID>
    <cbc:CriterionTypeCode listID="CriteriaTaxonomy">CRITERION.EXCLUSION.CONVICTIONS.PARTICIPATION_IN_CRIMINAL_ORGANISATION</cbc:CriterionTypeCode>
  </cac:TenderingCriterion>
  <cac:TenderingCriterion>
    <cbc:ID schemeID="CriteriaID">499efc97-2ac1-4af2-9e84-323c2ca67747</cbc:ID>
    <cbc:CriterionTypeCode listID="CriteriaTaxonomy">criterion.selection.economic_financial_standing.turnover.general_yearly</cbc:CriterionTypeCode>
  </cac:TenderingCriterion>
  <cac:TenderingCriterion>
    <cbc:ID schemeID="CriteriaID">499efc97-2ac1-4af2-9e84-323c2ca67747</cbc:ID>
    <cbc:CriterionTypeCode listID="CriteriaTaxonomy">CRITERION.SELECTION.ECONOMIC_FINANCIAL_STANDING.TURNOVER.GENERAL_YEARLY</cbc:CriterionTypeCode>
  </cac:TenderingCriterion>
  <cac:TenderingCriterion>
    <cbc:ID schemeID="CriteriaID">9999</cbc:ID>
    <cbc:CriterionTypeCode listID="CriteriaTaxonomy">CRITERION.SELECTION.OTHER.SOMETHING_NEW</cbc:CriterionTypeCode>
  </cac:TenderingCriterion>
</QualificationApplicationRequest>`

func sample(version string) []byte {
	return []byte(strings.ReplaceAll(sampleRequest, "%VERSION%", version))
}

func TestParseRequestReadsPartIAndCriteria(t *testing.T) {
	req, err := ParseRequest(sample("2.1.1"))
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.Version != EDM211 {
		t.Errorf("Version = %s", req.Version)
	}
	if req.BuyerName != "Comune di Milano" || req.ProcedureTitle != "Manutenzione strade 2027" || req.ProcedureReference != "A0B1C2D3E4" {
		t.Errorf("Part I = %+v", req)
	}
	if req.Country != "IT" || req.NoticeRef != "2026/S 100-123456" {
		t.Errorf("country/notice = %q %q", req.Country, req.NoticeRef)
	}
	if len(req.Lots) != 2 || req.Lots[1] != "LOT-0002" {
		t.Errorf("Lots = %v", req.Lots)
	}
	// Two criteria mapped (case-insensitively, de-duplicated), one preserved as unmapped.
	if len(req.Criteria) != 2 || req.Criteria[0] != CritParticipationCriminalOrg || req.Criteria[1] != CritGeneralTurnover {
		t.Errorf("Criteria = %v", req.Criteria)
	}
	if len(req.UnmappedCriteria) != 1 || req.UnmappedCriteria[0] != "CRITERION.SELECTION.OTHER.SOMETHING_NEW" {
		t.Errorf("UnmappedCriteria = %v", req.UnmappedCriteria)
	}
	if len(req.SHA256) != 64 {
		t.Errorf("SHA256 = %q", req.SHA256)
	}
}

func TestParseRequestVersions(t *testing.T) {
	for v, want := range map[string]Version{"2.1.1": EDM211, "2.0.2": EDM211, "3.0.1": EDM4, "4.0.0": EDM4} {
		req, err := ParseRequest(sample(v))
		if err != nil || req.Version != want {
			t.Errorf("version %s: %v %v", v, req.Version, err)
		}
	}
	if _, err := ParseRequest(sample("1.0.2")); !errors.Is(err, ErrUnsupportedRequest) {
		t.Errorf("1.0.2 accepted: %v", err)
	}
	if _, err := ParseRequest(sample("")); !errors.Is(err, ErrUnsupportedRequest) {
		t.Errorf("missing VersionID accepted: %v", err)
	}
}

func TestParseRequestRejectsWhatItCannotRead(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want error
	}{
		"empty":      {"", ErrInvalidArgument},
		"not xml":    {"{}", ErrInvalidArgument},
		"edm 1.x":    {`<ESPDRequest xmlns="urn:grow:names:specification:ubl:schema:xsd:ESPDRequest-1"><VersionID>1.0.2</VersionID></ESPDRequest>`, ErrUnsupportedRequest},
		"other root": {`<QualificationApplicationResponse><VersionID>2.1.1</VersionID></QualificationApplicationResponse>`, ErrUnsupportedRequest},
		"oversized":  {"<a>" + strings.Repeat("x", maxRequestBytes) + "</a>", ErrInvalidArgument},
	}
	for name, tc := range cases {
		if _, err := ParseRequest([]byte(tc.raw)); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", name, err, tc.want)
		}
	}
}

func TestTaxonomyCoversEveryExclusionGround(t *testing.T) {
	covered := map[CriterionKey]bool{}
	for _, k := range taxonomy {
		covered[k] = true
	}
	for _, k := range ExclusionCriteria() {
		if !covered[k] {
			t.Errorf("exclusion ground %s has no taxonomy code", k)
		}
	}
}
