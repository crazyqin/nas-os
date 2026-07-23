# Experimental frontend (`web/`)

> **Primary product UI is [`webui/`](../webui/).**  
> Sources under `web/` and `web/src` are **experimental / non-delivery** and are **not** the default management surface for `nasd`.

## Status

| Path | Role | Shipped by default? |
|------|------|---------------------|
| `webui/` | Static HTML/JS management UI (`/webui`) | **Yes** |
| `web/src` | React experiments (components/views) | **No** |
| `web/acl` | Legacy/static ACL sketch | **No** |

## Rules for contributors

1. **Do not** add production API assumptions only in `web/src` without the matching `webui/` page.
2. New user-facing features land in **`webui/pages/`** first.
3. Docker / `make build` only embed or serve **`webui/`** (see root `Dockerfile` and `registerWebUI`).
4. This tree may be removed or rebased without a major-version bump until promoted.

## Optional local React work

```bash
# If package.json exists under web/ — otherwise this tree is source-only sketches.
cd web && npm install && npm run dev   # experimental only
```

See [docs/STRUCTURE.md](../docs/STRUCTURE.md) § main UI vs experiment frontend.
