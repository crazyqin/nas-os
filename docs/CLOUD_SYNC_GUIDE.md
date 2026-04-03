# Cloud Sync 配置指南

**版本**: v2.378.0 | **更新日期**: 2026-04-03 | **作者**: 礼部

---

## 📋 目录

1. [功能概述](#功能概述)
2. [支持的云存储](#支持的云存储)
3. [Google Drive配置](#google-drive配置)
4. [OneDrive配置](#onedrive配置)
5. [Dropbox配置](#dropbox配置)
6. [其他云存储配置](#其他云存储配置)
7. [同步任务配置](#同步任务配置)
8. [最佳实践](#最佳实践)
9. [常见问题FAQ](#常见问题faq)

---

## 功能概述

### 什么是Cloud Sync？

Cloud Sync模块提供本地NAS与云存储之间的双向同步能力，实现数据云端备份和多设备协作。

### 核心价值

| 价值 | 说明 |
|------|------|
| **多云支持** | 阿里云OSS、腾讯云COS、AWS S3、Google Drive、OneDrive、Dropbox等 |
| **双向同步** | 本地↔云端实时同步 |
| **增量传输** | 仅传输变更部分，节省带宽 |
| **加密保护** | 支持端到端加密传输 |
| **冲突处理** | 多种自动冲突解决策略 |
| **灵活调度** | 实时/定时/Cron多种模式 |

---

## 支持的云存储

### 云存储矩阵

| 提供商 | 类型标识 | 特性 | 推荐场景 |
|--------|----------|------|----------|
| **Google Drive** | `google_drive` | 文件共享、协作办公 | 个人/团队协作 |
| **OneDrive** | `onedrive` | Office集成、企业办公 | 企业用户 |
| **Dropbox** | `dropbox` | 跨平台同步、版本历史 | 跨设备协作 |
| **阿里云OSS** | `aliyun_oss` | 分片上传、低成本 | 国内大文件备份 |
| **腾讯云COS** | `tencent_cos` | 分片上传、CDN加速 | 国内加速备份 |
| **AWS S3** | `aws_s3` | 版本控制、全球部署 | 国际业务备份 |
| **Backblaze B2** | `backblaze_b2` | 低成本、S3兼容 | 经济型备份 |
| **WebDAV** | `webdav` | 通用协议、广泛兼容 | 自建云存储 |

---

## Google Drive配置

### 准备工作

#### 1. 创建Google Cloud项目

```
1. 访问 https://console.cloud.google.com/
2. 创建新项目或选择现有项目
3. 启用 Google Drive API
4. 创建 OAuth 2.0 客户端凭证
   - 应用类型：Web应用
   - 授权重定向URI：http://nas-ip:8080/api/v1/cloudsync/callback
5. 记录 Client ID 和 Client Secret
```

#### 2. 获取Refresh Token

```bash
# 步骤1：构造授权URL
https://accounts.google.com/o/oauth2/v2/auth?
  client_id=YOUR_CLIENT_ID&
  redirect_uri=http://nas-ip:8080/api/v1/cloudsync/callback&
  response_type=code&
  scope=https://www.googleapis.com/auth/drive&
  access_type=offline

# 步骤2：用户授权后获取code
# 步骤3：交换code获取refresh_token

curl -X POST "https://oauth2.googleapis.com/token" \
  -d "code=AUTHORIZATION_CODE" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "redirect_uri=http://nas-ip:8080/api/v1/cloudsync/callback" \
  -d "grant_type=authorization_code"

# 响应包含refresh_token
{
  "access_token": "...",
  "refresh_token": "1//...",  ← 保存此值
  "expires_in": 3600
}
```

### 配置步骤

#### WebUI配置

```
1. 进入「云同步」模块
2. 点击「添加云存储」
3. 选择「Google Drive」
4. 输入凭证信息：
   - Client ID: [你的Client ID]
   - Client Secret: [你的Client Secret]
   - Refresh Token: [获取的refresh_token]
5. 点击「连接测试」验证
6. 保存配置
```

#### API配置

```bash
# 添加Google Drive提供商
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的Google Drive",
    "type": "google_drive",
    "clientId": "YOUR_CLIENT_ID",
    "clientSecret": "YOUR_CLIENT_SECRET",
    "refreshToken": "YOUR_REFRESH_TOKEN",
    "rootFolderId": "root"
  }'

# 响应
{
  "id": "provider-gdrive-001",
  "name": "我的Google Drive",
  "type": "google_drive",
  "status": "connected",
  "createdAt": "2026-04-03T10:00:00Z"
}
```

### 配置参数说明

| 参数 | 说明 | 必填 |
|------|------|------|
| `clientId` | OAuth 2.0客户端ID | ✅ |
| `clientSecret` | OAuth 2.0客户端密钥 | ✅ |
| `refreshToken` | 长期刷新令牌 | ✅ |
| `rootFolderId` | 根目录ID（默认"root"） | ❌ |

---

## OneDrive配置

### 准备工作

#### 1. 创建Azure应用

```
1. 访问 https://portal.azure.com/
2. 进入「Azure Active Directory」→「应用注册」
3. 创建新应用：
   - 名称：NAS-OS Cloud Sync
   - 类型：Web应用
   - 重定向URI：http://nas-ip:8080/api/v1/cloudsync/callback
4. 记录 Application (client) ID
5. 创建客户端密钥，记录密钥值
6. 配置API权限：
   - Microsoft Graph → Files.ReadWrite.All
7. 授权管理员同意（企业账户）
```

#### 2. 获取Refresh Token

```bash
# 构造授权URL（个人账户）
https://login.microsoftonline.com/common/oauth2/v2.0/authorize?
  client_id=YOUR_CLIENT_ID&
  redirect_uri=http://nas-ip:8080/api/v1/cloudsync/callback&
  response_type=code&
  scope=files.readwrite offline_access

# 交换code获取token
curl -X POST "https://login.microsoftonline.com/common/oauth2/v2.0/token" \
  -d "code=AUTHORIZATION_CODE" \
  -d "client_id=YOUR_CLIENT_ID" \
  -d "client_secret=YOUR_CLIENT_SECRET" \
  -d "redirect_uri=http://nas-ip:8080/api/v1/cloudsync/callback" \
  -d "grant_type=authorization_code"
```

### 配置步骤

```bash
# 添加OneDrive提供商
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的OneDrive",
    "type": "onedrive",
    "clientId": "YOUR_CLIENT_ID",
    "clientSecret": "YOUR_CLIENT_SECRET",
    "tenantId": "common",
    "refreshToken": "YOUR_REFRESH_TOKEN"
  }'

# 企业账户使用实际tenantId替换"common"
```

### 配置参数说明

| 参数 | 说明 | 必填 |
|------|------|------|
| `clientId` | Azure应用ID | ✅ |
| `clientSecret` | Azure客户端密钥 | ✅ |
| `tenantId` | 租户ID（个人用"common"） | ✅ |
| `refreshToken` | 长期刷新令牌 | ✅ |

---

## Dropbox配置

### 准备工作

#### 1. 创建Dropbox应用

```
1. 访问 https://www.dropbox.com/developers/apps
2. 创建新应用：
   - API：Dropbox API
   - 访问类型：App folder（推荐）或 Full Dropbox
   - 名称：NAS-OS-Cloud-Sync
3. 记录 App key 和 App secret
4. 设置OAuth2重定向URI：
   http://nas-ip:8080/api/v1/cloudsync/callback
```

#### 2. 获取Refresh Token

```bash
# 构造授权URL
https://www.dropbox.com/oauth2/authorize?
  client_id=YOUR_APP_KEY&
  response_type=code&
  redirect_uri=http://nas-ip:8080/api/v1/cloudsync/callback

# 交换code获取token
curl -X POST "https://api.dropboxapi.com/oauth2/token" \
  -d "code=AUTHORIZATION_CODE" \
  -d "client_id=YOUR_APP_KEY" \
  -d "client_secret=YOUR_APP_SECRET" \
  -d "redirect_uri=http://nas-ip:8080/api/v1/cloudsync/callback" \
  -d "grant_type=authorization_code"
```

### 配置步骤

```bash
# 添加Dropbox提供商
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的Dropbox",
    "type": "dropbox",
    "clientId": "YOUR_APP_KEY",
    "clientSecret": "YOUR_APP_SECRET",
    "refreshToken": "YOUR_REFRESH_TOKEN"
  }'
```

---

## 其他云存储配置

### 阿里云OSS

```bash
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "阿里云OSS",
    "type": "aliyun_oss",
    "endpoint": "oss-cn-hangzhou.aliyuncs.com",
    "region": "cn-hangzhou",
    "bucket": "your-bucket",
    "accessKey": "YOUR_ACCESS_KEY_ID",
    "secretKey": "YOUR_ACCESS_KEY_SECRET",
    "maxConnections": 10,
    "timeout": 300
  }'
```

| 参数 | 说明 |
|------|------|
| `endpoint` | OSS地域节点地址 |
| `region` | 地域ID |
| `bucket` | 存储桶名称 |
| `accessKey` | AccessKey ID |
| `secretKey` | AccessKey Secret |

### AWS S3

```bash
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "AWS S3",
    "type": "aws_s3",
    "region": "us-east-1",
    "bucket": "your-bucket",
    "accessKey": "YOUR_ACCESS_KEY_ID",
    "secretKey": "YOUR_SECRET_ACCESS_KEY"
  }'
```

### Backblaze B2

```bash
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Backblaze B2",
    "type": "backblaze_b2",
    "endpoint": "https://s3.us-west-002.backblazeb2.com",
    "bucket": "your-bucket",
    "accessKey": "YOUR_KEY_ID",
    "secretKey": "YOUR_APPLICATION_KEY"
  }'
```

### WebDAV

```bash
curl -X POST "http://localhost:8080/api/v1/cloudsync/providers" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "自建WebDAV",
    "type": "webdav",
    "endpoint": "https://webdav.example.com",
    "accessKey": "username",
    "secretKey": "password",
    "insecure": false
  }'
```

---

## 同步任务配置

### 创建同步任务

```bash
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks" \
  -H "Authorization: Bearer TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "照片备份到Google Drive",
    "providerId": "provider-gdrive-001",
    "localPath": "/data/photos",
    "remotePath": "/backup/photos",
    "direction": "upload",
    "mode": "backup",
    "scheduleType": "realtime",
    "conflictStrategy": "newer",
    "includePatterns": ["*.jpg", "*.png", "*.heic"],
    "excludePatterns": ["*.tmp", "cache/*"],
    "maxFileSize": 104857600,
    "bandwidthLimit": 2048,
    "encrypt": false
  }'
```

### 同步方向

| 方向 | 说明 | 适用场景 |
|------|------|----------|
| `upload` | 本地 → 云端 | 数据备份 |
| `download` | 云端 → 本地 | 数据恢复 |
| `bidirect` | 双向同步 | 多设备协作 |

### 同步模式

| 模式 | 说明 |
|------|------|
| `mirror` | 镜像模式：本地为主，云端完全同步 |
| `backup` | 备份模式：保留历史版本 |
| `sync` | 同步模式：双向同步，保留双方变更 |
| `increment` | 增量模式：仅同步变更部分 |

### 调度类型

| 类型 | 说明 |
|------|------|
| `manual` | 手动触发 |
| `realtime` | 实时监控文件变更 |
| `interval` | 固定间隔（如每6小时） |
| `cron` | Cron表达式（如每天凌晨3点） |

### 冲突解决策略

| 策略 | 说明 |
|------|------|
| `skip` | 跳过冲突文件 |
| `local` | 本地优先 |
| `remote` | 云端优先 |
| `newer` | 较新文件优先 |
| `rename` | 重命名冲突文件 |
| `ask` | 询问用户（需交互） |

### 完整配置示例

```json
{
  "name": "重要文档同步",
  "providerId": "provider-gdrive-001",
  "localPath": "/data/documents",
  "remotePath": "/backup/documents",
  "direction": "bidirect",
  "mode": "sync",
  
  "scheduleType": "cron",
  "scheduleExpr": "0 */6 * * *",  // 每6小时
  
  "includePatterns": ["*.docx", "*.xlsx", "*.pdf"],
  "excludePatterns": ["*.tmp", "~$*"],
  "maxFileSize": 104857600,  // 100MB
  
  "conflictStrategy": "newer",
  "deleteRemote": false,  // 不删除云端文件
  "deleteLocal": false,   // 不删除本地文件
  "preserveModTime": true,
  "checksumVerify": true,
  
  "encrypt": true,
  "encryptKey": "your-encryption-key",
  
  "bandwidthLimit": 1024  // 1MB/s
}
```

---

## 最佳实践

### 1. 选择合适的同步模式

| 场景 | 推荐配置 |
|------|----------|
| **重要数据备份** | `upload` + `backup` + `interval` |
| **多设备协作** | `bidirect` + `sync` + `realtime` |
| **节省带宽** | `increment` + `interval` |

### 2. 限制文件范围

```json
{
  "includePatterns": ["*.jpg", "*.png", "*.mp4"],
  "excludePatterns": ["*.tmp", "cache/*", "thumbnails/*"],
  "maxFileSize": 104857600  // 100MB限制
}
```

### 3. 带宽控制

```json
{
  "bandwidthLimit": 2048  // 限制2MB/s，避免影响其他业务
}
```

### 4. 加密敏感数据

```json
{
  "encrypt": true,
  "encryptKey": "your-strong-encryption-key-here"
}
```

### 5. 定期检查同步状态

```bash
# 查看所有任务状态
curl "http://localhost:8080/api/v1/cloudsync/tasks" \
  -H "Authorization: Bearer TOKEN"

# 查看单个任务状态
curl "http://localhost:8080/api/v1/cloudsync/tasks/task-001/status" \
  -H "Authorization: Bearer TOKEN"
```

### 6. 监控同步进度

```bash
# 响应示例
{
  "taskId": "task-001",
  "status": "running",
  "startTime": "2026-04-03T10:00:00Z",
  "totalFiles": 1000,
  "processedFiles": 450,
  "totalBytes": 1073741824,
  "transferredBytes": 483183820,
  "speed": 2048,  // KB/s
  "progress": 45.0,
  "currentFile": "/data/photos/IMG_001.jpg",
  "eta": "2h 15m"
}
```

---

## 常见问题FAQ

### Q1: 如何处理同步冲突？

**答**：系统提供多种自动策略：
- `newer`：优先使用较新版本（推荐）
- `rename`：重命名冲突文件，保留两个版本
- `ask`：手动处理（需交互）

### Q2: 同步中断后如何恢复？

**答**：支持断点续传：
```bash
# 重新执行任务自动继续
curl -X POST "http://localhost:8080/api/v1/cloudsync/tasks/task-001/run"
```

### Q3: 如何限制同步速度？

**答**：设置`bandwidthLimit`参数：
```json
{
  "bandwidthLimit": 1024  // 单位KB/s
}
```

### Q4: 支持选择性同步吗？

**答**：支持，使用过滤规则：
```json
{
  "includePatterns": ["*.jpg", "*.png"],
  "excludePatterns": ["*.tmp", "cache/*"]
}
```

### Q5: 云存储凭证安全吗？

**答**：凭证AES-256加密存储：
- 本地数据库加密保存
- 建议定期轮换密钥
- 启用提供商的密钥轮换功能

### Q6: 如何查看同步历史？

**答**：
```bash
curl "http://localhost:8080/api/v1/cloudsync/tasks/task-001/history"
```

### Q7: Google Drive空间不足怎么办？

**答**：
- 清理云端无用文件
- 使用`excludePatterns`排除大文件
- 升级Google Drive存储套餐
- 切换到其他提供商（如Backblaze B2）

---

## 竞品对比

### 功能对比矩阵

| 功能 | nas-os | 群晖Cloud Sync | 飞牛fnOS | TrueNAS |
|------|--------|----------------|----------|---------|
| **Google Drive** | ✅ | ✅ | ✅ | ❌ |
| **OneDrive** | ✅ | ✅ | ✅ | ❌ |
| **Dropbox** | ✅ | ✅ | ❌ | ❌ |
| **阿里云OSS** | ✅ | ✅ | ✅ | ❌ |
| **腾讯云COS** | ✅ | ❌ | ✅ | ❌ |
| **AWS S3** | ✅ | ✅ | ❌ | ✅ |
| **Backblaze B2** | ✅ | ✅ | ❌ | ✅ |
| **双向同步** | ✅ | ✅ | ✅ | ❌ |
| **实时监控** | ✅ | ✅ | ✅ | ❌ |
| **加密传输** | ✅ | ✅ | ❌ | ❌ |
| **断点续传** | ✅ | ✅ | ✅ | ✅ |
| **冲突处理** | ✅ 6种策略 | ✅ 3种 | ❌ | ❌ |

### nas-os差异化优势

| 优势 | 说明 |
|------|------|
| **多云覆盖** | 国内+国际全覆盖 |
| **冲突策略丰富** | 6种自动处理策略 |
| **加密传输** | 端到端加密支持 |
| **API开放** | 支持第三方集成 |
| **灵活调度** | realtime/interval/cron全支持 |

---

## 相关文档

| 文档 | 说明 |
|------|------|
| [CLOUDSYNC.md](./CLOUDSYNC.md) | 云同步技术文档 |
| [API_GUIDE.md](./API_GUIDE.md) | API完整文档 |
| [USER_GUIDE.md](./USER_GUIDE.md) | 用户指南 |

---

## 更新记录

| 版本 | 日期 | 更新内容 |
|------|------|----------|
| v1.0 | 2026-04-03 | 礼部创建完整配置指南 |

---

**文档维护**: 礼部（品牌营销） | **功能状态**: ✅ 已完成