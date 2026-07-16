// Package smartcarbon 智能碳管理模块
// 碳足迹追踪、碳排放计算、碳补偿建议、绿色存储优化
package smartcarbon

import (
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"
)

// CarbonSource 碳排放来源.
type CarbonSource string

const (
	SourceStorage CarbonSource = "storage"
	SourceCompute CarbonSource = "compute"
	SourceNetwork CarbonSource = "network"
	SourceCooling CarbonSource = "cooling"
	SourceIdle    CarbonSource = "idle"
)

// EnergyType 能源类型.
type EnergyType string

const (
	EnergySolar   EnergyType = "solar"
	EnergyWind    EnergyType = "wind"
	EnergyGrid    EnergyType = "grid"
	EnergyBattery EnergyType = "battery"
	EnergyHybrid  EnergyType = "hybrid"
)

// CarbonIntensity 碳强度级别.
type CarbonIntensity string

const (
	IntensityLow      CarbonIntensity = "low"
	IntensityMedium   CarbonIntensity = "medium"
	IntensityHigh     CarbonIntensity = "high"
	IntensityCritical CarbonIntensity = "critical"
)

// CarbonRecord 碳排放记录.
type CarbonRecord struct {
	ID          string            `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	Source      CarbonSource      `json:"source"`
	ValueKg     float64           `json:"value_kg"`
	EnergyKWh   float64           `json:"energy_kwh"`
	EnergyType  EnergyType        `json:"energy_type"`
	Description string            `json:"description"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// CarbonBudget 碳预算.
type CarbonBudget struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	DailyLimitKg     float64   `json:"daily_limit_kg"`
	MonthlyLimitKg   float64   `json:"monthly_limit_kg"`
	YearlyLimitKg    float64   `json:"yearly_limit_kg"`
	CurrentDailyKg   float64   `json:"current_daily_kg"`
	CurrentMonthlyKg float64   `json:"current_monthly_kg"`
	AlertThreshold   float64   `json:"alert_threshold"`
	Enabled          bool      `json:"enabled"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// CarbonFootprint 碳足迹.
type CarbonFootprint struct {
	TotalKg         float64                  `json:"total_kg"`
	BySource        map[CarbonSource]float64 `json:"by_source"`
	ByEnergy        map[EnergyType]float64   `json:"by_energy"`
	PeriodStart     time.Time                `json:"period_start"`
	PeriodEnd       time.Time                `json:"period_end"`
	EquivalentTrees float64                  `json:"equivalent_trees"`
	CarbonCredits   float64                  `json:"carbon_credits"`
	Intensity       CarbonIntensity          `json:"intensity"`
}

// CarbonOffset 碳补偿.
type CarbonOffset struct {
	ID          string     `json:"id"`
	ProjectName string     `json:"project_name"`
	Type        string     `json:"type"`
	CreditsKg   float64    `json:"credits_kg"`
	CostUSD     float64    `json:"cost_usd"`
	Status      string     `json:"status"`
	CreatedAt   time.Time  `json:"created_at"`
	VerifiedAt  *time.Time `json:"verified_at,omitempty"`
}

// GreenOptimization 绿色优化建议.
type GreenOptimization struct {
	ID          string  `json:"id"`
	Category    string  `json:"category"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	SavingsKg   float64 `json:"savings_kg"`
	SavingsUSD  float64 `json:"savings_usd"`
	Priority    int     `json:"priority"`
	Implemented bool    `json:"implemented"`
}

// CarbonStats 碳管理统计.
type CarbonStats struct {
	TodayKg        float64         `json:"today_kg"`
	ThisWeekKg     float64         `json:"this_week_kg"`
	ThisMonthKg    float64         `json:"this_month_kg"`
	ThisYearKg     float64         `json:"this_year_kg"`
	AverageDailyKg float64         `json:"average_daily_kg"`
	Trend          string          `json:"trend"`
	Intensity      CarbonIntensity `json:"intensity"`
	TopSource      CarbonSource    `json:"top_source"`
	GreenEnergyPct float64         `json:"green_energy_pct"`
	TotalOffsets   float64         `json:"total_offsets"`
	NetEmissions   float64         `json:"net_emissions"`
}

// CarbonManager 碳管理器.
type CarbonManager struct {
	mu             sync.RWMutex
	records        []CarbonRecord
	budget         *CarbonBudget
	offsets        []CarbonOffset
	optimizations  []GreenOptimization
	config         *CarbonConfig
	emissionFactor float64 // kg CO2 per kWh
}

// CarbonConfig 碳管理配置.
type CarbonConfig struct {
	DefaultRegion    string  `json:"default_region"`
	EmissionFactor   float64 `json:"emission_factor"`
	GreenEnergyPct   float64 `json:"green_energy_pct"`
	OffsetEnabled    bool    `json:"offset_enabled"`
	AlertEnabled     bool    `json:"alert_enabled"`
	OptimizationMode string  `json:"optimization_mode"`
	TrackingInterval int     `json:"tracking_interval_minutes"`
}

// NewCarbonManager 创建碳管理器.
func NewCarbonManager(config *CarbonConfig) *CarbonManager {
	if config == nil {
		config = &CarbonConfig{
			DefaultRegion:    "default",
			EmissionFactor:   0.5, // 0.5 kg CO2 per kWh (中国电网平均)
			GreenEnergyPct:   0.0,
			OffsetEnabled:    true,
			AlertEnabled:     true,
			OptimizationMode: "balanced",
			TrackingInterval: 5,
		}
	}
	return &CarbonManager{
		records:        make([]CarbonRecord, 0),
		offsets:        make([]CarbonOffset, 0),
		optimizations:  make([]GreenOptimization, 0),
		config:         config,
		emissionFactor: config.EmissionFactor,
	}
}

// RecordEmission 记录碳排放.
func (cm *CarbonManager) RecordEmission(record *CarbonRecord) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if record.ID == "" {
		return fmt.Errorf("record ID is required")
	}

	record.Timestamp = time.Now()
	if record.ValueKg == 0 && record.EnergyKWh > 0 {
		record.ValueKg = record.EnergyKWh * cm.emissionFactor
	}

	cm.records = append(cm.records, *record)

	// 检查预算
	if cm.budget != nil && cm.budget.Enabled {
		cm.updateBudgetUsage()
		cm.checkBudgetAlert()
	}

	return nil
}

// SetBudget 设置碳预算.
func (cm *CarbonManager) SetBudget(budget *CarbonBudget) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	budget.UpdatedAt = time.Now()
	if budget.CreatedAt.IsZero() {
		budget.CreatedAt = time.Now()
	}
	cm.budget = budget
	return nil
}

// AddOffset 添加碳补偿.
func (cm *CarbonManager) AddOffset(offset *CarbonOffset) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if offset.ID == "" {
		return fmt.Errorf("offset ID is required")
	}

	now := time.Now()
	offset.CreatedAt = now
	offset.Status = "pending"

	cm.offsets = append(cm.offsets, *offset)
	return nil
}

// VerifyOffset 验证碳补偿.
func (cm *CarbonManager) VerifyOffset(offsetID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for i, offset := range cm.offsets {
		if offset.ID == offsetID {
			now := time.Now()
			cm.offsets[i].Status = "verified"
			cm.offsets[i].VerifiedAt = &now
			return nil
		}
	}
	return fmt.Errorf("offset %s not found", offsetID)
}

// GetFootprint 获取碳足迹.
func (cm *CarbonManager) GetFootprint(start, end time.Time) *CarbonFootprint {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	footprint := &CarbonFootprint{
		BySource:    make(map[CarbonSource]float64),
		ByEnergy:    make(map[EnergyType]float64),
		PeriodStart: start,
		PeriodEnd:   end,
	}

	for _, record := range cm.records {
		if record.Timestamp.After(start) && record.Timestamp.Before(end) {
			footprint.TotalKg += record.ValueKg
			footprint.BySource[record.Source] += record.ValueKg
			footprint.ByEnergy[record.EnergyType] += record.ValueKg
		}
	}

	// 计算等效树木（一棵树每年吸收约 22kg CO2）
	days := end.Sub(start).Hours() / 24
	if days > 0 {
		footprint.EquivalentTrees = footprint.TotalKg / (22.0 / 365.0 * days)
	}

	// 计算碳信用
	for _, offset := range cm.offsets {
		if offset.Status == "verified" {
			footprint.CarbonCredits += offset.CreditsKg
		}
	}

	// 确定碳强度
	footprint.Intensity = cm.calculateIntensity(footprint.TotalKg)

	return footprint
}

// GetStats 获取统计信息.
func (cm *CarbonManager) GetStats() *CarbonStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	yearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())

	stats := &CarbonStats{}

	for _, record := range cm.records {
		if record.Timestamp.After(todayStart) {
			stats.TodayKg += record.ValueKg
		}
		if record.Timestamp.After(weekStart) {
			stats.ThisWeekKg += record.ValueKg
		}
		if record.Timestamp.After(monthStart) {
			stats.ThisMonthKg += record.ValueKg
		}
		if record.Timestamp.After(yearStart) {
			stats.ThisYearKg += record.ValueKg
		}
	}

	// 计算日均
	daysSinceFirst := 1.0
	if len(cm.records) > 0 {
		daysSinceFirst = now.Sub(cm.records[0].Timestamp).Hours() / 24
		if daysSinceFirst < 1 {
			daysSinceFirst = 1
		}
	}
	stats.AverageDailyKg = stats.ThisYearKg / daysSinceFirst

	// 趋势分析
	if stats.TodayKg > stats.AverageDailyKg*1.1 {
		stats.Trend = "increasing"
	} else if stats.TodayKg < stats.AverageDailyKg*0.9 {
		stats.Trend = "decreasing"
	} else {
		stats.Trend = "stable"
	}

	stats.Intensity = cm.calculateIntensity(stats.TodayKg)

	// 绿色能源比例
	stats.GreenEnergyPct = cm.config.GreenEnergyPct * 100

	// 碳补偿
	for _, offset := range cm.offsets {
		if offset.Status == "verified" {
			stats.TotalOffsets += offset.CreditsKg
		}
	}

	stats.NetEmissions = stats.ThisYearKg - stats.TotalOffsets

	return stats
}

// GetOptimizations 获取优化建议.
func (cm *CarbonManager) GetOptimizations() []GreenOptimization {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	optimizations := []GreenOptimization{
		{
			ID:          "opt-1",
			Category:    "storage",
			Title:       "启用数据分层存储",
			Description: "将不常用数据迁移到低功耗存储层，可减少 30% 存储能耗",
			SavingsKg:   50.0,
			SavingsUSD:  25.0,
			Priority:    1,
		},
		{
			ID:          "opt-2",
			Category:    "compute",
			Title:       "智能休眠策略",
			Description: "在低负载时段自动降低 CPU 频率，减少 20% 计算能耗",
			SavingsKg:   30.0,
			SavingsUSD:  15.0,
			Priority:    2,
		},
		{
			ID:          "opt-3",
			Category:    "cooling",
			Title:       "动态风扇控制",
			Description: "根据温度动态调整风扇转速，减少 15% 冷却能耗",
			SavingsKg:   20.0,
			SavingsUSD:  10.0,
			Priority:    3,
		},
		{
			ID:          "opt-4",
			Category:    "energy",
			Title:       "使用可再生能源",
			Description: "配置使用太阳能或风能供电，可减少 80% 碳排放",
			SavingsKg:   200.0,
			SavingsUSD:  100.0,
			Priority:    0,
		},
	}

	return optimizations
}

// MarshalJSON 序列化.
func (cm *CarbonManager) MarshalJSON() ([]byte, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return json.Marshal(struct {
		Records       []CarbonRecord      `json:"records"`
		Budget        *CarbonBudget       `json:"budget"`
		Offsets       []CarbonOffset      `json:"offsets"`
		Optimizations []GreenOptimization `json:"optimizations"`
		Config        *CarbonConfig       `json:"config"`
	}{
		Records:       cm.records,
		Budget:        cm.budget,
		Offsets:       cm.offsets,
		Optimizations: cm.optimizations,
		Config:        cm.config,
	})
}

// 内部方法

func (cm *CarbonManager) updateBudgetUsage() {
	if cm.budget == nil {
		return
	}

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	cm.budget.CurrentDailyKg = 0
	cm.budget.CurrentMonthlyKg = 0

	for _, record := range cm.records {
		if record.Timestamp.After(todayStart) {
			cm.budget.CurrentDailyKg += record.ValueKg
		}
		if record.Timestamp.After(monthStart) {
			cm.budget.CurrentMonthlyKg += record.ValueKg
		}
	}

	cm.budget.UpdatedAt = now
}

func (cm *CarbonManager) checkBudgetAlert() {
	if cm.budget == nil || !cm.config.AlertEnabled {
		return
	}

	threshold := cm.budget.AlertThreshold
	if threshold == 0 {
		threshold = 0.8 // 默认 80% 告警
	}

	if cm.budget.DailyLimitKg > 0 && cm.budget.CurrentDailyKg/cm.budget.DailyLimitKg > threshold {
		// 触发告警（实际实现中会发送通知）
		fmt.Printf("ALERT: Daily carbon budget at %.1f%%\n", cm.budget.CurrentDailyKg/cm.budget.DailyLimitKg*100)
	}
}

func (cm *CarbonManager) calculateIntensity(dailyKg float64) CarbonIntensity {
	switch {
	case dailyKg < 1.0:
		return IntensityLow
	case dailyKg < 5.0:
		return IntensityMedium
	case dailyKg < 10.0:
		return IntensityHigh
	default:
		return IntensityCritical
	}
}

// EstimateCarbonFootprint 估算碳足迹.
func EstimateCarbonFootprint(storageTB float64, computeHours float64, networkGB float64) float64 {
	// 存储：每 TB 每年约 50 kg CO2
	storageKg := storageTB * 50.0

	// 计算：每小时约 0.1 kg CO2
	computeKg := computeHours * 0.1

	// 网络：每 GB 约 0.01 kg CO2
	networkKg := networkGB * 0.01

	return storageKg + computeKg + networkKg
}

// ConvertToTrees 将碳排放转换为等效树木.
func ConvertToTrees(carbonKg float64) float64 {
	// 一棵树每年吸收约 22 kg CO2
	return math.Ceil(carbonKg / 22.0)
}
