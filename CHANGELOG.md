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
| 前端代码行数 | 8,186 |
| 配置文件 | 91 |
| 文档(MD) | 10 文件 / 4,814 行 |
| Shell脚本 | 49 |
| Dockerfile | 7 |
| docker-compose | 10 |
| Makefile | 2 |
| 总文件数 | 4,552 |
| 总目录数 | 1,057 |
| 项目大小(不含.git) | 202 MB |
| nasd 二进制 | 128 MB |
| nasctl 二进制 | 8.9 MB |

### 新增模块
- **AI隐私卫士** (ai_privacy_guardian): AI推理时隐私保护，自动识别敏感数据、实时脱敏、GDPR/个人信息保护法合规检查、数据流向追踪（216行）
- **工作流自动化引擎** (workflow_automation): 对标群晖DSM Agent，支持触发器/条件/动作/执行器/日志完整工作流编排（源码3,316行，测试2,494行）

### 版本同步
- VERSION: v2.555.0 → v2.567.0
- internal/version/version.go: 2.566.0 → 2.567.0

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
- **NAS守护者** (nasguardian): 系统安全监控、入侵检测、异常告警
- **项目中心** (projecthub): 项目管理、任务跟踪、团队协作
- **统一AI助手** (smartaiassistant): 集成所有AI模块的统一入口、自然语言NAS管理
- **智能计费系统** (smartbilling): 家庭多用户资源使用计费、预算管理、账单生成
- **智能能耗预测** (smartenergyforecast): 能耗数据分析、预测建模、异常检测、节能建议
- **智能硬盘LED** (smarthddled): 硬盘状态LED指示、故障定位、维护提醒
- **智能配额推荐** (smartquotarecommender): 基于使用模式的配额智能推荐
- **智能工作流引擎** (smartworkflow): 可视化工作流编排、定时任务、条件触发、多步骤自动化
- **存储网络2** (storagefabric2): 增强型存储网络管理、多协议支持、性能优化

### 竞品学习
- 学习群晖 Active Backup for Business: 智能备份编排器（备份调度、依赖管理、多目标备份）
- 学习 TrueCommand: NAS舰队指挥官（集中管理、远程监控、统一分发）
- 学习飞牛智能家居: 智能家居Hub Pro（Matter/Thread协议、场景联动、能源管理）
- 创新方向: AI系统管理员（自然语言管理）、持续数据保护（RPO趋近于零）、统一AI助手面板

## v2.552.0 (2026-06-02)

### 新增功能
- **CXL内存池化** (cxlmempool): CXL 1.1/2.0/3.0/3.1设备管理、NUMA感知内存分层、多策略分配、设备发现与监控
- **向量数据库** (vectordb): 嵌入式向量数据库，支持余弦/欧氏/内积/曼哈顿距离、元数据过滤、批量插入、TopK搜索
- **气隙备份** (airgapbackup): 物理隔离备份管理、WORM合规/治理模式、链式校验、定时断连、灾难恢复
- **DPDK高性能网络加速** (dpdkaccel): 用户态网络栈、RSS多队列、流表规则、流量分类QoS、零拷贝收发包
- **SmartNIC/DPU网络卸载** (smartnicoffload): SmartNIC设备发现、OVS/IPsec/TLS/压缩/RDMA卸载、性能监控
- **GPU推理服务** (gpuinference): 多模型并发推理、模型热加载、显存管理、推理队列调度、多精度支持

### 竞品学习
- 学习 TrueNAS 25.10: CXL内存池化、DPDK网络加速、SmartNIC卸载、GPU推理
- 学习 Synology DSM: 气隙备份、WORM保护、链式校验
- 创新方向: 向量数据库(RAG/AI)、GPU推理服务、CXL内存分层

## v2.550.0 (2026-06-02)

### 新功能
- 智能容器资源限制器（containerreslimiter）：基于历史使用模式自动调整CPU/内存限制，支持保守/平衡/激进策略，百分位数分析，自动调优
- 智能数据放置引擎（smartdataplacement）：基于数据温度（热/温/冷/冻结）自动在NVMe/SSD/HDD/Cloud间迁移数据，放置策略可配置
- NAS功率预算管理（naspowerbudget）：设备能耗监控、功率预算控制、能耗报告、碳排放计算、节能建议、定时调度

### 改进
- 存储成本分析器增强：新增TCO分析、ROI计算、多方案对比、数据优化收益估算、增强成本预测
- Zero Trust访问控制增强：新增ABAC属性访问控制、设备信任评估、风险引擎、审计日志
- 修复Compatibility Check workflow并发问题：改为cancel-in-progress模式避免旧任务堆积

### 竞品对齐
- 对标群晖Container Manager：智能容器资源限制器（借鉴容器资源管理功能）
- 对标TrueNAS分层存储：智能数据放置引擎（借鉴ZFS分层存储策略）
- 对标飞牛节能管理：功率预算管理（借鉴飞牛功耗优化功能）

## v2.549.0 (2026-06-01)

### 新功能
- 智能存储健康预测（smarthealthpredict）：S.M.A.R.T数据分析、AI故障预测、健康评分、趋势分析、告警建议

### 改进
- 新增系统诊断中心（diagnostics）：一键诊断、健康评分、问题检测、诊断报告、历史记录
- 新增智能温控管理（thermalmanager）：温度监控、风扇控制、散热策略、过热保护
- 新增文件版本控制（fileversion）：版本历史、差异对比、回滚恢复、自动清理
- 新增功率管理器（powermanager）：功耗监控、节能策略、定时开关机、负载均衡
- 新增数据完整性校验（dataintegrity）：校验和计算、损坏检测、自动修复、完整性报告

### 竞品对齐
- 对标群晖：存储健康预测（借鉴存储管理器S.M.A.R.T监控）、系统诊断（借鉴存储分析器）
- 对标 TrueNAS：数据完整性校验（借鉴Self-Healing Checksums）
- 对标飞牛：智能温控管理（借鉴飞牛散热管理）

## v2.548.0 (2026-06-01)

### 新功能
- 合规扫描器（compliancescanner）：自动化合规检查、规则引擎、修复建议、合规报告生成
- dRAID分布式RAID（draid）：分布式热备、动态重建、性能优化、容错管理
- 功率预算管理（powerbudget）：功耗实时监控、预算告警、能耗分析、节能优化
- 智能带宽预测（smartbandwidthpredict）：流量模式学习、带宽预测、QoS调度、异常检测
- 合规审计（complianceaudit）：多标准合规评估、审计日志、整改跟踪、合规报告
- 成本优化器（costoptimizer）：存储成本分析、去重压缩、成本优化建议、ROI计算
- 存储分层（storagetiering）：智能数据分层、冷热迁移、性能优化、容量规划
- WAN链路规划（wanreplanner）：链路监控、故障切换、负载均衡、QoS管理

### 改进
- 修复 filecache CleanupInterval 为零时 NewTicker panic
- 更新 VERSION 至 v2.548.0

### 竞品对齐
- 对标 TrueNAS：合规扫描器（借鉴TrueCommand合规检查）、WAN链路规划
- 对标群晖：功率预算管理（借鉴DSM电源管理）、成本优化器
- 对标飞牛：智能带宽预测（借鉴飞牛网络优化）

## v2.547.0 (2026-06-01)

### 新功能
- AI文件恢复系统（aifilerecovery）：深度扫描、AI模式识别、多文件系统支持、恢复预览、完整性校验
- 存储ROI分析（storageroi）：ROI计算、TCO总拥有成本分析、容量规划、云端对比
- 家庭实验室应用商店（homelabstore）：精选自托管应用目录、一键安装/卸载、自动更新、社区评分
- 零信任网络访问（zerotrustaccess）：身份驱动访问控制、微分段隔离、持续认证、设备态势评估

### 改进
- 更新 VERSION 至 v2.547.0

### 竞品对齐
- 对标群晖：应用商店（借鉴套件中心）、存储ROI分析（借鉴存储分析器）
- 对标 TrueNAS：零信任安全（借鉴TrueCommand安全架构）
- 对标飞牛：AI文件恢复（借鉴飞牛数据恢复功能）
