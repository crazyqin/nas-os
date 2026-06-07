// Package naspowerbudget 提供NAS功率预算管理
// 监控和优化NAS系统功耗，提供功率预算规划和节能建议
package naspowerbudget

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrDeviceNotFound   = errors.New("device not found")
	ErrBudgetExceeded   = errors.New("power budget exceeded")
	ErrInvalidConfig    = errors.New("invalid configuration")
	ErrInsufficientData = errors.New("insufficient data")
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceCPU     DeviceType = "cpu"
	DeviceGPU     DeviceType = "gpu"
	DeviceDisk    DeviceType = "disk"
	DeviceMemory  DeviceType = "memory"
	DeviceNetwork DeviceType = "network"
	DeviceFan     DeviceType = "fan"
	DevicePSU     DeviceType = "psu"
	DeviceOther   DeviceType = "other"
)

// PowerState 功率状态
type PowerState string

const (
	PowerStateActive  PowerState = "active"
	PowerStateIdle    PowerState = "idle"
	PowerStateStandby PowerState = "standby"
	PowerStateSleep   PowerState = "sleep"
	PowerStateOff     PowerState = "off"
)

// PowerReading 功率读数
type PowerReading struct {
	Timestamp   time.Time  `json:"timestamp"`
	DeviceID    string     `json:"deviceId"`
	DeviceType  DeviceType `json:"deviceType"`
	Watts       float64    `json:"watts"`
	Voltage     float64    `json:"voltage"`
	Current     float64    `json:"current"`
	Temperature float64    `json:"temperature"`
	State       PowerState `json:"state"`
}

// DeviceProfile 设备功率画像
type DeviceProfile struct {
	DeviceID      string         `json:"deviceId"`
	DeviceName    string         `json:"deviceName"`
	DeviceType    DeviceType     `json:"deviceType"`
	MaxPowerWatts float64        `json:"maxPowerWatts"`
	TypicalWatts  float64        `json:"typicalWatts"`
	IdleWatts     float64        `json:"idleWatts"`
	StandbyWatts  float64        `json:"standbyWatts"`
	CurrentWatts  float64        `json:"currentWatts"`
	CurrentState  PowerState     `json:"currentState"`
	Readings      []PowerReading `json:"-"`
	LastUpdated   time.Time      `json:"lastUpdated"`
}

// PowerBudget 功率预算
type PowerBudget struct {
	BudgetID       string   `json:"budgetId"`
	Name           string   `json:"name"`
	MaxWatts       float64  `json:"maxWatts"`
	CurrentWatts   float64  `json:"currentWatts"`
	Utilization    float64  `json:"utilization"`
	WarningWatts   float64  `json:"warningWatts"`
	CriticalWatts  float64  `json:"criticalWatts"`
	DailyBudgetKWh float64  `json:"dailyBudgetKWh"`
	DailyActualKWh float64  `json:"dailyActualKWh"`
	MonthlyCostEst float64  `json:"monthlyCostEst"`
	Devices        []string `json:"devices"`
}

// EnergyReport 能耗报告
type EnergyReport struct {
	GeneratedAt     time.Time              `json:"generatedAt"`
	Period          string                 `json:"period"`
	TotalEnergyKWh  float64                `json:"totalEnergyKWh"`
	TotalCost       float64                `json:"totalCost"`
	AvgPowerWatts   float64                `json:"avgPowerWatts"`
	PeakPowerWatts  float64                `json:"peakPowerWatts"`
	DeviceBreakdown map[DeviceType]float64 `json:"deviceBreakdown"`
	CostBreakdown   map[DeviceType]float64 `json:"costBreakdown"`
	CarbonFootprint float64                `json:"carbonFootprint"`
	Suggestions     []PowerSuggestion      `json:"suggestions"`
}

// PowerSuggestion 节能建议
type PowerSuggestion struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	SavingsKWh  float64 `json:"savingsKWh"`
	SavingsCost float64 `json:"savingsCost"`
	Priority    int     `json:"priority"`
	Category    string  `json:"category"`
}

// ScheduleRule 调度规则
type ScheduleRule struct {
	RuleID      string     `json:"ruleId"`
	Name        string     `json:"name"`
	DeviceIDs   []string   `json:"deviceIds"`
	TargetState PowerState `json:"targetState"`
	StartTime   string     `json:"startTime"` // HH:MM format
	EndTime     string     `json:"endTime"`
	DaysOfWeek  []int      `json:"daysOfWeek"` // 0=Sunday
	Enabled     bool       `json:"enabled"`
}

// Manager 功率预算管理器
type Manager struct {
	mu           sync.RWMutex
	config       *Config
	devices      map[string]*DeviceProfile
	budgets      map[string]*PowerBudget
	readings     []PowerReading
	schedules    []ScheduleRule
	running      bool
	stopCh       chan struct{}
	nowFunc      func() time.Time
	readingCount int64
}

// Config 配置
type Config struct {
	Enabled           bool          `json:"enabled"`
	ElectricityRate   float64       `json:"electricityRate"` // 电价（元/kWh）
	CarbonFactor      float64       `json:"carbonFactor"`    // 碳排放因子（kg CO2/kWh）
	SamplingInterval  time.Duration `json:"samplingInterval"`
	ReadingRetention  time.Duration `json:"readingRetention"`
	WarningThreshold  float64       `json:"warningThreshold"` // 预算使用百分比
	CriticalThreshold float64       `json:"criticalThreshold"`
}

// NewManager 创建管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{
			Enabled:           true,
			ElectricityRate:   0.56, // 0.56元/kWh
			CarbonFactor:      0.57, // 0.57 kg CO2/kWh
			SamplingInterval:  time.Minute,
			ReadingRetention:  time.Hour * 24 * 30, // 30天
			WarningThreshold:  80,
			CriticalThreshold: 95,
		}
	}
	return &Manager{
		config:    config,
		devices:   make(map[string]*DeviceProfile),
		budgets:   make(map[string]*PowerBudget),
		readings:  make([]PowerReading, 0),
		schedules: make([]ScheduleRule, 0),
		stopCh:    make(chan struct{}),
		nowFunc:   time.Now,
	}
}

// RegisterDevice 注册设备
func (m *Manager) RegisterDevice(id, name string, deviceType DeviceType, maxWatts, typicalWatts, idleWatts, standbyWatts float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		return fmt.Errorf("device id is required")
	}

	m.devices[id] = &DeviceProfile{
		DeviceID:      id,
		DeviceName:    name,
		DeviceType:    deviceType,
		MaxPowerWatts: maxWatts,
		TypicalWatts:  typicalWatts,
		IdleWatts:     idleWatts,
		StandbyWatts:  standbyWatts,
		CurrentWatts:  typicalWatts,
		CurrentState:  PowerStateActive,
		Readings:      make([]PowerReading, 0),
		LastUpdated:   m.nowFunc(),
	}
	return nil
}

// RecordReading 记录功率读数
func (m *Manager) RecordReading(reading PowerReading) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[reading.DeviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	if reading.Timestamp.IsZero() {
		reading.Timestamp = m.nowFunc()
	}

	device.CurrentWatts = reading.Watts
	device.CurrentState = reading.State
	device.LastUpdated = m.nowFunc()
	device.Readings = append(device.Readings, reading)

	m.readings = append(m.readings, reading)
	m.readingCount++

	// 清理过期读数
	cutoff := m.nowFunc().Add(-m.config.ReadingRetention)
	validReadings := make([]PowerReading, 0)
	for _, r := range m.readings {
		if r.Timestamp.After(cutoff) {
			validReadings = append(validReadings, r)
		}
	}
	m.readings = validReadings

	return nil
}

// CreateBudget 创建功率预算
func (m *Manager) CreateBudget(id, name string, maxWatts float64, deviceIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if maxWatts <= 0 {
		return ErrInvalidConfig
	}

	m.budgets[id] = &PowerBudget{
		BudgetID:       id,
		Name:           name,
		MaxWatts:       maxWatts,
		WarningWatts:   maxWatts * m.config.WarningThreshold / 100,
		CriticalWatts:  maxWatts * m.config.CriticalThreshold / 100,
		DailyBudgetKWh: maxWatts * 24 / 1000,
		Devices:        deviceIDs,
	}
	return nil
}

// GetBudgetStatus 获取预算状态
func (m *Manager) GetBudgetStatus(budgetID string) (*PowerBudget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	budget, ok := m.budgets[budgetID]
	if !ok {
		return nil, ErrDeviceNotFound
	}

	// 计算当前总功率
	currentWatts := 0.0
	for _, devID := range budget.Devices {
		if dev, ok := m.devices[devID]; ok {
			currentWatts += dev.CurrentWatts
		}
	}

	result := *budget
	result.CurrentWatts = currentWatts
	result.Utilization = (currentWatts / budget.MaxWatts) * 100

	// 计算今日能耗
	hoursElapsed := float64(m.nowFunc().Hour()) + float64(m.nowFunc().Minute())/60
	result.DailyActualKWh = currentWatts * hoursElapsed / 1000
	result.MonthlyCostEst = result.DailyActualKWh * 30 * m.config.ElectricityRate

	return &result, nil
}

// GenerateReport 生成能耗报告
func (m *Manager) GenerateReport(period string) (*EnergyReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.readings) == 0 {
		return nil, ErrInsufficientData
	}

	report := &EnergyReport{
		GeneratedAt:     m.nowFunc(),
		Period:          period,
		DeviceBreakdown: make(map[DeviceType]float64),
		CostBreakdown:   make(map[DeviceType]float64),
	}

	// 按设备类型统计能耗
	deviceEnergy := make(map[DeviceType]float64)
	deviceReadings := make(map[DeviceType]int)

	for _, reading := range m.readings {
		deviceEnergy[reading.DeviceType] += reading.Watts
		deviceReadings[reading.DeviceType]++
	}

	totalEnergy := 0.0
	for dtype, totalWatts := range deviceEnergy {
		count := deviceReadings[dtype]
		if count == 0 {
			continue
		}
		avgWatts := totalWatts / float64(count)
		// 假设采样间隔为1分钟，转换为kWh
		energyKWh := avgWatts * float64(count) / 60 / 1000
		report.DeviceBreakdown[dtype] = energyKWh
		report.CostBreakdown[dtype] = energyKWh * m.config.ElectricityRate
		totalEnergy += energyKWh
	}

	report.TotalEnergyKWh = totalEnergy
	report.TotalCost = totalEnergy * m.config.ElectricityRate
	report.CarbonFootprint = totalEnergy * m.config.CarbonFactor

	// 计算平均和峰值功率
	totalWatts := 0.0
	peakWatts := 0.0
	for _, reading := range m.readings {
		totalWatts += reading.Watts
		if reading.Watts > peakWatts {
			peakWatts = reading.Watts
		}
	}
	if len(m.readings) > 0 {
		report.AvgPowerWatts = totalWatts / float64(len(m.readings))
	}
	report.PeakPowerWatts = peakWatts

	// 生成节能建议
	report.Suggestions = m.generateSuggestions()

	return report, nil
}

// AddSchedule 添加调度规则
func (m *Manager) AddSchedule(rule ScheduleRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.RuleID == "" {
		return fmt.Errorf("rule id is required")
	}

	m.schedules = append(m.schedules, rule)
	return nil
}

// GetScheduleStatus 获取调度状态
func (m *Manager) GetScheduleStatus() []ScheduleRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ScheduleRule, len(m.schedules))
	copy(result, m.schedules)
	return result
}

// GetDevice 获取设备信息
func (m *Manager) GetDevice(deviceID string) (*DeviceProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return nil, ErrDeviceNotFound
	}

	copy := *device
	copy.Readings = nil // 不暴露原始读数
	return &copy, nil
}

// GetAllDevices 获取所有设备
func (m *Manager) GetAllDevices() []*DeviceProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DeviceProfile, 0, len(m.devices))
	for _, d := range m.devices {
		copy := *d
		copy.Readings = nil
		result = append(result, &copy)
	}
	return result
}

// GetDashboard 获取仪表板
func (m *Manager) GetDashboard() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCurrentWatts := 0.0
	totalMaxWatts := 0.0
	deviceCount := len(m.devices)

	for _, d := range m.devices {
		totalCurrentWatts += d.CurrentWatts
		totalMaxWatts += d.MaxPowerWatts
	}

	utilization := 0.0
	if totalMaxWatts > 0 {
		utilization = (totalCurrentWatts / totalMaxWatts) * 100
	}

	monthlyCost := totalCurrentWatts * 24 * 30 / 1000 * m.config.ElectricityRate

	return map[string]interface{}{
		"deviceCount":       deviceCount,
		"totalCurrentWatts": totalCurrentWatts,
		"totalMaxWatts":     totalMaxWatts,
		"utilization":       utilization,
		"monthlyCostEst":    monthlyCost,
		"budgetCount":       len(m.budgets),
		"scheduleCount":     len(m.schedules),
		"readingCount":      m.readingCount,
		"electricityRate":   m.config.ElectricityRate,
	}
}

// EstimateSavings 估算节能效果
func (m *Manager) EstimateSavings(deviceID string, targetState PowerState) (float64, float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[deviceID]
	if !ok {
		return 0, 0, ErrDeviceNotFound
	}

	var targetWatts float64
	switch targetState {
	case PowerStateActive:
		targetWatts = device.TypicalWatts
	case PowerStateIdle:
		targetWatts = device.IdleWatts
	case PowerStateStandby:
		targetWatts = device.StandbyWatts
	case PowerStateSleep, PowerStateOff:
		targetWatts = 0
	default:
		targetWatts = device.CurrentWatts
	}

	savingsWatts := device.CurrentWatts - targetWatts
	if savingsWatts < 0 {
		savingsWatts = 0
	}

	// 每月节省（kWh）
	monthlySavingsKWh := savingsWatts * 24 * 30 / 1000
	monthlySavingsCost := monthlySavingsKWh * m.config.ElectricityRate

	return monthlySavingsKWh, monthlySavingsCost, nil
}

// 内部方法

func (m *Manager) generateSuggestions() []PowerSuggestion {
	suggestions := make([]PowerSuggestion, 0)

	// 检查各设备是否可以优化
	for _, device := range m.devices {
		// 空闲设备建议
		if device.CurrentState == PowerStateActive && device.IdleWatts < device.CurrentWatts*0.5 {
			savingsKWh := (device.CurrentWatts - device.IdleWatts) * 24 / 1000
			suggestions = append(suggestions, PowerSuggestion{
				ID:          fmt.Sprintf("idle-%s", device.DeviceID),
				Title:       fmt.Sprintf("将 %s 切换到空闲模式", device.DeviceName),
				Description: fmt.Sprintf("当前功耗 %.1fW，空闲模式 %.1fW", device.CurrentWatts, device.IdleWatts),
				SavingsKWh:  savingsKWh,
				SavingsCost: savingsKWh * m.config.ElectricityRate,
				Priority:    3,
				Category:    "idle_mode",
			})
		}

		// 待机建议
		if device.CurrentWatts > device.StandbyWatts*2 && device.StandbyWatts > 0 {
			savingsKWh := (device.CurrentWatts - device.StandbyWatts) * 8 / 1000 // 假设8小时待机
			suggestions = append(suggestions, PowerSuggestion{
				ID:          fmt.Sprintf("standby-%s", device.DeviceID),
				Title:       fmt.Sprintf("夜间将 %s 切换到待机", device.DeviceName),
				Description: fmt.Sprintf("每日可节省 %.2f kWh", savingsKWh),
				SavingsKWh:  savingsKWh,
				SavingsCost: savingsKWh * m.config.ElectricityRate,
				Priority:    2,
				Category:    "standby",
			})
		}
	}

	// 通用建议
	suggestions = append(suggestions, PowerSuggestion{
		ID:          "schedule-power",
		Title:       "设置定时开关机计划",
		Description: "根据使用习惯设置定时开关机，避免不必要的功耗",
		SavingsKWh:  5.0,
		SavingsCost: 5.0 * m.config.ElectricityRate,
		Priority:    4,
		Category:    "scheduling",
	})

	suggestions = append(suggestions, PowerSuggestion{
		ID:          "disk-hibernate",
		Title:       "启用磁盘休眠",
		Description: "不常用的磁盘自动进入休眠状态，降低功耗和噪音",
		SavingsKWh:  3.0,
		SavingsCost: 3.0 * m.config.ElectricityRate,
		Priority:    3,
		Category:    "disk_power",
	})

	return suggestions
}
