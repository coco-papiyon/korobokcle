#!/usr/bin/env bash

set -u

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_PORT="8081"
FORWARD_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --backend-port|-p)
      if [[ $# -lt 2 ]]; then
        echo "Missing value for $1"
        exit 1
      fi
      BACKEND_PORT="$2"
      shift 2
      ;;
    *)
      FORWARD_ARGS+=("$1")
      shift
      ;;
  esac
done

BACKEND_PORT="${BACKEND_PORT#:}"

if [[ ! -d "$ROOT/frontend/node_modules" ]]; then
  echo "Installing frontend dependencies..."
  (
    cd "$ROOT/frontend" || exit 1
    npm ci
  ) || exit 1
fi

echo "Building frontend..."
(
  cd "$ROOT/frontend" || exit 1
  npm run build
) || exit 1

echo "Syncing frontend build to tests static contents..."
STATIC_DIR="$ROOT/tests/static"
DIST_DIR="$ROOT/frontend/dist"
mkdir -p "$STATIC_DIR"
if command -v rsync >/dev/null 2>&1; then
  rsync -a --delete "$DIST_DIR"/ "$STATIC_DIR"/ || exit 1
else
  rm -rf "$STATIC_DIR"
  mkdir -p "$STATIC_DIR"
  cp -a "$DIST_DIR"/. "$STATIC_DIR"/ || exit 1
fi

echo "Building backend executable..."
(
  cd "$ROOT" || exit 1
  go build -o "$ROOT/tests/korobokcle" ./cmd/korobokcle
) || exit 1

echo "Running korobokcle from tests directory..."
(
  cd "$ROOT/tests" || exit 1

  echo "Creating test data..."
  go run ./scripts/create-testdata -root "." || exit 1

  echo "Starting korobokcle in mock mode..."
  ./korobokcle --addr ":$BACKEND_PORT" --mock-mode "${FORWARD_ARGS[@]}"
)
