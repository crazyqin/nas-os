// Package reports 提供报表生成和管理功能
package reports

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 电价配置测试 ==========

func TestElectricityTariff_GetTimeOfDay(t *testing.T) {
	tariff := ElectricityTariff{
		ID: "test_tariff",
		TimeSlots: []TimeSlot{
			{StartHour: 10, EndHour: 15, Type: TimeOfDayPeak},
			{StartHour: 23, EndHour: 7, Type: TimeOfDayValley},
			{StartHour: 7, EndHour: 10, Type: TimeOfDayFlat},
			{StartHour: 15, EndHour: 23, Type: TimeOfDayFlat},
		},
		SeasonalPrices: []SeasonalPrice{
			{Season: SeasonSummer, PeakPrice: 1.0, FlatPrice: 0.6, ValleyPrice: 0.3},
		},
	}

	tests := []struct {
		hour     int
		expected TimeOfDay
	}{
		{10, TimeOfDayPeak},
		{12, TimeOfDayPeak},
		{14, TimeOfDayPeak},
		{7, TimeOfDayFlat},
		{8, TimeOfDayFlat},
		{16, TimeOfDayFlat},
		{20, TimeOfDayFlat},
		{23, TimeOfDayValley},
		{0, TimeOfDayValley},
		{3, TimeOfDayValley},
		{6, TimeOfDayValley},
	}

	for _, tt := range tests {
		result := tariff.GetTimeOfDay(tt.hour, SeasonSummer)
		assert.Equal(t, tt.expected, result, "hour %d should be %s", tt.hour, tt.expected)
	}
}

func TestElectricityTariff_GetPrice(t *testing.T) {
	tariff := ElectricityTariff{
		ID: "test_tariff",
		SeasonalPrices: []SeasonalPrice{
			{
				Season:      SeasonSummer,
				PeakPrice:   1.2,
				FlatPrice:   0.7,
				ValleyPrice: 0.3,
				CritPrice:   1.5,
			},
			{
				Season:      SeasonWinter,
				PeakPrice:   1.1,
				FlatPrice:   0.6,
				ValleyPrice: 0.2,
				CritPrice:   1.4,
			},
		},
	}

	// 夏季电价
	assert.Equal(t, 1.2, tariff.GetPrice(TimeOfDayPeak, SeasonSummer))
	assert.Equal(t, 0.7, tariff.GetPrice(TimeOfDayFlat, SeasonSummer))
	assert.Equal(t, 0.3, tariff.GetPrice(TimeOfDayValley, SeasonSummer))
	assert.Equal(t, 1.5, tariff.GetPrice(TimeOfDayCritical, SeasonSummer))

	// 冬季电价
	assert.Equal(t, 1.1, tariff.GetPrice(TimeOfDayPeak, SeasonWinter))
	assert.Equal(t, 0.2, tariff.GetPrice(TimeOfDayValley, SeasonWinter))
}

// ========== 电费计算器测试 ==========

func TestElectricityCostCalculator_AddPowerProfile(t *testing.T) {
	config := ElectricityCostConfig{
		Region:       "北京",
		TaxRate:      0.13,
		AddOnCharges: 0.05,
	}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	profile := PowerProfile{
		ID:                "server_001",
		DeviceName:        "主服务器",
		DeviceType:        DeviceTypeServer,
		IdlePowerWatts:    100,
		MaxPowerWatts:     500,
		TypicalPowerWatts: 300,
		DailyOnHours:      24,
		WeeklyOnDays:      7,
	}

	calculator.AddPowerProfile(profile)

	retrieved, ok := calculator.GetPowerProfile("server_001")
	assert.True(t, ok)
	assert.Equal(t, "主服务器", retrieved.DeviceName)
	assert.Equal(t, DeviceTypeServer, retrieved.DeviceType)
}

func TestElectricityCostCalculator_EstimateFromProfile(t *testing.T) {
	config := ElectricityCostConfig{
		Region:       "北京",
		TaxRate:      0.13,
		AddOnCharges: 0.05,
	}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	profile := PowerProfile{
		ID:                "server_001",
		DeviceName:        "主服务器",
		DeviceType:        DeviceTypeServer,
		TypicalPowerWatts: 300, // 300W
		DailyOnHours:      24,
		WeeklyOnDays:      7,
	}

	calculator.AddPowerProfile(profile)

	// 估算30天电费
	result := calculator.EstimateFromProfile("server_001", 30)

	assert.NotNil(t, result)
	assert.Equal(t, "server_001", result.DeviceID)
	assert.Equal(t, "主服务器", result.DeviceName)

	// 每日用电量 = 300W * 24h / 1000 = 7.2 kWh
	// 30天用电量 = 7.2 * 30 = 216 kWh
	assert.InDelta(t, 216.0, result.TotalEnergyKWh, 1.0)

	// 总费用应该大于0
	assert.Greater(t, result.TotalCost, 0.0)

	// 验证平均电价在合理范围内
	assert.Greater(t, result.AveragePricePerKWh, 0.3)
	assert.Less(t, result.AveragePricePerKWh, 1.5)

	// 验证日均电费
	assert.Greater(t, result.CostPerDay, 0.0)
}

func TestElectricityCostCalculator_Calculate(t *testing.T) {
	config := ElectricityCostConfig{
		Region:       "北京",
		TaxRate:      0.13,
		AddOnCharges: 0.05,
	}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	profile := PowerProfile{
		ID:         "device_001",
		DeviceName: "测试设备",
	}
	calculator.AddPowerProfile(profile)

	// 创建模拟用电读数
	now := time.Now()
	readings := make([]PowerReading, 0)

	// 模拟24小时用电数据
	for hour := 0; hour < 24; hour++ {
		timestamp := now.Add(time.Duration(hour) * time.Hour)
		energyKWh := 1.0 // 每小时1度电

		readings = append(readings, PowerReading{
			DeviceID:   "device_001",
			DeviceName: "测试设备",
			Timestamp:  timestamp,
			EnergyKWh:  energyKWh,
			Source:     "sensor",
		})
	}

	period := ReportPeriod{
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	}

	result := calculator.Calculate("device_001", readings, period)

	assert.NotNil(t, result)
	assert.Equal(t, "device_001", result.DeviceID)
	assert.Equal(t, 24.0, result.TotalEnergyKWh)
	assert.Greater(t, result.TotalCost, 0.0)

	// 验证分时电量
	totalByTime := result.PeakEnergyKWh + result.FlatEnergyKWh + result.ValleyEnergyKWh
	assert.InDelta(t, result.TotalEnergyKWh, totalByTime, 0.1)

	// 验证小时分解
	assert.NotEmpty(t, result.HourlyBreakdown)
}

func TestElectricityCostCalculator_CalculateAll(t *testing.T) {
	config := ElectricityCostConfig{
		Region:       "北京",
		TaxRate:      0.13,
		AddOnCharges: 0.05,
	}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	now := time.Now()
	readings := make([]PowerReading, 0)

	// 设备1: 每小时2度电
	for hour := 0; hour < 24; hour++ {
		readings = append(readings, PowerReading{
			DeviceID:   "device_001",
			Timestamp:  now.Add(time.Duration(hour) * time.Hour),
			EnergyKWh:  2.0,
		})
	}

	// 设备2: 每小时1度电
	for hour := 0; hour < 24; hour++ {
		readings = append(readings, PowerReading{
			DeviceID:   "device_002",
			Timestamp:  now.Add(time.Duration(hour) * time.Hour),
			EnergyKWh:  1.0,
		})
	}

	period := ReportPeriod{
		StartTime: now,
		EndTime:   now.Add(24 * time.Hour),
	}

	results := calculator.CalculateAll(readings, period)

	assert.Len(t, results, 2)
	assert.Equal(t, 48.0, results["device_001"].TotalEnergyKWh)
	assert.Equal(t, 24.0, results["device_002"].TotalEnergyKWh)

	// 设备1应该比设备2费用高
	assert.Greater(t, results["device_001"].TotalCost, results["device_002"].TotalCost)
}

func TestElectricityCostCalculator_GenerateBill(t *testing.T) {
	config := ElectricityCostConfig{
		Region:       "北京",
		TaxRate:      0.13,
		AddOnCharges: 0.05,
	}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	// 添加设备配置
	calculator.AddPowerProfile(PowerProfile{
		ID:         "device_001",
		DeviceName: "服务器A",
		DeviceType: DeviceTypeServer,
	})
	calculator.AddPowerProfile(PowerProfile{
		ID:         "device_002",
		DeviceName: "存储设备B",
		DeviceType: DeviceTypeStorage,
	})

	// 创建计算结果
	now := time.Now()
	results := map[string]*ElectricityCostResult{
		"device_001": {
			DeviceID:       "device_001",
			DeviceName:     "服务器A",
			TotalEnergyKWh: 100.0,
			TotalCost:      80.0,
			EnergyCost:     70.0,
		},
		"device_002": {
			DeviceID:       "device_002",
			DeviceName:     "存储设备B",
			TotalEnergyKWh: 50.0,
			TotalCost:      40.0,
			EnergyCost:     35.0,
		},
	}

	period := ReportPeriod{
		StartTime: now.AddDate(0, -1, 0),
		EndTime:   now,
	}

	bill := calculator.GenerateBill(results, period, "测试公司")

	assert.NotNil(t, bill)
	assert.NotEmpty(t, bill.ID)
	assert.NotEmpty(t, bill.BillNumber)
	assert.Equal(t, "测试公司", bill.CustomerName)
	assert.Equal(t, "pending", bill.Status)

	// 验证总电量
	assert.Equal(t, 150.0, bill.EnergyConsumption)

	// 验证设备分解
	assert.Len(t, bill.DeviceBreakdown, 2)

	// 验证费用
	assert.Greater(t, bill.TotalAmount, 0.0)
}

func TestElectricityCostCalculator_Forecast(t *testing.T) {
	config := ElectricityCostConfig{
		Region:       "北京",
		TaxRate:      0.13,
		AddOnCharges: 0.05,
	}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	// 创建历史数据
	now := time.Now()
	historical := make([]ElectricityTrendPoint, 0)

	for i := 11; i >= 0; i-- {
		date := now.AddDate(0, -i, 0)
		// 模拟每月用电量递增
		energy := 200.0 + float64(12-i)*10.0
		cost := energy * 0.7 // 假设平均电价0.7元
		historical = append(historical, ElectricityTrendPoint{
			Date:      date,
			Cost:      cost,
			EnergyKWh: energy,
		})
	}

	// 预测未来3个月
	forecast := calculator.Forecast(historical, 3)

	assert.NotNil(t, forecast)
	assert.Len(t, forecast.MonthlyBreakdown, 3)
	assert.Greater(t, forecast.TotalPredictedKWh, 0.0)
	assert.Greater(t, forecast.TotalPredictedCost, 0.0)

	// 验证置信区间
	for _, mb := range forecast.MonthlyBreakdown {
		assert.GreaterOrEqual(t, mb.ConfidenceUpper, mb.PredictedCost)
		assert.LessOrEqual(t, mb.ConfidenceLower, mb.PredictedCost)
	}

	// 验证节省机会
	assert.NotEmpty(t, forecast.SavingsOpportunities)
}

// ========== 季节测试 ==========

func TestGetSeason(t *testing.T) {
	tests := []struct {
		month    time.Month
		expected SeasonType
	}{
		{time.January, SeasonWinter},
		{time.February, SeasonWinter},
		{time.March, SeasonSpring},
		{time.April, SeasonSpring},
		{time.May, SeasonSpring},
		{time.June, SeasonSummer},
		{time.July, SeasonSummer},
		{time.August, SeasonSummer},
		{time.September, SeasonAutumn},
		{time.October, SeasonAutumn},
		{time.November, SeasonAutumn},
		{time.December, SeasonWinter},
	}

	for _, tt := range tests {
		date := time.Date(2024, tt.month, 15, 0, 0, 0, 0, time.UTC)
		result := getSeason(date)
		assert.Equal(t, tt.expected, result, "month %d should be %s", tt.month, tt.expected)
	}
}

// ========== 默认电价配置测试 ==========

func TestDefaultTariffs(t *testing.T) {
	// 验证北京电价配置
	beijing, ok := DefaultTariffs["beijing_general"]
	assert.True(t, ok)
	assert.Equal(t, "北京", beijing.Region)
	assert.NotEmpty(t, beijing.TimeSlots)
	assert.NotEmpty(t, beijing.SeasonalPrices)

	// 验证上海电价配置
	shanghai, ok := DefaultTariffs["shanghai_general"]
	assert.True(t, ok)
	assert.Equal(t, "上海", shanghai.Region)
	assert.NotEmpty(t, shanghai.TimeSlots)
	assert.NotEmpty(t, shanghai.SeasonalPrices)

	// 验证电价合理性
	for _, sp := range beijing.SeasonalPrices {
		assert.Greater(t, sp.PeakPrice, sp.FlatPrice)
		assert.Greater(t, sp.FlatPrice, sp.ValleyPrice)
	}
}

// ========== 边界条件测试 ==========

func TestElectricityCostCalculator_EmptyReadings(t *testing.T) {
	config := ElectricityCostConfig{
		Region:  "北京",
		TaxRate: 0.13,
	}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	period := ReportPeriod{
		StartTime: time.Now(),
		EndTime:   time.Now().Add(24 * time.Hour),
	}

	result := calculator.Calculate("non_existent", []PowerReading{}, period)

	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalEnergyKWh)
	assert.Equal(t, 0.0, result.TotalCost)
}

func TestElectricityCostCalculator_NonExistentProfile(t *testing.T) {
	config := ElectricityCostConfig{}
	tariff := ElectricityTariff{}
	calculator := NewElectricityCostCalculator(config, tariff)

	result := calculator.EstimateFromProfile("non_existent", 30)
	assert.Nil(t, result)
}

func TestElectricityCostCalculator_ShortHistory(t *testing.T) {
	config := ElectricityCostConfig{}
	tariff := ElectricityTariff{}
	calculator := NewElectricityCostCalculator(config, tariff)

	// 只有一个数据点
	historical := []ElectricityTrendPoint{
		{Date: time.Now(), Cost: 100, EnergyKWh: 100},
	}

	forecast := calculator.Forecast(historical, 3)
	assert.Nil(t, forecast)
}

// ========== 配置更新测试 ==========

func TestElectricityCostCalculator_UpdateConfig(t *testing.T) {
	config := ElectricityCostConfig{
		Region:  "北京",
		TaxRate: 0.13,
	}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	newConfig := ElectricityCostConfig{
		Region:       "上海",
		TaxRate:      0.10,
		AddOnCharges: 0.08,
	}
	calculator.UpdateConfig(newConfig)

	retrieved := calculator.GetConfig()
	assert.Equal(t, "上海", retrieved.Region)
	assert.Equal(t, 0.10, retrieved.TaxRate)
	assert.Equal(t, 0.08, retrieved.AddOnCharges)
}

func TestElectricityCostCalculator_UpdateTariff(t *testing.T) {
	config := ElectricityCostConfig{}
	tariff := ElectricityTariff{ID: "old_tariff"}
	calculator := NewElectricityCostCalculator(config, tariff)

	newTariff := ElectricityTariff{ID: "new_tariff"}
	calculator.UpdateTariff(newTariff)

	retrieved := calculator.GetTariff()
	assert.Equal(t, "new_tariff", retrieved.ID)
}

// ========== 节省机会测试 ==========

func TestSavingsOpportunity_Prioritization(t *testing.T) {
	config := ElectricityCostConfig{}
	tariff := DefaultTariffs["beijing_general"]
	calculator := NewElectricityCostCalculator(config, tariff)

	// 创建有增长的历史数据
	now := time.Now()
	historical := make([]ElectricityTrendPoint, 0)
	for i := 11; i >= 0; i-- {
		date := now.AddDate(0, -i, 0)
		// 每月增长5%
		energy := 200.0 * math.Pow(1.05, float64(12-i))
		cost := energy * 0.7
		historical = append(historical, ElectricityTrendPoint{
			Date:      date,
			Cost:      cost,
			EnergyKWh: energy,
		})
	}

	forecast := calculator.Forecast(historical, 3)

	// 应该有节省机会
	assert.NotEmpty(t, forecast.SavingsOpportunities)

	// 验证优先级排序
	for i := 1; i < len(forecast.SavingsOpportunities); i++ {
		assert.GreaterOrEqual(t,
			forecast.SavingsOpportunities[i-1].Priority,
			forecast.SavingsOpportunities[i].Priority,
		)
	}
}