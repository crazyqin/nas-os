# 快速传输

> **功能模块**: `fasttransfer` | **API 前缀**: `/api/v1/fasttransfer`

## 概述

高速文件传输引擎，对标群晖 Presto File Server。支持 AES 加密、压缩传输、带宽控制和并发传输管理，专为大文件跨存储迁移优化。

## 核心能力

- **并发传输** — 最多 4 个任务同时运行（可配置）
- **AES 加密** — 传输数据端到端加密
- **智能压缩** — 0-9 级压缩，节省带宽
- **带宽限制** — 可设置最大传输速率（MB/s）
- **进度追踪** — 实时速度、已传输量、预估剩余时间

## API 接口

### 创建传输任务

```
POST /api/v1/fasttransfer/transfers
```

**请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 任务名称 |
| `source_path` | string | ✅ | 源文件/目录路径 |
| `dest_path` | string | ✅ | 目标路径 |

**响应**: `201 Created`

```json
{
  "id": "xfer-abc123",
  "name": "照片迁移",
  "source_path": "/mnt/hdd/photos",
  "dest_path": "/mnt/ssd/photos",
  "status": "running",
  "total_bytes": 10737418240,
  "transferred": 2147483648,
  "speed_mbps": 450.5,
  "compressed": true,
  "encrypted": true,
  "started_at": "2026-05-06T09:00:00Z"
}
```

### 查询传输列表

```
GET /api/v1/fasttransfer/transfers
```

### 查询单个传输

```
GET /api/v1/fasttransfer/transfers/:id
```

### 取消传输

```
POST /api/v1/fasttransfer/transfers/:id/cancel
```

### 获取统计信息

```
GET /api/v1/fasttransfer/stats
```

返回并发传输数、总吞吐量等汇总数据。

## 配置参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `max_concurrent` | 4 | 最大并发传输数 |
| `compress_level` | 6 | 压缩级别（0=不压缩，9=最大压缩） |
| `encrypt_aes` | true | 是否 AES 加密 |
| `chunk_size_mb` | 8 | 分块大小 |
| `bandwidth_mbps` | 0 | 带宽限制（0=不限速） |

## 使用场景

| 场景 | 说明 |
|------|------|
| 存储迁移 | HDD → SSD 数据迁移 |
| 跨池复制 | Fusion Pool 间数据搬迁 |
| 大文件传输 | 4K 视频、数据库备份等大文件 |
| 安全传输 | 加密传输敏感数据到远程存储 |
