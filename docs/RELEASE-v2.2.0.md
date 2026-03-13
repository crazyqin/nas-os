# NAS-OS v2.2.0 发布说明

**发布日期**: 2026-03-21  
**版本类型**: Stable  

---

## 🎉 版本亮点

v2.2.0 版本带来了企业级存储功能增强，包括 iSCSI 目标服务（Beta）、自动化快照策略系统、全新的 WebUI 仪表板和增强的性能监控能力。

---

## ✨ 新增功能

### 🎯 iSCSI 目标服务 (Beta)

iSCSI (Internet Small Computer System Interface) 提供基于网络的块级存储访问，适合虚拟化环境和高性能存储需求。

**核心功能**:
- iSCSI Target 配置和管理
- LUN (Logical Unit Number) 创建和管理
- CHAP 认证支持
- IQN 自动生成和管理
- Initiator 访问控制
- 连接会话监控

**使用场景**:
- VMware ESXi 数据存储
- Hyper-V 虚拟机存储
- 数据库块存储
- 高性能应用存储

**快速开始**:
```bash
# 创建 iSCSI 目标
nasctl iscsi create --name target1 --lun 0 --path /data/iscsi/lun0.img --size 100G

# 列出所有目标
nasctl iscsi list
```

> ⚠️ **注意**: iSCSI 功能当前为 Beta 版本，建议在生产环境中谨慎使用。

---

### 📸 快照策略系统

自动化的快照管理策略，支持基于时间和空间的智能快照调度。

**核心功能**:
- 定时快照策略（Cron 表达式）
- 快照保留策略（按数量/时间/空间）
- 快照自动清理
- 多策略并行执行
- 策略状态监控和告警

**预设模板**:
| 模板 | 调度 | 保留策略 |
|------|------|----------|
| hourly | 每小时 | 保留 24 个 |
| daily | 每天 02:00 | 保留 30 个 |
| weekly | 每周一 03:00 | 保留 12 个 |
| monthly | 每月 1 日 | 保留 12 个 |

**快速开始**:
```bash
# 创建每日快照策略
nasctl snapshot-policy create \
  --name daily-backup \
  --volume data \
  --schedule "0 2 * * *" \
  --keep-count 30
```

---

### 🖥️ WebUI 仪表板增强

全新的仪表板界面，提供直观的系统状态展示和快速操作入口。

**新增功能**:
- 实时系统资源监控（CPU/内存/磁盘/网络）
- 存储健康状态一览
- 可自定义的小部件布局
- 快速操作面板
- 活动告警展示
- 最近任务历史
- PWA 支持（可安装到主屏幕）

**仪表板小部件**:
| 小部件 | 功能 | 刷新频率 |
|--------|------|----------|
| 系统概览 | CPU/内存/磁盘使用率 | 1s |
| 网络流量 | 实时上传/下载速度 | 1s |
| 存储健康 | 磁盘 SMART 状态 | 60s |
| 活动告警 | 未处理的告警列表 | 10s |
| 快速操作 | 常用功能快捷入口 | - |

---

### 📊 性能监控配置增强

更强大的性能监控和优化建议功能。

**新增功能**:
- 可配置的性能阈值
- 自动性能基线学习
- 异常检测和告警
- 性能优化建议生成
- API 性能追踪
- 健康分数计算

**健康分数**:
- 90-100：优秀 🟢
- 70-89：良好 🟡
- 50-69：一般 🟠
- 0-49：差 🔴

---

## 🔧 改进优化

### WebDAV 模块
- 测试覆盖率提升至 85%
- 修复文件锁在特定场景下的死锁问题
- 优化大文件上传性能
- 新增 handlers_test.go

### 配额管理模块
- 测试覆盖率提升至 82%
- 新增目录级配额支持
- 新增 handlers_test.go
- 增加配额预警通知

### 集成测试
- 新增 v2.2.0 集成测试套件
- 新增并发安全测试
- 改进测试固件

---

## 🐛 Bug 修复

- 修复 WebDAV 在高并发下的锁竞争问题
- 修复配额统计在目录移动后的计算错误
- 修复性能监控在长时间运行后的内存泄漏
- 修复快照恢复时的时间戳错误
- 改进错误消息的可读性

---

## 📦 API 变更

### 新增端点

**iSCSI 模块**:
```
GET    /api/v1/iscsi/targets
POST   /api/v1/iscsi/targets
GET    /api/v1/iscsi/targets/:id
PUT    /api/v1/iscsi/targets/:id
DELETE /api/v1/iscsi/targets/:id
GET    /api/v1/iscsi/targets/:id/sessions
```

**快照策略模块**:
```
GET    /api/v1/snapshot-policies
POST   /api/v1/snapshot-policies
GET    /api/v1/snapshot-policies/:id
PUT    /api/v1/snapshot-policies/:id
DELETE /api/v1/snapshot-policies/:id
POST   /api/v1/snapshot-policies/:id/enable
POST   /api/v1/snapshot-policies/:id/disable
POST   /api/v1/snapshot-policies/:id/run
```

---

## 📚 文档更新

| 文档 | 说明 |
|------|------|
| CHANGELOG-v2.2.0.md | v2.2.0 变更日志 |
| ISCSI_GUIDE.md | iSCSI 目标使用指南 |
| SNAPSHOT_POLICY_GUIDE.md | 快照策略配置指南 |
| WEBUI_DASHBOARD_GUIDE.md | WebUI 仪表板使用说明 |
| PERFORMANCE_MONITORING_GUIDE.md | 性能监控配置指南 |

---

## ⚠️ 升级说明

### 从 v2.1.0 升级

```bash
# 停止服务
systemctl stop nas-os

# 备份配置
cp -r /etc/nas-os /etc/nas-os.bak

# 升级
nasd upgrade --from v2.1.0

# 启动服务
systemctl start nas-os
```

### 配置变更

新增配置节：
```yaml
# iSCSI 配置
iscsi:
  enabled: false
  port: 3260
  default_target_iqn: "iqn.2026-03.com.nas-os"

# 快照策略配置
snapshot_policies:
  enabled: true
  default_retention_days: 30

# 性能监控配置
perf:
  slow_threshold: 500
  enable_baseline: true
  baseline_interval: 5m
```

### 依赖要求

- Linux 内核: 5.10+（推荐）
- Go: 1.26.1+
- iSCSI 功能需要 `target_core_user` 内核模块

---

## 🔜 下一步计划

### v2.3.0 (计划中)

- LDAP/AD 集成
- 高级权限管理增强
- 审计日志完善
- 多语言支持扩展

---

## 🙏 致谢

感谢所有参与 v2.2.0 开发和测试的贡献者！

---

**发布团队**: NAS-OS 吏部  
**发布日期**: 2026-03-21