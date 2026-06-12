// Package smartstoragecostopt 提供智能存储成本优化分析
package smartstoragecostopt

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// 错误定义
var (
	ErrStorageNotFound = errors.New("存储池不存在")
	ErrInvalidInput    = errors.New("无效输入参数")
	ErrNoData          = errors.New("无数据")
)

// StorageTier 存储层级
type StorageTier string

const (
	TierHot     StorageTier = "hot"     // 热存储 - SSD/NVMe
	TierWarm    StorageTier = "warm"    // 温存储 - SAS
	TierCold    StorageTier = "cold"    // 冷存储 - SATA/HDD
	TierArchive StorageTier = "archive" // 归档 - 磁带/对象存储
)

// CostUnit 成本单位
type CostUnit string

const (
	CostPerGBMonth CostUnit = "per_gb_month" // 每GB每月
	CostPerTBMonth CostUnit = "per_tb_month" // 每TB每月
)

// StoragePool 存储池
type StoragePool struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Tier         StorageTier `json:"tier"`
	TotalGB      float64     `json:"total_gb"`
	UsedGB       float64     `json:"used_gb"`
	FreeGB       float64     `json:"free_gb"`
	UsagePercent float64     `json:"usage_percent"`
	CostPerGB    float64     `json:"cost_per_gb"` // 每GB每月成本
	Provider     string      `json:"provider"`     // 本地/云提供商
	Region       string      `json:"region,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// CostRecord 成本记录
type CostRecord struct {
	ID          string      `json:"id"`
	PoolID      string      `json:"pool_id"`
	Tier        StorageTier `json:"tier"`
	UsedGB      float64     `json:"used_gb"`
	CostUSD     float64     `json:"cost_usd"`
	Period      string      `json:"period"` // 2026-01, 2026-02...
	RecordedAt  time.Time   `json:"recorded_at"`
}

// CostForecast 成本预测
type CostForecast struct {
	PoolID          string    `json:"pool_id"`
	CurrentCostUSD  float64   `json:"current_cost_usd"`
	PredictedCostUSD float64  `json:"predicted_cost_usd"`
	GrowthRate      float64   `json:"growth_rate"`
	MonthsAhead     int       `json:"months_ahead"`
	Confidence      float64   `json:"confidence"`
	ForecastAt      time.Time `json:"forecast_at"`
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	ID           string      `json:"id"`
	Type         string      `json:"type"` // tier_migration, dedup, compression, cleanup
	Priority     string      `json:"priority"` // high, medium, low
	Title        string      `json:"title"`
	Description  string      `json:"description"`
	SourcePool   string      `json:"source_pool,omitempty"`
	TargetTier   StorageTier `json:"target_tier,omitempty"`
	EstimatedGB  float64     `json:"estimated_gb"`
	SavingsUSD   float64     `json:"savings_usd"`
	SavingsPercent float64   `json:"savings_percent"`
	CreatedAt    time.Time   `json:"created_at"`
}

// CostBreakdown 成本分解
type CostBreakdown struct {
	Tier         StorageTier `json:"tier"`
	UsedGB       float64     `json:"used_gb"`
	CostUSD      float64     `json:"cost_usd"`
	PercentTotal float64     `json:"percent_total"`
}

// ROIAnalysis ROI分析
type ROIAnalysis struct {
	TotalInvestmentUSD float64 `json:"total_investment_usd"`
	AnnualSavingsUSD   float64 `json:"annual_savings_usd"`
	ROI                float64 `json:"roi"`
	PaybackMonths     int     `json:"payback_months"`
	TimelineYears     int     `json:"timeline_years"`
	NPVUSD            float64 `json:"npv_usd"`
}

// Manager 智能存储成本管理器
type Manager struct {
	mu          sync.RWMutex
	pools       map[string]*StoragePool
	records     []*CostRecord
	suggestions []*OptimizationSuggestion
	startTime   time.Time
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		pools:       make(map[string]*StoragePool),
		records:     make([]*CostRecord, 0),
		suggestions: make([]*OptimizationSuggestion, 0),
		startTime:   time.Now(),
	}
}

// CreatePool 创建存储池
func (m *Manager) CreatePool(pool *StoragePool) error {
	if pool == nil || pool.ID == "" || pool.Name == "" {
		return ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	pool.UsagePercent = pool.UsedGB / pool.TotalGB * 100
	pool.FreeGB = pool.TotalGB - pool.UsedGB
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()
	m.pools[pool.ID] = pool

	return nil
}

// GetPool 获取存储池
func (m *Manager) GetPool(poolID string) (*StoragePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, ErrStorageNotFound
	}
	return pool, nil
}

// ListPools 列出存储池
func (m *Manager) ListPools() []*StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*StoragePool, 0, len(m.pools))
	for _, pool := range m.pools {
		result = append(result, pool)
	}
	return result
}

// UpdatePoolUsage 更新存储池使用量
func (m *Manager) UpdatePoolUsage(poolID string, usedGB float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return ErrStorageNotFound
	}

	pool.UsedGB = usedGB
	pool.FreeGB = pool.TotalGB - usedGB
	pool.UsagePercent = usedGB / pool.TotalGB * 100
	pool.UpdatedAt = time.Now()

	return nil
}

// RecordCost 记录成本
func (m *Manager) RecordCost(record *CostRecord) error {
	if record == nil || record.PoolID == "" {
		return ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	record.RecordedAt = time.Now()
	m.records = append(m.records, record)

	return nil
}

// GetCostHistory 获取成本历史
func (m *Manager) GetCostHistory(poolID string, months int) []*CostRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*CostRecord, 0)
	for i := len(m.records) - 1; i >= 0; i-- {
		if poolID == "" || m.records[i].PoolID == poolID {
			result = append(result, m.records[i])
			if months > 0 && len(result) >= months {
				break
			}
		}
	}
	return result
}

// CalculateTotalCost 计算总成本
func (m *Manager) CalculateTotalCost() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0.0
	for _, pool := range m.pools {
		total += pool.UsedGB * pool.CostPerGB
	}
	return total
}

// GetCostBreakdown 获取成本分解
func (m *Manager) GetCostBreakdown() []*CostBreakdown {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCost := 0.0
	for _, pool := range m.pools {
		totalCost += pool.UsedGB * pool.CostPerGB
	}

	breakdown := make(map[StorageTier]*CostBreakdown)
	for _, pool := range m.pools {
		cost := pool.UsedGB * pool.CostPerGB
		if b, exists := breakdown[pool.Tier]; exists {
			b.UsedGB += pool.UsedGB
			b.CostUSD += cost
		} else {
			breakdown[pool.Tier] = &CostBreakdown{
				Tier:    pool.Tier,
				UsedGB:  pool.UsedGB,
				CostUSD: cost,
			}
		}
	}

	result := make([]*CostBreakdown, 0)
	for _, b := range breakdown {
		if totalCost > 0 {
			b.PercentTotal = b.CostUSD / totalCost * 100
		}
		result = append(result, b)
	}

	return result
}

// ForecastCost 预测成本
func (m *Manager) ForecastCost(poolID string, monthsAhead int) (*CostForecast, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, ErrStorageNotFound
	}

	currentCost := pool.UsedGB * pool.CostPerGB

	// 基于历史数据计算增长率
	growthRate := 0.05 // 默认5%月增长率
	if len(m.records) > 1 {
		var recentCosts []float64
		for _, r := range m.records {
			if r.PoolID == poolID {
				recentCosts = append(recentCosts, r.CostUSD)
			}
		}
		if len(recentCosts) > 1 {
			growth := (recentCosts[len(recentCosts)-1] - recentCosts[0]) / recentCosts[0]
			growthRate = growth / float64(len(recentCosts))
		}
	}

	predictedCost := currentCost
	for i := 0; i < monthsAhead; i++ {
		predictedCost *= (1 + growthRate)
	}

	return &CostForecast{
		PoolID:           poolID,
		CurrentCostUSD:   currentCost,
		PredictedCostUSD: predictedCost,
		GrowthRate:       growthRate,
		MonthsAhead:      monthsAhead,
		Confidence:       0.85,
		ForecastAt:       time.Now(),
	}, nil
}

// GenerateOptimizationSuggestions 生成优化建议
func (m *Manager) GenerateOptimizationSuggestions() []*OptimizationSuggestion {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.suggestions = make([]*OptimizationSuggestion, 0)

	for _, pool := range m.pools {
		// 建议1: 使用率低的热存储迁移到冷存储
		if pool.Tier == TierHot && pool.UsagePercent < 30 {
			savings := pool.UsedGB * (pool.CostPerGB - 0.01) // 假设冷存储成本0.01/GB
			m.suggestions = append(m.suggestions, &OptimizationSuggestion{
				ID:             fmt.Sprintf("opt-%s-%d", pool.ID, time.Now().UnixNano()),
				Type:           "tier_migration",
				Priority:       "high",
				Title:          fmt.Sprintf("将 %s 迁移到冷存储", pool.Name),
				Description:    fmt.Sprintf("热存储使用率仅 %.1f%%，建议将低频数据迁移到冷存储层", pool.UsagePercent),
				SourcePool:     pool.ID,
				TargetTier:     TierCold,
				EstimatedGB:    pool.UsedGB * 0.7,
				SavingsUSD:     savings * 0.7,
				SavingsPercent: 50,
				CreatedAt:      time.Now(),
			})
		}

		// 建议2: 使用率高的存储池扩容或清理
		if pool.UsagePercent > 80 {
			m.suggestions = append(m.suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%s-%d", pool.ID, time.Now().UnixNano()),
				Type:        "cleanup",
				Priority:    "medium",
				Title:       fmt.Sprintf("清理 %s 中的冗余数据", pool.Name),
				Description: fmt.Sprintf("存储池使用率 %.1f%%，建议清理重复和过期数据", pool.UsagePercent),
				SourcePool:  pool.ID,
				EstimatedGB: pool.UsedGB * 0.1,
				SavingsUSD:  pool.UsedGB * 0.1 * pool.CostPerGB,
				CreatedAt:   time.Now(),
			})
		}
	}

	return m.suggestions
}

// GetSuggestions 获取优化建议
func (m *Manager) GetSuggestions() []*OptimizationSuggestion {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.suggestions
}

// CalculateROI 计算ROI
func (m *Manager) CalculateROI(investmentUSD, annualSavingsUSD float64, years int) *ROIAnalysis {
	roi := 0.0
	if investmentUSD > 0 {
		roi = (annualSavingsUSD*float64(years) - investmentUSD) / investmentUSD * 100
	}

	paybackMonths := 0
	if annualSavingsUSD > 0 {
		paybackMonths = int(investmentUSD / annualSavingsUSD * 12)
	}

	// 简单NPV计算（假设8%折现率）
	npv := -investmentUSD
	discountRate := 0.08
	for i := 1; i <= years; i++ {
		npv += annualSavingsUSD / (1 + discountRate)
	}

	return &ROIAnalysis{
		TotalInvestmentUSD: investmentUSD,
		AnnualSavingsUSD:   annualSavingsUSD,
		ROI:                roi,
		PaybackMonths:     paybackMonths,
		TimelineYears:     years,
		NPVUSD:            npv,
	}
}

// CompareTiers 比较不同存储层级成本
func (m *Manager) CompareTiers(usedGB float64) map[StorageTier]float64 {
	costs := map[StorageTier]float64{
		TierHot:     usedGB * 0.10,  // $0.10/GB/月
		TierWarm:    usedGB * 0.05,  // $0.05/GB/月
		TierCold:    usedGB * 0.01,  // $0.01/GB/月
		TierArchive: usedGB * 0.002, // $0.002/GB/月
	}
	return costs
}

// GetTotalStorage 获取总存储量
func (m *Manager) GetTotalStorage() map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0.0
	used := 0.0
	for _, pool := range m.pools {
		total += pool.TotalGB
		used += pool.UsedGB
	}

	return map[string]float64{
		"total_gb": total,
		"used_gb":  used,
		"free_gb":  total - used,
	}
}

// GetCostTrend 获取成本趋势
func (m *Manager) GetCostTrend(months int) []map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	trend := make([]map[string]interface{}, 0)
	monthlyCosts := make(map[string]float64)

	for _, record := range m.records {
		monthlyCosts[record.Period] += record.CostUSD
	}

	for period, cost := range monthlyCosts {
		trend = append(trend, map[string]interface{}{
			"period":   period,
			"cost_usd": cost,
		})
	}

	return trend
}
