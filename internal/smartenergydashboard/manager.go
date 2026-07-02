// Package smartenergydashboard 提供智能能源仪表盘核心管理逻辑
package smartenergydashboard

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Manager 能源仪表盘管理器.
type Manager struct {
	mu            sync.RWMutex
	settings      *EnergySettings
	budget        *EnergyBudget
	readings      []*PowerReading
	records       []*EnergyRecord
	devices       map[string]*DevicePower
	lastReadingID int64
}

// NewManager 创建能源仪表盘管理器.
func NewManager() *Manager {
	m := &Manager{
		settings: DefaultEnergySettings(),
		readings: make([]*PowerReading, 0),
		records:  make([]*EnergyRecord, 0),
		devices:  make(map[string]*DevicePower),
	}

	// 初始化模拟设备
	m.initMockDevices()
	// 初始化模拟历史数据
	m.initMockHistory()

	return m
}

// initMockDevices 初始化模拟设备.
func (m *Manager) initMockDevices() {
	mockDevices := []*DevicePower{
		{
			DeviceID:    "cpu-001",
			DeviceName:  "CPU (ARM RK3588)",
			DeviceType:  "cpu",
			CurrentWatt: 8.5,
			DailyKWh:    0.204,
			MonthlyKWh:  6.12,
			Status:      "online",
		},
		{
			DeviceID:    "psu-001",
			DeviceName:  "电源供应器",
			DeviceType:  "psu",
			CurrentWatt: 45.0,
			DailyKWh:    1.08,
			MonthlyKWh:  32.4,
			Status:      "online",
		},
		{
			DeviceID:    "fan-001",
			DeviceName:  "散热风扇",
			DeviceType:  "fan",
			CurrentWatt: 2.3,
			DailyKWh:    0.055,
			MonthlyKWh:  1.65,
			Status:      "online",
		},
		{
			DeviceID:    "nvme-001",
			DeviceName:  "NVMe SSD (系统盘)",
			DeviceType:  "ssd",
			CurrentWatt: 3.5,
			DailyKWh:    0.084,
			MonthlyKWh:  2.52,
			Status:      "online",
		},
		{
			DeviceID:    "hdd-001",
			DeviceName:  "HDD 8TB (数据盘)",
			DeviceType:  "hdd",
			CurrentWatt: 6.8,
			DailyKWh:    0.163,
			MonthlyKWh:  4.89,
			Status:      "online",
		},
		{
			DeviceID:    "hdd-002",
			DeviceName:  "HDD 8TB (备份盘)",
			DeviceType:  "hdd",
			CurrentWatt: 6.8,
			DailyKWh:    0.163,
			MonthlyKWh:  4.89,
			Status:      "standby",
		},
		{
			DeviceID:    "nic-001",
			DeviceName:  "2.5G 网卡",
			DeviceType:  "nic",
			CurrentWatt: 1.8,
			DailyKWh:    0.043,
			MonthlyKWh:  1.29,
			Status:      "online",
		},
	}

	for _, d := range mockDevices {
		m.devices[d.DeviceID] = d
	}
}

// initMockHistory 初始化模拟历史数据.
func (m *Manager) initMockHistory() {
	now := time.Now()

	// 生成过去 30 天的每日记录
	for i := 30; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		// 模拟能耗波动
		baseKWh := 1.8 + float64(i%7)*0.1
		if i%3 == 0 {
			baseKWh += 0.3 // 偶尔的高负载
		}

		record := &EnergyRecord{
			ID:           fmt.Sprintf("record-%d", 30-i),
			Date:         date,
			KWh:          baseKWh,
			Cost:         baseKWh * m.settings.ElectricityRate,
			CarbonKg:     baseKWh * m.settings.CarbonFactor,
			PeakWattage:  85.0 + float64(i%5)*5.0,
			AvgWattage:   45.0 + float64(i%3)*3.0,
			RuntimeHours: 24.0,
		}
		m.records = append(m.records, record)
	}
}

// generateID 生成唯一 ID.
func (m *Manager) generateID() string {
	m.lastReadingID++
	return fmt.Sprintf("pr-%d-%d", time.Now().UnixNano(), m.lastReadingID)
}

// RecordPowerReading 记录功耗读数.
func (m *Manager) RecordPowerReading(source string, wattage, voltage, current float64) *PowerReading {
	m.mu.Lock()
	defer m.mu.Unlock()

	reading := &PowerReading{
		ID:        m.generateID(),
		Timestamp: time.Now(),
		Wattage:   wattage,
		Voltage:   voltage,
		Current:   current,
		Source:    source,
	}

	m.readings = append(m.readings, reading)

	// 限制历史记录数量
	if len(m.readings) > 10000 {
		m.readings = m.readings[len(m.readings)-10000:]
	}

	return reading
}

// GetCurrentPower 获取当前功耗.
func (m *Manager) GetCurrentPower() *PowerReading {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 汇总所有在线设备的功耗
	totalWattage := 0.0
	for _, d := range m.devices {
		if d.Status == "online" {
			totalWattage += d.CurrentWatt
		}
	}

	return &PowerReading{
		ID:        "current",
		Timestamp: time.Now(),
		Wattage:   totalWattage,
		Voltage:   12.0,
		Current:   totalWattage / 12.0,
		Source:    "system",
	}
}

// GetHistory 获取历史记录.
func (m *Manager) GetHistory(period string) []*EnergyRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var startDate time.Time

	switch period {
	case "daily":
		startDate = now.AddDate(0, 0, -7)
	case "weekly":
		startDate = now.AddDate(0, 0, -30)
	case "monthly":
		startDate = now.AddDate(-1, 0, 0)
	default:
		startDate = now.AddDate(0, 0, -30)
	}

	result := make([]*EnergyRecord, 0)
	for _, r := range m.records {
		if r.Date.After(startDate) || r.Date.Equal(startDate) {
			result = append(result, r)
		}
	}

	return result
}

// GetDevicePower 获取各设备功耗.
func (m *Manager) GetDevicePower() []*DevicePower {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*DevicePower, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, d)
	}

	return devices
}

// SetBudget 设置预算.
func (m *Manager) SetBudget(monthlyLimitKWh, monthlyLimitCost, alertThreshold float64) *EnergyBudget {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 计算当月已用
	currentUsage := m.calculateCurrentMonthUsage()
	projected := m.projectMonthUsage(currentUsage)

	if m.budget == nil {
		m.budget = &EnergyBudget{
			ID:        "budget-main",
			CreatedAt: time.Now(),
		}
	}

	m.budget.MonthlyLimitKWh = monthlyLimitKWh
	m.budget.MonthlyLimitCost = monthlyLimitCost
	m.budget.AlertThreshold = alertThreshold
	m.budget.CurrentUsage = currentUsage
	m.budget.ProjectedUsage = projected
	m.budget.UpdatedAt = time.Now()

	return m.budget
}

// GetBudget 获取当前预算.
func (m *Manager) GetBudget() *EnergyBudget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.budget == nil {
		return nil
	}

	// 更新当前用量
	currentUsage := m.calculateCurrentMonthUsage()
	projected := m.projectMonthUsage(currentUsage)

	budget := *m.budget
	budget.CurrentUsage = currentUsage
	budget.ProjectedUsage = projected

	return &budget
}

// calculateCurrentMonthUsage 计算当月已用.
func (m *Manager) calculateCurrentMonthUsage() float64 {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	total := 0.0
	for _, r := range m.records {
		if r.Date.After(startOfMonth) || r.Date.Equal(startOfMonth) {
			total += r.KWh
		}
	}
	return total
}

// projectMonthUsage 预测当月总用量.
func (m *Manager) projectMonthUsage(currentUsage float64) float64 {
	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	daysElapsed := now.Sub(startOfMonth).Hours() / 24.0
	daysInMonth := float64(time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day())

	if daysElapsed <= 0 {
		return 0
	}

	return currentUsage / daysElapsed * daysInMonth
}

// GenerateReport 生成能源报告.
func (m *Manager) GenerateReport(period string) *EnergyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	var startDate time.Time

	switch period {
	case "daily":
		startDate = now.AddDate(0, 0, -1)
	case "weekly":
		startDate = now.AddDate(0, 0, -7)
	case "monthly":
		startDate = now.AddDate(0, -1, 0)
	default:
		startDate = now.AddDate(0, -1, 0)
	}

	totalKWh := 0.0
	totalCost := 0.0
	totalCarbon := 0.0
	peakWattage := 0.0
	avgWattageSum := 0.0
	count := 0

	for _, r := range m.records {
		if r.Date.After(startDate) {
			totalKWh += r.KWh
			totalCost += r.Cost
			totalCarbon += r.CarbonKg
			if r.PeakWattage > peakWattage {
				peakWattage = r.PeakWattage
			}
			avgWattageSum += r.AvgWattage
			count++
		}
	}

	// 获取 Top 设备
	topDevices := m.getTopDevices(5)

	// 计算趋势
	trend := m.calculateTrend(period)

	// 生成节能建议
	tips := m.generateSavingsTips()

	return &EnergyReport{
		ID:          fmt.Sprintf("report-%s-%d", period, now.Unix()),
		Period:      period,
		StartDate:   startDate,
		EndDate:     now,
		TotalKWh:    totalKWh,
		TotalCost:   totalCost,
		CarbonKg:    totalCarbon,
		TopDevices:  topDevices,
		Trend:       trend,
		SavingsTips: tips,
	}
}

// getTopDevices 获取功耗最高的设备.
func (m *Manager) getTopDevices(limit int) []DevicePower {
	// 按月耗电量排序
	sorted := make([]*DevicePower, 0, len(m.devices))
	for _, d := range m.devices {
		sorted = append(sorted, d)
	}

	// 简单冒泡排序
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].MonthlyKWh > sorted[i].MonthlyKWh {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	result := make([]DevicePower, 0, limit)
	for i := 0; i < limit && i < len(sorted); i++ {
		result = append(result, *sorted[i])
	}

	return result
}

// calculateTrend 计算能耗趋势.
func (m *Manager) calculateTrend(period string) string {
	if len(m.records) < 2 {
		return "stable"
	}

	// 比较最近两个周期的能耗
	var currentStart, prevStart time.Time
	now := time.Now()

	switch period {
	case "daily":
		currentStart = now.AddDate(0, 0, -1)
		prevStart = now.AddDate(0, 0, -2)
	case "weekly":
		currentStart = now.AddDate(0, 0, -7)
		prevStart = now.AddDate(0, 0, -14)
	case "monthly":
		currentStart = now.AddDate(0, -1, 0)
		prevStart = now.AddDate(0, -2, 0)
	default:
		currentStart = now.AddDate(0, -1, 0)
		prevStart = now.AddDate(0, -2, 0)
	}

	currentTotal := 0.0
	prevTotal := 0.0

	for _, r := range m.records {
		if r.Date.After(currentStart) {
			currentTotal += r.KWh
		} else if r.Date.After(prevStart) {
			prevTotal += r.KWh
		}
	}

	diff := currentTotal - prevTotal
	threshold := prevTotal * 0.05 // 5% 阈值

	if diff > threshold {
		return "up"
	} else if diff < -threshold {
		return "down"
	}
	return "stable"
}

// ForecastCost 成本预测.
func (m *Manager) ForecastCost() []*CostForecast {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	forecasts := make([]*CostForecast, 0, 3)

	// 基于过去 3 个月的趋势预测未来 3 个月
	recentRecords := make([]*EnergyRecord, 0)
	for _, r := range m.records {
		if r.Date.After(now.AddDate(0, -3, 0)) {
			recentRecords = append(recentRecords, r)
		}
	}

	if len(recentRecords) == 0 {
		return forecasts
	}

	// 计算日均
	totalKWh := 0.0
	for _, r := range recentRecords {
		totalKWh += r.KWh
	}
	avgDaily := totalKWh / float64(len(recentRecords))

	for i := 1; i <= 3; i++ {
		month := now.AddDate(0, i, 0)
		monthName := month.Format("2006-01")
		daysInMonth := float64(time.Date(month.Year(), month.Month()+1, 0, 0, 0, 0, 0, month.Location()).Day())

		projectedKWh := avgDaily * daysInMonth
		// 添加季节性波动
		seasonalFactor := 1.0 + float64(i)*0.02
		projectedKWh *= seasonalFactor

		factors := []string{"历史平均", "季节性趋势"}
		if i == 1 {
			factors = append(factors, "当前负载水平")
		}

		forecasts = append(forecasts, &CostForecast{
			Month:         monthName,
			ProjectedKWh:  math.Round(projectedKWh*100) / 100,
			ProjectedCost: math.Round(projectedKWh*m.settings.ElectricityRate*100) / 100,
			Confidence:    math.Max(50, 90-float64(i)*10),
			Factors:       factors,
		})
	}

	return forecasts
}

// GetTips 获取节能建议.
func (m *Manager) GetTips() []EnergyTip {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tips := make([]EnergyTip, 0)

	// 检查待机设备
	standbyCount := 0
	for _, d := range m.devices {
		if d.Status == "standby" {
			standbyCount++
		}
	}
	if standbyCount > 0 {
		tips = append(tips, EnergyTip{
			ID:          "tip-standby",
			Title:       "关闭待机设备",
			Description: fmt.Sprintf("您有 %d 个设备处于待机状态，建议完全关闭不使用的设备", standbyCount),
			Category:    "hardware",
			Impact:      "medium",
			SavingsKWh:  float64(standbyCount) * 0.5,
			SavingsCost: float64(standbyCount) * 0.5 * m.settings.ElectricityRate,
		})
	}

	// 检查 HDD 使用
	hddCount := 0
	for _, d := range m.devices {
		if d.DeviceType == "hdd" {
			hddCount++
		}
	}
	if hddCount > 1 {
		tips = append(tips, EnergyTip{
			ID:          "tip-hdd-consolidate",
			Title:       "合并存储",
			Description: "多块硬盘同时运行会增加功耗，考虑合并存储或使用 RAID 节能模式",
			Category:    "hardware",
			Impact:      "high",
			SavingsKWh:  3.0,
			SavingsCost: 3.0 * m.settings.ElectricityRate,
		})
	}

	// CPU 优化建议
	tips = append(tips, EnergyTip{
		ID:          "tip-cpu-schedule",
		Title:       "优化任务调度",
		Description: "将高负载任务安排在电价低谷期执行，可降低电费",
		Category:    "software",
		Impact:      "medium",
		SavingsKWh:  1.5,
		SavingsCost: 1.5 * m.settings.ElectricityRate,
	})

	// 风扇优化
	tips = append(tips, EnergyTip{
		ID:          "tip-fan-control",
		Title:       "智能风扇控制",
		Description: "启用温度自动调节风扇转速，低负载时降低风扇功率",
		Category:    "hardware",
		Impact:      "low",
		SavingsKWh:  0.5,
		SavingsCost: 0.5 * m.settings.ElectricityRate,
	})

	// 网卡节能
	tips = append(tips, EnergyTip{
		ID:          "tip-nic-eee",
		Title:       "启用网络节能",
		Description: "开启网卡的 Energy Efficient Ethernet (EEE) 功能",
		Category:    "software",
		Impact:      "low",
		SavingsKWh:  0.3,
		SavingsCost: 0.3 * m.settings.ElectricityRate,
	})

	return tips
}

// UpdateSettings 更新能源设置.
func (m *Manager) UpdateSettings(settings *EnergySettings) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if settings.ElectricityRate > 0 {
		m.settings.ElectricityRate = settings.ElectricityRate
	}
	if settings.CarbonFactor > 0 {
		m.settings.CarbonFactor = settings.CarbonFactor
	}
	if settings.Currency != "" {
		m.settings.Currency = settings.Currency
	}
	m.settings.MonitoringEnabled = settings.MonitoringEnabled
	m.settings.AlertEnabled = settings.AlertEnabled
	m.settings.UpdatedAt = time.Now()
}

// GetSettings 获取能源设置.
func (m *Manager) GetSettings() *EnergySettings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	settings := *m.settings
	return &settings
}

// generateSavingsTips 生成节能建议文本.
func (m *Manager) generateSavingsTips() []string {
	return []string{
		"关闭不使用的硬盘可节省约 20% 能耗",
		"启用 CPU 频率调节可降低闲置时功耗",
		"使用 SSD 替代 HDD 可显著降低存储能耗",
		"优化风扇曲线在保证散热的同时降低风扇功耗",
	}
}
