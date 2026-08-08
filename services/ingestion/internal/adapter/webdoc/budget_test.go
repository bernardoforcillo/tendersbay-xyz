package webdoc

import "testing"

func TestBudgetStopsAtTheFileCap(t *testing.T) {
	var b Budget
	for i := range MaxFilesPerTender {
		if !b.Allow() {
			t.Fatalf("Allow = false at file %d, want the cap to be %d", i, MaxFilesPerTender)
		}
		b.Spend(1 << 10)
	}
	if b.Allow() {
		t.Errorf("Allow = true after %d files, want false", MaxFilesPerTender)
	}
	if b.Files() != MaxFilesPerTender {
		t.Errorf("Files = %d, want %d", b.Files(), MaxFilesPerTender)
	}
}

// TestBudgetStopsAtTheByteCap covers the other half. The two caps are not
// redundant: fifteen 20 MiB files would satisfy the file cap and still be
// 300 MiB pulled from one comune for one row.
func TestBudgetStopsAtTheByteCap(t *testing.T) {
	var b Budget
	if !b.Allow() {
		t.Fatal("Allow = false on an empty budget")
	}
	b.Spend(MaxTenderBytes)
	if b.Allow() {
		t.Error("Allow = true once the byte cap is reached, want false")
	}
	if b.Bytes() != MaxTenderBytes {
		t.Errorf("Bytes = %d, want %d", b.Bytes(), MaxTenderBytes)
	}
}

// TestBudgetChargesForFilesThatYieldedNothing is the rule that keeps the budget
// honest about what it is measuring. A file that downloaded and then failed to
// extract cost the server exactly as much as one that worked, and a budget that
// only counted successes would let a tender of scanned PDFs spend the cap
// several times over.
func TestBudgetChargesForFilesThatYieldedNothing(t *testing.T) {
	var b Budget
	b.Spend(0)
	b.Spend(0)
	if b.Files() != 2 {
		t.Errorf("Files = %d, want 2 — a file that yielded no text still cost a request", b.Files())
	}
	if b.Bytes() != 0 {
		t.Errorf("Bytes = %d, want 0", b.Bytes())
	}
}
