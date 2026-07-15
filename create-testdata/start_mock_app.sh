#!/usr/bin/env bash

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$SCRIPT_DIR/mock-app"

if [[ ! -f "$APP_DIR/package.json" ]]; then
  echo "tests/mock-app not found. Run go run ./create-testdata first."
  exit 1
fi

echo "Changing to script directory: $SCRIPT_DIR"
echo "Starting mock app with npm run dev..."

cd "$APP_DIR" || exit 1
npm run dev
