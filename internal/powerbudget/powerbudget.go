// Package powerbudget 提供用电预算管理功能
package powerbudget

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Engine 用电预算引擎.
type Engine struct {
	budget   *Budget
	records  []*PowerRecord
	alerts   []*Alert
	devices  map[string]*DevicePower
	tracker  *Tracker
	analyzer *Analyzer
	alertMgr *AlertManager
	logger   *zap.Logger
	running  bool
	mu       sync.RWMutex
	stopCh   chan struct{}
}

// NewEngine 创建用电预算引擎.
func NewEngine(logger *zap.Logger) *Engine {
	if logger == nil {
		logger, _ = zap.NewDevelopment()
	}

	e := &Engine{
		records:  make([]*PowerRecord, 0),
		alerts:   make([]*Alert, 0),
		devices:  make(map[string]*DevicePower),
		logger:   logger,
		stopCh:   make(chan struct{}),
	}

	e.tracker = NewTracker(e, logger)
	e.analyzer = NewAnalyzer(e, logger)
	e.alertMgr = NewAlertManager(e, logger)

	return e
}

// Start 启动引擎.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return nil
	}

	e.running = true
	e.logger.Info("用电预算引擎已启动")

	go e.monitorLoop()

	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	e.running = false
	close(e.stopCh)
	e.logger.Info("用电预算引擎已停止")

	return nil
}

// IsRunning 返回引擎是否运行中.
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// ========== 核心方法 ==========

// RecordPower 记录用电数据.
func (e *Engine) RecordPower(req RecordPowerRequest) (*PowerRecord, error) {
	if !e.IsRunning() {
		return nil, ErrEngineNotRunning
	}

	if req.PowerWatts < 0 {
		return nil, ErrInvalidPowerWatts
	}

	duration := req.DurationSec
	if duration <= 0 {
		duration = 60 // 默认60秒
	}

	electricityPrice := DefaultElectricityPrice
	if e.budget != nil {
		electricityPrice = e.budget.ElectricityPrice
	}

	energyKWh := req.PowerWatts * float64(duration) / 3600.0 / 1000.0
	costCents := int64(energyKWh * electricityPrice)

	record := &PowerRecord{
		ID:         uuid.New().String(),
		Timestamp:  time.Now(),
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		PowerWatts: req.PowerWatts,
		EnergyKWh:  energyKWh,
		CostCents:  costCents,
		Duration:   duration,
		Service:    req.Service,
		Metadata:   req.Metadata,
	}

	e.mu.Lock()
	e.records = append(e.records, record)
	e.tracker.updateDeviceProfile(record)
	e.mu.Unlock()

	e.logger.Debug("记录用电数据",
		zap.String("device", req.DeviceName),
		zap.Float64("power_watts", req.PowerWatts),
		zap.Float64("energy_kwh", energyKWh),
	)

	// 检查预算告警
	e.alertMgr.CheckBudgetAlerts()

	return record, nil
}

// SetBudget 设置用电预算.
func (e *Engine) SetBudget(req SetBudgetRequest) (*Budget, error) {
	if req.MonthlyAmount <= 0 {
		return nil, ErrInvalidBudgetAmount
	}
	if req.ElectricityPrice <= 0 {
		return nil, ErrInvalidElectricityPrice
	}

	now := time.Now()

	if req.WarningThreshold <= 0 {
		req.WarningThreshold = DefaultWarningThreshold
	}
	if req.CriticalThreshold <= 0 {
		req.CriticalThreshold = DefaultCriticalThreshold
	}
	if req.Name == "" {
		req.Name = "用电预算"
	}

	e.mu.Lock()
	if e.budget == nil {
		e.budget = &Budget{
			ID:        uuid.New().String(),
			CreatedAt: now,
		}
	}

	e.budget.Name = req.Name
	e.budget.MonthlyAmount = req.MonthlyAmount
	e.budget.ElectricityPrice = req.ElectricityPrice
	e.budget.WarningThreshold = req.WarningThreshold
	e.budget.CriticalThreshold = req.CriticalThreshold
	e.budget.Enabled = true
	e.budget.UpdatedAt = now
	budget := e.budget
	e.mu.Unlock()

	e.logger.Info("设置用电预算",
		zap.String("name", req.Name),
		zap.Float64("monthly_amount", req.MonthlyAmount),
		zap.Float64("electricity_price", req.ElectricityPrice),
	)

	return budget, nil
}

// GetBudgetStatus 获取预算状态.
func (e *Engine) GetBudgetStatus() (*BudgetStatus, error) {
	e.mu.RLock()
	budget := e.budget
	e.mu.RUnlock()

	if budget == nil {
		return nil, ErrBudgetNotSet
	}

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	daysElapsed := int(now.Sub(startOfMonth).Hours()/24) + 1
	daysInMonth := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Day()
	daysRemaining := daysInMonth - daysElapsed + 1

	var usedEnergy float64
	var usedCost int64

	e.mu.RLock()
	for _, r := range e.records {
		if r.Timestamp.After(startOfMonth) || r.Timestamp.Equal(startOfMonth) {
			usedEnergy += r.EnergyKWh
			usedCost += r.CostCents
		}
	}
	activeAlerts := e.getActiveAlertsLocked()
	e.mu.RUnlock()

	remaining := budget.MonthlyAmount - float64(usedCost)
	if remaining < 0 {
		remaining = 0
	}

	var usedPercent float64
	if budget.MonthlyAmount > 0 {
		usedPercent = float64(usedCost) / budget.MonthlyAmount * 100.0
	}

	dailyAvg := usedEnergy / float64(daysElapsed)

	trend := e.analyzer.CalculateTrend(7)

	status := &BudgetStatus{
		Budget:        budget,
		UsedEnergy:    usedEnergy,
		UsedCost:      usedCost,
		Remaining:     int64(remaining),
		UsedPercent:   usedPercent,
		DailyAvg:      dailyAvg,
		DaysElapsed:   daysElapsed,
		DaysRemaining: daysRemaining,
		Trend:         trend,
		Alerts:        activeAlerts,
	}

	return status, nil
}

// GetMonthlyReport 获取月度报告.
func (e *Engine) GetMonthlyReport() (*PowerReport, error) {
	return e.GetReport(ReportRequest{Period: PeriodMonthly})
}

// GetReport 获取用电报告.
func (e *Engine) GetReport(req ReportRequest) (*PowerReport, error) {
	now := time.Now()
	var start, end time.Time

	if req.StartTime != nil && req.EndTime != nil {
		start = *req.StartTime
		end = *req.EndTime
	} else {
		switch req.Period {
		case PeriodDaily:
			start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			end = now
		case PeriodWeekly:
			start = now.AddDate(0, 0, -7)
			end = now
		case PeriodMonthly:
			start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
			end = now
		default:
			return nil, ErrInvalidDateRange
		}
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	var totalEnergy float64
	var totalCost int64
	deviceEnergy := make(map[string]float64)
	deviceCost := make(map[string]int64)
	deviceRecords := make(map[string]int)
	dailyEnergy := make(map[string]float64)
	dailyCost := make(map[string]int64)

	for _, r := range e.records {
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}

		if req.DeviceID != "" && r.DeviceID != req.DeviceID {
			continue
		}

		totalEnergy += r.EnergyKWh
		totalCost += r.CostCents
		deviceEnergy[r.DeviceID] += r.EnergyKWh
		deviceCost[r.DeviceID] += r.CostCents
		deviceRecords[r.DeviceID]++

		dateKey := r.Timestamp.Format("2006-01-02")
		dailyEnergy[dateKey] += r.EnergyKWh
		dailyCost[dateKey] += r.CostCents
	}

	// 构建每日趋势
	var dailyTrend []TrendPoint
	for dateStr, energy := range dailyEnergy {
		t, _ := time.Parse("2006-01-02", dateStr)
		dailyTrend = append(dailyTrend, TrendPoint{
			Date:   t,
			Energy: energy,
			Cost:   dailyCost[dateStr],
		})
	}

	// 排序趋势数据
	sortTrendPoints(dailyTrend)

	// 构建设备列表
	var topDevices []*DevicePower
	for deviceID, energy := range deviceEnergy {
		dp, ok := e.devices[deviceID]
		if !ok {
			dp = &DevicePower{
				DeviceID:   deviceID,
				DeviceName: deviceID,
			}
		}
		dpCopy := *dp
		dpCopy.TotalEnergy = energy
		dpCopy.TotalCost = deviceCost[deviceID]
		dpCopy.RecordCount = deviceRecords[deviceID]
		if totalEnergy > 0 {
			dpCopy.UsagePercent = energy / totalEnergy * 100.0
		}
		topDevices = append(topDevices, &dpCopy)
	}

	// 排序设备
	sortDevicePowers(topDevices)

	days := end.Sub(start).Hours() / 24
	if days < 1 {
		days = 1
	}
	avgDailyCost := totalCost / int64(days)

	trend := e.analyzer.CalculateTrend(7)

	var prediction *Prediction
	if req.Period == PeriodMonthly {
		prediction = e.analyzer.PredictMonthly()
	}

	report := &PowerReport{
		ID:           uuid.New().String(),
		Period:       req.Period,
		StartTime:    start,
		EndTime:      end,
		TotalEnergy:  totalEnergy,
		TotalCost:    totalCost,
		AvgDailyCost: avgDailyCost,
		DailyTrend:   dailyTrend,
		TopDevices:   topDevices,
		Trend:        trend,
		Prediction:   prediction,
		GeneratedAt:  now,
	}

	// 计算预算使用率
	if e.budget != nil && e.budget.MonthlyAmount > 0 {
		report.BudgetUsed = float64(totalCost) / e.budget.MonthlyAmount * 100.0
		report.BudgetRemain = int64(e.budget.MonthlyAmount) - totalCost
		if report.BudgetRemain < 0 {
			report.BudgetRemain = 0
		}
	}

	return report, nil
}

// GetDeviceProfile 获取设备功耗画像.
func (e *Engine) GetDeviceProfile(deviceID string) (*DevicePower, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	dp, ok := e.devices[deviceID]
	if !ok {
		return nil, ErrDeviceNotFound
	}

	return dp, nil
}

// GetAllDeviceProfiles 获取所有设备功耗画像.
func (e *Engine) GetAllDeviceProfiles() []*DevicePower {
	e.mu.RLock()
	defer e.mu.RUnlock()

	profiles := make([]*DevicePower, 0, len(e.devices))
	for _, dp := range e.devices {
		profiles = append(profiles, dp)
	}

	sortDevicePowers(profiles)
	return profiles
}

// GetActiveAlerts 获取活跃告警.
func (e *Engine) GetActiveAlerts() []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.getActiveAlertsLocked()
}

// AcknowledgeAlert 确认告警.
func (e *Engine) AcknowledgeAlert(alertID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, alert := range e.alerts {
		if alert.ID == alertID {
			alert.Active = false
			now := time.Now()
			alert.ResolvedAt = &now
			e.logger.Info("告警已确认", zap.String("alert_id", alertID))
			return nil
		}
	}

	return ErrRecordNotFound
}

// ========== 内部方法 ==========

func (e *Engine) getActiveAlertsLocked() []*Alert {
	var active []*Alert
	for _, alert := range e.alerts {
		if alert.Active {
			active = append(active, alert)
		}
	}
	if active == nil {
		return make([]*Alert, 0)
	}
	return active
}

func (e *Engine) monitorLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			e.alertMgr.CheckBudgetAlerts()
			e.alertMgr.CheckAnomalyPower()
		case <-e.stopCh:
			return
		}
	}
}

// sortTrendPoints 按日期排序趋势点.
func sortTrendPoints(points []TrendPoint) {
	for i := 0; i < len(points)-1; i++ {
		for j := 0; j < len(points)-i-1; j++ {
			if points[j].Date.After(points[j+1].Date) {
				points[j], points[j+1] = points[j+1], points[j]
			}
		}
	}
}

// sortDevicePowers 按总能耗降序排序.
func sortDevicePowers(devices []*DevicePower) {
	for i := 0; i < len(devices)-1; i++ {
		for j := 0; j < len(devices)-i-1; j++ {
			if devices[j].TotalEnergy < devices[j+1].TotalEnergy {
				devices[j], devices[j+1] = devices[j+1], devices[j]
			}
		}
	}
}

// FormatCost 格式化成本（分转元）.
func FormatCost(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100.0)
}

// formatCost 格式化成本（内部使用，接受float64）.
func formatCost(cents float64) string {
	return fmt.Sprintf("%.2f", cents/100.0)
}
