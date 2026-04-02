# 竞品调研报告 - 2026-04-02

## 调研对象
- TrueNAS SCALE 24.10 (Electric Eel)
- 群晖 DSM 7.x
- 飞牛 fnOS

---

## TrueNAS 24.10 Electric Eel 新功能

### 核心新特性
| 功能 | 说明 | nas-os 对标建议 |
|------|------|----------------|
| **RAIDZ Expansion** | 单盘扩容RAIDZ vdev（OpenZFS赞助功能） | P0 - 已规划M106 |
| **Docker Apps** | 从Kubernetes转向Docker，简化部署 | ✅ 已有Docker管理 |
| **TrueCloud Backup** | Storj云备份任务简化 | ⚠️ 可增强云备份 |
| **Global Search** | 全局UI搜索页面/设置 | ✅ WebShare搜索已实现 |
| **Dashboard重做** | 更多widget、自定义报告 | ✅ 已有Dashboard |
| **NVMe S.M.A.R.T.** | UI支持NVMe健康检测 | ⚠️ 需增强 |
| **ZFS Fast Dedup** | OpenZFS快速去重（实验性） | 📋 评估 |
| **SMB ADS保留** | 从远程服务器摄取数据时保留ADS | ✅ SMB已有 |

### 技术改进
- 安装程序重写，支持未来开发
- sssd替换nslcd，改进Kerberos/NFS/SMB
- 生成唯一系统ID
- UI表格优化+全局搜索集成

---

## 群晖 DSM 优势功能

### 应用生态
| 功能 | 说明 | nas-os 状态 |
|------|------|------------|
| Photos | 智能相册 | ✅ 已有 |
| Drive | 文件同步 | ⚠️ 可增强 |
| Cloud Sync | 云同步 | ✅ 已有 |
| Office | 协作办公 | ⚠️ OnlyOffice集成 |
| Hyper Backup | 多目标备份 | ✅ 已有 |
| Snapshot Replication | 快照复制 | ✅ 已有 |
| VMM | 虚机管理 | ✅ 有容器管理 |
| CMS | 多系统集中管理 | ✅ 已实现 |
| Active Insight | 集群监控 | ✅ 已有监控 |
| Hybrid Share | 本地+云混合 | ✅ 云挂载 |
| Synology Tiering | 存储分层 | ✅ Fusion Pool |

### 差异化优势
- 完整应用生态系统
- 强大的多租户协作能力
- 企业级备份方案

---

## 飞牛 fnOS 特点

### 核心功能
| 功能 | 说明 | nas-os 状态 |
|------|------|------------|
| FN Connect | 多系统云端管理 | ✅ CMS已实现 |
| AI人脸相册 | 智能识别 | ✅ 已有 |
| 按需唤醒硬盘 | 眀电特性 | ❌ 缺失 |
| Intel核显加速 | GPU调度 | ✅ 已有 |

---

## nas-os 竞争力分析

### 已超越竞品
1. **本地LLM服务** - Ollama集成，独家AI能力
2. **AI数据脱敏** - 隐私保护，竞品无
3. **WriteOnce** - WORM文件系统，合规优势
4. **勒索实时防护** - WriteOnce + 实时监控

### 对标重点
1. **RAIDZ Expansion** - TrueNAS独家，OpenZFS 2.3
2. **全局搜索** - TrueNAS 24.10新功能
3. **Docker Apps简化** - 简化应用部署流程
4. **NVMe S.M.A.R.T.** - UI集成健康检测

### 差异化策略
| 维度 | nas-os | TrueNAS | 群晖 | 飞牛 |
|------|--------|---------|------|------|
| 本地AI | ✅独家 | ❌ | ✅ | ❌ |
| 开源免费 | ✅ | ✅社区版 | ❌付费 | ✅ |
| 企业特性 | ⚠️增强中 | ✅ | ✅ | ⚠️ |
| 应用生态 | ⚠️建设中 | ✅ | ✅ | ⚠️ |

---

## 第141轮开发建议

### P0 - 核心对标
1. RAIDZ Expansion（已规划M106）
2. NVMe S.M.A.R.T. UI集成
3. 全局搜索增强

### P1 - 差异化增强
1. AI服务API完善
2. 应用模板扩展
3. 成本分析多节点

### P2 - 体验优化
1. Dashboard widget扩展
2. 安装程序简化
3. 文档完善

---

## 六部任务分配（第141轮）

### 兵部
- RAIDZ Expansion API设计
- NVMe S.M.A.R.T.集成

### 工部
- CI/CD稳定性保障
- Docker简化部署

### 刑部
- 安全扫描持续
- 漏洞修复

### 户部
- 多节点成本分析
- 资源预测报告

### 礼部
- 竞品对比文档更新
- 功能发布宣传

### 吏部
- 版本规划协调
- 测试覆盖验证

---

**调研日期**: 2026-04-02
**下次更新**: 2026-04-15