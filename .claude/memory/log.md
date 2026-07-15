# Memory wiki log

Append-only. Prefix `## [YYYY-MM-DD] <op> | <topic>` so `grep "^## \[" log.md | tail` shows the timeline.

## [2026-07-01] migrate | initial project-memory import
- Migrated 5 project/reference pages from harness memory into `.claude/memory/`.
- Personal pages (user-working-style, parallel-wip-commit-hygiene) left in harness memory.
- Built index.md; no dangling links after migration.

## [2026-07-13] ingest | redesign-surfaces (Phase 4)
- Updated `react-aria-motion-gotchas.md`: a concrete `isInvalid` boolean (even false) on
  RAC TextField permanently disables native constraint-validation display; wrappers must
  forward undefined when uncontrolled; Form-submit → aria-invalid regression-test pattern.
- Updated `core-component-kit.md`: Phase 4 additions (native Select, Banner tone→role,
  Switch min-h-10, tabClass), Field TextFieldProps-only pass-through + JS clamping,
  kit-adoption checklist (enumerate every dropped native validation attribute and prove
  it preserved), deferred kit polish backlog (labelHidden, link-button recipe, mounted
  Banner live region, etc).
- New page `locale-namespace-insertion.md` (reference): 24-locale namespace splice recipe
  (key inventory from code → anchor-splice script → biome → <ns>-keys completeness test);
  precedents shell/today/account.
- Personal note (mapping-based task briefs work for mechanical restyles, but per-task
  review still catches behavior regressions) → harness memory, not committed here.
- Skipped as already recorded: batch locale-edit basics (.claude/rules/frontend.md),
  commit/staging hygiene (git-flow rule), plan-only details (specific per-page mappings).

## [2026-07-11] ingest | redesign-foundation (Phase 0)
- New page `core-component-kit.md` (project): @tendersbay/components/core conventions —
  cn helper, signal tokens, kit rules, RAC peer lockstep, Fraunces + deferred opsz decision.
- New page `tailwind-v4-shared-kit-css.md` (reference): Tailwind v4 @source for
  node_modules kit sources; extendTailwindMerge registration for non-stock utility scales.
- Updated `frontend-ui-stack.md`: inbound link to the kit page (check the kit before
  hand-rolling primitives in an app).
- Personal/feedback lessons (implementer --no-verify vs Biome; task-brief Windows-path
  bug) routed to harness memory, not committed here.
- Skipped as already recorded: Biome-owns-formatting (CLAUDE.md), staged-paths-only
  commit hygiene (git-flow rule).
