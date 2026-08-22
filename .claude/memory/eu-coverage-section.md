---
name: eu-coverage-section
description: "Landing 'coverage' section — TED-native + national-portals story, 27 EU flags, AVAILABLE toggle, editable portals"
metadata:
  type: project
  updated: 2026-08-22
  sources:
    [
      docs/superpowers/plans/2026-07-25-landing-usp-redesign.md,
      docs/superpowers/specs/2026-07-25-landing-usp-redesign-design.md,
      docs/superpowers/prd/2026-08-17-competitive-program.md,
    ]
---

The landing page has a **coverage** section
(`apps/platform/src/features/landing/components/organisms/coverage-section`)
showing all **27 EU countries** as flag tiles in a 3-row **marquee** (grayscale
teaser; hover/focus opens a card with the country's national procurement portal).
Part of [[landing-page-design]].

- Which countries are "live" (full colour) is driven by a single `AVAILABLE`
  `Set` returned by `useCoverage()`/`getCoverage()` at runtime — **do not hardcode
  which countries are live**; the copy stays deliberately country-agnostic.
  Per current national ingestion the live/in-rollout national portals are
  PL/FR/ES (`services/ingestion/internal/adapter/source/{es,fr,pl}`, alongside
  the EU-wide `ted`/`tedapi`/`eforms`); **IT is not integrated**.
- **ANAC correction (2026-08-22) — the old reason here was wrong.** This page
  used to say IT was out because "ANAC is retrospective, not a live portal."
  **Superseded.** Since **2024-01-01** ANAC's *Piattaforma per la Pubblicità
  Legale* (`pubblicitalegale.anticorruzione.it`) is the **mandatory legal
  publication venue** for Italian bandi and avvisi, replacing the Gazzetta
  Ufficiale — live, prospective, credential-free. IT is unintegrated for
  engineering reasons, not because the data is retrospective. Two nuances
  that survived the correction:
  - The documented "interoperabilità ANAC" APIs are the **publish** side —
    certified platforms transmitting notices *into* PVL over PDND — **not** a
    bidder-side read feed. Integrator marketing about "ANAC APIs" is about the
    wrong direction of travel.
  - PVL is a SPA with UUID-keyed notice routes (`/bandi/<uuid>`, plus `/esiti`,
    `/ricerca`, `/bdncp`), which *implies* a JSON backend — **not confirmed**:
    every ANAC host is unreachable from the agent sandbox (`000`, not even a WAF
    403), so this is an open Phase-0 measurement, not a finding. See
    [[sandbox-verification-capabilities]] for what the sandbox can and cannot
    reach, and [[positive-control-for-negative-results]] for why an unreachable
    host must not be written up as an absent feed.
- **Story reframe (2026-07-26): TED-native + national-portals, not "coming soon
  everywhere".** TED — the EU-wide Tenders Electronic Daily — is covered
  natively day one across all 27 countries; the lit (non-grayscale) flags are
  the *national portals* plugged in directly, market by market. A new always-on
  TED badge (`landing.coverage.tedNative` = "TED · supported natively") renders
  above the flag marquee, never grayscale and never driven by `useCoverage`.
  Grayscale logic (`atoms/country-flag/index.tsx`) and `use-coverage.ts` are
  untouched by this reframe.
- **Stat-box now leads on `{total}` (27) mapped, not on "Live".** The `<dl>`
  order is: lead row `landing.coverage.statusMapped` ("Markets mapped") = the
  computed `total` (`EU_COUNTRIES.length`, no magic literal); "In rollout" row
  (`statusComingSoon`, relabelled from "Coming soon") = `comingCount`; a "Live"
  row (`statusAvailable`) renders **only when `availableCount > 0`** — this
  removes the "Live 0" anchor pre-launch while staying honest (the row appears
  the moment a national portal goes live). New copy keys:
  `landing.coverage.tedNative`, `nationalPortals`, `statusMapped`.
- Country **names** come from `Intl.DisplayNames` (localised across all 24
  locales for free — no per-locale name lists).
- National **portal names** live in `country-flag/portals.ts` (e.g. IT →
  "Acquisti in Rete (MEPA)", IE → "eTenders") — factual, editable.
- Section framing copy is `landing.coverage.*` in every locale `common.json`.
- Flags use the `country-flag-icons` package (`react/3x2/<CODE>`); Greece is `GR`.
- Interaction/animation specifics (controlled tooltip, decorative duplicates,
  reduced-motion fallback) in [[react-aria-motion-gotchas]].
- **Section order (2026-07-26):** Coverage now renders **before** Assurance in
  `landing-template/index.tsx` (capability → mission → ask, right before the
  `CtaBand`) — see [[landing-page-design]] for the full current flow.
