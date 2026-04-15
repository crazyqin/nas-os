# Cloud Drive Sync - API 参考文档

> 文档版本：v1.0 | 基础路径：`/api/v1/cloudsync`

---

## 目录

1. [通用说明](#1-通用说明)
2. [提供商管理](#2-提供商管理)
3. [同步任务管理](#3-同步任务管理)
4. [同步操作](#4-同步操作)
5. [OAuth2 授权](#5-oauth2-授权)
6. [实时同步](#6-实时同步)
7. [断点续传](#7-断点续传)
8. [统计与状态](#8-统计与状态)
9. [数据结构](#9-数据结构)

---

## 1. 通用说明

### 1.1 基础路径

```
http://<nas-ip>:8080/api/v1/cloudsync
```

### 1.2 认证

所有 API 需要携带认证信息：

```
Authorization: Bearer <token>
```

### 1.3 响应格式

所有响应均采用以下 JSON 格式：

```json
{
  "code": 0,       // 0=成功, 非0=失败
  "message": "...", // 描述信息
  "data": {}        // 响应数据（可选）
}
```

### 1.4 错误码

| Code | HTTP Status | 说明 |
|------|-------------|------|
| 0 | 200 | 成功 |
| 400 | 400 | 请求参数错误 |
| 401 | 401 | 未认证 |
| 403 | 403 | 权限不足 |
| 404 | 404 | 资源不存在 |
| 500 | 500 | 服务内部错误 |

---

## 2. 提供商管理

### 2.1 创建提供商

```
POST /providers
```

**请求体：**

```json
{
  "name": "我的阿里云OSS",
  "type": "aliyun_oss",
  "endpoint": "oss-cn-hangzhou.aliyuncs.com",
  "region": "cn-hangzhou",
  "bucket": "my-bucket",
  "accessKey": "AKID_xxx",
  "secretKey": "xxx",
  "pathStyle": false,
  "maxConnections": 10,
  "timeout": 300,
  "retryCount": 3
}
```

**S3 兼容存储必填字段：**
- `name` - 提供商名称
- `type` - 提供商类型（见下方枚举）
- `accessKey` - 访问密钥
- `secretKey` - 密钥（不会序列化到响应中）
- `bucket` - 存储桶名称

**WebDAV 必填字段：**
- `endpoint` - 服务地址
- `accessKey` - 用户名
- `secretKey` - 密码

**OAuth2 提供商（Google Drive/OneDrive/Dropbox）必填字段：**
- `clientId` - OAuth Client ID
- `clientSecret` - OAuth Client Secret（不会序列化到响应中）
- `refreshToken` - OAuth Refresh Token（不会序列化到响应中）

**中国网盘必填字段：**
- `accessToken` - 访问令牌（不会序列化到响应中）

**提供商类型枚举：**

| 值 | 说明 |
|----|------|
| `aliyun_oss` | 阿里云 OSS |
| `tencent_cos` | 腾讯云 COS |
| `aws_s3` | AWS S3 |
| `google_drive` | Google Drive |
| `onedrive` | Microsoft OneDrive |
| `dropbox` | Dropbox |
| `backblaze_b2` | Backblaze B2 |
| `webdav` | WebDAV |
| `s3_compatible` | S3 兼容存储 |
| `115` | 115网盘 |
| `quark` | 夸克网盘 |
| `aliyun_pan` | 阿里云盘 |
| `baidu_pan` | 百度网盘 |

**响应：**

```json
{
  "code": 0,
  "message": "提供商创建成功",
  "data": {
    "id": "provider_a1b2c3d4",
    "name": "我的阿里云OSS",
    "type": "aliyun_oss",
    "enabled": true,
    "createdAt": "2026-04-15T10:00:00Z",
    "updatedAt": "2026-04-15T10:00:00Z",
    "endpoint": "oss-cn-hangzhou.aliyuncs.com",
    "region": "cn-hangzhou",
    "bucket": "my-bucket",
    "maxConnections": 10,
    "timeout": 300,
    "retryCount": 3
  }
}
```

### 2.2 列出所有提供商

```
GET /providers
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "provider_a1b2c3d4",
      "name": "我的阿里云OSS",
      "type": "aliyun_oss",
      "enabled": true,
      "createdAt": "2026-04-15T10:00:00Z"
    }
  ]
}
```

### 2.3 获取提供商详情

```
GET /providers/:id
```

**路径参数：**
- `id` - 提供商 ID

**响应：** 同创建响应中的 `data` 字段。

### 2.4 更新提供商

```
PUT /providers/:id
```

**请求体：** 同创建请求体。

**响应：**

```json
{
  "code": 0,
  "message": "提供商更新成功"
}
```

### 2.5 删除提供商

```
DELETE /providers/:id
```

**响应：**

```json
{
  "code": 0,
  "message": "提供商已删除"
}
```

**错误情况：**

```json
{
  "code": 500,
  "message": "存在使用此提供商的同步任务，请先删除任务"
}
```

### 2.6 测试连接

```
POST /providers/:id/test
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "provider": "aliyun_oss",
    "endpoint": "oss-cn-hangzhou.aliyuncs.com",
    "bucket": "my-bucket",
    "latencyMs": 45,
    "message": "连接成功"
  }
}
```

### 2.7 获取支持的提供商列表

```
GET /providers-info
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "type": "aliyun_oss",
      "name": "阿里云 OSS",
      "description": "阿里云对象存储服务",
      "features": ["upload", "download", "delete", "list", "multipart"]
    }
  ]
}
```

---

## 3. 同步任务管理

### 3.1 创建同步任务

```
POST /tasks
```

**请求体：**

```json
{
  "name": "照片备份",
  "providerId": "provider_a1b2c3d4",
  "enabled": true,
  "localPath": "/data/photos",
  "remotePath": "/backup/photos",
  "direction": "upload",
  "mode": "backup",
  "scheduleType": "cron",
  "scheduleExpr": "0 2 * * *",
  "includePatterns": ["*.jpg", "*.png", "*.raw"],
  "excludePatterns": [".DS_Store", "tmp/"],
  "maxFileSize": 0,
  "conflictStrategy": "newer",
  "deleteRemote": false,
  "deleteLocal": false,
  "preserveModTime": true,
  "checksumVerify": false,
  "encrypt": false,
  "bandwidthLimit": 0
}
```

**必填字段：**
- `name` - 任务名称
- `providerId` - 提供商 ID（必须已存在）
- `localPath` - 本地路径
- `remotePath` - 远程路径

**默认值：**
- `direction` → `bidirect`
- `mode` → `sync`
- `scheduleType` → `manual`
- `conflictStrategy` → `newer`
- `enabled` → `true`

**方向枚举：** `upload` | `download` | `bidirect`

**模式枚举：** `mirror` | `backup` | `sync` | `increment`

**调度类型枚举：** `manual` | `realtime` | `interval` | `cron`

**冲突策略枚举：** `skip` | `local` | `remote` | `newer` | `rename` | `ask`

**响应：**

```json
{
  "code": 0,
  "message": "同步任务创建成功",
  "data": {
    "id": "task_e5f6g7h8",
    "name": "照片备份",
    "providerId": "provider_a1b2c3d4",
    "enabled": true,
    "localPath": "/data/photos",
    "remotePath": "/backup/photos",
    "direction": "upload",
    "mode": "backup",
    "scheduleType": "cron",
    "scheduleExpr": "0 2 * * *",
    "conflictStrategy": "newer",
    "status": "idle",
    "createdAt": "2026-04-15T10:30:00Z",
    "updatedAt": "2026-04-15T10:30:00Z"
  }
}
```

### 3.2 列出所有同步任务

```
GET /tasks
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "task_e5f6g7h8",
      "name": "照片备份",
      "providerId": "provider_a1b2c3d4",
      "enabled": true,
      "status": "idle",
      "lastSync": "2026-04-14T02:00:00Z"
    }
  ]
}
```

### 3.3 获取同步任务详情

```
GET /tasks/:id
```

### 3.4 更新同步任务

```
PUT /tasks/:id
```

**请求体：** 同创建请求体。

### 3.5 删除同步任务

```
DELETE /tasks/:id
```

> 删除正在运行的任务会先取消，再删除。

---

## 4. 同步操作

### 4.1 触发同步

```
POST /tasks/:id/run
```

**响应：**

```json
{
  "code": 0,
  "message": "同步任务已启动",
  "data": {
    "taskId": "task_e5f6g7h8",
    "status": "running",
    "startTime": "2026-04-15T10:35:00Z",
    "totalFiles": 0,
    "processedFiles": 0,
    "totalBytes": 0,
    "transferredBytes": 0,
    "speed": 0,
    "progress": 0
  }
}
```

### 4.2 暂停同步

```
POST /tasks/:id/pause
```

### 4.3 恢复同步

```
POST /tasks/:id/resume
```

### 4.4 取消同步

```
POST /tasks/:id/cancel
```

### 4.5 获取同步状态

```
GET /tasks/:id/status
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "taskId": "task_e5f6g7h8",
    "status": "running",
    "startTime": "2026-04-15T10:35:00Z",
    "totalFiles": 1500,
    "processedFiles": 750,
    "totalBytes": 5368709120,
    "transferredBytes": 2684354560,
    "speed": 10240,
    "progress": 50.0,
    "currentFile": "photos/2026/IMG_001.jpg",
    "currentAction": "upload",
    "uploadedFiles": 700,
    "downloadedFiles": 0,
    "skippedFiles": 50,
    "failedFiles": 0,
    "deletedFiles": 0,
    "conflicts": [],
    "errors": []
  }
}
```

**任务状态枚举：** `idle` | `running` | `paused` | `completed` | `failed` | `cancelled`

---

## 5. OAuth2 授权

### 5.1 获取授权 URL

```
GET /oauth2/auth-url/:providerType?redirect_url=...&provider_id=...
```

**路径参数：**
- `providerType` - 提供商类型（如 `google_drive`）

**查询参数：**
- `redirect_url` - 授权完成后的回调地址
- `provider_id` - 关联的提供商 ID

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "authUrl": "https://accounts.google.com/o/oauth2/v2/auth?...",
    "state": "random_state_string"
  }
}
```

### 5.2 OAuth2 回调

```
POST /oauth2/callback
```

**请求体：**

```json
{
  "providerType": "google_drive",
  "providerId": "provider_a1b2c3d4",
  "code": "authorization_code",
  "state": "random_state_string"
}
```

**响应：**

```json
{
  "code": 0,
  "message": "授权成功",
  "data": {
    "providerId": "provider_a1b2c3d4",
    "providerType": "google_drive",
    "expiresAt": "2026-04-15T11:00:00Z"
  }
}
```

### 5.3 删除 OAuth2 Token

```
DELETE /oauth2/token/:providerId
```

### 5.4 列出 OAuth2 Token

```
GET /oauth2/tokens
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "providerId": "provider_a1b2c3d4",
      "providerType": "google_drive",
      "expiresAt": "2026-04-15T11:00:00Z",
      "updatedAt": "2026-04-15T10:00:00Z"
    }
  ]
}
```

> 注意：Token 列表不包含 access_token 和 refresh_token 等敏感信息。

---

## 6. 实时同步

### 6.1 获取实时同步状态

```
GET /realtime/status
```

### 6.2 启动实时同步

```
POST /realtime/start
```

### 6.3 停止实时同步

```
POST /realtime/stop
```

### 6.4 添加监控路径

```
POST /realtime/watch/:taskId
```

### 6.5 移除监控路径

```
DELETE /realtime/watch/:taskId
```

---

## 7. 断点续传

### 7.1 获取断点续传状态

```
GET /resumable/status
```

### 7.2 获取待恢复上传列表

```
GET /resumable/pending
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "fileId": "upload_x9y8z7",
      "localPath": "/data/videos/movie.mp4",
      "remotePath": "/backup/videos/movie.mp4",
      "fileSize": 5368709120,
      "uploadedSize": 2684354560,
      "uploadedChunks": 512,
      "totalChunks": 1024,
      "status": "paused",
      "startTime": "2026-04-14T20:00:00Z",
      "lastError": ""
    }
  ]
}
```

### 7.3 恢复上传

```
POST /resumable/resume/:fileId
```

---

## 8. 统计与状态

### 8.1 获取所有任务状态

```
GET /statuses
```

**响应：** 返回所有任务（包括未运行任务）的状态映射。

### 8.2 获取统计信息

```
GET /stats
```

**响应：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "totalTasks": 5,
    "activeTasks": 2,
    "totalProviders": 3,
    "totalSynced": 15234,
    "totalBytes": 1099511627776,
    "totalBytesHuman": "1.00 TB",
    "lastSyncTime": "2026-04-15T10:30:00Z"
  }
}
```

---

## 9. 数据结构

### 9.1 ProviderConfig

```go
type ProviderConfig struct {
    ID        string       `json:"id"`
    Name      string       `json:"name"`
    Type      ProviderType `json:"type"`
    Enabled   bool         `json:"enabled"`
    CreatedAt time.Time    `json:"createdAt"`
    UpdatedAt time.Time    `json:"updatedAt"`
    LastUsed  time.Time    `json:"lastUsed,omitempty"`

    // S3 兼容存储
    Endpoint  string `json:"endpoint,omitempty"`
    Region    string `json:"region,omitempty"`
    Bucket    string `json:"bucket,omitempty"`
    AccessKey string `json:"accessKey,omitempty"`
    SecretKey string `json:"-"` // 不序列化
    PathStyle bool   `json:"pathStyle,omitempty"`

    // OAuth2
    ClientID     string `json:"clientId,omitempty"`
    ClientSecret string `json:"-"`
    RefreshToken string `json:"-"`
    RootFolderID string `json:"rootFolderId,omitempty"`

    // 中国网盘
    AccessToken string `json:"-"`
    UserID      string `json:"userId,omitempty"`
    DriveID     string `json:"driveId,omitempty"`

    // 通用配置
    MaxConnections int `json:"maxConnections,omitempty"`
    Timeout        int `json:"timeout,omitempty"`
    RetryCount     int `json:"retryCount,omitempty"`
}
```

### 9.2 SyncTask

```go
type SyncTask struct {
    ID         string    `json:"id"`
    Name       string    `json:"name"`
    ProviderID string    `json:"providerId"`
    Enabled    bool      `json:"enabled"`
    CreatedAt  time.Time `json:"createdAt"`
    UpdatedAt  time.Time `json:"updatedAt"`

    LocalPath  string        `json:"localPath"`
    RemotePath string        `json:"remotePath"`
    Direction  SyncDirection `json:"direction"`
    Mode       SyncMode      `json:"mode"`

    ScheduleType ScheduleType `json:"scheduleType"`
    ScheduleExpr string       `json:"scheduleExpr,omitempty"`
    NextRun      time.Time    `json:"nextRun,omitempty"`

    IncludePatterns []string `json:"includePatterns,omitempty"`
    ExcludePatterns []string `json:"excludePatterns,omitempty"`
    MaxFileSize     int64    `json:"maxFileSize,omitempty"`

    ConflictStrategy ConflictStrategy `json:"conflictStrategy"`

    DeleteRemote    bool `json:"deleteRemote"`
    DeleteLocal     bool `json:"deleteLocal"`
    PreserveModTime bool `json:"preserveModTime"`
    ChecksumVerify  bool `json:"checksumVerify"`
    Encrypt         bool `json:"encrypt"`

    BandwidthLimit int64 `json:"bandwidthLimit,omitempty"`

    Status    TaskStatus `json:"status"`
    LastSync  time.Time  `json:"lastSync,omitempty"`
    LastError string     `json:"lastError,omitempty"`
}
```

### 9.3 SyncStatus

```go
type SyncStatus struct {
    TaskID    string     `json:"taskId"`
    Status    TaskStatus `json:"status"`
    StartTime time.Time  `json:"startTime,omitempty"`
    EndTime   time.Time  `json:"endTime,omitempty"`

    TotalFiles       int64   `json:"totalFiles"`
    ProcessedFiles   int64   `json:"processedFiles"`
    TotalBytes       int64   `json:"totalBytes"`
    TransferredBytes int64   `json:"transferredBytes"`
    Speed            int64   `json:"speed"`
    Progress         float64 `json:"progress"`

    CurrentFile   string `json:"currentFile,omitempty"`
    CurrentAction string `json:"currentAction,omitempty"`

    UploadedFiles   int64 `json:"uploadedFiles"`
    DownloadedFiles int64 `json:"downloadedFiles"`
    SkippedFiles    int64 `json:"skippedFiles"`
    FailedFiles     int64 `json:"failedFiles"`
    DeletedFiles    int64 `json:"deletedFiles"`

    Conflicts []ConflictInfo `json:"conflicts,omitempty"`
    Errors    []SyncError    `json:"errors,omitempty"`
}
```

### 9.4 ConflictInfo

```go
type ConflictInfo struct {
    Path          string           `json:"path"`
    LocalModTime  time.Time        `json:"localModTime"`
    LocalSize     int64            `json:"localSize"`
    LocalHash     string           `json:"localHash,omitempty"`
    RemoteModTime time.Time        `json:"remoteModTime"`
    RemoteSize    int64            `json:"remoteSize"`
    RemoteHash    string           `json:"remoteHash,omitempty"`
    Resolution    ConflictStrategy `json:"resolution,omitempty"`
}
```

### 9.5 SyncError

```go
type SyncError struct {
    Time   time.Time `json:"time"`
    Path   string    `json:"path"`
    Action string    `json:"action"`
    Error  string    `json:"error"`
}
```

### 9.6 ConnectionTestResult

```go
type ConnectionTestResult struct {
    Success   bool         `json:"success"`
    Provider  ProviderType `json:"provider"`
    Endpoint  string       `json:"endpoint"`
    Bucket    string       `json:"bucket,omitempty"`
    LatencyMs int64        `json:"latencyMs"`
    Message   string       `json:"message"`
    Error     string       `json:"error,omitempty"`
}
```

### 9.7 ResolutionResult

```go
type ResolutionResult struct {
    OriginalPath  string     `json:"originalPath"`
    RenamedPath   string     `json:"renamedPath,omitempty"`
    Action        SyncOpType `json:"action"`
    Message       string     `json:"message"`
    NeedUserInput bool       `json:"needUserInput"`
}
```

### 9.8 SyncStats

```go
type SyncStats struct {
    TotalTasks      int64     `json:"totalTasks"`
    ActiveTasks     int64     `json:"activeTasks"`
    TotalProviders  int64     `json:"totalProviders"`
    TotalSynced     int64     `json:"totalSynced"`
    TotalBytes      int64     `json:"totalBytes"`
    TotalBytesHuman string    `json:"totalBytesHuman"`
    LastSyncTime    time.Time `json:"lastSyncTime,omitempty"`
}
```
