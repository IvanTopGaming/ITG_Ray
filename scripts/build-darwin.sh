#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist"
mkdir -p "$OUT"

VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
LDFLAGS="-s -w -X main.Version=$VERSION"

# itgray-helper is Windows-only. Skipped.
# Wails GUI darwin build is out of scope (Wails is being removed in Phase 8).

echo ">> building itgray-cli darwin/amd64"
GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$OUT/itgray-cli-darwin-amd64" "$ROOT/cmd/itgray-cli"

SINGBOX_TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,with_ocm,with_naive_outbound,with_purego,badlinkname,tfogo_checklinkname0"
SINGBOX_LDFLAGS="-s -w -buildid= -X internal/godebug.defaultGODEBUG=multipathtcp=0 -checklinkname=0"

echo ">> building sing-box darwin/amd64"
GOOS=darwin GOARCH=amd64 go build -mod=mod -trimpath \
    -ldflags "$SINGBOX_LDFLAGS" \
    -tags "$SINGBOX_TAGS" \
    -o "$OUT/sing-box-darwin-amd64" \
    github.com/sagernet/sing-box/cmd/sing-box

echo ">> building xray darwin/amd64"
GOOS=darwin GOARCH=amd64 go build -mod=mod -trimpath \
    -ldflags "-s -w -buildid=" \
    -o "$OUT/xray-darwin-amd64" \
    github.com/xtls/xray-core/main

# arm64 builds skipped — pragmatic shortcut for Phase 6.
# Add later when arm64 dmg target is enabled (requires paths.ts arch-suffix logic too).

echo "==> dist/ (darwin amd64 artefacts)"
ls -la "$OUT" | grep darwin
