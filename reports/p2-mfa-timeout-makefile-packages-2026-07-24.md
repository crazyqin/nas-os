# P2 residual — 2026-07-24

## MFA fail-closed
- `auth.MFAConfigRequiresManager(path)` peeks mfa-config for any enabled MFA.
- `application.New`: if MFA init fails and (requires manager OR `NAS_OS_REQUIRE_MFA=1`) → boot error.
- Fresh installs without MFA config still fail-open with loud warning.

## HTTP timeouts
- Core/Full `WriteTimeout: 0` (unlimited) for large file transfers.
- `ReadHeaderTimeout: 10s` retained (slowloris).
- `IdleTimeout: 120s` retained.

## Rate limit
- Health paths exempt: `/api/v1/system/health`, `/api/v1/health`, `/healthz`, `/health`.

## Makefile
- `help` documents Core vs Full, `build-full`, `build-version-full`, `test-lab`.
- `docker-build-full` with `--build-arg BUILD_TAGS=nasd_full`.

## modules.* dual-read migration
- `NAS_OS_STRICT_PACKAGES=1`: ResolvePackages ignores modules.* (warn if set).
- `NAS_OS_REJECT_MODULES=1`: Validate() errors if modules.* present.
- Deprecation warning text points operators to STRICT mode.

## Tests
- auth MFA requires, config strict packages, web/application short suite.
