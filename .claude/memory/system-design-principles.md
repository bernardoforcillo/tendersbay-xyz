---
name: system-design-principles
description: System-design decision framework (stateful->stateless servers, load balancing, microservices, API gateway, auth, object storage, event broker, caching/CDN, rate limiting) — full reasoning behind the tendersbay-xyz system-design rule set
metadata:
  type: reference
  updated: 2026-07-18
---

This page holds the full reasoning behind each principle; the tendersbay-xyz-specific,
actionable version of the same rules lives in `.claude/rules/system-design.md` and is what
the `software-architect` agent applies to reviews.

The throughline across these principles: every architecture layer below is **separation of
concerns repeated at a bigger scale** (server/db → service/service → gateway/services), and
each one should be added because of an observed, current pain point — never preemptively.

- **Stateful → stateless servers.** Data living inside a server instance means the server
  can't die or scale without losing it. Decouple state into a database immediately; the
  server becomes a pure request handler that any instance can serve.
- **Horizontal vs. vertical scaling.** Horizontal = more machines (redundancy, parallel
  capacity); vertical = bigger machines (simpler, but a bigger blast radius per failure).
  Autoscalers should target the minimum fleet size that holds load — machine count is a
  direct cost line.
- **Load balancer strategy is a real decision.** Round-robin assumes every request costs the
  same, which is false the moment one route (e.g. an upload) is heavier than another.
  Options: health-check-aware routing, least-connections, path-based routing (route
  `/upload` to a specialized pool), sticky sessions (cookie affinity when a server holds
  per-request state).
- **Microservices are an organizational trigger, not a default.** Split when a team needs
  independent ownership of a domain, or a bottleneck is proven and localized — not because
  "microservices is the mature architecture." Splitting too early adds network calls, deploy
  complexity, and cross-service consistency problems for no real gain.
- **An API gateway becomes necessary once there's more than one service.** Single entry
  point, hides service topology behind a private network, does routing + response
  aggregation, and is the natural place to validate auth without every service
  re-implementing it.
- **Separate authentication from authorization, and avoid a network hop per request.**
  Validate a signed token's signature + expiry at the edge with no call to an auth service;
  only the login flow itself talks to the auth service to issue a token.
- **Never proxy large file uploads through app servers or store blobs in a relational DB.**
  The client asks the API for a presigned upload URL (API only writes metadata), then
  uploads directly to object storage (S3/GCS-style). Presigned URLs get short expiry and a
  size cap — both a security and a cost control.
- **Service-to-service fan-out needs a broker, not direct calls.** A synchronous chain
  (storage → thumbnail service → notification service → …) means any one slow or down
  dependency breaks the whole flow. A durable pub/sub broker (Kafka/RabbitMQ-style) decouples
  producer from consumers, and needs ack/redelivery plus a dead-letter queue with alerting
  for messages that never get delivered.
- **Cache metadata in memory; never cache blobs in memory.** RAM is scarce and expensive, so
  an in-memory KV store (Redis-style) is for small, hot, frequently-read data — not the
  files themselves. Standard pattern: check cache → miss → hit DB → populate cache → return.
- **Large static assets belong on a CDN, not in the cache layer.** A CDN's edge
  points-of-presence put bytes physically close to users; this solves a different problem
  (network latency) than an in-memory cache (compute/DB load), so it's a different tool, not
  a bigger cache.
- **Rate limit at the edge, backed by a fast counter store.** Per-user/IP request counts in a
  sliding window (Redis-style), returning 429 past a threshold — protects cost and other
  tenants from abusive or unbounded load. Add this when there's a credible risk, not as a
  reflexive default.
- **The meta-principle.** Every layer is a trade-off examined on its own merits. Evolve the
  architecture step by step in response to real pain; resist skipping ahead to complexity the
  system doesn't need yet.
