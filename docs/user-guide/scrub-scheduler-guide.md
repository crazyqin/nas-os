# 智能Scrub调度指南

> **版本**: v2.484.0 | **更新日期**: 2026-05-05 | **适用模块**: ZFS Scrub Scheduler

## 概述

智能Scrub调度系统可自动避开业务高峰时段执行存储池Scrub操作，确保数据校验不影响正常使用。系统支持IO负载检测、CPU阈值监控和自动暂停恢复。

## 为什么需要智能Scrub？

Scrub是ZFS/btrfs存储池的数据完整性校验操作，会消耗大量IO和CPU资源。传统Scrub存在以下问题：

- **性能影响**: Scrub期间读写性能下降30-50%
- **不可控时机**: 手动触发或固定时间可能撞上业务高峰
- **资源争抢**: Scrub与正常IO争抢磁盘带宽

智能Scrub调度通过**避峰 + 负载感知 + 自动恢复**解决以上问题。

---

## 快速开始

### 1. 查看当前Scrub状态

```bash
# API查询
curl http://localhost:8080/api/v1/scrub/status

# 响应示例
{
  "pools": [
    {
      "name": "tank",
      "last_scrub": "2026-05-01T03:00:00Z",
      "status": "completed",
      "errors": 0,
      "duration_hours": 4.2
    }
  ],
  "scheduler": {
    "enabled": true,
    "next_scheduled": "2026-05-08T01:00:00Z",
    "current_state": "idle"
  }
}
```

### 2. 配置避峰窗口

系统内置三个默认高峰窗口，也可自定义：

```bash
# 查看当前高峰窗口配置
curl http://localhost:8080/api/v1/scrub/peak-windows

# 响应示例
{
  "windows": [
    {
      "id": "morning-work",
      "name": "上午工作时段",
      "weekdays": [1, 2, 3, 4, 5],
      "start": "09:00",
      "end": "12:00",
      "description": "工作日上午高峰"
    },
    {
      "id": "afternoon-work",
      "name": "下午工作时段",
      "weekdays": [1, 2, 3, 4, 5],
      "start": "14:00",
      "end": "18:00",
      "description": "工作日下午高峰"
    },
    {
      "id": "evening-usage",
      "name": "晚间使用时段",
      "weekdays": [0, 1, 2, 3, 4, 5, 6],
      "start": "19:00",
      "end": "23:00",
      "description": "晚间娱乐高峰"
    }
  ]
}
```

### 3. 自定义高峰窗口

```bash
# 添加自定义高峰窗口
curl -X POST http://localhost:8080/api/v1/scrub/peak-windows \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "视频会议时段",
    "weekdays": [1, 3, 5],
    "start": "10:00",
    "end": "11:30",
    "description": "每周一三五上午视频会议"
  }'
```

---

## 核心功能

### 避峰调度

Scrub默认在凌晨1:00-6:00（安静时段）启动，自动避开所有高峰窗口：

| 时段 | 状态 | 说明 |
|------|------|------|
| 凌晨 01:00-06:00 | ✅ Scrub运行 | 默认安静时段，IO负载低 |
| 上午 09:00-12:00 | ⏸️ 自动暂停 | 工作高峰，Scrub暂停 |
| 下午 14:00-18:00 | ⏸️ 自动暂停 | 工作高峰，Scrub暂停 |
| 晚间 19:00-23:00 | ⏸️ 自动暂停 | 娱乐高峰，Scrub暂停 |
| 其他时段 | ✅ Scrub运行 | 非高峰时段继续 |

### IO负载检测

系统实时监控磁盘IO负载，超过阈值自动暂停Scrub：

```bash
# 配置IO负载阈值
curl -X PUT http://localhost:8080/api/v1/scrub/config \
  -H 'Content-Type: application/json' \
  -d '{
    "io_threshold_percent": 70,
    "cpu_threshold_percent": 80,
    "consecutive_delay_count": 3,
    "resume_delay_seconds": 300
  }'
```

**参数说明**:
| 参数 | 默认值 | 说明 |
|------|--------|------|
| `io_threshold_percent` | 70 | 磁盘IO使用率超过此值暂停Scrub |
| `cpu_threshold_percent` | 80 | CPU使用率超过此值暂停Scrub |
| `consecutive_delay_count` | 3 | 连续延迟次数达到此值暂停Scrub |
| `resume_delay_seconds` | 300 | 负载降低后等待此时间再恢复 |

### CPU阈值监控

CPU使用率超过阈值时，Scrub自动降低优先级或暂停：

```
[Scrub Scheduler] CPU 85% > 阈值 80%, Scrub暂停
[Scrub Scheduler] CPU 45% < 阈值 80%, 等待300s后恢复Scrub
```

### 连续延迟追踪

当Scrub多次因负载过高被延迟时，系统会累积延迟计数：

- 延迟计数 < 3: 继续尝试运行
- 延迟计数 ≥ 3: 暂停Scrub，等待低负载窗口
- 负载降低后: 重置延迟计数，恢复Scrub

---

## 调度策略

### 默认策略

```
每周日凌晨 01:00 → 自动启动Scrub
├── 检查是否在高峰窗口 → 是 → 等待高峰结束
├── 检查IO负载 → 超过70% → 暂停等待
├── 检查CPU负载 → 超过80% → 暂停等待
├── 连续延迟 ≥ 3次 → 暂停，等待安静时段
└── 以上均满足 → 执行Scrub
```

### 自定义调度

```bash
# 修改Scrub调度频率
curl -X PUT http://localhost:8080/api/v1/scrub/schedule \
  -H 'Content-Type: application/json' \
  -d '{
    "frequency": "biweekly",
    "preferred_time": "02:00",
    "preferred_weekday": 0,
    "max_duration_hours": 12,
    "skip_if_recent_days": 7
  }'
```

| 参数 | 说明 |
|------|------|
| `frequency` | 调度频率: daily / weekly / biweekly / monthly |
| `preferred_time` | 首选启动时间 (HH:MM) |
| `preferred_weekday` | 首选星期 (0=周日, 6=周六) |
| `max_duration_hours` | 最大运行时长，超时自动暂停 |
| `skip_if_recent_days` | 如果最近N天内已执行过则跳过 |

---

## 监控与告警

### 实时状态查询

```bash
# 查询当前Scrub进度
curl http://localhost:8080/api/v1/scrub/progress

# 响应示例
{
  "pool": "tank",
  "state": "running",
  "progress_percent": 45.2,
  "scanned_bytes": 1234567890,
  "total_bytes": 2730000000,
  "started_at": "2026-05-05T01:00:00Z",
  "paused_count": 2,
  "pause_duration_seconds": 3600,
  "estimated_remaining_hours": 2.1
}
```

### 告警配置

```bash
# 配置Scrub告警
curl -X POST http://localhost:8080/api/v1/alerts/rules \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Scrub错误告警",
    "condition": "scrub.errors > 0",
    "severity": "critical",
    "channels": ["email", "telegram"],
    "message": "存储池 {pool} Scrub发现 {errors} 个错误"
  }'
```

---

## 对标竞品

| 功能 | NAS-OS | TrueNAS 26 | 群晖DSM | 飞牛fnOS |
|------|:------:|:----------:|:-------:|:--------:|
| 定时Scrub | ✅ | ✅ | ✅ | ✅ |
| **避峰调度** | ✅ | ✅ | ❌ | ❌ |
| **IO负载检测** | ✅ | ⚠️ 基础 | ❌ | ❌ |
| **CPU阈值监控** | ✅ | ❌ | ❌ | ❌ |
| **自动暂停/恢复** | ✅ | ✅ | ❌ | ❌ |
| **连续延迟追踪** | ✅ | ❌ | ❌ | ❌ |
| 自定义调度策略 | ✅ | ⚠️ 有限 | ✅ | ❌ |

**NAS-OS优势**: 竞品中仅TrueNAS 26支持基础避峰，NAS-OS的IO+CPU双阈值检测+连续延迟追踪是独有功能。

---

## 常见问题

### Q: Scrub会影响正常使用吗？
A: 智能调度会自动避开高峰时段和高负载场景。如果IO/CPU使用率超过阈值，Scrub会自动暂停，几乎不影响正常使用。

### Q: 如何手动触发Scrub？
A: 使用API或Web界面手动触发，系统仍会遵循避峰策略：
```bash
curl -X POST http://localhost:8080/api/v1/scrub/trigger \
  -d '{"pool": "tank", "force": false}'
```

### Q: Scrub发现错误怎么办？
A: 系统会自动生成引导式告警（Guided Alert），附带排查步骤和修复建议。严重错误会通过配置的告警渠道通知。

### Q: 如何查看历史Scrub记录？
A: 
```bash
curl http://localhost:8080/api/v1/scrub/history?pool=tank&limit=10
```

---

## 相关文档

- [引导式告警系统指南](guided-alerts-guide.md)
- [存储健康评分指南](storage-health-guide.md)
- [NVMe健康监控指南](nvme-health-guide.md)
