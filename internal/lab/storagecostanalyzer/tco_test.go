package storagecostanalyzer

import (
	"testing"
	"time"
)

func TestNewTCOEngine(t *testing.T) {
	config := &Config{
		Enabled:              true,
		Currency:             "CNY",
		ReportRetentionDays:  90,
		ForecastMonths:       12,
		AlertThreshold:       80.0,
		AutoAnalyze:          false,
		AnalyzeIntervalHours: 24,
	}

	manager := NewManager(config)
	engine := NewTCOEngine(manager)

	if engine == nil {
		t.Fatal("NewTCOEngine returned nil")
	}
	if engine.manager != manager {
		t.Error("Engine manager mismatch")
	}
}

func TestDefaultTCOInput(t *testing.T) {
	input := DefaultTCOInput(TierSSD, 12)

	if input.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, input.Tier)
	}
	if input.AnalysisMonths != 12 {
		t.Errorf("Expected 12 months, got %d", input.AnalysisMonths)
	}
	if input.ElectricityPrice != 0.8 {
		t.Errorf("Expected electricity price 0.8, got %f", input.ElectricityPrice)
	}
	if input.PowerPerTBW != 8.0 {
		t.Errorf("Expected power per TB 8.0, got %f", input.PowerPerTBW)
	}
}

// TestTCOEngine_CalculateTCO 测试 TCO 计算引擎.
func TestTCOEngine_CalculateTCO(t *testing.T) {
	config := &Config{
		Enabled:        true,
		Currency:       "CNY",
		ForecastMonths: 12,
	}

	manager := NewManager(config)
	engine := NewTCOEngine(manager)

	// 注册存储层级
	err := manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD存储",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         5,
	})
	if err != nil {
		t.Fatalf("Failed to register tier: %v", err)
	}

	input := DefaultTCOInput(TierSSD, 12)
	result, err := engine.CalculateTCO(input)

	if err != nil {
		t.Fatalf("CalculateTCO failed: %v", err)
	}
	if result == nil {
		t.Fatal("CalculateTCO returned nil")
	}

	// 验证基本字段
	if result.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, result.Tier)
	}
	if result.AnalysisPeriodMonths != 12 {
		t.Errorf("Expected 12 months, got %d", result.AnalysisPeriodMonths)
	}
	if result.TotalTCO <= 0 {
		t.Error("TotalTCO should be positive")
	}
	if result.HardwareCost <= 0 {
		t.Error("HardwareCost should be positive")
	}
	if result.PowerCost <= 0 {
		t.Error("PowerCost should be positive")
	}

	// 验证成本明细
	if len(result.CostBreakdown) == 0 {
		t.Error("CostBreakdown should not be empty")
	}

	// 验证占比总和为100%
	totalPercentage := 0.0
	for _, item := range result.CostBreakdown {
		totalPercentage += item.Percentage
	}
	if totalPercentage < 99.9 || totalPercentage > 100.1 {
		t.Errorf("Cost breakdown percentages should sum to ~100%%, got %f", totalPercentage)
	}

	// 验证年度预测
	if len(result.YearlyProjection) == 0 {
		t.Error("YearlyProjection should not be empty")
	}
}

func TestCalculateTCOWithOverride(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	engine := NewTCOEngine(manager)

	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD存储",
		CostPerTBMonth: 50,
		CapacityTB:     100,
		UsedTB:         80,
	})

	input := DefaultTCOInput(TierHDD, 24)
	hardwareOverride := 50000.0
	input.HardwareCostOverride = &hardwareOverride

	result, err := engine.CalculateTCO(input)
	if err != nil {
		t.Fatalf("CalculateTCO failed: %v", err)
	}

	if result.HardwareCost != 50000.0 {
		t.Errorf("Expected hardware cost 50000, got %f", result.HardwareCost)
	}
}

func TestCompareTCO(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	engine := NewTCOEngine(manager)

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         5,
	})

	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD",
		CostPerTBMonth: 30,
		CapacityTB:     50,
		UsedTB:         40,
	})

	result, err := engine.CompareTCO(TierSSD, TierHDD, 12)
	if err != nil {
		t.Fatalf("CompareTCO failed: %v", err)
	}

	if result.TCO1 == nil || result.TCO2 == nil {
		t.Error("TCO comparison results should not be nil")
	}
	if result.AnalysisPeriod != 12 {
		t.Errorf("Expected analysis period 12, got %d", result.AnalysisPeriod)
	}
}

func TestCalculateBreakEven(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	engine := NewTCOEngine(manager)

	result, err := engine.CalculateBreakEven(TierSSD, 10000, 1000)
	if err != nil {
		t.Fatalf("CalculateBreakEven failed: %v", err)
	}

	if result.InvestmentCost != 10000 {
		t.Errorf("Expected investment 10000, got %f", result.InvestmentCost)
	}
	if result.MonthlySavings != 1000 {
		t.Errorf("Expected monthly savings 1000, got %f", result.MonthlySavings)
	}
	if result.BreakEvenMonths != 10 {
		t.Errorf("Expected break-even 10 months, got %d", result.BreakEvenMonths)
	}
	if result.FiveYearROI <= 0 {
		t.Error("Five-year ROI should be positive")
	}
}

func TestProjectTCO(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	engine := NewTCOEngine(manager)

	now := time.Now()
	manager.nowFunc = func() time.Time { return now }

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     20,
		UsedTB:         10,
	})

	result, err := engine.ProjectTCO(TierSSD, 12, 5.0)
	if err != nil {
		t.Fatalf("ProjectTCO failed: %v", err)
	}

	if result.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, result.Tier)
	}
	if len(result.ProjectedPoints) != 12 {
		t.Errorf("Expected 12 projected points, got %d", len(result.ProjectedPoints))
	}
	if result.TotalProjectedCost <= 0 {
		t.Error("Total projected cost should be positive")
	}
}
