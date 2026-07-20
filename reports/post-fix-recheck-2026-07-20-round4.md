# Post-fix recheck round 4 — Core vs Full build tags (2026-07-20)

## Goal

Shrink default `nasd` link surface so Core delivery does not compile-in docker/vm/photos/… managers.

## What shipped

| Item | Detail |
|------|--------|
| Build tag `nasd_full` | Full product surface in `server_full.go` + `product_packages.go` |
| Core default (`!nasd_full`) | `server_core.go` + `product_packages_stub.go` — no product package imports |
| Shared runtime | `product_runtime.go` — package catalog gates, mount/unmount (always built) |
| Make | `make build` / `build-core` = Core; `make build-full` = `-tags nasd_full` |
| Docker | `ARG BUILD_TAGS=nasd_full` (images keep products); pass empty for slim image |
| Tests | Core tests without tag; product isolation tests require `nasd_full` |

### Size evidence (this environment)

| Binary | Size | Command |
|--------|------|---------|
| **nasd-core** | **~59 MB** | `go build -o nasd-core ./cmd/nasd` |
| **nasd-full** | **~118 MB** | `go build -tags nasd_full -o nasd-full ./cmd/nasd` |

Approx **50% smaller** Core binary when product managers are not linked.

### Tests

```
go test ./internal/web ./internal/users ./internal/application ./internal/storage   # Core PASS
go test -tags nasd_full ./internal/web -run 'Optional|Product|…'                    # Full PASS
```

## Operator notes

```bash
# Slim Core (default make)
make build
# or
go build -o nasd ./cmd/nasd

# Full products (docker/vm/photos/ai/backup/… when packages enable them)
make build-full
# or
go build -tags nasd_full -o nasd ./cmd/nasd

# Docker slim image
docker build --build-arg BUILD_TAGS= -t nas-os:core .
# Docker full (default)
docker build -t nas-os:full .
```

Core binary: enabling products via App Center logs that managers are not linked — rebuild with `nasd_full`.

## Still open

1. Extensions (`internal/extensions/*`) still link into Core via `extensions_loader` / `system_packages` — further slim possible.
2. Lab tree still in repo (~600 packages) but not imported by either build.
3. README long historical feature lists remain marketing-heavy (honesty callout present).
4. Hardware btrfs E2E still unit-gate only.
