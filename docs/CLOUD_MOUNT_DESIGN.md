# 网盘挂载核心模块设计

## 1. 概述

本文档描述 NAS-OS 系统中网盘挂载核心模块的设计，支持 115网盘、夸克网盘、阿里云盘等主流中国网盘的 FUSE 挂载功能。

## 2. 系统架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        用户空间                                   │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │   Web UI    │  │   CLI/API   │  │  FileMgr    │              │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘              │
│         │                │                │                      │
│         └────────────────┼────────────────┘                      │
│                          ▼                                      │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                    CloudFuse Manager                       │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │  │
│  │  │ Mount Mgmt  │  │ Auth Mgmt   │  │ Cache Mgmt  │        │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘        │  │
│  └───────────────────────────────────────────────────────────┘  │
│                          │                                      │
│                          ▼                                      │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                      FUSE Layer                            │  │
│  │  ┌─────────────────────────────────────────────────────┐  │  │
│  │  │                    CloudFS                           │  │  │
│  │  │  ┌───────────┐  ┌───────────┐  ┌───────────┐        │  │  │
│  │  │  │ DirNode   │  │ FileNode  │  │ Handle    │        │  │  │
│  │  │  └───────────┘  └───────────┘  └───────────┘        │  │  │
│  │  └─────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────┘  │
│                          │                                      │
├──────────────────────────┼──────────────────────────────────────┤
│                     FUSE Kernel                               │
├──────────────────────────┼──────────────────────────────────────┤
│                          ▼                                      │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                   Provider Layer                           │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐      │  │
│  │  │  115    │  │  Quark  │  │ AliPan  │  │  WebDAV │      │  │
│  │  │ Provider│  │ Provider│  │ Provider│  │ Provider│      │  │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘      │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## 3. 支持的网盘类型

### 3.1 网盘能力对比

| 网盘 | 读取 | 写入 | 秒传 | 离线下载 | 分享 | 流媒体 |
|------|------|------|------|----------|------|--------|
| 115网盘 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 夸克网盘 | ✅ | ✅ | ❌ | ❌ | ✅ | ✅ |
| 阿里云盘 | ✅ | ✅ | ✅ | ❌ | ✅ | ✅ |
| WebDAV | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| S3 兼容 | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |

### 3.2 Provider 接口设计

```go
type Provider interface {
    // 基础操作
    Upload(ctx context.Context, localPath, remotePath string) error
    Download(ctx context.Context, remotePath, localPath string) error
    Delete(ctx context.Context, remotePath string) error
    List(ctx context.Context, prefix string, recursive bool) ([]FileInfo, error)
    Stat(ctx context.Context, remotePath string) (*FileInfo, error)

    // 目录操作
    CreateDir(ctx context.Context, remotePath string) error
    DeleteDir(ctx context.Context, remotePath string) error

    // 连接管理
    TestConnection(ctx context.Context) (*ConnectionTestResult, error)
    Close() error

    // 元信息
    GetType() ProviderType
    GetCapabilities() []string
}
```

## 4. 挂载点管理

### 4.1 挂载配置

```go
type MountConfig struct {
    ID          string    `json:"id"`
    Name        string    `json:"name"`
    Type        string    `json:"type"`        // 115, quark, aliyun_pan
    MountPoint  string    `json:"mountPoint"`  // 本地挂载路径
    RemotePath  string    `json:"remotePath"`  // 远程根路径
    Enabled     bool      `json:"enabled"`
    AutoMount   bool      `json:"autoMount"`
    ReadOnly    bool      `json:"readOnly"`
    AllowOther  bool      `json:"allowOther"`  // 允许其他用户访问

    // 缓存配置
    CacheEnabled bool   `json:"cacheEnabled"`
    CacheDir     string `json:"cacheDir"`
    CacheSize    int64  `json:"cacheSize"`    // MB

    // 认证信息
    AccessToken  string `json:"-"`
    RefreshToken string `json:"-"`
    UserID       string `json:"userId,omitempty"`
    DriveID      string `json:"driveId,omitempty"`

    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

### 4.2 挂载状态机

```
         ┌──────────┐
         │   Idle   │
         └────┬─────┘
              │ Mount()
              ▼
         ┌──────────┐
         │ Mounting │
         └────┬─────┘
              │
    ┌─────────┴─────────┐
    │ Success           │ Failure
    ▼                   ▼
┌───────────┐     ┌──────────┐
│  Mounted  │     │  Error   │
└─────┬─────┘     └────┬─────┘
      │                │ Retry/Unmount
      │ Unmount()      ▼
      ▼           ┌──────────┐
┌─────────────┐   │   Idle   │
│ Unmounting  │───┴──────────┘
└─────────────┘
```

### 4.3 挂载管理器 API

```go
type Manager interface {
    // 生命周期管理
    Initialize() error
    Close() error

    // 挂载操作
    Mount(cfg *MountConfig) (*MountInfo, error)
    Unmount(mountID string) error
    Remount(mountID string) error

    // 配置管理
    AddMountConfig(cfg *MountConfig) error
    UpdateMountConfig(mountID string, cfg *MountConfig) error
    RemoveMountConfig(mountID string) error

    // 查询
    GetMount(mountID string) (*MountInfo, error)
    ListMounts() []MountInfo
    GetStats(mountID string) (*MountStats, error)

    // 测试
    TestMountConfig(cfg *MountConfig) (*ConnectionTestResult, error)
}
```

## 5. 缓存策略

### 5.1 缓存架构

```
┌──────────────────────────────────────────────────────┐
│                   CacheManager                        │
│  ┌────────────────────────────────────────────────┐  │
│  │                 Memory Cache                    │  │
│  │   - 热点数据缓存                                │  │
│  │   - 元数据缓存                                  │  │
│  │   - 目录结构缓存                                │  │
│  └────────────────────────────────────────────────┘  │
│  ┌────────────────────────────────────────────────┐  │
│  │                 Disk Cache                      │  │
│  │   - 文件内容缓存                                │  │
│  │   - LRU 淘汰策略                                │  │
│  │   - 持久化元数据                                │  │
│  └────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────┘
```

### 5.2 缓存策略配置

```go
type CacheConfig struct {
    Enabled      bool   `json:"enabled"`
    CacheDir     string `json:"cacheDir"`     // 缓存目录
    MaxSize      int64  `json:"maxSize"`      // 最大缓存大小 (MB)
    MaxFileSize  int64  `json:"maxFileSize"`  // 单文件最大缓存 (MB)
    TTL          int    `json:"ttl"`          // 缓存有效期 (秒)
    Policy       string `json:"policy"`       // lru, lfu, fifo
    WriteBack    bool   `json:"writeBack"`    // 写回策略
    Prefetch     bool   `json:"prefetch"`     // 预读开关
    PrefetchSize int64  `json:"prefetchSize"` // 预读大小 (MB)
}
```

### 5.3 缓存淘汰策略

1. **LRU (Least Recently Used)**
   - 默认策略
   - 淘汰最长时间未被访问的数据
   - 适合大多数场景

2. **LFU (Least Frequently Used)**
   - 淘汰访问频率最低的数据
   - 适合热点数据明显的场景

3. **FIFO (First In First Out)**
   - 先进先出
   - 实现简单，适合顺序访问

### 5.4 缓存一致性

```
┌─────────────────────────────────────────────────────────┐
│                    Cache Invalidation                    │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐│
│  │  写操作流程                                          ││
│  │  1. 上传文件到云端                                   ││
│  │  2. 更新本地缓存（如果启用写回）                      ││
│  │  3. 标记缓存有效                                     ││
│  └─────────────────────────────────────────────────────┘│
│                                                          │
│  ┌─────────────────────────────────────────────────────┐│
│  │  读操作流程                                          ││
│  │  1. 检查缓存是否存在                                 ││
│  │  2. 检查缓存是否过期（TTL）                          ││
│  │  3. 命中：返回缓存数据                               ││
│  │  4. 未命中：从云端下载并缓存                         ││
│  └─────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────┘
```

## 6. 认证流程

### 6.1 115网盘认证

```go
type Provider115Auth struct {
    // 步骤1: 获取登录二维码
    GetQRCode() (*QRCodeInfo, error)

    // 步骤2: 轮询扫码状态
    CheckScanStatus(qrCodeKey string) (*ScanStatus, error)

    // 步骤3: 获取 Token
    GetToken(scanResult *ScanResult) (*TokenInfo, error)

    // 步骤4: 刷新 Token
    RefreshToken(refreshToken string) (*TokenInfo, error)
}

type QRCodeInfo struct {
    QRCodeURL  string    `json:"qrCodeUrl"`
    QRCodeKey  string    `json:"qrCodeKey"`
    ExpiresAt  time.Time `json:"expiresAt"`
}

type ScanStatus struct {
    Status     string `json:"status"` // waiting, scanned, confirmed, expired
    QRCodeKey  string `json:"qrCodeKey"`
    OpenID     string `json:"openId,omitempty"`
}

type TokenInfo struct {
    AccessToken  string    `json:"accessToken"`
    RefreshToken string    `json:"refreshToken"`
    ExpiresAt    time.Time `json:"expiresAt"`
    UserID       string    `json:"userId"`
}
```

### 6.2 阿里云盘认证

```go
type ProviderAliPanAuth struct {
    // OAuth2 授权码流程
    GetAuthURL(redirectURI string) (string, error)
    ExchangeCode(code string) (*TokenInfo, error)
    RefreshToken(refreshToken string) (*TokenInfo, error)

    // 扫码登录
    GetQRCode() (*QRCodeInfo, error)
    CheckQRCodeStatus(qrCodeKey string) (*QRCodeStatus, error)
}

type AliPanTokenInfo struct {
    AccessToken  string    `json:"accessToken"`
    RefreshToken string    `json:"refreshToken"`
    ExpiresIn    int       `json:"expiresIn"`
    UserID       string    `json:"userId"`
    DriveID      string    `json:"driveId"`    // 个人空间
    ResourceDriveID string `json:"resourceDriveId"` // 资源库
}
```

### 6.3 夸克网盘认证

```go
type ProviderQuarkAuth struct {
    // Cookie 登录
    LoginWithCookie(cookie string) (*TokenInfo, error)

    // 扫码登录
    GetQRCode() (*QRCodeInfo, error)
    CheckQRCodeStatus(qrCodeKey string) (*QRCodeStatus, error)
}
```

### 6.4 认证状态管理

```
┌─────────────────────────────────────────────────────────┐
│                   Auth Manager                           │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Token Storage (加密存储)                          │  │
│  │  - AccessToken                                    │  │
│  │  - RefreshToken                                   │  │
│  │  - ExpiresAt                                      │  │
│  │  - UserID                                         │  │
│  └───────────────────────────────────────────────────┘  │
│                                                          │
│  ┌───────────────────────────────────────────────────┐  │
│  │  Token Lifecycle                                   │  │
│  │  ┌───────────┐    ┌───────────┐    ┌───────────┐  │  │
│  │  │  Valid    │───▶│  Expiring │───▶│  Expired  │  │  │
│  │  └───────────┘    └───────────┘    └─────┬─────┘  │  │
│  │        ▲                                 │        │  │
│  │        │─────────────────────────────────┘        │  │
│  │                    Refresh                        │  │
│  └───────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

## 7. API 接口

### 7.1 RESTful API

```
# 挂载管理
GET    /api/v1/cloudfuse/mounts           # 列出所有挂载
POST   /api/v1/cloudfuse/mounts           # 创建挂载配置
GET    /api/v1/cloudfuse/mounts/:id       # 获取挂载详情
PUT    /api/v1/cloudfuse/mounts/:id       # 更新挂载配置
DELETE /api/v1/cloudfuse/mounts/:id       # 删除挂载配置

# 挂载操作
POST   /api/v1/cloudfuse/mounts/:id/mount   # 执行挂载
POST   /api/v1/cloudfuse/mounts/:id/unmount # 卸载挂载
GET    /api/v1/cloudfuse/mounts/:id/stats   # 获取统计

# 认证
GET    /api/v1/cloudfuse/providers          # 列出支持的网盘
POST   /api/v1/cloudfuse/auth/:provider/qrcode   # 获取登录二维码
GET    /api/v1/cloudfuse/auth/:provider/status    # 检查扫码状态
POST   /api/v1/cloudfuse/auth/:provider/refresh   # 刷新Token

# 测试
POST   /api/v1/cloudfuse/mounts/test        # 测试挂载配置
```

### 7.2 请求/响应示例

**创建挂载请求：**
```json
{
  "name": "我的阿里云盘",
  "type": "aliyun_pan",
  "mountPoint": "/mnt/aliyun",
  "remotePath": "/",
  "autoMount": true,
  "cacheEnabled": true,
  "cacheSize": 1024,
  "accessToken": "xxx",
  "refreshToken": "xxx",
  "driveId": "xxx"
}
```

**挂载信息响应：**
```json
{
  "id": "mount-123456",
  "name": "我的阿里云盘",
  "type": "aliyun_pan",
  "mountPoint": "/mnt/aliyun",
  "status": "mounted",
  "createdAt": "2026-03-24T12:00:00Z",
  "mountedAt": "2026-03-24T12:01:00Z",
  "readBytes": 1048576,
  "writeBytes": 512,
  "cacheHitRate": 0.85
}
```

## 8. 性能优化

### 8.1 并发控制

- 连接池管理
- 请求队列
- 限流控制

### 8.2 预读优化

- 顺序读取预读
- 智能预测
- 流媒体优化

### 8.3 断点续传

```go
type ResumeUpload struct {
    FileID      string    `json:"fileId"`
    UploadID    string    `json:"uploadId"`
    Parts       []PartInfo `json:"parts"`
    Progress    int64     `json:"progress"`
    CreatedAt   time.Time `json:"createdAt"`
}
```

## 9. 错误处理

### 9.1 错误码定义

| 错误码 | 说明 |
|--------|------|
| 1001 | 网盘认证失败 |
| 1002 | Token 已过期 |
| 1003 | 网络连接失败 |
| 1004 | 文件不存在 |
| 1005 | 权限不足 |
| 1006 | 空间不足 |
| 1007 | 挂载点已存在 |
| 1008 | 挂载点不存在 |
| 1009 | FUSE 挂载失败 |

### 9.2 错误恢复

- 自动重试机制
- Token 自动刷新
- 断线重连
- 缓存降级

## 10. 安全考虑

### 10.1 敏感信息存储

- Token 加密存储
- 内存中不持久化明文
- 使用系统密钥库

### 10.2 访问控制

- 挂载点权限管理
- 用户隔离
- 只读模式支持

### 10.3 数据传输

- HTTPS 强制使用
- 证书验证
- 传输加密

## 11. 监控与日志

### 11.1 监控指标

```go
type MountMetrics struct {
    // 基础指标
    Status          MountStatus `json:"status"`
    Uptime          int64       `json:"uptime"`
    LastActivity    time.Time   `json:"lastActivity"`

    // I/O 指标
    TotalReadBytes  int64 `json:"totalReadBytes"`
    TotalWriteBytes int64 `json:"totalWriteBytes"`
    TotalReadOps    int64 `json:"totalReadOps"`
    TotalWriteOps   int64 `json:"totalWriteOps"`

    // 缓存指标
    CacheHitRate    float64 `json:"cacheHitRate"`
    CacheUsedBytes  int64   `json:"cacheUsedBytes"`
    CacheEvictions  int64   `json:"cacheEvictions"`

    // 性能指标
    AvgReadLatency  int64 `json:"avgReadLatencyMs"`
    AvgWriteLatency int64 `json:"avgWriteLatencyMs"`
    ErrorCount      int64 `json:"errorCount"`
}
```

### 11.2 日志级别

- DEBUG: 详细调试信息
- INFO: 正常操作日志
- WARN: 警告信息
- ERROR: 错误信息

## 12. 部署配置

### 12.1 Docker 部署

```yaml
services:
  cloudfuse:
    image: nas-os/cloudfuse:latest
    privileged: true  # FUSE 需要
    volumes:
      - /mnt/cloud:/mnt/cloud:rshared
      - ./config:/etc/nas-os/cloudfuse
      - ./cache:/var/cache/nas-os/cloudfuse
    environment:
      - LOG_LEVEL=info
      - CACHE_SIZE=1024
```

### 12.2 系统要求

- Linux 内核 4.0+
- FUSE 3.0+
- 足够的内存用于缓存
- 可用的挂载点目录

---

**版本**: v1.0
**更新日期**: 2026-03-24
**作者**: NAS-OS 兵部