// Package storagesimulator 存储容量模拟器
// 根据历史增长趋势预测未来存储使用量，支持多种场景模拟和容量规划
package storagesimulator

import (
	"fmt"
	"sync"
	"time"
)

// ForecastPeriod 预测周期
type ForecastPeriod string

const (
	PeriodDaily   ForecastPeriod = "daily"
	PeriodWeekly  ForecastPeriod = "weekly"
	PeriodMonthly ForecastPeriod = "monthly"
	PeriodYearly  ForecastPeriod = "yearly"
)

// GrowthScenario 增长场景
type GrowthScenario string

const (
	ScenarioHigh    GrowthScenario = "high"    // 高增长
	ScenarioMedium  GrowthScenario = "medium"  // 中等增长（基于历史趋势）
	ScenarioLow     GrowthScenario = "low"     // 低增长
	ScenarioStable  GrowthScenario = "stable"  // 稳定（无增长）
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// StorageUsage 存储使用量记录
type StorageUsage struct {
	Timestamp    time.Time `json:"timestamp"`
	UsedBytes    int64     `json:"used_bytes"`
	TotalBytes   int64     `json:"total_bytes"`
	UsedPercent  float64   `json:"used_percent"`
}

// ForecastPoint 预测点
type ForecastPoint struct {
	Date        time.Time     `json:"date"`
	UsedBytes   int64         `json:"used_bytes"`
	UsedPercent float64       `json:"used_percent"`
	Scenario    GrowthScenario `json:"scenario"`
}

// ForecastResult 预测结果
type ForecastResult struct {
	Period       ForecastPeriod    `json:"period"`
	Points       []ForecastPoint   `json:"points"`
	Scenario     GrowthScenario    `json:"scenario"`
	TotalCapacity int64            `json:"total_capacity"`
	GeneratedAt  time.Time         `json:"generated_at"`
}

// ScenarioConfig 场景配置
type ScenarioConfig struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	GrowthRate    float64        `json:"growth_rate"`    // 日增长率（百分比）
	GrowthType    string         `json:"growth_type"`    // linear, exponential
	Period        ForecastPeriod `json:"period"`
	Duration      int            `json:"duration"`       // 预测时长（按周期单位）
	CreatedAt     time.Time      `json:"created_at"`
}

// ScenarioResult 场景模拟结果
type ScenarioResult struct {
	Config       ScenarioConfig   `json:"config"`
	Forecasts    []ForecastPoint  `json:"forecasts"`
	FullDate     *time.Time       `json:"full_date,omitempty"`     // 容量用尽日期
	WarningDate  *time.Time       `json:"warning_date,omitempty"`  // 告警触发日期
	GeneratedAt  time.Time        `json:"generated_at"`
}

// CapacityAlert 容量告警
type CapacityAlert struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Threshold    float64    `json:"threshold"`     // 使用率阈值（百分比）
	Level        AlertLevel `json:"level"`
	Enabled      bool       `json:"enabled"`
	NotifyMethod string     `json:"notify_method"` // email, webhook, sms
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// AlertStatus 告警状态
type AlertStatus struct {
	Alert       CapacityAlert `json:"alert"`
	Triggered   bool          `json:"triggered"`
	CurrentUsed float64       `json:"current_used"`
	Message     string        `json:"message,omitempty"`
}

// CostEstimate 成本估算
type CostEstimate struct {
	CurrentUsedGB    float64            `json:"current_used_gb"`
	TotalCapacityGB  float64            `json:"total_capacity_gb"`
	CostPerGBPerMonth float64           `json:"cost_per_gb_per_month"`
	CurrentMonthlyCost float64          `json:"current_monthly_cost"`
	ProjectedCosts   []ProjectedCost    `json:"projected_costs"`
	Currency         string             `json:"currency"`
	GeneratedAt      time.Time          `json:"generated_at"`
}

// ProjectedCost 预测成本
type ProjectedCost struct {
	Date       time.Time `json:"date"`
	UsedGB     float64   `json:"used_gb"`
	MonthlyCost float64  `json:"monthly_cost"`
	Scenario   string    `json:"scenario"`
}

// CapacityReport 容量规划报告
type CapacityReport struct {
	Summary         ReportSummary      `json:"summary"`
	CurrentUsage    StorageUsage       `json:"current_usage"`
	Forecasts       []ForecastResult   `json:"forecasts"`
	Scenarios       []ScenarioResult   `json:"scenarios"`
	AlertStatuses   []AlertStatus      `json:"alert_statuses"`
	CostEstimate    CostEstimate       `json:"cost_estimate"`
	Recommendations []string           `json:"recommendations"`
	GeneratedAt     time.Time          `json:"generated_at"`
}

// ReportSummary 报告摘要
type ReportSummary struct {
	TotalCapacityGB   float64   `json:"total_capacity_gb"`
	UsedCapacityGB    float64   `json:"used_capacity_gb"`
	AvailableGB       float64   `json:"available_gb"`
	UsedPercent       float64   `json:"used_percent"`
	DaysUntilFull     int       `json:"days_until_full"`
	DaysUntilWarning  int       `json:"days_until_warning"`
	GrowthRatePerDay  float64   `json:"growth_rate_per_day"`
	HealthStatus      string    `json:"health_status"` // healthy, warning, critical
}

// Manager 存储容量模拟器管理器
type Manager struct {
	mu              sync.RWMutex
	usageHistory    []StorageUsage
	totalCapacity   int64
	customScenarios map[string]*ScenarioConfig
	alerts          map[string]*CapacityAlert
	costPerGB       float64
	currency        string
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		usageHistory:    make([]StorageUsage, 0),
		totalCapacity:   10 * 1024 * 1024 * 1024 * 1024, // 默认 10TB
		customScenarios: make(map[string]*ScenarioConfig),
		alerts:          make(map[string]*CapacityAlert),
		costPerGB:       0.02, // 默认 $0.02/GB/月
		currency:        "USD",
	}
}

// SetTotalCapacity 设置总容量
func (m *Manager) SetTotalCapacity(bytes int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalCapacity = bytes
}

// AddUsageRecord 添加使用量记录
func (m *Manager) AddUsageRecord(usage StorageUsage) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if usage.TotalBytes == 0 {
		usage.TotalBytes = m.totalCapacity
	}
	if usage.TotalBytes > 0 {
		usage.UsedPercent = float64(usage.UsedBytes) / float64(usage.TotalBytes) * 100
	}
	m.usageHistory = append(m.usageHistory, usage)
}

// GetUsageHistory 获取使用量历史
func (m *Manager) GetUsageHistory() []StorageUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]StorageUsage, len(m.usageHistory))
	copy(result, m.usageHistory)
	return result
}

// calculateGrowthRate 计算日增长率
func (m *Manager) calculateGrowthRate() float64 {
	if len(m.usageHistory) < 2 {
		return 0
	}

	// 使用最近30天的数据计算增长率
	history := m.usageHistory
	if len(history) > 30 {
		history = history[len(history)-30:]
	}

	totalGrowth := 0.0
	days := 0
	for i := 1; i < len(history); i++ {
		if history[i].UsedBytes > history[i-1].UsedBytes {
			growth := float64(history[i].UsedBytes-history[i-1].UsedBytes) / float64(history[i-1].UsedBytes) * 100
			totalGrowth += growth
			days++
		}
	}

	if days == 0 {
		return 0
	}
	return totalGrowth / float64(days)
}

// Forecast 生成容量预测
func (m *Manager) Forecast(period ForecastPeriod, duration int, scenario GrowthScenario) (*ForecastResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.usageHistory) == 0 {
		return nil, fmt.Errorf("无历史使用数据")
	}

	if duration <= 0 {
		duration = 30
	}

	latest := m.usageHistory[len(m.usageHistory)-1]
	growthRate := m.calculateGrowthRate()

	// 根据场景调整增长率
	switch scenario {
	case ScenarioHigh:
		growthRate *= 1.5
	case ScenarioLow:
		growthRate *= 0.5
	case ScenarioStable:
		growthRate = 0
	case ScenarioMedium:
		// 使用原始增长率
	}

	points := make([]ForecastPoint, 0)
	currentUsed := float64(latest.UsedBytes)

	for i := 1; i <= duration; i++ {
		var date time.Time
		switch period {
		case PeriodDaily:
			date = latest.Timestamp.AddDate(0, 0, i)
		case PeriodWeekly:
			date = latest.Timestamp.AddDate(0, 0, i*7)
		case PeriodMonthly:
			date = latest.Timestamp.AddDate(0, i, 0)
		case PeriodYearly:
			date = latest.Timestamp.AddDate(i, 0, 0)
		}

		// 应用增长率
		switch period {
		case PeriodDaily:
			currentUsed *= (1 + growthRate/100)
		case PeriodWeekly:
			currentUsed *= (1 + growthRate*7/100)
		case PeriodMonthly:
			currentUsed *= (1 + growthRate*30/100)
		case PeriodYearly:
			currentUsed *= (1 + growthRate*365/100)
		}

		usedPercent := currentUsed / float64(m.totalCapacity) * 100

		points = append(points, ForecastPoint{
			Date:        date,
			UsedBytes:   int64(currentUsed),
			UsedPercent: usedPercent,
			Scenario:    scenario,
		})
	}

	return &ForecastResult{
		Period:        period,
		Points:        points,
		Scenario:      scenario,
		TotalCapacity: m.totalCapacity,
		GeneratedAt:   time.Now(),
	}, nil
}

// AddScenario 添加自定义场景
func (m *Manager) AddScenario(config *ScenarioConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		return fmt.Errorf("场景ID不能为空")
	}
	if config.GrowthRate < 0 {
		return fmt.Errorf("增长率不能为负数")
	}
	if config.Duration <= 0 {
		config.Duration = 30
	}
	if config.Period == "" {
		config.Period = PeriodDaily
	}

	config.CreatedAt = time.Now()
	m.customScenarios[config.ID] = config
	return nil
}

// ListScenarios 列出所有场景
func (m *Manager) ListScenarios() []ScenarioConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 内置场景
	builtin := []ScenarioConfig{
		{
			ID:          "builtin-high",
			Name:        "高增长场景",
			Description: "日增长率提高50%",
			GrowthRate:  m.calculateGrowthRate() * 1.5,
			GrowthType:  "exponential",
			Period:      PeriodDaily,
			Duration:    365,
		},
		{
			ID:          "builtin-medium",
			Name:        "标准增长场景",
			Description: "基于历史趋势的预测",
			GrowthRate:  m.calculateGrowthRate(),
			GrowthType:  "linear",
			Period:      PeriodDaily,
			Duration:    365,
		},
		{
			ID:          "builtin-low",
			Name:        "低增长场景",
			Description: "日增长率降低50%",
			GrowthRate:  m.calculateGrowthRate() * 0.5,
			GrowthType:  "linear",
			Period:      PeriodDaily,
			Duration:    365,
		},
		{
			ID:          "builtin-stable",
			Name:        "稳定场景",
			Description: "无增长",
			GrowthRate:  0,
			GrowthType:  "linear",
			Period:      PeriodDaily,
			Duration:    365,
		},
	}

	// 自定义场景
	custom := make([]ScenarioConfig, 0, len(m.customScenarios))
	for _, s := range m.customScenarios {
		custom = append(custom, *s)
	}

	return append(builtin, custom...)
}

// SimulateScenario 模拟场景
func (m *Manager) SimulateScenario(scenarioID string) (*ScenarioResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.usageHistory) == 0 {
		return nil, fmt.Errorf("无历史使用数据")
	}

	var config ScenarioConfig
	found := false

	// 检查内置场景
	switch scenarioID {
	case "builtin-high":
		config = ScenarioConfig{
			ID:         "builtin-high",
			Name:       "高增长场景",
			GrowthRate: m.calculateGrowthRate() * 1.5,
			Period:     PeriodDaily,
			Duration:   365,
		}
		found = true
	case "builtin-medium":
		config = ScenarioConfig{
			ID:         "builtin-medium",
			Name:       "标准增长场景",
			GrowthRate: m.calculateGrowthRate(),
			Period:     PeriodDaily,
			Duration:   365,
		}
		found = true
	case "builtin-low":
		config = ScenarioConfig{
			ID:         "builtin-low",
			Name:       "低增长场景",
			GrowthRate: m.calculateGrowthRate() * 0.5,
			Period:     PeriodDaily,
			Duration:   365,
		}
		found = true
	case "builtin-stable":
		config = ScenarioConfig{
			ID:         "builtin-stable",
			Name:       "稳定场景",
			GrowthRate: 0,
			Period:     PeriodDaily,
			Duration:   365,
		}
		found = true
	default:
		// 检查自定义场景
		if s, exists := m.customScenarios[scenarioID]; exists {
			config = *s
			found = true
		}
	}

	if !found {
		return nil, fmt.Errorf("场景不存在: %s", scenarioID)
	}

	latest := m.usageHistory[len(m.usageHistory)-1]
	currentUsed := float64(latest.UsedBytes)
	forecasts := make([]ForecastPoint, 0)
	var fullDate, warningDate *time.Time

	for i := 1; i <= config.Duration; i++ {
		var date time.Time
		switch config.Period {
		case PeriodDaily:
			date = latest.Timestamp.AddDate(0, 0, i)
		case PeriodWeekly:
			date = latest.Timestamp.AddDate(0, 0, i*7)
		case PeriodMonthly:
			date = latest.Timestamp.AddDate(0, i, 0)
		case PeriodYearly:
			date = latest.Timestamp.AddDate(i, 0, 0)
		}

		currentUsed *= (1 + config.GrowthRate/100)
		usedPercent := currentUsed / float64(m.totalCapacity) * 100

		forecasts = append(forecasts, ForecastPoint{
			Date:        date,
			UsedBytes:   int64(currentUsed),
			UsedPercent: usedPercent,
		})

		// 检查是否达到告警阈值
		if warningDate == nil && usedPercent >= 80 {
			warningDate = &date
		}
		if fullDate == nil && usedPercent >= 100 {
			fullDate = &date
		}
	}

	return &ScenarioResult{
		Config:      config,
		Forecasts:   forecasts,
		FullDate:    fullDate,
		WarningDate: warningDate,
		GeneratedAt: time.Now(),
	}, nil
}

// AddAlert 添加容量告警
func (m *Manager) AddAlert(alert *CapacityAlert) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if alert.ID == "" {
		return fmt.Errorf("告警ID不能为空")
	}
	if alert.Threshold <= 0 || alert.Threshold > 100 {
		return fmt.Errorf("阈值必须在 0-100 之间")
	}
	if alert.Level == "" {
		alert.Level = AlertWarning
	}

	alert.CreatedAt = time.Now()
	alert.UpdatedAt = time.Now()
	m.alerts[alert.ID] = alert
	return nil
}

// UpdateAlert 更新告警
func (m *Manager) UpdateAlert(alert *CapacityAlert) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.alerts[alert.ID]; !exists {
		return fmt.Errorf("告警不存在: %s", alert.ID)
	}

	alert.UpdatedAt = time.Now()
	m.alerts[alert.ID] = alert
	return nil
}

// DeleteAlert 删除告警
func (m *Manager) DeleteAlert(alertID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.alerts[alertID]; !exists {
		return false
	}
	delete(m.alerts, alertID)
	return true
}

// ListAlerts 列出所有告警
func (m *Manager) ListAlerts() []AlertStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	currentUsed := float64(0)
	if len(m.usageHistory) > 0 {
		latest := m.usageHistory[len(m.usageHistory)-1]
		currentUsed = latest.UsedPercent
	}

	result := make([]AlertStatus, 0, len(m.alerts))
	for _, alert := range m.alerts {
		status := AlertStatus{
			Alert:       *alert,
			CurrentUsed: currentUsed,
		}
		if currentUsed >= alert.Threshold {
			status.Triggered = true
			status.Message = fmt.Sprintf("当前使用率 %.2f%% 已超过阈值 %.2f%%", currentUsed, alert.Threshold)
		}
		result = append(result, status)
	}
	return result
}

// EstimateCost 估算存储成本
func (m *Manager) EstimateCost(months int) *CostEstimate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if months <= 0 {
		months = 12
	}

	currentUsedGB := float64(0)
	if len(m.usageHistory) > 0 {
		latest := m.usageHistory[len(m.usageHistory)-1]
		currentUsedGB = float64(latest.UsedBytes) / (1024 * 1024 * 1024)
	}

	totalCapacityGB := float64(m.totalCapacity) / (1024 * 1024 * 1024)
	growthRate := m.calculateGrowthRate()

	projectedCosts := make([]ProjectedCost, 0)
	usedGB := currentUsedGB

	for i := 1; i <= months; i++ {
		// 按月增长
		usedGB *= (1 + growthRate*30/100)
		if usedGB > totalCapacityGB {
			usedGB = totalCapacityGB
		}

		date := time.Now().AddDate(0, i, 0)
		cost := usedGB * m.costPerGB

		projectedCosts = append(projectedCosts, ProjectedCost{
			Date:        date,
			UsedGB:      usedGB,
			MonthlyCost: cost,
			Scenario:    "medium",
		})
	}

	return &CostEstimate{
		CurrentUsedGB:      currentUsedGB,
		TotalCapacityGB:    totalCapacityGB,
		CostPerGBPerMonth:  m.costPerGB,
		CurrentMonthlyCost: currentUsedGB * m.costPerGB,
		ProjectedCosts:     projectedCosts,
		Currency:           m.currency,
		GeneratedAt:        time.Now(),
	}
}

// SetCostConfig 设置成本配置
func (m *Manager) SetCostConfig(costPerGB float64, currency string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.costPerGB = costPerGB
	m.currency = currency
}

// GenerateReport 生成容量规划报告
func (m *Manager) GenerateReport() (*CapacityReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.usageHistory) == 0 {
		return nil, fmt.Errorf("无历史使用数据")
	}

	latest := m.usageHistory[len(m.usageHistory)-1]
	totalCapacityGB := float64(m.totalCapacity) / (1024 * 1024 * 1024)
	usedGB := float64(latest.UsedBytes) / (1024 * 1024 * 1024)
	availableGB := totalCapacityGB - usedGB
	growthRate := m.calculateGrowthRate()

	// 计算天数直到满容量
	daysUntilFull := 0
	daysUntilWarning := 0
	if growthRate > 0 {
		dailyGrowthGB := usedGB * growthRate / 100
		if dailyGrowthGB > 0 {
			daysUntilFull = int(availableGB / dailyGrowthGB)
			warningThresholdGB := totalCapacityGB * 0.8
			if usedGB < warningThresholdGB {
				daysUntilWarning = int((warningThresholdGB - usedGB) / dailyGrowthGB)
			}
		}
	}

	// 健康状态
	healthStatus := "healthy"
	if latest.UsedPercent >= 90 {
		healthStatus = "critical"
	} else if latest.UsedPercent >= 80 {
		healthStatus = "warning"
	}

	summary := ReportSummary{
		TotalCapacityGB:  totalCapacityGB,
		UsedCapacityGB:   usedGB,
		AvailableGB:      availableGB,
		UsedPercent:      latest.UsedPercent,
		DaysUntilFull:    daysUntilFull,
		DaysUntilWarning: daysUntilWarning,
		GrowthRatePerDay: growthRate,
		HealthStatus:     healthStatus,
	}

	// 生成预测
	forecasts := make([]ForecastResult, 0)
	for _, period := range []ForecastPeriod{PeriodDaily, PeriodWeekly, PeriodMonthly} {
		forecast := m.forecastInternal(period, 30, ScenarioMedium)
		if forecast != nil {
			forecasts = append(forecasts, *forecast)
		}
	}

	// 生成场景模拟
	scenarios := make([]ScenarioResult, 0)
	for _, id := range []string{"builtin-high", "builtin-medium", "builtin-low"} {
		scenario := m.simulateScenarioInternal(id)
		if scenario != nil {
			scenarios = append(scenarios, *scenario)
		}
	}

	// 告警状态
	alertStatuses := make([]AlertStatus, 0)
	for _, alert := range m.alerts {
		status := AlertStatus{
			Alert:       *alert,
			CurrentUsed: latest.UsedPercent,
		}
		if latest.UsedPercent >= alert.Threshold {
			status.Triggered = true
			status.Message = fmt.Sprintf("当前使用率 %.2f%% 已超过阈值 %.2f%%", latest.UsedPercent, alert.Threshold)
		}
		alertStatuses = append(alertStatuses, status)
	}

	// 成本估算
	costEstimate := m.estimateCostInternal(12)

	// 建议
	recommendations := m.generateRecommendations(summary)

	return &CapacityReport{
		Summary:         summary,
		CurrentUsage:    latest,
		Forecasts:       forecasts,
		Scenarios:       scenarios,
		AlertStatuses:   alertStatuses,
		CostEstimate:    *costEstimate,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
	}, nil
}

// forecastInternal 内部预测方法（无锁）
func (m *Manager) forecastInternal(period ForecastPeriod, duration int, scenario GrowthScenario) *ForecastResult {
	if len(m.usageHistory) == 0 {
		return nil
	}

	latest := m.usageHistory[len(m.usageHistory)-1]
	growthRate := m.calculateGrowthRate()

	switch scenario {
	case ScenarioHigh:
		growthRate *= 1.5
	case ScenarioLow:
		growthRate *= 0.5
	case ScenarioStable:
		growthRate = 0
	}

	points := make([]ForecastPoint, 0)
	currentUsed := float64(latest.UsedBytes)

	for i := 1; i <= duration; i++ {
		var date time.Time
		switch period {
		case PeriodDaily:
			date = latest.Timestamp.AddDate(0, 0, i)
		case PeriodWeekly:
			date = latest.Timestamp.AddDate(0, 0, i*7)
		case PeriodMonthly:
			date = latest.Timestamp.AddDate(0, i, 0)
		case PeriodYearly:
			date = latest.Timestamp.AddDate(i, 0, 0)
		}

		switch period {
		case PeriodDaily:
			currentUsed *= (1 + growthRate/100)
		case PeriodWeekly:
			currentUsed *= (1 + growthRate*7/100)
		case PeriodMonthly:
			currentUsed *= (1 + growthRate*30/100)
		case PeriodYearly:
			currentUsed *= (1 + growthRate*365/100)
		}

		usedPercent := currentUsed / float64(m.totalCapacity) * 100

		points = append(points, ForecastPoint{
			Date:        date,
			UsedBytes:   int64(currentUsed),
			UsedPercent: usedPercent,
			Scenario:    scenario,
		})
	}

	return &ForecastResult{
		Period:        period,
		Points:        points,
		Scenario:      scenario,
		TotalCapacity: m.totalCapacity,
		GeneratedAt:   time.Now(),
	}
}

// simulateScenarioInternal 内部场景模拟方法（无锁）
func (m *Manager) simulateScenarioInternal(scenarioID string) *ScenarioResult {
	if len(m.usageHistory) == 0 {
		return nil
	}

	var config ScenarioConfig
	found := false

	switch scenarioID {
	case "builtin-high":
		config = ScenarioConfig{
			ID:         "builtin-high",
			Name:       "高增长场景",
			GrowthRate: m.calculateGrowthRate() * 1.5,
			Period:     PeriodDaily,
			Duration:   365,
		}
		found = true
	case "builtin-medium":
		config = ScenarioConfig{
			ID:         "builtin-medium",
			Name:       "标准增长场景",
			GrowthRate: m.calculateGrowthRate(),
			Period:     PeriodDaily,
			Duration:   365,
		}
		found = true
	case "builtin-low":
		config = ScenarioConfig{
			ID:         "builtin-low",
			Name:       "低增长场景",
			GrowthRate: m.calculateGrowthRate() * 0.5,
			Period:     PeriodDaily,
			Duration:   365,
		}
		found = true
	}

	if !found {
		return nil
	}

	latest := m.usageHistory[len(m.usageHistory)-1]
	currentUsed := float64(latest.UsedBytes)
	forecasts := make([]ForecastPoint, 0)
	var fullDate, warningDate *time.Time

	for i := 1; i <= config.Duration; i++ {
		date := latest.Timestamp.AddDate(0, 0, i)
		currentUsed *= (1 + config.GrowthRate/100)
		usedPercent := currentUsed / float64(m.totalCapacity) * 100

		forecasts = append(forecasts, ForecastPoint{
			Date:        date,
			UsedBytes:   int64(currentUsed),
			UsedPercent: usedPercent,
		})

		if warningDate == nil && usedPercent >= 80 {
			warningDate = &date
		}
		if fullDate == nil && usedPercent >= 100 {
			fullDate = &date
		}
	}

	return &ScenarioResult{
		Config:      config,
		Forecasts:   forecasts,
		FullDate:    fullDate,
		WarningDate: warningDate,
		GeneratedAt: time.Now(),
	}
}

// estimateCostInternal 内部成本估算方法（无锁）
func (m *Manager) estimateCostInternal(months int) *CostEstimate {
	currentUsedGB := float64(0)
	if len(m.usageHistory) > 0 {
		latest := m.usageHistory[len(m.usageHistory)-1]
		currentUsedGB = float64(latest.UsedBytes) / (1024 * 1024 * 1024)
	}

	totalCapacityGB := float64(m.totalCapacity) / (1024 * 1024 * 1024)
	growthRate := m.calculateGrowthRate()

	projectedCosts := make([]ProjectedCost, 0)
	usedGB := currentUsedGB

	for i := 1; i <= months; i++ {
		usedGB *= (1 + growthRate*30/100)
		if usedGB > totalCapacityGB {
			usedGB = totalCapacityGB
		}

		date := time.Now().AddDate(0, i, 0)
		cost := usedGB * m.costPerGB

		projectedCosts = append(projectedCosts, ProjectedCost{
			Date:        date,
			UsedGB:      usedGB,
			MonthlyCost: cost,
			Scenario:    "medium",
		})
	}

	return &CostEstimate{
		CurrentUsedGB:      currentUsedGB,
		TotalCapacityGB:    totalCapacityGB,
		CostPerGBPerMonth:  m.costPerGB,
		CurrentMonthlyCost: currentUsedGB * m.costPerGB,
		ProjectedCosts:     projectedCosts,
		Currency:           m.currency,
		GeneratedAt:        time.Now(),
	}
}

// generateRecommendations 生成建议
func (m *Manager) generateRecommendations(summary ReportSummary) []string {
	recommendations := make([]string, 0)

	if summary.HealthStatus == "critical" {
		recommendations = append(recommendations, "⚠️ 存储使用率已超过90%，建议立即扩容或清理数据")
	}

	if summary.HealthStatus == "warning" {
		recommendations = append(recommendations, "⚠️ 存储使用率已超过80%，建议规划扩容")
	}

	if summary.DaysUntilFull > 0 && summary.DaysUntilFull < 30 {
		recommendations = append(recommendations, fmt.Sprintf("📈 按当前增长趋势，预计 %d 天后存储将满", summary.DaysUntilFull))
	}

	if summary.DaysUntilFull > 0 && summary.DaysUntilFull < 90 {
		recommendations = append(recommendations, "💡 建议在30天内完成扩容规划")
	}

	if summary.GrowthRatePerDay > 1 {
		recommendations = append(recommendations, "📊 数据增长较快，建议启用数据分层策略优化存储")
	}

	if summary.AvailableGB < 100 {
		recommendations = append(recommendations, "💾 可用空间不足100GB，建议尽快扩容")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "✅ 存储状态良好，无需立即操作")
	}

	return recommendations
}
