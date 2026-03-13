# 快照策略配置指南

**版本**: v2.2.0  
**更新日期**: 2026-03-21

---

## 📖 概述

快照策略是 NAS-OS 提供的自动化快照管理功能，支持按时间计划自动创建快照，并根据保留策略自动清理旧快照，确保数据安全的同时避免存储空间无限增长。

### 特性

- ✅ 灵活的调度配置（Cron 表达式）
- ✅ 多种保留策略组合
- ✅ 自动快照清理
- ✅ 策略状态监控
- ✅ 策略执行历史记录
- ✅ 告警通知集成

---

## 🚀 快速开始

### 创建第一个快照策略

#### 方式一：WebUI

1. 登录 NAS-OS Web 管理界面
2. 导航到 **存储** → **快照策略**
3. 点击 **创建策略**
4. 配置策略参数：
   - 策略名称：`daily-backup`
   - 目标卷：`data`
   - 调度时间：`每天 02:00`
   - 保留数量：`30`
5. 点击 **创建**

#### 方式二：CLI

```bash
# 创建每日快照策略
nasctl snapshot-policy create \
  --name daily-backup \
  --volume data \
  --schedule "0 2 * * *" \
  --keep-count 30
```

#### 方式三：API

```bash
curl -X POST http://localhost:8080/api/v1/snapshot-policies \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "daily-backup",
    "volume": "data",
    "schedule": "0 2 * * *",
    "retention": {
      "count": 30
    },
    "enabled": true
  }'
```

---

## 📅 调度配置

### Cron 表达式

使用标准 5 位 Cron 表达式定义快照时间：

```
┌───────────── 分钟 (0 - 59)
│ ┌───────────── 小时 (0 - 23)
│ │ ┌───────────── 日期 (1 - 31)
│ │ │ ┌───────────── 月份 (1 - 12)
│ │ │ │ ┌───────────── 星期几 (0 - 6，0 = 周日)
│ │ │ │ │
* * * * *
```

### 常用调度示例

| 描述 | Cron 表达式 |
|------|-------------|
| 每小时整点 | `0 * * * *` |
| 每 4 小时 | `0 */4 * * *` |
| 每天凌晨 2 点 | `0 2 * * *` |
| 每天中午 12 点 | `0 12 * * *` |
| 每周一凌晨 3 点 | `0 3 * * 1` |
| 每月 1 日凌晨 0 点 | `0 0 1 * *` |
| 工作日上午 6 点 | `0 6 * * 1-5` |

### 预设模板

NAS-OS 提供常用策略模板：

| 模板名称 | 调度 | 保留策略 |
|----------|------|----------|
| hourly | 每小时 | 保留 24 个 |
| daily | 每天 02:00 | 保留 30 个 |
| weekly | 每周一 03:00 | 保留 12 个 |
| monthly | 每月 1 日 00:00 | 保留 12 个 |

```bash
# 使用预设模板创建策略
nasctl snapshot-policy create --template daily --volume data
```

---

## 🗂️ 保留策略

### 策略类型

| 类型 | 参数 | 说明 |
|------|------|------|
| 数量限制 | `count` | 保留最近 N 个快照 |
| 时间限制 | `max_age` | 保留指定时间内的快照 |
| 空间限制 | `max_size` | 快照总大小不超过限制 |

### 组合使用

保留策略可以组合使用，任一条件达到即触发清理：

```json
{
  "retention": {
    "count": 30,
    "max_age": "30d",
    "max_size": "500GB"
  }
}
```

上述配置表示：
- 最多保留 30 个快照
- 最长保留 30 天
- 总大小不超过 500GB

### 保留优先级

当多个限制条件同时生效时：
1. 先删除最旧的快照
2. 按创建时间排序清理
3. 确保不删除正在使用的快照

---

## 📋 策略管理

### 查看策略列表

```bash
# CLI
nasctl snapshot-policy list

# API
curl http://localhost:8080/api/v1/snapshot-policies \
  -H "Authorization: Bearer $TOKEN"
```

### 查看策略详情

```bash
nasctl snapshot-policy show daily-backup
```

响应示例：
```json
{
  "code": 0,
  "data": {
    "id": "policy-001",
    "name": "daily-backup",
    "volume": "data",
    "schedule": "0 2 * * *",
    "retention": {
      "count": 30,
      "max_age": "30d"
    },
    "enabled": true,
    "status": "active",
    "last_run": "2026-03-21T02:00:00Z",
    "next_run": "2026-03-22T02:00:00Z",
    "snapshot_count": 15,
    "total_size": "250GB"
  }
}
```

### 启用/禁用策略

```bash
# 禁用策略
nasctl snapshot-policy disable daily-backup

# 启用策略
nasctl snapshot-policy enable daily-backup
```

### 手动执行策略

```bash
# 立即执行一次快照
nasctl snapshot-policy run daily-backup
```

### 更新策略

```bash
curl -X PUT http://localhost:8080/api/v1/snapshot-policies/daily-backup \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "schedule": "0 3 * * *",
    "retention": {
      "count": 60
    }
  }'
```

### 删除策略

```bash
nasctl snapshot-policy delete daily-backup
```

> ⚠️ **注意**: 删除策略不会删除已创建的快照。

---

## 📊 监控与告警

### 策略状态监控

```bash
# 查看所有策略状态
nasctl snapshot-policy status

# 查看执行历史
nasctl snapshot-policy history daily-backup
```

### 告警配置

快照策略支持以下告警事件：

| 事件 | 说明 | 级别 |
|------|------|------|
| `snapshot_failed` | 快照创建失败 | Error |
| `cleanup_failed` | 快照清理失败 | Warning |
| `storage_exceeded` | 超出空间限制 | Warning |
| `policy_disabled` | 策略被禁用 | Info |

告警通知渠道在系统设置中配置。

---

## 🔄 典型应用场景

### 场景一：生产数据保护

```yaml
# 每小时快照，保留 24 小时
name: production-hourly
volume: production
schedule: "0 * * * *"
retention:
  count: 24
  max_age: "24h"
enabled: true
```

### 场景二：长期数据归档

```yaml
# 每日快照，保留 90 天
name: archive-daily
volume: archive
schedule: "0 3 * * *"
retention:
  count: 90
  max_age: "90d"
enabled: true
```

### 场景三：虚拟机备份

```yaml
# 每 4 小时快照，保留 1 周
name: vm-backup
volume: vm-storage
schedule: "0 */4 * * *"
retention:
  count: 42
  max_age: "7d"
enabled: true
```

### 场景四：高频率保护

```yaml
# 每 15 分钟快照，保留 4 小时
name: high-frequency
volume: critical-data
schedule: "*/15 * * * *"
retention:
  count: 16
  max_age: "4h"
enabled: true
```

---

## ⚠️ 最佳实践

1. **合理设置保留策略**
   - 根据数据变化频率和恢复需求设置
   - 避免保留过多快照占用存储空间

2. **错峰调度**
   - 多个策略避免同时执行
   - 建议间隔至少 30 分钟

3. **监控存储空间**
   - 设置空间告警阈值
   - 定期检查快照总大小

4. **测试恢复流程**
   - 定期测试快照恢复
   - 验证数据完整性

5. **文档记录**
   - 记录策略创建原因
   - 保留恢复操作指南

---

## 🔧 故障排查

### 快照创建失败

```bash
# 检查卷状态
nasctl volume show data

# 检查存储空间
df -h /data

# 查看策略日志
nasctl snapshot-policy logs daily-backup
```

### 策略未执行

1. 检查策略是否启用
2. 检查 Cron 表达式是否正确
3. 查看系统服务状态
4. 检查执行日志

### 快照清理异常

1. 检查保留策略配置
2. 确认无活跃的快照使用
3. 查看清理日志

---

## 📚 相关文档

- [iSCSI 目标使用指南](ISCSI_GUIDE.md)
- [API 文档 - 快照策略模块](API_GUIDE.md#snapshot-policy)
- [性能监控配置指南](PERFORMANCE_MONITORING_GUIDE.md)

---

**最后更新**: 2026-03-21  
**维护团队**: NAS-OS 吏部