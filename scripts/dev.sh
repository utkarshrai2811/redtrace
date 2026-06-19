#!/usr/bin/env bash
# Run the RedTrace backend (proxy + API) and the Vite dev server together.
# The Vite dev server proxies /api and /ws to the Go backend on :9090.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "Installing frontend dependencies (if needed)…"
( cd web && npm install )

echo "Starting RedTrace backend…"
go run ./cmd/redtrace serve &
BACKEND=$!
trap 'kill "$BACKEND" 2>/dev/null || true' EXIT INT TERM

echo "Starting Vite dev server on http://127.0.0.1:5173 …"
( cd web && npm run dev )
