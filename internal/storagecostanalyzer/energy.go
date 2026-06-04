// Package storagecostanalyzer 存储成本分析器 - 能耗成本预测
package storagecostanalyzer

import (
	"fmt"
	"math"
	"time"
)

// EnergyAnalyzer 能耗分析器.
type EnergyAnalyzer struct {
	manager *Manager
	config  EnergyConfig
}

// NewEnergyAnalyzer 创建能耗分析器.
func NewEnergyAnalyzer(manager *Manager, config EnergyConfig) *EnergyAnalyzer {
	// 设置默认值
	if config.ElectricityPrice <= 0 {
		config.ElectricityPrice = 0.8 // 0.8元/kWh
	}
	if config.CoolingPUE <= 0 {
		config.CoolingPUE = 1.3 // PUE 1.3
	}
	if config.DiskPower == nil {
		config.DiskPower = defaultDiskPower()
	}

	return &EnergyAnalyzer{
		manager: manager,
		config:  config,
	}
}

// defaultDiskPower 默认硬盘功耗配置.
func defaultDiskPower() map[DiskType]DiskPowerSpec {
	return map[DiskType]DiskPowerSpec{
		DiskTypeSSD: {
			IdlePowerW:    0.5,
			ActivePowerW:  3.0,
			MaxPowerW:     5.0,
			TypicalPowerW: 2.0,
		},
		DiskTypeNVMe: {
			IdlePowerW:    0.8,
			ActivePowerW:  5.0,
			MaxPowerW:     8.5,
			TypicalPowerW: 3.5,
		},
		DiskTypeHDD7200: {
			IdlePowerW:    5.0,
			ActivePowerW:  8.0,
			MaxPowerW:     12.0,
			TypicalPowerW: 6.5,
		},
		DiskTypeHDD5400: {
			IdlePowerW:    3.5,
			ActivePowerW:  6.0,
			MaxPowerW:     9.0,
			TypicalPowerW: 4.5,
		},
	}
}

// AnalyzeEnergy 分析单个存储层级的能耗.
func (a *EnergyAnalyzer) AnalyzeEnergy(tier StorageTier, diskType DiskType, diskCount int, capacityPerDiskTB float64) (*EnergyConsumption, error) {
	a.manager.mu.RLock()
	defer a.manager.mu.RUnlock()

	ts, ok := a.manager.tiers[tier]
	if !ok {
		return nil, ErrTierNotFound
	}

	powerSpec, ok := a.config.DiskPower[diskType]
	if !ok {
		return nil, fmt.Errorf("%w: unsupported disk type %s", ErrInvalidConfig, diskType)
	}

	if diskCount <= 0 {
		return nil, fmt.Errorf("%w: disk count must be positive", ErrInvalidConfig)
	}
	if capacityPerDiskTB <= 0 {
		return nil, fmt.Errorf("%w: capacity per disk must be positive", ErrInvalidConfig)
	}

	cfg := ts.config
	totalCapacityTB := float64(diskCount) * capacityPerDiskTB
	utilization := 0.0
	if totalCapacityTB > 0 {
		utilization = (cfg.UsedTB / totalCapacityTB) * 100
	}

	// 计算功耗
	idlePower := float64(diskCount) * powerSpec.IdlePowerW
	activePower := float64(diskCount) * powerSpec.ActivePowerW

	// 当前功耗基于利用率（线性插值）
	currentPower := idlePower + (activePower-idlePower)*(utilization/100)

	// 耗电量计算
	hoursPerDay := 24.0
	dailyKWh := currentPower * hoursPerDay / 1000
	monthlyKWh := dailyKWh * 30
	annualKWh := dailyKWh * 365

	// 电力成本
	dailyCost := dailyKWh * a.config.ElectricityPrice
	monthlyCost := monthlyKWh * a.config.ElectricityPrice
	annualCost := annualKWh * a.config.ElectricityPrice

	// 散热成本（PUE 系数）
	coolingMonthlyCost := monthlyCost * (a.config.CoolingPUE - 1)
	totalMonthlyCost := monthlyCost + coolingMonthlyCost

	// 碳排放（中国平均 0.581 kg CO2/kWh）
	co2KgPerYear := annualKWh * 0.581

	return &EnergyConsumption{
		GeneratedAt:        a.manager.nowFunc(),
		Tier:               tier,
		TierName:           cfg.Name,
		DiskType:           diskType,
		DiskCount:          diskCount,
		CapacityPerDiskTB:  capacityPerDiskTB,
		TotalCapacityTB:    totalCapacityTB,
		Utilization:        utilization,
		IdlePowerW:         idlePower,
		ActivePowerW:       activePower,
		CurrentPowerW:      currentPower,
		DailyKWh:           dailyKWh,
		MonthlyKWh:         monthlyKWh,
		AnnualKWh:          annualKWh,
		DailyCost:          dailyCost,
		MonthlyCost:        monthlyCost,
		AnnualCost:         annualCost,
		CoolingMonthlyCost: coolingMonthlyCost,
		TotalMonthlyCost:   totalMonthlyCost,
		CO2KgPerYear:       co2KgPerYear,
	}, nil
}

// CompareEnergyEfficiency 对比不同硬盘类型的能效.
func (a *EnergyAnalyzer) CompareEnergyEfficiency(tier StorageTier, diskTypes []DiskType, diskCount int, capacityPerDiskTB float64) (*EnergyComparison, error) {
	if len(diskTypes) < 2 {
		return nil, fmt.Errorf("%w: at least 2 disk types required", ErrInvalidConfig)
	}

	results := make([]*EnergyConsumption, 0, len(diskTypes))
	for _, dt := range diskTypes {
		result, err := a.AnalyzeEnergy(tier, dt, diskCount, capacityPerDiskTB)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	// 找出最节能和最高能效比
	bestEfficiency := 0
	bestCostPerTB := 0
	minAnnualCost := math.MaxFloat64
	minCostPerTB := math.MaxFloat64

	for i, r := range results {
		totalCapacityTB := float64(diskCount) * capacityPerDiskTB
		costPerTB := 0.0
		if totalCapacityTB > 0 {
			costPerTB = r.AnnualCost / totalCapacityTB
		}

		if r.AnnualCost < minAnnualCost {
			minAnnualCost = r.AnnualCost
			bestEfficiency = i
		}
		if costPerTB < minCostPerTB {
			minCostPerTB = costPerTB
			bestCostPerTB = i
		}
	}

	return &EnergyComparison{
		GeneratedAt:    a.manager.nowFunc(),
		Tier:           tier,
		DiskCount:      diskCount,
		CapacityPerTB:  capacityPerDiskTB,
		Results:        results,
		BestEfficiency: bestEfficiency,
		BestCostPerTB:  bestCostPerTB,
	}, nil
}

// EnergyComparison 能耗对比结果.
type EnergyComparison struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// DiskCount 硬盘数量.
	DiskCount int `json:"diskCount"`
	// CapacityPerTB 单盘容量（TB）.
	CapacityPerTB float64 `json:"capacityPerTB"`
	// Results 各硬盘类型能耗结果.
	Results []*EnergyConsumption `json:"results"`
	// BestEfficiency 最节能方案索引.
	BestEfficiency int `json:"bestEfficiency"`
	// BestCostPerTB 最佳每TB成本索引.
	BestCostPerTB int `json:"bestCostPerTB"`
}

// ForecastEnergy 预测能耗.
func (a *EnergyAnalyzer) ForecastEnergy(tier StorageTier, diskType DiskType, diskCount int, capacityPerDiskTB float64, months int, growthRatePercent float64) (*EnergyForecast, error) {
	if months <= 0 {
		months = 12
	}
	if growthRatePercent < 0 {
		return nil, fmt.Errorf("%w: growth rate must be non-negative", ErrInvalidConfig)
	}

	current, err := a.AnalyzeEnergy(tier, diskType, diskCount, capacityPerDiskTB)
	if err != nil {
		return nil, err
	}

	growthRate := growthRatePercent / 100
	currentMonthlyKWh := current.MonthlyKWh

	forecastPoints := make([]EnergyForecastPoint, 0, months)
	totalForecastKWh := 0.0
	totalForecastCost := 0.0
	cumulativeKWh := 0.0
	cumulativeCost := 0.0

	for i := 1; i <= months; i++ {
		projectedKWh := currentMonthlyKWh * math.Pow(1+growthRate, float64(i))
		projectedCost := projectedKWh * a.config.ElectricityPrice

		totalForecastKWh += projectedKWh
		totalForecastCost += projectedCost
		cumulativeKWh += projectedKWh
		cumulativeCost += projectedCost

		forecastPoints = append(forecastPoints, EnergyForecastPoint{
			Month:          i,
			Date:           a.manager.nowFunc().AddDate(0, i, 0),
			ProjectedKWh:   projectedKWh,
			ProjectedCost:  projectedCost,
			CumulativeKWh:  cumulativeKWh,
			CumulativeCost: cumulativeCost,
		})
	}

	return &EnergyForecast{
		GeneratedAt:       a.manager.nowFunc(),
		ForecastMonths:    months,
		CurrentMonthlyKWh: currentMonthlyKWh,
		GrowthRate:        growthRatePercent,
		MonthlyForecasts:  forecastPoints,
		TotalForecastKWh:  totalForecastKWh,
		TotalForecastCost: totalForecastCost,
	}, nil
}

// CalculateEnergySavings 计算节能优化收益.
func (a *EnergyAnalyzer) CalculateEnergySavings(
	tier StorageTier,
	currentDiskType DiskType,
	targetDiskType DiskType,
	diskCount int,
	capacityPerDiskTB float64,
	months int,
) (*EnergySavings, error) {
	if months <= 0 {
		months = 12
	}

	current, err := a.AnalyzeEnergy(tier, currentDiskType, diskCount, capacityPerDiskTB)
	if err != nil {
		return nil, err
	}

	target, err := a.AnalyzeEnergy(tier, targetDiskType, diskCount, capacityPerDiskTB)
	if err != nil {
		return nil, err
	}

	monthlySavingsKWh := current.MonthlyKWh - target.MonthlyKWh
	monthlySavingsCost := current.MonthlyCost - target.MonthlyCost
	annualSavingsKWh := monthlySavingsKWh * 12
	annualSavingsCost := monthlySavingsCost * 12
	totalSavingsKWh := monthlySavingsKWh * float64(months)
	totalSavingsCost := monthlySavingsCost * float64(months)
	co2ReductionKgPerYear := (current.CO2KgPerYear - target.CO2KgPerYear)

	savingsPercent := 0.0
	if current.AnnualCost > 0 {
		savingsPercent = (annualSavingsCost / current.AnnualCost) * 100
	}

	return &EnergySavings{
		GeneratedAt:           a.manager.nowFunc(),
		Tier:                  tier,
		CurrentDiskType:       currentDiskType,
		TargetDiskType:        targetDiskType,
		DiskCount:             diskCount,
		CapacityPerDiskTB:     capacityPerDiskTB,
		AnalysisMonths:        months,
		CurrentMonthlyKWh:     current.MonthlyKWh,
		TargetMonthlyKWh:      target.MonthlyKWh,
		MonthlySavingsKWh:     monthlySavingsKWh,
		MonthlySavingsCost:    monthlySavingsCost,
		AnnualSavingsKWh:      annualSavingsKWh,
		AnnualSavingsCost:     annualSavingsCost,
		TotalSavingsKWh:       totalSavingsKWh,
		TotalSavingsCost:      totalSavingsCost,
		CO2ReductionKgPerYear: co2ReductionKgPerYear,
		SavingsPercent:        savingsPercent,
	}, nil
}

// EnergySavings 节能收益.
type EnergySavings struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// CurrentDiskType 当前硬盘类型.
	CurrentDiskType DiskType `json:"currentDiskType"`
	// TargetDiskType 目标硬盘类型.
	TargetDiskType DiskType `json:"targetDiskType"`
	// DiskCount 硬盘数量.
	DiskCount int `json:"diskCount"`
	// CapacityPerDiskTB 单盘容量（TB）.
	CapacityPerDiskTB float64 `json:"capacityPerDiskTB"`
	// AnalysisMonths 分析周期（月）.
	AnalysisMonths int `json:"analysisMonths"`
	// CurrentMonthlyKWh 当前月耗电（kWh）.
	CurrentMonthlyKWh float64 `json:"currentMonthlyKWh"`
	// TargetMonthlyKWh 目标月耗电（kWh）.
	TargetMonthlyKWh float64 `json:"targetMonthlyKWh"`
	// MonthlySavingsKWh 月节省电量（kWh）.
	MonthlySavingsKWh float64 `json:"monthlySavingsKWh"`
	// MonthlySavingsCost 月节省成本.
	MonthlySavingsCost float64 `json:"monthlySavingsCost"`
	// AnnualSavingsKWh 年节省电量（kWh）.
	AnnualSavingsKWh float64 `json:"annualSavingsKWh"`
	// AnnualSavingsCost 年节省成本.
	AnnualSavingsCost float64 `json:"annualSavingsCost"`
	// TotalSavingsKWh 总节省电量（kWh）.
	TotalSavingsKWh float64 `json:"totalSavingsKWh"`
	// TotalSavingsCost 总节省成本.
	TotalSavingsCost float64 `json:"totalSavingsCost"`
	// CO2ReductionKgPerYear 年碳减排（kg）.
	CO2ReductionKgPerYear float64 `json:"co2ReductionKgPerYear"`
	// SavingsPercent 节省比例（%）.
	SavingsPercent float64 `json:"savingsPercent"`
}

// EstimatePowerByUtilization 基于利用率估算功耗.
func (a *EnergyAnalyzer) EstimatePowerByUtilization(diskType DiskType, diskCount int, utilizationPercent float64) (float64, error) {
	powerSpec, ok := a.config.DiskPower[diskType]
	if !ok {
		return 0, fmt.Errorf("%w: unsupported disk type %s", ErrInvalidConfig, diskType)
	}

	if utilizationPercent < 0 || utilizationPercent > 100 {
		return 0, fmt.Errorf("%w: utilization must be between 0 and 100", ErrInvalidConfig)
	}

	idlePower := float64(diskCount) * powerSpec.IdlePowerW
	activePower := float64(diskCount) * powerSpec.ActivePowerW

	// 线性插值
	currentPower := idlePower + (activePower-idlePower)*(utilizationPercent/100)

	return currentPower, nil
}
