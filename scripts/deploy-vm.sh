#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"

VM_USER="${VM_USER:-itg}"
VM_HOST="${VM_HOST:-10.20.0.3}"
VM_DEST='C:/Program Files/ITG Ray'

if [[ ! -f "$DIST/itgray-helper.exe" || ! -f "$DIST/ITGRay.exe" ]]; then
    echo "dist/ not built; run scripts/build-windows.sh first" >&2
    exit 1
fi

echo ">> ensuring destination directory exists on VM"
ssh "$VM_USER@$VM_HOST" "if not exist \"$VM_DEST\" mkdir \"$VM_DEST\""

echo ">> staging in %USERPROFILE%\\itgray-stage"
ssh "$VM_USER@$VM_HOST" "if not exist \"%USERPROFILE%\\itgray-stage\" mkdir \"%USERPROFILE%\\itgray-stage\""

scp "$DIST/itgray-helper.exe" "$DIST/itgray-cli.exe" "$DIST/ITGRay.exe" \
    "$DIST/wintun.dll" "$DIST/sing-box.exe" "$DIST/xray.exe" \
    "$VM_USER@$VM_HOST:itgray-stage/"

echo ">> moving stage -> $VM_DEST"
ssh "$VM_USER@$VM_HOST" "move /Y \"%USERPROFILE%\\itgray-stage\\*.*\" \"$VM_DEST\\\""

echo ">> verifying"
ssh "$VM_USER@$VM_HOST" "dir \"$VM_DEST\""
echo "done."
