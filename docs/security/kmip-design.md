# KMIP密钥管理设计文档

**版本**: v1.0  
**日期**: 2026-04-09  
**对标**: TrueNAS Enterprise KMIP集成  
**参考**: internal/security/kmip.go

---

## 1. KMIP协议概述

### 1.1 协议背景

KMIP (Key Management Interoperability Protocol) 是OASIS标准化的密钥管理协议，版本2.0于2020年发布。

**核心价值**:
- 统一的密钥管理接口
- 支持多种密钥类型（对称、非对称、证书）
- 标准化的生命周期管理
- 与企业KMS无缝集成

### 1.2 协议架构

```
┌─────────────────┐     KMIP 2.0      ┌─────────────────┐
│   NAS-OS        │◄────────────────►│   External KMS  │
│   KMIP Client   │    TCP/5696      │   (KMIP Server) │
│                 │    TLS加密       │                 │
└─────────────────┘                  └─────────────────┘
```

**传输层**:
- 默认端口: 5696
- 强制TLS 1.2+
- 双向证书认证（可选）

---

## 2. TrueNAS Enterprise KMIP对标分析

### 2.1 TrueNAS KMIP功能

| 功能 | TrueNAS Enterprise | NAS-OS当前 | 对标差距 |
|------|-------------------|------------|----------|
| 密钥创建 | ✅ Create操作 | ✅ CreateKey | 达标 |
| 密钥注册 | ✅ Register操作 | ✅ RegisterKey | 达标 |
| 密钥激活 | ✅ Activate操作 | ✅ ActivateKey | 达标 |
| 密钥撤销 | ✅ Revoke操作 | ✅ RevokeKey | 达标 |
| 密钥销毁 | ✅ Destroy操作 | ✅ DestroyKey | 达标 |
| 密钥定位 | ✅ Locate操作 | ❌ 未实现 | **缺失** |
| 密钥获取 | ✅ Get操作 | ❌ 未实现 | **缺失** |
| 密钥属性查询 | ✅ GetAttributes | ❌ 未实现 | **缺失** |
| 客户端认证 | ✅ TLS双向认证 | ⚠️ 仅单向 | **部分** |
| 多KMS支持 | ✅ 多配置 | ✅ KMSProvider字段 | 达标 |
| 审计日志 | ✅ 操作日志 | ✅ zap logger | 达标 |

### 2.2 差距分析

**核心差距**:
1. **Locate/Get操作**: TrueNAS支持按属性搜索和获取密钥材料
2. **双向TLS认证**: TrueNAS要求客户端证书
3. **密钥属性管理**: TrueNAS支持自定义属性

---

## 3. NAS-OS KMIP架构设计

### 3.1 栄件架构

```
┌──────────────────────────────────────────────────────────────┐
│                     NAS-OS KMIP Layer                        │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │  API Layer  │  │  Core Ops   │  │  Storage    │          │
│  │ handlers.go │  │  kmip.go    │  │  config.go  │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
│         │               │               │                   │
│         ▼               ▼               ▼                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              KMIPClient (Protocol Layer)             │   │
│  │  - Message Encoding/Decoding                         │   │
│  │  - TLS Connection Management                         │   │
│  │  - Request/Response Handling                         │   │
│  └─────────────────────────────────────────────────────┘   │
│                         │                                   │
│                         ▼                                   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │              External KMS Integration                │   │
│  │  - HashiCorp Vault                                   │   │
│  │  - AWS KMS                                           │   │
│  │  - Azure Key Vault                                   │   │
│  │  - Thales CipherTrust                                │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### 3.2 密钥生命周期

```
           Create/Register
                 │
                 ▼
         ┌───────────────┐
         │  Pre-Active   │ ───────────────────┐
         └───────────────┘                     │
                 │ Activate                    │ Destroy
                 ▼                             │
         ┌───────────────┐                     │
         │    Active     │◄───────────────────┤
         └───────────────┘                     │
                 │                             │
        ┌────────┴────────┐                    │
        │                 │                    │
        ▼                 ▼                    │
┌───────────────┐ ┌───────────────┐            │
│  Deactivated  │ │  Compromised  │────────────┤
└───────────────┘ └───────────────┘            │
        │                 │                    │
        │                 │ Destroy            │
        ▼                 ▼                    ▼
┌───────────────┐ ┌───────────────────────────────┐
│   Destroyed   │ │  Destroyed-Compromised        │
└───────────────┘ └───────────────────────────────┘
```

### 3.3 数据模型

```go
// KMIPKey - 密钥对象完整模型
type KMIPKey struct {
    // 标识
    ID              string         `json:"id"`              // 内部ID
    UniqueIdentifier string        `json:"unique_identifier"` // KMIP标准ID
    
    // 基本信息
    Name            string         `json:"name"`
    ObjectType      KMIPObjectType `json:"object_type"`     // symmetric_key, public_key, private_key, certificate
    KeyState        KMIPKeyState   `json:"key_state"`
    
    // 密钥规格
    KeyAlgorithm    string         `json:"key_algorithm"`   // AES, RSA, ECDSA, Ed25519
    KeyLength       int            `json:"key_length"`      // Bits
    KeyUsage        []string       `json:"key_usage"`       // encrypt, decrypt, sign, verify, wrap, unwrap
    
    // 时间管理
    CreatedAt       time.Time      `json:"created_at"`
    ActivatedAt     time.Time      `json:"activated_at,omitempty"`
    DeactivatedAt   time.Time      `json:"deactivated_at,omitempty"`
    ExpiresAt       time.Time      `json:"expires_at,omitempty"`
    
    // 来源与归属
    CreatedBy       string         `json:"created_by"`      // User or service
    KMSProvider     string         `json:"kms_provider"`    // External KMS name
    
    // 扩展属性
    Tags            []string       `json:"tags"`
    Attributes      map[string]string `json:"attributes"`   // 自定义属性
    CryptographicUsageMask uint16  `json:"cryptographic_usage_mask"`
    
    // 安全标记
    CompromiseDate  *time.Time     `json:"compromise_date,omitempty"`
    RevocationReason string        `json:"revocation_reason,omitempty"`
}
```

---

## 4. 协议实现细节

### 4.1 KMIP消息格式

```go
// KMIPMessage - 协议消息结构
type KMIPMessage struct {
    Header  KMIPHeader
    Items   []KMIPItem
}

// KMIPHeader - 消息头
type KMIPHeader struct {
    ProtocolVersion  ProtocolVersion
    MaxBufferSize    int32
    MaximumResponseSize int32
    TimeStamp        time.Time
    BatchCount       int32
}

// KMIPItem - TTLV编码项 (Tag-Type-Length-Value)
type KMIPItem struct {
    Tag        KMIPTag
    Type       KMIPType    // Integer, LongInteger, ByteString, Structure
    Length     int32
    Value      interface{}
}
```

### 4.2 核心操作实现

#### Create操作

```go
// CreateRequest - KMIP Create请求
type CreateRequest struct {
    ObjectType          KMIPObjectType
    TemplateAttribute   TemplateAttribute
    KeyFormatType       KeyFormatType
    KeyWrapType         KeyWrapType
}

// CreateResponse - KMIP Create响应
type CreateResponse struct {
    UniqueIdentifier    string
    KeyBlock            KeyBlock
    TemplateAttribute   TemplateAttribute
}

// CreateKey实现（增强版）
func (m *KMIPManager) CreateKeyEnhanced(ctx context.Context, req CreateRequest) (*KMIPKey, error) {
    // 1. 构建KMIP请求消息
    msg := buildCreateMessage(req)
    
    // 2. 发送到外部KMS（如配置）
    if m.clientConfig.ServerAddress != "localhost" {
        resp, err := m.sendKMIPRequest(msg)
        if err != nil {
            return nil, err
        }
        // 解析响应，提取UniqueIdentifier和KeyBlock
        return m.parseCreateResponse(resp)
    }
    
    // 3. 本地密钥生成（无外部KMS时）
    keyMaterial, err := generateKeyMaterial(req.ObjectType, req.KeyFormatType)
    if err != nil {
        return nil, err
    }
    
    // 4. 存储密钥元数据（不存储密钥材料）
    key := &KMIPKey{
        UniqueIdentifier: generateKMIPUID(),
        ObjectType:       req.ObjectType,
        KeyState:         KMIPStatePreActive,
        ...
    }
    
    m.keys[key.ID] = key
    return key, m.saveConfig()
}
```

#### Locate操作（新增）

```go
// LocateRequest - KMIP Locate请求
type LocateRequest struct {
    MaximumItems        int32
    OffsetItems         int32
    StorageUniqueIDs    []string
    AttributeFilters    []AttributeFilter
}

// LocateKeys - 按属性搜索密钥
func (m *KMIPManager) LocateKeys(ctx context.Context, filters []AttributeFilter) ([]*KMIPKey, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    results := []*KMIPKey{}
    
    for _, key := range m.keys {
        if matchFilters(key, filters) {
            results = append(results, key)
        }
    }
    
    return results, nil
}

// AttributeFilter - 属性过滤条件
type AttributeFilter struct {
    AttributeName       string
    AttributeValue      interface{}
    MatchOperator       MatchOperator // Equals, Contains, GreaterThan, LessThan
}
```

#### Get操作（新增）

```go
// GetRequest - KMIP Get请求
type GetRequest struct {
    UniqueIdentifier    string
    KeyFormatType       KeyFormatType
    KeyWrapType         KeyWrapType
    KeyCompressionType  KeyCompressionType
}

// GetKeyMaterial - 获取密钥材料（仅限Active密钥）
func (m *KMIPManager) GetKeyMaterial(ctx context.Context, uniqueID string) ([]byte, error) {
    // 安全检查：仅Active状态密钥可获取
    key, err := m.GetKey(ctx, uniqueID)
    if err != nil {
        return nil, err
    }
    
    if key.KeyState != KMIPStateActive {
        return nil, errors.New("key not in active state")
    }
    
    // 向外部KMS请求密钥材料
    if m.clientConfig.ServerAddress != "localhost" {
        msg := buildGetMessage(uniqueID)
        resp, err := m.sendKMIPRequest(msg)
        if err != nil {
            return nil, err
        }
        return extractKeyMaterial(resp)
    }
    
    // 本地模式：从安全存储获取
    return m.secureStorage.GetKeyMaterial(uniqueID)
}
```

---

## 5. 安全增强设计

### 5.1 双向TLS认证

```go
// KMIPClientConfig - 增强TLS配置
type KMIPClientConfig struct {
    ServerAddress        string
    ServerPort           int
    UseTLS               bool
    
    // TLS证书配置
    ClientCertificatePath string   `json:"client_certificate_path"`  // 新增
    ClientKeyPath         string   `json:"client_key_path"`          // 新增
    CACertificatePath     string   `json:"ca_certificate_path"`      // 新增
    
    // 安全策略
    MinTLSVersion         uint16   `json:"min_tls_version"`          // TLS 1.2
    VerifyServerCert      bool     `json:"verify_server_cert"`       // 强制true
    
    // 连接管理
    TimeoutSeconds       int
    ConnectionPoolSize   int
    MaxRetries           int      `json:"max_retries"`              // 新增
    RetryInterval        int      `json:"retry_interval"`           // 新增
}

// buildTLSConfig构建安全的TLS配置
func (c *KMIPClientConfig) buildTLSConfig() (*tls.Config, error) {
    config := &tls.Config{
        MinVersion: tls.VersionTLS12,
        InsecureSkipVerify: false,
    }
    
    // 加载CA证书
    if c.CACertificatePath != "" {
        caCert, err := os.ReadFile(c.CACertificatePath)
        if err != nil {
            return nil, err
        }
        caPool := x509.NewCertPool()
        caPool.AppendCertsFromPEM(caCert)
        config.RootCAs = caPool
    }
    
    // 加载客户端证书（双向认证）
    if c.ClientCertificatePath != "" && c.ClientKeyPath != "" {
        cert, err := tls.LoadX509KeyPair(c.ClientCertificatePath, c.ClientKeyPath)
        if err != nil {
            return nil, err
        }
        config.Certificates = []tls.Certificate{cert}
    }
    
    return config, nil
}
```

### 5.2 密钥材料安全存储

```go
// SecureKeyStorage - 密钥材料安全存储接口
type SecureKeyStorage interface {
    StoreKeyMaterial(uniqueID string, material []byte) error
    GetKeyMaterial(uniqueID string) ([]byte, error)
    DeleteKeyMaterial(uniqueID string) error
}

// EncryptedFileStorage - 加密文件存储实现
type EncryptedFileStorage struct {
    storagePath    string
    masterKey      []byte         // 从KMS或TPM获取
    encryptionKey  []byte         // 派生密钥
}

func (s *EncryptedFileStorage) StoreKeyMaterial(uniqueID string, material []byte) error {
    // 使用AES-256-GCM加密
    encrypted, err := s.encrypt(material)
    if err != nil {
        return err
    }
    
    // 写入安全存储文件
    path := filepath.Join(s.storagePath, uniqueID+".key")
    return os.WriteFile(path, encrypted, 0600)
}
```

### 5.3 操作审计增强

```go
// KMIPAuditLog - 操作审计日志
type KMIPAuditLog struct {
    OperationID    string
    OperationType  string       // Create, Get, Locate, Destroy, etc.
    ObjectType     KMIPObjectType
    UniqueIdentifier string
    UserID         string
    ClientIP       string
    Timestamp      time.Time
    Result         string       // success, failure
    ErrorMessage   string
    Duration       time.Duration
}

// LogOperation记录操作审计
func (m *KMIPManager) LogOperation(op KMIPAuditLog) {
    m.logger.Info("KMIP Operation",
        zap.String("op_id", op.OperationID),
        zap.String("op_type", op.OperationType),
        zap.String("uid", op.UniqueIdentifier),
        zap.String("user", op.UserID),
        zap.String("result", op.Result),
        zap.Duration("duration", op.Duration),
    )
    
    // 写入审计文件
    m.appendAuditFile(op)
}
```

---

## 6. 与NAS-OS功能集成

### 6.1 ZFS加密集成

```go
// ZFSEncryptionManager使用KMIP密钥
type ZFSEncryptionManager struct {
    kmipManager    *KMIPManager
}

// CreateEncryptedPool创建加密池
func (m *ZFSEncryptionManager) CreateEncryptedPool(poolName string, keyName string) error {
    // 1. 从KMIP获取密钥
    key, err := m.kmipManager.GetKeyByName(ctx, keyName)
    if err != nil {
        return err
    }
    
    // 2. 验证密钥状态
    if key.KeyState != KMIPStateActive {
        return errors.New("key not active")
    }
    
    // 3. 获取密钥材料（通过安全通道）
    keyMaterial, err := m.kmipManager.GetKeyMaterial(ctx, key.UniqueIdentifier)
    if err != nil {
        return err
    }
    
    // 4. 创建加密ZFS池
    return zfsCreateEncrypted(poolName, keyMaterial)
}
```

### 6.2 WriteOnce签名集成

```go
// WriteOnceManager使用KMIP签名密钥
type WriteOnceManager struct {
    kmipManager    *KMIPManager
    signer         *Signer
}

// InitializeSigner初始化签名器
func (m *WriteOnceManager) InitializeSigner(ctx context.Context) error {
    // 1. Locate签名密钥
    keys, err := m.kmipManager.LocateKeys(ctx, []AttributeFilter{
        {AttributeName: "KeyUsage", AttributeValue: "sign", MatchOperator: Contains},
    })
    if err != nil || len(keys) == 0 {
        // 创建新的签名密钥
        key, err := m.kmipManager.CreateKey(ctx, "writeonce-sign", 
            KMIPTypePrivateKey, "Ed25519", 256, []string{"sign"}, "system")
        if err != nil {
            return err
        }
        keys = []*KMIPKey{key}
    }
    
    // 2. 激活密钥
    if keys[0].KeyState == KMIPStatePreActive {
        err := m.kmipManager.ActivateKey(ctx, keys[0].ID)
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

### 6.3 SFTP/LDAP证书集成

```go
// CertificateManager管理TLS证书
type CertificateManager struct {
    kmipManager    *KMIPManager
}

// GetServerCertificate获取服务器证书
func (m *CertificateManager) GetServerCertificate(ctx context.Context, certName string) (*tls.Certificate, error) {
    // Locate证书
    certs, err := m.kmipManager.LocateKeys(ctx, []AttributeFilter{
        {AttributeName: "ObjectType", AttributeValue: KMIPTypeCertificate},
        {AttributeName: "Name", AttributeValue: certName},
    })
    if err != nil {
        return nil, err
    }
    
    // 获取证书和私钥
    certMaterial, err := m.kmipManager.GetKeyMaterial(ctx, certs[0].UniqueIdentifier)
    if err != nil {
        return nil, err
    }
    
    // 解析为tls.Certificate
    return parseCertificate(certMaterial)
}
```

---

## 7. 实现路线图

### Phase 1: 基础增强 (本周)

| 任务 | 工作量 | 优先级 |
|------|--------|--------|
| Locate/Get操作实现 | 4h | P1 |
| 双向TLS认证 | 3h | P1 |
| 操作审计增强 | 2h | P2 |

### Phase 2: 安全存储 (下周)

| 任务 | 工作量 | 优先级 |
|------|--------|--------|
| EncryptedFileStorage实现 | 4h | P1 |
| 密钥轮换策略 | 3h | P2 |
| 配置加密 | 2h | P1 |

### Phase 3: 功能集成 (本月)

| 任务 | 工作量 | 优先级 |
|------|--------|--------|
| ZFS加密集成 | 4h | P1 |
| WriteOnce签名集成 | 3h | P2 |
| LDAP证书集成 | 2h | P2 |

---

## 8. 测试策略

### 8.1 单元测试

```go
// KMIP核心操作测试
func TestKMIPLifecycle(t *testing.T) {
    mgr, _ := NewKMIPManager("/tmp/kmip-test", zap.NewNop())
    
    // Create
    key, err := mgr.CreateKey(ctx, "test-key", KMIPTypeSymmetricKey, "AES", 256, []string{"encrypt"}, "test")
    assert.NoError(t, err)
    assert.Equal(t, KMIPStatePreActive, key.KeyState)
    
    // Activate
    err = mgr.ActivateKey(ctx, key.ID)
    assert.NoError(t, err)
    assert.Equal(t, KMIPStateActive, key.KeyState)
    
    // Revoke
    err = mgr.RevokeKey(ctx, key.ID)
    assert.NoError(t, err)
    assert.Equal(t, KMIPStateCompromised, key.KeyState)
    
    // Destroy
    err = mgr.DestroyKey(ctx, key.ID)
    assert.NoError(t, err)
    assert.Equal(t, KMIPStateDestroyed, key.KeyState)
}
```

### 8.2 集成测试

- HashiCorp Vault KMIP插件集成测试
- PyKMIP模拟服务器测试
- TLS双向认证测试

---

## 9. 总结

### 9.1 设计要点

1. **密钥生命周期完整**: PreActive→Active→Deactivated/Compromised→Destroyed
2. **安全传输**: TLS 1.2+双向认证
3. **安全存储**: 加密文件存储，不暴露密钥材料
4. **操作审计**: 完整的操作日志记录
5. **多KMS支持**: HashiCorp Vault, AWS KMS, Azure Key Vault

### 9.2 对标TrueNAS差距

- ✅ 核心生命周期操作达标
- ⚠️ Locate/Get操作需实现
- ⚠️ 双向TLS需增强
- ✅ 多KMS支持达标
- ✅ 审计日志达标

### 9.3 下一步

1. 立即实现Locate/Get操作
2. 增强双向TLS认证
3. 实现密钥材料安全存储
4. 与ZFS加密/WriteOnce集成

---

**[刑部] KMIP预研完成**