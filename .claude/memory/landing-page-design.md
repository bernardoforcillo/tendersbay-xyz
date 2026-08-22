---
name: landing-page-design
description: "tendersbay landing page — positioning, brand palette, tone, and key product framing"
metadata:
  type: project
  updated: 2026-08-22
  sources:
    [
      docs/gtm/2026-07-15-landing-restructure.md,
      docs/superpowers/plans/2026-07-25-landing-usp-redesign.md,
      docs/superpowers/specs/2026-07-25-landing-usp-redesign-design.md,
    ]
---

The tendersbay marketing/landing page (route `/$locale/` in `apps/platform`) is an
**informational** pre-launch page (no pricing, no forms/waitlist, no social).

**Positioning (important):** tendersbay is **a team of AI agents** that, for SMEs and
entrepreneurs, **find** the best public tender across Europe, **prepare** the document
bureaucracy, and help them **win**. It is explicitly **NOT** a translation product
(an earlier brief framed it around language barriers — that framing was dropped).

**Brand / design language:**
- Palette "Warm": brand teal `#0d9488`, deep greens `#13322c`/`#0f3d36`, cream `#fbf7f0`.
- Type system (editorial, self-hosted via fontsource): **Calistoga** display serif for
  headings, **Inter** body, **JetBrains Mono** for eyebrows/labels + tender-card data.
  (Replaced the original Plus Jakarta Sans.) Soft-elevation shadow scale on cards
  (`--shadow-soft*` tokens); the header is transparent and overlays the hero.
- Tone: human-centric, neuro-UX informed (isolation effect on a single CTA, rule of
  three, processing fluency, peak-end), second person, bold/"grinta", never jargon.
- The user wants HIGH-CRAFT, distinctive design and rejects generic SaaS templates — use
  the `ui-ux-pro-max` skill for UI decisions. No emoji; SVG icons. UI stack per [[frontend-ui-stack]].
- **Copy terminology (mixed, decided 2026-06-26):** keep the procurement-correct
  **"awarded"** (en) / **"aggiudicata"** (it — de "zugeschlagen", fr "attribué",
  es "adjudicada") in the **technical / SEO** spots (hero highlight, meta title); the
  bolder **"yours" / "win"** swagger is allowed in **emotional micro-copy** (footer
  tagline "Europe's tenders. Yours."). (Supersedes the earlier blanket "never 'won'"
  rule.) **CTA copy changed 2026-07-16** from the waitlist-era "Claim your spot" to
  **"Create your account"** — see the restructure block below.

**Notable specifics:** hero shows a rotating sample-tender card (`SAMPLE_TENDERS` fixture →
swap for real tenders in phase 2); footer contact is `mailto:me@bernardoforcillo.com`;
copyright line is "© Bernardo Forcillo — Tutti i diritti riservati"; landing copy is
authored in all 24 EU locales (default/source `en-ie`). The page's static SEO `<head>` tags,
`robots.txt`, `sitemap.xml` and `llms.txt` are emitted at build by
[[vite-plugin-seo]], not by anything in the landing feature.

**Floating search dock** (`features/landing/.../organisms/search-dock`): a permanent
Gemini-style bar docked bottom-center (`z-40`, under the header) with a **looping localized
placeholder** of detailed tender example queries (`landing.search.examples`, rotated by
`useRotatingPlaceholder`, paused once you type). Fades out over the footer via
`useHideNearFooter` (IntersectionObserver on `#site-footer`).

**It is LIVE — corrected 2026-08-22.** This page previously described it as "grayscale +
disabled (pre-launch teaser, not functional)"; that is **stale**. It is a real `<input>`
driving `useLandingSearch` → `tenderClient.searchTenders`: 300ms debounce, 2-char minimum,
3-result cap, anon-safe (server clamps the anon tier), with a monotonic request-id guard so
a slow superseded response cannot clobber a newer one. Results render **upward** above the
input (Spotlight/Raycast pattern) through `SearchResults`, over an honest five-state machine
(`idle | loading | results | empty | error` — there is deliberately no "sample" state; an
empty result is `empty`, never faked into cards). On each resolved search it fires
`landing_search_performed` and writes a **carry-over** (`~/lib/landing-carry-over`,
sessionStorage key `tb.landing.lastSearch`, read-and-cleared) that pre-fills the first-run
client-profile capture in `features/workspace/.../first-run-profile`. Events in
[[landing-analytics]].

The "coming soon" teaser did not disappear — it **moved**. The disabled-but-focusable RAC
`Button` (`aria-disabled`, no-op `onPress`, `cursor-default grayscale`) and the
`landing.search.hint` tooltip ("AI-powered search — coming soon") now live in the
**account** dock, `features/account/.../organisms/search-dock`. So the `landing.search.hint`
key is still live copy — don't delete it as dead.

**Copy re-architecture (2026-06-26, persona-led + cutting tone):** the landing was
rewritten for max "desire at the end", driven by the buyer-personas + vertical study
PDFs. Tone is **cutting**: provoke the rigged status quo and big players' bid offices,
never the reader; no invented metrics (pre-launch). Section flow: Hero → Problem
("the cost") → Agents → **Audience rebuilt to 3 persona cards** (run the bids · own the
number · multiply across clients) → **new `assurance-section`** (4 Q&A objection cards:
data not used for training, no hallucination/cites the page, per-client data isolation,
integration) → Coverage (flags, kept) → Vision → CTA. **Propagated to all 24 locales**
(new keys `landing.audience.items` 3 cards + `landing.assurance.*` 4 cards), with the
ESPD acronym localised per country (it DGUE, fr DUME, de EEE, es DEUC, pt DEUCP, pl JEDZ,
ro DUAE, sk JED, hu EEKD, lt EBVPD, bg ЕЕДОП, others ESPD). Completeness test
`src/assets/locales/landing-content-keys.test.ts` asserts all 24 locales carry both blocks.
Full suite green (194 tests). Follows the user's usual SDD flow of writing copy first in
one locale, then propagating it across all 24.

**Competitor-informed restructure (2026-07-16, `feature/landing-restructure`):** a
competitor teardown (TED, TenderNed, Mercell, Stotles, Tendium, Tussell…) drove a
category-defining rework. Three changes to the flow, now
Hero → **Proof strip** → Problem → **Agents (reworked)** → Audience → Assurance →
Coverage → Vision → CTA:
- **New `proof-strip` organism** after the hero (`landing.proof`): "honesty judo" — flex
  *the prize, not us*. Real EU-sourced scale (~€2tn/yr public spend · 250k+ contracting
  authorities · ~800k TED notices/yr) in the slot competitors fill with fake logos, with a
  **visible citation line** as the trust signal (no invented metrics — figures are
  EC/TED-sourced). Loss aversion + authority + processing fluency.
- **Agents section reworked from the find/prepare/win triptych to an open-loop "overnight
  shift" hook** (`landing.agents`: `title` + `lead` + 3 `items` each with a new `time`
  field). Headline is a curiosity gap ("Here's what your agents did while you slept.");
  the three cards became a timestamped timeline **02:14 → 05:30 → 07:00** (`<time>` mono
  eyebrows, `tabular-nums`, `text-brand-200`) — show-don't-tell over abstract labels
  (Zeigarnik open loop + peak-end). The tools-vs-agents wedge moved into the `lead`. Same
  brand-700 band, 3-up grid, icons, Reveal stagger.
- **CTA now drives signup, not a waitlist.** All "join the waitlist / claim your spot"
  copy dropped; `landing.cta` + hero primary CTA route to the real `/$locale/auth/signup`
  flow (button "Create your account"). The authenticated product (auth/workbench/
  workspace/explore) already exists in-repo, so the waitlist framing was wrong.
  **Stale doc, flagged 2026-08-22 (not rewritten):** `docs/gtm/feature-growth-priorities.md`
  is built end-to-end on waitlist capture — 25 mentions, a whole "1.1 Waitlist capture with
  position-in-line + country gating" section, an invite-link referral loop keyed on
  `waitlist_signup`, and the claim that the closing CTA reads "Join the waitlist — Claim
  your spot". Every one of those is false as of this date. Its loop layer needs a rewrite
  against the real signup funnel ([[landing-analytics]]) before anything is built from it.
- **Instrumentation:** `AgentsSection` fires `agents_section_viewed` ({ location: 'agents' })
  once via motion `useInView` (once, amount 0.4) — measures whether the open-loop hook pulls
  readers toward the CTA. Consent-gating is automatic. (`locale` is a super-property.)
- Copy propagated across all 24 locales (ESPD localised per market as before); the
  completeness test carries the new keys. Ported to `feature/landing-restructure` off `dev`
  via cherry-pick (the search-dock work already on `dev` auto-merged cleanly). Strategy doc:
  `docs/gtm/2026-07-15-landing-restructure.md`.

**USP redesign (2026-07-26, `feature/landing-usp-redesign`):** rewrote the hero to assert
the intersection USP directly instead of a rhetorical hook, hoisted the buried
differentiators, and shipped the first PostHog conversion-funnel baseline.
- **Hero H1** now states the USP head-on: "Every public tender in 27 countries — found,
  prepared, **awarded.**" (replaces the earlier rhetorical-question framing). Primary CTA
  now **converts**: routes to `/$locale/auth/signup` (`search={{ entry: 'hero' }}`) instead
  of the in-page `#agents` anchor; secondary CTA repoints `#vision` → `#agents` ("See how it
  works").
- **Section order changed again** (supersedes the 2026-07-16 order below): Coverage now
  renders **before** Assurance. Current full flow: Hero → Proof strip → Problem → Agents →
  Audience → **Coverage → Assurance** → Vision → CtaBand.
- **CTA salience:** the `SiteHeader` signup pill is scroll-aware — ghost/outline while at the
  top of the page (hero primary CTA is the single dominant filled control) and filled
  `bg-brand-600` once scrolled (header pill becomes the standing filled CTA), driven by the
  header's existing `useScrolled()` and its ~32px threshold (same as the header's frost
  morph). The "Log in" link is untouched.
- **Agents section** gained a mono eyebrow above its title, `landing.agents.eyebrow` =
  "Agents, not another search box", reinforcing the hero <-> Agents wedge without touching
  the guarded `agents.items` timeline timestamps (`['02:14','05:30','07:00']`).
- **Coverage reframed** around TED-native + national portals — see [[eu-coverage-section]]
  for the stat-box/badge/copy details.
- **Conversion funnel instrumented** (PostHog) — see [[landing-analytics]] for the event
  contract and vocabulary.
- **Hero H1 width is grid-bound, not `max-w`-bound (layout gotcha).** Verified live
  (Vite+Playwright, 1280x900 desktop + 390x844 mobile): widening the H1's `max-w-[15ch]`
  utility is a no-op — its rendered width is capped by the `md:grid-cols-[1.05fr_0.95fr]`
  hero column (~554px) on desktop and by viewport padding on mobile, both narrower than what
  `15ch` computes to (~588px, never binding). To actually change hero H1 wrapping, change
  the grid ratio or the font-size, not the `ch` utility — parked as a design call (not fixed
  in this session; see [[spacing-and-visual-rhythm]] for the adjacent spacing discipline).
- **Two distinct `Button` components — don't conflate them.** The landing `Button` atom
  (`features/landing/components/atoms/button`) is href/`to`-only (react-aria `Link` for
  `href`, TanStack Router `Link` for `to`), and now also carries an optional `onPress`
  (forwarded on all four render branches) plus a route-only `search` passthrough. It is a
  separate component from `@tendersbay/components/core`'s `Button` ([[core-component-kit]]),
  which instead has `onPress`/`isDisabled`/`type='submit'`/`variant='danger'`.

**Env gotcha (2026-06-27):** `pnpm --filter platform exec vitest`/`pnpm exec biome` started
failing in a pre-run deps check (`runDepsStatusCheck` → `pnpm install` → `ERR_PNPM_IGNORED_BUILDS:
core-js`). Bypass by calling the binaries directly: `apps/platform/node_modules/.bin/vitest run
--root apps/platform` and `node_modules/.bin/biome check --write <paths>`.

**Light/dark section rhythm (2026-06-27, intentional — don't "fix" back; ladder updated
2026-07-16, order updated 2026-07-26):** value ladder `cream-50 → cream-100 → ink-900 →
ink-950` used as an arousal/attention ladder. Current rhythm (verified against the code):
Hero L · **Proof L** (cream-100, the new proof strip continues the hero field) ·
**Problem D** (`bg-ink-900` — now dark; the earlier "Problem L" note is stale) ·
Agents (brand-700 teal band) · Audience L (cream-100, warm) · **Coverage D** (`bg-ink-950` —
deliberately dark so the flag tiles, which are `bg-white`, *light up* against the dark; now
sits before Assurance, not after) · Assurance L (cream-50, distinct) · **Vision L**
(cream-100 — an airy "breath of light" before the close) · CtaBand (ink card) · Footer D.
**Note (2026-07-16):** with Problem now dark, Coverage is no longer the *only* mid-page dark
beat, so the original Von-Restorff rationale is partly superseded — whether the two dark
beats + the teal Agents band read well together is a **design/UX call** (route to `/ux` if
the rhythm needs a pass). Coverage still carries an aurora top bleed + bottom fade to
`#fbf7f0`.

Full spec: `docs/superpowers/specs/2026-06-21-landing-page-design.md` (original) and
`docs/superpowers/specs/2026-06-26-landing-copy-rearchitecture-design.md` (this rewrite) —
both gitignored, local.
