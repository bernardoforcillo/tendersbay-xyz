# Memory wiki

The project's compounding knowledge base lives at `.claude/memory/` — a committed,
git-versioned, Obsidian-browsable markdown wiki (the "LLM Wiki" pattern). The
`capture-learnings` skill maintains it; you rarely hand-edit it.

## Two stores, one maintainer

| Store | Location | Committed | Holds |
| --- | --- | --- | --- |
| **Project wiki** | `.claude/memory/` | Yes | `type: project`, `reference` |
| **Personal memory** | `~/.claude/projects/<slug>/memory/` | No | `type: user`, `feedback` |

Project/reference knowledge → the repo wiki. Personal notes (who the user is,
how I should work) → the harness memory, never committed.

## Page format

```markdown
---
name: <slug>                 # kebab-case, equals the filename
description: <one-line>       # recall relevance + index hook
metadata:
  type: project | reference
  updated: YYYY-MM-DD         # bump on every edit — drives stale detection
  sources: [docs/superpowers/plans/<file>.md]   # provenance, optional
---

<the fact. Link related pages with [[their-slug]] — liberally; a [[slug]] with no
page yet is a to-write marker. For project pages, prefer stating the why.>
```

## index.md and log.md

- `index.md` — content catalog, categorized, one line per page:
  `- [Title](slug.md) — hook`. Every page appears exactly once. `@`-imported by CLAUDE.md.
- `log.md` — append-only. Entry prefix `## [YYYY-MM-DD] <op> | <topic>` where `<op>` is
  `ingest`, `lint`, or `migrate`.

## Operations (run by the capture-learnings skill)

- **Ingest** — after a plan is executed: read the plan + reflect on the session,
  extract compounding lessons, route each (rubric below), **integrate into existing
  pages** (don't duplicate), update `index.md`, append `log.md`. Then run the checker.
- **Lint** — health-check: contradictions, stale claims (verify against code),
  orphans, dangling `[[links]]`, missing pages, index drift. Propose fixes + questions.
- Both end with `node .claude/memory/check.mjs` passing, and **never auto-commit** —
  always show a summary and let the user approve.

## Routing rubric

| Lesson | Destination |
| --- | --- |
| Ongoing project context / constraints / external pointers | `.claude/memory/` (`project`/`reference`) |
| Preference, working style, feedback on how I should work | harness memory (`user`/`feedback`) — not committed |
| Durable convention/gotcha for everyone | **propose** moving to `.claude/rules/*.md` |
| Reusable multi-step workflow | **propose** a `.claude/skills/<name>/` skill |
| Only-this-feature detail | note beside the plan in `docs/superpowers/`, or skip |

## Optional: reminder hook (opt-in, not enabled by default)

A guarded `Stop` hook can nudge you to run `/capture-learnings` after plan work.
It only prints a reminder — it never writes the wiki. To enable, add to
`.claude/settings.json` (or `settings.local.json`):

```json
{
  "hooks": {
    "Stop": [
      { "hooks": [ { "type": "command", "command": ".claude/hooks/capture-learnings-nudge.sh" } ] }
    ]
  }
}
```

The guard (`.claude/hooks/capture-learnings-nudge.sh`) fires only when a plan file
under `docs/superpowers/plans/` was modified recently, so it stays quiet on
unrelated sessions.
