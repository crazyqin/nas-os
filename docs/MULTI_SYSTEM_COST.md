# 户部：多系统成本分析扩展

## 概述
扩展成本分析功能，支持多节点成本聚合与对比。

## 功能设计

### 1. 多节点成本聚合

```go
// MultiSystemCostManager 多系统成本管理
type MultiSystemCostManager struct {
    nodeCosts map[string]*NodeCostTracker
    aggregator *CostAggregator
    predictor  *CostPredictor
}

// NodeCostTracker 单节点成本追踪
type NodeCostTracker struct {
    NodeID       string
    StorageCost  float64 // 存储成本（按容量）
    ComputeCost  float64 // 计算成本（CPU/内存）
    NetworkCost  float64 // 网络成本（带宽）
    MaintenanceCost float64 // 维护成本
    TotalCost    float64
    CostPerGB    float64 // 每GB成本
}

// CostAggregator 成本聚合器
type CostAggregator struct {
    nodes      NodeRegistry
    cache      *CostCache
}

// AggregateCosts 聚合所有节点成本
func (a *CostAggregator) AggregateCosts(ctx context.Context) (*AggregatedCostReport, error) {
    report := &AggregatedCostReport{
        Timestamp: time.Now(),
        Nodes:     []NodeCostTracker{},
    }
    
    for _, node := range a.nodes.GetAll() {
        cost := a.FetchNodeCost(node.ID)
        report.Nodes = append(report.Nodes, cost)
        report.TotalCost += cost.TotalCost
        report.TotalStorage += cost.StorageCapacity
    }
    
    report.AverageCostPerGB = report.TotalCost / report.TotalStorage
    
    return report, nil
}

// AggregatedCostReport 聚合成本报告
type AggregatedCostReport struct {
    Timestamp        time.Time
    Nodes            []NodeCostTracker
    TotalCost        float64
    TotalStorage     float64
    AverageCostPerGB float64
    CostByType       map[string]float64 // 存储/计算/网络
    Recommendations  []CostRecommendation
}
```

### 2. 成本对比报告

```go
// CostComparator 成本对比器
type CostComparator struct {
    aggregator *CostAggregator
}

// CompareNodes 对比节点成本
func (c *CostComparator) CompareNodes(ctx context.Context, nodes []string) (*CostComparisonReport, error) {
    report := &CostComparisonReport{
        Nodes: nodes,
        Comparisons: []NodeComparison{},
    }
    
    for _, nodeID := range nodes {
        cost := c.aggregator.FetchNodeCost(nodeID)
        report.Comparisons = append(report.Comparisons, NodeComparison{
            NodeID: nodeID,
            Cost: cost,
            Efficiency: c.CalculateEfficiency(cost),
        })
    }
    
    // 排序找出最高/最低成本节点
    report.HighCostNode = c.FindHighestCost(report.Comparisons)
    report.LowCostNode = c.FindLowestCost(report.Comparisons)
    
    return report, nil
}

// CostComparisonReport 成本对比报告
type CostComparisonReport struct {
    Nodes         []string
    Comparisons   []NodeComparison
    HighCostNode  string
    LowCostNode   string
    AverageCost   float64
    Variance      float64 // 成本差异系数
}

// NodeComparison 节点对比
type NodeComparison struct {
    NodeID     string
    Cost       NodeCostTracker
    Efficiency float64 // 成本效率评分
}
```

### 3. 成本趋势预测

```go
// CostPredictor 成本预测器
type CostPredictor struct {
    history    *CostHistory
    model      *PredictionModel
}

// PredictCost 预测未来成本
func (p *CostPredictor) PredictCost(ctx context.Context, horizon PredictHorizon) (*CostPrediction, error) {
    // 1. 获取历史成本数据
    history := p.history.GetHistory(horizon.BaseDate, 90) // 90天历史
    
    // 2. 分析增长趋势
    trend := p.model.AnalyzeTrend(history)
    
    // 3. 预测未来成本
    prediction := &CostPrediction{
        Horizon: horizon,
        PredictedCost: p.model.Predict(history, horizon.Days),
        Confidence: p.model.CalculateConfidence(history),
        Trend: trend,
    }
    
    // 4. 考虑扩容计划
    if horizon.ExpansionPlan != nil {
        prediction = p.AdjustForExpansion(prediction, horizon.ExpansionPlan)
    }
    
    return prediction, nil
}

// CostPrediction 成本预测结果
type CostPrediction struct {
    Horizon        PredictHorizon
    PredictedCost  float64
    Confidence     float64 // 预测置信度
    Trend          CostTrend
    MonthlyBreakdown []MonthlyPrediction
    ExpansionImpact  *ExpansionImpact
}

// PredictHorizon 预测范围
type PredictHorizon struct {
    BaseDate      time.Time
    Days          int
    ExpansionPlan *ExpansionPlan // 可选：扩容计划
}

// CostTrend 成本趋势
type CostTrend struct {
    Direction      string // increasing, stable, decreasing
    Rate           float64 // 月增长率
    ProjectedDate  time.Time // 预计达到阈值日期
}
```

### 4. 成本优化建议

```go
// CostOptimizer 成本优化器
type CostOptimizer struct {
    comparator *CostComparator
    predictor  *CostPredictor
}

// GenerateRecommendations 生成优化建议
func (o *CostOptimizer) GenerateRecommendations(ctx context.Context) ([]CostRecommendation, error) {
    recommendations := []CostRecommendation{}
    
    // 1. 识别高成本节点
    highCostNodes := o.IdentifyHighCostNodes()
    
    // 2. 分析原因
    for _, node := range highCostNodes {
        reasons := o.AnalyzeHighCostReasons(node)
        
        for _, reason := range reasons {
            recommendations = append(recommendations, CostRecommendation{
                Type: reason.Type,
                Node: node.NodeID,
                Description: reason.Description,
                PotentialSaving: reason.Saving,
                Priority: reason.Priority,
                Actions: o.GenerateActions(reason),
            })
        }
    }
    
    // 3. 数据迁移建议
    migrationOps := o.SuggestMigration()
    recommendations = append(recommendations, migrationOps...)
    
    return recommendations, nil
}

// CostRecommendation 成本建议
type CostRecommendation struct {
    Type           string // storage, compute, network, migration
    Node           string
    Description    string
    PotentialSaving float64
    Priority       string // P0, P1, P2
    Actions        []RecommendedAction
    Impact         string // 实施影响
}

// RecommendedAction 建议操作
type RecommendedAction struct {
    Action       string
    EstimatedCost float64
    EstimatedSaving float64
    RiskLevel    string
}
```

## 报告模板

### 月度成本报告

```markdown
# NAS-OS 多系统月度成本报告

## 报告周期: 2026-04-01 至 2026-04-30

## 总体概况
| 指标 | 数值 | 变化 |
|------|------|------|
| 总成本 | ¥1,250 | +5% |
| 总容量 | 12TB | +2TB |
| 平均成本/GB | ¥0.10 | -3% |

## 节点成本对比
| 节点 | 存储成本 | 计算成本 | 网络成本 | 总成本 | 成本效率 |
|------|----------|----------|----------|--------|----------|
| Node A | ¥300 | ¥50 | ¥20 | ¥370 | 85% |
| Node B | ¥250 | ¥30 | ¥15 | ¥295 | 92% |
| Node C | ¥200 | ¥40 | ¥25 | ¥265 | 95% |

## 成本趋势预测
- 未来30天预计成本: ¥1,300
- 月增长率: 3%
- 建议: 考虑数据迁移至Node C以降低成本

## 优化建议
1. [P0] Node A存储效率低，建议迁移冷数据至Node C
2. [P1] 网络带宽优化，减少跨节点复制频率
3. [P2] Node B增加缓存层，提升计算效率
```

## API端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/api/v1/cost/aggregated` | GET | 获取聚合成本报告 |
| `/api/v1/cost/compare` | GET | 节点成本对比 |
| `/api/v1/cost/predict` | GET | 成本预测 |
| `/api/v1/cost/recommendations` | GET | 优化建议 |
| `/api/v1/cost/history` | GET | 历史成本数据 |

## 实现计划

| 阶段 | 任务 | 时间 |
|------|------|------|
| M1 | MultiSystemCostManager基础 | 04-03 |
| M2 | CostComparator对比报告 | 04-05 |
| M3 | CostPredictor预测模型 | 04-08 |
| M4 | CostOptimizer建议生成 | 04-10 |
| M5 | 月度报告模板与API | 04-15 |

## 与现有系统集成

- 扩展 `internal/cost/` 现有实现
- 利用 `internal/storage/` 容量数据
- 集成 `internal/cluster/` 节点管理