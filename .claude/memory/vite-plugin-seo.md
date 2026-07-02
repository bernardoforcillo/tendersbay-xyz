---
name: vite-plugin-seo
description: "The @tendersbay/vite-plugin-seo package — what it does, key decisions, deferred polish"
metadata:
  type: project
  updated: 2026-07-01
  sources: []
---

`packages/vite-plugin-seo` (`@tendersbay/vite-plugin-seo`) is a private, **no-build** Vite
plugin wired into `apps/platform/vite.config.ts`. At build it emits `dist/robots.txt` and
`dist/sitemap.xml` (every public route under `$locale` × all 24 locales, with the full
`xhtml:link` hreflang alternate set + `x-default`, BCP-47 casing like `en-IE`) and injects
static `<head>` tags via `transformIndexHtml` (description, OG, Twitter, theme-color, JSON-LD
Organization+WebSite). Modules: `locale`(bcp47), `robots`, `sitemap`, `routes`(hybrid
discovery), `head`, `options`(normalizeOptions), `index`(the `seo()` plugin). Locales come
from `apps/platform/src/i18n/locales.ts` — DOM-free constants split out of `detect-locale.ts`
(which now re-exports them) so the Node-side vite config can import them without DOM-typed code.

**Design decisions (built 2026-06-26):** static meta identical across routes (SPA, no SSR);
**no hardcoded canonical** (a static one would mis-point the 24 locale URLs — rely on the
sitemap hreflang set); always production/indexable (no canary no-index toggle); hybrid route
discovery (auto-derive under `$locale` + include/exclude). The config-time `.ts`-import gotcha
(explicit `.ts` extensions + `allowImportingTsExtensions`) is documented in
`.claude/rules/frontend.md`.

Spec + plan are local-only/gitignored at
`docs/superpowers/{specs,plans}/2026-06-25-vite-plugin-seo*.md`; SDD audit trail in
`.superpowers/sdd/progress.md`. As of 2026-06-26 the 11 SEO commits live unpushed on
`feature/eu-coverage-grid` (interleaved with unrelated WIP), kept as-is per the user.

**Deferred polish** (final opus review = ready to merge, none blocking):
- `discoverRoutes` adds `include` to the set then filters `exclude`, so an overlapping
  include+exclude drops the include (spec prose says include wins); app config doesn't overlap.
- test-coverage gaps: head `twitterSite`/`themeColor`/absolute-URL passthrough; `twitter:card`
  asserted by presence not value; sitemap `toContain` wouldn't catch a split-alternate bug;
  `generateBundle` file-emission only covered e2e; flat-route convention untested.
- `routeMeta.changefreq` typed as plain `string` (could be the sitemap frequency union).

Built under the same parallel-WIP commit-hygiene discipline as the rest of the repo (stage
only the files a change touches, never `git add -A`), following the user's usual SDD flow.
