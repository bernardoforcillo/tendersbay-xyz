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

# ── Postgres ─────────────────────────────────────────────────────────────────
$COMPOSE up -d

printf 'Waiting for postgres'
until $COMPOSE exec -T postgres pg_isready -U root -d tendersbay >/dev/null 2>&1; do
  printf '.'
  sleep 1
done
echo ' ready.'

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

echo ''
echo '  Backend  (Air)  → http://localhost:8080'
echo '  Platform (Air)  → http://localhost:3000'
echo '  Frontend (Vite) → http://localhost:5173'
echo '  Press Ctrl+C to stop everything.'
echo ''

cleanup() {
  echo 'Stopping…'
  kill "$BACKEND_PID" "$PLATFORM_PID" 2>/dev/null || true
  $COMPOSE stop
}
trap cleanup INT TERM
wait "$BACKEND_PID" "$PLATFORM_PID"
