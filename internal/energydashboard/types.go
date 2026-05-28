// Package energydashboard 提供智能能源监控功能，支持功耗监控、电费计算、碳排放分析和能效评估。
// 对标群晖 Energy Dashboard，为 NAS 系统提供全面的能源管理能力。
package energydashboard

import "time"

// EnergyUnit 能源单位
type EnergyUnit string

const (
	EnergyUnitWh  EnergyUnit = "Wh"  // 瓦时
	EnergyUnitKWh EnergyUnit = "kWh" // 千瓦时
	EnergyUnitMWh EnergyUnit = "MWh" // 兆瓦时
)

// ReportType 报告类型
type ReportType string

const (
	ReportTypeDaily   ReportType = "daily"   // 日报
	ReportTypeWeekly  ReportType = "weekly"  // 周报
	ReportTypeMonthly ReportType = "monthly" // 月报
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceTypeCPU     DeviceType = "cpu"
	DeviceTypeDisk    DeviceType = "disk"
	DeviceTypeNetwork DeviceType = "network"
	DeviceTypeGPU     DeviceType = "gpu"
	DeviceTypeMemory  DeviceType = "memory"
	DeviceTypeFan     DeviceType = "fan"
	DeviceTypePSU     DeviceType = "psu"
	DeviceTypeOther   DeviceType = "other"
)

// PowerConsumption 功耗数据
type PowerConsumption struct {
	Timestamp  time.Time `json:"timestamp"`
	TotalWatts float64   `json:"total_watts"`
	CPUWatts   float64   `json:"cpu_watts"`
	DiskWatts  float64   `json:"disk_watts"`
	NetWatts   float64   `json:"net_watts"`
	GPUWatts   float64   `json:"gpu_watts"`
	OtherWatts float64   `json:"other_watts"`
}

// EnergyRecord 能耗记录
type EnergyRecord struct {
	ID        string     `json:"id"`
	Timestamp time.Time  `json:"timestamp"`
	Wh        float64    `json:"wh"`
	Cost      float64    `json:"cost"`
	Region    string     `json:"region"`
	Devices   []DeviceRecord `json:"devices,omitempty"`
}

// DeviceRecord 设备能耗记录
type DeviceRecord struct {
	DeviceID   string     `json:"device_id"`
	DeviceName string     `json:"device_name"`
	DeviceType DeviceType `json:"device_type"`
	Watts      float64    `json:"watts"`
	Timestamp  time.Time  `json:"timestamp"`
}

// RegionConfig 地区电价配置
type RegionConfig struct {
	Code          string    `json:"code"`           // 地区代码
	Name          string    `json:"name"`           // 地区名称
	Currency      string    `json:"currency"`       // 货币单位
	RatePerKWh    float64   `json:"rate_per_kwh"`   // 每千瓦时电价
	OffPeakRate   float64   `json:"off_peak_rate"`  // 谷时电价
	PeakRate      float64   `json:"peak_rate"`      // 峰时电价
	SuperPeakRate float64   `json:"super_peak_rate"` // 尖峰电价
	TaxRate       float64   `json:"tax_rate"`       // 税率
	UpdatedAt     time.Time `json:"updated_at"`
}

// CarbonSource 碳排放来源
type CarbonSource struct {
	SourceName     string  `json:"source_name"`      // 来源名称
	EmissionFactor float64 `json:"emission_factor"`  // 排放因子 kgCO2/kWh
	Percentage     float64 `json:"percentage"`       // 占比
}

// CarbonEmission 碳排放数据
type CarbonEmission struct {
	TotalKgCO2    float64        `json:"total_kg_co2"`    // 总碳排放 kg
	TotalTonsCO2  float64        `json:"total_tons_co2"`  // 总碳排放吨
	BySource      []CarbonSource `json:"by_source"`       // 按来源分布
	TreeEquiv     int            `json:"tree_equiv"`      // 等效树木
	Period        string         `json:"period"`          // 统计周期
	Timestamp     time.Time      `json:"timestamp"`
}

// EfficiencyScore 能效评分
type EfficiencyScore struct {
	Score        float64            `json:"score"`         // 总分 0-100
	Performance  float64            `json:"performance"`   // 性能分
	WattRatio    float64            `json:"watt_ratio"`    // 性能/瓦特比
	Rating       string             `json:"rating"`        // 评级: A+, A, B, C, D
	Breakdown    map[string]float64 `json:"breakdown"`     // 分项评分
	Suggestions  []string           `json:"suggestions"`   // 改进建议
	Timestamp    time.Time          `json:"timestamp"`
}

// PowerBudget 功耗预算
type PowerBudget struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	DailyLimitWh  float64   `json:"daily_limit_wh"`
	MonthlyLimitKWh float64 `json:"monthly_limit_kwh"`
	AlertThreshold float64  `json:"alert_threshold"` // 告警阈值百分比 0-100
	Enabled       bool      `json:"enabled"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// PowerAlert 功耗告警
type PowerAlert struct {
	ID        string     `json:"id"`
	BudgetID  string     `json:"budget_id"`
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	CurrentWh float64    `json:"current_wh"`
	LimitWh   float64    `json:"limit_wh"`
	Threshold float64    `json:"threshold"`
	Timestamp time.Time  `json:"timestamp"`
	Acked     bool       `json:"acked"`
}

// EnergyReport 能源报告
type EnergyReport struct {
	ID           string            `json:"id"`
	ReportType   ReportType        `json:"report_type"`
	Period       string            `json:"period"`
	StartDate    time.Time         `json:"start_date"`
	EndDate      time.Time         `json:"end_date"`
	TotalWh      float64           `json:"total_wh"`
	TotalCost    float64           `json:"total_cost"`
	TotalCarbon  float64           `json:"total_carbon"`
	AvgDailyWh   float64           `json:"avg_daily_wh"`
	PeakWh       float64           `json:"peak_wh"`
	PeakTime     time.Time         `json:"peak_time"`
	LowWh        float64           `json:"low_wh"`
	LowTime      time.Time         `json:"low_time"`
	Efficiency   *EfficiencyScore  `json:"efficiency"`
	Devices      []DeviceStats     `json:"devices"`
	Recommendations []string       `json:"recommendations"`
	GeneratedAt  time.Time         `json:"generated_at"`
}

// DeviceStats 设备统计
type DeviceStats struct {
	DeviceID     string     `json:"device_id"`
	DeviceName   string     `json:"device_name"`
	DeviceType   DeviceType `json:"device_type"`
	TotalWh      float64    `json:"total_wh"`
	AvgWatts     float64    `json:"avg_watts"`
	MaxWatts     float64    `json:"max_watts"`
	MinWatts     float64    `json:"min_watts"`
	Percentage   float64    `json:"percentage"` // 占总功耗百分比
}

// PUEData PUE (Power Usage Effectiveness) 数据
type PUEData struct {
	PUE           float64   `json:"pue"`            // PUE 值
	ITEnergy      float64   `json:"it_energy"`      // IT 设备能耗
	TotalEnergy   float64   `json:"total_energy"`   // 总能耗
	CoolingEnergy float64   `json:"cooling_energy"` // 制冷能耗
	OtherEnergy   float64   `json:"other_energy"`   // 其他能耗
	Rating        string    `json:"rating"`         // 评级
	Timestamp     time.Time `json:"timestamp"`
}

// TrendData 趋势数据
type TrendData struct {
	Timestamp time.Time `json:"timestamp"`
	Wh        float64   `json:"wh"`
	Watts     float64   `json:"watts"`
}

// EnergyDashboardConfig 能源看板配置
type EnergyDashboardConfig struct {
	Enabled           bool                `json:"enabled"`
	Region            string              `json:"region"`
	Regions           map[string]RegionConfig `json:"regions"`
	DefaultCurrency   string              `json:"default_currency"`
	MonitorInterval   int                 `json:"monitor_interval"`    // 监控间隔秒
	RetentionDays     int                 `json:"retention_days"`      // 数据保留天数
	PUEEnabled        bool                `json:"pue_enabled"`
	CarbonTracking    bool                `json:"carbon_tracking"`
	BudgetAlerts      bool                `json:"budget_alerts"`
	ReportSchedule    string              `json:"report_schedule"`     // cron 表达式
	Devices           []DeviceConfig      `json:"devices"`
}

// DeviceConfig 设备配置
type DeviceConfig struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	DeviceType  DeviceType `json:"device_type"`
	Enabled     bool       `json:"enabled"`
	PollInterval int       `json:"poll_interval"` // 轮询间隔秒
	MaxWatts    float64    `json:"max_watts"`
}

// EnergyDashboardRequest 能源看板请求
type EnergyDashboardRequest struct {
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	ReportType ReportType `json:"report_type,omitempty"`
	DeviceID   string    `json:"device_id,omitempty"`
}

// EnergyDashboardResponse 能源看板响应
type EnergyDashboardResponse struct {
	CurrentPower   *PowerConsumption `json:"current_power"`
	TodayWh        float64           `json:"today_wh"`
	TodayCost      float64           `json:"today_cost"`
	MonthWh        float64           `json:"month_wh"`
	MonthCost      float64           `json:"month_cost"`
	Carbon         *CarbonEmission   `json:"carbon"`
	Efficiency     *EfficiencyScore  `json:"efficiency"`
	PUE            *PUEData          `json:"pue,omitempty"`
	Budgets        []PowerBudget     `json:"budgets"`
	Alerts         []PowerAlert      `json:"alerts"`
	Trend          []TrendData       `json:"trend"`
	Devices        []DeviceStats     `json:"devices"`
	Timestamp      time.Time         `json:"timestamp"`
}

// SetBudgetRequest 设置预算请求
type SetBudgetRequest struct {
	Name            string  `json:"name" binding:"required"`
	DailyLimitWh    float64 `json:"daily_limit_wh"`
	MonthlyLimitKWh float64 `json:"monthly_limit_kwh"`
	AlertThreshold  float64 `json:"alert_threshold"`
}

// UpdateRegionRequest 更新地区配置请求
type UpdateRegionRequest struct {
	Code          string  `json:"code" binding:"required"`
	Name          string  `json:"name"`
	Currency      string  `json:"currency"`
	RatePerKWh    float64 `json:"rate_per_kwh"`
	OffPeakRate   float64 `json:"off_peak_rate"`
	PeakRate      float64 `json:"peak_rate"`
	SuperPeakRate float64 `json:"super_peak_rate"`
	TaxRate       float64 `json:"tax_rate"`
}

// DefaultEnergyDashboardConfig 默认配置
func DefaultEnergyDashboardConfig() *EnergyDashboardConfig {
	return &EnergyDashboardConfig{
		Enabled:         true,
		Region:          "CN",
		DefaultCurrency: "CNY",
		MonitorInterval: 60,
		RetentionDays:   365,
		PUEEnabled:      true,
		CarbonTracking:  true,
		BudgetAlerts:    true,
		ReportSchedule:  "0 0 * * *", // 每天凌晨
		Regions: map[string]RegionConfig{
			"CN": {
				Code:          "CN",
				Name:          "中国大陆",
				Currency:      "CNY",
				RatePerKWh:    0.55,
				OffPeakRate:   0.35,
				PeakRate:      0.55,
				SuperPeakRate: 0.85,
				TaxRate:       0.13,
			},
			"US": {
				Code:          "US",
				Name:          "United States",
				Currency:      "USD",
				RatePerKWh:    0.12,
				OffPeakRate:   0.08,
				PeakRate:      0.15,
				SuperPeakRate: 0.20,
				TaxRate:       0.0,
			},
		},
		Devices: []DeviceConfig{},
	}
}