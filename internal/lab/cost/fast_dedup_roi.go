// Package cost - ZFS Fast Dedup ROI计算器
// TrueNAS宣称Fast Dedup可减少90%内存需求
// 本模块计算Fast Dedup的成本效益与ROI
package cost

import (
	"fmt"
	"math"
	"time"
)

// ========== Fast Dedup配置与数据结构 ==========

// FastDedupConfig Fast Dedup配置参数.
type FastDedupConfig struct {
	// 传统DDT条目内存占用（字节）- 约320字节/条目
	TraditionalDDTEntryBytes uint64 `json:"traditional_ddt_entry_bytes"`

	// Fast Dedup DDT条目内存占用（字节）- 约32字节/条目（压缩后）
	FastDedupDDTEntryBytes uint64 `json:"fast_dedup_ddt_entry_bytes"`

	// 内存节省率宣称（TrueNAS宣称90%）
	MemorySavingClaim float64 `json:"memory_saving_claim"`

	// SSD缓存成本（元/GB/月）- Fast Dedup需要SSD缓存DDT
	SSDCacheCostPerGBMonthly float64 `json:"ssd_cache_cost_per_gb_monthly"`

	// 内存成本（元/GB/月）
	MemoryCostPerGBMonthly float64 `json:"memory_cost_per_gb_monthly"`

	// 数据总量（字节）
	TotalDataBytes uint64 `json:"total_data_bytes"`

	// 平均块大小（字节）- ZFS默认128KB
	AvgBlockSizeBytes uint64 `json:"avg_block_size_bytes"`

	// 预期去重率（%）
	ExpectedDedupRate float64 `json:"expected_dedup_rate"`

	// SSD缓存命中率（%）- 影响性能
	SSDCacheHitRate float64 `json:"ssd_cache_hit_rate"`

	// 性能损耗系数 - Fast Dedup相比传统去重的性能影响
	PerformanceImpactFactor float64 `json:"performance_impact_factor"`

	// 运维复杂度系数 - Fast Dedup需要更多配置和维护
	OpsComplexityFactor float64 `json:"ops_complexity_factor"`
}

// DefaultFastDedupConfig 默认Fast Dedup配置.
func DefaultFastDedupConfig() FastDedupConfig {
	return FastDedupConfig{
		TraditionalDDTEntryBytes: 320,                            // 传统ZFS DDT条目大小
		FastDedupDDTEntryBytes:   32,                             // Fast Dedup压缩后的条目大小
		MemorySavingClaim:        90.0,                           // TrueNAS宣称90%内存节省
		SSDCacheCostPerGBMonthly: 0.08,                           // SSD缓存成本
		MemoryCostPerGBMonthly:   0.15,                           // 内存成本
		TotalDataBytes:           10 * 1024 * 1024 * 1024 * 1024, // 10TB
		AvgBlockSizeBytes:        128 * 1024,                     // 128KB块大小
		ExpectedDedupRate:        30.0,                           // 30%去重率
		SSDCacheHitRate:          85.0,                           // SSD缓存命中率85%
		PerformanceImpactFactor:  0.05,                           // 5%性能损耗
		OpsComplexityFactor:      0.15,                           // 15%运维复杂度增加
	}
}

// FastDedupAnalysis Fast Dedup分析结果.
type FastDedupAnalysis struct {
	// 分析ID
	ID string `json:"id"`

	// 分析时间
	AnalysisTime time.Time `json:"analysis_time"`

	// 配置参数
	Config FastDedupConfig `json:"config"`

	// ========== 传统去重成本 ==========

	// 传统DDT内存需求（GB）
	TraditionalMemoryGB float64 `json:"traditional_memory_gb"`

	// 传统DDT内存成本（元/月）
	TraditionalMemoryCost float64 `json:"traditional_memory_cost"`

	// 传统DDT条目数
	TraditionalDDTEntries uint64 `json:"traditional_ddt_entries"`

	// ========== Fast Dedup成本 ==========

	// Fast Dedup内存需求（GB）
	FastDedupMemoryGB float64 `json:"fast_dedup_memory_gb"`

	// Fast Dedup内存成本（元/月）
	FastDedupMemoryCost float64 `json:"fast_dedup_memory_cost"`

	// Fast Dedup SSD缓存需求（GB）
	FastDedupSSDCacheGB float64 `json:"fast_dedup_ssd_cache_gb"`

	// Fast Dedup SSD缓存成本（元/月）
	FastDedupSSDCacheCost float64 `json:"fast_dedup_ssd_cache_cost"`

	// Fast Dedup总成本（元/月）
	FastDedupTotalCost float64 `json:"fast_dedup_total_cost"`

	// ========== 内存节省效果 ==========

	// 实际内存节省（GB）
	ActualMemorySavedGB float64 `json:"actual_memory_saved_gb"`

	// 内存节省率（%）
	ActualMemorySavingRate float64 `json:"actual_memory_saving_rate"`

	// 与宣称节省率差异
	MemorySavingDifference float64 `json:"memory_saving_difference"`

	// 内存节省验证状态
	MemorySavingVerified string `json:"memory_saving_verified"` // verified/close/underperform

	// ========== 成本收益对比 ==========

	// 成本节省（元/月）
	CostSavedMonthly float64 `json:"cost_saved_monthly"`

	// SSD成本增量（元/月）
	SSDCostIncrease float64 `json:"ssd_cost_increase"`

	// 净成本节省（元/月）
	NetCostSavedMonthly float64 `json:"net_cost_saved_monthly"`

	// 成本节省率（%）
	CostSavingRate float64 `json:"cost_saving_rate"`

	// ========== ROI指标 ==========

	// Fast Dedup ROI（%）
	FastDedupROI float64 `json:"fast_dedup_roi"`

	// 投资回收期（月）
	PaybackMonths int `json:"payback_months"`

	// 效益评分（0-100）
	BenefitScore float64 `json:"benefit_score"`

	// ========== 性能影响 ==========

	// 性能损耗（%）
	PerformanceLoss float64 `json:"performance_loss"`

	// SSD缓存性能补偿（%）
	SSDCachePerformanceGain float64 `json:"ssd_cache_performance_gain"`

	// 综合性能影响（%）
	NetPerformanceImpact float64 `json:"net_performance_impact"`

	// ========== 建议与风险 ==========

	// 是否推荐使用Fast Dedup
	RecommendFastDedup bool `json:"recommend_fast_dedup"`

	// 推荐场景
	RecommendedScenarios []string `json:"recommended_scenarios"`

	// 不推荐场景
	NotRecommendedScenarios []string `json:"not_recommended_scenarios"`

	// 风险提示
	Risks []string `json:"risks"`

	// 优化建议
	Suggestions []string `json:"suggestions"`

	// 成本明细
	CostBreakdown map[string]float64 `json:"cost_breakdown"`
}

// FastDedupScenarioResult 场景分析结果.
type FastDedupScenarioResult struct {
	// 场景名称
	Scenario string `json:"scenario"`

	// 数据量（TB）
	DataSizeTB float64 `json:"data_size_tb"`

	// 去重率（%）
	DedupRate float64 `json:"dedup_rate"`

	// 传统内存需求（GB）
	TraditionalMemoryGB float64 `json:"traditional_memory_gb"`

	// Fast Dedup内存需求（GB）
	FastDedupMemoryGB float64 `json:"fast_dedup_memory_gb"`

	// 内存节省率（%）
	MemorySavingRate float64 `json:"memory_saving_rate"`

	// 传统总成本（元/月）
	TraditionalCost float64 `json:"traditional_cost"`

	// Fast Dedup总成本（元/月）
	FastDedupCost float64 `json:"fast_dedup_cost"`

	// 成本节省（元/月）
	CostSaved float64 `json:"cost_saved"`

	// ROI（%）
	ROI float64 `json:"roi"`

	// 推荐状态
	Recommendation string `json:"recommendation"`
}

// FastDedupComparison Fast Dedup与传统去重对比.
type FastDedupComparison struct {
	// 各场景对比结果
	Scenarios []FastDedupScenarioResult `json:"scenarios"`

	// 最佳使用场景
	BestScenario string `json:"best_scenario"`

	// 内存节省曲线
	MemorySavingCurve []MemorySavingPoint `json:"memory_saving_curve"`

	// 成本效益曲线
	CostBenefitCurve []CostBenefitPoint `json:"cost_benefit_curve"`

	// 推荐启用阈值
	EnableThreshold struct {
		MinDataSizeTB     float64 `json:"min_data_size_tb"`
		MinDedupRate      float64 `json:"min_dedup_rate"`
		MaxMemoryBudgetGB float64 `json:"max_memory_budget_gb"`
	} `json:"enable_threshold"`

	// 总体建议
	OverallRecommendation string `json:"overall_recommendation"`
}

// MemorySavingPoint 内存节省曲线数据点.
type MemorySavingPoint struct {
	// 数据量（TB）
	DataSizeTB float64 `json:"data_size_tb"`

	// 传统内存需求（GB）
	TraditionalMemoryGB float64 `json:"traditional_memory_gb"`

	// Fast Dedup内存需求（GB）
	FastDedupMemoryGB float64 `json:"fast_dedup_memory_gb"`

	// 节省率（%）
	SavingRate float64 `json:"saving_rate"`

	// 是否达到宣称效果
	MeetsClaim bool `json:"meets_claim"`
}

// ========== Fast Dedup ROI计算器 ==========

// FastDedupROICalculator Fast Dedup ROI计算器.
type FastDedupROICalculator struct {
	config FastDedupConfig
}

// NewFastDedupROICalculator 创建Fast Dedup ROI计算器.
func NewFastDedupROICalculator(config FastDedupConfig) *FastDedupROICalculator {
	return &FastDedupROICalculator{config: config}
}

// Analyze 执行Fast Dedup分析.
func (c *FastDedupROICalculator) Analyze() *FastDedupAnalysis {
	now := time.Now()
	analysis := &FastDedupAnalysis{
		ID:                      fmt.Sprintf("fast_dedup_analysis_%d", now.Unix()),
		AnalysisTime:            now,
		Config:                  c.config,
		Risks:                   make([]string, 0),
		Suggestions:             make([]string, 0),
		CostBreakdown:           make(map[string]float64),
		RecommendedScenarios:    make([]string, 0),
		NotRecommendedScenarios: make([]string, 0),
	}

	// ========== 计算DDT条目数 ==========

	// 计算唯一块数量
	totalBlocks := c.config.TotalDataBytes / c.config.AvgBlockSizeBytes
	uniqueBlocks := totalBlocks * uint64(100-c.config.ExpectedDedupRate) / 100

	// ========== 计算传统去重成本 ==========

	// 传统DDT内存需求
	traditionalDDTBytes := uniqueBlocks * c.config.TraditionalDDTEntryBytes
	analysis.TraditionalMemoryGB = float64(traditionalDDTBytes) / (1024 * 1024 * 1024)
	analysis.TraditionalDDTEntries = uniqueBlocks

	// 传统DDT内存成本
	analysis.TraditionalMemoryCost = analysis.TraditionalMemoryGB * c.config.MemoryCostPerGBMonthly
	analysis.CostBreakdown["traditional_memory"] = analysis.TraditionalMemoryCost

	// ========== 计算Fast Dedup成本 ==========

	// Fast Dedup内存需求（压缩后的DDT）
	fastDedupDDTBytes := uniqueBlocks * c.config.FastDedupDDTEntryBytes
	analysis.FastDedupMemoryGB = float64(fastDedupDDTBytes) / (1024 * 1024 * 1024)

	// Fast Dedup内存成本
	analysis.FastDedupMemoryCost = analysis.FastDedupMemoryGB * c.config.MemoryCostPerGBMonthly
	analysis.CostBreakdown["fast_dedup_memory"] = analysis.FastDedupMemoryCost

	// Fast Dedup SSD缓存需求（需要存储完整DDT在SSD）
	// SSD缓存大小 = 传统DDT大小的50%（部分条目）
	analysis.FastDedupSSDCacheGB = analysis.TraditionalMemoryGB * 0.5

	// Fast Dedup SSD缓存成本
	analysis.FastDedupSSDCacheCost = analysis.FastDedupSSDCacheGB * c.config.SSDCacheCostPerGBMonthly
	analysis.CostBreakdown["fast_dedup_ssd_cache"] = analysis.FastDedupSSDCacheCost

	// Fast Dedup总成本
	analysis.FastDedupTotalCost = analysis.FastDedupMemoryCost + analysis.FastDedupSSDCacheCost
	analysis.CostBreakdown["fast_dedup_total"] = analysis.FastDedupTotalCost

	// ========== 计算内存节省效果 ==========

	// 实际内存节省
	analysis.ActualMemorySavedGB = analysis.TraditionalMemoryGB - analysis.FastDedupMemoryGB

	// 内存节省率
	if analysis.TraditionalMemoryGB > 0 {
		analysis.ActualMemorySavingRate = (analysis.ActualMemorySavedGB / analysis.TraditionalMemoryGB) * 100
	}

	// 与宣称节省率差异
	analysis.MemorySavingDifference = analysis.ActualMemorySavingRate - c.config.MemorySavingClaim

	// 验证内存节省效果
	if analysis.ActualMemorySavingRate >= c.config.MemorySavingClaim {
		analysis.MemorySavingVerified = "verified"
	} else if analysis.ActualMemorySavingRate >= c.config.MemorySavingClaim*0.8 {
		analysis.MemorySavingVerified = "close"
	} else {
		analysis.MemorySavingVerified = "underperform"
	}

	// ========== 计算成本收益 ==========

	// 内存成本节省
	analysis.CostSavedMonthly = analysis.TraditionalMemoryCost - analysis.FastDedupMemoryCost

	// SSD成本增量
	analysis.SSDCostIncrease = analysis.FastDedupSSDCacheCost

	// 净成本节省
	analysis.NetCostSavedMonthly = analysis.CostSavedMonthly - analysis.SSDCostIncrease

	// 成本节省率
	if analysis.TraditionalMemoryCost > 0 {
		analysis.CostSavingRate = (analysis.NetCostSavedMonthly / analysis.TraditionalMemoryCost) * 100
	}

	// ========== 计算ROI ==========

	// Fast Dedup ROI = 净节省 / SSD投资成本
	ssdInvestment := analysis.FastDedupSSDCacheGB * c.config.SSDCacheCostPerGBMonthly * 12 // 年化SSD成本
	if ssdInvestment > 0 {
		analysis.FastDedupROI = (analysis.NetCostSavedMonthly * 12 / ssdInvestment) * 100
	}

	// 投资回收期
	if analysis.NetCostSavedMonthly > 0 {
		analysis.PaybackMonths = int(math.Ceil(ssdInvestment / analysis.NetCostSavedMonthly))
	} else {
		analysis.PaybackMonths = -1 // 不回收
	}

	// 效益评分
	analysis.BenefitScore = c.calculateBenefitScore(analysis)

	// ========== 计算性能影响 ==========

	// 基础性能损耗（Fast Dedup的间接引用开销）
	analysis.PerformanceLoss = c.config.PerformanceImpactFactor * 100

	// SSD缓存性能补偿（SSD加速DDT访问）
	analysis.SSDCachePerformanceGain = c.config.SSDCacheHitRate * 0.02 // 假设每1%命中率带来0.02%性能提升

	// 综合性能影响
	analysis.NetPerformanceImpact = analysis.PerformanceLoss - analysis.SSDCachePerformanceGain

	// ========== 生成建议 ==========

	analysis.RecommendFastDedup = analysis.NetCostSavedMonthly > 0 && analysis.PaybackMonths <= 12

	// 推荐场景
	analysis.RecommendedScenarios = []string{
		"内存受限场景（传统DDT内存需求超过可用内存）",
		"虚拟化环境（VM镜像去重率高，内存需求大）",
		"备份数据库（大量重复数据，传统DDT内存压力大）",
		"多数据集去重（需要同时启用多个数据集去重）",
	}

	// 不推荐场景
	analysis.NotRecommendedScenarios = []string{
		"低去重率场景（<15%，内存需求本来就小）",
		"小数据量场景（<5TB，DDT内存需求可接受）",
		"无SSD缓存场景（无法获得性能补偿）",
		"高写入频率场景（DDT更新开销大）",
	}

	// 风险提示
	analysis.Risks = c.generateRisks(analysis)

	// 优化建议
	analysis.Suggestions = c.generateSuggestions(analysis)

	return analysis
}

// AnalyzeScenario 分析特定场景.
func (c *FastDedupROICalculator) AnalyzeScenario(dataSizeTB float64, dedupRate float64) *FastDedupScenarioResult {
	// 临时调整配置
	originalData := c.config.TotalDataBytes
	originalRate := c.config.ExpectedDedupRate

	c.config.TotalDataBytes = uint64(dataSizeTB * 1024 * 1024 * 1024 * 1024)
	c.config.ExpectedDedupRate = dedupRate

	analysis := c.Analyze()

	// 恢复原始配置
	c.config.TotalDataBytes = originalData
	c.config.ExpectedDedupRate = originalRate

	result := &FastDedupScenarioResult{
		Scenario:            fmt.Sprintf("%.1fTB @ %.1f%%去重", dataSizeTB, dedupRate),
		DataSizeTB:          dataSizeTB,
		DedupRate:           dedupRate,
		TraditionalMemoryGB: fastDedupRound(analysis.TraditionalMemoryGB, 2),
		FastDedupMemoryGB:   fastDedupRound(analysis.FastDedupMemoryGB, 2),
		MemorySavingRate:    fastDedupRound(analysis.ActualMemorySavingRate, 2),
		TraditionalCost:     fastDedupRound(analysis.TraditionalMemoryCost, 2),
		FastDedupCost:       fastDedupRound(analysis.FastDedupTotalCost, 2),
		CostSaved:           fastDedupRound(analysis.NetCostSavedMonthly, 2),
		ROI:                 fastDedupRound(analysis.FastDedupROI, 2),
		Recommendation:      "不推荐",
	}

	// 根据ROI和内存节省判断推荐状态
	if result.MemorySavingRate >= 80 && result.CostSaved > 0 {
		result.Recommendation = "强烈推荐"
	} else if result.MemorySavingRate >= 70 && result.CostSaved > 0 {
		result.Recommendation = "推荐"
	} else if result.MemorySavingRate >= 60 {
		result.Recommendation = "可考虑"
	}

	return result
}

// CompareScenarios 对比多场景.
func (c *FastDedupROICalculator) CompareScenarios() *FastDedupComparison {
	comparison := &FastDedupComparison{
		Scenarios:             make([]FastDedupScenarioResult, 0),
		MemorySavingCurve:     make([]MemorySavingPoint, 0),
		CostBenefitCurve:      make([]CostBenefitPoint, 0),
		OverallRecommendation: "根据场景选择",
	}

	// 场景矩阵：数据量 x 去重率
	dataSizes := []float64{5, 10, 20, 50, 100}      // TB
	dedupRates := []float64{15, 20, 30, 40, 50, 60} // %

	bestROI := -999.0
	bestScenarioName := ""

	for _, dataSize := range dataSizes {
		for _, dedupRate := range dedupRates {
			result := c.AnalyzeScenario(dataSize, dedupRate)
			comparison.Scenarios = append(comparison.Scenarios, *result)

			// 记录内存节省曲线
			comparison.MemorySavingCurve = append(comparison.MemorySavingCurve,
				MemorySavingPoint{
					DataSizeTB:          dataSize,
					TraditionalMemoryGB: result.TraditionalMemoryGB,
					FastDedupMemoryGB:   result.FastDedupMemoryGB,
					SavingRate:          result.MemorySavingRate,
					MeetsClaim:          result.MemorySavingRate >= c.config.MemorySavingClaim,
				})

			// 记录成本效益曲线
			comparison.CostBenefitCurve = append(comparison.CostBenefitCurve,
				CostBenefitPoint{
					DedupRate:  dedupRate,
					Cost:       result.FastDedupCost,
					Benefit:    result.TraditionalCost - result.FastDedupCost,
					NetBenefit: result.CostSaved,
					ROI:        result.ROI,
				})

			// 找最佳场景
			if result.ROI > bestROI {
				bestROI = result.ROI
				bestScenarioName = result.Scenario
			}
		}
	}

	comparison.BestScenario = bestScenarioName

	// 设置启用阈值
	comparison.EnableThreshold.MinDataSizeTB = 10.0    // 最小10TB
	comparison.EnableThreshold.MinDedupRate = 20.0     // 最小20%去重率
	comparison.EnableThreshold.MaxMemoryBudgetGB = 4.0 // 传统DDT超过4GB时强烈推荐

	// 生成总体建议
	if bestROI > 100 {
		comparison.OverallRecommendation = "强烈推荐启用Fast Dedup，ROI优秀"
	} else if bestROI > 50 {
		comparison.OverallRecommendation = "推荐启用Fast Dedup，成本效益明显"
	} else if bestROI > 0 {
		comparison.OverallRecommendation = "可考虑启用Fast Dedup，需评估具体场景"
	} else {
		comparison.OverallRecommendation = "当前条件不建议启用Fast Dedup"
	}

	return comparison
}

// ========== 私有方法 ==========

// calculateBenefitScore 计算效益评分.
func (c *FastDedupROICalculator) calculateBenefitScore(analysis *FastDedupAnalysis) float64 {
	score := 0.0

	// 内存节省贡献（最高40分）
	if analysis.ActualMemorySavingRate >= 90 {
		score += 40
	} else if analysis.ActualMemorySavingRate >= 80 {
		score += 35
	} else if analysis.ActualMemorySavingRate >= 70 {
		score += 25
	} else if analysis.ActualMemorySavingRate >= 60 {
		score += 15
	} else if analysis.ActualMemorySavingRate >= 50 {
		score += 5
	}

	// 成本节省贡献（最高30分）
	if analysis.CostSavingRate >= 50 {
		score += 30
	} else if analysis.CostSavingRate >= 40 {
		score += 25
	} else if analysis.CostSavingRate >= 30 {
		score += 20
	} else if analysis.CostSavingRate >= 20 {
		score += 15
	} else if analysis.CostSavingRate >= 10 {
		score += 10
	}

	// ROI贡献（最高20分）
	if analysis.FastDedupROI >= 200 {
		score += 20
	} else if analysis.FastDedupROI >= 100 {
		score += 15
	} else if analysis.FastDedupROI >= 50 {
		score += 10
	} else if analysis.FastDedupROI >= 0 {
		score += 5
	}

	// 回收期贡献（最高10分）
	if analysis.PaybackMonths <= 3 {
		score += 10
	} else if analysis.PaybackMonths <= 6 {
		score += 8
	} else if analysis.PaybackMonths <= 12 {
		score += 5
	}

	return fastDedupRound(score, 1)
}

// generateRisks 生成风险提示.
func (c *FastDedupROICalculator) generateRisks(analysis *FastDedupAnalysis) []string {
	risks := make([]string, 0)

	// 内存节省未达宣称效果
	if analysis.MemorySavingVerified == "underperform" {
		risks = append(risks,
			fmt.Sprintf("⚠️ 实际内存节省率 %.2f%% 低于TrueNAS宣称的 %.2f%%",
				analysis.ActualMemorySavingRate, c.config.MemorySavingClaim))
	}

	// SSD缓存风险
	if analysis.FastDedupSSDCacheGB > 10 {
		risks = append(risks,
			fmt.Sprintf("需要 %.2f GB SSD缓存，确保有足够SSD容量", analysis.FastDedupSSDCacheGB))
	}

	// 性能风险
	if analysis.NetPerformanceImpact > 0 {
		risks = append(risks,
			fmt.Sprintf("Fast Dedup可能带来 %.2f%% 性能损耗", analysis.NetPerformanceImpact))
	}

	// 成本风险
	if analysis.NetCostSavedMonthly < 0 {
		risks = append(risks,
			fmt.Sprintf("当前条件下Fast Dedup成本高于传统去重，月增加 %.2f 元",
				-analysis.NetCostSavedMonthly))
	}

	// 技术风险
	risks = append(risks, "Fast Dedup需要ZFS特定版本支持（TrueNAS 24.10+）")
	risks = append(risks, "DDT缓存策略需要合理配置，否则性能下降明显")
	risks = append(risks, "SSD缓存故障时去重性能会显著下降")

	return risks
}

// generateSuggestions 生成优化建议.
func (c *FastDedupROICalculator) generateSuggestions(analysis *FastDedupAnalysis) []string {
	suggestions := make([]string, 0)

	// 内存优化建议
	if analysis.TraditionalMemoryGB > 4 && analysis.RecommendFastDedup {
		suggestions = append(suggestions,
			"💡 传统DDT需要 %.2f GB内存，强烈推荐使用Fast Dedup")
	}

	// SSD配置建议
	suggestions = append(suggestions,
		fmt.Sprintf("💡 建议配置 %.2f GB SSD作为DDT专用缓存", analysis.FastDedupSSDCacheGB))
	suggestions = append(suggestions,
		"💡 使用高耐久度SSD（如企业级NVMe）作为DDT缓存")

	// 性能优化建议
	if analysis.Config.SSDCacheHitRate < 80 {
		suggestions = append(suggestions,
			"💡 优化DDT缓存策略提高命中率，目标>85%")
	}
	suggestions = append(suggestions,
		"💡 定期监控DDT访问模式和缓存效率")

	// 成本优化建议
	if analysis.NetCostSavedMonthly > 0 {
		suggestions = append(suggestions,
			fmt.Sprintf("💡 Fast Dedup每月可节省 %.2f 元成本", analysis.NetCostSavedMonthly))
	}

	// 实施建议
	suggestions = append(suggestions,
		"💡 先在测试环境验证Fast Dedup效果")
	suggestions = append(suggestions,
		"💡 逐步启用，先对高去重率数据集测试")
	suggestions = append(suggestions,
		"💡 建立内存和SSD监控告警机制")

	return suggestions
}

// ========== 工具方法 ==========

// EstimateFastDedupMemory 估算Fast Dedup内存需求.
func EstimateFastDedupMemory(totalDataTB float64, blockSizeKB uint64, dedupRate float64) float64 {
	totalDataBytes := totalDataTB * 1024 * 1024 * 1024 * 1024
	totalBlocks := uint64(totalDataBytes) / (blockSizeKB * 1024)
	uniqueBlocks := totalBlocks * uint64(100-dedupRate) / 100
	fastDedupBytes := uniqueBlocks * 32 // Fast Dedup条目32字节
	return float64(fastDedupBytes) / (1024 * 1024 * 1024)
}

// EstimateTraditionalMemory 估算传统DDT内存需求.
func EstimateTraditionalMemory(totalDataTB float64, blockSizeKB uint64, dedupRate float64) float64 {
	totalDataBytes := totalDataTB * 1024 * 1024 * 1024 * 1024
	totalBlocks := uint64(totalDataBytes) / (blockSizeKB * 1024)
	uniqueBlocks := totalBlocks * uint64(100-dedupRate) / 100
	traditionalBytes := uniqueBlocks * 320 // 传统条目320字节
	return float64(traditionalBytes) / (1024 * 1024 * 1024)
}

// QuickMemoryCheck 快速内存需求检查.
func QuickMemoryCheck(dataSizeTB float64, dedupRate float64) string {
	traditionalMem := EstimateTraditionalMemory(dataSizeTB, 128, dedupRate)
	fastDedupMem := EstimateFastDedupMemory(dataSizeTB, 128, dedupRate)
	savingRate := (traditionalMem - fastDedupMem) / traditionalMem * 100

	return fmt.Sprintf("%.1fTB %.1f%%去重: 传统DDT %.2fGB, Fast Dedup %.2fGB, 节省 %.2f%%",
		dataSizeTB, dedupRate, traditionalMem, fastDedupMem, savingRate)
}

// GenerateFastDedupReport 生成Fast Dedup报告.
func GenerateFastDedupReport(config FastDedupConfig) string {
	calc := NewFastDedupROICalculator(config)
	analysis := calc.Analyze()

	report := fmt.Sprintf(`
# ZFS Fast Dedup ROI分析报告

## 配置参数
- 数据总量: %.2f TB
- 预期去重率: %.1f%%
- 平均块大小: %d KB
- TrueNAS宣称内存节省: %.1f%%

## 内存需求对比
| 方案 | 内存需求(GB) | 月成本(元) |
|------|-------------|-----------|
| 传统DDT | %.2f | %.2f |
| Fast Dedup | %.2f | %.2f |

## 内存节省效果
- 实际节省: %.2f GB
- 节省率: %.2f%%
- 与宣称差异: %.2f%%
- 验证状态: %s

## 成本分析
| 项目 | 月成本(元) |
|------|-----------|
| 传统DDT内存成本 | %.2f |
| Fast Dedup内存成本 | %.2f |
| Fast Dedup SSD缓存成本 | %.2f |
| Fast Dedup总成本 | %.2f |
| 净成本节省 | %.2f |

## ROI指标
- ROI比率: %.2f%%
- 投资回收期: %d 个月
- 效益评分: %.1f/100

## 推荐建议
是否推荐: %s

### 推荐场景
%s

### 不推荐场景
%s

### 风险提示
%s

### 优化建议
%s
`,
		float64(config.TotalDataBytes)/1024/1024/1024/1024,
		config.ExpectedDedupRate,
		config.AvgBlockSizeBytes/1024,
		config.MemorySavingClaim,
		analysis.TraditionalMemoryGB,
		analysis.TraditionalMemoryCost,
		analysis.FastDedupMemoryGB,
		analysis.FastDedupMemoryCost,
		analysis.ActualMemorySavedGB,
		analysis.ActualMemorySavingRate,
		analysis.MemorySavingDifference,
		analysis.MemorySavingVerified,
		analysis.TraditionalMemoryCost,
		analysis.FastDedupMemoryCost,
		analysis.FastDedupSSDCacheCost,
		analysis.FastDedupTotalCost,
		analysis.NetCostSavedMonthly,
		analysis.FastDedupROI,
		analysis.PaybackMonths,
		analysis.BenefitScore,
		fastDedupBoolToStr(analysis.RecommendFastDedup),
		fastDedupJoinList(analysis.RecommendedScenarios),
		fastDedupJoinList(analysis.NotRecommendedScenarios),
		fastDedupJoinList(analysis.Risks),
		fastDedupJoinList(analysis.Suggestions),
	)

	return report
}

// ========== 辅助函数 ==========

func fastDedupRound(val float64, precision int) float64 {
	factor := math.Pow10(precision)
	return math.Round(val*factor) / factor
}

func fastDedupBoolToStr(b bool) string {
	if b {
		return "✅ 推荐启用"
	}
	return "❌ 不推荐启用"
}

func fastDedupJoinList(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += "\n"
		}
		result += "- " + item
	}
	return result
}
