# 发票与支付 API 文档

**版本**: v2.76.0  
**更新日期**: 2026-03-16

## 概述

发票与支付 API 提供账单管理、发票生成、支付处理等功能。

## 基础路径

```
/api/v1/billing
```

---

## 发票管理

### 获取发票列表

```http
GET /api/v1/billing/invoices
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| status | string | 否 | 状态: draft/pending/paid/overdue/void |
| year | int | 否 | 年份 |
| month | int | 否 | 月份 |
| limit | int | 否 | 返回数量，默认 20 |
| offset | int | 否 | 偏移量 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 36,
    "limit": 20,
    "offset": 0,
    "invoices": [
      {
        "id": "INV-2026-03-001",
        "number": "INV-2026-03-001",
        "period_start": "2026-03-01T00:00:00Z",
        "period_end": "2026-03-31T23:59:59Z",
        "issue_date": "2026-04-01T00:00:00Z",
        "due_date": "2026-04-15T00:00:00Z",
        "amount": 1250.50,
        "tax": 144.31,
        "total": 1394.81,
        "currency": "CNY",
        "status": "pending",
        "payment_status": "unpaid"
      }
    ]
  }
}
```

### 获取发票详情

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
    "number": "INV-2026-03-001",
    "period_start": "2026-03-01T00:00:00Z",
    "period_end": "2026-03-31T23:59:59Z",
    "issue_date": "2026-04-01T00:00:00Z",
    "due_date": "2026-04-15T00:00:00Z",
    "amount": 1250.50,
    "tax": 144.31,
    "discount": 0,
    "total": 1394.81,
    "currency": "CNY",
    "status": "pending",
    "payment_status": "unpaid",
    "items": [
      {
        "id": "item-001",
        "description": "SSD 存储 2000 GB",
        "category": "storage",
        "quantity": 2000,
        "unit": "GB",
        "unit_price": 0.15,
        "amount": 300.00
      },
      {
        "id": "item-002",
        "description": "HDD 存储 2500 GB",
        "category": "storage",
        "quantity": 2500,
        "unit": "GB",
        "unit_price": 0.05,
        "amount": 125.00
      },
      {
        "id": "item-003",
        "description": "带宽流量 500 GB",
        "category": "bandwidth",
        "quantity": 500,
        "unit": "GB",
        "unit_price": 0.5,
        "amount": 250.00
      }
    ],
    "summary": {
      "subtotal": 1250.50,
      "discount": 0,
      "taxable_amount": 1250.50,
      "tax_rate": 0.1154,
      "tax": 144.31,
      "total": 1394.81
    },
    "company_info": {
      "name": "示例科技有限公司",
      "address": "北京市朝阳区科技路100号",
      "tax_id": "91110000MA01234567",
      "phone": "400-123-4567",
      "email": "billing@example.com"
    },
    "customer_info": {
      "name": "客户公司名称",
      "address": "客户地址",
      "tax_id": "客户税号"
    },
    "payments": [],
    "created_at": "2026-04-01T00:00:00Z",
    "updated_at": "2026-04-01T00:00:00Z"
  }
}
```

### 创建发票（管理员）

```http
POST /api/v1/billing/invoices
```

**请求体**

```json
{
  "period_start": "2026-03-01T00:00:00Z",
  "period_end": "2026-03-31T23:59:59Z",
  "issue_date": "2026-04-01T00:00:00Z",
  "due_date": "2026-04-15T00:00:00Z",
  "items": [
    {
      "description": "存储服务",
      "category": "storage",
      "quantity": 1000,
      "unit": "GB",
      "unit_price": 0.1
    }
  ],
  "discount": 0,
  "customer_info": {
    "name": "客户公司",
    "address": "客户地址",
    "tax_id": "客户税号"
  }
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "INV-2026-03-002",
    "number": "INV-2026-03-002",
    "status": "draft",
    "amount": 100.00,
    "tax": 11.54,
    "total": 111.54,
    "created_at": "2026-03-16T16:00:00Z"
  }
}
```

### 更新发票

```http
PUT /api/v1/billing/invoices/:id
```

**请求体**

```json
{
  "due_date": "2026-04-30T00:00:00Z",
  "discount": 50.00,
  "items": [
    {
      "description": "存储服务（折扣后）",
      "category": "storage",
      "quantity": 1000,
      "unit": "GB",
      "unit_price": 0.08
    }
  ]
}
```

### 作废发票

```http
POST /api/v1/billing/invoices/:id/void
```

**请求体**

```json
{
  "reason": "重复开票"
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "INV-2026-03-002",
    "status": "void",
    "voided_at": "2026-03-16T16:00:00Z",
    "void_reason": "重复开票"
  }
}
```

### 下载发票 PDF

```http
GET /api/v1/billing/invoices/:id/pdf
```

**响应**

```
Content-Type: application/pdf
Content-Disposition: attachment; filename="INV-2026-03-001.pdf"

[PDF Binary Data]
```

### 发送发票邮件

```http
POST /api/v1/billing/invoices/:id/send
```

**请求体**

```json
{
  "recipients": ["finance@example.com", "admin@example.com"],
  "subject": "您的 NAS-OS 发票 INV-2026-03-001",
  "message": "请查收附件中的发票。",
  "include_pdf": true
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "invoice_id": "INV-2026-03-001",
    "sent": true,
    "recipients": ["finance@example.com", "admin@example.com"],
    "message_id": "msg-xyz789",
    "sent_at": "2026-03-16T16:00:00Z"
  }
}
```

---

## 支付管理

### 获取支付记录列表

```http
GET /api/v1/billing/payments
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| invoice_id | string | 否 | 发票ID |
| status | string | 否 | 状态: pending/completed/failed/refunded |
| method | string | 否 | 支付方式 |
| start_date | string | 否 | 开始日期 |
| end_date | string | 否 | 结束日期 |
| limit | int | 否 | 返回数量 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 45,
    "payments": [
      {
        "id": "PAY-2026-03-001",
        "invoice_id": "INV-2026-03-001",
        "amount": 1394.81,
        "currency": "CNY",
        "method": "alipay",
        "status": "completed",
        "transaction_id": "2026031522001401234567890",
        "paid_at": "2026-03-16T14:30:00Z",
        "created_at": "2026-03-16T14:25:00Z"
      }
    ]
  }
}
```

### 获取支付详情

```http
GET /api/v1/billing/payments/:id
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "PAY-2026-03-001",
    "invoice_id": "INV-2026-03-001",
    "amount": 1394.81,
    "currency": "CNY",
    "method": "alipay",
    "status": "completed",
    "transaction_id": "2026031522001401234567890",
    "gateway": "alipay",
    "gateway_response": {
      "trade_no": "2026031522001401234567890",
      "buyer_id": "2088123456789012",
      "buyer_logon_id": "user***@example.com"
    },
    "paid_at": "2026-03-16T14:30:00Z",
    "created_at": "2026-03-16T14:25:00Z",
    "updated_at": "2026-03-16T14:30:00Z",
    "receipt_url": "https://example.com/receipts/PAY-2026-03-001.pdf"
  }
}
```

### 创建支付

```http
POST /api/v1/billing/payments
```

**请求体**

```json
{
  "invoice_id": "INV-2026-03-001",
  "method": "alipay",
  "amount": 1394.81,
  "return_url": "https://example.com/payment/success",
  "notify_url": "https://example.com/api/webhooks/payment"
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "payment_id": "PAY-2026-03-002",
    "invoice_id": "INV-2026-03-001",
    "amount": 1394.81,
    "currency": "CNY",
    "method": "alipay",
    "status": "pending",
    "payment_url": "https://openapi.alipay.com/gateway.do?...",
    "qr_code": "https://qr.alipay.com/xxx",
    "expires_at": "2026-03-16T15:25:00Z"
  }
}
```

### 支付回调处理

```http
POST /api/v1/billing/payments/webhook
```

此接口由支付网关调用，用于通知支付结果。

**响应示例**

```json
{
  "code": 0,
  "message": "success"
}
```

### 申请退款

```http
POST /api/v1/billing/payments/:id/refund
```

**请求体**

```json
{
  "amount": 500.00,
  "reason": "服务取消",
  "notify_url": "https://example.com/api/webhooks/refund"
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "refund_id": "REF-2026-03-001",
    "payment_id": "PAY-2026-03-001",
    "amount": 500.00,
    "currency": "CNY",
    "status": "processing",
    "reason": "服务取消",
    "created_at": "2026-03-16T16:00:00Z"
  }
}
```

### 获取退款详情

```http
GET /api/v1/billing/refunds/:id
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "REF-2026-03-001",
    "payment_id": "PAY-2026-03-001",
    "invoice_id": "INV-2026-03-001",
    "amount": 500.00,
    "currency": "CNY",
    "status": "completed",
    "reason": "服务取消",
    "transaction_id": "2026031522001401234567891",
    "refunded_at": "2026-03-16T16:30:00Z",
    "created_at": "2026-03-16T16:00:00Z"
  }
}
```

---

## 支付方式管理

### 获取可用支付方式

```http
GET /api/v1/billing/payment-methods
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "methods": [
      {
        "id": "alipay",
        "name": "支付宝",
        "type": "online",
        "enabled": true,
        "fee_rate": 0,
        "fee_fixed": 0,
        "currencies": ["CNY"],
        "min_amount": 0.01,
        "max_amount": 100000.00
      },
      {
        "id": "wechat",
        "name": "微信支付",
        "type": "online",
        "enabled": true,
        "fee_rate": 0,
        "fee_fixed": 0,
        "currencies": ["CNY"],
        "min_amount": 0.01,
        "max_amount": 100000.00
      },
      {
        "id": "bank_transfer",
        "name": "银行转账",
        "type": "offline",
        "enabled": true,
        "fee_rate": 0,
        "fee_fixed": 0,
        "currencies": ["CNY", "USD"],
        "bank_info": {
          "bank_name": "中国工商银行",
          "account_name": "示例科技有限公司",
          "account_number": "1234567890123456789"
        }
      },
      {
        "id": "credit_card",
        "name": "信用卡",
        "type": "online",
        "enabled": false,
        "fee_rate": 0.03,
        "fee_fixed": 0,
        "currencies": ["USD", "EUR", "GBP"]
      }
    ]
  }
}
```

### 配置支付方式（管理员）

```http
PUT /api/v1/billing/payment-methods/:id
```

**请求体**

```json
{
  "enabled": true,
  "fee_rate": 0.01,
  "fee_fixed": 0.5,
  "config": {
    "app_id": "2021xxx",
    "private_key": "MIIEvgIBADANBgkq...",
    "public_key": "MIIBIjANBgkqhk..."
  }
}
```

---

## 支付账户余额

### 获取账户余额

```http
GET /api/v1/billing/balance
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "balance": 5000.00,
    "pending": 200.00,
    "available": 4800.00,
    "currency": "CNY",
    "last_updated": "2026-03-16T16:00:00Z"
  }
}
```

### 充值余额

```http
POST /api/v1/billing/balance/recharge
```

**请求体**

```json
{
  "amount": 1000.00,
  "method": "alipay",
  "return_url": "https://example.com/balance/success"
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "recharge_id": "REC-2026-03-001",
    "amount": 1000.00,
    "currency": "CNY",
    "method": "alipay",
    "status": "pending",
    "payment_url": "https://openapi.alipay.com/gateway.do?..."
  }
}
```

### 使用余额支付

```http
POST /api/v1/billing/payments/balance
```

**请求体**

```json
{
  "invoice_id": "INV-2026-03-001"
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "payment_id": "PAY-2026-03-003",
    "invoice_id": "INV-2026-03-001",
    "amount": 1394.81,
    "currency": "CNY",
    "method": "balance",
    "status": "completed",
    "balance_before": 5000.00,
    "balance_after": 3605.19,
    "paid_at": "2026-03-16T16:00:00Z"
  }
}
```

### 获取余额交易记录

```http
GET /api/v1/billing/balance/transactions
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| type | string | 否 | 类型: recharge/payment/refund |
| start_date | string | 否 | 开始日期 |
| end_date | string | 否 | 结束日期 |
| limit | int | 否 | 返回数量 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "transactions": [
      {
        "id": "TXN-2026-03-001",
        "type": "payment",
        "amount": -1394.81,
        "balance_after": 3605.19,
        "description": "支付发票 INV-2026-03-001",
        "created_at": "2026-03-16T16:00:00Z"
      },
      {
        "id": "TXN-2026-03-002",
        "type": "recharge",
        "amount": 1000.00,
        "balance_after": 5000.00,
        "description": "余额充值",
        "created_at": "2026-03-14T10:00:00Z"
      }
    ]
  }
}
```

---

## 数据模型

### Invoice 发票

```typescript
interface Invoice {
  id: string;
  number: string;
  period_start: string;
  period_end: string;
  issue_date: string;
  due_date: string;
  amount: number;
  tax: number;
  discount: number;
  total: number;
  currency: string;
  status: InvoiceStatus;
  payment_status: PaymentStatus;
  items: InvoiceItem[];
  company_info: CompanyInfo;
  customer_info?: CustomerInfo;
  payments: Payment[];
  created_at: string;
  updated_at: string;
}

type InvoiceStatus = 'draft' | 'pending' | 'paid' | 'overdue' | 'void';
type PaymentStatus = 'unpaid' | 'partial' | 'paid';
```

### InvoiceItem 发票项目

```typescript
interface InvoiceItem {
  id: string;
  description: string;
  category: string;         // storage, bandwidth, service
  quantity: number;
  unit: string;
  unit_price: number;
  amount: number;
}
```

### Payment 支付

```typescript
interface Payment {
  id: string;
  invoice_id: string;
  amount: number;
  currency: string;
  method: PaymentMethod;
  status: PaymentStatus;
  transaction_id?: string;
  gateway?: string;
  gateway_response?: Record<string, any>;
  paid_at?: string;
  created_at: string;
  updated_at: string;
  receipt_url?: string;
}

type PaymentMethod = 'alipay' | 'wechat' | 'credit_card' | 'bank_transfer' | 'balance';
type PaymentStatus = 'pending' | 'processing' | 'completed' | 'failed' | 'refunded';
```

### Refund 退款

```typescript
interface Refund {
  id: string;
  payment_id: string;
  invoice_id: string;
  amount: number;
  currency: string;
  status: RefundStatus;
  reason: string;
  transaction_id?: string;
  refunded_at?: string;
  created_at: string;
}

type RefundStatus = 'processing' | 'completed' | 'failed';
```

### Balance 余额

```typescript
interface Balance {
  balance: number;
  pending: number;
  available: number;
  currency: string;
  last_updated: string;
}
```

### Transaction 交易记录

```typescript
interface Transaction {
  id: string;
  type: 'recharge' | 'payment' | 'refund';
  amount: number;
  balance_after: number;
  description: string;
  reference_id?: string;
  created_at: string;
}
```

---

## 错误码

| Code | 说明 |
|------|------|
| 10001 | 发票不存在 |
| 10002 | 发票状态不允许此操作 |
| 10003 | 支付金额不匹配 |
| 10004 | 支付方式不可用 |
| 10005 | 余额不足 |
| 10006 | 退款金额超过已付金额 |
| 10007 | 发票已支付 |
| 10008 | 发票已作废 |
| 10009 | 支付超时 |
| 10010 | 支付失败 |

---

## Webhook 事件

### payment.completed

支付完成时触发。

```json
{
  "event": "payment.completed",
  "timestamp": "2026-03-16T16:00:00Z",
  "data": {
    "payment_id": "PAY-2026-03-001",
    "invoice_id": "INV-2026-03-001",
    "amount": 1394.81,
    "currency": "CNY",
    "method": "alipay"
  }
}
```

### payment.failed

支付失败时触发。

```json
{
  "event": "payment.failed",
  "timestamp": "2026-03-16T16:00:00Z",
  "data": {
    "payment_id": "PAY-2026-03-001",
    "invoice_id": "INV-2026-03-001",
    "amount": 1394.81,
    "currency": "CNY",
    "method": "alipay",
    "error_code": "INSUFFICIENT_FUNDS",
    "error_message": "余额不足"
  }
}
```

### refund.completed

退款完成时触发。

```json
{
  "event": "refund.completed",
  "timestamp": "2026-03-16T16:30:00Z",
  "data": {
    "refund_id": "REF-2026-03-001",
    "payment_id": "PAY-2026-03-001",
    "amount": 500.00,
    "currency": "CNY"
  }
}
```

### invoice.overdue

发票逾期时触发。

```json
{
  "event": "invoice.overdue",
  "timestamp": "2026-04-16T00:00:00Z",
  "data": {
    "invoice_id": "INV-2026-03-001",
    "amount": 1394.81,
    "currency": "CNY",
    "due_date": "2026-04-15T00:00:00Z",
    "days_overdue": 1
  }
}
```