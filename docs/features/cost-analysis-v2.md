# 成本分析增强设计

## 功能概述
增强存储成本分析模块，提供更精细的成本统计和预测。

## API设计

### 成本统计API
```
GET  /api/v1/cost/summary                  - 成本总览
GET  /api/v1/cost/by-user                  - 按用户统计
GET  /api/v1/cost/by-directory             - 按目录统计
GET  /api/v1/cost/by-volume                - 按卷统计
GET  /api/v1/cost/trends                   - 成本趋势
GET  /api/v1/cost/predictions              - 成本预测
```

### 成本优化建议API
```
GET  /api/v1/cost/suggestions              - 获取优化建议
POST /api/v1/cost/suggestions/:id/apply    - 应用建议
```

### 效率评分API
```
GET  /api/v1/cost/efficiency               - 效率评分
GET  /api/v1/cost/efficiency/report        - 效率报告
```

## 数据模型

### 成本统计
```go
type CostSummary struct {
    TotalSize          float64   `json:"total_size"` // GB
    TotalCost          float64   `json:"total_cost"` // 元
    StorageCost        float64   `json:"storage_cost"`
    CompressionSavings float64   `json:"compression_savings"`
    DedupSavings       float64   `json:"dedup_savings"`
    EfficiencyScore    float64   `json:"efficiency_score"` // 0-100
    CostPerGB          float64   `json:"cost_per_gb"`
    Trend              string    `json:"trend"` // up, down, stable
}
```

### 用户成本
```go
type UserCost struct {
    UserID       string    `json:"user_id"`
    UsageSize    float64   `json:"usage_size"` // GB
    UsageCost    float64   `json:"usage_cost"`
    QuotaLimit   float64   `json:"quota_limit"`
    QuotaUsed    float64   `json:"quota_used"`
    Efficiency   float64   `json:"efficiency"`
}
```

### 成本预测
```go
type CostPrediction struct {
    Month          string    `json:"month"`
    PredictedSize  float64   `json:"predicted_size"`
    PredictedCost  float64   `json:"predicted_cost"`
    GrowthRate     float64   `json:"growth_rate"` // %/月
    Confidence     float64   `json:"confidence"` // 0-1
}
```

## 成本计算规则

| 存储类型 | 单价 (元/GB/月) | 说明 |
|----------|-----------------|------|
| SSD热数据 | 0.5 | 高性能存储 |
| HDD冷数据 | 0.1 | 大容量存储 |
| 云归档 | 0.05 | 云存储成本 |
| 压缩存储 | -30% | 压缩节省 |

## 优化建议类型

1. **数据迁移**: 将冷数据迁移到低成本存储
2. **压缩启用**: 启用压缩节省空间
3. **去重优化**: 优化去重策略
4. **配额调整**: 调整用户配额
5. **清理建议**: 清理过期/重复数据

## WebUI展示

- 成本仪表板
- 用户成本排行
- 目录成本分布
- 趋势预测图表
- 优化建议列表

## 版本计划

- v2.362.0: API设计完成
- v2.365.0: 用户/目录统计实现
- v2.370.0: 预测和建议实现
- v2.375.0: WebUI集成