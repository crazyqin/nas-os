# AI服务安全设计

**版本**: v2.308.0  
**创建日期**: 2026-03-29  
**作者**: 刑部安全审计

---

## 1. API密钥管理

### 1.1 密钥存储架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                     API密钥管理架构                                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│   ┌─────────────────────────────────────────────────────────────┐   │
│   │                    密钥存储层                                  │   │
│   ├─────────────────────────────────────────────────────────────┤   │
│   │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │   │
│   │  │ 环境变量 │  │ 加密配置 │  │ 密钥文件 │  │ Vault   │        │   │
│   │  │ (临时)  │  │ (推荐)  │  │ (加密)  │  │ (可选) │        │   │
│   │  └─────────┘  └─────────┘  └─────────┘  └─────────┘        │   │
│   └─────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│   ┌─────────────────────────────────────────────────────────────┐   │
│   │                    密钥访问层                                  │   │
│   ├─────────────────────────────────────────────────────────────┤   │
│   │  ┌─────────────────┐  ┌─────────────────┐                  │   │
│   │  │ KeyManager API  │  │ 访问审计日志    │                  │   │
│   │  │ - GetKey()      │  │ - 记录所有访问  │                  │   │
│   │  │ - RotateKey()   │  │ - 异常检测     │                  │   │
│   │  │ - RevokeKey()   │  │ - 访问限制     │                  │   │
│   │  └─────────────────┘  └─────────────────┘                  │   │
│   └─────────────────────────────────────────────────────────────┘   │
│                               │                                      │
│                               ▼                                      │
│   ┌─────────────────────────────────────────────────────────────┐   │
│   │                    AI服务层                                   │   │
│   │  OpenAI  │  Gemini  │  Claude  │  AzureOpenAI │ ...        │   │
│   └─────────────────────────────────────────────────────────────┘   │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### 1.2 密钥存储策略

```go
// KeyStorage 密钥存储配置
type KeyStorage struct {
    Primary   StorageType  // 主存储方式
    Secondary StorageType  // 备用存储方式
    Encrypted bool         // 是否加密存储
    Rotation  bool         // 是否支持轮换
}

// 推荐存储配置
var RecommendedKeyStorage = KeyStorage{
    Primary:   StorageEncryptedConfig,  // 加密配置文件
    Secondary: StorageEnvVar,           // 环境变量备用
    Encrypted: true,
    Rotation:  true,
}

// 存储类型优先级
// 1. 加密配置文件 (最推荐) - ~/.nas/ai_keys.enc
// 2. HashiCorp Vault (企业级) - vault.nas.local
// 3. 环境变量 (临时/开发) - NAS_AI_OPENAI_KEY
// 4. 禁止: 硬编码、明文配置文件、日志输出
```

### 1.3 密钥加密实现

```go
// KeyEncryptor 密钥加密器
type KeyEncryptor struct {
    masterKey []byte  // 主密钥 (从环境变量或文件加载)
    algorithm string  // 加密算法
}

// EncryptKey 加密API密钥
func (ke *KeyEncryptor) EncryptKey(apiKey string) ([]byte, error) {
    // 使用 AES-256-GCM 加密
    block, err := aes.NewCipher(ke.masterKey)
    if err != nil {
        return nil, err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    
    return gcm.Seal(nonce, nonce, []byte(apiKey), nil), nil
}

// DecryptKey 解密API密钥
func (ke *KeyEncryptor) DecryptKey(encrypted []byte) (string, error) {
    block, err := aes.NewCipher(ke.masterKey)
    if err != nil {
        return "", err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    
    nonceSize := gcm.NonceSize()
    if len(encrypted) < nonceSize {
        return "", errors.New("invalid encrypted data")
    }
    
    nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
    decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }
    
    return string(decrypted), nil
}
```

### 1.4 密钥轮换机制

```go
// KeyRotation 密钥轮换配置
type KeyRotation struct {
    Enabled        bool          // 是否启用轮换
    IntervalDays   int           // 轮换间隔 (天)
    GracePeriodDays int          // 过渡期 (天)
    NotifyDaysBefore int         // 提前通知天数
}

// RotateKey 执行密钥轮换
func (km *KeyManager) RotateKey(provider string) error {
    // 1. 生成新密钥标识
    newKeyID := uuid.New().String()
    
    // 2. 存储新密钥 (加密)
    newKey, err := km.generateNewKey(provider)
    if err != nil {
        return err
    }
    
    encrypted, err := km.encryptor.EncryptKey(newKey)
    if err != nil {
        return err
    }
    
    // 3. 设置过渡期 (旧密钥仍可用)
    km.setGracePeriod(provider, km.config.Rotation.GracePeriodDays)
    
    // 4. 更新密钥映射
    km.keys[provider] = KeyRecord{
        ID:        newKeyID,
        Key:       encrypted,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().AddDate(0, 0, km.config.Rotation.IntervalDays),
        Status:    KeyStatusActive,
    }
    
    // 5. 记录审计日志
    km.auditLog.LogRotation(provider, newKeyID)
    
    // 6. 发送通知
    km.notifyKeyRotation(provider)
    
    return nil
}
```

---

## 2. 请求限流

### 2.1 多层限流架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                      请求限流架构                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│   用户请求                                                            │
│       │                                                               │
│       ▼                                                               │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │ Layer 1: 全局限流                                              │ │
│   │ - 总 QPS: 100/s                                                │ │
│   │ - 并发连接: 50                                                 │ │
│   │ - 拒绝策略: HTTP 429                                           │ │
│   └───────────────────────────────────────────────────────────────┘ │
│       │                                                               │
│       ▼                                                               │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │ Layer 2: 用户限流                                              │ │
│   │ - 单用户 QPS: 10/s                                             │ │
│   │ - 单用户分钟: 100/min                                          │ │
│   │ - 单用户小时: 1000/h                                           │ │
│   └───────────────────────────────────────────────────────────────┘ │
│       │                                                               │
│       ▼                                                               │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │ Layer 3: Provider 限流                                         │ │
│   │ - OpenAI: 60/min (API限制)                                     │ │
│   │ - Gemini: 60/min                                               │ │
│   │ - Claude: 根据订阅计划                                         │ │
│   └───────────────────────────────────────────────────────────────┘ │
│       │                                                               │
│       ▼                                                               │
│   AI Provider API                                                     │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### 2.2 限流器实现

```go
// RateLimiterConfig 限流配置
type RateLimiterConfig struct {
    Global        LimitConfig  // 全局限制
    PerUser       LimitConfig  // 用户限制
    PerProvider   map[string]LimitConfig  // Provider限制
}

type LimitConfig struct {
    QPS       int           // 每秒请求
    QPM       int           // 每分钟请求
    QPH       int           // 每小时请求
    Burst     int           // 突发容量
    Strategy  LimitStrategy // 拒绝策略
}

// 多层限流器
type MultiLayerRateLimiter struct {
    global    *tokenBucketLimiter
    users     map[string]*tokenBucketLimiter  // 用户级别
    providers map[string]*tokenBucketLimiter  // Provider级别
    lock      sync.RWMutex
}

// Allow 检查是否允许请求
func (mlrl *MultiLayerRateLimiter) Allow(userID, provider string) bool {
    // 1. 全局限流检查
    if !mlrl.global.Allow() {
        return false
    }
    
    // 2. 用户限流检查
    mlrl.lock.RLock()
    userLimiter, exists := mlrl.users[userID]
    mlrl.lock.RUnlock()
    
    if exists {
        if !userLimiter.Allow() {
            return false
        }
    } else {
        // 创建新用户限流器
        mlrl.createUserLimiter(userID)
    }
    
    // 3. Provider限流检查
    mlrl.lock.RLock()
    providerLimiter, exists := mlrl.providers[provider]
    mlrl.lock.RUnlock()
    
    if exists && !providerLimiter.Allow() {
        return false
    }
    
    return true
}

// Wait 等待直到允许
func (mlrl *MultiLayerRateLimiter) Wait(userID, provider string, timeout time.Duration) error {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    // 等待全局限流
    if err := mlrl.global.Wait(ctx); err != nil {
        return err
    }
    
    // 等待用户限流
    mlrl.lock.RLock()
    userLimiter := mlrl.users[userID]
    mlrl.lock.RUnlock()
    
    if userLimiter != nil {
        if err := userLimiter.Wait(ctx); err != nil {
            return err
        }
    }
    
    // 等待Provider限流
    mlrl.lock.RLock()
    providerLimiter := mlrl.providers[provider]
    mlrl.lock.RUnlock()
    
    if providerLimiter != nil {
        if err := providerLimiter.Wait(ctx); err != nil {
            return err
        }
    }
    
    return nil
}
```

### 2.3 限流响应处理

```go
// HandleRateLimit 处理限流响应
func HandleRateLimit(resp http.ResponseWriter, reason string, retryAfter int) {
    resp.Header().Set("X-RateLimit-Limit", "100")
    resp.Header().Set("X-RateLimit-Remaining", "0")
    resp.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Duration(retryAfter)*time.Second).Unix()))
    resp.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
    
    resp.WriteHeader(http.StatusTooManyRequests)
    
    json.NewEncoder(resp).Encode(RateLimitError{
        Error:   "rate_limit_exceeded",
        Message: fmt.Sprintf("请求频率超限: %s", reason),
        RetryAfter: retryAfter,
    })
}

// Provider限流重试
func (ac *AIClient) callWithRetry(ctx context.Context, req AIRequest) (*AIResponse, error) {
    maxRetries := 3
    retryDelay := 1 * time.Second
    
    for i := 0; i < maxRetries; i++ {
        resp, err := ac.call(ctx, req)
        if err == nil {
            return resp, nil
        }
        
        // 检查是否为限流错误
        if isRateLimitError(err) {
            retryAfter := getRetryAfter(err)
            if retryAfter > 0 {
                retryDelay = time.Duration(retryAfter) * time.Second
            }
            
            ac.limiter.Wait(ctx, retryDelay)
            continue
        }
        
        // 其他错误直接返回
        return nil, err
    }
    
    return nil, errors.New("max retries exceeded")
}
```

---

## 3. 数据脱敏流程

### 3.1 数据脱敏架构

```
┌─────────────────────────────────────────────────────────────────────┐
│                      数据脱敏流程                                    │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│   用户输入                                                            │
│       │                                                               │
│       ▼                                                               │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │ Step 1: 识别敏感数据                                           │ │
│   │ - 正则匹配: 手机号、邮箱、身份证、银行卡                        │ │
│   │ - 关键词匹配: password, token, secret                          │ │
│   │ - NER识别: 姓名、地址、公司名称                                │ │
│   └───────────────────────────────────────────────────────────────┘ │
│       │                                                               │
│       ▼                                                               │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │ Step 2: 分类标记                                               │ │
│   │ - PII (个人身份信息)                                           │ │
│   │ - FIN (金融信息)                                               │ │
│   │ - AUTH (认证信息)                                              │ │
│   │ - MED (医疗信息)                                               │ │
│   └───────────────────────────────────────────────────────────────┘ │
│       │                                                               │
│       ▼                                                               │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │ Step 3: 脱敏处理                                               │ │
│   │ - 替换: 张三 → 张*                                             │ │
│   │ - 部分掩码: 13812345678 → 138****5678                          │ │
│   │ - 哈希: password123 → [REDACTED_AUTH]                          │ │
│   │ - 占位符: 身份证 → [ID_NUMBER]                                 │ │
│   └───────────────────────────────────────────────────────────────┘ │
│       │                                                               │
│       ▼                                                               │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │ Step 4: 发送到AI服务                                           │ │
│   │ - 记录脱敏日志                                                 │ │
│   │ - 保留原始数据映射 (可选恢复)                                  │ │
│   └───────────────────────────────────────────────────────────────┘ │
│       │                                                               │
│       ▼                                                               │
│   AI响应                                                              │
│       │                                                               │
│       ▼                                                               │
│   ┌───────────────────────────────────────────────────────────────┐ │
│   │ Step 5: 响应处理                                               │ │
│   │ - 检查响应是否包含敏感数据                                     │ │
│   │ - 清理日志中的敏感信息                                         │ │
│   └───────────────────────────────────────────────────────────────┘ │
│       │                                                               │
│       ▼                                                               │
│   返回用户                                                            │
│                                                                       │
└─────────────────────────────────────────────────────────────────────┘
```

### 3.2 敏感数据识别规则

```go
// SensitiveDataPatterns 敏感数据识别模式
var SensitiveDataPatterns = []SensitivePattern{
    // 个人身份信息 (PII)
    {
        Type:     DataTypePII,
        Category: "phone_cn",
        Pattern:  regexp.MustCompile(`1[3-9]\d{9}`),
        Mask:     MaskPartial,  // 138****5678
    },
    {
        Type:     DataTypePII,
        Category: "email",
        Pattern:  regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
        Mask:     MaskPartial,  // a***@b.com
    },
    {
        Type:     DataTypePII,
        Category: "id_cn",
        Pattern:  regexp.MustCompile(`\d{17}[\dXx]`),
        Mask:     MaskPlaceholder,  // [ID_NUMBER]
    },
    {
        Type:     DataTypePII,
        Category: "name",
        Pattern:  regexp.MustCompile(`(?:姓名|名字)[:：]\s*(\S{2,4})`),
        Mask:     MaskReplace,  // 张三 → 张*
    },
    
    // 金融信息 (FIN)
    {
        Type:     DataTypeFIN,
        Category: "bank_card",
        Pattern:  regexp.MustCompile(`\d{16,19}`),
        Mask:     MaskPartial,  // 6222****1234
    },
    {
        Type:     DataTypeFIN,
        Category: "credit_card",
        Pattern:  regexp.MustCompile(`\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4}`),
        Mask:     MaskPartial,
    },
    
    // 认证信息 (AUTH)
    {
        Type:     DataTypeAUTH,
        Category: "password",
        Pattern:  regexp.MustCompile(`(?i)(?:password|密码|pwd)[:：]\s*\S+`),
        Mask:     MaskRedact,  // [REDACTED]
    },
    {
        Type:     DataTypeAUTH,
        Category: "token",
        Pattern:  regexp.MustCompile(`(?i)(?:token|令牌|密钥)[:：]\s*[a-zA-Z0-9_-]{20,}`),
        Mask:     MaskRedact,
    },
    {
        Type:     DataTypeAUTH,
        Category: "api_key",
        Pattern:  regexp.MustCompile(`(?i)(?:api[_-]?key|密钥)[:：]\s*[a-zA-Z0-9]{32,}`),
        Mask:     MaskRedact,
    },
}
```

### 3.3 脱敏处理器实现

```go
// DataSanitizer 数据脱敏器
type DataSanitizer struct {
    patterns []SensitivePattern
    logger   *zap.Logger
    options  SanitizerOptions
}

type SanitizerOptions struct {
    LogOriginal    bool  // 是否记录原始数据 (默认 false)
    PreserveMap    bool  // 是否保留映射 (用于恢复)
    StrictMode     bool  // 严格模式 (遇到未识别敏感数据则拒绝)
}

// Sanitize 执行数据脱敏
func (ds *DataSanitizer) Sanitize(input string) (*SanitizedResult, error) {
    result := &SanitizedResult{
        Original:  input,  // 仅在 LogOriginal=true 时保留
        Sanitized: input,
        Detected:  []DetectedSensitive{},
    }
    
    // 按优先级顺序处理
    order := []DataType{DataTypeAUTH, DataTypeFIN, DataTypePII}
    
    for _, dataType := range order {
        for _, pattern := range ds.patterns {
            if pattern.Type != dataType {
                continue
            }
            
            matches := pattern.Pattern.FindAllStringSubmatchIndex(input, -1)
            for _, match := range matches {
                // 记录检测结果
                detected := DetectedSensitive{
                    Type:     pattern.Type,
                    Category: pattern.Category,
                    Position: Position{Start: match[0], End: match[1]},
                    Original: input[match[0]:match[1]],
                }
                
                // 应用脱敏
                masked := ds.applyMask(pattern.Mask, detected.Original)
                result.Sanitized = strings.Replace(result.Sanitized, detected.Original, masked, 1)
                
                // 记录映射 (可选)
                if ds.options.PreserveMap {
                    result.Mapping = append(result.Mapping, MappingEntry{
                        Original: detected.Original,
                        Masked:   masked,
                    })
                }
                
                result.Detected = append(result.Detected, detected)
            }
        }
    }
    
    // 记录审计日志
    ds.logSanitization(result)
    
    return result, nil
}

// applyMask 应用脱敏策略
func (ds *DataSanitizer) applyMask(maskType MaskType, value string) string {
    switch maskType {
    case MaskReplace:
        // 替换部分字符
        if len(value) <= 2 {
            return "*"
        }
        return value[:1] + "*" + value[len(value)-1:]
        
    case MaskPartial:
        // 保留部分可见
        if len(value) <= 4 {
            return "***" + value[len(value)-1:]
        }
        return value[:3] + "****" + value[len(value)-4:]
        
    case MaskRedact:
        // 完全移除
        return "[REDACTED]"
        
    case MaskPlaceholder:
        // 占位符替换
        return "[" + ds.getPlaceholder(value) + "]"
        
    default:
        return value
    }
}
```

### 3.4 响应数据清理

```go
// ResponseCleaner 响应数据清理器
func (ds *DataSanitizer) CleanResponse(response string) string {
    cleaned := response
    
    // 1. 检查响应中是否意外包含敏感数据
    for _, pattern := range ds.patterns {
        matches := pattern.Pattern.FindAllString(cleaned, -1)
        for _, match := range matches {
            // 移除或替换
            cleaned = strings.ReplaceAll(cleaned, match, ds.applyMask(pattern.Mask, match))
        }
    }
    
    // 2. 清理可能的泄露模式
    leakPatterns := []string{
        "password",
        "secret",
        "token",
        "api_key",
        "credential",
    }
    
    for _, leak := range leakPatterns {
        // 移除包含敏感关键词的行
        lines := strings.Split(cleaned, "\n")
        filteredLines := []string{}
        for _, line := range lines {
            if !strings.Contains(strings.ToLower(line), leak) {
                filteredLines = append(filteredLines, line)
            }
        }
        cleaned = strings.Join(filteredLines, "\n")
    }
    
    return cleaned
}
```

---

## 4. 实现路线图

### 4.1 Phase 1 - 密钥管理 (P0)

**时间**: 2026-03-29 ~ 2026-04-05

| 任务 | 状态 | 优先级 |
|------|------|--------|
| 密钥加密存储 | 已有基础 | P0 |
| 密钥访问审计 | 待实现 | P0 |
| 禁止硬编码密钥 | 需审查 | P0 |
| 密钥轮换框架 | 待实现 | P1 |

### 4.2 Phase 2 - 限流系统 (P0)

**时间**: 2026-04-06 ~ 2026-04-15

| 任务 | 状态 | 优先级 |
|------|------|--------|
| 全局限流器 | 已有基础 | P0 |
| 用户级别限流 | 待实现 | P0 |
| Provider限流 | 待实现 | P0 |
| 限流告警 | 待实现 | P1 |

### 4.3 Phase 3 - 数据脱敏 (P0)

**时间**: 2026-04-16 ~ 2026-04-25

| 任务 | 状态 | 优先级 |
|------|------|--------|
| PII识别规则 | 待实现 | P0 |
| AUTH脱敏规则 | 待实现 | P0 |
| FIN脱敏规则 | 待实现 | P0 |
| 响应清理 | 待实现 | P1 |

---

## 5. 安全检查清单

### 5.1 代码审查清单

- [ ] 无硬编码API密钥
- [ ] 密钥不在日志中输出
- [ ] 密钥不在错误消息中暴露
- [ ] 使用加密存储密钥
- [ ] 实现密钥访问审计
- [ ] 配置合理的限流参数
- [ ] 输入数据经过脱敏处理
- [ ] 响应数据经过清理

### 5.2 配置审查清单

- [ ] 限流配置合理
- [ ] 密钥轮换已配置
- [ ] 敏感数据类型已定义
- [ ] 脱敏规则已测试
- [ ] 告警渠道已配置

---

## 6. 当前代码审查结果

### 6.1 发现问题

| 问题 | 位置 | 严重程度 | 建议 |
|------|------|---------|------|
| 硬编码默认密码 | `internal/apps/catalog.go:303` | 中 | PostgreSQL模板默认密码需提示用户修改 |
| OAuth URL配置 | `internal/auth/oauth2.go` | 低 | OAuth URL为公开配置，非敏感 |
| 限流器已实现 | `internal/concurrency/rate_limiter.go` | - | 已有基础限流实现 |

### 6.2 安全配置建议

```yaml
# AI服务安全配置示例
ai_service:
  key_management:
    storage: encrypted
    rotation_interval_days: 90
    grace_period_days: 7
    
  rate_limiting:
    global_qps: 100
    user_qps: 10
    user_qpm: 100
    provider_limits:
      openai: 60/min
      gemini: 60/min
      
  data_sanitization:
    enabled: true
    strict_mode: true
    log_original: false
    preserve_mapping: false
    
    sensitive_types:
      - pii
      - fin
      - auth
      
    placeholder_patterns:
      phone: "[PHONE_NUMBER]"
      email: "[EMAIL_ADDRESS]"
      id: "[ID_NUMBER]"
      password: "[REDACTED]"
```

---

**文档状态**: 已创建  
**下一步**: 实现 Phase 1 密钥管理审计