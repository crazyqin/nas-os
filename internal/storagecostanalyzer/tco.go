// Package storagecostanalyzer 存储成本分析器 - TCO 计算引擎
package storagecostanalyzer

import (
	"fmt"
	"math"
	"time"
)

// TCOEngine TCO 计算引擎.
type TCOEngine struct {
	manager *Manager
}

// NewTCOEngine 创建 TCO 计算引擎.
func NewTCOEngine(manager *Manager) *TCOEngine {
	return &TCOEngine{manager: manager}
}

// TCOInput TCO 计算输入参数.
type TCOInput struct {
	// Tier 存储层级.
	Tier StorageTier
	// AnalysisMonths 分析周期（月）.
	AnalysisMonths int
	// HardwareCostOverride 硬件成本覆盖（可选）.
	HardwareCostOverride *float64
	// ElectricityPrice 电价（元/kWh）.
	ElectricityPrice float64
	// PowerPerTBW 每TB功耗（W）.
	PowerPerTBW float64
	// CoolingPUE 散热PUE系数.
	CoolingPUE float64
	// LaborCostMonthly 月人力成本.
	LaborCostMonthly float64
	// IncludeBandwidth 是否包含带宽成本.
	IncludeBandwidth bool
	// BandwidthCostPerTB 带宽每TB成本.
	BandwidthCostPerTB float64
}

// DefaultTCOInput 默认 TCO 输入.
func DefaultTCOInput(tier StorageTier, months int) TCOInput {
	return TCOInput{
		Tier:               tier,
		AnalysisMonths:     months,
		ElectricityPrice:   0.8,   // 0.8元/kWh
		PowerPerTBW:        8.0,   // 每TB 8W
		CoolingPUE:         1.3,   // PUE 1.3
		LaborCostMonthly:   500.0, // 月人力成本500元
		IncludeBandwidth:   true,
		BandwidthCostPerTB: 10.0, // 每TB带宽10元
	}
}

// CalculateTCO 计算总拥有成本.
func (e *TCOEngine) CalculateTCO(input TCOInput) (*TCOAnalysis, error) {
	e.manager.mu.RLock()
	defer e.manager.mu.RUnlock()

	ts, ok := e.manager.tiers[input.Tier]
	if !ok {
		return nil, ErrTierNotFound
	}

	if input.AnalysisMonths <= 0 {
		input.AnalysisMonths = 12
	}

	cfg := ts.config
	usedTB := cfg.UsedTB

	// 计算硬件成本
	hardwareCost := usedTB * 500 // 默认每TB 500元
	if input.HardwareCostOverride != nil {
		hardwareCost = *input.HardwareCostOverride
	}

	// 计算电力成本
	powerW := usedTB * input.PowerPerTBW
	dailyKWh := powerW * 24 / 1000
	monthlyKWh := dailyKWh * 30
	powerCost := monthlyKWh * input.ElectricityPrice * float64(input.AnalysisMonths)

	// 计算散热成本（PUE 系数）
	coolingCost := powerCost * (input.CoolingPUE - 1)

	// 计算维护成本（硬件成本的15%年化）
	maintenanceCost := hardwareCost * 0.15 * float64(input.AnalysisMonths) / 12

	// 计算订阅/服务成本
	subscriptionCost := usedTB * cfg.CostPerTBMonth * float64(input.AnalysisMonths)

	// 计算带宽成本
	bandwidthCost := 0.0
	if input.IncludeBandwidth {
		bandwidthCost = usedTB * input.BandwidthCostPerTB * float64(input.AnalysisMonths)
	}

	// 计算人力成本
	laborCost := input.LaborCostMonthly * float64(input.AnalysisMonths)

	// 计算折旧成本（3年折旧期）
	depreciationCost := hardwareCost / 36 * float64(input.AnalysisMonths)

	// 汇总 TCO
	totalTCO := hardwareCost + powerCost + coolingCost + maintenanceCost +
		subscriptionCost + bandwidthCost + laborCost + depreciationCost

	// 计算单位成本
	costPerTBPerMonth := 0.0
	costPerTBPerYear := 0.0
	if usedTB > 0 && input.AnalysisMonths > 0 {
		costPerTBPerMonth = totalTCO / usedTB / float64(input.AnalysisMonths)
		costPerTBPerYear = costPerTBPerMonth * 12
	}

	// 成本明细
	breakdown := []TCOCostItem{
		{Category: CategoryHardware, Amount: hardwareCost, Description: "硬件采购成本"},
		{Category: CategoryPower, Amount: powerCost, Description: fmt.Sprintf("电力成本（%.0fW x %.0f月）", powerW, float64(input.AnalysisMonths))},
		{Category: CategoryCooling, Amount: coolingCost, Description: fmt.Sprintf("散热成本（PUE=%.1f）", input.CoolingPUE)},
		{Category: CategoryMaintenance, Amount: maintenanceCost, Description: "维护成本"},
		{Category: CategorySubscription, Amount: subscriptionCost, Description: "订阅/服务费用"},
		{Category: CategoryBandwidth, Amount: bandwidthCost, Description: "带宽成本"},
		{Category: CategoryLabor, Amount: laborCost, Description: "人力成本"},
		{Category: CategoryDepreciation, Amount: depreciationCost, Description: "折旧成本"},
	}

	// 计算占比
	for i := range breakdown {
		if totalTCO > 0 {
			breakdown[i].Percentage = (breakdown[i].Amount / totalTCO) * 100
		}
	}

	// 年度成本预测
	yearlyProjection := e.calculateYearlyProjection(
		input.AnalysisMonths,
		hardwareCost,
		powerCost,
		coolingCost,
		maintenanceCost,
		subscriptionCost,
		bandwidthCost,
		laborCost,
		depreciationCost,
	)

	return &TCOAnalysis{
		GeneratedAt:          e.manager.nowFunc(),
		Tier:                 input.Tier,
		TierName:             cfg.Name,
		AnalysisPeriodMonths: input.AnalysisMonths,
		InitialCost:          hardwareCost,
		RecurringCost:        totalTCO - hardwareCost,
		HardwareCost:         hardwareCost,
		PowerCost:            powerCost,
		CoolingCost:          coolingCost,
		MaintenanceCost:      maintenanceCost,
		SubscriptionCost:     subscriptionCost,
		BandwidthCost:        bandwidthCost,
		LaborCost:            laborCost,
		DepreciationCost:     depreciationCost,
		TotalTCO:             totalTCO,
		CostPerTBPerMonth:    costPerTBPerMonth,
		CostPerTBPerYear:     costPerTBPerYear,
		CostBreakdown:        breakdown,
		YearlyProjection:     yearlyProjection,
	}, nil
}

// calculateYearlyProjection 计算年度成本预测.
func (e *TCOEngine) calculateYearlyProjection(
	months int,
	hardware, power, cooling, maintenance, subscription, bandwidth, labor, depreciation float64,
) []YearlyCost {
	years := months / 12
	if months%12 > 0 {
		years++
	}

	yearlyProjection := make([]YearlyCost, 0, years)
	cumulativeCost := 0.0

	for y := 1; y <= years; y++ {
		// 硬件成本主要在第一年
		yearHardware := 0.0
		if y == 1 {
			yearHardware = hardware
		} else {
			yearHardware = hardware * 0.05 // 后续年份硬件维护5%
		}

		// 运营成本按年分摊
		yearOperating := (power + cooling + maintenance + bandwidth + labor) / float64(years)
		yearSubscription := subscription / float64(years)
		yearDepreciation := depreciation / float64(years)

		yearTotal := yearHardware + yearOperating + yearSubscription + yearDepreciation
		cumulativeCost += yearTotal

		yearlyProjection = append(yearlyProjection, YearlyCost{
			Year:           y,
			HardwareCost:   yearHardware,
			OperatingCost:  yearOperating + yearSubscription + yearDepreciation,
			TotalCost:      yearTotal,
			CumulativeCost: cumulativeCost,
		})
	}

	return yearlyProjection
}

// CompareTCO 对比两个存储层级的 TCO.
func (e *TCOEngine) CompareTCO(tier1, tier2 StorageTier, months int) (*TCOComparison, error) {
	input1 := DefaultTCOInput(tier1, months)
	input2 := DefaultTCOInput(tier2, months)

	tco1, err := e.CalculateTCO(input1)
	if err != nil {
		return nil, fmt.Errorf("计算 %s TCO 失败: %w", tier1, err)
	}

	tco2, err := e.CalculateTCO(input2)
	if err != nil {
		return nil, fmt.Errorf("计算 %s TCO 失败: %w", tier2, err)
	}

	savings := tco1.TotalTCO - tco2.TotalTCO
	savingsPercent := 0.0
	if tco1.TotalTCO > 0 {
		savingsPercent = (savings / tco1.TotalTCO) * 100
	}

	recommendation := tier2
	if tco1.TotalTCO < tco2.TotalTCO {
		recommendation = tier1
	}

	return &TCOComparison{
		GeneratedAt:    e.manager.nowFunc(),
		TCO1:           tco1,
		TCO2:           tco2,
		Savings:        savings,
		SavingsPercent: savingsPercent,
		Recommendation: recommendation,
		AnalysisPeriod: months,
	}, nil
}

// TCOComparison TCO 对比结果.
type TCOComparison struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// TCO1 第一个层级 TCO.
	TCO1 *TCOAnalysis `json:"tco1"`
	// TCO2 第二个层级 TCO.
	TCO2 *TCOAnalysis `json:"tco2"`
	// Savings 节省金额（TCO1 - TCO2）.
	Savings float64 `json:"savings"`
	// SavingsPercent 节省百分比.
	SavingsPercent float64 `json:"savingsPercent"`
	// Recommendation 推荐层级.
	Recommendation StorageTier `json:"recommendation"`
	// AnalysisPeriod 分析周期（月）.
	AnalysisPeriod int `json:"analysisPeriod"`
}

// CalculateBreakEven 计算盈亏平衡点.
func (e *TCOEngine) CalculateBreakEven(tier StorageTier, investmentCost float64, monthlySavings float64) (*BreakEvenAnalysis, error) {
	if investmentCost <= 0 {
		return nil, fmt.Errorf("%w: investment cost must be positive", ErrInvalidConfig)
	}
	if monthlySavings <= 0 {
		return nil, fmt.Errorf("%w: monthly savings must be positive", ErrInvalidConfig)
	}

	breakEvenMonths := int(math.Ceil(investmentCost / monthlySavings))
	breakEvenDate := e.manager.nowFunc().AddDate(0, breakEvenMonths, 0)

	// 计算5年ROI
	fiveYearSavings := monthlySavings * 60
	fiveYearROI := 0.0
	if investmentCost > 0 {
		fiveYearROI = ((fiveYearSavings - investmentCost) / investmentCost) * 100
	}

	return &BreakEvenAnalysis{
		GeneratedAt:     e.manager.nowFunc(),
		Tier:            tier,
		InvestmentCost:  investmentCost,
		MonthlySavings:  monthlySavings,
		BreakEvenMonths: breakEvenMonths,
		BreakEvenDate:   breakEvenDate,
		FiveYearSavings: fiveYearSavings,
		FiveYearROI:     fiveYearROI,
	}, nil
}

// BreakEvenAnalysis 盈亏平衡分析.
type BreakEvenAnalysis struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// InvestmentCost 投资成本.
	InvestmentCost float64 `json:"investmentCost"`
	// MonthlySavings 月节省金额.
	MonthlySavings float64 `json:"monthlySavings"`
	// BreakEvenMonths 盈亏平衡月数.
	BreakEvenMonths int `json:"breakEvenMonths"`
	// BreakEvenDate 盈亏平衡日期.
	BreakEvenDate time.Time `json:"breakEvenDate"`
	// FiveYearSavings 5年节省.
	FiveYearSavings float64 `json:"fiveYearSavings"`
	// FiveYearROI 5年ROI.
	FiveYearROI float64 `json:"fiveYearROI"`
}

// ProjectTCO 基于增长趋势预测 TCO.
func (e *TCOEngine) ProjectTCO(tier StorageTier, months int, growthRatePercent float64) (*TCOProjection, error) {
	e.manager.mu.RLock()
	defer e.manager.mu.RUnlock()

	ts, ok := e.manager.tiers[tier]
	if !ok {
		return nil, ErrTierNotFound
	}

	if months <= 0 {
		months = 12
	}

	cfg := ts.config
	currentUsedTB := cfg.UsedTB
	growthRate := growthRatePercent / 100

	projectedPoints := make([]TCOProjectionPoint, 0, months)
	totalProjectedCost := 0.0

	for i := 1; i <= months; i++ {
		projectedUsed := currentUsedTB * math.Pow(1+growthRate, float64(i))
		if projectedUsed > cfg.CapacityTB {
			projectedUsed = cfg.CapacityTB
		}

		// 估算月成本（简化模型）
		monthlyCost := projectedUsed * cfg.CostPerTBMonth
		totalProjectedCost += monthlyCost

		projectedPoints = append(projectedPoints, TCOProjectionPoint{
			Month:           i,
			Date:            e.manager.nowFunc().AddDate(0, i, 0),
			ProjectedUsedTB: projectedUsed,
			ProjectedCost:   monthlyCost,
			CumulativeCost:  totalProjectedCost,
		})
	}

	return &TCOProjection{
		GeneratedAt:        e.manager.nowFunc(),
		Tier:               tier,
		TierName:           cfg.Name,
		CurrentUsedTB:      currentUsedTB,
		GrowthRatePercent:  growthRatePercent,
		ProjectMonths:      months,
		ProjectedPoints:    projectedPoints,
		TotalProjectedCost: totalProjectedCost,
	}, nil
}

// TCOProjection TCO 预测.
type TCOProjection struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// TierName 层级名称.
	TierName string `json:"tierName"`
	// CurrentUsedTB 当前已用（TB）.
	CurrentUsedTB float64 `json:"currentUsedTB"`
	// GrowthRatePercent 增长率（%）.
	GrowthRatePercent float64 `json:"growthRatePercent"`
	// ProjectMonths 预测月数.
	ProjectMonths int `json:"projectMonths"`
	// ProjectedPoints 预测数据点.
	ProjectedPoints []TCOProjectionPoint `json:"projectedPoints"`
	// TotalProjectedCost 总预测成本.
	TotalProjectedCost float64 `json:"totalProjectedCost"`
}

// TCOProjectionPoint TCO 预测数据点.
type TCOProjectionPoint struct {
	// Month 月份（从1开始）.
	Month int `json:"month"`
	// Date 预测日期.
	Date time.Time `json:"date"`
	// ProjectedUsedTB 预测已用（TB）.
	ProjectedUsedTB float64 `json:"projectedUsedTB"`
	// ProjectedCost 预测成本.
	ProjectedCost float64 `json:"projectedCost"`
	// CumulativeCost 累计成本.
	CumulativeCost float64 `json:"cumulativeCost"`
}
