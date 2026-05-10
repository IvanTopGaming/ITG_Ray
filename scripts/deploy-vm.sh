#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"

VM_USER="${VM_USER:-itg}"
VM_HOST="${VM_HOST:-10.20.0.3}"

INSTALLER=$(ls "$DIST/ITGRay-Setup-"*.exe 2>/dev/null | head -1)
if [[ -z "$INSTALLER" || ! -f "$INSTALLER" ]]; then
    echo "no NSIS installer in dist/; run scripts/build-windows.sh first" >&2
    exit 1
fi
INSTALLER_NAME=$(basename "$INSTALLER")

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

echo ">> deploying $INSTALLER_NAME"

echo ">> staging in %USERPROFILE%\\itgray-stage"
ssh "$VM_USER@$VM_HOST" "if not exist \"%USERPROFILE%\\itgray-stage\" mkdir \"%USERPROFILE%\\itgray-stage\""

scp "$INSTALLER" "$VM_USER@$VM_HOST:itgray-stage/$INSTALLER_NAME"

# Uninstall any prior version. NSIS records the uninstaller path in
# HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall — match by
# DisplayName then run with /S for silent uninstall.
echo ">> uninstalling prior version (if any)"
ssh "$VM_USER@$VM_HOST" 'powershell -NoProfile -Command "$u = Get-ItemProperty HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\* -ErrorAction SilentlyContinue | Where-Object { $_.DisplayName -eq \"ITG Ray\" } | Select-Object -First 1 -ExpandProperty UninstallString; if ($u) { $exe = ($u -replace \"^`\"|`\"$\", \"\"); Start-Process -FilePath $exe -ArgumentList \"/S\" -Wait -ErrorAction SilentlyContinue }"'

echo ">> running NSIS installer silently"
ssh "$VM_USER@$VM_HOST" "%USERPROFILE%\\itgray-stage\\$INSTALLER_NAME /S"

echo ">> verifying install"
ssh "$VM_USER@$VM_HOST" "dir \"%LOCALAPPDATA%\\Programs\\ITG Ray\""

echo ">> cleaning stage"
ssh "$VM_USER@$VM_HOST" "del /Q \"%USERPROFILE%\\itgray-stage\\$INSTALLER_NAME\""

echo "done."
