# NAS-OS 🖥️

基于 Go 的家用 NAS 系统，支持 btrfs 存储管理、SMB/NFS 共享、Web 管理界面。

> **最新版本**: v3.24.6 Stable（文档同步 2026-09-05）  
> **文档索引**: [docs/README.md](docs/README.md)  
> **项目结构**: [STRUCTURE.md](docs/STRUCTURE.md)（含 **Core / Full 编译面**） · **运维**: [ops-packages.md](docs/ops-packages.md) · **架构**: [ARCHITECTURE.md](docs/ARCHITECTURE.md)  
> **默认**: 仅 Core 能力面 + Core 二进制（`make build`）；套件走 `packages.*` / 应用中心；完整产品需 `make build-full`（`-tags nasd_full`）  



> **CI/CD**: [![CI/CD](https://github.com/crazyqin/nas-os/actions/workflows/ci-cd.yml/badge.svg)](https://github.com/crazyqin/nas-os/actions)
> **Docker**: [![Docker](https://img.shields.io/badge/ghcr.io-crazyqin%2Fnas--os-blue?logo=docker)](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)
> **Release**: [v3.24.6](https://github.com/crazyqin/nas-os/releases/tag/v3.24.6)（最新已发布 tag）

> **怎么读**：想跑起来 → 直接跳「快速开始」；想知道默认有什么 → 「默认交付面」；想看全部能力 → 「扩展能力」（130+ 项折叠清单）；升级历史 → 「版本状态」。

## 默认交付面（请先读）

默认 `nasd` **只启用 Core**：身份 / 存储 / 网络 / 共享（SMB·NFS）/ 系统。  
主配置面（ADR-0001 Stage 3）：`packages.recommended_system: false`、`packages.enabled: []`。  
`modules.optional` / `modules.extensions` **已弃用**，仍双读兼容，启动时会 warn。

> **诚实说明**  
> - 下文大量「对标 / 新增」能力多在 **Package Surface** 或 **`internal/lab/`**，**默认不进 `nasd` 热路径**。  
> - **二进制双面**：默认 `make build` / 无 tag = **Core-only**（约 47MB；不链接 docker/vm/photos、HTTP extensions、bleve、cluster、downloader）；完整产品+扩展需 `make build-full` 或 `-tags nasd_full`（约 118MB）。**Docker 镜像默认也是 Core**（`Dockerfile` `ARG BUILD_TAGS=` 为空）；完整镜像：`docker build --build-arg BUILD_TAGS=nasd_full …`。  
> - `packages.recommended_system` 只开 **8 个编目产品**（docker/vm/photos/…），**不会**再拉 tunnel/trash/ftp 等 bulk 附属。  
> - 旧版全家桶附属仅在弃用开关 `modules.optional: true` 时构造。  
> - Docker 默认 **非 privileged** + bridge + `127.0.0.1:8080`；全特权见 `docker-compose.privileged.yml`。

| 能力 | 默认是否开启 | 如何启用 |
|------|--------------|----------|
| 用户 / 卷 / SMB / NFS / 网络 / 健康探针 | **是** | 无需额外配置 |
| Docker / VM / 相册 / AI / 备份 / 云同步等产品管理器 | **否** | `packages.recommended_system: true` 或 `packages.enabled` |
| tunnel / trash / ftp / webdav / monitor 等 bulk 附属 | **否** | 仅 `modules.optional: true`（弃用） |
| WriteOnce / 本地 LLM / CLIP / MCP 等差异化 | **否** | packages 和/或 Lab 源码 |
| 已编目 HTTP Extension（7） | **否** | 应用中心或 `packages.enabled` |
| Lab 实验包（~600） | **否** | 不在默认路径；不可经 packages 列表加载 |
| 容器应用商店（Docker 模板） | **否** | `apps.html`（需 docker 产品启用） |

```yaml
# 示例：启用官方推荐产品面 + 一个 HTTP 扩展
packages:
  recommended_system: true
  enabled: [voicehub]
```

下文「差异化能力 / 企业级存储」描述的是仓库内已实现或对标中的能力面，**不是**默认开机全开。  
仓库怎么分层、入口在哪、Lab/套件怎么分：见 **[项目结构图](docs/STRUCTURE.md)**；进程与 API 边界见 [架构说明](docs/ARCHITECTURE.md)。

## 核心功能（默认 `nasd`）

> **启用口径**：`默认` = Core 热路径；`recommended` = `packages.recommended_system: true`；`extensions` = `packages.enabled`；`Lab` = 仅源码，默认不加载。源码「完成」≠ 默认开机开启。

| 模块 | 说明 | 启用 |
|------|------|------|
| 💾 btrfs 存储 | 卷/子卷/快照/RAID | **默认** |
| 🌐 Web 界面 | Core WebUI（`/webui`） | **默认** |
| 📁 文件共享 | SMB/NFS | **默认** |
| 👥 用户权限 | 账户/RBAC 基础 | **默认** |
| 🔒 安全认证 | 会话/JWT/强制改密 | **默认** |
| 📊 系统健康 | Core `Health()` 聚合 | **默认** |
| 🐳 Docker 镜像发布 | 多架构容器产物 | 部署产物（非运行时模块） |
| 📊 监控告警 | 指标/多通道通知 | optional |
| ⚡ 性能优化 | 缓存/工作池等 | optional |
| 🛡️ 集群支持 | 多节点/负载均衡 | optional |

---

## 🌟 差异化能力（需显式启用，非默认）

| # | 功能 | 说明 | 启用提示 |
|---|------|------|----------|
| 1 | 🔒 **WriteOnce 不可变存储** | WORM / 防篡改归档 | optional 产品面或对应模块 |
| 2 | 🤖 **本地 LLM 服务** | Ollama + OpenAI 兼容 API | `packages.recommended_system: true` + AI 部署 |
| 3 | 🔐 **AI 以文搜图** | CLIP 本地推理搜图 | optional + Photos/AI |
| 4 | ☁️ **多云存储挂载** | 多云统一挂载 | optional + cloudsync |
| 5 | 🔗 **MCP 服务器集成** | Model Context Protocol | optional / 实验路径 |

> 💡 **竞品对标**: 持续对标 Synology DSM 7.4、TrueNAS 26、飞牛 fnOS、QNAP QuTS hero h6.0；对标能力可能落在 Extension 或 Lab，**以是否进入默认 `nasd` 热路径为准** → [详细分析](docs/competitive-analysis.md)

### 💾 企业级存储相关（部分 optional / Lab）

| # | 功能 | 说明 | 备注 |
|---|------|------|------|
| 1 | 💾 **RAID-Z Expansion** | 在线扩展 RAID-Z vdev | 源码/工具链能力；非 Core 生命周期名 |
| 2 | 🔄 **dRAID 分布式热备** | 分布式热备 RAID | 多为 Lab / 实验路径 |
| 3 | 🏗️ **Enclosure Management** | SES-3/SGPIO 机箱管理 | 视 optional 与硬件环境 |
| 4 | 🔀 **SAS Multipath** | 多路径故障切换 | 视 optional 与硬件环境 |
| 5 | 🧠 **Disk Health AI** | 磁盘健康评分 / 预测 | 非默认 Core；optional 或 Lab |

---

## 扩展能力（需显式启用）

| 类别 | 内容 | 启用方式 |
|------|------|----------|
| 产品套件（8） | docker / vm / photos / ai / backup / cloudsync / downloader / cluster | `packages.recommended_system: true` 或 `packages.enabled`（需 Full 构建） |
| HTTP 扩展（7） | activeprotect / agentworkflow / aiguardrails / compliancescan / deployorch / netdiag / voicehub | 应用中心或 `packages.enabled` |
| 容器应用商店 | 30+ 常用应用一键部署模板 | `apps.html`（需 docker 套件） |
| Lab 实验包（~626） | 实验 / 重复 / 零生产引用实现 | 仅源码，不进默认路径，不可经 packages 加载 |

> 停用语义（路由真卸载 + 内存回收）、启用状态 SSOT 与运维操作见 [docs/ops-packages.md](docs/ops-packages.md)；下方折叠清单为源码口径全量列表。

<details>
<summary>全部可选 / 实验能力清单（130+ 项，源码口径）</summary>

### 扩展功能（optional / extensions / Lab）

| 模块 | 说明 | 启用 |
|------|------|------|
| 📦 容器管理 | Docker 容器/镜像/网络/卷 | optional |
| 🖥️ 虚拟机管理 | VM/ISO/快照 | optional |
| 🗂️ 存储分层 | 热/冷分层/SSD 缓存 | optional |
| 🗜️ 压缩存储 | 透明压缩 | Lab |
| 🔄 存储复制 | 跨节点同步/灾备 | optional |
| 📸 快照策略 | 定时快照/保留策略 | optional |
| 🎯 iSCSI 目标 | Target/LUN/CHAP | optional |
| 📁 WebDAV | WebDAV 协议 | optional |
| 📡 FTP/SFTP | 文件传输协议 | optional |
| 📊 配额管理 | 用户/组/目录配额 | optional |
| 🗑️ 回收站 | 安全删除/恢复 | optional |
| 🤖 AI 分类 | 照片/文件智能分类 | optional |
| 📜 版本控制 | 文件历史版本 | optional |
| ☁️ 云同步 | 多云双向同步 | optional |
| 🔄 数据去重 | 文件/块级去重 | optional |
| 🔍 智能搜索 | 全文/语义搜索 | optional |
| 🏷️ 文件标签 | 标签分类 | optional |
| 📈 预测分析 | 磁盘健康/容量趋势 | optional / Lab |
| 🔗 LDAP/AD | 企业目录集成/统一认证 | optional |
| 📋 自动化引擎 | 工作流/定时任务/触发器 | optional |
| 🧠 CXL内存池化 | CXL 1.1/2.0/3.0设备管理、NUMA感知分层、多策略分配 | Lab / optional |
| 🔢 向量数据库 | 嵌入式向量DB、HNSW/IVF索引、余弦/欧氏/内积相似度、RAG支持 | Lab / optional |
| 🛡️ 气隙备份 | 物理隔离备份、WORM保护、链式校验、定时断连、灾难恢复演练 | Lab / optional |
| ⚡ DPDK网络加速 | 用户态网络栈、RSS多队列、流表规则、QoS流量分类、零拷贝 | Lab / optional |
| 🔌 SmartNIC/DPU卸载 | OVS/IPsec/TLS/压缩/RDMA卸载、SmartNIC设备管理与监控 | Lab / optional |
| 🎮 GPU推理服务 | 多模型并发、模型热加载、显存管理、推理队列、多精度(FP16/INT8/INT4) | Lab / optional |
| 🖥️ **GPU管理增强** | 多厂商GPU统一管理、显存智能分配、温度监控、性能调优 | Lab / optional |
| 📰 下载器 | Transmission/qBittorrent 集成 | optional |
| 🔗 **MCP协议增强** | MCP Server v2、工具注册中心、资源管理优化、HTTP/Stdio双传输 | Lab / optional |
| 🚪 门禁管理 | 设备管理、卡片授权、AI行为分析（对标群晖AC100） | Lab / optional |
| 🔎 AI深度搜索 | 语义搜索、OCR、视觉分析（对标群晖Deep Search） | optional |
| 🤖 智能运维代理 | 工作流调度、健康监控、自动修复（对标群晖DSM Agent） | optional |
| 💬 本地通讯套件 | 即时通讯、视频会议、AI摘要/翻译（对标群晖ChatPlus） | Lab / optional |
| 🌐 P2P远程访问 | NAT穿透、端到端加密、连接管理（对标飞牛FN Connect） | optional |
| 🔍 增强搜索 | 全文索引、语义搜索、Spotlight兼容（对标TrueNAS TrueSearch） | optional |
| 📂 增强文件共享 | FIPS加密、在线预览、浏览器管理（对标TrueNAS WebShare） | optional |
| 🎬 媒体服务 | HLS/DASH 流媒体/转码/字幕 | optional |
| 🖼️ 照片管理 | 相册/AI 分析/缩略图 | optional |
| 📝 在线文档 | OnlyOffice 集成/协作编辑 | optional |
| 🔧 网络诊断 | Ping/Traceroute/DNS/端口扫描 | optional |
| 🛡️ 安全增强 | 限流/MFA/密码策略/会话管理 | optional |
| 💾 智能备份 | 增量备份/多压缩算法/加密/版本管理 | optional |
| ☁️ 网盘挂载 | 多云存储挂载/本地化访问/透明读写 | optional |
| 🔐 AI 脱敏 | PII 智能识别/隐私保护/多提供商支持 | optional |
| 🔒 WriteOnce | 不可变存储/防篡改/合规归档 | optional |
| 🛡️ AMFA | 智能多重验证/自适应安全策略 | optional |
| 🚫 自动封锁 | SMB/NFS 防暴力破解/自动封禁 | optional |
| 🔥 Hot Spare | 热备盘自动切换/RAID自愈 | optional |
| 🔗 分布式共识 | Raft算法/领导者选举/日志复制/集群协调 | optional |
| 🔒 不可变审计 | SHA-256哈希链/防篡改/Merkle验证/完整性告警 | optional |
| 📊 Fusion Pool | 智能分层存储/热冷数据分离 | optional |
| 📈 SSD健康监控 | 寿命预测/三级预警/健康评分 | optional |
| 🤖 AI相册 | CLIP以文搜图/智能照片搜索 | optional |
| 🧑 **人脸识别** | **本地AI人脸检测/聚类/人物相册** | optional |
| 🌐 内网穿透 | 远程访问/零配置 | optional |
| 🔐 **数据遮罩** | **AI训练数据脱敏/隐私保护** | optional |
| 💰 **成本分析** | **存储成本统计/趋势预测** | optional |
| 🤖 AI服务独立镜像 | GPU加速推理/CLIP模型/本地LLM | optional |
| 📊 **文件活动监控** | **实时文件操作追踪/事件记录/对标群晖Active Insight** | optional |
| ⚙️ **自定义事件规则** | **用户自定义监控规则/路径过滤/告警触发** | optional |
| 🖥️ **Fleet多节点监控** | **多节点集中监控/健康聚合/跨节点告警联动** | optional |
| 📦 **应用模板商店** | **30+常用应用一键部署模板/对标TrueNAS TrueCharts** | optional |
| 🛡️ **AdGuard Home** | **DNS广告过滤/家庭网络防护/一键部署** | optional |
| 🖼️ **Immich** | **AI照片管理/Google Photos开源替代/一键部署** | optional |
| 🔄 **n8n** | **工作流自动化/低代码集成平台/一键部署** | optional |
| 📄 **Paperless** | **文档数字化管理/OCR智能分类/一键部署** | optional |
| 📤 **File Request** | **安全文件收集/链接分享/配额限制** | optional |
| 📊 **Alert Digest** | **告警摘要/批量通知/多通道配置** | optional |
| 💾 **Immutable Backup** | **不可变备份/SHA-256校验/审计日志/合规归档** | optional |
| 🖥️ **Storage Health Dashboard** | **统一健康监控/容量趋势预测/SMART告警** | optional |
| 🤖 **AI Console** | **本地LLM集成/隐私数据自动脱敏/OpenAI兼容API** | optional |
| 📧 **Email Moderation** | **邮件审核管控/多级审核策略/审计追踪** | optional |
| 🔄 **Smart Domain Sync** | **选择性OU同步/最小权限原则** | optional |
| 📊 **SMB 审计日志** | **SMB操作审计/文件追踪/用户记录/多通道告警** | optional |
| 🎬 **智能海报墙** | **影视自动刮削/海报展示/分类浏览/对标飞牛** | optional |
| ♻️ **数据生命周期** | **自动化迁移策略/成本优化/多层存储** | optional |
| 🔐 **KMIP密钥管理** | **企业级密钥管理协议/密钥轮换/对标TrueNAS** | optional |
| 🛡️ **FIPS合规加密** | **FIPS 140合规/自检/审计/对标TrueNAS** | optional |
| 📧 **邮件服务** | **SMTP/IMAP服务/域名管理/用户配额/对标群晖MailPlus** | optional |
| 📊 **活动洞察** | **设备健康监控/实时指标采集/多级告警/对标群晖Active Insight** | optional |
| 🔐 **双因素认证** | **TOTP/HOTP支持/二维码生成/备份码管理/对标TrueNAS 2FA** | optional |
| 🌐 **SMB 多通道聚合** | **多网卡并行传输/自适应负载均衡/自动故障切换** | optional |
| 👤 **无 Root 管理员** | **Rootless Admin/命令白名单/审计日志/对标TrueNAS** | optional |
| 📦 **LXC 容器沙箱** | **轻量级隔离环境/资源限制/内置模板/对标TrueNAS** | optional |
| 🔄 **智能数据迁移** | **SHA-256校验/带宽控制/重试策略/迁移历史** | optional |
| 🛡️ **合规仪表盘** | **安全评分/CIS/STIG/GDPR合规检查/趋势追踪** | optional |
| 💰 **预算告警管理** | **多级预算/三级告警/成本估算/趋势分析** | optional |
| 💰 **成本分析报告** | **存储成本统计/趋势预测/资源计费** | optional |
| 📡 **Prometheus 监控** | **原生指标导出/Grafana预置模板/实时WebSocket** | optional |
| 🔄 **迁移助手** | **多平台配置迁移/数据校验/进度追踪/回滚支持** | optional |
| 🔒 **NVMe-oF TLS加密** | **NVMe/TCP传输加密/mTLS双向认证/TLS 1.3** | optional |
| 📋 **NVMe-oF 访问审计** | **NVMe-oF操作审计/JSONL持久化/事件过滤** | optional |
| 📊 **存储效率统计** | **压缩比/去重比/存储池效率分析API** | optional |
| 📋 **合规报告** | **WriteOnce合规链/审计保留/Excel导出/GDPR脱敏** | optional |
| 🔌 **NVMe over Fabric** | **NVMe-oF/TCP目标管理/子系统/命名空间/端口/主机访问控制** | optional |
| 🔒 **全卷加密** | **LUKS全卷加密/密钥管理/KMIP支持/AES-NI硬件加速** | optional |
| 💾 **块级备份** | **块级增量备份/去重压缩/速度提升2倍+/调度恢复API** | optional |
| 🐳 **Docker Compose UI** | **YAML可视化编辑/多容器一键部署/模板市场/日志监控** | optional |
| 🔐 **SSO协议扩展** | **OAuth2/OIDC/SAML/第三方应用SSO集成/客户端管理** | optional |
| 🔗 **全局文件锁** | **跨节点文件锁定/冲突检测/分布式锁服务** | optional |
| 🛡️ **设备信任** | **设备指纹识别/信任管理/新设备风险评估** | optional |
| 📦 **应用商店增强** | **批量安装/依赖解析/推荐引擎/沙箱隔离** | optional |
| 🌐 **VPN Server** | WireGuard/OpenVPN内置/用户授权/连接监控 | optional |
| 📦 **Git Server** | 自托管Git仓库/Webhook/SSH+HTTP/权限管理 | optional |
| 🖥️ **远程桌面网关** | 浏览器RDP/VNC/剪贴板同步/文件传输 | optional |
| 🏠 **家庭仪表盘** | 智能家居+NAS统一面板/可配置Widget | optional |
| 💰 **成本预测** | 存储成本趋势预测/优化建议/多维分析 | optional |
| 📊 **预算管理** | 企业级存储预算/审批流程/超支告警 | optional |
| 💬 **Chat即时通讯** | WebSocket实时消息/群组/频道/消息搜索/未读计数 | Lab / optional |
| ⚡ **SMB Multichannel增强** | 多通道并行传输/带宽监控/会话管理 | optional |
| 🌐 **网络测速** | 下载/上传/延迟测试/历史记录/服务器管理 | optional |
| ☁️ **云存储成本分析** | 多云费用追踪/对比分析/优化建议/告警 | optional |
| 🔒 **安全评分** | 系统安全态势/多维检查/等级评分/历史趋势/改进建议 | optional |
| 📋 **审计报告** | 安全审计/合规检查/事件日志/安全扫描/JSONL导出 | optional |
| 🔄 **Smart Dedup智能去重** | 内容哈希去重/块级去重/空间节省50%+/去重回滚 | optional |
| ☁️ **CloudSync Manager** | 6+云平台统一管理/智能同步策略/冲突解决引擎 | optional |
| 🏥 **Health Probe健康探针** | 系统全面检测/健康评分/趋势分析/自动告警 | optional |
| 🛡️ **Security Audit Reporter** | 自动化审计/CIS/STIG/GDPR合规/多格式导出 | optional |
| 💾 **RAID-Z Expansion** | 在线扩展RAID-Z vdev，不停机添加磁盘 | optional |
| 🔄 **dRAID分布式热备** | 分布式热备RAID，加速重建 | optional |
| 🏗️ **Enclosure Management** | SES-3/SGPIO机箱管理，指示灯/温控/电源 | optional |
| 🔀 **SAS Multipath** | SAS多路径故障切换与负载均衡 | optional |
| 🧠 **Disk Health AI** | AI驱动磁盘故障预测与健康评分 | optional |
| ♻️ **智能文件生命周期** | 文件老化分析/自动归档/智能清理/存储回收 | optional |
| ⚖️ **磁盘磨损均衡** | SMART监控/磨损均衡策略/磁盘轮换/寿命预测 | optional |
| 🔍 **统一搜索门户** | 跨模块全局搜索/快捷操作/智能推荐 | optional |
| 📊 **存储健康评分** | 多维度健康评估/趋势分析/智能预警 | optional |
| 🔒 **合规快照审计** | 快照完整性验证/合规性检查/篡改检测 | optional |
| 💿 **系统克隆备份** | 全盘克隆/增量镜像/系统恢复/PXE部署 | optional |
| 🔍 **智能数据分层** | 基于访问模式/冷热分离/多层策略/对标群晖DSM 7.3 | optional |
| 🤖 **本地AI控制台** | 本地LLM管理/多模型切换/隐私脱敏/对标群晖AI Console | optional |
| 🔎 **TrueSearch搜索** | 亚秒级全文搜索/Spotlight兼容/语义搜索/对标TrueNAS 26 | optional |
| 📂 **WebShare浏览器共享** | 浏览器文件管理/FIPS加密/在线预览/对标TrueNAS 26 | optional |
| 🏷️ **共享标签系统** | 跨用户标签同步/权限控制/批量操作/对标群晖Drive 4.0 | optional |
| 📧 **邮件审核控制** | 多级审核策略/审计追踪/违规检测/对标群晖邮件安全 | optional |
| 🔄 **磁盘生命周期管理** | 智能磁盘故障预测/退役建议/舰队管理/预测性维护 | optional |
| 🔍 **数字取证工具** | 事件时间线重建/证据收集/取证报告/安全审计 | optional |
| 🌐 **GeoIP防火墙** | 基于国家/地区的访问控制/威胁情报集成/地理围栏 | optional |
| 💡 **智能存储成本分析** | 多层级成本分析/云vs本地对比/成本优化建议/TCO报告 | optional |
| 🏠 **智能家居中枢** | MQTT/Zigbee/Z-Wave集成/自动化规则引擎/设备分组管理 | optional |
| ⚡ **边缘计算引擎** | IoT数据采集/边缘AI推理(TFLite/ONNX)/离线缓存续传 | optional |
| 🌱 **碳感知调度** | 电网碳强度API/碳足迹计算/绿色能源优先策略 | optional |
| 🔧 **自愈存储** | 后台Scrub调度/Bitrot检测修复/多副本自动同步 | optional |
| 🎭 **数字孪生** | NAS配置快照/虚拟实例/灾难恢复演练/拓扑可视化 | optional |
| 🔐 **后量子加密** | NIST标准算法(Kyber/ML-KEM)/混合加密/密钥轮换 | optional |
| 🎮 **AMD显卡硬件加速** | AMD显卡检测/VA-API视频转码/GCN5-RDNA4全覆盖/对标飞牛fnOS v1.1.29 | optional |
| 🌐 **IPv6网络支持增强** | 双栈网络/隐私扩展/DHCPv6/DNS64/对标群晖DSM 7.4 | optional |
| 🛡️ **安全加固模块** | CVE漏洞检测/安全评分/自动修复/合规检查/对标群晖安全更新 | optional |
| 🤖 **OpenClaw应用集成** | NAS变身AI助手/应用部署/工作流管理/自动化任务/对标飞牛OpenClaw | optional |
| 🎨 **云风格管理界面** | 现代化Web UI/响应式设计/深色模式/可定制仪表板/对标TrueNAS Connect | optional |
| 💰 **存储成本智能分析** | 多维度成本对比/云vs本地/TCO报告/优化建议/成本预测 | optional |
| 🔀 **SMB有状态HA故障转移** | 会话状态跨控制器故障转移保持/SMB Guardian | optional |
| 📊 **数据分层自定义规则** | 按访问频率/时间自动冷热数据分层/用户自定义策略 | optional |
| 📧 **邮件OAuth通知** | 支持Gmail/Outlook OAuth2授权/安全邮件通知 | optional |
| 🔌 **WebSocket API** | JSON-RPC 2.0 + SCRAM-SHA-512认证/实时通信 | optional |
| 🔐 **弹性存储加密** | AES-256-XTS密码解锁加密存储/动态密钥管理 | optional |
| 📋 **邮件审核机制** | 敏感邮件管理员审批工作流/多级审核策略 | optional |

</details>

---

## 快速开始

### 方式一：下载二进制文件 (推荐)

```bash
# 从 Release 下载（示例为最新已发布 tag v3.24.6，按需替换）
VER=v3.24.6

# AMD64 (x86_64)
wget https://github.com/crazyqin/nas-os/releases/download/${VER}/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# ARM64 (Orange Pi 5, Raspberry Pi 4/5)
wget https://github.com/crazyqin/nas-os/releases/download/${VER}/nasd-linux-arm64
chmod +x nasd-linux-arm64
sudo mv nasd-linux-arm64 /usr/local/bin/nasd

# ARMv7 (Raspberry Pi 3, 旧款 ARM；Release 资产名为 nasd-linux-arm)
wget https://github.com/crazyqin/nas-os/releases/download/${VER}/nasd-linux-arm
chmod +x nasd-linux-arm
sudo mv nasd-linux-arm /usr/local/bin/nasd

# 验证安装
nasctl --version
```

### 方式二：Docker 部署

```bash
# 拉取镜像（按 Release tag 或 latest）
docker pull ghcr.io/crazyqin/nas-os:v3.24.6   # 或 :latest


# 运行容器（默认 Core 面：非 privileged + bridge + 127.0.0.1:8080）
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 127.0.0.1:8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v3.24.6


# 查看日志
docker logs -f nasd
```

> 完整体验推荐直接用仓库根的 `docker-compose.yml`（见下文「部署」）。

### 方式三：源码编译

#### 依赖

```bash
# 安装 Go 1.26.1+
# 安装 btrfs 工具
sudo apt install btrfs-progs

# 安装 Samba（如需 SMB 共享）
sudo apt install samba

# 安装 NFS（如需 NFS 共享）
sudo apt install nfs-kernel-server
```

#### 构建

```bash
cd nas-os
go mod tidy
go build -o nasd ./cmd/nasd
go build -o nasctl ./cmd/nasctl
```

### 运行

```bash
# 需要 root 权限（访问磁盘设备）
sudo nasd
```

访问 http://localhost:8080

**首次登录**：
- 用户名：`admin`
- 初始密码：**随机生成**（16 位），写入首次启动日志提示的密码文件
  （`<config_dir>/.admin_password`，如 `/etc/nas-os/.admin_password`，权限 0600）
- 首次登录后系统强制要求修改密码（`MustChangePassword`）

⚠️ **改密后请删除该密码文件。**

## 部署

### Docker 部署
```bash
# 默认：非 privileged + bridge + 127.0.0.1:8080 + /dev/disk
docker compose up -d

# 强制 CSRF（生产推荐）：在 .env 设置 NAS_CSRF_KEY 与 NAS_OS_ENV=production
docker compose -f docker-compose.yml -f docker-compose.secure.yml up -d

# 旧行为：privileged + host 网络（仅在确需时）
docker compose -f docker-compose.yml -f docker-compose.privileged.yml up -d

# 查看日志
docker compose logs -f
```

### 裸机安装
```bash
# 一键安装脚本
curl -fsSL https://raw.githubusercontent.com/crazyqin/nas-os/master/scripts/install.sh | sudo bash

# 或手动安装
sudo ./scripts/install.sh
```

### 系统服务
```bash
systemctl status nas-os
systemctl restart nas-os
journalctl -u nas-os -f
```

## 配置示例

以仓库 `configs/default.yaml` 为准。示意：

```yaml
# 默认监听本机；局域网访问请显式配置并配合防火墙
server:
  host: 127.0.0.1
  port: 8080

paths:
  mount_base: /mnt
  config_dir: /etc/nas-os
  data_dir: /var/lib/nas-os

storage:
  default_profile: single
  auto_scrub: true
  scrub_schedule: "0 2 * * 0"

packages:
  recommended_system: false   # true 时需 Full 二进制（-tags nasd_full）
  enabled: []                 # 如 docker、voicehub；Core 二进制配置了未链接能力会启动失败

auth:
  session_ttl_hours: 24
```

生产建议：`NAS_OS_ENV=production` + `NAS_CSRF_KEY`（见 `.env.example`）。  
用户身份：`config_dir/users.json`。删卷需 JSON `confirm_name`；擦盘还需 `allow_wipe: true`。

## 快速使用

### 1. 创建存储卷
```bash
sudo nasctl volume create mydata --devices /dev/sda1 --raid single
```

### 2. 创建 SMB 共享
```bash
sudo nasctl share create smb public --path /data/public --guest-ok
```

### 3. 创建 NFS 共享
```bash
sudo nasctl share create nfs backup --path /data/backup --network 192.168.1.0/24
```

### 4. 从客户端访问
- **Windows**: `\\<服务器 IP>\public`
- **macOS**: `smb://<服务器 IP>/public`
- **Linux (NFS)**: `sudo mount <服务器 IP>:/backup /mnt/local_backup`

---

## API 接口

### 存储管理（`/api/v1/storage/*` 唯一契约）
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/storage/volumes | 获取卷列表 |
| POST | /api/v1/storage/volumes | 创建卷 |
| DELETE | /api/v1/storage/volumes/:name | 删除卷 |
| POST | /api/v1/storage/volumes/:name/subvolumes | 创建子卷 |
| POST | /api/v1/storage/volumes/:name/subvolumes/:subvol/mount | 挂载子卷（body: `mountPath`） |
| GET | /api/v1/storage/snapshots | 列出全部快照 |
| POST | /api/v1/storage/volumes/:name/snapshots | 创建快照 |
| POST | /api/v1/storage/volumes/:name/snapshots/:snap/restore | 恢复快照（body: `targetName`） |
| POST | /api/v1/storage/volumes/:name/balance | 平衡数据 |
| POST | /api/v1/storage/volumes/:name/scrub | 数据校验 |
| GET | /api/v1/storage/pools | 列出存储池 |

### 共享管理
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/shares | 列出所有共享（SMB+NFS 聚合） |
| GET | /api/v1/shares/status | 服务状态 |
| POST | /api/v1/shares/smb | 创建 SMB 共享 |
| PUT/DELETE | /api/v1/shares/smb/:name | 更新 / 删除 SMB 共享 |
| POST | /api/v1/shares/smb/:name/permission | 设置 SMB 权限 |
| POST | /api/v1/shares/nfs | 创建 NFS 导出 |
| PUT/DELETE | /api/v1/shares/nfs/:path | 更新 / 删除 NFS 导出 |

### 配置管理
系统级配置经配置文件 + `NAS_OS_*` 环境变量加载（启动前校验），**没有**全局
`/api/v1/config` 读写端点；模块级配置走各自子域，如
`GET/PUT /api/v1/shares/smb/config`、`GET/PUT /api/v1/shares/nfs/config`。

### WriteOnce 不可变存储（对象不可变，需 Full 构建）
> 路由实际为 `/api/v1/object-immutable/*`（`internal/objectimmutable`，仅
> `-tags nasd_full` 构建注册），WORM 桶/对象/保留/法律保留/审计：

| 方法 | 路径（前缀 /api/v1/object-immutable） | 说明 |
|------|------|------|
| GET/POST | /buckets | 列出 / 创建桶 |
| GET/PUT | /buckets/:name/lock | 查询 / 设置桶对象锁 |
| PUT/DELETE | /buckets/:name/objects/:key/retention | 设置 / 释放对象保留 |
| PUT | /buckets/:name/objects/:key/legal-hold | 法律保留 |
| GET | /audit-logs · /stats | 审计日志 / 统计 |

完整 API 文档见 [docs/api/](docs/api/) 与 [docs/swagger/](docs/swagger/)。

---

## 📚 文档

| 文档 | 内容 |
|------|------|
| [架构概览](docs/ARCHITECTURE.md) | 系统架构、模块生命周期、API 安全边界 |
| [项目结构](docs/STRUCTURE.md) | 仓库分层：Core / Extension / Lab 与入口 |
| [运维手册](docs/ops-packages.md) | 套件与应用中心：启用 SSOT、停用语义 |
| [API 文档](docs/api/) · [Swagger](docs/swagger/) | REST API 参考（运行时 `/swagger/`） |
| [竞品分析](docs/competitive-analysis.md) | 与群晖 / TrueNAS / 飞牛 / QNAP 对照 |
| [资源统计](docs/resource-stats.md) | 代码规模与包数量 |
| [文档索引](docs/README.md) | 按角色导航 |

---

## 🏆 差异化优势（2026Q2竞品对标）

### 🥇 四大独家功能（竞品均无）

| 功能 | 说明 | 竞品状态 | 价值主张 |
|------|------|----------|----------|
| 🔒 **WriteOnce** | 不可变存储(WORM)，防篡改/防勒索，合规归档 | 群晖/飞牛/TrueNAS均无 | **企业数据安全终极防线** - 金融/医疗/政务必备 |
| 🤖 **本地LLM服务** | Ollama集成，OpenAI兼容API，本地AI推理 | 群晖有本地LLM，飞牛/TrueNAS无 | **私有化AI能力** - 零数据外泄 |
| 🔐 **AI以文搜图** | CLIP本地推理，自然语言搜索照片 | 飞牛/群晖仅人脸，TrueNAS无 | **超越人脸识别** - 自然语言精准匹配 |
| ☁️ **多云存储挂载** | 阿里/腾讯/AWS/GDrive/OneDrive 6+平台统一挂载 | 群晖有限，飞牛网盘，TrueNAS无 | **云本地化** - 覆盖最广 |

### ⭐ 领先功能

| 功能 | 说明 | 竞品对比 |
|------|------|----------|
| 📊 **Fusion Pool** | 智能热冷数据分层，SSD缓存+HDD容量 | TrueNAS无，群晖有Tiering |
| 🔥 **Hot Spare** | 热备盘自动故障切换，RAID自愈 | 飞牛fnOS无此功能 |
| 📈 **SSD三级预警** | 寿命预测+健康评分+预警通知 | 领先竞品方案 |
| 🛡️ **勒索防护** | WriteOnce + SMB行为监控 | TrueNAS规划中，竞品无 |
| 🚪 **内网穿透(免费)** | NAT穿透远程访问 | 竞品 Connect 需订阅 |

> 完整对比矩阵（DSM 7.4 / TrueNAS 26 / fnOS / QNAP h6.0）与差异化策略见 [docs/competitive-analysis.md](docs/competitive-analysis.md)。

---

## 项目结构

```
nas-os/
├── cmd/               # 可执行程序入口（nasd / nasctl）
├── internal/          # 内部模块
│   ├── application/   # 组合根：模块生命周期与依赖注入
│   ├── identity 核心域：storage / network / sharing / system / users …
│   ├── web/           # Web 服务（生产路由；禁 import lab）
│   ├── extensions/    # 7 个可选 HTTP Extension
│   └── lab/           # 实验温室（独立 go.mod，默认不编译）
├── pkg/               # 公共库
├── webui/             # 产品前端（web/ 为实验前端，非交付面）
├── configs/           # 配置文件
└── deploy/            # 部署脚本与样例
```

> 完整结构见 [docs/STRUCTURE.md](docs/STRUCTURE.md)。

## 🏗️ 架构（现行，v3.22.0）

### 应用组合与模块生命周期 (arch/application)
- Application 组合根: 统一构造依赖、启动入口和逆序关闭资源
- Typed Config: 配置文件、`NAS_OS_*` 环境覆盖和启动前校验
- Module 接口: Init/Start/Stop/Health/Dependencies 生命周期
- 拓扑排序: 检查缺失依赖与循环依赖，确定性启动和逆序停止
- 故障回滚: 模块启动失败时逆序停止已启动模块
- 分层路由: 公开、已认证和管理员路由由模块分别声明
- 健康状态: `/api/v1/system/modules` 提供核心模块状态
- **Core 仅五名**: identity / storage / network / sharing / system
- **Extension / Lab**: 目录与 catalog 标签必须一致；路径边界测试防止伪核心回流

### 设计原则
- 显式注入: 依赖由 `internal/application` 通过构造函数注入
- 单一所有者: 后台任务只能由一个 Module 或 Server 生命周期管理
- 构造无副作用: goroutine 在 Start 启动，在 Stop 关闭
- 渐进迁移: 保留兼容 API，通过 contract test 逐端点收口
- Container 不是 Service Locator: 新业务代码不得依赖字符串查找获取依赖
- 版本源: 根目录 `VERSION` 与 `internal/version` 同步；`GET /api/v1/system/info` 返回真实版本

详细说明见 [架构文档](docs/ARCHITECTURE.md)。

---

## 📊 项目资源统计

| 指标 | 实测（2026-09-06，死代码清理后） |
|------|------|
| Go 源码总行数 | **~1,493,000 行** |
| Go 源文件 | **~2,437 个** |
| 测试文件 | **~927 个** |
| `internal/` 顶层目录 | **~100**（治理收敛后） |
| Lab 包（`internal/lab/*`） | **~626** |
| Extension 包（`internal/extensions/*`） | **7** |
| go.mod 依赖（require，含 indirect） | **~158** |

> 📋 详细统计报告：[docs/resource-stats.md](docs/resource-stats.md)

---

## 版本状态

完整变更见 [CHANGELOG.md](CHANGELOG.md)。

### 当前状态 (2026-09-05) - v3.24.6 Stable ✅

**8/8 里程碑全部完成**——存储 / 共享 / 权限 / 监控 / 容器 / 虚拟机等能力均已交付；能力启用口径见上方「默认交付面」。

### 版本路线图
| 版本 | 类型 | 发布日期 | 核心功能 | 状态 |
|------|------|----------|----------|------|
| **v3.24.6** | **Stable** | **2026-09-05** | **删除非 lab 死代码 21.4 万行 + swagger 577 端点重生成 + benchmark 超时修复** | ✅ **已发布** |
| v3.24.5 | Stable | 2026-08-04 | 版本对齐 + 集成测试 Lab 路径 + 默认面诚实 + 伪核心再降 Lab | 未发 Release（维护性提交） |
| v3.24.1 | **Stable** | **2026-07-16** | **WebUI 门控、强制改密、api/middleware 清除** | ✅ **已发布** |
| v3.24.0 | **Stable** | **2026-07-16** | **optional 默认关、去 /volumes、删根 api、Core 真健康** | ✅ **已发布** |
| v3.23.1 | **Stable** | **2026-07-16** | **传递 Lab 切断、Extension 真加载、诚实日志、deps 治理** | ✅ **已发布** |
| v3.23.0 | Stable | 2026-07-16 | P0–P3：Lab 默认剥离、Extension 按需、Core 健康、治理锁 | ✅ 已发布 |
| v3.22.0 | Stable | 2026-07-16 | 163 伪核心降入 Lab；catalog 路径对齐；Core 仍为 5 | ✅ 已发布 |
| v3.21.0 | Stable | 2026-07-16 | 195+ 伪核心降入 Lab（AI/smart/backup/security/media…） | ✅ 已发布 |
| v3.20.0 | Stable | 2026-07-16 | 版本 bump 与文档同步 | ✅ 已发布 |
| v3.19.0 | Stable | 2026-07-16 | 架构收敛波次 | ✅ 已发布 |

<details>
<summary>更早版本路线（v3.11.0 → v1.x）</summary>

| v3.11.0 | Stable | 2026-07-04 | 智能文件夹、成本/容量风险摘要、发布安全护栏 | ✅ 已发布 |
| v2.490.61 | Alpha | 2026-03-10 | 项目骨架、btrfs 基础 | ✅ 发布 |
| v2.490.61 | Alpha | 2026-03-10 | 文件共享、配置持久化 | ✅ 发布 |
| v2.490.61 | Stable | 2026-03-11 | 生产就绪版本 | ✅ 已发布 |
| v2.490.61 | Stable | 2026-03-12 | 功能大更新 (10 个新模块) | ✅ 已发布 |
| v2.490.61 | Stable | 2026-03-12 | 安全加固与性能优化 | ✅ 已发布 |
| v2.490.61 | Stable | 2026-03-12 | 容器管理和 VM 功能 | ✅ 已发布 |
| v1.4.x | Stable | 2026-03-12 | RBAC 权限系统 + WebUI | ✅ 已发布 |
| v1.5.x | Stable | 2026-03-13 | 监控告警系统 + WebUI | ✅ 已发布 |
| v2.490.61 | Stable | 2026-03-13 | 性能优化 + CI/CD 完善 | ✅ 已发布 |
| v2.490.61 | Stable | 2026-03-13 | 配额/回收站/WebDAV/复制/AI | ✅ 已发布 |
| v2.490.61 | Stable | 2026-03-20 | 版本控制/云同步/去重 | ✅ 已发布 |
| v2.490.61 | Stable | 2026-04-01 | 存储复制/回收站增强 | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-21** | **iSCSI/快照策略/仪表板增强/性能监控** | ✅ **已发布** |
| **v2.490.61** | **Stable** | **2026-03-28** | **存储分层/FTP-SFTP/压缩存储/文件标签** | ✅ **已发布** |
| **v2.490.61** | **Stable** | **2026-03-14** | **集成测试完善/文档更新** | ✅ **已发布** |
| **v2.490.61** | **Stable** | **2026-03-14** | **快照复制/高可用/备份恢复集成测试** | ✅ **已发布** |
| **v2.490.61** | **Stable** | **2026-03-14** | **安全增强/性能优化/集成测试完善** | ✅ **已发布** |
| **v2.490.61** | **Stable** | **2026-03-14** | **Bug 修复/测试完善/安全审计** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-14** | **gin 依赖修复/CI/CD 修复** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-14** | **CI/CD 修复/代码质量改进** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-14** | **SMB/NFS 修复/Go 1.25 升级/安全审计** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-14** | **SQLite 驱动替换/测试覆盖率提升/WebUI 完善** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **预测分析/i18n 国际化/API 文档系统** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-14** | **文档完善/版本更新** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **功能完善/代码质量** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **数据竞争修复/并发安全** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-14** | **代码清理/CI优化/文档完善** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **网络诊断/Docker 增强/自动化完善** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **媒体服务/配额自动扩展/监控增强** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **文档完善/API 文档覆盖** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **API 文档完善/国际化更新** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **请求日志/Excel导出/开发环境增强** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **i18n框架/API中间件/成本分析** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **国际化补全/CI优化/文档更新** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **文档同步/版本号更新** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **项目治理/文档体系完善** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **安全审计/并发修复/CI优化** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **测试修复/CI优化/Swagger文档** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **测试修复/文档完善** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-15** | **文档体系完善/用户指南优化/API文档补充** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-20** | **代码质量提升/Lint修复/安全加固/自动化协同** | ✅ 已发布 |
| **v2.490.61** | **Stable** | **2026-03-21** | **依赖更新/安全增强/文档同步** | ✅ **已发布** |
| **v2.490.61** | **Stable** | **2026-03-24** | **网盘挂载/AI脱敏/智能分层** | ✅ **已发布** |
| **v2.490.61** | **Stable** | **2026-03-21** | **版本迭代/自动化协同维护** | ✅ **已发布** |

</details>

<details>
<summary>v2.490.61 历史更新汇总（合并自 15 个重复章节）</summary>

| ☁️ 网盘挂载 | 支持阿里云 OSS、腾讯云 COS、AWS S3、Google Drive、OneDrive 等多云存储挂载为本地目录，透明读写 |
| 🔐 AI 脱敏服务 | 智能 PII 识别与脱敏（邮箱/手机/身份证/信用卡/IP），保护隐私数据安全 |
| 🤖 多 AI 提供商 | 支持 OpenAI、Google、Azure、百度、本地 LLM 多种 AI 服务接入 |
| 🗂️ 智能存储分层 | 热/温/冷数据自动分层，SSD 缓存加速，云存储归档 |

| 🔧 Lint 修复 | 修复 50+ golangci-lint revive 错误（命名规范、注释规范） |
| 🛡️ 安全加固 | 修复整数溢出漏洞 (G115)，文件权限修复 (0644→0600) |
| 🏗️ 代码重构 | 解决类型命名 stuttering 问题 (SnapshotExecutor→Executor 等) |
| 📊 自动化协同 | 兵/刑/礼/工/吏/户自动化开发流程 |
| 📚 文档同步 | 版本号一致性维护，CHANGELOG 规范化 |

| 🔄 依赖更新 | 安全依赖更新，输入验证增强 |
| 📚 文档同步 | 版本号一致性维护 |

| 📚 文档体系完善 | README 版本同步、用户指南索引优化 |
| 📖 API 文档补充 | 新增存储 API、用户 API 文档 |
| 📝 用户指南优化 | 添加版本号、优化文档结构 |

| 🧪 测试修复 | 告警模块测试用例优化 |
| 📚 文档完善 | 用户快速入门、API示例、FAQ更新 |

| 📚 Swagger API 文档 | 完整 OpenAPI/Swagger 文档生成，覆盖所有主要模块 |
| 🧪 测试修复 | 并发测试、存储成本测试、容量规划测试完善 |
| 🚀 CI/CD 优化 | Node.js 24 支持、缓存策略优化、构建并行化 |

| 🛡️ 安全审计系统 | 9 项安全检查，完整测试覆盖 |
| 🔧 并发安全修复 | WebSocket、Response、LDAP 等模块修复 |
| 🚀 CI/CD 优化 | 超时配置、测试并行化、健康检查修复 |
| 📊 配额管理优化 | 成本计算验证、资源效率分析 |

| 📚 项目治理完善 | 版本号统一、CHANGELOG 规范化 |
| 📖 文档体系完善 | 文档结构优化、发布说明完善 |

| 📚 文档同步 | 所有文档版本号同步至 v2.490.61 |
| 📖 README 更新 | 更新下载链接、Docker 镜像版本 |
| 📝 docs 更新 | 更新文档中心索引和英文文档 |

| 🌐 国际化补全 | 日韩文翻译补全，四种语言键数一致 (286个) |
| 📚 文档更新 | CHANGELOG 添加 v2.490.61，README 版本号同步 |
| 🔧 CI/CD 优化 | 工作流优化，安全扫描增强 |
| 🧪 测试改进 | 测试用例修复，覆盖率保持稳定 |

| 🌐 i18n 国际化框架 | 完整翻译系统，支持中/英/日/韩四种语言 |
| 🔌 API 中间件系统 | 统一错误处理、响应时间记录、WebSocket 增强 |
| 💰 成本分析报告 | 存储成本分析、资源计费统计、趋势预测 |
| 📊 监控配置增强 | Prometheus 集成优化、告警规则完善 |

| 📝 请求日志中间件 | 完整请求日志记录、请求ID追踪、结构化输出 |
| 📊 Excel 报告导出 | 完整Excel导出器、样式设置、多工作表支持 |
| 🔧 开发环境增强 | Air热重载、Docker Compose开发环境 |
| 📖 文档完善 | API快速入门指南、发布流程文档 |

| 📊 稳定性提升 | 核心模块测试覆盖率提升 |
| 📚 文档完善 | API 文档 Swagger 注释完善 |
| ⚡ 性能优化 | 缓存和并发性能优化 |
| 🔒 安全增强 | 权限检查和安全审计 |

| 📚 文档完善 | 快速开始指南、用户文档更新 |
| 📡 API 文档覆盖 | 完善所有 API 模块的 Swagger 注释 |
| 📖 文档索引优化 | 更新文档中心索引，按角色导航 |

| 🎬 媒体服务 | HLS/DASH 流媒体、字幕处理、视频转码、缩略图生成 |
| 📈 配额自动扩展 | 自动扩展配额策略、审批流程、回滚支持 |
| 📊 监控增强 | 健康评分系统、指标收集器、报告集成 |

**v2.490.61 新增功能**

| 功能 | 说明 |
|------|------|
| 🔍 网络诊断 | Ping/Traceroute/DNS 查询/端口扫描/Whois 查询 |
| 🐳 Docker 增强 | 容器批量操作、镜像管理、网络配置、卷管理 |
| ⚙️ 自动化完善 | 工作流执行优化、Action 解析增强、错误处理改进 |

**v2.490.61 新增功能**

| 功能 | 说明 |
|------|------|
| 🗂️ 存储分层 | 热/冷数据自动分层，SSD 缓存层加速，云存储归档 |
| 📡 FTP 服务器 | 被动/主动模式，匿名登录，带宽限制，虚拟目录 |
| 🔐 SFTP 服务器 | SSH 密钥认证，用户权限隔离，chroot 限制 |
| 🗜️ 压缩存储 | 文件级/块级压缩，透明压缩，节省空间 |
| 🏷️ 文件标签 | 标签分类，颜色图标，批量操作，标签云 |

**v2.490.61 新增功能**

| 功能 | 说明 |
|------|------|
| 🎯 iSCSI 目标 | iSCSI Target 服务，支持 LUN 管理和 CHAP 认证 |
| 📸 快照策略 | 自动化快照调度，支持多种保留策略 |
| 🖥️ 仪表板增强 | 全新 WebUI 仪表板，可自定义小部件布局 |
| 📊 性能监控增强 | 性能基线学习、异常检测、优化建议 |

**v2.490.61 新增功能**

| 功能 | 说明 |
|------|------|
| 📜 文件版本控制 | 自动保存历史版本，支持版本恢复和对比 |
| ☁️ 云同步增强 | 支持阿里云 OSS、腾讯云 COS、AWS S3、Google Drive、OneDrive、Backblaze B2 |
| 🔄 双向同步 | 本地↔云端实时/定时同步，冲突自动解决 |
| 🗜️ 数据去重 | 文件级/块级去重，节省存储空间 |
| 📊 去重报告 | 详细的空间节省统计和可视化 |
| 🌐 多云存储 | 统一管理多个云存储提供商 |

**v2.490.61 新增功能**

| 功能 | 说明 |
|------|------|
| 📊 存储配额 | 用户/组/目录三级配额控制 |
| 🗑️ 回收站 | 安全删除，支持恢复 |
| 📁 WebDAV | 完整 WebDAV 协议支持 |
| 🔄 存储复制 | 跨节点数据同步 |
| 🤖 AI 分类 | 照片/文件智能分类 |
| ⚡ 性能优化 | LRU 缓存/连接池/工作池 |
| 📈 报告系统 | 定时生成存储/使用报告 |

</details>

---

## 历史版本亮点

逐版本完整变更见 [CHANGELOG.md](CHANGELOG.md)；以下存档保留各版本亮点速览与模块治理历史（其中 v3.12.0 未进 CHANGELOG，此处为唯一记录）。

<details>
<summary>版本亮点速览（v3.24.0 → v3.1.0）</summary>

### 🚀 v3.24.0 默认表面收敛（破坏性） ✅

| 项 | 说明 |
|----|------|
| **packages 默认关** | 默认不构造 Docker/VM/Photos/AI 等非 Core 产品管理器（`packages.recommended_system=false`） |
| **单一存储契约** | 仅 `/api/v1/storage/*`；移除 legacy `/api/v1/volumes` |
| **删除根 api/ 源码** | 非 nasd 入口，避免误用 |
| **主 UI** | `webui/`（`/webui` + 核心页面）；`web/src` 为实验 |
| **主部署** | 根 `docker-compose.yml` + `Dockerfile` |
| **Core 健康** | storage/users/smb/nfs/network 真实 `Health()` |
| **安全默认** | 监听 `127.0.0.1`；生产强制 `NAS_CSRF_KEY`；bootstrap admin `MustChangePassword` |
| **零引用削减** | 再降 ~120 顶层包入 Lab |

### 🚀 v3.23.0 运行时诚实与治理 (P0–P3) ✅

| 项 | 说明 |
|----|------|
| **Lab 默认剥离** | 生产 `web` 不再 import/构造 `internal/lab/*`；实验能力仅在 Lab 目录保留 |
| **Extension 按需加载** | `packages.enabled`；默认空=不加载；7 个 HTTP 扩展可显式启用（`modules.extensions` 弃用兼容） |
| **Core 健康探针** | `GET /api/v1/system/health` 聚合 Core 模块 `Health()`，失败返回 unhealthy |
| **治理测试** | 禁止 web→lab import；Core 仅五名；顶层 allowlist 冻结；未知 catalog→Lab |
| **入口诚实** | 根 `api/` 标明非 nasd 入口；主 UI=`webui/`；主部署见 docker-compose |

### 🚀 v3.22.0 架构收敛（Core / Extension / Lab） ✅

生产进程生命周期主图仍只注册五个 Core 模块；本轮继续把 **163** 个零生产引用的伪核心包迁入 `internal/lab/`，并修正 catalog 与磁盘路径不一致的 Lab 标签。

| 层级 | 规则 | 现状 |
|------|------|------|
| **Core** | 仅 `identity` / `storage` / `network` / `sharing` / `system` | 生命周期主图，不可扩张 |
| **Extension** | 可选产品能力，不得伪装成 Core | 7 包位于 `internal/extensions/`（如 activeprotect、voicehub） |
| **Lab** | 实验/重复/零生产引用实现 | ~467 包位于 `internal/lab/`（含 media、filemanager、selfheal、ztna 等） |

> 历史版本亮点见下方时间线；**实验性能力以 Lab 路径为准**，不再视为顶层生产核心。完整变更见 [CHANGELOG](CHANGELOG.md)。

### 🚀 v3.17.0 数据分层与恢复置信 ✅

本轮对标 TrueNAS 26 OpenZFS 2.4 dataset tiering、Synology TCO Calculator、TrueNAS FEC 网络纠错、TrueNAS Clean Restore Confidence 勒索恢复、GDPR/PIPL/HIPAA 多标准合规审计和 Apple Memories/Synology Photos 故事生成，新增 6 个模块覆盖数据集冷热分层、5年TCO可视化、AI照片故事、FEC网络纠错、合规工作流编排和恢复置信度评估。

| 亮点 | 用户收益 | 状态 |
|------|----------|------|
| 💾 **ZFS 数据集分层调度** | 冷热数据自动分层、预测式分层、闪存容量预警、归档层启用 | ✅ v3.17.0 新增 |
| 💰 **TCO 可视化仪表板** | 5年总拥有成本分析、电力/云/人工成本拆解、竞品对比 | ✅ v3.17.0 新增 |
| 📸 **AI 照片故事生成** | 7种故事主题（旅行/家庭/季节/冒险/回溯/精选/日常）、自动聚类 | ✅ v3.17.0 新增 |
| 🔧 **FEC 网络纠错配置** | RS/Hamming/BCH/LDPC 编码推荐、存储接口保护、长距离纠错 | ✅ v3.17.0 新增 |
| 📋 **合规审计工作流** | GDPR/PIPL/HIPAA/SOC2/ISO27001/PCI-DSS 6阶段审计流程编排 | ✅ v3.17.0 新增 |
| 🛡️ **勒索恢复置信度** | 恢复演练追踪、RTO/RPO达标、不可变/异地检查、Clean Restore评分 | ✅ v3.17.0 新增 |


### 🚀 v3.16.0 智能调度与数据主权 ✅

本轮对标 Synology SSD Cache Advisor/Power Schedule/CMS、TrueNAS L2ARC/Cluster/Cloud Sync Cost 和飞牛影视墙，新增 6 个模块覆盖 SSD 缓存调度、云成本审计、海报刮削、电源管理、集群编排和数据主权合规。

| 亮点 | 用户收益 | 状态 |
|------|----------|------|
| 💾 **SSD 缓存智能调度** | 分析缓存命中率、磨损、温度和读写放大，给出扩容、预热和 NVMe 升级建议 | ✅ v3.16.0 新增 |
| ☁️ **云存储成本审计** | 检测预算超支、休眠账户、高出口流量和缺失分层策略，建议 R2 迁移节省出口费 | ✅ v3.16.0 新增 |
| 🎬 **多媒体海报刮削** | 批量检测缺失海报、文件名解析失败、字幕缺失，建议自动刮削和缓存清理 | ✅ v3.16.0 新增 |
| ⚡ **电源管理调度** | 空闲切换、待机调度、磁盘转速策略、夜间计划和太阳能对齐，降低功耗 | ✅ v3.16.0 新增 |
| 🔗 **多 NAS 集群编排** | HA 启用建议、故障转移验证、脑裂检测、复制延迟监控和集群健康评分 | ✅ v3.16.0 新增 |
| 🌐 **数据主权审计** | PII 加密检测、跨境复制审查、访问日志和保留策略缺失审计，覆盖 GDPR/PIPL/HIPAA | ✅ v3.16.0 新增 |


### 🚀 v3.14.0 备份健康顾问 ✅

本轮参考群晖 Active Backup 的集中备份与自助恢复、TrueNAS 的快照/校验/不可变保护，以及飞牛家庭 NAS 的低门槛灾备体验，新增备份健康顾问：把终端保护、备份新鲜度、恢复演练和灾备准备转化为可执行建议。

| 亮点 | 用户收益 | 状态 |
|------|----------|------|
| 🛡️ **终端保护率** | 自动识别未纳入备份的电脑/移动设备，提示下发代理、套用模板和展示未保护清单 | ✅ v3.14.0 新增 |
| 🔁 **备份失败修复** | 对过期备份和失败任务给出重试、错误定位、容量/凭据/任务锁检查建议 | ✅ v3.14.0 新增 |
| 🔒 **快照与不可变恢复点** | 关键共享缺少快照或出现勒索告警时，提示只读快照、时间线恢复和不可变保留 | ✅ v3.14.0 新增 |
| 🧪 **恢复演练与灾备介质** | 最近 30 天无演练、缺少异地副本或恢复介质时，提示自动试恢复和 RPO/RTO 预估 | ✅ v3.14.0 新增 |


### 🚀 v3.13.0 WebShare 搜索顾问 ✅

本轮继续对标 TrueNAS 26 WebShare/TrueSearch、群晖 DSM 的移动端分享体验和飞牛家庭媒体易用性，新增 WebShare 搜索顾问：把文件规模、索引覆盖、外链安全和快照保护转化为可执行建议。

| 亮点 | 用户收益 | 状态 |
|------|----------|------|
| 🌐 **WebShare 启用建议** | 文件库存在但未开放浏览器访问时，自动提示开启上传、下载、筛选和可撤销外链 | ✅ v3.13.0 新增 |
| 🔎 **搜索索引覆盖率** | 计算本地索引覆盖率，提示 SSD 索引缓存、文档内容索引和加密数据集说明 | ✅ v3.13.0 新增 |
| 🔐 **外链安全加固** | 对外部分享建议 passkey/一次性访问码、过期时间、下载次数和审计记录 | ✅ v3.13.0 新增 |
| 📸 **分享前快照保护** | 共享协作场景自动提示只读快照与时间线入口，降低误删覆盖风险 | ✅ v3.13.0 新增 |


### 🚀 v3.12.0 文件洞察与媒体整理建议 ✅

本轮参考 DSM 7.4 的本地 AI/语义搜索、飞牛的相册与影视易用性、TrueNAS 26 的 WebShare/TrueSearch 与数据治理体验，新增文件洞察引擎：把智能文件夹结果转换成可执行的清理、相册索引和媒体库整理建议。

| 亮点 | 用户收益 | 状态 |
|------|----------|------|
| 🧠 **文件洞察 Advisor** | 自动识别大文件、照片库和视频库，给出清理、归档、索引、刮削与转码建议 | ✅ v3.12.0 新增 |
| 🖼️ **照片整理触发器** | 当照片积累到阈值时提示 EXIF 时间线、人脸聚合和本地以文搜图索引 | ✅ v3.12.0 新增 |
| 🎬 **媒体库体验建议** | 视频库达到规模后提示海报墙、字幕匹配和跨端续播准备 | ✅ v3.12.0 新增 |
| 🧹 **容量治理动作化** | 大文件占用转化为 warning/info 级别建议，便于前端和通知系统直接展示 | ✅ v3.12.0 新增 |

### 🚀 v3.11.0 智能文件夹与治理状态同步 ✅

v3.11.0 把 v3.10.0 的“下一步推荐”继续落到文件整理、容量治理和安全发布上：用户能更快找到该整理的文件、看懂容量风险，也能获得版本一致、可审计的发布包。

| 亮点 | 用户收益 | 状态 |
|------|----------|------|
| 🗂️ **智能文件夹** | 内置 recent、large-files、photos、videos、documents 等规则化视图，按类型、扩展名、大小和最近修改快速整理文件 | ✅ v3.11.0 新增 |
| 📊 **成本/容量风险摘要** | 汇总低利用、高水位、过载资源和浪费估算，把容量风险转化为清理、分层或扩容建议 | ✅ v3.11.0 增强 |
| 🔒 **发布安全护栏** | CI/Docker 发布中的 Trivy Action 固定到明确版本，并补充 workflow 安全基线测试，降低供应链漂移风险 | ✅ v3.11.0 增强 |
| 🧾 **版本一致性** | 命令行、API 构建信息、README、CHANGELOG、资源统计和竞品分析统一到 v3.11.0，减少用户升级时的版本判断成本 | ✅ v3.11.0 同步 |

### 🚀 v3.10.0 用户体验顾问与下一步推荐 ✅

v3.10.0 把飞牛的家庭影音易用性、群晖的套件化引导、TrueNAS 的数据保护思路融合成一个“下一步推荐”体验：系统不只展示状态，还会根据本地使用信号告诉用户接下来最值得做什么。

| 亮点 | 用户收益 | 状态 |
|------|----------|------|
| 🧭 **体验顾问引擎** | 聚合照片、媒体、备份、远程访问、应用和存储信号，自动生成可执行建议，降低 NAS 配置门槛 | ✅ v3.10.0 新增 |
| 🖼️ **AI 相册整理建议** | 大型照片库会提示人物聚类、动态照片解析、重复照片清理，让家庭相册更接近开箱即用 | ✅ v3.10.0 新增 |
| 🎬 **影视库体验建议** | 针对影片数量、字幕与转码场景，推荐海报墙刮削、字幕匹配、跨端播放进度同步等优化 | ✅ v3.10.0 新增 |
| 🔐 **备份与快照建议** | 根据备份规模、存储异常和快照状态，提示恢复校验、不可变快照、scrub 与生命周期分层 | ✅ v3.10.0 新增 |
| 🌐 **远程访问健康建议** | 多设备访问或异常连接时，建议 NAT 穿透、DDNS、证书续期与公网入口收敛，兼顾易用与安全 | ✅ v3.10.0 新增 |
| 📦 **应用与存储下一步** | 低活跃应用提示清理或替代方案，容量增长时提示分层、归档和扩容规划 | ✅ v3.10.0 新增 |

### 🚀 v3.9.0 安全与运维体验增强 ✅

| 模块 | 说明 | 状态 |
|------|------|------|
| 📋 重启原因运维闭环 | Log Center 增强重启原因处理、历史记录与 handler 测试，覆盖用户触发、异常、计划维护等复盘场景 | ✅ v3.9.0 增强 |
| 🌐 WebShare 交互增强 | 优化文件分享前端逻辑、分享状态展示与用户操作一致性 | ✅ v3.9.0 增强 |
| 👥 登录与用户管理体验 | 调整登录页和用户管理页交互细节，改善权限管理与日常运维体验 | ✅ v3.9.0 增强 |
| 📚 竞品分析与资源统计 | 新增 2026-06-29 竞品分析，刷新 competitive-analysis 与 resource-stats 文档 | ✅ v3.9.0 更新 |
| 🌐 网络 FEC 管理 | 25G/100G 链路 FEC 模式推荐、配置意图与审计记录，对标 TrueNAS 26 高速网络可用性 | ✅ v3.8.0 新增 |
| 🔎 本地 AI 语义搜索治理 | local-only 查询、脱敏返回、请求人/用途审计，对标 DSM 7.4 私有 AI 与 TrueSearch 体验 | ✅ v3.8.0 新增 |
| 🐧 LXC 迁移计划 | 冷/热/在线迁移步骤生成，含预检、快照、同步、回滚，对标 TrueNAS 26 LXC 运维 | ✅ v3.8.0 新增 |
| 🧊 不可变快照策略 | WORM/合规快照/审计链组合，覆盖 QNAP h6.0 immutable snapshots 类场景 | ✅ 已支持 |
| 🗂️ 存储效率与分层 | 压缩/去重/生命周期/冷热数据策略，回应 DSM 存储效率与 QNAP FileTiers 方向 | ✅ 已支持 |
| 🔑 现代身份安全 | RBAC/MFA/设备信任/审计，预留 passkeys 等无密码登录演进路线 | ✅ 已支持 |
| 🎬 媒体与远程访问 | 智能海报墙、P2P 远程访问、WebShare 与多端体验，对标 fnOS 家庭场景 | ✅ 已支持 |

### 🚀 v3.7.0 存储与应用生态增强 ✅

| 模块 | 说明 | 状态 |
|------|------|------|
| 💾 RAIDZ vdev 扩展 | 扩展前健康检查、Dry Run、进度跟踪、取消/失败状态管理 | ✅ v3.7.0 新增 |
| 📈 存储 ROI 分析 | 采购成本、容量利用率、寿命追踪、TCO/ROI 评分与优化建议 | ✅ v3.7.0 新增 |
| 🎬 智能海报墙 | 媒体刮削、布局展示、播放进度同步、观影清单与推荐 | ✅ v3.7.0 新增 |
| ⭐ 应用中心评价 | 评分评论、开发者回复、举报审核、统计聚合 | ✅ v3.7.0 新增 |
| 🔐 NFS Kerberos 审计 | 认证事件、加密类型、风险告警与合规报告 | ✅ v3.7.0 新增 |
| 🧭 生命周期/容量/安全顾问 | 应用 Dry Run、预算容量规划、容器运维洞察、RAIDZ 预飞计划、安全评分 | ✅ v3.7.0 增强 |

### 🚀 v3.1.0 架构重构稳定化 ✅

| 模块 | 说明 | 状态 |
|------|------|------|
| 💾 块级备份增强 | 增强引擎、REST API、增量/去重/恢复测试 | ✅ v3.1.0 稳定化 |
| 📊 性能监控增强 | 指标采集、阈值告警、历史查询、HTTP API | ✅ v3.1.0 稳定化 |
| 🛡️ ML 勒索检测 | 熵分析、写频异常、批量扩展名检测、响应 API | ✅ v3.1.0 稳定化 |
| 📸 快照管理增强 | 保留策略、团队快照、ZFS 快照封装、扩展测试 | ✅ v3.1.0 稳定化 |
| 🖥️ 系统监控增强 | CPU/内存/磁盘/网络指标、告警、API 测试 | ✅ v3.1.0 稳定化 |
| 🔐 安全与协作模块 | Secure Boot、SMB Guard、WORM 合规、Team File、User API Key | ✅ v3.1.0 稳定化 |
| 🌐 WebShare Pro | 协作、分享链接、WebRTC 分享命名冲突清理 | ✅ v3.1.0 稳定化 |

</details>

<details>
<summary>模块治理与历史整合</summary>

## 🔄 模块治理与历史整合

### 当前分层（v3.18+ → v3.22.0）

| 层级 | 路径 | 说明 |
|------|------|------|
| Core | 生命周期注册名 | `identity` / `storage` / `network` / `sharing` / `system` |
| Extension | `internal/extensions/<name>` | 可选产品能力（activeprotect、agentworkflow、aiguardrails、compliancescan、deployorch、netdiag、voicehub） |
| Lab | `internal/lab/<name>` | 实验、重复与零生产引用实现（不得再当作顶层 Core） |

### 历史合并域（v3.0.0，路径以现状为准）

早期曾合并重复实现到领域包；**其中多数实验/辅件现已降入 Lab**：

| 功能域 | 当前路径 | 说明 |
|--------|----------|------|
| 勒索防护 | `internal/ransomware`（及相关 lab 实验） | 行为检测/蜜罐等能力 |
| 合规审计 | `internal/lab/compliance` | CIS/STIG/GDPR 等实验与报告能力 |
| 存储分层 | `internal/tiering` | 生产侧分层；lab 中另有增强实现 |
| AI 相册 | `internal/lab/photoai` | 人脸/以文搜图等实验能力 |
| 成本分析 | `internal/lab/costanalyzer` | TCO/成本实验实现 |
| 磁盘健康 | `internal/lab/diskhealth` | AI 故障预测实验实现 |
| AI 控制台 | `internal/lab/aiconsole` | 本地 LLM 管理实验 |
| AI Agent | `internal/lab/aiagentorch` | 多 Agent 编排实验 |

</details>

<details>
<summary>历史版本亮点存档（v2.800 / v2.900，无对应 git tag，能力多在 Lab）</summary>

## 🚀 v2.800 存储引擎升级

### ZFS完整管理器
- 池管理: 创建/销毁/导入/导出/扫描
- 数据集管理: 创建/删除/挂载/卸载/属性设置
- ZVOL管理: 创建/调整大小/销毁
- 快照管理: 创建/删除/回滚
- 块级备份: Send/Receive/远程复制

### NVMe-oF生产就绪
- 多路径管理: RDMA/TCP双协议
- 故障切换: 自动故障检测+切换
- 健康监控: 实时路径状态监控
- IO统计: 延迟/带宽/IOPS

### Btrfs存储引擎
- 池管理: 创建/挂载/卸载
- 子卷管理: 创建/删除/列表
- 快照: 创建/发送/接收
- 在线RAID转换: RAID1/5/6/10
- 设备管理: 热添加/移除
- 碎片整理: 压缩模式

### 块级增量备份
- 全量备份: dd/zstd/lz4压缩
- 增量备份: rsync差异同步
- ZFS备份: 原生send/receive
- 备份验证: SHA256校验
- 自动清理: 保留策略配置

## 🤖 v2.900 AI平台化

### 统一AI推理平台 (aiplatform)
- 多Provider注册: 支持Ollama/LocalAI/OpenAI等多后端
- 模型注册表: 统一管理LLM/Embedding/Vision模型
- 负载均衡: Round-Robin策略分配请求
- 响应缓存: 5分钟TTL减少重复调用
- 流式输出: SSE实时返回结果

### RAG知识库 (ragserver)
- 向量存储: 内存向量库+余弦相似度检索
- 文档分块: 可配置chunk_size和overlap
- 语义检索: TopK+阈值过滤
- 集合管理: 创建/查询/列表

### MCP协议增强 (mcpserver)
- 工具注册: 标准MCP工具定义
- 资源暴露: URI-based资源访问
- 会话管理: 多会话支持

</details>

---

## 获取帮助

- 📖 **完整文档**: [docs/](docs/) 目录
- 🐛 **报告问题**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- 💬 **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)
- 📦 **Docker 镜像**: [GHCR](https://github.com/crazyqin/nas-os/pkgs/container/nas-os)

## License

MIT
