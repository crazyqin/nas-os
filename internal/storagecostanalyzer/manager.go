package storagecostanalyzer

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// Manager 存储成本分析器管理器.
type Manager struct {
	mu      sync.RWMutex
	config  *Config
	tiers   map[StorageTier]*tierState
	records []CostRecord
	reports []*CostReport
	running bool
	stopCh  chan struct{}
	nextID  atomic.Int64
	nowFunc func() time.Time // 用于测试的时间函数
}

type tierState struct {
	config  TierConfig
	records []CostRecord
}

// NewManager 创建新的存储成本分析器管理器.
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{
			Enabled:              true,
			Currency:             "CNY",
			ReportRetentionDays:  90,
			ForecastMonths:       12,
			AlertThreshold:       80.0,
			AutoAnalyze:          true,
			AnalyzeIntervalHours: 24,
		}
	}
	return &Manager{
		config:  config,
		tiers:   make(map[StorageTier]*tierState),
		records: make([]CostRecord, 0),
		reports: make([]*CostReport, 0),
		stopCh:  make(chan struct{}),
		nowFunc: time.Now,
	}
}

// Start 启动分析器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return ErrAlreadyRunning
	}
	if m.config == nil || !m.config.Enabled {
		return ErrInvalidConfig
	}

	m.running = true
	m.stopCh = make(chan struct{})

	if m.config.AutoAnalyze {
		go m.autoAnalyzeLoop()
	}
	return nil
}

// Stop 停止分析器.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return ErrNotRunning
	}

	close(m.stopCh)
	m.running = false
	return nil
}

// IsRunning 返回分析器是否运行中.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// RegisterTier 注册存储层级.
func (m *Manager) RegisterTier(tier StorageTier, cfg TierConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg.Tier = tier
	if cfg.CapacityTB <= 0 {
		return fmt.Errorf("%w: capacity must be positive", ErrInvalidConfig)
	}
	if cfg.CostPerTBMonth < 0 {
		return fmt.Errorf("%w: cost per TB must be non-negative", ErrInvalidConfig)
	}
	if cfg.UsedTB > cfg.CapacityTB {
		return fmt.Errorf("%w: used exceeds capacity", ErrInvalidConfig)
	}

	m.tiers[tier] = &tierState{
		config:  cfg,
		records: make([]CostRecord, 0),
	}
	return nil
}

// RecordCost 记录成本.
func (m *Manager) RecordCost(tier StorageTier, category CostCategory, amount float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ts, ok := m.tiers[tier]
	if !ok {
		return ErrTierNotFound
	}

	id := fmt.Sprintf("cost-%d", m.nextID.Add(1))
	capacityTB := ts.config.CapacityTB
	costPerTB := 0.0
	if capacityTB > 0 {
		costPerTB = amount / capacityTB
	}

	record := CostRecord{
		ID:          id,
		Timestamp:   m.nowFunc(),
		Tier:        tier,
		Category:    category,
		Amount:      amount,
		CapacityTB:  capacityTB,
		CostPerTB:   costPerTB,
		Provider:    "local",
	}

	ts.records = append(ts.records, record)
	m.records = append(m.records, record)
	return nil
}

// CalculateCostPerTB 计算某层级每TB成本.
func (m *Manager) CalculateCostPerTB(tier StorageTier) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ts, ok := m.tiers[tier]
	if !ok {
		return 0, ErrTierNotFound
	}

	if ts.config.CapacityTB <= 0 {
		return 0, nil
	}

	totalCost := 0.0
	for _, r := range ts.records {
		totalCost += r.Amount
	}

	if len(ts.records) == 0 {
		return ts.config.CostPerTBMonth, nil
	}

	return totalCost / ts.config.CapacityTB, nil
}

// PredictCapacity 预测容量趋势.
func (m *Manager) PredictCapacity(months int) (*CapacityTrend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if months <= 0 {
		months = m.config.ForecastMonths
	}

	type tierForecast struct {
		Tier            StorageTier
		CurrentUsedTB   float64
		CapacityTB      float64
		GrowthPerMonth  float64
		FullInMonths    float64
	}

	var forecasts []tierForecast
	for tier, ts := range m.tiers {
		used := ts.config.UsedTB
		cap := ts.config.CapacityTB
		growthPerMonth := estimateGrowthPerMonth(ts, m.nowFunc())
		fullInMonths := -1.0
		if growthPerMonth > 0 && used < cap {
			fullInMonths = (cap - used) / growthPerMonth
		}
		forecasts = append(forecasts, tierForecast{
			Tier:           tier,
			CurrentUsedTB:  used,
			CapacityTB:     cap,
			GrowthPerMonth: growthPerMonth,
			FullInMonths:   fullInMonths,
		})
	}

	if len(forecasts) == 0 {
		return nil, ErrInsufficientData
	}

	totalUsed := 0.0
	totalCap := 0.0
	for _, f := range forecasts {
		totalUsed += f.CurrentUsedTB
		totalCap += f.CapacityTB
	}

	// 生成月度预测
	monthPoints := make([]CapacityPoint, 0, months+1)
	now := m.nowFunc()
	for i := 0; i <= months; i++ {
		date := now.AddDate(0, i, 0)
		projUsed := totalUsed
		for _, f := range forecasts {
			projUsed += f.GrowthPerMonth * float64(i)
		}
		if projUsed > totalCap {
			projUsed = totalCap
		}
		utilization := 0.0
		if totalCap > 0 {
			utilization = (projUsed / totalCap) * 100
		}
		monthPoints = append(monthPoints, CapacityPoint{
			Date:        date,
			UsedTB:      projUsed,
			TotalTB:     totalCap,
			Utilization: utilization,
		})
	}

	// 生成建议
	var suggestions []string
	for _, f := range forecasts {
		utilization := 0.0
		if f.CapacityTB > 0 {
			utilization = (f.CurrentUsedTB / f.CapacityTB) * 100
		}
		if utilization > m.config.AlertThreshold {
			suggestions = append(suggestions,
				fmt.Sprintf("层级 %s 利用率 %.1f%% 超过阈值 %.1f%%，建议扩容", f.Tier, utilization, m.config.AlertThreshold))
		}
		if f.FullInMonths > 0 && f.FullInMonths <= float64(months) {
			suggestions = append(suggestions,
				fmt.Sprintf("层级 %s 预计 %.0f 个月后满容量", f.Tier, f.FullInMonths))
		}
	}

	overallGrowth := 0.0
	for _, f := range forecasts {
		overallGrowth += f.GrowthPerMonth
	}

	trend := &CapacityTrend{
		GeneratedAt:   now,
		TotalUsedTB:   totalUsed,
		TotalCapacityTB: totalCap,
		GrowthRateTBPerMonth: overallGrowth,
		Months:        monthPoints,
		Suggestions:   suggestions,
	}
	return trend, nil
}

// GenerateReport 生成成本报告.
func (m *Manager) GenerateReport(period string) (*CostReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := m.nowFunc()
	var periodStart, periodEnd time.Time
	reportType := period

	switch period {
	case "monthly":
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd = periodStart.AddDate(0, 1, 0).Add(-time.Nanosecond)
	case "quarterly":
		quarter := (int(now.Month()) - 1) / 3
		periodStart = time.Date(now.Year(), time.Month(quarter*3+1), 1, 0, 0, 0, 0, now.Location())
		periodEnd = periodStart.AddDate(0, 3, 0).Add(-time.Nanosecond)
	case "yearly":
		periodStart = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		periodEnd = time.Date(now.Year(), 12, 31, 23, 59, 59, 999999999, now.Location())
	default:
		return nil, fmt.Errorf("%w: unsupported period %s", ErrInvalidConfig, period)
	}

	id := fmt.Sprintf("report-%s-%d", period, now.UnixNano())
	totalCost := 0.0
	totalCap := 0.0
	totalUsed := 0.0
	var breakdowns []TierCostBreakdown
	var costDrivers []CostDriver

	categoryTotals := make(map[CostCategory]float64)
	tierCosts := make(map[StorageTier]float64)

	for tier, ts := range m.tiers {
		tierCost := 0.0
		for _, r := range ts.records {
			if !r.Timestamp.Before(periodStart) && !r.Timestamp.After(periodEnd) {
				tierCost += r.Amount
				categoryTotals[r.Category] += r.Amount
			}
		}
		tierCosts[tier] = tierCost
	}

	for tier, ts := range m.tiers {
		capacityTB := ts.config.CapacityTB
		usedTB := ts.config.UsedTB
		utilization := 0.0
		if capacityTB > 0 {
			utilization = (usedTB / capacityTB) * 100
		}
		costPerTB := ts.config.CostPerTBMonth
		tierCost := tierCosts[tier]
		monthlyCost := usedTB * costPerTB
		if tierCost > 0 {
			monthlyCost = tierCost
		}

		totalCost += monthlyCost
		totalCap += capacityTB
		totalUsed += usedTB

		costShare := 0.0
		breakdowns = append(breakdowns, TierCostBreakdown{
			Tier:        tier,
			TierName:    ts.config.Name,
			CapacityTB:  capacityTB,
			UsedTB:      usedTB,
			Utilization: utilization,
			CostPerTB:   costPerTB,
			MonthlyCost: monthlyCost,
			CostShare:   costShare,
		})
	}

	// 计算成本占比
	for i := range breakdowns {
		if totalCost > 0 {
			breakdowns[i].CostShare = (breakdowns[i].MonthlyCost / totalCost) * 100
		}
	}

	avgCostPerTB := 0.0
	if totalUsed > 0 {
		avgCostPerTB = totalCost / totalUsed
	}

	overallUtil := 0.0
	if totalCap > 0 {
		overallUtil = (totalUsed / totalCap) * 100
	}

	// 计算浪费成本
	wastedCost := 0.0
	for _, b := range breakdowns {
		unusedTB := b.CapacityTB - b.UsedTB
		if unusedTB > 0 {
			wastedCost += unusedTB * b.CostPerTB
		}
	}

	// 成本驱动因素
	totalCatCost := 0.0
	for _, v := range categoryTotals {
		totalCatCost += v
	}
	for cat, amount := range categoryTotals {
		pct := 0.0
		if totalCatCost > 0 {
			pct = (amount / totalCatCost) * 100
		}
		costDrivers = append(costDrivers, CostDriver{
			Category:    cat,
			Amount:      amount,
			Percentage:  pct,
			Trend:       "stable",
			Description: string(cat),
		})
	}

	report := &CostReport{
		ID:                  id,
		Title:               fmt.Sprintf("%s 成本分析报告", period),
		ReportType:          reportType,
		PeriodStart:         periodStart,
		PeriodEnd:           periodEnd,
		GeneratedAt:         now,
		TotalCost:           totalCost,
		TotalCapacityTB:     totalCap,
		TotalUsedTB:         totalUsed,
		AvgCostPerTB:        avgCostPerTB,
		OverallUtilization:  overallUtil,
		WastedCost:          wastedCost,
		TierBreakdown:       breakdowns,
		CostTrend:           []TrendPoint{},
		CostChangePercent:   0,
		TopCostDrivers:      costDrivers,
		OptimizationSavings: wastedCost * 0.3, // 简单估算
	}

	m.reports = append(m.reports, report)
	return report, nil
}

// GetOptimizationSuggestions 获取优化建议.
func (m *Manager) GetOptimizationSuggestions() []*OptimizationSuggestion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var suggestions []*OptimizationSuggestion
	id := 0

	for _, ts := range m.tiers {
		utilization := 0.0
		if ts.config.CapacityTB > 0 {
			utilization = (ts.config.UsedTB / ts.config.CapacityTB) * 100
		}

		// 高利用率建议扩容
		if utilization > m.config.AlertThreshold {
			id++
			extraTB := ts.config.CapacityTB * 0.5 // 建议扩容50%
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", id),
				Title:       fmt.Sprintf("%s 层级利用率过高", ts.config.Name),
				Category:    "rightsizing",
				Priority:    PriorityHigh,
				SourceTier:  ts.config.Tier,
				TargetTier:  ts.config.Tier,
				AffectedTB:  ts.config.UsedTB,
				CurrentCost: ts.config.UsedTB * ts.config.CostPerTBMonth,
				AnnualSavings: 0,
				Description: fmt.Sprintf("层级 %s 利用率 %.1f%%，建议扩容 %.1f TB", ts.config.Name, utilization, extraTB),
				Rationale:   "高利用率可能导致性能下降和服务中断",
				Steps:       []string{"评估增长趋势", "采购额外存储", "完成扩容"},
				Impact:      "high",
				Effort:      "medium",
			})
		}

		// 低利用率建议缩减或迁移到冷存储
		if utilization < 30 && ts.config.Tier == TierSSD {
			id++
			savings := (ts.config.CapacityTB - ts.config.UsedTB) * ts.config.CostPerTBMonth * 12
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:             fmt.Sprintf("opt-%d", id),
				Title:          fmt.Sprintf("%s SSD 利用率过低，建议分层", ts.config.Name),
				Category:       "tiering",
				Priority:       PriorityMedium,
				SourceTier:     TierSSD,
				TargetTier:     TierHDD,
				AffectedTB:     ts.config.UsedTB,
				CurrentCost:    ts.config.CapacityTB * ts.config.CostPerTBMonth,
				OptimizedCost:  ts.config.UsedTB * ts.config.CostPerTBMonth,
				AnnualSavings:  savings,
				Description:    fmt.Sprintf("SSD 层级 %s 利用率仅 %.1f%%，建议迁移冷数据到 HDD", ts.config.Name, utilization),
				Rationale:      "低利用率的 SSD 存储性价比低",
				Steps:          []string{"分析数据热度", "制定分层策略", "执行数据迁移", "缩减 SSD 容量"},
				Impact:         "medium",
				Effort:         "medium",
			})
		}

		// 云存储成本优化
		if ts.config.Tier == TierCloud && ts.config.CostPerTBMonth > 200 {
			id++
			annualCost := ts.config.UsedTB * ts.config.CostPerTBMonth * 12
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:             fmt.Sprintf("opt-%d", id),
				Title:          "云存储成本优化",
				Category:       "migration",
				Priority:       PriorityMedium,
				SourceTier:     TierCloud,
				TargetTier:     TierCold,
				AffectedTB:     ts.config.UsedTB,
				CurrentCost:    annualCost,
				OptimizedCost:  annualCost * 0.4,
				AnnualSavings:  annualCost * 0.6,
				Description:    "云存储单位成本较高，建议将冷数据迁移到归档层",
				Rationale:      "归档存储成本通常是标准云存储的 20-40%",
				Steps:          []string{"识别冷数据", "配置生命周期策略", "自动归档"},
				Impact:         "high",
				Effort:         "low",
			})
		}
	}

	return suggestions
}

// GetDashboard 获取仪表板统计数据.
func (m *Manager) GetDashboard() *DashboardStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := m.nowFunc()
	totalMonthly := 0.0
	totalCap := 0.0
	totalUsed := 0.0
	var tierStats []TierStatsSummary
	var alerts []CostAlert

	for tier, ts := range m.tiers {
		capacityTB := ts.config.CapacityTB
		usedTB := ts.config.UsedTB
		utilization := 0.0
		if capacityTB > 0 {
			utilization = (usedTB / capacityTB) * 100
		}
		monthlyCost := usedTB * ts.config.CostPerTBMonth

		totalMonthly += monthlyCost
		totalCap += capacityTB
		totalUsed += usedTB

		tierStats = append(tierStats, TierStatsSummary{
			Tier:        tier,
			Name:        ts.config.Name,
			CapacityTB:  capacityTB,
			UsedTB:      usedTB,
			Utilization: utilization,
			MonthlyCost: monthlyCost,
			CostPerTB:   ts.config.CostPerTBMonth,
		})

		if utilization > m.config.AlertThreshold {
			alerts = append(alerts, CostAlert{
				ID:        fmt.Sprintf("alert-%s", tier),
				Level:     "warning",
				Tier:      tier,
				Message:   fmt.Sprintf("%s 利用率 %.1f%% 超过阈值", ts.config.Name, utilization),
				Value:     utilization,
				Threshold: m.config.AlertThreshold,
				CreatedAt: now,
			})
		}
	}

	overallUtil := 0.0
	if totalCap > 0 {
		overallUtil = (totalUsed / totalCap) * 100
	}

	avgCost := 0.0
	if totalUsed > 0 {
		avgCost = totalMonthly / totalUsed
	}

	potentialSavings := 0.0
	for _, s := range m.GetOptimizationSuggestions() {
		potentialSavings += s.AnnualSavings
	}

	nextAnalyze := now.Add(time.Duration(m.config.AnalyzeIntervalHours) * time.Hour)

	return &DashboardStats{
		TotalMonthlyCost:     totalMonthly,
		TotalCapacityTB:      totalCap,
		TotalUsedTB:          totalUsed,
		OverallUtilization:   overallUtil,
		AvgCostPerTB:         avgCost,
		TierCount:            len(m.tiers),
		MonthlyReports:       len(m.reports),
		PendingOptimizations: len(m.GetOptimizationSuggestions()),
		PotentialAnnualSavings: potentialSavings,
		CostChangePercent:    0,
		LastAnalyzeTime:      now,
		NextAnalyzeTime:      nextAnalyze,
		TierStats:            tierStats,
		Alerts:               alerts,
	}
}

// autoAnalyzeLoop 自动分析循环.
func (m *Manager) autoAnalyzeLoop() {
	interval := time.Duration(m.config.AnalyzeIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			// 自动分析逻辑（简化）
			m.mu.Lock()
			m.mu.Unlock()
		}
	}
}

// estimateGrowthPerMonth 估算月增长量.
func estimateGrowthPerMonth(ts *tierState, now time.Time) float64 {
	if len(ts.records) < 2 {
		// 没有足够数据，假设每月增长当前使用量的 5%
		return ts.config.UsedTB * 0.05
	}

	// 使用简单线性回归估算增长
	first := ts.records[0]
	last := ts.records[len(ts.records)-1]
	duration := last.Timestamp.Sub(first.Timestamp)
	if duration <= 0 {
		return 0
	}

	months := duration.Hours() / (24 * 30)
	if months <= 0 {
		return 0
	}

	costGrowth := last.Amount - first.Amount
	if costGrowth <= 0 {
		return 0
	}

	// 估算数据增长（假设成本增长与数据量成正比）
	if first.Amount <= 0 {
		return 0
	}
	growthRatio := costGrowth / first.Amount
	currentUsed := ts.config.UsedTB
	estimatedGrowth := currentUsed * growthRatio / months

	return math.Max(0, estimatedGrowth)
}

// CapacityTrend 容量趋势.
type CapacityTrend struct {
	GeneratedAt         time.Time       `json:"generatedAt"`
	TotalUsedTB         float64         `json:"totalUsedTB"`
	TotalCapacityTB     float64         `json:"totalCapacityTB"`
	GrowthRateTBPerMonth float64        `json:"growthRateTBPerMonth"`
	Months              []CapacityPoint `json:"months"`
	Suggestions         []string        `json:"suggestions"`
}

// CapacityPoint 容量数据点.
type CapacityPoint struct {
	Date        time.Time `json:"date"`
	UsedTB      float64   `json:"usedTB"`
	TotalTB     float64   `json:"totalTB"`
	Utilization float64   `json:"utilization"`
}
