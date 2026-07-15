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

The `services/backend` service publishes `bernardoforcillo/tendersbay-backend` on the same
channel scheme via `.github/workflows/ci-backend.yml` (`dev` → `…-canary`, `main` → `…` +
`latest`).

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
- **Never `git commit --amend` on a shared branch.** Git authorship is the same repo user
  for *every* commit (yours and the human's), so you cannot tell them apart by author — only
  by message/content. If the human commits between your `git rev-parse HEAD` check and your
  amend, the amend silently rewrites **their** commit. Make fresh commits only. Recover a bad
  amend with `git reset --soft <their-original-sha>` (from the reflog): it restores their
  commit byte-identical and leaves *your* delta staged to re-commit on top.
- **Committing your hunk in a file the human is also editing (uncommitted):** save their WIP
  with `git diff HEAD -- <file> > wip.patch`, `git checkout HEAD -- <file>`, apply only your
  edit, stage + commit, then `git apply wip.patch` to restore their WIP on top. Works when
  the two edits touch non-overlapping regions (it's a clean text patch, not a 3-way merge).

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

## Agent worktrees → `dev` (port, don't merge)

Subagents dispatched with worktree isolation (the GTM desk — `gtm-engineer`,
`growth-marketer`, `neuro-ux-designer` — and any `Agent` call using `isolation: "worktree"`)
run in a **harness-managed** `.claude/worktrees/agent-*` tree on a throwaway
`worktree-agent-*` branch. That branch is cut from **whatever branch the session is on**, not
necessarily `dev` — so it can sit on top of an unrelated in-flight feature (e.g.
`feature/redesign-explore`). Two consequences:

- **Never merge or PR an agent's `worktree-agent-*` branch straight into `dev`** — its base
  carries commits from the feature branch it happened to fork, and merging drags all of them
  along. The agent tree is disposable scratch, not a release branch.
- **Port the agent's commit onto a fresh `feature/<name>` cut from `dev`, then run the normal
  flow.** Commit the agent's work on its own branch, create `feature/<name>` off `dev`, and
  **`git cherry-pick <sha>`** it across — cherry-pick's 3-way merge reconciles the divergent
  base cleanly where a flat `git apply` would reject (resolve any conflict, it's usually just
  the regions the unrelated base also touched). Verify on the `dev` base (build + tests), then
  PR `feature/<name>` → `dev` (canary) as usual.

Remove the finished agent tree with `git worktree remove .claude/worktrees/agent-<id>` and
delete its `worktree-agent-*` branch once the port is verified.
