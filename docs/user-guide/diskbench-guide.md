# 磁盘性能基准测试

> **功能模块**: `diskbench` | **API 前缀**: `/api/v1/diskbench`

## 概述

磁盘性能基准测试工具，用于评估存储设备的顺序读写、随机读写 IOPS 和延迟表现。支持自定义测试路径和文件大小，帮助用户选择最优存储方案。

## 核心能力

- **顺序读写测试** — 评估大文件传输性能（MB/s）
- **随机 IOPS 测试** — 评估小文件/数据库场景性能
- **延迟测量** — 平均延迟 + P99 延迟
- **多存储对比** — 同时测试 SSD、HDD、网络存储等不同路径

## API 接口

### 启动测试

```
POST /api/v1/diskbench/run
```

**请求体**:

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `target_path` | string | ✅ | 测试目标路径（如 `/mnt/ssd`、`/mnt/hdd`） |
| `file_size_mb` | int | ❌ | 测试文件大小（MB），默认 256 |

**响应**: `202 Accepted` — 返回测试任务对象

```json
{
  "id": "bench-1714950000000",
  "target_path": "/mnt/ssd",
  "status": "running",
  "file_size_mb": 256,
  "started_at": "2026-05-06T09:00:00Z"
}
```

### 查询结果列表

```
GET /api/v1/diskbench/results
```

返回所有历史测试结果。

### 查询单个结果

```
GET /api/v1/diskbench/results/:id
```

**完成后的结果示例**:

```json
{
  "id": "bench-1714950000000",
  "target_path": "/mnt/ssd",
  "status": "completed",
  "seq_read_mbps": 3520.5,
  "seq_write_mbps": 2980.2,
  "random_read_iops": 85000,
  "random_write_iops": 72000,
  "latency_avg": "0.12ms",
  "latency_p99": "0.85ms",
  "file_size_mb": 256,
  "block_size_kb": 4,
  "started_at": "2026-05-06T09:00:00Z",
  "completed_at": "2026-05-06T09:00:45Z"
}
```

## 使用场景

| 场景 | 说明 |
|------|------|
| 新盘验收 | 购入新 SSD/HDD 后验证标称性能 |
| 存储选型 | 对比 NVMe vs SATA SSD vs HDD 实际表现 |
| 故障排查 | 存储变慢时定位瓶颈 |
| RAID 评估 | 比较不同 RAID 级别的性能差异 |

## 注意事项

- 测试会生成临时文件到 `/tmp/nas-bench`，测试完成后自动清理
- 避免在生产数据盘上运行大文件测试
- 建议在系统空闲时运行以获得准确结果
