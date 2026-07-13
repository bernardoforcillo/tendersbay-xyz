#!/usr/bin/env sh
set -e

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

# ── Engine detection (Docker → Podman) ────────────────────────────────────────
if docker compose version >/dev/null 2>&1; then
  ENGINE="docker compose"
elif podman compose version >/dev/null 2>&1; then
  ENGINE="podman compose"
else
  echo "Error: 'docker compose' (v2 plugin) or 'podman compose' is required." >&2
  exit 1
fi
COMPOSE="$ENGINE -f $ROOT/docker-compose.dev.yml"

# ── Postgres + Qdrant ──────────────────────────────────────────────────────────
$COMPOSE up -d

printf 'Waiting for postgres'
until $COMPOSE exec -T postgres pg_isready -U root -d tendersbay >/dev/null 2>&1; do
  printf '.'
  sleep 1
done
echo ' ready.'
# Qdrant has no wait loop: the official image ships without curl/wget (no
# in-container healthcheck command), and search indexing is best-effort —
# ingestion logs and skips indexing if it isn't reachable yet, retrying on
# the next Air rebuild.

# ── Backend env ───────────────────────────────────────────────────────────────
export DATABASE_URL="postgres://root:toor@localhost:5432/tendersbay?sslmode=disable"
export JWT_SECRET="dev-only-secret-change-in-production"
export APP_BASE_URL="http://localhost:5173"
export JWT_EXPIRY="15m"
export REFRESH_EXPIRY="168h"
# localhost:5173 = Vite dev server, localhost:3000 = platform Air server
export CORS_ORIGINS="http://localhost:5173,http://localhost:3000"

# ── Backend (Air) — port 8080 ─────────────────────────────────────────────────
(cd "$ROOT/services/backend" && PORT=8080 air) &
BACKEND_PID=$!

# ── Platform (Vite + Air) — Vite :5173, Go server :3000 ──────────────────────
(cd "$ROOT/apps/platform" && PORT=3000 VITE_API_URL=http://localhost:8080 pnpm dev) &
PLATFORM_PID=$!

# ── Ingestion (Air) — batch worker, no port; reruns one full cycle per rebuild,
# scoped to its own `tenders` schema inside the same database as backend ─────
(cd "$ROOT/services/ingestion" && air) &
INGESTION_PID=$!

echo ''
echo '  Backend   (Air)  → http://localhost:8080'
echo '  Platform  (Air)  → http://localhost:3000'
echo '  Frontend  (Vite) → http://localhost:5173'
echo '  Ingestion (Air)  → no port; runs one ingestion cycle per rebuild'
echo '  Qdrant           → http://localhost:6333 (search indexing)'
echo '  Search indexing also needs Ollama running locally with'
echo '  embeddinggemma:latest pulled (`ollama pull embeddinggemma`) — optional,'
echo '  ingestion runs fine without it, just skips indexing until reachable.'
echo '  Press Ctrl+C to stop everything.'
echo ''

cleanup() {
  echo 'Stopping…'
  kill "$BACKEND_PID" "$PLATFORM_PID" "$INGESTION_PID" 2>/dev/null || true
  $COMPOSE stop
}
trap cleanup INT TERM
wait "$BACKEND_PID" "$PLATFORM_PID" "$INGESTION_PID"
