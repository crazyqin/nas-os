# NAS-OS v2.2.0 变更日志

**发布日期**: 2026-03-21  
**版本类型**: 功能增强版本

---

## 🎉 版本亮点

v2.2.0 版本带来了企业级存储功能增强、全新的 WebUI 仪表板和更强大的性能监控能力。

---

## ✨ 新增功能

### 📦 iSCSI 目标服务 (Beta)

iSCSI (Internet Small Computer System Interface) 提供基于网络的块级存储访问，适合虚拟化环境和高性能存储需求。

**核心功能**:
- iSCSI Target 配置和管理
- LUN (Logical Unit Number) 创建和管理
- CHAP 认证支持
- IQN (iSCSI Qualified Name) 自动生成
- Initiator 访问控制
- 连接会话监控

**API 端点**:
```
POST   /api/v1/iscsi/targets          # 创建 iSCSI 目标
GET    /api/v1/iscsi/targets          # 列出所有目标
GET    /api/v1/iscsi/targets/:id      # 获取目标详情
PUT    /api/v1/iscsi/targets/:id      # 更新目标配置
DELETE /api/v1/iscsi/targets/:id      # 删除目标
POST   /api/v1/iscsi/targets/:id/luns # 创建 LUN
GET    /api/v1/iscsi/targets/:id/sessions # 查看连接会话
```

**CLI 命令**:
```bash
# 创建 iSCSI 目标
nasctl iscsi create --name target1 --lun 0 --path /data/iscsi/lun0.img --size 100G

# 列出所有目标
nasctl iscsi list

# 查看目标详情
nasctl iscsi show target1

# 删除目标
nasctl iscsi delete target1
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

**策略类型**:
| 类型 | 说明 | 配置示例 |
|------|------|----------|
| 时间策略 | 按固定间隔创建快照 | `every 4h` 或 cron 表达式 |
| 保留数量 | 保留最近 N 个快照 | `keep: 10` |
| 保留时长 | 保留指定时间内的快照 | `retention: 30d` |
| 空间限制 | 快照总大小不超过限制 | `max_size: 100GB` |

**API 端点**:
```
POST   /api/v1/snapshot-policies          # 创建策略
GET    /api/v1/snapshot-policies          # 列出所有策略
GET    /api/v1/snapshot-policies/:id      # 获取策略详情
PUT    /api/v1/snapshot-policies/:id      # 更新策略
DELETE /api/v1/snapshot-policies/:id      # 删除策略
POST   /api/v1/snapshot-policies/:id/enable  # 启用策略
POST   /api/v1/snapshot-policies/:id/disable # 禁用策略
POST   /api/v1/snapshot-policies/:id/run     # 手动执行
```

**配置示例**:
```yaml
# 每日快照策略
name: daily-snapshot
volume: data
schedule: "0 2 * * *"  # 每天凌晨 2 点
retention:
  count: 30            # 保留最近 30 个
  max_age: 30d         # 最长保留 30 天
  max_size: 500GB      # 总大小不超过 500GB
enabled: true
```

---

### 🖥️ WebUI 仪表板增强

全新的仪表板界面，提供直观的系统状态展示和快速操作入口。

**新增功能**:
- 实时系统资源监控（CPU/内存/磁盘/网络）
- 存储健康状态一览
- 快速操作面板
- 活动告警展示
- 最近任务列表
- 自定义小部件布局

**仪表板小部件**:
| 小部件 | 功能 | 刷新频率 |
|--------|------|----------|
| 系统概览 | CPU/内存/磁盘使用率 | 1s |
| 网络流量 | 实时上传/下载速度 | 1s |
| 存储健康 | 磁盘 SMART 状态 | 60s |
| 活动告警 | 未处理的告警列表 | 10s |
| 快速操作 | 常用功能快捷入口 | - |
| 最近任务 | 最近的任务记录 | 30s |
| 系统日志 | 最新系统日志 | 5s |

**访问路径**: WebUI 首页即为仪表板，或访问 `/pages/dashboard.html`

---

### 📊 性能监控配置增强

更强大的性能监控和优化建议功能。

**新增功能**:
- 可配置的性能阈值
- 自动性能基线学习
- 异常检测和告警
- 性能优化建议
- 历史性能报告
- API 性能追踪

**监控指标**:
| 指标类别 | 具体指标 | 默认阈值 |
|----------|----------|----------|
| API 性能 | 平均响应时间 | < 100ms |
| API 性能 | P95 响应时间 | < 500ms |
| API 性能 | 错误率 | < 1% |
| 系统性能 | CPU 使用率 | < 80% |
| 系统性能 | 内存使用率 | < 85% |
| 系统性能 | 磁盘 I/O 延迟 | < 50ms |

**API 端点**:
```
GET    /api/v1/perf/metrics              # 性能指标概览
GET    /api/v1/perf/metrics/endpoints    # 端点性能详情
GET    /api/v1/perf/slow-logs            # 慢请求日志
GET    /api/v1/perf/analyze/health       # 健康分数
GET    /api/v1/perf/analyze/recommendations # 优化建议
PUT    /api/v1/perf/config/threshold     # 更新阈值配置
```

---

## 🔧 改进优化

### WebDAV 模块
- 测试覆盖率提升至 85%
- 修复文件锁在特定场景下的死锁问题
- 优化大文件上传性能
- 改进错误处理和日志记录

### 配额管理模块
- 测试覆盖率提升至 82%
- 新增目录级配额支持
- 优化配额统计计算性能
- 增加配额预警通知

### 集成测试
- 新增 iSCSI 集成测试套件
- 新增快照策略集成测试
- 改进测试固件和 Mock 实现
- 优化测试执行速度

---

## 📦 API 变更

### 新增端点

**iSCSI 模块**:
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/iscsi/targets` | GET, POST | 目标管理 |
| `/api/v1/iscsi/targets/:id` | GET, PUT, DELETE | 单个目标操作 |
| `/api/v1/iscsi/targets/:id/luns` | GET, POST | LUN 管理 |
| `/api/v1/iscsi/targets/:id/sessions` | GET | 会话监控 |

**快照策略模块**:
| 端点 | 方法 | 说明 |
|------|------|------|
| `/api/v1/snapshot-policies` | GET, POST | 策略管理 |
| `/api/v1/snapshot-policies/:id` | GET, PUT, DELETE | 单个策略操作 |
| `/api/v1/snapshot-policies/:id/run` | POST | 手动执行 |

---

## 🐛 Bug 修复

- 修复 WebDAV 在高并发下的锁竞争问题
- 修复配额统计在目录移动后的计算错误
- 修复性能监控在长时间运行后的内存泄漏
- 修复快照恢复时的时间戳错误
- 改进错误消息的可读性

---

## 📚 文档更新

- 新增 `docs/ISCSI_GUIDE.md` - iSCSI 目标使用指南
- 新增 `docs/SNAPSHOT_POLICY_GUIDE.md` - 快照策略配置指南
- 新增 `docs/WEBUI_DASHBOARD_GUIDE.md` - WebUI 仪表板使用说明
- 新增 `docs/PERFORMANCE_MONITORING_GUIDE.md` - 性能监控配置指南
- 更新 `docs/API_GUIDE.md` - 添加新端点文档

---

## ⚠️ 升级说明

### 配置变更
- 新增 `iscsi` 配置节
- 新增 `snapshot_policies` 配置节
- `perf` 配置节新增 `baseline_interval` 选项

### 依赖要求
- iSCSI 功能需要 Linux 内核支持 `target_core_user` 模块
- 推荐内核版本: 5.10+

---

## 🔄 升级路径

从 v2.1.0 升级:
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

---

## 📅 下一步计划

### v2.3.0 (计划中)
- LDAP/AD 集成
- 高级权限管理增强
- 审计日志完善
- 多语言支持扩展

---

**发布团队**: NAS-OS 吏部  
**文档编写**: 2026-03-21