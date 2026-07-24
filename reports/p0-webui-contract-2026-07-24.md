# P0 WebUI / health / docs contract fix — 2026-07-24

## Changes

### CSRF (Bearer SPA policy)
- `csrfMiddleware`: skip when `Authorization: Bearer …` present
- Exempt `/api/v1/auth/login|refresh|mfa/verify|mfa/challenge`
- Cookie double-submit retained for non-Bearer mutating clients
- Tests: Bearer skip, login path exempt

### WebUI shared helper
- `webui/js/api.js` — `apiFetch` / `apiLogin` / token helpers
- Wired: `login.html`, `storage.html`, `index.html`
- Storage delete sends `{ confirm_name, allow_wipe, force }` with typed name confirm
- Soft-delete copy (24h grace) replaces “不可恢复” false claim
- storage.html: auth-aware fetch wrapper for remaining raw calls

### Health probe honesty
- `getHealth` returns **HTTP 503** when Core modules unhealthy (Docker probe fails closed)
- Tests updated + healthy 200 case

### Purge / audit / lifecycle
- Purge pending requires `confirm_name` body
- Audit paths add `/api/v1/storage`, `/api/v1/packages`
- `storageModule.Stop` → `StopSoftDeleteReaper`
- system/info exposes `mfa_available`
- MFA init failure log clarified

### Docs / version
- README: Docker image default is **Core** (`BUILD_TAGS=` empty); full via build-arg
- README docker run binds `127.0.0.1:8080`
- login version badge + swagger `@version` → 3.24.3

## Verification
- `go test ./internal/web ./internal/application ./internal/storage -short` OK
