package storagecostanalyzer

import (
	"math"
	"testing"
	"time"
)

func newTestManager() *Manager {
	cfg := &Config{
		Enabled:              true,
		Currency:             "CNY",
		ReportRetentionDays:  90,
		ForecastMonths:       12,
		AlertThreshold:       80.0,
		AutoAnalyze:          false, // 测试不启动自动分析
		AnalyzeIntervalHours: 24,
	}
	return NewManager(cfg)
}

func registerTestTiers(t *testing.T, m *Manager) {
	t.Helper()
	tiers := []struct {
		tier StorageTier
		cfg  TierConfig
	}{
		{TierSSD, TierConfig{
			Name:            "SSD高速层",
			CostPerTBMonth:  500,
			CapacityTB:      10,
			UsedTB:          7,
			ReadIOPS:        100000,
			WriteIOPS:       80000,
			ThroughputMBps:  3000,
			LatencyMs:       0.1,
			Durability:      "99.999999999%",
			AvailabilitySLA: 99.99,
		}},
		{TierHDD, TierConfig{
			Name:            "HDD容量层",
			CostPerTBMonth:  100,
			CapacityTB:      50,
			UsedTB:          30,
			ReadIOPS:        200,
			WriteIOPS:       150,
			ThroughputMBps:  200,
			LatencyMs:       10,
			Durability:      "99.999999999%",
			AvailabilitySLA: 99.9,
		}},
		{TierCold, TierConfig{
			Name:            "冷存储归档层",
			CostPerTBMonth:  20,
			CapacityTB:      100,
			UsedTB:          40,
			ReadIOPS:        50,
			WriteIOPS:       30,
			ThroughputMBps:  100,
			LatencyMs:       100,
			Durability:      "99.999999999%",
			AvailabilitySLA: 99.0,
		}},
		{TierCloud, TierConfig{
			Name:            "云存储层",
			CostPerTBMonth:  250,
			CapacityTB:      20,
			UsedTB:          8,
			ReadIOPS:        5000,
			WriteIOPS:       3000,
			ThroughputMBps:  500,
			LatencyMs:       5,
			Durability:      "99.999999999%",
			AvailabilitySLA: 99.95,
		}},
	}
	for _, t2 := range tiers {
		if err := m.RegisterTier(t2.tier, t2.cfg); err != nil {
			t.Fatalf("RegisterTier(%s) failed: %v", t2.tier, err)
		}
	}
}

func TestNewManager(t *testing.T) {
	m := newTestManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config == nil {
		t.Fatal("config is nil")
	}
	if m.config.Currency != "CNY" {
		t.Errorf("expected currency CNY, got %s", m.config.Currency)
	}
	if len(m.tiers) != 0 {
		t.Errorf("expected 0 tiers, got %d", len(m.tiers))
	}

	// 测试 nil config
	m2 := NewManager(nil)
	if m2 == nil {
		t.Fatal("NewManager(nil) returned nil")
	}
	if !m2.config.Enabled {
		t.Error("default config should be enabled")
	}
}

func TestManagerStartStop(t *testing.T) {
	m := newTestManager()

	// 初始状态未运行
	if m.IsRunning() {
		t.Error("should not be running initially")
	}

	// 启动
	if err := m.Start(); err != nil {
		t.Fatalf("Start() failed: %v", err)
	}
	if !m.IsRunning() {
		t.Error("should be running after Start()")
	}

	// 重复启动应报错
	if err := m.Start(); err != ErrAlreadyRunning {
		t.Errorf("expected ErrAlreadyRunning, got %v", err)
	}

	// 停止
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop() failed: %v", err)
	}
	if m.IsRunning() {
		t.Error("should not be running after Stop()")
	}

	// 重复停止应报错
	if err := m.Stop(); err != ErrNotRunning {
		t.Errorf("expected ErrNotRunning, got %v", err)
	}

	// 测试 disabled config
	mDisabled := NewManager(&Config{Enabled: false})
	if err := mDisabled.Start(); err != ErrInvalidConfig {
		t.Errorf("expected ErrInvalidConfig for disabled config, got %v", err)
	}
}

func TestRegisterTier(t *testing.T) {
	m := newTestManager()

	// 正常注册
	cfg := TierConfig{
		Name:           "测试SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         5,
	}
	if err := m.RegisterTier(TierSSD, cfg); err != nil {
		t.Fatalf("RegisterTier failed: %v", err)
	}
	if len(m.tiers) != 1 {
		t.Errorf("expected 1 tier, got %d", len(m.tiers))
	}

	// 重复注册覆盖
	cfg2 := TierConfig{
		Name:           "测试SSD-v2",
		CostPerTBMonth: 120,
		CapacityTB:     20,
		UsedTB:         10,
	}
	if err := m.RegisterTier(TierSSD, cfg2); err != nil {
		t.Fatalf("re-register should succeed: %v", err)
	}
	if len(m.tiers) != 1 {
		t.Errorf("expected 1 tier after re-register, got %d", len(m.tiers))
	}

	// 无效配置：容量为0
	if err := m.RegisterTier(TierHDD, TierConfig{CapacityTB: 0}); err == nil {
		t.Error("expected error for zero capacity")
	}

	// 无效配置：负成本
	if err := m.RegisterTier(TierHDD, TierConfig{CapacityTB: 10, CostPerTBMonth: -1}); err == nil {
		t.Error("expected error for negative cost")
	}

	// 无效配置：已用超过容量
	if err := m.RegisterTier(TierHDD, TierConfig{CapacityTB: 10, UsedTB: 20, CostPerTBMonth: 100}); err == nil {
		t.Error("expected error when used > capacity")
	}
}

func TestRecordCost(t *testing.T) {
	m := newTestManager()
	registerTestTiers(t, m)

	// 记录成本
	if err := m.RecordCost(TierSSD, CategoryHardware, 1000); err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}
	if err := m.RecordCost(TierSSD, CategoryPower, 200); err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}
	if err := m.RecordCost(TierHDD, CategoryHardware, 500); err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}

	// 检查记录数
	if len(m.records) != 3 {
		t.Errorf("expected 3 records, got %d", len(m.records))
	}

	// 检查 ID 唯一性
	ids := make(map[string]bool)
	for _, r := range m.records {
		if ids[r.ID] {
			t.Errorf("duplicate ID: %s", r.ID)
		}
		ids[r.ID] = true
	}

	// 不存在的层级
	if err := m.RecordCost("nonexistent", CategoryHardware, 100); err != ErrTierNotFound {
		t.Errorf("expected ErrTierNotFound, got %v", err)
	}
}

func TestCalculateCostPerTB(t *testing.T) {
	m := newTestManager()
	registerTestTiers(t, m)

	// 无记录时使用配置值
	costPerTB, err := m.CalculateCostPerTB(TierSSD)
	if err != nil {
		t.Fatalf("CalculateCostPerTB failed: %v", err)
	}
	if costPerTB != 500 {
		t.Errorf("expected 500 (from config), got %f", costPerTB)
	}

	// 记录成本后计算
	m.RecordCost(TierSSD, CategoryHardware, 5000) // 5000 / 10 = 500 per TB
	m.RecordCost(TierSSD, CategoryPower, 1000)    // 总计 6000 / 10 = 600 per TB

	costPerTB, err = m.CalculateCostPerTB(TierSSD)
	if err != nil {
		t.Fatalf("CalculateCostPerTB failed: %v", err)
	}
	expected := 6000.0 / 10.0
	if costPerTB != expected {
		t.Errorf("expected %f, got %f", expected, costPerTB)
	}

	// 不存在的层级
	_, err = m.CalculateCostPerTB("nonexistent")
	if err != ErrTierNotFound {
		t.Errorf("expected ErrTierNotFound, got %v", err)
	}
}

func TestPredictCapacity(t *testing.T) {
	m := newTestManager()

	// 模拟时间
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	m.nowFunc = func() time.Time { return baseTime }

	registerTestTiers(t, m)

	// 添加一些历史成本记录用于增长估算
	m.RecordCost(TierSSD, CategoryHardware, 3000)
	m.RecordCost(TierHDD, CategoryHardware, 2000)

	// 预测12个月
	trend, err := m.PredictCapacity(12)
	if err != nil {
		t.Fatalf("PredictCapacity failed: %v", err)
	}

	if trend == nil {
		t.Fatal("trend is nil")
	}
	if trend.TotalUsedTB <= 0 {
		t.Errorf("expected positive total used, got %f", trend.TotalUsedTB)
	}
	if trend.TotalCapacityTB <= 0 {
		t.Errorf("expected positive total capacity, got %f", trend.TotalCapacityTB)
	}
	if len(trend.Months) != 13 { // 0..12 个月 = 13 个点
		t.Errorf("expected 13 data points, got %d", len(trend.Months))
	}

	// 使用默认月数
	trend2, err := m.PredictCapacity(0)
	if err != nil {
		t.Fatalf("PredictCapacity(0) failed: %v", err)
	}
	if len(trend2.Months) != 13 { // 默认 ForecastMonths=12
		t.Errorf("expected 13 data points for default forecast, got %d", len(trend2.Months))
	}

	// 空管理器
	m2 := newTestManager()
	_, err = m2.PredictCapacity(6)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestGenerateReport(t *testing.T) {
	m := newTestManager()
	baseTime := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	m.nowFunc = func() time.Time { return baseTime }

	registerTestTiers(t, m)

	// 添加成本记录
	m.RecordCost(TierSSD, CategoryHardware, 5000)
	m.RecordCost(TierSSD, CategoryPower, 800)
	m.RecordCost(TierHDD, CategoryHardware, 3000)
	m.RecordCost(TierCold, CategoryHardware, 800)

	// 月度报告
	report, err := m.GenerateReport("monthly")
	if err != nil {
		t.Fatalf("GenerateReport(monthly) failed: %v", err)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if report.ReportType != "monthly" {
		t.Errorf("expected report type monthly, got %s", report.ReportType)
	}
	if report.TotalCost <= 0 {
		t.Error("expected positive total cost")
	}
	if report.TotalCapacityTB <= 0 {
		t.Error("expected positive total capacity")
	}
	if len(report.TierBreakdown) == 0 {
		t.Error("expected tier breakdown")
	}
	if report.GeneratedAt.IsZero() {
		t.Error("expected non-zero generated time")
	}

	// 季度报告
	reportQ, err := m.GenerateReport("quarterly")
	if err != nil {
		t.Fatalf("GenerateReport(quarterly) failed: %v", err)
	}
	if reportQ.ReportType != "quarterly" {
		t.Errorf("expected quarterly, got %s", reportQ.ReportType)
	}

	// 年度报告
	reportY, err := m.GenerateReport("yearly")
	if err != nil {
		t.Fatalf("GenerateReport(yearly) failed: %v", err)
	}
	if reportY.ReportType != "yearly" {
		t.Errorf("expected yearly, got %s", reportY.ReportType)
	}

	// 无效报告类型
	_, err = m.GenerateReport("weekly")
	if err == nil {
		t.Error("expected error for unsupported period")
	}
}

func TestGetOptimizationSuggestions(t *testing.T) {
	m := newTestManager()
	registerTestTiers(t, m)

	suggestions := m.GetOptimizationSuggestions()

	// SSD 利用率 70% (< 80%)，不触发扩容建议
	// SSD 利用率 70% (> 30%)，不触发分层建议
	// Cloud 成本 250 (> 200)，应触发云优化建议
	if len(suggestions) == 0 {
		t.Error("expected at least one suggestion for cloud optimization")
	}

	foundCloud := false
	for _, s := range suggestions {
		if s.Category == "migration" && s.SourceTier == TierCloud {
			foundCloud = true
			if s.AnnualSavings <= 0 {
				t.Error("expected positive annual savings for cloud optimization")
			}
		}
		if s.ID == "" {
			t.Error("suggestion ID should not be empty")
		}
		if s.Title == "" {
			t.Error("suggestion title should not be empty")
		}
		if s.Description == "" {
			t.Error("suggestion description should not be empty")
		}
	}
	if !foundCloud {
		t.Error("expected cloud cost optimization suggestion")
	}

	// 测试高利用率场景
	m2 := newTestManager()
	m2.RegisterTier(TierSSD, TierConfig{
		Name:           "高利用率SSD",
		CostPerTBMonth: 500,
		CapacityTB:     10,
		UsedTB:         9.5, // 95% 利用率
	})
	suggestions2 := m2.GetOptimizationSuggestions()
	foundExpansion := false
	for _, s := range suggestions2 {
		if s.Category == "rightsizing" {
			foundExpansion = true
		}
	}
	if !foundExpansion {
		t.Error("expected rightsizing suggestion for high utilization")
	}

	// 测试低利用率 SSD 场景
	m3 := newTestManager()
	m3.RegisterTier(TierSSD, TierConfig{
		Name:           "低利用率SSD",
		CostPerTBMonth: 500,
		CapacityTB:     100,
		UsedTB:         10, // 10% 利用率
	})
	suggestions3 := m3.GetOptimizationSuggestions()
	foundTiering := false
	for _, s := range suggestions3 {
		if s.Category == "tiering" && s.SourceTier == TierSSD {
			foundTiering = true
			if s.AnnualSavings <= 0 {
				t.Error("expected positive savings for tiering suggestion")
			}
		}
	}
	if !foundTiering {
		t.Error("expected tiering suggestion for low utilization SSD")
	}
}

func TestCalculateTCO(t *testing.T) {
	m := newTestManager()
	registerTestTiers(t, m)

	// 计算SSD层级的TCO（12个月）
	tco, err := m.CalculateTCO(TierSSD, 12)
	if err != nil {
		t.Fatalf("CalculateTCO failed: %v", err)
	}
	if tco == nil {
		t.Fatal("tco is nil")
	}

	// 验证基本字段
	if tco.Tier != TierSSD {
		t.Errorf("expected tier SSD, got %s", tco.Tier)
	}
	if tco.TierName != "SSD高速层" {
		t.Errorf("expected tier name SSD高速层, got %s", tco.TierName)
	}
	if tco.AnalysisPeriodMonths != 12 {
		t.Errorf("expected 12 months, got %d", tco.AnalysisPeriodMonths)
	}

	// 验证成本为正
	if tco.TotalTCO <= 0 {
		t.Errorf("expected positive TCO, got %f", tco.TotalTCO)
	}
	if tco.HardwareCost <= 0 {
		t.Errorf("expected positive hardware cost, got %f", tco.HardwareCost)
	}
	if tco.PowerCost <= 0 {
		t.Errorf("expected positive power cost, got %f", tco.PowerCost)
	}
	if tco.SubscriptionCost <= 0 {
		t.Errorf("expected positive subscription cost, got %f", tco.SubscriptionCost)
	}

	// 验证每TB成本
	if tco.CostPerTBPerMonth <= 0 {
		t.Errorf("expected positive cost per TB per month, got %f", tco.CostPerTBPerMonth)
	}
	if tco.CostPerTBPerYear <= 0 {
		t.Errorf("expected positive cost per TB per year, got %f", tco.CostPerTBPerYear)
	}

	// 验证成本明细
	if len(tco.CostBreakdown) == 0 {
		t.Error("expected cost breakdown")
	}
	totalBreakdown := 0.0
	for _, item := range tco.CostBreakdown {
		totalBreakdown += item.Amount
		if item.Percentage < 0 || item.Percentage > 100 {
			t.Errorf("invalid percentage: %f", item.Percentage)
		}
	}
	// 成本明细总和应近似等于总TCO
	if math.Abs(totalBreakdown-tco.TotalTCO)/tco.TotalTCO > 0.01 {
		t.Errorf("breakdown sum %f differs from TCO %f by more than 1%%", totalBreakdown, tco.TotalTCO)
	}

	// 验证年度预测
	if len(tco.YearlyProjection) == 0 {
		t.Error("expected yearly projection")
	}
	for _, yp := range tco.YearlyProjection {
		if yp.TotalCost <= 0 {
			t.Errorf("expected positive yearly cost for year %d", yp.Year)
		}
		if yp.CumulativeCost <= 0 {
			t.Errorf("expected positive cumulative cost for year %d", yp.Year)
		}
	}

	// 测试默认月数
	tcoDefault, err := m.CalculateTCO(TierSSD, 0)
	if err != nil {
		t.Fatalf("CalculateTCO(0) failed: %v", err)
	}
	if tcoDefault.AnalysisPeriodMonths != 12 {
		t.Errorf("expected default 12 months, got %d", tcoDefault.AnalysisPeriodMonths)
	}

	// 测试不存在的层级
	_, err = m.CalculateTCO("nonexistent", 12)
	if err != ErrTierNotFound {
		t.Errorf("expected ErrTierNotFound, got %v", err)
	}
}

func TestCompareStorageOptions(t *testing.T) {
	m := newTestManager()

	options := []StorageOption{
		{
			Name:           "SSD方案",
			Tier:           TierSSD,
			CostPerTBMonth: 500,
			SetupCost:      5000,
			Performance:    "high",
			Durability:     "99.999999999%",
			Scalability:    "medium",
			Pros:           []string{"高性能", "低延迟"},
			Cons:           []string{"成本高", "容量有限"},
		},
		{
			Name:           "HDD方案",
			Tier:           TierHDD,
			CostPerTBMonth: 100,
			SetupCost:      2000,
			Performance:    "medium",
			Durability:     "99.999999999%",
			Scalability:    "high",
			Pros:           []string{"成本低", "容量大"},
			Cons:           []string{"性能一般", "延迟较高"},
		},
		{
			Name:           "云存储方案",
			Tier:           TierCloud,
			CostPerTBMonth: 250,
			SetupCost:      0,
			Performance:    "medium",
			Durability:     "99.999999999%",
			Scalability:    "high",
			Pros:           []string{"弹性扩展", "无需维护"},
			Cons:           []string{"长期成本高", "依赖网络"},
		},
	}

	// 正常对比
	comparison, err := m.CompareStorageOptions(10, options)
	if err != nil {
		t.Fatalf("CompareStorageOptions failed: %v", err)
	}
	if comparison == nil {
		t.Fatal("comparison is nil")
	}

	// 验证方案数
	if len(comparison.Options) != 3 {
		t.Errorf("expected 3 options, got %d", len(comparison.Options))
	}

	// 验证成本对比点
	if len(comparison.CostComparison) != 12 {
		t.Errorf("expected 12 comparison points, got %d", len(comparison.CostComparison))
	}
	for _, cp := range comparison.CostComparison {
		if len(cp.OptionCosts) != 3 {
			t.Errorf("expected 3 option costs, got %d", len(cp.OptionCosts))
		}
	}

	// 验证最优方案索引有效
	if comparison.BestForCost < 0 || comparison.BestForCost >= len(options) {
		t.Errorf("invalid best for cost index: %d", comparison.BestForCost)
	}
	if comparison.BestForPerformance < 0 || comparison.BestForPerformance >= len(options) {
		t.Errorf("invalid best for performance index: %d", comparison.BestForPerformance)
	}
	if comparison.Recommendation < 0 || comparison.Recommendation >= len(options) {
		t.Errorf("invalid recommendation index: %d", comparison.Recommendation)
	}

	// 验证分析说明
	if comparison.Analysis == "" {
		t.Error("expected non-empty analysis")
	}

	// 测试无效输入
	_, err = m.CompareStorageOptions(10, options[:1])
	if err != ErrComparisonFailed {
		t.Errorf("expected ErrComparisonFailed for single option, got %v", err)
	}

	_, err = m.CompareStorageOptions(0, options)
	if err == nil {
		t.Error("expected error for zero capacity")
	}
}

func TestCalculateROI(t *testing.T) {
	m := newTestManager()
	registerTestTiers(t, m)

	// 计算SSD层级的ROI
	investmentCost := 10000.0
	roi, err := m.CalculateROI(TierSSD, investmentCost, 12)
	if err != nil {
		t.Fatalf("CalculateROI failed: %v", err)
	}
	if roi == nil {
		t.Fatal("roi is nil")
	}

	// 验证基本字段
	if roi.Tier != TierSSD {
		t.Errorf("expected tier SSD, got %s", roi.Tier)
	}
	if roi.TotalInvestment != investmentCost {
		t.Errorf("expected investment %f, got %f", investmentCost, roi.TotalInvestment)
	}

	// 验证收益为正
	if roi.TotalBenefits <= 0 {
		t.Errorf("expected positive benefits, got %f", roi.TotalBenefits)
	}
	if roi.CostSavings <= 0 {
		t.Errorf("expected positive cost savings, got %f", roi.CostSavings)
	}
	if roi.EfficiencyGain <= 0 {
		t.Errorf("expected positive efficiency gain, got %f", roi.EfficiencyGain)
	}

	// 验证ROI计算
	if roi.ROIPercent == 0 {
		t.Error("expected non-zero ROI percent")
	}
	if roi.PaybackPeriodMonths <= 0 {
		t.Errorf("expected positive payback months, got %d", roi.PaybackPeriodMonths)
	}

	// 验证收益明细
	if len(roi.BenefitBreakdown) != 3 {
		t.Errorf("expected 3 benefit items, got %d", len(roi.BenefitBreakdown))
	}
	totalBenefitsBreakdown := 0.0
	for _, item := range roi.BenefitBreakdown {
		totalBenefitsBreakdown += item.Amount
		if item.Percentage < 0 || item.Percentage > 100 {
			t.Errorf("invalid percentage: %f", item.Percentage)
		}
	}
	if math.Abs(totalBenefitsBreakdown-roi.TotalBenefits)/roi.TotalBenefits > 0.01 {
		t.Errorf("benefits breakdown sum %f differs from total %f", totalBenefitsBreakdown, roi.TotalBenefits)
	}

	// 测试无效输入
	_, err = m.CalculateROI(TierSSD, 0, 12)
	if err == nil {
		t.Error("expected error for zero investment")
	}

	_, err = m.CalculateROI("nonexistent", 10000, 12)
	if err != ErrTierNotFound {
		t.Errorf("expected ErrTierNotFound, got %v", err)
	}
}

func TestEstimateDataOptimization(t *testing.T) {
	m := newTestManager()
	registerTestTiers(t, m)

	// 测试去重压缩估算
	dedupRatio := 0.3       // 30%去重率
	compressionRatio := 0.4 // 40%压缩率
	estimate, err := m.EstimateDataOptimization(TierSSD, dedupRatio, compressionRatio)
	if err != nil {
		t.Fatalf("EstimateDataOptimization failed: %v", err)
	}
	if estimate == nil {
		t.Fatal("estimate is nil")
	}

	// 验证基本字段
	if estimate.Tier != TierSSD {
		t.Errorf("expected tier SSD, got %s", estimate.Tier)
	}
	if estimate.OriginalDataTB != 7 { // SSD used 7TB
		t.Errorf("expected original 7 TB, got %f", estimate.OriginalDataTB)
	}

	// 验证去重节省
	if estimate.DeduplicationSavingsTB <= 0 {
		t.Errorf("expected positive dedup savings, got %f", estimate.DeduplicationSavingsTB)
	}
	expectedDedupSavings := 7.0 * dedupRatio
	if math.Abs(estimate.DeduplicationSavingsTB-expectedDedupSavings) > 0.01 {
		t.Errorf("expected dedup savings %f, got %f", expectedDedupSavings, estimate.DeduplicationSavingsTB)
	}

	// 验证压缩节省
	if estimate.CompressionSavingsTB <= 0 {
		t.Errorf("expected positive compression savings, got %f", estimate.CompressionSavingsTB)
	}

	// 验证总节省
	if estimate.TotalSavingsTB <= 0 {
		t.Errorf("expected positive total savings, got %f", estimate.TotalSavingsTB)
	}
	if estimate.TotalSavingsTB != estimate.DeduplicationSavingsTB+estimate.CompressionSavingsTB {
		t.Error("total savings should equal dedup + compression savings")
	}

	// 验证优化后数据量
	if estimate.OptimizedDataTB >= estimate.OriginalDataTB {
		t.Error("optimized data should be less than original")
	}

	// 验证空间缩减率
	if estimate.SpaceReductionPercent <= 0 || estimate.SpaceReductionPercent >= 100 {
		t.Errorf("invalid space reduction: %f", estimate.SpaceReductionPercent)
	}

	// 验证成本节省
	if estimate.MonthlyCostSaving <= 0 {
		t.Errorf("expected positive monthly saving, got %f", estimate.MonthlyCostSaving)
	}
	if estimate.AnnualCostSaving <= 0 {
		t.Errorf("expected positive annual saving, got %f", estimate.AnnualCostSaving)
	}
	if estimate.AnnualCostSaving != estimate.MonthlyCostSaving*12 {
		t.Error("annual saving should be 12x monthly")
	}

	// 验证回收期
	if estimate.PaybackMonths < 0 {
		t.Errorf("invalid payback months: %d", estimate.PaybackMonths)
	}

	// 测试无效输入
	_, err = m.EstimateDataOptimization(TierSSD, 1.5, 0.4)
	if err == nil {
		t.Error("expected error for invalid dedup ratio")
	}

	_, err = m.EstimateDataOptimization(TierSSD, 0.3, -0.1)
	if err == nil {
		t.Error("expected error for invalid compression ratio")
	}

	_, err = m.EstimateDataOptimization("nonexistent", 0.3, 0.4)
	if err != ErrTierNotFound {
		t.Errorf("expected ErrTierNotFound, got %v", err)
	}
}

func TestForecastCost(t *testing.T) {
	m := newTestManager()
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	m.nowFunc = func() time.Time { return baseTime }

	registerTestTiers(t, m)

	// 添加历史记录用于增长估算
	m.RecordCost(TierSSD, CategoryHardware, 3000)
	m.RecordCost(TierSSD, CategoryHardware, 3500)
	m.RecordCost(TierHDD, CategoryHardware, 2000)

	// 预测12个月
	forecast, err := m.ForecastCost(12)
	if err != nil {
		t.Fatalf("ForecastCost failed: %v", err)
	}
	if forecast == nil {
		t.Fatal("forecast is nil")
	}

	// 验证基本字段
	if forecast.ForecastMonths != 12 {
		t.Errorf("expected 12 months, got %d", forecast.ForecastMonths)
	}
	if forecast.CurrentMonthlyCost <= 0 {
		t.Errorf("expected positive current cost, got %f", forecast.CurrentMonthlyCost)
	}
	if forecast.TotalForecastCost <= 0 {
		t.Errorf("expected positive total forecast, got %f", forecast.TotalForecastCost)
	}
	if forecast.CostGrowthRate <= 0 {
		t.Errorf("expected positive growth rate, got %f", forecast.CostGrowthRate)
	}

	// 验证预测点
	if len(forecast.ProjectedMonthlyCosts) != 12 {
		t.Errorf("expected 12 forecast points, got %d", len(forecast.ProjectedMonthlyCosts))
	}
	for i, point := range forecast.ProjectedMonthlyCosts {
		if point.Month != i+1 {
			t.Errorf("expected month %d, got %d", i+1, point.Month)
		}
		if point.ProjectedCost <= 0 {
			t.Errorf("expected positive cost for month %d", point.Month)
		}
		if point.LowerBound > point.ProjectedCost {
			t.Errorf("lower bound should be <= projected cost for month %d", point.Month)
		}
		if point.UpperBound < point.ProjectedCost {
			t.Errorf("upper bound should be >= projected cost for month %d", point.Month)
		}
		if point.CumulativeCost <= 0 {
			t.Errorf("expected positive cumulative cost for month %d", point.Month)
		}
	}

	// 验证建议
	if len(forecast.Recommendations) == 0 {
		t.Error("expected recommendations")
	}

	// 验证置信水平
	if forecast.ConfidenceLevel != 95 {
		t.Errorf("expected 95 confidence level, got %f", forecast.ConfidenceLevel)
	}

	// 测试默认月数
	forecastDefault, err := m.ForecastCost(0)
	if err != nil {
		t.Fatalf("ForecastCost(0) failed: %v", err)
	}
	if forecastDefault.ForecastMonths != m.config.ForecastMonths {
		t.Errorf("expected default months %d, got %d", m.config.ForecastMonths, forecastDefault.ForecastMonths)
	}

	// 测试空管理器
	m2 := newTestManager()
	_, err = m2.ForecastCost(6)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}
