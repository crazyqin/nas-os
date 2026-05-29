// Package energytracker 提供碳排放追踪与能源成本分析核心业务逻辑
package energytracker

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// EnergyManager 能耗管理器.
type EnergyManager struct {
	readings []*EnergyReading
	config   PowerConfig
	mu       sync.RWMutex
}

// NewManager 创建能耗管理器.
func NewManager() *EnergyManager {
	return &EnergyManager{
		readings: make([]*EnergyReading, 0),
		config: PowerConfig{
			CarbonFactor:     0.5703, // 中国电网平均碳排放因子
			PricePerKWhCents: 56,    // 居民电价约0.56元/度
			SamplingInterval: 60,    // 60秒采样一次
			IdleThreshold:    10,    // 10瓦为空闲阈值
		},
	}
}

// ========== 核心方法 ==========

// TrackUsage 记录能耗数据.
func (m *EnergyManager) TrackUsage(req TrackRequest) (*EnergyReading, error) {
	if req.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}
	if req.DeviceName == "" {
		return nil, fmt.Errorf("device_name is required")
	}
	if req.PowerWatts < 0 {
		return nil, fmt.Errorf("power_watts must be non-negative")
	}

	reading := &EnergyReading{
		ID:         uuid.New().String(),
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		PowerWatts: req.PowerWatts,
		Timestamp:  time.Now(),
		Service:    req.Service,
	}

	m.mu.Lock()
	m.readings = append(m.readings, reading)
	m.mu.Unlock()

	return reading, nil
}

// CalculateCarbon 计算碳排放.
func (m *EnergyManager) CalculateCarbon(deviceID string, start, end time.Time) (*CarbonFootprint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.readings) == 0 {
		return nil, fmt.Errorf("no readings available")
	}

	// 筛选指定设备和时间范围的读数
	var readings []*EnergyReading
	for _, r := range m.readings {
		if r.DeviceID == deviceID && !r.Timestamp.Before(start) && !r.Timestamp.After(end) {
			readings = append(readings, r)
		}
	}

	if len(readings) == 0 {
		return nil, fmt.Errorf("no readings found for device %s in specified time range", deviceID)
	}

	// 计算能耗：功率 * 时间
	var totalEnergyKWh float64
	deviceName := readings[0].DeviceName

	for i := 1; i < len(readings); i++ {
		duration := readings[i].Timestamp.Sub(readings[i-1].Timestamp).Hours()
		if duration <= 0 {
			continue
		}
		avgPower := (readings[i].PowerWatts + readings[i-1].PowerWatts) / 2
		energyKWh := avgPower * duration / 1000
		totalEnergyKWh += energyKWh
	}

	carbonKg := totalEnergyKWh * m.config.CarbonFactor

	return &CarbonFootprint{
		DeviceID:     deviceID,
		DeviceName:   deviceName,
		EnergyKWh:    math.Round(totalEnergyKWh*1000) / 1000,
		CarbonKg:     math.Round(carbonKg*1000) / 1000,
		CarbonFactor: m.config.CarbonFactor,
		PeriodStart:  start,
		PeriodEnd:    end,
	}, nil
}

// GenerateReport 生成能源报告.
func (m *EnergyManager) GenerateReport(req ReportRequest) (*EnergyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.readings) == 0 {
		return nil, fmt.Errorf("no readings available")
	}

	// 确定时间范围
	end := time.Now()
	var start time.Time

	if req.StartTime != nil && req.EndTime != nil {
		start = *req.StartTime
		end = *req.EndTime
	} else {
		switch req.Period {
		case PeriodDaily:
			start = end.Add(-24 * time.Hour)
		case PeriodWeekly:
			start = end.Add(-7 * 24 * time.Hour)
		case PeriodMonthly:
			start = end.Add(-30 * 24 * time.Hour)
		default:
			return nil, fmt.Errorf("invalid period: %s", req.Period)
		}
	}

	// 筛选时间范围内的读数并按设备分组
	deviceMap := make(map[string]*DeviceEnergy)
	deviceReadings := make(map[string][]*EnergyReading)

	for _, r := range m.readings {
		if !r.Timestamp.Before(start) && !r.Timestamp.After(end) {
			if _, ok := deviceMap[r.DeviceID]; !ok {
				deviceMap[r.DeviceID] = &DeviceEnergy{
					DeviceID:   r.DeviceID,
					DeviceName: r.DeviceName,
				}
			}
			deviceReadings[r.DeviceID] = append(deviceReadings[r.DeviceID], r)
		}
	}

	if len(deviceReadings) == 0 {
		return nil, fmt.Errorf("no readings found in specified time range")
	}

	// 对每个设备的读数按时间排序
	for _, devR := range deviceReadings {
		sort.Slice(devR, func(i, j int) bool {
			return devR[i].Timestamp.Before(devR[j].Timestamp)
		})
	}

	// 计算每个设备的能耗
	var totalEnergyKWh float64
	for deviceID, dev := range deviceMap {
		devR := deviceReadings[deviceID]
		var energyKWh float64
		var totalPower float64

		for i := 1; i < len(devR); i++ {
			duration := devR[i].Timestamp.Sub(devR[i-1].Timestamp).Hours()
			if duration <= 0 {
				continue
			}
			avgPower := (devR[i].PowerWatts + devR[i-1].PowerWatts) / 2
			energyKWh += avgPower * duration / 1000
			totalPower += devR[i].PowerWatts
		}

		dev.EnergyKWh = math.Round(energyKWh*1000) / 1000
		dev.CarbonKg = math.Round(energyKWh*m.config.CarbonFactor*1000) / 1000
		dev.CostCents = int64(math.Round(energyKWh * float64(m.config.PricePerKWhCents)))

		if len(devR) > 0 {
			dev.AvgPowerWatts = math.Round(totalPower/float64(len(devR))*100) / 100
		}

		totalEnergyKWh += energyKWh
	}

	// 计算百分比
	for _, dev := range deviceMap {
		if totalEnergyKWh > 0 {
			dev.Percentage = math.Round(dev.EnergyKWh/totalEnergyKWh*10000) / 100
		}
	}

	// 按设备能耗排序
	deviceBreakdown := make([]DeviceEnergy, 0, len(deviceMap))
	for _, dev := range deviceMap {
		deviceBreakdown = append(deviceBreakdown, *dev)
	}
	sort.Slice(deviceBreakdown, func(i, j int) bool {
		return deviceBreakdown[i].EnergyKWh > deviceBreakdown[j].EnergyKWh
	})

	// 收集所有读数用于服务分析和趋势
	var allReadings []*EnergyReading
	for _, devR := range deviceReadings {
		allReadings = append(allReadings, devR...)
	}

	// 按服务分组
	serviceMap := make(map[string]*ServiceEnergy)
	for _, r := range allReadings {
		if r.Service == "" {
			continue
		}
		if _, ok := serviceMap[r.Service]; !ok {
			serviceMap[r.Service] = &ServiceEnergy{
				ServiceName: r.Service,
			}
		}
	}

	// 计算服务能耗（简化：均分到设备）
	serviceBreakdown := make([]ServiceEnergy, 0, len(serviceMap))
	for _, svc := range serviceMap {
		svc.EnergyKWh = totalEnergyKWh / float64(len(serviceMap))
		svc.CarbonKg = math.Round(svc.EnergyKWh*m.config.CarbonFactor*1000) / 1000
		svc.CostCents = int64(math.Round(svc.EnergyKWh * float64(m.config.PricePerKWhCents)))
		if totalEnergyKWh > 0 {
			svc.Percentage = math.Round(svc.EnergyKWh/totalEnergyKWh*10000) / 100
		}
		serviceBreakdown = append(serviceBreakdown, *svc)
	}

	// 生成小时趋势
	hourlyTrend := m.generateHourlyTrend(allReadings, start, end)

	// 生成优化建议
	tips := m.generateOptimizationTips(deviceBreakdown, totalEnergyKWh)

	totalCarbonKg := math.Round(totalEnergyKWh*m.config.CarbonFactor*1000) / 1000
	totalCostCents := int64(math.Round(totalEnergyKWh * float64(m.config.PricePerKWhCents)))

	return &EnergyReport{
		ID:               uuid.New().String(),
		Period:           req.Period,
		StartTime:        start,
		EndTime:          end,
		TotalEnergyKWh:   math.Round(totalEnergyKWh*1000) / 1000,
		TotalCarbonKg:    totalCarbonKg,
		TotalCostCents:   totalCostCents,
		DeviceBreakdown:  deviceBreakdown,
		ServiceBreakdown: serviceBreakdown,
		HourlyTrend:      hourlyTrend,
		OptimizationTips: tips,
		GeneratedAt:      time.Now(),
	}, nil
}

// SuggestOptimization 生成节能优化建议.
func (m *EnergyManager) SuggestOptimization(deviceID string) ([]OptimizationTip, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.readings) == 0 {
		return nil, fmt.Errorf("no readings available")
	}

	// 收集设备读数
	var readings []*EnergyReading
	for _, r := range m.readings {
		if deviceID == "" || r.DeviceID == deviceID {
			readings = append(readings, r)
		}
	}

	if len(readings) == 0 {
		return nil, fmt.Errorf("no readings found for device %s", deviceID)
	}

	// 分析功耗模式
	var totalPower, maxPower, minPower float64
	idleCount := 0

	for _, r := range readings {
		totalPower += r.PowerWatts
		if r.PowerWatts > maxPower {
			maxPower = r.PowerWatts
		}
		if r.PowerWatts < minPower || minPower == 0 {
			minPower = r.PowerWatts
		}
		if r.PowerWatts < m.config.IdleThreshold {
			idleCount++
		}
	}

	avgPower := totalPower / float64(len(readings))

	tips := make([]OptimizationTip, 0)

	// 空闲功耗过高
	if idleCount > len(readings)/2 {
		idleSavings := avgPower * 0.3 * 24 / 1000 // 假设30%可节省
		tips = append(tips, OptimizationTip{
			Category:    "power_management",
			Title:       "启用硬盘休眠",
			Description: "检测到设备长时间处于低功耗状态，建议启用硬盘休眠功能",
			SavingsKWh:  math.Round(idleSavings*100) / 100,
			SavingsCents: int64(math.Round(idleSavings * float64(m.config.PricePerKWhCents))),
			Priority:    "high",
		})
	}

	// 峰值功耗过高
	if maxPower > 100 {
		tips = append(tips, OptimizationTip{
			Category:    "power_cap",
			Title:       "设置功耗上限",
			Description: "检测到设备峰值功耗较高，建议设置功耗上限以降低能耗",
			SavingsKWh:  math.Round((maxPower-80)*24/1000*100) / 100,
			SavingsCents: int64(math.Round((maxPower-80) * 24 / 1000 * float64(m.config.PricePerKWhCents))),
			Priority:    "medium",
		})
	}

	// CPU 调度优化
	if avgPower > 50 {
		tips = append(tips, OptimizationTip{
			Category:    "scheduling",
			Title:       "优化任务调度",
			Description: "将高负载任务调度到电价低谷时段执行",
			SavingsKWh:  math.Round(avgPower*0.2*8/1000*100) / 100,
			SavingsCents: int64(math.Round(avgPower * 0.2 * 8 / 1000 * float64(m.config.PricePerKWhCents))),
			Priority:    "medium",
		})
	}

	// 温控优化
	if maxPower-minPower > 30 {
		tips = append(tips, OptimizationTip{
			Category:    "thermal",
			Title:       "优化散热方案",
			Description: "功耗波动较大，优化散热可降低风扇能耗",
			SavingsKWh:  2.5,
			SavingsCents: int64(math.Round(2.5 * float64(m.config.PricePerKWhCents))),
			Priority:    "low",
		})
	}

	// 定时开关机
	tips = append(tips, OptimizationTip{
		Category:    "schedule",
		Title:       "设置定时开关机",
		Description: "在非使用时段自动关机，可节省约30%能耗",
		SavingsKWh:  math.Round(avgPower*0.3*8/1000*100) / 100,
		SavingsCents: int64(math.Round(avgPower * 0.3 * 8 / 1000 * float64(m.config.PricePerKWhCents))),
		Priority:    "high",
	})

	return tips, nil
}

// ========== 辅助方法 ==========

// GetReadings 获取能耗读数.
func (m *EnergyManager) GetReadings(deviceID string, limit int) []*EnergyReading {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*EnergyReading
	for i := len(m.readings) - 1; i >= 0; i-- {
		if deviceID == "" || m.readings[i].DeviceID == deviceID {
			cp := *m.readings[i]
			result = append(result, &cp)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result
}

// GetConfig 获取配置.
func (m *EnergyManager) GetConfig() PowerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *EnergyManager) UpdateConfig(config PowerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if config.CarbonFactor > 0 {
		m.config.CarbonFactor = config.CarbonFactor
	}
	if config.PricePerKWhCents > 0 {
		m.config.PricePerKWhCents = config.PricePerKWhCents
	}
	if config.SamplingInterval > 0 {
		m.config.SamplingInterval = config.SamplingInterval
	}
	if config.IdleThreshold > 0 {
		m.config.IdleThreshold = config.IdleThreshold
	}
}

// ClearReadings 清除读数.
func (m *EnergyManager) ClearReadings() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readings = make([]*EnergyReading, 0)
}

// generateHourlyTrend 生成小时趋势.
func (m *EnergyManager) generateHourlyTrend(readings []*EnergyReading, start, end time.Time) []HourlyEnergy {
	hourlyData := make(map[int]*HourlyEnergy)
	for h := 0; h < 24; h++ {
		hourlyData[h] = &HourlyEnergy{Hour: h}
	}

	hourlyCounts := make(map[int]int)
	for _, r := range readings {
		hour := r.Timestamp.Hour()
		hourlyData[hour].AvgPower += r.PowerWatts
		hourlyCounts[hour]++
	}

	// 计算平均功率
	for h, data := range hourlyData {
		if count := hourlyCounts[h]; count > 0 {
			data.AvgPower = math.Round(data.AvgPower/float64(count)*100) / 100
			data.EnergyKWh = math.Round(data.AvgPower/1000*1000) / 1000
		}
	}

	result := make([]HourlyEnergy, 24)
	for h := 0; h < 24; h++ {
		result[h] = *hourlyData[h]
	}

	return result
}

// generateOptimizationTips 生成优化建议.
func (m *EnergyManager) generateOptimizationTips(devices []DeviceEnergy, totalEnergyKWh float64) []OptimizationTip {
	tips := make([]OptimizationTip, 0)

	// 找出高能耗设备
	for _, dev := range devices {
		if dev.Percentage > 30 {
			tips = append(tips, OptimizationTip{
				Category:    "high_consumption",
				Title:       fmt.Sprintf("优化 %s 能耗", dev.DeviceName),
				Description: fmt.Sprintf("%s 占总能耗 %.1f%%，建议检查是否有异常", dev.DeviceName, dev.Percentage),
				SavingsKWh:  math.Round(dev.EnergyKWh*0.1*100) / 100,
				SavingsCents: int64(math.Round(dev.EnergyKWh * 0.1 * float64(m.config.PricePerKWhCents))),
				Priority:    "high",
			})
		}
	}

	// 总体建议
	if totalEnergyKWh > 100 {
		tips = append(tips, OptimizationTip{
			Category:    "overall",
			Title:       "考虑升级节能设备",
			Description: "当前能耗较高，升级到低功耗设备可长期节省成本",
			SavingsKWh:  math.Round(totalEnergyKWh*0.2*100) / 100,
			SavingsCents: int64(math.Round(totalEnergyKWh * 0.2 * float64(m.config.PricePerKWhCents))),
			Priority:    "medium",
		})
	}

	return tips
}
