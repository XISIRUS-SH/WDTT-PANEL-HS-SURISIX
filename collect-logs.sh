#!/usr/bin/env bash
set -Eeuo pipefail
OUT="${1:-wdtt-panel-diagnostics-$(date +%Y%m%d-%H%M%S)}"
mkdir -p "$OUT"
run(){ local n="$1"; shift; { "$@"; } >"$OUT/$n.txt" 2>&1 || true; }
run system uname -a
run os-release cat /etc/os-release
run go-version go version
run panel-status systemctl status wdtt-panel-hs-surisix.service --no-pager
run wdtt-status systemctl status wdtt.service --no-pager
run panel-journal journalctl -u wdtt-panel-hs-surisix.service -n 3000 --no-pager
run wdtt-journal journalctl -u wdtt.service -n 3000 --no-pager
run ports ss -lntup
run routes ip route
run wg wg show
run disk df -h
run memory free -h
python3 - "$OUT" <<'PY'
from pathlib import Path
import re,sys
root=Path(sys.argv[1])
patterns=[(re.compile(r'(?i)(password|passwd|token|cookie|authorization|private[_-]?key)\s*[:=]\s*\S+'),r'\1=[REDACTED]'),(re.compile(r'(?i)Bearer\s+\S+'),'Bearer [REDACTED]')]
for p in root.rglob('*.txt'):
 s=p.read_text(errors='replace')
 for a,b in patterns:s=a.sub(b,s)
 p.write_text(s)
PY
tar -czf "${OUT}.tar.gz" "$OUT"
rm -rf "$OUT"
echo "Created ${OUT}.tar.gz"
