# 用户 API 文档

**版本**: v2.63.0  
**更新日期**: 2026-03-15

---

## 概述

用户 API 提供用户管理、认证、权限分配等功能。

## 基础路径

```
/api/v1/users
```

---

## 认证

### 用户登录

```http
POST /api/v1/auth/login
```

**请求体**

```json
{
  "username": "admin",
  "password": "your-password"
}
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIs...",
    "expires_at": "2026-03-16T10:00:00Z",
    "user": {
      "id": "user-001",
      "username": "admin",
      "role": "admin"
    }
  }
}
```

### 用户登出

```http
POST /api/v1/auth/logout
```

### 刷新 Token

```http
POST /api/v1/auth/refresh
```

---

## 用户管理

### 获取用户列表

```http
GET /api/v1/users
```

**查询参数**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| role | string | 否 | 按角色筛选 |
| status | string | 否 | 按状态筛选: active/inactive |
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页数量，默认 20 |

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "users": [
      {
        "id": "user-001",
        "username": "admin",
        "email": "admin@example.com",
        "role": "admin",
        "status": "active",
        "created_at": "2026-03-10T00:00:00Z",
        "last_login": "2026-03-15T08:30:00Z"
      }
    ],
    "total": 10,
    "page": 1,
    "page_size": 20
  }
}
```

### 创建用户

```http
POST /api/v1/users
```

**请求体**

```json
{
  "username": "developer",
  "password": "secure-password",
  "email": "developer@example.com",
  "role": "operator"
}
```

### 获取用户详情

```http
GET /api/v1/users/:id
```

### 更新用户

```http
PUT /api/v1/users/:id
```

**请求体**

```json
{
  "email": "new-email@example.com",
  "role": "readonly"
}
```

### 删除用户

```http
DELETE /api/v1/users/:id
```

---

## 密码管理

### 修改密码

```http
PUT /api/v1/users/:id/password
```

**请求体**

```json
{
  "current_password": "old-password",
  "new_password": "new-secure-password"
}
```

### 重置密码 (管理员)

```http
POST /api/v1/users/:id/password/reset
```

---

## 用户组管理

### 获取用户所属组

```http
GET /api/v1/users/:id/groups
```

### 将用户添加到组

```http
POST /api/v1/users/:id/groups/:group_id
```

### 从组中移除用户

```http
DELETE /api/v1/users/:id/groups/:group_id
```

---

## 权限管理

### 获取用户权限

```http
GET /api/v1/users/:id/permissions
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "role": "operator",
    "role_permissions": [
      "system:read",
      "storage:read",
      "storage:write"
    ],
    "direct_permissions": [],
    "inherited_permissions": []
  }
}
```

### 授予权限

```http
POST /api/v1/users/:id/permissions
```

**请求体**

```json
{
  "permission": "storage:admin"
}
```

### 撤销权限

```http
DELETE /api/v1/users/:id/permissions/:permission
```

---

## MFA 管理

### 启用 MFA

```http
POST /api/v1/users/:id/mfa/enable
```

**响应示例**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "secret": "JBSWY3DPEHPK3PXP",
    "qr_code_url": "otpauth://totp/NAS-OS:admin?secret=...",
    "recovery_codes": [
      "abcd-efgh-ijkl",
      "mnop-qrst-uvwx"
    ]
  }
}
```

### 禁用 MFA

```http
POST /api/v1/users/:id/mfa/disable
```

### 验证 MFA

```http
POST /api/v1/users/:id/mfa/verify
```

**请求体**

```json
{
  "code": "123456"
}
```

---

## 会话管理

### 获取活跃会话

```http
GET /api/v1/users/:id/sessions
```

### 终止会话

```http
DELETE /api/v1/users/:id/sessions/:session_id
```

### 终止所有会话

```http
DELETE /api/v1/users/:id/sessions
```

---

## 错误码

| Code | 说明 |
|------|------|
| 0 | 成功 |
| 400 | 参数错误 |
| 401 | 未认证 |
| 403 | 权限不足 |
| 404 | 用户不存在 |
| 409 | 用户名已存在 |
| 500 | 服务器内部错误 |

---

## 相关文档

- [权限管理指南](../user-guide/permission-guide.md) - RBAC 权限系统
- [权限 API](permission-api.md) - 权限管理 API

---

*最后更新：2026-03-15*