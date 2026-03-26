// Package reports 提供报表生成和管理功能
package reports

import (
	"math"
	"time"
)

// ========== 存储成本分析 v2.60.0 ==========

// CostConfig 成本配置.
type CostConfig struct {
	// 电费单价（元/kWh）
	ElectricityRate float64 `json:"electricity_rate"`

	// 设备功率（瓦特）
	DevicePowerWatts float64 `json:"device_power_watts"`

	// 硬件购置成本（元）
	HardwareCost float64 `json:"hardware_cost"`

	// 折旧年限
	DepreciationYears int `json:"depreciation_years"`

	// 维护成本比例（年维护费/硬件成本）
	MaintenanceRate float64 `json:"maintenance_rate"`

	// 机柜租金（元/月）
	RackRent float64 `json:"rack_rent"`

	// 带宽成本（元/Mbps/月）
	BandwidthCost float64 `json:"bandwidth_cost"`

	// 人员成本（元/月）
	PersonnelCost float64 `json:"personnel_cost"`

	// 货币单位
	Currency string `json:"currency"`
}

// DefaultCostConfig 默认成本配置.
func DefaultCostConfig() CostConfig {
	return CostConfig{
		ElectricityRate:   0.6,    // 元/kWh
		DevicePowerWatts:  150,    // 瓦特
		HardwareCost:      50000,  // 元
		DepreciationYears: 5,      // 年
		MaintenanceRate:   0.1,    // 10%
		RackRent:          500,    // 元/月
		BandwidthCost:     50,     // 元/Mbps/月
		PersonnelCost:     2000,   // 元/月（分摊）
		Currency:          "CNY",
	}
}

// StorageCost 存储成本明细.
type StorageCost struct {
	// 时间周期
	Period ReportPeriod `json:"period"`

	// 电力成本
	ElectricityCost ElectricityCost `json:"electricity_cost"`

	// 硬件折旧成本
	DepreciationCost DepreciationCost `json:"depreciation_cost"`

	// 维护成本
	MaintenanceCost MaintenanceCost `json:"maintenance_cost"`

	// 其他运营成本
	OperatingCost OperatingCost `json:"operating_cost"`

	// 总成本（元）
	TotalCost float64 `json:"total_cost"`

	// 单位存储成本（元/TB/月）
	CostPerTB float64 `json:"cost_per_tb"`

	// 存储容量（TB）
	StorageCapacityTB float64 `json:"storage_capacity_tb"`

	// 计算时间
	CalculatedAt time.Time `json:"calculated_at"`
}

// ElectricityCost 电力成本.
type ElectricityCost struct {
	// 功率（瓦特）
	PowerWatts float64 `json:"power_watts"`

	// 日用电量（kWh）
	DailyKWh float64 `json:"daily_kwh"`

	// 月用电量（kWh）
	MonthlyKWh float64 `json:"monthly_kwh"`

	// 年用电量（kWh）
	YearlyKWh float64 `json:"yearly_kwh"`

	// 电费单价（元/kWh）
	Rate float64 `json:"rate"`

	// 日电费（元）
	DailyCost float64 `json:"daily_cost"`

	// 月电费（元）
	MonthlyCost float64 `json:"monthly_cost"`

	// 年电费（元）
	YearlyCost float64 `json:"yearly_cost"`

	// PUE（电能利用效率）
	PUE float64 `json:"pue"`
}

// DepreciationCost 硬件折旧成本.
type DepreciationCost struct {
	// 硬件原值（元）
	OriginalValue float64 `json:"original_value"`

	// 残值率
	SalvageRate float64 `json:"salvage_rate"`

	// 残值（元）
	SalvageValue float64 `json:"salvage_value"`

	// 折旧年限
	DepreciationYears int `json:"depreciation_years"`

	// 年折旧额（元）
	YearlyDepreciation float64 `json:"yearly_depreciation"`

	// 月折旧额（元）
	MonthlyDepreciation float64 `json:"monthly_depreciation"`

	// 日折旧额（元）
	DailyDepreciation float64 `json:"daily_depreciation"`

	// 已计提折旧（元）
	AccumulatedDepreciation float64 `json:"accumulated_depreciation"`

	// 账面净值（元）
	NetBookValue float64 `json:"net_book_value"`

	// 已使用年限
	YearsUsed float64 `json:"years_used"`

	// 折旧方法
	Method string `json:"method"` // straight_line, declining_balance
}

// MaintenanceCost 维护成本.
type MaintenanceCost struct {
	// 年维护费率
	AnnualRate float64 `json:"annual_rate"`

	// 年维护费（元）
	YearlyCost float64 `json:"yearly_cost"`

	// 月维护费（元）
	MonthlyCost float64 `json:"monthly_cost"`

	// 日维护费（元）
	DailyCost float64 `json:"daily_cost"`

	// 硬件更换储备（元）
	HardwareReserve float64 `json:"hardware_reserve"`

	// 软件许可费（元/月）
	SoftwareLicense float64 `json:"software_license"`
}

// OperatingCost 运营成本.
type OperatingCost struct {
	// 机柜租金（元/月）
	RackRent float64 `json:"rack_rent"`

	// 带宽成本（元/月）
	BandwidthCost float64 `json:"bandwidth_cost"`

	// 人员成本（元/月）
	PersonnelCost float64 `json:"personnel_cost"`

	// 月运营成本（元）
	MonthlyTotal float64 `json:"monthly_total"`

	// 年运营成本（元）
	YearlyTotal float64 `json:"yearly_total"`
}

// CostTrend 成本趋势.
type CostTrend struct {
	// 时间点
	Timestamp time.Time `json:"timestamp"`

	// 总成本
	TotalCost float64 `json:"total_cost"`

	// 电力成本
	ElectricityCost float64 `json:"electricity_cost"`

	// 折旧成本
	DepreciationCost float64 `json:"depreciation_cost"`

	// 运营成本
	OperatingCost float64 `json:"operating_cost"`

	// 单位成本（元/TB）
	CostPerTB float64 `json:"cost_per_tb"`

	// 存储容量（TB）
	StorageCapacityTB float64 `json:"storage_capacity_tb"`
}

// CostForecast 成本预测.
type CostForecast struct {
	// 预测时间
	Timestamp time.Time `json:"timestamp"`

	// 预测总成本（元）
	ForecastCost float64 `json:"forecast_cost"`

	// 预测单位成本（元/TB）
	ForecastCostPerTB float64 `json:"forecast_cost_per_tb"`

	// 置信区间下限
	ConfidenceLower float64 `json:"confidence_lower"`

	// 置信区间上限
	ConfidenceUpper float64 `json:"confidence_upper"`

	// 预测模型
	Model string `json:"model"`
}

// CostBreakdown 成本分解.
type CostBreakdown struct {
	// 成本类型
	Category string `json:"category"`

	// 成本金额
	Amount float64 `json:"amount"`

	// 占比
	Percent float64 `json:"percent"`

	// 描述
	Description string `json:"description"`
}

// CostAnalyzer 成本分析器.
type CostAnalyzer struct {
	config        CostConfig
	purchaseDate  time.Time
	storageBytes  uint64
	hoursPerDay   float64
	pue           float64
}

// NewCostAnalyzer 创建成本分析器.
func NewCostAnalyzer(config CostConfig) *CostAnalyzer {
	return &CostAnalyzer{
		config:       config,
		purchaseDate: time.Now().AddDate(-1, 0, 0), // 默认1年前购买
		hoursPerDay:  24,
		pue:          1.5, // 默认PUE
	}
}

// SetPurchaseDate 设置购买日期.
func (a *CostAnalyzer) SetPurchaseDate(date time.Time) {
	a.purchaseDate = date
}

// SetStorageCapacity 设置存储容量.
func (a *CostAnalyzer) SetStorageCapacity(bytes uint64) {
	a.storageBytes = bytes
}

// SetPUE 设置PUE.
func (a *CostAnalyzer) SetPUE(pue float64) {
	a.pue = pue
}

// SetOperatingHours 设置每日运行时间.
func (a *CostAnalyzer) SetOperatingHours(hours float64) {
	a.hoursPerDay = hours
}

// CalculateCost 计算存储成本.
func (a *CostAnalyzer) CalculateCost(period ReportPeriod) *StorageCost {
	now := time.Now()

	// 计算电力成本
	electricity := a.calculateElectricityCost()

	// 计算折旧成本
	depreciation := a.calculateDepreciationCost(now)

	// 计算维护成本
	maintenance := a.calculateMaintenanceCost()

	// 计算运营成本
	operating := a.calculateOperatingCost()

	// 计算总成本（月度）
	totalCost := electricity.MonthlyCost + depreciation.MonthlyDepreciation + maintenance.MonthlyCost + operating.MonthlyTotal

	// 计算单位成本
	storageTB := float64(a.storageBytes) / (1024 * 1024 * 1024 * 1024)
	costPerTB := 0.0
	if storageTB > 0 {
		costPerTB = totalCost / storageTB
	}

	return &StorageCost{
		Period:            period,
		ElectricityCost:   electricity,
		DepreciationCost:  depreciation,
		MaintenanceCost:   maintenance,
		OperatingCost:     operating,
		TotalCost:         round(totalCost, 2),
		CostPerTB:         round(costPerTB, 2),
		StorageCapacityTB: round(storageTB, 2),
		CalculatedAt:      now,
	}
}

// calculateElectricityCost 计算电力成本.
func (a *CostAnalyzer) calculateElectricityCost() ElectricityCost {
	// 考虑PUE的实际功率
	effectivePower := a.config.DevicePowerWatts * a.pue

	// 日用电量（kWh）= 功率(W) * 小时 / 1000
	dailyKWh := effectivePower * a.hoursPerDay / 1000
	monthlyKWh := dailyKWh * 30
	yearlyKWh := dailyKWh * 365

	// 电费
	dailyCost := dailyKWh * a.config.ElectricityRate
	monthlyCost := monthlyKWh * a.config.ElectricityRate
	yearlyCost := yearlyKWh * a.config.ElectricityRate

	return ElectricityCost{
		PowerWatts:  a.config.DevicePowerWatts,
		DailyKWh:    round(dailyKWh, 2),
		MonthlyKWh:  round(monthlyKWh, 2),
		YearlyKWh:   round(yearlyKWh, 2),
		Rate:        a.config.ElectricityRate,
		DailyCost:   round(dailyCost, 2),
		MonthlyCost: round(monthlyCost, 2),
		YearlyCost:  round(yearlyCost, 2),
		PUE:         a.pue,
	}
}

// calculateDepreciationCost 计算折旧成本.
func (a *CostAnalyzer) calculateDepreciationCost(now time.Time) DepreciationCost {
	salvageRate := 0.05 // 5%残值率
	salvageValue := a.config.HardwareCost * salvageRate
	depreciableValue := a.config.HardwareCost - salvageValue

	// 直线法折旧
	yearlyDepreciation := depreciableValue / float64(a.config.DepreciationYears)
	monthlyDepreciation := yearlyDepreciation / 12
	dailyDepreciation := yearlyDepreciation / 365

	// 计算已使用年限
	yearsUsed := now.Sub(a.purchaseDate).Hours() / (24 * 365)
	if yearsUsed > float64(a.config.DepreciationYears) {
		yearsUsed = float64(a.config.DepreciationYears)
	}

	// 已计提折旧
	accumulatedDepreciation := yearlyDepreciation * yearsUsed
	if accumulatedDepreciation > depreciableValue {
		accumulatedDepreciation = depreciableValue
	}

	// 账面净值
	netBookValue := a.config.HardwareCost - accumulatedDepreciation
	if netBookValue < salvageValue {
		netBookValue = salvageValue
	}

	return DepreciationCost{
		OriginalValue:           a.config.HardwareCost,
		SalvageRate:             salvageRate,
		SalvageValue:            round(salvageValue, 2),
		DepreciationYears:       a.config.DepreciationYears,
		YearlyDepreciation:      round(yearlyDepreciation, 2),
		MonthlyDepreciation:     round(monthlyDepreciation, 2),
		DailyDepreciation:       round(dailyDepreciation, 2),
		AccumulatedDepreciation: round(accumulatedDepreciation, 2),
		NetBookValue:            round(netBookValue, 2),
		YearsUsed:               round(yearsUsed, 2),
		Method:                  "straight_line",
	}
}

// calculateMaintenanceCost 计算维护成本.
func (a *CostAnalyzer) calculateMaintenanceCost() MaintenanceCost {
	yearlyCost := a.config.HardwareCost * a.config.MaintenanceRate
	monthlyCost := yearlyCost / 12
	dailyCost := yearlyCost / 365

	// 硬件更换储备（通常为硬件成本的3-5%/年）
	hardwareReserve := a.config.HardwareCost * 0.04 / 12

	return MaintenanceCost{
		AnnualRate:      a.config.MaintenanceRate,
		YearlyCost:      round(yearlyCost, 2),
		MonthlyCost:     round(monthlyCost, 2),
		DailyCost:       round(dailyCost, 2),
		HardwareReserve: round(hardwareReserve, 2),
		SoftwareLicense: 0, // 可选配置
	}
}

// calculateOperatingCost 计算运营成本.
func (a *CostAnalyzer) calculateOperatingCost() OperatingCost {
	monthlyTotal := a.config.RackRent + a.config.BandwidthCost + a.config.PersonnelCost
	yearlyTotal := monthlyTotal * 12

	return OperatingCost{
		RackRent:      a.config.RackRent,
		BandwidthCost: a.config.BandwidthCost,
		PersonnelCost: a.config.PersonnelCost,
		MonthlyTotal:  round(monthlyTotal, 2),
		YearlyTotal:   round(yearlyTotal, 2),
	}
}

// CalculateTrend 计算成本趋势.
func (a *CostAnalyzer) CalculateTrend(history []CostTrend) []CostTrend {
	return history
}

// ForecastCost 预测成本.
func (a *CostAnalyzer) ForecastCost(history []CostTrend, months int) []CostForecast {
	if len(history) < 2 {
		return nil
	}

	forecasts := make([]CostForecast, 0)
	latest := history[len(history)-1]

	// 计算月增长率
	growthRate := a.calculateGrowthRate(history)

	// 预测未来成本
	for month := 1; month <= months; month++ {
		forecastDate := latest.Timestamp.AddDate(0, month, 0)

		// 预测总成本（考虑增长）
		forecastCost := latest.TotalCost * math.Pow(1+growthRate, float64(month))

		// 预测存储容量
		forecastCapacity := latest.StorageCapacityTB * math.Pow(1+growthRate*0.5, float64(month))

		// 预测单位成本
		forecastCostPerTB := 0.0
		if forecastCapacity > 0 {
			forecastCostPerTB = forecastCost / forecastCapacity
		}

		// 置信区间（±15%）
		confidenceMargin := forecastCost * 0.15

		forecasts = append(forecasts, CostForecast{
			Timestamp:         forecastDate,
			ForecastCost:      round(forecastCost, 2),
			ForecastCostPerTB: round(forecastCostPerTB, 2),
			ConfidenceLower:   round(forecastCost-confidenceMargin, 2),
			ConfidenceUpper:   round(forecastCost+confidenceMargin, 2),
			Model:             "linear_growth",
		})
	}

	return forecasts
}

// calculateGrowthRate 计算增长率.
func (a *CostAnalyzer) calculateGrowthRate(history []CostTrend) float64 {
	if len(history) < 2 {
		return 0
	}

	first := history[0]
	last := history[len(history)-1]

	// 计算月数
	months := last.Timestamp.Sub(first.Timestamp).Hours() / (24 * 30)
	if months == 0 {
		return 0
	}

	// 计算复合增长率
	if first.TotalCost == 0 {
		return 0
	}

	ratio := last.TotalCost / first.TotalCost
	growthRate := math.Pow(ratio, 1/months) - 1

	return growthRate
}

// GetCostBreakdown 获取成本分解.
func (a *CostAnalyzer) GetCostBreakdown(cost *StorageCost) []CostBreakdown {
	breakdown := []CostBreakdown{
		{
			Category:    "electricity",
			Amount:      cost.ElectricityCost.MonthlyCost,
			Percent:     round(cost.ElectricityCost.MonthlyCost/cost.TotalCost*100, 1),
			Description: "电力成本",
		},
		{
			Category:    "depreciation",
			Amount:      cost.DepreciationCost.MonthlyDepreciation,
			Percent:     round(cost.DepreciationCost.MonthlyDepreciation/cost.TotalCost*100, 1),
			Description: "硬件折旧",
		},
		{
			Category:    "maintenance",
			Amount:      cost.MaintenanceCost.MonthlyCost,
			Percent:     round(cost.MaintenanceCost.MonthlyCost/cost.TotalCost*100, 1),
			Description: "维护费用",
		},
		{
			Category:    "rack_rent",
			Amount:      cost.OperatingCost.RackRent,
			Percent:     round(cost.OperatingCost.RackRent/cost.TotalCost*100, 1),
			Description: "机柜租金",
		},
		{
			Category:    "bandwidth",
			Amount:      cost.OperatingCost.BandwidthCost,
			Percent:     round(cost.OperatingCost.BandwidthCost/cost.TotalCost*100, 1),
			Description: "带宽成本",
		},
		{
			Category:    "personnel",
			Amount:      cost.OperatingCost.PersonnelCost,
			Percent:     round(cost.OperatingCost.PersonnelCost/cost.TotalCost*100, 1),
			Description: "人员成本",
		},
	}

	return breakdown
}

// CompareCost 对比成本变化.
func (a *CostAnalyzer) CompareCost(current, previous *StorageCost) *CostComparison {
	if previous == nil || current == nil {
		return nil
	}

	return &CostComparison{
		TotalCostChange:      current.TotalCost - previous.TotalCost,
		TotalCostChangePercent: round((current.TotalCost-previous.TotalCost)/previous.TotalCost*100, 2),
		CostPerTBChange:      current.CostPerTB - previous.CostPerTB,
		CostPerTBChangePercent: round((current.CostPerTB-previous.CostPerTB)/previous.CostPerTB*100, 2),
		ElectricityChange:    current.ElectricityCost.MonthlyCost - previous.ElectricityCost.MonthlyCost,
		DepreciationChange:   current.DepreciationCost.MonthlyDepreciation - previous.DepreciationCost.MonthlyDepreciation,
		OperatingChange:      current.OperatingCost.MonthlyTotal - previous.OperatingCost.MonthlyTotal,
	}
}

// CostComparison 成本对比.
type CostComparison struct {
	// 总成本变化（元）
	TotalCostChange float64 `json:"total_cost_change"`

	// 总成本变化百分比
	TotalCostChangePercent float64 `json:"total_cost_change_percent"`

	// 单位成本变化（元/TB）
	CostPerTBChange float64 `json:"cost_per_tb_change"`

	// 单位成本变化百分比
	CostPerTBChangePercent float64 `json:"cost_per_tb_change_percent"`

	// 电力成本变化
	ElectricityChange float64 `json:"electricity_change"`

	// 折旧成本变化
	DepreciationChange float64 `json:"depreciation_change"`

	// 运营成本变化
	OperatingChange float64 `json:"operating_change"`
}

// 辅助函数：四舍五入.
func round(value float64, places int) float64 {
	multiplier := math.Pow(10, float64(places))
	return math.Round(value*multiplier) / multiplier
}