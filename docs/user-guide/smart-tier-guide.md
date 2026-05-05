# 智能存储分层调度器 (Smart Tier)

> **适用版本**: v2.483.0+ | **模块**: `internal/smarttier`

Smart Tier 是 NAS-OS 的智能数据分层引擎，自动将热点数据提升到 SSD、冷数据降级到 HDD，无需手动管理。

## 核心特性

### I/O 模式感知
自动检测四种访问模式并据此优化决策：
- **顺序读写** — 大文件流式访问（视频、备份）
- **随机读写** — 数据库、小文件密集访问
- **突发访问** — 短时间内高频访问（自动提升优先级）
- **流式访问** — 持续稳定的大块读写

### 预取预测
基于指数平滑算法预测文件访问时间，**提前**将数据从 HDD 提升到 SSD，减少首次访问延迟。

### 自适应阈值
根据 SSD 使用率动态调整提升/降级阈值：
- SSD 空间充足 → 降低提升阈值，更多数据上 SSD
- SSD 接近满载 → 提高提升阈值，优先保护 SSD 空间

### 批量迁移
按优先级排序迁移，支持批量大小限制，减少迁移碎片化。

## API 接口

所有接口挂载在 `/api/v1/smarttier/` 下。

### 记录 I/O 访问

```bash
POST /api/v1/smarttier/record
Content-Type: application/json

{
  "filePath": "/data/videos/movie.mp4",
  "bytesRead": 1048576,
  "bytesWritten": 0
}
```

### 获取 I/O 模式

```bash
GET /api/v1/smarttier/patterns
```

返回每个文件的检测到的访问模式和热度评分。

### 触发分析

```bash
POST /api/v1/smarttier/analyze
Content-Type: application/json

{
  "heatScores": {
    "/data/videos/movie.mp4": 85.5,
    "/data/archive/old.tar.gz": 12.3
  },
  "currentTiers": {
    "/data/videos/movie.mp4": "hdd",
    "/data/archive/old.tar.gz": "hdd"
  }
}
```

返回分层决策（promote/demote/keep）。

### 查看统计

```bash
GET /api/v1/smarttier/stats
```

### 配置管理

```bash
# 查看配置
GET /api/v1/smarttier/config

# 热更新配置（无需重启）
POST /api/v1/smarttier/config
Content-Type: application/json

{
  "ssdUsageThreshold": 0.85,
  "enableAdaptiveThreshold": true,
  "enablePrefetch": true,
  "basePromoteThreshold": 0.7,
  "baseDemoteThreshold": 0.3,
  "batchSize": 50,
  "minConfidence": 0.6
}
```

### 启停控制

```bash
POST /api/v1/smarttier/start
POST /api/v1/smarttier/stop
```

## 配置参数说明

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `ssdCapacityBytes` | — | SSD 总容量（字节） |
| `ssdUsageThreshold` | 0.85 | SSD 使用率上限 |
| `tieringInterval` | — | 分层扫描间隔 |
| `enableAdaptiveThreshold` | true | 是否启用自适应阈值 |
| `basePromoteThreshold` | 0.7 | 基础提升热度阈值 |
| `baseDemoteThreshold` | 0.3 | 基础降级热度阈值 |
| `enablePrefetch` | true | 是否启用预取预测 |
| `prefetchWindow` | — | 预取时间窗口 |
| `minConfidence` | 0.5 | 最低预取置信度 |
| `batchSize` | 50 | 单次批量迁移文件数 |
| `migrationBandwidth` | — | 迁移带宽限制 (bytes/s) |

## 工作原理

```
I/O 记录 → 模式识别 → 热度评分 → 预取预测 → 自适应阈值 → 批量迁移决策
```

1. 每次文件读写自动记录到调度器
2. 根据访问模式（顺序/随机/突发/流式）计算热度评分
3. 预取模块基于历史数据预测下一次访问时间
4. 自适应阈值根据 SSD 当前状态动态调整
5. 达到阈值的文件进入批量迁移队列，按优先级执行

---

*文档版本: v2.483.0 | 最后更新: 2026-05-05*
