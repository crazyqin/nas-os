// Package carbontracker 提供碳足迹追踪功能，支持能耗监控、碳排放计算、
// 历史趋势分析、碳减排建议、绿色能源调度和 ESG 报告生成。
package carbontracker

import (
	"fmt"
	"time"
)

// CarbonSource 碳排放来源类型
type CarbonSource string

const (
	CarbonSourceGrid    CarbonSource = "grid"    // 电网供电
	CarbonSourceSolar   CarbonSource = "solar"   // 太阳能
	CarbonSourceWind    CarbonSource = "wind"    // 风能
	CarbonSourceBattery CarbonSource = "battery" // 电池储能
	CarbonSourceDiesel  CarbonSource = "diesel"  // 柴油发电
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeCPU     DeviceType = "cpu"
	DeviceTypeDisk    DeviceType = "disk"
	DeviceTypeNetwork DeviceType = "network"
	DeviceTypeMemory  DeviceType = "memory"
	DeviceTypeGPU     DeviceType = "gpu"
	DeviceTypeFan     DeviceType = "fan"
	DeviceTypePSU     DeviceType = "psu"
	DeviceTypeOther   DeviceType = "other"
)

// ReportType 报告类型
type ReportType string

const (
	ReportTypeDaily   ReportType = "daily"
	ReportTypeWeekly  ReportType = "weekly"
	ReportTypeMonthly ReportType = "monthly"
	ReportTypeAnnual  ReportType = "annual"
)

// CarbonIntensityUnit 碳强度单位
type CarbonIntensityUnit string

const (
	CarbonIntensityGramKWh CarbonIntensityUnit = "gCO2/kWh"
	CarbonIntensityKgKWh   CarbonIntensityUnit = "kgCO2/kWh"
)

// EnergyConsumption 能耗数据
type EnergyConsumption struct {
	Timestamp   time.Time `json:"timestamp"`
	TotalWatts  float64   `json:"total_watts"`
	CPUWatts    float64   `json:"cpu_watts"`
	DiskWatts   float64   `json:"disk_watts"`
	NetWatts    float64   `json:"net_watts"`
	MemoryWatts float64   `json:"memory_watts"`
	GPUWatts    float64   `json:"gpu_watts"`
	OtherWatts  float64   `json:"other_watts"`
}

// CarbonRecord 碳排放记录
type CarbonRecord struct {
	ID           string      `json:"id"`
	Timestamp    time.Time   `json:"timestamp"`
	EnergyKWh    float64     `json:"energy_kwh"`
	CarbonKg     float64     `json:"carbon_kg"`
	Source       CarbonSource `json:"source"`
	DataCenterID string      `json:"datacenter_id"`
	DeviceStats  []DeviceCarbonStat `json:"device_stats,omitempty"`
}

// DeviceCarbonStat 设备碳排放统计
type DeviceCarbonStat struct {
	DeviceID   string     `json:"device_id"`
	DeviceName string     `json:"device_name"`
	DeviceType DeviceType `json:"device_type"`
	EnergyKWh  float64    `json:"energy_kwh"`
	CarbonKg   float64    `json:"carbon_kg"`
	Percentage float64    `json:"percentage"`
}

// CarbonIntensity 碳强度数据
type CarbonIntensity struct {
	Region       string              `json:"region"`
	Value        float64             `json:"value"` // gCO2/kWh
	Unit         CarbonIntensityUnit `json:"unit"`
	Source       string              `json:"source"` // 数据来源
	LastUpdated  time.Time           `json:"last_updated"`
	ForecastNext float64             `json:"forecast_next,omitempty"` // 预测下一小时
}

// CarbonTarget 碳中和目标
type CarbonTarget struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	TargetYear      int       `json:"target_year"`
	ReductionPct    float64   `json:"reduction_pct"`    // 减排目标百分比
	BaselineYear    int       `json:"baseline_year"`
	BaselineCarbonT float64   `json:"baseline_carbon_t"` // 基准年碳排放吨数
	CurrentCarbonT  float64   `json:"current_carbon_t"`  // 当前碳排放吨数
	Progress        float64   `json:"progress"`          // 进度百分比
	Status          string    `json:"status"`            // on_track, at_risk, behind
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// GreenEnergySuggestion 绿色能源调度建议
type GreenEnergySuggestion struct {
	ID            string    `json:"id"`
	Timestamp     time.Time `json:"timestamp"`
	Suggestion    string    `json:"suggestion"`
	Priority      string    `json:"priority"` // high, medium, low
	Category      string    `json:"category"` // scheduling, hardware, behavior
	EstimatedSave float64   `json:"estimated_save_kg"` // 预计减排 kg CO2
	Detail        string    `json:"detail"`
}

// CarbonReductionTip 碳减排建议
type CarbonReductionTip struct {
	ID            string  `json:"id"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Category      string  `json:"category"`
	Impact        string  `json:"impact"` // high, medium, low
	EstimatedSave float64 `json:"estimated_save_kg_per_year"`
	Implemented   bool    `json:"implemented"`
}

// ESGReport ESG 报告
type ESGReport struct {
	ID              string           `json:"id"`
	ReportType      ReportType       `json:"report_type"`
	Period          string           `json:"period"`
	StartDate       time.Time        `json:"start_date"`
	EndDate         time.Time        `json:"end_date"`
	TotalEnergyKWh  float64          `json:"total_energy_kwh"`
	TotalCarbonT    float64          `json:"total_carbon_t"`
	CarbonIntensity float64          `json:"carbon_intensity"` // gCO2/kWh
	GreenEnergyPct  float64          `json:"green_energy_pct"`
	Emissions       []EmissionEntry  `json:"emissions"`
	Reductions      []ReductionEntry `json:"reductions"`
	Score           *ESGScore        `json:"score"`
	Recommendations []string         `json:"recommendations"`
	GeneratedAt     time.Time        `json:"generated_at"`
}

// EmissionEntry 排放条目
type EmissionEntry struct {
	Source      CarbonSource `json:"source"`
	EnergyKWh   float64      `json:"energy_kwh"`
	CarbonKg    float64      `json:"carbon_kg"`
	Percentage  float64      `json:"percentage"`
}

// ReductionEntry 减排条目
type ReductionEntry struct {
	Action     string  `json:"action"`
	CarbonKg   float64 `json:"carbon_kg"`
	Percentage float64 `json:"percentage"`
}

// ESGScore ESG 评分
type ESGScore struct {
	Overall    float64           `json:"overall"`     // 0-100
	Environmental float64        `json:"environmental"`
	Social     float64           `json:"social"`
	Governance float64           `json:"governance"`
	Breakdown  map[string]float64 `json:"breakdown"`
	Rating     string            `json:"rating"` // A+, A, B, C, D, E
}

// DataCenterInfo 数据中心信息
type DataCenterInfo struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Location     string  `json:"location"`
	Region       string  `json:"region"`
	CarbonIntensity float64 `json:"carbon_intensity"` // gCO2/kWh
	GreenEnergyPct  float64 `json:"green_energy_pct"`
	PUE          float64  `json:"pue"`
}

// DataCenterComparison 数据中心对比
type DataCenterComparison struct {
	DataCenters []DataCenterInfo   `json:"data_centers"`
	Comparison  []ComparisonMetric `json:"comparison"`
	BestChoice  string             `json:"best_choice"`
	Timestamp   time.Time          `json:"timestamp"`
}

// ComparisonMetric 对比指标
type ComparisonMetric struct {
	Metric     string             `json:"metric"`
	Unit       string             `json:"unit"`
	Values     map[string]float64 `json:"values"` // datacenter_id -> value
	BestDC     string             `json:"best_dc"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	EnergyKWh  float64   `json:"energy_kwh"`
	CarbonKg   float64   `json:"carbon_kg"`
	Intensity  float64   `json:"intensity"`
}

// CarbonTrackerConfig 碳足迹追踪配置
type CarbonTrackerConfig struct {
	Enabled           bool              `json:"enabled"`
	DefaultRegion     string            `json:"default_region"`
	RegionIntensities map[string]float64 `json:"region_intensities"` // region -> gCO2/kWh
	MonitorInterval   int               `json:"monitor_interval"`   // seconds
	RetentionDays     int               `json:"retention_days"`
	GreenThreshold    float64           `json:"green_threshold"`    // gCO2/kWh, below this is "green"
	Targets           []CarbonTarget    `json:"targets"`
	DataCenters       []DataCenterInfo  `json:"data_centers"`
}

// DashboardResponse 仪表盘响应
type DashboardResponse struct {
	CurrentEnergy     *EnergyConsumption     `json:"current_energy"`
	TodayCarbonKg     float64                `json:"today_carbon_kg"`
	MonthCarbonKg     float64                `json:"month_carbon_kg"`
	YearCarbonT       float64                `json:"year_carbon_t"`
	CarbonIntensity   *CarbonIntensity       `json:"carbon_intensity"`
	Targets           []CarbonTarget         `json:"targets"`
	Suggestions       []GreenEnergySuggestion `json:"suggestions"`
	ReductionTips     []CarbonReductionTip   `json:"reduction_tips"`
	Trend             []TrendPoint           `json:"trend"`
	DeviceBreakdown   []DeviceCarbonStat     `json:"device_breakdown"`
	GreenEnergyPct    float64                `json:"green_energy_pct"`
	Timestamp         time.Time              `json:"timestamp"`
}

// DefaultCarbonTrackerConfig 默认配置
func DefaultCarbonTrackerConfig() *CarbonTrackerConfig {
	return &CarbonTrackerConfig{
		Enabled:         true,
		DefaultRegion:   "CN",
		MonitorInterval: 60,
		RetentionDays:   365,
		GreenThreshold:  100.0,
		RegionIntensities: map[string]float64{
			"CN":  581.0,  // 中国平均
			"US":  386.0,  // 美国平均
			"EU":  276.0,  // 欧盟平均
			"JP":  462.0,  // 日本平均
			"IN":  708.0,  // 印度平均
			"AU":  530.0,  // 澳大利亚平均
		},
		Targets:     []CarbonTarget{},
		DataCenters: []DataCenterInfo{},
	}
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("ct_%d", time.Now().UnixNano())
}
