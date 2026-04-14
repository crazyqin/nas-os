# 安全审计报告 Round228 — Passkey/WebAuthn 安全评估

**审计部门**: 刑部（安全合规）
**审计时间**: 2026-04-15
**审计范围**: `internal/auth/`、`internal/rbac/`、`internal/smb/`、`internal/container/`、`internal/monitor/`
**风险等级说明**: P0=紧急 P1=高危 P2=中危 P3=低危

---

## 一、总体安全态势摘要

| 模块 | 风险等级 | 关键问题数 |
|------|---------|-----------|
| Passkey/WebAuthn 实现 | 🔴 P0 | 5个 |
| MFA 会话管理 | 🟠 P1 | 3个 |
| SMB 安全 | 🟡 P2 | 2个 |
| 容器运行时 | 🟡 P2 | 2个 |
| RBAC 权限系统 | 🟢 P3 | 1个 |

**本轮审计结论**: Passkey/WebAuthn 实现处于「演示级别」，存在多个 P0 密码学漏洞，在生产环境部署前必须重构核心验证逻辑。

---

## 二、P0 严重漏洞（必须修复）

### 🔴 漏洞 2.1: WebAuthn 签名验证完全缺失

**文件**: `internal/auth/webauthn.go`
**严重级别**: P0 — 直接影响认证完整性

**问题描述**:
`FinishRegistration()` 和 `FinishAuthentication()` 均有明确注释"实际应验证..."但实际上**完全没有进行任何密码学签名验证**：

```go
// FinishRegistration 中的问题：
// 1. attestationObject 只做类型检查，未解析
// 2. PublicKey 直接存储 "public-key-placeholder"
credential := &WebAuthnCredential{
    ID:              credID,
    PublicKey:       []byte("public-key-placeholder"), // ❌ 假公钥
    AttestationType: "none",
    // ...
}

// FinishAuthentication 中的问题：
// 1. 没有验证 signature
// 2. 没有验证 authenticatorData
// 3. 没有验证 signCount（可重放攻击）
```

**攻击场景**:
- 攻击者可以伪造任意认证响应通过验证
- 即使不知道用户私钥，也能冒充已注册用户登录
- 存储的"公钥"无任何实际用途，后续无法用于验证用户签名

**修复建议**:
1. 使用 `github.com/go-webauthn/webauthn` 或 `github.com/keys-pure/go-webauthn` 库
2. 注册时从 `attestationObject.authData` 正确提取公钥
3. 认证时验证 `authenticatorData.rpIdHash` 与期望值匹配
4. 认证时验证 `authenticatorData.counter` (signCount) 大于存储值
5. 使用提取的公钥验证 `assertion.signature`

---

### 🔴 漏洞 2.2: 凭证公钥存储为占位符

**文件**: `internal/auth/webauthn.go`、`internal/auth/passkey.go`
**严重级别**: P0 — 影响认证链路完整性

**问题描述**:
```go
// webauthn.go FinishRegistration:
PublicKey: []byte("public-key-placeholder"), // ❌ 占位符

// passkey.go FinishPasskeyRegistration:
authDataBytes, err := base64.URLEncoding.DecodeString(attestationObject)
if err != nil {
    authDataBytes = []byte("mock-auth-data")  // ❌ 失败时降级为模拟数据
}
```

**风险**: 认证时无法使用存储的公钥验证用户签名，整个认证链无效。

---

### 🔴 漏洞 2.3: FinishWebAuthnRegistration 存在用户混淆漏洞

**文件**: `internal/auth/manager.go`
**严重级别**: P0 — 权限混淆

```go
func (m *MFAManager) FinishWebAuthnRegistration(sessionID string, responseData interface{}) error {
    _, err := m.webauthnMgr.FinishRegistration(sessionID, responseData)
    if err != nil {
        return err
    }

    // ❌ userIDFromSession(sessionID) 是占位符函数，永远返回 sessionID 本身
    userID := userIDFromSession(sessionID)

    if m.configs[userID] == nil {
        m.configs[userID] = &MFAConfig{UserID: userID, CreatedAt: time.Now()}
    }
    m.configs[userID].WebAuthnEnabled = true  // ❌ 可能为错误的用户启用
}
```

```go
// 同一个文件中的占位符：
func userIDFromSession(sessionID string) string {
    return sessionID  // ❌ 返回值恒等于输入，没有实际查询会话数据
}
```

**影响**: WebAuthn 注册完成后，配置可能被写入错误的用户记录。

---

### 🔴 漏洞 2.4: Origin 验证默认不生效

**文件**: `internal/auth/passkey.go`、`internal/auth/manager.go`
**严重级别**: P0 — 中间人攻击 (MITM)

```go
// manager.go 中 WebAuthnManager 初始化：
webauthnCfg := WebAuthnConfig{
    RPID:      "localhost",  // ❌ localhost 在生产环境不安全
    RPOrigins: []string{"http://localhost:8080", "https://localhost:8080"},
}

// passkey.go 中 Origin 验证有条件：
if len(m.config.RPOrigins) > 0 {  // ❌ 若 RPOrigins 为空则跳过验证
    originValid := false
    // ...
    if !originValid {
        return "", fmt.Errorf("Origin 不允许: %s", clientData.Origin)
    }
}
```

**攻击场景**:
- 攻击者架设钓鱼网站 `https://evil.com`，用户在此登录
- 钓鱼网站调用 WebAuthn API，用户用指纹/FaceID 确认
- 由于 Origin 验证可被绕过，攻击者获得有效断言，冒充用户

**修复建议**:
1. 生产环境必须配置实际域名作为 RPID（如 `nas.example.com`）
2. Origin 验证必须强制执行，不应存在空列表跳过的逻辑
3. 考虑使用 FIDO MDS (Metadata Service) 验证认证器真伪

---

### 🔴 漏洞 2.5: 无 signCount 重放攻击防护

**文件**: `internal/auth/webauthn.go`、`internal/auth/passkey.go`
**严重级别**: P0 — 可重放旧认证

```go
// FinishAuthentication 完全未验证 signCount：
// 注释只说"简化验证"，但实际未实现
// 更新凭据最后使用时间，但不验证计数器
for _, cred := range m.credentials[session.UserID] {
    cred.LastUsedAt = &now
}
```

**攻击场景**:
1. 用户在受感染设备上完成 WebAuthn 认证（signCount=100）
2. 攻击者从网络流量/日志中获取认证响应包
3. 攻击者重放该响应，signCount=100 被接受（服务器未记录更高值）
4. 攻击者冒充用户登录

---

## 三、P1 高危问题

### 🟠 问题 3.1: MFA 会话存储存在竞态条件

**文件**: `internal/auth/manager.go`
```go
var (
    mfaSessionsMu sync.RWMutex  // 全局互斥锁
    mfaSessions   = make(map[string]*MFASession)  // 全局会话存储
)
```

**问题**: `mfaSessionsMu` 是包级全局变量，与 `MFAManager` 的 `mu` 锁完全独立。`CompleteMFASession` 和 `DeleteMFASession` 操作的是同一个全局 map，但锁不统一：

```go
// MFASession.CompleteMFASession:
mfaSessionsMu.Lock()
mfaSessions[tokenStr] = session
mfaSessionsMu.Unlock()

// MFAManager 中的操作：
mfaSessionsMu.RLock()  // ❌ 可能与上面的 Lock 并发
```

**修复建议**: 将 `mfaSessions` 和 `mfaSessionsMu` 纳入 `MFAManager` 结构体内部，使用单一锁保护。

---

### 🟠 问题 3.2: 内存会话在服务重启后丢失

**文件**: `internal/auth/manager.go`
```go
// WebAuthnManager 使用内存存储：
sessions map[string]*WebAuthnSession

// PasskeyManager 使用内存存储：
sessions map[string]*PasskeySession
```

**问题**: 服务重启后所有进行中的 WebAuthn 注册/认证会话丢失。用户需要重新开始流程，影响用户体验和安全流程的连续性。

**修复建议**: 使用 Redis 或数据库持久化会话，存储内容应加密：
- `sessionID` → 加密的 `SessionData`
- 包含 challenge（可设置较短 TTL 如 5 分钟）
- 完成后立即删除会话数据

---

### 🟠 问题 3.3: TOTP Secret 加密存储路径问题

**文件**: `internal/auth/manager.go`
```go
m.configs[userID].TOTPSecret = setup.Secret  // ❌ 直接存储，未加密
// saveConfig 存储到 JSON 文件（文件权限 0600，但内容明文）
```

**对比** `secret_encryption.go` 提供了加密能力：
```go
type SecretEncryption struct {
    key         []byte
    keyPath     string
    initialized bool
}
```

但 `MFAManager` 初始化时**未使用** `SecretEncryption`：
```go
// NewMFAManager 中：
m.smsManager = NewSMSManager(smsProvider)
m.backupManager = NewBackupCodeManager()
// ❌ 没有初始化 SecretEncryption
```

`EnhancedMFAManager` 有加密，但普通 `MFAManager` 没有。

---

## 四、P2 中危问题

### 🟡 问题 4.1: SMB 默认禁用电缆加密

**文件**: `internal/smb/security.go`
```go
func newDefaultSecurityConfig() *SMBSecurityConfig {
    return &SMBSecurityConfig{
        // ...
        MinProtocol: "SMB2",    // 🟡 最低 SMB2，未强制要求 SMB3
        EncryptData: false,     // ❌ 默认不加密传输
        // ...
    }
}
```

**风险**: SMB2 不强制要求签名和加密，明文传输可被局域网嗅探和中间人攻击。

**建议**: 默认改为 `MinProtocol: "SMB3"`, `EncryptData: true`。

---

### 🟡 问题 4.2: 容器镜像未验证签名

**文件**: `internal/container/image.go` (未见完整内容)
**推测风险**: 如果容器镜像拉取不验证签名或来源，可能存在恶意镜像注入风险。

**建议**: 实现镜像签名验证（Cosign / Notary），只允许来自可信仓库的签名镜像部署。

---

### 🟡 问题 4.3: passkey.go 存在重复 package 声明

**文件**: `internal/auth/passkey.go`
```go
// 文件顶部：
package auth        // ← 第一个 package 声明

// ... 中间大量代码 ...

// 文件末尾：
package auth        // ← 重复的 package 声明（虽然不影响运行）
```

**风险**: 文件被分割为两个编译单元，可能导致意外的编译/链接行为。Go 文件应只有一个 package 声明。

---

### 🟡 问题 4.4: WebAuthn 会话 TTL 配置不一致

**文件**: `internal/auth/webauthn.go` vs `internal/auth/passkey.go`

```go
// webauthn.go: 会话过期时间
ExpiresAt: time.Now().Add(5 * time.Minute),  // 5 分钟

// passkey.go: 使用配置的 timeout
ExpiresAt: time.Now().Add(time.Duration(m.config.Timeout) * time.Millisecond),
// 默认 m.config.Timeout = 60000ms = 60 秒
```

**不一致**: 同一个系统的不同组件使用不同的超时配置，可能导致奇怪的 UX 问题。

---

## 五、WebAuthn 安全检查清单

| 检查项 | 状态 | 说明 |
|--------|------|------|
| RP ID 验证 | ⚠️ 部分 | `localhost` 默认值，生产危险 |
| Challenge 随机性 | ✅ 良好 | 32 字节 `crypto/rand` |
| Challenge 长度 | ✅ 通过 | 32 字节 >= 16 字节要求 |
| Origin 验证 | ❌ 可选 | 可被空列表跳过 |
| User Verification | ⚠️ 可选 | 默认 `preferred` |
| signCount 验证 | ❌ 缺失 | 无重放攻击防护 |
| 签名验证 | ❌ 缺失 | **P0** |
| Attestation 验证 | ⚠️ "none" | 默认不验证认证器真伪 |
| 会话存储 | ❌ 内存 | 重启丢失，无加密 |
| 公钥存储 | ❌ 占位符 | 实际存储无意义数据 |

---

## 六、Passkey 实现安全建议（至少5条）

### 建议 1: 使用成熟的 WebAuthn 库 (P0)

不要自己实现 WebAuthn 密码学操作。推荐使用：

```go
import "github.com/go-webauthn/webauthn/webauthn"

wb, _ := webauthn.New(&webauthn.Config{
    RPID:     "nas.example.com",
    RPOrigin: "https://nas.example.com",
    RPName:   "NAS-OS",
})

// 注册
session, err := wb.BeginRegistration(user)
// ... 前端调用 navigator.credentials.create ...

// 完成注册（库自动验证签名、提取公钥）：
credential, err := wb.FinishRegistration(user, session, rawCred)
```

**理由**: WebAuthn 规范复杂（涉及 CBOR 解析、签名格式、authData 结构等），自行实现极容易引入密码学漏洞。

---

### 建议 2: 强制 Origin 和 RPID 严格匹配 (P0)

```go
// 必须在初始化时强制配置：
if cfg.RPID == "" || cfg.RPID == "localhost" {
    return nil, errors.New("生产环境必须配置非 localhost 的 RPID")
}

// 验证 Origin 必须在 HTTPS 下：
if !strings.HasPrefix(origin, "https://") && !strings.HasPrefix(origin, "http://localhost") {
    return "", errors.New("WebAuthn 仅支持 HTTPS 或 localhost")
}

// 禁止使用空 Origin 列表：
if len(m.config.RPOrigins) == 0 {
    return "", errors.New("必须配置至少一个允许的 Origin")
}
```

---

### 建议 3: 正确实现 signCount 验证 (P0)

```go
type StoredCredential struct {
    ID              string
    PublicKey       []byte  // 真实公钥
    SignCount       uint32  // 上次验证的计数器
    AAGUID          string
    LastUsedAt      time.Time
}

// FinishAuthentication 中验证：
if assertion.AuthData.Counter <= cred.SignCount {
    // 如果计数器 <= 存储值，说明认证器被克隆或响应被重放
    return "", errors.New("signCount 验证失败：可能的认证器克隆或重放攻击")
}
cred.SignCount = assertion.AuthData.Counter
```

---

### 建议 4: 持久化加密会话存储 (P1)

```go
// 使用 Redis 存储会话
type WebAuthnSessionStore struct {
    redis *redis.Client
    key   string  // "webauthn:session:{sessionID}"
}

func (s *WebAuthnSessionStore) Save(sessionID string, data *SessionData, ttl time.Duration) error {
    enc, err := encrypt(json.Marshal(data))
    if err != nil {
        return err
    }
    return s.redis.Set(ctx, "webauthn:session:"+sessionID, enc, ttl).Err()
}
```

**TTL 建议**: 注册会话 5 分钟，认证会话 2 分钟。

---

### 建议 5: 限制自动认证的用户标识范围 (P1)

`BeginPasskeyAuthenticationAuto` 允许不指定 `allowCredentials`：

```go
// 当前代码：
options := map[string]interface{}{
    "challenge":        challenge,
    // 无 allowCredentials，浏览器自动选择任意 Passkey
}
```

**风险**: 恶意用户用自己的 Passkey 尝试认证，可能混淆用户界面。

**建议**: 自动认证只用于已知用户上下文中（如已登录用户换设备），不应用于首次登录。

---

### 建议 6: 添加认证器克隆检测 (P1)

基于 signCount 异常检测：

```go
// 检测逻辑：
if cred.SignCount > 0 && assertion.AuthData.Counter == cred.SignCount {
    // 认证器被克隆：克隆的认证器没有递增计数器
    // 立即触发安全告警
    sendSecurityAlert(userID, "authenticator_cloning_detected", cred)
}
```

---

### 建议 7: 设备克隆/盗用场景的补充防护 (P1)

| 场景 | 防护措施 |
|------|---------|
| 攻击者获取用户 Passkey 备份 | 绑定设备指纹（AAGUID + 传输类型） |
| 用户设备丢失 | 提供远程撤销 Passkey 的能力 |
| 钓鱼网站诱导认证 | 强制 Origin 验证 + 域名显示 |
| 社会工程攻击 | 要求 MFA 二次确认（如 TOTP） |
| 认证器克隆 | signCount 单调递增检测 |

---

## 七、代码安全扫描结果

### go vet 检查

| 目录 | 结果 |
|------|------|
| `internal/auth/...` | ✅ 无问题 |
| `internal/monitor/...` | ✅ 无问题 |

### 代码质量问题

| 文件 | 问题 | 级别 |
|------|------|------|
| `internal/auth/passkey.go` | 重复 package 声明 | P3 |
| `internal/auth/manager.go` | 占位符 `userIDFromSession` | P0 |
| `internal/auth/webauthn.go` | 签名验证缺失 | P0 |
| `internal/auth/passkey.go` | 签名验证缺失 | P0 |
| `internal/auth/manager.go` | MFA 会话全局锁竞态 | P1 |

---

## 八、后续行动建议

### 立即行动（上线前必须完成）
- [ ] 使用标准 WebAuthn 库替换所有自定义密码学实现
- [ ] 修复 `userIDFromSession` 占位符，正确存储/查询用户会话
- [ ] 实现 signCount 验证和重放攻击防护
- [ ] 强制 Origin 验证，禁止 localhost 用于生产

### 短期行动（1-2周内）
- [ ] 将 WebAuthn 会话从内存迁移到加密 Redis 存储
- [ ] 统一 MFA 会话锁机制，消除竞态条件
- [ ] 确保 TOTP Secret 在 MFAManager 中也使用 SecretEncryption
- [ ] SMB 默认启用 SMB3 + 传输加密

### 中期行动（1个月内）
- [ ] 实现认证器克隆检测和告警
- [ ] 添加 WebAuthn 元数据服务（MDS）验证
- [ ] 完善 Passkey 审计日志
- [ ] 容器镜像签名验证集成

---

*报告生成: 刑部 Round228 审计*
*问题总数: P0×5, P1×3, P2×4*
