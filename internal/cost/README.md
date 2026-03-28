# Cost 模块

> 版本: v2.90.0 | 户部 - 财务运营

## 概述

`internal/cost` 模块提供 NAS-OS 系统的成本计算和管理功能，支持资源定价、成本追踪、预算管理、成本优化等核心能力。

## 模块结构

```
internal/cost/
├── types.go        # 成本相关类型定义
├── calculator.go   # 成本计算器和定价模型
└── README.md       # 模块说明文档
```

## 核心功能

### 1. 成本类型

支持以下成本类型：
- `cpu` - CPU计算成本
- `memory` - 内存成本
- `storage` - 存储成本
- `network` - 网络流量成本
- `electricity` - 电力成本
- `hardware` - 硬件摊销
- `license` - 许可证费用

### 2. 定价模型

#### 默认定价

| 资源类型 | 单价 | 单位 | 周期 |
|---------|-----|-----|-----|
| CPU | 0.05 CNY | core | hour |
| 内存 | 0.02 CNY | GB | hour |
| SSD存储 | 0.50 CNY | GB | month |
| HDD存储 | 0.15 CNY | GB | month |
| 网络出流量 | 0.80 CNY | GB | 累计 |

#### 阶梯定价

大量使用时可获得折扣：
- CPU: 2-8核心 80%, 8+核心 60%
- 内存: 4-16GB 90%, 16+GB 75%
- 存储: 大量使用可获得额外折扣

### 3. 成本计算

```go
calculator := NewCostCalculator()

// 计算CPU成本
cpuCost := calculator.CalculateCPUCost(cores, hours)

// 计算内存成本
memCost := calculator.CalculateMemoryCost(gb, hours)

// 计算存储成本
storageCost := calculator.CalculateStorageCost(gb, storageType, months)

// 计算网络成本
networkCost := calculator.CalculateNetworkCost(txGB, rxGB)
```

### 4. 预算管理

```go
// 设置预算
budget := &Budget{
    TargetID:         "app-001",
    Limit:            100.0,  // CNY
    Period:           "monthly",
    AlertThreshold1:  70,
    AlertThreshold2:  90,
    AlertThreshold3:  100,
}
calculator.SetBudget(budget)

// 检查预算告警
alerts := calculator.CheckBudgetAlerts(ctx)
```

### 5. 成本优化建议

系统会自动分析资源使用情况，生成优化建议：

- **scale_down**: 资源利用率低，建议降配
- **scale_up**: 资源利用率高，建议扩容
- **optimize**: 存在优化空间，建议优化配置
- **migrate**: 可迁移到更经济的资源
- **terminate**: 未使用资源，建议终止

### 6. 成本报告

```go
request := CostReportRequest{
    Type:              "summary",
    TargetID:          "app-001",
    PeriodStart:       start,
    PeriodEnd:         end,
    IncludeForecast:   true,
    IncludeSuggestions: true,
}

summary, _ := calculator.CalculateTotalCost(ctx, targetID, start, end)
suggestions := calculator.GenerateOptimizationSuggestions(ctx, targetID, start, end)
```

## 与其他模块的集成

### internal/reports 集成

`app_usage.go` 模块采集的资源数据用于成本计算：

```go
// 获取应用资源汇总
summary := usageCollector.GetSummary(appID, start, end)

// 成本估算由 app_usage.go 的 CostEstimate 提供
costEstimate := summary.CostEstimate
```

### 监控系统集成

通过 `internal/monitor` 模块获取实时资源数据：
- CPU使用率
- 内存使用量
- 存储占用
- 网络流量

## API 接口

成本模块提供的 REST API：

```
GET  /api/v1/cost/apps/{app_id}         # 获取应用成本
GET  /api/v1/cost/reports               # 成本报告
GET  /api/v1/cost/suggestions/{app_id}  # 优化建议
GET  /api/v1/cost/trends/{app_id}       # 成本趋势
POST /api/v1/cost/budgets               # 设置预算
GET  /api/v1/cost/alerts                # 获取告警
```

## 配置说明

成本模块配置项：

```yaml
cost:
  # 数据保留天数
  retention_days: 90
  
  # 采集间隔（秒）
  collect_interval: 60
  
  # 预算检查间隔（秒）
  budget_check_interval: 300
  
  # 定价模型配置
  pricing:
    cpu:
      base_rate: 0.05
    memory:
      base_rate: 0.02
    storage:
      ssd_rate: 0.50
      hdd_rate: 0.15
    network:
      outbound_rate: 0.80
```

## 版本历史

| 版本 | 日期 | 变更 |
|-----|-----|-----|
| v2.90.0 | 2024-03-28 | 初始版本 |

---

**户部出品** - 精细化成本管理