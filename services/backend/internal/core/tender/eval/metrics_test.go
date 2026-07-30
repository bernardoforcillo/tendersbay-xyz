package eval

import (
	"math"
	"testing"
)

// close reports whether got is within 1e-9 of want. Metric values are ratios of
// small integers, so an exact float comparison would fail on representation
// rather than on behaviour.
func close(got, want float64) bool { return math.Abs(got-want) < 1e-9 }

func TestRecallAtK_CountsOnlyGradedHitsInsideTheWindow(t *testing.T) {
	j := Judgements{"ted:a": 2, "ted:b": 1, "ted:c": 2}
	ranked := []string{"ted:x", "ted:a", "ted:y", "ted:b"}

	// 2 of the 3 relevant tenders appear in the top 4.
	if got := RecallAtK(ranked, j, 4); !close(got, 2.0/3.0) {
		t.Errorf("RecallAtK(k=4) = %v, want 2/3", got)
	}
	// Narrowing the window must drop the hit that fell outside it.
	if got := RecallAtK(ranked, j, 2); !close(got, 1.0/3.0) {
		t.Errorf("RecallAtK(k=2) = %v, want 1/3", got)
	}
}

func TestRecallAtK_NoGradedTendersIsZeroNotNaN(t *testing.T) {
	// A query whose judgements are all 0 has no attainable recall. Returning
	// NaN would poison every average that includes it, so it must be 0.
	if got := RecallAtK([]string{"ted:a"}, Judgements{"ted:a": 0}, 10); got != 0 {
		t.Errorf("RecallAtK = %v, want 0 for a query with nothing relevant", got)
	}
}

func TestNDCGAtK_PerfectOrderIsOne(t *testing.T) {
	j := Judgements{"ted:a": 2, "ted:b": 1}
	if got := NDCGAtK([]string{"ted:a", "ted:b"}, j, 10); !close(got, 1) {
		t.Errorf("NDCGAtK = %v, want 1 for the ideal ordering", got)
	}
}

func TestNDCGAtK_PenalisesTheWrongOrderByDiscountNotByCount(t *testing.T) {
	j := Judgements{"ted:a": 2, "ted:b": 1}
	// Swapped: DCG = 1/log2(2) + 3/log2(3) = 1 + 1.8927892607…
	// Ideal:   DCG = 3/log2(2) + 1/log2(3) = 3 + 0.6309297535…
	want := (1 + 3/math.Log2(3)) / (3 + 1/math.Log2(3))
	if got := NDCGAtK([]string{"ted:b", "ted:a"}, j, 10); !close(got, want) {
		t.Errorf("NDCGAtK = %v, want %v — the same hits in the wrong order must score lower", got, want)
	}
}

func TestNDCGAtK_IdealIsBoundedByK(t *testing.T) {
	// Three relevant tenders but a window of 1: the ideal DCG must also be
	// truncated to 1, or a query with more relevant results than the window
	// could never reach 1.0 however well it ranked.
	j := Judgements{"ted:a": 2, "ted:b": 2, "ted:c": 2}
	if got := NDCGAtK([]string{"ted:a"}, j, 1); !close(got, 1) {
		t.Errorf("NDCGAtK(k=1) = %v, want 1 — the ideal must be truncated to k too", got)
	}
}

func TestMRR_UsesTheFirstRelevantRankOnly(t *testing.T) {
	j := Judgements{"ted:b": 1, "ted:c": 2}
	if got := MRR([]string{"ted:a", "ted:b", "ted:c"}, j); !close(got, 0.5) {
		t.Errorf("MRR = %v, want 1/2", got)
	}
	if got := MRR([]string{"ted:z"}, j); got != 0 {
		t.Errorf("MRR = %v, want 0 when nothing relevant was retrieved", got)
	}
}

func TestMetrics_IgnoreDuplicateIDsAfterTheFirst(t *testing.T) {
	// A retriever that returns the same tender twice must not be able to
	// inflate recall past 1.0 — dedupe is the metric's job, not the caller's.
	j := Judgements{"ted:a": 2}
	if got := RecallAtK([]string{"ted:a", "ted:a"}, j, 10); !close(got, 1) {
		t.Errorf("RecallAtK = %v, want exactly 1", got)
	}
}
