# 📊 NAS-OS 项目资源统计报告

> **统计日期**: 2026-07-03
> **统计版本**: v3.9.0 工作区 / v3.8.0 发布基线
> **最新提交**: `04eb4b41` (`v3.8.0`, `v3.8.0-canary`)
> **当前状态**: v3.9.0 文档与版本号准备中；本轮聚焦竞品对标价值沉淀，未提交

---

## 一、代码规模概览

| 指标 | 数值 |
|------|------|
| **Git 跟踪文件总数** | 3,976 个 |
| **Git 跟踪内容体积** | 45.5 MB |
| **工作区总占用** | 658 MB |
| **项目总行数（Git 跟踪）** | 1,788,639 行 |
| **Go 源码总行数** | 1,631,719 行 |
| **源代码行数（不含测试）** | 1,213,121 行 |
| **测试代码行数** | 418,598 行 |
| **测试代码占比** | 25.7% |

---

## 二、文件与模块统计

| 指标 | 数值 |
|------|------|
| **Go 文件总数** | 3,661 个 |
| **Go 源代码文件** | 2,703 个 |
| **Go 测试文件** | 958 个 |
| **internal/ 顶层模块数量** | 736 个 |
| **cmd/ 应用数量** | 3 个（backup、nasctl、nasd） |
| **Dockerfile 数量** | 6 个 |
| **docker-compose 配置数量** | 7 个 |
| **直接依赖** | 47 个 |
| **间接依赖** | 125 个 |
| **依赖合计** | 172 个 |

### 主要语言/文件类型分布

| 类型 | 文件数 | 行数 |
|------|------:|------:|
| Go | 3,661 | 1,631,719 |
| HTML | 63 | 54,220 |
| Shell | 50 | 24,508 |
| JSON | 19 | 18,417 |
| YAML | 24 | 16,472 |
| YML | 54 | 11,163 |
| JavaScript | 13 | 8,195 |
| Markdown | 21 | 7,498 |
| CSS | 9 | 6,495 |
| TSX | 12 | 4,550 |
| TypeScript | 14 | 1,756 |

---

## 三、依赖资源概览

直接依赖维持 **47** 个，核心资源域包括：

- **Web/API**：Gin、Gorilla mux/websocket、Swagger。
- **存储/文件系统**：FUSE、WebDAV、SQLite、AWS S3 SDK、Excelize、imaging/exif。
- **AI/搜索**：Bleve、Goldmark、本地语义搜索相关模块。
- **安全/认证**：JWT、LDAP、OTP、x/crypto。
- **监控/可观测**：Prometheus、OpenTelemetry、gopsutil、zap。
- **运维/CLI**：Cobra、cron、fsnotify、YAML。
- **网络**：QUIC、zeroconf、x/sys、x/time。

依赖规模与上一轮统计一致：**直接 47 / 间接 125 / 合计 172**，本轮未引入新的第三方依赖。

---

## 四、本轮功能价值与资源成本摘要

本轮工作区变更集中在 `README.md`、`CHANGELOG.md`、`VERSION`、`internal/version/version.go`，将项目推进到 **v3.9.0 准备态**，并补充 2026-07-03 竞品对标叙事。

### 功能/市场价值

- **竞品对标更完整**：新增/强化 Synology DSM 7.4、TrueNAS 26、飞牛 fnOS、QNAP QuTS hero h6.0 对照。
- **价值表达更清晰**：README 从“四大独家功能”调整为“五大差异化能力”，覆盖本地 AI、多云挂载、MCP、不可变存储等长期差异化。
- **v3.8.0 能力沉淀**：网络 FEC、重启原因历史、本地 AI 语义搜索治理、LXC 迁移计划已在文档中形成可传播说明。
- **既有能力再包装**：不可变快照、存储效率与分层、现代身份安全、媒体与远程访问被归入竞品回应框架，提升已有模块复用价值。

### 资源/成本评估

| 项目 | 评估 |
|------|------|
| **代码增量成本** | 低。本轮主要是文档与版本标识变更，未增加运行时代码路径。 |
| **依赖成本** | 低。`go.mod` 未新增依赖，供应链面未扩大。 |
| **维护成本** | 中。文档承诺覆盖多个竞品方向，后续需要保持 README、CHANGELOG、竞争分析与实际模块一致。 |
| **测试成本** | 低到中。当前变更不涉及核心逻辑，但版本号变更建议跑 `go test ./internal/version`；发布前再跑关键模块或全量测试。 |
| **产品收益** | 高。以较低工程成本强化 v3.8/v3.9 的市场叙事与路线可信度。 |

---

## 五、重点资源域

- **智能运维**：`internal/network`、`internal/logcenter`、`internal/lxc` 等模块支撑高速网络、重启复盘与容器迁移计划。
- **本地 AI/搜索治理**：`internal/semanticsearch` 与 AI 相关模块支撑 local-only 查询、脱敏返回和用途审计。
- **不可变与合规**：`internal/wormcomply`、`internal/objectimmutable`、`internal/immutablebackup`、快照相关模块覆盖勒索防护与合规归档。
- **存储效率与分层**：`internal/tiering`、`internal/tieringrules`、`internal/storagetiering`、生命周期模块支撑冷热数据治理。
- **媒体与远程访问**：`internal/posterwall`、`internal/mediascraper`、`internal/p2premote`、`internal/remoteaccess` 等模块对齐家庭 NAS 体验。

本轮重点四模块（`internal/network`、`internal/logcenter`、`internal/semanticsearch`、`internal/lxc`）合计约 **47 个文件 / 18,709 行**，是 v3.8.0 智能运维与本地 AI 治理价值的主要工程承载。

---

## 六、验证与风险记录

- 已完成资源统计：Git 跟踪文件、行数、Go 源码/测试、依赖、Docker 配置、模块数量。
- 当前工作区已有未提交变更：`CHANGELOG.md`、`README.md`、`VERSION`、`internal/version/version.go`；本报告新增更新 `docs/resource-stats.md`。
- 未执行 git commit/push。
- 建议发布前验证：
  - `go test ./internal/version`
  - `go test ./internal/network ./internal/logcenter ./internal/semanticsearch ./internal/lxc ./internal/version`
  - 如进入正式发布，再运行全量 `go test ./...`、`go vet ./...`、`go build ./...`

---

*报告由资源统计更新生成*
