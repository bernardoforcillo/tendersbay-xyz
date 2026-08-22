---
name: published-requirement-modelling
description: "A published selection criterion must inform, never decide — RequirementNoticePublished is non-authoritative by construction, everything maps to RequirementOther, and Blocking must be false or the go verdict deadlocks permanently"
metadata:
  type: project
  updated: 2026-08-22
  sources: [docs/superpowers/prd/2026-08-17-competitive-program.md]
---

How a selection criterion a buyer published (see [[selection-criteria-sources]] for what each
source actually gives) becomes a requirement on the scheda gara without ever failing a user.
Code: `services/backend/internal/core/company/selection.go` +
`eligibility.go`.

**Non-authoritative by construction, not by a new rule.** `RequirementNoticePublished` gets
its non-authority for free: `Requirement.CanBlock()`'s switch returns `true` for
`RequirementUserStated`, `r.Confirmed()` for `RequirementDocumentExcerpt`, and **`false` by
default** — so a new source is non-blocking unless someone deliberately makes it otherwise.
Preferring the safe direction as the *default branch* is the design; a test pins it in both
states, including after human confirmation (confirming attests the *quote* is right, and the
quote is prose either way).

**Everything maps to `RequirementOther`, regardless of category — the point, not a shortcut.**
A category says which shelf the buyer filed the requirement on. It does not give the
threshold, the currency, or the window of esercizi that `RequirementTurnover` exists to hold.
Mapping "financial" onto `RequirementTurnover` would build an `AmountRequirement` with a
**zero `MinMinor`** — a requirement the engine would evaluate and **pass**, against a number
nobody published. `RequirementOther` is the honest destination: captured verbatim, gap always
`GapUnknown`, remedy always "read it yourself".

## The trap worth remembering: `Blocking: false`, against the field's default

`Requirement.Blocking` defaults to **true** at capture time — an unclassified requirement a
human *captured* should raise a question rather than pass silently. Published criteria invert
that, and the interaction is the reason:

1. `RequirementOther` returns `unknownGap` **unconditionally** — no recorded fact ever moves
   it off `GapUnknown`.
2. `Assessment.Consistent` rejects a **go** when any gap is `GapUnknown` on a **Blocking**
   requirement.

Together: `Blocking: true` here would make *every* tender from a criteria-publishing source
permanently incapable of reaching go, carrying a capture question the user can answer and
whose answer changes nothing. That is not caution — it is a dead end the user cannot clear,
arriving without them having done anything. An earlier commit shipped `true` following the
field's default and the next one fixed it; a regression test pins **both** halves (a go
survives a published criterion, *and* the guard still rejects a genuinely blocking unknown)
so the test cannot pass by the rule having been weakened.

Generalisable: **whenever a requirement kind's gap is unconditionally unknown, `Blocking` on
it is a deadlock, not a safeguard.** Check the gap function before trusting a capture-time
default.

## Two more deliberate absences

- **No `Citation`.** The verification affordance points at a passage in a document the user
  can open; this came from a feed. An empty citation is honest — a citation pointing at the
  notice would be exactly the unverifiable claim that field exists to prevent.
- **Not deduped against captured requirements.** Matching prose to a parsed payload would mean
  deciding that *"volumen anual de negocios igual o superior a 309.552,00 EUR"* is the same
  requirement as a turnover threshold someone typed — which is the extraction problem this
  path explicitly declines to pretend it has solved.

Derived **on read** rather than stored per workspace: the notice's requirement is a fact about
the tender, true for every tenant, so writing it into the per-workspace requirement table
would duplicate identical rows and mis-model ownership. This keeps `internal/core` owning the
decision and the adapter owning the row, per [[code-organization-principles]].
