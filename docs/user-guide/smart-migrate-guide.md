# 智能数据迁移指南

> **版本**: v2.477.0+ | **适用版本**: NAS-OS v2.477.0 及以上

## 概述

智能数据迁移支持跨存储池、跨设备、跨节点的数据迁移，对标群晖 Data Migration 和 TrueNAS 数据迁移功能。内置 SHA-256 校验、带宽控制、重试策略，确保数据完整性和传输可靠性。

## 核心特性

- **四种迁移模式**：复制 / 移动 / 同步 / 校验复制
- **SHA-256 校验**：传输完成后自动校验数据完整性
- **带宽限制**：可控制传输速度，避免影响正常业务
- **失败重试**：最多 3 次重试，间隔 5 秒
- **进度追踪**：实时显示传输速度、已完成文件数、预计剩余时间
- **并发控制**：最多 3 个任务并行执行
- **历史记录**：完整的迁移历史和结果存档

## 迁移模式

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| `copy` | 复制文件到目标路径 | 数据备份、副本创建 |
| `move` | 移动文件到目标路径 | 存储池迁移、空间释放 |
| `sync` | 同步源到目标 | 数据同步、镜像维护 |
| `replicate` | 复制并校验（含 SHA-256） | 关键数据迁移、灾备 |

## API 接口

### 创建迁移任务

```bash
curl -X POST http://localhost:8080/api/v1/migrate/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "name": "迁移到新存储池",
    "source_path": "/data/old-pool/important",
    "dest_path": "/data/new-pool/important",
    "type": "replicate",
    "options": {
      "exclude_patterns": ["*.tmp", ".cache"],
      "compress": false,
      "dry_run": false
    }
  }'
```

### 启动任务

```bash
curl -X POST http://localhost:8080/api/v1/migrate/tasks/{id}/start
```

### 暂停任务

```bash
curl -X POST http://localhost:8080/api/v1/migrate/tasks/{id}/pause
```

### 取消任务

```bash
curl -X POST http://localhost:8080/api/v1/migrate/tasks/{id}/cancel
```

### 查看任务列表

```bash
curl http://localhost:8080/api/v1/migrate/tasks
```

### 查看任务详情

```bash
curl http://localhost:8080/api/v1/migrate/tasks/{id}
```

### 查看迁移历史

```bash
curl http://localhost:8080/api/v1/migrate/history
```

## 迁移选项

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `exclude_patterns` | string[] | [] | 排除文件模式（glob） |
| `include_patterns` | string[] | [] | 仅包含指定模式 |
| `dry_run` | bool | false | 模拟运行，不实际传输 |
| `sync_delete` | bool | false | 同步删除目标端多余文件 |
| `compress` | bool | false | 传输时压缩数据 |
| `encrypt` | bool | false | 传输时加密数据 |

## 默认配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 最大并发任务 | 3 | 同时运行的迁移任务数 |
| 分块大小 | 64 MB | 单次传输块大小 |
| SHA-256 校验 | 开启 | 传输后自动校验 |
| 保留权限 | 开启 | 保持文件权限不变 |
| 重试次数 | 3 | 失败后重试次数 |
| 重试间隔 | 5 秒 | 两次重试之间的等待时间 |

## 任务状态

| 状态 | 说明 |
|------|------|
| `pending` | 等待启动 |
| `running` | 正在传输 |
| `paused` | 已暂停 |
| `completed` | 迁移完成 |
| `failed` | 迁移失败 |
| `cancelled` | 已取消 |

## 工作流程

```
创建任务 → 等待/启动 → 扫描源文件 → 分块传输 → 校验 → 完成
                ↓                         ↓
            暂停/取消                   失败重试 (最多3次)
```

## 最佳实践

1. **先 Dry Run**：大规模迁移前先用 `dry_run: true` 预估时间和文件数
2. **排除临时文件**：使用 `exclude_patterns` 排除 `.tmp`、`.cache` 等
3. **关键数据用 replicate**：重要数据使用校验复制模式确保完整性
4. **带宽控制**：业务高峰期限制带宽，低谷期全速传输
5. **检查历史**：迁移完成后检查历史记录确认校验状态
