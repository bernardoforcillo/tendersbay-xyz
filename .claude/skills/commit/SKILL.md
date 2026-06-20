---
name: commit
description: Use when the user asks to commit changes, stage work, or write a git commit message in this repository — produces a Conventional Commits message that passes commitlint and reads like a senior engineer wrote it.
---

# Commit

Write git commits that pass this repo's `commit-msg` hook (commitlint with
`@commitlint/config-conventional`) and that a senior engineer would be proud of:
atomic, clearly typed, imperative, and explaining **why** the change was made.

## A good commit IS

1. **Atomic** — one logical change. Unrelated edits go in separate commits.
2. **Conventionally typed** — `type(scope): subject` header.
3. **Imperative & specific** — "add tender filters", not "added stuff".
4. **Motivated** — the body says *why*, not just *what* (the diff already shows what).

## Header format

```
type(scope): subject
```

Rules enforced by commitlint (the hook rejects violations):

| Rule | Requirement |
|------|-------------|
| `type` | Required. One of the list below. Lower-case. |
| `scope` | Optional. Lower-case. Use the workspace/package name. |
| `subject` | Required. Imperative mood. **No** leading capital. **No** trailing period. |
| header length | ≤ 100 chars (aim for ≤ 72). |
| `!` before `:` | Marks a breaking change, e.g. `feat(api)!: …`. |

**Types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`,
`ci`, `chore`, `revert`.

**Scopes (monorepo):** the affected workspace — an app dir name (`web`, `api`)
or a package name (`ui`, `config`). Root-level tooling changes (`turbo.json`,
`biome.json`, hooks) may omit the scope or use the tool name.

## Body and footers

- Leave a **blank line** after the subject before the body.
- Body lines ≤ 100 chars. Explain motivation, trade-offs, and anything a
  reviewer can't infer from the diff.
- Footers go after a **blank line**. Use them for:
  - Issue refs: `Closes #142`, `Refs #98`
  - Breaking changes: `BREAKING CHANGE: <what broke and the migration>`
  - Co-authors (if an agent or pair authored it): `Co-Authored-By: Name <email>`

## Workflow

1. **Understand the change** — `git status` and `git diff` (and `git diff --staged`).
2. **Make it atomic** — if the working tree mixes unrelated changes, stage only
   the related hunks (`git add <paths>`) and commit them separately.
3. **Classify** — pick the `type`, the narrowest accurate `scope`, and write an
   imperative subject.
4. **Write the body** — why this change, now; note breaking changes and issues.
5. **Commit** with a real multi-line message (don't cram everything in `-m` one-liners):
   ```sh
   git commit -m "feat(web): add tender search filters" \
     -m "Users could only browse the full list. Add status, region, and deadline" \
     -m "filters so they can narrow results before opening a tender." \
     -m "Closes #142"
   ```
   Each `-m` becomes a paragraph separated by a blank line.
6. **Respect the hooks** — `pre-commit` runs `pnpm lint` and `commit-msg` runs
   commitlint. If either fails, fix the cause and retry. **Never** bypass with
   `--no-verify`.

## Examples

```
feat(web): add tender search filters

Users could only browse the full tender list. Add status, region, and
deadline filters so they can narrow results before opening a tender.

Closes #142
```

```
fix(api): handle missing deadline in tender serializer

Draft tenders have no deadline, which made the serializer throw and
return a 500. Default to null and let the client render "TBD".
```

```
refactor(db)!: rename tenders.owner_id to created_by

BREAKING CHANGE: the tenders API now returns `createdBy` instead of
`ownerId`. Update any client reading `ownerId`.
```

```
chore: pin pnpm to 11.8.0 via packageManager
```

## Common mistakes

| Mistake | Fix |
|---------|-----|
| `Fix bug` (capitalized) | Lower-case subject: `fix: …` |
| `fixed the login bug` (past tense) | Imperative: `fix: handle expired session` |
| `fix: update stuff.` (vague + period) | Be specific, drop the trailing period |
| `feat: …` for a bug fix | Match intent — use `fix:` |
| Bundling unrelated edits | Split into atomic commits |
| `git commit --no-verify` to skip a failing hook | Fix the lint/message; never bypass |
