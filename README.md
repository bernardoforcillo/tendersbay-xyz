# tendersbay-xyz

A pnpm monorepo managed with Turborepo.

## Stack

- **[pnpm](https://pnpm.io)** — package manager + workspaces
- **[Turborepo](https://turbo.build)** — task orchestration & caching
- **[Biome](https://biomejs.dev)** — linting & formatting
- **[commitlint](https://commitlint.js.org)** — Conventional Commits enforcement
- **[Husky](https://typicode.github.io/husky)** — git hooks

## Requirements

- Node `>=24` (see `.nvmrc`)
- pnpm `11.8.0` (pinned via `packageManager`; run `corepack enable` to match)
- Go `1.26` (for `apps/platform`)
- Air (`go install github.com/air-verse/air@latest`) for Go hot reload in dev

## Getting started

```sh
pnpm install
```

This installs dependencies and sets up git hooks (`prepare` → `husky`).

## Layout

```
apps/
  platform/  # Vite + React frontend, embedded into & served by a Go server (Air in dev)
packages/
  tsconfig/   # @tendersbay/tsconfig — shared TypeScript configs
  tailwind/   # @tendersbay/tailwind — shared Tailwind v4 theme
  components/ # @tendersbay/components — shared React components
```

Add a new app/package by creating a directory with its own `package.json`;
pnpm workspaces and Turbo pick it up automatically.

## Commands

| Command | Description |
|---------|-------------|
| `pnpm build` | `turbo run build` across the workspace |
| `pnpm dev` | `turbo run dev` |
| `pnpm lint` | `biome check` (read-only) |
| `pnpm format` | `biome check --write` (lint + format with autofix) |
| `pnpm check` | `turbo run check` |
| `pnpm test` | `turbo run test` |

## Git hooks

- **pre-commit** → `pnpm lint` (Biome across the repo)
- **commit-msg** → commitlint (Conventional Commits)
