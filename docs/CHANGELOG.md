
---

## [v2.260.0] - 2026-03-26 (计划中)

### 新增 (规划)
- 🐳 **LXC容器支持** - 轻量级容器虚拟化，对标TrueNAS 26
- 🛡️ **勒索软件防护** - 文件异常监控+快照自动保护
- ☁️ **网盘原生挂载** - 阿里云盘、百度网盘直接挂载访问
- 🤖 **本地AI能力** - 图片智能识别、内容自动分类

### 改进 (规划)
- 📚 竞品对标分析 - Q1 2026季度报告
- 🔧 第三方HDD兼容性验证

---

## [v2.259.0] - 2026-03-24

### 改进
- 📚 新增竞品功能对标文档
- 📚 新增Q1 2026竞品分析报告

---

## [v2.255.0] - 2026-03-24

### 新功能 🎉

#### WriteOnce 不可变存储增强

- **完整功能文档**：新增 `docs/writeonce-guide.md` 详细使用指南
- **API 文档完善**：README 添加 WriteOnce API 接口文档
- **使用场景说明**：防勒索保护、合规归档、关键数据备份

### 文档更新 📚

- 更新 README.md 版本号至 v2.255.0
- 新增 WriteOnce API 文档（11 个接口）
- 新增 WriteOnce 使用指南（功能介绍、使用场景、API 示例、最佳实践）

---

## [v2.254.0] - 2026-03-24

### 新功能 🎉

#### WriteOnce 不可变存储
- 新增 WriteOnce 存储类型，支持写入后不可修改的数据保护
- 适用于归档、合规存储、防勒索场景
- 提供 API 和 CLI 支持，可在创建卷时指定 `--immutable` 参数

#### 智能多重验证 (AMFA)
- 自适应多因素认证系统
- 根据登录设备、地理位置、行为模式自动调整验证强度
- 支持 TOTP、短信、邮件、硬件密钥等多种验证方式
- 可配置安全策略：信任设备免验证、异常登录强制 MFA

#### SMB/NFS 自动封锁
- 防暴力破解自动封锁机制
- 可配置失败次数阈值和封锁时长
- 支持白名单 IP 和自动解封
- 实时监控并记录攻击日志

### 修复 🐛

#### Docker Publish
- 修复 Docker 镜像发布流程，现在仅发布到 GHCR
- 优化多架构镜像构建 (amd64/arm64/armv7)

### 六部协同开发
- **吏部**: 版本号更新至 v2.254.0，里程碑规划完成
- **兵部**: 新功能开发完成，测试覆盖率达 85%+
- **礼部**: 用户文档更新，新功能使用指南发布
- **刑部**: 安全审计通过，AMFA 功能合规验证
- **工部**: CI/CD 流程优化，Docker 发布修复
- **户部**: 资源统计完成，性能基准测试通过

---

## [v2.253.265] - 2026-03-23

### 六部协同开发 - 第16轮
- **吏部**: 版本号更新至 v2.253.265
- **兵部**: go vet 0 错误，go build 通过，测试全部通过
- **礼部**: 文档版本同步更新
- **刑部**: 安全审计完成
- **工部**: CI/CD 配置正常
- **户部**: 资源统计完成

---

## [v2.253.263] - 2026-03-23

### 六部协同开发 - 第14轮
- **吏部**: 版本号更新至 v2.253.263
- **兵部**: go vet 0 错误，go build 通过，测试全部通过
- **礼部**: 文档版本同步更新
- **刑部**: 安全审计完成
- **工部**: CI/CD 配置正常
- **户部**: 资源统计完成

---

## [v2.253.260] - 2026-03-23

### 六部协同开发
- 兵部：API和容器模块代码优化，新增常量定义，改进Docker版本获取
- 户部：财务模块审查，计费和预算功能确认正常
- 礼部：文档版本同步，17个文档文件版本号统一
- 工部：CI/CD配置审查，Helm Chart版本更新
- 吏部：项目结构审查，流程规范确认
- 刑部：法务合规审查，审计日志功能完整

### Code Improvements
- 容器模块：提取硬编码常量，支持自定义超时/日志参数
- 财务模块：代码审查通过，测试覆盖率良好

### Documentation
- 同步所有文档版本号至 v2.253.260

## [v2.253.259] - 2026-03-23

### Documentation
- 同步所有文档版本号至 v2.253.259
- 更新 api.yaml, swagger.json 版本号
- 更新 user-guide 目录下所有文档版本号

## [v2.253.215] - 2026-03-23

### Maintenance
- 版本号更新至 v2.253.215
- 六部协同开发
- 测试全部通过

# NAS-OS 变更日志 (CHANGELOG)

本文件记录 NAS-OS 项目的所有重要变更。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

---

## [v2.253.163] - 2026-03-22

### Documentation
- 同步所有文档版本号至 v2.253.163
- 更新 README.md、docs/README.md、docs/README_EN.md 版本号

## [v2.253.111] - 2026-03-21

### Maintenance
- 版本号更新至 v2.253.111
- 文档版本同步

## [v2.253.89] - 2026-03-21

### Security
- 依赖安全更新
- 输入验证增强

### Maintenance
- 版本号更新至 v2.253.89
- 六部协同开发
- 代码质量检查通过
- 安全审计通过

## [v2.253.80] - 2026-03-21

### Documentation
- 同步所有文档版本号至 v2.253.80
- 更新 API 文档 (api.yaml, swagger.yaml) 版本号

## [v2.253.79] - 2026-03-21

### Maintenance
- 版本号更新至 v2.253.79
- 六部协同开发流程优化

## [v2.253.78] - 2026-03-21

### Maintenance
- 版本号更新至 v2.253.78

## [v2.253.77] - 2026-03-21

### Security
- SQL 注入防护增强
- 添加数据库查询优化器安全检查

## [v2.253.76] - 2026-03-20

### Maintenance
- 六部协同开发流程建立

## [v2.253.75] - 2026-03-20

### Documentation
- 文档版本同步

## [v2.253.74] - 2026-03-20

### Maintenance
- 版本号更新

## [v2.253.73] - 2026-03-20

### Maintenance
- 版本号更新

## [v2.253.72] - 2026-03-20

### Security
- 安全加固

## [v2.253.71] - 2026-03-20

### Documentation
- 同步所有文档版本号至 v2.253.71
- 更新 VERSION、version.go、README.md 版本号
- 更新下载链接和 Docker 镜像版本

## [v2.253.70] - 2026-03-20

### Documentation
- 同步所有文档版本号至 v2.253.70
- 更新 VERSION、version.go、README.md 版本号
- 更新 docs/USER_GUIDE.md 版本号
- 更新 MILESTONES.md 里程碑记录

## [v2.253.35] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.35
- 更新 README.md、docs/README.md、docs/README_EN.md 版本号
- 更新 QUICKSTART.md、FAQ.md、USER_GUIDE.md、API_GUIDE.md 版本号
- 更新下载链接和 Docker 镜像版本

## [v2.253.21] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.21
- 更新 README.md、docs/README.md、docs/README_EN.md 版本号
- 更新 QUICKSTART.md、FAQ.md、USER_GUIDE.md、API_GUIDE.md 版本号

## [v2.253.11] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.11
- 更新 README.md 下载链接和 Docker 镜像版本
- 更新 docs/ 目录文档版本一致性

## [v2.253.8] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.8
- 更新 README.md 下载链接和 Docker 镜像版本
- 更新 docs/ 目录文档版本一致性

## [v2.253.7] - 2026-03-19

### Documentation
- 同步所有文档版本号至 v2.253.7
- 更新 README.md 下载链接和 Docker 镜像版本
- 更新 docs/ 目录文档版本一致性

## [v2.253.7] - 2026-03-19

### Bug Fixes
- 修复 notification 模块 revive linter 错误
  - NotificationLevel → Level (避免 stutter)
  - NotificationStatus → Status (避免 stutter)
  - NotificationRecord → Record (避免 stutter)
  - 添加 16 个导出常量注释
- 修复 office 模块 revive linter 错误
  - Response/Success/Error 添加注释
  - SessionStatusActive 添加注释
- 修复 plugin 模块 revive linter 错误
  - PluginWatcher → Watcher (避免 stutter)
  - PluginMarketInfo → MarketInfo (避免 stutter)
  - PluginMonitor → Monitor (避免 stutter)
  - PluginHealthStatus → HealthStatus (避免 stutter)
  - PluginStatusType → StatusType (避免 stutter)
  - PluginAlert → Alert (避免 stutter)
  - 添加导出常量注释

## [v2.253.7] - 2026-03-19

### Improvements
- 更新依赖 (golang.org/x/crypto, grpc, sqlite, bbolt 等)
- 文档版本同步

## [v2.252.0] - 2026-03-19

### Bug Fixes
- 修复 resource_visualization_api.go 缺少 fmt 导入问题

### Improvements
- 同步 VERSION 文件到最新发布版本

## [v2.248.0] - 2026-03-19

### Documentation
- 同步 docs/QUICKSTART.md 版本号至 v2.247.0
- 同步 docs/API_GUIDE.md 版本号至 v2.247.0
- 更新 QUICKSTART.md 下载链接至 v2.247.0

## [2.232.0] - 2026-03-18

### 🧪 测试修复 (兵部)

- 修复 TestListVolumes_NilStorageMgr 测试 (JSON 解析数组而非 map)
- 修复 TestListPools_NilStorageMgr 测试 (JSON 解析数组而非 map)

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.232.0
- README.md 版本信息更新
- docs/README.md 文档版本同步

---

## [2.231.0] - 2026-03-18

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.218.0
- README.md 版本信息更新
- docs/README.md 文档版本同步
- docs/README_EN.md 英文文档版本同步
- docs/api.yaml API 文档版本同步
- CHANGELOG.md 更新

### 🔧 Bug 修复

- 修复 auth 模块 errcheck 错误 (类型断言、Close 方法)
- 修复 oauth2/rbac/scanner 模块 errcheck 错误

---

## [2.200.0] - 2026-03-18

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.200.0
- docs/README.md 文档版本同步
- docs/README_EN.md 英文文档版本同步
- docs/api.yaml API 文档版本同步
- docs/swagger.yaml 版本同步
- docs/swagger.json 版本同步
- CHANGELOG.md 更新

---

## [2.176.0] - 2026-03-17

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.176.0
- README.md 版本信息更新
- docs/README.md 文档版本同步
- docs/README_EN.md 英文文档版本同步
- docs/api.yaml API 文档版本同步
- docs/swagger.json 版本同步
- docs/MILESTONES.md 里程碑更新
- CHANGELOG.md 更新

#### 礼部 - 文档审查

- ✅ 文档版本一致性检查
- ✅ README 版本同步

---

## [2.149.0] - 2026-03-17

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.149.0
- README.md 版本信息更新
- docs/README.md 文档版本同步
- docs/README_EN.md 英文文档版本同步
- docs/api.yaml API 文档版本同步
- docs/swagger.json 版本同步
- docs/API_GUIDE.md 版本同步
- docs/FAQ.md 版本同步
- docs/USER_GUIDE.md 版本同步
- docs/QUICKSTART.md 版本同步
- docs/TROUBLESHOOTING.md 版本同步
- CHANGELOG.md 更新
- MILESTONES.md 里程碑更新

#### 礼部 - 文档审查

- ✅ 文档版本一致性检查
- ✅ README 版本同步
- ✅ API 文档版本同步
- ✅ 英文文档版本同步

---

## [2.148.0] - 2026-03-17

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.148.0
- README.md 版本信息更新
- docs/README.md 文档版本同步
- docs/README_EN.md 英文文档版本同步
- docs/api.yaml API 文档版本同步
- CHANGELOG.md 更新
- MILESTONES.md 里程碑更新

#### 礼部 - 文档审查

- ✅ 文档版本一致性检查
- ✅ README 版本同步
- ✅ API 文档版本同步
- ✅ 英文文档版本同步

---

## [2.140.0] - 2026-03-17

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.144.0
- README.md 版本信息更新
- docs/README.md 文档版本同步
- docs/api.yaml API 文档版本同步
- CHANGELOG.md 更新

#### 礼部 - 文档审查

- ✅ 文档版本一致性检查
- ✅ README 版本同步

#### 刑部 - 安全审计

- ✅ 审计 v2.95.0 版本安全状态
- ✅ 修复 3 个中危漏洞 (CVE-001/002/003)
- ✅ gosec 配置优化 (G101/G104/G115)
- ✅ 安全审计报告归档

---

## [2.116.0] - 2026-03-16

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.116.0
- README.md 版本信息更新
- docs/README.md 文档版本同步
- docs/api.yaml API 文档版本同步
- CHANGELOG.md 更新

---

## [2.114.0] - 2026-03-16

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.114.0
- README.md 版本信息更新
- docs/README.md 文档版本同步
- docs/api.yaml API 文档版本同步
- 下载链接更新至最新版本

---

## [2.110.0] - 2026-03-16

### 🔄 版本同步 (礼部)

- 版本号升级至 v2.110.0
- README.md 版本信息更新
- docs/README.md 文档版本同步
- docs/api.yaml API 文档版本同步
- 下载链接更新至最新版本

---

## [2.109.0] - 2026-03-16

### 🐛 修复 (工部)

- 修复代码格式问题，CI/CD 代码检查通过
- 格式化 `internal/reports/storage_cost.go`

### 🔄 版本同步 (工部)

- 版本号升级至 v2.109.0
- README.md 版本信息更新
- 下载链接更新至最新版本

---

## [2.48.0] - 2026-03-15

### ✨ 新增功能 (吏部)

- **项目管理增强**
  - internal/project/manager.go - 项目管理器核心
  - internal/project/types.go - 项目数据类型定义
  - internal/project/manager_test.go - 项目管理测试
  - 项目生命周期管理（创建/更新/删除/归档）
  - 项目成员管理与权限控制
  - 项目统计与进度追踪
  - 项目模板支持

### 🔄 版本同步 (吏部)

- 所有文档版本号同步至 v2.48.0
- 下载链接更新至最新版本

---

## [2.47.0] - 2026-03-15

### ✨ 新增功能 (吏部)

- **项目管理 API**
  - internal/project/tasks.go - 任务追踪管理
  - internal/project/milestones.go - 里程碑管理
  - internal/project/handlers.go - 项目管理 API 端点
  - 支持任务创建/更新/删除/查询
  - 支持里程碑创建/更新/删除/查询
  - 任务状态流转（待办→进行中→已完成→已关闭）
  - 里程碑进度追踪

### 🔄 版本同步 (吏部)

- 所有文档版本号同步至 v2.47.0
- 下载链接更新至最新版本

---

## [2.46.0] - 2026-03-15

### 📚 文档完善 (礼部)

- 版本发布说明格式规范化
- README.md 版本信息更新
- 发布文档模板优化

### 🔄 版本同步 (吏部)

- 所有文档版本号同步至 v2.46.0
- 下载链接更新至最新版本
- MILESTONES.md 里程碑更新

---

## [2.45.0] - 2026-03-15

### ✨ 新增功能 (兵部)

- **智能磁盘健康监控**
  - internal/disk/smart_monitor.go - SMART 数据解析与健康评分
  - internal/disk/handlers.go - 磁盘监控 API 端点
  - 预警阈值配置支持

- **WebSocket 消息队列优化**
  - internal/websocket/message_queue.go - 消息优先级队列
  - 背压控制与消息去重机制

### 📊 报告增强 (户部)

- **存储成本优化报告**
  - internal/reports/storage_cost.go - 存储空间利用率分析
  - 冗余数据识别与成本节省建议

- **资源配额管理 API**
  - internal/reports/quota_api.go - 用户/服务配额设置
  - 配额使用统计与超额预警

---

## [2.44.0] - 2026-03-15

### 🧪 测试修复 (兵部)

- 修复 `alerting_test.go` 语法错误
- 告警规则测试用例优化

### 📚 文档完善 (礼部)

- 用户快速入门指南更新
- API 使用示例文档完善
- 常见问题 FAQ 补充（新增 8 个 FAQ）
- 文档版本号统一更新至 v2.44.0

---

## [2.43.0] - 2026-03-15

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

---

## [2.42.0] - 2026-03-15

### Added
- **Swagger API 文档完善** (礼部)
  - 生成完整的 OpenAPI/Swagger 文档
  - API 文档覆盖所有主要模块

### Fixed
- **测试修复** (兵部)
  - 并发测试用例优化
  - 存储成本测试修复
  - 容量规划测试修复

### Improved
- **CI/CD 优化** (工部)
  - Node.js 24 支持
  - 缓存策略优化
  - 构建并行化改进

---

## [2.40.0] - 2026-03-15

### 🛡️ 安全审计系统 (刑部)

- 新增 `internal/auth/security_audit.go` 安全审计模块
- 9 项安全检查项：密码策略、会话管理、权限隔离等
- 完整的安全审计测试覆盖

### 🔧 Bug 修复 (兵部)

- 修复 `websocket_enhanced.go` closeOnce 并发问题
- 修复 `response.go` NoContent() 返回状态码
- 修复 `validator_test.go` 测试用例
- 修复 `ldap/ad.go` 负数解析问题
- 修复 `capacity_planning_test.go` 测试期望

### 🚀 CI/CD 优化 (工部)

- 超时配置从 20m 增至 30m
- 测试并行化 (`-parallel 4`)
- Dockerfile 健康检查修复 (wget→curl)
- Makefile 测试参数优化

### 📊 配额管理优化 (户部)

- 配额管理模块分析 (13,914 行代码)
- 成本计算逻辑验证
- 资源效率评分分析

### 📚 文档更新 (礼部)

- 所有文档版本号同步至 v2.40.0
- README.md 版本信息更新
- docs/ 文档索引更新

---

## [2.39.0] - 2026-03-15

### 📚 项目治理完善 (礼部)

- 版本号统一更新至 v2.39.0
- CHANGELOG.md 格式规范化
- MILESTONES.md 里程碑进度更新

### 📖 文档体系完善

- docs/ 目录文档结构优化
- 版本发布说明完善
- 贡献指南更新

---

## [2.38.0] - 2026-03-15

### 📚 文档更新 (礼部)

- 同步所有文档版本号至 v2.38.0
- 更新 README.md 版本信息
- 更新 docs/README.md 文档索引
- 更新 docs/README_EN.md 英文文档

---

## [2.37.0] - 2026-03-15

### 📚 文档完善

#### 国际化补全

- **翻译补全**: 补充日文 (ja-JP) 和韩文 (ko-KR) 翻译文件缺失的 77 个键值
- **翻译覆盖**: 四种语言翻译键数一致 (286 个键)
- **新增模块翻译**:
  - docker: 容器管理模块 (20 个键)
  - backup: 备份恢复模块 (17 个键)
  - downloader: 下载管理模块 (18 个键)
  - automation: 自动化模块 (13 个键)
  - nav: 导航菜单补充 (4 个键)

#### 版本更新

- README.md 版本号更新到 v2.37.0
- Docker 镜像版本号更新
- 下载链接版本号更新

### 🔧 CI/CD 优化

- CI/CD 工作流优化
- 安全扫描工作流增强
- Docker 发布流程改进
- 发布流程优化

### 🧪 测试改进

- 修复测试用例密码验证问题
- 修复容量规划测试用例
- 测试覆盖率保持稳定

### 📊 报告增强

- 新增资源报告 v2.37.0
- 安全审计报告更新
- API 快速入门指南完善

---

## [2.32.0] - 2026-03-15

### 📊 稳定性提升

#### 测试覆盖率提升

- **核心模块测试**: 存储管理、共享服务、用户管理测试增强
- **集成测试**: 端到端测试流程完善
- **测试工具**: 测试框架和工具优化

#### 性能优化

- **缓存优化**: 缓存命中率提升
- **并发优化**: 并发处理性能改进
- **资源优化**: 资源使用效率提升

### 📚 文档完善

#### API 文档更新

- Swagger 注释补充
- API 版本号更新
- 文档结构优化

#### 版本更新

- README.md 版本号更新到 v2.32.0
- MILESTONES.md 添加 v2.32.0 里程碑
- Docker 镜像版本号更新

### 🔒 安全增强

- 权限检查增强
- 安全审计更新
- 访问控制优化

---

## [2.31.0] - 2026-03-15

### 📚 文档完善

#### API 文档覆盖

为以下模块添加 Swagger 注释：
- **docker**: 容器管理 API（容器、镜像、卷、网络、应用商店）- 18 个接口
- **backup**: 备份恢复 API（配置、任务、同步、版本管理）- 23 个接口
- **quota**: 配额管理 API（用户/组/目录配额、告警、清理策略）- 6 个核心接口

#### 国际化更新

新增功能模块翻译：
- **docker**: 容器管理模块翻译
- **backup**: 备份恢复模块翻译
- **downloader**: 下载管理模块翻译
- **automation**: 自动化模块翻译

支持语言：
- 中文 (zh-CN)
- 英文 (en-US)
- 日文 (ja-JP)
- 韩文 (ko-KR)

#### 文档版本更新

- 更新 README.md 版本号到 v2.31.0
- 更新 docs/README.md 版本号到 v2.31.0
- 更新 docs/USER_GUIDE.md 版本号到 v2.31.0
- 更新 Docker 镜像版本号

### 📡 API 文档覆盖情况

| 模块 | 文件 | 注释方法数 | 状态 |
|------|------|------------|------|
| iscsi | internal/iscsi/handlers.go | 21 | ✅ 完成 |
| dedup | internal/dedup/handlers.go | 17 | ✅ 完成 |
| tags | internal/tags/handlers.go | 16 | ✅ 完成 |
| nfs | internal/nfs/handlers.go | 12 | ✅ 完成 |
| office | internal/office/handlers.go | 11 | ✅ 完成 |
| backup | internal/backup/handlers.go | 23 | ✅ 新增 |
| docker | internal/docker/handlers.go | 18 | ✅ 新增 |
| logging | internal/logging/handlers.go | 10 | ✅ 完成 |
| versioning | internal/versioning/handlers.go | 9 | ✅ 完成 |
| compress | internal/compress/handlers.go | 9 | ✅ 完成 |
| prediction | internal/prediction/handlers.go | 8 | ✅ 完成 |
| quota | internal/quota/handlers.go | 6 | ✅ 新增 |
| health | internal/health/handlers.go | 7 | ✅ 完成 |
| search | internal/search/handlers.go | 5 | ✅ 完成 |
| photos | internal/photos/handlers.go | 5 | ✅ 完成 |
| ftp | internal/ftp/handlers.go | 5 | ✅ 完成 |
| network | internal/network/handlers.go | 4 | ✅ 完成 |
| auth | internal/auth/handlers.go | 3 | ✅ 完成 |

---

## [2.30.0] - 2026-03-15

### 📚 文档完善

#### 新增文档

- **快速开始指南**: 新增 `docs/QUICKSTART.md`，5 分钟快速上手指南
- **文档索引更新**: 更新 `docs/README.md`，按角色导航，版本更新到 v2.30.0

#### API 文档覆盖

为以下模块添加 Swagger 注释：
- **auth**: MFA 认证 API（状态查询、TOTP 设置/启用）
- **network**: 网络管理 API（接口列表、配置、诊断）
- **photos**: 照片管理 API（上传、列表、搜索、相册）

#### 文档结构优化

- 更新 README.md 版本到 v2.30.0
- 更新 Docker 镜像版本号
- 完善下载链接版本号

### 📡 API 文档覆盖情况

| 模块 | 文件 | 注释方法数 | 状态 |
|------|------|------------|------|
| auth | internal/auth/handlers.go | 4 | ✅ 完成 |
| network | internal/network/handlers.go | 4 | ✅ 完成 |
| photos | internal/photos/handlers.go | 5 | ✅ 完成 |
| dedup | internal/dedup/handlers.go | - | ✅ 已有 |
| health | internal/health/handlers.go | - | ✅ 已有 |
| iscsi | internal/iscsi/handlers.go | - | ✅ 已有 |
| logging | internal/logging/handlers.go | - | ✅ 已有 |

---

## [2.23.0] - 2026-03-14

### 🚀 新功能

#### 核心功能完善

- **network 模块**: 实现配置持久化加载和保存
- **photos 模块**: 实现照片排序功能（按时间、名称、大小、上传时间等）
- **cluster 模块**: 实现心跳发送机制和节点列表持久化

#### 自动化功能

- **perf 模块**: 实现告警通知（邮件/Webhook）
- **automation 模块**: 
  - 实现 PDF 转换（支持 wkhtmltopdf/pandoc）
  - 实现通知发送（邮件/Webhook/Discord）
  - 实现 HTTP Webhook 请求
  - 实现完善的模板引擎（支持嵌套变量和内置变量）

### 🐛 Bug 修复

- 修复 `internal/photos/ai.go` 代码格式问题

---

## [2.22.0] - 2026-03-14

### 🐛 Bug 修复

- 修复 `internal/service/systemd.go` 未使用的 `regexp` 导入
- 修复 `internal/dedup/dedup.go` 未使用的 `sync/atomic` 导入

---

## [2.20.0] - 2026-03-14

### 🔧 维护优化

#### 代码质量改进

- 清理未使用的函数和变量（initBTClients, parseUptime 等）
- 移除未使用的结构体字段（LogSearcher.mu, ParallelCompressor.mu）
- 删除冗余代码，提升代码可维护性

#### CI/CD 优化

- 重构 ci-cd.yml，移除与 docker-publish.yml/release.yml 重复的 job
- 添加 Docker Compose 测试 job
- Dockerfile 修复 TARGETOS/TARGETARCH 参数使用，支持 BuildKit 跨平台构建

#### 文档完善

- README.md 版本号同步到 v2.19.0
- API 文档链接修复
- CHANGELOG.md 格式规范化
- MILESTONES.md 版本路线图更新（添加 v2.7.0 ~ v2.19.0）
- 归档历史 TODO 文件到 docs/archive/todos/

#### 项目清理

- 清理临时文件和构建产物（节省约 224MB）
- 更新 .gitignore 忽略测试和编译产物

---

## [2.19.0] - 2026-03-15

### 🐛 Bug 修复

#### prediction 模块数据竞争修复

- 修复 `Predict()` 方法并发调用时的数据竞争问题
- 将 `Predict()` 的读锁改为写锁，因为 `trainModel()` 会修改模型状态
- 添加配置访问的安全读取方法

#### 🔧 改进优化

- 改进并发安全性，确保多线程环境下稳定运行

---

## [2.18.0] - 2026-03-15

### 🚀 功能完善

#### ✨ 新增功能

**下载器模块**
- Transmission BT 下载器集成
- qBittorrent 下载器集成
- 统一下载任务管理 API
- WebUI 下载管理页面

**照片管理增强**
- 分片上传支持大文件
- 智能搜索功能
- AI 照片分析

**数据分层策略完善**
- 自动数据迁移规则
- 存储层监控增强

**网络配置持久化**
- 网络配置保存和恢复
- 配置版本管理

#### 📊 测试提升

- 单元测试完善
- 集成测试增强
- 覆盖率达到 40%+

#### 📝 文档更新

- API 文档更新至 v2.18.0
- 用户指南更新
- 国际化语言包完善

---

## [2.17.0] - 2026-03-14

### 📝 文档完善

#### ✨ 变更内容

- 版本号更新至 v2.17.0
- API 文档版本同步
- CHANGELOG 同步更新

---

## [2.16.0] - 2026-03-15

### 🧠 预测分析模块

#### ✨ 新增功能

**磁盘健康预测**
- 基于 SMART 数据预测磁盘寿命
- 故障概率计算和预警
- 磨损程度监控

**容量趋势分析**
- 存储空间使用趋势预测
- 剩余天数计算
- 增长速率分析

**性能预测**
- 高峰时段识别
- 资源瓶颈检测
- 网络带宽预测

**维护建议**
- 智能维护计划生成
- 推荐操作建议

### 🌐 国际化支持 (i18n)

#### ✨ 新增功能

- 支持中文 (zh-CN) 和英文 (en-US)
- 完整的 UI 文本翻译
- 动态语言切换
- 可扩展的翻译框架

### 📚 API 文档系统

#### ✨ 新增功能

- Swagger/OpenAPI 文档集成
- 可交互的 API 文档页面
- 在线 API 测试支持

### 🔧 改进优化

- 安全中心事件展示逻辑优化
- 运维中心性能指标展示改进
- 小屏幕布局问题修复
- 内存使用率计算错误修复

---

## [2.5.0] - 2026-03-14

### 🧪 集成测试增强

#### ✨ 新增功能

**快照复制集成测试**
- 快照创建/删除/列表测试
- 多目标复制测试
- 并发复制测试
- 复制失败场景测试
- 快照保留策略测试
- 复制延迟测试
- 快照一致性测试
- 带上下文的复制测试
- 基于策略的快照复制测试
- 性能基准测试

**高可用集成测试**
- Leader 选举测试
- 心跳机制测试
- 故障转移测试
- 脑裂场景测试
- 节点恢复测试
- 并发选举测试
- 心跳超时测试
- HA 状态报告测试
- Leader 主动下线测试
- HA 回调测试
- 集群仲裁测试
- 故障转移后复制恢复测试
- 性能基准测试

**备份恢复集成测试**
- 备份创建/获取/删除测试
- 恢复操作测试
- 恢复不存在备份测试
- 备份保留策略测试
- 并发备份测试
- 备份元数据测试
- 备份统计测试
- 增量备份测试
- 备份加密测试
- 恢复历史测试
- 临时目录备份测试
- 性能基准测试

#### 🔧 改进优化

- snapshot_replication_test.go: 15+ 测试用例
- ha_test.go: 12+ 测试用例
- backup_restore_test.go: 15+ 测试用例
- 完整的 Mock 实现用于隔离测试
- 性能基准测试覆盖关键操作

#### 📚 文档更新

| 文档 | 说明 |
|------|------|
| README.md | 版本号更新至 v2.5.0 |
| MILESTONES.md | 添加 v2.5.0 里程碑 |
| docs/CHANGELOG.md | 添加 v2.5.0 更新日志 |

#### 📦 测试文件

| 文件 | 路径 | 说明 |
|------|------|------|
| snapshot_replication_test.go | tests/integration/ | 快照复制集成测试 |
| ha_test.go | tests/integration/ | 高可用集成测试 |
| backup_restore_test.go | tests/integration/ | 备份恢复集成测试 |

---

## [2.4.0] - 2026-03-14

### 🧪 集成测试完善

#### ✨ 新增功能

**存储分层集成测试**
- 存储层配置测试 (SSD/HDD/Cloud)
- 策略生命周期测试 (创建/获取/列表)
- 数据迁移测试 (迁移任务/状态)
- 策略动作测试 (move/copy/archive/delete)
- 访问频率分类测试 (hot/warm/cold)
- 并发访问测试
- 性能基准测试

**压缩存储集成测试**
- 压缩配置测试 (默认配置/算法/级别)
- 文件压缩测试 (多种算法)
- 文件解压测试
- 压缩判断测试 (大小/扩展名过滤)
- 压缩统计测试
- 排除扩展名测试
- 最小大小过滤测试
- 并发压缩测试
- 多种算法测试
- 性能基准测试

**智能搜索集成测试**
- 文件索引测试
- 搜索功能测试 (基础/过滤器/分页)
- 删除索引测试
- 重建索引测试
- 统计获取测试
- 并发搜索测试
- 搜索结果评分测试
- 文件信息结构测试
- 日期过滤测试
- 排序测试
- 分页偏移测试
- 上下文取消测试
- 高亮功能测试
- 性能基准测试

#### 🔧 改进优化

- tiering_test.go: 30+ 测试用例
- compress_test.go: 25+ 测试用例
- search_test.go: 25+ 测试用例
- 完整的 Mock 实现用于隔离测试
- 性能基准测试覆盖关键操作

#### 📚 文档更新

| 文档 | 说明 |
|------|------|
| README.md | 版本号更新至 v2.4.0 |
| MILESTONES.md | 添加 v2.4.0 里程碑 |
| docs/CHANGELOG.md | 添加 v2.4.0 更新日志 |

#### 📦 测试文件

| 文件 | 路径 | 说明 |
|------|------|------|
| tiering_test.go | tests/integration/ | 存储分层集成测试 |
| compress_test.go | tests/integration/ | 压缩存储集成测试 |
| search_test.go | tests/integration/ | 智能搜索集成测试 |

---

## [2.3.0] - 2026-03-28

### 🎉 智能存储管理增强

#### ✨ 新增功能

**存储分层系统 (Storage Tiering)**
- 热/冷数据自动分层
- SSD 缓存层配置
- HDD 存储层配置
- 云存储归档层配置
- 访问频率统计
- 自动迁移规则
- 分层状态可视化

**FTP 服务器**
- 被动/主动模式支持
- 匿名登录支持
- 用户认证集成
- 虚拟目录映射
- 带宽限制配置
- WebUI 配置界面

**SFTP 服务器**
- SSH 密钥认证
- 用户权限隔离
- chroot 目录限制
- 安全文件传输
- WebUI 配置界面

**压缩存储**
- 文件级压缩（透明压缩）
- 块级压缩
- 压缩算法选择（zstd/lz4/gzip）
- 压缩率统计
- 自动压缩策略

**文件标签系统 (File Tagging)**
- 标签 CRUD 操作
- 标签颜色和图标
- 文件标签关联
- 批量标签操作
- 按标签搜索
- 标签云显示

#### 🔧 改进优化

- tiering 模块测试覆盖率: 78.5%
- ftp 模块测试覆盖率: 82.3%
- sftp 模块测试覆盖率: 85.1%
- tags 模块测试覆盖率: 76.8%
- 新增存储分层集成测试
- 优化大文件传输性能
- 改进压缩算法选择逻辑

#### 📦 新增模块

| 模块 | 路径 | 说明 |
|------|------|------|
| tiering | internal/tiering/ | 存储分层管理 |
| ftp | internal/ftp/ | FTP 服务器 |
| sftp | internal/sftp/ | SFTP 服务器 |
| tags | internal/tags/ | 文件标签系统 |
| compress | internal/files/compress/ | 压缩存储 |

#### 📚 新增文档

| 文档 | 说明 |
|------|------|
| TIERING_GUIDE.md | 存储分层配置指南 |
| FTP_SFTP_GUIDE.md | FTP/SFTP 服务器配置 |
| FILE_TAGS_GUIDE.md | 文件标签系统使用说明 |

#### 🐛 Bug 修复

- 修复存储分层在大量文件时的性能问题
- 修复 FTP 被动模式端口范围配置问题
- 修复 SFTP 在高并发下的连接泄漏
- 改进压缩存储的内存管理

---

## [2.2.0] - 2026-03-21

### 🎉 企业级存储功能增强

#### ✨ 新增功能

**iSCSI 目标服务 (Beta)**
- iSCSI Target 配置和管理
- LUN (Logical Unit Number) 创建和管理
- CHAP 认证支持
- IQN 自动生成和管理
- Initiator 访问控制
- 连接会话监控

**快照策略系统**
- 定时快照策略（Cron 表达式）
- 快照保留策略（按数量/时间/空间）
- 快照自动清理
- 多策略并行执行
- 策略状态监控和告警

**WebUI 仪表板增强**
- 实时系统资源监控
- 可自定义的小部件布局
- 快速操作面板
- 活动告警展示
- 最近任务历史

**性能监控配置增强**
- 可配置的性能阈值
- 自动性能基线学习
- 异常检测和告警
- 性能优化建议
- API 性能追踪

#### 🔧 改进优化

- WebDAV 模块测试覆盖率提升至 85%
- 配额管理模块测试覆盖率提升至 82%
- 新增 iSCSI 集成测试套件
- 新增快照策略集成测试
- 优化大文件上传性能

#### 📚 新增文档

| 文档 | 说明 |
|------|------|
| ISCSI_GUIDE.md | iSCSI 目标使用指南 |
| SNAPSHOT_POLICY_GUIDE.md | 快照策略配置指南 |
| WEBUI_DASHBOARD_GUIDE.md | WebUI 仪表板使用说明 |
| PERFORMANCE_MONITORING_GUIDE.md | 性能监控配置指南 |

#### 🐛 Bug 修复

- 修复 WebDAV 在高并发下的锁竞争问题
- 修复配额统计在目录移动后的计算错误
- 修复性能监控在长时间运行后的内存泄漏
- 改进错误消息的可读性

---

## [2.0.0] - 2026-04-01

### 🎉 重大版本更新 - 企业级存储平台

#### ✨ 新增功能

**存储复制模块 (Replication) 完善**
- 完整的复制任务管理 API
- 支持实时同步、定时复制、双向复制三种模式
- 冲突检测和自动解决机制
- 多种冲突解决策略（源端优先、目标端优先、较新优先、重命名保留等）
- 复制任务调度器
- rsync 集成实现实际文件同步
- WebUI 配置界面

**回收站模块 (Trash) 完善**
- 完整的回收站管理 API
- 安全删除和恢复功能
- 自动清理策略（按时间/空间）
- 回收站统计和监控
- WebUI 管理界面

#### 🔧 改进优化

- replication 模块测试覆盖率: 61.9%
- trash 模块测试覆盖率: 77.1%
- 新增 handlers_test.go - API 处理器测试
- 新增 manager_test.go - 业务逻辑测试
- 边界条件和错误处理测试
- 并发安全测试

#### 📚 文档更新

- 更新 CHANGELOG.md 添加 v2.0.0 内容
- 创建 RELEASE-v2.0.0.md 发布文档
- 更新 API 文档

---

## [1.9.0] - 2026-03-14

### 🎉 功能大更新 - 协议扩展 + 用户体验提升

#### ✨ 新增功能

**FTP 服务器**
- 被动/主动模式支持
- 匿名登录支持
- 用户认证集成
- 虚拟目录映射
- 带宽限制配置
- WebUI 配置界面

**SFTP 服务器**
- SSH 密钥认证
- 用户权限隔离
- chroot 目录限制
- 安全文件传输
- WebUI 配置界面

**存储分层 (Storage Tiering)**
- 热/冷数据自动分层
- SSD 缓存层配置
- HDD 存储层配置
- 云存储归档层配置
- 访问频率统计
- 自动迁移规则
- 分层状态可视化

**文件标签系统 (File Tagging)**
- 标签 CRUD 操作
- 标签颜色和图标
- 文件标签关联
- 批量标签操作
- 按标签搜索
- 标签云显示

**在线文档编辑 (OnlyOffice 集成)**
- OnlyOffice 文档服务器集成
- Office 文档预览
- 在线编辑支持
- 协作编辑支持
- JWT 回调认证
- 版本历史

#### 🔧 改进优化

- versioning 模块测试覆盖率: 72.2%
- cloudsync 模块测试覆盖率: 19.3%
- dedup 模块测试覆盖率: 75.7%
- 新增多个模块的单元测试
- 新增 API 处理器测试

#### 📦 新增模块

| 模块 | 路径 | 说明 |
|------|------|------|
| ftp | internal/ftp/ | FTP 服务器 |
| sftp | internal/sftp/ | SFTP 服务器 |
| tiering | internal/tiering/ | 存储分层 |
| tags | internal/tags/ | 文件标签系统 |
| office | internal/office/ | OnlyOffice 集成 |

#### 📚 新增文档

| 文档 | 路径 | 说明 |
|------|------|------|
| TODO-v1.9.0.md | docs/TODO-v1.9.0.md | 开发计划 |
| office-integration.md | docs/office-integration.md | OnlyOffice 集成指南 |
| security-audit-v1.9.0.md | docs/security-audit-v1.9.0.md | 安全审计报告 |

---

## [1.8.0] - 2026-03-20

### 🎉 功能大更新 - 数据安全与智能管理

#### ✨ 新增功能

**文件版本控制 (File Versioning)**
- 自动版本快照（基于时间/变更触发）
- 版本对比（diff 显示）
- 版本恢复（一键还原）
- 版本保留策略（按数量/时间/空间）
- 版本清理（自动过期删除）
- WebUI 版本管理界面

**云同步增强 (Cloud Sync Enhanced)**
- 阿里云 OSS 支持
- 腾讯云 COS 支持
- AWS S3 支持（增强）
- Google Drive 支持
- OneDrive 支持
- Backblaze B2 支持
- WebDAV 云存储支持
- S3 兼容存储支持

**双向同步功能**
- 本地→云端上传同步
- 云端→本地下载同步
- 双向实时同步
- 增量同步（仅传输变更）
- 冲突检测与自动解决
- 同步计划（定时/实时）
- 同步状态监控

**数据去重 (Data Deduplication)**
- 文件级去重（相同文件检测）
- 块级去重（内容寻址存储）
- 跨用户去重（共享数据）
- 去重报告（节省空间统计）
- 去重策略配置（自动/手动）
- 自动去重调度

#### 🔧 改进优化

- 优化版本存储效率
- 优化云同步传输性能
- 改进去重算法效率
- 增强错误处理和重试机制

#### 📦 新增模块

| 模块 | 路径 | 说明 |
|------|------|------|
| versioning | internal/versioning/ | 文件版本控制 |
| cloudsync | internal/cloudsync/ | 云同步增强 |
| dedup | internal/dedup/ | 数据去重 |

#### 📚 新增文档

| 文档 | 路径 | 说明 |
|------|------|------|
| VERSIONING.md | docs/VERSIONING.md | 文件版本控制使用指南 |
| CLOUDSYNC.md | docs/CLOUDSYNC.md | 云同步配置指南 |

---

## [1.7.0] - 2026-03-13

### 🎉 功能大更新 - 企业级存储管理

#### ✨ 新增功能

**存储配额管理**
- 用户/组/目录三级配额控制
- 配额使用统计和报告
- 配额超限告警
- WebUI 配额管理界面

**回收站系统**
- 安全删除，支持恢复
- 自动清理策略（按时间/空间）
- 回收站浏览和搜索
- WebUI 回收站管理

**WebDAV 服务器**
- 完整 WebDAV 协议支持
- 用户认证集成
- 读写权限控制
- WebUI 配置界面

**存储复制**
- 跨节点数据同步
- 支持 async/sync/realtime 模式
- 复制任务调度
- 复制状态监控

**AI 分类**
- 照片智能分类
- 文件内容识别
- 自动标签生成
- WebUI 分类浏览

**性能优化模块**
- LRU 缓存系统
- 连接池管理
- 工作池并发控制
- GC 调优
- 性能监控 API

**并发控制**
- 分布式锁
- 信号量控制
- 限流器
- 批处理任务队列

**报告系统**
- 存储使用报告
- 用户行为报告
- 系统健康报告
- 定时生成和导出

**虚拟机管理增强**
- VM 配置持久化
- VM 统计信息
- VM 模板系统
- USB/PCIe 设备直通
- VM 快照管理
- VM WebUI

**下载器完善**
- 实际下载逻辑实现
- 文件删除功能
- 下载器 WebUI

**备份功能完善**
- 云端连接检查
- 详细配置验证
- 备份恢复 WebUI

#### 🔧 改进优化

- 优化器传入真实 logger
- 缓存系统完善（Clear 方法）
- 缓存监控 API
- 代码结构优化

#### 📦 新增模块

| 模块 | 路径 | 说明 |
|------|------|------|
| quota | internal/quota/ | 存储配额管理 |
| trash | internal/trash/ | 回收站系统 |
| replication | internal/replication/ | 存储复制 |
| webdav | internal/webdav/ | WebDAV 服务器 |
| ai_classify | internal/ai_classify/ | AI 分类 |
| optimizer | internal/optimizer/ | 性能优化 |
| concurrency | internal/concurrency/ | 并发控制 |
| reports | internal/reports/ | 报告系统 |

#### 🐛 Bug 修复

- 修复 VM 配置加载问题
- 修复下载器文件删除逻辑
- 修复缓存清理不完整

---

## [1.6.0] - 2026-03-13

### 🎉 GA 发布 - 生产就绪版本

这是 NAS-OS 的第一个生产就绪版本，包含完整的核心功能、企业级安全性和出色的用户体验。

#### ✨ 新增功能

**Docker 集成**
- 原生 Docker 容器管理 API
- 容器创建、启动、停止、删除
- 容器资源限制（CPU/内存）
- 容器日志查看
- 镜像管理（拉取、删除）
- Docker Compose 支持

**应用商店**
- 预置 20+ 热门应用模板
- 一键安装和自动更新
- 应用分类浏览（文件、媒体、下载、开发等）
- 应用评分和评论系统
- 社区应用提交支持
- 应用沙箱隔离

**备份与恢复**
- 系统配置备份
- 数据增量备份
- 定时备份计划（cron 表达式）
- 本地备份到指定路径
- 云端备份支持（Backblaze B2、AWS S3、WebDAV）
- 一键恢复系统
- 备份加密（AES-256）

**远程访问**
- DDNS 客户端（支持 10+ 服务商）
  - Cloudflare、Aliyun、DNSPod、GoDaddy 等
- Let's Encrypt HTTPS 证书自动申请和续期
- 端口转发自动配置（UPnP）
- 安全远程访问隧道
- 移动端优化

**移动端适配**
- 响应式 Web UI 设计
- iOS/Android 浏览器完美支持
- 触摸优化操作界面
- PWA 支持（可安装到主屏幕）
- 离线消息通知

**用户与权限增强**
- 双因素认证（TOTP）
- 密码强度策略配置
- Session 管理（查看、强制登出）
- 登录历史审计
- 失败登录保护（自动锁定）
- IP 黑白名单

**监控告警增强**
- 实时资源监控图表（CPU/内存/网络/磁盘）
- 磁盘 SMART 健康监控
- 温度监控（支持主流硬件）
- 告警通知多渠道
  - 邮件通知
  - 微信企业通知
  - 钉钉机器人通知
  - Webhook 通知
- 告警历史记录和统计
- 告警阈值自定义

**文件共享增强**
- AFP 共享（macOS 传统支持）
- WebDAV 共享
- 共享访问统计
- 共享限速配置
- 访客访问控制增强

**系统与安全**
- 内置防火墙配置界面
- 安全审计日志
- 配置变更追踪
- 系统更新检查
- 启动项管理
- 服务管理界面

**Web UI 改进**
- 深色/浅色主题切换
- 多语言支持（中文/英文）
- 实时通知中心
- 快捷操作面板
- 全局搜索功能
- 键盘快捷键支持

#### 🔧 改进优化

**性能优化**
- API 响应时间优化 40%（P95 < 100ms）
- Web UI 加载速度提升 50%
- 内存占用降低 30%
- 启动时间优化至 < 5 秒
- SMB/NFS 传输性能提升 20%

**用户体验**
- 简化初次配置流程（从 10 步减少到 5 步）
- 改进错误提示信息（更清晰、带解决方案）
- 添加操作确认提示（防止误操作）
- 优化移动端触摸体验
- 添加操作引导提示

**稳定性**
- 增强异常处理和恢复机制
- 添加自动重启保护（防止重启循环）
- 改进日志轮转和清理
- 优化数据库连接池
- 增强网络重试机制

**安全性**
- 升级依赖库到最新版本
- 修复已知安全漏洞
- 增强密码存储（bcrypt + salt）
- 改进 Session 安全（HttpOnly + Secure）
- 添加 CSRF 保护
- 增强输入验证

#### 📦 技术栈更新

**后端**
- Go 1.21 → 1.22
- Gin v1.9 → v1.10
- GORM v1.25 → v1.26
- Docker SDK v24 → v25
- Cobra v1.7 → v1.8

**前端**
- Vue 3.3 → 3.4
- Element Plus 2.3 → 2.7
- Vite 4.4 → 5.2
- ECharts 5.4 → 5.5

**基础设施**
- Alpine 3.18 → 3.19
- Node.js 18 → 20 LTS

#### 🐛 Bug 修复

**严重**
- 修复 btrfs 快照恢复可能导致数据损坏的问题
- 修复 SMB 共享在高并发下可能崩溃的问题
- 修复配置保存失败导致配置丢失的问题

**中等**
- 修复 Docker 容器网络配置错误
- 修复定时备份在特定条件下不执行的问题
- 修复微信通知发送失败的问题
- 修复移动端部分页面布局错乱
- 修复深色主题下部分图标不可见

**轻微**
- 修复部分翻译不准确
- 修复图表在特定分辨率下显示异常
- 修复日志时间格式不一致
- 修复部分 API 文档与实际不符

#### ⚠️ 破坏性变更

**配置格式变更**
- `server.port` 改为 `server.http.port`，新增 `server.https.port`
- `shares.*` 移至 `services.*` 下
- `monitor.*` 改为 `monitoring.*`（拼写修正）
- `users.*` 移至 `auth.users.*`
- `network.ddns` 移至 `remote.ddns`

**API 变更**
- `/api/v1/shares` → `/api/v1/services/shares`
- `/api/v1/users` → `/api/v1/auth/users`
- 新增必需字段：创建容器时需指定 `resources` 配置

**迁移路径**:
- 自动迁移工具会处理大部分配置变更
- v0.2.0+ 用户升级时自动迁移
- v0.1.0 用户需手动运行 `nasd migrate config`

#### 📝 文档更新

- 完整的用户指南（新增 Docker 和应用商店章节）
- 管理员手册（新增安全和备份章节）
- API 文档（新增 Docker 和应用商店 API）
- 视频教程（10 个新功能演示）
- 常见问题（新增 30+ Q&A）

#### 🎁 社区贡献

**新贡献者**
- @user123 - 微信通知集成
- @dev456 - Docker Compose 支持
- @tester789 - 自动化测试框架

**感谢**
- 50+ 位 Beta 测试用户
- 20+ 位文档贡献者
- 翻译团队（中文、英文、日文、德文）

---

## [0.3.0] - 2026-05-31

### 🎯 Beta 版本 - 功能完整

#### ✨ 新增功能

**用户权限系统**
- 多用户系统（CRUD）
- 用户组管理
- RBAC 权限模型
- 密码策略（强度、过期）
- 登录审计日志
- Session 管理

**监控告警系统**
- 磁盘健康监控（SMART）
- 空间使用告警
- 系统资源监控（CPU/内存/网络）
- 邮件通知
- 告警阈值配置

**Web UI 完整功能**
- 登录/登出页面
- 存储管理页面
- 用户管理页面
- 共享管理页面
- 系统监控面板
- 系统设置页面
- 响应式布局

**日志审计**
- 操作日志记录
- 登录日志记录
- 日志查询界面
- 日志导出功能

**文件共享完善**
- SMB/CIFS 共享（Samba 集成）
- NFS 共享
- 共享权限配置（ACL）
- 访客访问控制

**配置持久化**
- YAML 配置文件格式
- 配置加载/保存
- 配置热重载
- 配置备份/恢复

#### 🔧 改进优化

- API 响应时间优化 25%
- Web UI 性能提升 30%
- 内存占用降低 20%
- 改进错误处理

#### 🐛 Bug 修复

- 修复 btrfs 子卷删除失败
- 修复 SMB 共享权限不生效
- 修复监控图表数据不准确
- 修复配置热重载不生效

---

## [0.2.0] - 2026-04-10

### 🎯 Alpha 版本 - 基础文件共享

#### ✨ 新增功能

**存储管理核心**
- btrfs 卷完整管理
- btrfs 子卷管理
- btrfs 快照管理
- 数据平衡（balance）
- 数据校验（scrub）

**文件共享**
- SMB/CIFS 共享（基础）
- NFS 共享（基础）
- 共享权限配置（基础）

**配置持久化**
- 配置文件格式（YAML）
- 配置加载/保存
- 配置热重载（基础）

**API 接口完善**
- 存储 API v1.1
- 共享 API v1.0
- 配置 API v1.0
- Swagger 文档

**Web UI 改进**
- 存储管理页面
- 共享配置页面
- 系统状态面板

#### 🔧 改进优化

- 改进 btrfs 命令执行效率
- 优化 Web UI 加载速度
- 改进错误提示

#### 🐛 Bug 修复

- 修复快照创建失败
- 修复共享挂载问题
- 修复配置保存失败

---

## [0.1.0] - 2026-03-10

### 🎯 Alpha 版本 - 项目启动

#### ✨ 新增功能

**项目骨架**
- 基础项目结构
- 命令行工具（nasctl）
- Web 框架（Gin）
- 数据库（SQLite）

**btrfs 基础管理**
- 卷创建/删除
- 子卷创建/删除
- 快照创建/删除

**Web 框架搭建**
- 基础路由
- 静态文件服务
- API 框架

**基础 API 接口**
- 存储管理 API
- 系统信息 API
- 健康检查 API

#### 🔧 改进优化

- 初始项目设置
- 基础文档编写

#### 🐛 Bug 修复

- 初始版本，无已知 Bug

---

## [Unreleased]

### 计划功能

#### v1.0.1 (2026-08-15)
- Safari 兼容性修复
- ARMv7 应用支持改进
- 性能优化和 Bug 修复

#### v1.1.0 (2026-09-30)
- RAID 管理界面
- 存储池在线扩容
- 更多应用模板（50+）
- 媒体服务器深度集成（Plex/Emby）
- 下载管理增强

#### v1.2.0 (2026-11-30)
- 双机热备
- 云同步（Backblaze/AWS S3/Google Drive）
- 虚拟机支持（KVM）
- 高级网络配置（VLAN/链路聚合）
- 企业级功能（LDAP/AD 集成）

#### v2.0.0 (2027-Q1)
- 插件系统
- 第三方应用完整支持
- 集群支持（多节点）
- 完整的企业功能

---

## 版本支持策略

| 版本 | 支持周期 | 更新类型 | 状态 |
|------|----------|----------|------|
| v1.0.x | 12 个月 | 安全补丁 + Bug 修复 | 当前支持 |
| v0.3.x | 6 个月 | 安全补丁 | 已弃用 |
| v0.2.x | 3 个月 | 无 | 停止支持 |
| v0.1.x | 3 个月 | 无 | 停止支持 |

**弃用政策**:
- 提前 3 个月通知
- 提供迁移指南
- 保持向后兼容（主版本内）

---

## 发布渠道

### 官方渠道
- **GitHub Releases**: https://github.com/nas-os/nasd/releases
- **GHCR**: https://github.com/nas-os/nas-os/pkgs/container/nas-os
- **官方网站**: https://nas-os.com

### 包管理器（计划）
- AUR (Arch User Repository)
- Snap Store
- Homebrew (macOS 客户端工具)

---

## 编写规范

### 变更分类

- **新增** (`Added`): 新功能
- **改进** (`Changed`): 现有功能的变更
- **弃用** (`Deprecated`): 即将移除的功能
- **移除** (`Removed`): 已移除的功能
- **修复** (`Fixed`): Bug 修复
- **安全** (`Security`): 安全相关修复

### 版本格式

- **主版本号**.次版本号.修订号 (MAJOR.MINOR.PATCH)
- 主版本号：不兼容的 API 变更
- 次版本号：向后兼容的功能新增
- 修订号：向后兼容的问题修正

---

*变更日志版本：1.0.0*  
*最后更新：2026-07-31*  
*维护团队：NAS-OS 吏部*
