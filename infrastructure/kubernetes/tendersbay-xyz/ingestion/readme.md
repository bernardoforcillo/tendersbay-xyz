# ingestion secrets

The `ingestion` CronJob consumes its config from a `Secret` via `envFrom`
(`ingestion-secrets`). There is only one channel (`main`) — see the design
doc for why ingestion doesn't run a canary CronJob.

This secret is **applied out-of-band** with `kubectl` and is **not** part of
the Flux kustomization. The cluster has no secret-encryption controller
(sealed-secrets / SOPS), so real values are never committed. If the Secret
were listed in `kustomization.yaml`, Flux (`prune: true`) would overwrite it
with the empty template on every reconcile and zero out `DATABASE_URL`.

## Applying

Real values live in `local.secret.yaml` in this folder — it is **gitignored**
(the repo's `.gitignore` already matches the bare filename `local.secret.yaml`
anywhere in the tree, the same rule `backend/local.secret.yaml` relies on).
Create it yourself (it is never generated or committed by tooling), then:

```sh
kubectl apply -f infrastructure/kubernetes/tendersbay-xyz/ingestion/local.secret.yaml
```

## Secret keys (template)

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ingestion-secrets
  namespace: tendersbay-xyz
type: Opaque
stringData:
  DATABASE_URL: "" # same CNPG cluster as backend — postgresql://app:<pw>@postgres-cluster-rw.postgres:5432/tendersbay-xyz
```

`DATABASE_URL` points at the same CloudNativePG cluster `postgres-cluster` in
the `postgres` namespace that `backend` uses (shared cluster; database
`tendersbay-xyz`, role `app`) — ingestion's tables live in their own
`tenders` Postgres *schema* within that same database, not a separate
database. Cross-namespace egress to `postgres:5432` is already allowed by the
`webapp-restricted` CiliumNetworkPolicy (it selects by `app: tendersbay-xyz`,
which this CronJob's pod carries), so no new network policy is needed.

## Migrations run unattended, inside a bounded window — plan schema changes accordingly

`services/ingestion/internal/adapter/postgres/db.go`'s `New` applies every
pending migration (`migrations.Migrator.Up`) as part of opening its database
connection, which happens at the very start of every run. There is no
separate "apply migrations" step and no human in the loop: whichever
migration is next in `services/ingestion/migrations/` simply runs, unattended,
the next time this CronJob fires — see `main/cronjob.yaml`'s `schedule: "0 *
* * *"` (hourly).

That run is **not open-ended**. `main/cronjob.yaml` sets
`activeDeadlineSeconds: 3000` (50 minutes) specifically to leave headroom
before the next hourly fire, and `backoffLimit: 2` retries a failed job. A
migration that is still running when the deadline hits gets its pod killed
mid-transaction, the transaction rolls back (migrations here run inside one
transaction — see e.g. migration 0008/0009's header comments on why `CREATE
INDEX CONCURRENTLY` is unavailable), and the same migration is attempted again
on the next hourly fire. A migration that reliably overruns 50 minutes would
therefore never complete on its own — it would retry forever, every hour,
each attempt burning the same wall-clock budget the real ingestion work
(fetching notices, parsing PDFs) needs to share the deadline with.

**A prior migration (0008) instructed "run during a quiet window" — that
instruction is not achievable here.** Nobody chooses when this CronJob fires;
there is no maintenance-window concept in this deployment, and there being no
human triggering the run whatsoever is exactly why db.go wires migrations to
run automatically. Any future migration's own comments should describe the
real constraint (this section) rather than repeat the unactionable "quiet
window" phrasing.

**Before merging a migration that rewrites the whole `ingested_tenders`
table** (dropping and re-adding a generated column — e.g. changing
`search_vector`'s expression — rewrites the table and rebuilds its indexes
under an `ACCESS EXCLUSIVE` lock, same as any full-table `ALTER`), check its
cost against the *current production row count*, not the dev table. As a data
point: rebuilding `search_vector` (drop + re-add the generated column, then
`CREATE INDEX`) against the dev table's 13,036 rows measured at ~5 seconds
wall-clock (migration 0009, verified directly) — nowhere near the 50-minute
deadline at that scale. That number does not extrapolate blindly to
production; check the actual row count there before assuming a similar
migration is safe, and if a migration is likely to run long, consider
breaking it into smaller steps (e.g. backfill a new column with batched
`UPDATE`s before the schema change that depends on it, the way
`cmd/backfill-descriptions` already does for description backfill) rather
than one long rewrite that risks the deadline.

## Two CronJobs: fetching and indexing are scheduled separately, on purpose

`main/cronjob.yaml` (`ingestion`, hourly) fetches from every provider and
persists. `main/indexer-cronjob.yaml` (`indexer`, every 15 minutes) embeds
unindexed tenders into Qdrant and extracts the text of their attached
documents.

They used to be one process — ingestion first, indexing second. Fetching
regularly outran `activeDeadlineSeconds`, so the job was killed before
indexing ever started. The failure was near-invisible: the CronJob's own
history showed the run failing on a deadline, while the real damage was that
`indexed_at` stayed `NULL` for every tender in the database and document
extraction never advanced past whatever backlog predated the slowdown. Dense
search silently degraded to whatever had been indexed months earlier.

Two things follow for anyone changing these:

- **Only `ingestion` applies migrations.** `postgres.New` migrates;
  `postgres.Connect` does not, and the indexer uses `Connect`. Adding a second
  migrating binary would put two schedules in a race to apply the same
  migration.
- **The indexer's deadline is not a data-loss risk.** It drains in batches and
  each batch commits independently, so a run cut short by
  `activeDeadlineSeconds` simply leaves the remainder for the next fire.

To watch a drain, the per-batch progress is on stdout:

```sh
kubectl logs -n tendersbay-xyz -l tier=ingestion-indexer --tail=50
```

`indexed_total` climbing across `indexing batch complete` lines means it is
working; `indexing pass complete` with a low total means the backlog is clear.

## Seeding the CPV vocabulary — required once per environment, fails silently if skipped

`cmd/seed-cpv` loads the embedded CPV 2008 vocabulary into `tenders.cpv_terms`.
Nothing in the CronJob or its migrations does this automatically — it is a
separate command precisely because embedding ~227k `INSERT`s in a migration
would bloat every binary that imports the migrations package and would hold
the migration transaction's locks for minutes (see `cmd/seed-cpv`'s package
doc). **It must be run by hand, once per environment, after migrations have
applied** (the CronJob applies those automatically on its first run — see
above).

Until it has run, `cpv_terms` is empty. This is not a startup failure: the
CronJob starts and ingests tenders normally, `MatchCodes` simply resolves
nothing against an empty table, `AppliedCPV` is always `[]`, the chips UI
never renders anything, and `CPVBoost` never contributes to ranking. Nothing
logs an error and nothing crash-loops — the CPV feature is just silently
inert.

Run it with the one-off Kubernetes Job in this folder (deliberately **not**
part of `kustomization.yaml` — see that file's header comment for why a
completed Job can't be Flux-managed):

```sh
kubectl apply -f infrastructure/kubernetes/tendersbay-xyz/ingestion/main/seed-cpv-job.yaml
kubectl logs -n tendersbay-xyz job/seed-cpv -f
```

**A CPV vocabulary revision means re-running it.** `UpsertTerms` is
idempotent (`ON CONFLICT DO UPDATE` on `(code, lang)`), and `seed-cpv` also
recomputes `cpv_labels` on every already-ingested tender afterward, so a
re-run is the intended way to roll out a revised vocabulary — not a one-time
bootstrap step to forget about. Delete the completed Job object first
(`kubectl delete job seed-cpv -n tendersbay-xyz --ignore-not-found`) since a
Job's spec is immutable once created.
