#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist"
mkdir -p "$OUT"

VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
LDFLAGS="-s -w -X main.Version=$VERSION"

echo ">> building itgray-helper.exe (version=$VERSION)"
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$OUT/itgray-helper.exe" "$ROOT/cmd/itgray-helper"

echo ">> building itgray-cli.exe (version=$VERSION)"
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$OUT/itgray-cli.exe" "$ROOT/cmd/itgray-cli"

echo ">> copying wintun.dll"
cp "$ROOT/third_party/wintun/wintun.dll" "$OUT/wintun.dll"

echo "==> dist/"
ls -la "$OUT"
