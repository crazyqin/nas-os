# Cloud Drive Sync - 用户使用手册

> 文档版本：v1.0 | NAS-OS v2.x 兼容

---

## 目录

1. [功能概述](#1-功能概述)
2. [快速入门](#2-快速入门)
3. [添加云存储提供商](#3-添加云存储提供商)
4. [创建同步任务](#4-创建同步任务)
5. [管理同步任务](#5-管理同步任务)
6. [冲突处理](#6-冲突处理)
7. [实时同步](#7-实时同步)
8. [断点续传](#8-断点续传)
9. [常见问题](#9-常见问题)

---

## 1. 功能概述

Cloud Drive Sync（云盘同步）支持将本地存储与以下云存储服务进行双向同步：

| 类别 | 支持的提供商 |
|------|-------------|
| **国际云存储** | AWS S3、阿里云 OSS、腾讯云 COS、Backblaze B2、WebDAV |
| **常用网盘** | Google Drive、OneDrive、Dropbox |
| **中国网盘** | 115网盘、夸克网盘、阿里云盘、百度网盘 |

### 同步模式

- **镜像模式（Mirror）**：本地为主，同步时覆盖云端
- **备份模式（Backup）**：保留历史版本，支持版本回溯
- **同步模式（Sync）**：双向同步，实时保持一致
- **增量模式（Increment）**：仅同步新增/修改的文件

### 同步方向

- **上传（Upload）**：本地 → 云端
- **下载（Download）**：云端 → 本地
- **双向（Bidirectional）**：本地 ↔ 云端

---

## 2. 快速入门

### 2.1 通过 Web UI 操作

1. 登录 NAS-OS Web 管理界面
2. 进入 **应用中心** → **Cloud Drive Sync**
3. 添加云存储提供商
4. 创建同步任务
5. 触发首次同步

### 2.2 通过 CLI 操作

```bash
# 列出所有同步任务
nas-cli sync list

# 创建同步任务
nas-cli sync create \
  --name "我的照片备份" \
  --provider <provider-id> \
  --local /data/photos \
  --remote /backup/photos \
  --direction upload \
  --mode backup

# 手动触发同步
nas-cli sync trigger <task-id>

# 查看同步状态
nas-cli sync status <task-id>
```

---

## 3. 添加云存储提供商

### 3.1 S3 兼容存储（阿里云 OSS、腾讯云 COS、AWS S3、Backblaze B2）

```bash
nas-cli provider create \
  --name "我的阿里云OSS" \
  --type aliyun_oss \
  --endpoint "oss-cn-hangzhou.aliyuncs.com" \
  --region "cn-hangzhou" \
  --bucket "my-backup-bucket" \
  --access-key "YOUR_ACCESS_KEY" \
  --secret-key "YOUR_SECRET_KEY"
```

### 3.2 WebDAV

```bash
nas-cli provider create \
  --name "我的WebDAV" \
  --type webdav \
  --endpoint "https://webdav.example.com" \
  --access-key "username" \
  --secret-key "password"
```

### 3.3 Google Drive / OneDrive / Dropbox

这三者需要通过 OAuth2 授权：

1. 在 Web UI 中点击「添加提供商」，选择对应的类型
2. 点击「授权」按钮，会跳转到云服务的授权页面
3. 授权完成后返回，系统会自动保存 access token

> **注意**：OAuth2 token 会在过期前自动刷新，无需手动操作。

### 3.4 中国网盘（115、夸克、阿里云盘、百度网盘）

```bash
# 115网盘
nas-cli provider create \
  --name "我的115" \
  --type 115 \
  --access-token "YOUR_ACCESS_TOKEN"

# 百度网盘
nas-cli provider create \
  --name "我的百度云" \
  --type baidu_pan \
  --access-token "YOUR_ACCESS_TOKEN"
```

> 获取 Access Token 的方式请参考各网盘的开放平台文档。

---

## 4. 创建同步任务

### 4.1 基本参数

| 参数 | 说明 | 必填 |
|------|------|------|
| `--name` | 任务名称 | ✅ |
| `--provider` | 提供商 ID | ✅ |
| `--local` | 本地路径 | ✅ |
| `--remote` | 云端路径 | ✅ |
| `--direction` | 同步方向（upload/download/bidirect） | ✅ |
| `--mode` | 同步模式（mirror/backup/sync/increment） | ❌ |

### 4.2 调度配置

```bash
# 手动触发（默认）
nas-cli sync create --name "手动备份" --schedule manual

# 定时执行（每小时）
nas-cli sync create --name "每小时备份" --schedule interval --interval "1h"

# Cron 表达式（每天凌晨2点）
nas-cli sync create --name "每日备份" --schedule cron --cron "0 2 * * *"
```

### 4.3 过滤规则

```bash
# 仅同步指定类型的文件
nas-cli sync create \
  --name "照片备份" \
  --include "*.jpg" \
  --include "*.png" \
  --include "*.raw"

# 排除指定文件/目录
nas-cli sync create \
  --name "文档同步" \
  --exclude "*.tmp" \
  --exclude ".DS_Store" \
  --exclude "node_modules/"

# 文件大小限制（单位：字节，0表示不限制）
--max-file-size 10737418240  # 10GB
```

### 4.4 冲突策略

```bash
# 跳过冲突文件
--conflict skip

# 本地优先（上传覆盖云端）
--conflict local

# 远程优先（下载覆盖本地）
--conflict remote

# 较新优先（默认）
--conflict newer

# 重命名冲突文件（保留两个版本）
--conflict rename
```

### 4.5 高级选项

| 选项 | 说明 |
|------|------|
| `--delete-remote` | 删除本地文件时同时删除云端文件 |
| `--delete-local` | 删除云端文件时同时删除本地文件 |
| `--preserve-modtime` | 保留文件的原始修改时间 |
| `--checksum-verify` | 使用哈希校验代替时间戳比较 |
| `--bandwidth-limit` | 带宽限制（KB/s，0 表示不限制） |

---

## 5. 管理同步任务

### 5.1 查看任务列表

```bash
nas-cli sync list
```

输出示例：

```
ID         NAME           PROVIDER       STATUS    LAST SYNC
task_a1b2c3 我的照片备份    aliyun_oss     idle      2026-04-14 10:30
task_d4e5f6 工作文档同步    google_drive   running   --
task_g7h8i9 系统配置备份    s3_compatible  completed 2026-04-13 22:00
```

### 5.2 触发同步

```bash
# 触发指定任务
nas-cli sync trigger <task-id>

# 查看实时状态
nas-cli sync status <task-id>
```

### 5.3 暂停/恢复/取消

```bash
# 暂停运行中的任务
nas-cli sync pause <task-id>

# 恢复已暂停的任务
nas-cli sync resume <task-id>

# 取消运行中的任务
nas-cli sync cancel <task-id>
```

### 5.4 删除任务

```bash
nas-cli sync delete <task-id>
```

> 删除任务不会删除已同步的文件数据。

### 5.5 查看同步历史

```bash
# 查看任务执行历史
nas-cli sync history <task-id>

# 输出包含：执行时间、文件数量、传输量、错误信息
```

### 5.6 查看文件版本

```bash
# 查看某个文件的版本历史
nas-cli sync versions /data/photos/vacation.jpg

# 输出：版本号、修改时间、文件大小、同步任务
```

---

## 6. 冲突处理

当同一文件在本地和云端都有修改时，会产生冲突。

### 6.1 冲突策略详解

| 策略 | 行为 |
|------|------|
| `skip` | 跳过冲突文件，不进行同步 |
| `local` | 强制上传本地版本，覆盖云端 |
| `remote` | 强制下载云端版本，覆盖本地 |
| `newer` | 以修改时间较新的版本为准（**默认**） |
| `rename` | 自动重命名冲突文件，**同时保留两个版本** |
| `ask` | 暂停同步，等待用户在 Web UI 中手动选择 |

### 6.2 重命名策略示例

当 `photo.jpg` 发生冲突时：

```
photo.jpg_conflict_20260414_103000_local.jpg   # 保留本地版本
photo.jpg_conflict_20260414_103000_remote.jpg  # 保留云端版本
```

---

## 7. 实时同步

实时同步通过文件系统监控（inotify/fanotify）自动检测文件变化，并触发增量同步。

### 7.1 启用实时同步

```bash
# 在创建任务时指定
nas-cli sync create --name "实时文档" --schedule realtime

# 对已有任务启用
nas-cli realtime start --task <task-id>
```

### 7.2 实时同步状态

```bash
nas-cli realtime status
```

---

## 8. 断点续传

大文件传输中断后，系统会自动记录已传输的部分，下次触发时从断点继续。

### 8.1 查看待恢复的上传

```bash
nas-cli sync resumable list
```

### 8.2 恢复上传

```bash
nas-cli sync resumable resume <file-id>
```

> 断点续传记录会在任务成功完成或手动清除后删除。

---

## 9. 常见问题

### Q1: 同步失败，提示"空间不足"

检查云存储账户的可用配额。如果配额不足，需要升级云存储套餐或清理不需要的文件。

### Q2: 某些文件同步失败

常见原因：
- 文件名包含特殊字符（`\ / : * ? " < > |`）
- 文件路径超过云服务商的限制（如 Google Drive 单文件路径不超过 255 字符）
- 文件正在被其他程序占用

### Q3: 如何查看详细的同步日志？

```bash
# Web UI 中查看
应用中心 → Cloud Drive Sync → 日志

# CLI 中查看最近的错误
nas-cli sync status <task-id> --verbose
```

### Q4: 可以同时运行多个同步任务吗？

可以。多个任务可以并行运行，互不影响。

### Q5: 同步过程中可以关闭 NAS 吗？

可以。下次启动后，未完成的任务会自动从断点继续。

### Q6: 如何彻底删除云端数据？

Cloud Drive Sync **不会**自动删除云端数据。如果需要清理，请手动登录云盘管理界面删除。
