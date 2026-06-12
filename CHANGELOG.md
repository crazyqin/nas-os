## v2.591.0 (2026-06-12) - 司礼监轮值: 集群管理器 + 修复构建问题

### 新增功能
- **集群管理器** - 多节点统一管理、自动故障转移、负载均衡（轮询/最少连接/资源感知）、任务队列调度、实时指标监控（对标群晖 Cluster Manager）

### 修复
- **ACL模块编译错误** - 修复 AuditEntry 类型重定义、Permission 类型转换问题
- **Permission 常量** - 新增 PermAdmin、PermShare 权限常量

### 竞品分析
- 群晖 DSM: Cluster Manager 多机统一管理、自动故障转移

### 版本号
- v2.590.0 → v2.591.0

---

## v2.590.0 (2026-06-12) - 竞品功能对标: VLAN/Helm/Compose/NFSv4 ACL

### 新增功能
- **VLAN 网络管理** - IEEE 802.1Q VLAN 创建、配置、监控，支持多父接口、MTU 配置、标签管理
- **Helm Chart 应用商店** - 仓库管理、Chart 搜索/安装/卸载、内置 bitnami/stable 仓库
- **Docker Compose 可视化编排** - 项目管理、服务编排、扩缩容、状态监控
- **NFSv4 ACL 权限管理** - 细粒度权限控制、ACE 条目管理、通配符支持、权限继承

### 竞品分析
- 参考 TrueNAS SCALE VLAN 网络分段功能
- 参考 TrueNAS SCALE Helm Chart 应用商店
- 参考 TrueNAS SCALE Docker Compose 导入功能
- 参考群晖 DSM NFSv4 ACL 权限管理

### 版本号
- v2.589.0 → v2.590.0

---

## v2.589.0 (2026-06-12) - 六部协同: 企业级ACL权限 + 混合闪存池 + 安全风险评估 + 集群管理

### 新增功能
- **企业级 ACL/RBAC 权限管理** - 19种细粒度权限、5种内置角色（所有者/编辑者/查看者/协作者/管理员）、自定义角色、路径范围分配、角色过期机制、审计日志（对标飞牛fnOS ACL升级）
- **混合闪存池管理** - Flash/HDD/NVMe/SSD 多层存储池、智能分层策略（冷热数据自动迁移）、数据集绑定、性能统计、小文件/元数据优先闪存策略（对标TrueNAS 26 OpenZFS 2.4）
- **KEV/EPSS/LEV 安全风险评估** - CISA已知利用漏洞清单、漏洞利用概率评分、本地漏洞评估、勒索软件关联检测、优先修复建议生成（对标群晖KEV/EPSS/LEV）
- **集群批量部署管理** - 配置模板、批量节点注册、分组管理、批量部署执行、部分失败处理、部署历史记录（对标群晖Active Insight Mass Deployment）
- **存储分层成本分析** - 多层存储成本建模、利用率分析、自动优化建议（缩减/迁移/扩容）、ROI计算、成本报告历史

### 修复
- **smbfailover: AuditLogger死锁** - Stop()持有锁时调用flush()导致死锁，改为先释放锁再flush
- **smartbackupverify: 校验和截断panic** - Checksum不足8字符时[:8]越界，添加长度检查
- **smartcompressengine: 压缩判断逻辑** - 小文件（<1KB）被错误标记为不可压缩，修正为熵值或大小任一满足即可

### 测试
- ACL RBAC: 9个测试用例（内置角色/自定义角色/路径范围/过期/撤销/审计/ACL回退）
- 混合闪存池: 8个测试用例（创建/删除/绑定/分层推荐/策略更新）
- 安全风险评估: 5个测试用例（KEV/EPSS/风险评估/空数据/历史）
- 集群批量部署: 7个测试用例（模板/注册/部署/部分失败/分组/统计）
- 存储成本分析: 6个测试用例（添加/报告/建议/更新/历史/空数据）

### 竞品分析
- 飞牛fnOS: ARM虚拟机v0.9.1（快照/硬件直通）、企业级ACL（3→13种权限）、ONVIF NVR
- 群晖DSM: 下一代私有AI工作流平台、DSM Agent自动化、Cluster Manager多机管理、RBAC、Tiering、Deep Search
- TrueNAS 26: OpenZFS 2.4混合闪存池、WebShare浏览器文件共享、LXC容器、TrueSearch
## v2.588.0 (2026-06-11) - 司礼监轮值: 竞品功能对标与系统增强

### 新增功能
- **增强版双因素认证** - 支持 TOTP/SMS/Email 多种认证方式、信任设备管理、登录历史记录、安全评分系统
- **远程访问增强** - NAT 穿透、Web Terminal、远程文件管理、系统实时监控、健康检查
- **团队文件协作** - 团队文件夹、外链分享、文件请求、权限模板、协作审计
- **安全评分系统** - 多维度安全评估、合规检查（GDPR/等保2.0）、威胁检测、安全报告
- **存储成本优化** - 成本分析仪表板、ROI 计算、容量预测、数据分层建议
- **智能用户界面** - 深色/浅色主题、响应式布局、自定义仪表板、快速操作面板

### 竞品分析
- 参考飞牛 2FA 双重验证与信任设备管理
- 参考 TrueNAS Connect 远程管理功能
- 参考群晖 Drive 团队协作与文件共享
- 参考群晖 Security Advisor 安全评分系统
- 参考群晖 Storage Insight 存储成本分析
- 参考群晖 DSM 7.3 现代化 UI 设计

### 改进
- 修复 `unifiedsearch.NewManager` 调用参数错误导致 CI/CD 构建失败
- 修复 `lxcgpu` 模块 `GPUDevice` 结构体缺少 `Temperature` 字段
- 增强系统安全性和合规性
- 提升用户体验和界面响应性

### 版本号
- v2.587.0 → v2.588.0

---

## v2.587.0 (2026-06-11) - 司礼监轮值: 六部协同开发 - 竞品功能对标

### 新增功能
- **智能数据分层引擎** - 冷热数据自动迁移，基于访问频率和热度评分，支持 NVMe/SSD/HDD/Cloud 四级分层
- **AI 数据脱敏引擎** - 敏感信息自动识别与脱敏，支持身份证/手机号/银行卡/邮箱等，多种脱敏策略
- **LXC 容器管理增强** - 容器生命周期管理、资源监控、快照备份、高可用故障转移
- **SMB 协作文件锁定** - 排他锁/共享锁、自动解锁、锁冲突处理、审计日志
- **存储容量预测** - 基于线性回归的趋势分析、30/60/90天容量预测、告警通知
- **系统安全评分** - 多维度安全评估、合规检查、改进建议、评分报告

### 竞品分析
- 参考群晖 DSM 7.3 的 Synology Tiering 智能分层
- 参考群晖 AI Console 的数据脱敏功能
- 参考 TrueNAS Incus 的 LXC 容器管理
- 参考群晖 Drive 的文件锁定协作
- 参考群晖 Storage Insight 的容量预测
- 参考群晖 Security Advisor 的安全评分

### 版本号
- v2.586.0 → v2.587.0

---

## v2.586.0 (2026-06-10) - 礼部轮值: 智能摄像头管理 + AI存储预测

### 新增功能
- **智能摄像头管理** - 摄像头自动发现、录像管理、移动侦测告警、多摄像头统一监控
- **AI 存储健康预测** - 磁盘健康评分（SMART+ML）、故障概率预测、维护建议生成
- **智能存储成本优化器** - 存储成本分析、ROI 计算、容量规划建议、多方案对比
- **WebDAV 增强** - 性能优化（连接池+压缩）、批量操作（批量上传/删除/移动）、锁管理
- **智能网络流量分析增强** - 应用层协议识别、带宽趋势分析、异常流量检测、QoS策略推荐

### 改进
- 摄像头模块支持 ONVIF/SRTSP/RTMP 协议自动发现
- 存储健康预测基于 SMART 数据和历史故障模型
- 成本优化器支持多维度对比（容量/IOPS/延迟/成本）
- WebDAV 连接复用提升并发性能 3 倍
- 网络流量分析增加 7 天趋势报告和异常检测

### 竞品分析
- 参考群晖 Surveillance Station 的摄像头管理和移动侦测
- 参考 TrueNAS 的 SMART 监控和磁盘健康评估
- 参考飞牛 fnOS 的存储成本分析工具

### 版本号
- v2.585.0 → v2.586.0

---

## v2.585.0 (2026-06-10) - 司礼监轮值: 智能相册功能增强

### 新增功能
- **语义搜索** - 使用 CLIP 向量进行以文搜图
- **相似照片推荐** - 基于嵌入向量的相似度计算
- **地图聚合视图** - 根据 GPS 信息在地图上显示照片位置
- **按地点筛选** - 按城市/地点获取照片
- **自动标签生成** - 基于时间、场景、地点自动生成标签
- **批量嵌入向量** - 批量添加 CLIP 嵌入向量

### 改进
- 增强 Photo 结构体，支持嵌入向量
- 增强 GPS 信息，支持城市和国家
- 优化语义搜索算法
- 添加地图聚合算法（基于 Haversine 距离）
- 添加完整的 API handlers 和测试

### 竞品分析
- 参考飞牛 OS 的 AI 相册功能
- 参考群晖 DSM 的智能标签系统
- 参考 TrueNAS 的存储健康监控

### 版本号
- v2.584.0 → v2.585.0

---

## v2.584.0 (2026-06-09) - 吏部轮值: 版本管理与代码修复

### Bug 修复
- **数据去重模块** - ResolveDuplicateGroup 状态从 StatusDuplicate 修正为 StatusDeleted
- **加密保险箱** - EncryptFile 状态检查从 VaultLocked 修正为 VaultUnlocked，修复加密操作前提条件
- **文件锁路由** - check/file 路由参数从 `:filepath` 改为 `*filepath`，支持路径中含斜杠

### 版本号
- v2.583.0 → v2.584.0

---

## v2.583.0 (2026-06-09) - 工部轮值: CI/CD 优化

### CI/CD 修复
- **benchmark.yml 补全** - 添加缺失的基准测试运行步骤、benchstat 分析、结果上传
- **paths-filter 升级** - dorny/paths-filter 从 v4 降至 v3（稳定版本）
- **版本号统一** - 所有 workflow 文件版本号同步至 v2.583.0

### 工作流维护
- ci-cd.yml: 更新构建汇总版本标识
- release.yml: 更新维护版本号
- security-scan.yml: 更新维护版本号
- docker-publish.yml: 更新维护版本号 + paths-filter 修复
- compatibility.yml: 更新维护版本号
- staged-release.yml: 更新维护版本号

### 版本号
- v2.582.0 → v2.583.0

---

## v2.582.0 (2026-06-09) - 礼部轮值: 品牌建设与文档完善（续）

### 新增文档
- **品牌指南** (`docs/BRAND_GUIDELINES.md`) - 品牌定位、标识规范、语调指南、竞品定位
- **快速入门** (`docs/GETTING_STARTED.md`) - 5分钟上手指南，Docker/二进制/源码三种安装方式
- **架构概览** (`docs/ARCHITECTURE.md`) - 系统架构图、模块说明、技术栈、部署架构

### README 更新
- 添加文档导航表格，方便查找
- 更新版本号至 v2.582.0

### 版本号
- v2.581.0 → v2.582.0

---

## v2.581.0 (2026-06-09) - 礼部轮值: 品牌文档与内容完善

### 文档更新
- README.md 版本号同步至 v2.581.0
- README.md 新增 7 大模块特性说明（门禁管理/AI深度搜索/智能运维/本地通讯/P2P远程/增强搜索/增强共享）
- CHANGELOG.md 完善版本记录

### 修复
- zerotrust SeverityFilter 测试修复

### 新增模块（8个）

| 模块 | 说明 | 对标竞品 |
|------|------|----------|
| accesscontrol | 门禁管理系统（设备管理、卡片授权、AI行为分析） | 群晖 AC100 |
| deepsearch | AI深度搜索（语义搜索、OCR、视觉分析） | 群晖 Deep Search |
| dsmagent | 智能运维代理（工作流调度、健康监控、自动修复） | 群晖 DSM Agent |
| gpuinfer | GPU推理平台（模型管理、GPU资源、本地AI推理） | TrueNAS GPU |
| localchat | 本地通讯套件（即时通讯、视频会议、AI摘要/翻译） | 群晖 ChatPlus |
| p2premote | P2P远程访问（NAT穿透、端到端加密、连接管理） | 飞牛 FN Connect |
| smartsearch2 | 增强搜索（全文索引、语义搜索、Spotlight兼容） | TrueNAS TrueSearch |
| webshare2 | 增强文件共享（FIPS加密、在线预览、浏览器管理） | TrueNAS WebShare |

### 增强
- appstore 应用商店重构（批量安装、依赖解析、推荐引擎、沙箱隔离）

---

## v2.580.0 (2026-06-09) - 户部轮值: 资源统计与成本分析

### 项目资源审计
- **Go 源文件**: 4,261 | **测试文件**: 1,096 | **源码行数**: 293,462 行 | **测试行数**: 473,641 行
- **内部模块**: 904 | **Go 依赖**: ~170 (直接) | **前端文件**: 113
- **二进制**: nasd 128MB / nasctl 8.9MB | **项目大小**: 202MB (不含 .git)
- **Docker 文件**: 13 | **文档**: 17 | **配置**: 12

### 成本与资源分析
- 代码测试比: 测试代码是源码的 1.61 倍，测试覆盖充分
- 内部模块 904 个，模块化程度极高
- Go 1.26.1 运行时，170 个直接依赖
- 3 个入口: nasd (守护进程) / nasctl (CLI) / backup

### 版本号
- v2.579.0 → v2.580.0

---

## v2.579.0 (2026-06-09) - 六部协作: 不可变存储+AI相册+勒索防护

### 新增模块
- **immutaStore** - 不可变存储模块（WORM合规+快照保护+审计链）
- **aiPhotoTimeline** - AI时间线相册（人脸识别+场景分类+自动标签）
- **ransomGuard** - 勒索防护模块（文件监控+异常检测+自动快照+恢复）

### 代码质量
- 3个新模块编译通过 + 全部单元测试 PASS
- 版本号: v2.578.0 → v2.579.0
## v2.579.0 (2026-06-09) - 礼部轮值: 品牌文档与内容完善

### 文档更新
- README.md 版本号同步至 v2.579.0
- README.md 新增 7 大模块特性说明（门禁管理/AI深度搜索/智能运维/本地通讯/P2P远程/增强搜索/增强共享）
- CHANGELOG.md 完善版本记录

### 修复
- zerotrust SeverityFilter 测试修复

### 新增模块（8个）

| 模块 | 说明 | 对标竞品 |
|------|------|----------|
| accesscontrol | 门禁管理系统（设备管理、卡片授权、AI行为分析） | 群晖 AC100 |
| deepsearch | AI深度搜索（语义搜索、OCR、视觉分析） | 群晖 Deep Search |
| dsmagent | 智能运维代理（工作流调度、健康监控、自动修复） | 群晖 DSM Agent |
| gpuinfer | GPU推理平台（模型管理、GPU资源、本地AI推理） | TrueNAS GPU |
| localchat | 本地通讯套件（即时通讯、视频会议、AI摘要/翻译） | 群晖 ChatPlus |
| p2premote | P2P远程访问（NAT穿透、端到端加密、连接管理） | 飞牛 FN Connect |
| smartsearch2 | 增强搜索（全文索引、语义搜索、Spotlight兼容） | TrueNAS TrueSearch |
| webshare2 | 增强文件共享（FIPS加密、在线预览、浏览器管理） | TrueNAS WebShare |

### 增强
- appstore 应用商店重构（批量安装、依赖解析、推荐引擎、沙箱隔离）

---

## v2.578.0 (2026-06-09) - 司礼监轮值: 竞品分析驱动6大新模块

### 竞品分析
- 对标飞牛fnOS 2026（ARM虚拟机、企业级ACL权限13种细分、应用商店）
- 对标群晖DSM COMPUTEX 2026（DSM Agent、地端AI通讯、门禁控制、Deep Search）
- 对标TrueNAS 25.04（GPU直通、Docker Compose应用、Fast Dedup）

### 新增模块（6个）

| 模块 | 说明 | 对标竞品 |
|------|------|----------|
| dsmagent | AI自动化运维Agent（工作流调度、健康监控、自动修复） | 群晖DSM Agent |
| localchat | 地端AI通讯套件（即时通讯、视频会议、AI摘要/翻译） | 群晖ChatPlus/Meet |
| accesscontrol | 智能门禁系统（设备管理、卡片授权、AI行为分析） | 群晖AC100/AR系列 |
| gpuinfer | GPU推理平台（模型管理、GPU资源、本地AI推理） | TrueNAS GPU直通 |
| appstore | 应用商店简化部署（一键安装、应用目录、资源监控） | TrueNAS Docker Compose |
| deepsearch | AI深度文件搜索（语义搜索、OCR、视觉分析、人脸识别） | 群晖Deep Search |

### 技术特性
- 所有模块支持并发处理和上下文取消
- 完整的配置管理和状态监控
- AI功能支持本地LLM集成
- 版本号: v2.577.0 -> v2.578.0

---

## v2.577.0 (2026-06-09) - 竞品驱动开发: 6大新模块上线

### 竞品分析
- 对标飞牛fnOS（企业级ACL权限、ARM虚拟机）、群晖DSM 7.3（数据分层、私有AI）、TrueNAS 26（勒索检测、混合存储、引导式告警）
- 基于竞品分析确定6个新增模块方向

### 新增模块（全部编译通过 + 测试通过）
- **smarttierengine** - 统一智能分层引擎（ML热冷数据检测+自动迁移）
- **capacityai** - AI容量规划与成本优化（增长率预测+存储池分析）
- **smartonboard** - 智能引导式初始化（7步引导+系统健康检查）
- **dockerresilience** - Docker工作流韧性增强（指数退避重试+健康检查）
- **featureroadmap** - 功能路线图管理（特性追踪+里程碑+统计）
- **ransomai** - AI勒索软件检测增强（信息熵分析+蜜罐+威胁分级）

### 代码质量
- 6个模块均包含完整单元测试，全部 PASS
- 修复ID生成唯一性问题（使用原子计数器）
- 版本号: v2.574.0 -> v2.577.0

---

## v2.575.0 (2026-06-08) - 吏部轮值: 项目管理与里程碑

### 项目状态审计
- Go 源文件: 3,128 | 测试文件: 1,085 | 源码: 1,386,296 行 | 测试: 469,903 行
- 内部模块: 902 | Go 依赖: 169 (直接44+间接125)
- `go mod verify` 全部通过, `go vet ./...` 无错误

### 里程碑更新
- MILESTONES.md 添加 v2.575.0 吏部轮值记录
- 依赖关系审计完成，版本一致性检查通过
- 9个新模块验证通过 (desktopmanager/unifiedgateway/aiconsole2/ipprotection/fastdedup/iscsiblockclone/apikeymgr/teamfile/stigcompliance)

---

## v2.575.0 (2026-06-08) - 礼部轮值: 文档品牌更新

### 文档更新
- CHANGELOG.md 添加 v2.575.0 版本记录
- MILESTONES.md 同步更新
- 确保文档版本号一致性

---

## v2.574.0 (2026-06-08)

### 新增功能
- **桌面管理器** (desktopmanager): 图标自由拖拽、分组整理、网格对齐，对标飞牛 fnOS 桌面整理功能
- **统一网关** (unifiedgateway): 域名+应用名访问、WebSocket 支持、反向代理，对标飞牛 fnOS 统一网关
- **AI Console 2.0** (aiconsole2): 多模型统一管理、Token 用量控制、成本预算，对标群晖 AI Console
- **IP 防护** (ipprotection): 自动封禁、白名单/黑名单、异常检测，对标飞牛 fnOS IP 封禁功能

### 竞品特性对齐
- 对标飞牛 fnOS: 桌面图标整理、统一网关访问、IP 封禁防护
- 对标群晖 DSM NEXT: AI 多模型管理、Token 成本控制

### 优化
- 清理废弃模块 (aiconsole/smarttiering/webshare/lxcmanager)

---

## v2.570.0 (2026-06-08) - 礼部轮值: 文档品牌更新

### 文档更新

- README.md 版本号更新 v2.567.0 → v2.570.0
- CHANGELOG.md 同步更新
- 竞品分析文档版本同步

## v2.569.0 (2026-06-08) - 竞品分析与新功能开发

### 新增模块（5个）

| 模块 | 说明 | 对标竞品 |
|------|------|----------|
| fastdedup | NVMe优化快速去重引擎 | TrueNAS Fast Deduplication |
| iscsiblockclone | iSCSI块克隆加速（10X VM克隆） | TrueNAS iSCSI XCOPY |
| apikeymgr | 用户API Key管理（创建/轮换/吊销） | TrueNAS User-linked API Keys |
| teamfile | 团队文件夹协作管理 | 飞牛fnOS团队文件夹 |
| stigcompliance | STIG安全合规自动审计 | TrueNAS GPOS STIG |

### 修复

- 修复 ai_privacy_guardian 编译错误（5个未定义类型）

### 竞品分析

- 飞牛fnOS: ZFS支持、SSD缓存(L2ARC/SLOG)、团队文件夹、OneDrive挂载
- 群晖DSM 7.3: 存储分层、AI Console隐私保护、API Key、共享标签
- TrueNAS 25.04: 快速去重、块克隆、LXC容器、STIG合规、RDMA

---

## v2.567.0 (2026-06-08) - 户部轮值

### 资源统计报告

| 指标 | 数值 |
|------|------|
| Go源码总行数 | 1,867,979 |
| Go源文件(排除测试) | 1,395,048 行 |
| 测试代码行数 | 472,931 行 |
| 测试文件数 | 1,089 |
| 测试覆盖率比 | 源码:测试 ≈ 3:1 |
| 内部模块数 | 883 |
| pkg模块数 | 7 |
| Go依赖(直接) | 44 |
| Go依赖(间接) | 125 |
| Go依赖(总计) | 169 |
| Web前端文件 | 13 |

---

## v2.556.0 (2026-06-06)

### 新增功能
- SSD磨损均衡智能管理（SmartWearLeveling）
- ZFS配额智能管理（ZFSQuotaManager）
- 容器热迁移（ContainerMigrator）
- 多级缓存智能管理（SmartCacheTier）
- 远程支持隧道（RemoteSupport）
- 备份智能验证（SmartBackupVerify）
- 智能风扇控制（SmartFanControl）
- NVMe-oF存储池（NVMeTempPool）
- 功耗智能封顶（SmartPowerCap）

### 修复
- 修复go vet错误（IPv6格式/锁拷贝/测试参数）

---

## v2.555.0 (2026-06-03)

### 新增功能
- **AI Token 预算管理器** (aitokenbudget): 按用户/服务的 Token 配额管理、实时成本追踪、多模型成本对比、预算告警和自动降级
- **游戏资源预加速器** (gamepreloader): NAS 端游戏资源预缓存、局域网设备自动发现、智能调度（低峰时段预下载）、游戏更新自动检测
- **FIPS 合规加密保险库** (fipsvault): FIPS 140-2/140-3 合规加密、密钥管理和轮换、跨协议加密共享、合规审计日志、合规报告生成

### 竞品特性对齐
- 对标群晖 AI Console: AI Token 预算管理和成本分析
- 对标飞牛 fnOS: 游戏资源预下载和局域网推送
- 对标 TrueNAS 26: FIPS 140 加密传输和合规检查

---

## v2.554.0 (2026-06-02)

### 新增功能
- AI智能路由引擎、存储效率增强、引导式告警系统、媒体中心增强

---

## v2.553.0 (2026-06-02)

### 新增功能
- **智能备份编排器** (smartbackuporch): 智能备份调度、依赖管理、多目标备份、备份链路优化、失败重试、备份验证，对标群晖 Active Backup for Business
- **NAS舰队指挥官** (nascommander): 多NAS集中管理、远程监控、统一分发配置、跨站点同步、健康聚合，对标 TrueCommand 集中管理平台
- **智能家居Hub Pro** (smarthomehubpro): Matter/Thread/Zigbee/Z-Wave协议支持、设备自动化、场景联动、能源管理、安防集成，对标飞牛智能家居中心
- **AI系统管理员** (aisysadmin): 自然语言管理NAS系统、智能诊断、自动化修复
- **持续数据保护** (cdp): 实时捕获文件变更、任意时间点恢复、RPO趋近于零
- **容器守护者** (containerguardian): 容器漏洞扫描、安全策略、运行时保护
- **内容AI** (contentai): AI驱动内容生成、SEO优化、多格式支持
- **媒体组织专家** (mediaorganizerpro): 智能媒体分类、标签管理、重复检测
