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
├── types.go      # 数据类型定义
├── manager.go    # 配额管理器核心逻辑
├── monitor.go    # 监控和告警
├── cleanup.go    # 自动清理策略
├── report.go     # 报告生成
├── handlers.go   # HTTP API 处理器
└── adapter.go    # 接口适配器
```

### 核心组件

- **Manager**: 配额管理器，负责配额的 CRUD 操作和使用计算
- **Monitor**: 监控器，定期检查配额使用情况并触发告警
- **CleanupManager**: 清理管理器，执行自动清理策略
- **ReportGenerator**: 报告生成器，生成各种格式的配额报告

### 数据流

```
用户请求 → Handlers → Manager → 计算使用量 → 返回结果
                  ↓
              Monitor（定期检查）→ 触发告警 → 通知
                  ↓
              CleanupManager（自动清理）→ 释放空间
```