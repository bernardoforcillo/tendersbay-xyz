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

# indexer drains the search-indexing backlog. It is a recurring workload, not
# a one-off like seed-cpv: it has its own CronJob (indexer-cronjob.yaml)
# overriding `command`, because sharing one job with ingestion let a slow
# fetch consume the whole deadline and starve indexing entirely.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /indexer ./cmd/indexer

# enricher drains the eForms-XML backlog (xml_fetched_at IS NULL). A third
# recurring binary rather than a phase of either job above, for the reason the
# indexer already exists: TED's XML host is paced at 250ms between download
# starts, so a drain measured in thousands of requests would eat whichever
# deadline it shared. It gets its own CronJob (enricher-cronjob.yaml)
# overriding `command`.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /enricher ./cmd/enricher

# retriever drains the portal-document backlog (grid_usable = false AND
# docs_fetched_at IS NULL): it follows the buyer's own documents link, reads
# the listing, and extracts the text of the files it publishes. A fourth
# recurring binary, and the split matters more here than for any of the three
# above — this is the only pass whose pace is set by hundreds of DIFFERENT
# third-party hosts, most of them small municipal portals, so its per-host
# politeness bounds make it minutes per tender where the enricher is seconds
# per notice. Sharing a deadline with any of them would starve one of the two.
# It gets its own CronJob (retriever-cronjob.yaml) overriding `command`.
#
# Note CGO_ENABLED=0 here as everywhere: gosseract (OCR) needs cgo and is
# therefore unreachable, which is why a scanned PDF yields no text and the
# retrieval pass measures that gap rather than trying to close it.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /retriever ./cmd/retriever

# ---- Stage 2: minimal runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /ingestion /ingestion
COPY --from=builder /seed-cpv /seed-cpv
COPY --from=builder /indexer /indexer
COPY --from=builder /enricher /enricher
COPY --from=builder /retriever /retriever
USER nonroot:nonroot
ENTRYPOINT ["/ingestion"]
