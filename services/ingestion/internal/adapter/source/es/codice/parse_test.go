package codice_test

import (
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/codice"
)

// fullFolder mirrors one real PLACSP ATOM entry's inline CODICE payload
// (namespaces cbc:/cac:/cbc-place-ext:/cac-place-ext:), trimmed to the
// elements the parser reads. It deliberately keeps:
//   - a ParentLocatedParty with a different <cbc:Name>, to prove the buyer
//     name is taken from the direct Party and never bleeds up the org tree;
//   - a DocumentAvailabilityPeriod with a different EndDate/EndTime, to prove
//     the deadline is the TenderSubmissionDeadlinePeriod and not that one;
//   - undeclared namespace prefixes (as they appear once lifted out of the
//     feed that declared them), to prove local-name matching still decodes.
const fullFolder = `<cac-place-ext:ContractFolderStatus>
  <cbc:ContractFolderID>104/2026</cbc:ContractFolderID>
  <cbc-place-ext:ContractFolderStatusCode listURI="x">EV</cbc-place-ext:ContractFolderStatusCode>
  <cac-place-ext:LocatedContractingParty>
    <cac:Party>
      <cac:PartyName>
        <cbc:Name>Alcaldía del Ayuntamiento de Navas del Madroño</cbc:Name>
      </cac:PartyName>
    </cac:Party>
    <cac-place-ext:ParentLocatedParty>
      <cac:PartyName>
        <cbc:Name>Navas del Madroño</cbc:Name>
      </cac:PartyName>
    </cac-place-ext:ParentLocatedParty>
  </cac-place-ext:LocatedContractingParty>
  <cac:ProcurementProject>
    <cbc:Name>Organización de los festejos taurinos en Navas del Madroño</cbc:Name>
    <cac:BudgetAmount>
      <cbc:EstimatedOverallContractAmount currencyID="EUR">70850</cbc:EstimatedOverallContractAmount>
      <cbc:TotalAmount currencyID="EUR">85728.5</cbc:TotalAmount>
      <cbc:TaxExclusiveAmount currencyID="EUR">70850</cbc:TaxExclusiveAmount>
    </cac:BudgetAmount>
    <cac:RequiredCommodityClassification>
      <cbc:ItemClassificationCode listURI="x">79954000</cbc:ItemClassificationCode>
    </cac:RequiredCommodityClassification>
    <cac:RealizedLocation>
      <cbc:CountrySubentity>Cáceres</cbc:CountrySubentity>
      <cbc:CountrySubentityCode listURI="x">ES432</cbc:CountrySubentityCode>
    </cac:RealizedLocation>
  </cac:ProcurementProject>
  <cac:TenderingProcess>
    <cac:DocumentAvailabilityPeriod>
      <cbc:EndDate>2026-05-01</cbc:EndDate>
      <cbc:EndTime>14:00:00</cbc:EndTime>
    </cac:DocumentAvailabilityPeriod>
    <cac:TenderSubmissionDeadlinePeriod>
      <cbc:EndDate>2026-05-13</cbc:EndDate>
      <cbc:EndTime>23:59:00</cbc:EndTime>
    </cac:TenderSubmissionDeadlinePeriod>
  </cac:TenderingProcess>
  <cac:LegalDocumentReference>
    <cbc:ID>PCAP Toros 2026 ok.pdf</cbc:ID>
    <cac:Attachment>
      <cac:ExternalReference>
        <cbc:URI>https://contrataciondelestado.es/FileSystem/servlet/GetDocumentByIdServlet?DocumentIdParam=abc</cbc:URI>
      </cac:ExternalReference>
    </cac:Attachment>
  </cac:LegalDocumentReference>
  <cac:TechnicalDocumentReference>
    <cbc:ID>PCAP Toros 2026.pdf</cbc:ID>
    <cac:Attachment>
      <cac:ExternalReference>
        <cbc:URI>https://contrataciondelestado.es/FileSystem/servlet/GetDocumentByIdServlet?DocumentIdParam=def</cbc:URI>
      </cac:ExternalReference>
    </cac:Attachment>
  </cac:TechnicalDocumentReference>
</cac-place-ext:ContractFolderStatus>`

func TestParse_FullFolder(t *testing.T) {
	d, err := codice.Parse([]byte(fullFolder))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.ContractFolderID != "104/2026" {
		t.Errorf("ContractFolderID = %q, want %q", d.ContractFolderID, "104/2026")
	}
	if d.StatusCode != "EV" {
		t.Errorf("StatusCode = %q, want %q", d.StatusCode, "EV")
	}
	if d.Title != "Organización de los festejos taurinos en Navas del Madroño" {
		t.Errorf("Title = %q", d.Title)
	}
	if d.BuyerName != "Alcaldía del Ayuntamiento de Navas del Madroño" {
		t.Errorf("BuyerName = %q, want the direct Party name (not the parent org)", d.BuyerName)
	}
	if len(d.CPV) != 1 || d.CPV[0] != "79954000" {
		t.Errorf("CPV = %v, want [79954000]", d.CPV)
	}
	if d.EstimatedValue == nil || *d.EstimatedValue != 7085000 {
		t.Errorf("EstimatedValue = %v, want 7085000 minor units (70850.00 EUR)", d.EstimatedValue)
	}
	if d.Currency != "EUR" {
		t.Errorf("Currency = %q, want EUR", d.Currency)
	}
	if d.NUTS != "ES432" {
		t.Errorf("NUTS = %q, want ES432", d.NUTS)
	}
	want := time.Date(2026, 5, 13, 23, 59, 0, 0, time.UTC)
	if d.SubmissionDeadline == nil || !d.SubmissionDeadline.Equal(want) {
		t.Errorf("SubmissionDeadline = %v, want %v (the submission deadline, not the doc-availability period)", d.SubmissionDeadline, want)
	}
	if len(d.Raw) == 0 {
		t.Error("Raw should carry the untouched payload")
	}
	wantDocs := []codice.DocumentRef{
		{Name: "PCAP Toros 2026 ok.pdf", Type: "legal", URI: "https://contrataciondelestado.es/FileSystem/servlet/GetDocumentByIdServlet?DocumentIdParam=abc"},
		{Name: "PCAP Toros 2026.pdf", Type: "technical", URI: "https://contrataciondelestado.es/FileSystem/servlet/GetDocumentByIdServlet?DocumentIdParam=def"},
	}
	if len(d.Documents) != len(wantDocs) {
		t.Fatalf("Documents = %+v, want %+v", d.Documents, wantDocs)
	}
	for i, want := range wantDocs {
		if d.Documents[i] != want {
			t.Errorf("Documents[%d] = %+v, want %+v", i, d.Documents[i], want)
		}
	}
}

func TestParse_NoDocumentReferences(t *testing.T) {
	const xml = `<cac-place-ext:ContractFolderStatus>
  <cbc:ContractFolderID>9/2026</cbc:ContractFolderID>
</cac-place-ext:ContractFolderStatus>`
	d, err := codice.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.Documents != nil {
		t.Errorf("Documents = %v, want nil when the folder carries no document references", d.Documents)
	}
}

func TestParse_MultipleCPV(t *testing.T) {
	const xml = `<cac-place-ext:ContractFolderStatus>
  <cbc:ContractFolderID>1275/2026</cbc:ContractFolderID>
  <cac:ProcurementProject>
    <cac:RequiredCommodityClassification><cbc:ItemClassificationCode>92310000</cbc:ItemClassificationCode></cac:RequiredCommodityClassification>
    <cac:RequiredCommodityClassification><cbc:ItemClassificationCode>79822500</cbc:ItemClassificationCode></cac:RequiredCommodityClassification>
    <cac:RequiredCommodityClassification><cbc:ItemClassificationCode>79950000</cbc:ItemClassificationCode></cac:RequiredCommodityClassification>
  </cac:ProcurementProject>
</cac-place-ext:ContractFolderStatus>`
	d, err := codice.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []string{"92310000", "79822500", "79950000"}
	if len(d.CPV) != len(want) {
		t.Fatalf("CPV = %v, want %v", d.CPV, want)
	}
	for i := range want {
		if d.CPV[i] != want[i] {
			t.Errorf("CPV[%d] = %q, want %q", i, d.CPV[i], want[i])
		}
	}
}

func TestParse_MissingOptionals(t *testing.T) {
	const xml = `<cac-place-ext:ContractFolderStatus>
  <cbc:ContractFolderID>9/2026</cbc:ContractFolderID>
  <cbc-place-ext:ContractFolderStatusCode>EV</cbc-place-ext:ContractFolderStatusCode>
</cac-place-ext:ContractFolderStatus>`
	d, err := codice.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.ContractFolderID != "9/2026" {
		t.Errorf("ContractFolderID = %q", d.ContractFolderID)
	}
	if d.EstimatedValue != nil {
		t.Errorf("EstimatedValue = %v, want nil when absent", d.EstimatedValue)
	}
	if d.SubmissionDeadline != nil {
		t.Errorf("SubmissionDeadline = %v, want nil when absent", d.SubmissionDeadline)
	}
	if d.CPV != nil {
		t.Errorf("CPV = %v, want nil when absent", d.CPV)
	}
	if d.NUTS != "" || d.BuyerName != "" || d.Currency != "" {
		t.Errorf("expected empty optional strings, got NUTS=%q Buyer=%q Currency=%q", d.NUTS, d.BuyerName, d.Currency)
	}
}

func TestParse_Malformed(t *testing.T) {
	if _, err := codice.Parse([]byte(`<ContractFolderStatus><cbc:ContractFolderID>oops`)); err == nil {
		t.Fatal("Parse: want error on malformed XML, got nil")
	}
}

// qualificationFolder carries the three child families PLACSP publishes under
// cac:TendererQualificationRequest, taken from the shape of a real feed entry.
// It deliberately keeps:
//   - two TechnicalEvaluationCriteria, to prove within-family published order
//     survives (the binding requirement is listed first);
//   - a FinancialEvaluationCriteria whose code is the bare "5", which is
//     meaningful only because its family says it is a financial capability
//     code — the reason Category exists;
//   - a SpecificTendererRequirement carrying a description that is only
//     whitespace, to prove an entry stating nothing is dropped rather than
//     counted;
//   - the families declared financial-before-technical, to prove the emitted
//     order is the fixed one collectSelectionCriteria documents and not the
//     document's.
const qualificationFolder = `<cac-place-ext:ContractFolderStatus>
  <cbc:ContractFolderID>QUAL/1</cbc:ContractFolderID>
  <cac:TenderingTerms>
    <cac:TendererQualificationRequest>
      <cac:FinancialEvaluationCriteria>
        <cbc:EvaluationCriteriaTypeCode listURI="x">5</cbc:EvaluationCriteriaTypeCode>
        <cbc:Description>Volumen anual de negocios igual o superior a 309.552,00 EUR.</cbc:Description>
      </cac:FinancialEvaluationCriteria>
      <cac:TechnicalEvaluationCriteria>
        <cbc:EvaluationCriteriaTypeCode listURI="x">OSR-COMPTASK</cbc:EvaluationCriteriaTypeCode>
        <cbc:Description>Relacion de los principales servicios realizados.</cbc:Description>
      </cac:TechnicalEvaluationCriteria>
      <cac:TechnicalEvaluationCriteria>
        <cbc:EvaluationCriteriaTypeCode listURI="x">OSR-TECH</cbc:EvaluationCriteriaTypeCode>
        <cbc:Description>Equipo minimo adscrito al contrato.</cbc:Description>
      </cac:TechnicalEvaluationCriteria>
      <cac:SpecificTendererRequirement>
        <cbc:RequirementTypeCode listURI="x">1</cbc:RequirementTypeCode>
        <cbc:Description>   </cbc:Description>
      </cac:SpecificTendererRequirement>
    </cac:TendererQualificationRequest>
  </cac:TenderingTerms>
</cac-place-ext:ContractFolderStatus>`

func TestParseSelectionCriteria(t *testing.T) {
	doc, err := codice.Parse([]byte(qualificationFolder))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []codice.SelectionRequirement{
		{Category: "technical", Code: "OSR-COMPTASK", Description: "Relacion de los principales servicios realizados."},
		{Category: "technical", Code: "OSR-TECH", Description: "Equipo minimo adscrito al contrato."},
		{Category: "financial", Code: "5", Description: "Volumen anual de negocios igual o superior a 309.552,00 EUR."},
	}
	if len(doc.SelectionCriteria) != len(want) {
		t.Fatalf("got %d criteria, want %d: %+v", len(doc.SelectionCriteria), len(want), doc.SelectionCriteria)
	}
	for i, w := range want {
		if got := doc.SelectionCriteria[i]; got != w {
			t.Errorf("criterion %d = %+v, want %+v", i, got, w)
		}
	}
}

// TestParseSelectionCriteriaAbsent pins the nil return: a folder publishing no
// qualification block must not yield an empty non-nil slice, so "published
// none" and "we did not look" stay distinguishable downstream.
func TestParseSelectionCriteriaAbsent(t *testing.T) {
	doc, err := codice.Parse([]byte(fullFolder))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if doc.SelectionCriteria != nil {
		t.Errorf("SelectionCriteria = %+v, want nil", doc.SelectionCriteria)
	}
}
