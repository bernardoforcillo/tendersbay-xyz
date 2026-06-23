# Infrastructure & deployment

Kubernetes manifests live in `infrastructure/kubernetes/` and are reconciled onto the
cluster by **Flux** (GitOps). The cluster stack the manifests target:

- **Traefik** — ingress via `IngressRoute` (CRD, *not* the native k8s `Ingress`), plus
  shared `Middleware` / `TLSOption` objects.
- **cert-manager** — TLS via `Certificate` issued by the `ClusterIssuer` named
  `cluster-issuer`.
- **Cilium** — `CiliumNetworkPolicy` for pod-level network policy.

## Layout

Layout is `<namespace>/<app>/<channel>`:

```
infrastructure/kubernetes/
├── repository.yaml             # Flux GitRepository + Kustomization (bootstrap only)
├── kustomization.yaml          # sync entry point — list every applied manifest here
├── policies/                   # CiliumNetworkPolicy (applied separately, NOT in kustomization)
└── <namespace>/                # e.g. tendersbay-xyz
    ├── namespace.yaml
    ├── commons.yaml            # shared Traefik middlewares + TLSOption (namespace-scoped)
    └── <app>/                  # e.g. platform
        ├── main/               # stable channel
        │   ├── deployment.yaml
        │   ├── service.yaml
        │   ├── ingress.yaml    # Traefik IngressRoute
        │   ├── certificate.yaml
        │   └── update.yaml     # ImageRepository + ImagePolicy + ImageUpdateAutomation
        └── canary/             # canary channel (optional)
            ├── deployment.yaml
            ├── service.yaml
            ├── ingress.yaml
            ├── certificate.yaml
            └── update.yaml     # ImagePolicy only (reuses main's repo + automation)
```

- `namespace.yaml` and `commons.yaml` are namespace-scoped and live at the
  `<namespace>/` level, shared by every app and channel in it.
- `kustomization.yaml` is the entry point Flux builds (`path: ./infrastructure/kubernetes`).
  **Add each new manifest to its `resources:` list** or it won't be applied.
- `repository.yaml` and `policies/` are intentionally excluded from `kustomization.yaml`:
  `repository.yaml` is applied once at bootstrap, and the Cilium policies are applied
  on their own (`kubectl apply -f infrastructure/kubernetes/policies`).
- Validate offline before committing: `kubectl kustomize infrastructure/kubernetes`.

## Conventions

- **Namespace:** `tendersbay-xyz`. **Labels:** `app: tendersbay-xyz`, `tier: <workload>`.
- **Resource names:** `<workload>` (Deployment), `<workload>-svc` (Service),
  `<workload>-ingress` (IngressRoute), `<workload>-tls` (Certificate + secret).
- **Pod hardening (match the reference):** `runAsNonRoot`, drop `ALL` capabilities,
  `readOnlyRootFilesystem: true` with an `emptyDir` mounted at `/tmp`, and
  `seccompProfile: RuntimeDefault`.
- **`platform` specifics:** the Go server listens on **8080** (`PORT` env, default 8080;
  Dockerfile `EXPOSE 8080`). The runtime image is `gcr.io/distroless/static-debian12:nonroot`,
  whose user/group is **UID/GID 65532** — set `runAsUser`/`runAsGroup`/`fsGroup` to that.
- **Service** is `NodePort`, fronted by the Traefik `IngressRoute`; the IngressRoute's
  `services[].port` must match the Service `port` (stable `30080`, canary `30081`).

## Channels (stable / canary)

`platform` runs two channels in the **same** `tendersbay-xyz` namespace, one folder each:

| Channel | Folder | `tier` label | Workload name | Host | Image tags |
| --- | --- | --- | --- | --- | --- |
| stable | `main/` | `platform` | `platform` | `tendersbay.xyz` (+`www`) | `<ts>-<sha>` from `main` |
| canary | `canary/` | `platform-canary` | `platform-canary` | `dev.tendersbay.xyz` | `<ts>-<sha>-canary` from `dev` |

- Canary reuses the shared `commons.yaml` middlewares (it drops `redirect-to-non-www`,
  since `dev.` has no `www` variant) and has its own `Certificate` for `dev.tendersbay.xyz`.
- The Cilium policies select by `app: tendersbay-xyz` (no `tier`) so they cover both
  channels.

## Image automation (Flux)

The `platform` image `bernardoforcillo/tendersbay-platform` is built and pushed by
`.github/workflows/ci-platform.yml` (see @.claude/rules/git-flow.md for the tag scheme).

- One **ImageRepository** (`platform/main/update.yaml`) scans the image; both channels
  share it.
- Two **ImagePolicy** objects: stable (`platform/main/update.yaml`) matches
  `^[0-9]{14}-[0-9a-fA-F]{40}$` (`<timestamp>-<full-sha>` from `main`); canary
  (`platform/canary/update.yaml`) matches `^[0-9]{14}-[0-9a-fA-F]{40}-canary$`
  (`…-canary` from `dev`). Stable excludes `:latest` and `-canary`.
- Each Deployment `image:` line carries a setter marker, e.g.
  `# {"$imagepolicy": "tendersbay-xyz:tendersbay-platform-image-policy"}` (canary uses
  `…-canary-image-policy`) — keep it, that's what the automation rewrites.
- A single **ImageUpdateAutomation** (`platform/main/update.yaml`) scans the whole
  `./infrastructure/kubernetes/tendersbay-xyz/platform` folder and rewrites **every**
  marker it can resolve in the namespace — so one automation drives both channels.
- It commits the bumped tag **directly to `main`**. This is a deliberate exception to the
  `feature → dev → main` PR flow (@.claude/rules/git-flow.md): the cluster only tracks
  `main`, so the GitOps loop requires it (canary still serves the `dev`-channel *image* —
  only the manifest edit lands on the `main` git branch). The Flux deploy-key secret
  (`tendersbay-xyz-auth`) therefore needs **write** access.

## Adding an app or channel

- **New app:** create `<namespace>/<app>/<channel>/` mirroring `platform/main`
  (deployment/service/ingress/certificate, plus `update.yaml` if it ships its own image),
  add each file to `kustomization.yaml`, and add a `CiliumNetworkPolicy` under `policies/`
  if it needs one.
- **New channel** of an existing app: add a `<channel>/` folder with its own
  deployment/service/ingress/certificate and an `ImagePolicy` (reuse the app's existing
  ImageRepository + ImageUpdateAutomation), give the workload a distinct `tier` label, and
  register the files in `kustomization.yaml`.
