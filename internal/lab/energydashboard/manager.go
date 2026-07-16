// Package energydashboard 提供能耗监控核心管理逻辑
package energydashboard

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 能耗仪表盘管理器.
type Manager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	config        *DashboardConfig
	snapshots     []*SystemPowerSnapshot
	readings      map[string][]*PowerReading
	rates         map[string]*ElectricityRate
	schedules     map[string]*SleepSchedule
	reports       map[string]*EnergyReport
	carbonHistory []*CarbonEstimate
	stopChan      chan struct{}
	running       bool
}

// NewManager 创建能耗仪表盘管理器.
func NewManager(logger *zap.Logger, config *DashboardConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultDashboardConfig()
	}

	m := &Manager{
		logger:        logger,
		config:        config,
		readings:      make(map[string][]*PowerReading),
		rates:         make(map[string]*ElectricityRate),
		schedules:     make(map[string]*SleepSchedule),
		reports:       make(map[string]*EnergyReport),
		carbonHistory: make([]*CarbonEstimate, 0),
		stopChan:      make(chan struct{}),
	}

	// 初始化默认电价
	m.initDefaultRates()
	// 初始化默认休眠计划
	m.initDefaultSchedules()

	return m
}

// generateID 生成唯一 ID.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// initDefaultRates 初始化默认电价.
func (m *Manager) initDefaultRates() {
	defaultRates := []*ElectricityRate{
		{
			ID:           "rate-cn-default",
			Region:       "cn-default",
			Currency:     "CNY",
			ProviderName: "国家电网",
			Rates:        GetDefaultRates(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "rate-cn-valley",
			Region:       "cn-valley",
			Currency:     "CNY",
			ProviderName: "国家电网-谷电",
			Rates: []RateTier{
				{Name: "深谷", StartTime: "00:00", EndTime: "06:00", PriceKWh: 0.15},
				{Name: "低谷", StartTime: "06:00", EndTime: "08:00", PriceKWh: 0.28},
				{Name: "平段", StartTime: "08:00", EndTime: "18:00", PriceKWh: 0.56},
				{Name: "高峰", StartTime: "18:00", EndTime: "22:00", PriceKWh: 0.85},
				{Name: "低谷", StartTime: "22:00", EndTime: "00:00", PriceKWh: 0.28},
			},
			UpdatedAt: time.Now(),
		},
	}

	for _, r := range defaultRates {
		m.rates[r.ID] = r
	}
}

// initDefaultSchedules 初始化默认休眠计划.
func (m *Manager) initDefaultSchedules() {
	defaultSchedules := []*SleepSchedule{
		{
			ID:           "sched-night",
			Name:         "夜间休眠",
			Policy:       SleepPolicyScheduled,
			TargetDevice: "system",
			StartTime:    "23:00",
			EndTime:      "07:00",
			Enabled:      true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
		{
			ID:           "sched-idle",
			Name:         "空闲休眠",
			Policy:       SleepPolicyIdle,
			TargetDevice: "disks",
			IdleMinutes:  30,
			Enabled:      true,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		},
	}

	for _, s := range defaultSchedules {
		m.schedules[s.ID] = s
	}
}

// Start 启动能耗监控.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("energy dashboard is already running")
	}

	m.running = true
	m.stopChan = make(chan struct{})

	go m.monitorLoop(ctx)

	m.logger.Info("energy dashboard started",
		zap.Int("interval", m.config.MonitorInterval),
		zap.String("region", m.config.Region))

	return nil
}

// Stop 停止能耗监控.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopChan)
	m.running = false
	m.logger.Info("energy dashboard stopped")
}

// IsRunning 是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// monitorLoop 监控循环.
func (m *Manager) monitorLoop(ctx context.Context) {
	interval := time.Duration(m.config.MonitorInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collectReadings()
		}
	}
}

// collectReadings 采集功耗读数.
func (m *Manager) collectReadings() {
	now := time.Now()

	// 模拟采集各组件功耗（实际环境需要对接硬件传感器）
	readings := m.simulateReadings(now)

	snapshot := &SystemPowerSnapshot{
		ID:         generateID(),
		Components: readings,
		Timestamp:  now,
	}

	for _, r := range readings {
		switch r.Component {
		case ComponentCPU:
			snapshot.CPUPower = r.PowerWatts
			snapshot.CPUTemperature = r.Temperature
		case ComponentDisk:
			snapshot.DiskPower += r.PowerWatts
		case ComponentFan:
			snapshot.FanPower += r.PowerWatts
		}
		snapshot.TotalPower += r.PowerWatts
	}

	m.mu.Lock()
	m.snapshots = append(m.snapshots, snapshot)
	// 限制快照数量（保留最近 24 小时）
	maxSnapshots := 86400 / m.config.MonitorInterval
	if len(m.snapshots) > maxSnapshots {
		m.snapshots = m.snapshots[len(m.snapshots)-maxSnapshots:]
	}
	m.mu.Unlock()
}

// simulateReadings 模拟功耗读数.
func (m *Manager) simulateReadings(now time.Time) []PowerReading {
	hour := float64(now.Hour()) + float64(now.Minute())/60.0

	// CPU 功耗随时间波动
	cpuBase := 45.0
	cpuVar := 20.0 * math.Sin(hour*math.Pi/12)
	cpuPower := cpuBase + cpuVar + (float64(now.Second()%10) - 5)

	readings := []PowerReading{
		{
			ID:          generateID(),
			Component:   ComponentCPU,
			DeviceName:  "CPU",
			PowerWatts:  math.Max(0, cpuPower),
			Temperature: 45.0 + cpuVar/2,
			State:       PowerStateActive,
			Timestamp:   now,
		},
		{
			ID:         generateID(),
			Component:  ComponentDisk,
			DeviceName: "HDD-01",
			PowerWatts: 8.5,
			State:      PowerStateActive,
			Timestamp:  now,
		},
		{
			ID:         generateID(),
			Component:  ComponentDisk,
			DeviceName: "HDD-02",
			PowerWatts: 7.2,
			State:      PowerStateIdle,
			Timestamp:  now,
		},
		{
			ID:         generateID(),
			Component:  ComponentDisk,
			DeviceName: "SSD-01",
			PowerWatts: 3.5,
			State:      PowerStateActive,
			Timestamp:  now,
		},
		{
			ID:         generateID(),
			Component:  ComponentFan,
			DeviceName: "系统风扇",
			PowerWatts: 5.0 + cpuVar/4,
			State:      PowerStateActive,
			Timestamp:  now,
		},
	}

	return readings
}

// RecordPowerReading 手动记录功耗读数.
func (m *Manager) RecordPowerReading(reading *PowerReading) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if reading.ID == "" {
		reading.ID = generateID()
	}
	if reading.Timestamp.IsZero() {
		reading.Timestamp = time.Now()
	}

	key := string(reading.Component) + "-" + reading.DeviceName
	m.readings[key] = append(m.readings[key], reading)
}

// GetLatestSnapshot 获取最新功耗快照.
func (m *Manager) GetLatestSnapshot() *SystemPowerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.snapshots) == 0 {
		return &SystemPowerSnapshot{
			ID:         generateID(),
			Components: []PowerReading{},
			Timestamp:  time.Now(),
		}
	}

	return m.snapshots[len(m.snapshots)-1]
}

// GetSnapshots 获取功耗快照历史.
func (m *Manager) GetSnapshots(since time.Time, limit int) []*SystemPowerSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 100
	}

	var result []*SystemPowerSnapshot
	for i := len(m.snapshots) - 1; i >= 0 && len(result) < limit; i-- {
		if m.snapshots[i].Timestamp.After(since) || m.snapshots[i].Timestamp.Equal(since) {
			result = append(result, m.snapshots[i])
		}
	}

	// 反转为时间顺序
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}

	return result
}

// CreateRate 创建电价配置.
func (m *Manager) CreateRate(req *ElectricityRate) (*ElectricityRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ID == "" {
		req.ID = "rate-" + req.Region + "-" + generateID()
	}
	req.UpdatedAt = time.Now()

	// 校验时间格式
	for _, tier := range req.Rates {
		if !isValidTimeFormat(tier.StartTime) || !isValidTimeFormat(tier.EndTime) {
			return nil, fmt.Errorf("invalid time format for tier '%s', expected HH:MM", tier.Name)
		}
		if tier.PriceKWh <= 0 {
			return nil, fmt.Errorf("invalid price for tier '%s', must be positive", tier.Name)
		}
	}

	m.rates[req.ID] = req
	m.logger.Info("electricity rate created", zap.String("id", req.ID), zap.String("region", req.Region))
	return req, nil
}

// GetRate 获取电价配置.
func (m *Manager) GetRate(id string) (*ElectricityRate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rate, ok := m.rates[id]
	if !ok {
		return nil, fmt.Errorf("rate not found: %s", id)
	}
	return rate, nil
}

// ListRates 列出所有电价配置.
func (m *Manager) ListRates() []*ElectricityRate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rates := make([]*ElectricityRate, 0, len(m.rates))
	for _, r := range m.rates {
		rates = append(rates, r)
	}
	return rates
}

// UpdateRate 更新电价配置.
func (m *Manager) UpdateRate(id string, req *ElectricityRate) (*ElectricityRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rate, ok := m.rates[id]
	if !ok {
		return nil, fmt.Errorf("rate not found: %s", id)
	}

	for _, tier := range req.Rates {
		if !isValidTimeFormat(tier.StartTime) || !isValidTimeFormat(tier.EndTime) {
			return nil, fmt.Errorf("invalid time format for tier '%s', expected HH:MM", tier.Name)
		}
		if tier.PriceKWh <= 0 {
			return nil, fmt.Errorf("invalid price for tier '%s', must be positive", tier.Name)
		}
	}

	rate.Region = req.Region
	rate.Currency = req.Currency
	rate.ProviderName = req.ProviderName
	rate.Rates = req.Rates
	rate.UpdatedAt = time.Now()

	return rate, nil
}

// DeleteRate 删除电价配置.
func (m *Manager) DeleteRate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rates[id]; !ok {
		return fmt.Errorf("rate not found: %s", id)
	}
	delete(m.rates, id)
	return nil
}

// CalculateEnergyCost 计算能耗费用.
func (m *Manager) CalculateEnergyCost(ctx context.Context, period EnergyReportPeriod, rateID string) (*EnergyCost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !IsValidPeriod(period) {
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	rate, ok := m.rates[rateID]
	if !ok {
		return nil, fmt.Errorf("rate not found: %s", rateID)
	}

	now := time.Now()
	startDate, endDate := calculatePeriodRange(period, now)

	// 获取周期内的快照数据
	totalKWh := 0.0
	componentKWh := make(map[ComponentType]float64)

	for _, snap := range m.snapshots {
		if snap.Timestamp.Before(startDate) || snap.Timestamp.After(endDate) {
			continue
		}
		// 功率 * 时间间隔（小时）= 电量（Wh）/ 1000 = kWh
		hours := float64(m.config.MonitorInterval) / 3600.0
		kwh := snap.TotalPower * hours / 1000.0
		totalKWh += kwh

		for _, comp := range snap.Components {
			compKwh := comp.PowerWatts * hours / 1000.0
			componentKWh[comp.Component] += compKwh
		}
	}

	// 按当前时段计算费用
	currentRate := getCurrentRate(rate, now)
	totalCost := totalKWh * currentRate

	// 生成费用细分
	breakdown := make([]CostBreakdown, 0)
	for comp, kwh := range componentKWh {
		pct := 0.0
		if totalKWh > 0 {
			pct = (kwh / totalKWh) * 100
		}
		breakdown = append(breakdown, CostBreakdown{
			Component:  comp,
			KWh:        kwh,
			Cost:       kwh * currentRate,
			Percentage: math.Round(pct*100) / 100,
		})
	}

	return &EnergyCost{
		ID:        generateID(),
		Region:    rate.Region,
		Period:    period,
		StartDate: startDate,
		EndDate:   endDate,
		KWh:       math.Round(totalKWh*1000) / 1000,
		Cost:      math.Round(totalCost*100) / 100,
		Currency:  rate.Currency,
		Breakdown: breakdown,
		CreatedAt: time.Now(),
	}, nil
}

// CalculateEfficiencyScore 计算能效评分.
func (m *Manager) CalculateEfficiencyScore() *EfficiencyScore {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 获取最近平均功耗
	avgPower := m.getAveragePower(time.Hour)

	wattsPerTB := 0.0
	if m.config.TotalStorageTB > 0 {
		wattsPerTB = avgPower / m.config.TotalStorageTB
	}

	score, rating := m.calculateScoreRating(wattsPerTB)

	recommendations := m.generateRecommendations(wattsPerTB, avgPower)

	return &EfficiencyScore{
		ID:              generateID(),
		TotalStorageTB:  m.config.TotalStorageTB,
		TotalPowerWatts: math.Round(avgPower*100) / 100,
		WattsPerTB:      math.Round(wattsPerTB*100) / 100,
		Score:           score,
		Rating:          rating,
		Recommendations: recommendations,
		UpdatedAt:       time.Now(),
	}
}

// getAveragePower 获取平均功耗.
func (m *Manager) getAveragePower(duration time.Duration) float64 {
	if len(m.snapshots) == 0 {
		return 0
	}

	since := time.Now().Add(-duration)
	var total float64
	var count int

	for _, snap := range m.snapshots {
		if snap.Timestamp.After(since) {
			total += snap.TotalPower
			count++
		}
	}

	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// calculateScoreRating 计算评分和等级.
func (m *Manager) calculateScoreRating(wattsPerTB float64) (int, string) {
	// 评分标准（W/TB）：
	// A+: < 3, A: 3-5, B: 5-8, C: 8-12, D: > 12
	if wattsPerTB <= 0 {
		return 100, "A+"
	}

	switch {
	case wattsPerTB < 3:
		return 95, "A+"
	case wattsPerTB < 5:
		return 85, "A"
	case wattsPerTB < 8:
		return 70, "B"
	case wattsPerTB < 12:
		return 50, "C"
	default:
		return 30, "D"
	}
}

// generateRecommendations 生成节能建议.
func (m *Manager) generateRecommendations(wattsPerTB, avgPower float64) []string {
	var recs []string

	if wattsPerTB > 8 {
		recs = append(recs, "考虑升级到低功耗硬盘（如 WD Red Plus）以降低每TB功耗")
	}
	if wattsPerTB > 5 {
		recs = append(recs, "启用硬盘自动休眠功能，在非访问时段减少硬盘功耗")
	}
	if avgPower > 100 {
		recs = append(recs, "整体功耗较高，检查是否有不必要的设备处于活动状态")
	}

	// 检查是否有未启用的休眠计划
	idleSchedActive := false
	for _, s := range m.schedules {
		if s.Policy == SleepPolicyIdle && s.Enabled {
			idleSchedActive = true
			break
		}
	}
	if !idleSchedActive {
		recs = append(recs, "建议启用空闲休眠策略，在无访问时自动待机硬盘")
	}

	if len(recs) == 0 {
		recs = append(recs, "当前能效表现良好，继续保持")
	}

	return recs
}

// EstimateCarbon 碳排放估算.
func (m *Manager) EstimateCarbon(ctx context.Context, period EnergyReportPeriod) (*CarbonEstimate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !IsValidPeriod(period) {
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	now := time.Now()
	startDate, endDate := calculatePeriodRange(period, now)

	totalKWh := 0.0
	for _, snap := range m.snapshots {
		if snap.Timestamp.Before(startDate) || snap.Timestamp.After(endDate) {
			continue
		}
		hours := float64(m.config.MonitorInterval) / 3600.0
		totalKWh += snap.TotalPower * hours / 1000.0
	}

	carbonKg := totalKWh * m.config.CarbonFactor
	// 一棵树一年约吸收 22 kg CO2
	equivalentTree := carbonKg / 22.0

	estimate := &CarbonEstimate{
		ID:             generateID(),
		KWh:            math.Round(totalKWh*1000) / 1000,
		CarbonKg:       math.Round(carbonKg*1000) / 1000,
		Factor:         m.config.CarbonFactor,
		Region:         m.config.Region,
		Period:         period,
		StartDate:      startDate,
		EndDate:        endDate,
		EquivalentTree: math.Round(equivalentTree*1000) / 1000,
		CreatedAt:      now,
	}

	return estimate, nil
}

// GenerateReport 生成能耗报表.
func (m *Manager) GenerateReport(ctx context.Context, period EnergyReportPeriod, rateID string) (*EnergyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !IsValidPeriod(period) {
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	rate, ok := m.rates[rateID]
	if !ok {
		return nil, fmt.Errorf("rate not found: %s", rateID)
	}

	now := time.Now()
	startDate, endDate := calculatePeriodRange(period, now)

	totalKWh := 0.0
	peakPower := 0.0
	var peakTime time.Time
	componentData := make(map[ComponentType]*ComponentReport)

	for _, snap := range m.snapshots {
		if snap.Timestamp.Before(startDate) || snap.Timestamp.After(endDate) {
			continue
		}

		hours := float64(m.config.MonitorInterval) / 3600.0
		snapKWh := snap.TotalPower * hours / 1000.0
		totalKWh += snapKWh

		if snap.TotalPower > peakPower {
			peakPower = snap.TotalPower
			peakTime = snap.Timestamp
		}

		for _, comp := range snap.Components {
			key := string(comp.Component) + "-" + comp.DeviceName
			compType := comp.Component

			report, exists := componentData[compType]
			if !exists {
				report = &ComponentReport{
					Component:  compType,
					DeviceName: comp.DeviceName,
				}
				componentData[compType] = report
			}

			compKWh := comp.PowerWatts * hours / 1000.0
			report.TotalKWh += compKWh
			if comp.PowerWatts > report.MaxPower {
				report.MaxPower = comp.PowerWatts
			}
			report.UptimeHours += hours
			_ = key
		}
	}

	// 计算平均值和汇总
	components := make([]ComponentReport, 0, len(componentData))
	for _, report := range componentData {
		if report.UptimeHours > 0 {
			report.AvgPower = (report.TotalKWh * 1000) / report.UptimeHours
		}
		report.TotalKWh = math.Round(report.TotalKWh*1000) / 1000
		report.AvgPower = math.Round(report.AvgPower*100) / 100
		report.MaxPower = math.Round(report.MaxPower*100) / 100
		report.UptimeHours = math.Round(report.UptimeHours*100) / 100
		components = append(components, *report)
	}

	// 按组件类型排序
	sort.Slice(components, func(i, j int) bool {
		return components[i].Component < components[j].Component
	})

	// 计算费用
	currentRate := getCurrentRate(rate, now)
	totalCost := totalKWh * currentRate

	// 计算碳排放
	carbonKg := totalKWh * m.config.CarbonFactor

	// 计算日均
	days := endDate.Sub(startDate).Hours() / 24
	if days < 1 {
		days = 1
	}
	dailyAvg := totalKWh / days

	// 能效评分
	wattsPerTB := 0.0
	avgPower := 0.0
	if len(m.snapshots) > 0 {
		avgPower = (totalKWh * 1000) / (endDate.Sub(startDate).Hours())
		if m.config.TotalStorageTB > 0 {
			wattsPerTB = avgPower / m.config.TotalStorageTB
		}
	}
	score, _ := m.calculateScoreRating(wattsPerTB)

	report := &EnergyReport{
		ID:              generateID(),
		Period:          period,
		StartDate:       startDate,
		EndDate:         endDate,
		TotalKWh:        math.Round(totalKWh*1000) / 1000,
		TotalCost:       math.Round(totalCost*100) / 100,
		Currency:        rate.Currency,
		CarbonKg:        math.Round(carbonKg*1000) / 1000,
		Components:      components,
		DailyAverage:    math.Round(dailyAvg*1000) / 1000,
		PeakPower:       math.Round(peakPower*100) / 100,
		PeakTime:        peakTime,
		EfficiencyScore: score,
		CreatedAt:       now,
	}

	// 保存报表
	m.reports[report.ID] = report

	return report, nil
}

// GetReport 获取能耗报表.
func (m *Manager) GetReport(id string) (*EnergyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report not found: %s", id)
	}
	return report, nil
}

// ListReports 列出能耗报表.
func (m *Manager) ListReports(period EnergyReportPeriod, limit int) []*EnergyReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	reports := make([]*EnergyReport, 0)
	for _, r := range m.reports {
		if period == "" || r.Period == period {
			reports = append(reports, r)
		}
	}

	// 按时间倒序
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].CreatedAt.After(reports[j].CreatedAt)
	})

	if len(reports) > limit {
		reports = reports[:limit]
	}

	return reports
}

// CreateSchedule 创建休眠计划.
func (m *Manager) CreateSchedule(req *SleepSchedule) (*SleepSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ID == "" {
		req.ID = "sched-" + generateID()
	}

	// 校验时间格式
	if req.Policy == SleepPolicyScheduled {
		if !isValidTimeFormat(req.StartTime) || !isValidTimeFormat(req.EndTime) {
			return nil, fmt.Errorf("invalid time format, expected HH:MM")
		}
	}

	if req.Policy == SleepPolicyIdle && req.IdleMinutes <= 0 {
		return nil, fmt.Errorf("idle_minutes must be positive for idle policy")
	}

	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	m.schedules[req.ID] = req
	m.logger.Info("sleep schedule created",
		zap.String("id", req.ID),
		zap.String("name", req.Name),
		zap.String("policy", string(req.Policy)))

	return req, nil
}

// GetSchedule 获取休眠计划.
func (m *Manager) GetSchedule(id string) (*SleepSchedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sched, ok := m.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	return sched, nil
}

// ListSchedules 列出所有休眠计划.
func (m *Manager) ListSchedules() []*SleepSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]*SleepSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// UpdateSchedule 更新休眠计划.
func (m *Manager) UpdateSchedule(id string, req *SleepSchedule) (*SleepSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sched, ok := m.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	if req.Policy == SleepPolicyScheduled {
		if !isValidTimeFormat(req.StartTime) || !isValidTimeFormat(req.EndTime) {
			return nil, fmt.Errorf("invalid time format, expected HH:MM")
		}
	}

	sched.Name = req.Name
	sched.Policy = req.Policy
	sched.TargetDevice = req.TargetDevice
	sched.StartTime = req.StartTime
	sched.EndTime = req.EndTime
	sched.IdleMinutes = req.IdleMinutes
	sched.Enabled = req.Enabled
	sched.UpdatedAt = time.Now()

	return sched, nil
}

// DeleteSchedule 删除休眠计划.
func (m *Manager) DeleteSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}
	delete(m.schedules, id)
	return nil
}

// ToggleSchedule 启用/禁用休眠计划.
func (m *Manager) ToggleSchedule(id string) (*SleepSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sched, ok := m.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	sched.Enabled = !sched.Enabled
	sched.UpdatedAt = time.Now()

	return sched, nil
}

// GetDashboardSummary 获取仪表盘总览.
func (m *Manager) GetDashboardSummary() *DashboardSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := m.GetLatestSnapshot()

	// 今日能耗
	todayStart := time.Now().Truncate(24 * time.Hour)
	todayKWh := 0.0
	monthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Now().Location())
	monthKWh := 0.0

	for _, snap := range m.snapshots {
		hours := float64(m.config.MonitorInterval) / 3600.0
		kwh := snap.TotalPower * hours / 1000.0

		if snap.Timestamp.After(todayStart) {
			todayKWh += kwh
		}
		if snap.Timestamp.After(monthStart) {
			monthKWh += kwh
		}
	}

	// 默认电价计算费用
	defaultRate := 0.56
	if r, ok := m.rates["rate-cn-default"]; ok && len(r.Rates) > 0 {
		defaultRate = r.Rates[0].PriceKWh
	}

	todayCost := todayKWh * defaultRate
	monthCost := monthKWh * defaultRate

	// 能效评分
	avgPower := m.getAveragePower(time.Hour)
	wattsPerTB := 0.0
	if m.config.TotalStorageTB > 0 {
		wattsPerTB = avgPower / m.config.TotalStorageTB
	}
	score, rating := m.calculateScoreRating(wattsPerTB)
	recs := m.generateRecommendations(wattsPerTB, avgPower)

	efficiencyScore := &EfficiencyScore{
		ID:              generateID(),
		TotalStorageTB:  m.config.TotalStorageTB,
		TotalPowerWatts: math.Round(avgPower*100) / 100,
		WattsPerTB:      math.Round(wattsPerTB*100) / 100,
		Score:           score,
		Rating:          rating,
		Recommendations: recs,
		UpdatedAt:       time.Now(),
	}

	// 碳排放
	carbonToday := todayKWh * m.config.CarbonFactor

	// 活跃休眠计划
	activeSchedules := 0
	for _, s := range m.schedules {
		if s.Enabled {
			activeSchedules++
		}
	}

	return &DashboardSummary{
		CurrentPower:    snapshot,
		TodayKWh:        math.Round(todayKWh*1000) / 1000,
		TodayCost:       math.Round(todayCost*100) / 100,
		MonthKWh:        math.Round(monthKWh*1000) / 1000,
		MonthCost:       math.Round(monthCost*100) / 100,
		Currency:        m.config.Currency,
		EfficiencyScore: efficiencyScore,
		CarbonToday:     math.Round(carbonToday*1000) / 1000,
		ActiveSchedules: activeSchedules,
		LastUpdated:     time.Now(),
	}
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *DashboardConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *DashboardConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// calculatePeriodRange 计算时间范围.
func calculatePeriodRange(period EnergyReportPeriod, now time.Time) (start, end time.Time) {
	end = now

	switch period {
	case PeriodDaily:
		start = now.Truncate(24 * time.Hour)
	case PeriodWeekly:
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		start = now.AddDate(0, 0, -(weekday - 1)).Truncate(24 * time.Hour)
	case PeriodMonthly:
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case PeriodYearly:
		start = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	}

	return start, end
}

// getCurrentRate 获取当前时段电价.
func getCurrentRate(rate *ElectricityRate, now time.Time) float64 {
	currentTime := now.Format("15:04")

	for _, tier := range rate.Rates {
		if isTimeInRange(currentTime, tier.StartTime, tier.EndTime) {
			return tier.PriceKWh
		}
	}

	// 默认返回第一个阶梯
	if len(rate.Rates) > 0 {
		return rate.Rates[0].PriceKWh
	}
	return 0
}

// isTimeInRange 检查时间是否在范围内.
func isTimeInRange(current, start, end string) bool {
	if start <= end {
		return current >= start && current < end
	}
	// 跨日（如 22:00-08:00）
	return current >= start || current < end
}

// isValidTimeFormat 校验时间格式 HH:MM.
func isValidTimeFormat(t string) bool {
	if len(t) != 5 {
		return false
	}
	if t[2] != ':' {
		return false
	}
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return false
	}
	hour := 0
	minute := 0
	for _, c := range parts[0] {
		if c < '0' || c > '9' {
			return false
		}
		hour = hour*10 + int(c-'0')
	}
	for _, c := range parts[1] {
		if c < '0' || c > '9' {
			return false
		}
		minute = minute*10 + int(c-'0')
	}
	return hour >= 0 && hour <= 23 && minute >= 0 && minute <= 59
}
