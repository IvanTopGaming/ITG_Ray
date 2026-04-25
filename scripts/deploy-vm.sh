#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"

VM_USER="${VM_USER:-itg}"
VM_HOST="${VM_HOST:-10.20.0.3}"
VM_DEST='C:/Program Files/ITG Ray'

if [[ ! -f "$DIST/itgray-helper.exe" ]]; then
    echo "dist/ not built; run scripts/build-windows.sh first" >&2
    exit 1
fi

echo ">> ensuring destination directory exists on VM"
ssh "$VM_USER@$VM_HOST" "if not exist \"$VM_DEST\" mkdir \"$VM_DEST\""

echo ">> copying binaries + wintun.dll"
scp "$DIST/itgray-helper.exe" "$DIST/itgray-cli.exe" "$DIST/wintun.dll" \
    "$VM_USER@$VM_HOST:\"$VM_DEST\""

echo ">> verifying"
ssh "$VM_USER@$VM_HOST" "dir \"$VM_DEST\""
echo "done."
