package source_test

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source"
)

func TestNewRegistryRegistersTED(t *testing.T) {
	got := source.NewRegistry()
	if len(got) != 1 {
		t.Fatalf("NewRegistry() = %d providers, want 1", len(got))
	}
	if got[0].Name() != "ted" {
		t.Errorf("provider[0].Name() = %q, want %q", got[0].Name(), "ted")
	}
}
