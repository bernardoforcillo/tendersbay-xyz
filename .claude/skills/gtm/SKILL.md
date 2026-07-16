---
name: gtm
description: Use when the user asks for go-to-market work on tendersbay — landing/positioning copy, SEO, PostHog conversion events/funnels/experiments, launch plans, channel or keyword research — or invokes /gtm <task>. Dispatches the gtm-engineer subagent and relays its GTM report.
---

# /gtm — dispatch the GTM engineer

Delegate the task to the `gtm-engineer` subagent (`.claude/agents/gtm-engineer.md`)
instead of doing GTM work inline. It carries the standing brief (positioning, tone
law, 24-locale recipe, PostHog rules) so the main session doesn't have to.

## How to dispatch

1. **Compose the prompt**: the user's task verbatim, plus any conversation context
   the agent can't discover from the repo (deadlines, target channel, locale to
   author first, links the user pasted).
2. **Pick isolation**:
   - Task will **edit files** (copy, locales, SEO config, instrumentation, docs) →
     `Agent` tool with `subagent_type: "gtm-engineer"`, `isolation: "worktree"`.
     The user works in parallel; a worktree keeps GTM edits out of their WIP.
   - Task **produces no files at all** — findings come back only in the report
     (keyword research, competitor scan, live-page audit) → same agent, no
     isolation. Anything that writes a file, including a `docs/gtm/` brief,
     takes the worktree branch above.
3. **Relay the report**: the agent's final message is a GTM report (changes,
   evidence, findings, next moves). Surface it to the user in full — including the
   worktree path where the edits live, so they can review and commit from there.

## Rules

- Never commit the agent's output yourself; the user reviews and commits
  (per-file staging, `/commit` skill).
- If the task mixes research + edits, one dispatch with worktree isolation is
  fine — the agent handles both remits in a single run.
- Multiple independent GTM tasks → dispatch multiple `gtm-engineer` agents in
  parallel, each in its own worktree.
- Routing: defining a *new* feature's why/who/what before any build →
  `/prd` (the design-thinking desk; its PRD then feeds
  `superpowers:brainstorming`). Strategy-first work (launch plans, channels,
  referral mechanics) → `/growth` (whose marketer chains gtm-engineer for
  implementation); in-product flow/retention work → `/ux`. `/gtm` stays the
  direct implementation route.
- The engineer may peer-dispatch other agents itself (growth-marketer,
  neuro-ux-designer); its report embeds theirs — you still relay one report.
