# P0 模块安全评估报告

**评估日期**: 2026-03-23  
**评估范围**: internal/files, internal/cloudsync, internal/transfer, internal/replication, internal/auth, internal/rbac, internal/audit, internal/security  
**风险等级**: P0 (最高优先级)

---

## 一、执行摘要

本报告对 NAS-OS 核心模块进行了全面的安全评估，重点关注文件操作、数据同步、用户认证和权限控制等关键功能模块。评估发现系统已具备基础安全防护措施，但存在若干需要改进的安全风险点。

### 关键发现

| 风险等级 | 数量 | 主要领域 |
|---------|------|---------|
| 高危 | 3 | 凭证存储、命令注入、路径验证 |
| 中危 | 5 | 日志脱敏、会话管理、TLS配置 |
| 低危 | 4 | 缓存安全、错误信息泄露、默认配置 |

---

## 二、现有模块安全审查

### 2.1 internal/files 模块

#### 安全措施 (已有)

1. **路径遍历防护** ✅
   ```go
   // sanitizePath 函数实现了路径安全验证
   func sanitizePath(baseDir, userPath string) (string, error) {
       baseDir = filepath.Clean(baseDir)
       cleaned := filepath.Clean(filepath.Join(baseDir, userPath))
       if !strings.HasPrefix(cleaned, baseDir+string(filepath.Separator)) && cleaned != baseDir {
           return "", errors.New("path traversal detected")
       }
       return cleaned, nil
   }
   ```

2. **文件上传验证** ✅
   - 文件名使用 `filepath.Base()` 过滤
   - 阻止 `.` 和 `..` 作为文件名

3. **ZIP 解压缩防护** ✅
   ```go
   // extractZipGo 中有路径遍历检查
   if !strings.HasPrefix(filepath.Clean(path), filepath.Clean(destPath)+string(os.PathSeparator)) {
       return fmt.Errorf("非法文件路径: %s", f.Name)
   }
   ```

4. **视频缩略图命令超时** ✅
   ```go
   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
   ```

#### 安全风险

| 风险ID | 描述 | 严重程度 | 位置 |
|--------|------|---------|------|
| F-001 | 分享令牌使用内存存储，重启后丢失 | 中 | `shareStore` 变量 |
| F-002 | 文件预览无大小限制检查顺序问题 | 低 | `PreviewFile` 函数 |
| F-003 | 视频缩略图使用外部命令(ffmpeg)，存在资源耗尽风险 | 低 | `GenerateVideoThumbnail` |

#### 改进建议

```go
// F-001: 分享令牌应持久化存储
type ShareStore interface {
    Save(share *ShareInfo) error
    Load(token string) (*ShareInfo, error)
    Delete(token string) error
}

// F-002: 预览前先验证文件大小
func (m *Manager) PreviewFile(path string) (io.ReadCloser, string, error) {
    info, err := os.Stat(path)
    if err != nil {
        return nil, "", err
    }
    // 先检查大小再打开文件
    if info.Size() > m.config.MaxPreviewSize {
        return nil, "", fmt.Errorf("文件过大")
    }
    // ... 继续处理
}
```

---

### 2.2 internal/cloudsync 模块

#### 安全措施 (已有)

1. **WebDAV TLS 验证** ✅
   ```go
   // 仅测试环境允许跳过 TLS 验证
   if cfg.Insecure {
       if os.Getenv("ENV") != "test" {
           return nil, fmt.Errorf("生产环境禁止跳过 TLS 验证")
       }
   }
   ```

2. **提供商配置验证** ✅
   - 验证必需字段（AccessKey, SecretKey, Bucket 等）

3. **上下文超时控制** ✅
   - HTTP 客户端配置了超时

#### 安全风险

| 风险ID | 描述 | 严重程度 | 位置 |
|--------|------|---------|------|
| CS-001 | **敏感凭证明文存储** | 高 | `saveConfigLocked` |
| CS-002 | OAuth tokens 未加密存储 | 高 | `ProviderConfig` |
| CS-003 | 配置文件权限过宽 (0640) | 中 | `saveConfigLocked` |
| CS-004 | 缺少传输加密配置强制验证 | 中 | 各 Provider |

#### 改进建议

```go
// CS-001/CS-002: 敏感凭证加密存储
type EncryptedConfig struct {
    Data      []byte `json:"data"`       // 加密后的数据
    KeyHash   string `json:"key_hash"`   // 密钥哈希(用于验证)
    Algorithm string `json:"algorithm"`  // 加密算法
    Nonce     []byte `json:"nonce"`       // 加密随机数
}

func (m *Manager) saveConfigLocked() error {
    cfg := configData{
        Providers: m.providers,
        Tasks:     m.tasks,
    }
    
    // 序列化
    data, err := json.MarshalIndent(cfg, "", "  ")
    if err != nil {
        return err
    }
    
    // 加密敏感字段
    encrypted, err := m.encryptSensitiveFields(data)
    if err != nil {
        return err
    }
    
    // 写入文件，权限 0600
    return os.WriteFile(m.configPath, encrypted, 0600)
}

// CS-004: 强制 TLS 配置
type TLSConfig struct {
    MinVersion   uint16 `json:"min_version"`   // 最低 TLS 版本 (1.2+)
    CipherSuites []uint16 `json:"cipher_suites"` // 允许的加密套件
    VerifyPeer   bool   `json:"verify_peer"`    // 强制证书验证
}
```

---

### 2.3 internal/transfer 模块

#### 安全措施 (已有)

1. **路径安全验证** ✅
   ```go
   // 使用 pathutil.ValidatePath 验证所有路径
   if err := pathutil.ValidatePath(filePath); err != nil {
       return nil, fmt.Errorf("invalid file path: %w", err)
   }
   ```

2. **安全哈希算法** ✅
   - 使用 SHA256 替代 MD5 进行校验

3. **解压缩炸弹防护** ✅
   ```go
   const maxDecompressSize = 10 << 30 // 10GB
   limitedReader := io.LimitReader(gr, maxDecompressSize)
   ```

#### 安全风险

| 风险ID | 描述 | 严重程度 | 位置 |
|--------|------|---------|------|
| T-001 | 分块上传无并发限制 | 低 | `ChunkedUploader` |
| T-002 | 重试间隔固定，可能被利用 | 低 | `UploadChunk` |

#### 改进建议

```go
// T-001: 添加并发限制
type ChunkedUploader struct {
    chunkSize   int64
    maxRetries  int
    maxConcurrent int           // 新增
    semaphore   *semaphore.Weighted // 新增
}

func (u *ChunkedUploader) UploadMultipleChunks(ctx context.Context, chunks []ChunkInfo) error {
    for _, chunk := range chunks {
        if err := u.semaphore.Acquire(ctx, 1); err != nil {
            return err
        }
        go func(c ChunkInfo) {
            defer u.semaphore.Release(1)
            // 上传逻辑
        }(chunk)
    }
    return nil
}
```

---

### 2.4 internal/replication 模块

#### 安全措施 (已有)

1. **任务状态管理** ✅
   - 有完善的任务状态机

2. **冲突检测机制** ✅
   - 支持多种冲突解决策略

#### 安全风险

| 风险ID | 描述 | 严重程度 | 位置 |
|--------|------|---------|------|
| R-001 | **rsync 命令注入风险** | 高 | `executeSync` |
| R-002 | SSH 密钥路径未验证 | 中 | `Config.SSHKeyPath` |
| R-003 | 缺少路径验证 | 中 | `Task.SourcePath/TargetPath` |

#### 改进建议

```go
// R-001: 消除命令注入风险
func (m *Manager) executeSync(task *Task) {
    // 使用 Go 原生实现替代 rsync 命令
    // 或使用 exec.Command 的参数分离方式（已正确实现）
    
    // 验证路径
    if err := validateSyncPath(task.SourcePath); err != nil {
        task.Status = StatusError
        task.ErrorMessage = "无效源路径"
        return
    }
    if err := validateSyncPath(task.TargetPath); err != nil {
        task.Status = StatusError
        task.ErrorMessage = "无效目标路径"
        return
    }
    
    // 对于远程主机，验证主机名格式
    if task.TargetHost != "" {
        if !isValidHostname(task.TargetHost) {
            task.Status = StatusError
            task.ErrorMessage = "无效目标主机"
            return
        }
    }
    
    // 继续执行...
}

func validateSyncPath(path string) error {
    // 防止路径遍历
    if strings.Contains(path, "..") {
        return fmt.Errorf("路径不能包含 ..")
    }
    // 防止命令注入字符
    if strings.ContainsAny(path, ";|&`$(){}[]<>") {
        return fmt.Errorf("路径包含非法字符")
    }
    return nil
}

func isValidHostname(host string) bool {
    // 验证主机名格式
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9][-a-zA-Z0-9.]*$`, host)
    return matched
}
```

---

### 2.5 internal/auth 模块

#### 安全措施 (已有)

1. **多因素认证** ✅
   - 支持 TOTP、SMS、WebAuthn
   - 备份码机制

2. **密码安全** ✅
   - 使用 bcrypt 哈希
   - 支持密码策略

3. **会话管理** ✅
   - MFA 会话超时机制

#### 安全风险

| 风险ID | 描述 | 严重程度 | 位置 |
|--------|------|---------|------|
| A-001 | MFA 配置文件权限过宽 | 中 | `saveConfig` |
| A-002 | TOTP Secret 未加密存储 | 中 | `MFAConfig.TOTPSecret` |
| A-003 | 短信验证码重试无限制 | 中 | `SMSManager` |
| A-004 | MFA 会话内存存储 | 低 | `mfaSessions` |

#### 改进建议

```go
// A-001/A-002: 敏感配置加密存储
func (m *MFAManager) saveConfig() error {
    // 加密 TOTP Secret
    for _, cfg := range m.configs {
        if cfg.TOTPSecret != "" {
            encrypted, err := encryptSecret(cfg.TOTPSecret, m.encryptionKey)
            if err != nil {
                return err
            }
            cfg.TOTPSecretEncrypted = encrypted
            cfg.TOTPSecret = "" // 清除明文
        }
    }
    
    // 保存文件，权限 0600
    return os.WriteFile(m.configPath, data, 0600)
}

// A-003: 短信验证码重试限制
type SMSRateLimiter struct {
    attempts map[string]int
    blocked  map[string]time.Time
    mu       sync.RWMutex
}

func (r *SMSRateLimiter) CheckAndIncrement(phone string) error {
    r.mu.Lock()
    defer r.mu.Unlock()
    
    // 检查是否被封禁
    if unblockTime, ok := r.blocked[phone]; ok {
        if time.Now().Before(unblockTime) {
            return fmt.Errorf("验证码验证次数过多，请 %v 后重试", unblockTime.Sub(time.Now()))
        }
        delete(r.blocked, phone)
        r.attempts[phone] = 0
    }
    
    // 检查尝试次数
    if r.attempts[phone] >= 5 {
        r.blocked[phone] = time.Now().Add(15 * time.Minute)
        return fmt.Errorf("验证码验证次数过多，已临时封禁")
    }
    
    r.attempts[phone]++
    return nil
}
```

---

### 2.6 internal/rbac 模块

#### 安全措施 (已有)

1. **完整的 RBAC 模型** ✅
   - 用户、角色、权限、策略
   - 组权限继承

2. **权限缓存** ✅
   - 减少频繁计算
   - 支持缓存失效

3. **审计回调** ✅
   - 权限检查可记录

#### 安全风险

| 风险ID | 描述 | 严重程度 | 位置 |
|--------|------|---------|------|
| RBAC-001 | 权限缓存可能被绕过 | 低 | `getCachedPermissions` |
| RBAC-002 | 配置文件权限 0600 应强制 | 低 | `save` |

#### 改进建议

```go
// RBAC-001: 加强缓存安全
func (m *Manager) getCachedPermissions(userID string) *PermissionCache {
    m.cacheMu.RLock()
    defer m.cacheMu.RUnlock()
    
    cached, exists := m.cache[userID]
    if !exists {
        return nil
    }
    
    // 验证缓存完整性
    if !m.validateCacheIntegrity(cached) {
        m.invalidateCache(userID)
        return nil
    }
    
    if time.Now().After(cached.ExpiresAt) {
        return nil
    }
    
    return cached
}
```

---

### 2.7 internal/audit 模块

#### 安全措施 (已有)

1. **日志完整性保护** ✅
   ```go
   // HMAC 签名验证
   func (m *Manager) generateSignature(entry *Entry) string {
       signData := fmt.Sprintf("%s|%s|%s|...", ...)
       h := hmac.New(sha256.New, m.signingKey)
       h.Write([]byte(signData))
       return hex.EncodeToString(h.Sum(nil))
   }
   ```

2. **XML 注入防护** ✅
   ```go
   func escapeXML(s string) string {
       var buf strings.Builder
       xml.Escape(&buf, []byte(s))
       return buf.String()
   }
   ```

3. **敏感信息过滤** ✅
   - 导出时可选择不包含签名

#### 安全风险

| 风险ID | 描述 | 严重程度 | 位置 |
|--------|------|---------|------|
| AUD-001 | 日志中可能包含敏感信息 | 中 | 各 `Log*` 方法 |
| AUD-002 | 签名密钥内存存储 | 中 | `signingKey` |
| AUD-003 | 日志文件权限 0600 可改进 | 低 | `save` |

#### 改进建议

```go
// AUD-001: 日志脱敏
type SensitiveField struct {
    Name     string
    Patterns []*regexp.Regexp
}

var sensitivePatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)password["\s:=]+[^,}\s]+`),
    regexp.MustCompile(`(?i)secret["\s:=]+[^,}\s]+`),
    regexp.MustCompile(`(?i)token["\s:=]+[^,}\s]+`),
    regexp.MustCompile(`(?i)key["\s:=]+[^,}\s]+`),
}

func sanitizeLogEntry(entry *Entry) *Entry {
    sanitized := *entry
    
    // 脱敏消息
    sanitized.Message = sanitizeString(entry.Message)
    
    // 脱敏详情
    if entry.Details != nil {
        sanitized.Details = sanitizeMap(entry.Details)
    }
    
    return &sanitized
}

func sanitizeString(s string) string {
    result := s
    for _, pattern := range sensitivePatterns {
        result = pattern.ReplaceAllStringFunc(result, func(match string) string {
            parts := strings.SplitN(match, ":", 2)
            if len(parts) == 2 {
                return parts[0] + ": [REDACTED]"
            }
            return "[REDACTED]"
        })
    }
    return result
}
```

---

## 三、Drive 模块安全建议

### 3.1 架构设计原则

```
┌─────────────────────────────────────────────────────────────┐
│                     API Gateway Layer                        │
│  - 请求验证                                                  │
│  - 速率限制                                                  │
│  - 认证/授权                                                │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                     Security Layer                           │
│  - 路径验证                                                  │
│  - 权限检查                                                  │
│  - 审计日志                                                  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                     Business Layer                           │
│  - 文件操作                                                  │
│  - 同步逻辑                                                  │
│  - 冲突处理                                                  │
└─────────────────────────────────────────────────────────────┘
                              │
┌─────────────────────────────────────────────────────────────┐
│                     Storage Layer                            │
│  - 本地存储                                                  │
│  - 云存储                                                    │
│  - 加密存储                                                  │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 关键安全要求

#### 3.2.1 文件操作安全

```go
// Drive 文件操作安全接口
type SecureFileOperation interface {
    // 验证路径是否在用户授权范围内
    ValidatePath(userID, path string) error
    
    // 检查用户对指定路径的权限
    CheckPermission(userID, path, action string) (*PermissionResult, error)
    
    // 记录操作审计日志
    AuditOperation(userID, path, action string, result error)
    
    // 执行安全文件操作
    Execute(ctx context.Context, op FileOperation) error
}

type FileOperation struct {
    Type      OperationType // read, write, delete, etc.
    UserID    string
    Path      string
    Options   map[string]interface{}
    Timestamp time.Time
    RequestID string
}
```

#### 3.2.2 同步安全

```go
// Drive 同步安全配置
type SyncSecurityConfig struct {
    // 传输安全
    TLSMinVersion      uint16   // 最低 TLS 1.2
    AllowedCiphers     []uint16 // 安全加密套件
    CertVerification   bool     // 强制证书验证
    
    // 数据安全
    EncryptionAtRest   bool     // 静态加密
    EncryptionInTransit bool    // 传输加密
    KeyRotationDays    int      // 密钥轮换周期
    
    // 访问控制
    MaxConcurrentOps   int      // 最大并发操作数
    RateLimitRPM       int      // 每分钟请求限制
    SessionTimeout     time.Duration // 会话超时
    
    // 审计
    AuditAllOps        bool     // 审计所有操作
    RetentionDays      int      // 日志保留天数
}
```

---

## 四、数据传输加密方案

### 4.1 传输层加密

```go
// TLS 配置标准
var DefaultTLSConfig = &tls.Config{
    MinVersion: tls.VersionTLS12,
    CipherSuites: []uint16{
        tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
        tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
        tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
    },
    PreferServerCipherSuites: true,
    CurvePreferences: []tls.CurveID{
        tls.CurveP521,
        tls.CurveP384,
        tls.X25519,
    },
}
```

### 4.2 数据加密

```go
// 数据加密管理器
type DataEncryptionManager struct {
    masterKey    []byte
    keyProvider  KeyProvider
    algorithm    string // AES-256-GCM
}

type KeyProvider interface {
    // 获取数据加密密钥
    GetDataKey(keyID string) (*DataKey, error)
    
    // 生成新数据加密密钥
    GenerateDataKey() (*DataKey, error)
    
    // 轮换密钥
    RotateKey(keyID string) (*DataKey, error)
}

type DataKey struct {
    ID        string
    Key       []byte
    CreatedAt time.Time
    ExpiresAt *time.Time
}

// 加密文件内容
func (m *DataEncryptionManager) EncryptFile(srcPath, dstPath string) error {
    // 1. 生成数据密钥
    dataKey, err := m.keyProvider.GenerateDataKey()
    if err != nil {
        return err
    }
    
    // 2. 读取文件内容
    data, err := os.ReadFile(srcPath)
    if err != nil {
        return err
    }
    
    // 3. 加密数据
    block, _ := aes.NewCipher(dataKey.Key)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, nonce)
    encrypted := gcm.Seal(nil, nonce, data, nil)
    
    // 4. 写入加密文件（包含密钥ID和nonce）
    // 格式: keyID(32) + nonce(12) + encrypted_data
    output := append([]byte(dataKey.ID), nonce...)
    output = append(output, encrypted...)
    
    return os.WriteFile(dstPath, output, 0600)
}
```

---

## 五、访问控制设计

### 5.1 权限模型

```
┌─────────────────────────────────────────────────────────────┐
│                     Resource Hierarchy                       │
│                                                              │
│   Drive (drive:xxx)                                         │
│   ├── Folder (folder:xxx)                                   │
│   │   ├── File (file:xxx)                                   │
│   │   └── Folder (folder:yyy)                               │
│   └── File (file:yyy)                                       │
│                                                              │
│   Permissions:                                              │
│   - drive:read      - 读取驱动器信息                        │
│   - drive:write     - 修改驱动器配置                        │
│   - folder:read     - 读取文件夹内容                        │
│   - folder:write    - 在文件夹中创建/修改                   │
│   - folder:delete   - 删除文件夹                            │
│   - file:read       - 读取文件内容                          │
│   - file:write      - 修改文件                              │
│   - file:delete     - 删除文件                              │
│   - share:create    - 创建分享链接                          │
│   - share:manage    - 管理分享链接                          │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 访问控制实现

```go
// Drive 访问控制管理器
type DriveAccessControl struct {
    rbac       *rbac.Manager
    audit      *audit.Manager
    shareACL   *ShareACLManager
}

func (ac *DriveAccessControl) CheckAccess(ctx context.Context, userID, resource, action string) (*AccessResult, error) {
    result := &AccessResult{
        Allowed: false,
        Reason:  "默认拒绝",
    }
    
    // 1. 检查用户权限
    permResult := ac.rbac.CheckPermission(userID, resource, action)
    if !permResult.Allowed {
        result.Reason = permResult.Reason
        ac.audit.LogAccess(userID, "", "", resource, action, audit.StatusFailure, map[string]interface{}{
            "reason": permResult.Reason,
        })
        return result, nil
    }
    
    // 2. 检查分享 ACL（如果是分享访问）
    if shareID := ctx.Value("share_id"); shareID != nil {
        shareResult := ac.shareACL.CheckShareAccess(shareID.(string), userID, action)
        if !shareResult.Allowed {
            result.Reason = "分享权限不足"
            return result, nil
        }
    }
    
    // 3. 记录审计日志
    ac.audit.LogAccess(userID, "", "", resource, action, audit.StatusSuccess, nil)
    
    result.Allowed = true
    result.Reason = "权限验证通过"
    return result, nil
}
```

---

## 六、审计日志要求

### 6.1 审计事件定义

```go
// Drive 审计事件类型
const (
    // 文件操作
    EventFileRead     = "file.read"
    EventFileWrite    = "file.write"
    EventFileDelete   = "file.delete"
    EventFileUpload   = "file.upload"
    EventFileDownload = "file.download"
    
    // 同步操作
    EventSyncStart    = "sync.start"
    EventSyncComplete = "sync.complete"
    EventSyncError    = "sync.error"
    EventConflict     = "sync.conflict"
    
    // 分享操作
    EventShareCreate  = "share.create"
    EventShareAccess  = "share.access"
    EventShareRevoke  = "share.revoke"
    
    // 安全事件
    EventAccessDenied = "security.access_denied"
    EventRateLimit    = "security.rate_limit"
    EventSuspicious   = "security.suspicious"
)

// 审计日志字段规范
type DriveAuditEntry struct {
    // 基本信息
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    
    // 事件信息
    Event     string `json:"event"`
    Category  string `json:"category"`
    Level     string `json:"level"` // info, warning, error, critical
    
    // 用户信息
    UserID    string `json:"user_id"`
    Username  string `json:"username"`
    SessionID string `json:"session_id,omitempty"`
    
    // 客户端信息
    ClientIP  string `json:"client_ip"`
    UserAgent string `json:"user_agent,omitempty"`
    
    // 资源信息
    ResourceType string `json:"resource_type"`
    ResourceID   string `json:"resource_id"`
    ResourcePath string `json:"resource_path,omitempty"`
    
    // 操作信息
    Action     string                 `json:"action"`
    Status     string                 `json:"status"` // success, failure
    Message    string                 `json:"message"`
    Details    map[string]interface{} `json:"details,omitempty"`
    
    // 安全信息
    RiskScore  int    `json:"risk_score,omitempty"`  // 0-100
    AlertLevel string `json:"alert_level,omitempty"` // none, low, medium, high, critical
    
    // 完整性
    Signature  string `json:"signature,omitempty"`
}
```

### 6.2 日志脱敏规则

```go
// 敏感字段脱敏规则
type SanitizationRule struct {
    Field     string
    Action    string // redact, hash, truncate
    Pattern   *regexp.Regexp
}

var DefaultSanitizationRules = []SanitizationRule{
    {Field: "password", Action: "redact", Pattern: regexp.MustCompile(`.********`)},
    {Field: "token", Action: "hash", Pattern: nil},
    {Field: "secret", Action: "redact", Pattern: nil},
    {Field: "api_key", Action: "redact", Pattern: nil},
    {Field: "content", Action: "truncate", Pattern: nil}, // 内容字段截断
}

func (e *DriveAuditEntry) Sanitize() *DriveAuditEntry {
    sanitized := *e
    
    // 对详情中的敏感字段脱敏
    if e.Details != nil {
        sanitized.Details = make(map[string]interface{})
        for k, v := range e.Details {
            if isSensitiveField(k) {
                sanitized.Details[k] = sanitizeValue(k, v)
            } else {
                sanitized.Details[k] = v
            }
        }
    }
    
    return &sanitized
}
```

---

## 七、风险修复优先级

### 7.1 立即修复 (P0)

| 风险ID | 描述 | 修复方案 |
|--------|------|---------|
| CS-001 | 敏感凭证明文存储 | 实现加密存储机制 |
| R-001 | rsync 命令注入风险 | 使用 Go 原生实现或参数化命令 |

### 7.2 短期修复 (P1)

| 风险ID | 描述 | 修复方案 |
|--------|------|---------|
| CS-002 | OAuth tokens 未加密存储 | 集成到凭证加密系统 |
| A-002 | TOTP Secret 未加密存储 | 使用硬件安全模块或密钥管理服务 |
| AUD-001 | 日志中敏感信息 | 实现自动化脱敏 |

### 7.3 中期改进 (P2)

| 风险ID | 描述 | 修复方案 |
|--------|------|---------|
| F-001 | 分享令牌内存存储 | 持久化存储 |
| A-003 | 短信验证码重试限制 | 实现速率限制 |
| CS-003 | 配置文件权限 | 强制 0600 权限 |

---

## 八、附录

### A. 安全检查清单

- [ ] 所有用户输入都经过验证和清理
- [ ] 敏感数据在存储前加密
- [ ] 所有 API 端点都有认证保护
- [ ] 权限检查在操作前执行
- [ ] 审计日志记录所有敏感操作
- [ ] 错误消息不泄露敏感信息
- [ ] 配置文件权限正确 (0600)
- [ ] TLS 配置符合安全标准
- [ ] 密钥和凭证定期轮换
- [ ] 有入侵检测和响应机制

### B. 参考资料

1. OWASP Top 10 Web Application Security Risks
2. NIST Cybersecurity Framework
3. CIS Benchmarks for NAS Systems
4. Go Security Best Practices

---

**报告编写**: 刑部安全评估组  
**审核日期**: 2026-03-23  
**下次审核**: 建议每季度重新评估