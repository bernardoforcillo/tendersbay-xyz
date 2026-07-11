// Package source assembles the registry of enabled tender providers — see
// doc.go for how to add one.
package source

import (
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source/ted"
	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"
)

// NewRegistry returns every registered provider.
func NewRegistry() []ingestion.Source {
	return []ingestion.Source{ted.New()}
}
