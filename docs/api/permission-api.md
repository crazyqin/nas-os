# 权限管理 API 文档

**版本**: v2.63.0  
**更新日期**: 2026-03-15

---

## 概述

权限管理 API 提供完整的 RBAC (基于角色的访问控制) 管理功能，包括用户权限、用户组权限、策略管理等。

---

## 认证

所有 API 请求需要在 Header 中携带 JWT Token：

```
Authorization: Bearer <token>
```

---

## 用户权限管理

### 获取用户权限

获取指定用户的权限详情。

**请求**:

```http
GET /api/v1/rbac/users/{user_id}
```

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user_id": "user1",
    "username": "developer1",
    "role": "operator",
    "direct_permissions": ["storage:write", "share:write"],
    "inherited_permissions": ["backup:read"],
    "effective_permissions": [
      "system:read", "system:write",
      "storage:read", "storage:write",
      "share:read", "share:write",
      "network:read", "network:write",
      "service:read", "service:write",
      "backup:read", "backup:write",
      "log:read", "monitor:read"
    ],
    "group_memberships": [
      {
        "group_id": "developers",
        "group_name": "开发组",
        "is_owner": false
      }
    ],
    "last_checked": "2026-03-15T10:30:00Z"
  }
}
```

---

### 设置用户角色

为用户分配角色。

**请求**:

```http
PUT /api/v1/rbac/users/{user_id}/role
```

**请求体**:

```json
{
  "role": "operator"
}
```

**响应**:

```json
{
  "code": 0,
  "message": "角色设置成功",
  "data": {
    "user_id": "user1",
    "role": "operator"
  }
}
```

**可用角色**:

| 角色 | 说明 |
|------|------|
| `admin` | 系统管理员 |
| `operator` | 运维员 |
| `readonly` | 只读用户 |
| `guest` | 访客 |

---

### 授予用户权限

为用户授予特定权限。

**请求**:

```http
POST /api/v1/rbac/users/{user_id}/permissions
```

**请求体**:

```json
{
  "permission": "storage:write"
}
```

**响应**:

```json
{
  "code": 0,
  "message": "权限授予成功",
  "data": {
    "user_id": "user1",
    "permission": "storage:write"
  }
}
```

---

### 撤销用户权限

撤销用户的特定权限。

**请求**:

```http
DELETE /api/v1/rbac/users/{user_id}/permissions/{permission}
```

**响应**:

```json
{
  "code": 0,
  "message": "权限撤销成功"
}
```

---

### 列出所有用户权限

获取所有用户的权限列表。

**请求**:

```http
GET /api/v1/rbac/users
```

**查询参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `role` | string | 按角色筛选 |
| `page` | int | 页码 (默认 1) |
| `page_size` | int | 每页数量 (默认 20) |

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 50,
    "page": 1,
    "page_size": 20,
    "items": [
      {
        "user_id": "user1",
        "username": "admin",
        "role": "admin",
        "effective_permissions": ["*:*"]
      }
    ]
  }
}
```

---

## 用户组权限管理

### 创建用户组权限

创建新的用户组并设置权限。

**请求**:

```http
POST /api/v1/rbac/groups
```

**请求体**:

```json
{
  "group_id": "developers",
  "group_name": "开发组",
  "permissions": ["storage:read", "storage:write", "share:read"]
}
```

**响应**:

```json
{
  "code": 0,
  "message": "用户组创建成功",
  "data": {
    "group_id": "developers",
    "group_name": "开发组",
    "permissions": ["storage:read", "storage:write", "share:read"],
    "created_at": "2026-03-15T10:30:00Z"
  }
}
```

---

### 获取用户组权限

获取指定用户组的权限详情。

**请求**:

```http
GET /api/v1/rbac/groups/{group_id}
```

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "group_id": "developers",
    "group_name": "开发组",
    "permissions": ["storage:read", "storage:write", "share:read"],
    "created_at": "2026-03-15T10:30:00Z",
    "updated_at": "2026-03-15T10:30:00Z"
  }
}
```

---

### 更新用户组权限

更新用户组的权限列表。

**请求**:

```http
PUT /api/v1/rbac/groups/{group_id}
```

**请求体**:

```json
{
  "permissions": ["storage:read", "storage:write", "share:read", "share:write"]
}
```

**响应**:

```json
{
  "code": 0,
  "message": "用户组权限更新成功"
}
```

---

### 删除用户组权限

删除用户组。

**请求**:

```http
DELETE /api/v1/rbac/groups/{group_id}
```

**响应**:

```json
{
  "code": 0,
  "message": "用户组删除成功"
}
```

---

### 将用户添加到组

将用户添加到指定用户组。

**请求**:

```http
POST /api/v1/rbac/users/{user_id}/groups/{group_id}
```

**请求体**:

```json
{
  "is_owner": false
}
```

**响应**:

```json
{
  "code": 0,
  "message": "用户已添加到组"
}
```

---

### 从组中移除用户

从用户组中移除用户。

**请求**:

```http
DELETE /api/v1/rbac/users/{user_id}/groups/{group_id}
```

**响应**:

```json
{
  "code": 0,
  "message": "用户已从组中移除"
}
```

---

### 列出所有用户组

获取所有用户组列表。

**请求**:

```http
GET /api/v1/rbac/groups
```

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "group_id": "developers",
        "group_name": "开发组",
        "permissions": ["storage:read", "storage:write"],
        "member_count": 5
      }
    ]
  }
}
```

---

## 策略管理

### 创建策略

创建新的权限策略。

**请求**:

```http
POST /api/v1/rbac/policies
```

**请求体**:

```json
{
  "name": "禁止访客修改网络",
  "description": "防止访客用户修改网络配置",
  "effect": "deny",
  "principals": ["group:guests"],
  "resources": ["network"],
  "actions": ["write"],
  "priority": 100,
  "conditions": [
    {
      "type": "time",
      "key": "hour",
      "operator": "in",
      "values": ["9-18"]
    }
  ]
}
```

**参数说明**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `name` | string | 策略名称 |
| `description` | string | 策略描述 |
| `effect` | string | 效果: `allow` 或 `deny` |
| `principals` | []string | 应用主体 (用户/组) |
| `resources` | []string | 资源列表 |
| `actions` | []string | 操作列表 |
| `priority` | int | 优先级 (数值越大优先级越高) |
| `conditions` | []object | 条件列表 (可选) |

**响应**:

```json
{
  "code": 0,
  "message": "策略创建成功",
  "data": {
    "id": "policy-001",
    "name": "禁止访客修改网络",
    "enabled": true,
    "created_at": "2026-03-15T10:30:00Z"
  }
}
```

---

### 获取策略详情

获取指定策略的详情。

**请求**:

```http
GET /api/v1/rbac/policies/{policy_id}
```

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "policy-001",
    "name": "禁止访客修改网络",
    "description": "防止访客用户修改网络配置",
    "effect": "deny",
    "principals": ["group:guests"],
    "resources": ["network"],
    "actions": ["write"],
    "priority": 100,
    "enabled": true,
    "conditions": [],
    "created_at": "2026-03-15T10:30:00Z",
    "updated_at": "2026-03-15T10:30:00Z"
  }
}
```

---

### 更新策略

更新策略配置。

**请求**:

```http
PUT /api/v1/rbac/policies/{policy_id}
```

**请求体**:

```json
{
  "enabled": false,
  "priority": 50
}
```

**响应**:

```json
{
  "code": 0,
  "message": "策略更新成功"
}
```

---

### 删除策略

删除指定策略。

**请求**:

```http
DELETE /api/v1/rbac/policies/{policy_id}
```

**响应**:

```json
{
  "code": 0,
  "message": "策略删除成功"
}
```

---

### 列出所有策略

获取所有策略列表。

**请求**:

```http
GET /api/v1/rbac/policies
```

**查询参数**:

| 参数 | 类型 | 说明 |
|------|------|------|
| `effect` | string | 按效果筛选: `allow` 或 `deny` |
| `enabled` | bool | 按启用状态筛选 |

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": "policy-001",
        "name": "禁止访客修改网络",
        "effect": "deny",
        "enabled": true,
        "priority": 100
      }
    ]
  }
}
```

---

## 权限检查

### 检查用户权限

检查用户是否有指定资源的操作权限。

**请求**:

```http
POST /api/v1/rbac/check
```

**请求体**:

```json
{
  "user_id": "user1",
  "resource": "storage",
  "action": "write"
}
```

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "allowed": true,
    "reason": "角色权限匹配",
    "matched_by": "operator",
    "missing_perms": []
  }
}
```

**响应字段**:

| 字段 | 类型 | 说明 |
|------|------|------|
| `allowed` | bool | 是否允许 |
| `reason` | string | 原因说明 |
| `matched_by` | string | 匹配的角色/策略 |
| `denied_by` | string | 拒绝的策略 (如有) |
| `missing_perms` | []string | 缺少的权限 |

---

### 快速权限检查

简化版权限检查，仅返回是否允许。

**请求**:

```http
GET /api/v1/rbac/check/{user_id}/{resource}/{action}
```

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "allowed": true
  }
}
```

---

## 角色管理

### 获取角色列表

获取所有可用角色及其权限。

**请求**:

```http
GET /api/v1/rbac/roles
```

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "name": "admin",
        "description": "系统管理员，拥有完全控制权限",
        "permissions": ["*:*"],
        "priority": 100
      },
      {
        "name": "operator",
        "description": "运维员，可以操作系统但不能管理用户",
        "permissions": [
          "system:read", "system:write",
          "storage:read", "storage:write",
          "share:read", "share:write"
        ],
        "priority": 75
      }
    ]
  }
}
```

---

### 获取角色详情

获取指定角色的详细信息。

**请求**:

```http
GET /api/v1/rbac/roles/{role}
```

**响应**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "name": "operator",
    "description": "运维员，可以操作系统但不能管理用户",
    "permissions": [
      "system:read", "system:write",
      "storage:read", "storage:write",
      "share:read", "share:write",
      "network:read", "network:write",
      "service:read", "service:write",
      "backup:read", "backup:write",
      "log:read", "monitor:read"
    ],
    "priority": 75
  }
}
```

---

## 缓存管理

### 清除用户缓存

清除指定用户的权限缓存。

**请求**:

```http
DELETE /api/v1/rbac/cache/{user_id}
```

**响应**:

```json
{
  "code": 0,
  "message": "缓存已清除"
}
```

---

### 清除所有缓存

清除所有用户的权限缓存。

**请求**:

```http
DELETE /api/v1/rbac/cache
```

**响应**:

```json
{
  "code": 0,
  "message": "所有缓存已清除"
}
```

---

## 错误码

| 错误码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1001 | 用户不存在 |
| 1002 | 角色不存在 |
| 1003 | 用户组不存在 |
| 1004 | 策略不存在 |
| 1005 | 无效的权限格式 |
| 1006 | 权限不足 |
| 1007 | 检测到循环依赖 |

---

## 相关文档

- [用户权限系统指南](../user-guide/permission-guide.md)
- [审计 API 文档](./audit-api.md)
- [用户管理 API](./README.md)

---

## 更新历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v2.55.0 | 2026-03-15 | 初始版本 |