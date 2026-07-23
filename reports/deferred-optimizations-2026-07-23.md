# Deferred optimizations implementation — 2026-07-23

## 1. Lab externalization (nested module)

- Added `internal/lab/go.mod` (`module nas-os/internal/lab`) + `go.sum` + `README.md`
- Root `go list/test ./...` **no longer includes** lab (verified: directory prefix not in main module)
- `make test-lab` for opt-in greenhouse tests
- Parent imports via `replace nas-os => ../..`

## 2. Product registry (Server field lifecycle SSOT)

- `internal/web/product_registry.go` — `productRegistry` put/get/drop/remove/clearAll
- Full Server holds `productReg`; `trackProduct` / `seedProductRegistry` on boot
- `releaseProductManager` drops registry + nils managers + clears `productRoutesRegistered` (re-enable rebinds)
- Typed fields remain for handlers; registry is lifecycle SSOT

## 3. Volume soft-delete grace period

- `internal/storage/soft_delete.go` — 24h default grace (`DefaultDeleteGracePeriod`)
- `DeleteVolumeConfirmed` → soft pending (restorable) unless `skip_grace` / negative grace
- API (web StorageHandlers):
  - `GET /api/v1/storage/volumes-pending`
  - `POST /api/v1/storage/volumes-pending/:name/restore`
  - `DELETE /api/v1/storage/volumes-pending/:name` (purge)
- Background reaper every 1m; `StopSoftDeleteReaper` for tests/shutdown

## 4. Dual UI convergence

- `web/README.md` — experimental, non-delivery
- `docs/STRUCTURE.md` — web/ and web/src marked non-primary
- Primary remains `webui/`

## 5. Package unload (closer to true unload)

- Documented in `docs/ops-packages.md` §7
- Disable path: unmount (404) → Runtime.Disable → releaseProductManager (memory)
- Re-enable clears route registration set so handlers rebind to new managers

## Verification

- Core ~47MB, Full ~119MB builds OK
- `go test` storage, web, application OK
- Soft-delete unit tests OK
- Product registry unit tests OK
