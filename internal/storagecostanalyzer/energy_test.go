package storagecostanalyzer

import (
	"testing"
	"time"
)

func TestNewEnergyAnalyzer(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	energyConfig := EnergyConfig{
		ElectricityPrice: 0.8,
		CoolingPUE:       1.3,
	}

	analyzer := NewEnergyAnalyzer(manager, energyConfig)

	if analyzer == nil {
		t.Fatal("NewEnergyAnalyzer returned nil")
	}
	if analyzer.manager != manager {
		t.Error("Analyzer manager mismatch")
	}
	// 验证默认值
	if analyzer.config.ElectricityPrice != 0.8 {
		t.Errorf("Expected electricity price 0.8, got %f", analyzer.config.ElectricityPrice)
	}
	if analyzer.config.CoolingPUE != 1.3 {
		t.Errorf("Expected PUE 1.3, got %f", analyzer.config.CoolingPUE)
	}
	if len(analyzer.config.DiskPower) == 0 {
		t.Error("Default disk power should be initialized")
	}
}

func TestDefaultDiskPower(t *testing.T) {
	power := defaultDiskPower()

	if len(power) != 4 {
		t.Errorf("Expected 4 disk types, got %d", len(power))
	}

	ssd, ok := power[DiskTypeSSD]
	if !ok {
		t.Fatal("SSD power spec not found")
	}
	if ssd.IdlePowerW != 0.5 {
		t.Errorf("Expected SSD idle power 0.5, got %f", ssd.IdlePowerW)
	}
	if ssd.ActivePowerW != 3.0 {
		t.Errorf("Expected SSD active power 3.0, got %f", ssd.ActivePowerW)
	}
}

func TestAnalyzeEnergy(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	now := time.Now()
	manager.nowFunc = func() time.Time { return now }

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD存储",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         8,
	})

	energyConfig := EnergyConfig{
		ElectricityPrice: 0.8,
		CoolingPUE:       1.3,
	}
	analyzer := NewEnergyAnalyzer(manager, energyConfig)

	result, err := analyzer.AnalyzeEnergy(TierSSD, DiskTypeSSD, 4, 2.5)
	if err != nil {
		t.Fatalf("AnalyzeEnergy failed: %v", err)
	}

	if result == nil {
		t.Fatal("AnalyzeEnergy returned nil")
	}

	// 验证基本字段
	if result.Tier != TierSSD {
		t.Errorf("Expected tier %s, got %s", TierSSD, result.Tier)
	}
	if result.DiskType != DiskTypeSSD {
		t.Errorf("Expected disk type %s, got %s", DiskTypeSSD, result.DiskType)
	}
	if result.DiskCount != 4 {
		t.Errorf("Expected 4 disks, got %d", result.DiskCount)
	}
	if result.TotalCapacityTB != 10.0 {
		t.Errorf("Expected total capacity 10 TB, got %f", result.TotalCapacityTB)
	}

	// 验证功耗计算
	if result.IdlePowerW <= 0 {
		t.Error("Idle power should be positive")
	}
	if result.ActivePowerW <= 0 {
		t.Error("Active power should be positive")
	}
	if result.CurrentPowerW <= 0 {
		t.Error("Current power should be positive")
	}
	if result.CurrentPowerW < result.IdlePowerW || result.CurrentPowerW > result.ActivePowerW {
		t.Error("Current power should be between idle and active")
	}

	// 验证耗电量
	if result.DailyKWh <= 0 {
		t.Error("Daily kWh should be positive")
	}
	if result.MonthlyKWh <= 0 {
		t.Error("Monthly kWh should be positive")
	}
	if result.AnnualKWh <= 0 {
		t.Error("Annual kWh should be positive")
	}

	// 验证成本
	if result.DailyCost <= 0 {
		t.Error("Daily cost should be positive")
	}
	if result.MonthlyCost <= 0 {
		t.Error("Monthly cost should be positive")
	}
	if result.AnnualCost <= 0 {
		t.Error("Annual cost should be positive")
	}
	if result.CoolingMonthlyCost <= 0 {
		t.Error("Cooling cost should be positive")
	}
	if result.TotalMonthlyCost <= result.MonthlyCost {
		t.Error("Total monthly cost should be greater than electricity cost alone")
	}

	// 验证碳排放
	if result.CO2KgPerYear <= 0 {
		t.Error("CO2 emission should be positive")
	}
}

func TestCompareEnergyEfficiency(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)
	manager.nowFunc = time.Now

	manager.RegisterTier(TierHDD, TierConfig{
		Tier:           TierHDD,
		Name:           "HDD存储",
		CostPerTBMonth: 30,
		CapacityTB:     100,
		UsedTB:         80,
	})

	energyConfig := EnergyConfig{
		ElectricityPrice: 0.8,
		CoolingPUE:       1.3,
	}
	analyzer := NewEnergyAnalyzer(manager, energyConfig)

	diskTypes := []DiskType{DiskTypeSSD, DiskTypeHDD7200, DiskTypeHDD5400}
	result, err := analyzer.CompareEnergyEfficiency(TierHDD, diskTypes, 10, 10.0)
	if err != nil {
		t.Fatalf("CompareEnergyEfficiency failed: %v", err)
	}

	if len(result.Results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result.Results))
	}
}

func TestForecastEnergy(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	now := time.Now()
	manager.nowFunc = func() time.Time { return now }

	manager.RegisterTier(TierSSD, TierConfig{
		Tier:           TierSSD,
		Name:           "SSD",
		CostPerTBMonth: 100,
		CapacityTB:     10,
		UsedTB:         5,
	})

	energyConfig := EnergyConfig{
		ElectricityPrice: 0.8,
		CoolingPUE:       1.3,
	}
	analyzer := NewEnergyAnalyzer(manager, energyConfig)

	result, err := analyzer.ForecastEnergy(TierSSD, DiskTypeSSD, 4, 2.5, 12, 5.0)
	if err != nil {
		t.Fatalf("ForecastEnergy failed: %v", err)
	}

	if result.ForecastMonths != 12 {
		t.Errorf("Expected 12 months, got %d", result.ForecastMonths)
	}
	if len(result.MonthlyForecasts) != 12 {
		t.Errorf("Expected 12 forecast points, got %d", len(result.MonthlyForecasts))
	}
	if result.TotalForecastKWh <= 0 {
		t.Error("Total forecast kWh should be positive")
	}
	if result.TotalForecastCost <= 0 {
		t.Error("Total forecast cost should be positive")
	}

	// 验证增长趋势
	for i := 1; i < len(result.MonthlyForecasts); i++ {
		if result.MonthlyForecasts[i].ProjectedKWh < result.MonthlyForecasts[i-1].ProjectedKWh {
			t.Error("Energy consumption should increase with growth rate")
		}
	}
}

func TestCalculateEnergySavings(t *testing.T) {
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

	energyConfig := EnergyConfig{
		ElectricityPrice: 0.8,
		CoolingPUE:       1.3,
	}
	analyzer := NewEnergyAnalyzer(manager, energyConfig)

	result, err := analyzer.CalculateEnergySavings(TierSSD, DiskTypeHDD7200, DiskTypeSSD, 4, 2.5, 12)
	if err != nil {
		t.Fatalf("CalculateEnergySavings failed: %v", err)
	}

	if result.CurrentDiskType != DiskTypeHDD7200 {
		t.Errorf("Expected current disk type %s, got %s", DiskTypeHDD7200, result.CurrentDiskType)
	}
	if result.TargetDiskType != DiskTypeSSD {
		t.Errorf("Expected target disk type %s, got %s", DiskTypeSSD, result.TargetDiskType)
	}

	// SSD 应该比 HDD 节能
	if result.MonthlySavingsKWh <= 0 {
		t.Error("SSD should save energy compared to HDD")
	}
	if result.MonthlySavingsCost <= 0 {
		t.Error("SSD should save cost compared to HDD")
	}
	if result.AnnualSavingsKWh <= 0 {
		t.Error("Annual savings should be positive")
	}
	if result.CO2ReductionKgPerYear <= 0 {
		t.Error("CO2 reduction should be positive")
	}
}

func TestEstimatePowerByUtilization(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	energyConfig := EnergyConfig{
		ElectricityPrice: 0.8,
		CoolingPUE:       1.3,
	}
	analyzer := NewEnergyAnalyzer(manager, energyConfig)

	// 测试空闲状态
	power, err := analyzer.EstimatePowerByUtilization(DiskTypeSSD, 4, 0)
	if err != nil {
		t.Fatalf("EstimatePowerByUtilization failed: %v", err)
	}
	expectedIdle := 4 * 0.5 // 4 disks * 0.5W
	if power != expectedIdle {
		t.Errorf("Expected idle power %f, got %f", expectedIdle, power)
	}

	// 测试满载状态
	power, err = analyzer.EstimatePowerByUtilization(DiskTypeSSD, 4, 100)
	if err != nil {
		t.Fatalf("EstimatePowerByUtilization failed: %v", err)
	}
	expectedActive := 4 * 3.0 // 4 disks * 3.0W
	if power != expectedActive {
		t.Errorf("Expected active power %f, got %f", expectedActive, power)
	}

	// 测试50%利用率
	power, err = analyzer.EstimatePowerByUtilization(DiskTypeSSD, 4, 50)
	if err != nil {
		t.Fatalf("EstimatePowerByUtilization failed: %v", err)
	}
	expectedHalf := (expectedIdle + expectedActive) / 2
	if power != expectedHalf {
		t.Errorf("Expected 50%% power %f, got %f", expectedHalf, power)
	}
}
