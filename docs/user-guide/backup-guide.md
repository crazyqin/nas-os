# NAS-OS 备份系统用户指南

**版本**: v2.253.276  
**更新日期**: 2026-03-15

本文档详细介绍 NAS-OS 智能备份系统的使用方法。

## 目录

- [备份系统概述](#备份系统概述)
- [配置指南](#配置指南)
- [使用示例](#使用示例)
- [最佳实践](#最佳实践)
- [常见问题](#常见问题)

---

## 备份系统概述

NAS-OS 智能备份系统提供企业级数据保护能力，核心特性包括：

### 核心特性

| 特性 | 说明 |
|------|------|
| 增量备份 | 只备份变化的数据块，节省时间和空间 |
| 多压缩算法 | gzip/zstd/lz4 三种算法可选 |
| 数据加密 | AES-256-GCM 端到端加密 |
| 版本管理 | 多版本保留，自动清理过期备份 |
| 定时调度 | Cron 表达式灵活配置 |
| 选择性恢复 | 支持单文件/目录恢复 |

### 架构图

```
┌─────────────────────────────────────────────────────────────┐
│                     备份管理系统                              │
├─────────────┬─────────────┬─────────────┬─────────────────┤
│   调度引擎   │   压缩引擎   │   加密引擎   │    版本管理     │
├─────────────┴─────────────┴─────────────┴─────────────────┤
│                      核心备份引擎                            │
├────────────────────────────────────────────────────────────┤
│  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────────┐  │
│  │ 增量检测 │  │ 数据传输 │  │ 校验验证 │  │ 恢复管理   │  │
│  └─────────┘  └─────────┘  └─────────┘  └─────────────┘  │
└────────────────────────────────────────────────────────────┘
```

---

## 配置指南

### 1. 基础配置

编辑配置文件 `/etc/nas-os/config.yaml`：

```yaml
backup:
  # 启用备份功能
  enabled: true
  
  # 默认压缩算法: gzip, zstd, lz4
  default_compress: zstd
  
  # 默认加密设置
  default_encrypt: false
  
  # 备份存储路径
  backup_root: /backup
  
  # 临时文件目录
  temp_dir: /tmp/nas-backup
  
  # 并发数（0 为自动检测 CPU 核心数）
  workers: 0
  
  # 日志级别
  log_level: info
```

### 2. 保留策略配置

```yaml
backup:
  retention:
    # 保留天数
    daily: 7      # 保留 7 天内的每日备份
    weekly: 4     # 保留 4 周内的每周备份
    monthly: 12   # 保留 12 个月内的每月备份
    yearly: 3     # 保留 3 年内的每年备份
    
    # 或使用数量策略
    # max_versions: 30  # 最多保留 30 个版本
```

### 3. 定时任务配置

```yaml
backup:
  schedules:
    # 每日凌晨 2 点增量备份
    - name: daily-incremental
      source: /data/important
      dest: /backup/daily
      cron: "0 2 * * *"
      incremental: true
      compress: zstd
      retention: "7d"
      
    # 每周日 3 点完整备份
    - name: weekly-full
      source: /data
      dest: /backup/weekly
      cron: "0 3 * * 0"
      incremental: false
      compress: gzip
      retention: "4w"
      
    # 每月 1 日加密备份
    - name: monthly-encrypted
      source: /data/sensitive
      dest: /backup/monthly
      cron: "0 4 1 * *"
      encrypt: true
      key_file: /etc/nas-os/backup.key
```

### 4. 加密密钥管理

```bash
# 生成加密密钥
openssl rand -base64 32 > /etc/nas-os/backup.key
chmod 600 /etc/nas-os/backup.key

# 或使用密码文件
echo "your-strong-password" > /etc/nas-os/backup.pass
chmod 600 /etc/nas-os/backup.pass
```

---

## 使用示例

### 创建备份任务

#### 基础备份

```bash
# 创建简单备份任务
nasctl backup create mydata \
  --source /data/documents \
  --dest /backup/documents

# 带压缩的备份
nasctl backup create mydata \
  --source /data/documents \
  --dest /backup/documents \
  --compress zstd

# 加密备份
nasctl backup create sensitive \
  --source /data/private \
  --dest /backup/encrypted \
  --encrypt \
  --key-file /etc/nas-os/backup.key
```

#### 增量备份

```bash
# 创建增量备份（基于上次备份）
nasctl backup create daily \
  --source /data \
  --dest /backup/daily \
  --incremental

# 指定基础版本
nasctl backup create incremental \
  --source /data \
  --dest /backup/incr \
  --base-version 2026-03-01
```

### 执行备份

```bash
# 立即执行备份
nasctl backup run mydata

# 后台执行
nasctl backup run mydata --background

# 查看执行进度
nasctl backup status mydata

# 实时日志
nasctl backup logs mydata --follow
```

### 管理备份版本

```bash
# 列出所有版本
nasctl backup versions mydata

# 查看版本详情
nasctl backup show mydata --version 2026-03-15

# 删除特定版本
nasctl backup delete mydata --version 2026-03-10

# 清理过期版本
nasctl backup prune mydata
```

### 恢复数据

```bash
# 恢复最新版本
nasctl backup restore mydata \
  --dest /data/restored

# 恢复特定版本
nasctl backup restore mydata \
  --version 2026-03-10 \
  --dest /data/restored

# 选择性恢复
nasctl backup restore mydata \
  --version latest \
  --files "/documents/reports,/photos/2026" \
  --dest /data/restored

# 预览恢复内容（不实际恢复）
nasctl backup restore mydata \
  --version latest \
  --dry-run

# 强制覆盖现有文件
nasctl backup restore mydata \
  --version latest \
  --dest /data/restored \
  --force
```

### 调度管理

```bash
# 列出所有调度任务
nasctl backup schedule list

# 启用调度任务
nasctl backup schedule enable daily-backup

# 禁用调度任务
nasctl backup schedule disable daily-backup

# 手动触发调度任务
nasctl backup schedule trigger daily-backup

# 查看调度执行历史
nasctl backup schedule history daily-backup
```

### API 调用示例

```bash
# 创建备份任务
curl -X POST http://localhost:8080/api/v1/backups \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "mydata",
    "source": "/data/documents",
    "destination": "/backup/documents",
    "compress": "zstd",
    "incremental": true
  }'

# 执行备份
curl -X POST http://localhost:8080/api/v1/backups/mydata/run \
  -H "Authorization: Bearer $TOKEN"

# 查看备份状态
curl http://localhost:8080/api/v1/backups/mydata/status \
  -H "Authorization: Bearer $TOKEN"

# 恢复备份
curl -X POST http://localhost:8080/api/v1/backups/mydata/restore \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "latest",
    "destination": "/data/restored"
  }'
```

---

## 最佳实践

### 1. 备份策略设计

**3-2-1 备份原则**:
- 保留 **3** 份数据副本
- 使用 **2** 种不同存储介质
- **1** 份异地备份

```yaml
# 推荐配置示例
schedules:
  # 本地快速备份（每天）
  - name: local-daily
    source: /data
    dest: /backup/local
    cron: "0 2 * * *"
    incremental: true
    compress: lz4
    
  # 网络存储备份（每周）
  - name: network-weekly
    source: /data
    dest: /mnt/nas-backup/weekly
    cron: "0 3 * * 0"
    incremental: false
    compress: zstd
    
  # 云端备份（每月，加密）
  - name: cloud-monthly
    source: /data/critical
    dest: s3://my-bucket/backups
    cron: "0 4 1 * *"
    encrypt: true
```

### 2. 压缩算法选择

```bash
# 性能优先（SSD + 高 CPU）
--compress lz4

# 平衡选择（推荐）
--compress zstd

# 存储优先（空间紧张）
--compress gzip

# 不压缩（已压缩文件）
--compress none
```

### 3. 加密最佳实践

```bash
# 1. 使用强密钥
openssl rand -base64 32 > /etc/nas-os/backup.key

# 2. 限制密钥权限
chmod 400 /etc/nas-os/backup.key
chown nasd:nasd /etc/nas-os/backup.key

# 3. 备份密钥到安全位置
# 不要与备份数据存储在一起！
cp /etc/nas-os/backup.key /secure-location/backup-key-backup

# 4. 定期轮换密钥
nasctl backup rotate-key mydata \
  --old-key /etc/nas-os/backup.key \
  --new-key /etc/nas-os/backup-new.key
```

### 4. 监控与告警

```yaml
# 配置备份告警
alerts:
  backup_failed:
    enabled: true
    channels: [email, webhook]
    
  backup_overdue:
    enabled: true
    threshold: 24h  # 超过 24 小时未备份则告警
    
  storage_warning:
    enabled: true
    threshold: 80%  # 存储使用超过 80% 告警
```

### 5. 定期验证

```bash
# 验证备份完整性
nasctl backup verify mydata --version latest

# 定期恢复测试（建议每月）
nasctl backup restore mydata \
  --version latest \
  --dest /tmp/restore-test \
  --verify-only

# 自动化验证脚本
#!/bin/bash
# /etc/cron.monthly/backup-verify

for backup in $(nasctl backup list --format name); do
  echo "Verifying $backup..."
  nasctl backup verify $backup --version latest
  if [ $? -ne 0 ]; then
    echo "FAILED: $backup verification failed"
    # 发送告警
    curl -X POST $WEBHOOK_URL -d "{\"text\": \"Backup verification failed: $backup\"}"
  fi
done
```

---

## 常见问题

### Q1: 增量备份速度仍然很慢？

**A**: 检查以下几点：

1. **文件数量过多**: 超过 100 万小文件时，扫描耗时较长
   ```bash
   # 使用排除规则减少扫描范围
   nasctl backup create mydata \
     --exclude "*.tmp,*.log,*.cache" \
     --exclude-dir ".git,node_modules"
   ```

2. **存储性能**: 检查目标存储 IOPS
   ```bash
   # 测试存储性能
   fio --name=test --filename=/backup/test --rw=write --bs=1M --size=1G
   ```

3. **启用并行处理**
   ```yaml
   backup:
     workers: 4  # 根据 CPU 核心数调整
   ```

### Q2: 加密备份忘记密码怎么办？

**A**: 遗憾的是，AES-256-GCM 加密无法在没有密钥的情况下恢复。建议：

1. **定期备份密钥到多个安全位置**
2. **使用密码管理器存储密钥**
3. **考虑企业级密钥管理服务**

### Q3: 备份占满磁盘空间？

**A**: 调整保留策略或启用自动清理：

```bash
# 查看备份空间使用
nasctl backup usage

# 手动清理
nasctl backup prune mydata --dry-run

# 更激进的清理策略
nasctl backup prune mydata \
  --keep-last 5 \
  --delete-all-older-than 30d
```

### Q4: 如何跨版本恢复？

**A**: 向前兼容，向后需注意：

```bash
# v2.50.0 可以恢复 v2.49.0 的备份
nasctl backup restore old-backup --version 2026-03-01

# 旧版本可能无法恢复新版本备份格式
# 建议升级到最新版本后再恢复
```

### Q5: 备份任务卡住不执行？

**A**: 排查步骤：

```bash
# 1. 检查服务状态
systemctl status nas-os

# 2. 查看任务队列
nasctl backup queue list

# 3. 检查锁文件
ls -la /var/lock/nas-backup/

# 4. 强制取消卡住的任务
nasctl backup cancel mydata --force

# 5. 查看详细日志
journalctl -u nas-os -f | grep backup
```

### Q6: 如何实现异地灾备？

**A**: 推荐方案：

```yaml
# 方案 1: 直接备份到远程存储
schedules:
  - name: remote-backup
    source: /data
    dest: rsync://remote-server/backups
    compress: zstd
    encrypt: true

# 方案 2: 本地备份后同步
# 先备份到本地，再用 rsync 同步
# crontab: 0 5 * * * rsync -avz /backup/ remote:/disaster-recovery/

# 方案 3: 云存储备份
schedules:
  - name: cloud-backup
    source: /data/critical
    dest: s3://bucket/backups
    encrypt: true
```

### Q7: 恢复时提示校验失败？

**A**: 可能原因和解决方案：

```bash
# 1. 备份文件损坏 - 使用验证功能定位问题
nasctl backup verify mydata --version 2026-03-10 --verbose

# 2. 存储介质问题 - 检查磁盘健康
smartctl -a /dev/sda

# 3. 网络传输错误（远程备份）- 重新传输
nasctl backup repair mydata --version 2026-03-10
```

---

## 相关文档

- [API 文档](../API_GUIDE.md)
- [配置参考](../DEPLOYMENT_GUIDE_v2.5.0.md)
- [安全指南](../SECURITY_RESPONSE.md)
- [故障排除](../TROUBLESHOOTING.md)

## 获取帮助

- 📖 **文档中心**: [docs/](../)
- 🐛 **问题反馈**: [GitHub Issues](https://github.com/crazyqin/nas-os/issues)
- 💬 **社区讨论**: [GitHub Discussions](https://github.com/crazyqin/nas-os/discussions)