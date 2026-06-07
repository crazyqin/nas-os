// Package carbontracker - 碳足迹追踪类型定义
// 监控 NAS 设备能耗并计算碳足迹
package carbontracker

import (
	"time"
)

// CarbonFootprint 碳足迹
type CarbonFootprint struct {
	ID              string            `json:"id"`
	DeviceID        string            `json:"device_id"`
	DeviceName      string            `json:"device_name"`
	Timestamp       time.Time         `json:"timestamp"`
	EnergyKWh       float64           `json:"energy_kwh"`
	CarbonKg        float64           `json:"carbon_kg"`
	CarbonIntensity float64           `json:"carbon_intensity"` // gCO2/kWh
	Source          CarbonSource      `json:"source"`
	Region          string            `json:"region"`
	Period          string            `json:"period"` // hourly, daily, weekly, monthly
	Details         *FootprintDetails `json:"details,omitempty"`
}

// FootprintDetails 碳足迹详情
type FootprintDetails struct {
	CPUWatts     float64 `json:"cpu_watts"`
	DiskWatts    float64 `json:"disk_watts"`
	NetworkWatts float64 `json:"network_watts"`
	MemoryWatts  float64 `json:"memory_watts"`
	GPUWatts     float64 `json:"gpu_watts"`
	FanWatts     float64 `json:"fan_watts"`
	PSUWatts     float64 `json:"psu_watts"`
	OtherWatts   float64 `json:"other_watts"`
	TotalWatts   float64 `json:"total_watts"`
	RuntimeHours float64 `json:"runtime_hours"`
}

// EnergySource 能源来源
type EnergySource struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Type            CarbonSource `json:"type"`
	Region          string       `json:"region"`
	CarbonIntensity float64      `json:"carbon_intensity"` // gCO2/kWh
	CostPerKWh      float64      `json:"cost_per_kwh"`
	Percentage      float64      `json:"percentage"` // 占比
	IsRenewable     bool         `json:"is_renewable"`
	Description     string       `json:"description"`
	IsActive        bool         `json:"is_active"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// EmissionRecord 排放记录
type EmissionRecord struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	DeviceID        string            `json:"device_id"`
	DeviceName      string            `json:"device_name"`
	SourceType      CarbonSource      `json:"source_type"`
	EnergyKWh       float64           `json:"energy_kwh"`
	CarbonKg        float64           `json:"carbon_kg"`
	CarbonIntensity float64           `json:"carbon_intensity"`
	Region          string            `json:"region"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// GreenScore 绿色评分
type GreenScore struct {
	Overall          float64               `json:"overall"`           // 0-100
	Grade            string                `json:"grade"`             // A+, A, B, C, D, E
	EnergyEfficiency float64               `json:"energy_efficiency"` // 能源效率评分
	RenewableUsage   float64               `json:"renewable_usage"`   // 可再生能源使用率
	CarbonReduction  float64               `json:"carbon_reduction"`  // 碳减排评分
	DeviceScores     []DeviceGreenScore    `json:"device_scores"`
	Recommendations  []GreenRecommendation `json:"recommendations"`
	UpdatedAt        time.Time             `json:"updated_at"`
}

// DeviceGreenScore 设备绿色评分
type DeviceGreenScore struct {
	DeviceID    string  `json:"device_id"`
	DeviceName  string  `json:"device_name"`
	Score       float64 `json:"score"`
	Grade       string  `json:"grade"`
	EnergyKWh   float64 `json:"energy_kwh"`
	CarbonKg    float64 `json:"carbon_kg"`
	IdlePercent float64 `json:"idle_percent"` // 空闲时间占比
}

// GreenRecommendation 绿色建议
type GreenRecommendation struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Category      string  `json:"category"` // hardware, software, behavior
	Impact        string  `json:"impact"`   // high, medium, low
	EstimatedSave float64 `json:"estimated_save_kg_per_year"`
	IsImplemented bool    `json:"is_implemented"`
}

// CarbonTrackerManager 碳足迹管理器配置
type CarbonTrackerManagerConfig struct {
	Enabled           bool    `json:"enabled"`
	DefaultRegion     string  `json:"default_region"`
	MonitorInterval   int     `json:"monitor_interval"` // seconds
	RetentionDays     int     `json:"retention_days"`
	GreenThreshold    float64 `json:"green_threshold"` // gCO2/kWh
	EnableAutoTune    bool    `json:"enable_auto_tune"`
	EnableSuggestions bool    `json:"enable_suggestions"`
}

// DefaultCarbonTrackerManagerConfig 默认管理器配置
func DefaultCarbonTrackerManagerConfig() *CarbonTrackerManagerConfig {
	return &CarbonTrackerManagerConfig{
		Enabled:           true,
		DefaultRegion:     "CN",
		MonitorInterval:   60,
		RetentionDays:     365,
		GreenThreshold:    100.0,
		EnableAutoTune:    true,
		EnableSuggestions: true,
	}
}
