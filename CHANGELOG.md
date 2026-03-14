# NAS-OS Changelog

All notable changes to this project will be documented in this file.

## [v2.7.0] - 2026-03-14

### Changed
- **Dockerfile 优化**
  - 使用 golang:1.24-alpine 构建镜像
  - 添加 UPX 二进制压缩，镜像大小减少约 15%
  - 添加 nasctl CLI 编译
  - 增强 OCI 标签和元数据
  - 优化健康检查命令

- **监控增强**
  - 整合并优化 Prometheus 告警规则 (alerts.yml)
  - 添加 CPU、内存、磁盘、网络、服务、Btrfs 等多维度告警
  - 增强健康检查模块，添加 Btrfs 和共享服务检查
  - 完善 API 处理器，支持更丰富的监控端点

### Fixed
- 修复 `internal/quota/history.go` 编译错误
  - 修复未使用的 `quotaID` 变量
  - 修复 `TrendStatistics.GrowthRate` 字段名错误（应为 `DailyGrowthRate`）

## [v2.6.0] - 2026-03-14

### Added
- **集成测试完善**
  - 去重功能集成测试 (`tests/integration/dedup_test.go`)
  - 健康检查集成测试 (`tests/integration/health_test.go`)
  - 日志系统集成测试 (`tests/integration/logging_test.go`)
  - 性能基准测试

### Testing
- **去重测试**: 文件扫描、重复检测、并发操作、用户权限隔离
- **健康检查测试**: 内存检查、磁盘空间检查、HTTP 服务检查、超时处理、并发检查
- **日志系统测试**: 日志级别过滤、JSON/文本格式化、字段支持、日志轮转、并发写入、上下文日志

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