# NAS-OS Changelog

All notable changes to this project will be documented in this file.

## [v2.5.0] - 2026-03-14

### Added
- **工部基础设施增强**
  - 系统健康检查模块 (`internal/health`)
  - 日志聚合模块 (`internal/logging`)
  - 结构化日志支持
  - 日志轮转和搜索 API

- **安全增强**
  - 安全文件管理器 v2 (`internal/security/v2`)
  - 路径遍历防护
  - 符号链接安全检查
  - 文件扩展名白名单

### Fixed
- 修复 `cmd/backup/main.go` 编译错误
- 修复 `SafeFileManager.SafePath` 绝对路径检测
- 修复 `tests/integration/dedup_test.go` 方法调用

### Changed
- 重构 backup 命令行工具以匹配实际 API
- 改进增量备份管理器接口

## [v2.4.0] - 2026-03-14

### Added
- 智能搜索增强 - 语义搜索、排序优化、搜索建议
- 压缩存储增强 - 算法自动选择、批量压缩
- 存储分层增强 - SSD 缓存优化、自动数据迁移
- 性能监控增强 - 报告导出、告警规则
- 在线文档编辑 - 协作编辑、版本历史
- WebUI 增强 - 分层可视化、压缩管理界面

## [v2.3.0] - 2026-03-14

### Added
- 存储分层模块 (tiering)
- FTP/SFTP 服务器增强
- 压缩存储模块 (compress)

## [v2.2.0] - 2026-03-14

### Added
- iSCSI 目标支持 - Target 配置、LUN 管理、CHAP 认证
- 快照策略管理 - 定时快照、保留策略、自动清理
- WebUI 仪表板 - 实时监控、图表、快速操作
- 性能监控集成 - Prometheus 导出、健康检查、告警规则

## [v2.1.0] - 2026-03-14

### Added
- WebDAV 服务器模块
- 配额管理增强
- 性能优化

## [v2.0.0] - 2026-03-14

### Added
- 存储复制模块
- 回收站模块
- WebUI 界面

---

For older versions, see git history.