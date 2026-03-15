# NAS-OS API 文档

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
- [计费 API](./billing-api.md) - 计费系统管理
- [监控 API](./monitor-api.md) - 系统监控与分布式监控
- [存储 API](./storage-api.md) - 存储池管理
- [用户 API](./user-api.md) - 用户与权限管理