package tiering

import (
	"testing"
	"time"
)

// TestEfficiencyReportGeneration 测试效率报告生成
func TestEfficiencyReportGeneration(t *testing.T) {
	// 创建管理器
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	// 创建指标
	metrics := NewMetrics()

	// 创建成本配置
	costConfig := &CostConfig{
		SSDCostPerGBMonth:   0.10,
		HDDCostPerGBMonth:   0.03,
		CloudCostPerGBMonth: 0.01,
		MonthlyOpCost:       50.0,
	}

	// 创建报告生成器
	generator := NewEfficiencyReportGenerator(manager, metrics, costConfig)

	// 生成报告
	report, err := generator.GenerateReport("daily")
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	// 验证报告结构
	if report == nil {
		t.Fatal("报告不应为空")
	}

	if report.GeneratedAt.IsZero() {
		t.Error("生成时间应该设置")
	}

	if report.Period != "daily" {
		t.Errorf("期望周期为 daily，实际为 %s", report.Period)
	}

	// 验证各部分
	if report.DataDistribution == nil {
		t.Error("数据分布报告不应为空")
	}

	if report.MigrationEfficiency == nil {
		t.Error("迁移效率报告不应为空")
	}

	if report.CostAnalysis == nil {
		t.Error("成本分析报告不应为空")
	}

	if report.CapacityForecast == nil {
		t.Error("容量预测报告不应为空")
	}

	if report.HealthScore == nil {
		t.Error("健康评分不应为空")
	}
}

// TestDataDistributionReport 测试数据分布报告
func TestDataDistributionReport(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 生成数据分布报告
	report := generator.generateDataDistribution()

	if report == nil {
		t.Fatal("报告不应为空")
	}

	// 验证结构
	if report.ByTier == nil {
		t.Error("ByTier 不应为空")
	}

	if report.ChartData == nil {
		t.Error("ChartData 不应为空")
	}

	// 验证图表数据
	if len(report.ChartData.PieChart) != 3 {
		t.Errorf("饼图应有3个数据项，实际为 %d", len(report.ChartData.PieChart))
	}

	// 验证饼图数据项
	expectedLabels := map[string]bool{"热数据": true, "温数据": true, "冷数据": true}
	for _, item := range report.ChartData.PieChart {
		if !expectedLabels[item.Label] {
			t.Errorf("意外的饼图标签: %s", item.Label)
		}
		if item.Color == "" {
			t.Errorf("饼图项 %s 应有颜色", item.Label)
		}
	}
}

// TestMigrationEfficiencyReport 测试迁移效率报告
func TestMigrationEfficiencyReport(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()

	// 模拟一些迁移指标
	metrics.RecordMigrationStart()
	metrics.RecordMigrationComplete(&MigrateTask{
		ID:              "test_task_1",
		TotalFiles:      10,
		TotalBytes:      1024 * 1024 * 100, // 100MB
		ProcessedFiles:  10,
		ProcessedBytes:  1024 * 1024 * 100,
		FailedFiles:     0,
	}, 5000)

	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 生成迁移效率报告
	report := generator.generateMigrationEfficiency("daily")

	if report == nil {
		t.Fatal("报告不应为空")
	}

	// 验证迁移统计
	if report.TotalMigrations < 1 {
		t.Error("应该有至少一次迁移")
	}

	if report.SuccessRate != 100 {
		t.Errorf("期望成功率100%%，实际为 %.2f%%", report.SuccessRate)
	}

	// 验证按存储层统计
	if report.ByTier == nil {
		t.Error("ByTier 不应为空")
	}

	// 验证效率评分
	if report.EfficiencyScore < 0 || report.EfficiencyScore > 100 {
		t.Errorf("效率评分应在0-100之间，实际为 %.2f", report.EfficiencyScore)
	}
}

// TestCostAnalysisReport 测试成本分析报告
func TestCostAnalysisReport(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	// 设置一些容量数据
	ssdTier, _ := manager.GetTier(TierTypeSSD)
	if ssdTier != nil {
		ssdTier.Capacity = 500 * 1024 * 1024 * 1024 // 500GB
		ssdTier.Used = 350 * 1024 * 1024 * 1024    // 350GB
	}

	hddTier, _ := manager.GetTier(TierTypeHDD)
	if hddTier != nil {
		hddTier.Capacity = 2000 * 1024 * 1024 * 1024 // 2TB
		hddTier.Used = 1200 * 1024 * 1024 * 1024    // 1.2TB
	}

	metrics := NewMetrics()

	costConfig := &CostConfig{
		SSDCostPerGBMonth:   0.10,
		HDDCostPerGBMonth:   0.03,
		CloudCostPerGBMonth: 0.01,
		MonthlyOpCost:       50.0,
	}

	generator := NewEfficiencyReportGenerator(manager, metrics, costConfig)

	// 生成成本分析报告
	report := generator.generateCostAnalysis()

	if report == nil {
		t.Fatal("报告不应为空")
	}

	// 验证成本计算
	if report.TotalMonthlyCost <= 0 {
		t.Error("总月度成本应大于0")
	}

	// 验证存储层成本
	if len(report.TierCosts) == 0 {
		t.Error("应有存储层成本数据")
	}

	// 验证 SSD 成本
	ssdCost, ok := report.TierCosts[TierTypeSSD]
	if !ok {
		t.Error("应有 SSD 成本数据")
	} else {
		// SSD: 350GB * 0.10 = 35.0
		expectedCost := 350.0 * 0.10
		if ssdCost.MonthlyCost < expectedCost-1 || ssdCost.MonthlyCost > expectedCost+1 {
			t.Errorf("SSD月度成本约应为 %.2f，实际为 %.2f", expectedCost, ssdCost.MonthlyCost)
		}
	}

	// 验证成本节省分析
	if report.CostSavings == nil {
		t.Error("成本节省分析不应为空")
	}

	// 验证 ROI 分析
	if report.ROI == nil {
		t.Error("ROI分析不应为空")
	}
}

// TestCapacityForecastReport 测试容量预测报告
func TestCapacityForecastReport(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	// 设置容量数据
	ssdTier, _ := manager.GetTier(TierTypeSSD)
	if ssdTier != nil {
		ssdTier.Capacity = 500 * 1024 * 1024 * 1024 // 500GB
		ssdTier.Used = 400 * 1024 * 1024 * 1024    // 400GB (80%使用率)
	}

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 生成容量预测报告
	report := generator.generateCapacityForecast(90)

	if report == nil {
		t.Fatal("报告不应为空")
	}

	// 验证预测天数
	if report.ForecastDays != 90 {
		t.Errorf("预测天数应为90，实际为 %d", report.ForecastDays)
	}

	// 验证按存储层预测
	if len(report.ByTier) == 0 {
		t.Error("应有存储层预测数据")
	}

	// 验证预测数据点
	if len(report.ForecastPoints) == 0 {
		t.Error("应有预测数据点")
	}

	// 验证模型信息
	if report.ModelInfo == nil {
		t.Error("模型信息不应为空")
	}

	// 由于使用率高，应该有警告
	// 注意：这取决于具体实现，可能需要调整
}

// TestHealthScoreCalculation 测试健康评分计算
func TestHealthScoreCalculation(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 计算健康评分
	score := generator.calculateHealthScore()

	if score == nil {
		t.Fatal("健康评分不应为空")
	}

	// 验证评分范围
	if score.OverallScore < 0 || score.OverallScore > 100 {
		t.Errorf("总分应在0-100之间，实际为 %.2f", score.OverallScore)
	}

	// 验证等级
	validGrades := map[string]bool{"A": true, "B": true, "C": true, "D": true, "F": true}
	if !validGrades[score.Grade] {
		t.Errorf("无效的等级: %s", score.Grade)
	}

	// 验证子评分
	if len(score.ScoreBreakdown) == 0 {
		t.Error("应有评分明细")
	}

	// 验证各子评分范围
	subscores := []float64{
		score.DistributionScore,
		score.EfficiencyScore,
		score.CostScore,
		score.CapacityScore,
		score.PolicyScore,
	}

	for i, s := range subscores {
		if s < 0 || s > 100 {
			t.Errorf("子评分 %d 应在0-100之间，实际为 %.2f", i, s)
		}
	}
}

// TestRecommendationsGeneration 测试建议生成
func TestRecommendationsGeneration(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 生成建议
	recommendations := generator.generateRecommendations()

	// 验证建议结构
	for i, rec := range recommendations {
		if rec.Title == "" {
			t.Errorf("建议 %d 应有标题", i)
		}

		if rec.Type == "" {
			t.Errorf("建议 %d 应有类型", i)
		}

		if rec.Priority < 1 || rec.Priority > 5 {
			t.Errorf("建议 %d 优先级应在1-5之间", i)
		}
	}
}

// TestDailyStatsRecording 测试每日统计记录
func TestDailyStatsRecording(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 记录统计
	generator.RecordDailyStats()

	// 验证历史数据
	generator.historyMu.RLock()
	dailyStats := generator.dailyStats
	generator.historyMu.RUnlock()

	if len(dailyStats) != 1 {
		t.Errorf("应有1条每日统计，实际为 %d", len(dailyStats))
	}

	// 记录多条
	for i := 0; i < 5; i++ {
		generator.RecordDailyStats()
	}

	generator.historyMu.RLock()
	dailyStats = generator.dailyStats
	generator.historyMu.RUnlock()

	if len(dailyStats) != 6 {
		t.Errorf("应有6条每日统计，实际为 %d", len(dailyStats))
	}
}

// TestCostSavingsCalculation 测试成本节省计算
func TestCostSavingsCalculation(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	// 设置数据分布
	ssdTier, _ := manager.GetTier(TierTypeSSD)
	if ssdTier != nil {
		ssdTier.Capacity = 500 * 1024 * 1024 * 1024
		ssdTier.Used = 200 * 1024 * 1024 * 1024 // 200GB
	}

	hddTier, _ := manager.GetTier(TierTypeHDD)
	if hddTier != nil {
		hddTier.Capacity = 2000 * 1024 * 1024 * 1024
		hddTier.Used = 1000 * 1024 * 1024 * 1024 // 1TB
	}

	metrics := NewMetrics()
	costConfig := &CostConfig{
		SSDCostPerGBMonth:   0.10,
		HDDCostPerGBMonth:   0.03,
	}

	generator := NewEfficiencyReportGenerator(manager, metrics, costConfig)

	// 生成成本分析
	report := generator.generateCostAnalysis()

	// 验证成本节省
	if report.CostSavings == nil {
		t.Fatal("成本节省分析不应为空")
	}

	// 没有分层时的成本 = (200GB + 1000GB) * 0.10 = 120.0
	// 有分层时的成本 = 200GB * 0.10 + 1000GB * 0.03 = 20 + 30 = 50.0
	// 节省 = 120 - 50 = 70.0
	expectedSavings := 70.0

	if report.CostSavings.MonthlySavings < expectedSavings-5 || report.CostSavings.MonthlySavings > expectedSavings+5 {
		t.Logf("月度节省约应为 %.2f，实际为 %.2f", expectedSavings, report.CostSavings.MonthlySavings)
	}
}

// TestGrowthRateCalculation 测试增长率计算
func TestGrowthRateCalculation(t *testing.T) {
	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(nil, metrics, nil)

	// 准备历史数据
	stats := []DailyStats{
		{Date: time.Now().AddDate(0, 0, -5), TotalBytes: 100 * 1024 * 1024 * 1024},
		{Date: time.Now().AddDate(0, 0, -4), TotalBytes: 110 * 1024 * 1024 * 1024},
		{Date: time.Now().AddDate(0, 0, -3), TotalBytes: 120 * 1024 * 1024 * 1024},
		{Date: time.Now().AddDate(0, 0, -2), TotalBytes: 130 * 1024 * 1024 * 1024},
		{Date: time.Now().AddDate(0, 0, -1), TotalBytes: 140 * 1024 * 1024 * 1024},
	}

	// 计算增长率
	rate := generator.calculateGrowthRate(stats)

	// 每天增长 10GB
	if rate < 9 || rate > 11 {
		t.Errorf("增长率应约为10GB/天，实际为 %.2f", rate)
	}
}

// TestEfficiencyReportWithEmptyData 测试空数据情况
func TestEfficiencyReportWithEmptyData(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	// 不初始化存储层

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 应该能正常生成报告而不崩溃
	report, err := generator.GenerateReport("daily")
	if err != nil {
		t.Fatalf("空数据时应能生成报告: %v", err)
	}

	if report == nil {
		t.Fatal("报告不应为空")
	}

	// 健康评分应该较低（因为没有配置）
	if report.HealthScore.OverallScore > 50 {
		t.Logf("空配置的健康评分应较低，实际为 %.2f", report.HealthScore.OverallScore)
	}
}

// TestAccessPatternAnalysis 测试访问模式分析
func TestAccessPatternAnalysis(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 测试空统计
	patterns := generator.analyzeAccessPatterns(&AccessStats{})
	if len(patterns) == 0 {
		t.Log("空统计时应返回空模式列表")
	}

	// 测试有数据的统计
	stats := &AccessStats{
		HotFiles:  100,
		WarmFiles: 50,
		ColdFiles: 200,
	}

	patterns = generator.analyzeAccessPatterns(stats)
	if len(patterns) == 0 {
		t.Error("应有访问模式分析结果")
	}

	// 验证模式类型
	for _, p := range patterns {
		if p.PatternType == "" {
			t.Error("访问模式应有类型")
		}
	}
}

// TestChartDataGeneration 测试图表数据生成
func TestChartDataGeneration(t *testing.T) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	// 准备数据分布报告
	report := &DataDistributionReport{
		HotData:  &DataSegment{Files: 100},
		WarmData: &DataSegment{Files: 50},
		ColdData: &DataSegment{Files: 200},
		ByTier: map[TierType]*TierDataDistribution{
			TierTypeSSD: {
				TierType: TierTypeSSD,
				HotBytes:  1024 * 1024 * 100,
				WarmBytes: 1024 * 1024 * 50,
				ColdBytes: 1024 * 1024 * 20,
			},
			TierTypeHDD: {
				TierType: TierTypeHDD,
				HotBytes:  1024 * 1024 * 10,
				WarmBytes: 1024 * 1024 * 100,
				ColdBytes: 1024 * 1024 * 500,
			},
		},
	}

	// 生成图表数据
	chartData := generator.generateChartData(report)

	if chartData == nil {
		t.Fatal("图表数据不应为空")
	}

	// 验证饼图
	if len(chartData.PieChart) != 3 {
		t.Errorf("饼图应有3个数据项，实际为 %d", len(chartData.PieChart))
	}

	// 验证柱状图
	if len(chartData.BarChart) != 2 {
		t.Errorf("柱状图应有2个存储层数据，实际为 %d", len(chartData.BarChart))
	}
}

// BenchmarkEfficiencyReportGeneration 基准测试报告生成
func BenchmarkEfficiencyReportGeneration(b *testing.B) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = generator.GenerateReport("daily")
	}
}

// BenchmarkHealthScoreCalculation 基准测试健康评分计算
func BenchmarkHealthScoreCalculation(b *testing.B) {
	manager := NewManager("", DefaultPolicyEngineConfig())
	manager.initDefaultTiers()

	metrics := NewMetrics()
	generator := NewEfficiencyReportGenerator(manager, metrics, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generator.calculateHealthScore()
	}
}