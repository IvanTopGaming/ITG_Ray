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

echo ">> building itgray-gui.exe (ITGRay.exe) (version=$VERSION)"
# wails build emits to build/bin/<name>; -o is the filename, not a path.
# We move the artifact to $OUT after the build completes.
( cd "$ROOT/cmd/itgray-gui" && \
  GOOS=windows GOARCH=amd64 wails build -clean -platform windows/amd64 \
    -ldflags "-X main.Version=$VERSION" -o "ITGRay.exe" )
mv "$ROOT/cmd/itgray-gui/build/bin/ITGRay.exe" "$OUT/ITGRay.exe"

echo ">> copying wintun.dll"
cp "$ROOT/third_party/wintun/wintun.dll" "$OUT/wintun.dll"

# sing-box and xray-core are imported as Go modules (see go.mod / internal/core).
# Build their stock cmd packages for windows/amd64 so the Helper can spawn them
# from C:\Program Files\ITG Ray\ on the VM.
#
# sing-box upstream tag set + ldflags come from
#   $GOMODCACHE/github.com/sagernet/sing-box@vX/release/DEFAULT_BUILD_TAGS_WINDOWS
#   $GOMODCACHE/github.com/sagernet/sing-box@vX/release/LDFLAGS
# The -checklinkname=0 ldflag is required: sing-box uses //go:linkname to access
# internal Go runtime symbols, which Go 1.23+ rejects without that flag.
#
# xray-core upstream needs no tags: ./main blank-imports
# github.com/xtls/xray-core/main/distro/all which pulls in every feature.
#
# -mod=mod is needed because the cmd packages pull in transitive dependencies
# (e.g. maxminddb, wgctrl) that the rest of the module never imports, so a
# go-mod-tidy run can't keep them in go.sum.
SINGBOX_TAGS="with_gvisor,with_quic,with_dhcp,with_wireguard,with_utls,with_acme,with_clash_api,with_tailscale,with_ccm,with_ocm,with_naive_outbound,with_purego,badlinkname,tfogo_checklinkname0"
SINGBOX_LDFLAGS="-s -w -buildid= -X internal/godebug.defaultGODEBUG=multipathtcp=0 -checklinkname=0"

echo ">> building sing-box.exe (tags=$SINGBOX_TAGS)"
GOOS=windows GOARCH=amd64 go build -mod=mod -trimpath \
    -ldflags "$SINGBOX_LDFLAGS" \
    -tags "$SINGBOX_TAGS" \
    -o "$OUT/sing-box.exe" \
    github.com/sagernet/sing-box/cmd/sing-box

echo ">> building xray.exe"
GOOS=windows GOARCH=amd64 go build -mod=mod -trimpath \
    -ldflags "-s -w -buildid=" \
    -o "$OUT/xray.exe" \
    github.com/xtls/xray-core/main

echo "==> dist/"
ls -la "$OUT"
