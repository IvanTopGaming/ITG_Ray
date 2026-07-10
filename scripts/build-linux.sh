#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/dist"
mkdir -p "$OUT"

VERSION="${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)}"
LDFLAGS="-s -w -X main.Version=$VERSION"

echo ">> building itgray-helper (version=$VERSION)"
GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$OUT/itgray-helper" "$ROOT/cmd/itgray-helper"

echo ">> building itgray-cli (version=$VERSION)"
GOOS=linux GOARCH=amd64 go build -ldflags "$LDFLAGS" -o "$OUT/itgray-cli" "$ROOT/cmd/itgray-cli"

SINGBOX_TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,with_ocm,with_naive_outbound,with_purego,badlinkname,tfogo_checklinkname0"
SINGBOX_LDFLAGS="-s -w -buildid= -X internal/godebug.defaultGODEBUG=multipathtcp=0 -checklinkname=0"

echo ">> building sing-box (tags=$SINGBOX_TAGS)"
GOOS=linux GOARCH=amd64 go build -mod=mod -trimpath \
    -ldflags "$SINGBOX_LDFLAGS" \
    -tags "$SINGBOX_TAGS" \
    -o "$OUT/sing-box" \
    github.com/sagernet/sing-box/cmd/sing-box

echo ">> building xray"
GOOS=linux GOARCH=amd64 go build -mod=mod -trimpath \
    -ldflags "-s -w -buildid=" \
    -o "$OUT/xray" \
    github.com/xtls/xray-core/main

echo "==> dist/ (linux artifacts)"
ls -la "$OUT" | grep -vE '\.exe|\.dll' || true

# ----- Electron AppImage -----
echo ">> building itgray-bridge for Linux (Electron bundle)"
( cd "$ROOT/cmd/itgray-electron" && npm run build:bridge:linux )

echo ">> building Electron bundle (main + preload + frontend)"
( cd "$ROOT/cmd/itgray-electron" && npm run build:main && npm run build:preload && npm run build:frontend )

echo ">> running electron-builder for Linux AppImage"
( cd "$ROOT/cmd/itgray-electron" && npx electron-builder --linux )

APPIMAGE=$(ls "$ROOT/cmd/itgray-electron/dist-installer/ITGRay-"*.AppImage 2>/dev/null | head -1)
if [[ -n "$APPIMAGE" && -f "$APPIMAGE" ]]; then
    echo ">> copying AppImage to dist/"
    cp "$APPIMAGE" "$OUT/"
    chmod +x "$OUT/$(basename "$APPIMAGE")"
    echo "==> Electron AppImage:"
    ls -la "$OUT/ITGRay-"*.AppImage
else
    echo "warning: AppImage not found in dist-installer/" >&2
fi
