# 六部任务分配 - Round237 - 群晖DSM 7.3深度对标 + TrueNAS 27预研

**日期**: 2026-04-24
**调度**: 司礼监
**主题**: 竞品深度对标 + 新功能规划 + v2.465.0发布

## 竞品调研成果（司礼监）

### 群晖 DSM 7.3 核心特性学习
| 功能 | 说明 | nas-os对标 | 优先级 |
|------|------|-----------|--------|
| Photos | AI照片管理 | ✅ 已实现 | - |
| Presto | 文件传输加速 | 📋 待开发 | **P1** |
| Audio Station | 音乐管理 | ❌ 未规划 | P1 |
| Hybrid Share | 混合云存储 | ✅ 云挂载 | - |
| Office | 在线协作 | ✅ OnlyOffice | - |
| Secure SignIn | 安全认证 | ✅ AMFA | - |
| Active Insight | 监控平台 | ✅ Fleet | - |

### TrueNAS 25.10/27 新特性学习
| 功能 | 说明 | nas-os对标 | 优先级 |
|------|------|-----------|--------|
| SMB Multichannel | 多通道传输 | 📋 待开发 | **P0** |
| SMB Auditing | 操作审计 | 📋 待开发 | **P0** |
| RDMA | 高性能传输 | 📋 预研 | P1 |
| LXC Sandboxes | 应用隔离 | 📋 评估 | P2 |
| KMIP | 密钥管理 | 📋 预研 | P2 |
| FIPS 140 | 加密合规 | ❌ 未规划 | P2 |
| Rootless Admins | 安全管理 | 📋 待开发 | P1 |

---

## Round237 六部任务分配

### ⚔️ 兵部（软件工程）- P0核心

| 任务 | 对标 | 状态 | 预估 |
|------|------|------|------|
| **SMB Multichannel设计文档** | TrueNAS 25.10 | 📋 待开发 | 3h |
| **SMB Auditing API实现** | TrueNAS 25.10 | 📋 待开发 | 2h |
| Presto传输加速原型 | 群晖DSM 7.3 | 📋 待开发 | 4h |

**交付要求**:
- SMB Multichannel设计文档（docs/SMB_MULTICHANNEL_DESIGN.md）
- SMB Auditing API代码（internal/smb/auditing.go）
- Presto传输加速设计文档

---

### 🏗️ 工部（DevOps）- P0基础

| 任务 | 对标 | 状态 | 预估 |
|------|------|------|------|
| **CI稳定性验证** | - | ✅ 已验证 | 0.5h |
| RDMA需求评估报告 | TrueNAS 25.10 | 📋 待开发 | 2h |
| LXC容器可行性分析 | TrueNAS 25.10 | 📋 待开发 | 3h |

**交付要求**:
- RDMA评估报告（docs/RDMA_EVALUATION.md）
- LXC可行性分析（docs/LXC_SANDBOX_ANALYSIS.md）

---

### 📜 刑部（安全合规）- P0安全

| 任务 | 对标 | 状态 | 预估 |
|------|------|------|------|
| **SMB Auditing安全设计** | TrueNAS 25.10 | 📋 待开发 | 2h |
| Rootless Admins安全模型 | TrueNAS 25.10 | 📋 待开发 | 3h |
| KMIP密钥管理预研 | TrueNAS 25.10 | 📋 待开发 | 2h |

**交付要求**:
- SMB Auditing安全设计（docs/SMB_AUDITING_SECURITY.md）
- Rootless权限模型设计（docs/ROOTLESS_ADMIN_MODEL.md）

---

### 💰 户部（财务预算）- P1成本

| 任务 | 对标 | 状态 | 预估 |
|------|------|------|------|
| Presto成本效益分析 | 群晖DSM 7.3 | 📋 待开发 | 2h |
| RDMA硬件成本评估 | TrueNAS 25.10 | 📋 待开发 | 1h |
| TrueNAS vs nas-os成本对比 | - | 📋 待开发 | 2h |

**交付要求**:
- 成本分析报告（docs/COST_ANALYSIS_COMPETITIVE.md）

---

### 🎭 礼部（品牌营销）- P1体验

| 任务 | 对标 | 状态 | 预估 |
|------|------|------|------|
| Audio Station UX设计 | 群晖DSM 7.3 | 📋 待开发 | 3h |
| SMB功能文档更新 | TrueNAS 25.10 | 📋 待开发 | 1h |
| 竞品对比宣传文案 | - | 📋 待开发 | 2h |

**交付要求**:
- Audio Station设计文档（docs/AUDIO_STATION_DESIGN.md）
- SMB功能文档（docs/SMB_FEATURES.md）

---

### 📋 吏部（项目管理）

| 任务 | 状态 | 预估 |
|------|------|------|
| v2.465.0版本规划 | 📋 待开发 | 0.5h |
| MILESTONES.md更新 | 📋 待开发 | 0.5h |
| Round237任务跟踪 | 🔄 持续 | - |

**交付要求**:
- 版本号更新
- MILESTONES更新
- 任务进度记录

---

## 版本规划

**v2.465.0** (预计 2026-04-24)
- 竞品深度对标文档
- SMB Multichannel设计启动
- SMB Auditing设计启动

## 提交流程

1. 司礼监汇总六部成果
2. 创建提交并推送
3. 等待CI验证通过
4. 发布Release v2.465.0
5. 更新文档

---

**调度**: 司礼监
**状态**: 🔄 进行中