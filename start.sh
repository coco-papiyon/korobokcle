#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
FRONTEND_PID=""

cleanup() {
  if [[ -n "$FRONTEND_PID" ]] && kill -0 "$FRONTEND_PID" 2>/dev/null; then
    kill "$FRONTEND_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

is_port_listening() {
  local port="$1"
  if command -v ss >/dev/null 2>&1; then
    ss -ltn "sport = :$port" 2>/dev/null | tail -n +2 | grep -q .
    return
  fi
  if command -v lsof >/dev/null 2>&1; then
    lsof -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1
    return
  fi
  return 1
}

if [[ ! -d "$FRONTEND_DIR/node_modules" ]]; then
  echo "Installing frontend dependencies..."
  (
    cd "$FRONTEND_DIR"
    npm ci
  )
fi

if is_port_listening 5173; then
  echo "Frontend is already running on http://localhost:5173. Skipping startup."
else
  echo "Starting frontend at http://localhost:5173..."
  (
    cd "$FRONTEND_DIR"
    npm run dev
  ) &
  FRONTEND_PID="$!"
  echo "Frontend source changes are applied automatically by Vite HMR."
fi

echo "Backend runs in this terminal."
echo "Starting backend at http://localhost:8080..."
go run ./cmd/korobokcle --tool-dir "$ROOT_DIR" --work-dir "$ROOT_DIR" "$@"
