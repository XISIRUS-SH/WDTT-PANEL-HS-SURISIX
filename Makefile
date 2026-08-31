.PHONY: fmt test vet build check web-check panel-check
fmt:
	gofmt -w *.go

test:
	go test ./...

vet:
	go vet ./...

build:
	mkdir -p bin
	go build -trimpath -ldflags='-s -w' -o bin/wdtt-server .

check: fmt test vet build web-check

web-check:
	@test -f panelui/index.html
	@python3 - <<'PY'
from pathlib import Path
import re
p=Path('panelui/index.html')
s=p.read_text(encoding='utf-8')
m=re.findall(r'<script[^>]*>(.*?)</script>', s, re.S)
assert m, 'no inline script found'
Path('/tmp/wdtt-panel-ui-check.js').write_text(m[-1], encoding='utf-8')
PY
	@node --check /tmp/wdtt-panel-ui-check.js
	@echo 'panel UI syntax: OK'

panel-check:
	@bash -n installer/*.sh deploy/*.sh
	@echo 'installer syntax: OK'
