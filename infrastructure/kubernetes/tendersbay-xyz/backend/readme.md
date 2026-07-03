# backend secrets

The backend Deployments consume their config from a `Secret` via `envFrom`
(`backend-secrets` for the stable channel, `backend-canary-secrets` for canary).

These secrets are **applied out-of-band** with `kubectl` and are **not** part of
the Flux kustomization. The cluster has no secret-encryption controller
(sealed-secrets / SOPS), so real values are never committed. If the Secret were
listed in `kustomization.yaml`, Flux (`prune: true`) would overwrite it with the
empty template on every reconcile and zero out `DATABASE_URL` etc.

## Applying

Real values live in `local.secret.yaml` in this folder — it is **gitignored**
(`.gitignore` → `local.secret.yaml`) and holds both channel Secrets. Apply it:

```sh
kubectl apply -f infrastructure/kubernetes/tendersbay-xyz/backend/local.secret.yaml
```

Re-apply after editing a value (e.g. setting `RESEND_API_KEY`). Running pods pick
up the change only on the next rollout (Flux rollout on merge, or
`kubectl -n tendersbay-xyz rollout restart deploy/backend deploy/backend-canary`).

## Secret keys (template)

Only truly-secret values live in the Secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: backend-secrets          # canary: backend-canary-secrets
  namespace: tendersbay-xyz
type: Opaque
stringData:
  DATABASE_URL: ""     # CNPG DSN — postgresql://app:<pw>@postgres-cluster-rw.postgres:5432/tendersbay-xyz
  JWT_SECRET: ""       # random, per-channel — e.g. `openssl rand -base64 48`
  RESEND_API_KEY: ""   # Resend API key (re_…); leave empty until email is wired
```

`DATABASE_URL` points at the CloudNativePG cluster `postgres-cluster` in the
`postgres` namespace (shared cluster; database `tendersbay-xyz`, role `app`).
Cross-namespace egress to `postgres:5432` is allowed by the `webapp-restricted`
CiliumNetworkPolicy in `policies/webapp-cnp.yaml`.

## Non-secret config (plaintext env in the Deployment)

These are **not** secrets, so they live as plain `env:` in each channel's
`deployment.yaml` (committed), not in the Secret:

| Key | main | canary |
| --- | --- | --- |
| `APP_BASE_URL` | `https://tendersbay.xyz` | `https://dev.tendersbay.xyz` |
| `JWT_EXPIRY` | `15m` | `15m` |
| `REFRESH_EXPIRY` | `168h` | `168h` |
