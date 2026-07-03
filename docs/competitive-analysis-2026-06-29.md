# 竞品调研报告 2026-07-03

本次更新聚焦 DSM 7.4、TrueNAS 26、飞牛 fnOS 与 QNAP QuTS hero h6.0 的公开方向，并同步到 README、CHANGELOG 与主竞品分析文档。

## Synology DSM 7.4 / COMPUTEX 2026

### 核心信号
- **本地 AI / AI Console**：强调私有化模型、数据驻留、个人信息脱敏和套件级 AI 能力。
- **存储效率**：持续强化压缩、去重、冷热分层、容量趋势和成本优化叙事。
- **RBAC 与企业权限**：细粒度角色、域控集成、审计和最小权限。
- **DSM Agent / Log Center / Cluster Manager**：把 AI 运维、集中日志和多节点管理打包为企业可观测能力。

### 对标映射
| 功能 | nas-os 状态 | DSM 7.4 方向 |
|------|------------|--------------|
| 本地 AI | ✅ 本地 LLM、CLIP、MCP、语义搜索治理 | ✅ 本地 AI / AI Console |
| AI 脱敏 | ✅ AI 脱敏、local-only 搜索审计 | ✅ 去识别化与私有 AI |
| 存储效率 | ✅ Smart Dedup、压缩/去重统计、生命周期、ROI | ✅ 存储效率与容量优化 |
| RBAC | ✅ RBAC/MFA/设备信任/SSO | ✅ 细粒度权限与企业目录 |
| 日志与运维 | ✅ 日志中心、重启原因历史、健康探针 | ✅ Log Center / DSM Agent |

## TrueNAS 26

### 核心信号
- **WebShare**：浏览器文件共享与协作体验。
- **TrueSearch**：面向大规模文件的快速搜索与 Spotlight 类体验。
- **LXC**：低开销 Linux 工作负载与容器运维能力。
- **FEC**：高速网络链路可靠性配置。
- **重启原因**：系统重启原因记录，服务于运维复盘。

### 对标映射
| 功能 | nas-os 状态 | TrueNAS 26 |
|------|------------|------------|
| WebShare | ✅ WebShare Pro、增强文件共享 | ✅ WebShare |
| TrueSearch | ✅ 增强搜索、语义搜索治理 | ✅ TrueSearch |
| LXC | ✅ LXC 容器沙箱、迁移计划 | ✅ LXC 支持 |
| FEC | ✅ v3.8.0 FEC 推荐/配置意图/审计 | ✅ 高速网络可靠性 |
| 重启原因 | ✅ v3.8.0 重启原因历史 | ✅ 重启原因记录 |

## 飞牛 fnOS

### 核心信号
- **媒体中心**：影视海报墙、刮削、播放进度、多端家庭影音体验。
- **远程访问**：低门槛外网访问、家庭用户友好。
- **AI 相册**：人物/场景识别与图片管理。
- **权限与应用易用性**：Windows ACL、Docker/应用中心持续优化。

### 对标映射
| 功能 | nas-os 状态 | fnOS 方向 |
|------|------------|-----------|
| 媒体中心 | ✅ 智能海报墙、媒体服务、Discovery Digest | ✅ 影视墙/媒体中心 |
| 远程访问 | ✅ P2P、内网穿透、VPN、WebShare | ✅ FN Connect 类体验 |
| AI 相册 | ✅ 人脸识别、CLIP 以文搜图 | ✅ AI 相册 |
| 应用中心 | ✅ 模板商店、Docker Compose UI、评价系统 | ✅ 家庭应用生态 |
| 权限 | ✅ ACL/RBAC/审计 | ✅ Windows ACL |

## QNAP QuTS hero h6.0

### 核心信号
- **passkeys**：无密码登录与现代身份安全。
- **immutable snapshots**：不可变快照、防勒索、合规保留。
- **FileTiers**：文件分层与冷热数据策略。
- **ZFS 数据保护**：快照、压缩、去重、校验等企业数据保护能力。

### 对标映射
| 功能 | nas-os 状态 | QNAP h6.0 方向 |
|------|------------|----------------|
| passkeys | 📋 规划：WebAuthn/passkeys；现有 MFA/设备信任/SSO | ✅ passkeys |
| 不可变快照 | ✅ WriteOnce、不可变备份、合规快照审计 | ✅ immutable snapshots |
| FileTiers | ✅ 智能分层、生命周期、ROI/预算预测 | ✅ FileTiers |
| ZFS 数据保护 | ✅ ZFS 兼容能力、快照、压缩/去重统计 | ✅ QuTS hero ZFS |

## 下一步建议（优先级排序）

### P0 - 公开文档与产品叙事
1. **统一“本地 AI 治理”叙事**：本地 LLM、CLIP、MCP、语义搜索、脱敏和审计统一描述。
2. **统一“存储效率中心”叙事**：压缩、去重、分层、生命周期、ROI、预算预测放在同一条产品线。
3. **不可变数据保护套件**：把 WriteOnce、immutable snapshots、不可变备份、合规快照审计和气隙备份合并表达。

### P1 - 功能路线
4. **passkeys/WebAuthn**：补齐无密码登录、设备绑定、恢复码和审计事件。
5. **TrueSearch 类规模指标**：补充索引规模、延迟、SSD 索引策略、重建策略和权限过滤。
6. **LXC 迁移向导**：从计划生成扩展到 Dry Run、可执行步骤、回滚和结果审计。

### P2 - 体验优化
7. **媒体中心端到端体验**：打通刮削、海报墙、转码、字幕、远程访问和分享权限。
8. **多平台迁移助手**：补充从 DSM/TrueNAS/fnOS/QNAP 迁移的配置映射和校验清单。

---

## 历史记录：2026-06-29

# 竞品调研报告 2026-06-29

## TrueNAS 26 (2026年4月 BETA)

### 核心更新
- **发布节奏**: 改为年度发布（之前半年一次），版本号简化为 "26.1" 格式
- **OpenZFS 2.4**: 混合池改进（flash+HDD）、物理块重写、动态gang header
- **Linux Kernel 6.18 LTS**: 新硬件支持
- **WebShare**: 浏览器文件分享，Dropbox式体验，FIPS 140加密传输，支持SMB/AD/NFSv4互操作
- **TrueSearch (Spotlight)**: 亚秒级搜索，支持10亿文件规模，SSD索引，macOS Spotlight集成
- **LXC容器**: 完全支持，低开销Linux工作负载部署
- **有状态SMB HA故障转移**: SMB会话状态跨控制器故障转移保持
- **SMB Spotlight搜索**: macOS客户端可直接Spotlight搜索SMB共享
- **勒索软件检测与防护**
- **400GbE网络支持**: V-Series硬件
- **API现代化**: JSON-RPC 2.0 WebSocket + SCRAM-SHA-512认证

### 对标差距
| 功能 | nas-os状态 | TrueNAS 26 |
|------|-----------|------------|
| WebShare | ✅ 已有WebShare Pro | ✅ WebShare + TrueSearch集成 |
| 全文搜索 | ⚠️ 基础搜索 | ✅ TrueSearch亚秒级/10亿文件 |
| LXC容器 | ✅ 已有 | ✅ 完全支持 |
| SMB HA有状态故障转移 | ⚠️ 基础HA | ✅ 有状态故障转移 |
| OpenZFS 2.4混合池 | ⚠️ 基础分层 | ✅ 混合池改进 |
| 勒索检测 | ✅ 已有ML检测 | ✅ 勒索检测防护 |
| API现代化 | ⚠️ REST API | ✅ WebSocket + SCRAM认证 |

## 群晖 DSM 7.3

### 核心更新
- **灵活数据分层**: 冷热数据自动分层，自定义规则（访问频率/时间）
- **共享标签**: 团队协作文件标签
- **文件请求**: 通过链接安全收集文件
- **文件锁定**: 防止协作同步错误
- **邮件审核**: 管理员审查敏感邮件
- **私有云AI**: 串接AI模型（云/本地LLM），MailPlus/Office AI辅助
- **AI去识别化**: Synology AI Console本地端遮蔽个人信息
- **弹性存储加密**: 密码解锁加密存储空间
- **灵活域控管**: 仅同步选定OU，最小权限原则

### 对标差距
| 功能 | nas-os状态 | DSM 7.3 |
|------|-----------|----------|
| 数据分层 | ✅ 已有Fusion Pool | ✅ 自动冷热分层+自定义规则 |
| 文件请求 | ❌ 无 | ✅ 链接收集文件 |
| 共享标签 | ⚠️ 基础 | ✅ 团队协作标签 |
| AI集成 | ✅ 已有AI模块 | ✅ 云+本地LLM，Office/Mail集成 |
| AI隐私脱敏 | ✅ 已有privacyproxy | ✅ AI Console去识别化 |
| 存储加密 | ✅ 已有 | ✅ 密码解锁弹性加密 |
| 邮件审核 | ❌ 无 | ✅ 敏感邮件审查 |

## 飞牛 fnOS v1.2.0012 (2026-06-25)

### 核心更新
- **Windows ACL权限**: 13种权限+"允许/拒绝"组合，父级继承至指定子级
- **预览加密PDF**: 预览时输入密码解锁
- **大文件读写优化**: 性能提升
- **ZFS快照目录适配**
- **RAID1/10写入性能提升**
- **RAID5/6初始化方式可选**
- **SMB日志优化**: 减少nobody无效探测日志
- **Docker稳定性**: 修复unless-stopped自启动、关机卡住问题
- **Gmail/Outlook邮件通知**: OAuth授权机制
- **CPU温度修复**

### 对标差距
| 功能 | nas-os状态 | fnOS v1.2 |
|------|-----------|-----------|
| Windows ACL | ✅ 已有19种权限 | ✅ 13种权限+继承 |
| PDF预览解锁 | ❌ 无 | ✅ 预览加密PDF |
| RAID性能优化 | ⚠️ 基础 | ✅ RAID1/10写入优化 |
| Docker稳定性 | ✅ 已有容器守护 | ✅ 修复多个Docker问题 |
| 邮件通知OAuth | ⚠️ 基础SMTP | ✅ Gmail/Outlook OAuth |

## 下一步建议（优先级排序）

### P0 - 快速跟进
1. **全文搜索引擎增强** - 对标TrueSearch，提升搜索性能到亚秒级
2. **文件请求功能** - 对标DSM，通过链接安全收集文件
3. **预览加密PDF** - 对标fnOS，预览时解锁

### P1 - 中期规划
4. **SMB有状态HA故障转移** - 对标TrueNAS，会话状态跨故障转移保持
5. **数据分层自定义规则** - 对标DSM，按访问频率自动分层
6. **邮件通知OAuth** - 对标fnOS，支持Gmail/Outlook OAuth
7. **API现代化** - 对标TrueNAS，WebSocket + SCRAM认证

### P2 - 长期规划
8. **混合池增强** - 对标OpenZFS 2.4，flash+HDD混合池改进
9. **存储弹性加密** - 对标DSM，密码解锁加密存储
10. **邮件审核机制** - 对标DSM，敏感邮件审查
