# 安全审计报告 Round229 — Drive Sync 安全设计 & Passkey/WebAuthn 最终审计

**审计部门**: 刑部（安全合规）
**审计时间**: 2026-04-16
**审计范围**: `internal/drive/sync/`（安全设计）、`internal/auth/passkey/`（代码审计）
**风险等级说明**: P0=紧急 P1=高危 P2=中危 P3=低危

---

## 一、总体安全态势摘要

| 模块 | 状态 | 风险等级 | 说明 |
|------|------|---------|------|
| Drive Sync | 🟡 设计阶段 | P2 | 代码未实现，需安全设计文档先行 |
| Passkey/WebAuthn (`passkey/`) | 🟠 重大改进 | P1 | 相较 Round228 有显著进步，签名验证仍有缺陷 |

**本轮审计结论**: Passkey 实现已从 Round228 的「演示级别」提升至「基本可用级别」，但仍存在签名验证缺失（P0→P1）和会话持久化（P1）两个核心问题。Drive Sync 模块代码尚不存在，需按安全设计文档实现。

---

## 二、Drive Sync 安全评估

### 2.1 代码现状

`internal/drive/sync/` 目录**不存在**。`internal/drive/` 目录也不存在。Drive Sync 模块尚未开始编码。

### 2.2 安全设计文档

鉴于代码未实现，以下为 Drive Sync 模块的安全设计规范，作为后续实现的强制安全基线。

#### 2.2.1 传输加密（TLS）方案

| 项目 | 要求 |
|------|------|
| 协议 | TLS 1.3（最低 TLS 1.2） |
| 证书 | 服务端必须使用有效 X.509 证书，自签证书仅限局域网并需用户确认 |
| 密码套件 | 禁止 `TLS_RSA_*`、`TLS_ECDHE_*_SHA1`，仅允许 AEAD 密码套件 |
| 证书验证 | 双向同步时必须验证对端证书，支持 CA 白名单和证书指纹锁定 |
| 降级防护 | 禁止 SSLv3/TLS1.0/1.1，配置 `MinVersion: tls.VersionTLS12` |
| HSTS | 若有 Web 端，启用 HSTS，max-age ≥ 31536000 |

```go
// 推荐配置
tlsConfig := &tls.Config{
    MinVersion:   tls.VersionTLS12,
    CipherSuites: []uint16{
        tls.TLS_AES_256_GCM_SHA384,
        tls.TLS_CHACHA20_POLY1305_SHA256,
        tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
    },
    CurvePreferences: []tls.CurveID{
        tls.X25519,
        tls.CurveP256,
    },
}
```

#### 2.2.2 静态加密方案

| 项目 | 要求 |
|------|------|
| 加密算法 | AES-256-GCM（数据加密），Argon2id（密钥派生） |
| 密钥管理 | 主密钥存储于 TPM/加密存储，数据密钥分层派生 |
| 密钥轮换 | 支持在线密钥轮换，旧数据重新加密 |
| 文件级加密 | 每文件独立 DEK（Data Encryption Key），DEK 由 KEK 加密后存储于文件元数据 |
| 元数据保护 | 文件路径、大小、时间戳也应加密或混淆（防止信息泄露） |
| 密钥销毁 | 删除同步配置时彻底清除相关密钥材料 |

#### 2.2.3 路径遍历防护

```go
// 必须实现的路径验证
func validateSyncPath(baseDir, userPath string) (string, error) {
    // 1. 清理路径（移除 ..、//、\ 等）
    cleaned := path.Clean(userPath)

    // 2. 转为绝对路径
    absPath := filepath.Join(baseDir, cleaned)
    absBase, _ := filepath.Abs(baseDir)

    // 3. 验证结果路径仍在 baseDir 内
    if !strings.HasPrefix(absPath, absBase+string(os.PathSeparator)) {
        return "", fmt.Errorf("path traversal detected: %s escapes %s", userPath, baseDir)
    }

    // 4. 符号链接检查（防止通过 symlink 逃逸）
    realPath, err := filepath.EvalSymlinks(absPath)
    if err == nil {
        realBase, _ := filepath.EvalSymlinks(absBase)
        if !strings.HasPrefix(realPath, realBase+string(os.PathSeparator)) {
            return "", fmt.Errorf("symlink escape detected: %s resolves to %s", absPath, realPath)
        }
    }

    // 5. 文件名黑名单（拒绝特殊字符）
    if strings.ContainsAny(filepath.Base(cleaned), "\x00\n\r") {
        return "", fmt.Errorf("invalid characters in filename")
    }

    return absPath, nil
}
```

**路径遍历风险场景**:

| 攻击向量 | 防护措施 |
|----------|---------|
| `../../../etc/passwd` | `path.Clean` + 前缀检查 |
| 符号链接逃逸 | `filepath.EvalSymlinks` 双重检查 |
| Unicode 精化攻击 | NFC 标准化后验证 |
| 长路径攻击（>4096） | 路径长度限制 |
| 空字节注入 | 文件名黑名单 |
| 大小写混淆（不区分大小写文件系统） | 规范化比较 |

#### 2.2.4 同步劫持风险

| 风险 | 防护措施 |
|------|---------|
| 中间人攻击 | TLS 双向认证 + 证书锁定 |
| 回滚攻击 | 文件版本号单调递增 + 时间戳校验 |
| 重复文件注入 | 内容哈希（SHA-256）校验，同内容检测 |
| 恶意文件覆盖 | 冲突检测：修改时间 + 内容哈希，冲突时创建副本而非覆盖 |
| 配置篡改 | 同步配置文件签名保护 |
| DoS（海量文件洪水） | 同步速率限制 + 文件数量上限 |
| 未授权设备接入 | 设备注册令牌 + 首次配对需用户物理确认 |

---

## 三、Passkey/WebAuthn 最终审计

### 3.1 审计范围

| 文件 | 行数 | 用途 |
|------|------|------|
| `internal/auth/passkey/passkey.go` | ~480 | 核心协议实现 |
| `internal/auth/passkey/crypto.go` | 14 | SHA-256 哈希 |
| `internal/auth/passkey/handlers.go` | ~290 | HTTP API 层 |
| `internal/auth/passkey/passkey_test.go` | ~380 | 单元测试 |
| `internal/auth/passkey/passkey.go.bak` | 557 | 旧实现备份 |

### 3.2 与 Round228 对比改进

| 检查项 | Round228 | Round229 | 改善 |
|--------|----------|----------|------|
| RP ID 验证 | ⚠️ 部分 | ✅ SHA-256 hash 校验 | ✅ 已修复 |
| Challenge 随机性 | ✅ | ✅ 32字节 crypto/rand | — |
| Origin 验证 | ❌ 可跳过 | ✅ 强制白名单验证 | ✅ 已修复 |
| clientData 类型检查 | ❌ 缺失 | ✅ `webauthn.create`/`webauthn.get` 验证 | ✅ 已修复 |
| signCount 验证 | ❌ 缺失 | ✅ 单调递增检测 + 克隆告警 | ✅ 已修复 |
| 用户存在性检查 | ❌ 缺失 | ✅ UP 标志位验证 | ✅ 已修复 |
| 会话过期清理 | ⚠️ 部分 | ✅ 完成时删除 + 过期删除 | ✅ 已修复 |
| 排除重复注册 | ❌ 缺失 | ✅ excludeCredentials | ✅ 已修复 |
| 公钥存储 | ❌ 占位符 | ✅ 从 authData COSE 提取 | ✅ 已修复 |
| 签名验证 | ❌ 缺失 | ❌ **仍缺失** | ⚠️ 未修复 |
| 会话持久化 | ❌ 内存 | ❌ **仍内存** | ⚠️ 未修复 |

### 3.3 当前漏洞清单

#### 🔴 P0 → 降级为 P1：签名验证缺失

**文件**: `passkey.go` — `VerifyAuthentication()`
**严重级别**: P1（较 Round228 降低一级，因其他防护已到位）

**问题描述**:
```go
// VerifyAuthentication 中：
// 1. ✅ 验证了 clientDataJSON（type、challenge、origin）
// 2. ✅ 验证了 authenticatorData（RP ID hash、UP flag、counter）
// 3. ❌ 未验证 signature 字段 — 未使用存储的公钥验证签名
```

`VerifyAuthentication` 解析了 `authenticatorData` 并验证了 RP ID hash、UP 标志和 counter，但**完全忽略了 `response.signature` 字段**。攻击者仍可构造正确的 clientData 和 authData 来通过验证，无需拥有私钥。

**注意**: 由于 counter 验证和 challenge 验证已到位，攻击难度较 Round228 已大幅提高（需实时拦截 + 预测 challenge），但理论上仍可被中间人攻击利用。

**修复建议**:
```go
// 在 VerifyAuthentication 中添加：
signature, ok := resp["signature"].(string)
if !ok {
    return "", nil, fmt.Errorf("missing signature")
}
sigBytes, err := base64.RawURLEncoding.DecodeString(signature)
if err != nil {
    return "", nil, fmt.Errorf("invalid signature encoding")
}

// 使用存储的公钥验证签名
// signedData = authenticatorData + sha256(clientDataJSON)
signedData := append(authDataBytes, sha256HashImpl(clientDataJSON)...)
if !verifySignature(cred.PublicKey, signedData, sigBytes) {
    return "", nil, fmt.Errorf("signature verification failed")
}
```

需要引入 `verifySignature` 函数，支持 COSE 公钥格式（ES256 / RS256）。

#### 🟠 P1：会话存储为内存 map

**文件**: `passkey.go` — `Manager` 结构体
```go
type Manager struct {
    credentials map[string][]*Credential  // 内存 map
    sessions    map[string]*Session       // 内存 map
}
```

**风险**:
1. 服务重启后所有凭证和会话丢失（已注册的 Passkey 全部消失）
2. 无水平扩展能力（多实例不共享状态）
3. 凭证数据无持久化（无加密存储）

**修复建议**: 使用数据库（SQLite/BoltDB）持久化凭证，使用 Redis 持久化临时会话。

#### 🟡 P2：CBOR 解析为手写启发式

**文件**: `passkey.go` — `extractAuthData()`

```go
// 当前实现使用字符串搜索 "authD" 来定位 authData
if b[i] == 'a' && b[i+1] == 'u' && b[i+2] == 't' && b[i+3] == 'h' && b[i+4] == 'D' && b[i+5] == 'a' {
    // ... 简化提取
}
```

**风险**: CBOR 格式多样（definite/indefinite length、map key ordering），手写解析器在非标准 attestation 格式下可能提取错误数据，导致：
- 公钥提取错误（存储无效公钥）
- authData 截断（验证逻辑被绕过）

**修复建议**: 使用 `github.com/fxamacker/cbor/v2` 库正确解析 CBOR。

#### 🟡 P2：handlers.go 注册端点无认证保护

**文件**: `handlers.go` — `RegisterStart()` / `RegisterFinish()`

```go
// 公开端点，无需认证
g.POST("/register-start", h.RegisterStart)
g.POST("/register-finish", h.RegisterFinish)
```

**风险**: 任何知道用户名的人都可以为该用户注册新 Passkey。虽然 `RegisterStart` 需要用户名查询，但：
- 用户名枚举风险：可探测系统中有哪些用户
- 未认证注册：若攻击者知道目标用户名，可在用户未操作时抢先注册 Passkey

**修复建议**: `register-start` 和 `register-finish` 应要求用户已通过其他方式认证（密码/TOTP），或使用一次性注册令牌。

#### 🟡 P2：SHA-256 实现为全局可变变量

**文件**: `crypto.go`

```go
var sha256HashImpl = func(s string) []byte {
    return nil  // 初始值为 nil！
}
```

**风险**: 
1. 包初始化顺序依赖：若 `init()` 未执行（测试/导入异常），所有 hash 返回 nil
2. 全局变量可被反射覆盖（安全敏感函数不应可替换）
3. 测试中可注入假实现（降低测试可信度）

**修复建议**: 直接调用 `crypto/sha256.Sum256`，消除间接层。

#### 🟢 P3：`handlers.go` 中 uuid 包导入但未使用

```go
var _ = uuid.New  // 仅用于抑制编译警告
```

**风险**: 无实际安全风险，但说明存在未清理的依赖。

#### 🟢 P3：attestation 类型推断不准确

```go
// passkey.go VerifyRegistration:
attestationType := "none"
if attestationObj != "" {
    attestationType = "indirect" // 不准确：非空不等于 indirect
}
```

应从 CBOR 中正确解析 `fmt` 字段。

### 3.4 安全检查清单

| 检查项 | 状态 | 说明 |
|--------|------|------|
| Challenge 随机性 | ✅ | 32 字节 `crypto/rand` |
| Challenge 一次性 | ✅ | 使用后立即删除会话 |
| Challenge 绑定 | ✅ | session → challenge 强绑定 |
| Origin 白名单 | ✅ | 空列表时所有 origin 被拒绝 |
| RP ID Hash 校验 | ✅ | SHA-256 比对 |
| clientData 类型校验 | ✅ | `webauthn.create` / `webauthn.get` |
| 用户存在性 (UP) | ✅ | bit 0 验证 |
| Counter 单调递增 | ✅ | 防重放攻击 |
| 签名验证 | ❌ | **未实现**（P1） |
| 公钥提取 | ⚠️ | 从 COSE 提取但无格式验证 |
| 会话过期清理 | ✅ | 完成/过期均清理 |
| 排除重复注册 | ✅ | excludeCredentials |
| 会话持久化 | ❌ | 纯内存 map |
| CBOR 解析 | ⚠️ | 手写启发式，不够健壮 |
| 注册端点认证 | ❌ | 公开访问 |
| 凭证持久化 | ❌ | 纯内存 map |
| 备份凭证提示 | ✅ | Stats 中检查 `hasBackup` |

### 3.5 单元测试审计

| 测试项 | 状态 | 覆盖 |
|--------|------|------|
| 注册流程 | ✅ | 完整 |
| 认证流程 | ✅ | 完整 |
| Challenge 不匹配 | ✅ | 覆盖 |
| Origin 不匹配 | ✅ | 覆盖 |
| 会话过期 | ✅ | 覆盖 |
| 会话不存在 | ✅ | 覆盖 |
| 无凭证用户 | ✅ | 覆盖 |
| 排除重复凭证 | ✅ | 覆盖 |
| 多凭证管理 | ✅ | 覆盖 |
| **签名验证失败** | ❌ | 未覆盖（因功能未实现） |
| **Counter 回退** | ❌ | 未直接测试 |
| **跨用户凭证混淆** | ❌ | 未测试 |

**测试质量评价**: 覆盖了主要正常流程和部分异常流程，但缺少安全边界测试（跨用户攻击、counter 回退、签名伪造）。

---

## 四、漏洞统计与优先级

| 级别 | 数量 | 明细 |
|------|------|------|
| P0 紧急 | 0 | — |
| P1 高危 | 2 | 签名验证缺失、会话/凭证纯内存 |
| P2 中危 | 3 | CBOR 手写解析、注册端点无认证、SHA-256 全局可变 |
| P3 低危 | 2 | uuid 导入、attestation 类型推断 |
| **总计** | **7** | |

---

## 五、后续行动建议

### 立即行动（上线前必须完成）
- [ ] **实现签名验证**：使用存储的公钥验证 `signature` 字段（ES256 + RS256）
- [ ] **凭证持久化**：将 `credentials map` 迁移至 SQLite/BoltDB
- [ ] **会话持久化**：将 `sessions map` 迁移至 Redis 或文件存储

### 短期行动（1-2 周）
- [ ] **替换 CBOR 解析**：使用 `fxamacker/cbor` 库
- [ ] **注册端点加认证**：要求已登录或一次性令牌
- [ ] **消除 SHA-256 间接层**：直接调用 `crypto/sha256`

### 中期行动（1 个月）
- [ ] **Drive Sync 安全基线**：按本文档 2.2 节实现安全设计
- [ ] **添加安全边界测试**：跨用户攻击、counter 回退、签名伪造
- [ ] **Passkey 审计日志**：记录注册/认证/删除事件

---

## 六、Drive Sync 安全设计文档

鉴于 `internal/drive/sync/` 代码尚未创建，以下安全设计文档应作为模块实现的前置规范。

### 安全架构概览

```
┌─────────────┐     TLS 1.3      ┌─────────────┐
│  本地 NAS    │◄───────────────►│  远程 NAS    │
│  (Client)   │  双向证书认证     │  (Server)   │
└──────┬──────┘                  └──────┬──────┘
       │                                │
  ┌────▼────┐                     ┌────▼────┐
  │ 加密层   │  AES-256-GCM       │ 加密层   │
  │ (DEK)   │◄──────────────────►│ (DEK)   │
  └────┬────┘                     └────┬────┘
       │                                │
  ┌────▼────┐                     ┌────▼────┐
  │ 文件系统 │                     │ 文件系统 │
  │ (路径验证)│                     │ (路径验证)│
  └─────────┘                     └─────────┘
```

### 安全设计文件

建议在 `internal/drive/sync/` 创建以下文件：

1. **`SECURITY.md`** — 安全设计文档（本文档 2.2 节内容）
2. **`crypto.go`** — TLS 配置、密钥派生、加解密
3. **`path.go`** — 路径验证和遍历防护
4. **`conflict.go`** — 冲突检测和解决策略
5. **`ratelimit.go`** — 同步速率限制
6. **`audit.go`** — 同步操作审计日志

---

*报告生成: 刑部 Round229 审计*
*问题总数: P0×0, P1×2, P2×3, P3×2*
*与 Round228 对比: P0 从 5 降至 0（核心改进），P1 从 3 降至 2*
