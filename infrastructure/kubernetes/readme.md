# Kubernetes manifests

GitOps deployment for tendersbay-xyz, reconciled by **Flux** onto a Traefik +
cert-manager + Cilium cluster.

```
infrastructure/
└── kubernetes/                 # synced by Flux (path: ./infrastructure/kubernetes)
    ├── repository.yaml         # GitRepository + Flux Kustomization (bootstrap only)
    ├── kustomization.yaml      # sync entry point — list every applied manifest here
    ├── policies/               # Cilium network policies (applied separately)
    │   ├── traefik-cnp.yaml
    │   └── webapp-cnp.yaml
    └── tendersbay-xyz/             # the namespace
        ├── namespace.yaml
        ├── commons.yaml            # shared Traefik middlewares + TLSOption (both channels)
        └── platform/               # the app
            ├── main/               # stable channel -> tendersbay.xyz
            │   ├── deployment.yaml
            │   ├── service.yaml
            │   ├── certificate.yaml
            │   ├── ingress.yaml
            │   └── update.yaml     # ImageRepository + stable ImagePolicy + ImageUpdateAutomation
            └── canary/             # canary channel -> dev.tendersbay.xyz
                ├── deployment.yaml
                ├── service.yaml
                ├── certificate.yaml
                ├── ingress.yaml
                └── update.yaml     # canary ImagePolicy (reuses main's repo + automation)
```

Layout is `<namespace>/<app>/<channel>`. The namespace-scoped `namespace.yaml` and
`commons.yaml` sit at the namespace level (shared by any app); each channel folder
holds one workload.

Both channels run the same app (Go server embedding the SPA) in the single
`tendersbay-xyz` namespace, distinguished by the `tier` label (`platform` vs
`platform-canary`).

## Flux

`repository.yaml` defines the `GitRepository` (`flux-system/tendersbay-xyz-repository`,
SSH, branch `main`, secret `tendersbay-xyz-auth`) and the `Kustomization` that
reconciles `./infrastructure/kubernetes`. It is applied once at bootstrap and is
not listed in `kustomization.yaml`, so it is not reconciled as part of the sync.

Image rollout is automated per channel:

- **ImageRepository** (`platform/main/update.yaml`) scans
  `bernardoforcillo/tendersbay-platform` — shared by both channels.
- **Stable ImagePolicy** (`platform/main/update.yaml`) selects the newest tag
  `^[0-9]{14}-[0-9a-fA-F]{40}$` (the `<timestamp>-<sha>` images pushed from `main`).
- **Canary ImagePolicy** (`platform/canary/update.yaml`) selects
  `^[0-9]{14}-[0-9a-fA-F]{40}-canary$` (the `…-canary` images pushed from `dev`).
- A single **ImageUpdateAutomation** (`platform/main/update.yaml`) scans the whole
  `tendersbay-xyz/platform/` folder, rewrites both Deployments' `$imagepolicy`
  markers, and commits the bumps back to `main` (which Flux then reconciles).

> Note: image bumps are committed straight to `main` by the Flux bot, bypassing
> the usual feature → dev → main PR flow. That is intentional for the GitOps loop —
> the cluster only tracks `main`. (Canary still serves the `dev`-channel *image*;
> only the manifest change lands on the `main` git branch.)

## Bootstrap / apply

```bash
# 1. Create the SSH auth secret (deploy key with write access for image automation)
flux create secret git tendersbay-xyz-auth \
  --namespace flux-system \
  --url ssh://git@github.com/bernardoforcillo/tendersbay-xyz

# 2. Apply the Flux source + sync (everything else follows from git)
kubectl apply -f infrastructure/kubernetes/repository.yaml

# Manual apply without Flux (e.g. local validation):
kubectl apply -k infrastructure/kubernetes
kubectl apply -f infrastructure/kubernetes/policies   # Cilium policies
```

## Traefik dashboard

```bash
kubectl port-forward -n kube-system $(kubectl -n kube-system get pods --selector "app.kubernetes.io/name=traefik" --output=name) 9000:9000
```
