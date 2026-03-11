# NAS-OS v1.0 API 参考文档

> **版本**: v1.0.0  
> **基础路径**: `/api/v1`  
> **认证方式**: Bearer Token  
> **内容类型**: `application/json`

---

## 目录

1. [概述](#概述)
2. [认证](#认证)
3. [存储管理](#存储管理)
4. [文件共享](#文件共享)
5. [用户管理](#用户管理)
6. [系统管理](#系统管理)
7. [监控与告警](#监控与告警)
8. [错误码](#错误码)
9. [SDK 示例](#sdk 示例)

---

## 概述

### API 风格

NAS-OS API 采用 RESTful 风格设计：

- **资源导向**：每个资源有独立的 URL 路径
- **HTTP 方法**：GET（查询）、POST（创建）、PUT（更新）、DELETE（删除）
- **统一响应格式**：所有响应使用统一的 JSON 结构
- **状态码**：使用标准 HTTP 状态码

### 基础 URL

```
生产环境：https://<your-nas-ip>:8080/api/v1
开发环境：http://localhost:8080/api/v1
```

### 统一响应格式

**成功响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

**错误响应**：
```json
{
  "code": 400,
  "message": "Invalid request parameters",
  "error": {
    "field": "name",
    "reason": "required"
  }
}
```

**响应字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | integer | 状态码（0=成功，非 0=错误） |
| `message` | string | 响应消息 |
| `data` | object/array | 响应数据（成功时） |
| `error` | object | 错误详情（失败时） |

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 201 | 创建成功 |
| 204 | 删除成功（无内容） |
| 400 | 请求参数错误 |
| 401 | 未授权/认证失败 |
| 403 | 权限不足 |
| 404 | 资源不存在 |
| 409 | 资源冲突 |
| 422 | 数据验证失败 |
| 500 | 服务器内部错误 |
| 503 | 服务不可用 |

### 速率限制

| 端点类型 | 限制 |
|----------|------|
| 认证端点 | 10 次/分钟 |
| 数据读取 | 100 次/分钟 |
| 数据写入 | 30 次/分钟 |
| 管理操作 | 10 次/分钟 |

**响应头**：
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1710144000
```

---

## 认证

### 登录

获取访问令牌。

**请求**：
```http
POST /api/v1/auth/login
Content-Type: application/json

{
  "username": "admin",
  "password": "admin123"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600,
    "token_type": "Bearer",
    "user": {
      "id": "admin",
      "username": "admin",
      "role": "admin",
      "email": "admin@example.com"
    }
  }
}
```

**错误响应**：
```json
{
  "code": 401,
  "message": "Invalid credentials",
  "error": {
    "field": "username",
    "reason": "invalid"
  }
}
```

### 登出

使当前令牌失效。

**请求**：
```http
POST /api/v1/auth/logout
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success"
}
```

### 刷新令牌

使用刷新令牌获取新的访问令牌。

**请求**：
```http
POST /api/v1/auth/refresh
Content-Type: application/json

{
  "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 3600,
    "token_type": "Bearer"
  }
}
```

### 获取当前用户

**请求**：
```http
GET /api/v1/auth/me
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "admin",
    "username": "admin",
    "role": "admin",
    "email": "admin@example.com",
    "created_at": "2026-03-01T00:00:00Z",
    "last_login": "2026-03-11T10:00:00Z"
  }
}
```

### 修改密码

**请求**：
```http
POST /api/v1/auth/password
Authorization: Bearer <token>
Content-Type: application/json

{
  "old_password": "oldpass123",
  "new_password": "newpass123"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Password updated successfully"
}
```

---

## 存储管理

### 卷管理

#### 获取卷列表

**请求**：
```http
GET /api/v1/volumes
Authorization: Bearer <token>
```

**查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | integer | 页码（默认 1） |
| `limit` | integer | 每页数量（默认 20，最大 100） |
| `status` | string | 过滤状态（healthy/degraded/failed） |

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "name": "data",
        "uuid": "abc-123-def-456",
        "devices": ["/dev/sdb1", "/dev/sdc1"],
        "size": 2000000000000,
        "used": 500000000000,
        "free": 1500000000000,
        "usage_percent": 25.0,
        "data_profile": "raid1",
        "meta_profile": "raid1",
        "mount_point": "/mnt/data",
        "status": "healthy",
        "created_at": "2026-03-01T00:00:00Z",
        "updated_at": "2026-03-11T00:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "limit": 20
  }
}
```

#### 创建卷

**请求**：
```http
POST /api/v1/volumes
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "data",
  "devices": ["/dev/sdb1", "/dev/sdc1"],
  "profile": "raid1",
  "mount_point": "/mnt/data"
}
```

**请求字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 卷名称（2-32 字符，字母数字下划线） |
| `devices` | array | 是 | 设备路径列表 |
| `profile` | string | 否 | RAID 配置（single/raid0/raid1/raid5/raid6/raid10），默认 single |
| `mount_point` | string | 否 | 挂载点，默认 `/mnt/<name>` |
| `compression` | string | 否 | 压缩算法（zstd/lzo/gzip/none），默认 zstd |

**RAID 配置说明**：

| 配置 | 最少设备 | 容错 | 利用率 |
|------|----------|------|--------|
| `single` | 1 | 0 | 100% |
| `raid0` | 2 | 0 | 100% |
| `raid1` | 2 | 1 | 50% |
| `raid10` | 4 | 1 | 50% |
| `raid5` | 3 | 1 | (n-1)/n |
| `raid6` | 4 | 2 | (n-2)/n |

**响应**：
```json
{
  "code": 0,
  "message": "Volume created successfully",
  "data": {
    "name": "data",
    "uuid": "abc-123-def-456",
    "mount_point": "/mnt/data",
    "status": "creating"
  }
}
```

#### 获取卷详情

**请求**：
```http
GET /api/v1/volumes/:name
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "name": "data",
    "uuid": "abc-123-def-456",
    "devices": [
      {
        "path": "/dev/sdb1",
        "size": 1000000000000,
        "used": 250000000000,
        "status": "healthy"
      },
      {
        "path": "/dev/sdc1",
        "size": 1000000000000,
        "used": 250000000000,
        "status": "healthy"
      }
    ],
    "size": 2000000000000,
    "used": 500000000000,
    "free": 1500000000000,
    "usage_percent": 25.0,
    "data_profile": "raid1",
    "meta_profile": "raid1",
    "mount_point": "/mnt/data",
    "status": "healthy",
    "features": {
      "compression": "zstd",
      "quota_enabled": true,
      "snapshot_count": 5
    },
    "maintenance": {
      "balance_running": false,
      "balance_progress": 0,
      "scrub_running": false,
      "scrub_progress": 0,
      "scrub_errors": 0,
      "last_scrub": "2026-03-10T03:00:00Z"
    },
    "created_at": "2026-03-01T00:00:00Z",
    "updated_at": "2026-03-11T00:00:00Z"
  }
}
```

#### 删除卷

⚠️ **警告：删除卷会永久丢失所有数据！**

**请求**：
```http
DELETE /api/v1/volumes/:name
Authorization: Bearer <token>
```

**查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `force` | boolean | 是否强制删除（默认 false） |

**响应**：
```json
{
  "code": 0,
  "message": "Volume deleted successfully"
}
```

#### 添加设备

**请求**：
```http
POST /api/v1/volumes/:name/devices
Authorization: Bearer <token>
Content-Type: application/json

{
  "device": "/dev/sdd1"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Device added successfully",
  "data": {
    "device": "/dev/sdd1",
    "status": "adding"
  }
}
```

#### 移除设备

**请求**：
```http
DELETE /api/v1/volumes/:name/devices/:device
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "Device removed successfully"
}
```

#### 启动数据平衡

**请求**：
```http
POST /api/v1/volumes/:name/balance
Authorization: Bearer <token>
Content-Type: application/json

{
  "convert": {
    "data_profile": "raid1",
    "meta_profile": "raid1"
  }
}
```

**请求字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `convert.data_profile` | string | 否 | 转换后的数据配置 |
| `convert.meta_profile` | string | 否 | 转换后的元数据配置 |
| `usage_filter` | string | 否 | 使用率过滤器（如 "usage=50"） |

**响应**：
```json
{
  "code": 0,
  "message": "Balance started",
  "data": {
    "status": "running",
    "progress": 0
  }
}
```

#### 获取平衡状态

**请求**：
```http
GET /api/v1/volumes/:name/balance
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "running": true,
    "progress": 45.5,
    "status": "balancing",
    "started_at": "2026-03-11T10:00:00Z",
    "estimated_remaining": 1800
  }
}
```

#### 停止平衡

**请求**：
```http
DELETE /api/v1/volumes/:name/balance
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "Balance stopped"
}
```

#### 启动数据校验

**请求**：
```http
POST /api/v1/volumes/:name/scrub
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "Scrub started",
  "data": {
    "status": "running"
  }
}
```

#### 获取校验状态

**请求**：
```http
GET /api/v1/volumes/:name/scrub
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "running": false,
    "progress": 100,
    "status": "completed",
    "errors": 0,
    "last_completed": "2026-03-10T05:00:00Z"
  }
}
```

### 子卷管理

#### 获取子卷列表

**请求**：
```http
GET /api/v1/volumes/:name/subvolumes
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": 256,
      "name": "docker",
      "path": "/mnt/data/docker",
      "parent_id": 5,
      "read_only": false,
      "uuid": "subvol-uuid-123",
      "size": 10000000000,
      "used": 5000000000,
      "snapshot_count": 3,
      "created_at": "2026-03-01T00:00:00Z"
    }
  ]
}
```

#### 创建子卷

**请求**：
```http
POST /api/v1/volumes/:name/subvolumes
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "docker",
  "parent": "root"
}
```

**请求字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 子卷名称 |
| `parent` | string | 否 | 父子卷名称（默认 root） |

**响应**：
```json
{
  "code": 0,
  "message": "Subvolume created successfully",
  "data": {
    "id": 256,
    "name": "docker",
    "path": "/mnt/data/docker"
  }
}
```

#### 删除子卷

**请求**：
```http
DELETE /api/v1/volumes/:name/subvolumes/:subvol
Authorization: Bearer <token>
```

**查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `force` | boolean | 是否强制删除（默认 false） |

**响应**：
```json
{
  "code": 0,
  "message": "Subvolume deleted successfully"
}
```

#### 设置子卷只读

**请求**：
```http
PUT /api/v1/volumes/:name/subvolumes/:subvol/readonly
Authorization: Bearer <token>
Content-Type: application/json

{
  "read_only": true
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Read-only status updated"
}
```

### 快照管理

#### 获取快照列表

**请求**：
```http
GET /api/v1/volumes/:name/snapshots
Authorization: Bearer <token>
```

**查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `subvolume` | string | 过滤子卷 |
| `limit` | integer | 返回数量限制 |

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "name": "backup-2026-03-10",
      "subvolume": "docker",
      "path": "/mnt/data/.snapshots/backup-2026-03-10",
      "read_only": true,
      "size": 10000000000,
      "created_at": "2026-03-10T02:00:00Z"
    }
  ]
}
```

#### 创建快照

**请求**：
```http
POST /api/v1/volumes/:name/snapshots
Authorization: Bearer <token>
Content-Type: application/json

{
  "subvolume": "docker",
  "name": "backup-2026-03-11",
  "read_only": true,
  "description": "Daily backup"
}
```

**请求字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `subvolume` | string | 是 | 子卷名称 |
| `name` | string | 是 | 快照名称 |
| `read_only` | boolean | 否 | 是否只读（默认 true） |
| `description` | string | 否 | 描述信息 |

**响应**：
```json
{
  "code": 0,
  "message": "Snapshot created successfully",
  "data": {
    "name": "backup-2026-03-11",
    "path": "/mnt/data/.snapshots/backup-2026-03-11"
  }
}
```

#### 删除快照

**请求**：
```http
DELETE /api/v1/volumes/:name/snapshots/:snapshot
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "Snapshot deleted successfully"
}
```

#### 恢复快照

**请求**：
```http
POST /api/v1/volumes/:name/snapshots/:snapshot/restore
Authorization: Bearer <token>
Content-Type: application/json

{
  "target": "docker-restored",
  "replace": false
}
```

**请求字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `target` | string | 是 | 目标子卷名称 |
| `replace` | boolean | 否 | 是否替换原子卷（默认 false） |

**响应**：
```json
{
  "code": 0,
  "message": "Snapshot restored successfully",
  "data": {
    "target": "docker-restored",
    "status": "restoring"
  }
}
```

---

## 文件共享

### SMB 共享

#### 获取 SMB 共享列表

**请求**：
```http
GET /api/v1/shares/smb
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "smb-001",
      "name": "public",
      "path": "/mnt/data/public",
      "comment": "公共共享",
      "public": true,
      "read_only": false,
      "guest_ok": true,
      "permissions": [
        {
          "username": "john",
          "read_write": true
        }
      ],
      "status": "active",
      "created_at": "2026-03-01T00:00:00Z"
    }
  ]
}
```

#### 创建 SMB 共享

**请求**：
```http
POST /api/v1/shares/smb
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "public",
  "path": "/mnt/data/public",
  "comment": "公共共享",
  "public": true,
  "read_only": false,
  "guest_ok": true
}
```

**请求字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 共享名称（1-32 字符） |
| `path` | string | 是 | 共享路径 |
| `comment` | string | 否 | 共享描述 |
| `public` | boolean | 否 | 是否公开（默认 false） |
| `read_only` | boolean | 否 | 是否只读（默认 false） |
| `guest_ok` | boolean | 否 | 是否允许访客（默认 false） |

**响应**：
```json
{
  "code": 0,
  "message": "SMB share created successfully",
  "data": {
    "id": "smb-001",
    "name": "public",
    "path": "/mnt/data/public"
  }
}
```

#### 更新 SMB 共享

**请求**：
```http
PUT /api/v1/shares/smb/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "comment": "更新后的描述",
  "read_only": true,
  "guest_ok": false
}
```

**响应**：
```json
{
  "code": 0,
  "message": "SMB share updated successfully"
}
```

#### 删除 SMB 共享

**请求**：
```http
DELETE /api/v1/shares/smb/:id
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "SMB share deleted successfully"
}
```

#### 设置 SMB 权限

**请求**：
```http
POST /api/v1/shares/smb/:id/permission
Authorization: Bearer <token>
Content-Type: application/json

{
  "username": "john",
  "read_write": true
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Permission updated successfully"
}
```

#### 移除 SMB 权限

**请求**：
```http
DELETE /api/v1/shares/smb/:id/permission/:username
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "Permission removed successfully"
}
```

#### 重启 SMB 服务

**请求**：
```http
POST /api/v1/shares/smb/restart
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "SMB service restarted"
}
```

#### 获取 SMB 服务状态

**请求**：
```http
GET /api/v1/shares/smb/status
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "running": true,
    "connections": 5,
    "uptime": 86400
  }
}
```

### NFS 共享

#### 获取 NFS 共享列表

**请求**：
```http
GET /api/v1/shares/nfs
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "nfs-001",
      "name": "backup",
      "path": "/mnt/data/backup",
      "clients": ["192.168.1.0/24"],
      "options": "rw,sync,no_subtree_check",
      "status": "active",
      "created_at": "2026-03-01T00:00:00Z"
    }
  ]
}
```

#### 创建 NFS 共享

**请求**：
```http
POST /api/v1/shares/nfs
Authorization: Bearer <token>
Content-Type: application/json

{
  "name": "backup",
  "path": "/mnt/data/backup",
  "clients": ["192.168.1.0/24"],
  "options": "rw,sync,no_subtree_check"
}
```

**请求字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | 是 | 共享名称 |
| `path` | string | 是 | 共享路径 |
| `clients` | array | 是 | 允许的客户端网络列表 |
| `options` | string | 否 | NFS 导出选项 |

**NFS 选项说明**：

| 选项 | 说明 |
|------|------|
| `rw` | 读写权限 |
| `ro` | 只读权限 |
| `sync` | 同步写入 |
| `async` | 异步写入 |
| `no_subtree_check` | 禁用子树检查 |
| `no_root_squash` | 允许 root 访问 |
| `root_squash` | 将 root 映射为匿名 |

**响应**：
```json
{
  "code": 0,
  "message": "NFS share created successfully",
  "data": {
    "id": "nfs-001",
    "name": "backup",
    "path": "/mnt/data/backup"
  }
}
```

#### 更新 NFS 共享

**请求**：
```http
PUT /api/v1/shares/nfs/:id
Authorization: Bearer <token>
Content-Type: application/json

{
  "clients": ["192.168.1.0/24", "192.168.2.0/24"],
  "options": "rw,sync,no_subtree_check,noatime"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "NFS share updated successfully"
}
```

#### 删除 NFS 共享

**请求**：
```http
DELETE /api/v1/shares/nfs/:id
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "NFS share deleted successfully"
}
```

#### 重启 NFS 服务

**请求**：
```http
POST /api/v1/shares/nfs/restart
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "NFS service restarted"
}
```

#### 获取 NFS 服务状态

**请求**：
```http
GET /api/v1/shares/nfs/status
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "running": true,
    "exports": 3,
    "active_clients": 2
  }
}
```

#### 获取 NFS 客户端信息

**请求**：
```http
GET /api/v1/shares/nfs/clients
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "clients": [
      {
        "ip": "192.168.1.100",
        "mount": "/mnt/data/backup",
        "status": "active",
        "connected_since": "2026-03-11T08:00:00Z"
      }
    ]
  }
}
```

---

## 用户管理

### 获取用户列表

**请求**：
```http
GET /api/v1/users
Authorization: Bearer <token>
```

**查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | integer | 页码 |
| `limit` | integer | 每页数量 |
| `role` | string | 过滤角色 |
| `status` | string | 过滤状态（active/disabled） |

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": "admin",
        "username": "admin",
        "role": "admin",
        "email": "admin@example.com",
        "status": "active",
        "created_at": "2026-03-01T00:00:00Z",
        "updated_at": "2026-03-11T00:00:00Z",
        "last_login": "2026-03-11T10:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "limit": 20
  }
}
```

### 创建用户

**请求**：
```http
POST /api/v1/users
Authorization: Bearer <token>
Content-Type: application/json

{
  "username": "john",
  "password": "password123",
  "role": "editor",
  "email": "john@example.com"
}
```

**请求字段**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | 是 | 用户名（3-32 字符） |
| `password` | string | 是 | 密码（最少 8 字符） |
| `role` | string | 否 | 角色（admin/operator/editor/viewer），默认 viewer |
| `email` | string | 否 | 邮箱地址 |

**响应**：
```json
{
  "code": 0,
  "message": "User created successfully",
  "data": {
    "id": "john",
    "username": "john",
    "role": "editor"
  }
}
```

### 获取用户详情

**请求**：
```http
GET /api/v1/users/:username
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "john",
    "username": "john",
    "role": "editor",
    "email": "john@example.com",
    "status": "active",
    "permissions": [
      "shares:read",
      "shares:write",
      "snapshots:create"
    ],
    "created_at": "2026-03-01T00:00:00Z",
    "updated_at": "2026-03-11T00:00:00Z",
    "last_login": "2026-03-11T09:00:00Z"
  }
}
```

### 更新用户

**请求**：
```http
PUT /api/v1/users/:username
Authorization: Bearer <token>
Content-Type: application/json

{
  "role": "admin",
  "email": "newemail@example.com",
  "status": "active"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "User updated successfully"
}
```

### 删除用户

⚠️ **注意：不能删除管理员账户**

**请求**：
```http
DELETE /api/v1/users/:username
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "User deleted successfully"
}
```

### 禁用用户

**请求**：
```http
POST /api/v1/users/:username/disable
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "User disabled successfully"
}
```

### 启用用户

**请求**：
```http
POST /api/v1/users/:username/enable
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "User enabled successfully"
}
```

### 修改用户密码

**请求**：
```http
POST /api/v1/users/:username/password
Authorization: Bearer <token>
Content-Type: application/json

{
  "new_password": "newpassword123"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Password updated successfully"
}
```

---

## 系统管理

### 获取系统信息

**请求**：
```http
GET /api/v1/system/info
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "hostname": "nas-os",
    "version": "1.0.0",
    "build": "2026-03-11T00:00:00Z",
    "os": "Linux",
    "arch": "arm64",
    "uptime": 86400,
    "kernel": "6.1.99-rockchip-rk3588"
  }
}
```

### 健康检查

**请求**：
```http
GET /api/v1/system/health
```

**响应**：
```json
{
  "code": 0,
  "message": "healthy",
  "data": {
    "status": "healthy",
    "checks": {
      "database": "healthy",
      "storage": "healthy",
      "smb": "healthy",
      "nfs": "healthy"
    }
  }
}
```

### 获取系统状态

**请求**：
```http
GET /api/v1/system/status
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "volumes": {
      "total": 2,
      "healthy": 2,
      "degraded": 0,
      "failed": 0
    },
    "shares": {
      "smb": 3,
      "nfs": 2
    },
    "users": {
      "total": 5,
      "active": 4,
      "disabled": 1
    },
    "resources": {
      "cpu_usage": 15.5,
      "memory_usage": 45.2,
      "disk_io_read": 1024000,
      "disk_io_write": 512000
    }
  }
}
```

### 重启服务

**请求**：
```http
POST /api/v1/system/restart
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "Service restarting"
}
```

### 关闭服务

**请求**：
```http
POST /api/v1/system/shutdown
Authorization: Bearer <token>
Content-Type: application/json

{
  "delay": 60,
  "reason": "System maintenance"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Shutdown scheduled"
}
```

### 获取配置

**请求**：
```http
GET /api/v1/system/config
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "server": {
      "port": 8080,
      "tls_enabled": false
    },
    "storage": {
      "auto_scrub": true,
      "compression": "zstd"
    },
    "smb": {
      "enabled": true,
      "workgroup": "WORKGROUP"
    },
    "nfs": {
      "enabled": true
    }
  }
}
```

### 更新配置

**请求**：
```http
PUT /api/v1/system/config
Authorization: Bearer <token>
Content-Type: application/json

{
  "server": {
    "port": 8080
  },
  "storage": {
    "auto_scrub": true
  }
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Configuration updated successfully"
}
```

### 导出配置

**请求**：
```http
GET /api/v1/system/config/export
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "config": "yaml 格式的配置内容",
    "exported_at": "2026-03-11T10:00:00Z"
  }
}
```

### 导入配置

**请求**：
```http
POST /api/v1/system/config/import
Authorization: Bearer <token>
Content-Type: application/json

{
  "config": "yaml 格式的配置内容",
  "validate_only": false
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Configuration imported successfully"
}
```

---

## 监控与告警

### 获取系统指标

**请求**：
```http
GET /api/v1/metrics
Authorization: Bearer <token>
```

**查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `start` | string | 开始时间（RFC3339） |
| `end` | string | 结束时间（RFC3339） |
| `step` | integer | 采样间隔（秒） |

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "cpu_usage": [
      {"timestamp": "2026-03-11T10:00:00Z", "value": 15.5},
      {"timestamp": "2026-03-11T10:01:00Z", "value": 18.2}
    ],
    "memory_usage": [
      {"timestamp": "2026-03-11T10:00:00Z", "value": 45.2},
      {"timestamp": "2026-03-11T10:01:00Z", "value": 46.1}
    ],
    "disk_io": {
      "read": 1024000,
      "write": 512000
    }
  }
}
```

### 获取磁盘健康

**请求**：
```http
GET /api/v1/disks/health
Authorization: Bearer <token>
```

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "device": "/dev/sdb1",
      "model": "WD40EFRX",
      "serial": "WD-123456",
      "temperature": 35,
      "smart_status": "PASSED",
      "power_on_hours": 8760,
      "reallocated_sectors": 0,
      "pending_sectors": 0,
      "health": "healthy"
    }
  ]
}
```

### 获取告警列表

**请求**：
```http
GET /api/v1/alerts
Authorization: Bearer <token>
```

**查询参数**：

| 参数 | 类型 | 说明 |
|------|------|------|
| `status` | string | 过滤状态（active/resolved） |
| `severity` | string | 过滤级别（warning/critical） |
| `limit` | integer | 返回数量限制 |

**响应**：
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": "alert-001",
        "type": "disk_usage",
        "severity": "warning",
        "message": "磁盘使用率超过 80%",
        "resource": "data",
        "status": "active",
        "created_at": "2026-03-11T08:00:00Z",
        "acknowledged": false
      }
    ],
    "total": 1
  }
}
```

### 确认告警

**请求**：
```http
POST /api/v1/alerts/:id/acknowledge
Authorization: Bearer <token>
Content-Type: application/json

{
  "note": "已处理"
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Alert acknowledged"
}
```

### 配置告警阈值

**请求**：
```http
PUT /api/v1/alerts/thresholds
Authorization: Bearer <token>
Content-Type: application/json

{
  "disk_usage_warning": 80,
  "disk_usage_critical": 95,
  "disk_temp_warning": 50,
  "disk_temp_critical": 60
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Thresholds updated successfully"
}
```

### 配置通知渠道

**请求**：
```http
POST /api/v1/alerts/notifications
Authorization: Bearer <token>
Content-Type: application/json

{
  "type": "email",
  "enabled": true,
  "config": {
    "smtp_server": "smtp.example.com",
    "smtp_port": 587,
    "smtp_user": "alerts@example.com",
    "smtp_password": "secret",
    "from": "nas-os@example.com",
    "to": ["admin@example.com"]
  }
}
```

**响应**：
```json
{
  "code": 0,
  "message": "Notification channel configured successfully"
}
```

---

## 错误码

### 通用错误码

| 错误码 | HTTP 状态 | 说明 |
|--------|----------|------|
| 0 | 200 | 成功 |
| 400 | 400 | 请求参数错误 |
| 401 | 401 | 未授权/认证失败 |
| 403 | 403 | 权限不足 |
| 404 | 404 | 资源不存在 |
| 409 | 409 | 资源冲突 |
| 422 | 422 | 数据验证失败 |
| 500 | 500 | 服务器内部错误 |
| 503 | 503 | 服务不可用 |

### 存储相关错误

| 错误码 | 说明 |
|--------|------|
| 1001 | 卷已存在 |
| 1002 | 卷不存在 |
| 1003 | 设备无效 |
| 1004 | 设备数量不足 |
| 1005 | 卷正在使用中 |
| 1006 | 快照已存在 |
| 1007 | 子卷已存在 |
| 1008 | btrfs 操作失败 |

### 共享相关错误

| 错误码 | 说明 |
|--------|------|
| 2001 | 共享已存在 |
| 2002 | 共享不存在 |
| 2003 | 路径无效 |
| 2004 | 权限配置错误 |
| 2005 | SMB 服务未启用 |
| 2006 | NFS 服务未启用 |

### 用户相关错误

| 错误码 | 说明 |
|--------|------|
| 3001 | 用户已存在 |
| 3002 | 用户不存在 |
| 3003 | 密码不符合要求 |
| 3004 | 角色无效 |
| 3005 | 不能删除管理员 |
| 3006 | 认证失败 |

---

## SDK 示例

### cURL 示例

```bash
# 登录
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'

# 获取卷列表
curl -X GET http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer <token>"

# 创建卷
curl -X POST http://localhost:8080/api/v1/volumes \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"data","devices":["/dev/sdb1"],"profile":"single"}'

# 创建 SMB 共享
curl -X POST http://localhost:8080/api/v1/shares/smb \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"public","path":"/mnt/data/public","guest_ok":true}'
```

### JavaScript 示例

```javascript
const API_BASE = 'http://localhost:8080/api/v1';

class NasOSClient {
  constructor(baseUrl = API_BASE) {
    this.baseUrl = baseUrl;
    this.token = null;
  }

  async login(username, password) {
    const res = await fetch(`${this.baseUrl}/auth/login`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });
    const data = await res.json();
    if (data.code === 0) {
      this.token = data.data.token;
    }
    return data;
  }

  async getVolumes() {
    const res = await fetch(`${this.baseUrl}/volumes`, {
      headers: { 'Authorization': `Bearer ${this.token}` }
    });
    const data = await res.json();
    return data.data;
  }

  async createVolume(name, devices, profile = 'single') {
    const res = await fetch(`${this.baseUrl}/volumes`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${this.token}`
      },
      body: JSON.stringify({ name, devices, profile })
    });
    return res.json();
  }

  async createSmbShare(name, path, options = {}) {
    const res = await fetch(`${this.baseUrl}/shares/smb`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${this.token}`
      },
      body: JSON.stringify({
        name,
        path,
        ...options
      })
    });
    return res.json();
  }
}

// 使用示例
const client = new NasOSClient();
await client.login('admin', 'admin123');
const volumes = await client.getVolumes();
console.log('Volumes:', volumes);
```

### Python 示例

```python
import requests

class NasOSClient:
    def __init__(self, base_url='http://localhost:8080/api/v1'):
        self.base_url = base_url
        self.token = None
        self.session = requests.Session()
    
    def login(self, username, password):
        res = self.session.post(
            f'{self.base_url}/auth/login',
            json={'username': username, 'password': password}
        )
        data = res.json()
        if data['code'] == 0:
            self.token = data['data']['token']
            self.session.headers.update({
                'Authorization': f'Bearer {self.token}'
            })
        return data
    
    def get_volumes(self):
        res = self.session.get(f'{self.base_url}/volumes')
        return res.json()
    
    def create_volume(self, name, devices, profile='single'):
        res = self.session.post(
            f'{self.base_url}/volumes',
            json={'name': name, 'devices': devices, 'profile': profile}
        )
        return res.json()
    
    def create_smb_share(self, name, path, **options):
        res = self.session.post(
            f'{self.base_url}/shares/smb',
            json={'name': name, 'path': path, **options}
        )
        return res.json()

# 使用示例
client = NasOSClient()
client.login('admin', 'admin123')
volumes = client.get_volumes()
print('Volumes:', volumes)
```

### Go 示例

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type NasOSClient struct {
    BaseURL string
    Token   string
    Client  *http.Client
}

type LoginRequest struct {
    Username string `json:"username"`
    Password string `json:"password"`
}

type LoginResponse struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Data    struct {
        Token string `json:"token"`
    } `json:"data"`
}

func NewClient(baseURL string) *NasOSClient {
    return &NasOSClient{
        BaseURL: baseURL,
        Client:  &http.Client{},
    }
}

func (c *NasOSClient) Login(username, password string) error {
    reqBody := LoginRequest{Username: username, Password: password}
    body, _ := json.Marshal(reqBody)
    
    res, err := c.Client.Post(
        c.BaseURL+"/auth/login",
        "application/json",
        bytes.NewBuffer(body),
    )
    if err != nil {
        return err
    }
    
    var resp LoginResponse
    json.NewDecoder(res.Body).Decode(&resp)
    
    if resp.Code == 0 {
        c.Token = resp.Data.Token
    }
    
    return nil
}

func (c *NasOSClient) GetVolumes() (interface{}, error) {
    req, _ := http.NewRequest("GET", c.BaseURL+"/volumes", nil)
    req.Header.Set("Authorization", "Bearer "+c.Token)
    
    res, err := c.Client.Do(req)
    if err != nil {
        return nil, err
    }
    
    var result interface{}
    json.NewDecoder(res.Body).Decode(&result)
    
    return result, nil
}

// 使用示例
func main() {
    client := NewClient("http://localhost:8080/api/v1")
    client.Login("admin", "admin123")
    
    volumes, _ := client.GetVolumes()
    fmt.Printf("Volumes: %+v\n", volumes)
}
```

---

*API 参考文档版本：v1.0.0 | 最后更新：2026-03-11*
