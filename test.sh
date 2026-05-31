#!/usr/bin/env bash

set -euo pipefail

export KOROBOKCLE_PORT="${KOROBOKCLE_PORT:-8080}"
export KOROBOKCLE_TOOL_ROOT="tests/data"

cd frontend
npm ci
npm run build
cd ..

go run ./scripts/create-testdata

mkdir -p ${KOROBOKCLE_TOOL_ROOT}/skills
cp -rf skills/default ${KOROBOKCLE_TOOL_ROOT}/skills/default

go build ./cmd/korobokcle
./korobokcle --port "${KOROBOKCLE_PORT}"
