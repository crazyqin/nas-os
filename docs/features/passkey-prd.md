# Passkey / WebAuthn 无密码登录 PRD

## 文档信息

| 项目 | 内容 |
|------|------|
| **版本** | v1.0 |
| **日期** | 2026-04-15 |
| **作者** | 礼部尚书 |
| **状态** | 待评审 |

---

## 一、背景与动机

### 1.1 问题陈述

当前 nas-os 使用传统密码认证方式，存在以下问题：

- **密码疲劳**：用户需要在多个设备上管理复杂密码
- **钓鱼攻击**：密码易被钓鱼网站窃取
- **弱密码风险**：用户倾向于使用简单密码
- **重放攻击**：密码在传输过程中可被截获重用

### 1.2 行业趋势

无密码认证已成为身份认证领域的行业标准。FIDO Alliance 提出的 WebAuthn 标准已被所有主流平台采纳。

### 1.3 竞品对标

| 产品 | Passkey 支持状态 | 实现方式 | 上线时间 |
|------|-----------------|----------|----------|
| **群晖 DSM 7.2+** | ✅ 已支持 | Synology Passkey（设备级绑定 + 云端同步） | 2023年 |
| **TrueNAS 26** | ✅ 已支持 | WebAuthn（通过 TrueNAS Scale 支持） | 2025年 |
| **飞牛 fnOS** | ⚠️ 规划中 | 尚未正式实现 | 未定 |
| **QNAP QTS** | ✅ 已支持 | QNAP Passkey | 2023年 |
| **asustor ADM** | ❌ 尚未支持 | 无 Passkey 实现 | — |
| **nas-os** | 🚀 **本次实现** | WebAuthn RPO（完全自研） | **v2.457.0** |

> **差距分析**：群晖和 TrueNAS 均已上线 Passkey，nas-os 需要尽快补齐此功能，避免在功能竞争中落后。

---

## 二、功能概述

### 2.1 核心定义

**Passkey** 是基于 FIDO2/WebAuthn 标准实现的公钥加密无密码认证凭证。用户在设备上注册 Passkey 后，登录时只需使用设备本地认证（指纹、面部、PIN、设备密码）完成验证，无需输入密码。

### 2.2 功能范围

| 模块 | 范围 |
|------|------|
| 注册 | 用户在 Web UI 注册 Passkey（支持多设备） |
| 登录 | 用户使用 Passkey 完成无密码登录 |
| 管理 | 用户查看、重命名、删除已注册的 Passkey |
| 设备认证 | 支持 Windows Hello、Touch ID、Face ID、指纹等平台认证器 |
| 备份恢复 | 支持跨设备恢复（通过云端密钥托管或手动导出） |
| 回退机制 | 保留密码登录作为备用方案 |

### 2.3 非功能范围

- 不支持 LDAP/AD 账号的 Passkey（仅支持本地账号）
- 不支持 SCP/SFTP 的 Passkey（仅限 Web UI）
- 不支持生物特征数据的云端同步（仅设备本地存储）

---

## 三、用户故事

### 3.1 注册 Passkey

```
作为:  nas-os 用户
目标:  在我的设备上注册 Passkey
背景:  我已登录 nas-os，希望使用指纹/面部快速登录
场景:
  1. 我进入"用户设置" → "安全" → "Passkey 管理"
  2. 点击"注册新 Passkey"
  3. 输入 Passkey 名称（如"MacBook Pro 指纹"）
  4. 系统弹出平台认证器提示（指纹/PIN/面部）
  5. 我完成本地认证
  6. 系统提示注册成功，显示设备信息和创建时间
验收标准:
  ✅ 最多支持注册 10 个 Passkey
  ✅ Passkey 名称不能重复
  ✅ 支持设置 Passkey 描述（可选）
  ✅ 注册成功后在 Passkey 列表中可见
```

### 3.2 使用 Passkey 登录

```
作为:  nas-os 用户
目标:  使用 Passkey 无密码登录
背景:  我已在设备上注册过 Passkey
场景:
  1. 我打开 nas-os 登录页面
  2. 输入用户名（不输入密码）
  3. 点击"使用 Passkey 登录"
  4. 系统弹出 Passkey 认证提示
  5. 我完成设备本地认证（指纹/面部/PIN）
  6. 登录成功，进入控制台
验收标准:
  ✅ 支持点击按钮触发 Passkey 认证
  ✅ 自动检测已注册 Passkey 的设备
  ✅ 认证耗时 < 1 秒
  ✅ 认证失败后保留原页面，可重试
```

### 3.3 管理已注册 Passkey

```
作为:  nas-os 用户
目标:  管理已注册的 Passkey
背景:  我有多个设备注册了 Passkey，需要清理
场景:
  1. 我进入"用户设置" → "安全" → "Passkey 管理"
  2. 看到所有已注册的 Passkey 列表（含设备名称/类型/注册时间）
  3. 点击某一 Passkey 右侧"⋮"菜单
  4. 可选择"重命名"或"删除"
  5. 删除时系统要求确认（"此操作不可撤销"）
  6. 确认后该 Passkey 从列表中移除
验收标准:
  ✅ 列表清晰显示每条 Passkey 的设备类型和名称
  ✅ 支持按注册时间排序
  ✅ 删除前必须二次确认
  ✅ 不能删除最后一个 Passkey（必须保留至少一种登录方式）
```

---

## 四、技术架构

### 4.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                      用户浏览器 / 客户端                       │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────┐ │
│  │  登录页面 UI   │   │ Passkey 弹窗  │   │  平台认证器        │ │
│  │  (HTML/CSS)  │   │ (WebAuthn)   │   │ (Touch/WinHello) │ │
│  └──────┬───────┘   └──────┬───────┘   └────────┬─────────┘ │
└─────────┼───────────────────┼─────────────────────┼───────────┘
          │                   │                     │
          │  HTTPS/WSS        │ Credential Create   │
          │  (表单提交)        │ (navigator.credentials)          │
          ▼                   ▼                     │
┌─────────────────────────────────────────────────────────────┐
│                    nas-os Web 服务层                          │
│  ┌──────────────────────────────────────────────────────┐   │
│  │              WebAuthn API (internal/auth/passkey/)    │   │
│  │  ┌────────────────┐  ┌─────────────────────────┐    │   │
│  │  │ PasskeyService │  │    AttestationService    │    │   │
│  │  │  - Register()  │  │  - VerifyAttestation()   │    │   │
│  │  │  - Authenticate()  │  - ParseAAGUID()      │    │   │
│  │  │  - List()      │  │  - ValidateAuthenticator│    │   │
│  │  │  - Delete()    │  └─────────────────────────┘    │   │
│  │  └───────┬────────┘                                   │   │
│  │          │ 存储层                                       │   │
│  │  ┌───────▼────────┐  ┌─────────────────────────┐       │   │
│  │  │ PasskeyRepository│ │ CredentialHandleStore  │       │   │
│  │  │ (MySQL/JSON)   │  │ (Challenge/Expires)    │       │   │
│  │  └───────┬────────┘  └─────────────────────────┘       │   │
│  └──────────┼─────────────────────────────────────────────┘   │
└─────────────┼─────────────────────────────────────────────────┘
              │
┌─────────────▼─────────────────────────────────────────────────┐
│                    MySQL / SQLite 数据库                      │
│  ┌────────────────┐  ┌────────────────────────────────────┐   │
│  │  users 表       │  │  user_passkeys 表                   │   │
│  │  - id          │  │  - id (PK)                          │   │
│  │  - username    │  │  - user_id (FK)                     │   │
│  │  - password    │  │  - credential_id (BLOB, UNIQUE)    │   │
│  │  - ...         │  │  - public_key (TEXT, PEM)           │   │
│  └────────────────┘  │  - device_name (VARCHAR 255)       │   │
│                      │  - device_type (ENUM)               │   │
│                      │  - aaguid (VARCHAR 36)              │   │
│                      │  - sign_count (INT)                 │   │
│                      │  - created_at (TIMESTAMP)           │   │
│                      │  - last_used_at (TIMESTAMP)         │   │
│                      └────────────────────────────────────┘   │
│  ┌────────────────────────────────────────────────────────┐   │
│  │  passkey_challenges 表 (一次性挑战值，防重放)              │   │
│  │  - challenge_id (PK)                                    │   │
│  │  - challenge (BLOB 32字节)                               │   │
│  │  - user_id (FK)                                         │   │
│  │  - expires_at (TIMESTAMP, 5分钟过期)                     │   │
│  │  - used (BOOLEAN, 默认 false)                            │   │
│  └────────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
```

### 4.2 注册流程（Sequence）

```
用户浏览器                  Web服务器                   数据库
    │                          │                          │
    │──GET /api/v2/passkey/registration-options──────────▶│
    │                          │                          │
    │                          │──生成 challenge (32B)────▶│ (存入 challenges 表)
    │                          │──查询用户信息────────────▶│
    │◀────────200: options JSON──────────────────────────│
    │                          │                          │
    │──POST /api/v2/passkey/register────────────────────▶│
    │   {credential, clientData, attestation}            │
    │                          │                          │
    │                          │──验证 attestation────────▶│
    │                          │──验证 challenge──────────▶│ (检查未使用+未过期)
    │                          │──验证 origin/RP ID───────▶│
    │                          │──生成 userHandle ────────▶│
    │                          │──存储公钥───────────────▶│ (user_passkeys)
    │◀────────200: success──────────────────────────────│
    │                          │                          │
    ▼                          ▼                          ▼
```

### 4.3 认证流程（Sequence）

```
用户浏览器                  Web服务器                   数据库
    │                          │                          │
    │──POST /api/v2/passkey/authentication-options────────▶│
    │   {username}             │                          │
    │                          │──生成 challenge──────────▶│ (存入 challenges 表)
    │                          │──查询该用户 Passkeys─────▶│
    │◀────────200: allowCredentials[]────────────────────│
    │                          │                          │
    │──POST /api/v2/passkey/authenticate─────────────────▶│
    │   {credentialId, clientData, authenticatorData, sig}│
    │                          │                          │
    │                          │──加载公钥───────────────▶│
    │                          │──验证 challenge──────────▶│ (检查未使用+未过期)
    │                          │──验证签名────────────────▶│
    │                          │──更新 sign_count────────▶│
    │                          │──生成 session token─────▶│
    │◀────────200: JWT session───────────────────────────│
    │                          │                          │
    ▼                          ▼                          ▼
```

### 4.4 关键数据结构

```go
// user_passkeys 表对应的 Go 结构
type Passkey struct {
    ID            uint64    `json:"id"`
    UserID        uint64    `json:"user_id"`
    CredentialID  []byte    `json:"credential_id"`  // 存储原始字节，Base64URL 序列化
    PublicKey     string    `json:"public_key"`     // PEM 格式公钥
    DeviceName    string    `json:"device_name"`
    DeviceType    string    `json:"device_type"`    // "platform" | "cross-platform"
    AAGUID        string    `json:"aaguid"`          // 认证器类型标识
    SignCount     uint32    `json:"sign_count"`      // 防克隆计数器
    CreatedAt     time.Time `json:"created_at"`
    LastUsedAt    time.Time `json:"last_used_at"`
}

// Challenge 缓存结构
type Challenge struct {
    ID        string    `json:"id"`
    Challenge []byte    `json:"challenge"`  // 32 字节随机数
    UserID    uint64    `json:"user_id"`
    ExpiresAt time.Time `json:"expires_at"` // 5 分钟后过期
    Used      bool      `json:"used"`        // 一次性使用标记
}
```

### 4.5 目录结构

```
internal/auth/passkey/
├── service.go           # PasskeyService 主服务
├── service_test.go       # 单元测试
├── handler.go            # HTTP 处理器
├── handler_test.go       # 处理器测试
├── repository.go         # 数据库操作
├── repository_test.go    # 仓储测试
├── challenge.go          # 挑战值生成与验证
├── attestation.go        # 认证器证明验证
├── options.go            # RegistrationOptions / AuthenticationOptions 生成
├── types.go              # WebAuthn 类型定义
└── doc.go                # 包文档

web/public/src/views/security/PasskeyManagement.vue  # 前端管理页面
web/public/src/views/login/PasskeyLogin.vue          # 登录组件
```

---

## 五、安全考虑

### 5.1 重放攻击防护

| 防护措施 | 实现方式 |
|----------|----------|
| **一次性 Challenge** | 每次认证使用 32 字节密码学安全随机数，存入数据库，过期 5 分钟，使用后立即标记为已使用 |
| **Challenge 绑定用户** | Challenge 与 userID 绑定，防止跨用户重放 |
| **TLS 传输** | 全程 HTTPS，防止中间人截获 |
| **时间戳验证** | `authenticatorData.timestamp` 与服务器时间偏差不超过 5 分钟 |

### 5.2 设备绑定

- **Credential ID 唯一性**：每个设备生成的 Credential ID 全局唯一，无法伪造
- **Sign Count 防克隆**：每次认证验证 sign_count 递增，检测设备克隆攻击
- **平台认证器 vs 漫游认证器**：
  - `platform`（Windows Hello、Touch ID）：设备绑定型
  - `cross-platform`（YubiKey）：可跨设备使用，可控性更低

### 5.3 备份与恢复

| 恢复方式 | 实现 | 说明 |
|----------|------|------|
| **多设备注册** | 用户主动在多个设备上注册 | 最推荐的方案 |
| **加密备份导出** | 导出 EncryptedKeyBundle（含私钥，需要用户设置的备份密码） | 适用于设备丢失场景 |
| **云端密钥托管（可选）** | 通过端到端加密将密钥安全分片存储到云 | 可选功能，需用户主动开启 |
| **保留密码回退** | 始终保留密码登录作为最后回退手段 | 强制的兜底方案 |

> ⚠️ **警告**：导出备份文件时必须包含私钥，用户必须妥善保管备份密码。备份文件泄露且备份密码被破解 = 账户被盗。

### 5.4 安全边界

- RP ID 必须匹配 `nas-os` 的域名/主机名
- 仅接受 `none` 或 `packed` 证明格式（不验证 FIDO 元数据可减少依赖）
- 禁止在 `userHandle` 中存储敏感信息
- 所有私钥相关操作在前端完成，服务端仅存储公钥

---

## 六、兼容性

### 6.1 浏览器支持矩阵

| 浏览器 | 版本要求 | 支持 Passkey 类型 |
|--------|----------|-------------------|
| **Chrome** | 98+ | ✅ 平台 + 漫游 |
| **Firefox** | 122+ | ✅ 平台 + 漫游 |
| **Safari** | macOS 16+, iOS 16+ | ✅ 平台（Touch ID/Face ID）+ 漫游 |
| **Edge** | 98+ | ✅ 平台（Windows Hello）+ 漫游 |
| **Arc** | 基于 Chromium 98+ | ✅ 平台 + 漫游 |
| **Brave** | 基于 Chromium 98+ | ✅ 平台 + 漫游 |

> **移动端说明**：
> - iOS Safari：支持 iCloud Keychain Passkey（跨设备同步）
> - Android Chrome：支持 Google Password Manager 和设备生物认证

### 6.2 操作系统支持

| OS | 平台认证器 | 漫游认证器 |
|----|-----------|-----------|
| macOS 13+ | ✅ Touch ID / PIN | ✅ YubiKey / 安全密钥 |
| Windows 10/11 | ✅ Windows Hello | ✅ YubiKey / 安全密钥 |
| iOS 16+ | ✅ Face ID / Touch ID | ✅ YubiKey |
| Android 9+ | ✅ 设备 PIN / 指纹 / 面部 | ✅ YubiKey NFC |
| Linux (Chrome/FFirefox) | ⚠️ 取决于桌面环境 | ✅ YubiKey |

### 6.3 退化策略

当检测到浏览器不支持 WebAuthn 时：
1. 检测 `window.PublicKeyCredential` 是否存在
2. 不存在 → 隐藏 Passkey 登录按钮，显示标准密码登录
3. 存在但用户取消认证 → 保留登录页面，用户可重试或切换到密码登录

---

## 七、落后指标（Lag Indicators）

| 指标名称 | 指标定义 | 目标值 | 告警阈值 |
|----------|----------|--------|----------|
| **Passkey 启用率** | 活跃用户中注册了至少 1 个 Passkey 的比例 | ≥ 30%（6 个月后） | < 10% |
| **认证成功率** | Passkey 认证请求中成功的比例 | ≥ 95% | < 85% |
| **认证耗时 P95** | 从点击"使用 Passkey"到登录完成的端到端耗时 P95 | < 800ms | > 1500ms |
| **认证失败率** | 因浏览器不支持或认证器错误导致的失败比例 | < 3% | > 8% |
| **注册成功率** | Passkey 注册请求中成功的比例 | ≥ 92% | < 80% |

> **采集方式**：通过 `/api/v2/metrics/passkey` 端点暴露 Prometheus 指标，在 Grafana 中配置告警规则。

---

## 八、开发计划

### 8.1 阶段划分

| 阶段 | 内容 | 优先级 |
|------|------|--------|
| **Phase 1（MVP）** | Passkey 注册/认证核心流程 + 基础管理界面 | P0 |
| **Phase 2（增强）** | 多设备管理、重命名、删除 + Passkey 登录引导页 | P1 |
| **Phase 3（高级）** | 备份导出/恢复、性能监控仪表盘 | P2 |

### 8.2 依赖

- Go 1.22+
- `github.com/go-webauthn/webauthn`（WebAuthn 协议库）
- MySQL 8.0+ / SQLite（存储）
- `golang.org/x/crypto`（加密工具）
- Prometheus client for Go（指标）

---

## 九、附录

### 9.1 WebAuthn 术语对照

| 术语 | 英文 | 含义 |
|------|------|------|
| 凭证 | Credential | 认证器生成的密钥对（公钥存服务器，私钥存设备） |
| 认证器 | Authenticator | 执行本地认证的硬件/软件（Touch ID、Windows Hello 等） |
| 依赖方 | Relying Party (RP) | nas-os 服务器端 |
| 挑战值 | Challenge | 服务器下发的一次性随机数，防重放 |
| 证明 | Attestation | 认证器提供的设备型号/厂商证明 |
| 签名计数 | Sign Count | 认证器维护的递增计数器，防克隆 |

### 9.2 参考资料

- [W3C WebAuthn Level 3 规范](https://www.w3.org/TR/webauthn-3/)
- [FIDO Alliance Passkey 白皮书](https://fidoalliance.org/passkey/)
- [MDN WebAuthn 指南](https://developer.mozilla.org/en-US/docs/Web/API/Web_Authentication_API)
