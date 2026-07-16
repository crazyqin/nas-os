// Package smartpowerschedule 提供智能电源调度管理器
package smartpowerschedule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 电源调度管理器.
type Manager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	config    *PowerConfig
	upsList   map[string]*UPSInfo
	devices   map[string]*DevicePowerState
	schedules map[string]*PowerSchedule
	events    []*PowerEvent
	records   []*PowerUsageRecord
	stopChan  chan struct{}
	running   bool
}

// NewManager 创建电源调度管理器.
func NewManager(logger *zap.Logger, config *PowerConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultPowerConfig()
	}

	return &Manager{
		logger:    logger,
		config:    config,
		upsList:   make(map[string]*UPSInfo),
		devices:   make(map[string]*DevicePowerState),
		schedules: make(map[string]*PowerSchedule),
		events:    make([]*PowerEvent, 0),
		records:   make([]*PowerUsageRecord, 0),
		stopChan:  make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("manager already running")
	}

	if !m.config.Enabled {
		return fmt.Errorf("power manager is disabled")
	}

	m.running = true
	m.stopChan = make(chan struct{})

	// 启动 UPS 监控
	if m.config.UPSMonitorEnabled {
		go m.monitorUPS(ctx)
	}

	// 启动调度执行器
	go m.runScheduler(ctx)

	m.logger.Info("power manager started",
		zap.Bool("ups_monitor", m.config.UPSMonitorEnabled),
		zap.Bool("tou_enabled", m.config.TOUConfig != nil && m.config.TOUConfig.Enabled))

	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("manager not running")
	}

	close(m.stopChan)
	m.running = false

	m.logger.Info("power manager stopped")
	return nil
}

// IsRunning 检查是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// generateID 生成唯一 ID.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// monitorUPS UPS 监控协程.
func (m *Manager) monitorUPS(ctx context.Context) {
	interval := time.Duration(m.config.UPSMonitorIntervalSec) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkUPSStatus()
		}
	}
}

// checkUPSStatus 检查 UPS 状态.
func (m *Manager) checkUPSStatus() {
	m.mu.RLock()
	upsList := make([]*UPSInfo, 0, len(m.upsList))
	for _, ups := range m.upsList {
		upsList = append(upsList, ups)
	}
	m.mu.RUnlock()

	for _, ups := range upsList {
		// 检查电池低电量
		if ups.BatteryLevel <= m.config.BatteryCriticalThreshold {
			m.addEvent(&PowerEvent{
				ID:           generateID(),
				EventType:    "battery_critical",
				Source:       PowerSourceUPS,
				Message:      fmt.Sprintf("UPS %s 电池电量危急: %.1f%%", ups.Name, ups.BatteryLevel),
				Severity:     PowerStatusCritical,
				BatteryLevel: ups.BatteryLevel,
				Timestamp:    time.Now(),
			})

			if m.config.ShutdownOnBatteryLow {
				m.logger.Warn("battery critical, initiating shutdown",
					zap.String("ups", ups.Name),
					zap.Float64("level", ups.BatteryLevel))
			}
		} else if ups.BatteryLevel <= m.config.BatteryLowThreshold {
			m.addEvent(&PowerEvent{
				ID:           generateID(),
				EventType:    "battery_low",
				Source:       PowerSourceUPS,
				Message:      fmt.Sprintf("UPS %s 电池电量低: %.1f%%", ups.Name, ups.BatteryLevel),
				Severity:     PowerStatusWarning,
				BatteryLevel: ups.BatteryLevel,
				Timestamp:    time.Now(),
			})
		}

		// 检查负载
		if ups.LoadPercent > 90 {
			m.addEvent(&PowerEvent{
				ID:        generateID(),
				EventType: "high_load",
				Source:    PowerSourceUPS,
				Message:   fmt.Sprintf("UPS %s 负载过高: %.1f%%", ups.Name, ups.LoadPercent),
				Severity:  PowerStatusWarning,
				Timestamp: time.Now(),
			})
		}
	}
}

// runScheduler 调度执行器.
func (m *Manager) runScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.executeSchedules()
		}
	}
}

// executeSchedules 执行调度计划.
func (m *Manager) executeSchedules() {
	m.mu.RLock()
	now := time.Now()
	currentPeriod := m.getCurrentTOUPeriod(now)

	schedules := make([]*PowerSchedule, 0)
	for _, s := range m.schedules {
		if !s.IsActive {
			continue
		}
		if s.TimePeriod != "" && s.TimePeriod != currentPeriod {
			continue
		}
		if !m.isScheduleActiveNow(s, now) {
			continue
		}
		schedules = append(schedules, s)
	}
	m.mu.RUnlock()

	for _, s := range schedules {
		m.logger.Debug("executing schedule",
			zap.String("id", s.ID),
			zap.String("name", s.Name),
			zap.String("action", string(s.Action)))

		for _, deviceID := range s.DeviceIDs {
			m.executeDeviceAction(deviceID, s.Action)
		}
	}
}

// getCurrentTOUPeriod 获取当前峰谷时段.
func (m *Manager) getCurrentTOUPeriod(t time.Time) TimeOfUsePeriod {
	if m.config.TOUConfig == nil || !m.config.TOUConfig.Enabled {
		return TOUShoulder
	}

	hourMin := t.Format("15:04")
	for _, p := range m.config.TOUConfig.Periods {
		if hourMin >= p.StartTime && hourMin < p.EndTime {
			return p.Period
		}
	}
	return TOUShoulder
}

// isScheduleActiveNow 检查调度计划是否在当前时间激活.
func (m *Manager) isScheduleActiveNow(s *PowerSchedule, now time.Time) bool {
	// 检查星期几
	if len(s.DaysOfWeek) > 0 {
		weekday := int(now.Weekday())
		found := false
		for _, d := range s.DaysOfWeek {
			if d == weekday {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查时间段
	currentTime := now.Format("15:04")
	return currentTime >= s.StartTime && currentTime < s.EndTime
}

// executeDeviceAction 执行设备动作.
func (m *Manager) executeDeviceAction(deviceID string, action ScheduleAction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[deviceID]
	if !ok {
		m.logger.Warn("device not found for schedule", zap.String("device_id", deviceID))
		return
	}

	switch action {
	case ActionPowerOn:
		device.IsPoweredOn = true
	case ActionPowerOff:
		device.IsPoweredOn = false
	case ActionReducePower:
		device.PowerUsageW *= 0.7 // 降低30%
	}

	device.LastChanged = time.Now()
}

// addEvent 添加事件.
func (m *Manager) addEvent(event *PowerEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.events = append(m.events, event)

	// 限制事件数量
	if len(m.events) > 1000 {
		m.events = m.events[len(m.events)-1000:]
	}

	m.logger.Info("power event",
		zap.String("type", event.EventType),
		zap.String("message", event.Message))
}

// RegisterUPS 注册 UPS.
func (m *Manager) RegisterUPS(ups *UPSInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ups.ID == "" {
		ups.ID = generateID()
	}
	ups.LastUpdated = time.Now()

	m.upsList[ups.ID] = ups
	m.logger.Info("UPS registered", zap.String("id", ups.ID), zap.String("name", ups.Name))
}

// GetUPS 获取 UPS 信息.
func (m *Manager) GetUPS(id string) (*UPSInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ups, ok := m.upsList[id]
	if !ok {
		return nil, fmt.Errorf("UPS not found: %s", id)
	}
	return ups, nil
}

// ListUPS 列出所有 UPS.
func (m *Manager) ListUPS() []*UPSInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*UPSInfo, 0, len(m.upsList))
	for _, u := range m.upsList {
		list = append(list, u)
	}
	return list
}

// UpdateUPSStatus 更新 UPS 状态.
func (m *Manager) UpdateUPSStatus(id string, status *UPSInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ups, ok := m.upsList[id]
	if !ok {
		return fmt.Errorf("UPS not found: %s", id)
	}

	ups.Status = status.Status
	ups.BatteryLevel = status.BatteryLevel
	ups.BatteryStatus = status.BatteryStatus
	ups.LoadPercent = status.LoadPercent
	ups.InputVoltageV = status.InputVoltageV
	ups.OutputVoltageV = status.OutputVoltageV
	ups.EstimatedRuntimeMin = status.EstimatedRuntimeMin
	ups.LastUpdated = time.Now()

	return nil
}

// RegisterDevice 注册设备.
func (m *Manager) RegisterDevice(device *DevicePowerState) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.DeviceID == "" {
		device.DeviceID = generateID()
	}
	device.LastChanged = time.Now()

	m.devices[device.DeviceID] = device
}

// GetDevice 获取设备信息.
func (m *Manager) GetDevice(id string) (*DevicePowerState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("device not found: %s", id)
	}
	return device, nil
}

// ListDevices 列出所有设备.
func (m *Manager) ListDevices() []*DevicePowerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*DevicePowerState, 0, len(m.devices))
	for _, d := range m.devices {
		list = append(list, d)
	}
	return list
}

// CreateSchedule 创建调度计划.
func (m *Manager) CreateSchedule(req *PowerScheduleRequest) (*PowerSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	schedule := &PowerSchedule{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		IsActive:    true,
		DeviceIDs:   req.DeviceIDs,
		Action:      req.Action,
		TimePeriod:  req.TimePeriod,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		DaysOfWeek:  req.DaysOfWeek,
		Priority:    req.Priority,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.schedules[schedule.ID] = schedule

	m.logger.Info("schedule created",
		zap.String("id", schedule.ID),
		zap.String("name", schedule.Name))

	return schedule, nil
}

// GetSchedule 获取调度计划.
func (m *Manager) GetSchedule(id string) (*PowerSchedule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}
	return s, nil
}

// ListSchedules 列出所有调度计划.
func (m *Manager) ListSchedules() []*PowerSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*PowerSchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		list = append(list, s)
	}
	return list
}

// UpdateSchedule 更新调度计划.
func (m *Manager) UpdateSchedule(id string, req *PowerScheduleRequest) (*PowerSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	s.Name = req.Name
	s.Description = req.Description
	s.DeviceIDs = req.DeviceIDs
	s.Action = req.Action
	s.TimePeriod = req.TimePeriod
	s.StartTime = req.StartTime
	s.EndTime = req.EndTime
	s.DaysOfWeek = req.DaysOfWeek
	s.Priority = req.Priority
	s.UpdatedAt = time.Now()

	return s, nil
}

// DeleteSchedule 删除调度计划.
func (m *Manager) DeleteSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.schedules[id]; !ok {
		return fmt.Errorf("schedule not found: %s", id)
	}
	delete(m.schedules, id)
	return nil
}

// ToggleSchedule 切换调度计划状态.
func (m *Manager) ToggleSchedule(id string) (*PowerSchedule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule not found: %s", id)
	}

	s.IsActive = !s.IsActive
	s.UpdatedAt = time.Now()

	return s, nil
}

// GetCurrentTOUPeriod 获取当前峰谷时段.
func (m *Manager) GetCurrentTOUPeriod() TimeOfUsePeriod {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.getCurrentTOUPeriod(time.Now())
}

// GetTOURate 获取当前电价.
func (m *Manager) GetTOURate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config.TOUConfig == nil || !m.config.TOUConfig.Enabled {
		return 0
	}

	period := m.getCurrentTOUPeriod(time.Now())
	switch period {
	case TOUPeak:
		return m.config.TOUConfig.PeakRate
	case TOUOffPeak:
		return m.config.TOUConfig.OffPeakRate
	case TOUSuperOff:
		return m.config.TOUConfig.SuperOffRate
	default:
		return m.config.TOUConfig.ShoulderRate
	}
}

// GetEvents 获取电源事件.
func (m *Manager) GetEvents(limit int) []*PowerEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.events) {
		limit = len(m.events)
	}

	start := len(m.events) - limit
	if start < 0 {
		start = 0
	}

	events := make([]*PowerEvent, limit)
	copy(events, m.events[start:])
	return events
}

// GetCostSummary 获取费用汇总.
func (m *Manager) GetCostSummary(period string) *CostSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &CostSummary{
		Period:   period,
		Currency: "CNY",
		ByPeriod: make(map[TimeOfUsePeriod]float64),
		ByDevice: make(map[string]float64),
	}

	if m.config.TOUConfig != nil {
		summary.Currency = m.config.TOUConfig.Currency
	}

	for _, r := range m.records {
		summary.TotalEnergyKWh += r.EnergyWh / 1000
		summary.TotalCost += r.CostAmount
		summary.ByPeriod[r.TimePeriod] += r.EnergyWh / 1000
		summary.ByDevice[r.DeviceID] += r.EnergyWh / 1000
	}

	return summary
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *PowerConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *PowerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalPower := 0.0
	for _, d := range m.devices {
		if d.IsPoweredOn {
			totalPower += d.PowerUsageW
		}
	}

	return map[string]interface{}{
		"running":        m.running,
		"ups_count":      len(m.upsList),
		"device_count":   len(m.devices),
		"schedule_count": len(m.schedules),
		"event_count":    len(m.events),
		"total_power_w":  totalPower,
		"current_period": string(m.getCurrentTOUPeriod(time.Now())),
		"current_rate":   m.GetTOURate(),
	}
}
