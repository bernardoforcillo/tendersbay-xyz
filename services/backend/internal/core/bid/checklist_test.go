package bid

import (
	"strings"
	"testing"
)

// The baseline codes are the canonical contract shared verbatim with the bid
// i18n namespace (P2.3) and the frontend renderer (P2.4): 14 items, sections
// part_ii/part_iii/part_iv/conclusion, and DOT-FREE item codes so the flat
// bid.checklist.items.* map resolves under i18next's '.' keySeparator.
func TestChecklistTemplate_BaselineStableCodes(t *testing.T) {
	seeds := checklistTemplate("", "")
	if len(seeds) != 14 {
		t.Fatalf("baseline must have 14 items, got %d", len(seeds))
	}
	seen := map[string]bool{}
	partIII := 0
	for i, s := range seeds {
		if s.SectionCode == "" || s.ItemCode == "" {
			t.Fatalf("seed %d has empty codes: %+v", i, s)
		}
		if strings.Contains(s.ItemCode, ".") {
			t.Fatalf("item_code %q must be a bare leaf (no '.') so the flat i18n map resolves", s.ItemCode)
		}
		if seen[s.ItemCode] {
			t.Fatalf("duplicate item_code %q (must be unique for the (bid_id,item_code) upsert)", s.ItemCode)
		}
		seen[s.ItemCode] = true
		if s.Position != i {
			t.Fatalf("seed %d position = %d, want %d (stable render order)", i, s.Position, i)
		}
		if s.SectionCode == "part_iii" {
			partIII++
		}
	}
	if partIII != 6 {
		t.Fatalf("part_iii must have 6 exclusion-ground items, got %d", partIII)
	}
	for _, section := range []string{"part_ii", "part_iii", "part_iv", "conclusion"} {
		found := false
		for _, s := range seeds {
			if s.SectionCode == section {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("baseline missing ESPD section %q", section)
		}
	}
}

func TestChecklistTemplate_FallbackEqualsBaseline(t *testing.T) {
	base := checklistTemplate("", "")
	unknown := checklistTemplate("weird-procedure", "99999999")
	if len(base) != len(unknown) {
		t.Fatalf("v1 fallback must equal baseline: %d vs %d", len(unknown), len(base))
	}
	for i := range base {
		if base[i] != unknown[i] {
			t.Fatalf("fallback seed %d differs: %+v vs %+v", i, unknown[i], base[i])
		}
	}
}
