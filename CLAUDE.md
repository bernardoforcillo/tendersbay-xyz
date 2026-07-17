# tendersbay-xyz

A pnpm + Turborepo monorepo. Applications live in `apps/`, standalone backend services in
`services/`, shared libraries in `packages/`.

- `apps/platform` — Vite + React + TypeScript frontend embedded into and served by a
  Go static server (`//go:embed`); Air provides hot reload in dev.
- `services/backend` — standalone Go HTTP service (hexagonal: `internal/core` +
  `internal/adapter/*`), its own `go.mod` and Docker image (`tendersbay-backend`); serves
  `api.tendersbay.xyz`. Unlike `apps/`, a service does not embed the frontend.
- `packages/tsconfig` (`@tendersbay/tsconfig`) — shared TypeScript configs.
- `packages/tailwind` (`@tendersbay/tailwind`) — shared Tailwind v4 theme.
- `packages/components` (`@tendersbay/components`) — shared React components.
- `packages/vite-plugin-seo` (`@tendersbay/vite-plugin-seo`) — Vite plugin that emits
  `robots.txt` + a locale-aware `sitemap.xml` and injects static SEO `<head>` tags
  (meta/OG/Twitter/JSON-LD) at build; wired into `apps/platform/vite.config.ts`.

## Commands

- Install: `pnpm install`
- Build: `pnpm build` (`turbo run build`)
- Dev: `pnpm dev` (`turbo run dev`)
- Lint: `pnpm lint` (`biome check`)
- Format: `pnpm format` (`biome check --write`)
- Check: `pnpm check` (`turbo run check`)
- Test: `pnpm test` (`turbo run test`)

## Stack

- **Package manager:** pnpm 11 — workspaces, pinned via `packageManager`
- **Task runner:** Turborepo 2
- **Lint & format:** Biome 2 (no ESLint or Prettier)
- **Commits:** Conventional Commits, enforced by commitlint
- **Git hooks:** Husky 9 — `pre-commit` runs `pnpm lint`, `commit-msg` runs commitlint
- **Node:** `>=24` (see `.nvmrc`)
- **Backend:** Go `1.26` — `apps/platform` serves the embedded frontend via `net/http`
- **Go hot reload:** Air — `pnpm dev` runs Vite + Air concurrently (install once with
  `go install github.com/air-verse/air@latest`; ensure `air` is on `PATH`)
- **Frontend:** Vite 6 + React 19 + Tailwind CSS v4 (`@tailwindcss/vite`)
- **Routing:** TanStack Router (file-based, `@tanstack/router-plugin`); `routeTree.gen.ts`
  is generated and committed, and excluded from Biome.
- **i18n:** i18next + react-i18next; bundled translations in
  `apps/platform/src/assets/locales/<locale>/common.json`; 24 official EU locales,
  default `en-ie`; public routes are locale-prefixed (`/<locale>/`).
- **Frontend tests:** vitest + jsdom (the app's `test` runs `vitest run` then `go test ./...`).
- **MCP servers:** project-scoped in `.mcp.json` — `posthog` (hosted,
  `https://mcp.posthog.com/mcp`, OAuth: authenticate once via `/mcp`, data
  region auto-routed) and `playwright` (stdio, launched from the root dev dep
  `@playwright/mcp`; run `pnpm exec playwright install chromium` once per
  machine — the browser cache is machine-global, shared across worktrees).

## Conventions

Frontend app structure (the `~` alias, `/<name>/index.tsx` modules, kebab-case names)
is documented in @.claude/rules/frontend.md.

- App components live under `src/features/<name>/…`; the shared `@tendersbay/components`
  library keeps `src/<feature>/…`. App routing/i18n infra (`src/routes/`, `src/i18n/`,
  `src/assets/locales/`) stays outside `features/`.

Branching and the canary release policy (`feature → dev → main`, and the Docker
image tags each branch publishes) are documented in @.claude/rules/git-flow.md.

Kubernetes deployment lives in `infrastructure/kubernetes/`, reconciled by Flux
(GitOps) onto a Traefik + cert-manager + Cilium cluster; the layout, naming, pod
hardening, and image-automation conventions are documented in
@.claude/rules/infrastructure.md.

## Memory wiki

Project knowledge accumulates in `.claude/memory/` — a committed markdown wiki
maintained by the `capture-learnings` skill. Its conventions (page format, routing,
ingest/lint operations) are documented in @.claude/rules/memory-wiki.md.

The catalog is imported here so it loads every session:

@.claude/memory/index.md

- Use **pnpm only** — never npm or yarn. Add root dev deps with `pnpm add -Dw <pkg>`;
  add to a workspace with `pnpm add <pkg> --filter <workspace>`.
- **Dependency build scripts are gated.** This repo treats ignored build scripts as a hard
  error, so a new dep that ships a postinstall (e.g. `core-js` via `posthog-js`) makes
  every `pnpm` task fail with `ERR_PNPM_IGNORED_BUILDS` until you decide. `pnpm add` writes
  a `<pkg>: set this to true or false` placeholder under `allowBuilds:` in
  `pnpm-workspace.yaml` — resolve it: `false` if the build isn't needed (the common case:
  funding-notice postinstalls like `core-js`), `true` to run it. Then `pnpm install` passes.
- A new app goes in `apps/<name>/` and a new library in `packages/<name>/`, each with
  its own `package.json`. pnpm workspaces and Turbo pick it up with no root changes.
- A standalone backend service goes in `services/<name>/` (its own `go.mod`, Dockerfile,
  CI workflow, and k8s app folder under `infrastructure/kubernetes/`). `services/*` is
  already a pnpm workspace glob, so Turbo orchestrates its `build`/`dev`/`lint`/`test`
  scripts like any other workspace.
- Reference internal packages with the `workspace:*` protocol.
- Let Biome own formatting and linting — don't add ESLint/Prettier configs. Run
  `pnpm format` before committing; the `pre-commit` hook blocks on lint errors.
- Commit messages follow Conventional Commits (e.g. `feat(web): …`, `fix: …`, `chore: …`).
  The `commit-msg` hook rejects non-conforming messages. Use the `/commit` skill to draft them.
- Declare per-package tasks (`build`, `dev`, `lint`, `check`, `test`) as scripts so Turbo
  can orchestrate them; wire cross-package ordering via `dependsOn` in `turbo.json`.
- Product requirements (`docs/superpowers/prd/`), design specs
  (`docs/superpowers/specs/`), and implementation plans
  (`docs/superpowers/plans/`) are local-only working docs — kept on disk, never
  committed (they are gitignored). The pipeline is
  `/prd → superpowers:brainstorming → writing-plans → implement`: `/prd` runs a
  design-thinking process (skill `.claude/skills/prd/` + agent
  `product-strategist`) to produce a PRD, which then feeds the technical spec.
- Go code lives in `apps/platform`. Format with `gofmt`, vet with `go vet ./...`
  (wired as the app's `lint`); run tests with `go test ./...` (the app's `test`).
- The Go binary embeds the Vite build with `//go:embed all:dist` — run `vite build`
  before `go build` (the app's `build` script does both in order). A committed
  `apps/platform/dist/.gitkeep` keeps the embed compiling before the first build.
- Biome does not lint Go and its CSS linter is disabled (Tailwind owns CSS semantics).
