---
name: sandbox-verification-capabilities
description: "Verify against a running system, not a compiler — Postgres 16 can be started in the agent sandbox (initdb + pg_ctl as the postgres user) and the backend boots against it; buf and kubectl are absent, so protobuf codegen and kustomize validation are blocked"
metadata:
  type: reference
  updated: 2026-08-22
  sources: [docs/superpowers/prd/2026-08-17-competitive-program.md]
---

**A compiler proves a query parses. It does not prove the query runs.** Every finding below was
invisible to `go build` and `go vet`, and each was caught within minutes of pointing the code at
a real database. Prefer a running system whenever one is reachable.

## Postgres 16 is available in the agent sandbox — use it

`/usr/lib/postgresql/16/bin/` ships the full server (16.13); `psql` is on `PATH`; a `postgres`
system user exists. Start a throwaway cluster with `initdb` + `pg_ctl` **as the `postgres`
user** (the server refuses to run as root), point the service's DSN at it, and run the package's
tests. The backend also **boots locally against it**, which is how the unsubscribe flow was
exercised the way a reader would meet it, rather than only through `httptest`.

What doing this actually caught, none of which reading the code did:

- **A duplicate index.** An explicit `CREATE UNIQUE INDEX` on a column already declared
  `.Unique()` in `schema.go` — Postgres backs a UNIQUE **constraint** with an index, so the
  table shipped two indexes on one column (`…_unsubscribe_token_key` **and**
  `uq_reminder_prefs_token`), paying two index writes per row to answer one query. Removed and
  re-verified on a fresh database that exactly the primary key and the constraint's index
  remain. Generalisable: **a `.Unique()` column already has its index — adding one is a silent
  write-amplification bug**, not a redundancy the planner absorbs.
- **Two wrong fixture assumptions**: `workspaces.slug` and `workbenches.owner_id` are both
  required, which the schema-as-code did not make obvious. Test fixtures written against a
  mental model of the schema are exactly what a real database corrects.
- **A bucket-assignment surprise**: a bid two days from its deadline lands in the **1**-day
  bucket, not the 3-day one, because `daysUntil` truncates — behaviour that was correct but
  under-documented until it was observed ([[deadline-reminder-pipeline]]).
- Migration ordering and column-order/scan agreement (all twelve ingestion migrations apply in
  order; a new query's column order matches its `Scan`).

Related discipline: **mutation-check the assertions that matter.** Stub the write to a no-op,
or remove the filter under test, and confirm the test goes red with its own message. A green
test that cannot go red is decoration — and a test that would pass for the wrong reason (e.g.
an unsubscribe test whose watermark alone would have stopped the mail) has to be re-armed
before the second pass or it cannot fail for the reason it names.

## What the sandbox cannot do — these are blocks, not findings

- **`buf` is not installed and `buf.build` is unreachable**, so **protobuf codegen is
  impossible here**. Anything requiring a new proto field is blocked at the point the field is
  needed — e.g. `UpdateProfileRequest.locale` for the account-settings language control
  ([[user-locale-capture]]). Plan cross-layer work so the proto-dependent slice is either done
  before the sandbox session or explicitly deferred; discovering it mid-phase strands a feature
  at 90%.
- **`kubectl` is absent**, so `kubectl kustomize infrastructure/kubernetes` — required by the
  infrastructure rules before committing a manifest — **could not be run** for the new digest
  CronJob. The manifest parses as YAML and is registered in `kustomization.yaml`; someone with
  `kubectl` must run the real validation before it reconciles. Record an unrun check in the
  commit message where whoever deploys will read it, never as "validated".
- **Most external hosts are egress-blocked**, including every ANAC host (`000`, not even a WAF
  403) and `ted.europa.eu/…/xml` (403 CONNECT). An unreachable host proves nothing about the
  data behind it — treat it as an open measurement, per
  [[positive-control-for-negative-results]]. Where a corpus is already stored in the database,
  query the corpus instead of the network.

**The habit:** before writing "verified", name *what* verified it — compiler, unit test, real
database, running server, or nothing. The four are not interchangeable, and the gap between
them is where this session's real bugs lived.
