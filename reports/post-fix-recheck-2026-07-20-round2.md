# Post-fix recheck round 2 — remaining P1 hardening (2026-07-20)

Follow-up to `reports/post-fix-recheck-2026-07-20.md`.

## Fixed this round

| Issue | Fix | Evidence |
|-------|-----|----------|
| CSRF token not HMAC-signed | Token format `ts.nonce.sig` with HMAC-SHA256(CSRFKey); cookie SameSite=Strict; Secure when TLS / X-Forwarded-Proto=https | `internal/web/middleware.go`; CSRF tests |
| Sessions in `users.json` | Tokens no longer serialized; reload drops legacy tokens | `users.saveConfig` / `loadConfig`; `TestManager_SessionsNotPersisted` |
| MFA TOTP plaintext on disk | AES-GCM via `SecretEncryption` + `mfa-master.key`; prefix `enc:v1:` | `internal/auth/manager.go`; `TestMFAManager_TOTPSecretEncryptedAtRest` |
| Wipe-only delete | Soft detach default (`allow_wipe=false`); wipe only with `allow_wipe=true` | `DetachVolume`; delete_confirm tests |
| Compose high privilege | Overlay `docker-compose.secure.yml` (no privileged, bridge, CSRF required) | `docker-compose.secure.yml` |

### Tests

```
go test ./internal/web/ -run 'CSRF|RateLimit|DeleteVolume'   # PASS
go test ./internal/users/ -run 'Persist|Session|Persistence' # PASS
go test ./internal/storage/ -run 'Delete|ValidateDelete|Detach' # PASS
go test ./internal/auth/ -run 'TOTPSecretEncrypted|…'       # PASS
```

## Still open (honest)

1. **Link-time surface / ~118MB binary** — product packages still imported by `server.go`; runtime nil-gating only. Needs build tags or process split.
2. **Base `docker-compose.yml` still privileged+host** — secure overlay is opt-in; operators must use `-f docker-compose.secure.yml`.
3. **CSRF double-submit only** — no origin binding beyond CORS allowlist; fine for same-site admin UI.
4. **MFA key is machine-local file** — not HSM/KMS; backup of `mfa-master.key` required for disaster recovery of TOTP secrets.
5. **Soft detach is not a grace-period scheduler** — no automatic delayed wipe; ops must re-run with `allow_wipe=true` to destroy signatures.
6. **Lab/README marketing** — unchanged.
7. **Real btrfs wipe E2E** — still not exercised without hardware.

## Operator notes

- Soft delete body: `{"confirm_name":"tank"}`  
- Hard wipe body: `{"confirm_name":"tank","allow_wipe":true,"force":true}`  
- Secure deploy: `docker compose -f docker-compose.yml -f docker-compose.secure.yml up -d` with `NAS_CSRF_KEY` set.
