---
name: growth
description: Use when the user asks for growth or launch strategy on tendersbay — launch plans, channel/audience strategy, waitlist and referral mechanics, neuromarketing message framing — or invokes /growth <task>. Dispatches the growth-marketer subagent, which itself peer-dispatches the gtm-engineer to implement the handoff when build was asked for.
---

# /growth — dispatch the growth marketer

Delegate the task to the `growth-marketer` subagent
(`.claude/agents/growth-marketer.md`) instead of doing growth work inline.
It carries the standing brief (neuromarketing toolkit, network-launch
playbook, tone law, pre-launch constraints) — and it chains the
`gtm-engineer` itself, by peer dispatch inside its own worktree, when the
ask includes implementation.

## How to dispatch

1. **Compose the prompt**: the user's task verbatim, plus context the agent
   can't discover from the repo (target market/locale, timing, channels
   already tried, links pasted) — and **state explicitly whether the user
   wants strategy only or strategy + build**. The marketer peer-dispatches
   gtm-engineer only when the prompt says build was asked for.
2. **Pick isolation**: strategy briefs are files in `docs/gtm/`, so `Agent`
   tool with `subagent_type: "growth-marketer"`, `isolation: "worktree"` is
   the default. Only a purely verbal research question — no files at all —
   goes without isolation.
3. **Relay the growth report** in full — when the chain ran it embeds the
   engineer's condensed report — including the single worktree path where
   everything lives, so the user can review and commit from there.

## Rules

- Never commit any agent's output yourself; the user reviews and commits
  (per-file staging, `/commit` skill).
- Never publish, send, or post anything external — the agents only draft.
- No strategy step needed (the ask is directly implementable copy/SEO/events
  work) → skip this skill and use `/gtm`. In-product flow and retention work
  → `/ux`.
- Multiple independent growth tracks → parallel marketer dispatches, one
  worktree each.
