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
