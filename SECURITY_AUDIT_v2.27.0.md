# NAS-OS 安全审计报告 v2.27.0

**审计日期**: 2026-03-15  
**审计范围**: 安全代码审查、认证授权模块、依赖安全、MFA 实现  
**项目路径**: /home/mrafter/clawd/nas-os  
**GitHub**: crazyqin/nas-os

---

## 一、执行摘要

本次安全审计针对 NAS-OS 项目进行了全面的安全评估，重点关注认证授权模块 (internal/auth/)、用户管理模块 (internal/users/)、MFA 实现以及依赖安全性。

**总体评价**: 项目安全架构设计合理，密码存储使用了行业标准 bcrypt 算法，RBAC 权限模型完整。但存在若干中等风险问题需要修复。

---

## 二、发现汇总

| 严重程度 | 数量 | 状态 |
|---------|------|------|
| 🔴 严重 | 1 | 需立即修复 |
| 🟠 高危 | 3 | 需尽快修复 |
| 🟡 中等 | 4 | 建议修复 |
| 🟢 低危 | 3 | 建议改进 |

---

## 三、详细发现

### 🔴 严重风险

#### 1. TOTP Secret 明文存储
**位置**: `internal/auth/manager.go:145`

**问题描述**:  
TOTP Secret 在代码注释中标记为"加密存储"，但实际以明文形式存储在 JSON 配置文件中。攻击者如果获得配置文件访问权限，可以读取所有用户的 TOTP Secret，从而绕过 MFA 保护。

```go
// types.go:17
TOTPSecret      string    `json:"totp_secret,omitempty"` // 加密存储

// manager.go:145 - 实际是明文存储
m.configs[userID].TOTPSecret = setup.Secret
```

**风险等级**: 严重  
**影响范围**: 所有启用 TOTP 的用户  
**修复建议**:
1. 使用 AES-256-GCM 或类似算法加密存储 TOTP Secret
2. 加密密钥应从安全存储（如 TPM、HSM 或加密配置文件）获取
3. 解密操作应在内存中进行，避免日志记录

---

### 🟠 高危风险

#### 2. WebAuthn 实现不完整
**位置**: `internal/auth/webauthn.go`

**问题描述**:  
WebAuthn 注册和认证流程是简化实现，未进行完整的安全验证：

```go
// webauthn.go:106 - 使用占位符
credential := &WebAuthnCredential{
    ID:              credID,
    PublicKey:       []byte("public-key-placeholder"),  // 🔴 占位符
    AttestationType: "none",
}

// webauthn.go:98 - 注释说明未实现验证
// 简化验证：实际应验证 response 中的 attestationObject 和 clientDataJSON
```

**风险等级**: 高危  
**影响范围**: 启用 WebAuthn MFA 的用户  
**修复建议**:
1. 实现完整的 WebAuthn 凭据验证流程
2. 验证 attestationObject 和 clientDataJSON
3. 正确存储 PublicKey 而非占位符
4. 考虑使用 `go-webauthn` 等成熟库

---

#### 3. 备份加密命令行密码泄露
**位置**: `internal/backup/manager.go:366`

**问题描述**:  
使用 openssl 命令行加密时，密码通过命令行参数传递：

```go
cmd := exec.Command("openssl", "enc", "-aes-256-cbc", "-salt", 
    "-in", backupPath, "-out", encryptedPath, 
    "-pass", "pass:"+encryptKey)  // 🔴 密码在命令行可见
```

**风险等级**: 高危  
**影响范围**: 启用加密备份的用户  
**修复建议**:
1. 使用文件传递密码：`-pass file:/path/to/keyfile`
2. 或使用环境变量：`-pass env:BACKUP_KEY`
3. 清理包含密码的临时文件
4. 考虑使用 Go 原生加密库替代命令行工具

---

#### 4. 默认密码输出到控制台
**位置**: `internal/users/manager.go:150-160`

**问题描述**:  
首次启动时，管理员默认密码被输出到 stdout：

```go
fmt.Println("========================================")
fmt.Println("⚠️  首次启动：默认管理员账号已创建")
fmt.Println("   用户名: admin")
fmt.Printf("   密码: %s\n", defaultPassword)  // 🔴 密码明文输出
fmt.Println("========================================")
```

**风险等级**: 高危  
**影响范围**: 首次部署的系统  
**修复建议**:
1. 将默认密码写入仅 root 可读的文件
2. 强制首次登录时修改密码
3. 不输出到 stdout，避免日志收集系统记录
4. 考虑生成一次性密码链接

---

### 🟡 中等风险

#### 5. MFA 会话存储在内存中
**位置**: `internal/auth/manager.go:320-330`

**问题描述**:  
MFA 会话仅存储在内存 map 中，服务重启后会话丢失：

```go
var (
    mfaSessionsMu sync.RWMutex
    mfaSessions   = make(map[string]*MFASession)  // 内存存储
)
```

**风险等级**: 中等  
**影响范围**: MFA 流程中的用户  
**修复建议**:
1. 使用 Redis 或数据库持久化会话
2. 实现会话恢复机制

---

#### 6. 备份码明文存储
**位置**: `internal/auth/backup.go`

**问题描述**:  
备份码在内存中以明文存储，未进行哈希处理：

```go
m.codes[userID][code] = &BackupCode{
    Code: code,  // 明文存储
    Used: false,
}
```

**风险等级**: 中等  
**影响范围**: 生成备份码的用户  
**修复建议**:
1. 使用 bcrypt 或 SHA-256 哈希存储备份码
2. 验证时比较哈希值而非明文

---

#### 7. 命令执行潜在的命令注入
**位置**: `internal/backup/manager.go`, `internal/snapshot/executor.go`

**问题描述**:  
多处使用 `exec.Command` 执行系统命令，部分参数来自用户配置：

```go
// manager.go:344
cmd := exec.Command("tar", "czf", backupPath, "-C", cfg.Source, ".")

// executor.go:89 - 执行脚本
cmd := exec.CommandContext(ctx, "sh", "-c", script)
```

**风险等级**: 中等  
**影响范围**: 使用备份/快照功能的用户  
**修复建议**:
1. 对所有用户输入进行严格验证和清理
2. 避免使用 `sh -c` 执行脚本
3. 使用参数化命令而非字符串拼接

---

#### 8. 敏感配置文件权限
**位置**: `configs/config.yaml`

**问题描述**:  
配置文件中可能包含敏感信息，需要确保文件权限正确：

```yaml
users:
  admin:
    password_hash: ""  # 首次启动时设置
```

**风险等级**: 中等  
**影响范围**: 配置文件存储的凭证  
**修复建议**:
1. 确保配置文件权限为 0600
2. 敏感配置使用环境变量或加密存储

---

### 🟢 低危风险

#### 9. TOTP 验证时间窗口
**位置**: `internal/auth/totp.go:64`

**说明**: 当前使用默认时间窗口验证，允许前后一个时间窗口的偏差，这是合理的实现，无需修改。

---

#### 10. 会话缓存过期清理
**位置**: `internal/auth/rbac.go`

**说明**: 会话缓存使用 5 分钟过期时间，但没有自动清理机制，可能导致内存泄漏。建议添加定期清理任务。

---

#### 11. 审计日志未启用
**位置**: `internal/auth/rbac_middleware.go:214`

**说明**: 审计日志中间件已实现但被注释，建议在生产环境启用：

```go
// log.Printf("[AUDIT] user=%s method=%s path=%s status=%d",
//     userID, c.Request.Method, c.Request.URL.Path, c.Writer.Status())
```

---

## 四、依赖安全审查

### 主要依赖版本
| 依赖 | 版本 | 状态 |
|-----|------|------|
| golang.org/x/crypto | v0.48.0 | ✅ 最新 |
| gin-gonic/gin | v1.11.0 | ✅ 最新 |
| pquerna/otp | v1.5.0 | ✅ 安全 |
| gorilla/websocket | v1.5.3 | ✅ 安全 |
| aws-sdk-go-v2 | v1.41.3 | ✅ 最新 |

### 依赖安全建议
1. 定期运行 `govulncheck` 扫描已知漏洞
2. 设置 Dependabot 或 Renovate 自动更新依赖
3. 锁定依赖版本，使用 `go.sum` 验证完整性

---

## 五、安全架构评价

### ✅ 良好实践

1. **密码存储**: 使用 bcrypt 算法，Cost=10 (默认)
2. **RBAC 实现**: 完整的角色-权限模型，支持继承
3. **会话管理**: Token 有效期 24 小时，支持刷新
4. **敏感文件检测**: 实现了敏感内容扫描功能
5. **审计中间件**: 提供审计日志基础设施
6. **HTTPS**: 建议生产环境强制 HTTPS

### ⚠️ 需要改进

1. 敏感配置加密存储
2. WebAuthn 完整实现
3. MFA 配置持久化
4. 备份码哈希存储
5. 命令注入防护

---

## 六、修复优先级建议

### P0 - 立即修复 (1-3 天)
1. 🔴 TOTP Secret 加密存储
2. 🔴 备份加密命令行密码泄露

### P1 - 尽快修复 (1 周)
1. 🟠 WebAuthn 完整实现
2. 🟠 默认密码安全处理

### P2 - 计划修复 (2-4 周)
1. 🟡 MFA 会话持久化
2. 🟡 备份码哈希存储
3. 🟡 命令注入防护加强

### P3 - 持续改进
1. 🟢 依赖安全监控
2. 🟢 审计日志启用
3. 🟢 安全测试覆盖

---

## 七、合规性检查

| 标准 | 状态 | 说明 |
|-----|------|------|
| OWASP Top 10 | ⚠️ 部分符合 | A02:加密失败需改进 |
| NIST 800-63B | ⚠️ 部分符合 | MFA 存储需改进 |
| PCI DSS | ⚠️ 部分符合 | 敏感数据加密需加强 |
| ISO 27001 | ✅ 基本符合 | 需完善审计日志 |

---

## 八、总结

NAS-OS 项目整体安全架构设计合理，核心认证功能使用了行业标准的安全实践。主要风险集中在 MFA 配置存储和 WebAuthn 实现上。建议按照优先级逐步修复发现的问题，并建立持续的安全监控机制。

**审计结论**: 项目可以继续开发，但建议在正式发布前完成 P0 和 P1 级别问题的修复。

---

**审计人**: 刑部  
**审计时间**: 2026-03-15 01:13 GMT+8