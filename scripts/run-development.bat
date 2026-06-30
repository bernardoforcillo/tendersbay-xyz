@echo off
setlocal EnableDelayedExpansion

set ROOT=%~dp0..

:: ── Postgres ──────────────────────────────────────────────────────────────────
docker compose -f "%ROOT%\docker-compose.dev.yml" up -d

echo Waiting for postgres...
:wait_loop
docker compose -f "%ROOT%\docker-compose.dev.yml" exec -T postgres pg_isready -U root -d tendersbay >nul 2>&1
if errorlevel 1 (
  timeout /t 1 /nobreak >nul
  goto wait_loop
)
echo Postgres ready.

:: ── Backend env ───────────────────────────────────────────────────────────────
set DATABASE_URL=postgres://root:toor@localhost:5432/tendersbay?sslmode=disable
set JWT_SECRET=dev-only-secret-change-in-production
set APP_BASE_URL=http://localhost:5173
set JWT_EXPIRY=15m
set REFRESH_EXPIRY=168h
:: localhost:5173 = Vite dev server, localhost:3000 = platform Air server
set CORS_ORIGINS=http://localhost:5173,http://localhost:3000
set VITE_API_URL=http://localhost:8080

:: ── Backend (Air) — port 8080 ─────────────────────────────────────────────────
start "Backend (Air :8080)" cmd /k "cd /d "%ROOT%\services\backend" && set PORT=8080 && air"

:: ── Platform (Vite :5173 + Air :3000) ────────────────────────────────────────
start "Platform (Vite+Air)" cmd /k "cd /d "%ROOT%\apps\platform" && set PORT=3000 && pnpm dev"

echo.
echo   Backend  (Air)  ^>  http://localhost:8080
echo   Platform (Air)  ^>  http://localhost:3000
echo   Frontend (Vite) ^>  http://localhost:5173
echo   Close the opened windows to stop the servers.
echo   Run: docker compose -f docker-compose.dev.yml stop   ^<-- to stop postgres
echo.
