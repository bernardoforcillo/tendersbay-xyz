# syntax=docker/dockerfile:1

# ---- Stage 1: compile the Go binary ----
# Built from the repo root (context: .) so the Go workspace (go.work)
# resolves ingestion's sibling modules go-services/tender and
# go-services/telemetry. They're wired in through the workspace — not via
# `replace` in go.mod — so the whole tree is copied and go.work drives
# resolution.
FROM golang:1.26-alpine AS builder
WORKDIR /workspace
COPY . .

# Internal monorepo modules resolve locally through go.work, so keep them off
# the public proxy/sumdb.
ENV GOPRIVATE=github.com/bernardoforcillo/tendersbay-xyz
RUN go mod download

WORKDIR /workspace/services/ingestion
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /ingestion .

# ---- Stage 2: minimal runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /ingestion /ingestion
USER nonroot:nonroot
ENTRYPOINT ["/ingestion"]
