# 下载中心模块

下载中心是 NAS-OS 的核心功能模块，提供 BT、HTTP、FTP 等多种协议的下载支持。

## 功能特性

### 核心功能
- ✅ **BT 下载**: 支持种子文件和磁力链接
- ✅ **HTTP/FTP 下载**: 多线程高速下载
- ✅ **PT 支持**: 做种/分享率监控
- ✅ **下载计划**: 定时下载/限速计划
- ✅ **RSS 订阅**: 自动下载新剧集（开发中）
- ⏳ **网盘对接**: 百度网盘/阿里云盘/115（规划中）

### 技术特性
- 实时进度显示（5 秒刷新）
- 下载速度限制
- 定时任务调度
- 任务优先级管理
- 自动分类保存

## 快速开始

### 1. 启动 Transmission（BT 下载后端）

```bash
# 创建下载目录
sudo mkdir -p /opt/nas/downloads/{config,watch}

# 启动 Transmission 容器
docker-compose -f docker-compose.transmission.yml up -d

# 访问 Web UI
# http://localhost:9091
# 默认账号：admin / admin123
```

### 2. 访问下载中心

启动 NAS-OS 后，访问：
- **Web UI**: `http://localhost:8080/downloader`
- **API**: `http://localhost:8080/api/v1/downloader`

## API 文档

### 创建下载任务

```bash
curl -X POST http://localhost:8080/api/v1/downloader/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "url": "magnet:?xt=urn:btih:xxx",
    "name": "示例任务",
    "dest_path": "/downloads/movies"
  }'
```

### 列出所有任务

```bash
curl http://localhost:8080/api/v1/downloader/tasks
```

### 获取任务详情

```bash
curl http://localhost:8080/api/v1/downloader/tasks/{id}
```

### 暂停/恢复任务

```bash
# 暂停
curl -X POST http://localhost:8080/api/v1/downloader/tasks/{id}/pause

# 恢复
curl -X POST http://localhost:8080/api/v1/downloader/tasks/{id}/resume
```

### 删除任务

```bash
curl -X DELETE "http://localhost:8080/api/v1/downloader/tasks/{id}?delete_files=false"
```

### 获取统计信息

```bash
curl http://localhost:8080/api/v1/downloader/stats
```

## 数据结构

### 下载任务 (DownloadTask)

```json
{
  "id": "abc123",
  "name": "示例任务",
  "type": "magnet",
  "url": "magnet:?xt=urn:btih:xxx",
  "status": "downloading",
  "progress": 45.5,
  "total_size": 1073741824,
  "downloaded": 488579031,
  "uploaded": 107374182,
  "speed": 1048576,
  "upload_speed": 262144,
  "peers": 15,
  "seeds": 8,
  "ratio": 0.22,
  "dest_path": "/downloads/movies",
  "created_at": "2024-01-01T10:00:00Z",
  "updated_at": "2024-01-01T10:05:00Z"
}
```

### 下载类型 (DownloadType)

- `bt`: BT 种子文件
- `magnet`: 磁力链接
- `http`: HTTP 下载
- `ftp`: FTP 下载
- `cloud`: 网盘下载

### 下载状态 (DownloadStatus)

- `waiting`: 等待中
- `downloading`: 下载中
- `paused`: 已暂停
- `completed`: 已完成
- `error`: 错误
- `seeding`: 做种中

## 限速配置

```json
{
  "speed_limit": {
    "download_limit": 1024,  // KB/s, 0=不限
    "upload_limit": 256,     // KB/s, 0=不限
    "enabled": true
  }
}
```

## 计划任务配置

```json
{
  "schedule": {
    "start_time": "02:00",
    "end_time": "08:00",
    "days": [0, 1, 2, 3, 4, 5, 6],  // 0=周日
    "enabled": true
  }
}
```

## 开发说明

### 目录结构

```
nas-os/
├── internal/downloader/
│   ├── types.go       # 数据类型定义
│   ├── manager.go     # 下载管理器核心
│   └── handlers.go    # API 处理器
├── webui/pages/downloader/
│   └── index.html     # Web UI 页面
└── docker-compose.transmission.yml
```

### 集成 Transmission

当前版本使用 Transmission 作为 BT 下载后端。管理器通过 RPC API 与 Transmission 通信：

```go
// 设置 Transmission 地址
downloadMgr.SetTransmissionURL("http://localhost:9091")
```

### 添加新的下载协议

1. 在 `types.go` 中添加新的 `DownloadType`
2. 在 `manager.go` 中实现协议特定的下载逻辑
3. 更新 `detectType()` 函数支持新协议

### TODO

- [ ] 实现真实的 Transmission RPC 客户端
- [ ] 添加 qBittorrent 支持
- [ ] RSS 订阅自动下载
- [ ] 网盘 API 对接（百度/阿里/115）
- [ ] 下载任务优先级队列
- [ ] 带宽调度算法优化
- [ ] 下载完成通知（邮件/推送）

## 故障排除

### Transmission 无法连接

1. 检查容器是否运行：`docker ps | grep transmission`
2. 检查端口是否开放：`netstat -tlnp | grep 9091`
3. 查看容器日志：`docker logs nas-transmission`

### 下载速度慢

1. 检查网络连接
2. 增加 Tracker 服务器
3. 检查防火墙设置
4. 尝试更换下载源

### 任务卡在 Waiting 状态

1. 检查磁力链接/种子是否有效
2. 检查存储空间是否充足
3. 查看 Transmission Web UI 确认任务状态
