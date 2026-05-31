// Package costanalyzer 存储成本分析器
package costanalyzer

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// CostAnalyzer 成本分析器
type CostAnalyzer struct {
	mgr *Manager
}

// NewCostAnalyzer 创建成本分析器
func NewCostAnalyzer(mgr *Manager) *CostAnalyzer {
	return &CostAnalyzer{mgr: mgr}
}

// ForecastCost 成本预测
// 根据历史数据预测未来N个月的成本
func (a *CostAnalyzer) ForecastCost(months int) []*CostRecord {
	a.mgr.mu.RLock()
	defer a.mgr.mu.RUnlock()

	if len(a.mgr.records) < 2 {
		return nil
	}

	// 计算增长趋势
	records := a.mgr.records
	growthRate := a.calculateGrowthRate(records)

	lastRecord := records[len(records)-1]
	var forecasts []*CostRecord

	for i := 1; i <= months; i++ {
		futureDate := lastRecord.Timestamp.AddDate(0, i, 0)
		forecast := &CostRecord{
			Timestamp:   futureDate,
			Period:      futureDate.Format("2006-01"),
			TotalCost:   lastRecord.TotalCost * math.Pow(1+growthRate, float64(i)),
			StorageCost: lastRecord.StorageCost * math.Pow(1+growthRate, float64(i)),
			PowerCost:   lastRecord.PowerCost * math.Pow(1+growthRate*0.5, float64(i)),
			MaintCost:   lastRecord.MaintCost * math.Pow(1+growthRate*0.3, float64(i)),
			UsedTB:      lastRecord.UsedTB * math.Pow(1+growthRate, float64(i)),
		}
		forecast.UnitCostTB = safeDiv(forecast.TotalCost, forecast.UsedTB)
		forecasts = append(forecasts, forecast)
	}

	return forecasts
}

// AnalyzePoolEfficiency 分析存储池效率
func (a *CostAnalyzer) AnalyzePoolEfficiency() []*PoolEfficiency {
	a.mgr.mu.RLock()
	defer a.mgr.mu.RUnlock()

	var results []*PoolEfficiency
	for _, p := range a.mgr.pools {
		eff := &PoolEfficiency{
			PoolID:      p.ID,
			PoolName:    p.Name,
			Type:        p.Type,
			Utilization: safeDiv(p.UsedTB, p.TotalTB) * 100,
			CostPerTB:   p.UnitCost,
			TotalCost:   p.UsedTB * p.UnitCost,
			Efficiency:  a.scoreEfficiency(p),
		}
		results = append(results, eff)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Efficiency < results[j].Efficiency
	})

	return results
}

// GetCostBreakdown 获取成本明细
func (a *CostAnalyzer) GetCostBreakdown() *CostBreakdown {
	a.mgr.mu.RLock()
	defer a.mgr.mu.RUnlock()

	breakdown := &CostBreakdown{
		ByType: make(map[StorageType]float64),
		ByPool: make(map[string]float64),
	}

	for _, p := range a.mgr.pools {
		cost := p.UsedTB * p.UnitCost
		breakdown.ByType[p.Type] += cost
		breakdown.ByPool[p.Name] = cost
		breakdown.Total += cost
	}

	// 计算百分比
	breakdown.TypePercent = make(map[StorageType]float64)
	for t, cost := range breakdown.ByType {
		breakdown.TypePercent[t] = safeDiv(cost, breakdown.Total) * 100
	}

	return breakdown
}

// ComparePeriods 对比两个时期的成本
func (a *CostAnalyzer) ComparePeriods(period1, period2 string) (*PeriodComparison, error) {
	a.mgr.mu.RLock()
	defer a.mgr.mu.RUnlock()

	r1 := a.findRecord(period1)
	r2 := a.findRecord(period2)

	if r1 == nil || r2 == nil {
		return nil, fmt.Errorf("period not found")
	}

	change := safeDiv(r2.TotalCost-r1.TotalCost, r1.TotalCost) * 100
	return &PeriodComparison{
		Period1:      period1,
		Period2:      period2,
		Cost1:        r1.TotalCost,
		Cost2:        r2.TotalCost,
		Change:       change,
		ChangeAmount: r2.TotalCost - r1.TotalCost,
		Trend:        a.trendLabel(change),
	}, nil
}

// GetRecommendations 获取优化建议（按优先级排序）
func (a *CostAnalyzer) GetRecommendations() []*OptimizationSuggestion {
	suggestions := a.mgr.generateSuggestions()
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority > suggestions[j].Priority
	})
	return suggestions
}

// EstimateSavings 估算优化后的节省
func (a *CostAnalyzer) EstimateSavings(applyDedup, applyCompress, applyTiering bool) *SavingsEstimate {
	a.mgr.mu.RLock()
	defer a.mgr.mu.RUnlock()

	estimate := &SavingsEstimate{}

	for _, p := range a.mgr.pools {
		baseCost := p.UsedTB * p.UnitCost

		if applyDedup {
			dedupSaving := baseCost * 0.15
			estimate.DedupSavings += dedupSaving
		}

		if applyCompress {
			compressSaving := baseCost * 0.20
			estimate.CompressSavings += compressSaving
		}

		if applyTiering && p.IsHot && p.Type != StorageHDD {
			// 将部分热数据迁移到冷层
			tierSaving := p.UsedTB * 0.3 * (p.UnitCost - 40) // 假设冷层40元/TB
			if tierSaving > 0 {
				estimate.TieringSavings += tierSaving
			}
		}
	}

	estimate.TotalSavings = estimate.DedupSavings + estimate.CompressSavings + estimate.TieringSavings
	estimate.PercentSaved = safeDiv(estimate.TotalSavings, a.totalCost()) * 100
	return estimate
}

// PoolEfficiency 存储池效率
type PoolEfficiency struct {
	PoolID      string      `json:"pool_id"`
	PoolName    string      `json:"pool_name"`
	Type        StorageType `json:"type"`
	Utilization float64     `json:"utilization"`
	CostPerTB   float64     `json:"cost_per_tb"`
	TotalCost   float64     `json:"total_cost"`
	Efficiency  float64     `json:"efficiency_score"` // 0-100
}

// CostBreakdown 成本明细
type CostBreakdown struct {
	Total        float64                `json:"total"`
	ByType       map[StorageType]float64 `json:"by_type"`
	ByPool       map[string]float64      `json:"by_pool"`
	TypePercent  map[StorageType]float64 `json:"type_percent"`
}

// PeriodComparison 时期对比
type PeriodComparison struct {
	Period1      string  `json:"period1"`
	Period2      string  `json:"period2"`
	Cost1        float64 `json:"cost1"`
	Cost2        float64 `json:"cost2"`
	Change       float64 `json:"change_percent"`
	ChangeAmount float64 `json:"change_amount"`
	Trend        string  `json:"trend"`
}

// SavingsEstimate 节省估算
type SavingsEstimate struct {
	DedupSavings     float64 `json:"dedup_savings"`
	CompressSavings  float64 `json:"compress_savings"`
	TieringSavings   float64 `json:"tiering_savings"`
	TotalSavings     float64 `json:"total_savings"`
	PercentSaved     float64 `json:"percent_saved"`
}

// 内部方法

func (a *CostAnalyzer) calculateGrowthRate(records []*CostRecord) float64 {
	if len(records) < 2 {
		return 0
	}
	first := records[0]
	last := records[len(records)-1]
	months := last.Timestamp.Sub(first.Timestamp).Hours() / 720 // 约30天
	if months <= 0 {
		return 0
	}
	return (last.TotalCost/first.TotalCost - 1) / months
}

func (a *CostAnalyzer) scoreEfficiency(p *StoragePool) float64 {
	utilization := safeDiv(p.UsedTB, p.TotalTB) * 100
	// 效率评分：利用率70-85%最佳，成本越低越好
	utilScore := 0.0
	switch {
	case utilization >= 70 && utilization <= 85:
		utilScore = 100
	case utilization >= 60 && utilization <= 90:
		utilScore = 80
	case utilization >= 50:
		utilScore = 60
	default:
		utilScore = 40
	}

	// 成本评分（越低越好，基准100元/TB）
	costScore := math.Max(0, 100-p.UnitCost)

	return (utilScore + costScore) / 2
}

func (a *CostAnalyzer) findRecord(period string) *CostRecord {
	for _, r := range a.mgr.records {
		if r.Period == period {
			return r
		}
	}
	return nil
}

func (a *CostAnalyzer) trendLabel(change float64) string {
	switch {
	case change > 10:
		return "大幅上涨"
	case change > 0:
		return "上涨"
	case change == 0:
		return "持平"
	case change > -10:
		return "下降"
	default:
		return "大幅下降"
	}
}

func (a *CostAnalyzer) totalCost() float64 {
	total := 0.0
	for _, p := range a.mgr.pools {
		total += p.UsedTB * p.UnitCost
	}
	return total
}

// GrowthForecast 增长预测结果
type GrowthForecast struct {
	Months         int       `json:"months"`
	CurrentCost    float64   `json:"current_cost"`
	ForecastCost   float64   `json:"forecast_cost"`
	GrowthRate     float64   `json:"monthly_growth_rate"`
	CurrentTB      float64   `json:"current_tb"`
	ForecastTB     float64   `json:"forecast_tb"`
	Recommendation string    `json:"recommendation"`
	ForecastDate   time.Time `json:"forecast_date"`
}

// ForecastGrowth 预测存储增长
func (a *CostAnalyzer) ForecastGrowth(months int) *GrowthForecast {
	a.mgr.mu.RLock()
	defer a.mgr.mu.RUnlock()

	if len(a.mgr.records) < 2 {
		return nil
	}

	growthRate := a.calculateGrowthRate(a.mgr.records)
	last := a.mgr.records[len(a.mgr.records)-1]

	forecastCost := last.TotalCost * math.Pow(1+growthRate, float64(months))
	forecastTB := last.UsedTB * math.Pow(1+growthRate, float64(months))

	totalCap := a.mgr.totalCapacity()
	rec := "存储容量充足"
	if forecastTB > totalCap*0.9 {
		rec = "建议扩容，预计将在 " + fmt.Sprintf("%d", months) + " 个月内达到容量上限"
	}

	return &GrowthForecast{
		Months:         months,
		CurrentCost:    last.TotalCost,
		ForecastCost:   math.Round(forecastCost*100) / 100,
		GrowthRate:     math.Round(growthRate*10000) / 100,
		CurrentTB:      last.UsedTB,
		ForecastTB:     math.Round(forecastTB*100) / 100,
		Recommendation: rec,
		ForecastDate:   time.Now().AddDate(0, months, 0),
	}
}
