# 权限模板使用指南

> **版本**: v2.484.0 | **更新日期**: 2026-05-05 | **适用模块**: Permission Templates

## 概述

权限模板系统提供预定义的权限集合，帮助管理员快速为新用户分配合适的权限。系统内置5个常用模板，支持自定义模板创建和管理。

## 为什么需要权限模板？

传统用户管理存在以下痛点：

- **配置繁琐**: 每个用户需要手动设置权限、配额、共享访问等十多项参数
- **容易出错**: 手动配置遗漏或错误导致安全隐患
- **标准不一**: 不同管理员配置风格不同，难以统一
- **效率低下**: 创建10个相同角色用户需要重复10次操作

权限模板通过**一键应用**解决以上问题。

---

## 内置模板

系统内置5个常用模板，覆盖常见使用场景：

### 🏠 家庭用户 (family)

适合家庭成员日常使用：

| 权限项 | 配置 |
|--------|------|
| 存储配额 | 1 TB |
| 文件数量上限 | 100,000 |
| 带宽限制 | 50 MB/s |
| 并发会话 | 3 |
| SMB访问 | ✅ 读写 |
| NFS访问 | ❌ |
| WebDAV | ✅ 只读 |
| 照片管理 | ✅ |
| 文件同步 | ✅ |
| Docker应用 | ❌ |
| 系统管理 | ❌ |

### 💼 办公用户 (office)

适合企业办公场景：

| 权限项 | 配置 |
|--------|------|
| 存储配额 | 500 GB |
| 文件数量上限 | 200,000 |
| 带宽限制 | 100 MB/s |
| 并发会话 | 5 |
| SMB访问 | ✅ 读写 |
| NFS访问 | ✅ 只读 |
| WebDAV | ✅ 读写 |
| 照片管理 | ❌ |
| 文件同步 | ✅ |
| Docker应用 | ❌ |
| 系统管理 | ❌ |

### 🎬 媒体用户 (media)

适合影音媒体管理：

| 权限项 | 配置 |
|--------|------|
| 存储配额 | 2 TB |
| 文件数量上限 | 50,000 |
| 带宽限制 | 200 MB/s |
| 并发会话 | 10 |
| SMB访问 | ✅ 读写 |
| NFS访问 | ✅ 读写 |
| WebDAV | ✅ 读写 |
| 照片管理 | ✅ |
| 文件同步 | ✅ |
| Docker应用 | ✅ 有限 |
| 系统管理 | ❌ |

### 🔧 开发者 (developer)

适合开发和技术人员：

| 权限项 | 配置 |
|--------|------|
| 存储配额 | 1 TB |
| 文件数量上限 | 500,000 |
| 带宽限制 | 无限制 |
| 并发会话 | 10 |
| SMB访问 | ✅ 读写 |
| NFS访问 | ✅ 读写 |
| WebDAV | ✅ 读写 |
| 照片管理 | ❌ |
| 文件同步 | ✅ |
| Docker应用 | ✅ 完全 |
| Git仓库 | ✅ |
| SSH访问 | ✅ 有限 |
| 系统管理 | ❌ |

### 👑 管理员 (admin)

拥有完整系统管理权限：

| 权限项 | 配置 |
|--------|------|
| 存储配额 | 无限制 |
| 文件数量上限 | 无限制 |
| 带宽限制 | 无限制 |
| 并发会话 | 无限制 |
| SMB访问 | ✅ 读写 |
| NFS访问 | ✅ 读写 |
| WebDAV | ✅ 读写 |
| 照片管理 | ✅ |
| 文件同步 | ✅ |
| Docker应用 | ✅ 完全 |
| Git仓库 | ✅ |
| SSH访问 | ✅ 完全 |
| 系统管理 | ✅ 完全 |

---

## 快速开始

### 1. 查看所有模板

```bash
curl http://localhost:8080/api/v1/perm-templates/

# 响应示例
{
  "templates": [
    {
      "id": "family",
      "name": "家庭用户",
      "builtin": true,
      "description": "适合家庭成员日常使用",
      "storage_quota_gb": 1000,
      "bandwidth_limit_mbps": 50,
      "max_sessions": 3,
      "created_at": "2026-05-01T00:00:00Z"
    }
    // ... 其他模板
  ]
}
```

### 2. 查看模板详情

```bash
curl http://localhost:8080/api/v1/perm-templates/office

# 响应示例
{
  "id": "office",
  "name": "办公用户",
  "builtin": true,
  "description": "适合企业办公场景",
  "storage_quota_gb": 500,
  "max_files": 200000,
  "bandwidth_limit_mbps": 100,
  "max_sessions": 5,
  "permissions": {
    "smb": "readwrite",
    "nfs": "readonly",
    "webdav": "readwrite",
    "photos": false,
    "sync": true,
    "docker": false,
    "git": false,
    "ssh": false,
    "admin": false
  },
  "created_at": "2026-05-01T00:00:00Z",
  "updated_at": "2026-05-01T00:00:00Z"
}
```

### 3. 应用模板创建用户

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{
    "username": "zhangsan",
    "display_name": "张三",
    "password": "secure_password",
    "template_id": "family"
  }'
```

### 4. 为现有用户应用模板

```bash
curl -X POST http://localhost:8080/api/v1/users/zhangsan/apply-template \
  -H 'Content-Type: application/json' \
  -d '{
    "template_id": "office",
    "merge_mode": "override"
  }'
```

| `merge_mode` | 说明 |
|---------------|------|
| `override` | 完全覆盖现有权限（推荐） |
| `merge` | 仅更新模板中定义的权限，保留其他 |
| `preview` | 预览变更，不实际应用 |

---

## 自定义模板

### 创建自定义模板

```bash
curl -X POST http://localhost:8080/api/v1/perm-templates/ \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "contractor",
    "name": "外包人员",
    "description": "外包合同人员，受限访问",
    "storage_quota_gb": 100,
    "max_files": 10000,
    "bandwidth_limit_mbps": 20,
    "max_sessions": 2,
    "permissions": {
      "smb": "readonly",
      "nfs": false,
      "webdav": "readonly",
      "photos": false,
      "sync": true,
      "docker": false,
      "git": false,
      "ssh": false,
      "admin": false
    }
  }'
```

### 编辑自定义模板

```bash
curl -X PUT http://localhost:8080/api/v1/perm-templates/contractor \
  -H 'Content-Type: application/json' \
  -d '{
    "storage_quota_gb": 200,
    "bandwidth_limit_mbps": 50,
    "description": "外包合同人员，配额提升"
  }'
```

### 删除自定义模板

```bash
curl -X DELETE http://localhost:8080/api/v1/perm-templates/contractor
```

> ⚠️ **注意**: 内置模板（family/office/media/developer/admin）不可删除或修改。

---

## 模板应用记录

系统记录所有模板应用操作，便于审计追踪：

```bash
curl http://localhost:8080/api/v1/perm-templates/records?limit=20

# 响应示例
{
  "records": [
    {
      "id": "rec_001",
      "user": "zhangsan",
      "template_id": "office",
      "applied_by": "admin",
      "applied_at": "2026-05-05T10:30:00Z",
      "merge_mode": "override",
      "changes": {
        "storage_quota": {"from": 1000, "to": 500},
        "smb_access": {"from": "readwrite", "to": "readwrite"},
        "docker_access": {"from": false, "to": false}
      }
    }
  ]
}
```

---

## 最佳实践

### 1. 根据角色选择模板

```
新用户 → 分析角色 → 选择模板 → 应用 → 微调特殊权限
```

### 2. 定期审查模板

建议每季度审查一次模板配置，确保：
- 配额设置符合当前存储容量
- 权限范围符合安全策略
- 新功能是否需要更新模板

### 3. 最小权限原则

- 优先选择权限最小的模板
- 需要时再逐步提升权限
- 避免直接使用管理员模板

### 4. 批量操作

```bash
# 批量为多个用户应用模板
curl -X POST http://localhost:8080/api/v1/users/batch-apply-template \
  -H 'Content-Type: application/json' \
  -d '{
    "usernames": ["user1", "user2", "user3"],
    "template_id": "office",
    "merge_mode": "override"
  }'
```

---

## 对标竞品

| 功能 | NAS-OS | TrueNAS 26 | 群晖DSM | 飞牛fnOS |
|------|:------:|:----------:|:-------:|:--------:|
| 内置权限模板 | ✅ 5个 | ❌ | ⚠️ 有限 | ❌ |
| **自定义模板** | ✅ | ❌ | ❌ | ❌ |
| **模板应用记录** | ✅ | ❌ | ❌ | ❌ |
| **内置模板保护** | ✅ | ❌ | ❌ | ❌ |
| 批量应用 | ✅ | ❌ | ❌ | ❌ |
| 权限预览 | ✅ | ❌ | ❌ | ❌ |

**NAS-OS优势**: 群晖仅有基础的权限分组，TrueNAS和fnOS均无模板系统。NAS-OS的模板系统大幅简化用户管理流程。

---

## 相关文档

- [用户管理指南](user-management-guide.md)
- [RBAC权限指南](rbac-guide.md)
- [MFA多因素认证指南](mfa-guide.md)
