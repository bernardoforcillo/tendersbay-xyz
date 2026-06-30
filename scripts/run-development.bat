@echo off
setlocal EnableDelayedExpansion

set ROOT=%~dp0..

:: ── Engine detection (Docker → Podman) ────────────────────────────────────────
docker compose version >nul 2>&1
if not errorlevel 1 (
  set ENGINE=docker compose
  goto engine_ok
)
podman compose version >nul 2>&1
if not errorlevel 1 (
  set ENGINE=podman compose
  goto engine_ok
)
echo Error: "docker compose" (v2 plugin) or "podman compose" is required.
exit /b 1
:engine_ok

:: ── Postgres ──────────────────────────────────────────────────────────────────
%ENGINE% -f "%ROOT%\docker-compose.dev.yml" up -d

echo Waiting for postgres...
:wait_loop
%ENGINE% -f "%ROOT%\docker-compose.dev.yml" exec -T postgres pg_isready -U root -d tendersbay >nul 2>&1
if errorlevel 1 (
  timeout /t 1 /nobreak >nul
  goto wait_loop
)
echo Postgres ready.

:: ── Backend env (inherited by the window started below) ───────────────────────
set DATABASE_URL=postgres://root:toor@localhost:5432/tendersbay?sslmode=disable
set JWT_SECRET=dev-only-secret-change-in-production
set APP_BASE_URL=http://localhost:5173
set JWT_EXPIRY=15m
set REFRESH_EXPIRY=168h
set CORS_ORIGINS=http://localhost:5173,http://localhost:3000

:: ── Backend (Air) — port 8080 ─────────────────────────────────────────────────
:: Set PORT before start so the new window inherits the exact value (no trailing
:: space — "set VAR=val && cmd" includes the space before && in the value).
set PORT=8080
start "Backend (Air :8080)" cmd /k "cd /d "%ROOT%\services\backend" && air"

:: ── Platform (Vite :5173 + Air :3000) ────────────────────────────────────────
set PORT=3000
set VITE_API_URL=http://localhost:8080
start "Platform (Vite+Air)" cmd /k "cd /d "%ROOT%\apps\platform" && pnpm dev"

echo.
echo   Backend  (Air)  ^>  http://localhost:8080
echo   Platform (Air)  ^>  http://localhost:3000
echo   Frontend (Vite) ^>  http://localhost:5173
echo   Close the opened windows to stop the servers.
echo   Run: %ENGINE% -f docker-compose.dev.yml stop   ^<-- to stop postgres
echo.
