# P1 follow-up — 2026-07-24

## WebUI api.js coverage
- `webui/js/api.js` now installs a global `fetch` interceptor: any `/api/v1` request gets `Authorization: Bearer` from localStorage when present.
- Injected `api.js` into 52 HTML pages (login/storage/index already had it). Zero pages with `fetch` remain without the script.

## packages API HTTP honesty
- `handlePackageEnable` / `handlePackageDisable` failures → **HTTP 500** (was 200 + code:1).

## docker-compose.prod.yml
- nas-os: `privileged: false`, bridge `127.0.0.1:8080` (not host network).
- Full hardware: stack with `docker-compose.privileged.yml`.
- Prometheus `--web.enable-admin-api` commented out.
- Env: `NAS_OS_ENV`, `NAS_CSRF_KEY`, WebAuthn RPID/origins.

## WebAuthn config
- `DefaultWebAuthnConfig` from env:
  - `NAS_OS_WEBAUTHN_RPID` (strips scheme/path)
  - `NAS_OS_WEBAUTHN_ORIGINS` (comma-separated)
  - `NAS_OS_WEBAUTHN_NAME`
- Defaults remain localhost for dev.
- Documented in `.env.example`.

## Tests
- `go test ./internal/auth ./internal/web ./internal/application -short` OK
