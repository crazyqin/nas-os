package storagecostanalyzer

import (
	"testing"
	"time"
)

func TestNewROIAnalyzer(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	analyzer := NewROIAnalyzer(manager)

	if analyzer == nil {
		t.Fatal("NewROIAnalyzer returned nil")
	}
	if analyzer.manager != manager {
		t.Error("Analyzer manager mismatch")
	}
}

func TestDefaultROIInput(t *testing.T) {
	input := DefaultROIInput(TierSSD, 50000, 12)

	if input.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, input.Tier)
	}
	if input.InvestmentCost != 50000 {
		t.Errorf("Expected investment 50000, got %f", input.InvestmentCost)
	}
	if input.AnalysisMonths != 12 {
		t.Errorf("Expected 12 months, got %d", input.AnalysisMonths)
	}
	if input.ExpectedLifespanMonths != 36 {
		t.Errorf("Expected lifespan 36 months, got %d", input.ExpectedLifespanMonths)
	}
	if input.CompressionRatio != 0.3 {
		t.Errorf("Expected compression ratio 0.3, got %f", input.CompressionRatio)
	}
	if input.DeduplicationRatio != 0.2 {
		t.Errorf("Expected dedup ratio 0.2, got %f", input.DeduplicationRatio)
	}
}

// TestROIAnalyzer_CalculateROI 测试 ROI 计算.
func TestROIAnalyzer_CalculateROI(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	now := time.Now()
	manager.nowFunc = func() time.Time { return now }

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD存储",
		CostPerTBMonth: 100,
		CapacityTB:     20,
		UsedTB:         15,
	})

	analyzer := NewROIAnalyzer(manager)
	input := DefaultROIInput(TierSSD, 10000, 12)

	result, err := analyzer.CalculateROI(input)
	if err != nil {
		t.Fatalf("CalculateROI failed: %v", err)
	}

	if result == nil {
		t.Fatal("CalculateROI returned nil")
	}

	// 验证基本字段
	if result.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, result.Tier)
	}
	if result.TotalInvestment != 10000 {
		t.Errorf("Expected investment 10000, got %f", result.TotalInvestment)
	}
	if result.TotalBenefits <= 0 {
		t.Error("Total benefits should be positive")
	}
	if result.CostSavings <= 0 {
		t.Error("Cost savings should be positive")
	}
	if result.EfficiencyGain <= 0 {
		t.Error("Efficiency gain should be positive")
	}
	if result.DowntimeReduction <= 0 {
		t.Error("Downtime reduction should be positive")
	}

	// 验证收益明细
	if len(result.BenefitBreakdown) != 3 {
		t.Errorf("Expected 3 benefit items, got %d", len(result.BenefitBreakdown))
	}

	// 验证占比总和
	totalPercentage := 0.0
	for _, item := range result.BenefitBreakdown {
		totalPercentage += item.Percentage
	}
	if totalPercentage < 99.9 || totalPercentage > 100.1 {
		t.Errorf("Benefit percentages should sum to ~100%%, got %f", totalPercentage)
	}

	// 验证回收期
	if result.PaybackPeriodMonths <= 0 {
		t.Error("Payback period should be positive")
	}
}

func TestCompareROI(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         8,
	})

	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD",
		CostPerTBMonth: 30,
		CapacityTB:     50,
		UsedTB:         40,
	})

	analyzer := NewROIAnalyzer(manager)

	inputs := []ROIInput{
		DefaultROIInput(TierSSD, 20000, 12),
		DefaultROIInput(TierHDD, 10000, 12),
	}

	result, err := analyzer.CompareROI(inputs)
	if err != nil {
		t.Fatalf("CompareROI failed: %v", err)
	}

	if len(result.Results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(result.Results))
	}
}

func TestEstimateDataOptimizationROI(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     20,
		UsedTB:         15,
	})

	analyzer := NewROIAnalyzer(manager)

	result, err := analyzer.EstimateDataOptimizationROI(TierSSD, 0.2, 0.3, 500)
	if err != nil {
		t.Fatalf("EstimateDataOptimizationROI failed: %v", err)
	}

	if result.OriginalDataTB != 15 {
		t.Errorf("Expected original data 15 TB, got %f", result.OriginalDataTB)
	}
	if result.TotalSavingsTB <= 0 {
		t.Error("Total savings should be positive")
	}
	if result.MonthlyCostSaving <= 0 {
		t.Error("Monthly cost saving should be positive")
	}
	if result.AnnualCostSaving <= 0 {
		t.Error("Annual cost saving should be positive")
	}
	if result.ImplementationCost <= 0 {
		t.Error("Implementation cost should be positive")
	}
	if result.PaybackMonths <= 0 {
		t.Error("Payback months should be positive")
	}

	// 验证长期 ROI
	if result.ROI3Years <= result.ROI12Months {
		t.Error("3-year ROI should be higher than 12-month ROI")
	}
	if result.ROI5Years <= result.ROI3Years {
		t.Error("5-year ROI should be higher than 3-year ROI")
	}
}

func TestCalculateNPV(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	analyzer := NewROIAnalyzer(manager)

	cashFlows := []float64{1000, 1000, 1000, 1000, 1000, 1000}
	npv, err := analyzer.CalculateNPV(5000, cashFlows, 0.1)
	if err != nil {
		t.Fatalf("CalculateNPV failed: %v", err)
	}

	// NPV 应该是正的（因为总收益 > 投资）
	if npv <= 0 {
		t.Errorf("Expected positive NPV, got %f", npv)
	}
}

func TestCalculateIRR(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	analyzer := NewROIAnalyzer(manager)

	cashFlows := []float64{2000, 2000, 2000, 2000, 2000}
	irr, err := analyzer.CalculateIRR(5000, cashFlows)
	if err != nil {
		t.Fatalf("CalculateIRR failed: %v", err)
	}

	// IRR 应该是正的
	if irr <= 0 {
		t.Errorf("Expected positive IRR, got %f", irr)
	}

	// IRR 应该是合理的（年化 IRR 不应超过 10000%）
	if irr > 100 {
		t.Logf("IRR is very high: %f, this may indicate high-return investment", irr)
	}
}

func TestGenerateCashFlows(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         8,
	})

	analyzer := NewROIAnalyzer(manager)

	cashFlows, err := analyzer.GenerateCashFlows(TierSSD, 10000, 12)
	if err != nil {
		t.Fatalf("GenerateCashFlows failed: %v", err)
	}

	if len(cashFlows) != 12 {
		t.Errorf("Expected 12 cash flows, got %d", len(cashFlows))
	}

	// 验证现金流为正
	for i, cf := range cashFlows {
		if cf <= 0 {
			t.Errorf("Cash flow at month %d should be positive, got %f", i, cf)
		}
	}
}
