// Package edm4 serializes an espd.Response as an ESPD-EDM 4.1.0
// QualificationApplicationResponse — the eForms-era line the EU service
// publishes.
//
// It is a Profile over internal/adapter/espd/edmxml plus the vendored 4.1.0
// code list. What differs from 2.1.1 is not the questions but their
// identifiers: UBL 2.3 instead of 2.2, one ProfileExecutionID instead of the
// CEN-BII customization pair, short criterion type codes ("fraud" for
// "CRITERION.EXCLUSION.CONVICTIONS.FRAUD"), and a different property UUID for
// every answer.
package edm4

import (
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/codelist"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/edmxml"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// New builds the 4.1.0 serializer.
func New() *edmxml.Serializer {
	return edmxml.New(edmxml.Profile{
		Version:            espd.EDM4,
		UBLVersionID:       "2.3",
		VersionID:          "4.1.0",
		ProfileExecutionID: "ESPD-EDMv4.1.0",
		SchemeAgencyID:     "OP",
		Table:              codelist.EDM4(),
	})
}
