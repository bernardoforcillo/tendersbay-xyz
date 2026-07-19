---
name: core-component-kit
description: "@tendersbay/components/core — kit conventions: cn helper, signal tokens, kit rules, native Select/Banner/Switch/tabClass, adoption checklist, polish backlog"
metadata:
  type: project
  updated: 2026-07-13
  sources:
    [
      docs/superpowers/plans/2026-07-11-redesign-foundation.md,
      docs/superpowers/plans/2026-07-12-redesign-surfaces.md,
    ]
---

The redesign's shared component kit (Phase 0, branch `feature/redesign-foundation`)
lives at `packages/components/src/core` and is consumed as
`@tendersbay/components/core` — raw TS, no build step, bundled by the app's Vite.
Kit-only Tailwind classes need the app-side `@source` line (see
[[tailwind-v4-shared-kit-css]]).

**Contents:** `Button`, `Pill`, `Card`, `Switch` (atoms) · `Field`, `EmptyState`,
`Select`, `Banner` (molecules) · `PageHeader` (organism, presentational — the app
injects the sidebar toggle via `leading`; since Phase 4 the app's own `PageHeader`
is a thin wrapper composing it) · `navItemClass` + `tabClass` · `cn`. Package-local
vitest + jsdom rig mirrors `apps/platform`'s.

**Decisions and why:**

- **`cn` helper** (`src/core/cn/`): shadcn-style `twMerge(clsx(...))`, with `clsx` +
  `tailwind-merge` as regular runtime deps. It uses `extendTailwindMerge`, not stock
  `twMerge` — non-stock theme scales must be registered there
  ([[tailwind-v4-shared-kit-css]]).
- **`react-aria-components` is a peer + dev dep** (`^1.19.0`). Keep the peer range in
  lockstep with the app's version, or add `vite resolve.dedupe` — two RAC copies break
  context. Lockstep check deferred to Phase 1.
- **Signal tokens** (in `packages/tailwind/theme.css`): one color = one meaning,
  everywhere. `signal-warm` = deadline approaching, `signal-urgent` = urgent/overdue;
  match/confirmation reuses `brand-100/700`; interactive is brand. `Pill` tones map
  onto these.
- **Kit rules (cognitive rules made structural):** click targets ≥ 40px (`h-10`/`h-12`
  only); transitions `duration-150`; no copy/i18n inside the package (all text via
  props — i18n stays app-side); no app-only deps (no `@tanstack/react-router`, no
  `i18next`). `navItemClass` keys current-page styling off `aria-current="page"` (set
  by the router's `Link`), so it stays a static string and the kit stays router-free.
- **Display font is Fraunces Variable** (swapped from Calistoga in Phase 0; every
  existing `font-display` picked it up via the token). Gotcha: the default
  `@fontsource-variable/fraunces` import loads **only the wght axis** — Fraunces'
  signature `opsz` display cut needs `@fontsource-variable/fraunces/opsz.css` plus
  `font-optical-sizing: auto`. Decision deferred to Phase 1 (ask the user).
- **`Select` is a styled NATIVE `<select>`** (Phase 4), deliberately not a RAC
  listbox — a zero-behavior-change drop-in for the app's former raw selects. The
  label element wraps the control, so no id wiring is needed.
- **`Banner` maps tone to a live-region role**: `error` → `role="alert"`,
  `success` → `role="status"` (replaced the app's `ERROR_BOX` + ad-hoc success divs).
- **`Switch`** is RAC Switch, track-left / label-right, with the ≥40px target baked
  in (`min-h-10`); `tabClass` (exported beside `navItemClass`) also carries
  `min-h-10` and keys active styling off `aria-current="page"`.
- **`Field` forwards `TextFieldProps` only** — native input attributes outside that
  type (e.g. `min`) cannot pass through; enforce numeric bounds with JS clamping
  (`Math.max(0, …)`) at the call site. `Field` leaves `isInvalid` **undefined** when
  uncontrolled (`props.isInvalid ?? (errorMessage ? true : undefined)`) so RAC's
  native constraint validation still displays — why in [[react-aria-motion-gotchas]].

**Adopting the kit (Phase 4 restyle lessons):** when a restyle swaps native HTML
validation attributes for a component API, **enumerate every dropped attribute**
(`required`, `minLength`, `type="email"`, `min`…) **and prove each constraint is
preserved elsewhere**. "Zero behavior change" implementers missed two real
regressions that only review caught: kit `Field` pinning `isInvalid={false}`
silently suppressed native validation in all five account forms, and "max uses"
losing its native `min=` let a negative value reach a write RPC. The mechanical
adoption itself ran cleanly from pattern + per-page-mapping briefs with the Today
page as the in-repo canonical example. Account settings gained its `account` i18n
namespace (30 keys × 24 locales) via the [[locale-namespace-insertion]] recipe.

**Kit polish backlog (deferred from the Phase 4 final review):** `Select`
`labelHidden` option (per-row selects currently show a visible "Role" label);
`Switch` label-position option; a shared **link-button recipe** — RAC `Button`
cannot render an anchor, so real `<a>`/router links styled as buttons need a class
recipe (two duplicated `SIGN_IN_LINK` copies sit in the workspace join/accept
pages); keep `Banner`'s live region mounted (a `role="alert"` that mounts together
with its message can be missed by screen readers); `Field` `min`/input-props
pass-through; the five account organisms have no rendering tests; the two
role-editor organisms (workspace/workbench) remain duplicated. **Two more, verified
against the actual components (not assumed):** `Button` has hover/pressed/
focus-visible/disabled states baked into every variant but no loading/pending
state (no spinner, no `isPending`-style prop) — any async action currently has
nowhere to show it's working; `Banner`'s `Tone` union is only `'error' | 'success'`
— there is no `warning` tone for a non-blocking, non-error notice.

Pairs with the app-side stack in [[frontend-ui-stack]] and the design language in
[[landing-page-design]]. Spacing/proximity decisions between elements inside a
component (e.g. `Card`, `PageHeader`) follow [[spacing-and-visual-rhythm]].
