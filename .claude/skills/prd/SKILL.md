---
name: prd
description: Use when the user wants to define a new product feature — turn a raw idea or problem into a rigorous PRD through a design-thinking process before any technical design — or invokes /prd <feature>. Facilitates the dialogue in the main loop and dispatches the product-strategist subagent for the heavy research passes.
---

# /prd — product design-thinking facilitator

Turn an idea or problem into a PRD via a five-phase design-thinking process, then hand the
approved PRD to `superpowers:brainstorming` for the technical spec. You (the main loop) are
the **facilitator**: you hold the dialogue and the two human gates. The `product-strategist`
subagent (`.claude/agents/product-strategist.md`) is the **research engine** — dispatch it
for each heavy per-phase pass; it peer-dispatches the specialists and returns condensed
findings.

Pipeline position:

    /prd → docs/superpowers/prd/<feature>.md → superpowers:brainstorming → spec → writing-plans
    (product layer — this skill)               (technical design — existing)

## When NOT to use this

- A bug or purely technical change → `superpowers:systematic-debugging`, or straight to
  `superpowers:brainstorming`.
- Pure GTM/marketing execution → `/gtm`; growth strategy → `/growth`; in-product flow
  polish with no new product definition → `/ux`.
- A tiny, already-well-understood tweak → skip the PRD; go to brainstorming.

`/prd` is for **defining a new feature's why / who / what / success** before solutioning.

## Procedure

**0. Kick off (main loop).** Restate the idea in one line. Confirm it is a
product-definition task; if it is really a bug or pure-execution task, route away (above).

**1. Empathize — dispatch `product-strategist`** (plain, no worktree — it edits nothing)
with an Empathize brief: the idea + any context the user gave. It returns an evidence pack
(who hurts, real PostHog signals, the current surface, personas). Relay a condensed version.

**2. GATE 1 — problem-lock.** Present the problem statement + job-to-be-done + who it is
for. Get explicit user confirmation before ideating. Use `AskUserQuestion` for crisp forks
(which persona, which problem framing).

**3. Define (main loop, with the user).** Synthesise a sharp problem statement, a "How
Might We" reframing, and success criteria. No dispatch needed.

**4. Ideate — dispatch `product-strategist`** for the three-lens pass (gtm-engineer,
growth-marketer, neuro-ux-designer, in parallel, report-only). Present 2–3 candidate
approaches with trade-offs and a recommendation.

**5. Prototype — dispatch `product-strategist`** for the feature-dev:code-architect
feasibility pass. Present the high-level technical shape and an MVP-vs-later scope cut. (No
detailed design — that is brainstorming's job downstream.)

**6. Test (main loop).** Draft success metrics + a measurement plan following the
`add-posthog-metrics` skill conventions (snake_case `object_verb` past-tense events, a
`location` prop, no PII, no invented metrics). List the events/funnels to instrument.

**7. Write the PRD** to `docs/superpowers/prd/YYYY-MM-DD-<feature>.md` using the template
below. Fill every section from the phases above.

**8. GATE 2 — PRD-approval.** Ask the user to review the file; revise on request.

**9. Hand-off.** Once approved, offer: "Shall I hand this to `superpowers:brainstorming` to
turn it into a technical spec?" On yes, invoke it with the PRD as input.

## PRD template (9 sections)

1. **Problem & context** — the problem, for whom, why now (evidence from Empathize).
2. **Users & JTBD** — persona(s) + job-to-be-done, linked to the memory wiki.
3. **How Might We** — the reframing that opens the solution space.
4. **Goals & non-goals** — what we solve and what we explicitly do not (YAGNI).
5. **Approaches considered** — 2–3 options from the three lenses, trade-offs, recommendation.
6. **Scope: MVP → later** — feasibility from code-architect, the incremental cut.
7. **Success metrics & measurement** — metrics + the PostHog events/funnels to instrument.
8. **Risks & open questions.**
9. **Hand-off** — pointer to the technical spec (filled once brainstorming runs).

## Rules

- **Router, not shotgun**: dispatch only the specialists a phase needs (Empathize: Explore +
  PostHog; Ideate: the 3 lenses; Prototype: code-architect). Never fire the whole panel
  every time.
- **Two hard gates only** (problem-lock, PRD-approval); phases flow in between. The user can
  interrupt anytime.
- **Report-only**: the specialists edit nothing during a PRD run — the PRD is a thinking
  artifact. Building happens later via the normal flow.
- **Relay one consolidated view** — condense sub-reports; do not dump every raw report.
- **Never commit.** The PRD lives under the already-gitignored `docs/superpowers/`; the user
  reviews. Do not stage or commit anything.
