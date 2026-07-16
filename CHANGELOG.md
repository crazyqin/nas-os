# Changelog

## v3.24.0 (2026-07-16) - 默认表面收敛（破坏性）

### 破坏性变更
- `modules.optional` **默认 false**：非 Core 产品管理器（Docker/VM/Photos/AI/备份/云同步等）默认不构造、不注册。
- **删除** legacy `/api/v1/volumes/*` 与 `internal/storage/legacy_api.go`。
- **删除** 根目录 `api/*.go` 产品源码。
- 默认监听 **127.0.0.1**（非 0.0.0.0）。
- 静态 UI 改挂 `/webui`（避免与 `/api` 路由冲突）；可选页面仅 optional=true 时注册。

### 功能
- Core `Health()`：storage / users / smb / nfs / network 领域检查；bootstrap admin `MustChangePassword`。
- 生产 CSRF：`NAS_OS_ENV=production` 时强制 `NAS_CSRF_KEY`。
- 再降 ~120 零生产引用顶层包入 Lab；allowlist 刷新。

### 验证
- `go build ./cmd/nasd` ×2
- `go test ./internal/web ./internal/application ./internal/config ./internal/storage ./internal/users`
- `go list -deps ./cmd/nasd` 无 lab

## v3.23.1 (2026-07-16) - Skeptic fixes: transitive Lab cut, real Extension load, honest logs

### Fixes
- **Transitive Lab**: remove `internal/monitor` → `internal/lab/reports` import; report integration no longer pulls Lab into `go list -deps ./cmd/nasd`.
- **Extensions**: activeprotect / compliancescan / deployorch / netdiag now **retain managers** and mount real status/list/run routes (no construct-and-discard no-op).
- **False ready logs**: delete "✅ …就绪" and stale Start comments for stripped Lab managers (ML 勒索、日志中心、应用中心等).
- **Governance**: `TestNasdDependencyGraphExcludesLab` runs `go list -deps` on `./cmd/nasd`, `./internal/web`, `./internal/monitor`.

### Verify
- `go list -deps ./cmd/nasd | grep lab` → empty
- `go test ./internal/web ./internal/application ./internal/monitor`

## v3.23.0 (2026-07-16) - 运行时诚实与架构治理 (P0–P3)

### P0 运行时诚实
- 生产 `internal/web` **移除全部 `internal/lab/*` import 与默认 Manager 构造/路由注册**（约 25 个 Lab 包不再进入默认 `nasd` 热路径）。
- Extension 通过 `modules.extensions` **按需加载**（`extensions_loader.go`）；默认 `[]` 不加载任何扩展。
- 已知 Extension：activeprotect、agentworkflow、aiguardrails、compliancescan、deployorch、netdiag、voicehub。

### P2 健康探针
- `GET /api/v1/system/health`（及兼容 `/health`）聚合 **Core** 模块 `Health()`；失败返回 `status=unhealthy`、`code=1`。
- 新增 `AggregateCoreHealth` 与 handler 级回归测试。

### P3 治理
- 未知 catalog 名默认 **Lab**（不再默认 Extension）。
- 测试：`TestProductionWebDoesNotImportLab`、`TestCoreCatalogExactlyFive`、`TestTopLevelInternalAllowlistFrozen`、路径锁。
- 冻结 `internal/application/toplevel_allowlist.txt`。

### P1 入口与文档诚实
- 根 `api/README.md` 标明 **非 nasd 入口**。
- 文档明确主 UI=`webui/`、主部署=`docker-compose.yml`、存储 `/volumes` 弃用时间表。
- 功能矩阵：Core | 配置启用的 Extension | Lab（默认无）。

### 验证
- `go build ./cmd/nasd`（×2）
- `go test ./internal/web ./internal/application ./internal/arch ./internal/version ./internal/config`

## Unreleased (docs + version integrity)

### 修复
- `internal/version`：默认版本从过时的 `3.20.0` 同步为 `3.22.0`，并将 `Version` 改为可被 `-ldflags -X` 覆盖的 `var`。
- `GET /api/v1/system/info`：不再硬编码 `0.1.0`，改为返回 `internal/version` 的真实版本与构建信息。
- `Makefile` `build-version`：ldflags 字段名与 `Commit`/`BuildTime` 对齐；版本去掉多余 `v` 前缀。
- `scripts/version.sh` / `scripts/version-info.sh`：从 `VERSION` 文件同步，修正 GitHub 仓库名。

### 文档
- 全面修订 `README.md`、`docs/ARCHITECTURE.md`、`docs/resource-stats.md`、`docs/competitive-analysis.md`、`docs/api/*.md`：最新版本 **v3.22.0**，Core/Extension/Lab 路径与统计一致。

### 测试
- `TestVersionMatchesVERSIONFile`：断言 shipped version 与根 `VERSION` 一致。
- `TestGetSystemInfoReportsShippedVersion`：驱动真实 `getSystemInfo` handler。

## v3.22.0 (2026-07-16) - 架构收敛：163 伪核心模块降入 Lab

### 架构收敛
- 将 **163** 个零生产引用、未接入 Core 生命周期主图的伪核心模块从 `internal/` 顶层迁入 `internal/lab/`。
- 主题分组：文件生命周期/去重、存储硬件辅助、网络边缘、身份安全辅件、协作内容应用、运维平台概念（含 `acme`/`alerting`/`selfheal`/`ztna` 等未接线旧 Extension 标签）。
- 修正 catalog 与路径不一致：`activebackup`、`reports`、`smartpricing`（及缺失磁盘包 `download`）统一登记为 **Lab**，与 `internal/lab/` 放置对齐。
- Core 仍仅 `identity` / `storage` / `network` / `sharing` / `system`；Extensions 命名空间包数量不变（7）。
- 扩充 `modulecatalog_test` 路径边界与 Core-only-five 回归，阻止旧顶层路径回流。
- 顶层 `internal/*` 业务包约 493→330；Lab 约 304→467。

### 验证
- `go build -o nasd ./cmd/nasd`（两次）
- `go test ./internal/application ./internal/arch`
- 抽样 `go test` 已迁移 lab 包
- `go build` 覆盖迁移后的 import 图

## v3.18.6 (2026-07-16) - Pro 伪核心模块降入 Lab

### 架构收敛
- 将零生产引用、未接入 Core 生命周期的 `containerpro`、`themepro`、`smarthomehubpro` 从 `internal/` 顶层迁入 `internal/lab/`。
- 三个 `pro` 增强模块与现有容器、主题、智能家居领域重叠；保留实现用于实验评估，但不再占用生产核心命名空间。
- 将三者登记为 Lab，并扩充目录边界回归测试，阻止旧顶层路径回流。

### 验证
- `go test ./internal/application ./internal/lab/containerpro ./internal/lab/themepro ./internal/lab/smarthomehubpro`
- `go build ./...`
- `git diff --check`

## v3.18.5 (2026-07-16) - 重复告警引擎降入 Lab

### 架构收敛
- 将零生产引用、未接入 Core 生命周期的 `guidedalert`、`guidedalerts`、`smartalert` 从 `internal/` 顶层迁入 `internal/lab/`。
- 三套模块均与现有告警能力重叠，继续保留用于实验和兼容评估，但不再占用生产核心命名空间。
- 将三者登记为 Lab，并扩充目录边界回归测试，阻止旧顶层路径回流。

### 验证
- `go test ./internal/application ./internal/lab/guidedalert ./internal/lab/guidedalerts ./internal/lab/smartalert`
- `go build ./...`
- `git diff --check`

## v3.18.4 (2026-07-16) - 概念型独立模块降入 Lab

### 架构收敛
- 将零生产引用、未接入 Core 生命周期的 `digitalassetvault`、`familyactivityhub`、`iotedgegateway` 从 `internal/` 顶层迁入 `internal/lab/`。
- 三个模块均为独立概念实现或学习型能力；保持 Go package 与行为不变，仅收紧目录层级。
- 扩充目录边界回归测试，锁定 Lab 新路径并禁止旧顶层路径回流。

### 验证
- `go test ./internal/application ./internal/lab/digitalassetvault ./internal/lab/familyactivityhub ./internal/lab/iotedgegateway`
- `go build ./...`
- `git diff --check`

## v3.18.3 (2026-07-16) - 可选能力迁入 Extension 命名空间

### 架构收敛
- 将未接入 Core 生命周期主图的 `agentworkflow`、`aiguardrails`、`voicehub` 从 `internal/` 顶层迁入 `internal/extensions/`。
- 保持 Go package 名称和对外行为不变，仅收紧目录所有权，避免可选能力继续占用伪核心顶层。
- 扩充目录边界回归测试，锁定 Extension 新路径并禁止旧顶层路径回流。

### 验证
- `go test ./internal/application ./internal/extensions/agentworkflow ./internal/extensions/aiguardrails ./internal/extensions/voicehub`
- `go build ./...`
- `git diff --check`

## v3.18.2 (2026-07-16) - 伪核心 v2/pro 模块降级

### 架构收敛
- 将未被生产代码引用的 `benchmarkpro`、`blockdedup2`、`cloudsync2`、`filetimemachine2`、`resmonpro`、`smartthermal2` 从 `internal/` 顶层迁入 `internal/lab/`。
- 将上述概念增强版统一登记为 Lab，阻止 `v2` / `pro` 命名的重复实现继续伪装成生产 Core。
- 扩充目录边界回归测试，同时断言 Lab 新路径存在、旧顶层路径消失。

### 验证
- `go test ./internal/application ./internal/lab/benchmarkpro ./internal/lab/blockdedup2 ./internal/lab/cloudsync2 ./internal/lab/filetimemachine2 ./internal/lab/resmonpro ./internal/lab/smartthermal2`
- `go build ./...`
- `git diff --check`

## v3.18.1 (2026-07-15) - tier 目录收口补强

### 架构收敛
- 补强 tier 命名空间校验测试：除验证 `internal/extensions/activeprotect` 与 `internal/lab/{aimediatag,brandinsight,releasemanager}` 存在外，新增断言旧的顶层 `internal/{activeprotect,aimediatag,brandinsight,releasemanager}` 路径必须消失，防止伪核心目录回流。
- 更新 `docs/ARCHITECTURE.md`，把已完成的 Core / Extension / Lab 收敛实例写入正式架构说明，收紧目录治理边界。

### 验证
- `go test ./internal/application ./internal/extensions/activeprotect ./internal/lab/aimediatag ./internal/lab/brandinsight ./internal/lab/releasemanager`
- `go build ./...`
- `git diff --check`

## Earlier notes (pre-v3.18, historical)
- API 按公开、已认证和管理员权限分层；管理接口默认要求管理员权限。
- 认证令牌仅接受 Bearer Header，不再接受查询参数；敏感业务身份统一来自认证上下文。
- 移除 Digital Legacy 硬编码加密密钥；无效或缺失密钥时安全禁用模块。
- 统一健康探针，并完善监听错误传播、优雅关闭和后台 worker 停止流程。

### 架构与配置
- 将 `brandinsight`、`releasemanager`、`aimediatag` 下沉到 `internal/lab/`，将 `activeprotect` 收敛到 `internal/extensions/`，避免非核心模块继续占用 `internal/` 顶层伪核心位置。
- 新增 tier 命名空间校验测试，确保 Core / Extension / Lab 目录边界持续可检验。
- 新增 typed runtime configuration、`--config` 参数及 `NAS_OS_*` 环境覆盖。
- 新增 `internal/application` 组合根，收口进程级依赖构造与生命周期。
- 激活 identity、storage、network、sharing、system 核心模块图。
- 模块容器新增缺失依赖/循环检测、确定性拓扑排序、启动失败回滚、逆序停止和错误聚合。
- identity、storage、network、sharing、system 逐步接管路由与健康状态所有权。
- 冻结历史 storage API 路由契约，后续按端点兼容迁移。

## v3.18.0 (2026-07-13)

### 新增模块 (27)

#### 兵部（软件工程）
- **NFS over RDMA** (`nfsrdma`) — InfiniBand/RoCE NFS 传输，对标 TrueNAS 25.04 NFS over RDMA
- **iSCSI Target Offload** (`iscsitgtoffload`) — iSCSI Target 硬件卸载到智能网卡/DPU/HBA
- **ActiveProtect** (`activeprotect`) — 集中备份保护控制台，对标 Synology ActiveProtect
- **AI Media Tagger** (`aimediatag`) — AI 媒体自动标签
- **Audio Quality** (`audioquality`) — 音频质量检测与增强
- **DRAID2** (`draid2`) — 分布式 RAID 增强
- **NetShield** (`netshield`) — 网络安全防护
- **Forensics2** (`forensics2`) — 存储取证增强

#### 工部（DevOps/运维）
- **LXC Manager** (`lxcmanager`) — LXC 系统容器管理，对标 TrueNAS 25.04 Containers
- **VM Restore** (`vmrestore`) — 虚拟机快照恢复管理，对标 TrueNAS 经典虚拟化恢复
- **TrueCloud Backup** (`truecloudbk`) — 混合云备份
- **USB Smart Mount** (`usbsmartmount`) — USB 智能挂载，对标 Synology USB Copy
- **Update Director** (`updatedirector`) — 智能更新管理

#### 户部（财务/电商）
- **API Level Meter** (`apilevelmeter`) — API 使用量监控与配额，对标 TrueNAS API Keys
- **Cost Benchmark** (`costbenchmark`) — 存储成本基准分析
- **Storage Budget** (`storagebudget`) — 存储预算管理
- **Storage Cost Predict** (`storagecostpredict`) — 存储成本预测

#### 礼部（品牌/内容）
- **SSO Center** (`ssocenter`) — 统一身份认证，对标 Synology SSO Server
- **Brand Insight** (`brandinsight`) — 品牌洞察分析
- **Poster Wall Pro** (`posterwallpro`) — 海报墙增强
- **Note Sync** (`notesync`) — 笔记同步

#### 吏部（项目管理）
- **Release Track** (`releasetrack`) — 版本通道管理，对标群晖/TrueNAS 通道
- **Release Manager** (`releasemanager`) — 发布管理器
- **Team Workspace** (`teamworkspace`) — 团队工作区，对标 Synology Drive
- **Feature Matrix** (`featurematrix`) — 功能矩阵

#### 刑部（法务/合规）
- **Data Sovereignty2** (`datasovereignty2`) — 数据主权增强
- **Destruction Audit** (`destructionaudit`) — 数据销毁审计
- **SafeAccess** (`safeaccess`) — 安全访问控制

### 竞品功能对标
- **TrueNAS 25.04**: NFS over RDMA, LXC 容器, ZFS Fast Dedup, API Keys, 虚拟化恢复
- **Synology DSM**: ActiveProtect, SSO Server, USB Copy, Drive 团队空间
- **飞牛 NAS**: 智能挂载, 媁破备份

## v3.17.0 (2026-07-12) - 数据分层与恢复置信

### 新增模块（6个）

#### 兵部（工程）
- **ZFS 数据集分层调度器** (datasettier): 对标 TrueNAS 26 OpenZFS 2.4 dataset tiering 和 Synology 存储效率分层，支持自动分层策略、预测式分层、冷热数据迁移、闪存容量管理、HDD 过载检测和归档层启用建议

#### 户部（财务）
- **TCO 可视化仪表板** (tcodashboard): 对标 Synology TCO Calculator 和 TrueNAS 成本智能，支持 5 年总拥有成本计算、电力/云存储/人工成本分析、与竞品 TCO 对比、NVMe 投资比例检测和维护预算建议

#### 礼部（内容）
- **AI 照片故事生成器** (photostory): 对标 Synology Photos Memories、飞牛 AI 相册和 Apple Memories，支持旅行/家庭/季节/冒险/回溯/精选 7 种故事主题、日期聚类、位置/人脸/场景标签分组和高质量精选集

#### 工部（运维）
- **FEC 网络纠错配置器** (fecconfig): 对标 TrueNAS 26 FEC 配置，支持 Reed-Solomon/Hamming/BCH/LDPC 编码推荐、高速链路纠错、存储接口保护、长距离信号完整性、WiFi 存储风险检测和协议错误诊断

#### 吏部（管理）
- **合规审计工作流编排器** (complianceflow): 对标 GDPR/PIPL/HIPAA/SOC2/ISO 27001/PCI-DSS 合规标准，支持 6 阶段审计流程（发现→差距分析→修复→取证→报告→审查）、PII/PHI/支付数据自动检测、加密/审计日志缺口识别和多标准并行编排

#### 刑部（合规）
- **勒索恢复置信度评估器** (restoreconfidence): 对标 TrueNAS 数据层勒索恢复和 Clean Restore Confidence 标准，支持恢复演练追踪、RTO/RPO 达标验证、不可变备份/异地副本检查、快照扫描验证、TFA 查验和恢复信心评分

### 测试
- 全部 6 个新模块编译通过，单元测试 100% 通过

# Changelog

## v3.16.0 (2026-07-09) - 智能调度与数据主权

### 新增模块（6个）

#### 兵部（工程）
- **SSD 缓存智能调度器** (ssdcacheschedule): 对标 Synology SSD Cache Advisor 和 TrueNAS L2ARC，支持缓存命中率分析、磨损预警、温度监控、读写放大治理、预热调度和 NVMe 升级建议

#### 户部（财务）
- **云存储成本审计器** (cloudcostaudit): 对标 Synology CloudRep 和 TrueNAS 云同步成本追踪，支持预算超支检测、休眠账户识别、出口流量优化、生命周期分层缺失检测、R2 迁移建议和 API 调用优化

#### 礼部（内容）
- **多媒体海报智能刮削器** (posterscraper): 对标 Synology Video Station 和飞牛影视墙，支持批量海报缺失检测、文件名解析失败修复、字幕获取建议、低置信度匹配审核、自动刮削调度和缓存清理

#### 工部（运维）
- **电源管理调度器** (powermanager): 对标 Synology Power Schedule 和 TrueNAS 电源管理，支持空闲模式切换、待机调度、磁盘转速策略、高功耗检测、夜间计划、太阳能对齐和 WoL 配置

#### 吏部（管理）
- **多 NAS 集群编排器** (clusterops): 对标 Synology CMS 和 TrueNAS Cluster，支持 HA 启用建议、故障转移验证、脑裂检测、见证节点状态、存储重分布、复制延迟和集群健康评分

#### 刑部（合规）
- **数据主权审计器** (datasovereigntyaudit): 对标 GDPR/CCPA/PIPL/HIPAA 合规要求，支持 PII 加密检测、跨境复制审查、访问日志缺失、保留策略缺失、数据处理清单和 DPA 缺失审计

### 测试
- 全部 6 个新模块编译通过，单元测试 100% 通过


## v3.15.0 (2026-07-08) - 存储效率与合规审计

### 新增模块（9个）

#### 存储效率
- **快照调度器** (snapscheduler): 对标 TrueNAS 定时快照策略和 Synology 快照计划，支持小时/日/周级快照策略、不可变快照、异地复制建议
- **存储成本预测器** (costforecast): 对标 Synology 存储分析器，预测容量耗尽时间、预算超支、去重/压缩/分层节省空间建议
- **去重顾问** (dedupadvisor): 对标 TrueNAS ZFS Fast Dedup，分析存储池去重信号并生成优化建议

#### 系统运维
- **系统公告板** (sysbulletin): 对标 DSM 通知中心，根据 SMART 警告、安全告警、备份失败、磁盘使用率生成优先级公告
- **固件更新编排器** (fworchestrator): 对标 TrueNAS 更新协调器，8阶段更新流程（预检→备份→下载→暂存→安装→后检→重启→验证）
- **网络健康顾问** (nethealthadvisor): 对标 TrueNAS 网络改进，分析链路速率、MTU、IPv6、证书、防火墙、丢包率等

#### 合规与配额
- **存储配额工作流** (quotaworkflow): 对标 Synology 配额管理，支持软/硬限制、自动告警、自动扩容建议
- **数据保留策略引擎** (retentionpolicy): 对标 Synology WORM 和 TrueNAS 不可变快照，支持财务/HR/法务/医疗/通用保留类别
- **数据清洗顾问** (scrubadvisor): 对标 TrueNAS 数据清洗，分析校验错误、SMART 警告、快照不可变性等

### 测试
- 全部9个新模块编译通过，单元测试 100% 通过


## v3.14.0 (2026-07-06) - 备份健康顾问

### 新增功能
- **备份健康顾问 (backuphealthadvisor)** - 基于终端保护率、最近备份、失败任务、增量/去重、快照、不可变恢复点、异地副本与恢复演练生成备份韧性建议。
- **保护率与就绪评分** - 输出设备保护覆盖率和备份就绪分，帮助仪表盘、通知系统和新手向导识别优先修复项。
- **恢复闭环建议** - 针对过期备份、失败任务、关键共享快照、勒索告警、恢复介质和异地副本提供可执行动作。

---

## v3.13.0 (2026-07-05) - WebShare 搜索顾问

### 新增功能
- **WebShare 搜索顾问 (sharesearchadvisor)** - 基于文件规模、索引覆盖、外链数量、移动访问、passkey 与快照状态生成浏览器分享和本地搜索建议。
- **索引覆盖与就绪评分** - 输出搜索覆盖率、体验就绪分和高/中优先级动作，便于前端、通知系统和向导直接消费。
- **外链安全与分享保护** - 对外部分享给出 passkey/一次性访问码、过期策略、审计记录和分享前快照保护建议。

---

# NAS-OS 变更日志

## [v3.12.0] - 2026-07-05

### Added
- 新增 `internal/fileinsights` 文件洞察引擎，将智能文件夹扫描结果转换成可执行建议。
- 增加大文件治理、照片语义整理、媒体库刮削/字幕/续播准备三类推荐动作。
- 补充 fileinsights 单元测试，覆盖阈值、排序和空结果场景。

### Improved
- 延续竞品对标：吸收 DSM 7.4 本地 AI/搜索、飞牛相册影视体验、TrueNAS 26 WebShare/TrueSearch 的动作化治理思路。


## v3.11.0 - 2026-07-04

### 面向用户的更新亮点
- 智能文件夹提供 recent、large-files、photos、videos、documents 等规则化视图，帮助用户按最近修改、大小和文件类型快速整理文件资产。
- 成本/容量风险摘要会识别低利用、高水位、过载资源和浪费估算，把容量状态转化为清理、分层或扩容建议。
- 发布流水线安全基线补齐第三方 Action 固定版本检查，减少 release 前供应链漂移风险，让发布包更可审计。
- `VERSION`、命令行/API 构建信息、README、CHANGELOG、资源统计和竞品分析同步到 v3.11.0，降低用户升级时的版本判断成本。

### 文档与对标
- README 将 v3.11.0 放在“发现-整理-治理”的用户路径中，承接 v3.10.0 体验顾问叙事。
- `docs/resource-stats.md` 按当前工作区重新记录文件规模、模块数量、依赖统计与本轮能力说明。
- `docs/competitive-analysis.md` 聚焦智能文件夹、成本/容量治理、体验顾问和安全发布护栏。

### 验证
- 待发布前执行 `go test ./internal/version ./internal/smartfolders ./internal/costgovernance ./internal/compliance ./internal/smartnotify`。

## v3.10.0 - 2026-07-04

### 面向用户的更新亮点
- 新增 `internal/experienceadvisor` 体验顾问引擎，将本地聚合使用信号转化为照片、媒体、备份、远程访问、应用和存储的下一步建议。
- AI 相册场景会在照片库增长时建议人物聚类、动态照片解析和重复照片清理，帮助家庭用户更快整理照片。
- 影视库场景会建议海报墙刮削、字幕匹配、转码预设和跨端播放进度同步，提升客厅与移动端播放体验。
- 备份与存储场景会提示恢复校验、不可变快照、scrub、生命周期分层和容量规划，把数据保护动作前置到问题发生之前。
- 远程访问场景会结合多设备与异常连接信号，提示 NAT 穿透、DDNS、证书续期和公网入口收敛，降低远程访问配置风险。
- 应用与容量场景会识别低活跃应用和容量增长趋势，给出清理、替代、归档或扩容建议。

### 竞品启发
- 借鉴飞牛 fnOS 的 AI 相册、影视墙和低门槛远程访问体验，把家庭 NAS 的“好用”转化为可操作建议。
- 借鉴群晖 DSM 的套件生态、照片/备份一体化和引导式管理，把复杂功能组织成用户能理解的下一步。
- 借鉴 TrueNAS 的 ZFS 快照、scrub、数据完整性和容量规划思路，让备份与存储建议更强调可靠性。

### 验证与运维
- 增加体验顾问单元测试，覆盖优先级排序、禁用信号过滤、远程访问异常提权和低活跃应用推荐。
- 发布前检查磁盘与内存状态；当前磁盘空间充足，无需清理。



## v3.9.0 (2026-07-03) - 安全与运维体验增强

### 新功能与增强
- 重启原因运维闭环：Log Center 新增重启原因处理与历史记录能力，覆盖用户触发、系统异常、计划维护等场景，并补充 handler 测试。
- WebShare 交互增强：优化文件分享前端逻辑，提升分享状态展示与用户操作一致性。
- 登录与用户管理界面优化：调整登录页和用户管理页交互细节，改善权限管理与日常运维体验。
- 版本推进：同步 `VERSION` 与 `internal/version/version.go` 到 v3.9.0。

### 文档与竞品对标
- 新增 2026-06-29 竞品分析文档，补充 Synology DSM、TrueNAS、飞牛 fnOS、QNAP 等方向的功能对照。
- 重构 `docs/competitive-analysis.md`，沉淀安全、存储效率、AI、本地化数据服务与运维体验路线。
- 更新 `docs/resource-stats.md`，记录 v3.9.0 工作区状态、资源统计与产品收益分析。

### 验证
- `go test ./internal/logcenter ./internal/version` 通过。

## v3.8.0 (2026-07-02) - 智能运维与本地 AI 治理

### 新功能
- 网络 FEC 管理：新增 FEC 模式推荐、配置意图与网络变更审计记录，提升 25G/100G 高速链路可靠性。
- 重启原因历史：新增重启事件记录与原因分类，覆盖用户触发、内核异常、计划更新、电源异常等场景。
- 本地 AI 语义搜索治理：新增 local-only governed search，支持查询用途审计与脱敏返回。
- LXC 迁移计划：新增冷/热/在线迁移计划生成，包含预检、快照、同步、验证、切换和回滚步骤。

### 验证
- `go test ./internal/network ./internal/logcenter ./internal/semanticsearch ./internal/lxc ./internal/version` 通过。

### 竞品对标
- Synology DSM 7.4/Computex 2026: 本地 AI、存储效率、RBAC、Cluster Manager、Log Center。
- TrueNAS 26: WebShare、TrueSearch、LXC、网络 FEC、重启原因历史与本地化数据服务。
- 飞牛 fnOS: 媒体中心、海报墙、AI 相册与远程访问体验。
- QNAP QuTS hero h6.0: passkeys、immutable snapshots、FileTiers 等身份安全/不可变保护/分层方向。

### 文档
- README 补充 v3.8.0 对标说明：FEC、重启原因、本地 AI 语义搜索治理、LXC 迁移计划、不可变快照、存储效率与媒体/远程访问体验。
- 竞品分析文档更新到 2026-07-03，加入 DSM 7.4、TrueNAS 26、fnOS、QNAP h6.0 的公开功能对照与下一步路线。

## v3.7.0 (2026-07-02) - 存储与应用生态增强

### 新功能（5个新模块）
- RAIDZ vdev 逐盘扩展（raidzexpand）— 扩展前健康检查、Dry Run、进度跟踪、取消/失败状态管理，参考 TrueNAS RAIDZ Expansion。
- 存储 ROI 可视化分析（storageroi）— 磁盘采购成本、容量利用率、寿命追踪、TCO/ROI 评分与优化建议。
- 智能海报墙（posterwall）— 媒体刮削、海报墙布局、多端播放进度同步、观影清单与推荐，参考飞牛 fnOS 影视墙。
- 应用中心评价系统（appreview）— 评分评论、开发者回复、评价举报审核、统计聚合，参考群晖 DSM 套件中心评价。
- NFS Kerberos 认证审计（nfspkaudit）— 认证事件记录、加密类型识别、风险告警、合规报告，参考企业 NFS 安全审计。

### 增量增强
- 应用中心新增生命周期 Dry Run 计划接口：安装/更新/卸载/启停前展示依赖、阻塞项、端口冲突和执行步骤。
- 成本分析新增预算容量规划：结合月预算、增长率、目标利用率预测扩容需求、预算状态和快速 ROI。
- 容器仪表盘新增运维洞察：按健康、状态、资源占用、重启次数生成优先级行动清单。
- 智能海报墙新增 Discovery Digest：首页 Hero、内容架、类型统计和媒体库概览。
- RAIDZ 扩展新增预飞计划：容量增益、重排数据量、预计耗时和风险分级。
- 新增 Security Advisor 轻量安全体检：SSH/MFA/防火墙/审计/备份/补丁等评分建议。

### 验证
- `go test ./internal/raidzexpand ./internal/storageroi ./internal/posterwall ./internal/appreview ./internal/nfspkaudit` 通过。
- `go test ./internal/appcenter ./internal/containerdashboard ./internal/costanalyzer ./internal/mediascraper ./internal/raidzexpand ./internal/securityadvisor` 通过。
- `go test ./...` 通过。
- `go vet ./...` 通过。
- `go build ./...` 通过。
- GitHub Actions：CI/CD、Security Scan、Docker Publish、Compatibility Check 全部成功。

### 竞品对标
- TrueNAS 24.10/26: RAIDZ 扩展、NVMe/存储健康能力。
- 飞牛 fnOS: 影视刮削、海报墙体验。
- 群晖 DSM: 应用中心评价、NFS Kerberos 安全审计。

## v3.4.0 (2026-06-29) - 竞品对标功能升级

### 新功能（6个新模块）
- SMB 有状态 HA 故障转移（smbhafailover）— 会话状态跨故障转移保持，客户端无需重新认证
- 数据分层自定义规则（tieringrules）— 按访问频率/修改时间自动冷热分层
- 邮件 OAuth 通知（emailoauth）— 支持 Gmail/Outlook OAuth2 授权发送通知
- WebSocket API 现代化（wsapi）— JSON-RPC 2.0 + SCRAM-SHA-512 认证
- 弹性存储加密（storencrypt）— 密码解锁加密存储空间
- 邮件审核机制（mailaudit）— 敏感邮件发送/接收前管理员审核

### CI/CD 修复
- 修复 Benchmark workflow 超时问题（添加 timeout-minutes，减少 bench count）
- 同步 VERSION 文件与代码版本

### 竞品对标
- TrueNAS 26: SMB HA、API 现代化
- 群晖 DSM 7.3: 数据分层规则、文件锁定、邮件审核、存储加密
- 飞牛 fnOS v1.2: 邮件 OAuth 通知

## v3.1.0 (2026-06-26) - 3.0 架构重构稳定化

### 稳定化重点
- 补齐 3.0 架构重构后的新模块测试：blockbackup、perfmon、ransommldetect、snapshotmgr、sysmonitor、clientthumb、lxcmanager、motionphoto、rdmanfs、secureboot、smbguard、teamfile、userapikey、websharepro、wormcomply。
- 修复编译兼容问题：`ransommldetect.NewDetector/NewHandlers` 接入 logger 参数，`internal/web` 初始化与路由注册同步更新。
- 清理重复定义：WebShare Pro 分享链接批量错误类型改为 `BatchShareError`，复用统一 `generateToken`，避免与批量操作模块冲突。
- 修复测试稳定性：filepreview 缓存异步索引保存增加等待机制，避免 TempDir 清理竞态；tiering 默认配置测试统一 int64 断言。
- 同步文档与资源统计到 v3.1.0。

### 验证
- `go test ./internal/blockbackup ./internal/perfmon ./internal/ransommldetect ./internal/snapshotmgr ./internal/sysmonitor ./internal/clientthumb ./internal/lxcmanager ./internal/motionphoto ./internal/rdmanfs ./internal/secureboot ./internal/smbguard ./internal/storagetiering ./internal/teamfile ./internal/userapikey ./internal/websharepro ./internal/wormcomply` 通过。
- `go test ./internal/filepreview ./internal/websharepro ./internal/web ./cmd/nasd` 通过。

## v2.621.0 (2026-06-25) - 模块整合重构

### 重构优化

本次重构合并63个重复功能模块为6个规范模块，消除功能重叠，提升代码可维护性：

#### 合并统计
- 删除 **63个**重复模块目录
- 删除 **83,366行**重复代码（262文件变更）
- 模块总数：1034 → 974
- `go vet` 全部通过

#### 合并明细

| 功能域 | 保留模块 | 原模块数 | 说明 |
|--------|----------|----------|------|
| 勒索防护 | `ransomware` | 12 | 保留最强(ransomshield)，合并12个重复模块 |
| 合规审计 | `compliance` | 15 | 保留最强(compliancereport)，合并14个重复模块 |
| 存储分层 | `tiering` | 13 | 保留最强(tiering)，合并12个重复模块 |
| AI相册 | `photoai` | 6 | 保留最强(photoai)，合并5个重复模块 |
| 成本分析 | `costanalyzer` | 5 | 保留最强(smartcostoptimizer)，合并4个重复模块 |
| 磁盘健康 | `diskhealth` | 5 | 保留最强(diskhealthai2)，合并4个重复模块 |
| AI控制台 | `aiconsole` | 3 | 保留最强(aiconsoledatamask)，合并2个重复模块 |
| AI Agent | `aiagentorch` | 2 | 保留最强(aiagentorch)，合并1个重复模块 |

#### 保留的依赖模块（web/server.go引用）
- `ransommldetect` - 勒索ML检测器
- `activebackup` - 整机备份管理
- `backupverify` - 备份验证

---

## v2.620.0 (2026-06-24) - 自动化协作新功能集

### 新增功能

#### 智能媒体库 (smartmedialib)
- 媒体文件自动扫描与分类（照片/视频/音频/文档）
- 智能相册创建与管理（手动/自动/智能规则）
- 文件收藏与评分系统
- 标签提取与搜索
- RESTful API 接口
- 参考飞牛 AI 相册、群晖 Photo Station

#### 仪表盘小组件引擎 (dashboardwidget)
- 可视化仪表盘创建与管理
- 多种组件类型（图表/仪表/列表/统计/时间线/地图）
- 组件位置与尺寸自定义
- 系统预置组件（CPU/内存/磁盘/网络/容器）
- RESTful API 接口
- 参考群晖 Dashboard、TrueNAS 管理界面

#### AI 文件整理器 (aifileorganizer)
- 文件自动分类（图片/视频/音频/代码/文档/压缩包）
- 智能整理规则引擎
- 文件评分与建议系统
- 自动/手动整理模式
- RESTful API 接口

#### 存储报告器 (storagereporter)
- 存储快照采集与历史追踪
- 容量趋势分析（7天/30天）
- 磁盘满预测与告警
- 分类存储统计
- 完整存储报告生成
- RESTful API 接口

#### 容器健康监控 (containerhealthmon)
- 容器注册与状态追踪
- 健康状态监控（healthy/degraded/critical）
- 自动重启与重启计数
- 健康规则引擎
- 监控事件日志
- RESTful API 接口

#### 安全合规检查 (securitycompliance)
- 10项安全检查（SSH/防火墙/密码/更新/加密/备份/SSL/端口/权限/审计）
- 合规评分系统（0-100分）
- 审计日志记录与查询
- 安全建议与修复方案
- RESTful API 接口
- 参考 TrueNAS 安全检查、群晖安全顾问

### 改进

#### 测试修复
- 修复 tiering 模块测试期望值（CheckInterval/MaxConcurrent）

## v2.619.0 (2026-06-23) - 智能磁盘健康预测系统

### 新增功能

#### 智能磁盘健康预测 (diskhealth)
- S.M.A.R.T. 数据实时采集与分析
- AI 驱动的磁盘故障预测（基于线性回归趋势分析）
- 健康评分系统（0-100分，自动评估磁盘状态）
- 多级告警机制（info/warning/critical/emergency）
- 风险因素识别（重分配扇区、温度过高、通电时间等）
- 智能建议生成（根据故障概率推荐操作）
- 历史趋势追踪与预测
- RESTful API 接口
- 参考群晖健康检查、TrueNAS S.M.A.R.T. 监控、飞牛 AI 能力

### 修复

#### CI/CD 修复
- 修复 tiering 模块 TierTypeMemory 未定义问题
- 修复 tiering 模块 WarmThreshold 缺失字段
- 修复 ups 模块 CreateShutdownTask 返回值处理
- 修复 integration 测试 int/int64 类型转换

## v2.618.0 (2026-06-23) - WebShare浏览器文件共享/AI智能文件搜索

### 新增功能

#### WebShare 浏览器文件共享 (webshare)
- 浏览器文件浏览与管理（目录列表、创建文件夹、上传下载）
- 文件操作（移动、复制、重命名、删除）
- 分享链接生成（支持密码保护、过期时间、下载次数限制）
- 多协议支持（本地文件系统，预留 SMB/NFS 接口）
- FIPS 加密传输支持
- 快照时间线浏览
- 统计与监控 API
- 对标 TrueNAS WebShare 功能

#### AI 智能文件搜索 (aifilesearch)
- 自然语言文件搜索
- 文件名、内容、标签多维度匹配
- 搜索评分与排序
- 索引统计与重建
- 搜索建议生成
- 对标群晖 Drive AI 搜索功能

### 修复

#### CI/CD 修复
- 修复 tiering 模块类型重复声明问题
- 修复 int/int64 类型转换错误
- 添加缺失的类型定义字段
- 创建 UPS handlers

---

## v2.617.0 (2026-06-23) - 桌面小组件/IPTV直播/AI磁盘健康预测/勒索蜜罐/FIDO2密钥/存储效率/LXC快照

### 新增功能

#### 桌面小组件系统 (desktopwidgets)
- 丰富的桌面小组件类型：时钟、天气、CPU/内存/磁盘/网络监控、日历、待办、便签等
- 自由拖拽布局与位置调整
- 多主题支持与透明度调节
- 实时数据刷新机制
- 桌面布局导入导出
- 对标飞牛fnOS桌面拖拽，功能更丰富

#### IPTV 直播服务 (iptvservice)
- 直播频道管理（添加/删除/搜索）
- 多协议支持：HLS/RTMP/RTSP/UDP/HTTP
- 频道分组管理
- M3U播放列表导入
- EPG节目单支持
- 频道状态监控与统计
- 对标飞牛fnOS局域网直播源功能

#### AI 磁盘健康预测 (aidiskhealth)
- 基于SMART数据的健康评分系统
- AI故障预测与剩余寿命估算
- 风险因素识别与告警
- 历史趋势记录
- 多磁盘统一监控
- 对标群晖DSM AI智能运维

#### 勒索软件蜜罐检测 (ransomware_honeypot)
- 创建诱饵文件（Office/PDF/图片/文本格式）
- 监控文件熵值变化，检测加密特征
- 检测异常批量重命名行为
- 自动告警并支持隔离受感染共享
- 恢复点管理与扫描历史
- HTTP API：创建蜜罐/列表/扫描/告警/响应
- 完整测试覆盖（23个测试用例）

#### FIDO2/WebAuthn 硬件密钥认证 (fido2)
- WebAuthn 注册与认证流程
- 硬件密钥验证（YubiKey/TouchID/Windows Hello）
- 多密钥管理与凭据存储
- 会话断言验证与挑战值校验
- 备份恢复码支持
- 完整测试覆盖（60+测试用例）

#### 存储效率仪表板 (storage_efficiency)
- 压缩率统计（文件级/块级）
- 去重效果分析与空间节省计算
- 优化建议引擎与趋势图表数据
- 深度扫描与采样分析
- HTTP API：效率概览/压缩统计/去重统计/优化建议/触发分析
- 完整测试覆盖

#### LXC容器快照管理 (lxccontainer)
- 容器生命周期管理（创建/启动/停止/重启/删除）
- 快照创建、恢复、删除与列表
- 网络管理（网桥/IP分配）
- 模板管理与资源限制验证
- 统一管理器整合容器、网络、模板、快照
- 完整测试覆盖（20+测试用例）

### 修复

#### CI/CD 修复
- 修复lxccontainer测试编译错误（添加validateResources和统一Manager类型）
- 修复ransomware包类型定义缺失（RansomwareDetector、DetectionRule）
- 修复 lxccontainer 测试中模板引用错误（"x" -> 有效模板名）
- 修复 storage_efficiency 未使用的 sort 导入

---

## v2.616.0 (2026-06-23) - 分布式共识/不可变审计/勒索检测修复

### 新增功能

#### 分布式共识引擎 (consensus)
- 基于 Raft 算法实现多 NAS 节点集群协调
- 领导者选举与日志复制
- 集群成员动态管理
- 快照与日志压缩
- HTTP API：状态查询、成员管理、日志提议

#### 不可变审计日志 (immutableaudit)
- 基于 SHA-256 哈希链实现防篡改审计追踪
- 每条记录包含前一条哈希值，形成不可篡改链式结构
- 支持 Merkle 树批量验证
- 提供完整性验证与链断裂告警
- 支持 JSON 格式导出

### 修复

#### 勒索软件检测修复 (ransomware)
- 补充 RansomwareDetector 核心实现
- 修复 CI 编译错误
- 实现检测规则管理、活动记录、统计信息

---

## v2.615.0 (2026-06-22) - GPU管理增强/MCP协议增强/安全审计修复/资源统计更新

### 新增功能

#### GPU管理增强 (gpumanager)
- 多厂商GPU统一管理（NVIDIA/AMD/Intel）
- 显存智能分配与回收机制
- GPU温度实时监控与告警阈值设置
- 性能调优配置（功耗模式、频率限制）
- 多模型并发推理支持优化
- 对标飞牛fnOS AMD GPU适配、群晖DSM GPU管理

#### MCP协议增强 (mcpserver2)
- MCP Server v2协议支持
- 工具注册中心优化，支持动态工具发现
- 资源管理增强，支持文件系统资源暴露
- HTTP/Stdio双传输协议完善
- 安全沙箱隔离机制强化
- 对标群晖AI Console MCP协议集成

### 修复

#### 安全审计修复
- 修复CVE漏洞检测误报问题
- 修复安全评分计算边界条件
- 修复审计日志时间戳格式问题
- 修复合规检查规则匹配逻辑
- 增强敏感数据脱敏算法准确性

#### 资源统计更新
- 更新Go源码统计至最新版本
- 优化资源使用统计API响应格式
- 修复内存使用统计精度问题
- 更新存储效率计算公式

### 优化
- 优化GPU监控数据采集频率
- 改进MCP工具调用性能
- 增强安全审计报告可读性
- 优化资源统计缓存策略

### 竞品功能对标
- 对标群晖DSM NEXT：AI Console MCP协议、GPU管理
- 对标飞牛fnOS：AMD GPU硬件加速适配
- 对标TrueNAS 26：云管理功能、安全审计

---

## v2.614.0 (2026-06-22) - QUIC传输层/WebAssembly运行时/测试修复

### 新增功能

#### QUIC传输层 (quictransport)
- 高性能QUIC协议传输层实现
- 支持多路复用和低延迟通信
- TLS 1.3加密传输
- 连接管理和统计
- 对标现代云原生传输方案

#### WebAssembly运行时 (wasmruntime)
- 轻量级WebAssembly模块运行环境
- 支持模块加载/卸载/实例管理
- 资源限制和沙箱隔离
- 函数调用和性能统计
- 对标TrueNAS插件扩展机制

### 修复
- 修复 filepreview 测试 TempDir 清理顺序问题
- 修复 Staged Release CI 中的测试失败问题

### 优化
- 优化测试稳定性
- 清理无关文件

---

## v2.613.0 (2026-06-22) - 智能文件标签/API密钥管理

### 新增功能
- **智能文件标签 (filetag)** - 文件标签管理，支持标签创建/分配/搜索/批量操作，对标群晖共享标签
- **API密钥管理 (apikey)** - 用户API密钥全生命周期管理，支持创建/撤销/权限控制/审计日志，对标TrueNAS API Keys

---

## v2.612.0 (2026-06-21) - HDD去重压缩/网络FEC/SMB有状态HA/AI数据脱敏/企业即时通讯

### 新增功能

#### HDD去重压缩引擎 (hdddedup)
- 对标群晖 DSM 7.4 HDD级后处理去重与压缩功能
- 基于内容哈希的数据块去重（4KB-1MB可配置块大小）
- 多种压缩算法支持（LZ4、ZSTD、GZIP）
- 异步后台处理，不影响前台IO性能
- 灵活的调度策略（按时间窗口执行）
- 存储节省统计与效率报告

#### 网络前向纠错 (networkfec)
- 对标 TrueNAS 26 网络FEC功能
- 支持 Reed Solomon/XOR/卷积码 多种FEC模式
- 可配置数据分片和校验分片比例
- 实时丢包恢复与统计
- 网络接口级FEC配置
- 低开销高效纠错

#### SMB有状态HA故障转移 (smbstatefulha)
- 对标 TrueNAS 26 SMB Stateful HA Failover
- SMB会话状态保持与无缝迁移
- 虚拟IP自动漂移
- 心跳检测与故障自动转移
- 会话实时同步到备用节点
- 可配置故障转移超时与回切策略

#### AI Console数据脱敏增强 (aiconsoledatamask)
- 对标群晖 DSM 7.4 AI Console数据脱敏功能
- 内置敏感信息模式识别（邮箱/手机/身份证/信用卡）
- 多种脱敏策略（部分遮挡/替换/哈希/加密/移除）
- 正则表达式自定义模式
- 批量脱敏处理
- 脱敏缓存与性能优化
- 脱敏统计与审计日志

#### 企业即时通讯 (chatplus)
- 对标群晖 ChatPlus & Meet 功能
- 私聊/群组/频道多场景支持
- 消息类型丰富（文本/图片/文件/代码/Markdown）
- 消息搜索与历史记录
- 用户在线状态管理
- 频道权限与管理员体系
- 多语言翻译支持（中英日韩）
- 消息编辑与删除
- 文件附件支持

### 竞品功能对标
- 对标群晖 DSM 7.4 HDD去重压缩、AI Console数据脱敏、ChatPlus企业通讯
- 对标 TrueNAS 26 网络FEC、SMB有状态HA故障转移
- 全面提升企业级存储与协作能力

---

## v2.611.0 (2026-06-21) - GPU硬件检测/ACL权限审计

### 新增功能

#### GPU硬件检测与监控 (gpudetect)
- 自动检测NVIDIA/AMD/Intel GPU
- GPU显存、温度、利用率实时监控
- CUDA/ROCm/OpenCL后端自动选择
- nvidia-smi/rocm-smi/lspci多源检测
- 运行时统计信息更新

#### ACL权限审计引擎 (aclaudit)
- 细粒度访问审计日志（13种权限类型）
- 访问模式追踪与分析
- 异常行为检测（非工作时间、高频访问、拒绝访问）
- 审计报告生成
- 数据导出JSON格式
- 可配置保留策略

### 竞品功能对标
- 对标飞牛fnOS AMD GPU适配（GCN5-RDNA4）
- 对标飞牛fnOS 企业级ACL权限管理（13种细分选项）
- 对标群晖DSM GPU信息显示
- 对标TrueNAS 存储架构设计

---

## v2.610.0 (2026-06-21) - AI安全威胁检测/智能分层成本优化/集群故障转移/分布式代理/知识库/语义搜索

### 新增功能

#### AI安全威胁检测 (threatdetect)
- 对标群晖 ActiveProtect Manager 2.0
- 暴力破解检测（5分钟内5次失败自动告警）
- 勒索软件行为检测（大量文件重命名自动隔离）
- 批量删除检测与告警
- 数据外泄检测（1小时超1GB传输告警）
- 文件启发式扫描（.encrypted/.locked等可疑扩展名）
- 隔离区管理与释放
- 安全事件解决与误报标记
- 自定义检测规则引擎

#### 分布式智能管理代理 (dsmaagent)
- 对标群晖 DSM Agent 自动化工作流
- 任务编排与步骤执行引擎
- 优先级调度与并发控制
- 任务状态追踪与超时处理

#### 本地AI知识库 (localknowledgebase)
- 知识条目管理与分类
- 向量化存储与相似度检索
- 本地AI推理数据支撑

#### 语义搜索引擎 (semanticsearch)
- 文档索引与全文搜索
- 向量相似度计算
- 多维度过滤与排序

#### 智能分层存储增强 (smarttiering)
- 成本优化报告生成（CostOptimizationReport）
- 预测性分层（PredictiveTiering）- 基于历史访问模式预测文件热度
- 存储层级成本分析与节省建议

#### 集群管理增强 (clustermanager)
- 故障转移检查与自动切换（PerformFailover）
- 负载均衡信息分析（GetLoadBalance）
- 最优节点选择（GetBestNode）

### 修复
- RBAC 权限管理测试用例补全

## v2.609.0 (2026-06-20) - 智能冗余/多云成本/联邦存储/边缘计算/API网关/合规审计

### 新增功能

#### 智能冗余引擎 (smartredundancy)
- 多节点集群管理与健康监控
- 多种冗余策略（Mirror/RAID5/RAID6/Triple/Erasure Coding）
- 智能数据放置决策引擎
- 自动故障转移与恢复
- 集群健康度评分

#### 多云成本优化 (smartmulticloud)
- 多云账号统一管理（AWS/Azure/GCP/阿里云/腾讯云/华为云）
- 存储成本追踪与分析
- 成本预测与趋势分析
- 智能优化建议（热/温/冷存储分层）
- 成本Breakdown报表

#### 联邦存储联盟 (smartfederation)
- 跨集群联邦管理
- 全局命名空间
- 数据同步策略与任务管理
- 跨集群查询
- 集群状态监控

#### 边缘计算网关 (smartedge)
- IoT设备注册与管理
- 实时数据采集与缓冲
- 边缘规则引擎（阈值告警）
- 数据管道配置
- 设备状态监控

#### API网关增强 (smartapi)
- API密钥生命周期管理
- 请求限流与配额控制
- API使用统计与分析
- 请求日志记录
- 端点注册与管理

#### 合规审计引擎 (smartcompliance)
- 多标准合规支持（GDPR/HIPAA/SOC2/ISO27001/PCI）
- 自动化合规审计
- 零信任访问控制策略
- 访问审计日志
- 合规评分与报告

---

## v2.605.0 (2026-06-20) - 桌面管理增强/文件请求/IP防护/AI翻译

### 新增功能
- **桌面管理增强 (desktopmanager)** - 新增壁纸管理（填充/拉伸/平铺/居中模式）、图标锁定功能、桌面布局持久化、网格对齐优化，对标飞牛fnOS桌面整理功能
- **文件请求管理 (filerequest)** - 完整的文件收集请求系统，支持创建请求、生成分享链接、匿名上传、过期管理、上传限制、统计分析，对标群晖Drive文件请求功能
- **IP防护 (ipguard)** - IP访问控制与威胁检测，支持IP封禁/解封、访问频率限制、暴力破解防护、规则引擎、告警系统，对标飞牛fnOS IP防护功能
- **AI实时翻译 (aitranslator)** - 多语言实时翻译引擎，支持13种语言、翻译记忆、术语表、缓存优化、批量翻译，对标群晖ChatPlus实时翻译功能

### 竞品调研更新
- **飞牛fnOS 2026-06-18**: 局域网直播源支持、相册AI设置优化、P2P文件共享、应用商店更新
- **群晖 COMPUTEX 2026**: DSM 7.4发布、DSM Agent AI助手、ChatPlus/Meet企业通讯、AI文件搜索
- **TrueNAS 26**: WebShare浏览器共享、TrueSearch全文搜索、LXC容器、勒索检测

### 优化
- 优化桌面管理器API接口，支持壁纸和锁定操作
- 完善文件请求模块，增加统计和管理功能
- 增强IP防护规则引擎，支持多种防护策略
- 改进AI翻译缓存机制，提升翻译效率

### 版本号
- v2.604.0 → v2.605.0

---

## v2.604.0 (2026-06-17) - 容器安全卫士/智能磁盘AI/远程访问

### 新增功能
- **容器安全卫士 (containerguardian)** - 完整容器安全扫描系统，支持CVE漏洞扫描、CIS Docker Benchmark合规检查、镜像签名验证、敏感数据泄露检测、安全评分（A/B/C/D/F）、自动修复建议、安全报告生成（JSON/HTML）、HTTP API处理器
- **智能磁盘AI (smartdiskai)** - SMART数据采集与分析引擎、健康评分系统（0-100加权评分）、线性回归故障预测、温度趋势分析与告警、磁盘生命周期管理（SSD磨损均衡/保修追踪）、数据迁移建议引擎、维护建议引擎、仪表板数据构建
- **远程访问 (remoteaccess)** - P2P连接管理器、NAT类型检测（STUN）、中继服务器管理、带宽自适应管理、访问控制（ACL规则+节点认证）、加密隧道管理、DDNS支持、HTTP API处理器

### 竞品对标
- 对标群晖 DSM 7.3：容器安全扫描、AI辅助分析
- 对标 TrueNAS 26：数据分层、存储管理
- 对标飞牛 fnOS：P2P远程访问、智能磁盘预测

### 修复
- 修复 containerguardian 类型冲突（统一 SeverityLevel 类型定义）
- 修复 appmarket types.go 重复类型声明
- 修复 CI 测试兼容性

## v2.603.0 (2026-06-15) - MCP服务器集成/事件总线/AI工作流引擎

### 新增功能
- **MCP服务器集成 (mcpserver)** - Model Context Protocol服务器，支持工具注册与发现、资源暴露、提示词模板管理、HTTP/Stdio传输、安全沙箱，对标群晖AI Console MCP支持
- **事件总线 (eventbus)** - 系统级事件驱动架构，支持发布订阅、通配符订阅、事件过滤与转换、Webhook集成、死信队列、事件关联聚合
- **AI工作流引擎 (aiworkflow)** - AI驱动自动化工作流，支持自然语言触发、条件分支、并行执行、审批网关、工作流模板、执行历史审计

### 竞品调研更新
- **群晖 COMPUTEX 2026**: DSM NEXT + AI Console（MCP协议集成）、DSM Agent私有AI助手、ActiveProtect Manager 2.0 AI威胁检测、GS3400分布式存储（48节点/70GB/s/13.8PB）、PAS7700全闪阵列（1.65PB）
- **TrueNAS 26 Beta**: OpenZFS 2.4 + Linux 6.18 LTS + 10X性能提升 + 混合闪存池优化

---

## v2.602.0 (2026-06-14) - 智能存储分层重构/SMB多通道/ZFS增强

### 新增功能
- **智能存储分层重构 (smarttier2)** - 基于机器学习的数据分层引擎，支持冷热数据自动迁移、访问模式学习、分层策略优化、性能预测
- **SMB多通道重构 (smbsmart2)** - SMB多通道优化，支持多网卡绑定、带宽聚合、故障转移、负载均衡
- **ZFS增强引擎 (zfsenhanced)** - ZFS高级功能，支持智能压缩、去重优化、快照管理、数据完整性校验

### 修复
- 修复 smartcacheprefetch 导出字段问题
- 修复 CI 编译错误

---

## v2.601.0 (2026-06-13) - AI代理编排/集群监控/混合存储池

### 新增功能
- **AI代理编排 (aiagentorch)** - 多AI代理协同工作，支持任务分解、代理调度、结果聚合、错误恢复
- **集群监控 (clustermanager)** - 多节点集群管理，支持节点发现、健康监控、负载均衡、故障转移
- **混合存储池 (hybridpool)** - 混合存储管理，支持SSD/HDD混合池、自动分层、性能优化

---

## v2.600.0 (2026-06-12) - 块级去重2.0/AI工作流可视化/智能分层

### 新增功能
- **块级去重2.0 (blockdedup2)** - 高性能块级去重，支持可变块大小、哈希索引、压缩优化
- **AI工作流可视化 (aiworkflowviz)** - 工作流可视化编辑器，支持拖拽设计、条件分支、并行执行
- **智能分层 (smarttierml)** - 基于ML的数据分层，支持访问模式预测、自动迁移策略

---

## v2.599.0 (2026-06-11) - 容器安全卫士/智能磁盘AI/远程访问

### 新增功能
- **容器安全卫士 (containerguardian)** - 容器安全扫描与防护
- **智能磁盘AI (smartdiskai)** - 磁盘健康预测与智能管理
- **远程访问 (remoteaccess)** - P2P远程访问与NAT穿透

---

## v2.598.0 (2026-06-10) - MCP服务器/事件总线/AI工作流

### 新增功能
- **MCP服务器 (mcpserver)** - Model Context Protocol集成
- **事件总线 (eventbus)** - 系统级事件驱动架构
- **AI工作流 (aiworkflow)** - AI驱动自动化工作流

---

## v2.597.0 (2026-06-09) - 智能存储分层/SMB多通道/ZFS增强

### 新增功能
- **智能存储分层 (smarttiering)** - 数据自动分层管理
- **SMB多通道 (smbmultichannel)** - SMB协议多通道优化
- **ZFS增强 (zfsenhanced)** - ZFS高级功能增强

---

## v2.596.0 (2026-06-08) - AI代理编排/集群监控/混合存储

### 新增功能
- **AI代理编排 (aiagentorch)** - 多AI代理协同
- **集群监控 (clustermon)** - 集群状态监控
- **混合存储 (hybridstorage)** - 混合存储池管理

---

## v2.595.0 (2026-06-07) - 块级去重/AI工作流/智能分层

### 新增功能
- **块级去重 (blockdedup)** - 高性能块级去重
- **AI工作流 (aiworkflow)** - 自动化工作流引擎
- **智能分层 (smarttier)** - 智能数据分层

---

## v2.594.0 (2026-06-06) - 容器安全/磁盘AI/远程访问

### 新增功能
- **容器安全 (containersec)** - 容器安全防护
- **磁盘AI (diskhealthai)** - 磁盘健康AI分析
- **远程访问 (remoteaccess)** - 远程访问管理

---

## v2.593.0 (2026-06-05) - MCP集成/事件系统/工作流

### 新增功能
- **MCP集成 (mcpintegration)** - MCP协议集成
- **事件系统 (eventsystem)** - 事件驱动架构
- **工作流 (workflow)** - 工作流自动化

---

## v2.592.0 (2026-06-04) - 智能相册/存储成本/应用商店

### 新增功能
- **AI智能相册 (aiphoto)** - 人脸检测与识别、场景分类、智能搜索、时间线+地图视图、自动去重
- **智能存储成本优化 (smartstoragecostopt)** - 存储成本分析引擎、多层级存储画像、ROI分析与趋势预测、预算告警机制
- **应用商店增强 (appstore)** - 应用分类与推荐、依赖关系管理、版本管理、安全扫描
