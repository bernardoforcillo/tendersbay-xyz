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
RUN pnpm --filter platform exec vite build

# ---- Stage 2: compile the Go binary that embeds dist ----
FROM golang:1.26-alpine AS backend
WORKDIR /src

# The Go module has no external deps, so no `go mod download` is needed.
# `.dockerignore` excludes dist/bin, so this copy brings source only.
COPY apps/platform/ ./
# Provide the freshly built SPA where `//go:embed all:dist` expects it.
COPY --from=frontend /repo/apps/platform/dist ./dist

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /platform .

# ---- Stage 3: minimal runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=backend /platform /platform
EXPOSE 8080
ENV PORT=8080
USER nonroot:nonroot
ENTRYPOINT ["/platform"]
