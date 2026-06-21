# tendersbay-xyz

A pnpm + Turborepo monorepo. Applications live in `apps/`, shared libraries in
`packages/`.

- `apps/platform` — Vite + React + TypeScript frontend embedded into and served by a
  Go static server (`//go:embed`); Air provides hot reload in dev.
- `packages/tsconfig` (`@tendersbay/tsconfig`) — shared TypeScript configs.
- `packages/tailwind` (`@tendersbay/tailwind`) — shared Tailwind v4 theme.
- `packages/components` (`@tendersbay/components`) — shared React components.

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

## Conventions

Frontend app structure (the `~` alias, `/<name>/index.tsx` modules, kebab-case names)
is documented in @.claude/rules/frontend.md.

- App components live under `src/features/<name>/…`; the shared `@tendersbay/components`
  library keeps `src/<feature>/…`. App routing/i18n infra (`src/routes/`, `src/i18n/`,
  `src/assets/locales/`) stays outside `features/`.

Branching and the canary release policy (`feature → dev → main`, and the Docker
image tags each branch publishes) are documented in @.claude/rules/git-flow.md.

- Use **pnpm only** — never npm or yarn. Add root dev deps with `pnpm add -Dw <pkg>`;
  add to a workspace with `pnpm add <pkg> --filter <workspace>`.
- A new app goes in `apps/<name>/` and a new library in `packages/<name>/`, each with
  its own `package.json`. pnpm workspaces and Turbo pick it up with no root changes.
- Reference internal packages with the `workspace:*` protocol.
- Let Biome own formatting and linting — don't add ESLint/Prettier configs. Run
  `pnpm format` before committing; the `pre-commit` hook blocks on lint errors.
- Commit messages follow Conventional Commits (e.g. `feat(web): …`, `fix: …`, `chore: …`).
  The `commit-msg` hook rejects non-conforming messages. Use the `/commit` skill to draft them.
- Declare per-package tasks (`build`, `dev`, `lint`, `check`, `test`) as scripts so Turbo
  can orchestrate them; wire cross-package ordering via `dependsOn` in `turbo.json`.
- Design specs (`docs/superpowers/specs/`) and implementation plans
  (`docs/superpowers/plans/`) are local-only working docs — kept on disk, never
  committed (they are gitignored).
- Go code lives in `apps/platform`. Format with `gofmt`, vet with `go vet ./...`
  (wired as the app's `lint`); run tests with `go test ./...` (the app's `test`).
- The Go binary embeds the Vite build with `//go:embed all:dist` — run `vite build`
  before `go build` (the app's `build` script does both in order). A committed
  `apps/platform/dist/.gitkeep` keeps the embed compiling before the first build.
- Biome does not lint Go and its CSS linter is disabled (Tailwind owns CSS semantics).
