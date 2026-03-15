# NAS-OS API 文档

**版本**: v2.63.0  
**更新日期**: 2026-03-15

## 概述

NAS-OS 提供 RESTful API 接口，支持完整的系统管理功能。所有 API 使用 JSON 格式传输数据。

## 认证

### JWT Token 认证

```http
Authorization: Bearer <token>
```

获取 Token：
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "your-password"
}
```

响应：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_at": "2026-03-16T11:13:00Z"
  }
}
```

## 通用响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

### 错误码

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误 |
| 401 | 未认证 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 500 | 服务器内部错误 |

## API 模块

- [审计 API](./audit-api.md) - 审计日志管理
- [计费 API](./billing-api.md) - 计费系统管理、成本分析、预算警报
- [仪表板 API](./dashboard-api.md) - 系统监控仪表板管理 (v2.52.0+)
- [健康检查 API](./health-api.md) - 系统健康状态检查 (v2.52.0+)
- [发票与支付 API](./invoice-api.md) - 发票管理、在线支付 (v2.58.0+)
- [监控 API](./monitor-api.md) - 系统监控与分布式监控
- [权限管理 API](./permission-api.md) - RBAC 权限管理 (v2.55.0+)
- [存储 API](./storage-api.md) - 卷管理、子卷、快照 (v2.61.0+)
- [用户 API](./user-api.md) - 用户管理、认证、MFA (v2.61.0+)