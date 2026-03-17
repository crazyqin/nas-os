# NAS-OS Security Audit Report

**Version:** v2.157.0  
**Date:** 2026-03-17  
**Tool:** gosec v2, govulncheck  

---

## Summary

| Metric | Value |
|--------|-------|
| **govulncheck** | ✅ No vulnerabilities found |
| **Total Issues (gosec)** | 1657 |
| **HIGH** | 158 |
| **MEDIUM** | 810 |
| **LOW** | 689 |

### 与 v2.151.0 对比

| 版本 | 总问题数 | HIGH | MEDIUM | LOW |
|------|----------|------|--------|-----|
| v2.151.0 | 1613 | 150 | 772 | 691 |
| v2.157.0 | 1657 | 158 | 810 | 689 |
| **变化** | +44 | +8 | +38 | -2 |

---

## govulncheck 结果

```
✅ No vulnerabilities found.
```

Go 标准库和依赖项无已知安全漏洞。

---

## 高严重性问题分析

### 1. G115 - Integer Overflow (74 issues)

整数类型转换潜在溢出问题，涉及 `uint64`/`int64` 转换。

**新增问题文件：**
- `internal/quota/optimizer/optimizer.go`
- `internal/monitor/metrics_collector.go`
- `internal/disk/smart_monitor.go`
- `internal/storage/distributed_storage.go`
- `internal/vm/snapshot.go`, `iso.go`
- `internal/backup/cleanup.go`

**风险等级：** LOW-MEDIUM  
在 NAS 系统中，这些转换主要用于磁盘空间计算和监控指标，实际溢出风险较低。

---

### 2. G703 - Path Traversal (48 issues)

路径穿越漏洞，可能导致未授权文件访问。

**最严重文件：**
- `internal/webdav/server.go` (30+ 处)
- `internal/vm/snapshot.go`
- `internal/backup/manager.go`, `encrypt.go`
- `internal/security/v2/encryption.go`

**风险等级：** HIGH  
WebDAV 服务暴露在网络中，需优先验证输入路径。

**建议：** 使用 `pkg/safeguards/paths.go` 中的安全路径处理函数。

---

### 3. G702 - Command Injection (10 issues)

命令注入风险，主要通过 virsh/qemu-img 执行。

**影响文件：**
- `internal/vm/manager.go` - virsh 命令
- `internal/vm/snapshot.go` - qemu-img 命令

**风险等级：** MEDIUM  
变量来自内部验证，风险可控。建议保持现有 #nosec 注释。

---

### 4. G402 - TLS InsecureSkipVerify (7 issues)

TLS 跳过验证。

**影响文件：**
- `internal/ldap/client.go` (2 处)
- `internal/auth/ldap.go` (2 处)
- `internal/cloudsync/providers.go`
- `internal/backup/sync.go`, `cloud.go`

**建议：** 仅在用户明确配置跳过验证时使用。

---

### 5. G101 - Hardcoded Credentials (7 issues)

潜在的硬编码凭证（大部分为误报）。

**影响文件：**
- `internal/auth/oauth2.go` - OAuth2 配置函数（误报）
- `internal/cloudsync/providers.go` - OAuth URL（误报）
- `internal/office/types.go` - 错误消息字符串（误报）

**状态：** 全部为误报，无需处理。

---

### 6. G122 - TOCTOU (7 issues)

Time-of-check to time-of-use 竞态条件。

**影响文件：**
- `internal/snapshot/replication.go`
- `internal/plugin/hotreload.go`
- `internal/files/manager.go`
- `internal/backup/manager.go`, `encrypt.go`, `verification.go`
- `internal/backup/config_backup.go`

---

### 7. G404 - Weak Random (3 issues)

弱随机数生成器。

**影响文件：**
- `internal/reports/cost_report.go:1220`
- `internal/budget/alert.go:936`
- `internal/cluster/loadbalancer.go:331`

**用途：** 生成随机字符串 ID，非加密用途，风险低。

---

### 8. G707 - SMTP Injection (1 issue)

**影响文件：** `internal/automation/action/action.go:264`

---

### 9. G118 - Context Leak (1 issue)

**影响文件：** `internal/plugin/monitor.go:759`

Goroutine 使用 `context.Background` 而非请求范围的 context。

---

## 敏感信息处理评估

### ✅ 正面发现

1. **凭证加密存储**
   - `internal/backup/credentials.go`: 使用 AES-GCM 加密
   - 密钥文件权限: 0600
   - 密钥目录权限: 0700

2. **敏感数据加密器**
   - `internal/auth/secret_encryption.go`: PBKDF2 派生密钥
   - 用于 TOTP Secret、备份码等

3. **无硬编码凭证**
   - 代码审查未发现实际硬编码的密码、API Key
   - G101 报告均为误报（函数参数、配置字段）

### ⚠️ 注意事项

1. 密钥文件存储在本地文件系统，需确保物理安全
2. 建议定期轮换加密密钥

---

## 中等严重性问题摘要

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

## 优先修复建议

### 🔴 P0 - 立即修复
无。govulncheck 未发现漏洞。

### 🟡 P1 - 本周修复
1. **webdav/server.go 路径穿越** - 添加路径验证和清理
2. **新增的整数溢出问题** - 使用 `pkg/safeguards/convert.go` 安全转换

### 🟢 P2 - 计划修复
1. 整数溢出处理 - 系统性使用安全转换函数
2. 文件权限问题 - 审计并调整权限设置
3. 错误处理完善 - 逐步添加错误检查

---

## 结论

**整体安全状态：中等风险**

| 项目 | 状态 |
|------|------|
| 依赖漏洞 (govulncheck) | ✅ 通过 |
| 敏感信息处理 | ✅ 合理 |
| 代码安全 (gosec) | ⚠️ 需关注 |
| 硬编码凭证 | ✅ 无 |

**主要发现：**
1. ✅ govulncheck 未发现依赖漏洞
2. ✅ 敏感信息加密存储机制健全
3. ⚠️ 问题数从 1613 增至 1657 (+44)
4. ⚠️ WebDAV 路径穿越需要输入验证

**建议：**
- 继续保持定期安全扫描
- 优先处理 WebDAV 路径验证
- 逐步使用 `pkg/safeguards` 安全函数

---

**Audit by:** 刑部安全审计  
**Report:** gosec-report-v2.157.0.json