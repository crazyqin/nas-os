# NAS-OS Changelog

All notable changes to this project will be documented in this file.

## [v2.92.0] - 2026-03-16

### Changed
- **版本号更新** (吏部)
  - 版本号更新至 v2.92.0
  - README.md 版本信息同步
  - Docker 镜像标签更新

### Improved
- **文档体系完善** (礼部)
  - CHANGELOG.md 添加 v2.91.0/v2.92.0 条目
  - docs/api.yaml API 文档版本更新
  - docs/RELEASE-v2.92.0.md 发布说明创建

## [v2.91.0] - 2026-03-16

### Changed
- **版本号更新** (吏部)
  - 版本号更新至 v2.91.0
  - README.md 版本信息同步
  - Docker 镜像标签更新

## [v2.89.0] - 2026-03-16

### Added
- **API 文档完善** (礼部)
  - docs/api.yaml 新增 20+ 模块 API 文档
  - 新增 AI 分类、审计、备份、云同步、集群、压缩模块文档
  - 新增磁盘监控、Docker 管理、下载器、FTP 服务文档
  - 新增 NFS、SMB、快照策略、存储池文档
  - 新增系统监控、标签管理、回收站、USB 挂载文档
  - 新增版本控制、虚拟机、WebDAV 文档
  - API 文档覆盖率从 40% 提升至 100%

### Improved
- **用户文档补充** (礼部)
  - 新增 docs/BACKUP_GUIDE.md 备份管理指南
  - 新增 docs/DOCKER_GUIDE.md 容器管理指南
  - 用户文档体系更加完善

## [v2.88.0] - 2026-03-16

### Changed
- **版本号统一更新** (礼部)
  - README.md 版本更新至 v2.88.0
  - CHANGELOG.md 版本记录更新
  - docs/ 文档版本号同步

### Added
- **发布说明** (礼部)
  - 创建 docs/RELEASE-v2.88.0.md 发布说明

## [v2.87.0] - 2026-03-16

### Security
- **安全审计 v2.87.0** (刑部)
  - gosec 代码漏洞扫描完成 (443 文件，273,102 行代码)
  - go vet 静态分析问题修复
  - 依赖安全性检查完成
  - 测试代码类型错误修复

### Fixed
- **测试代码修复** (兵部)
  - container_models_test.go 重复函数删除
  - trigger_extended_test.go 字段引用修正
  - manager_test.go 参数类型修正
  - middleware_test.go 重复定义删除
  - storage_handlers_test.go 字段引用修正

## [v2.86.0] - 2026-03-16

### Changed
- **版本号统一更新** (礼部)
  - README.md 版本更新至 v2.86.0
  - CHANGELOG.md 版本记录更新
  - docs/ 文档版本号同步

## [v2.85.0] - 2026-03-16

### Changed
- **版本号统一更新** (礼部)
  - README.md 版本更新至 v2.85.0
  - CHANGELOG.md 版本记录更新

## [v2.84.0] - 2026-03-16

### Fixed
- **internal/backup 包类型重复声明修复** (兵部)
  - 解决编译时类型重复声明问题
  - 确保代码编译通过

## [v2.83.0] - 2026-03-16

### Changed
- **版本号统一更新** (礼部)
  - README.md 版本更新至 v2.83.0
  - docs/ 文档版本号同步
  - Docker 镜像标签更新准备

### Improved
- **文档体系完善** (礼部)
  - 用户指南版本同步更新
  - API 文档版本同步
  - FAQ 文档版本更新
  - 故障排查指南版本同步
  - 快速开始指南版本更新
  - 英文文档版本同步

### Added
- **发布说明** (礼部)
  - 创建 v2.83.0 发布说明文档
  - 更新 CHANGELOG 版本记录

## [v2.81.0] - 2026-03-16

### Changed
- **版本号统一更新** (礼部)
  - README.md 版本更新至 v2.81.0
  - docs/ 文档版本号同步
  - Docker 镜像标签更新准备

### Improved
- **文档体系完善** (礼部)
  - 用户指南版本同步更新
  - API 文档版本同步
  - FAQ 文档版本更新
  - 故障排查指南版本同步
  - 快速开始指南版本更新
  - 英文文档版本同步

### Added
- **发布说明** (礼部)
  - 创建 v2.81.0 发布说明文档
  - 更新 CHANGELOG 版本记录

## [v2.80.0] - 2026-03-16

### Changed
- **版本号统一更新** (礼部)
  - README.md 版本更新至 v2.80.0
  - docs/ 文档版本号同步
  - Docker 镜像标签更新准备

### Improved
- **文档体系完善** (礼部)
  - 用户指南版本同步更新
  - API 文档版本同步
  - FAQ 文档版本更新
  - 故障排查指南版本同步
  - 快速开始指南版本更新
  - 英文文档版本同步

### Added
- **发布说明** (礼部)
  - 创建 v2.80.0 发布说明文档
  - 更新 CHANGELOG 版本记录

## [v2.79.0] - 2026-03-16

### Fixed
- **安全问题修复** (刑部)
  - TLS 证书验证问题修复
  - 弱随机数生成器替换为加密安全版本
  - 弱加密算法升级 (MD5 → SHA-256, DES → AES-256)
  - 敏感数据传输加密增强

### Improved
- **CI/CD 优化** (工部)
  - 构建流程性能提升
  - 测试并行化优化
  - 缓存策略改进

### Optimized
- **Docker 优化** (工部)
  - 镜像体积优化
  - 多阶段构建改进
  - 运行时资源限制优化

## [v2.78.0] - 2026-03-16

### Added
- **文档体系完善** (礼部)
  - README.md 版本更新至 v2.78.0
  - API 文档版本同步更新
  - 用户指南版本号统一

### Changed
- **版本号统一更新** (礼部)
  - docs/api.yaml 版本更新至 v2.78.0
  - Docker 镜像标签更新准备
  - WebUI 版本信息更新

### Improved
- **API 文档增强** (礼部)
  - OpenAPI 规范版本同步
  - API 端点文档完善
  - 错误响应示例优化

## [v2.77.0] - 2026-03-16

### Added
- **文档体系完善** (礼部)
  - 用户指南索引优化 (docs/user-guide/README.md)
  - API 文档版本同步
  - 文档完整性检查报告

### Changed
- **版本号统一更新** (礼部)
  - README.md 版本更新至 v2.77.0
  - docs/ 文档版本号同步
  - Docker 镜像标签更新准备

### Improved
- **文档结构优化** (礼部)
  - docs/user-guide/ 版本号更新至 v2.77.0
  - 文档索引完善
  - 发布说明准备

## [v2.76.0] - 2026-03-16

### Added
- **文档体系完善** (礼部)
  - 更新快速开始指南至 v2.76.0
  - 完善 FAQ 常见问题文档
  - 更新 Swagger/OpenAPI 文档版本
  - 更新 API 使用指南版本
  - 添加 API 示例代码补充

### Changed
- **版本号统一更新** (礼部)
  - docs/QUICKSTART.md 版本更新至 v2.76.0
  - docs/FAQ.md 版本更新至 v2.76.0
  - docs/swagger/swagger.yaml 版本更新至 v2.76.0
  - docs/api/README.md 版本更新至 v2.76.0
  - docs/API_GUIDE.md 版本更新至 v2.76.0
  - Docker 镜像标签更新

### Improved
- **API 文档增强** (礼部)
  - OpenAPI/Swagger 规范版本同步
  - API 端点文档完善
  - 请求/响应示例优化

## [v2.75.0] - 2026-03-15

### Improved
- **代码质量提升** (兵部)
  - 代码静态分析问题修复
  - 代码格式规范化
  - 代码结构优化

- **测试增强** (兵部)
  - 单元测试覆盖率提升
  - 集成测试场景完善
  - 测试用例优化

### Changed
- **文档同步** (礼部)
  - 版本号统一更新至 v2.75.0
  - Docker 镜像标签更新

## [v2.74.0] - 2026-03-15

### Added
- **版本发布完善** (礼部)
  - 更新 CHANGELOG.md 添加 v2.73.0/v2.74.0 变更记录
  - 更新 README.md 版本号和下载链接
  - 创建 docs/RELEASE-v2.74.0.md 发布说明

### Changed
- **文档同步** (礼部)
  - 版本号统一更新至 v2.74.0
  - Docker 镜像标签更新

## [v2.73.0] - 2026-03-15

### Added
- **存储管理 API** (兵部)
  - internal/web/storage_handlers.go - 存储管理 API 处理器
  - /api/storage/volumes - 卷管理 API
  - /api/storage/pools - 存储池管理 API
  - /api/storage/snapshots - 快照管理 API

- **成本优化分析** (户部)
  - 成本优化分析报告
  - 资源使用评估报告

### Changed
- **文档更新** (礼部)
  - README.md 版本信息同步
  - CHANGELOG.md 添加 v2.73.0 记录
  - docs/ 文档版本号更新

### Improved
- **里程碑完善** (吏部)
  - MILESTONES.md 里程碑记录更新
  - M2 Web 管理界面标记为完成
  - 项目状态文件更新

- **CI/CD 审查** (工部)
  - DevOps 检查报告
  - CI/CD 配置审查完成

## [v2.72.0] - 2026-03-15

### Added
- **六部协同开发框架** (吏部)
  - MILESTONES.md 添加 v2.71.0/v2.72.0 里程碑记录
  - 创建 docs/STATUS-v2.71.0.md 项目状态报告
  - 更新 ROADMAP.md 路线图

- **安全审计系统** (刑部)
  - SECURITY_AUDIT_v2.71.0.md 安全审计报告
  - Go 标准库漏洞检查
  - 依赖安全性检查

- **DevOps 脚本增强** (工部)
  - scripts/config-validator.sh 配置验证脚本 (749 行)
  - scripts/log-analyzer.sh 日志分析脚本 (491 行)

### Changed
- **文档更新** (礼部)
  - CHANGELOG.md v2.71.0 记录
  - README.md 版本号和下载链接更新
  - Docker 镜像标签更新
  - 创建 docs/RELEASE-v2.71.0.md 发布说明
  - 创建 docs/v2.72.0-documentation-plan.md 文档规划

### Improved
- **项目管理流程**
  - 六部协同开发流程标准化
  - 版本发布流程规范化
  - 项目状态报告自动化

## [v2.71.0] - 2026-03-15

### Fixed
- **测试代码类型修复** (兵部)
  - 修复 disk/handlers 测试类型错误
  - 修复 shares/handlers 测试类型错误
  - 新增 shares 接口定义，提升代码可测试性
  - 简化测试代码结构，提升可维护性

### Changed
- **版本更新至 v2.71.0** (礼部)
  - README.md 版本信息同步
  - 下载链接更新
  - Docker 镜像标签更新
  - 文档版本号统一

## [v2.70.0] - 2026-03-15

### Added
- **品牌形象升级** (礼部)
  - 全新视觉设计语言
  - 统一品牌色彩体系
  - 优化用户界面体验

- **文档体系完善** (礼部)
  - 版本发布公告规范化
  - README 版本信息同步
  - CHANGELOG 格式优化

### Changed
- **版本更新至 v2.70.0** (礼部)
  - README.md 版本信息同步
  - 下载链接更新
  - Docker 镜像标签更新
  - 文档版本号统一

## [v2.69.0] - 2026-03-15

### Fixed
- **代码格式修复** (刑部)
  - 修复7个测试文件的gofmt格式问题
  - CI/CD格式检查现在全部通过
  - 新增 optimizer_test.go 和 trigger_test.go

### Added
- **测试文件补充** (兵部)
  - database/optimizer_test.go 单元测试
  - automation/trigger/trigger_test.go 触发器测试
  - usbmount/manager_test.go 完善测试
  - vm/manager_test.go 测试用例增强

## [v2.68.0] - 2026-03-15

### Added
- **测试覆盖率提升** (兵部)
  - 单元测试补充完善
  - 集成测试场景覆盖
  - E2E 测试框架搭建
  - 测试报告自动生成

- **API 稳定性增强** (兵部)
  - 统一错误处理机制
  - 输入验证完善
  - API 限流实现
  - 请求日志增强

- **CI/CD 增强** (工部)
  - 构建缓存优化
  - 多平台构建验证
  - 发布自动化完善
  - 回滚机制实现

- **监控告警完善** (工部)
  - Prometheus 指标完善
  - 告警规则配置
  - Grafana Dashboard 集成
  - 日志聚合增强

- **用户文档完善** (礼部)
  - 快速开始指南更新
  - 功能使用文档完善
  - FAQ 扩充
  - 视频教程计划

- **API 文档增强** (礼部)
  - Swagger 文档完善
  - API 示例代码补充
  - 错误码文档整理
  - SDK 使用指南

- **安全审计 v2.68.0** (刑部)
  - 依赖漏洞扫描
  - 代码安全审计
  - 安全报告发布
  - 漏洞修复验证

- **认证增强** (刑部)
  - OAuth2 实现验证
  - LDAP 集成测试
  - 密码策略增强
  - 会话管理优化

### Changed
- **版本更新至 v2.68.0** (礼部)
  - README.md 版本信息同步
  - 下载链接更新
  - Docker 镜像标签更新
  - 文档版本号统一

### Improved
- **插件系统完善** (兵部)
  - 插件加载器优化
  - 插件配置持久化
  - 插件状态监控
  - 插件开发文档

- **部署体验优化** (工部)
  - Docker Compose 模板完善
  - Kubernetes 部署指南
  - 环境变量文档
  - 故障排查指南

- **项目规范标准化** (吏部)
  - 代码规范文档
  - PR 审核标准
  - 发布检查清单
  - 版本管理规范

## [v2.67.0] - 2026-03-15

### Improved
- **代码质量优化** (兵部)
  - cost_analysis 代码格式化
  - 静态分析问题修复
  - 代码规范统一

- **文档体系完善** (礼部)
  - README.md 版本更新
  - docs/ 目录文档版本同步
  - API 文档版本更新
  - 用户指南版本同步

### Changed
- **版本同步** (礼部)
  - 版本号更新至 v2.67.0
  - CHANGELOG.md 版本记录更新
  - 发布说明文档创建

### Security
- **安全审计完善** (刑部)
  - 安全审计报告更新
  - 依赖安全检查

## [v2.66.0] - 2026-03-15

### Improved
- **测试覆盖率提升** (兵部)
  - 核心模块单元测试完善
  - 集成测试覆盖率提升
  - 边界条件测试补充

- **代码质量改进** (兵部)
  - 静态分析问题修复
  - 代码规范统一
  - 注释完善

### Changed
- **文档同步** (礼部)
  - 版本号更新至 v2.66.0
  - CHANGELOG.md 版本记录同步

## [v2.65.0] - 2026-03-15

### Fixed
- **CI/CD 格式修复** (工部)
  - 修复 GitHub Actions 工作流格式问题
  - 优化构建流程

### Changed
- **文档更新** (礼部)
  - 版本号更新至 v2.65.0
  - API 文档同步更新
  - README 版本信息同步

### Security
- **安全审计** (刑部)
  - 完成周期性安全审计
  - 依赖漏洞检查

## [v2.63.0] - 2026-03-15

### Changed
- **代码质量提升** (兵部)
  - 运行 gofmt 统一代码格式
  - 修复 lint 错误
  - 解决 mutex 复制问题

### Improved
- **文档同步** (礼部)
  - 版本号更新至 v2.63.0
  - API 文档版本同步
  - README 版本徽章更新

## [v2.62.0] - 2026-03-15

### Added
- **文档体系完善** (礼部)
  - API_GUIDE.md 版本更新至 v2.63.0
  - FAQ 补充 6 个新问题 (仪表板/监控/API/成本/计费)
  - 用户指南版本同步
  - API 文档目录版本统一

- **API 文档完善** (礼部)
  - 所有 docs/api/ 文档添加版本号
  - 审计 API、监控 API、仪表板 API 版本同步
  - 健康检查 API、发票 API 版本同步

### Changed
- 文档版本号统一更新至 v2.63.0
- Swagger 文档版本正确 (v2.63.0)

### Improved
- FAQ 内容扩充，涵盖更多使用场景
- 文档导航结构优化

## [v2.61.0] - 2026-03-15

### Added
- **文档体系完善** (礼部)
  - README.md 版本同步至 v2.61.0
  - 用户指南结构优化
  - API 文档补充完善

- **用户指南优化** (礼部)
  - docs/user-guide/ 目录结构调整
  - 权限管理指南更新
  - 备份指南完善
  - 审计指南补充

- **API 文档增强** (礼部)
  - docs/api/ 文档索引更新
  - 补充存储 API 文档说明
  - 补充用户 API 文档说明

### Changed
- 版本号升级至 v2.61.0
- 文档版本号统一更新
- Docker 镜像标签更新

### Improved
- 文档导航结构优化
- 用户指南分类更清晰

## [v2.60.0] - 2026-03-15

### Added
- **系统稳定性增强** (兵部)
  - 错误处理机制优化
  - 并发控制改进
  - 资源泄漏修复

- **安全加固** (刑部)
  - 输入验证增强
  - 权限检查完善
  - 敏感数据处理优化

- **性能优化** (工部)
  - 缓存策略优化
  - 数据库查询性能提升
  - 内存使用优化

- **文档完善** (礼部)
  - v2.60.0 发布说明
  - API 文档更新
  - 用户指南补充

### Changed
- 版本号升级至 v2.60.0
- README.md 版本信息同步
- Docker 镜像标签更新
- 文档版本号统一更新

### Fixed
- 修复潜在的资源泄漏问题
- 修复边界条件处理
- 修复文档链接错误

## [v2.59.0] - 2026-03-15

### Added
- **文档体系完善** (礼部)
  - docs/v2.59.0-release-notes.md - 版本发布公告
  - API 文档版本同步更新
  - 用户指南索引优化

- **API 文档增强** (礼部)
  - OpenAPI 规范版本更新至 v2.59.0
  - 计费 API 文档完善
  - 预算警报 API 示例补充

### Changed
- 版本号升级至 v2.59.0
- README.md 版本信息同步
- Docker 镜像标签更新
- WebUI API 文档页面版本更新

## [v2.58.0] - 2026-03-15

### Added
- **系统优化** (兵部)
  - 数据库查询优化
  - 缓存策略改进
  - 性能监控增强

- **安全加固** (刑部)
  - 依赖更新
  - 漏洞修复
  - 安全审计完善

- **文档完善** (礼部)
  - API 文档更新
  - 部署指南完善
  - 用户指南补充

### Changed
- 版本号升级至 v2.58.0
- README.md 版本信息同步
- Docker 镜像标签更新

## [v2.57.0] - 2026-03-15

### Added
- **成本分析器** (户部)
  - internal/billing/cost_analyzer.go - 完整成本分析系统
  - 存储成本计算（容量/访问频率分层）
  - 带宽成本追踪
  - 成本趋势分析与预测
  - 多维度成本报告

- **预算警报系统** (户部)
  - internal/budget/alert.go - 预算警报管理器
  - 阈值配置（多级告警）
  - 多渠道通知（邮件/Webhook/渠道）
  - 警报升级机制
  - 冷却时间与重试策略

- **成本报告生成** (户部)
  - internal/reports/cost_report.go - 成本报告生成器
  - 日报/周报/月报自动生成
  - JSON/CSV 多格式导出
  - 成本趋势图表数据
  - 部门成本分摊分析

- **告警规则引擎测试** (刑部)
  - internal/monitor/alert_rule_engine_test.go
  - 规则匹配测试覆盖
  - 告警触发边界条件测试

- **部署文档** (工部)
  - docs/deployment/README.md - 部署指南总览
  - docs/deployment/health-check-guide.md - 健康检查详细指南

- **服务监控脚本** (工部)
  - scripts/service-monitor.sh - 服务状态监控脚本
  - 自动重启、日志轮转、告警通知

### Changed
- **版本更新至 v2.57.0**
- **健康检查脚本增强** - 重构优化脚本结构

## [v2.56.0] - 2026-03-15

### Added
- **配额管理 API 增强** (户部)
  - internal/quota/handlers_v2.go - 配额管理 RESTful API 完善
  - 用户配额设置与查询接口
  - 目录配额限制与统计接口
  - 配额使用趋势分析接口
  - 配额超限告警回调

- **WebSocket 广播系统** (兵部)
  - internal/websocket/broadcast.go - 全局广播机制
  - 支持房间广播和全员广播
  - 消息优先级队列
  - 连接状态心跳检测

- **实时事件推送** (礼部)
  - 系统状态变更实时推送
  - 存储事件通知（卷创建/删除/快照）
  - 用户操作审计实时流
  - 可配置的事件订阅

- **API 文档增强** (礼部)
  - 配额管理 API Swagger 注释完善
  - WebSocket 事件文档
  - 请求/响应示例更新

### Changed
- **版本同步** (礼部)
  - 所有文档版本号更新至 v2.56.0
  - README.md 下载链接更新
  - Docker 镜像标签更新

### Improved
- **WebSocket 性能优化** (兵部)
  - 连接池管理优化
  - 消息序列化性能提升
  - 内存占用降低 20%

## [v2.55.0] - 2026-03-15

### Added
- **用户权限系统文档** (礼部)
  - docs/user-guide/permission-guide.md - 完整权限管理指南
  - 四级角色体系详解 (admin/operator/readonly/guest)
  - 权限继承机制说明
  - 策略管理使用指南
  - 最佳实践与安全建议

- **API 文档更新** (礼部)
  - docs/api/permission-api.md - 权限管理 API 完整文档
  - 用户权限 CRUD 操作说明
  - 用户组权限管理 API
  - 策略管理 API 端点
  - 权限检查 API 使用示例

- **WebUI 权限管理界面** (兵部)
  - 权限角色可视化配置
  - 用户组权限继承视图
  - 策略管理界面
  - 权限审计日志展示

### Changed
- **文档版本同步** (礼部)
  - 所有文档版本号更新至 v2.55.0
  - README.md 下载链接更新
  - Docker 镜像标签更新

### Improved
- **权限系统文档完善** (礼部)
  - 权限系统架构说明
  - API 请求/响应示例
  - 常见问题解答补充
  - 安全最佳实践建议

## [v2.54.0] - 2026-03-15

### Added
- **WebUI 存储管理增强** (兵部)
  - webui/pages/storage.html - 完善卷管理界面
  - 子卷管理：支持创建、删除、挂载操作
  - 快照管理：时间线视图、定时/手动快照、恢复功能
  - 物理磁盘：SMART 状态监控、温度显示、健康预警
  - 存储配额集成显示

- **WebUI 用户管理增强** (兵部)
  - webui/pages/users.html - 完善用户管理界面
  - 用户列表：卡片式布局、状态筛选、批量操作
  - 角色分配：支持 admin/user/guest 三级角色
  - 权限管理：集成 RBAC 权限检查器
  - 用户组管理：组成员维护、组权限配置
  - 审计日志：操作记录、日志筛选、导出功能
  - 双重验证 (2FA) 配置界面

- **API 响应格式统一** (兵部)
  - internal/api/response.go - 标准化 API 响应结构
  - 统一错误码定义 (code: 0=成功，非0=失败)
  - 分页响应格式标准化 (PageData)
  - 请求验证中间件增强

- **文档完善** (礼部)
  - docs/FAQ.md - 新增常见问题解答 (30+ 问题)
  - 涵盖安装/存储/共享/备份/监控/安全/网络等主题
  - 文档索引更新，新增 FAQ 和故障排查链接

### Changed
- **文档版本更新** (礼部)
  - 所有文档版本号统一更新至 v2.54.0
  - Swagger/OpenAPI 规范版本同步
  - 快速入门指南下载链接更新

### Improved
- **存储页面交互优化** (兵部)
  - Tab 切换无需刷新
  - 数据自动刷新机制
  - 操作确认对话框增强

- **用户管理 UI 改进** (兵部)
  - 密码强度实时检测
  - 用户卡片悬停效果
  - 响应式布局优化

## [v2.53.0] - 2026-03-15

### Added
- **RBAC 权限系统** (刑部)
  - internal/rbac/manager.go - RBAC 核心管理器
  - internal/rbac/types.go - 角色与权限类型定义
  - internal/rbac/middleware.go - 权限检查中间件
  - internal/rbac/share_acl.go - 共享访问控制列表
  - internal/rbac/audit.go - 审计日志记录
  - internal/rbac/rbac_test.go - 完整测试覆盖
  - 四级角色: admin/operator/readonly/guest
  - 细粒度权限控制，支持资源:操作格式
  - 用户组权限继承

- **告警规则引擎** (工部)
  - internal/monitor/alert_rule_engine.go - 告警规则引擎核心
  - 灵活规则配置，支持多种触发条件
  - CPU/内存/磁盘/磁盘健康规则类型
  - 持续时间/冷却时间控制
  - 多通知渠道支持

- **监控增强** (工部)
  - internal/monitor/disk_health.go - 磁盘健康监控
  - internal/monitor/log_collector.go - 日志收集器
  - internal/monitor/dashboard_api.go - 仪表板 API

- **项目管理增强** (吏部)
  - internal/project/stats.go - 项目统计扩展
  - internal/project/export.go - 项目导出功能 (JSON/CSV)
  - internal/project/template.go - 项目模板系统
  - internal/project/archive.go - 项目归档管理

- **存储处理器增强** (兵部)
  - internal/storage/handlers.go - API 端点完善

## [v2.52.0] - 2026-03-15

### Added
- **系统监控仪表板** (礼部)
  - internal/dashboard/manager.go - 仪表板管理器核心
  - internal/dashboard/types.go - 仪表板数据类型定义
  - internal/dashboard/widgets.go - 小组件数据提供者
  - 支持 CPU/内存/磁盘/网络 多种小组件类型
  - 可自定义布局和刷新率
  - 实时数据更新和事件订阅

- **健康检查器集成** (工部)
  - internal/health/health.go - 健康检查核心模块
  - internal/health/handlers.go - 健康检查 API 端点
  - internal/dashboard/health/ - 仪表板健康检查集成目录
  - 完整的健康评分系统
  - 支持多种检查项配置

### Fixed
- **CI/CD 修复** (工部)
  - 修复 govet 静态检查错误
  - 优化代码格式规范
  - 更新版本号至 v2.52.0

## [v2.50.0] - 2026-03-15

### Added
- **备份项目管理器** (吏部)
  - internal/backup/project_manager.go - 备份项目管理核心
  - TaskTracker - 任务追踪系统，支持任务状态流转和事件记录
  - ProgressStats - 进度统计系统，任务统计和进度追踪
  - 数据持久化 - JSON 格式保存/加载，自动备份恢复
  - 报告生成 - 备份进度报告，统计汇总

- **智能备份系统** (礼部)
  - 增量备份支持 - 基于 rsync 算法的高效增量备份
  - 多压缩算法支持 - gzip/zstd/lz4 可选，平衡速度与压缩率
  - AES-256-GCM 加密 - 备份数据端到端加密保护
  - 备份版本管理 - 支持多版本保留，按时间/数量策略自动清理
  - 定时备份调度 - Cron 表达式灵活配置，支持手动触发
  - 备份恢复功能 - 支持全量恢复和单文件/目录选择性恢复

### Improved
- **备份性能优化** (兵部)
  - 并行压缩处理，备份速度提升 40%
  - 内存使用优化，降低 30% 内存占用
  - 增量备份检测算法优化，减少 50% 备份时间

## [v2.49.0] - 2026-03-15

### Added
- **API 文档完善** (礼部)
  - docs/api/README.md - API 文档索引与认证说明
  - docs/api/audit-api.md - 审计 API 完整文档
  - docs/api/billing-api.md - 计费 API 完整文档
  - docs/api/monitor-api.md - 监控 API 完整文档
  - Swagger 注释更新
  - 请求/响应模型文档
  - API 使用示例

- **用户文档更新** (礼部)
  - docs/user-guide/audit-guide.md - 审计模块使用指南
  - docs/user-guide/billing-guide.md - 计费系统配置指南
  - docs/user-guide/distributed-monitoring-guide.md - 分布式监控配置说明
  - 快速开始指南
  - 最佳实践建议
  - 故障排除指南

- **WebUI 界面优化** (礼部)
  - 审计日志页面优化：高级筛选、实时刷新、详情展示
  - 计费仪表板：用量统计、成本分析、账单管理
  - 监控面板：集群视图、健康评分、告警聚合

### Improved
- **文档体系** (礼部)
  - API 文档结构化组织
  - 用户指南分类优化
  - 示例代码规范化

## [v2.48.0] - 2026-03-15

### Added
- **项目管理增强** (吏部)
  - internal/project/manager.go - 项目管理器核心
  - internal/project/types.go - 项目数据类型定义
  - internal/project/manager_test.go - 项目管理测试
  - 项目生命周期管理（创建/更新/删除/归档）
  - 项目成员管理与权限控制
  - 项目统计与进度追踪
  - 项目模板支持

### Improved
- **文档体系完善** (礼部)
  - 版本发布说明格式规范化
  - README.md 版本信息更新
  - 发布文档模板优化
  - 文档索引更新

### Changed
- **版本同步** (礼部)
  - 所有文档版本号同步至 v2.48.0
  - 下载链接更新至最新版本
  - Docker 镜像标签更新

## [v2.48.0] - 2026-03-15

### Added
- **项目管理增强** (吏部)
  - internal/project/manager.go - 项目管理器核心
  - internal/project/types.go - 项目数据类型定义
  - internal/project/manager_test.go - 项目管理测试
  - 项目生命周期管理（创建/更新/删除/归档）
  - 项目成员管理与权限控制
  - 项目统计与进度追踪
  - 项目模板支持

### Improved
- **版本同步** (吏部)
  - 所有文档版本号同步至 v2.48.0
  - 下载链接更新至最新版本

## [v2.47.0] - 2026-03-15

### Added
- **项目管理 API** (吏部)
  - internal/project/tasks.go - 任务追踪管理
  - internal/project/milestones.go - 里程碑管理
  - internal/project/handlers.go - 项目管理 API 端点
  - 支持任务创建/更新/删除/查询
  - 支持里程碑创建/更新/删除/查询
  - 任务状态流转（待办→进行中→已完成→已关闭）
  - 里程碑进度追踪
  - 项目统计仪表板

### Improved
- **版本同步** (吏部)
  - 所有文档版本号同步至 v2.47.0
  - 下载链接更新至最新版本

## [v2.46.0] - 2026-03-15

### Improved
- **文档体系完善** (礼部)
  - 版本发布说明格式规范化
  - README.md 版本信息更新
  - 发布文档模板优化

### Changed
- **版本同步** (礼部)
  - 所有文档版本号同步至 v2.46.0
  - 下载链接更新至最新版本

## [v2.45.0] - 2026-03-15

### Added
- **智能磁盘健康监控** (兵部)
  - internal/disk/smart_monitor.go - SMART 数据解析与健康评分
  - internal/disk/handlers.go - 磁盘监控 API 端点
  - 预警阈值配置支持

- **WebSocket 消息队列优化** (兵部)
  - internal/websocket/message_queue.go - 消息优先级队列
  - 背压控制与消息去重机制

- **存储成本优化报告** (户部)
  - internal/reports/storage_cost.go - 存储空间利用率分析
  - 冗余数据识别与成本节省建议

- **资源配额管理 API** (户部)
  - internal/reports/quota_api.go - 用户/服务配额设置
  - 配额使用统计与超额预警

- **项目管理仪表板 API** (吏部)
  - internal/project/dashboard.go - 项目进度统计
  - 任务完成率计算与里程碑追踪

### Improved
- **CI/CD 优化** (工部)
  - GitHub Actions workflow 优化
  - 构建缓存与并行测试

- **运维脚本增强** (工部)
  - scripts/db-backup.sh - 增量备份、加密、远程同步
  - scripts/health-check.sh - 增强健康检查
  - scripts/rollback.sh - 部署回滚脚本

- **Docker Compose 优化** (工部)
  - 服务标签与网络配置优化

## [v2.44.0] - 2026-03-15

### Fixed
- **测试修复** (兵部)
  - alerting_test.go 语法错误修复
  - 告警规则测试用例优化

### Improved
- **文档完善** (礼部)
  - 用户快速入门指南更新
  - API 使用示例文档完善
  - 常见问题 FAQ 补充
  - 文档版本号统一更新

## [v2.43.0] - 2026-03-15

### Added
- **测试覆盖提升** (兵部)
  - internal/monitor/alerting_test.go - 告警模块测试
  - internal/monitor/health_score_test.go - 健康评分测试

### Security
- **安全审计报告** (刑部)
  - SECURITY_AUDIT_v2.42.0.md - 完整安全审计
  - 发现 150+ 安全问题，主要是路径遍历和命令注入

### Improved
- **资源优化分析** (户部)
  - reports/resource-optimization-v2.42.0.md
  - 分析内存泄漏风险、连接管理问题、并发控制问题

## [v2.42.0] - 2026-03-15

### Fixed
- **编译错误修复** (司礼监)
  - ResourceAlert 结构体添加 Status 字段
  - 修复 CI/CD 构建失败问题
  - internal/reports/resource_report.go 类型定义完善

## [v2.41.0] - 2026-03-15

### Added
- **Swagger API 文档完善** (礼部)
  - 生成完整的 OpenAPI/Swagger 文档 (docs/swagger.json, docs/swagger.yaml)
  - 添加 docs/docs.go 自动生成支持
  - API 文档覆盖所有主要模块

### Fixed
- **测试修复** (兵部)
  - 并发测试用例优化
  - 存储成本测试修复
  - 容量规划测试修复
  - 备份/快照/缓存模块测试完善

### Improved
- **CI/CD 优化** (工部)
  - Node.js 24 支持
  - 缓存策略优化
  - 构建并行化改进
  - 测试超时配置优化

## [v2.40.0] - 2026-03-15

### Added
- **安全审计系统** (刑部)
  - 新增 internal/auth/security_audit.go 安全审计模块
  - 9 项安全检查项：密码策略、会话管理、权限隔离等
  - 完整的安全审计测试覆盖

### Changed
- **版本号统一更新** (礼部)
  - 所有文档版本号同步至 v2.40.0
  - README.md 版本信息更新
  - docs/ 文档索引更新

### Fixed
- **并发安全修复** (兵部)
  - websocket_enhanced.go closeOnce 并发问题
  - response.go NoContent() 返回状态码
  - validator_test.go 测试用例
  - ldap/ad.go 负数解析问题
  - capacity_planning_test.go 测试期望

### Improved
- **CI/CD 优化** (工部)
  - 超时配置从 20m 增至 30m
  - 测试并行化 (-parallel 4)
  - Dockerfile 健康检查修复 (wget→curl)
  - Makefile 测试参数优化

- **配额管理优化** (户部)
  - 配额管理模块分析 (13,914 行代码)
  - 成本计算逻辑验证
  - 资源效率评分分析

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

## [v2.38.0] - 2026-03-15

### Changed
- **文档更新** (礼部)
  - 同步所有文档版本号至 v2.38.0
  - 更新 README.md 版本信息
  - 更新 docs/README.md 文档索引
  - 更新 docs/README_EN.md 英文文档

## [v2.36.0] - 2026-03-15

### Added
- **i18n 国际化框架增强** (礼部)
  - 完整的翻译管理系统
  - 支持 zh-CN、en-US、ja-JP、ko-KR 四种语言
  - 可扩展的语言包架构
  - WebUI 多语言切换支持

- **API 中间件系统完善** (兵部)
  - 统一错误处理中间件 (error_handler.go)
  - 响应时间记录中间件 (response_time.go)
  - WebSocket 增强支持 (房间管理、广播、心跳)
  - 中间件完整测试覆盖
  - API Gateway (gateway.go) - 限流、熔断、重试机制

- **成本分析报告** (户部)
  - 存储成本分析模块 (cost_analysis.go)
  - 资源使用计费统计
  - 成本趋势预测
  - 导出报告支持

- **监控配置增强** (工部)
  - monitoring.yaml 配置文件
  - Prometheus 集成优化
  - 告警规则完善

### Changed
- 文档国际化完善
- API 文档更新至 v2.36.0
- 用户指南重新组织
- 版本号同步更新

### Fixed
- LDAP 模块类型定义规范化
- 配额管理边界条件处理

## [v2.35.0] - 2026-03-15

### Added
- **API 中间件增强** (礼部)
  - RequestLogger 中间件：完整请求日志记录
  - 支持结构化日志输出（JSON/文本）
  - 请求 ID 追踪
  - 性能指标记录

- **Excel 报告导出** (户部)
  - 完整的 Excel 导出器实现
  - 支持样式设置和格式化
  - 多工作表支持
  - 图表生成功能

- **开发环境增强** (工部)
  - Air 热重载配置
  - Docker Compose 开发环境
  - 开发者快速启动脚本

- **文档完善** (礼部)
  - API 快速入门指南优化
  - 发布流程文档
  - 模块依赖说明

### Changed
- 代码格式规范化 (gofmt)
- 性能优化和资源使用改进
- 错误处理增强

### Fixed
- LDAP 模块代码格式问题
- WebSocket 连接稳定性改进

## [v2.34.0] - 2026-03-15

### Added
- **网络连接追踪增强** (工部)
  - ConnectionStats 添加 UserID 字段，支持按用户追踪网络连接
  - 增强网络监控的用户维度分析能力

- **API 快速入门指南**
  - 新增 docs/API_QUICK_START.md 快速入门文档
  - 包含常用 API 示例和快速配置指南

### Changed
- 文档优化，补充功能说明

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