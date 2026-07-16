# `api/` — removed product sources (v3.24+)

This directory no longer contains production Go packages for `nasd`.

- Live HTTP API: `internal/web` + Core module route registrars.
- Shared API helpers used by extensions: `internal/api`.
- Historical container/websocket handlers under this path were **deleted** (breaking change).

See [docs/ARCHITECTURE.md](../docs/ARCHITECTURE.md).
