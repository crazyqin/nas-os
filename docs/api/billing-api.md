# 计费 API 文档

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
        "date": "2026-03-15",
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