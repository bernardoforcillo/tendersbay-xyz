// Package source assembles the registry of enabled tender providers. It has
// no built-in connectors yet — see doc.go for how to add one.
package source

import "github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/core/ingestion"

// NewRegistry returns every registered provider. There are none yet; a real
// connector registers itself here once it exists.
func NewRegistry() []ingestion.Source {
	return nil
}
