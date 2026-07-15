# docs/gtm — go-to-market workspace

Committed home for tendersbay's GTM artifacts, maintained by the GTM desk
agents — `gtm-engineer` (via `/gtm`), `growth-marketer` (via `/growth`), and
`neuro-ux-designer` (via `/ux`) — and reviewed by a human before every commit.

## What lives here

Non-code GTM **outputs**: launch checklists, channel plans, messaging matrices,
keyword maps, funnel definitions, experiment briefs, competitor notes.

What does **not** live here:

- Positioning, brand palette, tone, terminology — source of truth is
  `.claude/memory/landing-page-design.md`. Link to it; never fork it.
- Design specs / implementation plans — `docs/superpowers/` (local-only, gitignored).
- Code. Copy ships in `apps/platform/src/assets/locales/`, SEO in
  `packages/vite-plugin-seo` + `apps/platform/vite.config.ts`, events per the
  `add-posthog-metrics` skill.

## Conventions

- **Filenames**: kebab-case, date-prefixed for point-in-time docs
  (`2026-07-14-launch-checklist.md`); undated for living docs (`channel-plan.md`).
- **One topic per file**; English as the working language (copy itself is
  authored per-locale in the app, not here).
- **Funnels before instrumentation**: a funnel/experiment gets a brief here
  (events, properties, success metric) before any `capture()` call is written.
- **No invented numbers**: pre-launch, every metric cited must have a source.

## Layout

Flat until it hurts. When a category accumulates ~5+ files, promote it to a
subfolder (`launch/`, `seo/`, `experiments/`, `research/`).
