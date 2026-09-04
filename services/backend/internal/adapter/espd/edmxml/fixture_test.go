package edmxml_test

import (
	"testing"
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// The fixture is a complete, ready PMI dossier composed against a bid, so the
// serializers are exercised on the document they will actually produce rather
// than on a hand-written Response that could drift from what Compose emits.

var (
	fixedNow = time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC)
	statedAt = fixedNow.Add(-72 * time.Hour)
	testUser = "11111111-1111-1111-1111-111111111111"
)

func ptr[T any](v T) *T { return &v }

func human() company.Attribution {
	return company.Attribution{
		Provenance: company.ProvenanceUserStated, StatedBy: testUser,
		StatedAt: statedAt, PromptedBy: company.PromptOnboarding,
	}
}

// readyDossier is an Italian construction PMI with every Part answered.
func readyDossier(t *testing.T) company.Dossier {
	t.Helper()
	attr := map[company.FieldKey]company.Attribution{}
	for _, k := range []company.FieldKey{
		company.FieldLegalName, company.FieldVATNumber, company.FieldFiscalCode,
		company.FieldLegalForm, company.FieldCCIAA, company.FieldCountry,
		company.FieldNUTS, company.FieldIsSME,
	} {
		attr[k] = human()
	}
	d := company.Dossier{
		WorkspaceID: "ws-1",
		Identity: company.Identity{
			LegalName: "Acme Costruzioni Srl", VATNumber: "IT01234567890",
			FiscalCode: "01234567890", LegalForm: company.LegalForm("srl"),
			CCIAAOffice: "MI", CCIAANumber: "1234567",
			Country: "IT", NUTS: "ITC4C", IsSME: true, Attribution: attr,
		},
		Representatives: []company.Representative{{
			ID: "rep-1", Role: "legale_rappresentante", GivenName: "Anna", FamilyName: "Rossi",
			BirthDate:  ptr(time.Date(1974, 3, 12, 0, 0, 0, 0, time.UTC)),
			BirthPlace: "Milano", Email: "anna.rossi@acme.example", Attribution: human(),
		}},
		FinancialYears: []company.FinancialYear{{
			Year: 2025, TurnoverMinor: ptr(int64(2_100_000_00)), Currency: "EUR",
			Headcount: ptr(int32(18)), Attribution: human(),
		}},
		PastContracts: []company.PastContract{{
			ID: "pc-1", Description: "Rifacimento copertura scuola primaria",
			BuyerName: "Comune di Lodi", ValueMinor: ptr(int64(450_000_00)), Currency: "EUR",
			Role: company.ContractRole("principal"), Attribution: human(),
		}},
		SOA: []company.SOACategory{{
			ID: "soa-1", Category: "OG1", Classifica: company.ClassificaIII, Attribution: human(),
		}},
		Certifications: []company.Certification{
			{ID: "cert-1", Standard: company.CertISO9001, Scope: "edilizia", Attribution: human()},
			{ID: "cert-2", Standard: company.CertISO14001, Attribution: human()},
		},
		Registrations: []company.Registration{{
			ID: "reg-1", Kind: company.RegWhiteList, Authority: "Prefettura di Milano", Attribution: human(),
		}},
		NationalGrounds: []company.NationalGround{{
			ID: "ng-1", Country: "IT", Criterion: "art94.c1", Answer: false, Attribution: human(),
		}},
	}
	for _, k := range espd.ExclusionCriteria() {
		d.Declarations = append(d.Declarations, company.Declaration{
			ID: "dec-" + string(k), Criterion: string(k), Answer: false, Attribution: human(),
		})
	}
	return d
}

func readyInput(d company.Dossier) espd.BidInput {
	return espd.BidInput{
		Bid: bid.Bid{ID: "bid-1", WorkbenchID: "wb-1", TenderID: 42},
		Data: bid.EspdData{
			Lots: []bid.Lot{{ID: "lot-1", LotRef: "LOT-0001", Position: 1}},
			Subcontractors: []bid.Subcontractor{
				{ID: "sub-1", Name: "Beta Impianti Srl", VAT: "IT09876543210", Country: "IT", Share: ptr(int32(20))},
			},
			Reliances: []bid.Reliance{
				{ID: "rel-1", EntityName: "Gamma Spa", VAT: "IT05555555555", Criterion: string(espd.CritGeneralTurnover)},
			},
		},
		Confirmation: &bid.DeclarationConfirmation{
			BidID: "bid-1", UserID: testUser, ConfirmedAt: fixedNow.Add(-time.Hour),
			DeclarationsHash: espd.HashDeclarations(d.Declarations),
		},
		Procedure: espd.Procedure{
			BuyerName: "Comune di Milano", Title: "Manutenzione strade 2027",
			Reference: "A0B1C2D3E4", NoticeRef: "2026/S 100-123456", Country: "IT",
		},
	}
}

// readyResponse composes the fixture. It asserts readiness, because a fixture
// that quietly stopped being ready would turn every serializer test into a test
// of the empty document.
func readyResponse(t *testing.T) espd.Response {
	t.Helper()
	d := readyDossier(t)
	r := espd.Compose(d, readyInput(d), nil, fixedNow)
	if !r.Ready() {
		t.Fatalf("fixture is not ready; gaps = %+v", r.Gaps)
	}
	return r
}
