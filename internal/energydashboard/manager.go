// Package energydashboard 提供智能能源监控核心逻辑
package energydashboard

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 能源看板管理器
type Manager struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	config          *EnergyDashboardConfig
	currentPower    *PowerConsumption
	records         []*EnergyRecord
	budgets         map[string]*PowerBudget
	alerts          []*PowerAlert
	devices         map[string]*DeviceConfig
	stopChan        chan struct{}
	running         bool
	lastUpdate      time.Time
}

// NewManager 创建能源看板管理器
func NewManager(logger *zap.Logger, config *EnergyDashboardConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultEnergyDashboardConfig()
	}

	m := &Manager{
		logger:    logger,
		config:    config,
		records:   make([]*EnergyRecord, 0),
		budgets:   make(map[string]*PowerBudget),
		alerts:    make([]*PowerAlert, 0),
		devices:   make(map[string]*DeviceConfig),
		stopChan:  make(chan struct{}),
		currentPower: &PowerConsumption{
			Timestamp: time.Now(),
		},
	}

	// 初始化设备
	for _, dev := range config.Devices {
		m.devices[dev.ID] = &DeviceConfig{
			ID:           dev.ID,
			Name:         dev.Name,
			DeviceType:   dev.DeviceType,
			Enabled:      dev.Enabled,
			PollInterval: dev.PollInterval,
			MaxWatts:     dev.MaxWatts,
		}
	}

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Start 启动监控
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("energy dashboard already running")
	}
	m.running = true
	m.mu.Unlock()

	m.logger.Info("starting energy dashboard")

	// 启动监控循环
	go m.monitorLoop(ctx)

	return nil
}

// Stop 停止监控
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.logger.Info("stopping energy dashboard")
	close(m.stopChan)
	m.running = false
}

// monitorLoop 监控循环
func (m *Manager) monitorLoop(ctx context.Context) {
	interval := time.Duration(m.config.MonitorInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectPowerData(ctx)
		}
	}
}

// collectPowerData 收集功耗数据
func (m *Manager) collectPowerData(ctx context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟功耗数据（实际应从硬件读取）
	power := &PowerConsumption{
		Timestamp:  time.Now(),
		TotalWatts: 45.5 + float64(time.Now().UnixNano()%20),
		CPUWatts:   15.2 + float64(time.Now().UnixNano()%10),
		DiskWatts:  12.3 + float64(time.Now().UnixNano()%5),
		NetWatts:   5.5 + float64(time.Now().UnixNano()%3),
		GPUWatts:   8.0,
		OtherWatts: 4.5,
	}

	m.currentPower = power
	m.lastUpdate = time.Now()

	// 记录能耗
	record := &EnergyRecord{
		ID:        generateID(),
		Timestamp: time.Now(),
		Wh:        power.TotalWatts * (float64(m.config.MonitorInterval) / 3600),
		Region:    m.config.Region,
	}
	m.records = append(m.records, record)

	// 检查预算告警
	if m.config.BudgetAlerts {
		m.checkBudgets()
	}

	// 清理过期数据
	m.cleanupOldData()
}

// checkBudgets 检查预算
func (m *Manager) checkBudgets() {
	todayStart := time.Now().Truncate(24 * time.Hour)
	todayWh := m.calculatePeriodEnergy(todayStart, time.Now())

	for _, budget := range m.budgets {
		if !budget.Enabled {
			continue
		}

		if budget.DailyLimitWh > 0 {
			percentage := (todayWh / budget.DailyLimitWh) * 100
			if percentage >= budget.AlertThreshold {
				alert := &PowerAlert{
					ID:        generateID(),
					BudgetID:  budget.ID,
					Level:     AlertLevelWarning,
					Message:   fmt.Sprintf("日耗电量已达预算的 %.1f%%", percentage),
					CurrentWh: todayWh,
					LimitWh:   budget.DailyLimitWh,
					Threshold: budget.AlertThreshold,
					Timestamp: time.Now(),
				}
				if percentage >= 100 {
					alert.Level = AlertLevelCritical
					alert.Message = "日耗电量已超出预算！"
				}
				m.alerts = append(m.alerts, alert)
			}
		}
	}
}

// calculatePeriodEnergy 计算时段能耗
func (m *Manager) calculatePeriodEnergy(start, end time.Time) float64 {
	total := 0.0
	for _, r := range m.records {
		if r.Timestamp.After(start) && r.Timestamp.Before(end) {
			total += r.Wh
		}
	}
	return total
}

// cleanupOldData 清理过期数据
func (m *Manager) cleanupOldData() {
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	newRecords := make([]*EnergyRecord, 0)
	for _, r := range m.records {
		if r.Timestamp.After(cutoff) {
			newRecords = append(newRecords, r)
		}
	}
	m.records = newRecords

	// 清理告警
	newAlerts := make([]*PowerAlert, 0)
	for _, a := range m.alerts {
		if a.Timestamp.After(cutoff) {
			newAlerts = append(newAlerts, a)
		}
	}
	m.alerts = newAlerts
}

// GetCurrentPower 获取当前功耗
func (m *Manager) GetCurrentPower() *PowerConsumption {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentPower
}

// GetDashboardData 获取看板数据
func (m *Manager) GetDashboardData() *EnergyDashboardResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	todayStart := now.Truncate(24 * time.Hour)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	todayWh := m.calculatePeriodEnergy(todayStart, now)
	monthWh := m.calculatePeriodEnergy(monthStart, now)

	region := m.config.Regions[m.config.Region]
	todayCost := m.calculateCost(todayWh, region)
	monthCost := m.calculateCost(monthWh, region)

	carbon := m.calculateCarbonEmission(monthWh)
	efficiency := m.calculateEfficiencyScore()
	pue := m.calculatePUE()

	budgets := make([]PowerBudget, 0)
	for _, b := range m.budgets {
		budgets = append(budgets, *b)
	}

	// 获取最近24小时趋势
	trend := m.getTrendData(now.Add(-24*time.Hour), now)

	// 获取设备统计
	deviceStats := m.getDeviceStats()

	return &EnergyDashboardResponse{
		CurrentPower: m.currentPower,
		TodayWh:      todayWh,
		TodayCost:    todayCost,
		MonthWh:      monthWh,
		MonthCost:    monthCost,
		Carbon:       carbon,
		Efficiency:   efficiency,
		PUE:          pue,
		Budgets:      budgets,
		Alerts:       m.getRecentAlerts(10),
		Trend:        trend,
		Devices:      deviceStats,
		Timestamp:    now,
	}
}

// calculateCost 计算电费
func (m *Manager) calculateCost(wh float64, region RegionConfig) float64 {
	kwh := wh / 1000.0
	cost := kwh * region.RatePerKWh
	cost = cost * (1 + region.TaxRate)
	return math.Round(cost*100) / 100
}

// calculateCarbonEmission 计算碳排放
func (m *Manager) calculateCarbonEmission(wh float64) *CarbonEmission {
	kwh := wh / 1000.0

	// 中国平均碳排放因子 0.581 kgCO2/kWh
	emissionFactor := 0.581
	totalKg := kwh * emissionFactor

	return &CarbonEmission{
		TotalKgCO2:   totalKg,
		TotalTonsCO2: totalKg / 1000,
		BySource: []CarbonSource{
			{
				SourceName:     "电网供电",
				EmissionFactor: emissionFactor,
				Percentage:     100.0,
			},
		},
		TreeEquiv: int(totalKg / 21.77), // 一棵树一年吸收约21.77kg CO2
		Period:    "monthly",
		Timestamp: time.Now(),
	}
}

// calculateEfficiencyScore 计算能效评分
func (m *Manager) calculateEfficiencyScore() *EfficiencyScore {
	// 简化实现：基于功耗和性能的评分
	baseScore := 75.0
	if m.currentPower != nil {
		// 功耗越低分数越高
		if m.currentPower.TotalWatts < 30 {
			baseScore = 95
		} else if m.currentPower.TotalWatts < 50 {
			baseScore = 85
		} else if m.currentPower.TotalWatts < 80 {
			baseScore = 70
		} else {
			baseScore = 55
		}
	}

	rating := "B"
	if baseScore >= 90 {
		rating = "A+"
	} else if baseScore >= 80 {
		rating = "A"
	} else if baseScore >= 70 {
		rating = "B"
	} else if baseScore >= 60 {
		rating = "C"
	} else {
		rating = "D"
	}

	return &EfficiencyScore{
		Score:       baseScore,
		Performance: baseScore * 0.9,
		WattRatio:   1.0 / (m.currentPower.TotalWatts + 0.1),
		Rating:      rating,
		Breakdown: map[string]float64{
			"cpu":     80.0,
			"storage": 75.0,
			"network": 90.0,
		},
		Suggestions: m.generateEfficiencySuggestions(baseScore),
		Timestamp:   time.Now(),
	}
}

// generateEfficiencySuggestions 生成能效建议
func (m *Manager) generateEfficiencySuggestions(score float64) []string {
	suggestions := make([]string, 0)

	if score < 70 {
		suggestions = append(suggestions, "考虑启用硬盘休眠功能以降低待机功耗")
		suggestions = append(suggestions, "检查是否有不必要的后台服务在运行")
	}
	if score < 80 {
		suggestions = append(suggestions, "优化 CPU 调度策略以提高性能/瓦特比")
	}
	if m.currentPower != nil && m.currentPower.DiskWatts > 15 {
		suggestions = append(suggestions, "磁盘功耗较高，考虑使用 SSD 替换 HDD")
	}

	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前能效表现良好，继续保持")
	}

	return suggestions
}

// calculatePUE 计算 PUE
func (m *Manager) calculatePUE() *PUEData {
	if !m.config.PUEEnabled {
		return nil
	}

	// 简化计算
	itEnergy := 40.0  // IT 设备能耗
	coolingEnergy := 10.0 // 制冷能耗
	otherEnergy := 5.0    // 其他能耗
	totalEnergy := itEnergy + coolingEnergy + otherEnergy

	pue := totalEnergy / itEnergy

	rating := "优秀"
	if pue > 2.0 {
		rating = "需改进"
	} else if pue > 1.6 {
		rating = "一般"
	} else if pue > 1.4 {
		rating = "良好"
	}

	return &PUEData{
		PUE:           math.Round(pue*100) / 100,
		ITEnergy:      itEnergy,
		TotalEnergy:   totalEnergy,
		CoolingEnergy: coolingEnergy,
		OtherEnergy:   otherEnergy,
		Rating:        rating,
		Timestamp:     time.Now(),
	}
}

// getTrendData 获取趋势数据
func (m *Manager) getTrendData(start, end time.Time) []TrendData {
	trend := make([]TrendData, 0)
	for _, r := range m.records {
		if r.Timestamp.After(start) && r.Timestamp.Before(end) {
			trend = append(trend, TrendData{
				Timestamp: r.Timestamp,
				Wh:        r.Wh,
				Watts:     r.Wh * 3600 / float64(m.config.MonitorInterval),
			})
		}
	}
	return trend
}

// getDeviceStats 获取设备统计
func (m *Manager) getDeviceStats() []DeviceStats {
	stats := make([]DeviceStats, 0)
	// 简化实现
	if m.currentPower != nil {
		stats = append(stats, DeviceStats{
			DeviceID:   "cpu",
			DeviceName: "CPU",
			DeviceType: DeviceTypeCPU,
			TotalWh:    m.currentPower.CPUWatts * 24,
			AvgWatts:   m.currentPower.CPUWatts,
			MaxWatts:   m.currentPower.CPUWatts * 1.5,
			MinWatts:   m.currentPower.CPUWatts * 0.5,
			Percentage: (m.currentPower.CPUWatts / m.currentPower.TotalWatts) * 100,
		})
		stats = append(stats, DeviceStats{
			DeviceID:   "disk",
			DeviceName: "Storage",
			DeviceType: DeviceTypeDisk,
			TotalWh:    m.currentPower.DiskWatts * 24,
			AvgWatts:   m.currentPower.DiskWatts,
			MaxWatts:   m.currentPower.DiskWatts * 1.3,
			MinWatts:   m.currentPower.DiskWatts * 0.7,
			Percentage: (m.currentPower.DiskWatts / m.currentPower.TotalWatts) * 100,
		})
	}
	return stats
}

// getRecentAlerts 获取最近告警
func (m *Manager) getRecentAlerts(limit int) []PowerAlert {
	alerts := make([]PowerAlert, 0)
	start := len(m.alerts) - limit
	if start < 0 {
		start = 0
	}
	for _, a := range m.alerts[start:] {
		alerts = append(alerts, *a)
	}
	return alerts
}

// SetBudget 设置功耗预算
func (m *Manager) SetBudget(req *SetBudgetRequest) *PowerBudget {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget := &PowerBudget{
		ID:              generateID(),
		Name:            req.Name,
		DailyLimitWh:    req.DailyLimitWh,
		MonthlyLimitKWh: req.MonthlyLimitKWh,
		AlertThreshold:  req.AlertThreshold,
		Enabled:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if budget.AlertThreshold == 0 {
		budget.AlertThreshold = 80
	}

	m.budgets[budget.ID] = budget
	return budget
}

// GetBudgets 获取所有预算
func (m *Manager) GetBudgets() []PowerBudget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budgets := make([]PowerBudget, 0)
	for _, b := range m.budgets {
		budgets = append(budgets, *b)
	}
	return budgets
}

// DeleteBudget 删除预算
func (m *Manager) DeleteBudget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.budgets[id]; !ok {
		return fmt.Errorf("budget %s not found", id)
	}

	delete(m.budgets, id)
	return nil
}

// UpdateRegionConfig 更新地区配置
func (m *Manager) UpdateRegionConfig(req *UpdateRegionRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Regions[req.Code] = RegionConfig{
		Code:          req.Code,
		Name:          req.Name,
		Currency:      req.Currency,
		RatePerKWh:    req.RatePerKWh,
		OffPeakRate:   req.OffPeakRate,
		PeakRate:      req.PeakRate,
		SuperPeakRate: req.SuperPeakRate,
		TaxRate:       req.TaxRate,
		UpdatedAt:     time.Now(),
	}
}

// GetRegionConfig 获取地区配置
func (m *Manager) GetRegionConfig(code string) (*RegionConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	region, ok := m.config.Regions[code]
	if !ok {
		return nil, fmt.Errorf("region %s not found", code)
	}
	return &region, nil
}

// GenerateReport 生成能源报告
func (m *Manager) GenerateReport(ctx context.Context, reportType ReportType, startDate, endDate time.Time) (*EnergyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalWh := m.calculatePeriodEnergy(startDate, endDate)
	region := m.config.Regions[m.config.Region]
	totalCost := m.calculateCost(totalWh, region)

	carbon := m.calculateCarbonEmission(totalWh)
	efficiency := m.calculateEfficiencyScore()

	// 计算统计
	records := make([]*EnergyRecord, 0)
	peakWh := 0.0
	lowWh := math.MaxFloat64
	var peakTime, lowTime time.Time

	for _, r := range m.records {
		if r.Timestamp.After(startDate) && r.Timestamp.Before(endDate) {
			records = append(records, r)
			if r.Wh > peakWh {
				peakWh = r.Wh
				peakTime = r.Timestamp
			}
			if r.Wh < lowWh {
				lowWh = r.Wh
				lowTime = r.Timestamp
			}
		}
	}

	days := endDate.Sub(startDate).Hours() / 24
	if days < 1 {
		days = 1
	}

	report := &EnergyReport{
		ID:           generateID(),
		ReportType:   reportType,
		Period:       fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")),
		StartDate:    startDate,
		EndDate:      endDate,
		TotalWh:      totalWh,
		TotalCost:    totalCost,
		TotalCarbon:  carbon.TotalKgCO2,
		AvgDailyWh:   totalWh / days,
		PeakWh:       peakWh,
		PeakTime:     peakTime,
		LowWh:        lowWh,
		LowTime:      lowTime,
		Efficiency:   efficiency,
		Devices:      m.getDeviceStats(),
		Recommendations: m.generateReportRecommendations(totalWh, efficiency),
		GeneratedAt:  time.Now(),
	}

	return report, nil
}

// generateReportRecommendations 生成报告建议
func (m *Manager) generateReportRecommendations(totalWh float64, efficiency *EfficiencyScore) []string {
	recs := make([]string, 0)

	if efficiency.Score < 70 {
		recs = append(recs, "建议优化系统配置以提高能效")
	}
	if totalWh > 1000 {
		recs = append(recs, "月度能耗较高，建议启用定时关机功能")
	}
	if len(recs) == 0 {
		recs = append(recs, "能源使用情况良好")
	}
	return recs
}

// AcknowledgeAlert 确认告警
func (m *Manager) AcknowledgeAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, a := range m.alerts {
		if a.ID == id {
			a.Acked = true
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", id)
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *EnergyDashboardConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *EnergyDashboardConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// AddDevice 添加设备
func (m *Manager) AddDevice(dev *DeviceConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.devices[dev.ID] = dev
}

// RemoveDevice 移除设备
func (m *Manager) RemoveDevice(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.devices, id)
}

// GetDevices 获取所有设备
func (m *Manager) GetDevices() []DeviceConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]DeviceConfig, 0)
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}