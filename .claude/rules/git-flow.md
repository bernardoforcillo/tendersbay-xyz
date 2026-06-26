# Git flow & release policy

Applies to the whole monorepo. Branch merges map to Docker image channels via the
platform CI workflow (`.github/workflows/ci-platform.yml`).

## Branch roles

- `feature/*`, `fixes/*`, `chore/*` — short-lived work branches off `dev`, prefixed
  by change type. Open a PR **into `dev`**. CI runs lint, type-check, and tests;
  **no image is published**.
- `dev` — integration / development branch. Merges here publish a **canary** image:
  `tendersbay-platform:<UTC-timestamp>-<sha>-canary`.
- `main` — production / stable. Promoted by merging `dev → main` via PR. Publishes
  the **stable** image `tendersbay-platform:<UTC-timestamp>-<sha>` plus the moving
  `latest` tag.

## Flow

```
feature|fixes|chore/* ──PR──▶ dev ──PR──▶ main
       (CI only)              (canary)    (stable + latest)
```

Promotion is strictly work-branch → `dev` → `main`. Never push work directly to
`main`.

## Image tags

| Branch | Tags pushed to Docker Hub                |
| ------ | ---------------------------------------- |
| `dev`  | `<YYYYMMDDHHMMSS>-<full-sha>-canary`      |
| `main` | `<YYYYMMDDHHMMSS>-<full-sha>`, `latest`   |

Every build keeps an immutable `timestamp-sha` tag, so any commit's image can be
pulled even after `latest` moves on. The `-canary` suffix is the only marker that
distinguishes a `dev` build — there are no moving channel tags.

## Commit hygiene

Work branches often carry **unrelated in-progress changes** (multiple features
are developed in parallel on the same branch). So:

- **Stage only the files your change touches** — `git add <specific paths>`, never
  `git add -A` / `git add .`. Don't sweep someone else's half-finished work into
  your commit.
- **Format only your own files** (`pnpm exec biome check --write <paths>`), not a
  repo-wide `pnpm format`, to avoid reformatting unrelated WIP.
- If a project-wide check (`pnpm check`) fails, confirm the error is in *your*
  files before reacting — it may come from someone else's uncommitted WIP (e.g. a
  new test importing a module that isn't written yet).
- The `pre-commit` hook runs `pnpm lint` (Biome) over the **whole tree**, so unrelated WIP
  with lint errors blocks an otherwise-clean, path-scoped commit. When your staged files are
  themselves clean (`pnpm exec biome check <paths>` passes), commit them with
  `git commit --no-verify` — you've already linted exactly what you're committing.
- **History can move under you.** On a shared work branch someone may rebase or reset in
  parallel, so a commit you made can drop off the tip (its objects survive and are
  recoverable by SHA / reflog). Re-check `git rev-parse HEAD` and `git log` after any gap
  before assuming your last commit is still HEAD, and re-stage from the working tree if
  needed.

## Parallel work: prefer git worktrees

When more than one feature is in flight at once (the common case here), the **preferred**
setup is **one git worktree per feature**, each on its own `feature/*` branch off `dev`:

```sh
git worktree add ../tb-<feature> -b feature/<feature> dev   # then: pnpm install
```

Each worktree is a single-purpose working tree, which removes every commit-hygiene hazard
above at the source: `git add -A` and `pnpm format` only ever touch one feature's files, the
whole-tree `pre-commit` lint can't be blocked by someone else's WIP, and one stream of work
can't rewrite history under another. Per-worktree caveat: `node_modules` is **not** shared —
run `pnpm install` in each (pnpm's global store keeps this cheap); each worktree builds its
own `dist/`. Remove a finished one with `git worktree remove ../tb-<feature>`.

This is the **recommended default, not a mandate — the final choice is the user's.** Suggest
a worktree when work would otherwise share a tree with unrelated WIP, but never create or
switch worktrees, or move the user's uncommitted work, without asking first.
