// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ========== 分时电价配置 ==========

// TimeOfDay 时段类型
type TimeOfDay string

const (
	TimeOfDayPeak     TimeOfDay = "peak"     // 峰时
	TimeOfDayFlat     TimeOfDay = "flat"     // 平时
	TimeOfDayValley   TimeOfDay = "valley"   // 谷时
	TimeOfDayCritical TimeOfDay = "critical" // 尖峰
)

// TimeSlot 时段定义
type TimeSlot struct {
	StartHour int       `json:"start_hour"` // 开始小时 (0-23)
	EndHour   int       `json:"end_hour"`   // 结束小时 (0-23)
	Type      TimeOfDay `json:"type"`       // 时段类型
}

// SeasonType 季节类型
type SeasonType string

const (
	SeasonSpring  SeasonType = "spring"  // 春季 (3-5月)
	SeasonSummer  SeasonType = "summer"  // 夏季 (6-8月)
	SeasonAutumn  SeasonType = "autumn"  // 秋季 (9-11月)
	SeasonWinter  SeasonType = "winter"  // 冬季 (12-2月)
	SeasonDefault SeasonType = "default" // 默认
)

// SeasonalPrice 季节电价
type SeasonalPrice struct {
	Season      SeasonType `json:"season"`
	PeakPrice   float64    `json:"peak_price"`   // 峰时电价 (元/kWh)
	FlatPrice   float64    `json:"flat_price"`   // 平时电价 (元/kWh)
	ValleyPrice float64    `json:"valley_price"` // 谷时电价 (元/kWh)
	CritPrice   float64    `json:"crit_price"`   // 尖峰电价 (元/kWh)
}

// ElectricityTariff 电价配置
type ElectricityTariff struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Region         string          `json:"region"`          // 地区
	VoltageLevel   string          `json:"voltage_level"`   // 电压等级 (低压/高压)
	TimeSlots      []TimeSlot      `json:"time_slots"`      // 时段定义
	SeasonalPrices []SeasonalPrice `json:"seasonal_prices"` // 季节电价
	EffectiveFrom  time.Time       `json:"effective_from"`  // 生效日期
	EffectiveTo    *time.Time      `json:"effective_to"`    // 失效日期
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// GetTimeOfDay 获取指定时间的时段类型
func (t *ElectricityTariff) GetTimeOfDay(hour int, season SeasonType) TimeOfDay {
	for _, slot := range t.TimeSlots {
		if slot.StartHour <= slot.EndHour {
			if hour >= slot.StartHour && hour < slot.EndHour {
				return slot.Type
			}
		} else {
			// 跨午夜时段 (如 23:00 - 06:00)
			if hour >= slot.StartHour || hour < slot.EndHour {
				return slot.Type
			}
		}
	}
	return TimeOfDayFlat
}

// GetPrice 获取指定时段的电价
func (t *ElectricityTariff) GetPrice(timeOfDay TimeOfDay, season SeasonType) float64 {
	for _, sp := range t.SeasonalPrices {
		if sp.Season == season {
			switch timeOfDay {
			case TimeOfDayCritical:
				if sp.CritPrice > 0 {
					return sp.CritPrice
				}
				return sp.PeakPrice
			case TimeOfDayPeak:
				return sp.PeakPrice
			case TimeOfDayValley:
				return sp.ValleyPrice
			default:
				return sp.FlatPrice
			}
		}
	}
	// 默认返回第一季节电价
	if len(t.SeasonalPrices) > 0 {
		sp := t.SeasonalPrices[0]
		switch timeOfDay {
		case TimeOfDayCritical:
			if sp.CritPrice > 0 {
				return sp.CritPrice
			}
			return sp.PeakPrice
		case TimeOfDayPeak:
			return sp.PeakPrice
		case TimeOfDayValley:
			return sp.ValleyPrice
		default:
			return sp.FlatPrice
		}
	}
	return 0.5 // 默认电价
}

// ========== 设备功耗 ==========

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeServer  DeviceType = "server"  // 服务器
	DeviceTypeStorage DeviceType = "storage" // 存储设备
	DeviceTypeNetwork DeviceType = "network" // 网络设备
	DeviceTypeCooling DeviceType = "cooling" // 制冷设备
	DeviceTypeUPS     DeviceType = "ups"     // UPS
	DeviceTypeOther   DeviceType = "other"   // 其他
)

// PowerProfile 功耗配置
type PowerProfile struct {
	ID                string     `json:"id"`
	DeviceName        string     `json:"device_name"`
	DeviceType        DeviceType `json:"device_type"`
	IdlePowerWatts    float64    `json:"idle_power_watts"`    // 空载功耗 (W)
	MaxPowerWatts     float64    `json:"max_power_watts"`     // 最大功耗 (W)
	TypicalPowerWatts float64    `json:"typical_power_watts"` // 典型功耗 (W)
	StandbyPowerWatts float64    `json:"standby_power_watts"` // 待机功耗 (W)
	DailyOnHours      float64    `json:"daily_on_hours"`      // 每日运行时间
	WeeklyOnDays      int        `json:"weekly_on_days"`      // 每周运行天数
	Location          string     `json:"location"`            // 物理位置
	Description       string     `json:"description"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// PowerReading 功耗读数
type PowerReading struct {
	DeviceID   string    `json:"device_id"`
	DeviceName string    `json:"device_name"`
	Timestamp  time.Time `json:"timestamp"`
	PowerWatts float64   `json:"power_watts"` // 实时功耗 (W)
	EnergyKWh  float64   `json:"energy_kwh"`  // 累计电量 (kWh)
	Source     string    `json:"source"`      // 数据来源 (sensor/manual/estimated)
}

// PowerUsageRecord 用电记录
type PowerUsageRecord struct {
	ID             string    `json:"id"`
	DeviceID       string    `json:"device_id"`
	DeviceName     string    `json:"device_name"`
	PeriodStart    time.Time `json:"period_start"`
	PeriodEnd      time.Time `json:"period_end"`
	EnergyKWh      float64   `json:"energy_kwh"`       // 用电量 (kWh)
	AvgPowerWatts  float64   `json:"avg_power_watts"`  // 平均功率 (W)
	PeakPowerWatts float64   `json:"peak_power_watts"` // 峰值功率 (W)
	TimeOfDay      TimeOfDay `json:"time_of_day"`      // 主要时段
	Cost           float64   `json:"cost"`             // 电费 (元)
}

// ========== 电费计算 ==========

// ElectricityCostConfig 电费计算配置
type ElectricityCostConfig struct {
	TariffID           string  `json:"tariff_id"`
	Region             string  `json:"region"`
	Currency           string  `json:"currency"`             // 货币单位
	TaxRate            float64 `json:"tax_rate"`             // 税率
	AddOnCharges       float64 `json:"add_on_charges"`       // 附加费 (元/kWh)
	PowerFactorPenalty float64 `json:"power_factor_penalty"` // 功率因数惩罚
	DiscountRate       float64 `json:"discount_rate"`        // 折扣率
}

// ElectricityCostResult 电费计算结果
type ElectricityCostResult struct {
	ID                 string            `json:"id"`
	DeviceID           string            `json:"device_id"`
	DeviceName         string            `json:"device_name"`
	Period             ReportPeriod      `json:"period"`
	TotalEnergyKWh     float64           `json:"total_energy_kwh"`      // 总用电量 (kWh)
	PeakEnergyKWh      float64           `json:"peak_energy_kwh"`       // 峰时用电量
	FlatEnergyKWh      float64           `json:"flat_energy_kwh"`       // 平时用电量
	ValleyEnergyKWh    float64           `json:"valley_energy_kwh"`     // 谷时用电量
	CriticalEnergyKWh  float64           `json:"critical_energy_kwh"`   // 尖峰用电量
	PeakCost           float64           `json:"peak_cost"`             // 峰时电费
	FlatCost           float64           `json:"flat_cost"`             // 平时电费
	ValleyCost         float64           `json:"valley_cost"`           // 谷时电费
	CriticalCost       float64           `json:"critical_cost"`         // 尖峰电费
	EnergyCost         float64           `json:"energy_cost"`           // 电度电费
	AddOnCharges       float64           `json:"add_on_charges"`        // 附加费
	TaxAmount          float64           `json:"tax_amount"`            // 税费
	DiscountAmount     float64           `json:"discount_amount"`       // 折扣金额
	TotalCost          float64           `json:"total_cost"`            // 总电费
	AveragePricePerKWh float64           `json:"average_price_per_kwh"` // 平均电价
	CostPerDay         float64           `json:"cost_per_day"`          // 日均电费
	CostPerHour        float64           `json:"cost_per_hour"`         // 时均电费
	HourlyBreakdown    []HourlyCost      `json:"hourly_breakdown"`      // 小时分解
	DailyBreakdown     []DailyCost       `json:"daily_breakdown"`       // 日分解
	TariffUsed         ElectricityTariff `json:"tariff_used"`
	GeneratedAt        time.Time         `json:"generated_at"`
}

// HourlyCost 小时成本
type HourlyCost struct {
	Hour      int       `json:"hour"`
	Date      time.Time `json:"date"`
	TimeOfDay TimeOfDay `json:"time_of_day"`
	EnergyKWh float64   `json:"energy_kwh"`
	Price     float64   `json:"price"`
	Cost      float64   `json:"cost"`
}

// DailyCost 日成本
type DailyCost struct {
	Date            time.Time `json:"date"`
	TotalEnergyKWh  float64   `json:"total_energy_kwh"`
	TotalCost       float64   `json:"total_cost"`
	PeakEnergyKWh   float64   `json:"peak_energy_kwh"`
	ValleyEnergyKWh float64   `json:"valley_energy_kwh"`
}

// ElectricityBill 电费账单
type ElectricityBill struct {
	ID                string                `json:"id"`
	BillNumber        string                `json:"bill_number"`        // 账单号
	CustomerName      string                `json:"customer_name"`      // 客户名称
	BillingPeriod     ReportPeriod          `json:"billing_period"`     // 计费周期
	DueDate           time.Time             `json:"due_date"`           // 到期日
	MeterReadings     []MeterReading        `json:"meter_readings"`     // 抄表读数
	EnergyConsumption float64               `json:"energy_consumption"` // 用电量 (kWh)
	PeakConsumption   float64               `json:"peak_consumption"`   // 峰时用电
	ValleyConsumption float64               `json:"valley_consumption"` // 谷时用电
	DemandCharge      float64               `json:"demand_charge"`      // 需量电费
	EnergyCharge      float64               `json:"energy_charge"`      // 电度电费
	AddOnCharges      float64               `json:"add_on_charges"`     // 附加费
	TaxAmount         float64               `json:"tax_amount"`         // 税费
	Adjustment        float64               `json:"adjustment"`         // 调整金额
	PreviousBalance   float64               `json:"previous_balance"`   // 上期余额
	TotalAmount       float64               `json:"total_amount"`       // 应付总额
	Status            string                `json:"status"`             // pending/paid/overdue
	PaymentDate       *time.Time            `json:"payment_date"`       // 支付日期
	DeviceBreakdown   []DeviceCostBreakdown `json:"device_breakdown"`   // 设备成本分解
	CostTrend         []CostTrendPoint      `json:"cost_trend"`         // 成本趋势
	GeneratedAt       time.Time             `json:"generated_at"`
}

// MeterReading 抄表读数
type MeterReading struct {
	MeterID      string    `json:"meter_id"`
	MeterName    string    `json:"meter_name"`
	ReadingDate  time.Time `json:"reading_date"`
	ReadingValue float64   `json:"reading_value"` // 表读数 (kWh)
	EnergyUsed   float64   `json:"energy_used"`   // 本期用电 (kWh)
}

// DeviceCostBreakdown 设备成本分解
type DeviceCostBreakdown struct {
	DeviceID       string     `json:"device_id"`
	DeviceName     string     `json:"device_name"`
	DeviceType     DeviceType `json:"device_type"`
	EnergyKWh      float64    `json:"energy_kwh"`
	Cost           float64    `json:"cost"`
	PercentOfTotal float64    `json:"percent_of_total"`
}

// ElectricityTrendPoint 电费趋势点
type ElectricityTrendPoint struct {
	Date           time.Time `json:"date"`
	Cost           float64   `json:"cost"`
	EnergyKWh      float64   `json:"energy_kwh"`
	AvgPricePerKWh float64   `json:"avg_price_per_kwh"`
}

// ========== 成本预测 ==========

// ElectricityForecast 电费预测
type ElectricityForecast struct {
	ID                   string                          `json:"id"`
	Period               ReportPeriod                    `json:"period"`               // 预测周期
	HistoricalPeriod     ReportPeriod                    `json:"historical_period"`    // 历史数据周期
	TotalPredictedKWh    float64                         `json:"total_predicted_kwh"`  // 预测总电量
	TotalPredictedCost   float64                         `json:"total_predicted_cost"` // 预测总电费
	PeakPredictedKWh     float64                         `json:"peak_predicted_kwh"`   // 峰时预测电量
	ValleyPredictedKWh   float64                         `json:"valley_predicted_kwh"` // 谷时预测电量
	MonthlyBreakdown     []MonthlyForecast               `json:"monthly_breakdown"`
	ConfidenceLevel      float64                         `json:"confidence_level"`      // 置信度
	SeasonalAdjustment   float64                         `json:"seasonal_adjustment"`   // 季节调整系数
	GrowthRate           float64                         `json:"growth_rate"`           // 增长率
	SavingsOpportunities []ElectricitySavingsOpportunity `json:"savings_opportunities"` // 节省机会
	GeneratedAt          time.Time                       `json:"generated_at"`
}

// MonthlyForecast 月度预测
type MonthlyForecast struct {
	Month           time.Time `json:"month"`
	PredictedKWh    float64   `json:"predicted_kwh"`
	PredictedCost   float64   `json:"predicted_cost"`
	ConfidenceLower float64   `json:"confidence_lower"`
	ConfidenceUpper float64   `json:"confidence_upper"`
	SeasonFactor    float64   `json:"season_factor"`
}

// ElectricitySavingsOpportunity 电费节省机会
type ElectricitySavingsOpportunity struct {
	ID               string  `json:"id"`
	Type             string  `json:"type"` // load_shift/efficiency/schedule
	Title            string  `json:"title"`
	Description      string  `json:"description"`
	PotentialSavings float64 `json:"potential_savings"` // 潜在节省 (元/月)
	Priority         int     `json:"priority"`          // 优先级 1-10
	Effort           string  `json:"effort"`            // low/medium/high
	Impact           string  `json:"impact"`            // 描述影响
}

// ========== 电费计算器 ==========

// ElectricityCostCalculator 电费计算器
type ElectricityCostCalculator struct {
	config   ElectricityCostConfig
	tariff   ElectricityTariff
	profiles map[string]PowerProfile
}

// NewElectricityCostCalculator 创建电费计算器
func NewElectricityCostCalculator(config ElectricityCostConfig, tariff ElectricityTariff) *ElectricityCostCalculator {
	return &ElectricityCostCalculator{
		config:   config,
		tariff:   tariff,
		profiles: make(map[string]PowerProfile),
	}
}

// AddPowerProfile 添加设备功耗配置
func (c *ElectricityCostCalculator) AddPowerProfile(profile PowerProfile) {
	c.profiles[profile.ID] = profile
}

// RemovePowerProfile 移除设备功耗配置
func (c *ElectricityCostCalculator) RemovePowerProfile(profileID string) {
	delete(c.profiles, profileID)
}

// GetPowerProfile 获取设备功耗配置
func (c *ElectricityCostCalculator) GetPowerProfile(profileID string) (PowerProfile, bool) {
	profile, ok := c.profiles[profileID]
	return profile, ok
}

// ListPowerProfiles 列出所有设备功耗配置
func (c *ElectricityCostCalculator) ListPowerProfiles() []PowerProfile {
	profiles := make([]PowerProfile, 0, len(c.profiles))
	for _, p := range c.profiles {
		profiles = append(profiles, p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].DeviceName < profiles[j].DeviceName
	})
	return profiles
}

// Calculate 计算指定设备的电费
func (c *ElectricityCostCalculator) Calculate(deviceID string, readings []PowerReading, period ReportPeriod) *ElectricityCostResult {
	profile, hasProfile := c.profiles[deviceID]

	result := &ElectricityCostResult{
		ID:              "elec_" + time.Now().Format("20060102150405") + "_" + deviceID,
		DeviceID:        deviceID,
		Period:          period,
		HourlyBreakdown: make([]HourlyCost, 0),
		DailyBreakdown:  make([]DailyCost, 0),
		TariffUsed:      c.tariff,
		GeneratedAt:     time.Now(),
	}

	if hasProfile {
		result.DeviceName = profile.DeviceName
	}

	// 按小时分组计算
	hourlyData := make(map[time.Time]*HourlyCost)
	dailyData := make(map[time.Time]*DailyCost)

	for _, reading := range readings {
		if reading.DeviceID != deviceID {
			continue
		}

		hourKey := reading.Timestamp.Truncate(time.Hour)
		dayKey := reading.Timestamp.Truncate(24 * time.Hour)

		// 获取时段类型
		hour := reading.Timestamp.Hour()
		season := getSeason(reading.Timestamp)
		timeOfDay := c.tariff.GetTimeOfDay(hour, season)
		price := c.tariff.GetPrice(timeOfDay, season)

		// 累计小时数据
		if _, ok := hourlyData[hourKey]; !ok {
			hourlyData[hourKey] = &HourlyCost{
				Hour:      hour,
				Date:      hourKey,
				TimeOfDay: timeOfDay,
				Price:     price,
			}
		}
		hourlyData[hourKey].EnergyKWh += reading.EnergyKWh
		hourlyData[hourKey].Cost += reading.EnergyKWh * price

		// 累计日数据
		if _, ok := dailyData[dayKey]; !ok {
			dailyData[dayKey] = &DailyCost{Date: dayKey}
		}
		dailyData[dayKey].TotalEnergyKWh += reading.EnergyKWh
		dailyData[dayKey].TotalCost += reading.EnergyKWh * price
		switch timeOfDay {
		case TimeOfDayPeak, TimeOfDayCritical:
			dailyData[dayKey].PeakEnergyKWh += reading.EnergyKWh
		case TimeOfDayValley:
			dailyData[dayKey].ValleyEnergyKWh += reading.EnergyKWh
		}
	}

	// 转换为切片并计算总计
	for _, hc := range hourlyData {
		result.HourlyBreakdown = append(result.HourlyBreakdown, *hc)
		result.TotalEnergyKWh += hc.EnergyKWh

		switch hc.TimeOfDay {
		case TimeOfDayCritical:
			result.CriticalEnergyKWh += hc.EnergyKWh
			result.CriticalCost += hc.Cost
		case TimeOfDayPeak:
			result.PeakEnergyKWh += hc.EnergyKWh
			result.PeakCost += hc.Cost
		case TimeOfDayValley:
			result.ValleyEnergyKWh += hc.EnergyKWh
			result.ValleyCost += hc.Cost
		default:
			result.FlatEnergyKWh += hc.EnergyKWh
			result.FlatCost += hc.Cost
		}
	}

	for _, dc := range dailyData {
		result.DailyBreakdown = append(result.DailyBreakdown, *dc)
	}

	// 排序
	sort.Slice(result.HourlyBreakdown, func(i, j int) bool {
		return result.HourlyBreakdown[i].Date.Before(result.HourlyBreakdown[j].Date)
	})
	sort.Slice(result.DailyBreakdown, func(i, j int) bool {
		return result.DailyBreakdown[i].Date.Before(result.DailyBreakdown[j].Date)
	})

	// 计算总费用
	result.EnergyCost = result.PeakCost + result.FlatCost + result.ValleyCost + result.CriticalCost
	result.AddOnCharges = result.TotalEnergyKWh * c.config.AddOnCharges
	result.TaxAmount = (result.EnergyCost + result.AddOnCharges) * c.config.TaxRate
	result.DiscountAmount = (result.EnergyCost + result.AddOnCharges) * c.config.DiscountRate
	result.TotalCost = result.EnergyCost + result.AddOnCharges + result.TaxAmount - result.DiscountAmount

	// 计算平均值
	if result.TotalEnergyKWh > 0 {
		result.AveragePricePerKWh = result.TotalCost / result.TotalEnergyKWh
	}

	days := period.EndTime.Sub(period.StartTime).Hours() / 24
	if days > 0 {
		result.CostPerDay = result.TotalCost / days
		result.CostPerHour = result.TotalCost / (days * 24)
	}

	return result
}

// CalculateAll 计算所有设备的电费
func (c *ElectricityCostCalculator) CalculateAll(readings []PowerReading, period ReportPeriod) map[string]*ElectricityCostResult {
	results := make(map[string]*ElectricityCostResult)

	deviceReadings := make(map[string][]PowerReading)
	for _, r := range readings {
		deviceReadings[r.DeviceID] = append(deviceReadings[r.DeviceID], r)
	}

	for deviceID := range deviceReadings {
		results[deviceID] = c.Calculate(deviceID, deviceReadings[deviceID], period)
	}

	return results
}

// EstimateFromProfile 从功耗配置估算电费
func (c *ElectricityCostCalculator) EstimateFromProfile(profileID string, days int) *ElectricityCostResult {
	profile, ok := c.profiles[profileID]
	if !ok {
		return nil
	}

	result := &ElectricityCostResult{
		ID:          "elec_est_" + time.Now().Format("20060102150405") + "_" + profileID,
		DeviceID:    profileID,
		DeviceName:  profile.DeviceName,
		Period:      ReportPeriod{StartTime: time.Now().AddDate(0, 0, -days), EndTime: time.Now()},
		TariffUsed:  c.tariff,
		GeneratedAt: time.Now(),
	}

	// 计算每日用电量
	dailyHours := profile.DailyOnHours * float64(profile.WeeklyOnDays) / 7.0
	dailyEnergy := (profile.TypicalPowerWatts / 1000.0) * dailyHours // kWh

	// 假设时段分布 (可配置)
	peakRatio := 0.3     // 峰时30%
	flatRatio := 0.4     // 平时40%
	valleyRatio := 0.3   // 谷时30%
	criticalRatio := 0.0 // 尖峰0%

	// 获取当前季节电价
	season := getSeason(time.Now())
	peakPrice := c.tariff.GetPrice(TimeOfDayPeak, season)
	flatPrice := c.tariff.GetPrice(TimeOfDayFlat, season)
	valleyPrice := c.tariff.GetPrice(TimeOfDayValley, season)
	criticalPrice := c.tariff.GetPrice(TimeOfDayCritical, season)

	// 计算分时电量
	result.TotalEnergyKWh = dailyEnergy * float64(days)
	result.PeakEnergyKWh = result.TotalEnergyKWh * peakRatio
	result.FlatEnergyKWh = result.TotalEnergyKWh * flatRatio
	result.ValleyEnergyKWh = result.TotalEnergyKWh * valleyRatio
	result.CriticalEnergyKWh = result.TotalEnergyKWh * criticalRatio

	// 计算分时费用
	result.PeakCost = result.PeakEnergyKWh * peakPrice
	result.FlatCost = result.FlatEnergyKWh * flatPrice
	result.ValleyCost = result.ValleyEnergyKWh * valleyPrice
	result.CriticalCost = result.CriticalEnergyKWh * criticalPrice

	// 计算总费用
	result.EnergyCost = result.PeakCost + result.FlatCost + result.ValleyCost + result.CriticalCost
	result.AddOnCharges = result.TotalEnergyKWh * c.config.AddOnCharges
	result.TaxAmount = (result.EnergyCost + result.AddOnCharges) * c.config.TaxRate
	result.DiscountAmount = (result.EnergyCost + result.AddOnCharges) * c.config.DiscountRate
	result.TotalCost = result.EnergyCost + result.AddOnCharges + result.TaxAmount - result.DiscountAmount

	if result.TotalEnergyKWh > 0 {
		result.AveragePricePerKWh = result.TotalCost / result.TotalEnergyKWh
	}
	result.CostPerDay = result.TotalCost / float64(days)
	result.CostPerHour = result.TotalCost / (float64(days) * 24)

	return result
}

// GenerateBill 生成电费账单
func (c *ElectricityCostCalculator) GenerateBill(results map[string]*ElectricityCostResult, period ReportPeriod, customerName string) *ElectricityBill {
	bill := &ElectricityBill{
		ID:              "bill_" + time.Now().Format("20060102150405"),
		BillNumber:      fmt.Sprintf("ELEC-%s", time.Now().Format("200601-000001")),
		CustomerName:    customerName,
		BillingPeriod:   period,
		DueDate:         period.EndTime.AddDate(0, 0, 15), // 15天后到期
		MeterReadings:   make([]MeterReading, 0),
		DeviceBreakdown: make([]DeviceCostBreakdown, 0),
		CostTrend:       make([]CostTrendPoint, 0),
		Status:          "pending",
		GeneratedAt:     time.Now(),
	}

	totalEnergy := 0.0
	totalPeak := 0.0
	totalValley := 0.0

	// 汇总各设备成本
	for deviceID, result := range results {
		totalEnergy += result.TotalEnergyKWh
		totalPeak += result.PeakEnergyKWh + result.CriticalEnergyKWh
		totalValley += result.ValleyEnergyKWh

		percent := 0.0
		if bill.EnergyConsumption > 0 {
			percent = result.TotalEnergyKWh / bill.EnergyConsumption * 100
		}

		profile, hasProfile := c.profiles[deviceID]
		deviceType := DeviceTypeOther
		if hasProfile {
			deviceType = profile.DeviceType
		}

		bill.DeviceBreakdown = append(bill.DeviceBreakdown, DeviceCostBreakdown{
			DeviceID:       deviceID,
			DeviceName:     result.DeviceName,
			DeviceType:     deviceType,
			EnergyKWh:      result.TotalEnergyKWh,
			Cost:           result.TotalCost,
			PercentOfTotal: percent,
		})

		bill.EnergyCharge += result.EnergyCost
	}

	bill.EnergyConsumption = totalEnergy
	bill.PeakConsumption = totalPeak
	bill.ValleyConsumption = totalValley
	bill.AddOnCharges = totalEnergy * c.config.AddOnCharges
	bill.TaxAmount = (bill.EnergyCharge + bill.AddOnCharges) * c.config.TaxRate
	bill.TotalAmount = bill.EnergyCharge + bill.AddOnCharges + bill.TaxAmount + bill.Adjustment + bill.PreviousBalance

	// 按成本排序设备
	sort.Slice(bill.DeviceBreakdown, func(i, j int) bool {
		return bill.DeviceBreakdown[i].Cost > bill.DeviceBreakdown[j].Cost
	})

	// 更新百分比
	for i := range bill.DeviceBreakdown {
		if bill.EnergyConsumption > 0 {
			bill.DeviceBreakdown[i].PercentOfTotal = bill.DeviceBreakdown[i].EnergyKWh / bill.EnergyConsumption * 100
		}
	}

	return bill
}

// Forecast 预测未来电费
func (c *ElectricityCostCalculator) Forecast(historical []ElectricityTrendPoint, months int) *ElectricityForecast {
	if len(historical) < 2 {
		return nil
	}

	now := time.Now()
	forecast := &ElectricityForecast{
		ID:                   "fcst_" + now.Format("20060102150405"),
		HistoricalPeriod:     ReportPeriod{StartTime: historical[0].Date, EndTime: historical[len(historical)-1].Date},
		Period:               ReportPeriod{StartTime: now, EndTime: now.AddDate(0, months, 0)},
		MonthlyBreakdown:     make([]MonthlyForecast, 0),
		SavingsOpportunities: make([]ElectricitySavingsOpportunity, 0),
		GeneratedAt:          now,
	}

	// 计算历史趋势
	growthRate := c.calculateGrowthRate(historical)
	seasonalFactors := c.calculateSeasonalFactors(historical)

	// 预测每月电费
	for i := 1; i <= months; i++ {
		month := now.AddDate(0, i, 0)
		season := getSeason(month)
		seasonFactor := seasonalFactors[season]

		// 基于最近数据预测
		lastPoint := historical[len(historical)-1]
		predictedEnergy := lastPoint.EnergyKWh * math.Pow(1+growthRate, float64(i)) * seasonFactor
		predictedCost := lastPoint.Cost * math.Pow(1+growthRate, float64(i)) * seasonFactor

		// 置信区间 (±15%)
		confidenceLower := predictedCost * 0.85
		confidenceUpper := predictedCost * 1.15

		forecast.MonthlyBreakdown = append(forecast.MonthlyBreakdown, MonthlyForecast{
			Month:           month,
			PredictedKWh:    predictedEnergy,
			PredictedCost:   predictedCost,
			ConfidenceLower: confidenceLower,
			ConfidenceUpper: confidenceUpper,
			SeasonFactor:    seasonFactor,
		})

		forecast.TotalPredictedKWh += predictedEnergy
		forecast.TotalPredictedCost += predictedCost
	}

	// 识别节省机会
	forecast.SavingsOpportunities = c.identifySavingsOpportunities(historical, forecast)
	forecast.GrowthRate = growthRate
	forecast.ConfidenceLevel = 0.85 // 固定置信度

	return forecast
}

// calculateGrowthRate 计算增长率
func (c *ElectricityCostCalculator) calculateGrowthRate(historical []ElectricityTrendPoint) float64 {
	if len(historical) < 2 {
		return 0
	}

	// 计算月增长率
	first := historical[0]
	last := historical[len(historical)-1]

	months := last.Date.Sub(first.Date).Hours() / (24 * 30)
	if months == 0 {
		return 0
	}

	if first.EnergyKWh == 0 {
		return 0
	}

	// 复合增长率
	ratio := last.EnergyKWh / first.EnergyKWh
	growthRate := math.Pow(ratio, 1/months) - 1

	return growthRate
}

// calculateSeasonalFactors 计算季节因子
func (c *ElectricityCostCalculator) calculateSeasonalFactors(historical []ElectricityTrendPoint) map[SeasonType]float64 {
	factors := make(map[SeasonType]float64)
	seasonTotals := make(map[SeasonType]float64)
	seasonCounts := make(map[SeasonType]int)

	// 计算各季节平均值
	for _, point := range historical {
		season := getSeason(point.Date)
		seasonTotals[season] += point.EnergyKWh
		seasonCounts[season]++
	}

	// 计算总体平均
	totalAvg := 0.0
	totalCount := 0
	for _, total := range seasonTotals {
		totalAvg += total
		totalCount++
	}
	if totalCount > 0 {
		totalAvg /= float64(totalCount)
	}

	// 计算季节因子
	for season, total := range seasonTotals {
		if totalAvg > 0 && seasonCounts[season] > 0 {
			avg := total / float64(seasonCounts[season])
			factors[season] = avg / totalAvg
		} else {
			factors[season] = 1.0
		}
	}

	// 确保所有季节都有值
	for _, season := range []SeasonType{SeasonSpring, SeasonSummer, SeasonAutumn, SeasonWinter} {
		if _, ok := factors[season]; !ok {
			factors[season] = 1.0
		}
	}

	return factors
}

// identifySavingsOpportunities 识别节省机会
func (c *ElectricityCostCalculator) identifySavingsOpportunities(historical []ElectricityTrendPoint, forecast *ElectricityForecast) []ElectricitySavingsOpportunity {
	opportunities := make([]ElectricitySavingsOpportunity, 0)

	// 分析峰谷用电比例
	season := getSeason(time.Now())
	peakPrice := c.tariff.GetPrice(TimeOfDayPeak, season)
	valleyPrice := c.tariff.GetPrice(TimeOfDayValley, season)

	if peakPrice > valleyPrice && peakPrice > 0 {
		// 建议负载转移
		savingsPerKWh := peakPrice - valleyPrice
		opportunities = append(opportunities, ElectricitySavingsOpportunity{
			ID:               "sav_1",
			Type:             "load_shift",
			Title:            "峰谷时段负载转移",
			Description:      "将部分负载从峰时转移到谷时运行",
			PotentialSavings: savingsPerKWh * forecast.TotalPredictedKWh * 0.1, // 假设10%可转移
			Priority:         8,
			Effort:           "medium",
			Impact:           "可将峰时用电降低10-20%",
		})
	}

	// 检查增长率
	if forecast.GrowthRate > 0.05 {
		// 增长过快，建议效率优化
		opportunities = append(opportunities, ElectricitySavingsOpportunity{
			ID:               "sav_2",
			Type:             "efficiency",
			Title:            "用电效率优化",
			Description:      "用电增长率超过5%，建议检查设备效率和闲置设备",
			PotentialSavings: forecast.TotalPredictedCost * 0.1,
			Priority:         7,
			Effort:           "low",
			Impact:           "可能节省5-15%的用电",
		})
	}

	// 建议定时调度
	opportunities = append(opportunities, ElectricitySavingsOpportunity{
		ID:               "sav_3",
		Type:             "schedule",
		Title:            "智能调度优化",
		Description:      "根据电价时段自动调整设备运行计划",
		PotentialSavings: forecast.TotalPredictedCost * 0.08,
		Priority:         6,
		Effort:           "medium",
		Impact:           "自动化调度可节省5-10%电费",
	})

	// 按优先级排序
	sort.Slice(opportunities, func(i, j int) bool {
		return opportunities[i].Priority > opportunities[j].Priority
	})

	return opportunities
}

// UpdateConfig 更新配置
func (c *ElectricityCostCalculator) UpdateConfig(config ElectricityCostConfig) {
	c.config = config
}

// UpdateTariff 更新电价配置
func (c *ElectricityCostCalculator) UpdateTariff(tariff ElectricityTariff) {
	c.tariff = tariff
}

// GetConfig 获取配置
func (c *ElectricityCostCalculator) GetConfig() ElectricityCostConfig {
	return c.config
}

// GetTariff 获取电价配置
func (c *ElectricityCostCalculator) GetTariff() ElectricityTariff {
	return c.tariff
}

// ========== 辅助函数 ==========

// getSeason 获取季节
func getSeason(t time.Time) SeasonType {
	month := t.Month()
	switch {
	case month >= 3 && month <= 5:
		return SeasonSpring
	case month >= 6 && month <= 8:
		return SeasonSummer
	case month >= 9 && month <= 11:
		return SeasonAutumn
	default:
		return SeasonWinter
	}
}

// ========== 默认电价配置 ==========

// DefaultTariffs 默认电价配置
var DefaultTariffs = map[string]ElectricityTariff{
	"beijing_general": {
		ID:           "beijing_general",
		Name:         "北京一般工商业电价",
		Region:       "北京",
		VoltageLevel: "低压",
		TimeSlots: []TimeSlot{
			{StartHour: 10, EndHour: 15, Type: TimeOfDayPeak},  // 峰时 10:00-15:00
			{StartHour: 18, EndHour: 21, Type: TimeOfDayPeak},  // 峰时 18:00-21:00
			{StartHour: 7, EndHour: 10, Type: TimeOfDayFlat},   // 平时 07:00-10:00
			{StartHour: 15, EndHour: 18, Type: TimeOfDayFlat},  // 平时 15:00-18:00
			{StartHour: 21, EndHour: 23, Type: TimeOfDayFlat},  // 平时 21:00-23:00
			{StartHour: 23, EndHour: 7, Type: TimeOfDayValley}, // 谷时 23:00-07:00
		},
		SeasonalPrices: []SeasonalPrice{
			{
				Season:      SeasonSummer,
				PeakPrice:   1.0378,
				FlatPrice:   0.6328,
				ValleyPrice: 0.2278,
				CritPrice:   1.2878,
			},
			{
				Season:      SeasonWinter,
				PeakPrice:   0.9878,
				FlatPrice:   0.5828,
				ValleyPrice: 0.1778,
				CritPrice:   1.2378,
			},
			{
				Season:      SeasonSpring,
				PeakPrice:   0.8878,
				FlatPrice:   0.5328,
				ValleyPrice: 0.1778,
				CritPrice:   0,
			},
			{
				Season:      SeasonAutumn,
				PeakPrice:   0.8878,
				FlatPrice:   0.5328,
				ValleyPrice: 0.1778,
				CritPrice:   0,
			},
		},
		EffectiveFrom: time.Now(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	},
	"shanghai_general": {
		ID:           "shanghai_general",
		Name:         "上海一般工商业电价",
		Region:       "上海",
		VoltageLevel: "低压",
		TimeSlots: []TimeSlot{
			{StartHour: 8, EndHour: 11, Type: TimeOfDayPeak},
			{StartHour: 13, EndHour: 15, Type: TimeOfDayPeak},
			{StartHour: 18, EndHour: 21, Type: TimeOfDayPeak},
			{StartHour: 6, EndHour: 8, Type: TimeOfDayFlat},
			{StartHour: 11, EndHour: 13, Type: TimeOfDayFlat},
			{StartHour: 15, EndHour: 18, Type: TimeOfDayFlat},
			{StartHour: 21, EndHour: 22, Type: TimeOfDayFlat},
			{StartHour: 22, EndHour: 6, Type: TimeOfDayValley},
		},
		SeasonalPrices: []SeasonalPrice{
			{
				Season:      SeasonSummer,
				PeakPrice:   1.1910,
				FlatPrice:   0.7220,
				ValleyPrice: 0.2530,
				CritPrice:   1.4910,
			},
			{
				Season:      SeasonWinter,
				PeakPrice:   1.1410,
				FlatPrice:   0.6720,
				ValleyPrice: 0.2030,
				CritPrice:   1.4410,
			},
			{
				Season:      SeasonSpring,
				PeakPrice:   1.0410,
				FlatPrice:   0.6220,
				ValleyPrice: 0.2030,
				CritPrice:   0,
			},
			{
				Season:      SeasonAutumn,
				PeakPrice:   1.0410,
				FlatPrice:   0.6220,
				ValleyPrice: 0.2030,
				CritPrice:   0,
			},
		},
		EffectiveFrom: time.Now(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	},
}
