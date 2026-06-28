# syntax=docker/dockerfile:1

# ---- Stage 1: compile the Go binary ----
FROM golang:1.26-alpine AS build
WORKDIR /src
# Manifest-first copy so module download caches independently of source churn.
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /backend .

# ---- Stage 2: minimal runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /backend /backend
EXPOSE 8080
ENV PORT=8080
USER nonroot:nonroot
ENTRYPOINT ["/backend"]
