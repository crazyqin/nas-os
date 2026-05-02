# 预算告警管理指南

> **版本**: v2.482.0 | **适用版本**: NAS-OS v2.477.0 及以上

## 概述

预算告警管理器对标群晖 Storage Analyzer 和 TrueNAS 报表功能，提供存储成本预测、预算管理和超支告警。支持多级预算（总/用户/共享/应用）、三级告警阈值和成本趋势分析。

## 核心特性

- **多级预算**：总存储 / 用户 / 共享 / 应用 / 备份 / 云存储 分类管理
- **三级告警**：正常 → 预警（80%）→ 严重（95%）
- **成本计算**：按每 GB 单价自动计算存储成本
- **趋势分析**：预算使用量趋势追踪
- **多通道通知**：告警通过配置的通知渠道推送

## 预算类别

| 类别 | 说明 | 适用场景 |
|------|------|----------|
| `total` | 总存储预算 | 全局存储容量限制 |
| `user` | 用户预算 | 按用户分配存储配额 |
| `share` | 共享预算 | 按共享目录分配 |
| `app` | 应用预算 | 应用数据存储限制 |
| `backup` | 备份预算 | 备份存储空间限制 |
| `cloud` | 云存储预算 | 云同步存储限制 |

## API 接口

### 创建预算

```bash
curl -X POST http://localhost:8080/api/v1/budgets \
  -H "Content-Type: application/json" \
  -d '{
    "name": "用户存储预算",
    "category": "user",
    "limit_bytes": 107374182400,
    "cost_per_gb": 0.5,
    "alert_at": 80,
    "critical_at": 95,
    "owner": "user1"
  }'
```

### 更新使用量

```bash
curl -X PUT http://localhost:8080/api/v1/budgets/{id}/usage \
  -H "Content-Type: application/json" \
  -d '{"used_bytes": 53687091200}'
```

### 获取预算报告

```bash
curl http://localhost:8080/api/v1/budgets/report
```

### 获取成本汇总

```bash
curl http://localhost:8080/api/v1/budgets/cost-summary
```

### 获取告警列表

```bash
curl http://localhost:8080/api/v1/budgets/alerts
```

## 告警级别

| 级别 | 触发条件 | 颜色 | 说明 |
|------|----------|------|------|
| 正常 | < 80% | 绿色 | 预算使用正常 |
| 预警 | ≥ 80% | 黄色 | 接近预算上限，需关注 |
| 严重 | ≥ 95% | 红色 | 即将超出预算，需立即处理 |

> 阈值可在创建预算时自定义（`alert_at` 和 `critical_at` 参数）

## 默认配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| 检查间隔 | 1 小时 | 预算使用量检查频率 |
| 默认货币 | CNY | 成本计算货币 |
| 告警阈值 | 80% | 触发预警的使用百分比 |
| 严重阈值 | 95% | 触发严重告警的使用百分比 |
| 预算周期 | 月度 | 预算重置周期 |

## 预算周期

| 周期 | 说明 |
|------|------|
| `monthly` | 每月重置（默认） |
| `quarterly` | 每季度重置 |
| `yearly` | 每年重置 |

## 报告示例

```json
[
  {
    "name": "用户存储预算",
    "category": "user",
    "limit_bytes": 107374182400,
    "used_bytes": 85899345920,
    "usage_percent": 80.0,
    "cost_estimate": 40.0,
    "currency": "CNY",
    "alert_level": "warning",
    "owner": "user1"
  }
]
```

## 最佳实践

1. **分层预算**：先设总预算，再按用户/应用细分
2. **合理阈值**：预警设 80%，严重设 95%，留出处理时间
3. **定期审查**：每月检查成本汇总，优化存储使用
4. **关联通知**：配置告警通知渠道，确保及时收到预警
5. **成本核算**：定期更新 `cost_per_gb` 反映实际存储成本

---

## 相关指南

- [智能配额与数据保留](quota-retention-guide.md) — 配额管理与数据生命周期
- [合规仪表盘](compliance-dashboard-guide.md) — 存储合规检查与安全评分
- [监控仪表板](dashboard-guide.md) — 系统整体健康监控
- [分布式监控](distributed-monitoring-guide.md) — 多节点统一监控
