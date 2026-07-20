#!/usr/bin/env bash
set -euo pipefail

# Assembles the Linux release tarball the AUR itgray-bin PKGBUILD consumes.
# Layout: ITGRay-<version>-linux-x64/{app/, itgray.desktop,
# itgray-helper.service, itgray-bin.install, icon.png} where app/ is
# electron-builder's linux-unpacked output (helper/cores under resources/).
# Run scripts/build-linux.sh first.

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
VERSION="${1:?usage: make-linux-tarball.sh <version>}"
UNPACKED="$ROOT/cmd/itgray-electron/dist-installer/linux-unpacked"
STAGE_NAME="ITGRay-${VERSION}-linux-x64"
STAGE="$ROOT/dist/$STAGE_NAME"

[[ -x "$UNPACKED/itgray-electron" ]] || { echo "error: run build-linux.sh first" >&2; exit 1; }

rm -rf "$STAGE"
mkdir -p "$STAGE"
cp -a "$UNPACKED" "$STAGE/app"
# electron-builder leaks the Windows bridge and a build log into the bundle.
rm -f "$STAGE/app/resources/bridge/itgray-bridge.exe" \
      "$STAGE/app/resources/bridge/build-errors.log"
cp "$ROOT/packaging/arch/itgray.desktop" \
   "$ROOT/packaging/arch/itgray-helper.service" \
   "$ROOT/packaging/arch/itgray-bin.install" "$STAGE/"
cp "$ROOT/cmd/itgray-electron/resources/icon.png" "$STAGE/icon.png"

tar -C "$ROOT/dist" -czf "$ROOT/dist/$STAGE_NAME.tar.gz" "$STAGE_NAME"
rm -rf "$STAGE"
echo "==> dist/$STAGE_NAME.tar.gz"
