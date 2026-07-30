// Package eval measures tender-search relevance offline.
//
// It exists because every ranking knob in internal/core/tender is an
// uncalibrated guess (see Ranking and FitThresholds), and there is no click
// data to calibrate against. Without a measurement, a change to fusion is
// indistinguishable from a regression: both look like "different results".
//
// Nothing in this package runs in production. The metrics are pure functions;
// the corpus and golden set are committed fixtures; the runner
// (eval_test.go) is gated on EVAL_* environment variables and stays out of PR
// CI, which has no Qdrant and no Ollama.
package eval

import (
	"math"
	"sort"
)

// Judgements grades how relevant each tender is to one query: 2 relevant,
// 1 marginal, 0 not relevant. A tender absent from the map is 0.
//
// Keys are "<source>:<source_ref>", never the bigserial id — a re-imported
// corpus snapshot gets fresh ids, and judgements keyed on those would silently
// detach from the tenders they were written for.
type Judgements map[string]int

// dedupe returns ranked with repeats after the first occurrence removed,
// truncated to k. A retriever returning the same tender twice must not be able
// to push recall past 1.0.
func dedupe(ranked []string, k int) []string {
	if k <= 0 || k > len(ranked) {
		k = len(ranked)
	}
	seen := make(map[string]bool, k)
	out := make([]string, 0, k)
	for _, id := range ranked {
		if len(out) == k {
			break
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// relevantCount is how many tenders carry a non-zero grade.
func relevantCount(j Judgements) int {
	n := 0
	for _, grade := range j {
		if grade > 0 {
			n++
		}
	}
	return n
}

// RecallAtK is the fraction of graded-relevant tenders that appear in the top k.
//
// A query with nothing relevant returns 0 rather than NaN: NaN would propagate
// through every average that includes the query and make the whole report
// unreadable.
func RecallAtK(ranked []string, j Judgements, k int) float64 {
	total := relevantCount(j)
	if total == 0 {
		return 0
	}
	hits := 0
	for _, id := range dedupe(ranked, k) {
		if j[id] > 0 {
			hits++
		}
	}
	return float64(hits) / float64(total)
}

// gain is the standard exponential relevance gain, 2^grade - 1: grade 2 is
// worth three times grade 1, which is what makes nDCG care about ordering
// between two relevant results rather than only about finding them.
func gain(grade int) float64 {
	if grade <= 0 {
		return 0
	}
	return math.Pow(2, float64(grade)) - 1
}

// discountedGain sums gain/log2(rank+1) over a ranked list.
func discountedGain(grades []int) float64 {
	sum := 0.0
	for i, grade := range grades {
		sum += gain(grade) / math.Log2(float64(i)+2)
	}
	return sum
}

// NDCGAtK is normalised discounted cumulative gain over the top k.
//
// The ideal ranking is truncated to k as well. Without that, a query with more
// relevant tenders than the window could never score 1.0 however perfectly it
// ranked, and the metric would silently punish breadth of judgement rather than
// quality of ranking.
func NDCGAtK(ranked []string, j Judgements, k int) float64 {
	window := dedupe(ranked, k)
	got := make([]int, len(window))
	for i, id := range window {
		got[i] = j[id]
	}

	ideal := make([]int, 0, len(j))
	for _, grade := range j {
		if grade > 0 {
			ideal = append(ideal, grade)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(ideal)))
	if k > 0 && len(ideal) > k {
		ideal = ideal[:k]
	}

	best := discountedGain(ideal)
	if best == 0 {
		return 0
	}
	return discountedGain(got) / best
}

// MRR is the reciprocal rank of the first relevant tender, or 0 if none was
// retrieved. It answers a different question from recall: not "did we find
// them" but "did the user have to scroll".
func MRR(ranked []string, j Judgements) float64 {
	for i, id := range dedupe(ranked, 0) {
		if j[id] > 0 {
			return 1 / (float64(i) + 1)
		}
	}
	return 0
}
