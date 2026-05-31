package storagecostanalyzer

import (
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
			Name:           "SSD高速层",
			CostPerTBMonth: 500,
			CapacityTB:     10,
			UsedTB:         7,
			ReadIOPS:       100000,
			WriteIOPS:      80000,
			ThroughputMBps: 3000,
			LatencyMs:      0.1,
			Durability:     "99.999999999%",
			AvailabilitySLA: 99.99,
		}},
		{TierHDD, TierConfig{
			Name:           "HDD容量层",
			CostPerTBMonth: 100,
			CapacityTB:     50,
			UsedTB:         30,
			ReadIOPS:       200,
			WriteIOPS:      150,
			ThroughputMBps: 200,
			LatencyMs:      10,
			Durability:     "99.999999999%",
			AvailabilitySLA: 99.9,
		}},
		{TierCold, TierConfig{
			Name:           "冷存储归档层",
			CostPerTBMonth: 20,
			CapacityTB:     100,
			UsedTB:         40,
			ReadIOPS:       50,
			WriteIOPS:      30,
			ThroughputMBps: 100,
			LatencyMs:      100,
			Durability:     "99.999999999%",
			AvailabilitySLA: 99.0,
		}},
		{TierCloud, TierConfig{
			Name:           "云存储层",
			CostPerTBMonth: 250,
			CapacityTB:     20,
			UsedTB:         8,
			ReadIOPS:       5000,
			WriteIOPS:      3000,
			ThroughputMBps: 500,
			LatencyMs:      5,
			Durability:     "99.999999999%",
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
