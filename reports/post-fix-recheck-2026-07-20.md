# Post-fix recheck — nas-os P0/P1 hot-path (2026-07-20)

**Scope**: Fixes for identity persistence, volume wipe confirmation, rate-limit concurrency, session TTL, default product construction gates, compose/env CSRF hooks.  
**Version**: v3.24.3 codebase after this change set.

## What was fixed (verified)

| Category | Fix | Evidence |
|----------|-----|----------|
| **P0 Identity not durable** | Production `application.New` uses `users.NewManagerWithConfig(..., cfg.ConfigPath("users.json"))` | `internal/application/application.go` |
| **P0 Password hash lost** | Disk DTO `diskUser` with `password_hash`; API `User` keeps `json:"-"` | `internal/users/manager.go`; `TestManager_PersistReloadAuthenticate` |
| **P0 Bootstrap password path** | Written under config store dir (or mountBase), never process CWD | `bootstrapPasswordPath()` + test |
| **P0 Wipe without confirm** | `DeleteVolumeConfirmed` + `ValidateDeleteConfirmation` (confirm_name + allow_wipe) | `internal/storage/delete_confirm.go`; web/storage handlers |
| **P1 Rate-limit race** | `sync.Mutex` + periodic prune on client map | `internal/web/middleware.go` |
| **P1 Session TTL ignored** | `SetSessionTTL` / Authenticate & Refresh use configured TTL | `users.Manager` + application wiring |
| **P1 Default non-Core constructors** | monitor/trash/repl/webdav/ftp/sftp/tunnel/frp/optimizer nil when packages default-off | `internal/web/server.go` |
| **P1 Compose CSRF hooks** | `NAS_CSRF_KEY` / `NAS_OS_ENV` env slots; version label → 3.24.3 | `docker-compose.yml`, `.env.example` |

### Test runs (this environment)

- `go test ./internal/users/` — **PASS** (includes Create → reload → Authenticate)
- `go test ./internal/storage/ -run 'Delete\|ValidateDelete'` — **PASS**
- `go test ./internal/web/ -run 'RateLimit\|DeleteVolume_Requires\|…'` — **PASS**
- `go test ./internal/application/ ./internal/web/ -run 'Optional\|Product\|…'` — **PASS**
- `go build ./cmd/nasd` — **PASS** (~118MB binary)
- `go list -deps ./cmd/nasd \| grep lab` — **empty** (NO_LAB_DEPS)
- `go test -race` — **cannot run on this host** (ThreadSanitizer: unsupported VMA range 39 vs 48). Concurrent rate-limit test **PASS** without race detector; mutex is present in shipped code.

## Remaining serious issues (honest)

### Still P1-ish / product risk

1. **Compile-time import surface still huge**  
   `internal/web/server.go` still imports docker/photos/vm/… at link time even when managers are nil. Default *runtime* construction is gated for product IDs + several former always-on managers, but **binary size (~118MB) and dependency graph width remain**. Full Core-only link would need build tags or package split (out of this fix scope).

2. **`privileged: true` + `network_mode: host` remain in default compose**  
   CSRF env vars were added; privileged/host network were **not** flipped (would break disk access for many users). Operators still need a non-privileged devices-based production path.

3. **CSRF cookie `Secure=false`; token is not HMAC of `CSRFKey`**  
   Production can still fail-closed without key (`NAS_OS_ENV=production`), but token strength remains weak vs a signed CSRF design.

4. **Session tokens still opaque server-side map; persisted in `users.json` with identities**  
   Acceptable for single-node; multi-instance / token theft from config file backup remains a risk. JWT redesign was non-goal.

5. **MFA TOTP secrets still not encrypted at rest**  
   Documented non-goal; still a residual if MFA is enabled.

6. **Destructive wipe still real once confirmed**  
   Gate is correct; mis-typed confirm of the real volume name + `allow_wipe` still wipes. No soft-delete / grace period.

7. **Storage integration tests still Skip without real btrfs**  
   Confirmation is unit-tested; end-to-end wipe on hardware is not exercised here.

8. **README / Lab marketing surface**  
   Large Lab tree (~600 packages) and historical README “✅ 新增” theater remain; not default runtime, but still confuse selection.

9. **Domain storage handler path** (`internal/storage/handlers.go`) now requires confirm body; any external clients that only sent `?force=true` will break (intentional breaking safety).

### No longer open (prior P0s)

| Prior P0 | Status |
|----------|--------|
| Users not persisted on production path | **Fixed** |
| PasswordHash stripped by `json:"-"` on disk | **Fixed** |
| Force wipe with only query param | **Fixed** (confirm_name + allow_wipe required) |

## Recommendation for next iteration

1. Build-tag or split `web` product imports to shrink default binary.  
2. Non-privileged compose profile with explicit `/dev/disk` mounts.  
3. HMAC CSRF + Secure cookies when TLS is on.  
4. Separate session store from `users.json`.  
5. Soft-delete / delayed wipe for volumes.

---

*Generated as part of the P0/P1 fix goal; re-run the verification commands above after further changes.*
