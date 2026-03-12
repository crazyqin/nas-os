# 相册功能部署指南

## 快速开始

### 1. 编译 NAS-OS

```bash
cd nas-os
go build -o nasd ./cmd/nasd
```

### 2. 创建数据目录

```bash
sudo mkdir -p /var/lib/nas-os/photos
sudo chown $USER:$USER /var/lib/nas-os/photos
```

### 3. 运行 NAS-OS

```bash
sudo ./nasd
```

### 4. 访问相册

- Web UI: http://localhost:8080/photos/
- API: http://localhost:8080/api/v1/photos

## 移动端备份

### iOS

使用「快捷指令」创建自动备份：

1. 打开「快捷指令」App
2. 创建新快捷指令
3. 添加操作：「获取最近的相片」
4. 添加操作：「URL」
5. 添加操作：「上传文件到 URL」
6. 设置 NAS-OS 地址：`http://your-nas-ip:8080/api/v1/photos/upload/batch`
7. 添加自动化：每天运行

### Android

使用 Tasker 或自动化工具：

1. 创建新任务
2. 动作：HTTP POST
3. URL: `http://your-nas-ip:8080/api/v1/photos/upload/batch`
4. 文件：选择相机文件夹
5. 触发器：充电时/连接到 WiFi 时

### 鸿蒙

参考 Android 方案，使用系统自动化功能。

## 配置说明

### 环境要求

- Go 1.26+
- ffmpeg（可选，用于 HEIC/RAW 支持）
- 至少 1GB 可用内存
- 充足的存储空间

### 性能调优

#### 缩略图配置

编辑 `/var/lib/nas-os/photos/photos-config.json`:

```json
{
  "thumbnailConfig": {
    "smallSize": 128,
    "mediumSize": 512,
    "largeSize": 1024,
    "quality": 85
  }
}
```

#### 上传限制

```json
{
  "maxUploadSize": 524288000
}
```

### AI 功能

需要额外安装：

```bash
# 安装 TensorFlow 或 ONNX Runtime
# 配置 AI 模型路径
```

## 备份策略

### 本地备份

```bash
# 定期备份照片数据
rsync -av /var/lib/nas-os/photos /backup/location
```

### 云备份

配置 S3 兼容存储：

```json
{
  "cloudBackup": {
    "enabled": true,
    "provider": "s3",
    "bucket": "your-bucket",
    "region": "cn-north-1"
  }
}
```

## 监控和维护

### 日志查看

```bash
journalctl -u nas-os -f
```

### 清理缓存

```bash
# 清理缩略图缓存
rm -rf /var/lib/nas-os/photos/cache/*
```

### 重建索引

```bash
# 删除索引文件，重启后自动重建
rm /var/lib/nas-os/photos/albums.json
rm /var/lib/nas-os/photos/persons.json
```

## 常见问题

### Q: 上传速度慢？
A: 检查网络带宽，考虑使用有线连接。

### Q: HEIC 格式不支持？
A: 安装 ffmpeg: `sudo apt install ffmpeg`

### Q: 内存占用高？
A: 减少并发上传数量，增加系统交换空间。

### Q: 如何迁移数据？
A: 直接复制 `/var/lib/nas-os/photos` 目录到新位置。

## 安全建议

1. 启用 HTTPS
2. 配置防火墙规则
3. 定期备份数据
4. 使用强密码
5. 限制外网访问

## 更新升级

```bash
# 停止服务
sudo systemctl stop nas-os

# 备份数据
cp -r /var/lib/nas-os/photos /backup/

# 更新二进制
wget https://github.com/your-org/nas-os/releases/latest/nasd
chmod +x nasd
sudo mv nasd /usr/local/bin/

# 重启服务
sudo systemctl start nas-os
```

## 技术支持

- 文档：https://nas-os.io/docs
- 社区：https://github.com/nas-os/nas-os/discussions
- Issue: https://github.com/nas-os/nas-os/issues
