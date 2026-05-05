# 系统资源预测告警 (Resource Prediction)

> **适用版本**: v2.483.0+ | **模块**: `internal/resourcepredict`

Resource Prediction 基于线性回归分析历史资源使用趋势，提前预测资源耗尽时间并分级告警，避免系统意外中断。

## 监控的资源类型

| 资源 | 说明 | 单位 |
|------|------|------|
| **disk** | 磁盘空间使用率 | percent |
| **memory** | 内存使用率 | percent |
| **cpu** | CPU 使用率 | percent |
| **network** | 网络带宽使用 | percent |
| **inode** | inode 使用率 | percent |

## 四级告警体系

| 级别 | 条件 | 默认阈值 | 说明 |
|------|------|----------|------|
| ℹ️ **Info** | 趋势正常 | — | 仅记录，不告警 |
| ⚠️ **Warning** | 预计 N 天内耗尽 | 30 天 | 提前规划 |
| 🔴 **Critical** | 预计 N 天内耗尽 | 14 天 | 需要关注 |
| 🚨 **Urgent** | 预计 N 天内耗尽 | 7 天 | 立即处理 |

## 预测精度

- 使用**线性回归**分析历史数据趋势
- **R² 拟合度评估**：低于阈值（默认 0.5）不输出预测，避免误报
- **置信度评分**：综合 R²、数据量、趋势稳定性给出可信度
- 至少需要 **10 个数据点**才触发预测

## API 接口

所有接口挂载在 `/api/v1/resourcepredict/` 下。

### 记录资源值

```bash
POST /api/v1/resourcepredict/record
Content-Type: application/json

{
  "resourceType": "disk",
  "value": 72.5
}
```

系统会按采样间隔自动记录，也可通过此接口手动上报。

### 获取预测结果

```bash
GET /api/v1/resourcepredict/predictions
```

返回各资源的预测耗尽时间、告警级别和置信度。

### 立即预测

```bash
POST /api/v1/resourcepredict/predict
```

跳过缓存，立即基于最新数据执行预测。

### 查看资源指标

```bash
GET /api/v1/resourcepredict/metrics
```

返回所有资源的历史数据点和当前值。

### 查看配置

```bash
GET /api/v1/resourcepredict/config
```

### 更新告警阈值

```bash
POST /api/v1/resourcepredict/thresholds
Content-Type: application/json

{
  "warningDays": 45,
  "criticalDays": 21,
  "urgentDays": 7,
  "minR2": 0.6
}
```

## 预测输出示例

```json
{
  "status": "ok",
  "data": {
    "disk": {
      "resourceType": "disk",
      "currentUsage": 72.5,
      "predictedDaysLeft": 45,
      "alertLevel": "warning",
      "confidence": 0.82,
      "r2": 0.91,
      "dataPoints": 30
    },
    "memory": {
      "resourceType": "memory",
      "currentUsage": 58.3,
      "predictedDaysLeft": 120,
      "alertLevel": "info",
      "confidence": 0.75,
      "r2": 0.65,
      "dataPoints": 25
    }
  }
}
```

## 最佳实践

1. **告警回调**：注册自定义回调函数，将告警推送到通知系统
2. **阈值调优**：根据业务场景调整告警天数（如数据库服务器可缩短 Critical 阈值）
3. **数据保留**：默认自动清理过期数据点，可通过 `retentionDays` 配置
4. **配合 Smart Tier**：磁盘空间预测 + 智能分层 = 存储自动化

---

*文档版本: v2.483.0 | 最后更新: 2026-05-05*
