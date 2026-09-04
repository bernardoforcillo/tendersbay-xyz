package espd_test

import (
	"time"

	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/bid"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/company"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// The service tests share one fixture: an Italian construction PMI whose
// dossier is complete, so every test that is not ABOUT incompleteness starts
// from a document that would export.

var now = time.Date(2026, 9, 4, 10, 30, 0, 0, time.UTC)

func p[T any](v T) *T { return &v }

func stated() company.Attribution {
	return company.Attribution{
		Provenance: company.ProvenanceUserStated, StatedBy: testUser,
		StatedAt: now.Add(-72 * time.Hour), PromptedBy: company.PromptOnboarding,
	}
}

func readyDossier() company.Dossier {
	attr := map[company.FieldKey]company.Attribution{}
	for _, k := range []company.FieldKey{
		company.FieldLegalName, company.FieldVATNumber, company.FieldFiscalCode,
		company.FieldLegalForm, company.FieldCCIAA, company.FieldCountry,
		company.FieldNUTS, company.FieldIsSME,
	} {
		attr[k] = stated()
	}
	d := company.Dossier{
		Identity: company.Identity{
			LegalName: "Acme Costruzioni Srl", VATNumber: "IT01234567890",
			FiscalCode: "01234567890", LegalForm: company.LegalForm("srl"),
			CCIAAOffice: "MI", CCIAANumber: "1234567",
			Country: "IT", NUTS: "ITC4C", IsSME: true, Attribution: attr,
		},
		Representatives: []company.Representative{{
			ID: "rep-1", Role: "legale_rappresentante", GivenName: "Anna",
			FamilyName: "Rossi", Attribution: stated(),
		}},
		FinancialYears: []company.FinancialYear{{
			Year: 2025, TurnoverMinor: p(int64(2_100_000_00)), Currency: "EUR",
			Headcount: p(int32(18)), Attribution: stated(),
		}},
		SOA: []company.SOACategory{{
			ID: "soa-1", Category: "OG1", Classifica: company.ClassificaIII, Attribution: stated(),
		}},
	}
	for _, k := range espd.ExclusionCriteria() {
		d.Declarations = append(d.Declarations, company.Declaration{
			ID: "dec-" + string(k), Criterion: string(k), Answer: false, Attribution: stated(),
		})
	}
	return d
}

func readyData() bid.EspdData {
	return bid.EspdData{
		Lots: []bid.Lot{{ID: "lot-1", BidID: testBidID, LotRef: "LOT-0001", Position: 1}},
	}
}
