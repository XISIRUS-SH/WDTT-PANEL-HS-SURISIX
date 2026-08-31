#!/usr/bin/env bash
set -euo pipefail
UNIT="wdtt.service"; DROPIN_DIR="/etc/systemd/system/${UNIT}.d"; DROPIN="${DROPIN_DIR}/10-panel.conf"; BACKUP_DIR="/var/backups/wdtt-panel"
[[ $EUID -eq 0 ]] || { echo "Запустите от root." >&2; exit 1; }
command -v systemctl >/dev/null || { echo "systemd не найден." >&2; exit 1; }
systemctl cat "$UNIT" >/dev/null 2>&1 || { echo "Не найден $UNIT. Установщик не создаёт второй WDTT-сервис." >&2; exit 1; }
free_port(){ for p in 8787 8800 8888 9000 8080; do if ! ss -H -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "(^|:)${p}$"; then echo "$p"; return; fi; done; return 1; }
PORT="$(free_port || true)"; PORT="${PORT:-8787}"; read -r -p "Порт панели [$PORT]: " ASK || true; PORT="${ASK:-$PORT}"
[[ "$PORT" =~ ^[0-9]+$ && "$PORT" -ge 1 && "$PORT" -le 65535 ]] || { echo "Некорректный порт." >&2; exit 1; }
if ss -H -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "(^|:)${PORT}$"; then echo "Порт $PORT уже занят. Ничего не меняю." >&2; exit 1; fi
PUBLIC4="$(curl -4fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"; [[ -n "$PUBLIC4" ]] && echo "Публичный IPv4: $PUBLIC4" || echo "Публичный IPv4 определить не удалось."
BIND="0.0.0.0:${PORT}"
if ip -6 addr show scope global >/dev/null 2>&1 && ip -6 addr show scope global | grep -q 'inet6 '; then read -r -p "Обнаружен IPv6. Использовать IPv6 для панели тоже? [нет]: " IPV6 || true; case "${IPV6,,}" in да|д|yes|y) BIND="[::]:${PORT}";; esac; fi
mkdir -p "$DROPIN_DIR" "$BACKUP_DIR"; if [[ -f "$DROPIN" ]]; then cp -a "$DROPIN" "$BACKUP_DIR/10-panel.conf.$(date +%Y%m%d-%H%M%S).bak"; fi
cat > "$DROPIN" <<EOF2
[Service]
Environment="WDTT_PANEL_LISTEN=$BIND"
EOF2
systemctl daemon-reload; systemctl restart "$UNIT"
if ! systemctl is-active --quiet "$UNIT"; then echo "wdtt не запустился — откат." >&2; rm -f "$DROPIN"; systemctl daemon-reload; systemctl restart "$UNIT" || true; exit 1; fi
echo; echo "WDTT Panel настроен в существующем $UNIT."; echo "Адрес: http://$BIND/panel/"; echo "Панель не создаёт отдельную БД или отдельный WDTT-сервис."; echo "Сохрани main_password: он используется для входа в панель."
