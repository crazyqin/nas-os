# 隐私相册功能设计文档

## 1. 功能概述

隐私相册为NAS用户提供安全、私密的图片存储空间，参考群晖DSM的隐私保护功能，实现独立密码保护、加密存储、隐蔽入口等核心特性。

## 2. 竞品分析

### 2.1 群晖DSM 私密空间
- **独立密码保护**: 与系统账户分离的独立访问密码
- **隐蔽入口**: 需要通过特定路径或手势才能访问
- **加密存储**: AES-256加密保护文件内容
- **无痕模式**: 不在最近访问记录中显示
- **自动锁定**: 一段时间不活动后自动锁定

### 2.2 飞牛NAS 用户需求
根据飞牛论坛调研，隐私相册是高频需求之一，用户期望：
- 独立于主相册系统的私密空间
- 不被家庭成员浏览到私密内容
- 简单易用但足够安全

## 3. 系统架构

### 3.1 整体架构

```
┌─────────────────────────────────────────────────────────────┐
│                     隐私相册系统架构                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐     │
│  │   入口层    │───▶│  认证层     │───▶│  加密层     │     │
│  │ (隐蔽入口)  │    │ (密码验证)  │    │ (文件加密)  │     │
│  └─────────────┘    └─────────────┘    └─────────────┘     │
│         │                  │                  │            │
│         ▼                  ▼                  ▼            │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                   存储层 (加密文件系统)              │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 模块划分

```
internal/photos/private/
├── manager.go          # 隐私相册管理器
├── crypto.go           # 加密模块
├── vault.go            # 保险箱实现
├── access.go           # 访问控制
├── types.go            # 数据结构定义
└── config.go           # 配置管理
```

## 4. 核心模块设计

### 4.1 数据结构定义 (types.go)

```go
// PrivateAlbum 隐私相册
type PrivateAlbum struct {
    ID              string          `json:"id"`
    Name            string          `json:"name"`            // 显示名称（加密存储）
    UserID          string          `json:"userId"`          // 所属用户
    PhotoCount      int             `json:"photoCount"`      // 照片数量
    StorageSize     int64           `json:"storageSize"`     // 存储大小
    CreatedAt       time.Time       `json:"createdAt"`       // 创建时间
    UpdatedAt       time.Time       `json:"updatedAt"`       // 更新时间
    LastAccessed    time.Time       `json:"lastAccessed"`    // 最后访问时间
    AutoLockMinutes int             `json:"autoLockMinutes"` // 自动锁定时间
    IsLocked        bool            `json:"isLocked"`        // 当前锁定状态
}

// PrivateVault 保险箱（顶层容器）
type PrivateVault struct {
    ID              string    `json:"id"`
    UserID          string    `json:"userId"`
    PasswordHash    string    `json:"-"`              // 不序列化到JSON
    Salt            string    `json:"-"`
    Hint            string    `json:"hint,omitempty"` // 密码提示
    CreatedAt       time.Time `json:"createdAt"`
    Albums          []string  `json:"albums"`         // 相册ID列表
    IsSetup         bool      `json:"isSetup"`        // 是否已初始化
}

// PrivatePhoto 隐私照片
type PrivatePhoto struct {
    ID            string    `json:"id"`
    Filename      string    `json:"filename"`        // 原始文件名（加密存储）
    EncryptedPath string    `json:"-"`               // 加密文件路径（不暴露）
    AlbumID       string    `json:"albumId"`
    Size          int64     `json:"size"`            // 原始大小
    EncryptedSize int64     `json:"encryptedSize"`   // 加密后大小
    MimeType      string    `json:"mimeType"`
    Width         int       `json:"width"`           // 解密后获取
    Height        int       `json:"height"`          // 解密后获取
    TakenAt       time.Time `json:"takenAt"`
    UploadedAt    time.Time `json:"uploadedAt"`
}

// AccessSession 访问会话
type AccessSession struct {
    SessionID   string    `json:"sessionId"`
    VaultID     string    `json:"vaultId"`
    UserID      string    `json:"userId"`
    UnlockedAt  time.Time `json:"unlockedAt"`
    ExpiresAt   time.Time `json:"expiresAt"`
    LastActive  time.Time `json:"lastActive"`
    ClientIP    string    `json:"clientIp,omitempty"`
}
```

### 4.2 加密模块 (crypto.go)

```go
// CryptoService 加密服务接口
type CryptoService interface {
    // Encrypt 加密数据
    Encrypt(plaintext []byte, key []byte) ([]byte, error)
    
    // Decrypt 解密数据
    Decrypt(ciphertext []byte, key []byte) ([]byte, error)
    
    // EncryptFile 加密文件
    EncryptFile(srcPath, dstPath string, key []byte) error
    
    // DecryptFile 解密文件
    DecryptFile(srcPath, dstPath string, key []byte) error
    
    // DeriveKey 从密码派生密钥
    DeriveKey(password string, salt []byte) ([]byte, error)
    
    // GenerateSalt 生成随机盐值
    GenerateSalt() ([]byte, error)
    
    // HashPassword 密码哈希（用于验证）
    HashPassword(password string, salt []byte) (string, error)
    
    // VerifyPassword 验证密码
    VerifyPassword(password, hash string, salt []byte) bool
}

// AES256Crypto AES-256加密实现
type AES256Crypto struct {
    // 使用AES-256-GCM模式
    // 提供认证加密，防止篡改
}

// 加密流程：
// 1. 使用Argon2id从用户密码派生加密密钥
// 2. 生成随机nonce
// 3. 使用AES-256-GCM加密文件内容
// 4. 将nonce和加密数据合并存储
```

**加密参数**：
- 算法: AES-256-GCM
- 密钥派生: Argon2id
  - 时间成本: 3
  - 内存: 64MB
  - 并行度: 4
- 盐值长度: 32字节
- Nonce长度: 12字节

### 4.3 保险箱管理 (vault.go)

```go
// VaultManager 保险箱管理器
type VaultManager struct {
    dataDir     string
    vaults      map[string]*PrivateVault
    sessions    map[string]*AccessSession
    crypto      CryptoService
    mu          sync.RWMutex
    config      VaultConfig
}

// 主要功能：
// - SetupVault()          初始化保险箱（设置密码）
// - Unlock()              解锁保险箱
// - Lock()                锁定保险箱
// - IsUnlocked()          检查锁定状态
// - ChangePassword()      修改密码
// - SetHint()             设置密码提示
// - ValidatePassword()    验证密码强度
```

### 4.4 隐蔽入口设计

**入口方式**：

1. **手势入口** (推荐)
   - 在相册页面执行特定手势（如：连续点击3次右上角）
   - 不显示任何入口按钮，防止他人发现

2. **路径入口**
   - 通过URL直接访问: `/photos/private/vault`
   - 可自定义路径，增加隐蔽性

3. **设置入口**
   - 在设置中开启隐私相册入口开关
   - 入口显示为普通功能按钮，降低敏感度

4. **伪装修饰**
   - 入口伪装成"回收站"或"系统设置"等普通功能
   - 点击后要求输入密码

**实现代码示例**：

```go
// EntryPoint 入口配置
type EntryPoint struct {
    Type           EntryPointType `json:"type"`
    GesturePattern []int          `json:"gesturePattern,omitempty"` // 点击坐标序列
    CustomPath     string         `json:"customPath,omitempty"`     // 自定义URL路径
    DisguiseAs     string         `json:"disguiseAs,omitempty"`     // 伪装成什么功能
    Enabled        bool           `json:"enabled"`
}

type EntryPointType string

const (
    EntryPointGesture EntryPointType = "gesture"  // 手势入口
    EntryPointPath    EntryPointType = "path"     // 路径入口
    EntryPointDisguise EntryPointType = "disguise" // 伪装入口
)
```

### 4.5 访问控制 (access.go)

```go
// AccessController 访问控制器
type AccessController struct {
    sessions    map[string]*AccessSession
    maxSessions int
    timeout     time.Duration
}

// 功能：
// - CreateSession()      创建访问会话
// - ValidateSession()    验证会话有效性
// - RefreshSession()     刷新会话（延长有效期）
// - RevokeSession()      撤销会话
// - AutoLock()           自动锁定检查
// - AuditAccess()        访问审计日志

// 安全策略：
// - 会话超时: 默认5分钟无操作自动锁定
// - 最大会话数: 每用户1个活跃会话
// - 失败锁定: 连续5次密码错误，锁定15分钟
// - 审计日志: 记录所有访问和操作
```

### 4.6 相册管理器 (manager.go)

```go
// PrivateManager 隐私相册管理器
type PrivateManager struct {
    vault    *VaultManager
    access   *AccessController
    crypto   CryptoService
    dataDir  string
    mu       sync.RWMutex
}

// 主要功能：
// - CreateAlbum()        创建隐私相册
// - DeleteAlbum()        删除隐私相册
// - ListAlbums()         列出相册（需先解锁）
// - UploadPhoto()        上传照片（加密存储）
// - DownloadPhoto()      下载照片（解密）
// - DeletePhoto()        删除照片
// - MoveToPrivate()      从普通相册移动到隐私相册
// - MoveToPublic()       从隐私相册移出到普通相册
```

## 5. 存储设计

### 5.1 目录结构

```
data/
└── private/
    └── {userId}/
        ├── vault.json          # 保险箱元数据
        ├── albums/
        │   └── {albumId}/
        │       ├── meta.enc    # 加密的相册元数据
        │       └── photos/
        │           └── {photoId}.enc  # 加密的照片文件
        ├── cache/
        │   └── {sessionId}/    # 会话临时缓存
        └── audit.log.enc       # 加密的审计日志
```

### 5.2 文件命名

- 照片文件使用UUID命名，不暴露原始文件名
- 元数据文件独立加密存储
- 文件名与内容分离，增加安全性

### 5.3 元数据加密

```go
// 加密的元数据结构
type EncryptedMetadata struct {
    Version     int       `json:"version"`      // 加密版本
    Nonce       []byte    `json:"nonce"`        // 加密nonce
    Ciphertext  []byte    `json:"ciphertext"`   // 加密数据
    Tag         []byte    `json:"tag"`          // 认证标签
}
```

## 6. API设计

### 6.1 REST API

```
POST   /api/v1/private/setup         # 初始化保险箱
POST   /api/v1/private/unlock        # 解锁保险箱
POST   /api/v1/private/lock          # 锁定保险箱
GET    /api/v1/private/status        # 获取状态

POST   /api/v1/private/albums        # 创建相册
GET    /api/v1/private/albums        # 列出相册
DELETE /api/v1/private/albums/{id}   # 删除相册

POST   /api/v1/private/photos/upload # 上传照片
GET    /api/v1/private/photos/{id}   # 获取照片
DELETE /api/v1/private/photos/{id}   # 删除照片

POST   /api/v1/private/move-in       # 从公开相册移入
POST   /api/v1/private/move-out      # 移出到公开相册

PUT    /api/v1/private/password      # 修改密码
PUT    /api/v1/private/hint          # 设置密码提示
```

### 6.2 WebSocket事件

```javascript
// 会话心跳，保持解锁状态
{ "type": "heartbeat" }

// 自动锁定警告
{ "type": "lock_warning", "secondsRemaining": 60 }

// 会话锁定通知
{ "type": "session_locked" }
```

## 7. 安全考虑

### 7.1 威胁模型

| 威胁 | 防护措施 |
|------|----------|
| 文件系统直接访问 | 文件内容加密，无密钥无法解密 |
| 内存转储攻击 | 密钥使用后立即清零，尽量减少内存驻留时间 |
| 暴力破解 | Argon2id慢哈希 + 失败锁定 |
| 中间人攻击 | 强制HTTPS |
| 会话劫持 | 短期会话 + 自动锁定 |
| 社会工程 | 隐蔽入口 + 伪装功能 |

### 7.2 密钥管理

```
用户密码
    │
    ▼ (Argon2id派生)
主密钥 (Master Key)
    │
    ├──▶ 文件加密密钥 (File Encryption Key)
    │
    └──▶ 元数据加密密钥 (Metadata Key)
```

- 主密钥不持久化存储
- 会话期间保存在内存中
- 锁定后立即清零

### 7.3 安全检查清单

- [ ] 所有敏感数据使用AES-256-GCM加密
- [ ] 密码使用Argon2id派生密钥
- [ ] 每个加密操作使用唯一nonce
- [ ] 实现完整性验证（GCM认证标签）
- [ ] 内存中密钥使用后清零
- [ ] 实现安全的密码强度检查
- [ ] 记录详细的审计日志
- [ ] 定期进行安全审计

## 8. 性能优化

### 8.1 缓存策略

- **缩略图缓存**: 解密后缓存缩略图，有效期与会话同步
- **元数据缓存**: 会话期间缓存解密的元数据
- **预解密**: 滚动浏览时预解密下一张照片

### 8.2 加密性能

| 操作 | 小图(< 1MB) | 大图(10MB) | 超大图(50MB) |
|------|-------------|------------|--------------|
| 加密 | ~50ms | ~500ms | ~2.5s |
| 解密 | ~50ms | ~500ms | ~2.5s |
| 缩略图 | ~20ms | ~100ms | ~300ms |

*测试环境: ARM Cortex-A76 @ 2.4GHz*

### 8.3 优化建议

1. **流式加密**: 大文件分块加密，减少内存占用
2. **并行处理**: 多核并行加密多个文件
3. **硬件加速**: 利用ARM NEON指令集加速AES

## 9. 用户体验

### 9.1 密码设置流程

```
1. 首次进入隐私相册
   └─▶ 设置密码页面
       ├─ 输入密码（显示强度指示）
       ├─ 确认密码
       ├─ 设置密码提示（可选）
       └─ 完成

2. 密码强度要求
   ├─ 最少8个字符
   ├─ 包含大小写字母
   ├─ 包含数字
   └─ 包含特殊字符（推荐）
```

### 9.2 日常使用流程

```
1. 隐蔽入口触发
   └─▶ 密码输入
       └─▶ 隐私相册主页
           ├─ 相册列表
           ├─ 照片网格
           ├─ 上传/删除
           └─ 设置（修改密码、入口配置）

2. 自动锁定
   └─▶ 离开页面5分钟
       └─▶ 自动锁定（可配置）

3. 切换到其他应用
   └─▶ 返回时要求重新输入密码
```

### 9.3 忘记密码处理

**重要**: 密码无法找回，这是安全设计的一部分。

- 提供"密码提示"功能
- 管理员可删除整个保险箱（数据丢失）
- 不提供密码重置功能（防止社会工程攻击）

## 10. 实现计划

### Phase 1: 基础框架 (Week 1-2)
- [ ] 数据结构定义
- [ ] 加密模块实现
- [ ] 保险箱管理器
- [ ] 访问控制器

### Phase 2: 核心功能 (Week 3-4)
- [ ] 相册管理
- [ ] 照片上传/下载
- [ ] 缩略图生成
- [ ] 文件操作

### Phase 3: 用户界面 (Week 5-6)
- [ ] 隐蔽入口实现
- [ ] Web UI页面
- [ ] 密码设置流程
- [ ] 自动锁定机制

### Phase 4: 测试与优化 (Week 7-8)
- [ ] 安全测试
- [ ] 性能优化
- [ ] 用户测试
- [ ] 文档完善

## 11. 与现有系统集成

### 11.1 photos模块集成

```go
// manager.go 中添加隐私相册支持
type Manager struct {
    // 现有字段...
    private *private.PrivateManager  // 隐私相册管理器
}

// 初始化时创建隐私相册管理器
func NewManager(dataDir string) (*Manager, error) {
    // 现有初始化...
    
    privateMgr, err := private.NewPrivateManager(
        filepath.Join(dataDir, "private"),
    )
    if err != nil {
        return nil, err
    }
    m.private = privateMgr
    
    return m, nil
}
```

### 11.2 文件格式支持

隐私相册的加密文件使用 `.enc` 扩展名，在扫描时跳过这些文件。

### 11.3 备份集成

- 隐私相册可选择是否包含在系统备份中
- 备份文件保持加密状态
- 恢复时需要原密码才能访问

## 12. 附录

### A. 加密算法选择理由

| 算法 | 选择理由 |
|------|----------|
| AES-256-GCM | 业界标准，提供认证加密，防止篡改 |
| Argon2id | 抗GPU/ASIC攻击的密码哈希，2015年密码哈希竞赛冠军 |
| XChaCha20-Poly1305 (备选) | 软件实现更快，适合无AES硬件加速的平台 |

### B. 参考资料

- [NIST SP 800-38D: GCM规范](https://csrc.nist.gov/publications/detail/sp/800-38d/final)
- [Argon2 RFC 9106](https://www.rfc-editor.org/rfc/rfc9106)
- [群晖DSM 私密空间技术白皮书](https://www.synology.com/)
- [OWASP 密码存储指南](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html)