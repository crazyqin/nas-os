// Package cost - 去重ROI计算器测试
package cost

import (
	"testing"
)

func TestDedupROICalculator_Analyze(t *testing.T) {
	config := DefaultDedupCostConfig()
	config.TotalDataBytes = 10 * 1024 * 1024 * 1024 * 1024 // 10TB
	config.ExpectedDedupRate = 30.0

	calc := NewDedupROICalculator(config)
	analysis := calc.Analyze()

	// 验证基本字段
	if analysis.ID == "" {
		t.Error("分析ID不应为空")
	}

	// 验证成本计算
	if analysis.TotalDedupCostMonthly <= 0 {
		t.Error("总去重成本应大于0")
	}

	// 验证收益计算
	if analysis.SavedSpaceGB <= 0 {
		t.Error("节省空间应大于0")
	}

	// 验证ROI计算
	if analysis.ROIRatio < -100 || analysis.ROIRatio > 500 {
		t.Errorf("ROI比率异常: %.2f%%", analysis.ROIRatio)
	}

	// 验证建议生成
	if len(analysis.Risks) == 0 {
		t.Error("应生成风险提示")
	}
	if len(analysis.Suggestions) == 0 {
		t.Error("应生成优化建议")
	}

	t.Logf("10TB 30%% dedup rate analysis result:")
	t.Logf("  monthly cost: %.2f yuan", analysis.TotalDedupCostMonthly)
	t.Logf("  monthly benefit: %.2f yuan", analysis.TotalBenefitMonthly)
	t.Logf("  ROI: %.2f%%", analysis.ROIRatio)
	t.Logf("  payback months: %d", analysis.PaybackMonths)
	t.Logf("  效益评分: %.1f", analysis.BenefitScore)
	t.Logf("  建议: %v", analysis.WorthEnabling)
}

func TestDedupROICalculator_AnalyzeScenario(t *testing.T) {
	config := DefaultDedupCostConfig()
	calc := NewDedupROICalculator(config)

	tests := []struct {
		dataSizeTB      float64
		dedupRate       float64
		wantPositiveROI bool
	}{
		{10, 30, true},  // 10TB 30%去重
		{10, 15, false}, // 10TB 15%去重（低去重率）
		{50, 50, true},  // 50TB 50%去重（高去重率）
		{1, 30, false},  // 1TB 30%去重（小数据量）
	}

	for _, tt := range tests {
		result := calc.AnalyzeScenario(tt.dataSizeTB, tt.dedupRate)

		if result.DataSizeTB != tt.dataSizeTB {
			t.Errorf("数据量不一致: got %.1f, want %.1f",
				result.DataSizeTB, tt.dataSizeTB)
		}

		if result.ActualDedupRate != tt.dedupRate {
			t.Errorf("去重率不一致: got %.1f, want %.1f",
				result.ActualDedupRate, tt.dedupRate)
		}

		t.Logf("场景 %.1fTB %.1f%%去重: ROI=%.2f%%, 建议=%s",
			tt.dataSizeTB, tt.dedupRate, result.ROI, result.Recommendation)
	}
}

func TestDedupROICalculator_MultipleScenarios(t *testing.T) {
	config := DefaultDedupCostConfig()
	calc := NewDedupROICalculator(config)

	scenarioAnalysis := calc.AnalyzeMultipleScenarios()

	if len(scenarioAnalysis.Scenarios) == 0 {
		t.Error("应生成多个场景分析结果")
	}

	if scenarioAnalysis.BestScenario == nil {
		t.Error("应识别最优场景")
	}

	if scenarioAnalysis.EnableThreshold <= 0 {
		t.Error("应计算启用阈值")
	}

	t.Logf("最优场景: %s", scenarioAnalysis.BestScenario.Scenario)
	t.Logf("建议启用阈值: %.1f%%", scenarioAnalysis.EnableThreshold)
	t.Logf("分析场景数: %d", len(scenarioAnalysis.Scenarios))
}

func TestEstimateDDTSize(t *testing.T) {
	// 10TB数据，32KB块，30%去重
	ddtSize := EstimateDDTSize(
		10*1024*1024*1024*1024,
		32*1024,
		30.0,
	)

	if ddtSize == 0 {
		t.Error("DDT大小不应为0")
	}

	ddtGB := float64(ddtSize) / (1024 * 1024 * 1024)
	t.Logf("10TB 32KB block 30%% dedup DDT size: %.2f GB", ddtGB)

	// 验证估算合理性（应该在0.1GB到20GB之间）
	if ddtGB < 0.1 || ddtGB > 20 {
		t.Errorf("DDT estimate abnormal: %.2f GB", ddtGB)
	}
}

func TestEstimateMemoryRequirement(t *testing.T) {
	memGB := EstimateMemoryRequirement(10, 32, 30.0)

	if memGB <= 0 {
		t.Error("memory requirement should be > 0")
	}

	t.Logf("10TB 32KB block 30%% dedup memory req: %.2f GB", memGB)

	// 验证合理性
	if memGB < 0.1 || memGB > 20 {
		t.Errorf("memory estimate abnormal: %.2f GB", memGB)
	}
}

func TestQuickROICheck(t *testing.T) {
	result := QuickROICheck(10, 30)

	if result == "" {
		t.Error("快速检查结果不应为空")
	}

	t.Logf("快速ROI检查: %s", result)
}

func TestCompareDedupStrategies(t *testing.T) {
	strategies := CompareDedupStrategies(10)

	if len(strategies) != 4 {
		t.Errorf("应返回4种策略对比，实际: %d", len(strategies))
	}

	for name, strategy := range strategies {
		t.Logf("策略[%s]: ROI=%.2f%%, 建议=%s",
			name, strategy.ROI, strategy.Recommendation)
	}
}

func TestAnalyzeFastDedupScenario(t *testing.T) {
	config := DefaultDedupCostConfig()
	calc := NewDedupROICalculator(config)

	result := calc.AnalyzeFastDedupScenario(10, 30)

	if result == nil {
		t.Error("Fast Dedup分析结果不应为空")
	}

	// Fast Dedup内存需求应更低
	standardResult := calc.AnalyzeScenario(10, 30)

	if result.MemoryRequiredGB > standardResult.MemoryRequiredGB {
		t.Errorf("Fast Dedup内存需求应更低: %.2f vs %.2f",
			result.MemoryRequiredGB, standardResult.MemoryRequiredGB)
	}

	t.Logf("Fast Dedup vs 标准: 内存 %.2fGB vs %.2fGB",
		result.MemoryRequiredGB, standardResult.MemoryRequiredGB)
}

func TestGenerateDedupReport(t *testing.T) {
	config := DefaultDedupCostConfig()
	config.TotalDataBytes = 20 * 1024 * 1024 * 1024 * 1024 // 20TB
	config.ExpectedDedupRate = 35.0

	report := GenerateDedupReport(config)

	if report == "" {
		t.Error("报告不应为空")
	}

	// 验证报告包含关键内容
	if !dedupContains(report, "成本效益分析") {
		t.Error("报告应包含标题")
	}
	if !dedupContains(report, "ROI") {
		t.Error("报告应包含ROI指标")
	}

	t.Logf("报告长度: %d 字符", len(report))
}

func TestDefaultDedupCostConfig(t *testing.T) {
	config := DefaultDedupCostConfig()

	// 验证默认值合理性
	if config.MemoryCostPerGBMonthly <= 0 {
		t.Error("内存成本应大于0")
	}
	if config.SSDCostPerGBMonthly <= 0 {
		t.Error("SSD成本应大于0")
	}
	if config.ExpectedDedupRate <= 0 || config.ExpectedDedupRate > 100 {
		t.Errorf("预期去重率异常: %.1f%%", config.ExpectedDedupRate)
	}
	if config.AvgChunkSizeBytes <= 0 {
		t.Error("平均块大小应大于0")
	}
}

func TestBenefitScoreCalculation(t *testing.T) {
	config := DefaultDedupCostConfig()

	tests := []struct {
		dedupRate  float64
		dataSizeTB float64
		minScore   float64
	}{
		{50, 100, 60}, // 高去重率大数据量
		{30, 10, 30},  // 中等场景
		{15, 5, 10},   // 低去重率小数据量
	}

	for _, tt := range tests {
		config.TotalDataBytes = uint64(tt.dataSizeTB * 1024 * 1024 * 1024 * 1024)
		config.ExpectedDedupRate = tt.dedupRate

		calc := NewDedupROICalculator(config)
		analysis := calc.Analyze()

		if analysis.BenefitScore < tt.minScore {
			t.Errorf("%.1fTB %.1f%%去重 效益评分 %.1f 低于预期 %.1f",
				tt.dataSizeTB, tt.dedupRate, analysis.BenefitScore, tt.minScore)
		}

		t.Logf("%.1fTB %.1f%%去重 效益评分: %.1f",
			tt.dataSizeTB, tt.dedupRate, analysis.BenefitScore)
	}
}

func TestThresholdCalculation(t *testing.T) {
	config := DefaultDedupCostConfig()
	config.TotalDataBytes = 20 * 1024 * 1024 * 1024 * 1024

	calc := NewDedupROICalculator(config)
	analysis := calc.Analyze()

	if analysis.RecommendedDedupThreshold < 10 {
		t.Errorf("threshold too low: %.1f%%", analysis.RecommendedDedupThreshold)
	}
	if analysis.RecommendedDedupThreshold > 60 {
		t.Errorf("threshold too high: %.1f%%", analysis.RecommendedDedupThreshold)
	}

	t.Logf("recommended dedup threshold: %.1f%%", analysis.RecommendedDedupThreshold)
}

func dedupContains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(len(s) >= len(substr) && s[:len(substr)] == substr ||
			len(s) > len(substr) && dedupContains(s[1:], substr))
}
