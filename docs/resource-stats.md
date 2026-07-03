# 📊 NAS-OS 项目资源统计报告

> **统计日期**: 2026-07-03
> **统计版本**: v3.9.0 Stable
> **最新提交基线**: `07869417` (`feat: prepare v3.9.0 security and operations updates`)
> **当前状态**: v3.9.0 文档、版本号、安全与运维体验增强已同步；本轮补齐 CHANGELOG、README、竞品分析与资源统计。

---

## 一、代码规模概览

| 指标 | 数值 |
|------|------|
| **Git 跟踪文件总数** | 3,977 个 |
| **Git 跟踪内容体积** | 45.5 MB |
| **工作区总占用** | 658 MB |
| **项目总行数（Git 跟踪）** | 1,789,117 行 |
| **Go 源码总行数** | 1,631,831 行 |
| **源代码行数（不含测试）** | 1,213,178 行 |
| **测试代码行数** | 418,653 行 |
| **测试代码占比** | 25.7% |

---

## 二、文件与模块统计

| 指标 | 数值 |
|------|------|
| **Go 文件总数** | 3,662 个 |
| **Go 源代码文件** | 2,703 个 |
| **Go 测试文件** | 959 个 |
| **internal/ 顶层模块数量** | 736 个 |
| **cmd/ 应用数量** | 3 个（backup、nasctl、nasd） |
| **Dockerfile 数量** | 7 个 |
| **docker-compose 配置数量** | 10 个 |
| **直接依赖** | 48 个 |
| **间接依赖** | 224 个 |
| **依赖合计** | 272 个 |

### 主要语言/文件类型分布

| 类型 | 文件数 | 行数 |
|------|------:|------:|
| Go | 3,662 | 1,631,831 |
| HTML | 63 | 54,245 |
| Shell | 50 | 24,508 |
| JSON | 19 | 18,417 |
| YAML | 24 | 16,472 |
| YML | 54 | 11,163 |
| JavaScript | 13 | 8,197 |
| Markdown | 21 | 7,648 |
| CSS | 9 | 6,495 |
| TSX | 12 | 4,550 |
| TypeScript | 14 | 1,756 |

---

## 三、依赖资源概览

直接依赖当前为 **48** 个，核心资源域包括：

- **Web/API**：Gin、Gorilla mux/websocket、Swagger。
- **存储/文件系统**：FUSE、WebDAV、SQLite、AWS S3 SDK、Excelize、imaging/exif。
- **AI/搜索**：Bleve、Goldmark、本地语义搜索相关模块。
- **安全/认证**：JWT、LDAP、OTP、x/crypto。
- **监控/可观测**：Prometheus、OpenTelemetry、gopsutil、zap。
- **运维/CLI**：Cobra、cron、fsnotify、YAML。
- **网络**：QUIC、zeroconf、x/sys、x/time。

依赖规模：**直接 48 / 间接 224 / 合计 272**。本轮文档同步不新增运行时代码路径，但统计口径已按当前 `go list -m all` 刷新。

---

## 四、本轮功能价值与资源成本摘要

本轮变更集中在 `CHANGELOG.md`、`README.md`、`docs/competitive-analysis.md`、`docs/competitive-analysis-2026-06-29.md`、`docs/resource-stats.md` 与 v3.9.0 版本标识，补齐安全与运维体验增强的发布叙事。

### 功能/市场价值

- **CHANGELOG 补齐**：新增 v3.9.0 条目，覆盖 Log Center 重启原因、WebShare、登录/用户管理体验、竞品分析与资源统计。
- **README 同步**：顶部版本更新到 v3.9.0，并新增 v3.9.0 安全与运维体验增强模块说明。
- **竞品对标更完整**：新增/强化 Synology DSM 7.4、TrueNAS 26、飞牛 fnOS、QNAP QuTS hero h6.0 对照。
- **价值表达更清晰**：README 从“四大独家功能”调整为“五大差异化能力”，覆盖本地 AI、多云挂载、MCP、不可变存储等长期差异化。
- **v3.8.0 能力沉淀**：网络 FEC、本地 AI 语义搜索治理、LXC 迁移计划已在文档中形成可传播说明；v3.9.0 进一步补强重启原因运维闭环和 WebShare 体验。

### 资源/成本评估

| 项目 | 评估 |
|------|------|
| **代码增量成本** | 低。本轮主要是文档、版本标识与轻量前端/Log Center 增强。 |
| **依赖成本** | 低。本轮文档修订不新增第三方依赖。 |
| **维护成本** | 中。文档承诺覆盖多个竞品方向，后续需要保持 README、CHANGELOG、竞争分析与实际模块一致。 |
| **测试成本** | 低到中。已验证 `internal/logcenter` 与 `internal/version`，发布前可按需补全全量测试。 |
| **产品收益** | 高。以较低工程成本强化 v3.9.0 的安全、运维、竞品对标和市场叙事。 |

---

## 五、重点资源域

- **智能运维**：`internal/network`、`internal/logcenter`、`internal/lxc` 等模块支撑高速网络、重启复盘与容器迁移计划。
- **本地 AI/搜索治理**：`internal/semanticsearch` 与 AI 相关模块支撑 local-only 查询、脱敏返回和用途审计。
- **不可变与合规**：`internal/wormcomply`、`internal/objectimmutable`、`internal/immutablebackup`、快照相关模块覆盖勒索防护与合规归档。
- **存储效率与分层**：`internal/tiering`、`internal/tieringrules`、`internal/storagetiering`、生命周期模块支撑冷热数据治理。
- **媒体与远程访问**：`internal/posterwall`、`internal/mediascraper`、`internal/p2premote`、`internal/remoteaccess`、WebShare 相关前端支撑家庭 NAS 体验。
- **身份与用户管理**：RBAC/MFA/设备信任/SSO 与用户管理页面共同支撑中小团队权限治理。

---

## 六、验证与风险记录

- 已完成 Markdown 全量扫描：共 21 个 `.md` 文件，重点同步 `README.md`、`CHANGELOG.md`、`docs/competitive-analysis.md`、`docs/resource-stats.md`。
- 已完成资源统计：Git 跟踪文件、行数、Go 源码/测试、依赖、Docker 配置、模块数量。
- 已完成验证：`go test ./internal/logcenter ./internal/version` 通过。
- 建议发布后继续关注 GitHub Actions：CI/CD、Security Scan、Docker Publish、Compatibility Check、Release 流程。

---

*报告由资源统计更新生成*
