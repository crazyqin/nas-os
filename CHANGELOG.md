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
