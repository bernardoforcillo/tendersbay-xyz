package source_test

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source"
)

func TestNewRegistryHasNoBuiltInProvidersYet(t *testing.T) {
	got := source.NewRegistry()
	if len(got) != 0 {
		t.Fatalf("NewRegistry() = %d providers, want 0 (no connectors yet)", len(got))
	}
}
