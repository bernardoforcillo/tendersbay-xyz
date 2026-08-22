---
name: landing-analytics
description: "PostHog instrumentation — house idiom, landing event vocabulary, the cta_clicked -> signup_started funnel join, and the typed EVENT_SPECS registry for authenticated surfaces"
metadata:
  type: project
  updated: 2026-08-22
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
| `landing_search_focused` | `location: 'search_dock'` | `search-dock`, once per focus cycle (flag resets on blur) |
| `landing_search_performed` | `query_length, result_count, location: 'search_dock'` | `search-dock`, on each *resolved* debounced search (not on error) |

The two `landing_search_*` events are live because the dock is live — see
[[landing-page-design]]; note `query_length` (a number), never the query text.

**Two mechanisms now, and the newer one is typed (added since the app grew past the
landing).** The raw `usePostHog()` idiom above is still what landing/auth uses. Everything
on the authenticated surfaces goes through the declared registry
`apps/platform/src/analytics/events/` (`EVENT_SPECS` + `useCaptureEvent` /
`useCaptureEventOnce`), whose own doc comment cites this page as the house idiom it follows.
Reach for the registry for any new non-landing event. What it buys, structurally rather than
by review: every payload type admits only literal unions/numbers/booleans (no `string`
anywhere, so free text cannot be passed); `analyticsEventProps` copies from the **spec**, not
from the caller's object, so an `as`-widened caller still cannot smuggle a key in; and an
out-of-vocabulary string becomes `INVALID_VALUE` rather than being forwarded — the event
still fires so the denominator stays honest, and a non-zero `invalid` bucket in PostHog *is*
the alarm that the TS vocabulary has drifted from the Go constants. Vocabularies are
imported from `~/features/{company,tenders}/constants`, never re-declared.

**`reminder_link_opened`** (`{ location: 'bid_detail', bucket }`, bucket from the closed
`REMINDER_BUCKETS = ['14','7','3','1']`) closes the loop on the deadline reminders —
[[deadline-reminder-pipeline]]. Three decisions in `use-reminder-attribution` worth reusing:
it reads `window.location.search` directly, not the router's `useSearch` (a one-shot read of
a marker the *backend* put in a mailed URL, and the route declares no `validateSearch` for
it); it strips the markers afterwards with `replaceState` (no extra history entry) so a
reload or a URL pasted to a colleague cannot each re-attribute one real click; and an
unrecognised bucket becomes `''` and **still fires** rather than being dropped — the value
comes from a URL anyone can edit, but a link that lost its bucket is still a reminder that
worked, and dropping it would undercount the very thing being measured. Carrying the bucket
is the point: a 1-day last call and a 14-day heads-up are different products sharing a
template, and averaging them describes neither.

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
