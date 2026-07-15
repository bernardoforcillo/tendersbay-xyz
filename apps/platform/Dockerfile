# syntax=docker/dockerfile:1

# ---- Stage 1: build the Vite SPA ----
FROM node:26-alpine AS frontend
# Node 26 no longer bundles corepack; reinstall it so it can activate the
# pnpm version pinned in the root package.json `packageManager` field.
RUN npm install -g corepack@latest && corepack enable
WORKDIR /repo

# Manifest-first copy so `pnpm install` caches independently of source churn.
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml .npmrc turbo.json ./
COPY apps/platform/package.json apps/platform/
COPY packages/components/package.json packages/components/
COPY packages/tailwind/package.json packages/tailwind/
COPY packages/tsconfig/package.json packages/tsconfig/

RUN pnpm install --frozen-lockfile

# Copy the rest of the sources and build only the frontend (not `go build`).
COPY . .
# Client config (API URL, PostHog) is provided at runtime by the Go server via
# window.__ENV__ (see internal/server), so nothing is baked into the bundle here.
# A dev/build-time value can still be supplied through apps/platform/.env.
RUN pnpm --filter platform exec vite build

# ---- Stage 2: compile the Go binary that embeds dist ----
# Built from the repo root so the Go workspace (go.work) resolves the app's
# sibling module go-services/telemetry. It's wired in through the workspace —
# not via `replace` in go.mod — so the whole tree is copied and go.work drives
# resolution (mirrors services/backend + services/ingestion).
FROM golang:1.26-alpine AS backend
# git is required for Go to resolve the GOPRIVATE internal modules through
# go.work; the alpine image ships without it, so the build otherwise fails with
# `exec: "git": executable file not found in $PATH`.
RUN apk add --no-cache git
WORKDIR /workspace

# Internal monorepo modules resolve locally through go.work, so keep them off
# the public proxy/sumdb.
ENV GOPRIVATE=github.com/bernardoforcillo/tendersbay-xyz
COPY . .
RUN go mod download
# Provide the freshly built SPA where `//go:embed all:dist` expects it
# (.dockerignore excludes dist, so `COPY . .` above brought no stale copy).
COPY --from=frontend /repo/apps/platform/dist ./apps/platform/dist

WORKDIR /workspace/apps/platform
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /platform .

# ---- Stage 3: minimal runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /platform /platform
EXPOSE 8080
ENV PORT=8080
USER nonroot:nonroot
ENTRYPOINT ["/platform"]
