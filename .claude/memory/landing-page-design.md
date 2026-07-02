---
name: landing-page-design
description: "tendersbay landing page — positioning, brand palette, tone, and key product framing"
metadata:
  type: project
  updated: 2026-07-01
  sources: []
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
  tagline "Europe's tenders. Yours.", CTA "Claim your spot"). (Supersedes the earlier
  blanket "never 'won'" rule.)

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
focusable RAC `Button` (`aria-disabled`, no-op `onPress`). Built under a parallel-WIP commit
hygiene discipline (the user edits in parallel; stage only the files a change touches, never
`git add -A`).

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

**Env gotcha (2026-06-27):** `pnpm --filter platform exec vitest`/`pnpm exec biome` started
failing in a pre-run deps check (`runDepsStatusCheck` → `pnpm install` → `ERR_PNPM_IGNORED_BUILDS:
core-js`). Bypass by calling the binaries directly: `apps/platform/node_modules/.bin/vitest run
--root apps/platform` and `node_modules/.bin/biome check --write <paths>`.

**Light/dark section rhythm (2026-06-27, intentional — don't "fix" back):** value ladder
`cream-50 → cream-100 → ink-900 → ink-950` used as an arousal/attention ladder. Rhythm:
Hero L · Problem L (top hairline seam marks the boundary from the hero) · Agents **D** ·
Audience L (cream-100, warm) · Assurance L (cream-50, distinct) · **Coverage D** (ink-900 —
deliberately dark so the flag tiles, which are `bg-white`, *light up* against the dark; also
breaks the long light trough) · **Vision L** (cream-100 — an airy "breath of light" before
the close) · CTA D (ink-950 card) · Footer D. Coverage is the only mid-page dark beat (Von
Restorff) and the close is the dark peak-end. Coverage carries an aurora top bleed + a bottom
fade to `#fbf7f0` to melt into the light Vision.

Full spec: `docs/superpowers/specs/2026-06-21-landing-page-design.md` (original) and
`docs/superpowers/specs/2026-06-26-landing-copy-rearchitecture-design.md` (this rewrite) —
both gitignored, local.
