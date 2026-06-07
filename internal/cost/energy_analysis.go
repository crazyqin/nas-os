package cost

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// EnergyAnalyzer 能耗分析器
type EnergyAnalyzer struct {
	configPath string
	config     EnergyConfig
}

// EnergyConfig 能耗配置
type EnergyConfig struct {
	// 磁盘功耗参数（瓦特）
	ActivePowerWatts float64 `json:"active_power_watts"` // 活跃状态功耗
	IdlePowerWatts   float64 `json:"idle_power_watts"`   // 空闲状态功耗
	SleepPowerWatts  float64 `json:"sleep_power_watts"`  // 休眠状态功耗

	// 电费参数
	ElectricityRatePerKWh float64 `json:"electricity_rate_per_kwh"` // 电费单价（元/kWh）

	// 时间参数
	DaysPerMonth int `json:"days_per_month"` // 每月天数
	HoursPerDay  int `json:"hours_per_day"`  // 每天运行小时
}

// EnergyStats 能耗统计
type EnergyStats struct {
	Device         string  `json:"device"`
	ActiveHours    float64 `json:"active_hours"`
	SleepHours     float64 `json:"sleep_hours"`
	ActivePowerKWh float64 `json:"active_power_kwh"`
	SleepPowerKWh  float64 `json:"sleep_power_kwh"`
	TotalPowerKWh  float64 `json:"total_power_kwh"`
	CostYuan       float64 `json:"cost_yuan"`
	SavingsKWh     float64 `json:"savings_kwh"`     // 节省电量
	SavingsYuan    float64 `json:"savings_yuan"`    // 节省费用
	SavingsPercent float64 `json:"savings_percent"` // 节省百分比
}

// EnergyReport 能耗报告
type EnergyReport struct {
	GeneratedAt      time.Time     `json:"generated_at"`
	Period           string        `json:"period"` // daily, monthly, yearly
	Stats            []EnergyStats `json:"stats"`
	TotalPowerKWh    float64       `json:"total_power_kwh"`
	TotalCostYuan    float64       `json:"total_cost_yuan"`
	TotalSavingsKWh  float64       `json:"total_savings_kwh"`
	TotalSavingsYuan float64       `json:"total_savings_yuan"`
	Recommendations  []string      `json:"recommendations"`
}

// NewEnergyAnalyzer 创建能耗分析器
func NewEnergyAnalyzer(configPath string) *EnergyAnalyzer {
	analyzer := &EnergyAnalyzer{
		configPath: configPath,
		config: EnergyConfig{
			ActivePowerWatts:      10.0, // HDD活跃约10W
			IdlePowerWatts:        5.0,  // HDD空闲约5W
			SleepPowerWatts:       1.0,  // HDD休眠约1W
			ElectricityRatePerKWh: 0.5,  // 电费约0.5元/kWh
			DaysPerMonth:          30,
			HoursPerDay:           24,
		},
	}
	analyzer.loadConfig()
	return analyzer
}

// loadConfig 加载配置
func (a *EnergyAnalyzer) loadConfig() error {
	data, err := os.ReadFile(a.configPath)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &a.config)
}

// SaveConfig 保存配置
func (a *EnergyAnalyzer) SaveConfig() error {
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.configPath, data, 0644)
}

// CalculateIdlePower 计算休眠节能
func (a *EnergyAnalyzer) CalculateIdlePower(device string, activeHours, sleepHours float64) EnergyStats {
	// 计算电量 (kWh)
	activePowerKWh := (activeHours * a.config.ActivePowerWatts) / 1000
	sleepPowerKWh := (sleepHours * a.config.SleepPowerWatts) / 1000
	totalPowerKWh := activePowerKWh + sleepPowerKWh

	// 计算费用
	costYuan := totalPowerKWh * a.config.ElectricityRatePerKWh

	// 计算节省（假设不休眠时的电量）
	noSleepPowerKWh := ((activeHours + sleepHours) * a.config.ActivePowerWatts) / 1000
	savingsKWh := noSleepPowerKWh - totalPowerKWh
	savingsYuan := savingsKWh * a.config.ElectricityRatePerKWh
	savingsPercent := (savingsKWh / noSleepPowerKWh) * 100

	return EnergyStats{
		Device:         device,
		ActiveHours:    activeHours,
		SleepHours:     sleepHours,
		ActivePowerKWh: activePowerKWh,
		SleepPowerKWh:  sleepPowerKWh,
		TotalPowerKWh:  totalPowerKWh,
		CostYuan:       costYuan,
		SavingsKWh:     savingsKWh,
		SavingsYuan:    savingsYuan,
		SavingsPercent: savingsPercent,
	}
}

// CompareCost 对比不同休眠策略的成本
func (a *EnergyAnalyzer) CompareCost(device string, scenarios []SleepScenario) []EnergyStats {
	results := make([]EnergyStats, len(scenarios))
	for i, scenario := range scenarios {
		results[i] = a.CalculateIdlePower(device, scenario.ActiveHours, scenario.SleepHours)
	}
	return results
}

// SleepScenario 休眠场景
type SleepScenario struct {
	Name        string  `json:"name"`
	ActiveHours float64 `json:"active_hours"`
	SleepHours  float64 `json:"sleep_hours"`
}

// GenerateDailyReport 生成每日报告
func (a *EnergyAnalyzer) GenerateDailyReport(stats []EnergyStats) EnergyReport {
	report := EnergyReport{
		GeneratedAt:     time.Now(),
		Period:          "daily",
		Stats:           stats,
		Recommendations: []string{},
	}

	for _, s := range stats {
		report.TotalPowerKWh += s.TotalPowerKWh
		report.TotalCostYuan += s.CostYuan
		report.TotalSavingsKWh += s.SavingsKWh
		report.TotalSavingsYuan += s.SavingsYuan
	}

	// 生成建议
	if report.TotalSavingsPercent() > 30 {
		report.Recommendations = append(report.Recommendations,
			"当前休眠策略节能效果显著，建议继续保持")
	}
	if report.TotalSavingsPercent() < 10 {
		report.Recommendations = append(report.Recommendations,
			"建议增加休眠时间或降低空闲阈值以提高节能效果")
	}

	return report
}

// TotalSavingsPercent 计算总节省百分比
func (r *EnergyReport) TotalSavingsPercent() float64 {
	if r.TotalPowerKWh+r.TotalSavingsKWh == 0 {
		return 0
	}
	return (r.TotalSavingsKWh / (r.TotalPowerKWh + r.TotalSavingsKWh)) * 100
}

// GenerateMonthlyReport 生成月度报告
func (a *EnergyAnalyzer) GenerateMonthlyReport(stats []EnergyStats) EnergyReport {
	daily := a.GenerateDailyReport(stats)

	// 扩展为月度
	daily.Period = "monthly"
	daily.TotalPowerKWh *= float64(a.config.DaysPerMonth)
	daily.TotalCostYuan *= float64(a.config.DaysPerMonth)
	daily.TotalSavingsKWh *= float64(a.config.DaysPerMonth)
	daily.TotalSavingsYuan *= float64(a.config.DaysPerMonth)

	for i := range daily.Stats {
		daily.Stats[i].ActiveHours *= float64(a.config.DaysPerMonth)
		daily.Stats[i].SleepHours *= float64(a.config.DaysPerMonth)
		daily.Stats[i].ActivePowerKWh *= float64(a.config.DaysPerMonth)
		daily.Stats[i].SleepPowerKWh *= float64(a.config.DaysPerMonth)
		daily.Stats[i].TotalPowerKWh *= float64(a.config.DaysPerMonth)
		daily.Stats[i].CostYuan *= float64(a.config.DaysPerMonth)
		daily.Stats[i].SavingsKWh *= float64(a.config.DaysPerMonth)
		daily.Stats[i].SavingsYuan *= float64(a.config.DaysPerMonth)
	}

	return daily
}

// GenerateAnnualReport 生成年度报告
func (a *EnergyAnalyzer) GenerateAnnualReport(stats []EnergyStats) EnergyReport {
	monthly := a.GenerateMonthlyReport(stats)

	// 扩展为年度
	monthly.Period = "annual"
	monthsPerYear := 12
	monthly.TotalPowerKWh *= float64(monthsPerYear)
	monthly.TotalCostYuan *= float64(monthsPerYear)
	monthly.TotalSavingsKWh *= float64(monthsPerYear)
	monthly.TotalSavingsYuan *= float64(monthsPerYear)

	for i := range monthly.Stats {
		monthly.Stats[i].CostYuan *= float64(monthsPerYear)
		monthly.Stats[i].SavingsYuan *= float64(monthsPerYear)
	}

	// 年度建议
	monthly.Recommendations = append(monthly.Recommendations,
		fmt.Sprintf("预计年度节省电费: %.2f元", monthly.TotalSavingsYuan))

	return monthly
}

// EstimateSavings 估算节能潜力
func (a *EnergyAnalyzer) EstimateSavings(diskCount int, avgSleepHoursPerDay float64) EnergyStats {
	totalActiveHours := float64(a.config.HoursPerDay) - avgSleepHoursPerDay
	totalSleepHours := avgSleepHoursPerDay

	// 每日节能
	dailySavings := (totalSleepHours * (a.config.ActivePowerWatts - a.config.SleepPowerWatts)) / 1000

	// 月度节能
	monthlySavings := dailySavings * float64(a.config.DaysPerMonth)

	// 年度节能（用于统计，暂不返回）
	_ = monthlySavings * 12

	return EnergyStats{
		Device:         fmt.Sprintf("%d disks", diskCount),
		ActiveHours:    totalActiveHours * float64(diskCount),
		SleepHours:     totalSleepHours * float64(diskCount),
		SavingsKWh:     dailySavings,
		SavingsYuan:    dailySavings * a.config.ElectricityRatePerKWh,
		SavingsPercent: (avgSleepHoursPerDay / float64(a.config.HoursPerDay)) * 100,
	}
}

// SetPowerConfig 设置功耗参数
func (a *EnergyAnalyzer) SetPowerConfig(activeWatts, idleWatts, sleepWatts float64) {
	a.config.ActivePowerWatts = activeWatts
	a.config.IdlePowerWatts = idleWatts
	a.config.SleepPowerWatts = sleepWatts
}

// SetElectricityRate 设置电费单价
func (a *EnergyAnalyzer) SetElectricityRate(rate float64) {
	a.config.ElectricityRatePerKWh = rate
}
