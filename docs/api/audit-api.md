# 审计 API 文档

**版本**: v2.67.0  
**更新日期**: 2026-03-15

## 概述

审计 API 提供安全审计日志的记录、查询、导出和合规报告功能。

## 基础路径

```
/api/v1/audit
```

---

## 日志查询

### 获取审计日志列表

```http
GET /api/v1/audit/logs
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| limit | int | 否 | 返回数量，默认 50 |
| offset | int | 否 | 偏移量，默认 0 |
| start_time | string | 否 | 开始时间 (RFC3339) |
| end_time | string | 否 | 结束时间 (RFC3339) |
| level | string | 否 | 日志级别: info/warning/error/critical |
| category | string | 否 | 日志分类: auth/access/data/system/security |
| user_id | string | 否 | 用户 ID |
| username | string | 否 | 用户名 |
| ip | string | 否 | IP 地址 |
| status | string | 否 | 操作状态: success/failure/pending |
| event | string | 否 | 事件类型 |
| keyword | string | 否 | 关键词搜索 |

**请求示例**

```bash
curl -X GET "https://nas.local/api/v1/audit/logs?limit=20&category=auth&start_time=2026-03-01T00:00:00Z" \
  -H "Authorization: Bearer <token>"
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 156,
    "entries": [
      {
        "id": "audit-20260315111300-001",
        "timestamp": "2026-03-15T11:13:00Z",
        "level": "info",
        "category": "auth",
        "event": "login",
        "user_id": "user-001",
        "username": "admin",
        "ip": "192.168.1.100",
        "user_agent": "Mozilla/5.0...",
        "status": "success",
        "message": "用户登录成功"
      },
      {
        "id": "audit-20260315111230-002",
        "timestamp": "2026-03-15T11:12:30Z",
        "level": "warning",
        "category": "auth",
        "event": "login_failed",
        "user_id": "",
        "username": "unknown",
        "ip": "192.168.1.200",
        "status": "failure",
        "message": "登录失败：密码错误",
        "details": {
          "attempt_count": 3
        }
      }
    ]
  }
}
```

---

### 获取单条日志详情

```http
GET /api/v1/audit/logs/:id
```

**请求示例**

```bash
curl -X GET "https://nas.local/api/v1/audit/logs/audit-20260315111300-001" \
  -H "Authorization: Bearer <token>"
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "audit-20260315111300-001",
    "timestamp": "2026-03-15T11:13:00Z",
    "level": "info",
    "category": "auth",
    "event": "login",
    "user_id": "user-001",
    "username": "admin",
    "ip": "192.168.1.100",
    "status": "success",
    "message": "用户登录成功",
    "signature": "sha256:abc123..."
  }
}
```

---

## 统计与仪表板

### 获取审计统计

```http
GET /api/v1/audit/statistics
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_entries": 15420,
    "today_entries": 156,
    "failed_auth_today": 3,
    "success_auth_today": 42,
    "top_users": [
      {"user_id": "user-001", "username": "admin", "count": 89},
      {"user_id": "user-002", "username": "user1", "count": 45}
    ],
    "top_ips": [
      {"ip": "192.168.1.100", "count": 120},
      {"ip": "192.168.1.101", "count": 36}
    ],
    "events_by_category": {
      "auth": 456,
      "access": 321,
      "data": 189,
      "system": 67
    },
    "events_by_level": {
      "info": 1400,
      "warning": 100,
      "error": 30,
      "critical": 5
    }
  }
}
```

### 获取仪表板数据

```http
GET /api/v1/audit/dashboard
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "summary": {
      "total_events": 15420,
      "auth_success_rate": 98.5,
      "unique_users": 12,
      "security_alerts": 3
    },
    "recent_alerts": [...],
    "hourly_distribution": {...},
    "category_breakdown": {...}
  }
}
```

---

## 日志导出

### 导出审计日志

```http
GET /api/v1/audit/export
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| format | string | 否 | 导出格式: json/csv/xml，默认 json |
| start_time | string | 否 | 开始时间 |
| end_time | string | 否 | 结束时间 |
| categories | string | 否 | 分类列表，逗号分隔 |
| include_signatures | bool | 否 | 是否包含签名 |

**请求示例**

```bash
curl -X GET "https://nas.local/api/v1/audit/export?format=csv&start_time=2026-03-01T00:00:00Z" \
  -H "Authorization: Bearer <token>" \
  -o audit-logs.csv
```

---

## 合规报告

### 获取合规报告

```http
GET /api/v1/audit/compliance/report
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| standard | string | 否 | 合规标准: gdpr/mlps/iso27001/hipaa/pci/sox |
| start_time | string | 否 | 开始时间 |
| end_time | string | 否 | 结束时间 |

**请求示例**

```bash
curl -X GET "https://nas.local/api/v1/audit/compliance/report?standard=gdpr" \
  -H "Authorization: Bearer <token>"
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "report_id": "rpt-20260315-001",
    "standard": "gdpr",
    "generated_at": "2026-03-15T11:13:00Z",
    "period_start": "2026-02-13T00:00:00Z",
    "period_end": "2026-03-15T11:13:00Z",
    "summary": {
      "total_events": 15420,
      "auth_events": 4560,
      "failed_auth_attempts": 23,
      "data_access_events": 3210,
      "config_changes": 45,
      "security_alerts": 3
    },
    "findings": [
      {
        "id": "f-001",
        "severity": "warning",
        "category": "auth",
        "title": "多次登录失败",
        "description": "IP 192.168.1.200 在1小时内尝试登录失败5次"
      }
    ],
    "recommendations": [
      "建议启用账户锁定策略",
      "建议增加多因素认证"
    ]
  }
}
```

### 获取支持的合规标准

```http
GET /api/v1/audit/compliance/standards
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {"code": "gdpr", "name": "GDPR (欧盟通用数据保护条例)"},
    {"code": "mlps", "name": "等级保护 (中国网络安全等级保护)"},
    {"code": "iso27001", "name": "ISO 27001 (信息安全管理体系)"},
    {"code": "hipaa", "name": "HIPAA (美国健康保险携带和责任法案)"},
    {"code": "pci", "name": "PCI DSS (支付卡行业数据安全标准)"},
    {"code": "sox", "name": "SOX (萨班斯-奥克斯利法案)"}
  ]
}
```

---

## 完整性验证

### 验证日志完整性

```http
GET /api/v1/audit/integrity
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "generated_at": "2026-03-15T11:13:00Z",
    "total_entries": 15420,
    "verified": 15420,
    "tampered": 0,
    "missing": 0,
    "valid": true
  }
}
```

---

## 配置管理

### 获取审计配置

```http
GET /api/v1/audit/config
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "enabled": true,
    "log_path": "/var/lib/nas-os/audit",
    "max_entries": 1000000,
    "max_age_days": 365,
    "auto_save": true,
    "save_interval": 300000000000,
    "enable_signatures": true,
    "enable_compression": true,
    "compression_type": "zstd"
  }
}
```

### 更新审计配置

```http
PUT /api/v1/audit/config
```

**请求体**

```json
{
  "enabled": true,
  "max_age_days": 180,
  "enable_signatures": true
}
```

---

## 日志记录接口

### 记录认证事件

```http
POST /api/v1/audit/log/auth
```

**请求体**

```json
{
  "event": "login",
  "user_id": "user-001",
  "username": "admin",
  "ip": "192.168.1.100",
  "user_agent": "Mozilla/5.0...",
  "status": "success",
  "message": "用户登录成功",
  "details": {}
}
```

### 记录访问事件

```http
POST /api/v1/audit/log/access
```

**请求体**

```json
{
  "user_id": "user-001",
  "username": "admin",
  "ip": "192.168.1.100",
  "resource": "/share/documents",
  "action": "read",
  "status": "success",
  "details": {
    "file_count": 5
  }
}
```

### 记录安全事件

```http
POST /api/v1/audit/log/security
```

**请求体**

```json
{
  "event": "intrusion_attempt",
  "user_id": "",
  "username": "",
  "ip": "192.168.1.200",
  "level": "critical",
  "message": "检测到可疑的入侵尝试",
  "details": {
    "attack_type": "brute_force"
  }
}
```

---

## 数据模型

### Entry 审计日志条目

```typescript
interface Entry {
  id: string;           // 唯一标识
  timestamp: string;    // ISO 8601 时间戳
  level: Level;         // 日志级别
  category: Category;   // 日志分类
  event: string;        // 事件类型
  user_id?: string;     // 用户ID
  username?: string;    // 用户名
  ip?: string;          // 客户端IP
  user_agent?: string;  // 用户代理
  resource?: string;    // 操作资源
  action?: string;      // 操作类型
  status: Status;       // 操作状态
  message?: string;     // 日志消息
  details?: object;     // 详细信息
  signature?: string;   // 数字签名
}
```

### Level 日志级别

```typescript
type Level = 'info' | 'warning' | 'error' | 'critical';
```

### Category 日志分类

```typescript
type Category = 
  | 'auth'       // 认证相关
  | 'access'     // 访问控制
  | 'data'       // 数据操作
  | 'system'     // 系统配置
  | 'security'   // 安全事件
  | 'compliance' // 合规相关
  | 'file'       // 文件操作
  | 'network'    // 网络操作
  | 'user';      // 用户管理
```

### Status 操作状态

```typescript
type Status = 'success' | 'failure' | 'pending';
```