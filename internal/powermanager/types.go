// Package powermanager 提供电源管理功能
package powermanager

import (
	"time"
)

// PowerPlan 电源计划类型.
type PowerPlan string

const (
	PowerPlanHighPerf   PowerPlan = "high_performance" // 高性能
	PowerPlanBalanced   PowerPlan = "balanced"         // 均衡
	PowerPlanPowerSave  PowerPlan = "power_save"       // 节能
)

// UPSStatus UPS 状态.
type UPSStatus string

const (
	UPSStatusOnline   UPSStatus = "online"   // 在线（市电供电）
	UPSStatusBattery  UPSStatus = "battery"  // 电池供电
	UPSStatusLow      UPSStatus = "low"      // 电池低电量
	UPSStatusCritical UPSStatus = "critical" // 电量危急
	UPSStatusUnknown  UPSStatus = "unknown"  // 未知
)

// PowerSchedule 电源定时任务.
type PowerSchedule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Action    string    `json:"action"`    // "power_on" 或 "power_off"
	Time      string    `json:"time"`      // HH:MM 格式
	Days      []string  `json:"days"`      // 星期几 ["mon","tue",...]
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

// PowerPlanConfig 电源计划配置.
type PowerPlanConfig struct {
	Plan           PowerPlan `json:"plan"`
	CPUGovernor    string    `json:"cpu_governor"`    // CPU 调频策略
	HDDStandby     int       `json:"hdd_standby"`     // 硬盘休眠时间（分钟）
	LEDBrightness  int       `json:"led_brightness"`  // LED 亮度 0-100
	FanProfile     string    `json:"fan_profile"`     // 风扇策略
	WoLEnabled     bool      `json:"wol_enabled"`     // 网络唤醒
	UpdatedAt      time.Time `json:"updated_at"`
}

// UPSInfo UPS 详细信息.
type UPSInfo struct {
	Status       UPSStatus `json:"status"`
	BatteryLevel int       `json:"battery_level"` // 电池电量百分比
	LoadPercent  int       `json:"load_percent"`  // 负载百分比
	InputVoltage float64   `json:"input_voltage"` // 输入电压
	OutputVoltage float64  `json:"output_voltage"` // 输出电压
	Temperature  float64   `json:"temperature"`   // 温度
	RuntimeMins  int       `json:"runtime_mins"`  // 剩余运行时间（分钟）
	LastUpdated  time.Time `json:"last_updated"`
}

// ConsumptionRecord 功耗记录.
type ConsumptionRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	PowerWatts  float64   `json:"power_watts"`  // 功耗（瓦特）
	CPUUsage    float64   `json:"cpu_usage"`    // CPU 使用率
	DiskIO      float64   `json:"disk_io"`      // 磁盘 IO
	NetworkIO   float64   `json:"network_io"`   // 网络 IO
}

// ConsumptionStats 功耗统计.
type ConsumptionStats struct {
	Current     *ConsumptionRecord   `json:"current"`
	Average24h  float64              `json:"average_24h"`  // 24小时平均功耗
	Peak24h     float64              `json:"peak_24h"`     // 24小时峰值功耗
	TotalKWh    float64              `json:"total_kwh"`    // 总用电量（千瓦时）
	History     []*ConsumptionRecord `json:"history,omitempty"`
}

// WoLRequest 网络唤醒请求.
type WoLRequest struct {
	MACAddress string `json:"mac_address" binding:"required"` // 目标 MAC 地址
	Broadcast  string `json:"broadcast"`                       // 广播地址，默认 255.255.255.255
	Port       int    `json:"port"`                            // 端口，默认 9
}
