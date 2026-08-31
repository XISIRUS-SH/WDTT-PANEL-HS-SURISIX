# Security notes

- Never commit `main_password`, WireGuard private keys, session cookies or production
  environment files.
- Prefer binding the panel to `127.0.0.1` and exposing it through an existing TLS
  reverse proxy.
- Diagnostics redact common secrets, but always review bundles before sharing.
- CAPTCHA handling is manual; the panel does not automate CAPTCHA solving.
- The panel must use the existing WDTT admin/database synchronization path.
