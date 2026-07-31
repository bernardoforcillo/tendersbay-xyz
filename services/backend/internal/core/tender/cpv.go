package tender

import (
	"context"
	"log/slog"
)

// CPVMatch is one CPV code a query resolved to, with the label that matched it.
//
// Lang and Label are carried all the way to the UI, not just used internally:
// a user must be able to see that "pulizie uffici" was read as
// "90919200 — Servizi di pulizia di uffici" and remove it. A signal the user
// cannot see is a signal they cannot correct — the same principle
// SearchOutput.AppliedFilters already exists to honour.
type CPVMatch struct {
	Code  string
	Lang  string // language of the label that matched, lowercase ISO 639-1
	Label string
	Score float64 // relative match strength; only the ordering is meaningful
}

// CPVLexicon resolves free text onto CPV codes.
//
// This is the bridge that makes cross-language search work. A CPV code is
// identical in all 24 EU languages, so a query resolved to codes reaches notices
// written in languages the query is not — something the lexical arm cannot do
// (it matches lexemes) and a stemmer cannot do either (it does not translate).
//
// Defined here, in the consumer, and implemented by the postgres adapter —
// mirroring KnowledgeBase and Repo.
type CPVLexicon interface {
	// MatchCodes returns the best-matching codes, best first, at most limit of
	// them. An unmatched query returns an empty slice and a nil error: "no code
	// describes this" is a normal answer, not a failure.
	MatchCodes(ctx context.Context, query string, limit int) ([]CPVMatch, error)
}

// maxCPVCodes bounds how many codes one query may resolve to.
//
// Eight because a real query names one or two concepts; a query that matches
// forty labels has matched a common word ("servizi", "services") rather than a
// concept, and letting all forty through would turn the arm into "anything in
// any of these categories" — which is not a relevance signal at all.
const maxCPVCodes = 8

// cpvHierarchyPrefix truncates a resolved CPV leaf code to a coarser
// hierarchy level, for prefix matching against tenders.cpv / cpv_secondary.
//
// Why this exists: CPV's 8 digits are hierarchical — 2-digit division, 4-digit
// class, 8-digit fully specified leaf — and an unspecified finer level is
// written as TRAILING ZEROS, not a shorter string: "90919200" is a
// fully-specified leaf ("office cleaning"), not a code with two blank digits.
// Task 12/13 wired the arm to prefix-match that leaf verbatim, which the live
// eval corpus showed is accidental EQUALITY, not a category match: the query
// "pulizie uffici" resolves to 90919200, but the judged German/French/Spanish
// tenders for that exact query carry 90911200, 90911000 and 90919000 —
// siblings that only agree with the resolved code on the first four digits,
// "9091" (cleaning services). Matching the full leaf asked every other
// language to have logged the identical leaf category, which they hadn't;
// that is why the four off-diagonal cells this phase targets (it→de, fr→de,
// nl→de, pl→de) sat at exactly 0.0000 with the leaf-equality version of this
// arm, and why other, previously-working cells regressed — a handful of
// leaf-only hits displaced good dense results instead of adding to them.
//
// This backs the code off by one hierarchy level: strip the trailing-zero
// padding to find how many digits are actually specified, then drop the
// least-significant 2-digit block of what's left. 90919200 → strip trailing
// zeros → 909192 (still leaf-precision, just not padded) → drop one level →
// 9091 (the level the four judged codes above actually share). An
// already-short or all-zero code (e.g. "90000000", division only) has nothing
// coarser to back off to without becoming an empty prefix — i.e. "match
// everything" — so it is returned as-is once at or below the 2-digit
// division level, never truncated to "".
//
// NOTE — measured against the live eval corpus (Task 14), this fix alone did
// NOT move the four target cells: the deeper blocker turned out to be
// FindByCPVPrefixes' recency-ordered, page-sized candidate window discarding
// the actually-relevant tender before fusion ever saw it. Task 15 fixed THAT
// too — ranking by the caller's own code order (cpvPrefixRanking) and giving
// the arm its own wide budget (cpvArmWindow) — and it STILL scored below
// baseline: RRF fuses on rank, and rank carries no relevance information
// inside one CPV category, so a widened window just floods fusion with more
// arbitrarily-ordered noise. See DefaultRanking's CPVWeight comment in
// hybrid.go for the full comparison against CPVBoost, the mechanism that
// replaced this arm in production. The arm ships disabled (CPVWeight 0) with
// this truncation bug — and the ordering/window ones — fixed underneath it,
// so re-enabling it later starts from a correct prefix instead of
// rediscovering bugs already found once.
func cpvHierarchyPrefix(code string) string {
	// How many digits are actually specified: the position just past the last
	// non-zero digit, rounded UP to the next even (2-digit) boundary so an
	// odd-positioned significant digit is never itself cut off — "90000000"
	// (division 90, nothing more specified) must round up to "90", not down
	// to the single digit "9".
	specified := 0
	for i, r := range code {
		if r != '0' {
			specified = i + 1
		}
	}
	if specified == 0 {
		// No non-zero digit anywhere — only a pathological all-zero input
		// reaches this (a real CPV code always has at least a division
		// digit). Treat it as "division-level, nothing more specified"
		// rather than truncating to "", which would match every row in the
		// table instead of narrowing anything.
		specified = 2
	} else if specified%2 != 0 {
		specified++
	}
	if specified > len(code) {
		specified = len(code)
	}
	prefix := code[:specified]

	if len(prefix) <= 2 {
		return prefix
	}
	return prefix[:len(prefix)-2]
}

// cpvArmWindow is the retrieval arm's OWN candidate budget, independent of
// the shared per-page window every other retriever uses (searchHybrid's
// `want`, sized off offset+limit).
//
// Task 14 found that sharing the small page window was the actual reason the
// arm never worked, not the resolved code's precision: for "pulizie uffici"
// -> class 9091, 56 tenders in the eval corpus carry that class, and the one
// judged-relevant German tender ranked 31st by publish date — outside the
// ~21-row shared window regardless of how the code was truncated. maxWindow
// is already this codebase's ceiling on how deep any one retrieval may reach
// (see its doc comment in hybrid.go), so reusing it here costs nothing new;
// it just stops the arm sharing the SMALLER page-sized budget with lexical
// and dense, which is the whole point — a category of 56 must not be
// truncated to ~21 before FindByCPVPrefixes' own ranking (see its doc
// comment) even gets to order them.
const cpvArmWindow = maxWindow

// cpvCandidates resolves the query onto CPV codes and, when the retrieval arm
// is enabled, returns the tenders carrying them, plus what was understood.
//
// Resolving codes is useful even when the arm itself is off: AppliedCPV shows
// the user what the query was read as regardless, and the CPVBoost multiplier
// (see fuse) needs the same resolved codes without needing a second
// retrieval. So the two concerns split here: MatchCodes always runs when the
// CPV signal is wanted in ANY form; FindByCPVPrefixes — the actual extra
// retrieval — only runs when the arm (CPVWeight) is switched on.
//
// Every failure degrades to "the arm contributes nothing", never to an error:
// the CPV bridge is an enrichment on top of two retrievers that already answer
// the query, and a missing or unseeded cpv_terms table must not be able to fail
// a search they could have served. This mirrors EmbeddingCache's contract, and
// it is deliberately NOT reflected in RetrievalMode — that field reports whether
// the search answered the question asked, and a missing enrichment does not
// change the question.
//
// The matches are returned even when the fetch failed: the inference itself
// succeeded, and the UI should still show what the server understood.
//
// suppressed are codes the caller does not want inferred — the user removed
// that chip on an earlier request. Filtered in here, immediately after
// MatchCodes returns, and before the codes reach EITHER consumer: the
// (disabled) retrieval arm's fetch and the AppliedCPV/CPVBoost reporting
// path both key off matches, so filtering once here is what keeps a
// suppressed code from being re-fetched OR re-reported through either route.
func (s *Service) cpvCandidates(ctx context.Context, query string, filters Filters, suppressed []string) ([]ScoredTender, []CPVMatch) {
	r := s.cfg.Ranking
	if s.lexicon == nil || (r.CPVWeight <= 0 && r.CPVBoost <= 1) {
		// Neither consumer of the CPV signal is switched on: resolving codes
		// would be pure waste on every search, same reasoning as before this
		// function had two consumers.
		return nil, nil
	}

	matches, err := s.lexicon.MatchCodes(ctx, query, maxCPVCodes)
	if err != nil {
		// Every CPV failure degrades to "the signal contributes nothing" (see
		// this function's doc comment) rather than failing the search, but a
		// silent, unlogged failure here is indistinguishable from the normal
		// "no code describes this query" answer — e.g. an unseeded cpv_terms
		// table would silently disable the CPV signal on every search, forever,
		// with nothing to notice. Logging costs nothing on the request path
		// (the search still degrades gracefully) and turns a silent outage into
		// a visible one.
		slog.WarnContext(ctx, "cpv lexicon match failed; degrading to no CPV signal", "error", err)
		return nil, nil
	}
	if len(matches) == 0 {
		return nil, nil
	}

	if len(suppressed) > 0 {
		skip := make(map[string]bool, len(suppressed))
		for _, code := range suppressed {
			skip[code] = true
		}
		kept := make([]CPVMatch, 0, len(matches))
		for _, m := range matches {
			if !skip[m.Code] {
				kept = append(kept, m)
			}
		}
		matches = kept
	}
	if len(matches) == 0 {
		// Every resolved code was suppressed, so there is nothing left for the
		// arm to fetch — and reporting the suppressed codes back as AppliedCPV
		// would re-render the very chips the user just removed.
		return nil, nil
	}

	if r.CPVWeight <= 0 {
		// The retrieval arm is off. CPVBoost, if enabled, is applied later
		// directly against candidates lexical/dense already found — see fuse's
		// CPVBoost doc comment for why a fourth ranked list is the wrong shape
		// for this signal. No separate fetch, no candidate slots spent.
		return nil, matches
	}

	// Match by CATEGORY, not by leaf — see cpvHierarchyPrefix. The resolved
	// code itself (matches[i].Code) is still what AppliedCPV reports to the
	// caller; only the repo query is widened.
	codes := make([]string, len(matches))
	for i, m := range matches {
		codes[i] = cpvHierarchyPrefix(m.Code)
	}

	// cpvArmWindow, not the shared page window — see its doc comment.
	tenders, err := s.repo.FindByCPVPrefixes(ctx, codes, filters, cpvArmWindow)
	if err != nil {
		return nil, matches
	}
	return tenders, matches
}
