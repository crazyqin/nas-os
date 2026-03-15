# 用户权限系统指南

**版本**: v2.61.0  
**更新日期**: 2026-03-15

---

## 概述

NAS-OS 采用基于角色的访问控制 (RBAC) 系统，实现最小权限原则，支持用户组权限继承。本指南帮助您理解和管理系统权限。

---

## 角色体系

### 四级角色

NAS-OS 定义了四级角色，权限从高到低：

| 角色 | 说明 | 优先级 |
|------|------|--------|
| **admin** | 系统管理员，拥有完全控制权限 | 100 |
| **operator** | 运维员，可操作系统但不能管理用户 | 75 |
| **readonly** | 只读用户，只能查看系统状态 | 50 |
| **guest** | 访客用户，最小权限 | 25 |

### 角色权限详情

#### Admin (管理员)

```yaml
角色: admin
描述: 系统管理员，拥有完全控制权限
权限:
  - "*:*"  # 所有权限
```

**适用场景**:
- 系统主管理员
- 需要完全控制权限的用户

#### Operator (运维员)

```yaml
角色: operator
描述: 运维员，可以操作系统但不能管理用户
权限:
  - system:read, system:write    # 系统管理
  - storage:read, storage:write  # 存储管理
  - share:read, share:write      # 共享管理
  - network:read, network:write  # 网络管理
  - service:read, service:write  # 服务管理
  - backup:read, backup:write    # 备份管理
  - log:read                     # 日志查看
  - monitor:read                 # 监控查看
```

**适用场景**:
- 日常运维人员
- 负责系统监控和维护的用户

#### ReadOnly (只读用户)

```yaml
角色: readonly
描述: 只读用户，只能查看系统状态
权限:
  - system:read     # 查看系统信息
  - storage:read    # 查看存储状态
  - share:read      # 查看共享列表
  - network:read    # 查看网络配置
  - service:read    # 查看服务状态
  - log:read        # 查看日志
  - monitor:read    # 查看监控
```

**适用场景**:
- 需要了解系统状态的非管理员
- 监控和审计人员

#### Guest (访客)

```yaml
角色: guest
描述: 访客用户，最小权限
权限:
  - system:read  # 仅查看基本信息
```

**适用场景**:
- 临时访客
- 演示账户

---

## 权限模型

### 权限格式

权限采用 `资源:操作` 格式：

```
resource:action
```

- **resource**: 资源类型，如 `system`、`storage`、`user`
- **action**: 操作类型，如 `read`、`write`、`admin`

### 权限层级

权限支持层级依赖关系：

```
system:admin → system:write → system:read
user:admin → user:write → user:read
storage:admin → storage:write → storage:read
```

**示例**: 授予 `storage:write` 时，自动隐含 `storage:read` 权限。

### 通配符权限

支持通配符格式：

| 格式 | 说明 |
|------|------|
| `*:*` | 所有资源的所有权限 |
| `storage:*` | 存储资源的所有操作 |
| `*:read` | 所有资源的读取权限 |

---

## 预定义权限

### 系统管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `system:read` | 查看系统信息 | - |
| `system:write` | 修改系统配置 | system:read |
| `system:admin` | 系统管理操作 | system:write |

### 用户管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `user:read` | 查看用户信息 | - |
| `user:write` | 创建/修改用户 | user:read |
| `user:admin` | 删除用户/修改角色 | user:write |

### 存储管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `storage:read` | 查看存储状态 | - |
| `storage:write` | 管理存储池/卷 | storage:read |
| `storage:admin` | 删除存储/格式化 | storage:write |

### 共享管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `share:read` | 查看共享列表 | - |
| `share:write` | 创建/修改共享 | share:read |
| `share:admin` | 删除共享/权限管理 | share:write |

### 网络管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `network:read` | 查看网络配置 | - |
| `network:write` | 修改网络配置 | network:read |

### 服务管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `service:read` | 查看服务状态 | - |
| `service:write` | 启动/停止服务 | service:read |

### 备份管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `backup:read` | 查看备份任务 | - |
| `backup:write` | 创建/执行备份 | backup:read |

### 日志查看

| 权限 | 说明 |
|------|------|
| `log:read` | 查看系统日志 |

### 监控

| 权限 | 说明 |
|------|------|
| `monitor:read` | 查看系统监控 |

### 审计

| 权限 | 说明 |
|------|------|
| `audit:read` | 查看审计日志 |
| `audit:admin` | 管理审计配置 |

### 快照管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `snapshot:read` | 查看快照 | - |
| `snapshot:write` | 创建/删除快照 | snapshot:read |

### 权限管理

| 权限 | 说明 | 依赖 |
|------|------|------|
| `permission:read` | 查看权限配置 | - |
| `permission:write` | 修改权限配置 | permission:read |

---

## 用户组权限继承

### 概述

用户可以加入用户组，继承用户组的权限。这简化了权限管理，适合团队协作场景。

### 用户组结构

```yaml
用户组:
  组ID: developers
  组名: 开发组
  权限:
    - storage:read
    - storage:write
    - share:read
  成员:
    - user1 (组管理员)
    - user2
    - user3
```

### 权限计算规则

用户最终权限 = 角色权限 ∪ 直接权限 ∪ 继承权限

**优先级**: 
1. 显式拒绝策略 (最高优先级)
2. 显式允许策略
3. 角色权限
4. 直接权限
5. 继承权限

### 使用示例

```bash
# 创建用户组权限
curl -X POST http://localhost:8080/api/v1/rbac/groups \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "group_id": "developers",
    "group_name": "开发组",
    "permissions": ["storage:read", "storage:write", "share:read"]
  }'

# 将用户添加到组
curl -X POST http://localhost:8080/api/v1/rbac/users/user1/groups/developers \
  -H "Authorization: Bearer $TOKEN"
```

---

## 策略管理

### 策略类型

支持两种策略效果：

- **allow**: 显式允许访问
- **deny**: 显式拒绝访问

### 策略结构

```yaml
策略:
  ID: policy-001
  名称: 禁止访客修改网络
  描述: 防止访客用户修改网络配置
  效果: deny
  主体:
    - "group:guests"
  资源:
    - "network"
  操作:
    - "write"
  优先级: 100
  启用: true
```

### 策略条件

支持多种条件类型：

| 条件类型 | 说明 | 示例 |
|----------|------|------|
| time | 时间限制 | 仅工作时间允许 |
| ip | IP 限制 | 仅特定 IP 允许 |
| resource | 资源限制 | 仅特定资源允许 |

**示例**:

```yaml
条件:
  - 类型: time
    键: hour
    操作符: in
    值: ["9-18"]  # 9:00-18:00
  - 类型: ip
    键: source_ip
    操作符: in
    值: ["192.168.1.0/24"]
```

### 使用示例

```bash
# 创建拒绝策略
curl -X POST http://localhost:8080/api/v1/rbac/policies \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "禁止访客修改网络",
    "description": "防止访客用户修改网络配置",
    "effect": "deny",
    "principals": ["group:guests"],
    "resources": ["network"],
    "actions": ["write"],
    "priority": 100
  }'
```

---

## 权限缓存

### 缓存机制

为提升性能，系统缓存用户权限：

- **缓存时间**: 5 分钟 (可配置)
- **自动刷新**: 权限变更时自动失效
- **手动清除**: 支持手动清除缓存

### 配置选项

```yaml
rbac:
  cache_enabled: true
  cache_ttl: 5m
  strict_mode: true     # 严格模式：默认拒绝
  audit_enabled: true   # 记录权限检查日志
```

---

## 审计日志

### 权限审计

每次权限检查都会记录审计日志：

```json
{
  "timestamp": "2026-03-15T10:30:00Z",
  "user_id": "user1",
  "resource": "storage",
  "action": "write",
  "result": {
    "allowed": true,
    "reason": "角色权限匹配",
    "matched_by": "operator"
  }
}
```

### 查看审计日志

```bash
# 获取权限审计日志
curl -X GET "http://localhost:8080/api/v1/audit/logs?module=rbac" \
  -H "Authorization: Bearer $TOKEN"
```

---

## 最佳实践

### 1. 最小权限原则

只授予用户完成任务所需的最小权限：

```bash
# ✅ 推荐：只授予需要的权限
curl -X POST .../permissions -d '{"permission": "storage:read"}'

# ❌ 避免：授予过大权限
curl -X POST .../permissions -d '{"permission": "*:*"}'
```

### 2. 使用角色而非直接权限

优先使用角色分配权限，便于统一管理：

```bash
# ✅ 推荐：使用角色
curl -X PUT .../users/user1/role -d '{"role": "operator"}'

# ❌ 避免：逐个授予权限
curl -X POST .../permissions -d '{"permission": "storage:read"}'
curl -X POST .../permissions -d '{"permission": "share:read"}'
# ...
```

### 3. 使用用户组管理团队权限

为团队创建用户组，统一管理：

```bash
# 创建开发组
curl -X POST .../groups -d '{
  "group_id": "dev-team",
  "permissions": ["storage:read", "storage:write"]
}'

# 添加成员
curl -X POST .../users/developer1/groups/dev-team
```

### 4. 定期审计权限

定期检查用户权限，清理不必要的权限：

```bash
# 列出所有用户权限
curl -X GET http://localhost:8080/api/v1/rbac/users

# 撤销不需要的权限
curl -X DELETE .../users/user1/permissions/storage:write
```

### 5. 使用策略细化控制

对于特殊需求，使用策略进行细粒度控制：

```bash
# 创建时间限制策略
curl -X POST .../policies -d '{
  "name": "工作时间限制",
  "effect": "deny",
  "principals": ["group:operators"],
  "resources": ["system"],
  "actions": ["write"],
  "conditions": [
    {"type": "time", "key": "hour", "operator": "not_in", "values": ["9-18"]}
  ]
}'
```

---

## 故障排查

### 权限检查失败

**症状**: 用户无法访问资源

**排查步骤**:

1. 检查用户角色
```bash
curl -X GET http://localhost:8080/api/v1/rbac/users/user1
```

2. 检查用户有效权限
```bash
curl -X GET http://localhost:8080/api/v1/rbac/users/user1/permissions
```

3. 检查是否有拒绝策略
```bash
curl -X GET http://localhost:8080/api/v1/rbac/policies
```

4. 查看审计日志
```bash
curl -X GET "http://localhost:8080/api/v1/audit/logs?user_id=user1"
```

### 权限缓存问题

**症状**: 权限修改后未生效

**解决方案**:

```bash
# 清除用户权限缓存
curl -X DELETE http://localhost:8080/api/v1/rbac/cache/user1

# 清除所有缓存
curl -X DELETE http://localhost:8080/api/v1/rbac/cache
```

---

## 相关文档

- [API 文档 - 权限管理](../api/permission-api.md)
- [用户管理指南](./audit-guide.md)
- [安全配置指南](../SECURITY.md)

---

## 更新历史

| 版本 | 日期 | 变更 |
|------|------|------|
| v2.61.0 | 2026-03-15 | 文档版本同步 |