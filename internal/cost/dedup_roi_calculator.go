// Package cost - 存储去重ROI计算器
// 提供ZFS Deduplication成本效益分析和ROI计算
package cost

import (
	"fmt"
	"math"
	"time"
)

// ========== 去重成本效益数据结构 ==========

// DedupCostConfig 去重成本配置
type DedupCostConfig struct {
	// 内存成本（元/GB/月）
	MemoryCostPerGBMonthly float64 `json:"memory_cost_per_gb_monthly"`

	// SSD成本（元/GB/月）- DDT表存储
	SSDCostPerGBMonthly float64 `json:"ssd_cost_per_gb_monthly"`

	// CPU成本系数（去重计算开销）
	CPUCostFactor float64 `json:"cpu_cost_factor"`

	// 电费（元/kWh）
	ElectricityCostPerkWh float64 `json:"electricity_cost_per_kwh"`

	// 服务器功率（W）
	ServerPowerWatts float64 `json:"server_power_watts"`

	// 运维人力成本（元/月）
	OpsCostMonthly float64 `json:"ops_cost_monthly"`

	// 存储节省价值（元/GB/月）
	StorageValuePerGBMonthly float64 `json:"storage_value_per_gb_monthly"`

	// 去重率预期（%）
	ExpectedDedupRate float64 `json:"expected_dedup_rate"`

	// 平均块大小（字节）
	AvgChunkSizeBytes uint64 `json:"avg_chunk_size_bytes"`

	// 总数据量（字节）
	TotalDataBytes uint64 `json:"total_data_bytes"`

	// DDT条目内存占用（字节/条目）
	DDTEntryMemoryBytes uint64 `json:"ddt_entry_memory_bytes"`
}

// DefaultDedupCostConfig 默认去重成本配置
func DefaultDedupCostConfig() DedupCostConfig {
	return DedupCostConfig{
		MemoryCostPerGBMonthly:   0.15,                           // 云内存价格参考
		SSDCostPerGBMonthly:      0.08,                           // SSD存储价格
		CPUCostFactor:            0.05,                           // 5% CPU开销系数
		ElectricityCostPerkWh:    0.6,                            // 国内电价
		ServerPowerWatts:         200,                            // 服务器功耗
		OpsCostMonthly:           500,                            // 运维成本
		StorageValuePerGBMonthly: 0.10,                           // 存储节省价值
		ExpectedDedupRate:        30.0,                           // 30%去重率预期
		AvgChunkSizeBytes:        32768,                          // 32KB块大小
		TotalDataBytes:           10 * 1024 * 1024 * 1024 * 1024, // 10TB
		DDTEntryMemoryBytes:      80,                             // DDT条目约80字节
	}
}

// DedupCostAnalysis 去重成本分析结果
type DedupCostAnalysis struct {
	// 分析ID
	ID string `json:"id"`

	// 分析时间
	AnalysisTime time.Time `json:"analysis_time"`

	// 配置参数
	Config DedupCostConfig `json:"config"`

	// ========== 成本项 ==========

	// DDT表内存成本（元/月）
	DDTMemoryCostMonthly float64 `json:"ddt_memory_cost_monthly"`

	// DDT表存储成本（元/月）
	DDTStorageCostMonthly float64 `json:"ddt_storage_cost_monthly"`

	// CPU计算成本（元/月）
	CPUCostMonthly float64 `json:"cpu_cost_monthly"`

	// 电费增量（元/月）
	ElectricityCostMonthly float64 `json:"electricity_cost_monthly"`

	// 运维成本增量（元/月）
	OpsCostMonthly float64 `json:"ops_cost_monthly"`

	// 总去重成本（元/月）
	TotalDedupCostMonthly float64 `json:"total_dedup_cost_monthly"`

	// ========== 收益项 ==========

	// 去重节省空间（字节）
	SavedSpaceBytes uint64 `json:"saved_space_bytes"`

	// 去重节省空间（GB）
	SavedSpaceGB float64 `json:"saved_space_gb"`

	// 节省存储成本（元/月）
	SavedStorageCostMonthly float64 `json:"saved_storage_cost_monthly"`

	// 减少备份成本（元/月）
	SavedBackupCostMonthly float64 `json:"saved_backup_cost_monthly"`

	// 减少带宽成本（元/月）
	SavedBandwidthCostMonthly float64 `json:"saved_bandwidth_cost_monthly"`

	// 总收益（元/月）
	TotalBenefitMonthly float64 `json:"total_benefit_monthly"`

	// ========== ROI指标 ==========

	// 净收益（元/月）
	NetBenefitMonthly float64 `json:"net_benefit_monthly"`

	// ROI比率（%）
	ROIRatio float64 `json:"roi_ratio"`

	// 投资回收期（月）
	PaybackMonths int `json:"payback_months"`

	// 效益评分（0-100）
	BenefitScore float64 `json:"benefit_score"`

	// ========== 建议与风险 ==========

	// 是否值得启用
	WorthEnabling bool `json:"worth_enabling"`

	// 推荐去重率阈值
	RecommendedDedupThreshold float64 `json:"recommended_dedup_threshold"`

	// 风险提示
	Risks []string `json:"risks"`

	// 优化建议
	Suggestions []string `json:"suggestions"`

	// 成本明细
	CostBreakdown map[string]float64 `json:"cost_breakdown"`

	// 收益明细
	BenefitBreakdown map[string]float64 `json:"benefit_breakdown"`
}

// DedupROIResult ROI计算结果
type DedupROIResult struct {
	// 场景名称
	Scenario string `json:"scenario"`

	// 数据量（TB）
	DataSizeTB float64 `json:"data_size_tb"`

	// 实际去重率（%）
	ActualDedupRate float64 `json:"actual_dedup_rate"`

	// 内存需求（GB）
	MemoryRequiredGB float64 `json:"memory_required_gb"`

	// 月度成本（元）
	MonthlyCost float64 `json:"monthly_cost"`

	// 月度收益（元）
	MonthlyBenefit float64 `json:"monthly_benefit"`

	// ROI（%）
	ROI float64 `json:"roi"`

	// 回收期（月）
	PaybackMonths int `json:"payback_months"`

	// 推荐状态
	Recommendation string `json:"recommendation"`
}

// DedupScenarioAnalysis 场景分析
type DedupScenarioAnalysis struct {
	// 各场景结果
	Scenarios []DedupROIResult `json:"scenarios"`

	// 最优场景
	BestScenario *DedupROIResult `json:"best_scenario"`

	// 建议启用阈值
	EnableThreshold float64 `json:"enable_threshold"`

	// 不建议启用的条件
	DisableConditions []string `json:"disable_conditions"`

	// 成本效益曲线数据
	CostBenefitCurve []CostBenefitPoint `json:"cost_benefit_curve"`
}

// CostBenefitPoint 成本效益曲线点
type CostBenefitPoint struct {
	// 去重率（%）
	DedupRate float64 `json:"dedup_rate"`

	// 成本（元/月）
	Cost float64 `json:"cost"`

	// 收益（元/月）
	Benefit float64 `json:"benefit"`

	// 净收益
	NetBenefit float64 `json:"net_benefit"`

	// ROI（%）
	ROI float64 `json:"roi"`
}

// ========== 去重ROI计算器 ==========

// DedupROICalculator 去重ROI计算器
type DedupROICalculator struct {
	config DedupCostConfig
}

// NewDedupROICalculator 创建去重ROI计算器
func NewDedupROICalculator(config DedupCostConfig) *DedupROICalculator {
	return &DedupROICalculator{config: config}
}

// Analyze 执行成本效益分析
func (c *DedupROICalculator) Analyze() *DedupCostAnalysis {
	now := time.Now()
	analysis := &DedupCostAnalysis{
		ID:               fmt.Sprintf("dedup_analysis_%d", now.Unix()),
		AnalysisTime:     now,
		Config:           c.config,
		Risks:            make([]string, 0),
		Suggestions:      make([]string, 0),
		CostBreakdown:    make(map[string]float64),
		BenefitBreakdown: make(map[string]float64),
	}

	// ========== 计算DDT表大小 ==========

	// 估算唯一块数量
	totalChunks := c.config.TotalDataBytes / c.config.AvgChunkSizeBytes
	uniqueChunks := totalChunks * uint64(100-c.config.ExpectedDedupRate) / 100

	// DDT表内存占用
	ddtMemoryBytes := uniqueChunks * c.config.DDTEntryMemoryBytes
	ddtMemoryGB := float64(ddtMemoryBytes) / (1024 * 1024 * 1024)

	// ========== 计算成本项 ==========

	// 1. DDT内存成本
	analysis.DDTMemoryCostMonthly = ddtMemoryGB * c.config.MemoryCostPerGBMonthly
	analysis.CostBreakdown["ddt_memory"] = analysis.DDTMemoryCostMonthly

	// 2. DDT存储成本（假设存储在SSD）
	ddtStorageGB := ddtMemoryGB * 1.5 // 存储通常比内存大
	analysis.DDTStorageCostMonthly = ddtStorageGB * c.config.SSDCostPerGBMonthly
	analysis.CostBreakdown["ddt_storage"] = analysis.DDTStorageCostMonthly

	// 3. CPU计算成本（基于数据量）
	dataGB := float64(c.config.TotalDataBytes) / (1024 * 1024 * 1024)
	analysis.CPUCostMonthly = dataGB * c.config.CPUCostFactor
	analysis.CostBreakdown["cpu_compute"] = analysis.CPUCostMonthly

	// 4. 电费增量（假设去重增加20%CPU负载）
	hoursPerMonth := 24 * 30
	additionalPower := c.config.ServerPowerWatts * 0.2 // 20%增量
	analysis.ElectricityCostMonthly = float64(additionalPower) / 1000 *
		float64(hoursPerMonth) * c.config.ElectricityCostPerkWh
	analysis.CostBreakdown["electricity"] = analysis.ElectricityCostMonthly

	// 5. 运维成本增量（假设10%运维时间用于去重管理）
	analysis.OpsCostMonthly = c.config.OpsCostMonthly * 0.1
	analysis.CostBreakdown["ops"] = analysis.OpsCostMonthly

	// 计算总成本
	analysis.TotalDedupCostMonthly = analysis.DDTMemoryCostMonthly +
		analysis.DDTStorageCostMonthly + analysis.CPUCostMonthly +
		analysis.ElectricityCostMonthly + analysis.OpsCostMonthly

	// ========== 计算收益项 ==========

	// 1. 去重节省空间
	analysis.SavedSpaceBytes = c.config.TotalDataBytes *
		uint64(c.config.ExpectedDedupRate) / 100
	analysis.SavedSpaceGB = float64(analysis.SavedSpaceBytes) / (1024 * 1024 * 1024)

	// 2. 节省存储成本
	analysis.SavedStorageCostMonthly = analysis.SavedSpaceGB *
		c.config.StorageValuePerGBMonthly
	analysis.BenefitBreakdown["storage_savings"] = analysis.SavedStorageCostMonthly

	// 3. 减少备份成本（假设备份成本为存储成本的30%）
	analysis.SavedBackupCostMonthly = analysis.SavedStorageCostMonthly * 0.3
	analysis.BenefitBreakdown["backup_savings"] = analysis.SavedBackupCostMonthly

	// 4. 减少带宽成本（假设带宽成本为存储成本的20%）
	analysis.SavedBandwidthCostMonthly = analysis.SavedStorageCostMonthly * 0.2
	analysis.BenefitBreakdown["bandwidth_savings"] = analysis.SavedBandwidthCostMonthly

	// 计算总收益
	analysis.TotalBenefitMonthly = analysis.SavedStorageCostMonthly +
		analysis.SavedBackupCostMonthly + analysis.SavedBandwidthCostMonthly

	// ========== 计算ROI指标 ==========

	// 净收益
	analysis.NetBenefitMonthly = analysis.TotalBenefitMonthly - analysis.TotalDedupCostMonthly

	// ROI比率
	if analysis.TotalDedupCostMonthly > 0 {
		analysis.ROIRatio = (analysis.NetBenefitMonthly / analysis.TotalDedupCostMonthly) * 100
	}

	// 投资回收期
	if analysis.NetBenefitMonthly > 0 {
		analysis.PaybackMonths = int(math.Ceil(analysis.TotalDedupCostMonthly /
			analysis.NetBenefitMonthly))
	} else {
		analysis.PaybackMonths = -1 // 永不回收
	}

	// 效益评分（综合评估）
	analysis.BenefitScore = c.calculateBenefitScore(analysis)

	// ========== 判断是否值得启用 ==========

	analysis.WorthEnabling = analysis.NetBenefitMonthly > 0 && analysis.PaybackMonths <= 12

	// 推荐去重率阈值
	analysis.RecommendedDedupThreshold = c.calculateThreshold(analysis)

	// ========== 生成风险提示 ==========

	analysis.Risks = c.generateRisks(analysis, ddtMemoryGB)

	// ========== 生成优化建议 ==========

	analysis.Suggestions = c.generateSuggestions(analysis)

	return analysis
}

// AnalyzeScenario 分析特定场景
func (c *DedupROICalculator) AnalyzeScenario(dataSizeTB float64, dedupRate float64) *DedupROIResult {
	// 临时调整配置
	originalData := c.config.TotalDataBytes
	originalRate := c.config.ExpectedDedupRate

	c.config.TotalDataBytes = uint64(dataSizeTB * 1024 * 1024 * 1024 * 1024)
	c.config.ExpectedDedupRate = dedupRate

	analysis := c.Analyze()

	// 恢复原始配置
	c.config.TotalDataBytes = originalData
	c.config.ExpectedDedupRate = originalRate

	// 计算内存需求
	totalChunks := c.config.TotalDataBytes / c.config.AvgChunkSizeBytes
	uniqueChunks := totalChunks * uint64(100-c.config.ExpectedDedupRate) / 100
	ddtMemoryGB := float64(uniqueChunks*c.config.DDTEntryMemoryBytes) / (1024 * 1024 * 1024)

	result := &DedupROIResult{
		Scenario:         fmt.Sprintf("%.1fTB @ %.1f%%去重", dataSizeTB, dedupRate),
		DataSizeTB:       dataSizeTB,
		ActualDedupRate:  dedupRate,
		MemoryRequiredGB: dedupRound(ddtMemoryGB, 2),
		MonthlyCost:      dedupRound(analysis.TotalDedupCostMonthly, 2),
		MonthlyBenefit:   dedupRound(analysis.TotalBenefitMonthly, 2),
		ROI:              dedupRound(analysis.ROIRatio, 2),
		PaybackMonths:    analysis.PaybackMonths,
		Recommendation:   "不建议启用",
	}

	if result.ROI > 50 && result.PaybackMonths <= 6 {
		result.Recommendation = "强烈推荐启用"
	} else if result.ROI > 0 && result.PaybackMonths <= 12 {
		result.Recommendation = "建议启用"
	} else if result.ROI > -20 {
		result.Recommendation = "可考虑启用"
	}

	return result
}

// AnalyzeMultipleScenarios 分析多场景
func (c *DedupROICalculator) AnalyzeMultipleScenarios() *DedupScenarioAnalysis {
	scenarioAnalysis := &DedupScenarioAnalysis{
		Scenarios:         make([]DedupROIResult, 0),
		DisableConditions: make([]string, 0),
		CostBenefitCurve:  make([]CostBenefitPoint, 0),
	}

	// 定义场景矩阵
	dataSizes := []float64{1, 5, 10, 20, 50, 100}       // TB
	dedupRates := []float64{10, 20, 30, 40, 50, 60, 70} // %

	// 分析各场景
	for _, dataSize := range dataSizes {
		for _, dedupRate := range dedupRates {
			result := c.AnalyzeScenario(dataSize, dedupRate)
			scenarioAnalysis.Scenarios = append(scenarioAnalysis.Scenarios, *result)

			// 记录成本效益曲线点
			scenarioAnalysis.CostBenefitCurve = append(scenarioAnalysis.CostBenefitCurve,
				CostBenefitPoint{
					DedupRate:  dedupRate,
					Cost:       result.MonthlyCost,
					Benefit:    result.MonthlyBenefit,
					NetBenefit: result.MonthlyBenefit - result.MonthlyCost,
					ROI:        result.ROI,
				})
		}
	}

	// 找出最优场景
	bestROI := -999.0
	for _, scenario := range scenarioAnalysis.Scenarios {
		if scenario.ROI > bestROI {
			bestROI = scenario.ROI
			scenarioAnalysis.BestScenario = &scenario
		}
	}

	// 计算建议启用阈值
	scenarioAnalysis.EnableThreshold = c.calculateEnableThreshold(scenarioAnalysis)

	// 不建议启用的条件
	scenarioAnalysis.DisableConditions = []string{
		"去重率低于15%时，成本大于收益",
		"数据量小于5TB时，DDT开销占比过高",
		"内存不足4GB时，DDT表性能受限",
		"高频率随机写入场景，去重开销过大",
	}

	return scenarioAnalysis
}

// ========== 私有方法 ==========

// calculateBenefitScore 计算效益评分
func (c *DedupROICalculator) calculateBenefitScore(analysis *DedupCostAnalysis) float64 {
	score := 0.0

	// ROI贡献（最高40分）
	if analysis.ROIRatio > 200 {
		score += 40
	} else if analysis.ROIRatio > 100 {
		score += 35
	} else if analysis.ROIRatio > 50 {
		score += 25
	} else if analysis.ROIRatio > 0 {
		score += 15
	} else if analysis.ROIRatio > -50 {
		score += 5
	}

	// 回收期贡献（最高30分）
	if analysis.PaybackMonths <= 3 {
		score += 30
	} else if analysis.PaybackMonths <= 6 {
		score += 25
	} else if analysis.PaybackMonths <= 12 {
		score += 15
	} else if analysis.PaybackMonths <= 24 {
		score += 5
	}

	// 去重率贡献（最高20分）
	if c.config.ExpectedDedupRate >= 50 {
		score += 20
	} else if c.config.ExpectedDedupRate >= 30 {
		score += 15
	} else if c.config.ExpectedDedupRate >= 20 {
		score += 10
	} else if c.config.ExpectedDedupRate >= 15 {
		score += 5
	}

	// 节省空间贡献（最高10分）
	if analysis.SavedSpaceGB >= 1000 {
		score += 10
	} else if analysis.SavedSpaceGB >= 500 {
		score += 8
	} else if analysis.SavedSpaceGB >= 100 {
		score += 5
	} else if analysis.SavedSpaceGB >= 50 {
		score += 3
	}

	return dedupRound(score, 1)
}

// calculateThreshold 计算推荐阈值
func (c *DedupROICalculator) calculateThreshold(analysis *DedupCostAnalysis) float64 {
	// 基于ROI反推最小去重率
	// 简化计算：成本固定时，去重率需要达到多少才能收支平衡

	costPerGBData := analysis.TotalDedupCostMonthly /
		(float64(c.config.TotalDataBytes) / (1024 * 1024 * 1024))

	// 收支平衡点：节省价值 = 成本
	// 去重率 = 成本 / (存储价值 * 总数据量)
	if c.config.StorageValuePerGBMonthly > 0 {
		threshold := costPerGBData / c.config.StorageValuePerGBMonthly * 100
		if threshold < 15 {
			threshold = 15 // 最小建议阈值
		}
		return dedupRound(threshold, 1)
	}

	return 20.0 // 默认阈值
}

// calculateEnableThreshold 计算启用阈值
func (c *DedupROICalculator) calculateEnableThreshold(sa *DedupScenarioAnalysis) float64 {
	// 找出ROI首次大于0的去重率
	minPositiveRate := 100.0

	for _, point := range sa.CostBenefitCurve {
		if point.ROI > 0 && point.DedupRate < minPositiveRate {
			minPositiveRate = point.DedupRate
		}
	}

	if minPositiveRate == 100.0 {
		return 50.0 // 默认较高阈值
	}

	return dedupRound(minPositiveRate, 1)
}

// generateRisks 生成风险提示
func (c *DedupROICalculator) generateRisks(analysis *DedupCostAnalysis, ddtMemoryGB float64) []string {
	risks := make([]string, 0)

	// 内存风险
	if ddtMemoryGB > 4 {
		risks = append(risks,
			fmt.Sprintf("DDT表需要 %.2f GB内存，可能影响系统性能", ddtMemoryGB))
	}
	if ddtMemoryGB > 16 {
		risks = append(risks,
			"⚠️ 内存需求过高，建议使用Fast Dedup或增大内存")
	}

	// ROI风险
	if analysis.ROIRatio < 0 {
		risks = append(risks,
			fmt.Sprintf("当前ROI为 %.2f%%，成本大于收益，不建议启用", analysis.ROIRatio))
	}
	if analysis.PaybackMonths > 24 {
		risks = append(risks,
			fmt.Sprintf("投资回收期超过 %d 个月，经济性较差", analysis.PaybackMonths))
	}

	// 去重率风险
	if c.config.ExpectedDedupRate < 20 {
		risks = append(risks,
			fmt.Sprintf("预期去重率 %.1f%% 较低，成本效益不明显",
				c.config.ExpectedDedupRate))
	}

	// 性能风险
	risks = append(risks,
		"去重会增加写入延迟，高写入场景需谨慎")
	risks = append(risks,
		"DDT表损坏可能导致数据丢失，需定期备份")

	return risks
}

// generateSuggestions 生成优化建议
func (c *DedupROICalculator) generateSuggestions(analysis *DedupCostAnalysis) []string {
	suggestions := make([]string, 0)

	// 成本优化建议
	if analysis.DDTMemoryCostMonthly > analysis.TotalDedupCostMonthly*0.5 {
		suggestions = append(suggestions,
			"💡 使用Fast Dedup减少内存占用")
		suggestions = append(suggestions,
			"💡 增大块大小（64KB/128KB）减少DDT条目数")
	}

	if !analysis.WorthEnabling {
		suggestions = append(suggestions,
			"💡 当前条件下不建议启用全局去重")
		suggestions = append(suggestions,
			"💡 可考虑仅对特定数据集（虚拟机镜像、备份）启用去重")
	}

	// 收益优化建议
	if analysis.ROIRatio > 0 && analysis.ROIRatio < 50 {
		suggestions = append(suggestions,
			"💡 优化数据布局提高去重率（如集中存储相似数据）")
		suggestions = append(suggestions,
			"💡 定期分析去重效果，调整策略")
	}

	// 性能优化建议
	suggestions = append(suggestions,
		"💡 使用L2ARC缓存DDT表提升读取性能")
	suggestions = append(suggestions,
		"💡 定期执行DDT清理释放无用条目")

	// 风险规避建议
	suggestions = append(suggestions,
		"💡 启用前先在测试环境验证效果")
	suggestions = append(suggestions,
		"💡 监控内存使用率和系统性能变化")

	return suggestions
}

// ========== 工具方法 ==========

// EstimateDDTSize 估算DDT表大小
func EstimateDDTSize(totalDataBytes uint64, avgChunkSize uint64, dedupRate float64) uint64 {
	totalChunks := totalDataBytes / avgChunkSize
	uniqueChunks := totalChunks * uint64(100-dedupRate) / 100
	return uniqueChunks * 80 // 每条目约80字节
}

// EstimateMemoryRequirement 估算内存需求（GB）
func EstimateMemoryRequirement(totalDataTB float64, avgChunkSizeKB uint64, dedupRate float64) float64 {
	totalDataBytes := totalDataTB * 1024 * 1024 * 1024 * 1024
	ddtSize := EstimateDDTSize(uint64(totalDataBytes), avgChunkSizeKB*1024, dedupRate)
	return float64(ddtSize) / (1024 * 1024 * 1024)
}

// QuickROICheck 快速ROI检查
func QuickROICheck(dataSizeTB float64, dedupRate float64) string {
	config := DefaultDedupCostConfig()
	calc := NewDedupROICalculator(config)
	result := calc.AnalyzeScenario(dataSizeTB, dedupRate)

	return fmt.Sprintf("数据量 %.1fTB，去重率 %.1f%%：月成本 %.2f元，月收益 %.2f元，ROI %.2f%%，建议：%s",
		dataSizeTB, dedupRate, result.MonthlyCost, result.MonthlyBenefit, result.ROI, result.Recommendation)
}

// CompareDedupStrategies 对比去重策略
func CompareDedupStrategies(dataSizeTB float64) map[string]DedupROIResult {
	config := DefaultDedupCostConfig()
	calc := NewDedupROICalculator(config)

	strategies := map[string]DedupROIResult{
		"标准去重":       *calc.AnalyzeScenario(dataSizeTB, 30),
		"高去重场景":      *calc.AnalyzeScenario(dataSizeTB, 50),
		"低去重场景":      *calc.AnalyzeScenario(dataSizeTB, 15),
		"Fast Dedup": *calc.AnalyzeFastDedupScenario(dataSizeTB, 30),
	}

	return strategies
}

// AnalyzeFastDedupScenario 分析Fast Dedup场景
func (c *DedupROICalculator) AnalyzeFastDedupScenario(dataSizeTB float64, dedupRate float64) *DedupROIResult {
	// Fast Dedup配置：内存占用减少约50%，但SSD缓存成本增加
	fastConfig := c.config
	fastConfig.DDTEntryMemoryBytes = c.config.DDTEntryMemoryBytes / 2 // 内存减半
	fastConfig.MemoryCostPerGBMonthly *= 0.6                          // 内存成本降低
	fastConfig.SSDCostPerGBMonthly *= 1.5                             // SSD缓存成本增加

	fastCalc := NewDedupROICalculator(fastConfig)
	return fastCalc.AnalyzeScenario(dataSizeTB, dedupRate)
}

// GenerateDedupReport 生成去重成本报告
func GenerateDedupReport(config DedupCostConfig) string {
	calc := NewDedupROICalculator(config)
	analysis := calc.Analyze()

	report := fmt.Sprintf(`
# ZFS Deduplication 成本效益分析报告

## 配置参数
- 数据总量: %.2f TB
- 预期去重率: %.1f%%
- 平均块大小: %d KB

## 成本分析
| 成本项 | 月度成本(元) |
|--------|-------------|
| DDT内存成本 | %.2f |
| DDT存储成本 | %.2f |
| CPU计算成本 | %.2f |
| 电费增量 | %.2f |
| 运维成本 | %.2f |
| **总成本** | **%.2f** |

## 收益分析
| 收益项 | 月度收益(元) |
|--------|-------------|
| 节省存储空间 | %.2f GB |
| 存储成本节省 | %.2f |
| 备份成本节省 | %.2f |
| 带宽成本节省 | %.2f |
| **总收益** | **%.2f** |

## ROI指标
- 净收益: %.2f 元/月
- ROI比率: %.2f%%
- 投资回收期: %d 个月
- 效益评分: %.1f/100

## 建议
是否值得启用: %s
推荐去重率阈值: %.1f%%

### 风险提示
%s

### 优化建议
%s
`,
		float64(config.TotalDataBytes)/1024/1024/1024/1024,
		config.ExpectedDedupRate,
		config.AvgChunkSizeBytes/1024,
		analysis.DDTMemoryCostMonthly,
		analysis.DDTStorageCostMonthly,
		analysis.CPUCostMonthly,
		analysis.ElectricityCostMonthly,
		analysis.OpsCostMonthly,
		analysis.TotalDedupCostMonthly,
		analysis.SavedSpaceGB,
		analysis.SavedStorageCostMonthly,
		analysis.SavedBackupCostMonthly,
		analysis.SavedBandwidthCostMonthly,
		analysis.TotalBenefitMonthly,
		analysis.NetBenefitMonthly,
		analysis.ROIRatio,
		analysis.PaybackMonths,
		analysis.BenefitScore,
		boolToStr(analysis.WorthEnabling),
		analysis.RecommendedDedupThreshold,
		stringsJoin(analysis.Risks, "\n"),
		stringsJoin(analysis.Suggestions, "\n"),
	)

	return report
}

// ========== 辅助函数 ==========

func boolToStr(b bool) string {
	if b {
		return "✅ 建议启用"
	}
	return "❌ 不建议启用"
}

func stringsJoin(strs []string, sep string) string {
	result := ""
	for i, s := range strs {
		if i > 0 {
			result += sep
		}
		result += "- " + s
	}
	return result
}

// dedupRound 去重ROI计算专用round函数
func dedupRound(val float64, precision int) float64 {
	factor := math.Pow10(precision)
	return math.Round(val*factor) / factor
}
