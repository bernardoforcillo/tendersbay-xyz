---
name: eu-coverage-section
description: "Landing 'coverage' section — marquee of 27 EU flags, AVAILABLE toggle, editable portals"
metadata:
  type: project
  updated: 2026-07-01
  sources: []
---

The landing page has a **coverage** section
(`apps/platform/src/features/landing/components/organisms/coverage-section`)
showing all **27 EU countries** as flag tiles in a 3-row **marquee** (grayscale
teaser; hover/focus opens a card with the country's national procurement portal).
Part of [[landing-page-design]].

- Which countries are "live" (full colour) is driven by a single `AVAILABLE`
  `Set` in the organism — **empty now (pre-launch teaser)**; add an ISO-3166
  alpha-2 code to light up that flag (no other change needed).
- Country **names** come from `Intl.DisplayNames` (localised across all 24
  locales for free — no per-locale name lists).
- National **portal names** live in `country-flag/portals.ts` (e.g. IT →
  "Acquisti in Rete (MEPA)", IE → "eTenders") — factual, editable.
- Section framing copy is `landing.coverage.*` in every locale `common.json`.
- Flags use the `country-flag-icons` package (`react/3x2/<CODE>`); Greece is `GR`.
- Interaction/animation specifics (controlled tooltip, decorative duplicates,
  reduced-motion fallback) in [[react-aria-motion-gotchas]].
