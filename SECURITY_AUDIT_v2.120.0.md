# nas-os 安全审计报告 v2.120.0

**审计日期**: 2026-03-16  
**审计工具**: gosec v2  
**项目版本**: v2.120.0  
**审计机构**: 刑部（法务合规）

---

## 📊 执行摘要

| 风险级别 | 数量 | 状态 |
|---------|------|------|
| **HIGH** | 171 | 🔴 需立即处理 |
| **MEDIUM** | 796 | 🟡 需计划修复 |
| **LOW** | 701 | 🟢 建议改进 |
| **总计** | **1668** | |

### 关键发现

本次审计共发现 **1668** 个安全问题，其中 **171** 个高危问题需要立即关注。主要风险集中在：

1. **整数溢出风险** (91处) - 数据类型转换未做边界检查
2. **路径遍历漏洞** (48处) - WebDAV 模块尤为严重
3. **命令注入风险** (202处) - 系统命令执行模块
4. **文件权限问题** (335处) - 权限设置过于宽松

---

## 🔴 高危问题清单

### 1. G115 - 整数溢出 (91处)

**风险等级**: HIGH  
**CWE-190**: 整数溢出或环绕

**影响模块**:
- `internal/system/monitor.go` - 网络速度计算
- `internal/quota/api.go` - 配额限制计算
- `internal/storage/smart_monitor.go` - SMART 数据处理
- `internal/reports/report_helpers.go` - 容量预测

**示例代码**:
```go
// internal/system/monitor.go:760-761
netRX += int64(n.RXSpeed)  // uint64 -> int64 可能溢出
netTX += int64(n.TXSpeed)
```

**修复建议**:
```go
// 方案1: 边界检查
if n.RXSpeed > math.MaxInt64 {
    netRX = math.MaxInt64
} else {
    netRX += int64(n.RXSpeed)
}

// 方案2: 使用安全转换函数
func safeUint64ToInt64(v uint64) (int64, error) {
    if v > math.MaxInt64 {
        return 0, errors.New("overflow")
    }
    return int64(v), nil
}
```

---

### 2. G703 - 路径遍历 (48处)

**风险等级**: HIGH  
**CWE-22**: 路径遍历

**影响模块**:
- `internal/webdav/server.go` - **严重**: 多处路径遍历风险
- `internal/vm/snapshot.go` - 快照文件路径
- `internal/backup/` - 备份恢复路径

**关键问题位置** (`internal/webdav/server.go`):
```
行 458: handleGet - os.Stat(fullPath)
行 482: handleHead - os.Stat(fullPath)
行 538: 文件创建 - os.Create(tmpPath)
行 602: 删除操作 - os.RemoveAll(fullPath)
行 801: 移动操作 - os.Rename(fullPath, destPath)
```

**修复建议**:
```go
import "path/filepath"

func (s *Server) sanitizePath(userPath string) (string, error) {
    // 清理路径，防止遍历
    cleanPath := filepath.Clean(userPath)
    
    // 确保路径在根目录内
    fullPath := filepath.Join(s.rootDir, cleanPath)
    if !strings.HasPrefix(fullPath, s.rootDir) {
        return "", errors.New("path traversal detected")
    }
    
    return fullPath, nil
}

// 使用示例
fullPath, err := s.sanitizePath(r.URL.Path)
if err != nil {
    http.Error(w, "Forbidden", http.StatusForbidden)
    return
}
```

---

### 3. G702/G204 - 命令注入 (202处)

**风险等级**: HIGH/MEDIUM  
**CWE-78**: OS命令注入

**影响模块**:
- `internal/vm/manager.go` - virsh 命令执行 (10处 HIGH)
- `pkg/btrfs/btrfs.go` - Btrfs 文件系统操作
- `internal/security/firewall.go` - iptables 规则
- `internal/network/` - 网络配置命令
- `internal/docker/` - Docker 容器管理

**高危示例** (`internal/vm/manager.go:558`):
```go
cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "start", vm.Name)
```

虽然代码注释声称 `vm.Name` 已验证，但需要确保验证逻辑完整。

**修复建议**:
```go
// 1. 严格验证输入
func validateVMName(name string) error {
    // 只允许字母、数字、下划线、连字符
    validName := regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
    if !validName.MatchString(name) {
        return errors.New("invalid VM name")
    }
    return nil
}

// 2. 使用参数化方式（避免 shell 解释）
cmd := exec.CommandContext(ctx, "virsh", "-c", "qemu:///system", "start", validatedName)

// 3. 避免使用 shell -c
// 错误: exec.Command("sh", "-c", fmt.Sprintf("echo %s", userInput))
// 正确: exec.Command("echo", validatedInput)
```

---

### 4. G402 - TLS 不安全 (4处)

**风险等级**: HIGH  
**CWE-295**: 证书验证不正确

**影响模块**:
- `internal/auth/ldap.go:184` - StartTLS 跳过验证
- `internal/ldap/client.go:73, 103` - LDAP 连接跳过验证

**问题代码**:
```go
// internal/auth/ldap.go:184
err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true})
```

**修复建议**:
```go
// 1. 使用正确的 TLS 配置
tlsConfig := &tls.Config{
    InsecureSkipVerify: false,
    ServerName:         serverName, // 设置服务器名称用于验证
    RootCAs:            systemCertPool, // 使用系统 CA
}

// 2. 如果必须跳过（仅限开发环境），添加警告日志
if skipVerify {
    log.Warn("TLS certificate verification disabled - NOT RECOMMENDED FOR PRODUCTION")
}
```

---

### 5. G404 - 弱随机数生成器 (2处)

**风险等级**: HIGH  
**CWE-338**: 使用弱随机数生成器

**影响模块**:
- `internal/reports/cost_report.go:1211`
- `internal/budget/alert.go:935`

**问题代码**:
```go
b[i] = letters[mrand.Intn(len(letters))] // 使用 math/rand
```

**修复建议**:
```go
import "crypto/rand"

func generateSecureRandomString(n int) (string, error) {
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

### 6. G101 - 潜在硬编码凭证 (7处)

**风险等级**: HIGH (误报)  
**CWE-798**: 硬编码凭证

**分析**:
经人工复核，以下报警为**误报**：
- `internal/auth/oauth2.go` - OAuth2 配置 URL（非凭证）
- `internal/cloudsync/providers.go` - API 端点 URL（非凭证）
- `internal/office/types.go` - 错误消息字符串（非凭证）

**状态**: ✅ 无需修复（误报）

---

### 7. G122 - TOCTOU 竞争条件 (7处)

**风险等级**: HIGH  
**CWE-367**: Time-of-check Time-of-use 竞争条件

**影响模块**:
- `internal/backup/manager.go:822`
- `internal/backup/encrypt.go:83`
- `internal/files/manager.go:892`
- `internal/snapshot/replication.go:827`

**问题场景**: `filepath.Walk` 回调中先检查再操作文件

**修复建议**:
```go
// 使用 os.Root (Go 1.24+) 防止 symlink 攻击
root, err := os.OpenRoot(baseDir)
if err != nil {
    return err
}
defer root.Close()

// 在 root 范围内操作，防止路径逃逸
f, err := root.Open(relativePath)
```

---

### 8. G707 - SMTP 注入 (1处)

**风险等级**: HIGH  
**CWE-93**: CRLF 注入

**影响模块**:
- `internal/automation/action/action.go:264`

**修复建议**:
```go
import "net/mail"

// 验证邮件地址格式
func validateEmail(email string) error {
    _, err := mail.ParseAddress(email)
    return err
}

// 清理邮件内容
func sanitizeEmailHeader(value string) string {
    // 移除 CRLF 字符
    return strings.NewReplacer("\r", "", "\n", "").Replace(value)
}
```

---

## 🟡 中危问题汇总

| 规则 | 数量 | 描述 | 主要模块 |
|------|------|------|---------|
| G304 | 230 | 文件路径注入 | 文件管理、备份 |
| G204 | 192 | 命令注入 | Docker、网络、磁盘 |
| G301 | 181 | 目录权限过宽 | 备份、配置 |
| G306 | 154 | 文件权限过宽 | 日志、配置 |
| G107 | 7 | HTTP URL 注入 | 云同步 |
| G110 | 4 | 潜在 DoS | 文件上传 |
| G505 | 3 | 弱加密算法 | 备份加密 |
| G705 | 2 | XSS | 自动化 API |
| G305 | 1 | Zip 路径遍历 | 解压缩 |
| G120 | 1 | 日志注入 | 审计日志 |

### G301/G306 - 权限问题修复建议

```go
// 目录权限: 0755 (rwxr-xr-x)
if err := os.MkdirAll(dir, 0755); err != nil {
    return err
}

// 文件权限: 0644 (rw-r--r--) 或更严格 0600 (rw-------)
if err := os.WriteFile(path, data, 0600); err != nil {
    return err
}

// 敏感文件: 0600 (仅所有者可读写)
// 配置文件: 0644 (所有者读写，其他人只读)
// 可执行文件: 0755
```

---

## 🟢 低危问题汇总

| 规则 | 数量 | 描述 |
|------|------|------|
| G104 | 701 | 未检查错误返回值 |

**建议**: 系统性添加错误检查，但优先级低于高危问题。

---

## 📋 许可证合规检查

### 项目许可证
- **主项目**: 待确认（未见 LICENSE 文件）

### 主要依赖许可证

| 依赖 | 许可证 | 兼容性 |
|------|--------|--------|
| gin-gonic/gin | MIT | ✅ |
| prometheus/client_golang | Apache-2.0 | ✅ |
| go-ldap/ldap | MIT | ✅ |
| blevesearch/bleve | Apache-2.0 | ✅ |
| aws/aws-sdk-go-v2 | Apache-2.0 | ✅ |
| gorilla/websocket | BSD-2-Clause | ✅ |
| spf13/cobra | Apache-2.0 | ✅ |
| stretchr/testify | MIT | ✅ |

**结论**: 所有主要依赖使用宽松许可证（MIT、Apache-2.0、BSD），无许可证冲突风险。

**建议**: 在项目根目录添加明确的 LICENSE 文件。

---

## 📈 修复优先级建议

### P0 - 立即修复（1周内）
1. ✅ WebDAV 路径遍历漏洞 (G703)
2. ✅ TLS InsecureSkipVerify (G402)
3. ✅ 弱随机数生成器 (G404)

### P1 - 短期修复（2周内）
1. 🔄 整数溢出风险 (G115)
2. 🔄 命令注入风险 (G702/G204)
3. 🔄 SMTP 注入 (G707)

### P2 - 中期修复（1个月内）
1. 📅 TOCTOU 竞争条件 (G122)
2. 📅 文件/目录权限 (G301/G306)
3. 📅 文件路径注入 (G304)

### P3 - 持续改进
1. 📋 错误检查完善 (G104)
2. 📋 代码质量优化

---

## 🔧 自动化修复脚本

```bash
# 运行安全扫描
gosec -fmt=json -out=gosec_report.json ./...

# 仅检查高危问题
gosec -severity=high ./...

# 排除测试文件
gosec -exclude-dir=tests ./...

# 生成 HTML 报告
gosec -fmt=html -out=security_report.html ./...
```

---

## 📝 附录

### A. 完整问题列表
详见: `gosec_report_v2.120.0.json`

### B. 修复验证命令
```bash
# 修复后重新扫描
gosec -severity=high ./... | grep -c "HIGH"

# 应降至 0 或接近 0
```

### C. 安全编码规范建议
1. 所有文件路径操作必须使用 `filepath.Clean()` 和路径边界检查
2. 所有用户输入在传递给系统命令前必须验证
3. 敏感操作使用 `crypto/rand` 而非 `math/rand`
4. TLS 连接必须验证证书
5. 文件权限遵循最小权限原则

---

**审计人**: 刑部安全审计组  
**审核日期**: 2026-03-16  
**下次审计**: 建议 v2.121.0 版本发布前