# Post-fix recheck round 3 — default surface honesty (2026-07-20)

## Fixed this round

| Issue | Fix |
|-------|-----|
| Single product pulled bulk companions | trash/tunnel/ftp/webdav/monitor/optimizer/frp only when `modules.optional` (bulk), **not** when merely `packages.enabled` or recommended_system |
| Always-on nil-unsafe routes | ups/wol/acl/webhook/recycle/notifychannel/notify register only if manager non-nil |
| NVMe/RAIDZ always registered | Only when bulk or `OptionalProductsEnabled()` |
| Compose default privileged+host | Base compose: `privileged: false`, bridge, `127.0.0.1:8080`, `/dev/disk`; legacy via `docker-compose.privileged.yml` |
| README overstated surface | Honesty callout + bulk companions row |

### Tests

```
go test ./internal/web/ -run 'Optional|Product|DockerOnly|DefaultConfig|…'  # PASS
go test ./internal/web/ ./internal/application/                             # PASS
go build ./cmd/nasd                                                        # PASS
```

New/extended: `TestDockerOnlyDoesNotPullBulkCompanions`, stricter Core nil checks, recommended_system asserts photos on + bulk off.

## Still open

1. **Link-time binary size (~118MB)** — product packages still imported by `server.go`; runtime nil only. True slim needs build tags / split packages (next major).
2. **Base compose still optional CSRF** — use `docker-compose.secure.yml` for fail-closed CSRF.
3. **Lab tree / historical README feature theater** — Lab not loaded; marketing history remains in long changelog sections.
4. **Hardware btrfs wipe E2E** — confirmation + soft detach unit-tested only.

## Operator cheat sheet

| Goal | Command / config |
|------|------------------|
| Core only | defaults |
| 8 catalog products | `packages.recommended_system: true` |
| One product | `packages.enabled: [docker]` |
| Kitchen sink | `modules.optional: true` (deprecated) |
| Safe docker | `docker compose up -d` |
| Privileged host net | `… -f docker-compose.privileged.yml` |
| Force CSRF | `… -f docker-compose.secure.yml` + `NAS_CSRF_KEY` |
