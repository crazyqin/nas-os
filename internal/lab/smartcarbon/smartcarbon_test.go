package smartcarbon

import (
	"fmt"
	"testing"
	"time"
)

func TestCarbonManager_RecordEmission(t *testing.T) {
	cm := NewCarbonManager(nil)

	record := &CarbonRecord{
		ID:          "rec-1",
		Source:      SourceStorage,
		EnergyKWh:   10.0,
		EnergyType:  EnergyGrid,
		Description: "Storage operation",
	}

	err := cm.RecordEmission(record)
	if err != nil {
		t.Fatalf("Failed to record emission: %v", err)
	}

	if record.ValueKg != 5.0 { // 10 kWh * 0.5 factor
		t.Errorf("Expected 5.0 kg, got %f", record.ValueKg)
	}
}

func TestCarbonManager_Budget(t *testing.T) {
	cm := NewCarbonManager(&CarbonConfig{
		AlertEnabled: true,
	})

	budget := &CarbonBudget{
		ID:             "budget-1",
		Name:           "Monthly Budget",
		DailyLimitKg:   10.0,
		MonthlyLimitKg: 300.0,
		YearlyLimitKg:  3600.0,
		AlertThreshold: 0.8,
		Enabled:        true,
	}

	err := cm.SetBudget(budget)
	if err != nil {
		t.Fatalf("Failed to set budget: %v", err)
	}

	// 记录排放
	for i := 0; i < 5; i++ {
		cm.RecordEmission(&CarbonRecord{
			ID:         fmt.Sprintf("rec-%d", i),
			Source:     SourceCompute,
			EnergyKWh:  2.0,
			EnergyType: EnergyGrid,
		})
	}
}

func TestCarbonManager_Offset(t *testing.T) {
	cm := NewCarbonManager(nil)

	offset := &CarbonOffset{
		ID:          "offset-1",
		ProjectName: "Solar Farm Project",
		Type:        "renewable_energy",
		CreditsKg:   100.0,
		CostUSD:     50.0,
	}

	err := cm.AddOffset(offset)
	if err != nil {
		t.Fatalf("Failed to add offset: %v", err)
	}

	// 验证补偿
	err = cm.VerifyOffset("offset-1")
	if err != nil {
		t.Fatalf("Failed to verify offset: %v", err)
	}
}

func TestCarbonManager_Footprint(t *testing.T) {
	cm := NewCarbonManager(nil)

	now := time.Now()
	start := now.Add(-2 * time.Hour)

	// 记录一些排放
	cm.RecordEmission(&CarbonRecord{
		ID:         "rec-1",
		Source:     SourceStorage,
		EnergyKWh:  10.0,
		EnergyType: EnergyGrid,
	})

	cm.RecordEmission(&CarbonRecord{
		ID:         "rec-2",
		Source:     SourceCompute,
		EnergyKWh:  5.0,
		EnergyType: EnergySolar,
	})

	footprint := cm.GetFootprint(start, now.Add(time.Hour))

	if footprint.TotalKg == 0 {
		t.Error("Expected non-zero total emissions")
	}

	if footprint.BySource[SourceStorage] == 0 {
		t.Error("Expected storage emissions")
	}
}

func TestCarbonManager_Stats(t *testing.T) {
	cm := NewCarbonManager(nil)

	cm.RecordEmission(&CarbonRecord{
		ID:         "rec-1",
		Source:     SourceStorage,
		EnergyKWh:  10.0,
		EnergyType: EnergyGrid,
	})

	stats := cm.GetStats()

	if stats.TodayKg == 0 {
		t.Error("Expected non-zero today emissions")
	}
}

func TestCarbonManager_Optimizations(t *testing.T) {
	cm := NewCarbonManager(nil)

	opts := cm.GetOptimizations()
	if len(opts) == 0 {
		t.Error("Expected optimization suggestions")
	}
}

func TestEstimateCarbonFootprint(t *testing.T) {
	// 测试碳足迹估算
	total := EstimateCarbonFootprint(1.0, 100.0, 50.0)

	// 存储: 1TB * 50 = 50kg
	// 计算: 100h * 0.1 = 10kg
	// 网络: 50GB * 0.01 = 0.5kg
	expected := 50.0 + 10.0 + 0.5

	if total != expected {
		t.Errorf("Expected %f, got %f", expected, total)
	}
}

func TestConvertToTrees(t *testing.T) {
	trees := ConvertToTrees(220.0)
	if trees != 10 {
		t.Errorf("Expected 10 trees, got %f", trees)
	}
}

func TestCarbonIntensity_Constants(t *testing.T) {
	intensities := []CarbonIntensity{
		IntensityLow, IntensityMedium, IntensityHigh, IntensityCritical,
	}

	for _, i := range intensities {
		if i == "" {
			t.Error("Intensity constant should not be empty")
		}
	}
}
