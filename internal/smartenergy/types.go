// Package smartenergy 提供智能能源管理功能
// 包括功耗监控、节能策略、碳排放追踪、能源优化建议
package smartenergy

import (
	"errors"
	"sync"
	"time"
)

// ========== 电源状态 ==========

// PowerState 电源状态
type PowerState string

const (
	PowerOn      PowerState = "on"       // 正常运行
	PowerSleep   PowerState = "sleep"    // 睡眠
	PowerHibernate PowerState = "hibernate" // 休眠
	PowerStandby PowerState = "standby"  // 待机
	PowerOff     PowerState = "off"      // 关机
)

// ========== 能源模式 ==========

// EnergyMode 能源模式
type EnergyMode string

const (
	ModePerformance EnergyMode = "performance" // 性能优先
	ModeBalanced    EnergyMode = "balanced"    // 均衡模式
	ModePowersave   EnergyMode = "powersave"   // 节能模式
	ModeCustom      EnergyMode = "custom"      // 自定义
	ModeScheduled   EnergyMode = "scheduled"   // 定时模式
)

// ========== 功耗记录 ==========

// PowerRecord 功耗记录
type PowerRecord struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id"`
	PowerW    float64   `json:"power_w"`    // 当前功耗（瓦特）
	Voltage   float64   `json:"voltage"`    // 电压
	Current   float64   `json:"current"`    // 电流
	TempCPU   float64   `json:"temp_cpu"`   // CPU温度
	TempDisk  float64   `json:"temp_disk"`  // 磁盘温度
	FanRPM    int       `json:"fan_rpm"`    // 风扇转速
	State     PowerState `json:"state"`
	Timestamp time.Time `json:"timestamp"`
}

// ========== 节能策略 ==========

// EnergyPolicy 节能策略
type EnergyPolicy struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Mode          EnergyMode `json:"mode"`
	Enabled       bool       `json:"enabled"`
	Rules         []PolicyRule `json:"rules"`
	Schedule      *Schedule  `json:"schedule,omitempty"`
	Priority      int        `json:"priority"`
	CreatedAt     time.Time  `json:"created_at"`
}

// PolicyRule 策略规则
type PolicyRule struct {
	Name      string      `json:"name"`
	Condition string      `json:"condition"` // cpu_temp>70, power>100, time=22:00-06:00
	Action    string      `json:"action"`    // reduce_freq, spin_down, sleep, notify
	Value     interface{} `json:"value"`
	Enabled   bool        `json:"enabled"`
}

// Schedule 定时计划
type Schedule struct {
	StartTime string `json:"start_time"` // HH:MM
	EndTime   string `json:"end_time"`   // HH:MM
	Days      []int  `json:"days"`       // 0-6 (周日-周六)
	Timezone  string `json:"timezone"`
}

// ========== 碳排放 ==========

// CarbonRecord 碳排放记录
type CarbonRecord struct {
	ID          string    `json:"id"`
	Date        string    `json:"date"`        // YYYY-MM-DD
	EnergyKWh   float64   `json:"energy_kwh"`  // 用电量（千瓦时）
	CarbonKg    float64   `json:"carbon_kg"`   // 碳排放（千克CO2）
	Factor      float64   `json:"factor"`      // 碳排放因子（kgCO2/kWh）
	Source      string    `json:"source"`      // 数据来源
	Timestamp   time.Time `json:"timestamp"`
}

// CarbonSummary 碳排放汇总
type CarbonSummary struct {
	Period      string  `json:"period"`
	TotalKWh    float64 `json:"total_kwh"`
	TotalKg     float64 `json:"total_kg"`
	AvgDaily    float64 `json:"avg_daily"`
	Trend       string  `json:"trend"`       // increasing, decreasing, stable
	TreesNeeded float64 `json:"trees_needed"` // 需要种植的树木数量（年吸收量）
}

// ========== 能源建议 ==========

// EnergySuggestion 能源优化建议
type EnergySuggestion struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"`       // disk_hibernate, cpu_freq, fan_curve, schedule
	Priority    string  `json:"priority"`   // high, medium, low
	Title       string  `json:"title"`
	Description string  `json:"description"`
	EstSaving   float64 `json:"est_saving_kwh"` // 预计节省（kWh/月）
	EstCost     float64 `json:"est_cost_saving"` // 预计节省（元/月）
	Confidence  float64 `json:"confidence"`
	Applied     bool    `json:"applied"`
}

// ========== 设备配置 ==========

// DeviceConfig 设备能源配置
type DeviceConfig struct {
	DeviceID         string     `json:"device_id"`
	Name             string     `json:"name"`
	Type             string     `json:"type"`     // nas, switch, ups, router
	MaxPowerW        float64    `json:"max_power_w"`
	IdlePowerW       float64    `json:"idle_power_w"`
	CurrentMode      EnergyMode `json:"current_mode"`
	DiskHibernateMin int        `json:"disk_hibernate_min"` // 磁盘休眠时间（分钟）
	CPUGovernor      string     `json:"cpu_governor"`        // powersave, performance, ondemand
	FanProfile       string     `json:"fan_profile"`         // silent, balanced, performance
	LEDEnabled       bool       `json:"led_enabled"`
	WakeOnLAN        bool       `json:"wake_on_lan"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ========== 能源引擎 ==========

// EnergyEngine 能源管理引擎
type EnergyEngine struct {
	mu          sync.RWMutex
	devices     map[string]*DeviceConfig
	records     map[string][]*PowerRecord
	policies    map[string]*EnergyPolicy
	carbon      map[string][]*CarbonRecord
	suggestions []EnergySuggestion
	currentMode EnergyMode
	totalEnergy float64 // 累计用电量（kWh）
}

// EngineOption 引擎配置选项
type EngineOption func(*EnergyEngine)

// WithDefaultMode 设置默认能源模式
func WithDefaultMode(mode EnergyMode) EngineOption {
	return func(e *EnergyEngine) {
		e.currentMode = mode
	}
}

// NewEnergyEngine 创建能源管理引擎
func NewEnergyEngine(opts ...EngineOption) *EnergyEngine {
	e := &EnergyEngine{
		devices:     make(map[string]*DeviceConfig),
		records:     make(map[string][]*PowerRecord),
		policies:    make(map[string]*EnergyPolicy),
		carbon:      make(map[string][]*CarbonRecord),
		currentMode: ModeBalanced,
	}
	for _, opt := range opts {
		opt(e)
	}
	e.initDefaultPolicies()
	return e
}

// ========== 设备管理 ==========

// RegisterDevice 注册设备
func (e *EnergyEngine) RegisterDevice(config *DeviceConfig) error {
	if config.DeviceID == "" {
		return errors.New("device ID cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	e.devices[config.DeviceID] = config
	return nil
}

// GetDevice 获取设备配置
func (e *EnergyEngine) GetDevice(deviceID string) (*DeviceConfig, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	device, ok := e.devices[deviceID]
	if !ok {
		return nil, errors.New("device not found")
	}
	return device, nil
}

// ListDevices 列出所有设备
func (e *EnergyEngine) ListDevices() []*DeviceConfig {
	e.mu.RLock()
	defer e.mu.RUnlock()
	devices := make([]*DeviceConfig, 0, len(e.devices))
	for _, d := range e.devices {
		devices = append(devices, d)
	}
	return devices
}

// ========== 功耗记录 ==========

// RecordPower 记录功耗
func (e *EnergyEngine) RecordPower(record *PowerRecord) error {
	if record.DeviceID == "" {
		return errors.New("device ID cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	record.Timestamp = time.Now()
	e.records[record.DeviceID] = append(e.records[record.DeviceID], record)
	e.totalEnergy += record.PowerW / 1000.0 / 3600.0 // 转换为kWh（假设1秒间隔）
	return nil
}

// GetPowerHistory 获取功耗历史
func (e *EnergyEngine) GetPowerHistory(deviceID string, limit int) []*PowerRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()
	records := e.records[deviceID]
	if len(records) <= limit {
		return records
	}
	return records[len(records)-limit:]
}

// GetCurrentPower 获取当前总功耗
func (e *EnergyEngine) GetCurrentPower() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var total float64
	for _, records := range e.records {
		if len(records) > 0 {
			total += records[len(records)-1].PowerW
		}
	}
	return total
}

// ========== 能源模式 ==========

// SetMode 设置能源模式
func (e *EnergyEngine) SetMode(mode EnergyMode) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentMode = mode
	return nil
}

// GetMode 获取当前能源模式
func (e *EnergyEngine) GetMode() EnergyMode {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentMode
}

// ========== 碳排放追踪 ==========

// RecordCarbon 记录碳排放
func (e *EnergyEngine) RecordCarbon(record *CarbonRecord) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	record.Timestamp = time.Now()
	if record.Factor == 0 {
		record.Factor = 0.581 // 中国电网平均碳排放因子
	}
	record.CarbonKg = record.EnergyKWh * record.Factor
	e.carbon[record.Date] = append(e.carbon[record.Date], record)
	return nil
}

// GetCarbonSummary 获取碳排放汇总
func (e *EnergyEngine) GetCarbonSummary(period string) *CarbonSummary {
	e.mu.RLock()
	defer e.mu.RUnlock()

	summary := &CarbonSummary{Period: period}
	for _, records := range e.carbon {
		for _, r := range records {
			summary.TotalKWh += r.EnergyKWh
			summary.TotalKg += r.CarbonKg
		}
	}
	if summary.TotalKg > 0 {
		summary.TreesNeeded = summary.TotalKg / 21.77 // 一棵树年均吸收约21.77kg CO2
	}
	return summary
}

// ========== 节能策略 ==========

// AddPolicy 添加节能策略
func (e *EnergyEngine) AddPolicy(policy *EnergyPolicy) error {
	if policy.ID == "" {
		return errors.New("policy ID cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	policy.CreatedAt = time.Now()
	e.policies[policy.ID] = policy
	return nil
}

// EnablePolicy 启用策略
func (e *EnergyEngine) EnablePolicy(policyID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	policy, ok := e.policies[policyID]
	if !ok {
		return errors.New("policy not found")
	}
	policy.Enabled = true
	return nil
}

// ListPolicies 列出所有策略
func (e *EnergyEngine) ListPolicies() []*EnergyPolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()
	policies := make([]*EnergyPolicy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}
	return policies
}

// ========== 智能建议 ==========

// GenerateSuggestions 生成节能建议
func (e *EnergyEngine) GenerateSuggestions() []EnergySuggestion {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var suggestions []EnergySuggestion

	// 检查磁盘是否可以休眠
	for _, device := range e.devices {
		if device.DiskHibernateMin == 0 {
			suggestions = append(suggestions, EnergySuggestion{
				ID:          "disk-hibernate-" + device.DeviceID,
				Type:        "disk_hibernate",
				Priority:    "medium",
				Title:       "启用磁盘休眠",
				Description: "为空闲磁盘设置自动休眠，可节省约30%磁盘功耗",
				EstSaving:   15.0,
				EstCost:     9.0,
				Confidence:  0.85,
			})
		}
	}

	// 检查能源模式
	if e.currentMode == ModePerformance {
		suggestions = append(suggestions, EnergySuggestion{
			ID:          "energy-mode",
			Type:        "mode_switch",
			Priority:    "low",
			Title:       "切换到均衡模式",
			Description: "当前为性能模式，切换到均衡模式可节省约20%功耗",
			EstSaving:   30.0,
			EstCost:     18.0,
			Confidence:  0.70,
		})
	}

	// 检查CPU调频策略
	for _, device := range e.devices {
		if device.CPUGovernor == "performance" {
			suggestions = append(suggestions, EnergySuggestion{
				ID:          "cpu-governor-" + device.DeviceID,
				Type:        "cpu_freq",
				Priority:    "medium",
				Title:       "调整CPU频率策略",
				Description: "将CPU调频策略从performance改为ondemand，按需调频",
				EstSaving:   20.0,
				EstCost:     12.0,
				Confidence:  0.80,
			})
		}
	}

	e.suggestions = suggestions
	return suggestions
}

// ========== 统计信息 ==========

// GetStatistics 获取能源统计
func (e *EnergyEngine) GetStatistics() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := map[string]interface{}{
		"total_devices":   len(e.devices),
		"total_records":   0,
		"total_energy_kwh": e.totalEnergy,
		"current_mode":    string(e.currentMode),
		"total_policies":  len(e.policies),
	}

	totalRecords := 0
	for _, records := range e.records {
		totalRecords += len(records)
	}
	stats["total_records"] = totalRecords

	var totalPower float64
	for _, records := range e.records {
		if len(records) > 0 {
			totalPower += records[len(records)-1].PowerW
		}
	}
	stats["current_power_w"] = totalPower

	return stats
}

// ========== 内部方法 ==========

func (e *EnergyEngine) initDefaultPolicies() {
	e.policies["default-night"] = &EnergyPolicy{
		ID:          "default-night",
		Name:        "夜间节能",
		Description: "夜间自动降低功耗",
		Mode:        ModePowersave,
		Enabled:     true,
		Priority:    1,
		Schedule: &Schedule{
			StartTime: "23:00",
			EndTime:   "07:00",
			Days:      []int{0, 1, 2, 3, 4, 5, 6},
			Timezone:  "Asia/Shanghai",
		},
		Rules: []PolicyRule{
			{Name: "disk_hibernate", Condition: "idle>30m", Action: "spin_down", Enabled: true},
			{Name: "led_off", Condition: "time=23:00-07:00", Action: "led_off", Enabled: true},
			{Name: "fan_silent", Condition: "temp<50", Action: "fan_silent", Enabled: true},
		},
		CreatedAt: time.Now(),
	}

	e.policies["default-eco"] = &EnergyPolicy{
		ID:          "default-eco",
		Name:        "日常节能",
		Description: "日常使用中的节能策略",
		Mode:        ModeBalanced,
		Enabled:     true,
		Priority:    2,
		Rules: []PolicyRule{
			{Name: "cpu_ondemand", Condition: "always", Action: "cpu_governor", Value: "ondemand", Enabled: true},
			{Name: "smart_fan", Condition: "always", Action: "fan_profile", Value: "balanced", Enabled: true},
		},
		CreatedAt: time.Now(),
	}
}
