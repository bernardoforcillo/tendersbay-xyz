---
name: vite-plugin-seo
description: "The @tendersbay/vite-plugin-seo package — what it does, key decisions, deferred polish"
metadata:
  type: project
  updated: 2026-07-26
  sources:
    [
      docs/superpowers/plans/2026-07-25-landing-usp-redesign.md,
      docs/superpowers/specs/2026-07-25-landing-usp-redesign-design.md,
    ]
---

`packages/vite-plugin-seo` (`@tendersbay/vite-plugin-seo`) is a private, **no-build** Vite
plugin wired into `apps/platform/vite.config.ts`. Modules: `locale`(bcp47), `robots`,
`sitemap`, `routes`(hybrid discovery), `head`, `llms`(llms.txt), `locale-pages`(per-locale
index.html rewrite), `options`(normalizeOptions), `index`(the `seo()` plugin). Locales come
from `apps/platform/src/i18n/locales.ts` — DOM-free constants split out of `detect-locale.ts`
(which now re-exports them) so the Node-side vite config can import them without DOM-typed code.

**Correction (2026-07-26): meta is now per-locale localized, not static/identical.** An
earlier version of this page claimed static meta identical across routes and no hardcoded
canonical — both are now false in code, verified directly:
- `generateBundle` still emits `dist/robots.txt` + `dist/sitemap.xml`, and now also
  `dist/llms.txt` (`llms.ts` `buildLlmsTxt` — the emerging "robots.txt for LLMs", a Markdown
  brief for AI crawlers/answer engines built from trusted build-time config, no invented
  metrics).
- `transformIndexHtml` still injects the **static** head tags (OG/Twitter/theme-color/JSON-LD
  Organization+WebSite) shared by every route at pre-order.
- **New:** `writeBundle` (`index.ts`) reads the built root `dist/index.html` and, when
  `opts.localeMeta` is set, re-emits it per locale via `locale-pages.ts`
  `localizeIndexHtml` into `dist/<locale>/index.html` — localized `<title>`, description,
  OG/Twitter title+description, `<html lang>` (BCP-47), `og:locale`, a **self-canonical**
  (this **reverses** the original no-canonical decision — a distinct page per locale now
  exists, so self-canonical is correct) plus the full hreflang alternate set + `x-default` +
  `og:locale:alternate`, an optional `FAQPage` JSON-LD block (`faqPageScript`, when
  `localeFaq[locale]` is set — highest-value GEO/AEO signal, answer engines quote FAQ
  structured data verbatim), and an optional `<noscript>` hero+FAQ content block
  (`noscriptBlock`, real page content for JS-off visitors and AI crawlers, not cloaking). A
  failed rewrite (head-shape drift) **fails the build** rather than silently shipping
  default-locale copy; a stale `<dir>/index.html` not covered by `localeMeta` warns.
- `apps/platform/vite.config.ts` builds `localeMeta`/`localeFaq`/`localeHero` from each
  locale's `common.json` `landing.meta`/`landing.hero`/`landing.assurance.items` at Vite
  **config-load** (`fs.readFileSync`, not a module import — config-time module resolution is
  fragile for raw-TS/JSON workspace imports) and passes them into `seo(...)`.
- `apps/platform/internal/server/server.go` `localeIndexes` reads every locale-shaped
  `<dir>/index.html` the plugin emitted (embedded FS) and serves it to that locale's route,
  same env injection as the root index.
- `features/landing/components/pages/landing-page/index.tsx` **also** rewrites 5 meta tags
  client-side on mount (`document.title`, `meta[name=description]`,
  `og:title`/`og:description`, `twitter:title`/`twitter:description`) from
  `t('landing.meta.*')`, for the SPA case.
- **Net effect: editing `landing.meta.*` copy in any locale's `common.json` propagates to
  SEO (server-rendered per-locale HTML + client-side SPA sync) with zero code change** —
  confirmed while shipping [[landing-page-design]]'s hero/meta copy rewrite.

**Other design decisions (built 2026-06-26, still true):** always production/indexable (no
canary no-index toggle); hybrid route discovery (auto-derive under `$locale` +
include/exclude). The config-time `.ts`-import gotcha (explicit `.ts` extensions +
`allowImportingTsExtensions`) is documented in `.claude/rules/frontend.md`.

Spec + plan are local-only/gitignored at
`docs/superpowers/{specs,plans}/2026-06-25-vite-plugin-seo*.md`; SDD audit trail in
`.superpowers/sdd/progress.md`.

**Deferred polish** (final opus review = ready to merge, none blocking):
- `discoverRoutes` adds `include` to the set then filters `exclude`, so an overlapping
  include+exclude drops the include (spec prose says include wins); app config doesn't overlap.
- test-coverage gaps: head `twitterSite`/`themeColor`/absolute-URL passthrough; `twitter:card`
  asserted by presence not value; sitemap `toContain` wouldn't catch a split-alternate bug;
  `generateBundle` file-emission only covered e2e; flat-route convention untested.
- `routeMeta.changefreq` typed as plain `string` (could be the sitemap frequency union).
