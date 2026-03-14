# NAS-OS Changelog

All notable changes to this project will be documented in this file.

## [v2.12.0] - 2026-03-14

### Fixed
- **CI/CD 修复**
  - 统一所有工作流 Go 版本到 1.24（稳定版）
  - 修复 golangci-lint 版本兼容性问题
  - 修复 Docker 构建基础镜像版本

### Changed
- 更新 `.github/workflows/ci-cd.yml` GO_VERSION
- 更新 `.github/workflows/docker-publish.yml` GO_VERSION
- 更新 `.github/workflows/release.yml` GO_VERSION
- 更新 `Dockerfile` 基础镜像到 `golang:1.24-alpine`
- 更新 `go.mod` Go 版本到 1.24

## [v2.11.2] - 2026-03-14

### Fixed
- 修复 CI/CD Go 版本配置错误（Go 1.26.1/1.25 → 1.24）
- 解决代码合并冲突
- 改进错误处理和日志记录

### Changed
- 统一使用 zap logger 进行日志记录
- 测试代码使用 t.Cleanup 替代 defer

## [v2.11.1] - 2026-03-14

### Fixed
- **依赖修复**
  - 修复 gin 依赖版本约束，解决编译兼容性问题

- **CI/CD 修复**
  - 修复 GitHub Actions 工作流配置
  - 重新发布 v2.11.1 版本

## [v2.11.0] - 2026-03-14

### Fixed
- **SMB 并发测试修复**
  - 修复 `TestConcurrentCreateShare` 竞态条件
  - 优化 manager.go 中的并发控制逻辑
  - 竞态检测器通过

### Changed
- **共享服务完善**
  - SMB 服务边界条件处理优化
  - NFS 服务错误处理增强

- **CI/CD 优化**
  - GitHub Actions 配置检查
  - 测试超时设置优化
  - 测试结果缓存

- **文档更新**
  - README.md 版本号同步
  - CHANGELOG.md 更新
  - SMB/NFS 配置示例补充

### Added
- **测试覆盖率**
  - 覆盖率分析报告
  - 低覆盖率模块识别

- **安全审计**
  - SMB/NFS 安全配置审查
  - 权限边界检查
  - golangci-lint 静态分析通过

## [v2.10.0] - 2026-03-14

### Added
- **SMB/CIFS 共享服务** (`internal/smb/`)
  - SMB 服务管理器，支持共享 CRUD 操作
  - 配置文件解析与生成 (`config.go`)
  - 用户权限映射和访客访问控制
  - 连接状态监控和管理

- **NFS 共享服务** (`internal/nfs/`)
  - NFS 服务管理器，支持导出 CRUD 操作
  - 客户端配置和权限选项管理
  - NFS 导出配置解析

- **系统服务集成** (`internal/service/`)
  - systemd 服务管理后端
  - 服务状态监控（运行状态、PID、内存、CPU）
  - 服务操作（启动/停止/重启/启用/禁用）
  - 开机自启控制

- **WebUI 共享管理页面**
  - `webui/pages/smb.html` - SMB 共享管理页面
  - `webui/pages/nfs.html` - NFS 共享管理页面
  - `webui/pages/shares.html` - 共享概览页面优化

- **测试完善**
  - SMB 服务单元测试
  - NFS 服务单元测试
  - 共享集成测试 (`tests/integration/shares_test.go`)

### Changed
- 更新 `cmd/nasd/main.go` 集成服务管理
- 优化共享处理器

## [v2.9.0] - 2026-03-14

### Added
- **API 响应格式统一**
  - 新增 `internal/api/response.go` 统一响应格式
  - 新增 `internal/api/validator.go` 请求验证器
  - 统一错误处理和响应格式

- **Handlers 重构**
  - `internal/docker/handlers.go` 使用统一响应格式
  - `internal/network/handlers.go` 使用统一响应格式
  - `internal/quota/handlers_v2.go` 响应格式优化

- **测试覆盖完善**
  - `internal/container/container_test.go` 容器测试
  - `internal/monitor/manager_test.go` 监控测试
  - `internal/users/manager_test.go` 用户管理测试

- **文档更新**
  - `docs/RELEASE-v2.9.0.md` WebUI 完成报告

### Changed
- API 端点使用统一的响应格式
- 错误处理标准化

## [v2.8.1] - 2026-03-14

### Bug Fixes

- **cmd/backup**: 修复 fs.Parse 和 file.Close 返回值未检查的问题
- **cmd/nasctl**: 修复 fmt.Fprintln/Fprintf 和 w.Flush 返回值未检查的问题
- **cmd/nasd**: 修复 logger.Sync、cluster.ShutdownCluster 和 webServer.Stop 返回值未检查的问题
- **internal/ai_classify**: 修复 os.RemoveAll、os.WriteFile 和 file.Close 返回值未检查的问题
- **CI/CD**: 修复 32 处 errcheck linter 错误

## [v2.8.0] - 2026-03-14

### Fixed
- 修复 golangci-lint 代码风格问题
  - 修复未使用的变量和导入
  - 修复错误处理规范
  - 修复代码格式问题

## [v2.7.0] - 2026-03-14

### Added
- **测试覆盖完善**
  - API 端点集成测试 (`tests/integration/api_endpoints_test.go`)
    - 系统端点测试 (health, system/info)
    - 存储端点测试 (volumes, subvolumes, snapshots)
    - 用户端点测试 (CRUD 操作)
    - 认证端点测试 (login, logout, refresh)
    - 配额、共享、备份、去重、搜索、监控等端点测试
    - 分层存储、压缩存储、WebDAV/FTP/iSCSI 端点测试
    - 容器和插件端点测试
    - 并发请求和错误处理测试

  - WebUI 端到端测试 (`tests/e2e/webui_test.go`)
    - 仪表板、存储管理、用户管理、权限管理测试
    - 系统设置、日志查看、服务管理测试
    - 分层存储可视化、压缩管理、性能监控测试
    - 告警管理、完整工作流、响应时间测试

  - 性能基准测试 (`tests/benchmark/performance_test.go`)
    - 版本信息、存储操作、缓存操作基准测试
    - JSON 序列化、HTTP API、并发基准测试
    - 内存分配基准测试

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
- 修复 CI/CD 构建问题
  - 更新 golangci-lint 到 v2 兼容配置格式
  - 更新 Dockerfile golang 镜像到 1.26 以匹配 go.mod 要求
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