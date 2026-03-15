# 预算管理模块 (Budget)

## 概述

预算管理模块提供完整的预算管理功能，包括：

1. **预算创建和管理** - 支持多种预算类型和周期
2. **预算使用跟踪** - 实时跟踪预算使用情况
3. **预算预警** - 多级预警阈值和通知机制
4. **预算预测** - 基于历史数据的预算预测
5. **预算报告** - 多维度预算分析报告

## 预算类型

| 类型 | 说明 |
|-----|------|
| `storage` | 存储预算 |
| `bandwidth` | 带宽预算 |
| `compute` | 计算预算 |
| `operations` | 运维预算 |
| `total` | 总预算 |

## 预算周期

| 周期 | 说明 |
|-----|------|
| `daily` | 日预算 |
| `weekly` | 周预算 |
| `monthly` | 月预算 |
| `quarter` | 季度预算 |
| `yearly` | 年预算 |

## 预算范围

| 范围 | 说明 |
|-----|------|
| `global` | 全局预算 |
| `user` | 用户预算 |
| `group` | 用户组预算 |
| `volume` | 卷预算 |
| `service` | 服务预算 |
| `directory` | 目录预算 |

## API 接口

### 预算管理

```bash
# 获取预算列表
GET /api/v1/budgets

# 创建预算
POST /api/v1/budgets
{
  "name": "月度存储预算",
  "type": "storage",
  "period": "monthly",
  "scope": "global",
  "amount": 10000.00,
  "auto_reset": true
}

# 获取预算详情
GET /api/v1/budgets/:id

# 更新预算
PUT /api/v1/budgets/:id

# 删除预算
DELETE /api/v1/budgets/:id
```

### 预算使用

```bash
# 记录预算使用
POST /api/v1/budgets/:id/usage
{
  "amount": 100.00,
  "source_type": "storage",
  "description": "存储使用费用"
}

# 获取使用记录
GET /api/v1/budgets/:id/usage

# 获取使用统计
GET /api/v1/budgets/:id/stats
```

### 预算预警

```bash
# 获取预警列表
GET /api/v1/budgets/alerts

# 确认预警
POST /api/v1/budgets/alerts/:id/acknowledge

# 解决预警
POST /api/v1/budgets/alerts/:id/resolve
```

### 预算预测

```bash
# 生成预算预测
POST /api/v1/budgets/:id/forecast
{
  "method": "exponential",
  "period": "monthly",
  "horizon": 3,
  "confidence": "medium"
}

# 获取趋势分析
GET /api/v1/budgets/:id/trend?days=90
```

## 预警配置

### 默认预警阈值

| 级别 | 阈值 | 说明 |
|-----|------|------|
| `info` | 50% | 预算已使用 50% |
| `warning` | 70% | 预算已使用 70%，请注意 |
| `critical` | 85% | 预算已使用 85%，请及时处理 |
| `emergency` | 95% | 预算即将耗尽，请立即处理 |

### 预警升级

预警支持自动升级机制：
- 未处理 30 分钟：升级为 warning
- 未处理 60 分钟：升级为 critical

## 预测方法

| 方法 | 说明 | 适用场景 |
|-----|------|---------|
| `moving_average` | 移动平均 | 数据平稳，无明显趋势 |
| `exponential` | 指数平滑 | 有轻微趋势，需要快速响应 |
| `linear` | 线性回归 | 明显线性趋势 |
| `seasonal` | 季节性预测 | 有周期性波动 |
| `arima` | ARIMA 模型 | 复杂时间序列 |

## 性能优化 (v2.88.0)

### 预测缓存
- 预测结果自动缓存
- 减少重复计算开销
- 提供 `ClearCache()` 方法手动清除缓存
- 通过 `GetCachedForecast()` 获取缓存的预测结果

### 使用示例

```go
// 创建预测引擎
engine := budget.NewForecastEngine(
    historyStore,
    budget.ForecastConfig{
        DefaultMethod:     budget.ForecastMethodExponential,
        DefaultHorizon:    3,
        DefaultConfidence: budget.ConfidenceMedium,
        CacheEnabled:      true,
    },
)

// 生成预测
forecast, err := engine.GenerateForecast(ctx, budget.ForecastRequest{
    BudgetID: "budget-xxx",
    Method:   budget.ForecastMethodExponential,
    Horizon:  3,
})

// 获取缓存的预测
cached, exists := engine.GetCachedForecast("budget-xxx")

// 清除缓存
engine.ClearCache()
```

## 架构说明

```
internal/budget/
├── types.go        # 数据类型定义
├── api.go          # HTTP API 处理器
├── handlers.go     # 请求处理器
├── forecast.go     # 预测引擎
├── forecast_test.go # 预测测试
└── alert.go        # 预警管理
```

### 核心组件

- **ForecastEngine**: 预测引擎，提供多种预测方法
- **AlertManager**: 预警管理器，处理预算预警
- **BudgetDataProvider**: 预算数据提供者接口
- **HistoryStore**: 历史数据存储接口