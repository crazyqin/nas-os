# 成本分析增强报告

> 户部出品 - v2.90.0 | 2024-03-28

## 一、现状分析

### 1.1 已有模块

| 模块 | 位置 | 功能 | 完善度 |
|------|------|------|--------|
| `internal/cost` | cost/ | 成本计算核心 | ⭐⭐⭐⭐ 基础完善 |
| `internal/billing` | billing/ | 计费管理 | ⭐⭐⭐⭐⭐ 功能完整 |
| `internal/quota` | quota/ | 存储配额 | ⭐⭐⭐⭐ 功能完整 |
| `internal/ai/usage` | ai/usage/ | AI使用量追踪 | ⭐⭐⭐⭐ 功能完整 |
| `internal/reports` | reports/ | 存储成本报告 | ⭐⭐⭐⭐⭐ 报告完善 |

### 1.2 成本计算现状

```go
// 已支持的成本类型
type CostType string
const (
    CostTypeCPU        // CPU成本 ✅
    CostTypeMemory     // 内存成本 ✅
    CostTypeStorage    // 存储成本 ✅
    CostTypeNetwork    // 网络成本 ✅
    CostTypeElectricity // 电力成本 ✅
    CostTypeHardware   // 硬件摊销 ✅
    CostTypeLicense    // 许可证费用 ✅
)
```

### 1.3 定价模型现状

| 资源 | 单价 | 单位 | 周期 | 阶梯定价 |
|------|------|------|------|---------|
| CPU | 0.05 CNY | core | hour | ✅ 2-8核80%, 8+核60% |
| 内存 | 0.02 CNY | GB | hour | ✅ 4-16GB90%, 16+GB75% |
| SSD存储 | 0.50 CNY | GB | month | ✅ |
| HDD存储 | 0.15 CNY | GB | month | ✅ |
| 网络出流量 | 0.80 CNY | GB | 累计 | ❌ 无阶梯 |

## 二、GPU推理成本模型设计

### 2.1 GPU成本类型定义

```go
// internal/cost/types.go 扩展

const (
    // 新增：GPU推理成本
    CostTypeGPUInference CostType = "gpu_inference"  // GPU推理成本
    CostTypeGPUMemory    CostType = "gpu_memory"     // GPU显存成本
    CostTypeGPUCompute   CostType = "gpu_compute"    // GPU算力成本
)

// GPU型号定义
type GPUModel string
const (
    GPUModelRTX4090     GPUModel = "rtx_4090"      // NVIDIA RTX 4090
    GPUModelRTX3090     GPUModel = "rtx_3090"      // NVIDIA RTX 3090
    GPUModelA100        GPUModel = "a100"          // NVIDIA A100
    GPUModelA800        GPUModel = "a800"          // NVIDIA A800 (中国版)
    GPUModelH800        GPUModel = "h800"          // NVIDIA H800 (中国版)
    GPUModelRTX3060     GPUModel = "rtx_3060"      // NVIDIA RTX 3060
)
```

### 2.2 GPU定价模型

```go
// internal/cost/gpu_pricing.go

// GPU定价配置
type GPUPricingConfig struct {
    // GPU型号
    Model GPUModel `json:"model"`
    
    // 显存大小 (GB)
    MemoryGB int `json:"memory_gb"`
    
    // 计算能力 (TFLOPS)
    ComputeTFLOPS float64 `json:"compute_tflops"`
    
    // 小时定价
    HourlyRate float64 `json:"hourly_rate"`  // CNY/小时
    
    // Token定价（推理成本）
    TokenInputRate  float64 `json:"token_input_rate"`   // CNY/千输入Token
    TokenOutputRate float64 `json:"token_output_rate"`  // CNY/千输出Token
    
    // 预留折扣（长期租用）
    ReservedDiscount float64 `json:"reserved_discount"` // 预留折扣百分比
}

// GPU定价参考表 (基于市场调研)
var DefaultGPUPricing = map[GPUModel]GPUPricingConfig{
    GPUModelRTX4090: {
        Model:           GPUModelRTX4090,
        MemoryGB:        24,
        ComputeTFLOPS:   82.6,
        HourlyRate:      15.0,   // 消费级GPU，较便宜
        TokenInputRate:  0.003,  // 约3元/百万输入Token
        TokenOutputRate: 0.006,  // 约6元/百万输出Token
    },
    GPUModelRTX3090: {
        Model:           GPUModelRTX3090,
        MemoryGB:        24,
        ComputeTFLOPS:   35.6,
        HourlyRate:      8.0,
        TokenInputRate:  0.002,
        TokenOutputRate: 0.004,
    },
    GPUModelA100: {
        Model:           GPUModelA100,
        MemoryGB:        80,
        ComputeTFLOPS:   312.0,
        HourlyRate:      50.0,   // 企业级GPU，昂贵
        TokenInputRate:  0.01,
        TokenOutputRate: 0.02,
    },
    GPUModelA800: {
        Model:           GPUModelA800,
        MemoryGB:        80,
        ComputeTFLOPS:   312.0,
        HourlyRate:      35.0,   // 中国特供版，稍便宜
        TokenInputRate:  0.008,
        TokenOutputRate: 0.016,
    },
    GPUModelH800: {
        Model:           GPUModelH800,
        MemoryGB:        80,
        ComputeTFLOPS:   1979.0,
        HourlyRate:      80.0,   // 最新架构，最贵
        TokenInputRate:  0.015,
        TokenOutputRate: 0.03,
    },
    GPUModelRTX3060: {
        Model:           GPUModelRTX3060,
        MemoryGB:        12,
        ComputeTFLOPS:   12.7,
        HourlyRate:      3.0,
        TokenInputRate:  0.001,
        TokenOutputRate: 0.002,
    },
}
```

### 2.3 GPU成本计算器

```go
// internal/cost/gpu_calculator.go

// GPUCostCalculator GPU成本计算器
type GPUCostCalculator struct {
    pricing    map[GPUModel]GPUPricingConfig
    usageStats *GPUUsageStats
}

// CalculateGPUInferenceCost 计算GPU推理成本
func (c *GPUCostCalculator) CalculateGPUInferenceCost(
    gpuModel GPUModel,
    inputTokens int64,
    outputTokens int64,
) *GPUInferenceCost {
    config := c.pricing[gpuModel]
    
    cost := &GPUInferenceCost{
        GPUModel:     gpuModel,
        InputTokens:  inputTokens,
        OutputTokens: outputTokens,
        InputCost:    float64(inputTokens) / 1000.0 * config.TokenInputRate,
        OutputCost:   float64(outputTokens) / 1000.0 * config.TokenOutputRate,
    }
    
    cost.TotalCost = cost.InputCost + cost.OutputCost
    cost.TotalTokens = inputTokens + outputTokens
    cost.CostPerToken = cost.TotalCost / float64(cost.TotalTokens) * 1000
    
    return cost
}

// CalculateGPUComputeCost 计算GPU算力成本（按时计费）
func (c *GPUCostCalculator) CalculateGPUComputeCost(
    gpuModel GPUModel,
    hours float64,
    reserved bool,
) *GPUComputeCost {
    config := c.pricing[gpuModel]
    hourlyRate := config.HourlyRate
    
    // 预留折扣
    if reserved {
        hourlyRate *= (1 - config.ReservedDiscount / 100)
    }
    
    cost := &GPUComputeCost{
        GPUModel:    gpuModel,
        Hours:       hours,
        HourlyRate:  hourlyRate,
        TotalCost:   hours * hourlyRate,
        Reserved:    reserved,
    }
    
    return cost
}

// GPU成本结构
type GPUInferenceCost struct {
    GPUModel     GPUModel `json:"gpu_model"`
    InputTokens  int64    `json:"input_tokens"`
    OutputTokens int64    `json:"output_tokens"`
    TotalTokens  int64    `json:"total_tokens"`
    InputCost    float64  `json:"input_cost"`     // CNY
    OutputCost   float64  `json:"output_cost"`    // CNY
    TotalCost    float64  `json:"total_cost"`     // CNY
    CostPerToken float64  `json:"cost_per_token"` // CNY/千Token
}

type GPUComputeCost struct {
    GPUModel   GPUModel `json:"gpu_model"`
    Hours      float64  `json:"hours"`
    HourlyRate float64  `json:"hourly_rate"`
    TotalCost  float64  `json:"total_cost"`
    Reserved   bool     `json:"reserved"`
}
```

### 2.4 GPU使用量统计

```go
// internal/ai/usage/gpu_tracker.go

// GPUUsageStats GPU使用统计
type GPUUsageStats struct {
    // 累计统计
    TotalInferenceRequests int64   `json:"total_inference_requests"`
    TotalInputTokens       int64   `json:"total_input_tokens"`
    TotalOutputTokens      int64   `json:"total_output_tokens"`
    TotalComputeHours      float64 `json:"total_compute_hours"`
    TotalInferenceCost     float64 `json:"total_inference_cost"`
    TotalComputeCost       float64 `json:"total_compute_cost"`
    
    // 模型分布
    ModelUsage map[string]*ModelGPUUsage `json:"model_usage"`
    
    // GPU型号分布
    GPUUsageByModel map[GPUModel]*GPUModelUsage `json:"gpu_usage_by_model"`
    
    // 时间段分布
    HourlyDistribution map[int]int64 `json:"hourly_distribution"`
    
    // 用户分布
    UserGPUUsage map[string]*UserGPUUsage `json:"user_gpu_usage"`
}

type ModelGPUUsage struct {
    ModelID        string  `json:"model_id"`
    RequestCount   int64   `json:"request_count"`
    InputTokens    int64   `json:"input_tokens"`
    OutputTokens   int64   `json:"output_tokens"`
    ComputeHours   float64 `json:"compute_hours"`
    TotalCost      float64 `json:"total_cost"`
    AvgLatencyMs   int64   `json:"avg_latency_ms"`
}

type GPUModelUsage struct {
    GPUModel         GPUModel `json:"gpu_model"`
    InferenceRequests int64   `json:"inference_requests"`
    ComputeHours     float64  `json:"compute_hours"`
    InputTokens      int64   `json:"input_tokens"`
    OutputTokens     int64   `json:"output_tokens"`
    InferenceCost    float64 `json:"inference_cost"`
    ComputeCost      float64 `json:"compute_cost"`
    MemoryUsedGB     float64 `json:"memory_used_gb"`
}

type UserGPUUsage struct {
    UserID       string  `json:"user_id"`
    UserName     string  `json:"user_name"`
    RequestCount int64   `json:"request_count"`
    TokensUsed   int64   `json:"tokens_used"`
    CostUsed     float64 `json:"cost_used"`
    ComputeHours float64 `json:"compute_hours"`
}
```

## 三、存储成本报告优化

### 3.1 增强指标

```go
// internal/reports/storage_cost.go 增强字段

type StorageCostResult struct {
    // 基础字段（已有）
    VolumeName              string
    TotalGB                 float64
    UsedGB                  float64
    UsagePercent            float64
    
    // 新增：成本细分
    CapacityCostMonthly     float64  // 容量成本
    IOPSCostMonthly         float64  // IOPS成本
    BandwidthCostMonthly    float64  // 带宽成本
    ElectricityCostMonthly  float64  // 电费成本
    OpsCostMonthly          float64  // 运维成本
    DepreciationCostMonthly float64  // 折旧成本
    
    // 新增：效率指标
    CostEfficiencyScore     float64  // 成本效率评分 (0-100)
    WastePercent            float64  // 资源浪费比例
    OptimizationPotential   float64  // 优化潜力百分比
    
    // 新增：趋势预测
    GrowthRateGBPerMonth    float64  // 月增长率 (GB)
    PredictedCostNextMonth  float64  // 下月预测成本
    PredictedFillDays       int      // 预计满载天数
    
    // 新增：分层存储分析
    HotDataGB               float64  // 热数据量
    WarmDataGB              float64  // 温数据量
    ColdDataGB              float64  // 冷数据量
    HotDataCost             float64  // 热数据成本
    ColdDataCost            float64  // 冷数据成本（优化空间）
}
```

### 3.2 存储分层优化

```go
// internal/cost/storage_tiering.go

// StorageTieringAnalyzer 存储分层分析器
type StorageTieringAnalyzer struct {
    hotThreshold   float64  // 热数据阈值（访问次数/天）
    warmThreshold  float64  // 温数据阈值
    pricingHot     float64  // 热存储价格
    pricingWarm    float64  // 温存储价格
    pricingCold    float64  // 冷存储价格
}

// TieringAnalysis 分层分析结果
type TieringAnalysis struct {
    TotalDataGB     float64              `json:"total_data_gb"`
    HotDataGB       float64              `json:"hot_data_gb"`
    WarmDataGB      float64              `json:"warm_data_gb"`
    ColdDataGB      float64              `json:"cold_data_gb"`
    CurrentCost     float64              `json:"current_cost"`
    OptimizedCost   float64              `json:"optimized_cost"`
    PotentialSavings float64             `json:"potential_savings"`
    Recommendations []TieringRecommendation `json:"recommendations"`
}

type TieringRecommendation struct {
    DataType       string  `json:"data_type"`      // hot/warm/cold
    CurrentTier    string  `json:"current_tier"`   // ssd/hdd/archive
    RecommendedTier string `json:"recommended_tier"`
    DataGB         float64 `json:"data_gb"`
    SavingsMonthly float64 `json:"savings_monthly"`
    MigrationCost  float64 `json:"migration_cost"` // 一次性迁移成本
    ROIMonths      int     `json:"roi_months"`     // 投资回报周期
}
```

## 四、资源使用统计完善

### 4.1 综合资源报告

```go
// internal/reports/resource_report.go 增强

type ResourceUsageReport struct {
    // 时间范围
    PeriodStart time.Time `json:"period_start"`
    PeriodEnd   time.Time `json:"period_end"`
    GeneratedAt time.Time `json:"generated_at"`
    
    // CPU资源
    CPUUsage *CPUUsageReport `json:"cpu_usage"`
    
    // 内存资源
    MemoryUsage *MemoryUsageReport `json:"memory_usage"`
    
    // 存储资源
    StorageUsage *StorageUsageReport `json:"storage_usage"`
    
    // 网络资源
    NetworkUsage *NetworkUsageReport `json:"network_usage"`
    
    // GPU资源（新增）
    GPUUsage *GPUUsageReport `json:"gpu_usage"`
    
    // AI服务（新增）
    AIServiceUsage *AIServiceUsageReport `json:"ai_service_usage"`
    
    // 综合成本
    TotalCostMonthly float64 `json:"total_cost_monthly"`
    CostBreakdown    map[string]float64 `json:"cost_breakdown"`
    
    // 效率评分
    EfficiencyScore  float64 `json:"efficiency_score"`
    OptimizationScore float64 `json:"optimization_score"`
}

type GPUUsageReport struct {
    // 汇总
    TotalInferenceRequests int64   `json:"total_inference_requests"`
    TotalTokensUsed        int64   `json:"total_tokens_used"`
    TotalComputeHours      float64 `json:"total_compute_hours"`
    TotalCost              float64 `json:"total_cost"`
    
    // 模型分布
    TopModels []ModelGPUStats `json:"top_models"`
    
    // GPU型号分布
    GPUModels []GPUModelStats `json:"gpu_models"`
    
    // 用户分布
    TopUsers []UserGPUStats `json:"top_users"`
    
    // 时段分布
    HourlyDistribution map[int]int64 `json:"hourly_distribution"`
    
    // 成本趋势
    CostTrend []CostTrendPoint `json:"cost_trend"`
}

type AIServiceUsageReport struct {
    // Token统计
    TotalInputTokens   int64   `json:"total_input_tokens"`
    TotalOutputTokens  int64   `json:"total_output_tokens"`
    TotalTokens        int64   `json:"total_tokens"`
    
    // 请求统计
    TotalRequests      int64   `json:"total_requests"`
    SuccessRequests    int64   `json:"success_requests"`
    FailedRequests     int64   `json:"failed_requests"`
    SuccessRate        float64 `json:"success_rate"`
    
    // 模型使用
    ModelsUsed         []ModelUsageStats `json:"models_used"`
    
    // 成本统计
    TotalCost          float64 `json:"total_cost"`
    CostPerRequest     float64 `json:"cost_per_request"`
    CostPerToken       float64 `json:"cost_per_token"`
    
    // 用户配额
    QuotaUsage         []UserQuotaStats `json:"quota_usage"`
    
    // 性能指标
    AvgLatencyMs       int64   `json:"avg_latency_ms"`
    P95LatencyMs       int64   `json:"p95_latency_ms"`
}
```

### 4.2 用户资源报告

```go
// internal/reports/user_resource_report.go

type UserResourceReport struct {
    UserID      string    `json:"user_id"`
    UserName    string    `json:"user_name"`
    PeriodStart time.Time `json:"period_start"`
    PeriodEnd   time.Time `json:"period_end"`
    
    // 存储使用
    StorageUsedGB     float64 `json:"storage_used_gb"`
    StorageQuotaGB    float64 `json:"storage_quota_gb"`
    StorageUsagePercent float64 `json:"storage_usage_percent"`
    StorageCostMonthly float64 `json:"storage_cost_monthly"`
    
    // AI使用
    AITokensUsed      int64   `json:"ai_tokens_used"`
    AITokenQuota      int64   `json:"ai_token_quota"`
    AIUsagePercent    float64 `json:"ai_usage_percent"`
    AIServiceCost     float64 `json:"ai_service_cost"`
    
    // GPU使用
    GPUComputeHours   float64 `json:"gpu_compute_hours"`
    GPUInferenceTokens int64  `json:"gpu_inference_tokens"`
    GPUCost           float64 `json:"gpu_cost"`
    
    // 网络使用
    NetworkTrafficGB  float64 `json:"network_traffic_gb"`
    NetworkCost       float64 `json:"network_cost"`
    
    // 总成本
    TotalCostMonthly  float64 `json:"total_cost_monthly"`
    CostTrend         []CostTrendPoint `json:"cost_trend"`
    
    // 配额告警
    QuotaAlerts       []QuotaAlert `json:"quota_alerts"`
}
```

## 五、成本优化建议增强

### 5.1 综合优化引擎

```go
// internal/cost/optimizer.go

// CostOptimizer 成本优化引擎
type CostOptimizer struct {
    calculators map[string]CostCalculator
    analyzers   map[string]ResourceAnalyzer
}

// OptimizationReport 优化报告
type OptimizationReport struct {
    GeneratedAt time.Time `json:"generated_at"`
    
    // 当前成本
    CurrentCostMonthly float64 `json:"current_cost_monthly"`
    
    // 优化后成本
    OptimizedCostMonthly float64 `json:"optimized_cost_monthly"`
    
    // 潜在节省
    PotentialSavingsMonthly float64 `json:"potential_savings_monthly"`
    PotentialSavingsYearly float64 `json:"potential_savings_yearly"`
    SavingsPercent float64 `json:"savings_percent"`
    
    // 优化建议分类
    QuickWins []OptimizationAction `json:"quick_wins"`     // 快速见效（ROI < 1月）
    ShortTerm []OptimizationAction `json:"short_term"`     // 短期优化（ROI 1-3月）
    LongTerm  []OptimizationAction `json:"long_term"`      // 长期规划（ROI > 3月）
    
    // 按资源类型
    StorageOptimizations []OptimizationAction `json:"storage_optimizations"`
    AIOptimizations      []OptimizationAction `json:"ai_optimizations"`
    GPUOptimizations     []OptimizationAction `json:"gpu_optimizations"`
    
    // 实施优先级
    PriorityOrder []OptimizationAction `json:"priority_order"`
}

type OptimizationAction struct {
    ID               string   `json:"id"`
    Type             string   `json:"type"`      // scale_down/migrate/optimize/archive
    Resource         string   `json:"resource"`  // storage/ai/gpu
    Target           string   `json:"target"`    // 目标对象
    Title            string   `json:"title"`
    Description      string   `json:"description"`
    CurrentCost      float64  `json:"current_cost"`
    OptimizedCost    float64  `json:"optimized_cost"`
    SavingsMonthly   float64  `json:"savings_monthly"`
    SavingsYearly    float64  `json:"savings_yearly"`
    ROIMonths        int      `json:"roi_months"`
    Implementation   string   `json:"implementation"` // easy/medium/hard
    Risk             string   `json:"risk"`      // low/medium/high
    Steps            []string `json:"steps"`
    Prerequisites    []string `json:"prerequisites"`
    ExpectedBenefits []string `json:"expected_benefits"`
    PotentialRisks   []string `json:"potential_risks"`
}
```

### 5.2 AI成本优化建议

```go
// internal/cost/ai_optimizer.go

// AICostOptimizer AI服务成本优化器
type AICostOptimizer struct {
    usageStats *usage.TokenTracker
    pricing    map[string]*usage.ModelPricing
}

// GenerateAIOptimizations 生成AI优化建议
func (o *AICostOptimizer) GenerateAIOptimizations(ctx context.Context) []OptimizationAction {
    var actions []OptimizationAction
    
    // 1. 模型选择优化
    actions = append(actions, o.analyzeModelSelection(ctx)...)
    
    // 2. Token使用优化
    actions = append(actions, o.analyzeTokenUsage(ctx)...)
    
    // 3. 用户配额优化
    actions = append(actions, o.analyzeQuotaUsage(ctx)...)
    
    // 4. 批量处理建议
    actions = append(actions, o.analyzeBatchProcessing(ctx)...)
    
    return actions
}

// 模型选择优化：建议使用更经济的模型
func (o *AICostOptimizer) analyzeModelSelection(ctx context.Context) []OptimizationAction {
    var actions []OptimizationAction
    
    // 分析各模型使用情况
    for modelID, stats := range o.usageStats.GetModelStats() {
        // 如果使用了昂贵模型，但任务简单
        if stats.AvgInputTokens < 500 && o.pricing[modelID].InputPricePer1K > 0.01 {
            // 建议切换到更经济的模型
            cheaperModel := o.findCheaperAlternative(modelID)
            if cheaperModel != "" {
                savings := o.calculateModelSwitchSavings(modelID, cheaperModel, stats)
                actions = append(actions, OptimizationAction{
                    Type:           "model_switch",
                    Resource:       "ai",
                    Target:         modelID,
                    Title:          fmt.Sprintf("切换到更经济的模型 %s", cheaperModel),
                    SavingsMonthly: savings,
                    Implementation: "easy",
                    Risk:           "low",
                })
            }
        }
    }
    
    return actions
}
```

## 六、总结

### 6.1 成本分析增强要点

1. **GPU推理成本计算**
   - 新增 GPUInferenceCost、GPUComputeCost 类型
   - 支持多种 GPU 型号定价（RTX4090、A100、H800等）
   - Token 计费和按时计费双模式

2. **存储成本报告优化**
   - 成本细分：容量、IOPS、带宽、电费、运维、折旧
   - 分层存储分析：热/温/冷数据成本对比
   - 趋势预测：增长率、预测成本、满载天数

3. **资源使用统计完善**
   - 综合资源报告：CPU、内存、存储、网络、GPU、AI
   - 用户资源报告：个人维度成本统计
   - AI服务统计：Token、请求、模型、配额

4. **成本优化建议增强**
   - 综合优化引擎：跨资源类型分析
   - AI成本优化：模型选择、Token使用、批量处理
   - ROI分析：快速见效、短期、长期分类

### 6.2 成本定价参考

| GPU型号 | 显存 | 小时价格 | 输入Token价格 | 输出Token价格 |
|---------|------|----------|--------------|--------------|
| RTX 4090 | 24GB | ¥15/h | ¥3/百万 | ¥6/百万 |
| RTX 3090 | 24GB | ¥8/h | ¥2/百万 | ¥4/百万 |
| A800 | 80GB | ¥35/h | ¥8/百万 | ¥16/百万 |
| H800 | 80GB | ¥80/h | ¥15/百万 | ¥30/百万 |

---

**户部出品** - 精细化成本管理，助力降本增效