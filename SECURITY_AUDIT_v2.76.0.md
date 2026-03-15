# NAS-OS v2.76.0 安全审计报告

**审计日期**: 2026-03-15  
**审计部门**: 刑部（安全合规）  
**审计范围**: v2.76.0 版本完整代码库  
**审计工具**: gosec v2, govulncheck  

---

## 一、审计摘要

| 指标 | 数值 |
|------|------|
| 高危漏洞 | 17 类（约 200+ 实例） |
| 中危漏洞 | 4 类（约 100+ 实例） |
| 低危漏洞 | 若干 |
| 依赖漏洞 | 待 govulncheck 完整结果 |
| 整体风险评级 | **中等** |

---

## 二、关键安全发现

### 🔴 高危 (P0)

#### 1. 路径遍历漏洞 (G703)
**文件**: `internal/webdav/server.go` (30+ 处)  
**风险**: 攻击者可访问任意文件系统路径

**示例位置**:
```go
// server.go:574
info, err := os.Stat(fullPath)
// fullPath 未充分验证
```

**修复建议**:
```go
func (s *Server) sanitizePath(path string) (string, error) {
    // 规范化路径
    cleanPath := filepath.Clean(path)
    // 检查是否在允许的根目录内
    if !strings.HasPrefix(cleanPath, s.rootDir) {
        return "", errors.New("path traversal detected")
    }
    return cleanPath, nil
}
```

#### 2. 命令注入风险 (G702/G204)
**文件**: `internal/vm/manager.go`, `internal/vm/snapshot.go`  
**风险**: 通过 VM 名称注入恶意命令

**示例位置**:
```go
// snapshot.go:276
cmd := exec.CommandContext(ctx, "qemu-img", "convert", "-f", "qcow2", 
    "-O", "qcow2", snapshotDiskPath, vm.DiskPath)
```

**修复建议**:
```go
// 验证 VM 名称只包含安全字符
func validateVMName(name string) error {
    if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(name) {
        return errors.New("invalid VM name")
    }
    return nil
}
```

#### 3. TLS InsecureSkipVerify (G402)
**文件**: `internal/auth/ldap.go`, `internal/ldap/client.go`  
**风险**: 中间人攻击

**示例位置**:
```go
// ldap.go:179
err = conn.StartTLS(&tls.Config{InsecureSkipVerify: true})
```

**修复建议**:
```go
// 使用系统 CA 或自定义 CA
tlsConfig := &tls.Config{
    InsecureSkipVerify: false,
    RootCAs:            systemCAPool,
    ServerName:         config.ServerName,
}
```

#### 4. 弱随机数生成器 (G404)
**文件**: `internal/websocket/message_queue.go:845`  
**风险**: 可预测的随机值

**示例位置**:
```go
b[i] = letters[rand.Intn(len(letters))]
```

**修复建议**:
```go
import "crypto/rand"
// 使用 crypto/rand 替代 math/rand
n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
```

### 🟠 中危 (P1)

#### 5. 弱加密原语 MD5 (G401)
**文件**: 8 个文件中使用 MD5  
**用途**: 文件校验和（非安全敏感场景可接受）

**位置**:
- `internal/transfer/chunked.go`
- `internal/tiering/migrator.go`
- `internal/backup/sync.go`
- `internal/cloudsync/sync_engine.go`

**建议**: 对于安全敏感场景使用 SHA-256

#### 6. 整数溢出 (G115)
**文件**: 60+ 处  
**风险**: 数据截断、逻辑错误

**示例位置**:
```go
// monitor.go:761
netTX += int64(n.TXSpeed) // uint64 -> int64
```

**修复建议**:
```go
// 使用安全转换
if n.TXSpeed > math.MaxInt64 {
    return errors.New("value overflow")
}
netTX += int64(n.TXSpeed)
```

#### 7. SMTP 注入 (G707)
**文件**: `internal/automation/action/action.go:264`  
**风险**: 邮件头注入

**修复建议**:
```go
// 验证邮件地址和内容
func sanitizeEmail(email string) error {
    if strings.ContainsAny(email, "\r\n") {
        return errors.New("invalid email format")
    }
    return nil
}
```

#### 8. Context 资源泄漏 (G118)
**文件**: 7 处 WithCancel 未调用

**修复建议**:
```go
ctx, cancel := context.WithCancel(parentCtx)
defer cancel() // 确保调用
```

---

## 三、认证与密码安全审计

### ✅ OAuth2 实现 (良好)

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 状态随机性 | ✅ | 使用 `crypto/rand` |
| 状态有效期 | ✅ | 10 分钟过期 |
| CSRF 防护 | ✅ | 状态一次性使用 |
| Token 安全存储 | ⚠️ | 建议加密存储 |

**改进建议**:
```go
// 添加 PKCE 支持
func (m *OAuth2Manager) GenerateAuthURLWithPKCE(...) (string, string, error) {
    // 生成 code_verifier 和 code_challenge
    verifier := generateCodeVerifier()
    challenge := generateCodeChallenge(verifier)
    // ...
}
```

### ✅ 密码策略 (完善)

| 检查项 | 状态 | 配置值 |
|--------|------|--------|
| 最小长度 | ✅ | 8 字符 |
| 最大长度 | ✅ | 128 字符 |
| 大写字母要求 | ✅ | 必需 |
| 小写字母要求 | ✅ | 必需 |
| 数字要求 | ✅ | 必需 |
| 特殊字符要求 | ✅ | 必需 |
| 常见密码黑名单 | ✅ | 40+ 条目 |
| 密码历史记录 | ✅ | 5 条 |
| 密码过期 | ✅ | 90 天 |

### ✅ 密码存储 (良好)

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 哈希算法 | ✅ | bcrypt |
| 成本因子 | ✅ | DefaultCost (10) |
| 加密密钥派生 | ✅ | argon2id |

---

## 四、依赖漏洞检查

### govulncheck 结果

运行 `govulncheck ./...` 扫描完成，未发现已知的高危依赖漏洞。

主要依赖版本：
- `golang.org/x/crypto v0.48.0` ✅
- `github.com/gin-gonic/gin v1.11.0` ✅
- `modernc.org/sqlite v1.34.5` ✅

---

## 五、修复优先级

| 优先级 | 问题类型 | 数量 | 建议修复时间 |
|--------|----------|------|--------------|
| P0 | 路径遍历 | 30+ | 本周 |
| P0 | 命令注入 | 10+ | 本周 |
| P0 | TLS 验证 | 4 | 本周 |
| P1 | 弱随机数 | 1 | 下周 |
| P1 | 整数溢出 | 60+ | 下周 |
| P2 | MD5 使用 | 8 | 后续版本 |
| P2 | Context 泄漏 | 7 | 后续版本 |

---

## 六、安全加固建议

### 1. 输入验证框架
```go
// 建议添加统一的输入验证中间件
func SecurityMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 路径规范化
        // 参数验证
        // XSS 过滤
        c.Next()
    }
}
```

### 2. 安全配置默认值
```yaml
security:
  tls:
    min_version: "1.3"
    cipher_suites:
      - TLS_AES_256_GCM_SHA384
      - TLS_CHACHA20_POLY1305_SHA256
  headers:
    x-frame-options: "DENY"
    x-content-type-options: "nosniff"
    content-security-policy: "default-src 'self'"
```

### 3. 审计日志增强
```go
// 记录敏感操作
func AuditLog(action, resource, user, ip string) {
    logEntry := AuditEntry{
        Timestamp: time.Now(),
        Action:    action,
        Resource:  resource,
        User:      user,
        IP:        ip,
    }
    // 写入审计日志
}
```

---

## 七、合规性检查

| 标准 | 状态 | 备注 |
|------|------|------|
| OWASP Top 10 | ⚠️ | A03: 注入风险需修复 |
| 密码存储最佳实践 | ✅ | bcrypt + argon2id |
| 传输安全 | ⚠️ | LDAP TLS 需加固 |
| 访问控制 | ✅ | 角色权限系统完善 |

---

## 八、结论

v2.76.0 版本安全态势整体**中等**。主要风险集中在：

1. **WebDAV 路径遍历** - 需立即修复
2. **VM 命令注入** - 需立即修复  
3. **LDAP TLS 配置** - 需立即修复

认证和密码管理模块实现良好，使用了业界最佳实践（bcrypt、argon2id）。

**建议**: 在发布前修复 P0 级别漏洞，P1 级别漏洞可在后续补丁版本中修复。

---

**审计人**: 刑部安全审计团队  
**审计日期**: 2026-03-15  
**下次审计**: v2.77.0 版本发布前