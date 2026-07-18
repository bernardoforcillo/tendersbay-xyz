# Code organization & dependency boundaries

Applies repo-wide, concretely to `services/backend` and `apps/platform`. The organizing
question is not "where does this file go" but "who is allowed to know about this" — every
layer has exactly one job, dependencies flow in one direction only, and no layer skips
ahead to talk to one it doesn't own. Full reasoning: `.claude/memory/code-organization-principles.md`.
Dispatch the `software-architect` agent to review a change against this rule or to help fix
a boundary violation.

## The layers, mapped to this repo

1. **UI** — `apps/platform/src/features/*`, `src/routes/*`. Expresses user intent. No
   business logic, no direct vendor or database calls.
2. **Transport** — `apps/platform/src/lib/api` (the frontend's API client) and
   `services/backend/internal/adapter/{httpapi,connectapi}` (the backend's handlers).
   Validates input, authenticates, authorizes, and delegates. Owns no business decisions.
3. **Domain** — `services/backend/internal/core/*`. Owns product/business decisions. The
   only layer allowed to call capabilities/adapters.
4. **Capabilities / vendors** — `services/backend/internal/adapter/{postgres,redis,email,...}`.
   Each wraps exactly one external system (database, cache, email provider, a future
   payments vendor, etc.) behind an interface **defined by the domain layer**, per the
   ports-and-adapters shape `internal/core`/`internal/adapter` already implies.
5. **Supporting foundations** — shared packages (`packages/*`) and cross-cutting config
   (`services/backend/internal/config`). The one layer allowed to be imported from more than
   one place in the chain above.

## The dependency rule

Never skip a layer, and never let a lower layer import a higher one:

- `apps/platform` must never import a vendor SDK or a database driver directly. If a feature
  needs a vendor integration (payments, a third-party API), it goes through
  `services/backend` — the frontend calls the backend's transport layer, nothing else.
- `internal/core` must never import `internal/adapter/*`. Core defines the interface; the
  adapter implements it and imports core — never the reverse. This is what "hexagonal"
  already means in this repo; this rule makes it explicit and reviewable.
- `internal/adapter/{httpapi,connectapi}` (and the frontend's `lib/api`) must never own
  business logic — validate, authenticate, authorize, delegate to `internal/core`, and
  nothing more.

The transport layer's job description is deliberately narrow. A vaguer one — "bridge between
the client and the backend" — is exactly how a handler quietly accumulates caching, retries,
response shaping, and eventually business rules until it's a god layer nobody can safely
change (the classic "fat controller"/"fat presenter" failure mode). The
`apps/platform` ↔ `services/backend` boundary already has its contract enforced for free: the
protobuf schema (`@tendersbay/proto`, generated, consumed via ConnectRPC on both sides) *is*
the typed request/response contract. Don't bolt on a separate runtime validator (Zod, Yup) at
that boundary the way an untyped REST/JSON API would need — it would duplicate a contract
that's already generated and already type-checked.

## Build order follows the dependency rule, reversed

The dependency rule above describes *call* direction (UI → transport → domain →
capabilities/vendors). *Build* order for a feature that spans more than one layer runs the
other way: data model/contracts (domain + supporting foundations) first, then backend wiring
(transport), then UI/polish last. Building UI before the backend layer it depends on means
the agent (human or AI) fills the gap with an assumption — a shape that gets thrown away once
the real backend lands. Phase plans and PRDs for cross-layer features accordingly (the `/prd`
skill and `software-architect` agent both apply this), and keep each phase's in-scope /
out-of-scope explicit rather than letting a phase creep into the next layer's work.

## Enforce it mechanically, not just by convention

Convention alone erodes as a codebase grows, and it erodes faster with AI agents in the loop
— an agent reuses whatever pattern it finds already in the code, good or bad. This repo uses
Biome, not ESLint, so there's no import-boundary lint rule wired up yet. Until one exists,
the cheapest real backstop is dependency-level: don't add a vendor SDK or DB client to
`apps/platform`'s `package.json` at all — code that can't be installed can't be imported.
Investigating a Biome or custom import-boundary rule (or a `go vet`-based check for the
`internal/core` → `internal/adapter` direction) to make this a CI failure instead of a review
comment is a reasonable next step, not yet in place.

## Naming and supply-chain hygiene

- Kebab-case is already the house rule for this repo's own files (@.claude/rules/frontend.md)
  — one predictable naming format everywhere reduces guesswork for both people and AI agents.
  Nothing new here; this rule reinforces it, it doesn't add to it.
- `pnpm-workspace.yaml` gates dependency build/postinstall scripts via `allowBuilds` **and**
  sets `minimumReleaseAge: 10080` (7 days, in minutes) — installs never resolve to a package
  version published more recently than that, since most compromised packages get caught and
  yanked within that window. `pnpm install` fails closed if a required version is too fresh;
  don't lower this to work around a failure without understanding why the version is new.
- **Go and pnpm have different supply-chain threat models — don't port one's mitigation to
  the other by analogy.** Go modules never execute install-time scripts (no npm-style
  postinstall hook exists), so `allowBuilds`/`minimumReleaseAge`-style gating has no Go
  equivalent and isn't needed — there's no "code runs automatically the moment you install
  it" attack surface. Go's structural defense is `GOSUMDB` + `go.sum`: every module version
  is checksum-verified against a public transparency log on first fetch, then pinned forever.
  What Go lacks instead is known-vulnerability scanning, so `services/backend`'s `lint`
  script runs `go tool govulncheck ./...` (the tool itself is pinned via `go.mod`'s `tool`
  directive, same reproducibility guarantee as any other dependency) — this catches
  known-CVE dependencies, a different risk than a too-fresh package version.

## The meta-rule

Fixing a bad dependency direction gets more expensive the more code is built on top of it —
structure this correctly while it's still cheap to change, not after the fact. And remember
the asymmetry with AI agents: they amplify whatever structure already exists, so an enforced
boundary today is worth more than the same boundary added later.
