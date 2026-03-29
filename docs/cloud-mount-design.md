# 网盘原生挂载设计文档

> 编制: 工部 | 日期: 2026-03-29
> 对标: 飞牛fnOS网盘挂载功能

---

## 🎯 功能目标

实现主流网盘的原生挂载，用户可直接在NAS-OS中访问网盘文件，无需同步下载。

对标飞牛fnOS支持的网盘：
- 阿里云盘
- 百度网盘
- 115网盘
- 夸克网盘
- 123网盘
- OneDrive

---

## 🏗️ 技术方案

### 方案一: rclone集成

**优势**:
- 成熟稳定，支持40+云存储
- 已有丰富的网盘驱动
- 开源维护活跃

**劣势**:
- 阿里云盘/百度网盘需要第三方驱动
- 性能可能不如原生API

**实现路径**:
```
NAS-OS → rclone mount → VFS挂载 → 用户访问
```

### 方案二: 原生API开发

**优势**:
- 性能最优
- 深度集成NAS功能
- 可定制性强

**劣势**:
- 开发周期长
- API维护成本高

**实现路径**:
```
NAS-OS → 云盘API SDK → FUSE → 用户访问
```

---

## 📋 推荐方案: rclone + 原生增强

采用混合方案:
1. **rclone作为基础驱动**: 快速支持主流网盘
2. **原生SDK增强**: 优化阿里云盘/百度网盘体验

---

## 🛠️ 模块设计

### 1. 云盘管理模块

```go
// internal/cloud/mount/
type CloudMountService struct {
    Drivers    map[string]CloudDriver
    Mounts     map[string]*MountPoint
    MountRoot  string  // 挂载根目录
}

type MountPoint struct {
    ID         string
    Provider   string  // aliyun/baidu/115/quark
    MountPath  string
    Config     MountConfig
    Status     MountStatus
}
```

### 2. 云盘驱动接口

```go
type CloudDriver interface {
    Name() string
    Authenticate(ctx context.Context, config AuthConfig) error
    List(ctx context.Context, path string) ([]CloudFile, error)
    Read(ctx context.Context, path string) (io.Reader, error)
    Write(ctx context.Context, path string, data io.Reader) error
    Delete(ctx context.Context, path string) error
    Mount(ctx context.Context, target string) error
    Unmount(ctx context.Context) error
}
```

### 3. 阿里云盘驱动

```go
// internal/cloud/drivers/aliyun/
type AliyunDriver struct {
    client     *AliyunClient
    refreshToken string
}

// 阿里云盘API:
// - 开放平台API (https://open.aliyundrive.com)
// - 文件列表: /adrive/v1.0/openFile/list
// - 文件下载: /adrive/v1.0/openFile/getDownloadUrl
```

### 4. 百度网盘驱动

```go
// internal/cloud/drivers/baidu/
type BaiduDriver struct {
    client     *BaiduClient
    accessToken string
}

// 百度网盘API:
// - PCS API (需要OAuth授权)
// - 文件列表: /rest/2.0/xpan/file
// - 文件下载: /rest/2.0/xpan/multimedia
```

---

## 🔐 认证流程

### OAuth2.0授权

```
1. 用户点击"添加网盘"
2. 弹出网盘授权页面
3. 用户登录网盘并授权
4. 获取refresh_token/access_token
5. 存储凭证(加密)
6. 完成挂载
```

### 凭证存储

```go
// internal/cloud/auth/
type CredentialStore interface {
    Save(ctx context.Context, provider, user string, cred Credential) error
    Get(ctx context.Context, provider, user string) (*Credential, error)
    Delete(ctx context.Context, provider, user string) error
}

// 加密存储凭证
type Credential struct {
    Provider     string
    AccessToken  string // AES加密
    RefreshToken string // AES加密
    ExpiresAt    time.Time
}
```

---

## 📁 挂载管理

### 挂载点结构

```
/mnt/cloud/
├── aliyun-{account1}/
│   ├── 我的文件/
│   └── 共享文件/
├── baidu-{account1}/
│   ├── 我的网盘/
│   └── 共享文件/
└── 115-{account1}/
    └── 我的文件/
```

### 挂载操作

```go
// API接口
POST /api/v1/cloud/mounts
{
    "provider": "aliyun",
    "account_id": "user123",
    "mount_name": "阿里云盘-主账号",
    "options": {
        "read_only": false,
        "cache_size": "100MB",
        "refresh_interval": "5min"
    }
}

// 响应
{
    "mount_id": "mount-001",
    "mount_path": "/mnt/cloud/aliyun-user123",
    "status": "mounted"
}
```

---

## ⚡ 性能优化

### 1. 缓存策略

```go
// internal/cloud/cache/
type CloudCache struct {
    FileCache   map[string]*CachedFile
    DirCache    map[string]*CachedDir
    MaxSize     int64
    TTL         time.Duration
}

// 缓存层级:
// - 目录列表缓存: 5分钟
// - 文件内容缓存: 按大小动态调整
// - 大文件流式读取: 不缓存
```

### 2. 并发控制

```go
// internal/cloud/concurrency/
type ConcurrencyManager struct {
    MaxConnections int
    RateLimit      RateLimitConfig
}

// 并发设置:
// - 最大并发下载: 5
// - 最大并发上传: 3
// - 限速配置: 可设置上传/下载限速
```

---

## 📊 状态监控

```go
// internal/cloud/monitor/
type MountMonitor struct {
    Mounts      map[string]*MountStats
}

type MountStats struct {
    MountID       string
    Status        string  // mounted/unmounted/error
    TotalSize     int64
    UsedSize      int64
    ReadOps       int64
    WriteOps      int64
    LastSync      time.Time
    ErrorCount    int
}
```

---

## 🎨 UI设计

### 挂载管理页面

```
[云盘挂载]
┌─────────────────────────────────────────────────┐
│ 已挂载网盘                                       │
├─────────────────────────────────────────────────┤
│ 🟢 阿里云盘-主账号  /mnt/cloud/aliyun-main      │
│    总容量: 2TB  已用: 1.2TB  状态: 正常         │
│    [卸载] [设置] [刷新]                          │
│                                                 │
│ 🟢 百度网盘-主账号  /mnt/cloud/baidu-main       │
│    总容量: 2TB  已用: 800GB  状态: 正常         │
│    [卸载] [设置] [刷新]                          │
│                                                 │
│ [添加网盘]  支持: 阿里云盘/百度/115/夸克/123    │
└─────────────────────────────────────────────────┘
```

### 添加网盘流程

1. 点击"添加网盘"
2. 选择网盘类型
3. 弹出OAuth授权页面
4. 完成授权后自动挂载
5. 显示在已挂载列表

---

## 🔧 配置文件

```yaml
# /etc/nas-os/cloud-mount.yaml
mount_root: /mnt/cloud
cache:
  enabled: true
  max_size: 500MB
  ttl: 5min
concurrency:
  max_downloads: 5
  max_uploads: 3
providers:
  aliyun:
    enabled: true
    api_url: https://open.aliyundrive.com
  baidu:
    enabled: true
    api_url: https://pan.baidu.com
  115:
    enabled: true
    api_url: https://115.com
```

---

## 📝 开发计划

### Phase 1: 基础框架 (v2.315.0)
- 云盘管理模块骨架
- 挂载点管理API
- UI页面框架

### Phase 2: rclone集成 (v2.316.0)
- rclone驱动集成
- 基础挂载功能
- 缓存机制

### Phase 3: 原生驱动 (v2.320.0)
- 阿里云盘原生驱动
- 百度网盘原生驱动
- 性能优化

### Phase 4: 高级功能 (v2.325.0)
- 多账号管理
- 文件双向同步
- 云盘间迁移

---

## ⚠️ 注意事项

1. **API合规**: 使用官方开放API，避免私用接口
2. **凭证安全**: 加密存储，定期刷新
3. **带宽控制**: 支持限速配置
4. **错误处理**: 网络异常重试机制
5. **日志审计**: 记录所有挂载操作

---

## 📚 参考资料

- rclone文档: https://rclone.org/docs/
- 阿里云盘开放API: https://open.aliyundrive.com
- 百度网盘PCS API: https://pan.baidu.com/union/doc/
- 飞牛fnOS网盘挂载功能文档

---

*工部技术组*