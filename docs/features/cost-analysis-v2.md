# 成本分析优化 v2

> 存储成本可视化与预测增强

## 概述

增强成本分析模块，提供详细成本可视化、容量预测、节省建议。

## API设计

### 成本分析

```
GET    /api/v1/cost/analysis                 # 成本概览
GET    /api/v1/cost/by-user                  # 用户成本统计
GET    /api/v1/cost/by-directory             # 目录成本统计
GET    /api/v1/cost/trends                   # 成本趋势
GET    /api/v1/cost/predictions              # 容量预测
GET    /api/v1/cost/savings                  # 节省建议
GET    /api/v1/cost/efficiency               # 效率评分
```

### 成本报告

```
GET    /api/v1/cost/reports/daily            # 日报告
GET    /api/v1/cost/reports/weekly           # 周报告
GET    /api/v1/cost/reports/monthly          # 月报告
POST   /api/v1/cost/reports/export           # 导出报告
```

## 数据模型

```go
type CostAnalysis struct {
    TotalStorage    int64   `json:"total_storage"` // 总容量(bytes)
    UsedStorage     int64   `json:"used_storage"`
    CostPerGB       float64 `json:"cost_per_gb"`   // 每GB成本
    MonthlyCost     float64 `json:"monthly_cost"`
    ProjectedCost   float64 `json:"projected_cost"` // 预计成本
    SavingsOpportunity float64 `json:"savings_opportunity"`
}

type UserCost struct {
    UserID       string  `json:"user_id"`
    StorageUsed  int64   `json:"storage_used"`
    Cost         float64 `json:"cost"`
    Efficiency   float64 `json:"efficiency"` // 0-100评分
}

type CostPrediction struct {
    Date         time.Time `json:"date"`
    PredictedUse int64     `json:"predicted_use"`
    PredictedCost float64  `json:"predicted_cost"`
    Confidence   float64   `json:"confidence"` // 0-1
}

type SavingsSuggestion struct {
    Type        string  `json:"type"` // dedupe, compression, archive
    Description string  `json:"description"`
    Savings     float64 `json:"savings"` // 估算节省
    Impact      string  `json:"impact"` // low, medium, high
}
```

## 功能特性

1. **多维度统计**: 用户、目录、文件类型
2. **趋势预测**: 30天容量趋势预测
3. **效率评分**: 存储效率评分系统
4. **节省建议**: 智能节省建议生成
5. **可视化报告**: 图表化成本报告
6. **导出功能**: Excel/PDF报告导出

## 实现要点

- 基于历史数据的时间序列预测
- 存储效率算法 (去重率、压缩率)
- 成本计算模型 (硬件+电费)