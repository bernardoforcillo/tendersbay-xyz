---
name: positive-control-for-negative-results
description: "Any 'we found nothing' claim is unverified until a positive control passes — a probe for something known to be present that the same method does find; three real false negatives from one session, two of which nearly became a PRD's headline findings"
metadata:
  type: reference
  updated: 2026-08-22
  sources: [docs/superpowers/prd/2026-08-17-competitive-program.md]
---

**The rule.** A negative result — "the codebase has no X", "the feed carries no Y", "that
value is discarded" — is **not a finding until a positive control passes**: a probe for
something *known to be present* that the same method, unchanged, does find. Without one you
have not measured absence, you have only observed that your method returned nothing, and those
two are indistinguishable from the outside.

Absence claims are disproportionately dangerous because they are **load-bearing and cheap to
state**. "There is nothing to parse" cancels a work item; "the page isn't routed" schedules
one. Nobody asks a negative result for its evidence the way they'd ask a positive one.

## Three false negatives from a single session, all confidently reported

1. **`grep -c today apps/platform/src/routeTree.gen.ts` = 0** → "the Today page is not
   routed." **False.** TanStack Router's generated tree names routes by **path**, never by the
   component or feature folder — the page is fully routed at
   `routes/_authenticated/workspaces/$workspaceId/index.tsx`, which imports
   `WorkspaceTodayPage`. Grepping a *generated* artifact for a *source-side* name is a
   guaranteed false negative. Positive control: grep the tree for any feature name you know is
   routed; it also returns 0.
2. **A `scan_tenders` regex for an XML element name = 0/60** → "no Italian notice carries the
   element." **False negative** — and this one carried its own proof: the **positive control
   also returned 0**. A regex for `AwardingTerms|ProcurementProject`, elements present in
   *every* eForms document, matched nothing. The tool searches **extracted text, not markup**,
   so element-name regexes cannot ever match. The correct instrument was the stored corpus
   (`xpath()` / `LIKE` over `xml_fetched_at` rows) — see [[selection-criteria-sources]].
3. **`needsProfile` "discarded"** → reported as a bug. **False.** It is wired through three
   pages (`workbench-overview`, `bid-detail`, `account/tenders`) and the separation is a
   *documented deliberate* one — the first-run profile carries a "double-prompt" comment
   explaining exactly why the two capture points stay distinct.

**Two of these came from a subagent report and were about to become a PRD's headline
findings.** That is the failure mode to guard: a delegated search returns a clean "nothing
found", and the summarising layer has no way to distinguish "searched correctly, found
nothing" from "searched with an instrument that cannot see this". Ask a subagent's negative
result for its positive control, or re-run one yourself.

## The practical checklist

- **Name the instrument and what it can see.** Text search vs. markup; source vs. generated
  artifact; extracted text vs. raw bytes; local tree vs. network.
- **Run the positive control before reporting**, not after being challenged. If it fails, you
  have learned about your tool, not about the world.
- **A network `000`/timeout is never evidence of absence** — it is evidence of egress. See
  [[sandbox-verification-capabilities]]; the ANAC correction in [[eu-coverage-section]] is what
  it costs to get this wrong and then write it down.
- **Distinguish structural from frequency claims.** "Where this idiom appears there is nothing
  to parse" (structural, n=2 is fine) is a different claim from "most notices use this idiom"
  (frequency, n=2 proves nothing).
- **Grep is the wrong instrument for import/dependency questions**; use the language's own
  graph tool. `services/backend/boundary_test.go` shells out to `go list` precisely because
  `grep -rn "internal/adapter" internal/core` matches the doc comments that *assert*
  compliance — see [[code-organization-principles]].

This is the search-side twin of [[sandbox-verification-capabilities]]: that page is about
preferring a running system to a compiler, this one is about not trusting a search that could
not have succeeded. Both are instances of the grounding discipline in
[[ai-coding-workflow-principles]].
