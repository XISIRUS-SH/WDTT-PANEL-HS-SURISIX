# WDTT Panel — production hardening pass

This pass is built on the real WDTT-Plus-15 source tree.

## Hardened

- Web Panel remains inside the existing `wdtt-server` process.
- Existing `dbMutex` + `executeAdminCommand(..., wgDev, true)` remain the source of truth.
- Session cookies are HttpOnly; CSRF is separate and required for mutations/logout.
- Login attempts are rate limited and reset after successful authentication.
- Mutating panel requests are rate limited per session.
- JSON bodies are size-limited and trailing JSON is rejected.
- Responses are `no-store` and include security headers plus a request ID.
- Expired sessions are periodically purged.
- Destructive UI actions require explicit confirmation.
- Existing outbound/proxy/WARP/WireGuard scripts are reused instead of duplicated.
- Release CI builds Linux amd64/arm64 and Windows amd64 and emits SHA256SUMS.
- Install/update uses the existing `wdtt.service` and refuses to create a second WDTT service.

## Verification in this environment

Passed:

- `gofmt -w *.go`
- JavaScript syntax check with Node.js
- `bash -n installer/*.sh deploy/*.sh`

Blocked:

- `go test ./...`
- `go vet ./...`
- `go build ./...`

The repository declares Go 1.25.0 while this execution environment provides Go 1.23.2.
Automatic toolchain download is unavailable in the environment, so these commands were
not falsely marked as passing.

## Production gate

Run the following on a Go 1.25 environment before tagging a release:

```bash
gofmt -w *.go
go test ./...
go vet ./...
go build ./...
```

Then test on a staging VPS with a copy of `passwords.json` and verify rollback before
using the installer on a production server.
