# WDTT Web Panel

The Web Panel is integrated into the existing `wdtt-server` process and uses the same
admin database and runtime integration as the Android/Telegram paths.

## Run

Default:

```text
127.0.0.1:8787/panel/
```

Configure with:

```text
--panel-listen 0.0.0.0:8787
```

or:

```text
WDTT_PANEL_LISTEN=0.0.0.0:8787
```

Use `--panel-listen -` to disable the panel.

## Security

- Main WDTT password is used for authentication.
- Password is not persisted in browser storage.
- Session is HttpOnly and SameSite Strict.
- CSRF token is required for state-changing operations.
- Login and mutation requests are rate limited.
- JSON request sizes are bounded.
- Destructive UI operations require confirmation.

For internet exposure, put the panel behind HTTPS/reverse proxy or an equivalent
network access control layer. The built-in listener is HTTP, not a replacement for TLS.

## Architecture rule

Do not introduce a second database or a second implementation of WireGuard/WRAP/DTLS.
The panel is an administration surface over the existing WDTT runtime.
