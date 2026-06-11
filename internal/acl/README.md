# 企业级ACL权限系统

## 概述

本系统为NAS-OS提供了企业级的访问控制列表（ACL）管理功能，支持13种细分权限和复杂的权限继承机制。

## 特性

### 13种细分权限

| 权限 | 标识符 | 说明 |
|------|--------|------|
| 读取 | `read` | 读取文件/目录内容 |
| 写入 | `write` | 写入/修改文件内容 |
| 删除 | `delete` | 删除文件/目录 |
| 执行 | `execute` | 执行文件 |
| 创建 | `create` | 创建新文件/目录 |
| 重命名 | `rename` | 重命名文件/目录 |
| 移动 | `move` | 移动文件/目录 |
| 复制 | `copy` | 复制文件/目录 |
| 查看属性 | `view_attr` | 查看文件/目录属性 |
| 修改属性 | `modify_attr` | 修改文件/目录属性 |
| 更改权限 | `change_perm` | 修改ACL权限 |
| 获取所有权 | `take_owner` | 获取文件/目录所有权 |
| 遍历文件夹 | `traverse` | 遍历目录结构 |

### 权限组

系统预定义了4个权限组：

- **ReadOnly**: 只读访问（read, view_attr, traverse）
- **ReadWrite**: 读写访问（read, write, create, rename, copy, view_attr, traverse）
- **Modify**: 修改访问（包含ReadWrite + delete, move）
- **FullControl**: 完全控制（所有13种权限）

### 权限继承

支持5种继承模式：

- **full**: 完全继承到所有子项
- **selective**: 选择性继承（仅继承指定的权限）
- **none**: 不继承（中断继承链）
- **container**: 仅继承到容器（目录）
- **object**: 仅继承到对象（文件）

## API接口

### ACL管理

```
GET    /api/v1/acl                    # 列出所有ACL
POST   /api/v1/acl                    # 创建新ACL
GET    /api/v1/acl/path/{path}        # 获取指定路径的ACL
PUT    /api/v1/acl/path/{path}        # 更新ACL
DELETE /api/v1/acl/path/{path}        # 删除ACL
```

### ACE管理

```
POST   /api/v1/acl/ace?path={path}           # 添加ACE
PUT    /api/v1/acl/ace?path={path}&aceId={id} # 更新ACE
DELETE /api/v1/acl/ace?path={path}&aceId={id} # 删除ACE
```

### 权限检查

```
POST   /api/v1/acl/check              # 检查访问权限
GET    /api/v1/acl/effective?subject={user}&path={path}  # 获取有效权限
```

### 继承管理

```
POST   /api/v1/acl/propagate?path={path}  # 传播继承
```

### 所有者/组管理

```
PUT    /api/v1/acl/owner?path={path}  # 设置所有者
PUT    /api/v1/acl/group?path={path}  # 设置组
```

### 辅助接口

```
GET    /api/v1/acl/groups             # 获取权限组
GET    /api/v1/acl/audit?limit=100    # 获取审计日志
```

## 使用示例

### 创建ACL

```bash
curl -X POST http://localhost:8080/api/v1/acl \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/data/documents",
    "entry_type": "directory",
    "owner": "admin",
    "group": "users",
    "inherit_enabled": true,
    "inherit_permissions": true
  }'
```

### 添加ACE

```bash
curl -X POST "http://localhost:8080/api/v1/acl/ace?path=/data/documents" \
  -H "Content-Type: application/json" \
  -d '{
    "subject": "john",
    "subject_type": "user",
    "permissions": ["read", "write", "create"],
    "allowed": true,
    "inherit_flags": ["full"]
  }'
```

### 检查访问权限

```bash
curl -X POST http://localhost:8080/api/v1/acl/check \
  -H "Content-Type: application/json" \
  -d '{
    "subject": "john",
    "path": "/data/documents/report.txt",
    "permission": "read"
  }'
```

### 获取有效权限

```bash
curl "http://localhost:8080/api/v1/acl/effective?subject=john&path=/data/documents"
```

## 前端界面

访问 `/web/acl/index.html` 使用图形化ACL编辑器界面。

功能包括：
- 文件系统路径浏览
- ACL和ACE的可视化管理
- 实时权限检查工具
- 审计日志查看

## 测试

运行单元测试：

```bash
go test ./internal/acl/ -v
```

## 数据结构

### ACL

```json
{
  "path": "/data/documents",
  "entry_type": "directory",
  "owner": "admin",
  "group": "users",
  "aces": [...],
  "inherit_enabled": true,
  "inherit_permissions": true,
  "protected": false,
  "created_at": "2026-06-11T09:00:00Z",
  "updated_at": "2026-06-11T09:00:00Z"
}
```

### ACE

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "subject": "john",
  "subject_type": "user",
  "permissions": ["read", "write", "create"],
  "access_type": "explicit",
  "allowed": true,
  "applies_to": "directory",
  "inherit_flags": ["full"],
  "effective_from": ""
}
```

## 权限评估逻辑

1. **显式优先**: 显式设置的权限优先于继承的权限
2. **拒绝优先**: 显式拒绝的权限优先于允许的权限
3. **路径匹配**: 从请求的路径向上查找所有匹配的ACE
4. **继承传播**: 根据继承标志决定权限如何传播到子路径

## 安全建议

1. 遵循最小权限原则
2. 使用权限组简化管理
3. 定期审查审计日志
4. 保护关键路径的ACL不被意外修改
5. 使用继承机制减少重复配置
