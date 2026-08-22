# Verifying a claim before acting on it

Applies repo-wide, and to every agent working in it. This rule exists because the
failure it describes happened three times in one session, twice inside a subagent
report that was about to become a product decision.

## The positive control

**A "we found nothing" is not a finding until the same method has found something.**

A search that returns zero and a search that *cannot* return anything look identical
from the outside. Both print nothing. Both feel like evidence. Only one is.

Before reporting an absence — no caller, no usage, no such field, no matches — run
the same probe against something you already know is present. If the control comes
back empty, the method is broken and the original zero means nothing.

Three real examples, all of which produced confident and wrong conclusions:

| The probe | Returned | Why it could not have worked |
| --- | --- | --- |
| `grep -c today routeTree.gen.ts` | 0 | TanStack names routes by PATH, not by component folder. The page was fully routed and linked from five places. |
| A regex for an XML element name over `scan_tenders` | 0/60 | The tool searches extracted TEXT, not markup. Its positive control returned 0 too. |
| "`needsProfile` is computed and discarded" | — | A documented deliberate separation, to stop two nudges double-prompting. Not a bug. |

The first killed a phase of a PRD before it was caught. The second was mine, and I
only found it because I ran the control.

## Delegated searches carry the same obligation

A subagent's clean "nothing found" is the highest-risk version of this, because the
report arrives already shaped as a conclusion and the method that produced it is not
in front of you. When dispatching a search, ask for the control alongside the result.
When receiving one, treat an absence as unverified until you can see what the probe
would have matched.

The same applies to any handed-over finding: a report is a claim with an author, not
a fact. Verify the load-bearing ones yourself before building on them, and retire a
claim you disprove **by name**, so a reader who remembers the old one can see it was
considered rather than quietly overwritten.

## What counts as verified

In rough order of strength, prefer the strongest you can afford:

- **A running system.** Postgres 16 starts in this sandbox and the backend boots
  against it (see `.claude/memory/sandbox-verification-capabilities.md`). Running the
  real thing has caught a duplicate index, two wrong fixture assumptions and a
  bucket-assignment surprise that reading the code did not.
- **A test that has been shown to fail.** A green assertion proves nothing until you
  have made it go red on purpose. Mutation-check the ones that matter: break the
  behaviour, watch the test name the breakage, restore.
- **The compiler.** Weakest of the three, and silent about anything it does not type:
  column order in a `Scan`, a URL that renders but 404s, an index that exists twice.

## Record what you could not check

An unrun check is not a passed check. When a required verification is impossible in
the environment — no `kubectl` for `kubectl kustomize`, no egress to a host, no
credentials for a corpus — say so in the commit message and in whatever document
depends on it, naming the check and who can run it. A gap that is written down gets
closed; one that is silently skipped reads as done.
