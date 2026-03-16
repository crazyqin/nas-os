# NAS-OS 安全审计报告 v2.142.0

**审计时间**: 2026-03-17  
**审计工具**: gosec v2  
**项目版本**: v2.141.0  
**目标版本**: v2.142.0  

---

## 📊 审计摘要

| 指标 | 数值 |
|------|------|
| **总问题数** | 1,645 |
| **高危 (HIGH)** | 153 |
| **中危 (MEDIUM)** | 791 |
| **低危** | 701 |

### 安全评级: ⚠️ **需要关注**

---

## 🔴 高危漏洞分析

### 1. 路径遍历漏洞 (G703) - 48 处

**风险等级**: 🔴 **严重**  
**CWE**: CWE-22 (Path Traversal)

**影响文件**:
- `internal/webdav/server.go` - 大量路径遍历点
- `internal/vm/snapshot.go`
- `internal/backup/encrypt.go`
- `internal/security/v2/encryption.go`
- `internal/automation/action/action.go`
- `plugins/filemanager-enhance/main.go`

**风险描述**: 用户输入可能未经验证直接用于文件系统操作，攻击者可通过 `../` 等路径遍历访问预期外的文件。

**修复建议**:
```go
import "path/filepath"

// 使用 filepath.Base() 或 filepath.Clean() 清理路径
safePath := filepath.Join(baseDir, filepath.Base(userInput))

// 或者使用 pkg/safeguards 中的安全路径检查
```

---

### 2. 命令注入漏洞 (G702) - 10 处

**风险等级**: 🔴 **严重**  
**CWE**: CWE-78 (OS Command Injection)

**影响文件**:
- `internal/vm/manager.go` - virsh 命令执行
- `internal/vm/snapshot.go` - qemu-img 命令执行

**风险描述**: 外部输入直接传递给系统命令，可能导致命令注入攻击。

**修复建议**:
```go
// 使用 exec.Command 的参数分离（已部分采用）
cmd := exec.Command("virsh", "-c", "qemu:///system", "start", vm.Name)

// 确保 vm.Name 经过严格验证，只允许安全字符
func validateVMName(name string) error {
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, name)
    if !matched {
        return errors.New("invalid VM name")
    }
    return nil
}
```

---

### 3. SMTP 注入漏洞 (G707) - 1 处

**风险等级**: 🔴 **严重**  
**CWE**: CWE-93 (CRLF Injection)

**影响文件**: `internal/automation/action/action.go:264`

**风险描述**: SMTP 命令可能被注入，导致邮件头注入攻击。

**修复建议**:
```go
// 对邮件地址和内容进行严格验证和清理
import "net/mail"

func validateEmail(email string) error {
    _, err := mail.ParseAddress(email)
    return err
}
```

---

### 4. 整数溢出 (G115) - 74 处

**风险等级**: 🟠 **高**  
**CWE**: CWE-190 (Integer Overflow)

**影响文件**:
- `internal/quota/optimizer/optimizer.go`
- `internal/monitor/metrics_collector.go`
- `internal/storage/distributed_storage.go`
- `internal/disk/smart_monitor.go`
- `internal/photos/handlers.go`

**风险描述**: uint64/int64 转换可能导致整数溢出。

**修复建议**:
```go
import "golang.org/x/exp/constraints"

// 使用安全转换或检查边界
func safeUint64ToInt64(v uint64) (int64, error) {
    if v > math.MaxInt64 {
        return 0, errors.New("value too large")
    }
    return int64(v), nil
}
```

---

### 5. TLS 不安全配置 (G402) - 3 处

**风险等级**: 🟠 **高**  
**CWE**: CWE-295 (Improper Certificate Validation)

**影响文件**:
- `internal/ldap/client.go` (2处)
- `internal/auth/ldap.go`

**风险描述**: `InsecureSkipVerify` 可能为 true，导致 TLS 证书验证被跳过。

**修复建议**:
```go
// 确保 InsecureSkipVerify 只在明确需要时设置
// 并记录警告日志
if skipVerify {
    log.Warn("TLS certificate verification disabled - use only in development")
}
```

---

### 6. 弱随机数生成器 (G404) - 2 处

**风险等级**: 🟠 **高**  
**CWE**: CWE-338 (Use of Cryptographically Weak PRNG)

**影响文件**:
- `internal/reports/cost_report.go:1220`
- `internal/budget/alert.go:936`

**修复建议**:
```go
import "crypto/rand"

// 使用 crypto/rand 替代 math/rand
func generateRandomString(n int) (string, error) {
    b := make([]byte, n)
    _, err := rand.Read(b)
    if err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(b), nil
}
```

---

### 7. 硬编码凭证风险 (G101) - 7 处

**风险等级**: 🟠 **中高**  
**CWE**: CWE-798 (Use of Hard-coded Credentials)

**影响文件**:
- `internal/auth/oauth2.go` - OAuth2 配置函数
- `internal/cloudsync/providers.go` - OAuth token URL
- `internal/office/types.go` - 错误消息（误报）

**风险描述**: 部分代码结构可能被误认为是硬编码凭证。

**实际情况**: 主要是 OAuth2 配置模板和 token URL，属于正常配置，但建议审查。

---

## 🟡 中危漏洞分析

### 1. 子进程变量注入 (G204) - 192 处

**风险等级**: 🟡 **中**  
**CWE**: CWE-78 (OS Command Injection)

**影响文件**: 广泛分布于：
- `internal/container/*.go` - Docker 命令
- `internal/network/*.go` - 网络配置命令
- `internal/security/*.go` - 防火墙/加密命令
- `pkg/btrfs/btrfs.go` - BTRFS 命令

**风险描述**: 命令参数使用变量，需要确保输入验证。

**修复建议**: 大部分使用了 `exec.Command` 的参数分离方式，相对安全，但仍需审查每个调用点的输入验证。

---

### 2. 文件路径注入 (G304) - 230 处

**风险等级**: 🟡 **中**  
**CWE**: CWE-22 (Path Traversal)

**影响文件**: 广泛分布

**风险描述**: 文件操作使用了变量路径，需要确保路径验证。

**修复建议**: 使用 `pkg/safeguards` 包中的路径验证函数。

---

### 3. 文件权限问题 (G301/G306/G302) - 345 处

**风险等级**: 🟡 **中**  
**CWE**: CWE-732 (Incorrect Permission Assignment)

**分布**:
- G301: 181 处 - 目录权限过于宽松
- G306: 154 处 - 文件权限过于宽松
- G302: 10 处 - 文件权限问题

**修复建议**:
```go
// 目录权限
os.MkdirAll(dir, 0755)  // 而不是 0777

// 敏感文件权限
os.WriteFile(file, data, 0600)  // 而不是 0644

// 配置文件权限
os.WriteFile(config, data, 0640)  // 组可读
```

---

### 4. Context 泄漏 (G118) - 10 处

**风险等级**: 🟡 **中**  
**CWE**: CWE-400 (Uncontrolled Resource Consumption)

**影响文件**:
- `internal/scheduler/executor.go`
- `internal/media/streaming.go`
- `internal/media/transcoder.go`
- `internal/backup/manager.go`

**风险描述**: Context 取消函数未被调用，可能导致资源泄漏。

**修复建议**:
```go
// 确保调用 cancel
ctx, cancel := context.WithTimeout(parentCtx, timeout)
defer cancel()  // 添加这行
```

---

### 5. TOCTOU 竞争条件 (G122) - 7 处

**风险等级**: 🟡 **中**  
**CWE**: CWE-367 (Time-of-check Time-of-use Race Condition)

**影响文件**:
- `internal/backup/manager.go`
- `internal/backup/encrypt.go`
- `internal/backup/config_backup.go`
- `internal/backup/advanced/verification.go`
- `internal/files/manager.go`
- `internal/plugin/hotreload.go`
- `internal/snapshot/replication.go`

**修复建议**: 考虑使用 Go 1.24+ 的 `os.Root` API 进行根作用域文件操作。

---

### 6. XSS 风险 (G705) - 2 处

**影响文件**: `internal/automation/api/handlers.go`

**风险描述**: 响应头可能包含用户输入。

**修复建议**: 对 `Content-Disposition` 文件名进行编码。

---

## 📋 按模块分布

| 模块 | 问题数 | 主要类型 |
|------|--------|----------|
| internal/backup | ~85 | G304, G204, G122 |
| internal/container | ~70 | G204, G304 |
| internal/security | ~45 | G204, G402 |
| internal/vm | ~35 | G702, G703 |
| internal/webdav | ~48 | G703 |
| internal/network | ~60 | G204 |
| pkg/btrfs | ~20 | G204 |

---

## ✅ 修复优先级

### P0 - 立即修复 (阻塞发布)
1. **G703 路径遍历** - webdav 和 filemanager-enhance 插件
2. **G702 命令注入** - VM 模块的 virsh 调用
3. **G707 SMTP 注入** - 自动化邮件模块

### P1 - 短期修复 (下个迭代)
1. **G402 TLS 配置** - LDAP 模块
2. **G404 弱随机数** - 使用 crypto/rand
3. **G115 整数溢出** - 关键数值计算

### P2 - 中期修复
1. **G118 Context 泄漏** - 资源管理
2. **G122 TOCTOU** - 文件操作原子性
3. **G301/G306 权限** - 文件系统安全加固

### P3 - 持续改进
1. **G104 错误处理** - 代码质量
2. **G204/G304 输入验证** - 全面审计

---

## 📝 审计结论

### 当前安全状态: ⚠️ **中等风险**

项目存在一定数量的安全漏洞，主要集中在：

1. **文件操作安全** - WebDAV、文件管理器等模块存在路径遍历风险
2. **命令执行安全** - VM 管理模块需加强输入验证
3. **配置安全** - LDAP TLS 配置需审查

### 建议

1. **立即处理 P0 级别漏洞**，特别是面向用户的 WebDAV 和文件管理模块
2. 为敏感操作添加 **输入验证中间件**
3. 扩展 `pkg/safeguards` 包，提供统一的 **安全路径操作 API**
4. 在 CI/CD 中 **集成 gosec 扫描**，防止新问题引入
5. 对安全相关代码进行 **代码审查**

---

**审计人**: 刑部 (安全审计子代理)  
**生成时间**: 2026-03-17 04:06 CST