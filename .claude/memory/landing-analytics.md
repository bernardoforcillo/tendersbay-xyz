---
name: landing-analytics
description: "Landing page PostHog instrumentation — house idiom, full event vocabulary, the cta_clicked -> signup_started funnel join"
metadata:
  type: project
  updated: 2026-07-26
  sources:
    [
      docs/superpowers/plans/2026-07-25-landing-usp-redesign.md,
      docs/superpowers/specs/2026-07-25-landing-usp-redesign-design.md,
    ]
---

Part of [[landing-page-design]]. PostHog events fired from the landing page and the signup
page it converts into.

**House idiom (verbatim, apply to any new landing/auth event):** `const posthog =
usePostHog()` (from `posthog-js/react`), then `posthog?.capture('event', { ...categorical,
location })` — optional-chained, **no manual consent guard** (global
`opt_out_capturing_by_default: true`), **never pass `locale`** (auto-attached as a
super-property). Event names are snake_case `object_verb` past-tense. Every event carries a
`location` (or `entry`) prop; all props are low-cardinality enums, no PII.

**Full event table (as of the 2026-07-25 USP redesign):**

| Event | Props | Fires from |
| --- | --- | --- |
| `landing_hero_deck_seen` | `location: 'hero'` | `hero` (sample-tender deck) |
| `proof_strip_viewed` | `location: 'proof'` | `proof-strip`, once on in-view (mirrors `agents_section_viewed`) |
| `agents_section_viewed` | `location: 'agents'` | `agents-section`, once on in-view |
| `coverage_market_focused` | `country, status, location: 'coverage_section'` | `coverage-section`, on flag hover/focus |
| `landing_cta_clicked` | `location: 'hero' \| 'header' \| 'cta_band'` | the 3 signup CTAs (hero primary, header pill, cta-band), on click |
| `signup_started` | `entry: 'hero' \| 'header' \| 'cta_band' \| 'direct'` | `features/auth/.../signup`, once on mount |

**Funnel join:** `landing_cta_clicked.location` and `signup_started.entry` deliberately share
the same vocabulary (`hero`/`header`/`cta_band`), so `$pageview -> landing_cta_clicked{location}
-> signup_started{entry}` joins cleanly per CTA origin. `entry: 'direct'` is the fallback
when a visitor reaches signup without a landing CTA (e.g. a bookmarked/typed URL).

**How `entry` gets from CTA to signup page:** the signup route's `validateSearch`
(`routes/$locale/auth/signup.tsx`) carries an optional `entry?: string` search param (no
`sanitizeRedirect` — it never drives navigation), added **before** any CTA could pass
`search={{ entry }}` (contract-first: the typed router rejects an unknown search key
otherwise). Each CTA's `RouterLink`/`Button` `search={{ entry: '<origin>' }}` sets it; the
signup page reads it via `useSearch({ from: '/$locale/auth/signup' })` and fires
`signup_started` on mount with `entry ?? 'direct'`.

**`landing_cta_clicked` fires before navigation** (inline in each CTA's click/press handler,
same tick as the client-side route change) — no page-unload beacon race, no
`capture`-with-callback plumbing needed.

**Known gotcha (deferred, non-blocking):** `signup_started`'s mount effect has no
once-guard, so React StrictMode double-fires it in **dev only** — production and the test
suite (no StrictMode) fire exactly once. Not fixed; a real once-guard (module-scoped flag,
the pattern `hero`'s `deckSeenCaptured` already uses) is a safe follow-up if it turns out to
matter for funnel counts.
