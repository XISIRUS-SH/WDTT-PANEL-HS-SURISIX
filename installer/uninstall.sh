#!/usr/bin/env bash
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'Нужен root.' >&2; exit 1; }
SERVICE="wdtt.service"; DROPIN="/etc/systemd/system/${SERVICE}.d/10-panel.conf"
if command -v systemctl >/dev/null && systemctl cat "$SERVICE" >/dev/null 2>&1; then
  rm -f "$DROPIN"
  systemctl daemon-reload
  systemctl restart "$SERVICE" || true
fi
echo 'Настройка Web Panel удалена. Существующий WDTT бинарник и данные НЕ удалены.'
