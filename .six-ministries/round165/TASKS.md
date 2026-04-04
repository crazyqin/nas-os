# 第165轮六部协同开发任务

## 版本信息
**版本**: v2.393.0 → v2.396.0
**发布日期**: 2026-04-05
**状态**: 六部任务分派中

## 司礼监调度

### 优先修复
- ✅ 编译错误修复：`internal/auth/enhanced_mfa_manager.go`
  - 移除未使用的strings包
  - 添加maskUserID函数（解决undefined错误）

### 竞品调研（已从CHANGELOG汇总）

#### TrueNAS 26 Goldeye（最新）
| 功能 | 说明 | nas-os对标状态 |
|------|------|---------------|
| WebShare + TrueSearch | 浏览器文件访问+全文搜索 | ✅ 已实现 |
| Ransomware Defense | 勒索软件实时防御+honeypot | 🎯 原型开发 |
| SMB Spotlight Search | macOS Spotlight搜索SMB | 📋 规划中 |
| NVMe over Fabric | NVMe/TCP+RDMA支持 | 📋 规划中 |
| RAIDZ Expansion | OpenZFS 2.3单盘扩容 | 🚧 M106进行中 |
| LXC容器HA | 容器故障转移 | ✅ 已有容器管理 |

#### 群晖 DSM 7.3
| 功能 | nas-os状态 |
|------|------------|
| Photos AI | ✅ 已有照片管理+人脸识别 |
| Drive同步 | 📋 规划中 |
| Office协同 | ✅ OnlyOffice集成 |
| Hyper Backup | ✅ 备份模块 |
| VMM虚拟化 | 📋 规划中 |

#### 飞牛 fnOS
| 功能 | nas-os状态 |
|------|------------|
| 按需唤醒硬盘 | 🚧 本轮实现 |
| Intel核显加速AI | ✅ GPU调度已有 |
| FN Connect云管理 | 📋 规划中 |

---

## 本轮开发优先级

### P0 - 必须完成
1. **编译修复提交** ✅ 已完成
2. **磁盘智能电源管理** - 对标飞牛按需唤醒

### P1 - 功能增强
3. **NVMe健康预测完善** - 三级预警机制
4. **勒索防护增强** - 对标TrueNAS Ransomware Defense
5. **应用模板标准化** - Docker体验优化

### P2 - 文档完善
6. **竞品对比更新**
7. **用户指南完善**

---

## 六部任务分配

### 🪖 兵部（软件工程）
**任务**: NVMe健康预测 + 磁盘电源管理
- NVMe寿命预测算法完善
- 三级预警机制（正常/警告/危险）
- 磁盘智能电源管理（standby/spindown策略）
- 按需唤醒逻辑实现

**交付**: `internal/disk/nvme_health.go` + `internal/disk/power_mgmt.go`

### 🔧 工部（DevOps）
**任务**: CI/CD保障 + Docker优化
- 验证Actions修复后CI状态
- Docker部署流程优化
- 应用模板标准化（Compose Wizard）
- armv7构建问题排查

**交付**: CI验证报告 + docker-compose优化

### ⚖️ 刑部（安全合规）
**任务**: 安全审计Round165
- 持续漏洞扫描跟进
- govulncheck结果验证
- 安全报告更新

**交付**: `SECURITY_AUDIT_ROUND165.md`

### 💰 户部（财务运营）
**任务**: 成本分析优化
- 多节点成本聚合报告
- RAIDZ扩容成本计算器完善
- 云vs自建对比更新

**交付**: 成本报告

### 📜 礼部（品牌内容）
**任务**: 文档完善
- NVMe监控使用指南
- 磁盘电源管理说明
- CHANGELOG本轮更新

**交付**: docs更新 + CHANGELOG.md

### 📋 吏部（项目管理）
**任务**: 版本发布协调
- VERSION更新 v2.396.0
- ROADMAP进度同步
- Milestone M106推进

**交付**: VERSION + ROADMAP.md

---

## 时间要求

- 各部完成时间：本轮内
- 提交格式：git commit message标注部门
- 司礼监汇总提交：各部完成后统一提交GitHub