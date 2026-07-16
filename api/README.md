# `api/` — non-shipped / orphan package

**Status**: Not part of the `nasd` production entry path.

- Live HTTP API is assembled in `internal/web` (and Core module route registrars).
- Package helpers used by some extensions live under `internal/api`.
- Sources in this directory (`container_handlers.go`, etc.) are **legacy / experimental**
  and are **not imported** by `cmd/nasd`. Do not document them as the primary API surface.

Primary docs: [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md), [docs/api/](../docs/api/).
