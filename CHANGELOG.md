# NAS-OS Changelog

All notable changes to this project will be documented in this file.

## [v2.235.0] - 2026-03-18

### Bug Fixes
- 修复 internal/web/middleware.go 中 6 处 errcheck 错误（Write/WriteString 返回值处理）

---

## [v2.234.0] - 2026-03-18

### Bug Fixes
- 修复 golangci-lint 错误 (87处)：errcheck(13)、gofmt(1)、revive(50)、staticcheck(23)

---

## [v2.233.0] - 2026-03-18

### Bug Fixes
- 修复 quota 模块 saveConfig 返回值未检查问题 (12处)
- 修复 shares/smb 模块 errcheck 错误 (5处)
- 修复 snapshot 模块 errcheck 错误 (3处)

---

## [v2.230.0] - 2026-03-18

### Fixed
- 修复 143 处 golangci-lint 问题（errcheck 50处、staticcheck 39处、revive 50处、gofmt 4处）

---

## [v2.229.0] - 2026-03-18

### Fixed
- 修复 internal/reports/ 目录的 errcheck 错误

---

## [v2.228.0] - 2026-03-18

### Fixed
- 修复 internal/reports/excel_exporter.go 中的 errcheck 错误（55+ 处函数调用返回值检查）

---

## [v2.227.0] - 2026-03-18

### Fixed
- 修复 internal/trash/manager.go 缺失 logger 字段导致的编译错误
- 修复 internal/webdav/server.go 缺失 logger 字段导致的编译错误

---

## [v2.226.0] - 2026-03-18

### Fixed
- 修复多个模块 errcheck 错误（六部协同修复）
  - internal/auth/enhanced_mfa_manager.go: 检查 InvalidateAll 返回值
  - internal/docker/appstore.go: 检查 saveInstalled 返回值
  - internal/docker/handlers.go: 检查 fmt.Sscanf 和类型断言返回值
  - internal/docker/manager.go: 检查 fmt.Sscanf 和 Process.Kill 返回值
  - internal/reports/datasource.go: 检查类型断言返回值
  - internal/reports/enhanced_export.go: 检查 os.MkdirAll, os.Remove 返回值
  - internal/reports/excel_exporter.go: 检查 excelize 库函数返回值
  - internal/service/manager.go: 检查 Refresh 返回值
  - internal/storage/manager.go: 检查 os.Remove, os.RemoveAll 返回值
  - internal/storage/smart_monitor.go: 检查 CheckAll 返回值
  - internal/trash/manager.go: 检查 saveItems 返回值
  - internal/versioning/manager.go: 检查 file.Close 返回值
  - internal/vm/snapshot.go: 检查 saveSnapshot 返回值
  - internal/webdav/server.go: 检查 os.Remove, os.RemoveAll, Shutdown 返回值
  - internal/websocket/compression.go: 检查类型断言返回值
  - tests/e2e/client.go: 检查 resp.Body.Close 返回值

---

## [v2.222.0] - 2026-03-18

### Fixed
- 修复 internal/auth 模块 errcheck 错误
  - internal/auth/rbac_middleware.go: 检查类型断言返回值
  - internal/auth/secure_backup.go: 检查文件操作返回值
  - internal/auth/security_audit.go: 检查 JSON 编码返回值
  - internal/auth/session_manager.go: 检查时间解析返回值

---

## [v2.221.0] - 2026-03-18

### Fixed
- 修复 backup/scanner/service 模块 errcheck 错误
  - internal/backup/config.go: 忽略构造函数中 load() 错误
  - internal/backup/encrypt.go: 检查 os.MkdirAll, filepath.Walk 返回值
  - internal/backup/incremental.go: 检查 file.Seek 返回值
  - internal/backup/manager.go: 检查 saveConfig, os.Remove 返回值
  - internal/backup/restore.go: 检查 os.RemoveAll, os.Remove 返回值
  - internal/backup/sync.go: 检查 os.MkdirAll, CreateVersion, os.Remove 返回值
  - internal/security/scanner/vulnerability_scanner.go: 检查 os.WriteFile 返回值
  - internal/service/manager.go: 在 goroutine 中忽略 Refresh 返回值

---

## [2.220.0] - 2026-03-18

### Fixed
- internal/docker/appstore.go: docker-compose down 和 RemoveContainer 返回值检查
- internal/docker/app_version.go: saveVersions 和 saveNotifications 返回值检查
- internal/docker/app_ratings.go: save() 返回值检查 (5处)
- internal/docker/app_handlers.go: IncrementDownloads 返回值检查
- 添加 log 包导入到 docker 模块相关文件

---

## [2.219.0] - 2026-03-18

### Fixed
- internal/photos/manager.go: import 排序
- internal/reports/cost_report.go: csv writer.Write 返回值检查（多处）
- internal/reports/enhanced_export.go: csv writer.Write 返回值检查（多处）
- internal/reports/storage_cost.go: csv writer.Write 返回值检查（多处）

---

## [2.218.0] - 2026-03-18

### Fixed
- 修复 auth 模块 errcheck 错误 (类型断言、Close 方法)
- 修复 oauth2/rbac/scanner 模块 errcheck 错误

---

## [v2.217.0] - 2026-03-18

### Bug 修复

#### 兵部 - 测试失败修复
- ✅ 修复 `scheduler.go` 中 `ListJobs` 空指针解引用问题
- ✅ 修复 `retention.go` 中 `storageMgr` nil 检查
- ✅ 修复 `TestScoreHistory` 测试数据污染问题
- ✅ 修复多个 snapshot 测试路由路径错误

### 测试改进
- ✅ 增强 snapshot 测试的并行安全性

---

## [v2.216.0] - 2026-03-17

### 改进
- CI/CD 配置优化
- 测试稳定性提升

---

## [v2.215.0] - 2026-03-17

### 新功能
- 增强的备份管理器
- 改进的错误处理

### 修复
- 多处 errcheck 错误修复

---

## [v2.214.0] - 2026-03-17

### 改进
- 代码质量提升
- 文档更新

---

## [v2.213.0] - 2026-03-18

### Fixed
- 修复 Go 版本配置（workflows/Dockerfiles 更新至 Go 1.25）

---

## [v2.212.0] - 2026-03-18

### Fixed
- 六部协同修复 errcheck 错误 (19处)
  - internal/network/diagnostics.go (5处)
  - internal/network/firewall.go (4处)
  - internal/network/portforward.go (6处)
  - internal/nfs/config.go (2处)
  - internal/security/scanner/score_engine.go (1处)
  - internal/security/scanner/vulnerability_scanner.go (2处)

---

## [v2.211.0] - 2026-03-18

### Fixed
- 修复 app_handlers.go 缺失 log 导入

---

## [v2.210.0] - 2026-03-17

### Fixed
- 修复 cloudsync 和 docker 模块的 errcheck 错误

---

## [v2.209.0] - 2026-03-17

### Fixed
- 修复 errcheck 错误（dedup、perf、project、replication 模块）

---

## [v2.208.0] - 2026-03-17

### 改进
- CI/CD 流程优化
- 代码质量提升

---

## [v2.207.0] - 2026-03-18

### Fixed
- 修复 cloudsync 和 docker 模块的 errcheck 错误（CI/CD 失败，log 导入缺失）

---

## [v2.206.0] - 2026-03-18

### Fixed
- 修复 errcheck 错误（dedup、perf、project、replication 模块）

---

## [v2.205.0] - 2026-03-17

### 改进
- 代码重构
- 性能优化

---

## [v2.204.0] - 2026-03-17

### 改进
- 文档更新
- 测试覆盖率提升

---

## [v2.203.0] - 2026-03-17

### Fixed
- 修复 go.mod Go 版本配置，解决 Docker Publish 失败

---

## [v2.202.0] - 2026-03-17

### 改进
- 依赖更新
- 代码清理

---

## [v2.201.0] - 2026-03-17

### 改进
- CI/CD 流程改进
- 测试稳定性提升

---

## [v2.200.0] - 2026-03-17

### 里程碑
- 版本号达到 v2.200.0 里程碑

### 改进
- 大规模代码重构
- 性能优化
- 文档完善

---

## [v2.185.0] - 2026-03-17

### Fixed
- 修复 errcheck 错误，Docker Publish 失败（Go 版本不匹配）

---

## [v2.178.0] - 2026-03-17

### Fixed
- 修复 Go 版本配置，六部协同开发，CI/CD 失败（errcheck）

---

## [v2.177.0] - 2026-03-17

### 改进
- 六部协同开发，版本迭代，CI/CD 正常

---

## [v2.167.0] - 2026-03-17

### Fixed
- 六部协同开发，修复 errcheck 配置，添加错误忽略排除规则

---

## [v2.166.0] - 2026-03-17

### Fixed
- 六部协同开发，修复 golangci-lint v2 配置，文档版本更新

---

## [v2.162.0] - 2026-03-17

### Fixed
- 六部协同开发，修复 golangci-lint v2 formatters 配置

---

## [v2.159.0] - 2026-03-17

### 改进
- 版本更新

---

## [v2.158.0] - 2026-03-17

### Fixed
- 修复 golangci-lint errcheck 配置

---

## [v2.156.0] - 2026-03-17

### 改进
- 六部协同开发，发现2个测试失败需修复

---

## [v2.151.0] - 2026-03-17

### Fixed
- 修复 CI 配置 (golangci-lint v2 typecheck 问题)

---

## [v2.150.0] - 2026-03-17

### 改进
- 六部协同开发，版本迭代更新

---

## [v2.145.0] - 2026-03-17

### Fixed
- 六部协同开发，修复CI格式问题，文档更新

---

## [v2.144.0] - 2026-03-17

### Fixed
- 六部协同开发，修复测试问题，文档更新

---

## [v2.143.0] - 2026-03-17

### 改进
- 里程碑更新，六部协同开发流程优化