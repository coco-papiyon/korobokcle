#!/usr/bin/env bash

set -euo pipefail

export KOROBOKCLE_COPILOT_DEBUG="${KOROBOKCLE_COPILOT_DEBUG:-1}"
export KOROBOKCLE_TOOL_ROOT="${KOROBOKCLE_TOOL_ROOT:-exec/base}"

cd frontend
npm run build
cd ..

mkdir -p exec/base/skills
cp -R skills/default exec/base/skills/

go build ./cmd/korobokcle
./korobokcle
cp -rf skills/default exec/base/skills/default

go build ./cmd/korobokcle
./korobokcle
