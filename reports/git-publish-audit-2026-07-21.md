# Git publish audit — 2026-07-21

## nas-os (`/home/mrafter/nas-os`)

| Item | Value |
|------|--------|
| Branch | `master` tracking `origin/master` |
| HEAD at audit start | `a755f1b6` (same as origin — **no unpushed commits**) |
| Working tree | **Dirty** — large uncommitted set (security/arch work: Core/Full build, honesty gates, storage delete, docs, reports) |
| Ahead / behind origin | `0 / 0` (committed history synced; **local changes not committed**) |
| Remote | `https://github.com/crazyqin/nas-os.git` |

**Conclusion at audit:** architecture and security work from this session existed only as **local uncommitted changes**, not yet pushed.

## nas-os-website canonical (`/home/mrafter/projects/nas-os-website`)

| Item | Value |
|------|--------|
| Branch | `main` → `origin/main` |
| HEAD | `eb92ef5`, clean working tree at audit |
| Ahead / behind | `0 / 0` |
| Remote | `https://github.com/crazyqin/nas-os-website.git` |

## Secondary tree

`/home/mrafter/clawd/nas-os-website` is a separate clone (different HEAD); **canonical edits go to `projects/nas-os-website`**.
