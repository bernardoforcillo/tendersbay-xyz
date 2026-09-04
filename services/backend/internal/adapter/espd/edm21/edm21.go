// Package edm21 serializes an espd.Response as an ESPD-EDM 2.1.1
// QualificationApplicationResponse — the model the Italian eDGUE-IT
// specification binds to, and the one national platforms accept today.
//
// It is a Profile over internal/adapter/espd/edmxml plus the vendored 2.1.1
// code list. Everything version-specific lives in this file: the UBL release
// (2.2), the CEN-BII customization and profile identifiers, and the criterion
// table.
package edm21

import (
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/codelist"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/adapter/espd/edmxml"
	"github.com/bernardoforcillo/tendersbay-xyz/services/backend/internal/core/espd"
)

// New builds the 2.1.1 serializer.
func New() *edmxml.Serializer {
	return edmxml.New(edmxml.Profile{
		Version:      espd.EDM211,
		UBLVersionID: "2.2",
		VersionID:    "2.1.1",
		// 2.1.1 identifies its customization through CEN-BII rather than
		// through the single ProfileExecutionID 4.x introduced.
		CustomizationID: "urn:www.cenbii.eu:transaction:biitrdm092:ver3.0",
		ProfileID:       "4.1",
		SchemeAgencyID:  "EU-COM-GROW",
		Table:           codelist.EDM211(),
	})
}
