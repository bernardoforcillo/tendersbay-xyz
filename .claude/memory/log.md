# Memory wiki log

Append-only. Prefix `## [YYYY-MM-DD] <op> | <topic>` so `grep "^## \[" log.md | tail` shows the timeline.

## [2026-08-22] ingest | competitive-program: selection criteria, deadline reminders, locale, verification method
- Source: no plan file — the work came from a PRD run
  (`docs/superpowers/prd/2026-08-17-competitive-program.md` +
  `…-evidence-kit.md`) and 16 commits on `claude/platform-competitive-features-c4oyng`,
  whose messages carry the reasoning. Every claim below was re-verified against code before
  writing; the brief was treated as findings to check, not facts to copy.
- New `selection-criteria-sources.md` (project): TED eForms qualification is a pointer
  (`epo-sub-espd`) with the n=2 sampling caveat stated as structural-not-frequency; ES PLACSP
  publishes inline prose with real thresholds + legal basis; three CODICE families; why
  category and type are two columns; why there is no weight and no threshold column; the
  `DetailedSource` / `SourceRef` / `SaveDetail`-bypass wiring notes.
- New `published-requirement-modelling.md` (project): `RequirementNoticePublished` is
  non-authoritative because `CanBlock`'s switch defaults false; everything maps to
  `RequirementOther` because a category is not a threshold; and the trap — `RequirementOther`
  returns `unknownGap` unconditionally, so `Blocking: true` would deadlock the go verdict
  permanently. Generalised to "an unconditionally-unknown gap makes Blocking a deadlock, not
  a safeguard."
- New `deadline-reminder-pipeline.md` (project): buckets + watermark ARE the idempotency
  mechanism (no lock); `bucketFor` must return the narrowest crossed threshold or the 7-day
  reminder never fires; `daysUntil` truncates so the error is always understated; nobody-to-mail
  advances the watermark and every-send-failed does not; digest CronJob shape, `backoffLimit: 0`,
  06:00 UTC, and the `DIGEST_DRY_RUN` first-pass burst.
- New `marketing-email-deliverability.md` (reference): the transactional/marketing sending-domain
  split enforced in `email.NewReminder`'s constructor rather than documented, because the
  protection would be lost silently; List-Unsubscribe + One-Click; GET on `/unsubscribe` must
  change nothing (scanners prefetch); plain vs. hashed token threat model.
- New `user-locale-capture.md` (project): `""` must stay representable; `SignUpRequest.locale`
  had been collected and discarded all along; `EnsureLocale` backfills on login and never
  overwrites (header = device, stored = choice).
- New `positive-control-for-negative-results.md` (reference): the method lesson — three
  false negatives from this session (routeTree grep, `scan_tenders` regex whose own positive
  control returned 0, `needsProfile` "discarded"), two of them from a subagent report and
  about to become PRD headline findings.
- New `sandbox-verification-capabilities.md` (reference): Postgres 16 is startable here and
  caught a duplicate index, two wrong fixture assumptions and a bucket surprise; `buf` and
  `kubectl` are absent, so proto codegen and `kubectl kustomize` are blocked.
- Updated `landing-analytics.md`: the two `landing_search_*` events (live because the dock is),
  `reminder_link_opened` + its three attribution decisions, and the typed
  `analytics/events/EVENT_SPECS` registry as the mechanism for authenticated surfaces.
- Personal note (treat handed-over findings as claims to verify; prefer mutation-checked
  assertions) → harness memory, not committed here.
- Skipped as already recorded: commit/staging hygiene (git-flow rule), hexagonal layering
  (code-organization rule), CronJob-vs-service threshold (system-design rule).

## [2026-08-22] lint | full sweep + three stale claims retired
- `eu-coverage-section.md`: retired "ANAC is retrospective, not a live portal." PVL
  (`pubblicitalegale.anticorruzione.it`) has been the mandatory legal-publication venue since
  2024-01-01 — live, prospective, credential-free. Recorded the two nuances: the
  "interoperabilità ANAC" APIs are the **publish** side (via PDND), not a bidder feed; and the
  SPA's UUID-keyed routes only *imply* a JSON backend — unconfirmed, every ANAC host is
  unreachable from the sandbox. IT stays unintegrated, for engineering reasons now.
- `landing-page-design.md`: retired "search dock is grayscale + disabled / not functional."
  Verified live — real `<input>` → `useLandingSearch` → `tenderClient.searchTenders`, debounced,
  five-state machine, results render upward, carry-over into the first-run profile. Recorded
  that the disabled teaser *moved* to `features/account/.../search-dock`, so
  `landing.search.hint` is still live copy and must not be deleted as dead.
- Flagged, not rewritten: `docs/gtm/feature-growth-priorities.md` is built end-to-end on a
  waitlist premise dropped 2026-07-16 (25 mentions, a whole capture section, an invite-referral
  loop keyed on `waitlist_signup`, and a false quote of the closing CTA). Noted in
  `landing-page-design.md` next to the waitlist-drop record.
- `code-organization-principles.md`: recorded that mechanical boundary enforcement now EXISTS
  (`services/backend/boundary_test.go`, `go list`-based, shrinking 5-entry allowlist) — the
  page previously described it only as a general principle and `.claude/rules/code-organization.md`
  still says it is "not yet in place" (proposed rules fix, not applied).
- Orphans fixed: `vite-plugin-seo` gained an inbound link from `landing-page-design`;
  `system-design-principles`, `code-organization-principles` and `ai-coding-workflow-principles`
  gained inbound links from the new pages. No orphans remain.
- No dangling wikilinks, no index drift after the update. `check.mjs` passes.
- Open questions carried forward: what share of Italian eForms notices set
  `selection-criteria-source = epo-sub-espd` (answer from the stored corpus, not the network);
  whether PVL exposes a consumer-readable JSON backend; and three docs that under-describe the
  tree — `CLAUDE.md` (no `services/ingestion`, no `go-services/*`),
  `.claude/rules/infrastructure.md` (no ingestion CronJobs), `.claude/rules/code-organization.md`
  (the stale enforcement claim above).

## [2026-07-26] ingest | landing-usp-redesign
- Corrected a stale page rather than trusting its "updated" date: `vite-plugin-seo.md`
  (last touched 2026-07-01) claimed static/identical meta and no hardcoded canonical — both
  false in current code (verified directly against `index.ts`/`locale-pages.ts`/
  `vite.config.ts`/`server.go`). Rewrote it: `writeBundle` -> `localizeIndexHtml` re-emits
  per-locale `dist/<locale>/index.html` (localized title/description/OG/Twitter/`html lang`/
  self-canonical — the no-canonical decision is now reversed/FAQPage JSON-LD/noscript hero
  block), `llms.txt` generation is new, and the SPA (`landing-page/index.tsx`) also
  resyncs 5 meta tags client-side on mount. Net: `landing.meta.*` copy edits propagate to
  SEO with zero code change — confirmed while shipping this session's hero/meta rewrite.
- Updated `eu-coverage-section.md`: reframed story is TED-native (always-on badge, pan-EU
  baseline) + national portals (the lit flags); stat-box now leads on `{total}` "Markets
  mapped", hides the "Live" row until `availableCount > 0`; new copy keys `tedNative`,
  `nationalPortals`, `statusMapped`; explicit that IT is not integrated (ANAC is
  retrospective, live/in-rollout portals are PL/FR/ES); noted the section now renders
  before Assurance. Grayscale logic and `useCoverage` confirmed untouched.
- Updated `landing-page-design.md`: new hero H1 asserts the intersection USP directly
  (replacing a rhetorical-question framing) and the primary CTA now converts (routes to
  signup instead of an in-page anchor); section order changed again (Coverage now before
  Assurance, supersedes the 2026-07-16 order) and the light/dark rhythm paragraph updated
  to match; scroll-aware header CTA salience (ghost at top, filled once scrolled); agents
  eyebrow; a verified layout gotcha (hero H1 `max-w-[15ch]` is a no-op — width is capped by
  the `md:grid-cols-[1.05fr_0.95fr]` hero column, not the `ch` utility; parked as a design
  call); a naming gotcha (two distinct `Button` components — the landing atom vs.
  `@tendersbay/components/core`'s — don't conflate their props).
- New page `landing-analytics.md` (project): the landing PostHog house idiom, the full
  event table (2 pre-existing + 3 new: `landing_cta_clicked`, `signup_started`,
  `proof_strip_viewed`), the `location`/`entry` shared-vocabulary funnel join, how `entry`
  rides the signup route's `?entry=` search param end to end, and a deferred non-blocking
  gotcha (`signup_started`'s mount effect double-fires under React StrictMode in dev only,
  not in prod or tests).
- Updated `index.md`: landing-page-design/eu-coverage-section hooks refreshed, new
  `landing-analytics.md` entry, vite-plugin-seo hook updated to mention llms.txt +
  per-locale localized meta instead of the stale "static meta" description.
- Skipped as already recorded / plan-only: the 24-locale translation mechanics (already
  `locale-namespace-insertion.md` + `.claude/rules/frontend.md`), the PostHog idiom's
  consent-gating/super-property mechanics (already established precedent, restated only
  inside the new page's own scope), the per-task SDD process notes and pre-existing dev
  debt (16 tsc errors, one Biome failure — unrelated WIP, not this branch's problem, surfaced
  to the user directly rather than filed in the wiki).

## [2026-07-18] ingest | feedback-timing bias + tension-model personas
- Source transcript (neuroscience/team-synchronization + course-design case study) was
  mostly not applicable — live-workshop facilitation research and a Typeform/Airtable/
  Stripe course-registration stack, neither relevant to this codebase. Asked the user
  to scope before writing anything rather than force-fitting weak content; confirmed
  scope was the two portable findings below, no new memory page.
- `.claude/agents/product-strategist.md`: Empathize remit now notes the tension-model
  technique (map user needs onto opposing-axis pairs) as an alternate persona-discovery
  lens for a genuinely new segment — explicit that tendersbay's existing 3 personas
  aren't to be regenerated by default. Evidence discipline section gained a note that
  feedback collected immediately after an experience reads systematically warmer than
  next-day feedback from the same person — a timing variable, not noise.
- `.claude/agents/neuro-ux-designer.md`: matching note added to the hypothesis-driven
  method section, since it also cites user feedback as evidence.

## [2026-07-18] ingest | UX psychology + visual-design findings
- Checked `neuro-ux-designer.md`'s existing behavioral toolkit before adding anything:
  smart defaults and endowed-progress were already named (Cognitive load /
  Memory & motivation bullets) — not duplicated. Added four genuinely new toolkit
  entries: reciprocity, IKEA/endowment effect (both under a new "Investment &
  reciprocity" bullet), loss aversion/status-quo bias (elevated from "used once in
  landing-page-design.md" to a named toolkit entry, bounded by the existing
  no-dark-patterns rule — must point at a real risk), and contrast/anchoring
  (forward-looking, for whenever a pricing surface exists).
- Deliberately did not port the video's specific stats (24-vs-6 jam flavors, "70-90%
  keep defaults", "2000% sample lift") into the toolkit — cited studies are
  real but commonly mis-quoted in pop-psychology retellings; kept the named
  principle, dropped the numbers.
- Verified two component gaps against actual source before writing them down (not
  assumed from the video): `Button` (`packages/components/src/core/components/atoms/button`)
  has hover/pressed/focus-visible/disabled per variant but no loading/pending state;
  `Banner`'s `Tone` union is `'error' | 'success'` only, no `warning`. Both added to
  `core-component-kit.md`'s polish backlog.
- New page `spacing-and-visual-rhythm.md` (reference): relationship-strength ->
  proximity, margin lives on the larger element in a pair, and the optical-correction
  gotcha for padding around text (font bounding-box vs. rendered glyph height
  mismatch). Linked from `core-component-kit.md`.
- Did not touch `.claude/rules/frontend.md` or add a new rule: generic UI/UX
  guidance (hierarchy, typography scale, color theory, dark mode, shadows, icons,
  overlays) is already the `ui-ux-pro-max` plugin skill's job per
  `landing-page-design.md` ("use the ui-ux-pro-max skill for UI decisions") —
  duplicating it into a rule file would fork one of two design authorities.

## [2026-07-18] ingest | AI-coding-workflow discipline integrated into /prd
- New page `ai-coding-workflow-principles.md` (reference): foundation-over-blank-slate,
  build-order phasing for cross-layer features, explicit per-phase scope; also notes that
  `superpowers:brainstorming`'s clarifying dialogue already supersedes the "compile a
  high-quality prompt before planning" pattern, so that pattern was deliberately not added.
- Updated `.claude/agents/product-strategist.md`: standing brief now reads
  `system-design.md`/`code-organization.md`; Prototype remit briefs `code-architect` with
  them and phases cross-layer MVP cuts in build order (data model/contracts → backend wiring
  → UI/polish).
- Updated `.claude/skills/prd/SKILL.md`: step 5 (Prototype) and the PRD template's "Scope:
  MVP → later" section both require grounding in the rules foundation and build-order
  phasing with explicit in/out-of-scope per phase; added a matching bullet under Rules.
- Updated `.claude/rules/code-organization.md`: new "Build order follows the dependency
  rule, reversed" section — build order is the dependency rule read backwards (foundation
  first, UI last), distinct from the existing call-direction rule.
- Updated `.claude/agents/software-architect.md`'s review checklist to also check a
  multi-phase plan's build order against this.
- Not everything from the source transcript ported: model/harness choice (Cursor-specific,
  no Claude Code analog) and the "compile a refined prompt before planning" step (superseded
  by brainstorming's dialogue, see above) were deliberately left out rather than force-fit.

## [2026-07-18] ingest | govulncheck wired into services/backend
- Added `golang.org/x/vuln/cmd/govulncheck` as a pinned Go tool dependency
  (`go get -tool ...` → `tool` directive in `services/backend/go.mod`, version locked via
  `go.sum` like any other dependency) and wired it into `lint`:
  `gofmt -l . && go vet ./... && go tool govulncheck ./...`.
- First run found a real, currently-unpatched finding: GO-2026-5856 (Encrypted Client Hello
  privacy leak in `crypto/tls`), fixed in go1.26.5; local toolchain here is go1.26.4. Both
  Dockerfiles pin the floating `golang:1.26-alpine` tag and CI's `actions/setup-go@v5` uses
  `go-version: '1.26'` (also floating), so built/deployed artifacts likely already pick up
  the fix — the exposure is stale *local* dev toolchains. Not fixed as part of this change
  (a local Go install upgrade, not a repo change); flagged to the user directly.
- Verified `go tool govulncheck ./...` directly (works, produces the finding above). Running
  it through `pnpm exec turbo run lint --filter backend` on this Windows/Git-Bash machine
  fails separately with "GOCACHE is not defined and %LocalAppData% is not defined" — looks
  like Turborepo's default env-var passthrough doesn't include `LocalAppData`, which Go's
  build cache needs on Windows. Likely pre-existing (not caused by this change) and specific
  to Windows local runs, not the Linux CI runner; not investigated further or fixed here —
  worth a look if a Windows contributor's local `turbo run lint` needs to work.
- Also surfaced (pre-existing, untouched): `gofmt -l` flags `internal/adapter/probe/probe.go`,
  `internal/core/auth/auth_test.go`, `internal/core/user/user_test.go` as unformatted. Not
  part of this change's scope.
- Updated `.claude/rules/code-organization.md`'s supply-chain section: Go vs. pnpm have
  different threat models (Go has no install-script attack surface, so no
  `minimumReleaseAge` equivalent is needed; `GOSUMDB`/`go.sum` already pin checksums;
  `govulncheck` fills the known-CVE gap instead).

## [2026-07-18] ingest | minimumReleaseAge applied
- Added `minimumReleaseAge: 10080` (7 days) to `pnpm-workspace.yaml`, alongside the existing
  `allowBuilds` gate. `pnpm install` verified clean against it (445 lockfile entries all pass).
- Updated `.claude/rules/code-organization.md`'s supply-chain section to state this as done,
  not a recommendation — supersedes that line in the entry below.

## [2026-07-18] ingest | code-organization framework
- New page `code-organization-principles.md` (reference): layered dependency-direction
  framework — ownership over folders, one-directional layer dependencies, business logic
  confined to one layer, mechanical boundary enforcement, the AI-agent amplification effect,
  naming consistency, supply-chain hygiene (build-script gating + minimum release age).
- New rule `.claude/rules/code-organization.md`: tendersbay-xyz-specific layer mapping —
  UI (`apps/platform/src/features`) → transport (`lib/api` / `internal/adapter/{httpapi,connectapi}`)
  → domain (`internal/core`) → capabilities/vendors (`internal/adapter/{postgres,redis,email}`)
  → supporting foundations (`packages/*`); explicit dependency rule (frontend never imports a
  vendor SDK, core never imports adapter); flags Biome/`go vet` import-boundary enforcement
  and `pnpm-workspace.yaml`'s `minimumReleaseAge` as not-yet-implemented recommendations, not
  claims of current state; @-imported into CLAUDE.md.
- Extended `.claude/agents/software-architect.md` (not a new agent — reuse over sprawl): now
  reads and reviews against both `system-design.md` and `code-organization.md`.
- Direct addition (not a `capture-learnings` ingest of an executed plan), run through the
  same page-format/index/log conventions; no source-provenance framing per prior feedback.

## [2026-07-18] ingest | system-design framework
- New page `system-design-principles.md` (reference): full 12-point decision framework
  (stateless servers, horizontal/vertical scaling, load-balancer strategy, microservices
  threshold, API gateway, authN/authZ split, presigned-upload pattern, event-broker fan-out,
  cache-vs-CDN, rate limiting, meta-rule).
- New rule `.claude/rules/system-design.md` (not memory — durable, everyone-facing
  convention): tendersbay-xyz-specific trigger→action version of the same rules, tied to
  services/backend's hexagonal layout and infrastructure/kubernetes's Traefik/channel setup;
  @-imported into CLAUDE.md.
- New agent `.claude/agents/software-architect.md`: applies the rule checklist to design
  reviews (report-only by default) and scaffolds the result on request, following
  git-flow.md/infrastructure.md conventions.
- Not a `capture-learnings` ingest of an executed plan — a direct addition, run through the
  same page-format/index/log conventions.

## [2026-07-01] migrate | initial project-memory import
- Migrated 5 project/reference pages from harness memory into `.claude/memory/`.
- Personal pages (user-working-style, parallel-wip-commit-hygiene) left in harness memory.
- Built index.md; no dangling links after migration.

## [2026-07-13] ingest | redesign-surfaces (Phase 4)
- Updated `react-aria-motion-gotchas.md`: a concrete `isInvalid` boolean (even false) on
  RAC TextField permanently disables native constraint-validation display; wrappers must
  forward undefined when uncontrolled; Form-submit → aria-invalid regression-test pattern.
- Updated `core-component-kit.md`: Phase 4 additions (native Select, Banner tone→role,
  Switch min-h-10, tabClass), Field TextFieldProps-only pass-through + JS clamping,
  kit-adoption checklist (enumerate every dropped native validation attribute and prove
  it preserved), deferred kit polish backlog (labelHidden, link-button recipe, mounted
  Banner live region, etc).
- New page `locale-namespace-insertion.md` (reference): 24-locale namespace splice recipe
  (key inventory from code → anchor-splice script → biome → <ns>-keys completeness test);
  precedents shell/today/account.
- Personal note (mapping-based task briefs work for mechanical restyles, but per-task
  review still catches behavior regressions) → harness memory, not committed here.
- Skipped as already recorded: batch locale-edit basics (.claude/rules/frontend.md),
  commit/staging hygiene (git-flow rule), plan-only details (specific per-page mappings).

## [2026-07-11] ingest | redesign-foundation (Phase 0)
- New page `core-component-kit.md` (project): @tendersbay/components/core conventions —
  cn helper, signal tokens, kit rules, RAC peer lockstep, Fraunces + deferred opsz decision.
- New page `tailwind-v4-shared-kit-css.md` (reference): Tailwind v4 @source for
  node_modules kit sources; extendTailwindMerge registration for non-stock utility scales.
- Updated `frontend-ui-stack.md`: inbound link to the kit page (check the kit before
  hand-rolling primitives in an app).
- Personal/feedback lessons (implementer --no-verify vs Biome; task-brief Windows-path
  bug) routed to harness memory, not committed here.
- Skipped as already recorded: Biome-owns-formatting (CLAUDE.md), staged-paths-only
  commit hygiene (git-flow rule).
