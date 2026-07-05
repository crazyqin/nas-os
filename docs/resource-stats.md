# 📊 NAS-OS 项目资源统计报告

> **统计日期**: 2026-07-04
> **统计版本**: v3.12.0 Stable
> **最新提交基线**: `0745473d` (`feat: add NAS experience advisor`)
> **当前状态**: v3.12.0 版本号、CHANGELOG、README、竞品分析与资源统计已同步；本轮补齐智能文件夹、成本/容量治理和发布安全护栏叙事。

---

## 一、代码规模概览

| 指标 | 数值 |
|------|------|
| **工作区文件总数（不含 .git）** | 3,997 个 |
| **Git 跟踪内容体积** | 54 MB |
| **工作区总占用** | 659 MB |
| **项目总行数（工作区）** | 1,789,384 行 |
| **Go 源码总行数** | 1,632,680 行 |
| **源代码行数（不含测试）** | 1,213,706 行 |
| **测试代码行数** | 418,974 行 |
| **测试代码占比** | 25.7% |

---

## 二、文件与模块统计

| 指标 | 数值 |
|------|------|
| **Go 文件总数** | 3,668 个 |
| **Go 源代码文件** | 2,706 个 |
| **Go 测试文件** | 962 个 |
| **internal/ 顶层模块数量** | 738 个 |
| **cmd/ 应用数量** | 3 个（backup、nasctl、nasd） |
| **Dockerfile 数量** | 7 个 |
| **docker-compose 配置数量** | 10 个 |
| **直接依赖** | 48 个 |
| **间接依赖** | 224 个 |
| **依赖合计** | 272 个 |

### 主要语言/文件类型分布

| 类型 | 文件数 | 行数 |
|------|------:|------:|
| Go | 3,668 | 1,632,680 |
| HTML | 63 | 54,245 |
| Shell | 50 | 24,508 |
| JSON | 19 | 18,417 |
| YAML | 24 | 16,472 |
| YML | 54 | 11,163 |
| JavaScript | 13 | 8,197 |
| Markdown | 20 | 7,489 |
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

本轮变更集中在 v3.12.0 发布材料一致性、`internal/smartfolders`、`internal/costgovernance`、CI 发布安全护栏和项目资源统计，确保版本/里程碑叙事与当前工作区能力一致。

### 功能/市场价值

- **发布材料一致性**：`VERSION`、`internal/version`、README、CHANGELOG、竞品分析和资源统计统一到 v3.12.0。
- **智能文件夹**：新增规则化虚拟文件夹能力，覆盖最近文件、大文件、照片、视频、文档等常用整理入口。
- **成本/容量护栏**：新增容量风险评估，统一输出存储利用率、闲置日成本、过载资源数与月化浪费估算。
- **发布安全护栏**：Trivy Action 固定版本，新增 GitHub Actions 安全基线测试，降低供应链漂移风险。
- **资源统计刷新**：按当前工作区文件、Go/测试行数、依赖和 Docker 配置重新校准统计口径。

### 资源/成本评估

| 项目 | 评估 |
|------|------|
| **代码增量成本** | 低。新增纯内存分析逻辑与只读 HTTP 接口，不引入后台任务。 |
| **依赖成本** | 低。不新增第三方依赖。 |
| **维护成本** | 低到中。阈值规则集中在 Analyzer，后续可按真实容量策略调整。 |
| **测试成本** | 低到中。建议发布前验证 `internal/version`、`internal/smartfolders`、`internal/costgovernance`、`internal/compliance`、`internal/smartnotify`。 |
| **产品收益** | 中到高。把体验顾问后的“下一步”延伸到文件整理、容量治理和可审计发布。 |

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

- 已完成 Markdown 扫描：共 20 个工作区 `.md` 文件，重点同步 README、CHANGELOG、`docs/competitive-analysis.md`、`docs/resource-stats.md`。
- 已完成资源统计：工作区文件、行数、Go 源码/测试、依赖、Docker 配置、模块数量。
- 验证建议：发布前执行 `go test ./internal/version ./internal/smartfolders ./internal/costgovernance ./internal/compliance ./internal/smartnotify`。
- 建议发布后继续关注 GitHub Actions：CI/CD、Security Scan、Docker Publish、Compatibility Check、Release 流程。

---

*报告由资源统计更新生成*


## v3.12.0 文件洞察与媒体整理建议（2026-07-05）
- 对标 DSM 7.4 本地 AI/语义搜索、飞牛相册影视、TrueNAS 26 WebShare/TrueSearch。
- 新增 fileinsights：把智能文件夹扫描结果转换为大文件治理、照片索引、媒体库整理建议。


## v3.13.0 资源与功能增量

- 新增 `internal/sharesearchadvisor`：WebShare/搜索体验建议引擎，核心逻辑无外部依赖，适合低资源设备运行。
- 计算项：索引覆盖率、体验就绪分、外链安全建议、分享前快照建议、搜索质量调优建议。
- 运行成本：纯内存规则评估，不扫描文件内容；由上游文件索引或统计模块提供聚合信号。
