---
name: tailwind-v4-shared-kit-css
description: "Tailwind v4 skips node_modules — @source the kit's src; register non-stock utility scales in extendTailwindMerge"
metadata:
  type: reference
  updated: 2026-07-11
  sources: [docs/superpowers/plans/2026-07-11-redesign-foundation.md]
---

Two toolchain gotchas when a workspace kit ([[core-component-kit]]) carries Tailwind
classes the app itself never uses:

- **Tailwind v4 does not auto-scan `node_modules`** during source detection, so
  utility classes used only inside `@tendersbay/components` silently emit **no CSS**
  in the app. Fix: in `apps/platform/src/index.css`, directly under the theme import:
  `@source "../node_modules/@tendersbay/components/src";`
  Every future consumer app of the kit needs the same line (open idea, deferred:
  move the `@source` into `@tendersbay/tailwind`'s CSS so consumers get it with the
  theme import).

- **tailwind-merge misclassifies non-stock theme scales.** The theme's
  `shadow-soft` / `shadow-soft-md` / `shadow-soft-lg` elevation scale is not a stock
  shadow, so stock `twMerge` reads them as shadow *colors* — e.g.
  `cn('shadow-soft', 'shadow-lg')` kept both. Fixed in
  `packages/components/src/core/cn/index.ts` with
  `extendTailwindMerge({ extend: { classGroups: { shadow: [{ shadow: ['soft', 'soft-md', 'soft-lg'] }] } } })`.
  **Any new non-stock utility scale added to the theme must be registered there**, or
  `cn` resolves its conflicts wrongly.
