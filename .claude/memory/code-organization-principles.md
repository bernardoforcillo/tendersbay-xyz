---
name: code-organization-principles
description: Layered code-organization and dependency-boundary framework (ownership over folders, one-directional dependency rule, mechanical boundary enforcement, naming consistency, supply-chain hygiene) — full reasoning behind the tendersbay-xyz code-organization rule set
metadata:
  type: reference
  updated: 2026-07-18
---

This page holds the full reasoning behind each principle; the tendersbay-xyz-specific,
actionable version lives in `.claude/rules/code-organization.md` and is what the
`software-architect` agent applies to reviews.

- **Ask "who should be allowed to know about this," not "where does this file go."** Folder
  structure is cosmetic and cheap to fix at any size. Ownership — which layer is allowed to
  call which other layer — is the hard part, and it gets harder to fix the bigger the
  codebase gets.
- **There's a point past which restructuring stops being easy.** Small codebases can absorb
  a messy start and still get cleaned up later. Past a certain size, enough incorrect
  conventions (a UI layer calling a database directly, business rules enforced client-side)
  get baked in that reversing them becomes impractical. The lesson is to fix the dependency
  direction while the codebase is still small, not to wait for a trigger to do it.
- **Every layer gets exactly one job, and dependencies flow one direction only.** A typical
  shape: UI (expresses intent) → transport (validates, authenticates, authorizes, delegates)
  → domain (owns business decisions) → capabilities/vendors (wraps one external system each,
  behind an interface the domain defines) → supporting foundations (shared code, importable
  from more than one layer). A lower layer never imports a higher one; a layer never skips
  the one directly below it to reach further down.
- **Business logic belongs in exactly one layer.** A transport/handler layer that "just
  delegates" but secretly contains a business decision (a pricing rule, an authorization
  check) has leaked domain logic into a layer that shouldn't own it — the tell is usually
  logic duplicated in two places because no one could agree where it belonged.
- **Convention alone erodes at scale — enforce boundaries mechanically.** Import-restriction
  lint rules (or an equivalent dependency-level guard, like never installing a vendor SDK in
  a package that shouldn't use it) turn a boundary violation into a build failure instead of
  a review comment that might get missed.
- **AI coding agents amplify whatever structure already exists.** An agent asked to add a
  feature will pattern-match the surrounding code — including bad patterns, like a frontend
  calling a vendor directly. A codebase with enforced boundaries produces agent-written code
  that respects those boundaries by construction; a codebase without them lets an agent
  entrench the mess further, faster than a human would.
- **Fail fast, not late.** A boundary violation caught by a lint/type error the moment it's
  written is cheap. The same violation caught after a feature is 80% built (by a human or an
  agent) means unwinding real work. Mechanical enforcement is what makes "fast" possible.
- **Pick one naming convention and never mix it with another.** kebab-case, camelCase,
  snake_case — the specific choice matters far less than using exactly one, consistently,
  for every file of a given kind. Mixed conventions cost time (from both humans and agents)
  re-deriving what a new file should be called. Standards imposed by a tool or ecosystem
  (e.g. `README.md`, `AGENTS.md`) are an explicit, narrow exception — they're not "your"
  naming convention to change.
- **Supply-chain hygiene compounds with dependency hygiene.** Gating which dependencies are
  allowed to run install/build scripts blocks one attack surface (malicious postinstall
  code). A minimum package release age blocks a different one (a compromised version
  published and yanked within days) — most supply-chain attacks target the latest version of
  a package, so refusing to resolve to anything published in, say, the last 7 days is a cheap
  defense against exactly that pattern.
