#!/usr/bin/env bash
set -Eeuo pipefail
APP="${WDTT_APP_DIR:-/opt/wdtt-panel-hs-surisix}"
BIN="${WDTT_BIN:-/usr/local/bin/wdtt-panel-hs-surisix}"
UNIT="/etc/systemd/system/wdtt-panel-hs-surisix.service"
[[ $EUID -eq 0 ]] || { echo 'ERROR: run as root.' >&2; exit 1; }
echo 'This removes only the panel deployment created by this project.'
echo 'It does NOT remove wdtt.service, its database, WireGuard keys or configuration.'
read -r -p 'Type REMOVE-WDTT-PANEL to continue: ' answer
[[ "$answer" == 'REMOVE-WDTT-PANEL' ]] || { echo 'Cancelled.'; exit 1; }
systemctl disable --now wdtt-panel-hs-surisix.service 2>/dev/null || true
rm -f "$UNIT" "$BIN"
rm -rf "$APP"
systemctl daemon-reload
echo 'Panel deployment removed; existing WDTT installation was left untouched.'
