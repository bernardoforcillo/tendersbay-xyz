# tendersbay-xyz

A pnpm + Turborepo monorepo. Applications live in `apps/`, shared libraries in
`packages/` (both currently empty).

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

## Conventions

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
