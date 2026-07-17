---
name: landing-page-design
description: "tendersbay landing page — positioning, brand palette, tone, and key product framing"
metadata:
  type: project
  updated: 2026-07-16
  sources: [docs/gtm/2026-07-15-landing-restructure.md]
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
authored in all 24 EU locales (default/source `en-ie`).

**Floating search dock** (`organisms/search-dock`): a permanent Gemini-style bar docked
bottom-center (`z-40`, under the header), **grayscale + disabled** (pre-launch teaser, not
functional) with a sparkle icon, a **looping localized placeholder** of detailed tender
example queries (`landing.search.examples`, rotated by `useRotatingPlaceholder`), and a
hover/focus hint "AI-powered search — coming soon" (`landing.search.hint`). Fades out over
the footer via `useHideNearFooter` (IntersectionObserver on `#site-footer`). Disabled-but-
focusable RAC `Button` (`aria-disabled`, no-op `onPress`).

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
- **Instrumentation:** `AgentsSection` fires `agents_section_viewed` ({ location: 'agents' })
  once via motion `useInView` (once, amount 0.4) — measures whether the open-loop hook pulls
  readers toward the CTA. Consent-gating is automatic. (`locale` is a super-property.)
- Copy propagated across all 24 locales (ESPD localised per market as before); the
  completeness test carries the new keys. Ported to `feature/landing-restructure` off `dev`
  via cherry-pick (the search-dock work already on `dev` auto-merged cleanly). Strategy doc:
  `docs/gtm/2026-07-15-landing-restructure.md`.

**Env gotcha (2026-06-27):** `pnpm --filter platform exec vitest`/`pnpm exec biome` started
failing in a pre-run deps check (`runDepsStatusCheck` → `pnpm install` → `ERR_PNPM_IGNORED_BUILDS:
core-js`). Bypass by calling the binaries directly: `apps/platform/node_modules/.bin/vitest run
--root apps/platform` and `node_modules/.bin/biome check --write <paths>`.

**Light/dark section rhythm (2026-06-27, intentional — don't "fix" back; ladder updated
2026-07-16):** value ladder `cream-50 → cream-100 → ink-900 → ink-950` used as an
arousal/attention ladder. Current rhythm (verified against the code):
Hero L · **Proof L** (cream-100, the new proof strip continues the hero field) ·
**Problem D** (`bg-ink-900` — now dark; the earlier "Problem L" note is stale) ·
Agents (brand-700 teal band) · Audience L (cream-100, warm) · Assurance L (cream-50,
distinct) · **Coverage D** (`bg-ink-950` — deliberately dark so the flag tiles, which are
`bg-white`, *light up* against the dark) · **Vision L** (cream-100 — an airy "breath of
light" before the close) · CtaBand (ink card) · Footer D. **Note (2026-07-16):** with
Problem now dark, Coverage is no longer the *only* mid-page dark beat, so the original
Von-Restorff rationale is partly superseded — whether the two dark beats + the teal Agents
band read well together is a **design/UX call** (route to `/ux` if the rhythm needs a
pass). Coverage still carries an aurora top bleed + bottom fade to `#fbf7f0`.

Full spec: `docs/superpowers/specs/2026-06-21-landing-page-design.md` (original) and
`docs/superpowers/specs/2026-06-26-landing-copy-rearchitecture-design.md` (this rewrite) —
both gitignored, local.
