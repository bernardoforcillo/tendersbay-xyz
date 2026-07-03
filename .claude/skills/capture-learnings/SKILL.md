---
name: capture-learnings
description: Use at the end of executing a plan (or when the user asks to capture lessons / update memory) to distil reusable knowledge into the .claude/memory wiki — ingest a finished plan's lessons or lint the wiki's health. Never auto-commits.
---

# Capture learnings

Maintain the project's compounding knowledge base (`.claude/memory/`) using the
LLM Wiki pattern. You do the bookkeeping — summarizing, cross-referencing, filing,
dedup — so the next session starts with accumulated knowledge. Conventions live in
@.claude/rules/memory-wiki.md; read it before operating.

## Modes

- `/capture-learnings [plan-path]` → **ingest** (default).
- `/capture-learnings lint` → **lint** (health-check only).

## Ingest

1. **Locate the source.** Use `plan-path` if given; else pick the most recently
   completed plan in `docs/superpowers/plans/` (confirm with the user if ambiguous).
2. **Read + reflect.** Read the plan/spec and reconstruct the session's lessons:
   what was non-obvious, what needed retries, decisions and their rationale, gotchas.
3. **Extract candidates.** Actionable, compounding lessons only. YAGNI filter: drop
   anything the repo already records — code structure, git history, facts already in
   CLAUDE.md/rules, or notes that mattered only to this one conversation.
4. **Route** each with the rubric in @.claude/rules/memory-wiki.md. Default gravity is
   `.claude/memory/`; personal notes go to the harness memory; propose (don't apply)
   promotions to `.claude/rules/` or a new skill.
5. **Integrate, don't append.** Search `index.md` + grep pages for a page it belongs
   to. Update the existing page (revise; note contradictions with older claims) rather
   than duplicate. Add/strengthen `[[links]]`; ensure the page has an inbound link (no
   orphans). Bump `metadata.updated`; add the plan path to `metadata.sources`.
6. **Update `index.md`** (one line per page, correct category) and **append `log.md`**
   (`## [<date>] ingest | <topic>`). Use today's date from the environment context.
7. **Gate.** Run `node .claude/memory/check.mjs` — it must pass.
8. **Report, don't commit.** Print a "what I saved and where" summary (a per-file
   bullet list, plus any proposed promotions). Let the user review and commit — respect
   selective staging (`git add .claude/memory ...`, never `-A`).

## Lint

Read every page + `index.md` and report (fix only on approval):
contradictions between pages; stale claims (verify referenced files/flags still exist
in the codebase; `updated` older than a superseding plan); orphans (no inbound
`[[link]]`); dangling `[[links]]`; concepts referenced repeatedly but lacking a page;
index drift. End with `node .claude/memory/check.mjs`. Suggest new questions/sources
to investigate. Append a `## [<date>] lint | <scope>` entry to `log.md`.

## Guardrails

- Never auto-commit; never edit the wiki without showing the summary first.
- Personal `user`/`feedback` notes never enter `.claude/memory/` — they go to the
  harness memory.
- No emoji. Keep pages terse; one fact/topic per page.
