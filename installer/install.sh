#!/usr/bin/env bash
set -euo pipefail

OWNER="XISIRUS-SH"; REPO="WDTT-PANEL-HS-SURISIX"; SERVICE="wdtt.service"
BIN="/usr/local/bin/wdtt-server"; BACKUP="/var/backups/wdtt-panel"
DROPIN_DIR="/etc/systemd/system/${SERVICE}.d"; DROPIN="${DROPIN_DIR}/10-panel.conf"

[[ $EUID -eq 0 ]] || { echo 'Нужен root.' >&2; exit 1; }
for cmd in systemctl curl sha256sum ss install awk sed grep; do command -v "$cmd" >/dev/null || { echo "Не найдено: $cmd" >&2; exit 1; }; done
systemctl cat "$SERVICE" >/dev/null 2>&1 || { echo "Не найден $SERVICE. Установщик не создаёт второй WDTT-сервис." >&2; exit 1; }

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ASSET="wdtt-server-linux-amd64";;
  aarch64|arm64) ASSET="wdtt-server-linux-arm64";;
  *) echo "Неподдерживаемая архитектура: $ARCH" >&2; exit 1;;
esac

choose_port() {
  local p
  for p in 8787 8800 8888 9000 8080; do
    if ! ss -H -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "(^|:)${p}$"; then echo "$p"; return 0; fi
  done
  return 1
}
PORT="$(choose_port || true)"
[[ -n "$PORT" ]] || { echo 'Не найден свободный стандартный порт.' >&2; exit 1; }
read -r -p "Порт Web Panel [$PORT]: " ASK || true
PORT="${ASK:-$PORT}"
[[ "$PORT" =~ ^[0-9]+$ && "$PORT" -ge 1 && "$PORT" -le 65535 ]] || { echo 'Некорректный порт.' >&2; exit 1; }
if ss -H -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "(^|:)${PORT}$"; then echo "Порт $PORT занят." >&2; exit 1; fi

BIND="0.0.0.0:${PORT}"
if ip -6 addr show scope global 2>/dev/null | grep -q 'inet6 '; then
  read -r -p 'Использовать IPv6 для панели тоже? [нет]: ' V6 || true
  case "${V6,,}" in да|д|yes|y) BIND="[::]:${PORT}";; esac
fi

API="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
TMP="$(mktemp -d)"; trap 'rm -rf "$TMP"' EXIT
curl -fsSL "$API" -o "$TMP/release.json"
URL="$(grep -o '"browser_download_url"[[:space:]]*:[[:space:]]*"[^"]*"' "$TMP/release.json" | sed -n "s/.*\"\(https:[^\"]*${ASSET}\)\"/\1/p" | head -n1)"
SHAURL="$(grep -o '"browser_download_url"[[:space:]]*:[[:space:]]*"[^"]*"' "$TMP/release.json" | sed -n 's/.*"\(https:[^\"]*SHA256SUMS\)"/\1/p' | head -n1)"
[[ -n "$URL" && -n "$SHAURL" ]] || { echo 'В последнем GitHub Release нет ожидаемых assets.' >&2; exit 1; }
curl -fsSL "$URL" -o "$TMP/$ASSET"
curl -fsSL "$SHAURL" -o "$TMP/SHA256SUMS"
grep "  $ASSET$" "$TMP/SHA256SUMS" | (cd "$TMP" && sha256sum -c -) >/dev/null

# Stage first, so a validation failure never replaces the working binary.
install -d -m 0755 "$BACKUP" "$DROPIN_DIR"
STAGED="$TMP/wdtt-server.staged"
install -m 0755 "$TMP/$ASSET" "$STAGED"

if [[ -f "$BIN" ]]; then cp -a "$BIN" "$BACKUP/wdtt-server.$(date +%Y%m%d-%H%M%S).bak"; fi
cp -a "$STAGED" "$BIN.new"
mv -f "$BIN.new" "$BIN"

cat > "$DROPIN" <<EOF
[Service]
Environment="WDTT_PANEL_LISTEN=$BIND"
EOF
systemctl daemon-reload

if ! systemctl restart "$SERVICE"; then
  echo 'Перезапуск не удался — восстанавливаю предыдущий бинарник и drop-in.' >&2
  LATEST="$(ls -1t "$BACKUP"/wdtt-server.*.bak 2>/dev/null | head -n1 || true)"
  if [[ -n "$LATEST" ]]; then cp -a "$LATEST" "$BIN"; fi
  rm -f "$DROPIN"
  systemctl daemon-reload
  systemctl restart "$SERVICE" || true
  exit 1
fi

if ! systemctl is-active --quiet "$SERVICE"; then
  echo 'wdtt не активен после обновления — выполняю rollback.' >&2
  LATEST="$(ls -1t "$BACKUP"/wdtt-server.*.bak 2>/dev/null | head -n1 || true)"
  if [[ -n "$LATEST" ]]; then cp -a "$LATEST" "$BIN"; fi
  rm -f "$DROPIN"
  systemctl daemon-reload
  systemctl restart "$SERVICE" || true
  exit 1
fi

HOST="$(hostname -f 2>/dev/null || hostname)"
PUBLIC4="$(curl -4fsS --max-time 5 https://api.ipify.org 2>/dev/null || true)"
if [[ "$BIND" == \[*\]* ]]; then URL_HOST="[::1]"; else URL_HOST="$HOST"; fi
echo "Панель установлена в существующий $SERVICE."
echo "URL: http://${URL_HOST}:${PORT}/panel/"
[[ -n "$PUBLIC4" ]] && echo "Публичный IPv4: $PUBLIC4"
echo 'Вход: тот же main_password WDTT.'
