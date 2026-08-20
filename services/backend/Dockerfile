# syntax=docker/dockerfile:1

# ---- Stage 1: compile the Go binary ----
# Built from the repo root (context: .) so the Go workspace (go.work) resolves
# the backend's sibling modules go-services/{token,password}. They are wired in
# through the workspace — not via `replace` in go.mod — so the whole tree is
# copied and go.work drives resolution. Only services/backend is compiled, so
# apps/platform's absent dist (excluded by .dockerignore) is never embedded.
FROM golang:1.26-alpine AS builder
# git is required for Go to resolve the GOPRIVATE internal modules through
# go.work; the alpine image ships without it, so `go mod download` otherwise
# fails with `exec: "git": executable file not found in $PATH`.
RUN apk add --no-cache git
WORKDIR /workspace
COPY . .

# Internal monorepo modules resolve locally through go.work, so keep them off the
# public proxy/sumdb.
ENV GOPRIVATE=github.com/bernardoforcillo/tendersbay-xyz
RUN go mod download

WORKDIR /workspace/services/backend
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /backend .

# The deadline-reminder pass, shipped in this same image and run by a CronJob
# that overrides the entrypoint — the shape services/ingestion already uses for
# its five binaries. It needs this module's internal/ packages (core/alerting,
# the repositories), so it cannot live in the ingestion image, and it does not
# earn a service of its own: see cmd/digest for why.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /digest ./cmd/digest

# ---- Stage 2: minimal runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /backend /backend
COPY --from=builder /digest /digest
EXPOSE 8080
ENV PORT=8080
USER nonroot:nonroot
ENTRYPOINT ["/backend"]
