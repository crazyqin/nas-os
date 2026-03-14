# 配额自动扩展用户指南

> NAS-OS v2.27.0 新增功能

配额自动扩展功能可以根据配额使用情况自动调整存储限制，避免存储空间不足导致的业务中断。

## 目录

- [功能概述](#功能概述)
- [配置选项说明](#配置选项说明)
- [使用场景示例](#使用场景示例)
- [API 参考](#api-参考)
- [最佳实践](#最佳实践)

---

## 功能概述

配额自动扩展提供以下核心功能：

| 功能 | 说明 |
|------|------|
| 多种触发模式 | 阈值触发、定时检查、手动触发 |
| 灵活扩展策略 | 固定值、百分比、动态计算 |
| 审批流程 | 支持扩展前审批，防止意外扩展 |
| 通知集成 | 扩展前后通知，支持邮件和 Webhook |
| 回滚支持 | 可回滚已执行的扩展操作 |

---

## 配置选项说明

### 扩展策略配置

```json
{
  "id": "policy-001",
  "name": "用户配额自动扩展",
  "quota_id": "",                 // 关联的配额ID，为空表示全局策略
  "volume_name": "data",          // 适用卷名
  "enabled": true,                // 是否启用
  "trigger_mode": "threshold",    // 触发模式：threshold, scheduled, manual
  
  // 扩展配置
  "expand_mode": "percent",       // 扩展模式：fixed, percent, dynamic
  "expand_value": 10737418240,    // 扩展值（固定模式：字节）
  "expand_percent": 20,           // 扩展百分比（百分比模式）
  "max_limit": 1073741824000,     // 最大限制（0 表示无限制）
  "min_free_space": 10737418240,  // 最小保留空闲空间（动态模式）
  
  // 条件约束
  "cooldown_period": 3600000000000,  // 扩展冷却期（纳秒），1小时
  "max_expansions": 10,               // 最大扩展次数（0 表示无限）
  "daily_limit": 107374182400,        // 每日最大扩展量（字节），100GB
  
  // 通知配置
  "notify_before_expand": true,   // 扩展前通知
  "notify_email": "admin@example.com",
  "notify_webhook": "https://hooks.example.com/quota",
  
  // 审批配置
  "require_approval": true,       // 是否需要审批
  "approver_email": "manager@example.com"
}
```

### 触发模式说明

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `threshold` | 使用率达到阈值时触发 | 生产环境，自动化管理 |
| `scheduled` | 定时检查并扩展 | 预测性扩容场景 |
| `manual` | 仅手动触发 | 审核流程严格的环境 |

### 扩展模式说明

| 模式 | 说明 | 计算方式 |
|------|------|----------|
| `fixed` | 固定值扩展 | 新限制 = 当前限制 + expand_value |
| `percent` | 百分比扩展 | 新限制 = 当前限制 × (1 + expand_percent/100) |
| `dynamic` | 动态计算 | 新限制 = 当前限制 + (min_free_space - 当前可用空间) |

### 触发规则配置

```json
{
  "trigger_rules": [
    {
      "id": "rule-001",
      "type": "usage_percent",      // 触发类型
      "operator": "gte",            // 比较操作符：gt, gte, lt, lte, eq
      "value": 85,                  // 触发阈值
      "duration": 300000000000,     // 持续时间（纳秒），5分钟
      "severity": "warning",        // 严重级别
      "consecutive_hits": 3         // 连续命中次数
    }
  ]
}
```

### 触发类型说明

| 类型 | 说明 | 单位 |
|------|------|------|
| `usage_percent` | 使用率百分比 | 百分比 |
| `free_space` | 剩余空间百分比 | 百分比 |
| `free_bytes` | 剩余字节数 | 字节 |
| `growth_rate` | 增长率 | 百分比/天 |

---

## 使用场景示例

### 场景一：用户主目录自动扩展

**需求**: 用户主目录使用率达到 85% 时自动扩展 20%，最大不超过 500GB。

```bash
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/policies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "用户主目录自动扩展",
    "volume_name": "data",
    "enabled": true,
    "trigger_mode": "threshold",
    "trigger_rules": [
      {
        "type": "usage_percent",
        "operator": "gte",
        "value": 85,
        "consecutive_hits": 3
      }
    ],
    "expand_mode": "percent",
    "expand_percent": 20,
    "max_limit": 536870912000,
    "cooldown_period": 86400000000000,
    "notify_before_expand": true,
    "notify_email": "admin@example.com"
  }'
```

### 场景二：共享目录按需扩展

**需求**: 共享目录剩余空间低于 10GB 时自动扩展 50GB，需要审批。

```bash
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/policies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "共享目录自动扩展",
    "quota_id": "quota-shared-001",
    "enabled": true,
    "trigger_mode": "threshold",
    "trigger_rules": [
      {
        "type": "free_bytes",
        "operator": "lte",
        "value": 10737418240,
        "consecutive_hits": 2
      }
    ],
    "expand_mode": "fixed",
    "expand_value": 53687091200,
    "require_approval": true,
    "approver_email": "manager@example.com",
    "notify_before_expand": true
  }'
```

### 场景三：开发环境动态扩展

**需求**: 开发环境确保至少保留 20% 空闲空间，动态调整。

```bash
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/policies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "开发环境动态扩展",
    "volume_name": "dev",
    "enabled": true,
    "trigger_mode": "threshold",
    "trigger_rules": [
      {
        "type": "free_space",
        "operator": "lte",
        "value": 20,
        "consecutive_hits": 1
      }
    ],
    "expand_mode": "dynamic",
    "min_free_space": 0,
    "daily_limit": 107374182400,
    "cooldown_period": 3600000000000
  }'
```

### 场景四：多级预警扩展

**需求**: 根据不同使用率级别采取不同扩展策略。

```bash
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/policies \
  -H "Content-Type: application/json" \
  -d '{
    "name": "多级预警扩展",
    "volume_name": "data",
    "enabled": true,
    "trigger_mode": "threshold",
    "trigger_rules": [
      {
        "type": "usage_percent",
        "operator": "gte",
        "value": 95,
        "severity": "critical",
        "consecutive_hits": 1
      },
      {
        "type": "usage_percent",
        "operator": "gte",
        "value": 85,
        "severity": "warning",
        "consecutive_hits": 3
      }
    ],
    "expand_mode": "percent",
    "expand_percent": 30,
    "max_limit": 1073741824000,
    "notify_before_expand": true,
    "notify_webhook": "https://hooks.slack.com/services/xxx"
  }'
```

### 审批流程

当策略配置了 `require_approval: true` 时，扩展操作需要审批：

```bash
# 查看待审批的扩展操作
curl http://localhost:8080/api/v1/quota-auto-expand/actions?status=pending

# 审批通过
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/actions/action-001/approve \
  -H "Content-Type: application/json" \
  -d '{
    "approver": "manager"
  }'

# 拒绝扩展
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/actions/action-001/reject \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "预算限制，暂不扩展"
  }'
```

### 回滚扩展

```bash
# 回滚已执行的扩展操作
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/actions/action-001/rollback \
  -H "Content-Type: application/json" \
  -d '{
    "reason": "扩展有误，需要回滚"
  }'
```

### 手动扩展

```bash
# 手动扩展配额（不受策略限制）
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/manual-expand \
  -H "Content-Type: application/json" \
  -d '{
    "quota_id": "quota-001",
    "expand_bytes": 10737418240,
    "reason": "用户申请扩容"
  }'

# 手动缩减配额
curl -X POST http://localhost:8080/api/v1/quota-auto-expand/manual-shrink \
  -H "Content-Type: application/json" \
  -d '{
    "quota_id": "quota-001",
    "shrink_bytes": 5368709120,
    "reason": "释放未使用空间"
  }'
```

---

## API 参考

### 策略管理

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/quota-auto-expand/policies` | 创建扩展策略 |
| GET | `/api/v1/quota-auto-expand/policies` | 列出扩展策略 |
| GET | `/api/v1/quota-auto-expand/policies/:id` | 获取策略详情 |
| PUT | `/api/v1/quota-auto-expand/policies/:id` | 更新扩展策略 |
| DELETE | `/api/v1/quota-auto-expand/policies/:id` | 删除扩展策略 |

### 扩展操作

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/quota-auto-expand/actions` | 列出扩展操作 |
| GET | `/api/v1/quota-auto-expand/actions/:id` | 获取操作详情 |
| POST | `/api/v1/quota-auto-expand/actions/:id/approve` | 审批通过 |
| POST | `/api/v1/quota-auto-expand/actions/:id/reject` | 拒绝扩展 |
| POST | `/api/v1/quota-auto-expand/actions/:id/rollback` | 回滚扩展 |

### 手动操作

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/quota-auto-expand/manual-expand` | 手动扩展 |
| POST | `/api/v1/quota-auto-expand/manual-shrink` | 手动缩减 |

### 统计和预测

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/quota-auto-expand/stats/:policyId` | 获取策略统计 |
| GET | `/api/v1/quota-auto-expand/recommendations` | 获取扩展建议 |
| GET | `/api/v1/quota-auto-expand/history/:quotaId` | 获取扩展历史 |
| POST | `/api/v1/quota-auto-expand/simulate` | 模拟扩展效果 |

---

## 最佳实践

### 1. 合理设置触发阈值

- **警告级别**: 80-85% 使用率
- **严重级别**: 90-95% 使用率
- **紧急级别**: 95%+ 使用率

### 2. 设置冷却期

避免频繁扩展，建议：
- 生产环境: 24 小时以上
- 开发环境: 1-4 小时
- 测试环境: 可更短

### 3. 设置最大限制

防止无限扩展导致资源浪费：
- 根据实际预算设置上限
- 定期审查配额使用情况

### 4. 启用通知

及时了解扩展情况：
- 配置邮件通知给管理员
- 配置 Webhook 集成到监控系统
- 配置 Slack/Discord 等即时通知

### 5. 审批流程

对于重要配额：
- 启用审批流程
- 指定审批人
- 设置审批超时

### 6. 监控和审计

定期检查：
- 扩展历史记录
- 策略执行统计
- 异常扩展告警

---

## 常见问题

### Q: 扩展后可以回滚吗？

A: 可以。通过 API 调用 `rollback` 接口可以回滚已执行的扩展操作，配额限制会恢复到扩展前的值。

### Q: 如何避免频繁扩展？

A: 
1. 设置合理的 `cooldown_period`（冷却期）
2. 设置 `max_expansions` 限制最大扩展次数
3. 设置 `daily_limit` 限制每日扩展量

### Q: 扩展会影响正在使用的用户吗？

A: 不会。配额扩展是增加限制值，不会中断正在进行的操作。扩展后的空间立即可用。

### Q: 策略可以绑定特定配额吗？

A: 可以。设置 `quota_id` 绑定特定配额，或设置 `volume_name` 绑定到特定卷。

### Q: 如何查看扩展建议？

A: 调用 `/api/v1/quota-auto-expand/recommendations` 接口，系统会根据使用率和增长趋势给出扩展建议。