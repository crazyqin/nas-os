# NAS-OS 备份与同步功能

## 功能概述

本模块实现了 NAS-OS 的完整备份与同步功能，参考竞品（飞牛 NAS/群晖 Active Backup/Synology Drive），提供企业级数据保护能力。

## 核心功能

### 1. 远程备份 ✅

支持备份到远程 NAS 和云存储：

- **S3 兼容存储**
  - 阿里云 OSS
  - 腾讯云 COS
  - MinIO
  - AWS S3

- **WebDAV 存储**
  - 支持标准 WebDAV 协议
  - 支持跳过 TLS 验证（自签名证书）

### 2. 双向同步 ✅

实现类似 rsync 的双向同步逻辑：

- **同步模式**
  - `bidirectional` - 双向同步（两边互相同步）
  - `master-slave` - 主从同步（源为主，单向）
  - `one-way` - 单向同步（不删除目标）

- **冲突解决策略**
  - `latest` - 最新者胜（默认）
  - `source` - 源优先
  - `dest` - 目标优先
  - `keep-both` - 保留两者（重命名冲突文件）
  - `manual` - 手动解决

### 3. 版本控制 ✅

文件历史版本管理：

- 自动创建版本快照（修改前备份）
- 保留最近 N 个版本（可配置）
- 版本恢复功能
- 版本元数据（大小、checksum、创建时间）

### 4. 增量备份 ✅

基于 rsync --link-dest 实现硬链接增量备份：

- 首次完整备份
- 后续仅备份变更文件
- 使用硬链接节省空间
- 每个备份看起来都是完整的

### 5. 备份计划

支持定时备份和实时同步：

- Cron 表达式配置
- 实时同步（文件变更检测）
- 备份进度显示
- 任务状态追踪

## API 接口

### 备份配置管理

```bash
# 列出所有备份配置
GET /api/v1/backup/configs

# 创建备份配置
POST /api/v1/backup/configs
{
  "name": "文档备份",
  "source": "/mnt/data/documents",
  "destination": "/mnt/backups/documents",
  "type": "local",
  "retention": 7,
  "compression": true,
  "encrypt": false,
  "exclude": ["*.tmp", "*.log"]
}

# 更新备份配置
PUT /api/v1/backup/configs/:id

# 删除备份配置
DELETE /api/v1/backup/configs/:id

# 启用/禁用备份
POST /api/v1/backup/configs/:id/enable
{
  "enabled": true
}
```

### 备份执行

```bash
# 立即执行备份
POST /api/v1/backup/run/:id

# 恢复备份
POST /api/v1/backup/restore
{
  "backupId": "backup_xxx",
  "targetPath": "/mnt/restore",
  "overwrite": true
}
```

### 同步任务管理

```bash
# 列出同步任务
GET /api/v1/backup/sync/tasks

# 创建同步任务
POST /api/v1/backup/sync/tasks
{
  "name": "文档双向同步",
  "source": "/mnt/data/documents",
  "destination": "/mnt/sync/documents",
  "mode": "bidirectional",
  "conflict": "latest",
  "remoteType": "local"
}

# 创建 S3 同步任务
POST /api/v1/backup/sync/tasks
{
  "name": "备份到阿里云",
  "source": "/mnt/data/important",
  "mode": "one-way",
  "remoteType": "s3",
  "remoteConfig": {
    "bucket": "my-backup",
    "region": "cn-hangzhou",
    "endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
    "accessKey": "xxx",
    "secretKey": "xxx"
  }
}

# 创建 WebDAV 同步任务
POST /api/v1/backup/sync/tasks
{
  "name": "备份到 WebDAV",
  "source": "/mnt/data/photos",
  "mode": "one-way",
  "remoteType": "webdav",
  "remoteConfig": {
    "url": "https://dav.example.com",
    "username": "admin",
    "password": "password"
  }
}

# 立即执行同步
POST /api/v1/backup/sync/run/:id

# 删除同步任务
DELETE /api/v1/backup/sync/tasks/:id
```

### 版本管理

```bash
# 列出文件版本
GET /api/v1/backup/sync/versions?path=/path/to/file

# 恢复版本
POST /api/v1/backup/sync/versions/restore
{
  "versionId": "v1234567890_file.txt",
  "targetPath": "/mnt/restore/file.txt"
}
```

### 任务管理

```bash
# 列出所有任务
GET /api/v1/backup/tasks

# 获取任务状态
GET /api/v1/backup/tasks/:id

# 取消任务
DELETE /api/v1/backup/tasks/:id
```

### 历史记录

```bash
# 获取备份历史
GET /api/v1/backup/history/:configId
```

## Web UI

访问：`http://localhost:8080/backup.html`

### 功能页面

1. **备份任务** - 创建、管理备份任务
2. **同步任务** - 配置双向/单向同步
3. **版本历史** - 查看和恢复文件版本
4. **远程目标** - 管理 S3/WebDAV 存储目标

### 界面特性

- 📊 实时统计卡片（总备份数、大小、成功/失败任务）
- 🔄 进度条显示备份进度
- 🏷️ 状态标签（运行中、已完成、失败、等待中）
- 📱 响应式设计，支持移动端

## 技术实现

### 文件结构

```
internal/backup/
├── manager.go      # 备份管理器（核心）
├── sync.go         # 同步管理器（新增）
├── config.go       # 配置管理
├── handlers.go     # API 处理器
├── incremental.go  # 增量备份
├── cloud.go        # 云端备份（S3/WebDAV）
├── restore.go      # 恢复功能
└── encrypt.go      # 加密功能
```

### 依赖库

```go
github.com/aws/aws-sdk-go-v2/service/s3  // S3 兼容存储
github.com/studio-b12/gowebdav           // WebDAV 客户端
```

### 增量备份原理

使用 rsync 的 `--link-dest` 参数实现：

```bash
rsync -av --delete --hard-links --numeric-ids \
  --link-dest=/path/to/previous/backup \
  /source/ /path/to/current/backup
```

- 未变更的文件创建硬链接到上一个备份
- 变更的文件实际复制
- 每个备份目录看起来都是完整的
- 实际磁盘占用仅为增量部分

### 双向同步流程

1. **扫描** - 递归扫描源和目标目录
2. **比较** - 计算文件 checksum 比较差异
3. **冲突检测** - 识别两边都修改的文件
4. **应用策略** - 根据配置的冲突解决策略处理
5. **执行同步** - 复制文件，创建版本备份
6. **更新状态** - 记录同步结果和统计

## 使用示例

### 示例 1：本地增量备份

```bash
# 创建备份配置
curl -X POST http://localhost:8080/api/v1/backup/configs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "重要文档",
    "source": "/mnt/data/documents",
    "type": "local",
    "retention": 7,
    "compression": true
  }'

# 执行备份
curl -X POST http://localhost:8080/api/v1/backup/run/backup_xxx
```

### 示例 2：备份到阿里云 OSS

```bash
curl -X POST http://localhost:8080/api/v1/backup/sync/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "阿里云备份",
    "source": "/mnt/data/critical",
    "mode": "one-way",
    "remoteType": "s3",
    "remoteConfig": {
      "bucket": "my-nas-backup",
      "region": "cn-hangzhou",
      "endpoint": "https://oss-cn-hangzhou.aliyuncs.com",
      "accessKey": "LTAI5t...",
      "secretKey": "xxx"
    }
  }'
```

### 示例 3：双向同步两个目录

```bash
curl -X POST http://localhost:8080/api/v1/backup/sync/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "办公文档同步",
    "source": "/mnt/data/office",
    "destination": "/mnt/sync/office",
    "mode": "bidirectional",
    "conflict": "latest"
  }'
```

### 示例 4：恢复文件版本

```bash
# 查看版本
curl "http://localhost:8080/api/v1/backup/sync/versions?path=/documents/report.docx"

# 恢复指定版本
curl -X POST http://localhost:8080/api/v1/backup/sync/versions/restore \
  -H "Content-Type: application/json" \
  -d '{
    "versionId": "v1234567890_report.docx",
    "targetPath": "/mnt/restore/report.docx"
  }'
```

## 安全考虑

1. **路径遍历防护** - 验证所有路径在允许范围内
2. **符号链接处理** - 检测并阻止恶意符号链接
3. **凭证加密** - 远程密码应加密存储
4. **访问控制** - API 需要 JWT 认证
5. **审计日志** - 记录所有备份/恢复操作

## 性能优化

1. **并发传输** - 大文件分块并行上传
2. **断点续传** - 支持中断后继续
3. **带宽限制** - 可配置最大带宽
4. **压缩传输** - 减少网络流量
5. **增量同步** - 仅传输变更部分

## 后续优化

- [ ] 备份去重（重复数据删除）
- [ ] 备份验证（定期校验完整性）
- [ ] 备份报告（邮件/通知）
- [ ] 备份策略模板
- [ ] 跨 NAS 集群同步
- [ ] 备份加密（AES-256-GCM）
- [ ] 备份压缩级别可调
- [ ] 文件级恢复（细粒度）

## 故障排查

### 常见问题

1. **rsync 失败**
   ```bash
   # 检查 rsync 是否安装
   which rsync
   
   # 测试手动 rsync
   rsync -av /source/ /dest/
   ```

2. **S3 上传失败**
   - 检查 Access Key / Secret Key
   - 验证 Endpoint 是否正确
   - 确认 Bucket 存在且有写权限

3. **WebDAV 连接失败**
   - 检查 URL 是否正确
   - 验证用户名密码
   - 如使用自签名证书，启用 Insecure 选项

## 总结

本模块实现了完整的备份与同步功能，涵盖：

✅ 本地/远程备份  
✅ S3/WebDAV 云存储支持  
✅ 双向/单向同步  
✅ 增量备份（节省空间）  
✅ 版本控制（可恢复）  
✅ Web UI 管理界面  
✅ RESTful API  

满足个人和企业用户的数据保护需求。
