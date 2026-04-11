# WebShare 架构设计文档

**版本**: v1.0 | **日期**: 2026-04-11 | **对标**: TrueNAS 26 WebShare

---

## 1. 概述

### 1.1 WebShare 定位

WebShare 是 NAS-OS 的浏览器文件访问服务，让用户无需配置 SMB/NFS/WebDAV 客户端，即可通过浏览器直接浏览、上传、下载和管理文件。

**核心价值**:
- 零客户端配置 - 打开浏览器即可访问
- 跨平台兼容 - 支持桌面、移动设备
- 安全分享 - 限时、限次、密码保护
- 内容搜索 - TrueSearch 全文检索

### 1.2 TrueNAS 26 WebShare 核心特性学习

| 特性 | TrueNAS 26 | 说明 |
|------|------------|------|
| **浏览器文件访问** | ✅ | 无需 SMB/NFS 客户端 |
| **上传/下载** | ✅ | 支持大文件分片上传、断点续传 |
| **文件夹创建** | ✅ | 右键菜单、拖拽创建 |
| **快照时间线** | ✅ | ZFS 快照历史版本查看与恢复 |
| **分享链接** | ✅ | 限时、限次、密码保护 |
| **隐藏文件切换** | ✅ | 显示/隐藏 `.*` 文件 |
| **TrueSearch** | ✅ | 文件名、内容、类型全文搜索 |
| **Passkey认证** | ✅ | WebAuthn 无密码登录 |

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           NAS-OS WebShare Architecture                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                         Frontend Layer                               │   │
│   │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │   │
│   │  │  Web Browser │  │  Mobile Web  │  │  Share Link  │               │   │
│   │  │   (Chrome)   │  │   (Safari)   │  │  (Anonymous) │               │   │
│   │  └──────────────┘  └──────────────┘  └──────────────┘               │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                          │
│                                    ▼ HTTPS                                   │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                       API Gateway Layer                              │   │
│   │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │   │
│   │  │  Gin Router  │  │   Auth MW    │  │  Rate Limit  │               │   │
│   │  │  /api/v1/ws  │  │ Passkey/JWT  │  │  Middleware  │               │   │
│   │  └──────────────┘  └──────────────┘  └──────────────┘               │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                          │
│                                    ▼                                          │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                     WebShare Service Layer                           │   │
│   │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐    │   │
│   │  │ Browse API │  │ Upload API │  │ Share API  │  │ Search API │    │   │
│   │  │ (目录浏览) │  │ (分片上传) │  │ (链接管理) │  │ (全文检索) │    │   │
│   │  └────────────┘  └────────────┘  └────────────┘  └────────────┘    │   │
│   │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐    │   │
│   │  │Download API│  │Snapshot API│  │  Preview   │  │  Passkey   │    │   │
│   │  │ (批量下载) │  │ (时间线)   │  │  Service   │  │  Auth      │    │   │
│   │  └────────────┘  └────────────┘  └────────────┘  └────────────┘    │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                          │
│                                    ▼                                          │
│   ┌─────────────────────────────────────────────────────────────────────┐   │
│   │                      Storage & Index Layer                           │   │
│   │  ┌────────────┐  ┌────────────┐  ┌────────────┐  ┌────────────┐    │   │
│   │  │ ZFS Pool   │  │  Bleve     │  │  Metadata  │  │  Snapshot  │    │   │
│   │  │ (File Data)│  │  Index     │  │   Cache    │  │  Manager   │    │   │
│   │  └────────────┘  └────────────┘  └────────────┘  └────────────┘    │   │
│   └─────────────────────────────────────────────────────────────────────┘   │
│                                                                               │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.2 核心模块结构

```
internal/webshare/
├── webshare.go              # 主服务入口
├── webshare_test.go         # 测试
├── browse.go                # 目录浏览
├── upload.go                # 文件上传（分片）
├── download.go              # 文件下载（批量）
├── share.go                 # 分享链接管理
├── snapshot.go              # 快照时间线
├── preview.go               # 文件预览
├── bleve_index.go           # Bleve 全文索引
├── content_search.go        # 内容搜索
├── searchindex.go           # 搜索索引构建
├── hidden_files.go          # 隐藏文件切换
├── handlers.go              # HTTP handlers
├── middleware.go            # 认证/限流中间件
├── config.go                # 配置
├── types.go                 # 类型定义

internal/auth/
├── passkey.go               # Passkey/WebAuthn 认证
├── webauthn.go              # WebAuthn 管理器
```

---

## 3. 核心功能设计

### 3.1 浏览器文件访问

#### API 设计

**目录浏览 API**:

```yaml
GET /api/v1/webshare/browse
Parameters:
  path: /shared/documents    # 目录路径
  showHidden: false          # 显示隐藏文件
  sortBy: name               # 排序字段 (name/size/time)
  sortDesc: false            # 降序
  page: 1                    # 分页
  limit: 100                 # 每页数量

Response:
  path: "/shared/documents"
  breadcrumb: ["shared", "documents"]
  files:
    - name: "工作报告.docx"
      type: "file"
      size: 2300000
      modified: "2026-04-11T10:00:00Z"
      mime: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
      isHidden: false
      snapshotCount: 5       # 快照版本数
    - name: ".config"
      type: "directory"
      isHidden: true
      modified: "2026-04-10T08:00:00Z"
  total: 150
  page: 1
```

**隐藏文件切换实现**:

```go
type BrowseOptions struct {
    Path       string
    ShowHidden bool      // 切换显示隐藏文件
    SortBy     string
    SortDesc   bool
    Page       int
    Limit      int
}

func (ws *WebShare) ListFiles(opts BrowseOptions) ([]FileInfo, error) {
    entries, err := os.ReadDir(opts.Path)
    if err != nil {
        return nil, err
    }

    files := []FileInfo{}
    for _, entry := range entries {
        // 隐藏文件过滤
        isHidden := strings.HasPrefix(entry.Name(), ".")
        if isHidden && !opts.ShowHidden {
            continue
        }
        
        info, err := entry.Info()
        if err != nil {
            continue
        }
        
        files.append(FileInfo{
            Name:      entry.Name(),
            Type:      entry.Type().String(),
            Size:      info.Size(),
            Modified:  info.ModTime(),
            IsHidden:  isHidden,
        })
    }
    
    // 排序
    sortFiles(files, opts.SortBy, opts.SortDesc)
    
    return files, nil
}
```

### 3.2 上传/下载

#### 分片上传架构

```
┌─────────────────────────────────────────────────────────────┐
│                    Chunked Upload Flow                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Client                    Server                           │
│    │                         │                             │
│    │ POST /upload/init       │                             │
│    │ {size, name, path}      │                             │
│    │────────────────────────▶│                             │
│    │                         │ 创建 upload_id              │
│    │ {upload_id, chunk_size} │                             │
│    │◀────────────────────────│                             │
│    │                         │                             │
│    │ POST /upload/chunk      │                             │
│    │ {upload_id, index, data}│                             │
│    │────────────────────────▶│                             │
│    │                         │ 写入临时文件                │
│    │ {success, written}      │                             │
│    │◀────────────────────────│                             │
│    │         ...             │                             │
│    │                         │                             │
│    │ POST /upload/complete   │                             │
│    │ {upload_id}             │                             │
│    │────────────────────────▶│                             │
│    │                         │ 合并分片 → 最终文件         │
│    │                         │ 更新索引                    │
│    │ {success, final_path}   │                             │
│    │◀────────────────────────│                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

**配置参数**:

```yaml
webshare:
  upload:
    chunk_size: 10MB           # 分片大小
    max_file_size: 5GB         # 单文件上限
    max_total_size: 20GB       # 批量上传上限
    timeout: 3600              # 上传超时(秒)
    concurrent_chunks: 3       # 并发分片数
    temp_dir: /tmp/webshare    # 临时目录
```

#### 断点续传实现

```go
type UploadSession struct {
    UploadID     string
    FileName     string
    TargetPath   string
    TotalSize    int64
    ChunkSize    int
    UploadedChunks map[int]bool    # 已上传分片索引
    TempFile     *os.File
    CreatedAt    time.Time
    ExpiresAt    time.Time
}

func (ws *WebShare) ResumeUpload(uploadID string) (*UploadSession, error) {
    session := ws.sessions[uploadID]
    if session == nil {
        return nil, ErrSessionNotFound
    }
    
    if time.Now().After(session.ExpiresAt) {
        ws.cleanupSession(uploadID)
        return nil, ErrSessionExpired
    }
    
    // 返回已上传分片信息，客户端可跳过已上传部分
    return session, nil
}
```

### 3.3 快照时间线查看

#### ZFS 快照集成

```go
type SnapshotTimeline struct {
    Path        string
    Snapshots   []SnapshotInfo
    CurrentFile FileInfo
}

type SnapshotInfo struct {
    Name        string      # 快照名称 (e.g., "auto-2026-04-11-10:00")
    CreatedAt   time.Time
    FileVersion FileVersion # 该快照中的文件版本
    Size        int64
    Changed     bool        # 是否有变化
}

func (ws *WebShare) GetSnapshotTimeline(path string) (*SnapshotTimeline, error) {
    // 获取 ZFS 数据集快照列表
    dataset := getZfsDataset(path)
    snapshots, err := zfs.ListSnapshots(dataset)
    if err != nil {
        return nil, err
    }
    
    timeline := &SnapshotTimeline{Path: path}
    
    for _, snap := range snapshots {
        // 查找快照中的文件版本
        snapPath := pathInSnapshot(path, snap.Name)
        if exists(snapPath) {
            info := getFileInfo(snapPath)
            timeline.Snapshots.append(SnapshotInfo{
                Name:      snap.Name,
                CreatedAt: snap.Created,
                FileVersion: info,
                Changed:   isDifferent(info, timeline.CurrentFile),
            })
        }
    }
    
    return timeline, nil
}
```

**API 设计**:

```yaml
GET /api/v1/webshare/snapshots?path=/shared/documents/report.docx

Response:
  path: "/shared/documents/report.docx"
  current:
    name: "report.docx"
    size: 2300000
    modified: "2026-04-11T10:00:00Z"
  timeline:
    - name: "auto-2026-04-11-08:00"
      created: "2026-04-11T08:00:00Z"
      size: 2200000
      changed: true
    - name: "auto-2026-04-10-20:00"
      created: "2026-04-10T20:00:00Z"
      size: 2000000
      changed: true
    - name: "manual-2026-04-10-10:00"
      created: "2026-04-10T10:00:00Z"
      size: 1500000
      changed: true
```

**快照恢复 API**:

```yaml
POST /api/v1/webshare/snapshots/restore
Request:
  path: "/shared/documents/report.docx"
  snapshot: "auto-2026-04-10-20:00"
  
Response:
  success: true
  restoredPath: "/shared/documents/report.docx"
  restoredSize: 2000000
  restoredTime: "2026-04-10T20:00:00Z"
```

### 3.4 分享链接管理

#### 分享链接类型

| 类型 | 场景 | 特性 |
|------|------|------|
| **限时分享** | 短期合作 | 过期时间 1-24h |
| **限次分享** | 限制传播 | 访问次数 1-10 次 |
| **密码分享** | 安全要求高 | 密码保护 + 限时 |
| **公开分享** | 大范围分发 | 无密码，长期有效 |
| **匿名分享** | 快速分享 | 无需登录即可访问 |

#### 分享链接数据结构

```go
type ShareLink struct {
    ShareID       string      # 分享 ID (短链接)
    Path          string      # 分享路径
    Owner         string      # 创建者
    CreatedAt     time.Time
    ExpiresAt     *time.Time  # 过期时间 (nil = 永久)
    MaxAccess     int         # 最大访问次数 (0 = 无限)
    AccessCount   int         # 已访问次数
    Password      string      # 访问密码 (可选)
    Permissions   SharePerms  # 权限配置
    IsActive      bool        # 是否激活
}

type SharePerms struct {
    CanPreview    bool        # 可预览
    CanDownload   bool        # 可下载
    CanUpload     bool        # 可上传 (目录分享)
    CanList       bool        # 可浏览目录
    ShowHidden    bool        # 显示隐藏文件
}
```

#### 分享链接 API

```yaml
# 创建分享
POST /api/v1/webshare/share/create
Request:
  path: "/shared/photos/vacation"
  expires: "24h"           # 过期时间
  maxAccess: 10            # 最大访问次数
  password: "secret123"    # 密码 (可选)
  permissions:
    preview: true
    download: true
    list: true
    
Response:
  shareId: "abc123"
  url: "https://nas.example.com/s/abc123"
  expiresAt: "2026-04-12T10:00:00Z"

# 访问分享链接
GET /s/{shareId}?password=secret123

Response:
  如果是文件: 返回文件预览/下载页面
  如果是目录: 返回目录浏览页面

# 分享管理
GET /api/v1/webshare/share/list
Response:
  shares:
    - shareId: "abc123"
      path: "/shared/photos/vacation"
      created: "2026-04-11T10:00:00Z"
      expiresAt: "2026-04-12T10:00:00Z"
      accessCount: 5
      isActive: true
```

### 3.5 TrueSearch 内容索引搜索

#### 搜索架构

```
┌─────────────────────────────────────────────────────────────┐
│                     TrueSearch Architecture                  │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                 Search Query Layer                   │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │   │
│  │  │ Keyword     │  │ Full-Text   │  │ Semantic    │  │   │
│  │  │ Search      │  │ Search      │  │ Search      │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│                           ▼                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  Bleve Index Engine                  │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │   │
│  │  │ File Name   │  │ File Content│  │ Metadata    │  │   │
│  │  │ Index       │  │ Index       │  │ Index       │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  │   │
│  │                                                     │   │
│  │  ┌─────────────────────────────────────────────┐   │   │
│  │  │         CJK Analyzer (中文分词)              │   │   │
│  │  └─────────────────────────────────────────────┘   │   │
│  └─────────────────────────────────────────────────────┘   │
│                           │                                 │
│                           ▼                                 │
│  ┌─────────────────────────────────────────────────────┐   │
│  │                  Index Manager                      │   │
│  │  - File Scanner                                     │   │
│  │  - Incremental Updater                              │   │
│  │  - Encryption Detector                              │   │
│  │  - Permission Filter                                │   │
│  └─────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### 搜索类型

| 类型 | 说明 | 实现 |
|------|------|------|
| **文件名搜索** | 匹配文件名/路径 | Bleve keyword analyzer |
| **内容搜索** | 文件内容全文检索 | Bleve text analyzer + CJK |
| **类型搜索** | 按文件类型过滤 | 元数据索引 |
| **时间搜索** | 按时间范围过滤 | 日期字段索引 |
| **大小搜索** | 按大小范围过滤 | 数值字段索引 |

#### 搜索 API

```yaml
POST /api/v1/webshare/search
Request:
  query: "财务分析"
  type: "all"              # file/content/metadata/all
  paths: ["/shared/docs"]  # 路径限制
  filters:
    ext: [".pdf", ".docx"]
    minSize: 0
    maxSize: 10485760      # 10MB
    fromDate: "2026-01-01"
    toDate: "2026-04-11"
  limit: 50
  offset: 0
  highlight: true
  
Response:
  results:
    - path: "/shared/docs/财务报告.pdf"
      name: "财务报告.pdf"
      size: 2048000
      score: 0.95
      highlights:
        - field: "content"
          fragments: ["...财务分析显示..."]
  total: 15
  took_ms: 25
```

### 3.6 Passkey 认证

#### WebAuthn 流程

```
┌─────────────────────────────────────────────────────────────┐
│                    Passkey Authentication                   │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Registration Flow:                                         │
│                                                             │
│  Browser                    Server                          │
│    │                          │                             │
│    │ GET /passkey/register    │                             │
│    │─────────────────────────▶│                             │
│    │                          │                             │
│    │ {challenge, options}     │                             │
│    │◀─────────────────────────│                             │
│    │                          │                             │
│    │ navigator.credentials    │                             │
│    │   .create(options)       │                             │
│    │                          │                             │
│    │ POST /passkey/register   │                             │
│    │ {credential, signature}  │                             │
│    │─────────────────────────▶│                             │
│    │                          │ Verify signature            │
│    │                          │ Store credential            │
│    │ {success}                │                             │
│    │◀─────────────────────────│                             │
│                                                             │
│  Authentication Flow:                                       │
│                                                             │
│  Browser                    Server                          │
│    │                          │                             │
│    │ GET /passkey/auth        │                             │
│    │─────────────────────────▶│                             │
│    │                          │                             │
│    │ {challenge, options}     │                             │
│    │◀─────────────────────────│                             │
│    │                          │                             │
│    │ navigator.credentials    │                             │
│    │   .get(options)          │                             │
│    │                          │                             │
│    │ POST /passkey/auth       │                             │
│    │ {credential, signature}  │                             │
│    │─────────────────────────▶│                             │
│    │                          │ Verify signature            │
│    │                          │ Lookup user                 │
│    │ {success, user, token}   │                             │
│    │◀─────────────────────────│                             │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

#### nas-os 已有实现

nas-os 已在 `internal/auth/passkey.go` 实现 Passkey 认证：

- `PasskeyManager` - Passkey 管理器
- `PasskeyCredential` - 凭据存储
- `PasskeySession` - 会话管理
- `BeginPasskeyRegistration` - 开始注册
- `FinishPasskeyRegistration` - 完成注册
- `BeginPasskeyAuthentication` - 开始认证
- `FinishPasskeyAuthentication` - 完成认证
- `BeginPasskeyAuthenticationAuto` - 自动认证（浏览器选择）

#### WebShare 集成 Passkey

```go
// WebShare Passkey 认证中间件
func PasskeyAuthMiddleware(passkey *PasskeyManager) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 从 cookie/header 获取 session token
        token := c.GetHeader("Authorization")
        if token == "" {
            token = c.Cookie("passkey_session")
        }
        
        // 验证 session
        session, err := passkey.ValidateSession(token)
        if err != nil {
            c.JSON(401, gin.H{"error": "未认证"})
            c.Abort()
            return
        }
        
        // 设置用户信息
        c.Set("user_id", session.UserID)
        c.Set("username", session.Username)
        c.Next()
    }
}
```

---

## 4. 安全设计

### 4.1 认证与授权

| 认证方式 | 场景 | 安全级别 |
|----------|------|----------|
| **Passkey** | 主要用户登录 | 高（无密码） |
| **JWT Token** | API 调用 | 中（有密码） |
| **分享密码** | 匿名分享链接 | 低（简单密码） |
| **匿名访问** | 公开分享 | 无 |

### 4.2 权限继承

WebShare 继承 NAS-OS 文件系统权限：

```go
type FileAccess struct {
    UserID      string
    Path        string
    CanRead     bool      // 读取权限
    CanWrite    bool      // 写入权限
    CanDelete   bool      // 删除权限
    CanShare    bool      // 分享权限
    CanSnapshot bool      // 快照查看权限
}

func (ws *WebShare) CheckPermission(userID, path, action string) bool {
    // 获取文件 ACL
    acl := ws.aclManager.GetACL(path)
    
    // 检查用户权限
    userPerms := acl.GetUserPermissions(userID)
    
    switch action {
    case "read":
        return userPerms.Read
    case "write":
        return userPerms.Write
    case "delete":
        return userPerms.Delete
    case "share":
        return userPerms.Share || userPerms.Write
    }
    
    return false
}
```

### 4.3 安全特性

| 特性 | 说明 | 实现 |
|------|------|------|
| **HTTPS 加密** | 全站 HTTPS | TLS 配置 |
| **审计日志** | 所有操作记录 | audit 模块 |
| **病毒扫描** | 上传文件扫描 | clamav 集成 |
| **勒索防护** | WriteOnce WORM | ransomware 模块 |
| **加密数据集** | 不索引内容 | zfs encryption 检测 |
| **IP 白名单** | 限制访问来源 | IP 过滤中间件 |
| **速率限制** | 防滥用 | Rate limit 中间件 |

---

## 5. 性能优化

### 5.1 关键指标

| 指标 | 目标值 | 说明 |
|------|--------|------|
| 目录浏览延迟 | <100ms | 1000 文件目录 |
| 文件上传速度 | 50MB/s | 分片上传 |
| 搜索延迟 | <200ms | 10 万文件索引 |
| 索引速度 | 1000 文件/秒 | 后台构建 |
| 内存占用 | <500MB | 索引服务 |

### 5.2 优化策略

1. **分片上传并发** - 多分片并发上传
2. **批量索引** - 100 文件/批次
3. **增量索引** - fsnotify 监听变更
4. **索引缓存** - Bleve 内置缓存
5. **异步预览** - 后台生成预览图

---

## 6. 与 TrueNAS 26 对比

| 功能 | TrueNAS 26 | nas-os 现状 | 差距 |
|------|------------|-------------|------|
| 浏览器文件访问 | ✅ | ✅ 已有 | 无 |
| 分片上传 | ✅ | ✅ 已有 | 无 |
| 快照时间线 | ✅ | 🚧 部分 | 需完善 UI |
| 分享链接 | ✅ | ✅ 已有 | 无 |
| 隐藏文件切换 | ✅ | ✅ 已有 | 无 |
| TrueSearch | ✅ Bleve | ✅ Bleve | 无 |
| Passkey 认证 | ✅ | ✅ 已有 | 无 |
| 加密数据集检测 | ✅ | 🚧 待完善 | 需添加 |

---

## 7. 开发计划

### Phase 1 (v2.450.0) - 核心完善

- ✅ 浏览器文件访问（已有）
- ✅ 分片上传/下载（已有）
- ✅ 分享链接（已有）
- ✅ Passkey 认证（已有）
- 🚧 快照时间线 UI
- 🚧 加密数据集检测

### Phase 2 (v2.500.0) - 功能增强

- 📋 搜索建议/补全
- 📋 文件预览增强
- 📋 离线下载
- 📋 移动端优化

### Phase 3 (v2.600.0) - 高级特性

- 📋 AI 语义搜索
- 📋 文件内容预览（PDF/Office）
- 📋 批量编辑
- 📋 版本对比

---

## 8. 参考资源

- TrueNAS 26 文档: https://www.truenas.com/docs/scale/webui/
- WebAuthn 规范: https://www.w3.org/TR/webauthn/
- Bleve 搜索引擎: https://blevesearch.github.io/bleve/
- ZFS 快照管理: https://openzfs.github.io/openzfs-docs/

---

**兵部软件工程 | 2026-04-11**