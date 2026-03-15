# 存储配额管理模块

## 概述

存储配额管理模块提供完整的配额管理功能，包括：

1. **用户/用户组存储空间限制** - 为用户或用户组设置存储配额
2. **配额使用监控和告警** - 实时监控配额使用情况，超过阈值自动告警
3. **自动清理策略** - 支持多种清理策略，自动清理过期或大文件
4. **配额报告生成** - 生成多种格式的配额使用报告
5. **配额预警增强** (v2.6.0) - 多级预警阈值配置、通知渠道管理
6. **自动清理增强** (v2.6.0) - 大文件检测、过期文件清理规则
7. **使用趋势分析** (v2.6.0) - 趋势数据 API、预测功能
8. **配额历史统计** (v2.7.0) - 历史数据存储、查询和统计分析
9. **配额使用图表** (v2.7.0) - 多种图表类型支持（折线图、柱状图、饼图、仪表盘、热力图）
10. **预警通知增强** (v2.7.0) - 多渠道通知（邮件、Webhook、Slack、Discord、Telegram）
11. **用户资源报告** (v2.7.0) - 用户级别资源使用报告
12. **系统资源报告** (v2.7.0) - 系统级别资源使用报告和趋势分析
13. **存储使用统计** (v2.7.0) - 存储使用统计 API 和趋势分析

## 性能优化 (v2.88.0)

### 使用量缓存
- 目录大小计算结果自动缓存 5 分钟
- 避免频繁调用 `du` 命令，提高查询性能
- 提供 `ClearUsageCache()` 和 `ClearUsageCacheForPath()` 方法手动清除缓存
- 在执行清理操作后建议清除相关路径的缓存

## API 接口

### 配额管理

#### 创建配额
```
POST /api/v1/quotas
{
  "type": "user",           // user 或 group
  "target_id": "username",  // 用户名或组名
  "volume_name": "data",    // 卷名（可选）
  "path": "/home/user",     // 限制路径（可选）
  "hard_limit": 107374182400,  // 硬限制（字节），100GB
  "soft_limit": 85899345920     // 软限制（字节），80GB
}
```

#### 获取配额列表
```
GET /api/v1/quotas?type=user&volume=data
```

#### 获取配额详情
```
GET /api/v1/quotas/:id
```

#### 更新配额
```
PUT /api/v1/quotas/:id
{
  "hard_limit": 214748364800,  // 200GB
  "soft_limit": 171798691840   // 160GB
}
```

#### 删除配额
```
DELETE /api/v1/quotas/:id
```

### 配额使用统计

#### 获取所有配额使用情况
```
GET /api/v1/quota-usage
```

#### 获取用户配额使用情况
```
GET /api/v1/quota-usage/users/:username
```

#### 检查配额（写入前验证）
```
POST /api/v1/quotas/check
{
  "username": "testuser",
  "volume_name": "data",
  "additional_size": 1048576  // 额外需要的空间（字节）
}
```

### 告警管理

#### 获取活跃告警
```
GET /api/v1/quota-alerts
```

#### 获取告警历史
```
GET /api/v1/quota-alerts/history?limit=100
```

#### 静默告警
```
POST /api/v1/quota-alerts/:id/silence
```

#### 解决告警
```
POST /api/v1/quota-alerts/:id/resolve
```

#### 获取/设置告警配置
```
GET /api/v1/quota-alerts/config
PUT /api/v1/quota-alerts/config
{
  "enabled": true,
  "soft_limit_threshold": 80,    // 软限制告警阈值（百分比）
  "hard_limit_threshold": 95,    // 硬限制告警阈值（百分比）
  "check_interval": 300000000000, // 检查间隔（纳秒），5分钟
  "notify_email": true,
  "notify_webhook": false,
  "webhook_url": ""
}
```

### 清理策略

#### 创建清理策略
```
POST /api/v1/cleanup-policies
{
  "name": "清理临时文件",
  "volume_name": "data",
  "path": "/mnt/data/temp",
  "type": "age",           // age, size, pattern, quota, access
  "action": "delete",      // delete, archive, move
  "enabled": true,
  "max_age": 30,           // 最大保留天数（age 类型）
  "min_size": 1048576,     // 最小文件大小（size 类型）
  "patterns": ["*.tmp"],   // 文件名模式（pattern 类型）
  "quota_percent": 90,     // 触发阈值（quota 类型）
  "max_access_age": 60,    // 最大未访问天数（access 类型）
  "archive_path": "",      // 归档目标路径（archive 动作）
  "move_path": "",         // 移动目标路径（move 动作）
  "schedule": "0 2 * * *"  // cron 表达式
}
```

#### 清理策略类型

| 类型 | 说明 | 必要参数 |
|------|------|----------|
| `age` | 按文件修改时间清理 | `max_age` |
| `size` | 按文件大小清理 | `min_size` |
| `pattern` | 按文件名模式清理 | `patterns` |
| `quota` | 按配额比例触发清理 | `quota_percent` |
| `access` | 按访问时间清理 | `max_access_age` |

#### 清理动作

| 动作 | 说明 | 必要参数 |
|------|------|----------|
| `delete` | 直接删除文件 | - |
| `archive` | 归档到指定目录 | `archive_path` |
| `move` | 移动到指定目录 | `move_path` |

#### 获取清理策略列表
```
GET /api/v1/cleanup-policies?volume=data
```

#### 执行清理策略
```
POST /api/v1/cleanup-policies/:id/run
```

#### 启用/禁用策略
```
POST /api/v1/cleanup-policies/:id/enable
POST /api/v1/cleanup-policies/:id/disable
```

### 清理任务

#### 获取清理任务列表
```
GET /api/v1/cleanup-tasks?limit=50
```

#### 获取清理任务详情
```
GET /api/v1/cleanup-tasks/:id
```

#### 运行自动清理
```
POST /api/v1/cleanup-tasks/auto
```

#### 获取清理统计
```
GET /api/v1/cleanup-tasks/stats
```

### 监控状态

#### 获取监控状态
```
GET /api/v1/quota-monitor/status
```

#### 启动/停止监控
```
POST /api/v1/quota-monitor/start
POST /api/v1/quota-monitor/stop
```

#### 获取趋势数据
```
GET /api/v1/quota-monitor/trends/:quotaId?duration=24h
```

### 报告生成

#### 生成报告
```
POST /api/v1/quota-reports
{
  "type": "summary",       // summary, user, group, volume, trend
  "format": "json",        // json, csv, html
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-31T23:59:59Z",
  "volume_name": "data",   // 可选
  "user_id": "username",   // 可选
  "group_id": "groupname"  // 可选
}
```

#### 获取报告列表
```
GET /api/v1/quota-reports
```

#### 导出报告
```
GET /api/v1/quota-reports/:id/export?format=csv
```

## 配置文件

配额配置保存在 `/etc/nas-os/quota.json`：

```json
{
  "quotas": [
    {
      "id": "abc123",
      "type": "user",
      "target_id": "testuser",
      "volume_name": "data",
      "hard_limit": 107374182400,
      "soft_limit": 85899345920
    }
  ],
  "policies": [
    {
      "id": "policy1",
      "name": "清理临时文件",
      "volume_name": "data",
      "type": "age",
      "action": "delete",
      "max_age": 30
    }
  ],
  "alert_config": {
    "enabled": true,
    "soft_limit_threshold": 80,
    "hard_limit_threshold": 95,
    "check_interval": 300000000000
  }
}
```

## 使用示例

### 1. 为用户设置 100GB 配额

```bash
curl -X POST http://localhost:8080/api/v1/quotas \
  -H "Content-Type: application/json" \
  -d '{
    "type": "user",
    "target_id": "zhangsan",
    "volume_name": "data",
    "hard_limit": 107374182400,
    "soft_limit": 85899345920
  }'
```

### 2. 查看用户配额使用情况

```bash
curl http://localhost:8080/api/v1/quota-usage/users/zhangsan
```

### 3. 创建自动清理策略

```bash
curl -X POST http://localhost:8080/api/v1/cleanup-policies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "清理30天前的临时文件",
    "volume_name": "data",
    "path": "/mnt/data/temp",
    "type": "age",
    "action": "delete",
    "max_age": 30,
    "enabled": true
  }'
```

### 4. 生成配额报告

```bash
curl -X POST http://localhost:8080/api/v1/quota-reports \
  -H "Content-Type: application/json" \
  -d '{
    "type": "summary",
    "format": "json"
  }'
```

## 架构说明

```
internal/quota/
├── types.go           # 数据类型定义
├── manager.go         # 配额管理器核心逻辑
├── monitor.go         # 监控和告警
├── cleanup.go         # 自动清理策略
├── report.go          # 报告生成
├── trend.go           # 趋势数据分析 (v2.6.0)
├── alert_enhanced.go  # 预警增强 (v2.6.0)
├── handlers.go        # HTTP API 处理器
├── handlers_enhanced.go # 增强功能处理器 (v2.6.0)
├── handlers_v2.go     # v2.7.0 新增处理器
├── history.go         # 历史数据管理 (v2.7.0)
└── adapter.go         # 接口适配器
```

### 核心组件

- **Manager**: 配额管理器，负责配额的 CRUD 操作和使用计算
- **Monitor**: 监控器，定期检查配额使用情况并触发告警
- **CleanupManager**: 清理管理器，执行自动清理策略
- **ReportGenerator**: 报告生成器，生成各种格式的配额报告
- **HistoryManager**: 历史数据管理器，采集和存储历史数据 (v2.7.0)
- **TrendDataManager**: 趋势数据管理器，趋势分析和预测 (v2.6.0)
- **ChartManager**: 图表数据管理器，生成多种图表数据 (v2.7.0)
- **NotificationManager**: 通知管理器，多渠道通知发送 (v2.7.0)
- **StorageStatsManager**: 存储统计管理器，存储使用统计 (v2.7.0)

### 数据流

```
用户请求 → Handlers → Manager → 计算使用量 → 返回结果
                  ↓
              Monitor（定期检查）→ 触发告警 → 通知
                  ↓
              CleanupManager（自动清理）→ 释放空间
                  ↓
              HistoryManager（历史采集）→ 统计分析 → 图表数据
```

---

## v2.7.0 新增 API

### 配额历史统计

#### 查询历史数据
```
GET /api/v1/quota-history?quota_id=xxx&start_time=2024-01-01T00:00:00Z&end_time=2024-01-31T23:59:59Z&limit=100
```

#### 高级查询
```
POST /api/v1/quota-history/query
{
  "quota_id": "xxx",          // 可选，指定配额ID
  "volume_name": "data",      // 可选，按卷过滤
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-31T23:59:59Z",
  "group_by": "day",          // hour, day, week, month
  "limit": 100
}
```

#### 获取历史统计
```
GET /api/v1/quota-history/statistics/:quotaId?duration=168h  // 7天
```

#### 获取所有配额历史统计
```
GET /api/v1/quota-history/statistics?duration=168h
```

### 配额使用图表

#### 获取图表数据（通用）
```
POST /api/v1/quota-chart/data
{
  "quota_id": "xxx",          // 可选
  "volume_name": "data",      // 可选
  "chart_type": "line",       // line, bar, pie, gauge, heatmap
  "start_time": "2024-01-01T00:00:00Z",
  "end_time": "2024-01-31T23:59:59Z",
  "granularity": "day",       // hour, day, week, month
  "compare_with": "previous_period"  // previous_period, same_last_year
}
```

#### 折线图
```
GET /api/v1/quota-chart/line/:quotaId?duration=24h
```

#### 柱状图
```
GET /api/v1/quota-chart/bar?volume=data
```

#### 饼图
```
GET /api/v1/quota-chart/pie?volume=data
```

#### 仪表盘
```
GET /api/v1/quota-chart/gauge/:quotaId
```

#### 热力图
```
GET /api/v1/quota-chart/heatmap/:quotaId?duration=168h
```

### 预警通知

#### 获取通知渠道
```
GET /api/v1/quota-notification/channels
```

#### 添加通知渠道
```
POST /api/v1/quota-notification/channels
{
  "name": "运维邮件",
  "type": "email",            // email, webhook, slack, discord, telegram, wechat, dingtalk
  "enabled": true,
  "config": {
    "to": "admin@example.com"
  },
  "severity": ["warning", "critical", "emergency"]
}
```

#### 更新通知渠道
```
PUT /api/v1/quota-notification/channels/:id
```

#### 删除通知渠道
```
DELETE /api/v1/quota-notification/channels/:id
```

#### 发送通知
```
POST /api/v1/quota-notification/send
{
  "alert_id": "alert-xxx",
  "channel_id": "channel-xxx"
}
```

#### 获取通知历史
```
GET /api/v1/quota-notification/history?limit=100
```

#### 测试通知渠道
```
POST /api/v1/quota-notification/test/:channelId
```

### 用户资源报告

#### 获取用户资源报告
```
GET /api/v1/quota-user-report/:username?duration=168h
```

#### 导出用户报告
```
GET /api/v1/quota-user-report/:username/export?format=json&duration=168h
```

响应示例：
```json
{
  "username": "zhangsan",
  "generated_at": "2024-01-15T10:00:00Z",
  "period": {
    "start_time": "2024-01-08T10:00:00Z",
    "end_time": "2024-01-15T10:00:00Z"
  },
  "quotas": [
    {
      "quota_id": "quota-xxx",
      "volume_name": "data",
      "hard_limit": 107374182400,
      "used_bytes": 85899345920,
      "usage_percent": 80,
      "status": "warning"
    }
  ],
  "summary": {
    "total_quotas": 1,
    "total_used_bytes": 85899345920,
    "avg_usage_percent": 80,
    "warning_count": 1
  },
  "recommendations": [
    "部分配额使用率较高，建议检查并清理不需要的文件"
  ]
}
```

### 系统资源报告

#### 获取系统资源报告
```
GET /api/v1/quota-system-report?duration=168h
```

#### 导出系统报告
```
GET /api/v1/quota-system-report/export?format=json
```

#### 获取系统摘要
```
GET /api/v1/quota-system-report/summary
```

响应示例：
```json
{
  "total_quotas": 10,
  "total_users": 5,
  "total_groups": 2,
  "total_used_bytes": 536870912000,
  "total_limit_bytes": 1073741824000,
  "avg_usage_percent": 50,
  "over_soft_count": 2,
  "over_hard_count": 0
}
```

### 存储使用统计

#### 获取全局存储统计
```
GET /api/v1/quota-storage-stats
```

#### 获取所有卷统计
```
GET /api/v1/quota-storage-stats/volumes
```

#### 获取指定卷统计
```
GET /api/v1/quota-storage-stats/volumes/:volumeName
```

#### 获取使用量最高的用户
```
GET /api/v1/quota-storage-stats/top-users?volume=data&limit=10
```

#### 获取存储趋势
```
GET /api/v1/quota-storage-stats/trend
```

响应示例：
```json
{
  "volume_name": "data",
  "total_bytes": 1073741824000,
  "used_bytes": 536870912000,
  "free_bytes": 536870912000,
  "usage_percent": 50,
  "quota_count": 10,
  "top_users": [
    {
      "target_name": "zhangsan",
      "used_bytes": 107374182400,
      "usage_percent": 80
    }
  ],
  "trend": {
    "daily_growth_bytes": 1073741824,
    "growth_percent": 0.2,
    "days_to_full": 500
  }
}
```