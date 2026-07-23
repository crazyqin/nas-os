# Lab isolation + Server holder bag — 2026-07-23 (updated)

## Lab (in-repo, nested module)

Lab sources remain under **`internal/lab`** inside nas-os (not a separate repo).

Isolation is via nested `go.mod` (`module nas-os/internal/lab`):

- Root `go test ./...` / `go list ./...` skip lab
- `make test-lab` runs lab opt-in
- Production nasd never imports lab

## Server field collapse

Full `Server` uses `h *holderBag` instead of 90+ typed optional manager fields.
See holders.go.

## Verification

- Core / Full builds OK
- web, application, storage tests OK
