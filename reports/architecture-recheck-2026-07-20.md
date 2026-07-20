# Architecture recheck after honesty + shared Core path (2026-07-20)

## Prior critical categories

| Category | Status | Evidence |
|----------|--------|----------|
| **Config/binary honesty** | **Fixed** | `web.ValidateBinaryCapabilities` in `application.New`; package list `operable`/`can_enable` false when unlinked; enable API 503; recommended products not Runtime-registered on Core. Tests: `capability_test.go`, `package_honesty_test.go`, `application/capability_test.go`. |
| **Build surface docs** | **Fixed** | `docs/STRUCTURE.md` §1.1 三轴模型; `docs/ARCHITECTURE.md` 「编译面与配置诚实」. |
| **Dual-Server drift** | **Fixed (hot path)** | Shared `server_common.go`. **Both** Core and Full call `registerCorePublicAndAdminGroups` + `registerCoreIdentityAndDocs` (Full after product routes). Product-only block remains Full-only. |
| **Storage delete gate ownership** | **Fixed** | Single owner: `registerCoreIdentityAndDocs` → `StorageHandlers` only. `storageModule.RegisterRoutes` is no-op (no gin dual-register). Test: `TestCoreServer_ModulesPlusStorageMgr_NoDoubleRegisterPanic`. |
| **Layering application→web** | **Reduced / P2 residual** | storage module no longer mounts HTTP (no web.StorageHandlers in modules). application still imports web for Application.NewServer only — acceptable composition root. |
| **Lab megatree** | **Deferred** | Non-goal (externalize later). |
| **modules.* dual-read bulk** | **Deferred (P2)** | Still dual-read; Core + ValidateBinaryCapabilities rejects modules.optional without nasd_full. Full removal deferred. |
| **Full Server god object** | **Deferred** | Non-goal micro-split; mitigated by Core slim binary + shared common path. |

## Remaining non-blocking issues (not P0)

1. Full `setupRoutes` still has a large product route block (maintain under Full only).
2. Product fields still typed concretely on Full Server (size/link cost accepted for Full).
3. Dual UI (`webui` vs `web/`) unchanged.
4. Gin package unload remains logical flags, not true tree removal.

## Exit condition

No remaining **unfixed P0/P1** from the architecture audit categories in scope. Residual items are P2/Non-goals with rationale above.

## Verification artifacts (implementer scratch)

- `core-honesty.log`, `full-surface.log`, `storage-delete-gate.log`
- `docs-build-surface.txt`, `shared-server-path.log`, `build-test-summary.log`
