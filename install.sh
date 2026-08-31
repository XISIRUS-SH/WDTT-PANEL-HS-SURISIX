#!/usr/bin/env bash
set -Eeuo pipefail
REPO_URL="${WDTT_REPO_URL:-https://github.com/XISIRUS-SH/WDTT-PANEL-HS-SURISIX.git}"
APP="${WDTT_APP_DIR:-/opt/wdtt-panel-hs-surisix}"
BIN="${WDTT_BIN:-/usr/local/bin/wdtt-panel-hs-surisix}"
UNIT="/etc/systemd/system/wdtt-panel-hs-surisix.service"
PORT="${WDTT_PANEL_PORT:-8787}"
BACKUP="${APP}.backup-$(date +%Y%m%d-%H%M%S)"
[[ $EUID -eq 0 ]] || { echo 'ERROR: run as root (sudo bash).' >&2; exit 1; }
command -v git >/dev/null 2>&1 || { apt-get update && apt-get install -y git; }
command -v curl >/dev/null 2>&1 || { apt-get update && apt-get install -y curl ca-certificates; }
command -v ss >/dev/null 2>&1 || { apt-get update && apt-get install -y iproute2; }
if ss -ltnH 2>/dev/null | awk '{print $4}' | grep -Eq "[:.]${PORT}$"; then echo "ERROR: TCP port ${PORT} is already occupied." >&2; exit 1; fi
version_ok(){
  command -v go >/dev/null 2>&1 || return 1
  local v; v=$(go version | sed -n 's/.*go\([0-9][0-9.]*\).*/\1/p')
  [[ -n "$v" ]] && [[ "$(printf '%s\n' '1.25.0' "$v" | sort -V | head -1)" == '1.25.0' ]]
}
if ! version_ok; then
  echo 'Installing Go 1.25.0...'
  tg=$(mktemp -d); trap 'rm -rf "$tg"' EXIT
  curl -fL --retry 3 https://go.dev/dl/go1.25.0.linux-amd64.tar.gz -o "$tg/go.tgz"
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$tg/go.tgz"
  export PATH="/usr/local/go/bin:$PATH"
fi
t=$(mktemp -d); trap 'rm -rf "$t"' EXIT
git clone --depth 1 "$REPO_URL" "$t/src"
cd "$t/src"
for f in go.mod server.go panel.go panelui/index.html; do [[ -f "$f" ]] || { echo "ERROR: repository root is missing $f" >&2; exit 1; }; done
# Validate before touching the live deployment.
go test ./...
go vet ./...
go build -trimpath -ldflags='-s -w' -o "$t/wdtt-server" .
if [[ -d "$APP" ]]; then mv "$APP" "$BACKUP"; fi
mkdir -p "$APP"
cp -a "$t/src/." "$APP/"
install -m 0755 "$t/wdtt-server" "$BIN"
cat > "$UNIT" <<UNIT2
[Unit]
Description=WDTT Plus 15 integrated Web Panel
After=network-online.target
Wants=network-online.target
[Service]
Type=simple
ExecStart=$BIN --panel-listen 127.0.0.1:$PORT
WorkingDirectory=$APP
Restart=on-failure
RestartSec=2
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
[Install]
WantedBy=multi-user.target
UNIT2
systemctl daemon-reload
if ! systemctl enable --now wdtt-panel-hs-surisix.service; then
  echo 'ERROR: service failed; rolling back.' >&2
  rm -rf "$APP"; [[ -d "$BACKUP" ]] && mv "$BACKUP" "$APP"
  rm -f "$BIN" "$UNIT"; systemctl daemon-reload; exit 1
fi
echo
echo '=== INSTALLED ==='
echo "Project: $APP"
echo "Panel:   http://127.0.0.1:$PORT/panel/"
echo
echo '=== RECENT LOGS ==='
journalctl -u wdtt-panel-hs-surisix.service -n 120 --no-pager || true
echo
echo '=== LIVE LOGS (Ctrl-C to stop) ==='
journalctl -u wdtt-panel-hs-surisix.service -f
