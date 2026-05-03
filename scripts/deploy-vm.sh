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

# helper service control: Stop-Service blocks until Stopped (sc stop returns
# while service is still in STOP_PENDING, which races with the move below).
helper_is_running() {
    local out
    out="$(ssh "$VM_USER@$VM_HOST" 'powershell -NoProfile -Command "$s = Get-Service -Name ITGRayHelper -ErrorAction SilentlyContinue; if ($s -and $s.Status -eq \"Running\") { \"yes\" } else { \"no\" }"' 2>/dev/null | tr -d '\r\n ')"
    [[ "$out" == "yes" ]]
}

stop_helper() {
    echo ">> stopping ITGRayHelper service"
    ssh "$VM_USER@$VM_HOST" 'powershell -NoProfile -Command "Stop-Service -Name ITGRayHelper -Force -ErrorAction Stop"'
}

start_helper() {
    echo ">> starting ITGRayHelper service"
    ssh "$VM_USER@$VM_HOST" 'powershell -NoProfile -Command "Start-Service -Name ITGRayHelper -ErrorAction Stop"'
}

HELPER_WAS_RUNNING=0
restart_helper_if_needed() {
    if [[ $HELPER_WAS_RUNNING -eq 1 ]]; then
        start_helper || echo "warning: helper restart failed; check service state on VM" >&2
    fi
}

if helper_is_running; then
    HELPER_WAS_RUNNING=1
    stop_helper
fi
# Trap runs on both clean exit and failure so a half-deployed VM still
# gets the helper service restarted.
trap restart_helper_if_needed EXIT

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
