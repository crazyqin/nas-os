# 存储复制模块 (Storage Replication)

跨设备/跨节点数据复制功能，支持实时同步、定时复制和双向复制。

## 功能特性

### 1. 复制类型

- **实时同步 (realtime)**: 监控文件变化实时同步到目标端
- **定时复制 (scheduled)**: 按计划定时同步增量数据
- **双向复制 (bidirectional)**: 支持双向同步，保持两端一致

### 2. 冲突检测与解决

支持多种冲突解决策略：

| 策略 | 说明 |
|------|------|
| `source_wins` | 源端优先，覆盖目标 |
| `target_wins` | 目标端优先，保留目标 |
| `newer_wins` | 较新文件优先 |
| `larger_wins` | 较大文件优先 |
| `rename` | 重命名冲突文件，保留两端 |
| `skip` | 跳过冲突文件 |
| `manual` | 手动解决冲突 |

### 3. 实时文件监控

基于 fsnotify 实现的文件系统监控：
- 自动监控源目录及其子目录
- 防抖处理避免频繁触发
- 支持目录创建时自动添加监控

## API 端点

```
POST   /api/v1/replications                    # 创建复制任务
GET    /api/v1/replications                    # 列出复制任务
GET    /api/v1/replications/stats              # 复制统计
GET    /api/v1/replications/conflicts          # 列出所有冲突
GET    /api/v1/replications/:id                # 获取任务详情
PUT    /api/v1/replications/:id                # 更新任务
DELETE /api/v1/replications/:id                # 删除任务
POST   /api/v1/replications/:id/sync           # 手动同步
POST   /api/v1/replications/:id/pause          # 暂停任务
POST   /api/v1/replications/:id/resume         # 恢复任务
GET    /api/v1/replications/:id/conflicts      # 列出任务冲突
POST   /api/v1/replications/conflicts/:id/resolve  # 解决冲突
```

## 使用示例

### 创建实时同步任务

```bash
curl -X POST http://localhost:8080/api/v1/replications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "photos-backup",
    "source_path": "/mnt/data/photos",
    "target_path": "/mnt/backup/photos",
    "type": "realtime",
    "enabled": true
  }'
```

### 创建定时复制任务

```bash
curl -X POST http://localhost:8080/api/v1/replications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "daily-docs-sync",
    "source_path": "/mnt/data/docs",
    "target_path": "/mnt/backup/docs",
    "type": "scheduled",
    "schedule": "daily",
    "enabled": true,
    "compress": true
  }'
```

### 创建双向同步任务

```bash
curl -X POST http://localhost:8080/api/v1/replications \
  -H "Content-Type: application/json" \
  -d '{
    "name": "bidir-sync",
    "source_path": "/mnt/data/projects",
    "target_path": "/mnt/backup/projects",
    "type": "bidirectional",
    "enabled": true
  }'
```

### 解决冲突

```bash
curl -X POST http://localhost:8080/api/v1/replications/conflicts/conflict-123/resolve \
  -H "Content-Type: application/json" \
  -d '{"strategy": "newer_wins"}'
```

## 配置

### 全局配置

```json
{
  "max_concurrent": 2,        // 最大并发任务数
  "bandwidth_limit": 0,       // 带宽限制 (KB/s, 0 表示不限)
  "ssh_key_path": "~/.ssh/id_rsa",
  "retries": 3,               // 重试次数
  "timeout": 3600             // 超时时间 (秒)
}
```

### 任务配置

```json
{
  "name": "任务名称",
  "source_path": "/源路径",
  "target_path": "/目标路径",
  "target_host": "远程主机",     // 可选，空表示本地
  "type": "realtime|scheduled|bidirectional",
  "schedule": "hourly|daily|weekly",
  "enabled": true,
  "compress": false,            // 是否压缩传输
  "delete_extraneous": false    // 是否删除目标端多余文件
}
```

## 依赖

- **rsync**: 底层同步工具
- **fsnotify**: 文件系统监控

## 目录结构

```
internal/replication/
├── manager.go        # 核心管理器
├── handlers.go       # REST API 处理器
├── conflict.go       # 冲突检测与解决
├── watcher.go        # 实时文件监控
├── *_test.go         # 单元测试
└── README.md         # 本文档
```

## 注意事项

1. **rsync 依赖**: 同步功能依赖系统安装的 rsync 工具
2. **SSH 配置**: 远程复制需要配置 SSH 密钥认证
3. **权限要求**: 需要对源路径和目标路径有读写权限
4. **双向复制**: 双向复制建议仅在同一网络内使用，避免数据冲突