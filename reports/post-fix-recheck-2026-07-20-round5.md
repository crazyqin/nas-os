# Post-fix recheck round 5 — extensions + search out of Core (2026-07-20)

## Changes

| Item | Detail |
|------|--------|
| HTTP extensions | `system_packages.go` behind `nasd_full`; core stub skips mounts |
| `extensions_loader` | No longer imports extension packages; holders only track names |
| `raidz_ui.go` | `nasd_full` (was pulling `internal/api` → search) |
| `api/search.go` | Renamed `search_full.go` with `nasd_full` (drops bleve from Core) |
| Tests | Enable/disable extension tests → `packages_api_full_test.go`; structure/extension loader tests require full |

## Size

| Binary | Size | Notes |
|--------|------|-------|
| Core | **~49 MB** | was ~59 MB after products split; was ~118 MB originally |
| Full (`nasd_full`) | **~118 MB** | products + extensions + search |

Core no longer depends on:
- `internal/extensions/*`
- `internal/search` / bleve
- `internal/docker` / photos / vm / …

## Core internal package list (verified)

```
application arch auth cluster config downloader logging network nfs
packageruntime shares smb storage system users version web api
```

## Still linked in Core (acceptable / next)

- `downloader` + `cluster` types via `application` constructor signatures (often nil at runtime)
- `shares` via application/modules
- Full btrfs tooling via storage

Further slimming: optional-tag application product constructors (cluster/downloader) if desired.
