# NAS-OS v2.56.0 安全审计报告

**审计日期**: 2026-03-15  
**审计部门**: 刑部  
**审计版本**: v2.56.0  
**审计工具**: gosec v2, golangci-lint v2.11.3

---

## 一、执行摘要

本次审计发现 **1552 个安全问题**，主要集中在整数溢出风险（G115）。代码质量检查通过，但测试覆盖率偏低（平均约 36.6%），存在 2 个构建失败模块。

### 风险等级分布

| 风险等级 | 数量 | 主要问题类型 |
|---------|------|-------------|
| HIGH    | 1552 | 整数溢出 (G115) |
| MEDIUM  | 0    | - |
| LOW     | 0    | - |

---

## 二、安全检查结果

### 2.1 gosec 静态分析

#### 高危问题：整数溢出 (G115/CWE-190)

**问题描述**: 大量 uint64/int64/int 类型转换未进行边界检查，可能导致整数溢出。

**受影响文件**:

| 文件 | 行号 | 问题类型 |
|-----|------|---------|
| internal/system/monitor.go | 760-761 | uint64 → int64 |
| internal/snapshot/adapter.go | 42 | uint64 → int64 |
| internal/quota/optimizer/optimizer.go | 516, 934 | uint64 → int64 |
| internal/optimizer/optimizer.go | 168, 170 | uint64 → int64 |
| internal/backup/smart_manager_v2.go | 709-710 | int64/uint64 互转 |
| internal/search/engine.go | 539 | uint64 → int |
| internal/monitor/disk_health.go | 378, 450 | uint64 → int |
| internal/disk/smart_monitor.go | 714, 1106 | uint64 → int |
| internal/storage/distributed_storage.go | 1229, 1280 | rune → uint32 |
| internal/ldap/client.go | 317 | rune → byte |
| internal/vm/snapshot.go | 188 | int64 → uint64 |
| internal/vm/iso.go | 80, 214 | int64 → uint64 |
| internal/vm/handlers.go | 469-470 | int → uint64 |
| internal/security/v2/mfa.go | 174 | int64 → uint64 |
| internal/quota/cleanup.go | 309, 312, 360, 445, 487 | int64 → uint64 |
| internal/quota/alert_enhanced.go | 411, 647 | int64 → uint64 |
| internal/photos/manager.go | 256 | int64 → uint64 |
| internal/photos/handlers.go | 189, 275, 536, 1376-1378 | int64 → uint64 |
| internal/performance/collector.go | 637-638 | int64 → uint64 |
| internal/perf/manager.go | 405 | int → uint64 |
| internal/health/health.go | 453-454 | int64 → uint64 |
| internal/container/volume.go | 362, 418-419 | int64 → uint64 |
| internal/reports/datasource.go | 116, 119, 331, 334, 337 | uint64 → int |

**修复建议**:

```go
// 不安全写法
netRX += int64(n.RXSpeed)

// 安全写法
if n.RXSpeed > math.MaxInt64 {
    return errors.New("value overflow")
}
netRX += int64(n.RXSpeed)
```

或使用 `math/bits` 包进行安全转换。

---

### 2.2 API 端点权限控制检查

#### 发现问题：多个 API 端点缺少权限中间件

**严重程度**: HIGH

**问题描述**: 以下 API 模块的 `RegisterRoutes` 方法未添加任何认证/授权中间件：

| 模块 | 文件 | 风险 |
|-----|------|-----|
| 容器管理 | api/container_handlers.go | Docker 容器完全控制 |
| 备份管理 | internal/backup/handlers.go | 备份/恢复/删除操作 |
| 快照管理 | internal/snapshot/handlers.go | 快照策略完全控制 |
| 报表管理 | internal/reports/handlers.go | 数据导出/查询 |
| WebDAV | internal/webdav/handlers.go | 文件访问 |
| 性能监控 | internal/performance/api_handlers.go | 系统信息暴露 |
| 预测分析 | internal/prediction/handlers.go | 数据分析接口 |

**示例问题代码** (`api/container_handlers.go`):

```go
func (h *ContainerHandlers) RegisterRoutes(r *gin.RouterGroup) {
    api := r.Group("/api/v1")
    {
        // 缺少权限中间件！
        api.GET("/containers", h.listContainers)
        api.POST("/containers", h.createContainer)
        api.DELETE("/containers/:id", h.removeContainer)
        // ...
    }
}
```

**修复建议**:

```go
func (h *ContainerHandlers) RegisterRoutes(r *gin.RouterGroup, auth *auth.AuthMiddleware) {
    api := r.Group("/api/v1")
    api.Use(auth.RequireAuth())  // 添加认证
    {
        // 读操作 - 需要 operator 及以上权限
        api.GET("/containers", auth.RequireRole(auth.RoleOperator), h.listContainers)
        
        // 写操作 - 需要 admin 权限
        api.POST("/containers", auth.RequireAdmin(), h.createContainer)
        api.DELETE("/containers/:id", auth.RequireAdmin(), h.removeContainer)
    }
}
```

---

### 2.3 敏感数据处理检查

#### 发现问题：敏感信息可能泄露到日志

**检查结果**: 未发现密码/密钥直接打印到日志的问题，但发现以下风险点：

| 文件 | 行号 | 风险描述 |
|-----|------|---------|
| internal/cloudsync/providers.go | 680-702 | OAuth token 错误信息可能包含敏感数据 |
| internal/auth/oauth2.go | 209 | token exchange 失败时返回完整 body |
| internal/network/ddns.go | 207 | DuckDNS token 暴露在 URL 中 |

**修复建议**:
- 在日志中脱敏 token/密码
- 使用 `zap.String("token", maskToken(token))` 格式

---

### 2.4 命令注入风险检查

**检查结果**: 所有 `exec.Command` 调用均使用参数分离方式，未发现命令注入漏洞。

**安全实践示例** (`internal/backup/manager.go`):

```go
// 安全：参数分离
cmd := exec.Command("tar", "czf", backupPath, "-C", cfg.Source, ".")
```

**需关注的代码**:

| 文件 | 行号 | 说明 |
|-----|------|-----|
| internal/backup/manager.go | 366 | openssl 密钥通过命令行传递 |
| internal/snapshot/executor.go | 89 | 使用 `sh -c` 执行脚本 |

**建议**:
- openssl 密钥改用环境变量或文件传递
- 脚本执行需要严格的输入验证

---

## 三、代码质量检查结果

### 3.1 golangci-lint 检查

**结果**: ✅ 通过（0 issues）

### 3.2 测试覆盖率

**整体覆盖率**: 36.6%

#### 覆盖率低于 20% 的模块

| 模块 | 覆盖率 | 建议 |
|-----|-------|-----|
| internal/security/v2 | 18.5% | 补充安全模块测试 |
| internal/security | 8.4% | 补充基础测试 |
| internal/sftp | 4.1% | 添加核心功能测试 |
| internal/storage | 4.3% | 添加存储操作测试 |
| internal/snapshot | 7.9% | 补充快照功能测试 |
| internal/websocket | 29.1% | 补充 WebSocket 测试 |
| internal/webdav | 26.7% | 补充 WebDAV 测试 |

#### 测试失败

| 模块 | 错误类型 |
|-----|---------|
| internal/rbac | 测试失败 |
| internal/reports | 构建失败 |
| internal/web | 构建失败 |

---

## 四、修复优先级建议

### P0 - 紧急（立即修复）

1. **API 权限控制缺失** - 所有未保护的 API 端点需要添加认证中间件

### P1 - 高优先级（一周内）

1. **整数溢出风险** - 对关键路径的类型转换添加边界检查
2. **构建失败修复** - 修复 reports/web 模块的构建错误
3. **测试失败修复** - 修复 rbac 模块测试

### P2 - 中优先级（两周内）

1. **测试覆盖率提升** - 为安全模块和存储模块补充测试
2. **敏感信息脱敏** - 日志中的 token/password 脱敏处理

### P3 - 低优先级（持续改进）

1. 代码审查其他静态分析警告
2. 补充集成测试

---

## 五、总结

本次审计发现的主要问题：

1. **权限控制缺失** - 多个核心 API 端点未实现认证授权
2. **整数溢出风险** - 1552 处类型转换未进行边界检查
3. **测试覆盖不足** - 平均覆盖率 36.6%，多个模块低于 10%
4. **构建问题** - 2 个模块构建失败

建议优先处理 P0 级别的权限控制问题，确保系统安全。

---

**审计人**: 刑部  
**审计时间**: 2026-03-15 16:53 GMT+8