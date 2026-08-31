# Production runbook

## Install from GitHub

After publishing the repository and a Linux release artifact:

```bash
curl -fsSL https://raw.githubusercontent.com/XISIRUS-SH/WDTT-Plus-15-WDTT-PANEL/main/install.sh | sudo bash
```

If the repository name differs, set `WDTT_REPO_RAW` to the raw `main` directory.

## Watch logs

```bash
journalctl -u wdtt-panel.service -n 200 -f
```

For the complete WDTT server:

```bash
journalctl -u wdtt.service -n 500 -f
```

## Collect a safe diagnostics bundle

```bash
sudo /opt/wdtt-panel/collect-logs.sh
```

The script creates a compressed bundle and redacts common password/token/private-key
patterns. Still inspect the archive before sharing it.

## Complete removal

```bash
curl -fsSL https://raw.githubusercontent.com/XISIRUS-SH/WDTT-Plus-15-WDTT-PANEL/main/uninstall.sh | sudo bash
```

The uninstaller removes the panel binary, panel systemd unit and panel directory.
It deliberately does not delete the existing WDTT database, WireGuard keys or the
original `wdtt.service`, because doing that would be destructive to the user's
existing VPN installation.

If you want a destructive purge of the entire WDTT product, that must be a separate,
explicit operation against the exact deployment paths and must not be hidden inside
the panel uninstaller.
