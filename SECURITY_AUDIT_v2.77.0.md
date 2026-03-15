# 安全审计报告

**项目**: nas-os  
**版本**: v2.77.0  
**审计日期**: 2026-03-16  
**审计部门**: 刑部  

---

## 执行摘要

本次安全审计对 nas-os v2.77.0 进行了全面的代码安全扫描，使用 gosec 静态分析工具检查了 443 个源文件，共计 272,004 行代码。

### 扫描统计

| 指标 | 数值 |
|------|------|
| 扫描文件数 | 443 |
| 代码行数 | 272,004 |
| 发现问题总数 | 1,691 |
| 高危问题 | 15+ |
| 中危问题 | 50+ |
| 低危问题 | 1,600+ |

---

## 严重安全问题

### 1. TLS 证书验证绕过 (G402 - CWE-295)

**严重级别**: 🔴 高危  
**影响**: 中间人攻击、数据泄露

**问题位置**:
- `internal/auth/ldap.go:179` - `InsecureSkipVerify: true` 硬编码
- `internal/ldap/client.go:99` - TLS 验证可能被禁用
- `internal/ldap/client.go:71` - TLS 验证可能被禁用
- `internal/auth/ldap.go:151` - TLS 验证可能被禁用

**风险描述**:
LDAP 连接时设置了 `InsecureSkipVerify: true`，这将跳过 TLS 证书验证，使连接容易受到中间人攻击。

**修复建议**:
```go
// 不推荐
tls.Config{InsecureSkipVerify: true}

// 推荐
tls.Config{
    ServerName: config.ServerName,
    // 仅在测试环境允许跳过验证
    InsecureSkipVerify: config.SkipTLSVerify && os.Getenv("ENV") == "test",
}
```

---

### 2. 弱随机数生成器 (G404 - CWE-338)

**严重级别**: 🔴 高危  
**影响**: 会话劫持、令牌预测

**问题位置**:
- `internal/websocket/message_queue.go:845` - 使用 `math/rand` 生成随机字符串

**风险描述**:
使用 `math/rand` 而非 `crypto/rand` 生成随机字符串，可能导致随机数可预测，影响安全性。

**修复建议**:
```go
import "crypto/rand"

func randomString(n int) (string, error) {
    const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
    b := make([]byte, n)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    for i := range b {
        b[i] = letters[int(b[i])%len(letters)]
    }
    return string(b), nil
}
```

---

### 3. 弱加密算法 (G401 - CWE-328)

**严重级别**: 🟠 中危  
**影响**: 数据完整性受损

**问题位置**:
- `internal/transfer/chunked.go:185, 176`
- `internal/tiering/migrator.go:294`
- `internal/security/scanner/filesystem_scanner.go:669`
- `internal/replication/conflict.go:253`
- `internal/cloudsync/sync_engine.go:800`
- `internal/backup/sync.go:842, 403`

**风险描述**:
使用 MD5 或 SHA1 等弱哈希算法，这些算法已被证明存在碰撞漏洞。

**修复建议**:
将 MD5/SHA1 替换为 SHA256 或更强的哈希算法。

---

### 4. 潜在硬编码凭证 (G101 - CWE-798)

**严重级别**: 🟠 中危  
**影响**: 凭证泄露

**问题位置**:
- `internal/office/types.go:581`
- `internal/cloudsync/providers.go:1320, 670`
- `internal/auth/oauth2.go:379-389, 364-374, 349-359, 334-344`

**风险描述**:
代码中可能存在硬编码的凭证或密钥，需要人工审查确认。

**修复建议**:
1. 审查所有标记位置
2. 将凭证移至环境变量或配置文件
3. 使用密钥管理服务

---

### 5. 命令注入风险 (G204 - CWE-78)

**严重级别**: 🟠 中危  
**影响**: 远程代码执行

**问题位置**:
- `pkg/btrfs/btrfs.go` - 多处使用 `exec.Command` 执行外部命令

**风险描述**:
使用变量作为命令参数，如果输入未经验证，可能导致命令注入攻击。

**修复建议**:
1. 对所有输入进行严格验证
2. 使用参数化方式传递命令参数
3. 限制可执行的命令白名单

---

### 6. 文件路径遍历 (G304 - CWE-22)

**严重级别**: 🟠 中危  
**影响**: 未授权文件访问

**问题位置**:
- `plugins/filemanager-enhance/main.go:448, 398`
- `tests/reports/generator.go:288`

**风险描述**:
使用用户输入构建文件路径，可能导致路径遍历攻击。

**修复建议**:
```go
import "path/filepath"

func safePath(base, userPath string) (string, error) {
    fullPath := filepath.Join(base, userPath)
    if !strings.HasPrefix(fullPath, base) {
        return "", errors.New("invalid path")
    }
    return fullPath, nil
}
```

---

### 7. 整数溢出 (G115 - CWE-190)

**严重级别**: 🟠 中危  
**影响**: 数据损坏、逻辑错误

**问题位置**:
- `internal/system/monitor.go:760, 761`
- `internal/snapshot/adapter.go:42`
- `internal/reports/report_helpers.go:156`
- `internal/quota/optimizer/optimizer.go:934, 516`
- `internal/photos/handlers.go:1378`

**风险描述**:
uint64 到 int64 的转换可能导致整数溢出。

---

## 中等安全问题

### 8. 错误处理不当 (G104 - CWE-703)

**严重级别**: 🟡 低危  
**数量**: 1,600+ 处

**风险描述**:
大量函数调用未检查返回的错误，可能导致程序在错误状态下继续运行。

**主要问题文件**:
- `internal/backup/` - 备份相关操作
- `internal/auth/` - 认证相关操作
- `internal/audit/` - 审计日志操作
- `api/websocket*.go` - WebSocket 连接操作

---

### 9. 文件权限过宽 (G301, G306 - CWE-276)

**严重级别**: 🟡 低危  
**影响**: 信息泄露

**问题位置**:
- `plugins/filemanager-enhance/main.go:249, 206` - 目录权限 0755
- `tests/reports/generator.go:106, 163` - 文件权限 0644

**修复建议**:
- 目录权限应设为 0750 或更严格
- 敏感文件权限应设为 0600

---

### 10. SSRF 风险 (G107 - CWE-88)

**严重级别**: 🟠 中危  
**影响**: 服务端请求伪造

**问题位置**:
- `internal/plugin/manager.go:511` - 使用变量 URL 发起 HTTP 请求

---

## 依赖安全分析

### 主要依赖

| 依赖 | 版本 | 风险评估 |
|------|------|----------|
| gin-gonic/gin | v1.11.0 | ✅ 最新稳定版 |
| golang.org/x/crypto | v0.48.0 | ✅ 最新版 |
| go-ldap/ldap/v3 | v3.4.12 | ✅ 最新版 |
| gorilla/websocket | v1.5.3 | ✅ 最新版 |
| modernc.org/sqlite | v1.34.5 | ✅ 最新版 |

### 依赖建议

1. 定期运行 `go list -m -u all` 检查更新
2. 使用 `govulncheck` 检查已知漏洞
3. 考虑使用 Dependabot 自动更新

---

## 安全最佳实践建议

### 1. 认证与授权
- ✅ 已实现 MFA 支持
- ✅ 已实现会话管理
- ⚠️ LDAP TLS 验证需要加强
- ⚠️ OAuth2 硬编码凭证需要审查

### 2. 数据保护
- ⚠️ 部分使用弱哈希算法
- ✅ 支持备份加密
- ⚠️ 随机数生成需要改进

### 3. 输入验证
- ⚠️ 命令参数需要更严格的验证
- ⚠️ 文件路径需要规范化处理
- ✅ 已实现请求验证器

### 4. 日志与审计
- ✅ 已实现安全审计日志
- ✅ 已实现敏感操作审计
- ⚠️ 部分错误未记录

---

## 修复优先级

| 优先级 | 问题类型 | 数量 | 建议修复时间 |
|--------|----------|------|--------------|
| P0 | TLS 验证绕过 | 4 | 立即 |
| P0 | 弱随机数生成 | 1 | 立即 |
| P1 | 弱加密算法 | 8 | 1周内 |
| P1 | 命令注入风险 | 15+ | 1周内 |
| P1 | 硬编码凭证 | 8 | 1周内 |
| P2 | 文件路径遍历 | 3 | 2周内 |
| P2 | 整数溢出 | 10+ | 2周内 |
| P3 | 错误处理 | 1600+ | 迭代修复 |

---

## 结论

nas-os v2.77.0 存在若干需要关注的安全问题，主要集中在：

1. **TLS 证书验证** - LDAP 连接存在中间人攻击风险
2. **加密安全** - 部分使用弱加密算法和随机数生成器
3. **输入验证** - 命令执行和文件路径处理需要加强

建议在下一版本发布前修复 P0 级别问题，并在后续迭代中逐步解决其他安全问题。

---

**审计人**: 刑部  
**审计日期**: 2026-03-16  
**报告版本**: 1.0