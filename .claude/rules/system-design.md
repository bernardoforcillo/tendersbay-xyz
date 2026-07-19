# System design & scaling decisions

Applies repo-wide, most concretely to `services/backend` and `infrastructure/kubernetes`.
These are trigger → action rules, not a target architecture to build toward — every rule
below is "reach for this when the trigger is real," not "add this preemptively." Full
reasoning and source: `.claude/memory/system-design-principles.md`. To apply this checklist
to a design review, or to scaffold one of these changes, dispatch the `software-architect`
agent.

## Statelessness

`services/backend` handlers must stay stateless — request-scoped only. Anything that needs
to persist goes to its datastore (Postgres, object storage, cache), never held in-process
across requests. In-process state is what kills horizontal scaling and pod restarts.

## Scale order: horizontal before vertical

Prefer more k8s replicas (`infrastructure/kubernetes/tendersbay-xyz/<app>/<channel>/deployment.yaml`)
over bigger pod resource requests/limits — replicas are cheap to add/remove and don't risk a
single-pod bottleneck. Reach for vertical scaling only when a workload is genuinely
CPU/memory-bound in a way replicas can't parallelize (e.g. a single long-running compute
job, not a request-serving handler).

Traefik's `IngressRoute` already load-balances across a Service's endpoints — don't build or
configure a custom load-balancing layer. If one route is disproportionately expensive (e.g.
uploads vs. reads), split it into its own workload/channel rather than fighting it inside one
Deployment.

## Microservices threshold

Don't split `services/backend` into a new `services/<name>` until there's a **proven,
isolated bottleneck** or a genuinely separate team/ownership boundary — not by default and
not preemptively. The hexagonal layout (`internal/core` + `internal/adapter/*`) already
isolates domains inside one binary; prefer adding a new `internal/core` domain or
`internal/adapter` first. See @.claude/rules/git-flow.md and @.claude/rules/infrastructure.md
for what a new service actually costs (own `go.mod`, Dockerfile, CI workflow, k8s app folder,
image automation) before proposing one.

## Gateway / routing

If a second backend service is ever added, route between them through Traefik `IngressRoute`
rules — mirroring the existing NodePort + IngressRoute pattern — rather than having services
call each other directly. Keep non-gateway services off the public network.

## AuthN vs authZ

Validate tokens (signature + expiry) at the edge without a network hop per request once a
token scheme exists. Keep token *issuance* centralized in one place; don't duplicate signing
logic per service.

## Large files / blobs

Never proxy large uploads through the Go binary and never store blob bytes in the relational
database. Use the presigned-URL pattern: the API writes metadata and returns a short-lived,
size-capped upload URL; the client uploads directly to object storage.

## Async fan-out

Once more than one downstream needs to react to the same event, introduce a broker
(queue/pub-sub) instead of a hand-rolled synchronous call chain — a chain means one slow or
down dependency breaks the whole flow. A broker needs ack/redelivery and a dead-letter path
with alerting, not just a happy path.

## Caching vs CDN

Cache small, hot **metadata** in an in-memory KV store. Never cache blobs/large assets there —
that's what a CDN is for. These are two different tools solving two different problems, not
one scaled up into the other.

## Rate limiting

Enforce at the edge, backed by a fast counter store (per-user/IP request counts in a sliding
window). Add it when there's a credible abuse or cost-blowup risk — not as a default on day
one.

## The meta-rule

Every layer above is a deliberate trade-off. Before adding one, state the **current,
observed** pain point it solves (link a project memory page, an incident, or a PR
description) — not a hypothetical future one. This is the same YAGNI discipline the rest of
this repo's conventions already apply to code.
