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
