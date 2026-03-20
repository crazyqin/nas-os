# 安全审计报告

**项目**: nas-os  
**日期**: 2026-03-20 11:44  
**审计范围**: 代码安全扫描、硬编码敏感信息检查、TLS/加密配置审查

---

## 执行摘要

| 风险等级 | 数量 | 状态 |
|---------|------|------|
| 🔴 高危 | 1 | 需关注 |
| 🟡 中危 | 3 | 已缓解/需验证 |
| 🟢 低危 | 4 | 已接受 |

**总体评价**: 项目安全状况良好，大部分安全问题已有适当的缓解措施。

---

## 详细发现

### 🔴 高危问题

#### 1. 初始密码输出到控制台
**位置**: `internal/users/manager.go:194`  
**规则**: G101 - 硬编码凭证/密码泄露

```go
fmt.Printf("   密码: %s\n", defaultPassword)
```

**风险**: 初始管理员密码在首次启动时打印到 stdout，可能被日志收集或终端历史记录捕获。

**建议**:
- 改为写入临时文件（如 `/run/nas-os/init-password`）并设置严格权限
- 或要求管理员在首次启动时交互式输入密码
- 当前缓解措施: 密码仅输出到 stdout，不记录到日志文件

---

### 🟡 中危问题

#### 2. InsecureSkipVerify 使用
**位置**: 
- `internal/backup/cloud.go:144`
- `internal/backup/sync.go:686`
- `internal/cloudsync/providers.go:414`
- `internal/auth/ldap.go:191`

**规则**: G402 - TLS 配置问题

**风险**: 跳过 TLS 证书验证可能导致中间人攻击。

**缓解措施**: ✅ 所有使用处都有环境检查 `os.Getenv("ENV") == "test"`
```go
skipVerify := config.SkipTLSVerify && os.Getenv("ENV") == "test"
if skipVerify {
    // #nosec G402
    TLSClientConfig: &tls.Config{InsecureSkipVerify: true}
}
```

**状态**: 已缓解，仅限测试环境使用。

---

#### 3. SHA1 用于 HMAC
**位置**: 
- `internal/security/v2/mfa.go:179,228` - TOTP 密钥生成
- `internal/network/ddns_providers.go:198`
- `internal/auth/sms.go:278`

**规则**: G505 - 弱加密算法

**风险**: SHA1 被认为不安全，但用于 HMAC 时仍可接受。

**说明**: TOTP 标准要求使用 HMAC-SHA1，这是 RFC 6238 规定的行为。用于 DDNS 签名的 HMAC-SHA1 同样符合各云服务商 API 规范。

**状态**: 已接受，符合行业标准。

---

#### 4. Shell 命令执行
**位置**: `internal/snapshot/executor.go:194`

```go
cmd := exec.CommandContext(ctx, "sh", "-c", script)
```

**风险**: 通过 `sh -c` 执行脚本可能存在命令注入风险。

**缓解措施**: 需验证 `script` 变量的来源和验证逻辑。

**建议**: 确保脚本内容来自可信源，或增加更严格的输入验证。

---

### 🟢 低危/已缓解问题

#### 5. 文件路径遍历风险
**位置**: 多处（webdav, vm, filemanager-enhance 等）

**缓解措施**: ✅ 所有路径操作前都有验证，代码中有 `#nosec G304` 注释说明验证逻辑。

---

#### 6. 随机数生成
**位置**: 多处

**状态**: ✅ 所有随机数生成都使用 `crypto/rand`，安全。

---

#### 7. 密码存储
**位置**: `internal/users/manager.go`

**状态**: ✅ 使用 bcrypt 进行密码哈希，安全。

---

#### 8. CSRF 保护
**位置**: `internal/web/middleware.go:33-34`

```go
log.Println("⚠️  [SECURITY WARNING] NAS_CSRF_KEY 环境变量未设置，已生成临时随机密钥")
```

**状态**: ✅ 有警告提示，生产环境应设置 `NAS_CSRF_KEY` 环境变量。

---

## 加密配置审查

### ✅ 正确实践

| 项目 | 实现 | 状态 |
|-----|------|------|
| 密码哈希 | bcrypt (DefaultCost) | ✅ 安全 |
| 敏感数据加密 | AES-256-GCM / AES-256-CBC | ✅ 安全 |
| 密钥派生 | argon2id / pbkdf2 | ✅ 安全 |
| 随机数 | crypto/rand | ✅ 安全 |
| JWT/Token | 使用环境变量配置 | ✅ 安全 |

### ⚠️ TLS 配置

| 配置 | 状态 |
|-----|------|
| InsecureSkipVerify | 仅测试环境可用 |
| CA 证书验证 | 支持 CA 证书路径配置 |
| StartTLS 支持 | LDAP 连接支持 StartTLS |

---

## 硬编码敏感信息检查

### 未发现硬编码密钥
扫描结果：未发现硬编码的 API 密钥、密码或私钥。

以下模式已检查：
- `password`, `secret`, `api_key`, `private_key`, `token`
- Base64 编码的长字符串（潜在密钥）

**注意**: `BindPassword` 等字段为配置项，从配置文件读取，非硬编码。

---

## 建议措施

### 立即处理
1. **初始密码输出**: 改用更安全的初始密码传递方式

### 短期改进
2. **脚本执行审计**: 审查 `executor.go` 中脚本来源验证
3. **CSRF Key**: 生产环境确保设置 `NAS_CSRF_KEY`

### 长期改进
4. **依赖扫描**: 集成 `govulncheck` 到 CI/CD
5. **安全测试**: 添加安全相关的单元测试

---

## 结论

项目整体安全状况良好，关键安全控制已到位：
- ✅ 密码使用 bcrypt 哈希
- ✅ 敏感数据加密使用 AES-256-GCM
- ✅ 密钥派生使用 argon2id
- ✅ 随机数使用 crypto/rand
- ✅ TLS 跳过验证仅限测试环境
- ⚠️ 建议改进初始密码传递方式

**审计完成时间**: 2026-03-20 11:44