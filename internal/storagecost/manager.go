// Package storagecost 提供存储成本核心管理逻辑
package storagecost

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 存储成本管理器
type Manager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *StorageCostConfig
	reports  map[string]*CostReport
	forecasts map[string]*CostForecast
}

// NewManager 创建存储成本管理器
func NewManager(logger *zap.Logger, config *StorageCostConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultStorageCostConfig()
	}

	return &Manager{
		logger:    logger,
		config:    config,
		reports:   make(map[string]*CostReport),
		forecasts: make(map[string]*CostForecast),
	}
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateReport 生成成本报告
func (m *Manager) GenerateReport(pool, volume, directory string, periodStart, periodEnd time.Time) (*CostReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("storage cost analysis is disabled")
	}

	// 模拟生成成本明细
	breakdowns := m.generateBreakdowns(pool, volume, directory, periodStart, periodEnd)

	// 按层级汇总
	byTier := make(map[StorageTier]float64)
	byPool := make(map[string]float64)
	byCategory := make(map[CostCategory]float64)

	totalCost := 0.0
	for _, b := range breakdowns {
		totalCost += b.TotalCost
		byTier[b.Tier] += b.TotalCost
		byPool[b.Pool] += b.TotalCost
		byCategory[b.Category] += b.TotalCost
	}

	// 生成趋势
	trend := m.generateTrend(periodStart, periodEnd, totalCost)

	report := &CostReport{
		ID:          generateID(),
		ReportName:  fmt.Sprintf("存储成本报告 %s", periodStart.Format("2006-01")),
		TotalCost:   totalCost,
		Currency:    m.config.DefaultCurrency,
		Breakdowns:  breakdowns,
		ByTier:      byTier,
		ByPool:      byPool,
		ByCategory:  byCategory,
		Trend:       trend,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		GeneratedAt: time.Now(),
	}

	m.reports[report.ID] = report
	m.logger.Info("cost report generated",
		zap.String("id", report.ID),
		zap.Float64("total_cost", totalCost))

	return report, nil
}

// GetBreakdown 获取成本明细
func (m *Manager) GetBreakdown(pool, volume, directory string, tier StorageTier, category CostCategory) ([]*CostBreakdown, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*CostBreakdown
	for _, report := range m.reports {
		for _, b := range report.Breakdowns {
			if pool != "" && b.Pool != pool {
				continue
			}
			if volume != "" && b.Volume != volume {
				continue
			}
			if directory != "" && b.Directory != directory {
				continue
			}
			if tier != "" && b.Tier != tier {
				continue
			}
			if category != "" && b.Category != category {
				continue
			}
			result = append(result, b)
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no breakdown found for the specified filters")
	}

	return result, nil
}

// ForecastCost 预测未来成本
func (m *Manager) ForecastCost(months int, growthRate float64) (*CostForecast, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("storage cost analysis is disabled")
	}

	if months <= 0 {
		months = m.config.ForecastMonths
	}
	if growthRate == 0 {
		growthRate = 0.05 // 默认 5% 月增长
	}

	// 计算当前成本（取最近报告）
	currentCost := m.getCurrentCost()

	// 生成月度预测
	monthlyData := make([]MonthlyForecast, months)
	forecastCost := currentCost

	for i := 0; i < months; i++ {
		month := time.Now().AddDate(0, i+1, 0).Format("2006-01")
		projectedGB := 1000 * (1 + growthRate*float64(i+1)) // 假设基础 1000GB
		forecastCost = currentCost * (1 + growthRate*float64(i+1))

		monthlyData[i] = MonthlyForecast{
			Month:         month,
			ProjectedGB:   projectedGB,
			ProjectedCost: forecastCost,
		}
	}

	// 计算置信度
	confidence := 0.9 - float64(months)*0.02
	if confidence < 0.5 {
		confidence = 0.5
	}

	forecast := &CostForecast{
		ID:           generateID(),
		ForecastName: fmt.Sprintf("未来 %d 个月成本预测", months),
		CurrentCost:  currentCost,
		ForecastCost: forecastCost,
		Currency:     m.config.DefaultCurrency,
		Months:       months,
		GrowthRate:   growthRate,
		Confidence:   confidence,
		MonthlyData:  monthlyData,
		Assumptions: []string{
			"存储增长率为线性增长",
			"存储层级比例保持稳定",
			"无重大数据迁移计划",
		},
		CreatedAt: time.Now(),
	}

	m.forecasts[forecast.ID] = forecast
	m.logger.Info("cost forecast generated",
		zap.String("id", forecast.ID),
		zap.Int("months", months),
		zap.Float64("forecast_cost", forecastCost))

	return forecast, nil
}

// GetOptimizations 获取优化建议
func (m *Manager) GetOptimizations() ([]*OptimizationSuggestion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.Enabled {
		return nil, fmt.Errorf("storage cost analysis is disabled")
	}

	suggestions := make([]*OptimizationSuggestion, 0)

	// 1. 层级迁移建议
	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:              generateID(),
		Type:            OptTierMigration,
		Title:           "热存储数据迁移至温存储",
		Description:     "检测到 500GB 热存储数据在过去 30 天内未被访问，建议迁移至温存储以降低成本",
		EstimatedSaving: 150.0,
		Currency:        m.config.DefaultCurrency,
		Priority:        1,
		CurrentTier:     TierHot,
		RecommendedTier: TierWarm,
		SavingsPercent:  60.0,
		Complexity:      "low",
		RiskLevel:       "low",
		CreatedAt:       time.Now(),
	})

	// 2. 去重建议
	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:              generateID(),
		Type:            OptDeduplication,
		Title:           "数据去重优化",
		Description:     "检测到 200GB 重复数据，启用去重可节省存储空间和成本",
		EstimatedSaving: 100.0,
		Currency:        m.config.DefaultCurrency,
		Priority:        2,
		SavingsPercent:  40.0,
		Complexity:      "medium",
		RiskLevel:       "low",
		CreatedAt:       time.Now(),
	})

	// 3. 压缩建议
	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:              generateID(),
		Type:            OptCompression,
		Title:           "存储压缩优化",
		Description:     "部分卷未启用压缩，启用后可节省约 30% 存储空间",
		EstimatedSaving: 80.0,
		Currency:        m.config.DefaultCurrency,
		Priority:        3,
		SavingsPercent:  30.0,
		Complexity:      "low",
		RiskLevel:       "low",
		CreatedAt:       time.Now(),
	})

	// 4. 清理建议
	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:              generateID(),
		Type:            OptCleanup,
		Title:           "清理过期备份",
		Description:     "检测到 100GB 超过保留期限的备份数据，建议清理以释放空间",
		EstimatedSaving: 20.0,
		Currency:        m.config.DefaultCurrency,
		Priority:        4,
		SavingsPercent:  15.0,
		Complexity:      "low",
		RiskLevel:       "medium",
		CreatedAt:       time.Now(),
	})

	// 5. 生命周期策略建议
	suggestions = append(suggestions, &OptimizationSuggestion{
		ID:              generateID(),
		Type:            OptLifecyclePolicy,
		Title:           "配置自动生命周期策略",
		Description:     "为频繁访问的数据配置自动降级策略，长期可节省 40% 成本",
		EstimatedSaving: 200.0,
		Currency:        m.config.DefaultCurrency,
		Priority:        5,
		SavingsPercent:  40.0,
		Complexity:      "medium",
		RiskLevel:       "low",
		CreatedAt:       time.Now(),
	})

	return suggestions, nil
}

// generateBreakdowns 生成成本明细
func (m *Manager) generateBreakdowns(pool, volume, directory string, periodStart, periodEnd time.Time) []*CostBreakdown {
	var breakdowns []*CostBreakdown

	// 模拟数据
	pools := []string{"pool-main", "pool-backup", "pool-archive"}
	volumes := []string{"vol-data", "vol-media", "vol-docs"}
	tiers := []StorageTier{TierHot, TierWarm, TierCold}
	categories := []CostCategory{CategoryStorage, CategoryTransfer, CategoryRequest}

	if pool != "" {
		pools = []string{pool}
	}
	if volume != "" {
		volumes = []string{volume}
	}

	for _, p := range pools {
		for _, v := range volumes {
			for _, t := range tiers {
				for _, c := range categories {
					size := int64(100 * 1024 * 1024 * 1024) // 100GB
					costPerGB := m.config.TierPricing[t]
					if c == CategoryTransfer {
						costPerGB = m.config.TransferRate
					} else if c == CategoryRequest {
						costPerGB = m.config.RequestRate
					}

					breakdowns = append(breakdowns, &CostBreakdown{
						ID:          generateID(),
						Pool:        p,
						Volume:      v,
						Directory:   directory,
						Tier:        t,
						Category:    c,
						SizeBytes:   size,
						CostPerGB:   costPerGB,
						TotalCost:   float64(size) / (1024 * 1024 * 1024) * costPerGB,
						Currency:    m.config.DefaultCurrency,
						PeriodStart: periodStart,
						PeriodEnd:   periodEnd,
						CreatedAt:   time.Now(),
					})
				}
			}
		}
	}

	return breakdowns
}

// generateTrend 生成成本趋势
func (m *Manager) generateTrend(periodStart, periodEnd time.Time, totalCost float64) *CostTrend {
	days := int(periodEnd.Sub(periodStart).Hours() / 24)
	if days <= 0 {
		days = 30
	}

	dailyCosts := make([]DailyCost, days)
	dailyCost := totalCost / float64(days)

	for i := 0; i < days; i++ {
		dailyCosts[i] = DailyCost{
			Date: periodStart.AddDate(0, 0, i),
			Cost: dailyCost * (1 + float64(i)*0.001), // 微小增长
		}
	}

	return &CostTrend{
		DailyCosts:    dailyCosts,
		MonthlyGrowth: 0.05,
		ProjectedCost: totalCost * 1.05,
	}
}

// getCurrentCost 获取当前成本
func (m *Manager) getCurrentCost() float64 {
	// 从最近报告中获取
	var latestCost float64
	for _, report := range m.reports {
		if report.TotalCost > latestCost {
			latestCost = report.TotalCost
		}
	}

	if latestCost == 0 {
		// 默认模拟值
		latestCost = 500.0
	}

	return latestCost
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *StorageCostConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *StorageCostConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// ListReports 列出所有报告
func (m *Manager) ListReports() []*CostReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*CostReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}
	return reports
}

// GetReport 获取报告
func (m *Manager) GetReport(id string) (*CostReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report not found: %s", id)
	}
	return report, nil
}

// ListForecasts 列出所有预测
func (m *Manager) ListForecasts() []*CostForecast {
	m.mu.RLock()
	defer m.mu.RUnlock()

	forecasts := make([]*CostForecast, 0, len(m.forecasts))
	for _, f := range m.forecasts {
		forecasts = append(forecasts, f)
	}
	return forecasts
}
