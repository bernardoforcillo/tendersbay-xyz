# AGENTS.md

This repository's conventions are documented for AI coding agents in general, not just
Claude Code. If you are Codex, Antigravity, Cursor, or another agent working on this
codebase, read the following before making any change.

## Start here

- **[CLAUDE.md](./CLAUDE.md)** — the primary instructions file: stack, commands,
  conventions, and workflow. Claude Code auto-loads it and its `@.claude/rules/...` /
  `@.claude/memory/index.md` references at session start; that `@path` inclusion syntax
  is Claude-Code-specific, so if your tool doesn't resolve it automatically, open the
  files below directly.

## Folders to read

- **[.claude/rules/](./.claude/rules/)** — the enforced conventions CLAUDE.md imports:
  [frontend.md](./.claude/rules/frontend.md) (app structure, `~` alias, atomic-design
  components), [git-flow.md](./.claude/rules/git-flow.md) (branching, commit hygiene,
  worktrees), [infrastructure.md](./.claude/rules/infrastructure.md) (Kubernetes/Flux
  layout), [code-organization.md](./.claude/rules/code-organization.md) (layering and
  dependency direction), [system-design.md](./.claude/rules/system-design.md)
  (scaling/service-boundary triggers), [memory-wiki.md](./.claude/rules/memory-wiki.md)
  (how the project memory wiki works).
- **[.claude/memory/](./.claude/memory/)** — the project's accumulated knowledge wiki;
  start at [index.md](./.claude/memory/index.md) for the catalog.
- **[.claude/skills/](./.claude/skills/)** — repo-specific workflows (e.g. `commit`,
  `gtm`, `ux`) written for Claude Code's skill system. They aren't directly executable
  by other agents, but document the intended workflow for those tasks and are worth
  reading before improvising an equivalent.

## Before contributing

This repository is source-available, not open source — see **[license.md](./license.md)**
and **[contributing.md](./contributing.md)** for what's permitted (forking only to
prepare a pull request) and the contribution process/CLA. Any agent preparing a change,
regardless of which tool is driving it, is expected to follow the same fork → branch →
PR workflow and is bound by the same terms as a human contributor.
