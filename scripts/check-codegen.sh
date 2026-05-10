#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

echo ">> running go generate"
go generate ./...

if ! git diff --exit-code -- cmd/itgray-electron/src/shared/protocol.ts internal/bridge/protocol/; then
    echo ""
    echo "ERROR: codegen output is out of date. Run:"
    echo "    go generate ./..."
    echo "and commit the diff."
    exit 1
fi

echo "OK: codegen up to date"
