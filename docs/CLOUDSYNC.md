# 云同步配置指南

**版本**: v1.8.0  
**更新日期**: 2026-03-20

---

## 📋 目录

1. [概述](#概述)
2. [支持的云存储](#支持的云存储)
3. [快速开始](#快速开始)
4. [云存储配置](#云存储配置)
5. [同步任务配置](#同步任务配置)
6. [API 参考](#api-参考)
7. [最佳实践](#最佳实践)
8. [常见问题](#常见问题)

---

## 概述

云同步模块为 NAS-OS 提供了强大的多云存储同步能力。支持主流云存储服务商，提供灵活的同步策略和冲突解决方案，实现本地数据与云端的无缝同步。

### 核心功能

| 功能 | 说明 |
|------|------|
| 多云支持 | 支持阿里云 OSS、腾讯云 COS、AWS S3、Google Drive、OneDrive、Backblaze B2 |
| 双向同步 | 本地↔云端实时同步 |
| 增量同步 | 仅传输变更部分，节省带宽 |
| 冲突解决 | 多种自动冲突处理策略 |
| 定时同步 | 支持 Cron 表达式调度 |
| 实时监控 | 文件变更实时同步 |
| 加密传输 | 支持端到端加密 |

---

## 支持的云存储

| 提供商 | 类型 | 特性 |
|--------|------|------|
| 阿里云 OSS | `aliyun_oss` | 分片上传、断点续传 |
| 腾讯云 COS | `tencent_cos` | 分片上传、断点续传 |
| AWS S3 | `aws_s3` | 分片上传、版本控制 |
| Google Drive | `google_drive` | 文件共享、协作 |
| OneDrive | `onedrive` | Office 集成、分享 |
| Backblaze B2 | `backblaze_b2` | 低成本、S3 兼容 |
| WebDAV | `webdav` | 通用协议 |
| S3 兼容存储 | `s3_compatible` | MinIO、Ceph 等 |

---

## 快速开始

### 1. 添加云存储提供商

```bash
# 添加阿里云 OSS
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的阿里云OSS",
    "type": "aliyun_oss",
    "endpoint": "oss-cn-hangzhou.aliyuncs.com",
    "region": "cn-hangzhou",
    "bucket": "my-bucket",
    "accessKey": "YOUR_ACCESS_KEY",
    "secretKey": "YOUR_SECRET_KEY"
  }'
```

### 2. 创建同步任务

```bash
# 创建双向同步任务
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "照片备份",
    "providerId": "provider-001",
    "localPath": "/data/photos",
    "remotePath": "/backup/photos",
    "direction": "bidirect",
    "mode": "sync",
    "scheduleType": "realtime",
    "conflictStrategy": "newer"
  }'
```

### 3. 执行同步

```bash
# 手动触发同步
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks/task-001/run" \
  -H "Authorization: Bearer TOKEN"
```

### 4. 查看同步状态

```bash
# 获取同步状态
curl "http://localhost:8080/api/v1/cloudsync/tasks/task-001/status" \
  -H "Authorization: Bearer TOKEN"
```

---

## 云存储配置

### 阿里云 OSS

```json
{
  "name": "阿里云OSS",
  "type": "aliyun_oss",
  "endpoint": "oss-cn-hangzhou.aliyuncs.com",
  "region": "cn-hangzhou",
  "bucket": "your-bucket",
  "accessKey": "YOUR_ACCESS_KEY_ID",
  "secretKey": "YOUR_ACCESS_KEY_SECRET",
  "maxConnections": 10,
  "timeout": 300,
  "retryCount": 3
}
```

**参数说明**:
- `endpoint`: OSS 地域节点地址
- `region`: 地域 ID
- `bucket`: 存储桶名称
- `accessKey`: AccessKey ID
- `secretKey`: AccessKey Secret

### 腾讯云 COS

```json
{
  "name": "腾讯云COS",
  "type": "tencent_cos",
  "endpoint": "cos.ap-guangzhou.myqcloud.com",
  "region": "ap-guangzhou",
  "bucket": "your-bucket-1250000000",
  "accessKey": "YOUR_SECRET_ID",
  "secretKey": "YOUR_SECRET_KEY"
}
```

### AWS S3

```json
{
  "name": "AWS S3",
  "type": "aws_s3",
  "region": "us-east-1",
  "bucket": "your-bucket",
  "accessKey": "YOUR_ACCESS_KEY_ID",
  "secretKey": "YOUR_SECRET_ACCESS_KEY"
}
```

### Google Drive

```json
{
  "name": "Google Drive",
  "type": "google_drive",
  "clientId": "YOUR_CLIENT_ID",
  "clientSecret": "YOUR_CLIENT_SECRET",
  "refreshToken": "YOUR_REFRESH_TOKEN",
  "rootFolderId": "root"
}
```

**获取 Refresh Token**:
1. 在 Google Cloud Console 创建 OAuth 2.0 客户端
2. 授权应用访问 Drive
3. 获取 refresh_token 用于 API 调用

### OneDrive

```json
{
  "name": "OneDrive",
  "type": "onedrive",
  "clientId": "YOUR_CLIENT_ID",
  "clientSecret": "YOUR_CLIENT_SECRET",
  "tenantId": "common",
  "refreshToken": "YOUR_REFRESH_TOKEN"
}
```

### Backblaze B2

```json
{
  "name": "Backblaze B2",
  "type": "backblaze_b2",
  "endpoint": "https://s3.us-west-002.backblazeb2.com",
  "bucket": "your-bucket",
  "accessKey": "YOUR_KEY_ID",
  "secretKey": "YOUR_APPLICATION_KEY"
}
```

### WebDAV

```json
{
  "name": "WebDAV",
  "type": "webdav",
  "endpoint": "https://webdav.example.com",
  "accessKey": "username",
  "secretKey": "password",
  "insecure": false
}
```

---

## 同步任务配置

### 同步方向

| 方向 | 说明 | 适用场景 |
|------|------|----------|
| `upload` | 本地 → 云端 | 数据备份 |
| `download` | 云端 → 本地 | 数据恢复 |
| `bidirect` | 双向同步 | 多设备协作 |

### 同步模式

| 模式 | 说明 |
|------|------|
| `mirror` | 镜像模式：本地为主，云端完全同步本地状态 |
| `backup` | 备份模式：保留历史版本 |
| `sync` | 同步模式：双向同步，保留双方变更 |
| `increment` | 增量模式：仅同步变更部分 |

### 调度类型

| 类型 | 说明 |
|------|------|
| `manual` | 手动触发 |
| `realtime` | 实时监控文件变更 |
| `interval` | 固定间隔（如每 6 小时） |
| `cron` | Cron 表达式（如每天凌晨 3 点） |

### 冲突解决策略

| 策略 | 说明 |
|------|------|
| `skip` | 跳过冲突文件 |
| `local` | 本地优先 |
| `remote` | 云端优先 |
| `newer` | 较新文件优先 |
| `rename` | 重命名冲突文件 |
| `ask` | 询问用户（需要交互） |

### 完整配置示例

```json
{
  "name": "重要文档同步",
  "providerId": "provider-001",
  "localPath": "/data/documents",
  "remotePath": "/backup/documents",
  "direction": "bidirect",
  "mode": "sync",
  
  "scheduleType": "cron",
  "scheduleExpr": "0 */6 * * *",
  
  "includePatterns": ["*.docx", "*.xlsx", "*.pdf"],
  "excludePatterns": ["*.tmp", "~$*"],
  "maxFileSize": 104857600,
  
  "conflictStrategy": "newer",
  "deleteRemote": false,
  "deleteLocal": false,
  "preserveModTime": true,
  "checksumVerify": true,
  
  "encrypt": true,
  "encryptKey": "your-encryption-key",
  
  "bandwidthLimit": 1024
}
```

---

## API 参考

### 云存储提供商管理

#### 添加云存储

```
POST /api/v1/cloudsync/providers
```

#### 列出云存储

```
GET /api/v1/cloudsync/providers
```

#### 删除云存储

```
DELETE /api/v1/cloudsync/providers/:id
```

### 同步任务管理

#### 创建同步任务

```
POST /api/v1/cloudsync/tasks
```

#### 列出同步任务

```
GET /api/v1/cloudsync/tasks
```

#### 执行同步

```
POST /api/v1/cloudsync/tasks/:id/run
```

#### 获取同步状态

```
GET /api/v1/cloudsync/tasks/:id/status
```

**响应示例**:
```json
{
  "code": 0,
  "data": {
    "taskId": "task-001",
    "status": "running",
    "startTime": "2026-03-20T10:00:00Z",
    "totalFiles": 1000,
    "processedFiles": 450,
    "totalBytes": 1073741824,
    "transferredBytes": 483183820,
    "speed": 2048,
    "progress": 45.0,
    "currentFile": "/data/photos/vacation/IMG_001.jpg",
    "currentAction": "upload",
    "uploadedFiles": 400,
    "downloadedFiles": 50,
    "skippedFiles": 0,
    "failedFiles": 0
  }
}
```

#### 暂停同步

```
POST /api/v1/cloudsync/tasks/:id/pause
```

#### 恢复同步

```
POST /api/v1/cloudsync/tasks/:id/resume
```

#### 取消同步

```
POST /api/v1/cloudsync/tasks/:id/cancel
```

### 统计信息

```
GET /api/v1/cloudsync/stats
```

---

## 最佳实践

### 1. 选择合适的同步模式

- **重要数据备份**: 使用 `upload` + `backup` 模式
- **多设备同步**: 使用 `bidirect` + `sync` 模式
- **节省带宽**: 使用 `increment` 模式

### 2. 设置合理的冲突策略

```json
{
  "conflictStrategy": "newer",  // 优先使用较新版本
  "preserveModTime": true       // 保留修改时间用于判断
}
```

### 3. 限制大文件同步

```json
{
  "maxFileSize": 104857600,  // 限制 100MB 以内
  "excludePatterns": ["*.iso", "*.vmdk"]
}
```

### 4. 带宽控制

```json
{
  "bandwidthLimit": 2048  // 限制 2MB/s
}
```

### 5. 加密敏感数据

```json
{
  "encrypt": true,
  "encryptKey": "your-strong-encryption-key"
}
```

### 6. 定期检查同步状态

```bash
# 查看所有任务状态
curl "/api/v1/cloudsync/tasks" -H "Authorization: Bearer TOKEN"
```

---

## 常见问题

### Q: 如何处理同步冲突？

A: 系统提供多种自动解决策略。推荐使用 `newer` 策略，自动选择较新的文件版本。对于重要数据，可以使用 `ask` 策略手动处理冲突。

### Q: 同步过程中断怎么办？

A: 系统支持断点续传。重新执行同步任务时，会自动从上次中断的位置继续。

### Q: 如何限制同步速度？

A: 通过 `bandwidthLimit` 参数设置传输速度上限（单位：KB/s）。

### Q: 支持选择性同步吗？

A: 支持。使用 `includePatterns` 和 `excludePatterns` 参数过滤文件：

```json
{
  "includePatterns": ["*.jpg", "*.png", "*.mp4"],
  "excludePatterns": ["*.tmp", "thumbnails/*"]
}
```

### Q: 如何查看同步历史？

A: 通过同步状态 API 查看历史记录：

```bash
curl "/api/v1/cloudsync/tasks/task-001/history" \
  -H "Authorization: Bearer TOKEN"
```

### Q: 云存储凭证如何安全存储？

A: 凭证使用 AES-256 加密存储在本地数据库中。建议定期轮换 Access Key。

---

## 相关文档

- [文件版本控制指南](VERSIONING.md)
- [API 使用指南](API_GUIDE.md)
- [用户手册](USER_MANUAL_v1.0.md)

---

**最后更新**: 2026-03-20  
**维护团队**: NAS-OS 礼部