package source_test

import (
	"testing"

	"github.com/bernardoforcillo/tendersbay-xyz/services/ingestion/internal/adapter/source"
)

func TestNewRegistryRegistersAllSources(t *testing.T) {
	got := source.NewRegistry()
	names := make([]string, len(got))
	for i, s := range got {
		names[i] = s.Name()
	}
	want := []string{"ted", "pl-bzp", "fr-boamp", "es-placsp"}
	if len(got) != len(want) {
		t.Fatalf("registry has %d sources, want %d (%v)", len(got), len(want), names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("names = %v, want %v", names, want)
		}
	}
}
