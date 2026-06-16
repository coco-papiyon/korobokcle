#!/usr/bin/env bash

set -euo pipefail

export KOROBOKCLE_COPILOT_DEBUG="${KOROBOKCLE_COPILOT_DEBUG:-1}"
export KOROBOKCLE_TOOL_ROOT="${KOROBOKCLE_TOOL_ROOT:-tests/base}"

cd frontend
npm run build
cd ..

mkdir -p ${KOROBOKCLE_TOOL_ROOT}/skills/default
cp -rf skills/default/* ${KOROBOKCLE_TOOL_ROOT}/skills/default/.

go build ./cmd/korobokcle
./korobokcle
