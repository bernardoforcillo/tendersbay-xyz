---
name: react-aria-motion-gotchas
description: "RAC tooltip warmup, TextField isInvalid vs native validation, inert-vs-aria-hidden, jsdom mocks, reduced-motion test trick"
metadata:
  type: reference
  updated: 2026-07-13
  sources: [docs/superpowers/plans/2026-07-12-redesign-surfaces.md]
---

Hard-won interaction/animation gotchas for `apps/platform` (react-aria-components
+ motion), pairing with [[frontend-ui-stack]]:

- **A concrete `isInvalid` boolean on RAC `TextField` — even `false` — permanently
  disables native constraint-validation display.** Required/minLength/`type=email`
  failures stop surfacing: no `aria-invalid`, no `FieldError` text. A wrapper with an
  optional `errorMessage` prop must forward `undefined` when uncontrolled:
  `isInvalid={props.isInvalid ?? (errorMessage ? true : undefined)}` — this is what
  the kit `Field` does ([[core-component-kit]]). Regression-test pattern: render a RAC
  `Form` with an `isRequired` field, submit, expect `aria-invalid` on the input.
  (Caught in review, not by the implementer — it silently broke all five account forms.)

- **RAC `TooltipTrigger` has a ~1.5s global hover warmup that `delay={0}` does
  NOT bypass** — the first hover feels dead. For instant hover cards, make the
  tooltip **controlled**: `isOpen={hovered || focused}` driven by the trigger
  `Button`'s `onHoverChange`/`onFocusChange`. (Focus opens immediately even when
  uncontrolled; only hover has the warmup.)
- **A marquee's duplicate track must not be `inert`** — `inert` disables pointer
  events, so hover (and the card + pause) die on ~half the (duplicated) tiles.
  Keep duplicates mouse-interactive but hidden from SR/keyboard via `aria-hidden`
  on the track + `excludeFromTabOrder` on each RAC `Button` (passed via a
  `decorative` prop). Confirmed in-browser with `document.elementFromPoint`.
- **Marquee via motion** (the user prefers motion over CSS keyframes even here):
  `useAnimationFrame` advances a `useMotionValue` x in px, wrapped by
  (trackWidth + gap) measured with `ResizeObserver`; pause on hover/focus;
  `useInView` to idle off-screen; static grid fallback under `useReducedMotion`.
- **jsdom test env** lacks `ResizeObserver` and `IntersectionObserver` — both are
  stubbed in `apps/platform/vitest.setup.ts`.
- **Deterministic tests for motion variants**: override `window.matchMedia` in a
  `beforeEach` so `(prefers-reduced-motion: reduce)` matches, forcing the static
  (grid) branch instead of duplicated marquee tracks — also keeps the heavy
  full-template render under the vitest timeout.
