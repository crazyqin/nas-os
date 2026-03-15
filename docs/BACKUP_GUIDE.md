# NAS-OS 备份管理指南

本文档介绍如何使用 NAS-OS 的备份管理功能，包括创建备份任务、配置备份策略、执行恢复操作等。

## 概述

NAS-OS 提供了强大的备份管理功能，支持：

- **多种备份目标**：本地存储、S3 对象存储、SFTP 远程服务器
- **灵活的调度**：支持 cron 表达式定时备份
- **数据压缩**：支持 gzip、zstd 等压缩算法
- **数据加密**：支持 AES-256 加密保护备份文件
- **增量备份**：只备份变化的数据，节省存储空间
- **版本管理**：保留多个历史版本，支持任意时间点恢复

## 快速开始

### 创建第一个备份任务

#### 1. 通过 Web 界面

1. 登录 NAS-OS 管理界面
2. 进入 **备份管理** > **备份任务**
3. 点击 **创建备份** 按钮
4. 填写备份配置：
   - **名称**：备份任务的显示名称
   - **源路径**：要备份的目录或文件
   - **目标类型**：选择本地/S3/SFTP
   - **目标路径**：备份存储位置
   - **调度**：设置备份频率（如 `0 2 * * *` 每天凌晨 2 点）

#### 2. 通过 API

```bash
# 创建备份配置
curl -X POST http://localhost:8080/api/v1/backup/configs \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "重要文档备份",
    "source_path": "/data/documents",
    "destination_type": "local",
    "destination_path": "/backup/documents",
    "schedule": "0 2 * * *",
    "retention_days": 30,
    "compression": "zstd",
    "encryption": true
  }'
```

### 手动执行备份

```bash
# 立即执行备份任务
curl -X POST http://localhost:8080/api/v1/backup/configs/{id}/run \
  -H "Authorization: Bearer {token}"
```

## 备份类型

### 完整备份

备份所有选定的文件和目录。首次备份通常是完整备份。

### 增量备份

只备份自上次备份以来变化的文件。增量备份速度快、占用空间小。

### 差异备份

备份自上次完整备份以来变化的所有文件。

## 备份目标配置

### 本地存储

最简单的备份方式，将数据备份到本地其他磁盘或目录。

```json
{
  "destination_type": "local",
  "destination_path": "/backup/daily"
}
```

**推荐配置**：
- 使用独立的备份磁盘
- 启用压缩节省空间
- 定期检查备份完整性

### S3 对象存储

将数据备份到云存储服务，如 AWS S3、阿里云 OSS、MinIO 等。

```json
{
  "destination_type": "s3",
  "destination_path": "s3://my-bucket/nas-backup",
  "s3_config": {
    "endpoint": "https://s3.amazonaws.com",
    "access_key": "YOUR_ACCESS_KEY",
    "secret_key": "YOUR_SECRET_KEY",
    "region": "us-east-1"
  }
}
```

**优势**：
- 数据异地存储，防止本地灾难
- 弹性扩展，无需担心容量
- 按需付费，成本可控

### SFTP 远程服务器

通过 SFTP 协议将数据备份到远程服务器。

```json
{
  "destination_type": "sftp",
  "destination_path": "/backup/nas",
  "sftp_config": {
    "host": "backup.example.com",
    "port": 22,
    "username": "backup",
    "private_key": "-----BEGIN RSA PRIVATE KEY-----\n..."
  }
}
```

## 调度配置

使用 cron 表达式设置备份计划：

| 表达式 | 说明 |
|--------|------|
| `0 2 * * *` | 每天凌晨 2 点 |
| `0 */6 * * *` | 每 6 小时一次 |
| `0 2 * * 0` | 每周日凌晨 2 点 |
| `0 2 1 * *` | 每月 1 日凌晨 2 点 |
| `0 2 1 1 *` | 每年 1 月 1 日凌晨 2 点 |

## 数据恢复

### 查看备份列表

```bash
# 获取备份任务列表
curl http://localhost:8080/api/v1/backup/tasks \
  -H "Authorization: Bearer {token}"
```

### 执行恢复

```bash
# 恢复备份
curl -X POST http://localhost:8080/api/v1/backup/restore \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "backup_id": "backup-20260314-020000",
    "target_path": "/data/restore"
  }'
```

### 选择性恢复

可以只恢复特定的文件或目录：

```json
{
  "backup_id": "backup-20260314-020000",
  "target_path": "/data/restore",
  "files": [
    "documents/report.pdf",
    "photos/vacation/"
  ]
}
```

## 最佳实践

### 3-2-1 备份原则

- **3** 份备份副本
- **2** 种不同存储介质
- **1** 份异地备份

### 备份验证

定期验证备份的完整性和可恢复性：

```bash
# 验证备份完整性
curl -X POST http://localhost:8080/api/v1/backup/verify/{id} \
  -H "Authorization: Bearer {token}"
```

### 保留策略

根据数据重要性和法规要求设置合理的保留时间：

| 数据类型 | 建议保留时间 |
|----------|-------------|
| 关键业务数据 | 1 年以上 |
| 普通文档 | 30-90 天 |
| 临时文件 | 7-14 天 |

### 加密建议

对于敏感数据，务必启用加密：

1. 使用强密码（至少 16 位）
2. 妥善保管加密密钥
3. 定期轮换密钥

## 故障排查

### 备份失败

**症状**：备份任务显示失败状态

**排查步骤**：
1. 检查目标存储空间是否充足
2. 验证源路径是否存在且可访问
3. 检查网络连接（远程备份）
4. 查看详细错误日志

### 恢复缓慢

**可能原因**：
- 备份文件过大
- 网络带宽不足
- 存储性能瓶颈

**解决方案**：
- 启用增量备份减少数据量
- 使用压缩传输
- 升级存储设备

## 相关文档

- [API 参考 - 备份管理](api.yaml#backup)
- [存储管理指南](STORAGE_GUIDE.md)
- [安全最佳实践](security/BEST_PRACTICES.md)

---

**版本**：v2.89.0  
**更新日期**：2026-03-16