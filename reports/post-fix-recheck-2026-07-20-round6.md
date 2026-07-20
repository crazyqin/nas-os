# Post-fix recheck round 6 — cluster/downloader out of Core (2026-07-20)

## Changes

| Item | Detail |
|------|--------|
| `application` | `products_full.go` / `products_stub.go` — cluster & downloader only with `nasd_full` |
| `web.Server` | `clusterServices` / `downloadMgr` / bootstrap typed as `any` |
| `product_runtime` | no longer imports `cluster` |
| `web` NewServer | `downloadMgr any` in both builds |

## Size

| Binary | Size | vs original ~118MB |
|--------|------|---------------------|
| **Core** | **~47 MB** | **≈ −60%** |
| Full | ~118 MB | product surface |

## Core internal packages (final)

```
api application arch auth config logging network nfs packageruntime
shares smb storage system users version web
```

**Not linked:** cluster, downloader, docker, photos, vm, extensions/*, search/bleve.

## Notes

- `shares` remains (sharing module handlers for SMB/NFS — Core).
- Enabling cluster/downloader on a Core binary requires rebuild with `-tags nasd_full`.
