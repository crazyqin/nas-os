# 备份增强功能实现计划

## 目标
1. ✅ 增量备份优化 - 使用 rsync + 硬链接实现类似 Time Machine 的增量备份
2. ✅ 云端备份支持 - S3/WebDAV 后端集成
3. ✅ 备份加密 - 使用 OpenSSL/gpg 加密
4. ✅ 一键恢复功能 - 简化的恢复流程

## 实现方案

### 1. 增量备份 (Incremental Backup)
- 使用 rsync 的 `--link-dest` 参数实现硬链接增量
- 每次完整扫描源目录，但仅存储变化的文件
- 未变化的文件通过硬链接指向上一版本
- 优势：节省空间，每个备份看起来都是完整的

### 2. 云端备份
- **S3 支持**: 使用 AWS SDK 或 rclone
- **WebDAV 支持**: 使用 webdav 客户端库
- 支持加密后上传
- 支持断点续传

### 3. 备份加密
- 使用 OpenSSL AES-256-CBC 加密
- 密码/密钥文件保护
- 加密元数据（文件名、大小）

### 4. 一键恢复
- 列出可用备份点
- 选择备份点一键恢复
- 支持整库恢复或单文件恢复
- 支持恢复到不同路径

## 文件结构
```
nas-os/internal/backup/
├── manager.go           # 现有备份管理器
├── handlers.go          # 现有 API 处理器
├── incremental.go       # 增量备份实现 (新增)
├── cloud.go             # 云端备份支持 (新增)
├── encrypt.go           # 加密功能 (新增)
├── restore.go           # 增强恢复功能 (新增)
└── config.go            # 配置结构扩展 (新增)
```

## 依赖
- github.com/aws/aws-sdk-go-v2 (S3)
- github.com/studio-b12/gowebdav (WebDAV)
- 或使用 rclone 作为统一后端
