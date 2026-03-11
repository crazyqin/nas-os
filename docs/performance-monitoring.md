# 性能监控模块

## 概述

`perf` 模块提供完整的 API 性能监控能力，包括：

- 响应时间/吞吐量指标收集
- 性能分析工具
- 慢查询日志
- 性能优化建议生成

## API 端点

### 性能指标

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/perf/metrics` | GET | 获取整体性能指标 |
| `/api/v1/perf/metrics/endpoints` | GET | 获取所有端点指标 |
| `/api/v1/perf/metrics/endpoints/:path` | GET | 获取特定端点详情 |
| `/api/v1/perf/metrics/throughput` | GET | 获取吞吐量统计 |
| `/api/v1/perf/metrics/window` | GET | 获取时间窗口统计 (最近 1 分钟) |
| `/api/v1/perf/metrics/baseline` | GET | 获取性能基线 |

### 慢查询日志

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/perf/slow-logs` | GET | 获取慢请求日志 |
| `/api/v1/perf/slow-logs` | DELETE | 清除慢请求日志 |

### 性能分析

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/perf/analyze` | GET | 执行完整分析 |
| `/api/v1/perf/analyze/report` | GET | 获取文本报告 |
| `/api/v1/perf/analyze/health` | GET | 获取健康分数 (0-100) |
| `/api/v1/perf/analyze/anomalies` | GET | 获取异常列表 |
| `/api/v1/perf/analyze/recommendations` | GET | 获取优化建议 |

### 性能对比

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/perf/compare` | GET | 与基线对比 |

### 配置

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/perf/config` | GET | 获取当前配置 |
| `/api/v1/perf/config/threshold` | PUT | 更新慢请求阈值 |

## 使用示例

### 获取健康分数

```bash
curl http://localhost:8080/api/v1/perf/analyze/health
```

响应:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "score": 85,
    "status": "good",
    "summary": {
      "avgLatency": 45.2,
      "errorRate": 0.5,
      "currentRPS": 12.3,
      "slowRequests": 5,
      "anomalyCount": 0
    }
  }
}
```

### 获取慢请求日志

```bash
curl http://localhost:8080/api/v1/perf/slow-logs?limit=10
```

### 获取优化建议

```bash
curl http://localhost:8080/api/v1/perf/analyze/recommendations
```

响应:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "priority": 1,
      "category": "performance",
      "title": "优化慢端点响应",
      "description": "端点 GET /api/v1/volumes 平均响应 520ms，建议添加缓存或优化查询",
      "impact": "减少该端点响应时间，提升吞吐量",
      "endpoint": "GET:/api/v1/volumes"
    }
  ]
}
```

### 更新慢请求阈值

```bash
curl -X PUT http://localhost:8080/api/v1/perf/config/threshold \
  -H "Content-Type: application/json" \
  -d '{"thresholdMs": 300}'
```

## 配置

默认配置:

| 参数 | 默认值 | 描述 |
|------|--------|------|
| `SlowThreshold` | 500ms | 慢请求阈值 |
| `SlowLogMax` | 1000 | 最大慢日志条数 |
| `SlowLogPath` | /var/log/nas-os/slow.log | 慢日志文件路径 |
| `EnableBaseline` | true | 是否启用基线计算 |
| `BaselineInterval` | 5m | 基线更新间隔 |

## 健康分数计算

健康分数 (0-100) 基于以下因素计算:

- **响应时间** (-20分): 平均响应时间超过 100ms 时扣分
- **错误率** (-30分): 错误率超过 1% 时扣分
- **慢请求比例** (-20分): 慢请求比例超过 0.5% 时扣分
- **异常检测** (-5~10分): 每个异常扣分

## 异常检测

自动检测以下异常:

- `latency`: 平均响应时间异常
- `latency_p95`: P95 响应时间异常
- `error_rate`: 错误率异常
- `slow_requests`: 慢请求比例异常
- `endpoint_error_rate`: 端点错误率异常
- `endpoint_latency`: 端点延迟异常

## 文件结构

```
internal/perf/
├── manager.go    # 性能监控管理器，指标收集
├── analyzer.go   # 性能分析器，异常检测，建议生成
└── handlers.go   # HTTP 处理器
```