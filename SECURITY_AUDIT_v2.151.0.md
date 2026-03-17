# NAS-OS Security Audit Report

**Version:** v2.151.0  
**Date:** 2026-03-17  
**Tool:** gosec v2  

---

## Summary

| Severity | Count |
|----------|-------|
| **HIGH** | 150 |
| **MEDIUM** | 772 |
| **LOW** | 691 |
| **TOTAL** | **1613** |

---

## Critical Issues by Category

### 1. G115 - Integer Overflow (HIGH, 75 issues)
整数溢出转换问题，主要涉及 `uint64`/`int64` 转换。

**影响文件：**
- `internal/quota/optimizer/optimizer.go`
- `internal/optimizer/optimizer.go`
- `internal/monitor/metrics_collector.go`
- `internal/disk/smart_monitor.go`
- `internal/storage/distributed_storage.go`

**风险等级：** MEDIUM（在 NAS 系统中，磁盘空间计算溢出可能导致显示错误）

---

### 2. G703 - Path Traversal (HIGH, 46 issues)
路径穿越漏洞，可能导致未授权文件访问。

**影响文件：**
- `internal/webdav/server.go` (最严重，多处)
- `internal/vm/snapshot.go`
- `internal/backup/manager.go`
- `internal/backup/encrypt.go`
- `internal/security/v2/encryption.go`

**风险等级：** HIGH（WebDAV 服务暴露在网络中，需优先修复）

---

### 3. G118 - Context Leak (HIGH, 11 issues)
Context 取消函数未被调用，可能导致 goroutine 泄漏。

**重点关注：`internal/scheduler/executor.go`**

```go
// Line 108: 创建了 cancel 但不保证被调用
execCtx, cancel := context.WithCancel(ctx)

// cleanup 函数只删除 map 记录，不调用 cancel
func (e *Executor) cleanup(taskID string) {
    e.mu.Lock()
    defer e.mu.Unlock()
    delete(e.running, taskID)  // 缺少 cancel() 调用
}
```

**状态：⚠️ 未修复**  
`cancel` 只在用户手动取消或 ExecuteSync 时调用，但 Execute 的 goroutine 完成后不会调用 cancel。

**其他 Context 泄漏位置：**
- `internal/media/transcoder.go:314`
- `internal/media/streaming.go:115, 485, 597`
- `internal/compress/service.go:192`
- `internal/backup/smart_manager_v2.go:461`
- `internal/backup/manager.go:318, 561`

---

### 4. G702 - Command Injection (HIGH, 10 issues)
命令注入风险，主要通过 virsh/qemu-img 执行。

**影响文件：**
- `internal/vm/manager.go` - virsh 命令
- `internal/vm/snapshot.go` - qemu-img 命令

**状态：** 已有 #nosec 注释，变量来自内部验证，风险可控

---

### 5. G101 - Hardcoded Credentials (HIGH, 7 issues)
潜在的硬编码凭证。

**影响文件：**
- `internal/auth/oauth2.go` - OAuth2 配置函数（误报，函数参数）
- `internal/cloudsync/providers.go` - OAuth URL（误报）
- `internal/office/types.go` - 错误消息字符串（误报）

**状态：** 大部分为误报

---

### 6. G122 - TOCTOU (HIGH, 5 issues)
Time-of-check to time-of-use 竞态条件。

**影响文件：**
- `internal/snapshot/replication.go`
- `internal/plugin/hotreload.go`
- `internal/files/manager.go`
- `internal/backup/manager.go`
- `internal/backup/encrypt.go`

---

### 7. G402 - TLS InsecureSkipVerify (HIGH, 3 issues)
TLS 跳过验证。

**影响文件：**
- `internal/ldap/client.go` (2 处)
- `internal/auth/ldap.go` (1 处)

**建议：** 仅在用户明确配置跳过验证时使用

---

### 8. G404 - Weak Random (HIGH, 2 issues)
弱随机数生成器。

**影响文件：**
- `internal/reports/cost_report.go:1220`
- `internal/budget/alert.go:936`

**用途：** 生成随机字符串 ID，非加密用途，风险低

---

### 9. G707 - SMTP Injection (HIGH, 1 issue)
SMTP 命令/头注入。

**影响文件：**
- `internal/automation/action/action.go:264`

---

## Medium Severity Issues

| Rule | Description | Count |
|------|-------------|-------|
| G304 | File path from variable | 215 |
| G204 | Subprocess launched with variable | 194 |
| G301 | Poor file permissions (mkdir 0755) | 177 |
| G306 | Poor file permissions (write 0644) | 152 |
| G302 | Poor file permissions (chmod) | 10 |
| G107 | URL from variable | 6 |
| G110 | Potential DoS | 2 |
| G117 | Concurrent map access | 2 |
| G505 | Weak crypto (SHA1/MD5) | 2 |
| G705 | XSS via taint | 2 |

---

## Low Severity Issues

| Rule | Description | Count |
|------|-------------|-------|
| G104 | Errors unhandled | 691 |

**说明：** 大量错误未处理，建议逐步完善错误处理。

---

## New Issues Since Last Audit

本次为 v2.151.0 首次安全扫描，无历史对比数据。

---

## Priority Recommendations

### 🔴 P0 - 立即修复
1. **scheduler/executor.go Context 泄漏** - 在 cleanup 中添加 cancel 调用
   ```go
   func (e *Executor) cleanup(taskID string) {
       e.mu.Lock()
       defer e.mu.Unlock()
       if rt, exists := e.running[taskID]; exists {
           rt.cancel()  // 添加此行
       }
       delete(e.running, taskID)
   }
   ```

### 🟡 P1 - 本周修复
1. **webdav/server.go 路径穿越** - 添加路径验证和清理
2. **media/streaming.go Context 泄漏** - 确保 cancel 在 defer 中调用

### 🟢 P2 - 计划修复
1. 整数溢出处理 - 使用安全转换函数
2. 文件权限问题 - 审计并调整权限设置
3. 错误处理完善 - 逐步添加错误检查

---

## Conclusion

**整体安全状态：中等风险**

主要风险点：
1. ✅ Context 泄漏问题已定位，需修复 `scheduler/executor.go`
2. ⚠️ WebDAV 路径穿越需要输入验证
3. ⚠️ 大量错误未处理可能影响稳定性

建议：
- 优先修复 Context 泄漏和路径穿越问题
- 建立定期安全扫描机制
- 对外部输入（WebDAV、API）加强验证