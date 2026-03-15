# 计费 API 文档

**版本**: v2.78.0  
**更新日期**: 2026-03-16

## 概述

计费 API 提供资源使用统计、账单管理、发票生成等功能。

## 基础路径

```
/api/v1/billing
```

---

## 计费配置

### 获取计费配置

```http
GET /api/v1/billing/config
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "enabled": true,
    "default_currency": "CNY",
    "billing_cycle": "monthly",
    "billing_day_of_month": 1,
    "storage_pricing": {
      "base_price_per_gb": 0.1,
      "ssd_price_per_gb": 0.15,
      "hdd_price_per_gb": 0.05,
      "archive_price_per_gb": 0.02,
      "free_storage_gb": 100
    },
    "bandwidth_pricing": {
      "price_per_gb": 0.5,
      "free_bandwidth_gb": 1000
    },
    "invoice_prefix": "INV",
    "invoice_due_days": 30,
    "tax_rate": 0.13,
    "tax_included": true
  }
}
```

### 更新计费配置

```http
PUT /api/v1/billing/config
```

**请求体**

```json
{
  "enabled": true,
  "billing_cycle": "monthly",
  "billing_day_of_month": 1,
  "storage_pricing": {
    "base_price_per_gb": 0.12
  },
  "tax_rate": 0.13
}
```

---

## 用量统计

### 获取当前用量

```http
GET /api/v1/billing/usage
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| user_id | string | 否 | 用户ID，管理员可查看所有用户 |
| period | string | 否 | 周期: current/last，默认 current |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "period_start": "2026-03-01T00:00:00Z",
    "period_end": "2026-03-31T23:59:59Z",
    "storage": {
      "total_gb": 500.5,
      "ssd_gb": 200.2,
      "hdd_gb": 300.3,
      "archive_gb": 0,
      "free_quota_gb": 100,
      "billable_gb": 400.5
    },
    "bandwidth": {
      "ingress_gb": 150.2,
      "egress_gb": 89.6,
      "total_gb": 239.8,
      "free_quota_gb": 1000,
      "billable_gb": 0
    },
    "services": [
      {
        "name": "storage_pool_main",
        "type": "storage",
        "usage_gb": 350.2,
        "price_per_gb": 0.1
      }
    ]
  }
}
```

### 获取用量历史

```http
GET /api/v1/billing/usage/history
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| start_date | string | 否 | 开始日期 |
| end_date | string | 否 | 结束日期 |
| granularity | string | 否 | 粒度: daily/weekly/monthly |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "records": [
      {
        "date": "2026-03-16",
        "storage_gb": 500.5,
        "bandwidth_gb": 23.5,
        "cost": 50.05
      }
    ]
  }
}
```

---

## 账单管理

### 获取账单列表

```http
GET /api/v1/billing/invoices
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| status | string | 否 | 状态: pending/paid/overdue/void |
| year | int | 否 | 年份 |
| limit | int | 否 | 返回数量 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 12,
    "invoices": [
      {
        "id": "INV-2026-03-001",
        "period_start": "2026-03-01T00:00:00Z",
        "period_end": "2026-03-31T23:59:59Z",
        "due_date": "2026-04-15T00:00:00Z",
        "amount": 50.05,
        "currency": "CNY",
        "status": "pending",
        "items": [
          {
            "description": "存储空间 400.5 GB",
            "quantity": 400.5,
            "unit": "GB",
            "unit_price": 0.1,
            "amount": 40.05
          }
        ]
      }
    ]
  }
}
```

### 获取账单详情

```http
GET /api/v1/billing/invoices/:id
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "INV-2026-03-001",
    "period_start": "2026-03-01T00:00:00Z",
    "period_end": "2026-03-31T23:59:59Z",
    "due_date": "2026-04-15T00:00:00Z",
    "amount": 50.05,
    "tax": 5.76,
    "total": 55.81,
    "currency": "CNY",
    "status": "pending",
    "items": [
      {
        "description": "SSD 存储 200.2 GB",
        "quantity": 200.2,
        "unit": "GB",
        "unit_price": 0.15,
        "amount": 30.03
      },
      {
        "description": "HDD 存储 200.3 GB",
        "quantity": 200.3,
        "unit": "GB",
        "unit_price": 0.05,
        "amount": 10.02
      }
    ],
    "company_info": {
      "name": "示例公司",
      "address": "北京市朝阳区...",
      "tax_id": "91110000..."
    }
  }
}
```

### 下载账单 PDF

```http
GET /api/v1/billing/invoices/:id/pdf
```

**响应**

```
Content-Type: application/pdf
Content-Disposition: attachment; filename="INV-2026-03-001.pdf"
```

---

## 成本估算

### 估算成本

```http
POST /api/v1/billing/estimate
```

**请求体**

```json
{
  "storage_gb": 500,
  "storage_type": "ssd",
  "bandwidth_gb": 100,
  "duration_months": 12
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "storage_cost": 75.0,
    "bandwidth_cost": 50.0,
    "subtotal": 125.0,
    "tax": 14.38,
    "total": 139.38,
    "currency": "CNY",
    "monthly_average": 11.62,
    "savings_tips": [
      "使用 HDD 存储可节省 66% 存储成本",
      "年度预付可享 10% 折扣"
    ]
  }
}
```

---

## 配额管理

### 获取配额设置

```http
GET /api/v1/billing/quotas
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "global": {
      "max_storage_gb": 10000,
      "max_bandwidth_gb": 50000
    },
    "users": [
      {
        "user_id": "user-001",
        "username": "admin",
        "storage_quota_gb": 1000,
        "storage_used_gb": 500.5,
        "bandwidth_quota_gb": 5000,
        "bandwidth_used_gb": 239.8
      }
    ]
  }
}
```

### 设置用户配额

```http
PUT /api/v1/billing/quotas/:user_id
```

**请求体**

```json
{
  "storage_quota_gb": 2000,
  "bandwidth_quota_gb": 10000
}
```

---

## 成本分析 API

成本分析 API 提供资源消耗统计、成本趋势分析、费用预测等功能。

### 获取成本概览

```http
GET /api/v1/billing/cost/overview
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| period | string | 否 | 周期: current/last/last3/last12，默认 current |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "period": "current",
    "period_start": "2026-03-01T00:00:00Z",
    "period_end": "2026-03-31T23:59:59Z",
    "total_cost": 1250.50,
    "currency": "CNY",
    "breakdown": {
      "storage": {
        "amount": 800.00,
        "percentage": 64.0,
        "trend": "+5.2%"
      },
      "bandwidth": {
        "amount": 300.50,
        "currency": "CNY",
        "percentage": 24.0,
        "trend": "-2.1%"
      },
      "services": {
        "amount": 150.00,
        "percentage": 12.0,
        "trend": "+0.8%"
      }
    },
    "comparison": {
      "previous_period_cost": 1180.00,
      "change_percentage": 5.97,
      "change_direction": "up"
    },
    "savings": {
      "free_tier_savings": 45.00,
      "reserved_savings": 120.00,
      "total_savings": 165.00
    }
  }
}
```

### 获取成本趋势

```http
GET /api/v1/billing/cost/trends
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| start_date | string | 否 | 开始日期 (YYYY-MM-DD) |
| end_date | string | 否 | 结束日期 (YYYY-MM-DD) |
| granularity | string | 否 | 粒度: daily/weekly/monthly，默认 daily |
| group_by | string | 否 | 分组: service/user/type |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "granularity": "daily",
    "start_date": "2026-03-01",
    "end_date": "2026-03-16",
    "trends": [
      {
        "date": "2026-03-01",
        "total_cost": 40.50,
        "breakdown": {
          "storage": 26.00,
          "bandwidth": 10.50,
          "services": 4.00
        }
      },
      {
        "date": "2026-03-02",
        "total_cost": 42.30,
        "breakdown": {
          "storage": 27.10,
          "bandwidth": 11.20,
          "services": 4.00
        }
      }
    ],
    "statistics": {
      "average_daily_cost": 41.68,
      "max_daily_cost": 45.20,
      "min_daily_cost": 38.90,
      "trend_direction": "up",
      "trend_percentage": 3.2
    }
  }
}
```

### 按服务分析成本

```http
GET /api/v1/billing/cost/by-service
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| period | string | 否 | 周期: current/last |
| service_type | string | 否 | 服务类型过滤 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "period": "current",
    "services": [
      {
        "name": "storage_pool_main",
        "type": "storage",
        "cost": 450.00,
        "percentage": 36.0,
        "usage": {
          "total_gb": 4500.5,
          "ssd_gb": 2000.2,
          "hdd_gb": 2500.3
        },
        "unit_price": 0.10
      },
      {
        "name": "backup_service",
        "type": "service",
        "cost": 80.00,
        "percentage": 6.4,
        "usage": {
          "backup_count": 45,
          "storage_gb": 800
        }
      }
    ],
    "total": 1250.50,
    "currency": "CNY"
  }
}
```

### 按用户分析成本

```http
GET /api/v1/billing/cost/by-user
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| period | string | 否 | 周期: current/last |
| user_id | string | 否 | 用户ID过滤 |
| limit | int | 否 | 返回数量，默认 20 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "period": "current",
    "users": [
      {
        "user_id": "user-001",
        "username": "admin",
        "cost": 500.20,
        "percentage": 40.0,
        "breakdown": {
          "storage": 320.00,
          "bandwidth": 150.20,
          "services": 30.00
        },
        "trend": "+3.5%"
      },
      {
        "user_id": "user-002",
        "username": "alice",
        "cost": 350.15,
        "percentage": 28.0,
        "breakdown": {
          "storage": 280.00,
          "bandwidth": 55.15,
          "services": 15.00
        },
        "trend": "-1.2%"
      }
    ],
    "total": 1250.50,
    "currency": "CNY"
  }
}
```

### 成本预测

```http
GET /api/v1/billing/cost/forecast
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| months | int | 否 | 预测月数，默认 3 |
| scenario | string | 否 | 场景: normal/growth/savings |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "current_month_cost": 1250.50,
    "currency": "CNY",
    "forecast": [
      {
        "month": "2026-04",
        "predicted_cost": 1320.00,
        "confidence": 0.85,
        "range": {
          "low": 1250.00,
          "high": 1400.00
        },
        "factors": [
          {
            "factor": "storage_growth",
            "impact": "+5.2%",
            "description": "存储使用量持续增长"
          },
          {
            "factor": "bandwidth_pattern",
            "impact": "+0.3%",
            "description": "带宽使用稳定"
          }
        ]
      },
      {
        "month": "2026-05",
        "predicted_cost": 1390.00,
        "confidence": 0.75,
        "range": {
          "low": 1300.00,
          "high": 1480.00
        }
      }
    ],
    "recommendations": [
      {
        "type": "savings",
        "description": "考虑使用归档存储减少长期存储成本",
        "potential_savings": 120.00,
        "effort": "low"
      },
      {
        "type": "optimization",
        "description": "清理未使用的快照可节省存储空间",
        "potential_savings": 85.00,
        "effort": "medium"
      }
    ]
  }
}
```

### 成本优化建议

```http
GET /api/v1/billing/cost/recommendations
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "recommendations": [
      {
        "id": "rec-001",
        "type": "savings",
        "priority": "high",
        "title": "使用归档存储降低成本",
        "description": "将 6 个月未访问的 500GB 数据迁移到归档存储",
        "current_cost": 50.00,
        "potential_cost": 10.00,
        "monthly_savings": 40.00,
        "annual_savings": 480.00,
        "effort": "low",
        "affected_resources": [
          {
            "type": "volume",
            "id": "vol-archival-001",
            "name": "历史数据卷"
          }
        ]
      },
      {
        "id": "rec-002",
        "type": "optimization",
        "priority": "medium",
        "title": "清理孤立快照",
        "description": "发现 23 个孤立快照占用 150GB 空间",
        "current_cost": 15.00,
        "potential_cost": 0,
        "monthly_savings": 15.00,
        "annual_savings": 180.00,
        "effort": "low",
        "affected_resources": []
      }
    ],
    "total_monthly_savings": 55.00,
    "total_annual_savings": 660.00
  }
}
```

### 应用优化建议

```http
POST /api/v1/billing/cost/recommendations/:id/apply
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "recommendation_id": "rec-001",
    "status": "applied",
    "applied_at": "2026-03-16T16:00:00Z",
    "estimated_savings": {
      "monthly": 40.00,
      "annual": 480.00,
      "currency": "CNY"
    }
  }
}
```

---

## 预算警报 API

预算警报 API 支持设置预算限制、配置警报规则、管理通知渠道。

### 获取预算列表

```http
GET /api/v1/billing/budgets
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "budgets": [
      {
        "id": "budget-001",
        "name": "月度总体预算",
        "type": "total",
        "amount": 2000.00,
        "currency": "CNY",
        "period": "monthly",
        "used": 1250.50,
        "remaining": 749.50,
        "percentage": 62.5,
        "status": "active",
        "alerts": [
          {
            "threshold": 50,
            "triggered": true,
            "triggered_at": "2026-03-10T12:00:00Z"
          },
          {
            "threshold": 80,
            "triggered": false
          }
        ]
      },
      {
        "id": "budget-002",
        "name": "存储预算",
        "type": "service",
        "service_filter": "storage",
        "amount": 1000.00,
        "currency": "CNY",
        "period": "monthly",
        "used": 800.00,
        "remaining": 200.00,
        "percentage": 80.0,
        "status": "warning"
      }
    ]
  }
}
```

### 创建预算

```http
POST /api/v1/billing/budgets
```

**请求体**

```json
{
  "name": "月度总体预算",
  "type": "total",
  "amount": 2000.00,
  "currency": "CNY",
  "period": "monthly",
  "reset_day": 1,
  "alert_thresholds": [50, 80, 100],
  "alert_channels": ["email", "webhook"],
  "recipients": ["admin@example.com"],
  "webhook_url": "https://hooks.example.com/budget-alert"
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "budget-003",
    "name": "月度总体预算",
    "type": "total",
    "amount": 2000.00,
    "currency": "CNY",
    "period": "monthly",
    "reset_day": 1,
    "alert_thresholds": [50, 80, 100],
    "alert_channels": ["email", "webhook"],
    "status": "active",
    "created_at": "2026-03-16T16:00:00Z"
  }
}
```

### 更新预算

```http
PUT /api/v1/billing/budgets/:id
```

**请求体**

```json
{
  "name": "月度总体预算（调整）",
  "amount": 2500.00,
  "alert_thresholds": [60, 85, 100]
}
```

### 删除预算

```http
DELETE /api/v1/billing/budgets/:id
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "budget-003",
    "deleted": true
  }
}
```

### 获取预算详情

```http
GET /api/v1/billing/budgets/:id
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "budget-001",
    "name": "月度总体预算",
    "type": "total",
    "amount": 2000.00,
    "currency": "CNY",
    "period": "monthly",
    "reset_day": 1,
    "used": 1250.50,
    "remaining": 749.50,
    "percentage": 62.5,
    "status": "active",
    "alert_thresholds": [50, 80, 100],
    "alert_channels": ["email", "webhook"],
    "recipients": ["admin@example.com"],
    "webhook_url": "https://hooks.example.com/budget-alert",
    "history": [
      {
        "period": "2026-02",
        "used": 1850.00,
        "percentage": 92.5,
        "status": "warning"
      },
      {
        "period": "2026-01",
        "used": 1620.00,
        "percentage": 81.0,
        "status": "warning"
      }
    ],
    "alerts_triggered": [
      {
        "threshold": 50,
        "triggered_at": "2026-03-10T12:00:00Z",
        "notified": true,
        "notification_channels": ["email", "webhook"]
      }
    ]
  }
}
```

### 获取预算警报历史

```http
GET /api/v1/billing/budgets/:id/alerts
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| start_date | string | 否 | 开始日期 |
| end_date | string | 否 | 结束日期 |
| limit | int | 否 | 返回数量 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "budget_id": "budget-001",
    "alerts": [
      {
        "id": "alert-001",
        "threshold": 50,
        "triggered_at": "2026-03-10T12:00:00Z",
        "actual_percentage": 51.2,
        "actual_amount": 1024.00,
        "budget_amount": 2000.00,
        "notification_status": "sent",
        "channels": ["email", "webhook"],
        "recipients": ["admin@example.com"],
        "webhook_response": {
          "status": 200,
          "response_time_ms": 145
        }
      }
    ]
  }
}
```

### 测试预算警报

```http
POST /api/v1/billing/budgets/:id/test-alert
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "budget_id": "budget-001",
    "test_result": {
      "email": {
        "status": "success",
        "sent_to": ["admin@example.com"],
        "message_id": "msg-abc123"
      },
      "webhook": {
        "status": "success",
        "response_code": 200,
        "response_time_ms": 145
      }
    }
  }
}
```

### 配置警报渠道

```http
POST /api/v1/billing/alert-channels
```

**请求体**

```json
{
  "type": "webhook",
  "name": "Slack 通知",
  "config": {
    "url": "https://hooks.slack.com/services/xxx",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    }
  },
  "enabled": true
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "channel-001",
    "type": "webhook",
    "name": "Slack 通知",
    "enabled": true,
    "created_at": "2026-03-16T16:00:00Z"
  }
}
```

### 获取警报渠道列表

```http
GET /api/v1/billing/alert-channels
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "channels": [
      {
        "id": "channel-001",
        "type": "email",
        "name": "管理员邮箱",
        "recipients": ["admin@example.com", "finance@example.com"],
        "enabled": true
      },
      {
        "id": "channel-002",
        "type": "webhook",
        "name": "Slack 通知",
        "url": "https://hooks.slack.com/services/xxx",
        "enabled": true
      }
    ]
  }
}
```

---

## 数据模型

### BillingConfig 计费配置

```typescript
interface BillingConfig {
  enabled: boolean;
  default_currency: string;      // CNY, USD
  billing_cycle: BillingCycle;   // daily, weekly, monthly, yearly
  billing_day_of_month: number;  // 1-28
  storage_pricing: StoragePricingConfig;
  bandwidth_pricing: BandwidthPricingConfig;
  invoice_prefix: string;
  invoice_due_days: number;
  tax_rate: number;              // 0.13 = 13%
  tax_included: boolean;
}
```

### BillingCycle 计费周期

```typescript
type BillingCycle = 'daily' | 'weekly' | 'monthly' | 'yearly';
```

### Invoice 发票

```typescript
interface Invoice {
  id: string;
  period_start: string;
  period_end: string;
  due_date: string;
  amount: number;
  tax: number;
  total: number;
  currency: string;
  status: InvoiceStatus;
  items: InvoiceItem[];
}

type InvoiceStatus = 'pending' | 'paid' | 'overdue' | 'void';
```

### InvoiceItem 发票项目

```typescript
interface InvoiceItem {
  description: string;
  quantity: number;
  unit: string;
  unit_price: number;
  amount: number;
}
```