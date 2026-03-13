# 性能监控配置指南

**版本**: v2.2.0  
**更新日期**: 2026-03-21

---

## 📖 概述

NAS-OS 提供全面的性能监控功能，帮助管理员实时了解系统运行状态、发现性能瓶颈、并获得优化建议。性能监控涵盖 API 响应、系统资源、存储 I/O 等多个维度。

### 特性

- ✅ API 性能追踪
- ✅ 系统资源监控
- ✅ 慢请求日志
- ✅ 性能基线学习
- ✅ 异常检测告警
- ✅ 优化建议生成

---

## 🚀 快速开始

### 访问性能监控

#### WebUI

1. 登录 NAS-OS Web 管理界面
2. 导航到 **系统** → **性能监控**
3. 查看实时性能指标

#### API

```bash
# 获取性能概览
curl http://localhost:8080/api/v1/perf/metrics \
  -H "Authorization: Bearer $TOKEN"

# 获取健康分数
curl http://localhost:8080/api/v1/perf/analyze/health \
  -H "Authorization: Bearer $TOKEN"
```

#### CLI

```bash
# 查看性能概览
nasctl perf status

# 查看健康分数
nasctl perf health
```

---

## 📊 性能指标

### API 性能指标

| 指标 | 说明 | 单位 |
|------|------|------|
| 平均响应时间 | 所有请求的平均响应时间 | 毫秒 |
| P95 响应时间 | 95% 的请求响应时间 | 毫秒 |
| P99 响应时间 | 99% 的请求响应时间 | 毫秒 |
| 错误率 | 错误请求占总请求的比例 | 百分比 |
| 吞吐量 | 每秒处理的请求数 | req/s |
| 慢请求数 | 响应时间超过阈值的请求数 | 个 |

### 系统资源指标

| 指标 | 说明 | 单位 |
|------|------|------|
| CPU 使用率 | CPU 占用百分比 | % |
| 内存使用率 | 内存占用百分比 | % |
| 磁盘 I/O | 磁盘读写速度 | MB/s |
| 网络流量 | 网络收发速度 | MB/s |
| Goroutine 数 | Go 协程数量 | 个 |
| GC 暂停时间 | 垃圾回收暂停时间 | 毫秒 |

### 存储性能指标

| 指标 | 说明 |
|------|------|
| IOPS | 每秒 I/O 操作数 |
| 延迟 | I/O 操作延迟 |
| 队列深度 | I/O 队列长度 |
| 缓存命中率 | 缓存命中比例 |

---

## ⚙️ 配置项

### 默认配置

```yaml
perf:
  enabled: true
  slow_threshold: 500        # 慢请求阈值 (ms)
  slow_log_max: 1000         # 最大慢日志条数
  slow_log_path: /var/log/nas-os/slow.log
  enable_baseline: true      # 启用基线计算
  baseline_interval: 5m      # 基线更新间隔
  collect_interval: 10s      # 数据采集间隔
  retention_days: 30         # 数据保留天数
```

### 更新配置

```bash
# 更新慢请求阈值
curl -X PUT http://localhost:8080/api/v1/perf/config/threshold \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"thresholdMs": 300}'

# CLI
nasctl perf config --slow-threshold 300
```

---

## 📈 性能分析

### 获取性能报告

```bash
# 完整分析报告
curl http://localhost:8080/api/v1/perf/analyze \
  -H "Authorization: Bearer $TOKEN"

# 文本格式报告
curl http://localhost:8080/api/v1/perf/analyze/report \
  -H "Authorization: Bearer $TOKEN"
```

报告示例：
```
=== NAS-OS 性能分析报告 ===
生成时间: 2026-03-21 10:00:00

【总体健康分数】85/100 (良好)

【API 性能】
- 平均响应时间: 45.2ms
- P95 响应时间: 120ms
- P99 响应时间: 350ms
- 错误率: 0.3%
- 当前吞吐量: 125 req/s

【系统资源】
- CPU 使用率: 35%
- 内存使用率: 62%
- Goroutine 数量: 156

【异常检测】
- 无异常

【优化建议】
1. [中] 端点 GET /api/v1/volumes 平均响应 520ms，建议添加缓存
2. [低] 内存使用率较高，建议检查内存泄漏
```

### 健康分数计算

健康分数 (0-100) 基于以下因素：

| 因素 | 扣分规则 |
|------|----------|
| 响应时间 | 平均响应 > 100ms 时扣 0-20 分 |
| 错误率 | 错误率 > 1% 时扣 0-30 分 |
| 慢请求比例 | 比例 > 0.5% 时扣 0-20 分 |
| 异常检测 | 每个异常扣 5-10 分 |

健康等级：
- 90-100：优秀 🟢
- 70-89：良好 🟡
- 50-69：一般 🟠
- 0-49：差 🔴

---

## 🔍 端点分析

### 查看所有端点性能

```bash
curl http://localhost:8080/api/v1/perf/metrics/endpoints \
  -H "Authorization: Bearer $TOKEN"
```

响应示例：
```json
{
  "code": 0,
  "data": [
    {
      "path": "GET:/api/v1/volumes",
      "count": 1520,
      "avgLatency": 45.2,
      "p95Latency": 120,
      "errorRate": 0.1,
      "lastAccess": "2026-03-21T10:00:00Z"
    },
    {
      "path": "POST:/api/v1/volumes",
      "count": 85,
      "avgLatency": 520,
      "p95Latency": 1200,
      "errorRate": 0.5,
      "lastAccess": "2026-03-21T09:55:00Z"
    }
  ]
}
```

### 查看特定端点详情

```bash
curl "http://localhost:8080/api/v1/perf/metrics/endpoints/GET:/api/v1/volumes" \
  -H "Authorization: Bearer $TOKEN"
```

---

## 📝 慢请求日志

### 查看慢请求

```bash
# 获取最近 20 条慢请求
curl "http://localhost:8080/api/v1/perf/slow-logs?limit=20" \
  -H "Authorization: Bearer $TOKEN"
```

响应示例：
```json
{
  "code": 0,
  "data": [
    {
      "id": "slow-001",
      "timestamp": "2026-03-21T09:45:00Z",
      "method": "POST",
      "path": "/api/v1/volumes/data/snapshots",
      "latency": 1520,
      "status": 200,
      "client_ip": "192.168.1.100",
      "user_agent": "Mozilla/5.0..."
    }
  ]
}
```

### 清除慢请求日志

```bash
curl -X DELETE http://localhost:8080/api/v1/perf/slow-logs \
  -H "Authorization: Bearer $TOKEN"
```

---

## 🎯 优化建议

### 获取优化建议

```bash
curl http://localhost:8080/api/v1/perf/analyze/recommendations \
  -H "Authorization: Bearer $TOKEN"
```

响应示例：
```json
{
  "code": 0,
  "data": [
    {
      "priority": 1,
      "category": "performance",
      "title": "优化慢端点响应",
      "description": "端点 GET /api/v1/volumes 平均响应 520ms，建议添加缓存或优化查询",
      "impact": "减少该端点响应时间，提升吞吐量",
      "endpoint": "GET:/api/v1/volumes"
    },
    {
      "priority": 2,
      "category": "resource",
      "title": "优化内存使用",
      "description": "内存使用率达到 85%，建议检查内存泄漏或增加系统内存",
      "impact": "避免 OOM，提高系统稳定性"
    }
  ]
}
```

### 优先级说明

| 优先级 | 说明 |
|--------|------|
| 1 (高) | 建议立即处理 |
| 2 (中) | 建议近期处理 |
| 3 (低) | 可选优化 |

---

## 🚨 异常检测

### 查看异常

```bash
curl http://localhost:8080/api/v1/perf/analyze/anomalies \
  -H "Authorization: Bearer $TOKEN"
```

### 异常类型

| 类型 | 说明 | 检测条件 |
|------|------|----------|
| `latency` | 平均响应时间异常 | 超过基线 2 倍标准差 |
| `latency_p95` | P95 响应时间异常 | 超过基线 2 倍标准差 |
| `error_rate` | 错误率异常 | 超过基线 2 倍标准差 |
| `slow_requests` | 慢请求比例异常 | 比例超过阈值 |
| `endpoint_error_rate` | 端点错误率异常 | 单端点错误率 > 5% |
| `endpoint_latency` | 端点延迟异常 | 超过基线 3 倍标准差 |

---

## 📊 性能基线

### 启用基线学习

性能基线是系统正常运行时的性能特征，用于异常检测。

```yaml
perf:
  enable_baseline: true
  baseline_interval: 5m
```

### 查看基线

```bash
curl http://localhost:8080/api/v1/perf/metrics/baseline \
  -H "Authorization: Bearer $TOKEN"
```

### 重置基线

```bash
curl -X POST http://localhost:8080/api/v1/perf/baseline/reset \
  -H "Authorization: Bearer $TOKEN"
```

---

## 📈 性能对比

### 与基线对比

```bash
curl http://localhost:8080/api/v1/perf/compare \
  -H "Authorization: Bearer $TOKEN"
```

响应示例：
```json
{
  "code": 0,
  "data": {
    "current": {
      "avgLatency": 45.2,
      "errorRate": 0.3,
      "throughput": 125
    },
    "baseline": {
      "avgLatency": 40.0,
      "errorRate": 0.2,
      "throughput": 150
    },
    "change": {
      "avgLatency": "+13%",
      "errorRate": "+50%",
      "throughput": "-17%"
    }
  }
}
```

---

## ⚠️ 最佳实践

### 阈值设置

根据系统规模合理设置阈值：

| 系统规模 | 慢请求阈值 | 错误率阈值 |
|----------|------------|------------|
| 小型 (< 10 用户) | 500ms | 1% |
| 中型 (10-100 用户) | 300ms | 0.5% |
| 大型 (> 100 用户) | 200ms | 0.1% |

### 监控策略

1. **持续监控**: 保持性能监控持续运行
2. **定期分析**: 每周查看性能报告
3. **告警响应**: 及时处理性能告警
4. **基线更新**: 系统升级后重置基线

### 性能优化

1. **缓存优化**: 对高频访问端点添加缓存
2. **查询优化**: 优化数据库查询
3. **资源扩容**: 根据负载增加资源
4. **代码优化**: 修复性能瓶颈代码

---

## 🔧 故障排查

### 性能指标不准确

1. 检查数据采集服务状态
2. 确认数据保留配置正确
3. 查看系统日志

### 健康分数异常

1. 查看具体扣分项
2. 检查异常检测日志
3. 确认基线数据有效

### 慢请求过多

1. 分析慢请求日志
2. 检查系统资源使用
3. 优化相关端点代码

---

## 📚 相关文档

- [WebUI 仪表板使用说明](WEBUI_DASHBOARD_GUIDE.md)
- [API 使用指南](API_GUIDE.md#performance)
- [快照策略配置指南](SNAPSHOT_POLICY_GUIDE.md)

---

**最后更新**: 2026-03-21  
**维护团队**: NAS-OS 吏部