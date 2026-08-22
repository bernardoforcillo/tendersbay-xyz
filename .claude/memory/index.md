# Memory wiki — tendersbay-xyz

Project knowledge maintained by the `capture-learnings` skill. One line per page.

## Project
- [Core component kit](core-component-kit.md) — @tendersbay/components/core: cn helper, signal tokens, kit rules, native Select/Banner/Switch/tabClass, adoption checklist, polish backlog
- [Landing page design](landing-page-design.md) — positioning, warm palette, tone, product framing, USP redesign
- [EU coverage section](eu-coverage-section.md) — TED-native + national-portals story, 27 EU flags, AVAILABLE toggle, editable portals
- [Landing analytics](landing-analytics.md) — PostHog house idiom, landing event vocabulary, cta_clicked -> signup_started funnel join, typed EVENT_SPECS registry
- [Vite plugin SEO](vite-plugin-seo.md) — @tendersbay/vite-plugin-seo: robots/sitemap/llms.txt, per-locale localized meta (writeBundle), config-time .ts gotcha
- [Selection criteria sources](selection-criteria-sources.md) — TED eForms is an epo-sub-espd pointer (n=2, structural not frequency); ES PLACSP publishes inline prose with thresholds; category+type two columns; no weight/threshold column
- [Published requirement modelling](published-requirement-modelling.md) — RequirementNoticePublished informs, never decides: everything maps to RequirementOther, and Blocking must be false or unknownGap deadlocks the go verdict
- [Deadline reminder pipeline](deadline-reminder-pipeline.md) — core/alerting 14/7/3/1 buckets + per-bid watermark as the whole idempotency mechanism, narrowest-bucket rule, truncating daysUntil, digest CronJob + dry run
- [User locale capture](user-locale-capture.md) — users.locale: "" must stay representable, signup already carried and discarded it, EnsureLocale backfills on login but never overwrites

## Reference
- [Frontend UI stack](frontend-ui-stack.md) — motion + react-aria-components, no emoji
- [Locale namespace insertion](locale-namespace-insertion.md) — add an i18n namespace across the 24 locales: anchor-splice script, biome, <ns>-keys completeness test
- [React-aria & motion gotchas](react-aria-motion-gotchas.md) — RAC tooltip warmup, TextField isInvalid vs native validation, inert-vs-aria-hidden, jsdom mocks, reduced-motion trick
- [Tailwind v4 shared-kit CSS](tailwind-v4-shared-kit-css.md) — @source node_modules kit sources; extendTailwindMerge for non-stock scales
- [Spacing and visual rhythm](spacing-and-visual-rhythm.md) — relationship strength -> proximity, margin on the larger element, optical correction for padding around text
- [System design principles](system-design-principles.md) — decision framework: stateless servers, load balancing, microservices threshold, gateway, auth, object storage, event broker, caching/CDN, rate limiting
- [Code organization principles](code-organization-principles.md) — layered dependency-direction framework: ownership over folders, mechanical boundary enforcement, AI-agent amplification effect, naming consistency, supply-chain hygiene
- [AI coding workflow principles](ai-coding-workflow-principles.md) — build on the existing rules foundation instead of a blank slate, phase cross-layer features in build order, explicit per-phase scope
- [Positive control for negative results](positive-control-for-negative-results.md) — "we found nothing" is unverified until a probe for something known-present passes with the same method; three real false negatives
- [Sandbox verification capabilities](sandbox-verification-capabilities.md) — verify against a running system not a compiler: Postgres 16 startable here (what it caught), buf + kubectl absent, egress-blocked hosts
- [Marketing email deliverability](marketing-email-deliverability.md) — separate sending domain enforced in the constructor, List-Unsubscribe + One-Click, GET on /unsubscribe must change nothing (scanners prefetch)
