# Optimization pass — 2026-07-23

## Summary

Implemented the actionable optimization backlog from the architecture review (v3.24.3).

## Changes

### Dependencies
- Unified `gopsutil` **v3 → v4** (sysresmon, dashboard/health, lab analytics/diag/prometheus/resourceoptimizer)
- Adapted temperature sensors to `sensors.SensorsTemperatures()` (v4 API)
- `go mod tidy` — only `gopsutil/v4` remains in go.mod

### Cache lifecycle
- `FileListCache.Stop()` / `ThumbnailCache.Stop()` with stop channel + WaitGroup
- `OptimizedManager.Stop()` / `Close()` shut down workers and caches (idempotent)
- Unit tests for Stop

### Auth / sessions
- Login / logout / refresh no longer rewrite `users.json` (sessions already memory-only)
- MFA at-rest encryption was already present (`secret_encryption.go`); verified tests pass

### HTTP access logs
- `loggerMiddleware` uses **zap** structured fields
- Always logs 4xx/5xx and slow (≥500ms) requests
- Successful fast requests sampled (`NAS_ACCESS_LOG_SAMPLE`, default 1/10)

### Web Server structure
- Extracted Full bulk routes → `internal/web/server_routes_bulk_full.go` (`registerBulkOptionalRoutes`)
- `setupRoutes` in `server_full.go` is now thin

### Docker AppStore split
- `appstore.go` (core install/lifecycle of apps)
- `appstore_templates.go` (builtin templates)
- `appstore_lifecycle.go` (versions, updates, backup, health)

### Storage handlers split
- `handlers.go` — volume + maintain + RegisterRoutes
- `handlers_subvolume.go`, `handlers_snapshot.go`, `handlers_device.go`, `handlers_raid.go`
- `handlers_hotspare.go`, `handlers_fusion.go`, `handlers_space.go`, `handlers_smartraid.go`

### Search / notify governance
- `internal/search/doc.go` — canonical Full search
- Deprecation notes on unifiedsearch, spotlight*, notify vs notification

### Docker image default
- `Dockerfile` `ARG BUILD_TAGS=` (Core-only, matches `make build`)
- Full: `--build-arg BUILD_TAGS=nasd_full`
- `docs/STRUCTURE.md` updated

## Deferred / non-goals this pass
- Externalizing entire `internal/lab` tree (32MB) to another repo
- Full Server field map refactor (100+ product fields → interface table)
- Soft-delete volume grace period
- Dual UI merge (`webui` vs `web/src`)
- True gin tree package unload

## Verification
- `go build ./cmd/nasd` (Core)
- `go build -tags nasd_full ./cmd/nasd` (Full)
- `go test` files, users, web, storage (subset), auth, docker, sysresmon, dashboard/health
- No lab deps on Core/Full production binary path (existing governance)
