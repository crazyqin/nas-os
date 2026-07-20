# Git publish audit — 2026-07-21

## nas-os (`/home/mrafter/nas-os`)

| Item | Value |
|------|--------|
| Branch | `master` tracking `origin/master` |
| HEAD after publish | `584ec2e6` feat(security,arch)+docs: Core/Full honesty, wipe gate, publish audit |
| Working tree | **Clean** (post-commit) |
| Ahead / behind origin | `0 / 0` — **pushed** |
| Remote | `https://github.com/crazyqin/nas-os.git` |

**Published:** Core/Full server split, package honesty, wipe confirm gate, MFA encrypt path, docs (README/STRUCTURE/ARCHITECTURE 2026-07-21), compose secure/privileged profiles, architecture recheck reports.

## nas-os-website (`/home/mrafter/projects/nas-os-website`)

| Item | Value |
|------|--------|
| Branch | `main` → `origin/main` |
| HEAD after publish | `e9497fa` polish(site): 中文大众向文案与视觉精修 |
| Working tree | **Clean** |
| Ahead / behind | `0 / 0` — **pushed** |
| Remote | `https://github.com/crazyqin/nas-os-website.git` |

**Published:** Mass-market Chinese copy aligned in HTML+JS; CSS refinement (typography, cards, FAQ, CTA, focus rings); plain-script i18n verified (83/83 keys).

## Secondary tree

`/home/mrafter/clawd/nas-os-website` is a separate clone; canonical site is `projects/nas-os-website`.

## Verification notes

- Website JS: `OK_plain_script` (no require/import/export); zh/en key parity 83/83; HTML i18n keys complete.
- Playwright browser screenshot: unavailable in this environment (no playwright module / browser install).
- Push logs: `implementer/website-push.log`, `implementer/nas-os-push.log`, final `implementer/git-push.log`.
