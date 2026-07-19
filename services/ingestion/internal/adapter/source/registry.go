// Package source assembles the registry of enabled tender providers — see
// doc.go for how to add one.
package source

import (
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/es/esplacsp"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/fr/frboamp"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/pl/plbzp"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/ted"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
)

// NewRegistry returns every registered provider: TED plus the three national
// below-threshold feeds (Poland BZP, France BOAMP, Spain PLACSP). Each is a
// platform client + protocol parser wired behind an ingestion.Source under
// source/<cc>/.
func NewRegistry() []ingestion.Source {
	return []ingestion.Source{ted.New(), plbzp.New(), frboamp.New(), esplacsp.New()}
}
