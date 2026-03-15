# 成本分析模块 (Cost Analysis)

## 概述

成本分析模块提供完整的成本分析报告功能，包括：

1. **存储成本趋势分析** - 分析存储使用趋势和成本变化
2. **资源利用率报告** - 监控存储和带宽资源利用率
3. **成本优化建议** - 自动生成成本优化建议
4. **预算跟踪报告** - 预算使用情况跟踪和预警
5. **综合成本分析报告** - 多维度综合分析

## 报告类型

| 报告类型 | 说明 | API 端点 |
|---------|------|---------|
| `storage_trend` | 存储成本趋势分析 | `/api/cost/reports/storage-trend` |
| `resource_util` | 资源利用率报告 | `/api/cost/reports/resource-utilization` |
| `optimization` | 成本优化建议报告 | `/api/cost/reports/optimization` |
| `budget_tracking` | 预算跟踪报告 | `/api/cost/reports/budget-tracking` |
| `comprehensive` | 综合成本分析报告 | `/api/cost/reports/comprehensive` |

## API 接口

### 报告生成

```bash
# 生成存储成本趋势报告（最近30天）
GET /api/cost/reports/storage-trend?days=30

# 生成资源利用率报告
GET /api/cost/reports/resource-utilization

# 生成成本优化建议报告
GET /api/cost/reports/optimization

# 生成预算跟踪报告
GET /api/cost/reports/budget-tracking?budget_id=budget-xxx

# 生成综合成本分析报告
GET /api/cost/reports/comprehensive
```

### 预算管理

```bash
# 获取预算列表
GET /api/cost/budgets

# 创建预算
POST /api/cost/budgets
{
  "name": "月度存储预算",
  "total_budget": 10000,
  "period": "monthly",
  "start_date": "2026-03-01T00:00:00Z",
  "end_date": "2026-03-31T23:59:59Z"
}

# 获取预算详情
GET /api/cost/budgets/:id

# 更新预算
PUT /api/cost/budgets/:id

# 删除预算
DELETE /api/cost/budgets/:id
```

### 告警管理

```bash
# 获取成本告警列表
GET /api/cost/alerts

# 确认告警
POST /api/cost/alerts/:id
{
  "action": "acknowledge"
}
```

## 性能优化 (v2.88.0)

### 缓存机制
- 成本分析结果自动缓存 5 分钟
- 减少 API 调用开销，提高响应速度
- 提供 `ClearCache()` 方法手动清除缓存
- 通过 `GetCacheStatus()` 查询缓存状态

### 使用示例

```go
// 创建成本分析引擎
engine := cost_analysis.NewCostAnalysisEngine(
    "/var/lib/nas-os/cost",
    billingProvider,
    quotaProvider,
    cost_analysis.DefaultAnalysisConfig(),
)

// 生成存储成本趋势报告
report, err := engine.GenerateStorageTrendReport(30)

// 清除缓存（在数据更新后）
engine.ClearCache()

// 查询缓存状态
valid, cacheTime := engine.GetCacheStatus()
```

## 数据结构

### 成本报告 (CostReport)

```json
{
  "id": "rpt-xxx",
  "type": "storage_trend",
  "generated_at": "2026-03-16T00:00:00Z",
  "period_start": "2026-02-14T00:00:00Z",
  "period_end": "2026-03-16T00:00:00Z",
  "summary": {
    "total_cost": 1500.00,
    "storage_cost": 1200.00,
    "bandwidth_cost": 300.00,
    "currency": "CNY",
    "cost_change_percent": 5.2
  },
  "recommendations": [
    {
      "id": "rec-xxx",
      "type": "storage",
      "priority": "high",
      "title": "存储池 data 容量紧张",
      "description": "存储池使用率已达 85%",
      "potential_savings": 0,
      "action": "评估扩容需求或清理无用数据"
    }
  ]
}
```

### 预算配置 (BudgetConfig)

```json
{
  "id": "budget-xxx",
  "name": "月度存储预算",
  "total_budget": 10000.00,
  "period": "monthly",
  "start_date": "2026-03-01T00:00:00Z",
  "end_date": "2026-03-31T23:59:59Z",
  "alert_thresholds": [50, 75, 90, 100],
  "enabled": true
}
```

## 架构说明

```
internal/billing/cost_analysis/
├── report.go            # 成本分析引擎核心
├── enhanced_report.go   # 增强版报告功能
├── api.go              # HTTP API 处理器
├── report_test.go      # 测试文件
├── enhanced_report_test.go
└── api_test.go
```

### 核心组件

- **CostAnalysisEngine**: 成本分析引擎，负责报告生成、预算管理、趋势分析
- **APIHandler**: HTTP API 处理器，提供 RESTful 接口
- **BillingDataProvider**: 计费数据提供者接口
- **QuotaDataProvider**: 配额数据提供者接口

### 数据流

```
HTTP 请求 → APIHandler → CostAnalysisEngine
                              ↓
                    BillingDataProvider (获取计费数据)
                              ↓
                    QuotaDataProvider (获取配额使用)
                              ↓
                    分析计算 → 生成报告 → 返回结果
```