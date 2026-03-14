# NAS-OS Changelog

All notable changes to this project will be documented in this file.

## [v2.39.0] - 2026-03-15

### Added
- **项目治理完善** (礼部)
  - 版本号统一更新至 v2.39.0
  - CHANGELOG.md 格式规范化
  - MILESTONES.md 里程碑进度更新

- **文档体系完善**
  - docs/ 目录文档结构优化
  - 版本发布说明完善
  - 贡献指南更新

### Changed
- 版本号一致性检查和更新
- 文档索引优化

## [v2.33.0] - 2026-03-15

### Added
- **API 文档增强** (礼部)
  - users/handlers.go: 用户管理 API Swagger 注释（登录、用户 CRUD、权限管理）
  - monitor/handlers.go: 监控 API Swagger 注释（系统统计、磁盘、网络、SMART、告警）
  - system/handlers.go: 系统信息 API Swagger 注释（系统信息、进程、历史数据）
  - shares/handlers.go: 共享管理 API Swagger 注释（SMB/NFS 共享管理）
  - network/handlers.go: 网络 API Swagger 注释（接口、DDNS、端口转发、防火墙）
  - docker/handlers.go: Docker API Swagger 注释（容器、镜像、网络、卷）

- **国际化支持增强**
  - README 功能列表更新
  - 版本号同步至 v2.33.0

### Changed
- 文档版本号统一更新
- Swagger 注释格式规范化

## [v2.32.0] - 2026-03-15

### Added
- **稳定性提升** (吏部)
  - 核心模块测试覆盖率提升
  - 集成测试增强
  - 测试工具完善

- **性能优化**
  - 缓存命中率优化
  - 并发处理优化
  - 资源使用效率提升

- **文档完善**
  - API 文档 Swagger 注释补充
  - 版本号更新到 v2.32.0
  - Docker 镜像版本号更新

### Changed
- 代码质量改进
- 安全检查增强

### Security
- 权限检查增强
- 安全审计更新

## [v2.30.0] - 2026-03-15

### Added
- **安全增强系统** (刑部)
  - API限流中间件（Token Bucket / Sliding Window 算法）
  - 增强版MFA管理器（TOTP加密存储、会话管理集成）
  - 密码策略验证器（强度评分、历史记录、弱密码检测）
  - 登录尝试跟踪器（用户/IP锁定、自动解锁）
  - 会话管理器（令牌刷新、多设备支持、MFA状态）
  - 配额错误处理API（统一错误响应）

- **测试完善**
  - 安全模块完整测试覆盖
  - 密码验证基准测试
  - 会话管理并发测试

### Fixed
- **类型错误修复**
  - 修复 sandbox_test.go Permission/PermissionType 类型不匹配
  - 统一插件权限类型定义

## [v2.29.1] - 2026-03-15

### Fixed
- **构建修复**
  - 修复测试文件类型错误
  - Windows 构建兼容性

## [v2.29.0] - 2026-03-15

### Added
- **六部联建功能增强**
  - API v2 规划（版本路由、WebSocket实时通信）
  - 插件系统增强（热加载、沙箱隔离）
  - WebUI响应式优化（移动端适配）
  - Kubernetes Helm Chart（HPA、PDB、NetworkPolicy）
  - 文档国际化（英文文档、翻译指南）
  - 安全审计 v2.29.0

## [v2.28.0] - 2026-03-15

### Added
- **六部联建完成**
  - 48 files changed, 9437 insertions(+)
  - 完整的插件系统架构
  - 监控与可观测性增强

## [v2.27.0] - 2026-03-15

### Added
- **媒体服务模块** (兵部)
  - 媒体流媒体服务（支持 HLS/DASH）
  - 字幕处理（SRT/VTT/ASS 格式）
  - 视频缩略图生成
  - FFmpeg 转码服务
  - 媒体 API 处理器

- **监控模块增强** (工部)
  - 健康评分系统
  - 指标收集器
  - 报告集成接口

- **配额自动扩展** (户部)
  - 自动扩展配额策略
  - 配额使用监控
  - 自动扩展 API 处理器

- **报告系统增强** (礼部)
  - 增强版报告导出器
  - 监控数据源集成
  - 报告模板系统

- **测试完善**
  - Docker 管理单元测试
  - 网络诊断单元测试

### Fixed
- **IPv6 地址格式问题修复**
  - 修复网络诊断中 IPv6 地址解析
  - 统一地址格式处理

## [v2.26.0] - 2026-03-15

### Added
- **网络诊断工具集** (工部)
  - Ping 测试（支持自定义选项）
  - Traceroute 路由追踪
  - DNS 查询（A/AAAA/MX/NS/TXT）
  - 端口扫描（TCP/UDP）
  - Whois 域名查询
  - ARP 表获取
  - 网络连通性检查
  - Netstat 网络状态

- **Docker 管理增强** (兵部)
  - 容器批量操作
  - 镜像管理完善
  - 网络配置
  - 卷管理

- **自动化引擎完善** (礼部)
  - 工作流执行优化
  - Action 解析增强
  - 错误处理改进

- **Photos AI 增强** (兵部)
  - 回忆数据持久化存储
  - AI 分析结果缓存

### Fixed
- **网络模块测试超时问题**
  - 移除重复的类型定义
  - 统一使用 diagnostics.go 实现
  - 修复方法签名不匹配问题

## [v2.25.0] - 2026-03-15

### Fixed
- **代码格式修复**
  - 修复 automation/engine/workflow.go gofmt 格式问题
  - 修复 media/library.go gofmt 格式问题
  - 修复 replication/manager.go gofmt 格式问题

### Changed
- **代码质量改进**
  - 统一代码格式，符合 Go 标准规范
  - 通过 CI/CD 代码检查

## [v2.24.0] - 2026-03-15

### Added
- **Photos AI 增强** (兵部)
  - AI 数据清除功能（ClearAIData、ClearPhotoAIData）
  - 重新分析所有照片功能（ReanalyzeAll）
  - AI 分析结果保存到存储

- **Media 媒体库完善** (兵部)
  - 获取/更新/删除单个媒体项
  - 元数据搜索（电影/电视剧）
  - 获取元数据详情
  - 播放历史记录
  - 收藏功能

- **自动化触发器解析** (礼部)
  - parseTriggerConfig 解析四种触发器类型
  - parseActionConfig 解析九种动作类型
  - 实际成功率计算

- **插件管理增强** (礼部)
  - 从插件实例获取完整依赖信息

- **Replication 增强** (工部)
  - rsync 输出完整解析
  - 提取传输统计（字节数、速度、文件数）
  - 新增 RsyncStats 结构体

### Fixed
- **代码格式修复**
  - 修复 perf/manager.go gofmt 格式问题

## [v2.22.0] - 2026-03-15

### Added
- **Photos AI 增强** (兵部)
  - HEIC 转 JPEG 支持（使用 ffmpeg）
  - 日期范围匹配
  - 回忆查询功能
  - 云端 API 调用框架（Azure Face API、AWS Rekognition）

- **Photos 处理器增强** (兵部)
  - 多尺寸缩略图支持
  - 从认证信息获取 userID
  - 实际使用空间计算

- **分层存储增强** (兵部)
  - 下次执行时间计算

- **自动化触发器** (礼部)
  - 文件监控（基于 fsnotify）
  - 事件订阅系统
  - Webhook 端点实现
  - HMAC 签名验证

### Fixed
- **集群竞态条件修复**
  - 修复 `TestEdgeIntegration` 中 edgeManager 未设置的竞态条件
  - 将 `SetEdgeManager` 调用移到 `Initialize` 之前

## [v2.21.0] - 2026-03-15

### Added
- **短信认证支持** (刑部)
  - 阿里云短信 API 集成
  - 腾讯云短信 API 集成
  - 支持 SMS 验证码发送和验证

- **MFA 增强** (刑部)
  - 恢复码使用后自动移除
  - 临时验证码存储（带过期时间）

- **RBAC 中间件增强** (刑部)
  - 从用户对象获取组信息

- **NFS 管理增强** (礼部)
  - 完整 NFS 服务配置管理
  - 导出配置保存和加载

- **Web 中间件安全增强** (礼部)
  - CSRF 密钥从环境变量读取
  - Token 验证完善

- **共享服务完善** (礼部)
  - 全局 SMB 配置更新
  - 全局 NFS 配置更新

### Fixed
- **代码格式修复**
  - 修复 `internal/photos/handlers.go` gofmt 格式问题
  - 修复 `internal/photos/manager.go` gofmt 格式问题
  - 解决 CI/CD 代码检查失败问题

## [v2.20.2] - 2026-03-15

### Fixed
- **代码格式修复**
  - 修复 `internal/photos/handlers.go` gofmt 格式问题
  - 修复 `internal/photos/manager.go` gofmt 格式问题
  - 解决 CI/CD 代码检查失败问题

## [v2.20.1] - 2026-03-14

### Added
- **WebUI 页面完善**
  - 新增 iSCSI 目标管理页面 (`/iscsi`)
  - 新增在线文档页面 (`/office`) - OnlyOffice 集成
  - 新增通知中心页面 (`/notify`) - 多渠道通知管理
  - 新增性能优化页面 (`/optimizer`) - 智能分析与优化建议

- **API 文档更新**
  - 添加 iSCSI API 端点定义
  - 添加 Office API 端点定义
  - 添加 Notify API 端点定义
  - 添加 Optimizer API 端点定义
  - 更新版本号至 2.20.1

### Changed
- **路由注册完善** - 新增页面的静态路由注册
- **模块覆盖检查** - 确保所有有 API 的模块都有对应 WebUI 页面

## [v2.20.0] - 2026-03-14

### Changed
- **代码清理** - 删除未使用的函数和变量
  - 移除 initBTClients, parseUptime 等未调用函数
  - 移除 LogSearcher.mu, ParallelCompressor.mu 等未使用字段
  - 清理冗余代码，提升可维护性

- **CI/CD 优化** - 重构 ci-cd.yml，避免重复 job
  - 添加 Docker Compose 测试 job
  - 修复 Dockerfile BuildKit 跨平台构建参数

- **文档完善**
  - MILESTONES.md 版本路线图更新至 v2.19.0
  - 归档历史 TODO 文件到 docs/archive/todos/
  - CHANGELOG.md 格式规范化

- **项目清理** - 清理临时文件，节省约 224MB 空间

## [v2.19.0] - 2026-03-15

### Fixed
- **prediction 模块数据竞争修复**
  - 修复 `Predict()` 方法并发调用时的数据竞争问题
  - 将 `Predict()` 的读锁改为写锁，因为 `trainModel()` 会修改模型状态
  - 添加配置访问的安全读取方法

### Changed
- 改进并发安全性，确保多线程环境下稳定运行

## [v2.18.0] - 2026-03-15

### Added
- **下载器模块** - Transmission/qBittorrent 集成
  - 支持 Transmission BT 下载器
  - 支持 qBittorrent 下载器
  - 统一下载任务管理 API
  - WebUI 下载管理页面

- **照片管理增强**
  - 分片上传支持大文件
  - 智能搜索功能
  - AI 照片分析

- **数据分层策略完善**
  - 自动数据迁移规则
  - 存储层监控增强

- **网络配置持久化**
  - 网络配置保存和恢复
  - 配置版本管理

- **测试覆盖率提升**
  - 单元测试完善
  - 集成测试增强
  - 覆盖率达到 40%+

### Changed
- API 文档完善，更新到 v2.18.0
- 用户指南更新
- 国际化语言包完善

### Fixed
- 文档版本号同步
- API 文档错误修正

## [v2.17.1] - 2026-03-14

### Fixed
- **CI/CD Go 版本统一**
  - 统一所有 workflow 文件的 Go 版本为 1.26
  - 修复 go.mod 使用 Go 1.26.1 但 CI 配置使用 Go 1.25 导致的编译错误
  - 解决 "compile: version go1.26.1 does not match go tool version go1.25.8" 问题

## [v2.17.0] - 2026-03-14

### Changed
- 文档完善和版本号更新
- API 文档版本同步

## [v2.16.0] - 2026-03-15

### Added
- **预测分析模块** - 新增智能预测功能
  - 磁盘健康预测：基于 SMART 数据预测磁盘寿命和故障概率
  - 容量趋势分析：预测存储空间使用趋势，提前预警
  - 性能预测：识别高峰时段和潜在资源瓶颈
  - 网络趋势：带宽使用预测和连接数预测
  - 维护建议：智能生成维护计划和操作建议

- **国际化支持 (i18n)**
  - 支持中文 (zh-CN) 和英文 (en-US) 切换
  - 完整的 UI 文本翻译
  - 可扩展的翻译框架

- **API 文档系统**
  - 集成 Swagger/OpenAPI 文档生成
  - 所有 API 端点添加完整注释
  - 生成可交互的 API 文档页面

### Changed
- 优化了安全中心的事件展示逻辑
- 改进了运维中心的性能指标展示
- CI/CD 工作流优化
- 数据库查询优化

### Fixed
- 修复了小屏幕下的布局问题
- 修复了内存使用率计算错误

## [v2.15.0] - 2026-03-14

### Fixed
- **SQLite 驱动替换**
  - 将 `github.com/mattn/go-sqlite3` 替换为 `modernc.org/sqlite`（纯 Go 实现）
  - 消除 CGO 依赖，测试可在 `CGO_ENABLED=0` 环境下运行
  - 修改 `internal/system/monitor.go` 和 `internal/tags/manager.go` 驱动名

### Changed
- CI/CD 添加 CGO 支持配置（备用）
- 提升跨平台兼容性

## [v2.14.0] - 2026-03-14

### Added
- **测试覆盖率提升**
  - quota 模块单元测试 (`internal/quota/quota_test.go`)
  - trash 模块单元测试 (`internal/trash/manager_test.go`, `internal/trash/handlers_test.go`)
  - webdav 模块单元测试 (`internal/webdav/server_test.go`, `internal/webdav/handlers_test.go`, `internal/webdav/lock_test.go`)
  - 测试覆盖率报告生成 (`coverage_quota.out`, `coverage_trash.out`, `coverage_webdav.out`)

- **WebUI 页面完善**
  - 配额管理页面 (`webui/pages/dir-quota.html`) - 配额设置、用户配额、历史趋势
  - 回收站管理页面 (`webui/pages/trash.html`) - 文件恢复、批量操作、清理策略
  - WebDAV 配置页面 (`webui/pages/webdav.html`) - 服务配置、共享管理、权限设置

### Changed
- 测试框架完善，覆盖率提升
- WebUI 功能模块化设计

### Security
- Security Scan 问题修复
- 权限边界检查增强

## [v2.13.0] - 2026-03-14

### Added
- **API 端点完善**
  - 存储配额 API 增强
  - 回收站管理 API 完善
  - WebDAV API 优化

- **测试基础设施**
  - 测试覆盖率报告自动生成
  - 代码质量门禁配置

### Changed
- CI/CD 工作流优化
- Docker 构建优化

## [v2.12.0] - 2026-03-14

### Fixed
- **CI/CD 修复**
  - 统一所有工作流 Go 版本到 1.24（稳定版）
  - 修复 golangci-lint v2 版本兼容性问题
  - 修复 Docker 构建基础镜像版本

- **代码质量改进**
  - 修复 SMB 共享权限验证逻辑
  - 修复 NFS 连接统计计数器竞态条件
  - 改进并发安全性

### Changed
- 更新 `.github/workflows/ci-cd.yml` GO_VERSION
- 更新 `.github/workflows/docker-publish.yml` GO_VERSION
- 更新 `.github/workflows/release.yml` GO_VERSION
- 更新 `Dockerfile` 基础镜像到 `golang:1.25-alpine`
- 更新 `go.mod` Go 版本到 1.25.0

### Security
- 安全审查完成，评分 **B+**
- SMB/NFS 权限边界检查通过
- 无高危漏洞

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

### Fixed

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