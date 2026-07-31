# syntax=docker/dockerfile:1

# ---- Stage 1: compile the Go binary ----
# Built from the repo root (context: .) so the Go workspace (go.work)
# resolves ingestion's sibling modules go-services/tender and
# go-services/telemetry. They're wired in through the workspace — not via
# `replace` in go.mod — so the whole tree is copied and go.work drives
# resolution.
FROM golang:1.26-alpine AS builder
# git is required for Go to resolve the GOPRIVATE internal modules through
# go.work; the alpine image ships without it, so `go mod download` otherwise
# fails with `exec: "git": executable file not found in $PATH`.
RUN apk add --no-cache git
WORKDIR /workspace
COPY . .

# Internal monorepo modules resolve locally through go.work, so keep them off
# the public proxy/sumdb.
ENV GOPRIVATE=github.com/bernardoforcillo/tendersbay-xyz
RUN go mod download

WORKDIR /workspace/services/ingestion
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /ingestion .

# seed-cpv is a one-off command (loads the CPV vocabulary into
# tenders.cpv_terms — see cmd/seed-cpv's package doc), not the service
# entrypoint. It rides along in the same image so it can be invoked with a
# Kubernetes Job that overrides `command` (see
# infrastructure/kubernetes/tendersbay-xyz/ingestion/main/seed-cpv-job.yaml)
# against a distroless, shell-less runtime where there is no other way to
# run it out-of-band. ENTRYPOINT below stays /ingestion.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /seed-cpv ./cmd/seed-cpv

# ---- Stage 2: minimal runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /ingestion /ingestion
COPY --from=builder /seed-cpv /seed-cpv
USER nonroot:nonroot
ENTRYPOINT ["/ingestion"]
