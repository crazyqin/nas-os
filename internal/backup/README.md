# 备份增强功能使用指南

## 功能概述

本次增强为 OpenClaw NAS 备份系统添加了以下核心功能：

1. **增量备份优化** - 使用 rsync + 硬链接，类似 Time Machine
2. **云端备份支持** - S3/WebDAV 后端集成
3. **备份加密** - OpenSSL AES-256 加密
4. **一键恢复** - 简化的恢复流程

---

## 1. 增量备份

### 原理
- 使用 `rsync --link-dest` 实现硬链接增量
- 每次备份看起来都是完整的
- 未变化的文件通过硬链接指向上一版本
- 节省 70-90% 存储空间

### 使用示例

```bash
# 创建增量备份
cd /home/mrafter/clawd/nas-os
go run cmd/backup/main.go incremental create --source /data --name mydata

# 列出所有备份点
go run cmd/backup/main.go incremental list --name mydata

# 查看空间使用
go run cmd/backup/main.go incremental space --name mydata
```

### Go 代码示例

```go
import "nas-os/internal/backup"

// 创建增量备份管理器
ib := backup.NewIncrementalBackup("/srv/backups")

// 执行备份
result, err := ib.CreateBackup("/data/source", "mydata")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("备份完成：%s\n", result.BackupPath)
fmt.Printf("增量备份：%v\n", result.IsIncremental)
fmt.Printf("文件数：%d\n", result.TotalFiles)
fmt.Printf("节省空间：%.1f%%\n", result.IncrementalRatio * 100)
```

---

## 2. 云端备份

### 支持的提供商

| 提供商 | 类型 | 说明 |
|--------|------|------|
| AWS S3 | S3 | 亚马逊云存储 |
| 阿里云 OSS | S3 兼容 | 国内访问快 |
| MinIO | S3 兼容 | 自建对象存储 |
| WebDAV | WebDAV | Nextcloud/ownCloud 等 |

### 配置示例

```json
{
  "provider": "s3",
  "bucket": "my-backups",
  "region": "cn-north-1",
  "endpoint": "https://oss-cn-beijing.aliyuncs.com",
  "accessKey": "YOUR_ACCESS_KEY",
  "secretKey": "YOUR_SECRET_KEY",
  "prefix": "nas-backups/",
  "encryption": true
}
```

### 使用示例

```bash
# 上传备份到云端
go run cmd/backup/main.go cloud upload \
  --config cloud-config.json \
  --file /srv/backups/mydata/20260311_120000.tar.gz \
  --remote backups/mydata-20260311.tar.gz

# 从云端下载
go run cmd/backup/main.go cloud download \
  --config cloud-config.json \
  --remote backups/mydata-latest.tar.gz \
  --file /tmp/restore.tar.gz

# 列出云端备份
go run cmd/backup/main.go cloud list --config cloud-config.json
```

### Go 代码示例

```go
cfg := backup.CloudConfig{
    Provider:  backup.CloudProviderS3,
    Bucket:    "my-backups",
    Region:    "cn-north-1",
    Endpoint:  "https://oss-cn-beijing.aliyuncs.com",
    AccessKey: os.Getenv("AWS_ACCESS_KEY"),
    SecretKey: os.Getenv("AWS_SECRET_KEY"),
}

cb, err := backup.NewCloudBackup(cfg)
if err != nil {
    log.Fatal(err)
}

// 上传
result, err := cb.UploadBackup(
    "/srv/backups/latest.tar.gz",
    "backups/latest.tar.gz",
)

// 下载
result, err := cb.DownloadBackup(
    "backups/latest.tar.gz",
    "/tmp/restored.tar.gz",
)
```

---

## 3. 备份加密

### 加密方式

1. **OpenSSL AES-256-CBC** (推荐)
   - 系统兼容性好
   - 使用 PBKDF2 密钥派生
   - 100,000 次迭代

2. **AES-GCM** (Go 原生)
   - 无需外部依赖
   - 认证加密
   - 适合流式处理

3. **GPG** (可选)
   - 公钥加密
   - 适合多用户场景

### 使用示例

```bash
# 加密备份文件
go run cmd/backup/main.go encrypt \
  --input /srv/backups/latest.tar.gz \
  --output /srv/backups/latest.tar.gz.enc \
  --password "your-secret-password"

# 解密备份文件
go run cmd/backup/main.go decrypt \
  --input /srv/backups/latest.tar.gz.enc \
  --output /tmp/restored.tar.gz \
  --password "your-secret-password"

# 验证完整性
go run cmd/backup/main.go verify \
  --file /srv/backups/latest.tar.gz
```

### Go 代码示例

```go
// 创建加密器
encryptor, err := backup.NewEncryptor("my-secret-password")
if err != nil {
    log.Fatal(err)
}

// 加密文件
err = encryptor.EncryptFile(
    "/srv/backups/data.tar.gz",
    "/srv/backups/data.tar.gz.enc",
)

// 解密文件
err = encryptor.DecryptFile(
    "/srv/backups/data.tar.gz.enc",
    "/tmp/restored.tar.gz",
)

// 验证校验和
checksum, err := backup.VerifyIntegrity("/srv/backups/data.tar.gz")
backup.WriteChecksum("/srv/backups/data.tar.gz")
valid, err := backup.VerifyChecksum("/srv/backups/data.tar.gz")
```

---

## 4. 一键恢复

### 恢复模式

1. **完整恢复** - 恢复整个备份
2. **单文件恢复** - 恢复指定文件
3. **预览模式** - 查看将恢复什么
4. **快速恢复** - 恢复到最新备份点

### 使用示例

```bash
# 列出所有可用备份
go run cmd/backup/main.go restore list

# 预览恢复（不实际执行）
go run cmd/backup/main.go restore preview \
  --backup mydata/20260311_120000 \
  --target /tmp/restore

# 完整恢复
go run cmd/backup/main.go restore full \
  --backup mydata/20260311_120000 \
  --target /data/restored \
  --overwrite

# 恢复单个文件
go run cmd/backup/main.go restore file \
  --backup mydata/20260311_120000 \
  --file documents/report.pdf \
  --target /tmp/recovered

# 快速恢复到最新
go run cmd/backup/main.go restore quick \
  --backup mydata \
  --target /data/restored
```

### Go 代码示例

```go
rm := backup.NewRestoreManager("/srv/backups", "/srv/storage")

// 完整恢复
result, err := rm.Restore(backup.RestoreOptions{
    BackupID:    "mydata/20260311_120000",
    TargetPath:  "/data/restored",
    Overwrite:   true,
    VerifyAfter: true,
})

// 快速恢复
result, err := rm.QuickRestore("mydata", "/data/restored")

// 恢复单个文件
result, err := rm.RestoreSingleFile(
    "mydata/20260311_120000",
    "documents/report.pdf",
    "/tmp/recovered",
)

// 预览模式
result, err := rm.Restore(backup.RestoreOptions{
    BackupID:   "mydata/20260311_120000",
    TargetPath: "/tmp/restore",
    DryRun:     true,
})

fmt.Printf("将恢复 %d 个文件，总大小 %s\n", 
    result.TotalFiles, 
    formatSize(result.TotalSize))
```

---

## 5. 健康检查

```go
manager := backup.NewManager("/etc/backup/config.json", "/srv/backups")
manager.Initialize()

// 执行健康检查
result := manager.HealthCheck()

fmt.Printf("状态：%s\n", result.Status)
for _, check := range result.Checks {
    fmt.Printf("  [%s] %s: %s\n", check.Status, check.Name, check.Message)
}

if len(result.Recommendations) > 0 {
    fmt.Println("建议:")
    for _, rec := range result.Recommendations {
        fmt.Printf("  - %s\n", rec)
    }
}
```

---

## 6. 完整工作流示例

### 日常备份脚本

```bash
#!/bin/bash
# daily-backup.sh

set -e

BACKUP_NAME="daily"
SOURCE_DIR="/data"
BACKUP_DIR="/srv/backups"
CLOUD_CONFIG="/etc/backup/cloud.json"
PASSWORD_FILE="/etc/backup/.password"

echo "[$(date)] 开始备份..."

# 1. 创建增量备份
backup incremental create \
  --source "$SOURCE_DIR" \
  --name "$BACKUP_NAME" \
  --dest "$BACKUP_DIR"

# 2. 加密最新备份
LATEST=$(readlink "$BACKUP_DIR/$BACKUP_NAME/latest")
backup encrypt \
  --input "$BACKUP_DIR/$BACKUP_NAME/$LATEST.tar.gz" \
  --output "$BACKUP_DIR/$BACKUP_NAME/$LATEST.tar.gz.enc" \
  --password-file "$PASSWORD_FILE"

# 3. 上传到云端
backup cloud upload \
  --config "$CLOUD_CONFIG" \
  --file "$BACKUP_DIR/$BACKUP_NAME/$LATEST.tar.gz.enc" \
  --remote "backups/$BACKUP_NAME/$LATEST.tar.gz.enc"

# 4. 验证云端备份
backup cloud verify \
  --config "$CLOUD_CONFIG" \
  --remote "backups/$BACKUP_NAME/$LATEST.tar.gz.enc"

# 5. 清理本地旧备份（保留 7 天）
backup cleanup --name "$BACKUP_NAME" --keep-days 7

echo "[$(date)] 备份完成"
```

### 恢复脚本

```bash
#!/bin/bash
# restore.sh

set -e

BACKUP_NAME="daily"
TARGET_DIR="/data/restored"
CLOUD_CONFIG="/etc/backup/cloud.json"
PASSWORD_FILE="/etc/backup/.password"

echo "[$(date)] 开始恢复..."

# 1. 从云端下载最新备份
LATEST_REMOTE=$(backup cloud list --config "$CLOUD_CONFIG" --latest)
backup cloud download \
  --config "$CLOUD_CONFIG" \
  --remote "$LATEST_REMOTE" \
  --file "/tmp/backup.enc"

# 2. 解密
backup decrypt \
  --input "/tmp/backup.enc" \
  --output "/tmp/backup.tar.gz" \
  --password-file "$PASSWORD_FILE"

# 3. 恢复
backup restore full \
  --file "/tmp/backup.tar.gz" \
  --target "$TARGET_DIR" \
  --overwrite \
  --verify

echo "[$(date)] 恢复完成到 $TARGET_DIR"
```

---

## 7. API 端点

备份功能通过 REST API 暴露：

```
POST   /api/backup/configs          # 创建备份配置
GET    /api/backup/configs          # 列出配置
PUT    /api/backup/configs/:id      # 更新配置
DELETE /api/backup/configs/:id      # 删除配置

POST   /api/backup/run/:id          # 执行备份
POST   /api/backup/restore          # 恢复备份

GET    /api/backup/tasks            # 列出任务
GET    /api/backup/tasks/:id        # 获取任务状态
DELETE /api/backup/tasks/:id        # 取消任务

GET    /api/backup/history/:configId # 备份历史

# 新增端点
GET    /api/backup/incremental/list  # 列出增量备份
POST   /api/backup/cloud/upload      # 上传云端
POST   /api/backup/cloud/download    # 下载云端
POST   /api/backup/encrypt           # 加密
POST   /api/backup/decrypt           # 解密
GET    /api/backup/health            # 健康检查
```

---

## 8. 最佳实践

### 备份策略

1. **3-2-1 规则**
   - 至少 3 份数据副本
   - 使用 2 种不同介质
   - 1 份异地备份

2. **增量 + 定期完整**
   - 每日增量备份
   - 每周完整备份
   - 每月归档到云端

3. **加密所有云端备份**
   - 使用强密码（16+ 字符）
   - 密码存储在安全位置
   - 定期更换密码

4. **定期验证**
   - 每月测试恢复
   - 验证校验和
   - 检查备份完整性

### 监控告警

```go
// 在定时任务中检查
result := manager.HealthCheck()
if result.Status != "healthy" {
    sendAlert("备份系统异常", result)
}
```

---

## 9. 故障排查

### 常见问题

1. **增量备份失败**
   ```bash
   # 检查 rsync 是否安装
   which rsync
   
   # 检查源目录权限
   ls -la /data/source
   ```

2. **云端上传失败**
   ```bash
   # 检查网络连接
   curl -I https://oss-cn-beijing.aliyuncs.com
   
   # 验证凭证
   backup cloud test --config cloud.json
   ```

3. **解密失败**
   ```bash
   # 确认密码正确
   backup decrypt --input file.enc --output file.out --password "test"
   
   # 检查文件完整性
   backup verify --file file.enc
   ```

---

## 10. 性能优化

1. **并行上传**
   - 大文件分片上传
   - 多线程压缩

2. **带宽限制**
   ```bash
   backup run --bandwidth-limit 10MB
   ```

3. **排除模式**
   ```json
   {
     "excludePatterns": [
       "*.tmp",
       "*.log",
       ".git/",
       "node_modules/"
     ]
   }
   ```

---

## 总结

备份增强功能提供了企业级的数据保护能力：

- ✅ 增量备份节省 70-90% 空间
- ✅ 云端备份实现异地容灾
- ✅ AES-256 加密保护隐私
- ✅ 一键恢复简化运维

建议配置定时任务每日自动备份，并定期测试恢复流程。
