# 竞品分析 - 群晖 DSM & 飞牛 fnOS

## 调研日期
2026-03-23

## 群晖 DSM 核心功能

### 1. Synology Photos
- **人脸识别**: 自动识别照片中的人物
- **地理标签**: 基于位置自动生成相册
- **条件相册**: 按人物/物体/地点/相机自动聚合
- **RAW 支持**: 保留原始画质
- **多端访问**: Web/移动端/Apple TV/Android TV
- **分享功能**: 链接分享、密码保护、有效期设置

### 2. Synology Drive
- **多端同步**: Windows/macOS/Linux/iOS/Android
- **按需同步**: 节省本地存储
- **版本控制**: Intelliversioning 算法
- **文件锁定**: 防止冲突
- **共享标签**: 团队协作
- **审计日志**: 40+ 种操作监控
- **NAS-to-NAS 同步**: ShareSync

### 3. Cloud Sync
- **多云支持**: Dropbox/Google Drive/OneDrive/BackBlaze B2/S3
- **双向同步**: 单向或双向
- **加密压缩**: 客户端 AES-256 加密
- **流量控制**: 带宽限制

### 4. Hyper Backup
- **多版本备份**: 块级增量 + 去重
- **加密**: AES-256
- **完整性检查**: 定期校验
- **多目标**: 本地/外部设备/其他 NAS/云服务
- **暂停恢复**: 断点续传

### 5. Virtual Machine Manager
- **多系统**: Windows/Linux/DSM
- **快照**: 最多 255 个保留
- **集群**: 最多 7 节点
- **高可用**: HA 支持
- **实时迁移**: Live Migration

### 6. 协作套件
- **Synology Office**: 文档/表格/幻灯片
- **Chat**: 团队即时通讯
- **MailPlus**: 私有邮件服务器
- **Calendar**: 日历管理
- **Contacts**: 联系人管理
- **Note Station**: 笔记管理

### 7. Active Backup for Business
- **物理机备份**: Windows/Linux
- **虚拟机备份**: VMware/Hyper-V
- **M365 备份**: Microsoft 365
- **GWS 备份**: Google Workspace

### 8. Audio Station
- **音乐库管理**: 专辑/艺术家/播放列表
- **多端播放**: Web/移动端/DLNA
- **歌词显示**: 自动匹配

---

## 飞牛 fnOS 特点

### 优势
- **免费开源**: 降低使用门槛
- **应用中心**: Docker 应用生态
- **简单部署**: 开箱即用
- **社区驱动**: 用户反馈快速迭代

### 功能特点
- 基于 Docker 的应用架构
- Web 端管理界面
- 文件管理基础功能
- 媒体库支持

---

## nas-os 差距分析

### 已有模块 (68个)
ai_classify, api, audit, auth, automation, backup, billing, budget, cache,
cloudsync, cluster, compliance, compress, concurrency, container, dashboard,
database, dedup, disk, docker, downloader, files, ftp, health, iscsi, ldap,
logging, media, monitor, network, nfs, notification, notify, office, optimizer,
perf, performance, photos, plugin, prediction, quota, rbac, recovery, reports,
replication, s3, scanner, scheduler, search, security, service, smb, stats,
storage, sync, system, transfer, trash, upgrade, user, vm, webdav, webhook,
wireguard

### 缺失功能 (优先级排序)

#### P0 - 核心功能缺失
| 功能 | 模块建议 | 工作量 | 说明 |
|------|----------|--------|------|
| Drive 同步客户端 | internal/drive | 大 | 多端文件同步核心功能 |
| Audio Station | internal/audio | 中 | 音乐管理播放 |
| Calendar | internal/calendar | 中 | 日历管理 |
| Contacts | internal/contacts | 中 | 联系人管理 |

#### P1 - 功能增强
| 功能 | 涉及模块 | 工作量 | 说明 |
|------|----------|--------|------|
| Photos AI 增强 | photos, ai_classify | 中 | 人脸识别、地理标签、条件相册 |
| 文件锁定 | files, sync | 小 | 协作冲突避免 |
| 版本控制增强 | files | 中 | Intelliversioning 算法 |
| 审计日志增强 | audit | 小 | 更多操作类型 |

#### P2 - 企业功能
| 功能 | 模块建议 | 工作量 | 说明 |
|------|----------|--------|------|
| Mail 邮件服务器 | internal/mail | 大 | 私有邮件解决方案 |
| Chat 团队通讯 | internal/chat | 大 | 即时通讯 |
| Note Station | internal/notes | 中 | 笔记管理 |
| M365/GWS 备份 | backup | 大 | 企业云服务备份 |

---

## 六部开发排期

### 第一阶段 (P0)
| 周期 | 吏部 | 兵部 | 礼部 | 刑部 | 工部 | 户部 |
|------|------|------|------|------|------|------|
| W1-W2 | 版本规划 | Drive 模块架构设计 | 功能文档 | 安全评估 | CI/CD 配置 | 资源评估 |
| W3-W4 | 里程碑管理 | Audio 模块开发 | API 文档 | 代码审计 | 测试覆盖 | 成本统计 |
| W5-W6 | 版本发布 | Calendar 模块开发 | 用户手册 | 漏洞扫描 | Docker 配置 | 进度报告 |

### 第二阶段 (P1)
| 周期 | 吏部 | 兵部 | 礼部 | 刑部 | 工部 | 户部 |
|------|------|------|------|------|------|------|
| W7-W8 | 版本规划 | Photos AI 增强 | 功能文档 | 安全评估 | 性能优化 | 资源评估 |
| W9-W10 | 里程碑管理 | 文件锁定实现 | API 文档 | 代码审计 | 测试覆盖 | 成本统计 |

### 第三阶段 (P2)
| 周期 | 吏部 | 兵部 | 礼部 | 刑部 | 工部 | 户部 |
|------|------|------|------|------|------|------|
| W11-W14 | 版本规划 | Mail/Chat 模块架构 | 功能文档 | 安全评估 | CI/CD 配置 | 资源评估 |
| W15-W18 | 里程碑管理 | Note Station 开发 | API 文档 | 代码审计 | 测试覆盖 | 成本统计 |

---

## 技术参考

### Synology Drive 同步算法
- 增量同步: 块级差异检测
- 冲突解决: 文件锁定 + 版本合并
- 按需同步: 本地占位符 + 远程拉取

### Photos AI 特性
- 人脸识别: 深度学习模型
- 物体检测: YOLO/SSD
- 地理标签: EXIF 解析 + 反向地理编码

### Hyper Backup 技术
- 去重: 块级去重 + 哈希索引
- 加密: AES-256-GCM
- 完整性: Merkle Tree 校验

---

## 结论

nas-os 在基础设施模块上已较为完善，但在用户体验功能上与群晖存在较大差距。建议：

1. **优先补齐 P0 功能**：Drive、Audio、Calendar、Contacts
2. **增强现有模块**：Photos AI、文件锁定、版本控制
3. **长期规划 P2**：Mail、Chat、企业备份

预计完整功能对标需 18-24 周开发周期。