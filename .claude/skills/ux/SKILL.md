---
name: ux
description: Use when the user asks for in-product UX work on tendersbay — flow audits, retention mechanics, onboarding/activation friction, hypothesis-driven UX changes with PostHog measurement — or invokes /ux <task>. Dispatches the neuro-ux-designer subagent and relays its UX report.
---

# /ux — dispatch the neuro-UX designer

Delegate the task to the `neuro-ux-designer` subagent
(`.claude/agents/neuro-ux-designer.md`) instead of doing UX work inline. It
carries the standing brief (behavioral toolkit, tone law, component-kit
rules, 24-locale recipe, PostHog rules, the no-dark-patterns ethic) so the
main session doesn't have to.

## How to dispatch

1. **Compose the prompt**: the user's task verbatim, plus any conversation
   context the agent can't discover from the repo (the flow or screen in
   question, deadlines, links the user pasted, whether they want an audit
   only or changes shipped).
2. **Pick isolation**:
   - Task will **edit files** (components, locales, instrumentation, docs) →
     `Agent` tool with `subagent_type: "neuro-ux-designer"`,
     `isolation: "worktree"`. The user works in parallel; a worktree keeps
     UX edits out of their WIP.
   - Task **produces no files at all** — findings come back only in the
     report (a flow audit, a live-page walkthrough) → same agent, no
     isolation. Anything that writes a file, including a `docs/gtm/`
     findings doc, takes the worktree branch above.
3. **Relay the report**: the agent's final message is a UX report (findings
   with principles, changes, evidence, experiments, next moves). Surface it
   to the user in full — including the worktree path where the edits live,
   so they can review and commit from there.

## Rules

- Never commit the agent's output yourself; the user reviews and commits
  (per-file staging, `/commit` skill).
- If the task mixes audit + edits, one dispatch with worktree isolation is
  fine — the agent handles both in a single run.
- Multiple independent UX tasks → dispatch multiple `neuro-ux-designer`
  agents in parallel, each in its own worktree.
- The designer may peer-dispatch other agents itself (gtm-engineer,
  growth-marketer); its report embeds theirs — you still relay one report.
- Growth/launch strategy is not this skill's remit — that's `/growth`;
  direct GTM implementation (copy, SEO, events) with no UX-flow angle is
  `/gtm`.
