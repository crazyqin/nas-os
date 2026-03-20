# NAS-OS Changelog

All notable changes to this project will be documented in this file.

## [v2.253.50] - 2026-03-20

### Dependencies
- 更新 RoaringBitmap v2.15.0 → v2.16.0
- 更新 google.golang.org/genproto 相关依赖

### Maintenance
- 六部协同开发检查完成
- 编译: 通过
- 测试: 全部通过 (265 测试文件)
- go vet: 通过
- 模块验证: 通过

## [v2.253.49] - 2026-03-20

### Bug Fixes
- 修复 snapshot 测试中函数名不匹配问题 (NewSnapshotExecutor -> NewExecutor)
- 解决 CI/CD typecheck 错误

### Maintenance
- 六部协同开发检查完成
- 编译: 通过
- 测试: 全部通过 (265 测试文件)
- 代码统计: 412,078 行 Go 代码
- go vet: 通过

## [v2.253.48] - 2026-03-20

### Bug Fixes
- 修复 golangci-lint revive 错误 (50 个问题)
  - internal/smb/handlers.go: 添加 Success/Error 函数注释
  - internal/users/handlers.go: 添加 LoginRequest 等类型注释
  - internal/users/manager.go: 添加 RoleAdmin 等常量和 ErrUserNotFound 等错误注释
  - internal/snapshot/replication.go: 添加 NodeStatus 等常量注释
  - internal/snapshot/retention.go: 重命名 SnapshotInfo 为 Info (解决 stutter)
  - internal/snapshot/executor.go: 重命名 SnapshotExecutor 为 Executor (解决 stutter)
  - internal/system/monitor.go: 重命名 SystemStats 为 Stats (解决 stutter)

### Maintenance
- 六部协同开发检查完成
- 户部: 资源统计 (467 源文件, 412,044 行代码, 265 测试文件, 68 功能模块)
- 礼部: 文档版本同步
- 工部: CI/CD 检查通过
- 刑部: 安全审计报告生成

## [v2.253.47] - 2026-03-20

### Maintenance
- 六部协同例行维护检查
- 更新文档版本一致性 (docs/USER_GUIDE.md, docs/api.yaml)
- 代码质量: go vet 通过，编译成功，测试通过
- 安全审计: 高危150个，中危716个 (无新增)

## [v2.253.46] - 2026-03-20

### Bug Fixes
- 修复 golangci-lint 代码格式问题 (7个文件)
  - internal/backup/restore.go
  - internal/cloudsync/types.go
  - internal/quota/api.go
  - internal/quota/optimizer/optimizer.go
  - internal/quota/types.go
  - internal/service/manager.go
  - internal/version/version.go
- 修复 revive linter 类型别名注释问题
  - internal/backup/manager.go: BackupTask/BackupHistory/BackupStats/BackupType/TaskStatus
  - 添加导出方法注释: ListConfigs/GetConfig/CreateConfig

## [v2.253.44] - 2026-03-20

### Bug Fixes
- 修复 golangci-lint 代码注释规范问题
- 修复 cloudsync/types.go 导出常量注释 (ProviderTencentCOS 等)
- 修复 quota 包导出类型和常量注释
  - types.go: 所有导出常量添加独立注释
  - errors.go: ErrCodeQuotaExists 等错误码添加注释
  - handlers.go: Response, Success, Error 添加注释
  - api.go: 修复类型注释与类型名不匹配问题
  - history.go: 修复 HistoryRecord 注释，添加常量注释
- 修复 quota 包类型命名 stuttering 问题

## [v2.253.43] - 2026-03-20

### Security
- 修复 G115 整数溢出转换漏洞 (兵部)
  - internal/backup/smart_manager_v2_unix.go: 使用 SafeMulUint64 安全计算磁盘空间
  - internal/optimizer/optimizer.go: 使用 SafeUint64ToInt64 安全转换 GC 统计
  - internal/quota/optimizer/optimizer.go: 重构差值计算避免溢出
  - internal/monitor/disk_health.go: 使用 SafeUint64ToInt 安全转换温度值

### Maintenance
- 六部协同例行维护检查
- 代码质量: go vet 通过，编译成功
- 安全审计: 无硬编码敏感信息
- CI/CD配置: Go 1.25.0 一致性确认
- 资源统计: 411,836行代码, 264测试文件, 68模块

### Improvements
- 六部协同开发流程优化
- CI/CD 配置检查完善 (工部)
- 文档版本同步检查 (礼部)
- 安全审计报告生成 (刑部)
- 代码量和测试覆盖率统计 (户部)

## [v2.253.42] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.42 (礼部)
  - VERSION、internal/version/version.go
  - README.md、docs/README.md
  - docs/USER_GUIDE.md、docs/api.yaml

## [v2.253.40] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.40 (礼部)
  - README.md、docs/README_EN.md
  - docs/USER_GUIDE.md、docs/QUICKSTART.md
  - docs/FAQ.md、docs/API_GUIDE.md
  - docs/api.yaml、docs/swagger.yaml