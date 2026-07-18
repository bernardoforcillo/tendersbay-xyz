---
name: ai-coding-workflow-principles
description: AI-coding-workflow discipline (build on an existing foundation rather than a blank slate, phase cross-layer features in build order, keep each phase's scope explicit) — reasoning behind the /prd skill's grounding-in-rules and build-order additions
metadata:
  type: reference
  updated: 2026-07-18
---

This page holds the reasoning; the actionable version is spread across
`.claude/skills/prd/SKILL.md`, `.claude/agents/product-strategist.md`, and
`.claude/rules/code-organization.md` (the "Build order follows the dependency rule,
reversed" section) — apply those, not this page.

- **Going from an established foundation to a finished feature is categorically easier than
  starting from nothing.** An agent (or a person) that can reference existing conventions,
  rules, and architecture makes far fewer wrong assumptions than one starting from a blank
  slate. This repo's `.claude/rules/*.md` **is** that foundation — a feasibility or planning
  pass should be grounded in it explicitly, not left to rediscover or reinvent it per task.
- **An agent that doesn't know your existing conventions will invent its own.** Left
  ungrounded, it reaches for whatever pattern it's seen most in training or in the
  surrounding code — which may not be this repo's pattern. Explicitly pointing a
  feasibility/planning pass at the relevant rule files is cheap insurance against that drift.
- **Phase cross-layer features in build order, not call order.** A request flows UI → …→
  vendor at runtime, but that is the wrong order to *build* it in — building UI before its
  backend exists means the UI either can't be tested against real behavior or gets built
  against a guessed shape that's discarded once the backend lands. Build foundation-first:
  data model/contracts, then backend wiring, then UI/polish.
- **Keep every phase's scope explicit, both what's in and what's deliberately deferred.** A
  phase that silently reaches into the next layer's work (a "backend wiring" phase that also
  redesigns a UI component) defeats the point of phasing at all — bite-sized, reviewable
  phases stay bite-sized only if their boundaries hold.
- **A conversational clarifying dialogue beats a one-shot rewritten prompt.** Some agent
  workflows compile a raw brief into a single polished prompt before generating a plan,
  because their harness doesn't support back-and-forth well. This repo's
  `superpowers:brainstorming` skill already does the equivalent job better — one question at
  a time, with the user, before a spec is written — so there's no separate "refine the
  prompt" step to add on top of the existing `/prd → brainstorming → writing-plans` pipeline.
