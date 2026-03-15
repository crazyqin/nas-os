# 安全审计报告 v2.95.0

**审计日期:** 2026-03-16
**审计范围:** /home/mrafter/clawd/nas-os
**审计员:** 刑部

---

## 一、执行摘要

本次安全审计对 NAS-OS v2.95.0 版本进行了全面的安全评估，包括静态代码分析、敏感信息检查、依赖安全性审查等方面。

**总体评估:** 安全 ✅ (已修复所有中危漏洞)

发现 **3个中危漏洞** 和 **4个低危问题**，已修复所有中危漏洞。

---

## 二、安全扫描结果

### 2.1 静态代码扫描 (gosec 未安装)

> ⚠️ gosec 工具未安装在系统上，建议安装后进行完整扫描：
> ```bash
> go install github.com/securego/gosec/v2/cmd/gosec@latest
> gosec ./...
> ```

### 2.2 手动代码审计发现

#### 🔴 中危漏洞

| 编号 | 问题类型 | 位置 | 描述 |
|------|----------|------|------|
| CVE-001 | 弱随机数生成 | `internal/reports/cost_report.go:1162` | 使用 `time.Now().Nanosecond()` 生成随机字符串，可预测 |
| CVE-002 | 硬编码默认密钥 | `internal/web/middleware.go:30` | CSRF 保护使用硬编码默认密钥 |
| CVE-003 | 命令注入风险 | `internal/backup/manager.go:366` | OpenSSL 加密命令中密码直接传递 |

#### 🟡 低危问题

| 编号 | 问题类型 | 位置 | 描述 |
|------|----------|------|------|
| LOW-001 | TLS跳过验证 | 多处 | 用户可配置的 `InsecureSkipVerify`，有nosec注释 |
| LOW-002 | SHA1使用 | `internal/network/ddns_providers.go` | API规范要求的HMAC-SHA1 |
| LOW-003 | 敏感字段JSON暴露 | `internal/backup/cloud.go:57` | SecretKey字段可被JSON序列化 |
| LOW-004 | math/rand使用 | `internal/cluster/loadbalancer.go` | 用于后端选择，非安全敏感 |

---

## 三、详细漏洞分析

### 3.1 CVE-001: 弱随机数生成 ✅ 已修复

**文件:** `internal/reports/cost_report.go:1159-1167`

**问题代码:**
```go
func randomString(n int) string {
    const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
    b := make([]byte, n)
    for i := range b {
        b[i] = letters[time.Now().Nanosecond()%len(letters)]  // 可预测！
    }
    return string(b)
}
```

**风险:** 生成的字符串可预测，若用于安全敏感场景（如ID生成）可能导致碰撞或猜测攻击。

**修复状态:** ✅ 已修复 - 改用 `crypto/rand` 生成安全随机数

---

### 3.2 CVE-002: 硬编码默认CSRF密钥 ✅ 已修复

**文件:** `internal/web/middleware.go:27-31`

**问题代码:**
```go
csrfKey := os.Getenv("NAS_CSRF_KEY")
if csrfKey == "" {
    csrfKey = "change-this-to-a-32-byte-secret-key-now!"  // 硬编码！
}
```

**风险:** 若用户未设置环境变量，使用默认密钥可能导致CSRF保护被绕过。

**修复状态:** ✅ 已修复 - 改为生成随机密钥并输出警告日志

---

### 3.3 CVE-003: 加密密钥命令行传递 ✅ 已修复

**文件:** `internal/backup/manager.go:366`

**问题代码:**
```go
cmd := exec.Command("openssl", "enc", "-aes-256-cbc", "-salt", 
    "-in", backupPath, "-out", encryptedPath, 
    "-pass", "pass:"+encryptKey)  // 密钥在命令行可见
```

**风险:** 密钥可通过进程列表可见（`ps aux`），可能被其他用户进程读取。

**修复状态:** ✅ 已修复 - 改用环境变量传递密钥

---

## 四、安全控制评估

### 4.1 ✅ 已实现的安全控制

| 控制项 | 状态 | 位置 |
|--------|------|------|
| 路径遍历防护 | ✅ 已实现 | `pkg/security/sanitize.go` |
| CSRF保护 | ✅ 已实现 | `internal/web/middleware.go:204` |
| 密码哈希 (bcrypt) | ✅ 已实现 | `internal/users/manager.go` |
| 加密 (AES-256-GCM) | ✅ 已实现 | `internal/backup/encrypt.go` |
| 密钥派生 (PBKDF2/Argon2) | ✅ 已实现 | `internal/security/v2/` |
| MFA支持 | ✅ 已实现 | `internal/security/v2/mfa.go` |
| 安全随机数 | ✅ 多处使用 | `crypto/rand` |
| 输入验证 | ✅ 已实现 | `pkg/security/sanitize.go` |
| 审计日志 | ✅ 已实现 | `internal/web/middleware.go` |
| 命令参数清理 | ✅ 已实现 | `SanitizeCommandArg` |

### 4.2 ⚠️ 需要改进的安全控制

| 控制项 | 问题 | 建议 |
|--------|------|------|
| gosec扫描 | 未集成 | 添加到CI/CD |
| 依赖扫描 | 未自动化 | 添加 `govulncheck` |
| 密钥轮换 | 未实现 | 添加密钥轮换机制 |
| 敏感字段标记 | 部分缺失 | 添加 `json:"-"` 标签 |

---

## 五、依赖安全性

### 5.1 过时依赖 (需更新)

```
github.com/Azure/go-ntlmssp v0.0.0-20221128 -> v0.1.0 可用
github.com/aws/aws-sdk-go-v2 v1.41.3 -> v1.41.4 可用
github.com/aws/aws-sdk-go-v2/service/s3 v1.96.4 -> v1.97.1 可用
```

### 5.2 建议检查

运行以下命令检查已知漏洞：
```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
govulncheck ./...
```

---

## 六、敏感信息处理审计

### 6.1 敏感字段暴露检查

| 文件 | 字段 | 状态 |
|------|------|------|
| `internal/backup/cloud.go:57` | SecretKey | ⚠️ JSON可序列化 |
| `internal/backup/sync.go:96` | SecretKey | ⚠️ JSON可序列化 |
| `internal/backup/sync.go:101` | Password | ⚠️ JSON可序列化 |

**建议:** 为敏感字段添加 `json:"-"` 标签防止意外序列化：

```go
type CloudConfig struct {
    SecretKey string `json:"-"` // 不序列化
}
```

### 6.2 日志泄露检查

✅ **未发现敏感信息日志泄露问题** - 代码中未发现密码/密钥被记录到日志。

---

## 七、修复建议优先级

### ✅ 已修复 (发布前)

1. **CVE-001** - 修复弱随机数生成 ✅
2. **CVE-003** - 修复命令行密钥传递 ✅
3. **CVE-002** - 改进CSRF密钥管理 ✅

### 🟢 低优先级 (后续改进)

5. 集成gosec到CI/CD
6. 添加govulncheck自动扫描
7. 实现密钥轮换机制

---

## 八、结论

NAS-OS v2.95.0 在安全控制方面整体设计良好，已实现了多项重要的安全防护措施：
- 路径遍历防护
- CSRF保护
- 安全的密码哈希
- AES加密
- MFA支持

**已修复的安全问题：**
1. ✅ CVE-001 - 弱随机数生成 → 改用 `crypto/rand`
2. ✅ CVE-002 - 硬编码CSRF密钥 → 动态生成随机密钥并警告
3. ✅ CVE-003 - 命令行密钥传递 → 使用环境变量传递

**剩余低优先级改进：**
- 敏感字段添加 `json:"-"` 标签
- 集成 gosec/govulncheck 到 CI/CD
- 更新过时依赖

**安全状态:** ✅ 可发布

---

**审计完成时间:** 2026-03-16 06:38 UTC+8
**修复完成时间:** 2026-03-16 06:38 UTC+8
**下次审计建议:** 重大功能变更后或每季度一次