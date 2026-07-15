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
