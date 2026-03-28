// Package cost 提供成本计算功能 (v2.90.0 户部)
package cost

import (
	"context"
	"math"
	"sync"
	"time"
)

// ========== 成本计算器 ==========

// CostCalculator 成本计算器.
type CostCalculator struct {
	pricingModels map[CostType]*PricingModel
	budgets       map[string]*Budget
	records       []CostRecord
	mu            sync.RWMutex
}

// NewCostCalculator 创建成本计算器.
func NewCostCalculator() *CostCalculator {
	calc := &CostCalculator{
		pricingModels: make(map[CostType]*PricingModel),
		budgets:       make(map[string]*Budget),
		records:       make([]CostRecord, 0),
	}

	// 初始化默认定价模型
	calc.initDefaultPricingModels()

	return calc
}

// initDefaultPricingModels 初始化默认定价模型.
func (c *CostCalculator) initDefaultPricingModels() {
	// CPU定价模型
	c.pricingModels[CostTypeCPU] = &PricingModel{
		ID:           "default-cpu",
		Name:         "CPU标准定价",
		CostType:     CostTypeCPU,
		BillingPeriod: "hourly",
		BasePrice:     0.05, // CNY/core/hour
		Tiers: []PricingTier{
			{Name: "small", StartValue: 0, EndValue: ptr(2.0), UnitPrice: 0.05},
			{Name: "medium", StartValue: 2, EndValue: ptr(8.0), UnitPrice: 0.04},
			{Name: "large", StartValue: 8, EndValue: nil, UnitPrice: 0.03},
		},
		Currency:       "CNY",
		Enabled:        true,
		EffectiveFrom:  time.Now(),
	}

	// 内存定价模型
	c.pricingModels[CostTypeMemory] = &PricingModel{
		ID:           "default-memory",
		Name:         "内存标准定价",
		CostType:     CostTypeMemory,
		BillingPeriod: "hourly",
		BasePrice:     0.02, // CNY/GB/hour
		Tiers: []PricingTier{
			{Name: "small", StartValue: 0, EndValue: ptr(4.0), UnitPrice: 0.02},
			{Name: "medium", StartValue: 4, EndValue: ptr(16.0), UnitPrice: 0.018},
			{Name: "large", StartValue: 16, EndValue: nil, UnitPrice: 0.015},
		},
		Currency:       "CNY",
		Enabled:        true,
		EffectiveFrom:  time.Now(),
	}

	// 存储定价模型
	c.pricingModels[CostTypeStorage] = &PricingModel{
		ID:           "default-storage",
		Name:         "存储标准定价",
		CostType:     CostTypeStorage,
		BillingPeriod: "monthly",
		BasePrice:     0.50, // CNY/GB/month
		Tiers: []PricingTier{
			{Name: "ssd", StartValue: 0, EndValue: ptr(1000.0), UnitPrice: 0.50},
			{Name: "hdd", StartValue: 0, EndValue: nil, UnitPrice: 0.15},
		},
		Currency:       "CNY",
		Enabled:        true,
		EffectiveFrom:  time.Now(),
	}

	// 网络定价模型
	c.pricingModels[CostTypeNetwork] = &PricingModel{
		ID:           "default-network",
		Name:         "网络流量定价",
		CostType:     CostTypeNetwork,
		BillingPeriod: "monthly",
		BasePrice:     0.80, // CNY/GB outbound
		Currency:      "CNY",
		Enabled:       true,
		EffectiveFrom: time.Now(),
	}
}

// ptr 辅助函数，创建float64指针.
func ptr(v float64) *float64 {
	return &v
}

// SetPricingModel 设置定价模型.
func (c *CostCalculator) SetPricingModel(model *PricingModel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pricingModels[model.CostType] = model
}

// GetPricingModel 获取定价模型.
func (c *CostCalculator) GetPricingModel(costType CostType) *PricingModel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.pricingModels[costType]
}

// SetBudget 设置预算.
func (c *CostCalculator) SetBudget(budget *Budget) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.budgets[budget.TargetID] = budget
}

// GetBudget 获取预算.
func (c *CostCalculator) GetBudget(targetID string) *Budget {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.budgets[targetID]
}

// CalculateCPUCost 计算CPU成本.
func (c *CostCalculator) CalculateCPUCost(cores float64, hours float64) float64 {
	model := c.GetPricingModel(CostTypeCPU)
	if model == nil || !model.Enabled {
		return 0
	}

	// 根据阶梯定价计算
	unitPrice := model.BasePrice
	for _, tier := range model.Tiers {
		if cores >= tier.StartValue {
			if tier.EndValue == nil || cores < *tier.EndValue {
				unitPrice = tier.UnitPrice
				break
			}
		}
	}

	return cores * hours * unitPrice
}

// CalculateMemoryCost 计算内存成本.
func (c *CostCalculator) CalculateMemoryCost(gb float64, hours float64) float64 {
	model := c.GetPricingModel(CostTypeMemory)
	if model == nil || !model.Enabled {
		return 0
	}

	// 根据阶梯定价计算
	unitPrice := model.BasePrice
	for _, tier := range model.Tiers {
		if gb >= tier.StartValue {
			if tier.EndValue == nil || gb < *tier.EndValue {
				unitPrice = tier.UnitPrice
				break
			}
		}
	}

	return gb * hours * unitPrice
}

// CalculateStorageCost 计算存储成本.
func (c *CostCalculator) CalculateStorageCost(gb float64, storageType string, months float64) float64 {
	model := c.GetPricingModel(CostTypeStorage)
	if model == nil || !model.Enabled {
		return 0
	}

	// 根据存储类型选择单价
	var unitPrice float64
	switch storageType {
	case "ssd":
		unitPrice = 0.50
	case "hdd":
		unitPrice = 0.15
	case "archive":
		unitPrice = 0.05
	default:
		unitPrice = model.BasePrice
	}

	return gb * months * unitPrice
}

// CalculateNetworkCost 计算网络成本.
func (c *CostCalculator) CalculateNetworkCost(txGB float64, rxGB float64) float64 {
	// 出流量成本
	txCost := txGB * 0.80

	// 入流量成本
	rxCost := rxGB * 0.20

	return txCost + rxCost
}

// CalculateTotalCost 计算总成本.
func (c *CostCalculator) CalculateTotalCost(ctx context.Context, targetID string, periodStart, periodEnd time.Time) (*CostSummary, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// 过滤目标对象的成本记录
	var targetRecords []CostRecord
	for _, r := range c.records {
		if r.TargetID == targetID {
			if !r.PeriodStart.Before(periodStart) && !r.PeriodEnd.After(periodEnd) {
				targetRecords = append(targetRecords, r)
			}
		}
	}

	// 汇总成本
	summary := &CostSummary{
		TargetID:     targetID,
		PeriodStart:  periodStart,
		PeriodEnd:    periodEnd,
		CostBreakdown: make(map[CostType]float64),
		Currency:     "CNY",
	}

	for _, r := range targetRecords {
		summary.CostBreakdown[r.CostType] += r.Amount
		summary.TotalCost += r.Amount
	}

	// 计算预算使用率
	budget := c.budgets[targetID]
	if budget != nil && budget.Limit > 0 {
		summary.BudgetLimit = budget.Limit
		summary.BudgetUsagePercent = (summary.TotalCost / budget.Limit) * 100
	}

	// 计算成本效率得分
	summary.EfficiencyScore = c.calculateEfficiencyScore(targetRecords)

	return summary, nil
}

// calculateEfficiencyScore 计算成本效率得分.
func (c *CostCalculator) calculateEfficiencyScore(records []CostRecord) float64 {
	if len(records) == 0 {
		return 0
	}

	// 简化计算：基于资源利用率
	// 实际实现应结合监控数据
	var cpuUtil, memUtil, storageUtil float64
	var cpuCount, memCount, storageCount int

	for _, r := range records {
		if r.CostType == CostTypeCPU {
			cpuUtil += r.Measurement
			cpuCount++
		}
		if r.CostType == CostTypeMemory {
			memUtil += r.Measurement
			memCount++
		}
		if r.CostType == CostTypeStorage {
			storageUtil += r.Measurement
			storageCount++
		}
	}

	var score float64
	if cpuCount > 0 {
		avgCPU := cpuUtil / float64(cpuCount)
		// CPU利用率60-80%为最佳
		score += c.utilizationScore(avgCPU, 60, 80)
	}
	if memCount > 0 {
		avgMem := memUtil / float64(memCount)
		// 内存利用率70-90%为最佳
		score += c.utilizationScore(avgMem, 70, 90)
	}
	if storageCount > 0 {
		avgStorage := storageUtil / float64(storageCount)
		// 存储利用率80-95%为最佳
		score += c.utilizationScore(avgStorage, 80, 95)
	}

	// 平均得分
	count := cpuCount + memCount + storageCount
	if count > 0 {
		score = score / float64(count/3+1)
	}

	return math.Max(0, math.Min(100, score))
}

// utilizationScore 计算利用率得分.
func (c *CostCalculator) utilizationScore(util, optimalMin, optimalMax float64) float64 {
	if util < optimalMin {
		// 过低利用率，扣分
		return util / optimalMin * 50
	}
	if util > optimalMax {
		// 过高利用率，扣分
		excess := util - optimalMax
		return 100 - excess
	}
	// 最佳区间
	return 100
}

// RecordCost 记录成本.
func (c *CostCalculator) RecordCost(record CostRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records = append(c.records, record)
}

// GetCostRecords 获取成本记录.
func (c *CostCalculator) GetCostRecords(targetID string, periodStart, periodEnd time.Time) []CostRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []CostRecord
	for _, r := range c.records {
		if targetID != "" && r.TargetID != targetID {
			continue
		}
		if !r.PeriodStart.Before(periodStart) && !r.PeriodEnd.After(periodEnd) {
			result = append(result, r)
		}
	}
	return result
}

// GenerateOptimizationSuggestions 生成优化建议.
func (c *CostCalculator) GenerateOptimizationSuggestions(ctx context.Context, targetID string, periodStart, periodEnd time.Time) []CostOptimizationSuggestion {
	summary, _ := c.CalculateTotalCost(ctx, targetID, periodStart, periodEnd)
	if summary == nil {
		return nil
	}

	var suggestions []CostOptimizationSuggestion

	// CPU优化建议
	if summary.CostBreakdown[CostTypeCPU] > 0 {
		cpuSuggestion := c.generateCPUSuggestion(targetID, summary)
		if cpuSuggestion != nil {
			suggestions = append(suggestions, *cpuSuggestion)
		}
	}

	// 内存优化建议
	if summary.CostBreakdown[CostTypeMemory] > 0 {
		memSuggestion := c.generateMemorySuggestion(targetID, summary)
		if memSuggestion != nil {
			suggestions = append(suggestions, *memSuggestion)
		}
	}

	// 存储优化建议
	if summary.CostBreakdown[CostTypeStorage] > 0 {
		storageSuggestion := c.generateStorageSuggestion(targetID, summary)
		if storageSuggestion != nil {
			suggestions = append(suggestions, *storageSuggestion)
		}
	}

	return suggestions
}

// generateCPUSuggestion 生成CPU优化建议.
func (c *CostCalculator) generateCPUSuggestion(targetID string, summary *CostSummary) *CostOptimizationSuggestion {
	// 如果效率得分低，建议优化
	if summary.EfficiencyScore < 50 {
		return &CostOptimizationSuggestion{
			ID:         "cpu-opt-" + targetID,
			TargetID:   targetID,
			Type:       "scale_down",
			Priority:   3,
			Title:      "降低CPU配置",
			Description: "应用CPU利用率较低，建议降低CPU配置以节省成本",
			EstimatedSavings: summary.CostBreakdown[CostTypeCPU] * 0.5,
			ImplementationComplexity: "medium",
			ImpactAssessment: "可能影响峰值性能",
			Confidence: 0.7,
			CreatedAt:  time.Now(),
			Status:     "pending",
		}
	}
	return nil
}

// generateMemorySuggestion 生成内存优化建议.
func (c *CostCalculator) generateMemorySuggestion(targetID string, summary *CostSummary) *CostOptimizationSuggestion {
	if summary.EfficiencyScore < 60 {
		return &CostOptimizationSuggestion{
			ID:         "mem-opt-" + targetID,
			TargetID:   targetID,
			Type:       "optimize",
			Priority:   2,
			Title:      "优化内存使用",
			Description: "内存使用效率较低，建议检查内存泄漏或优化内存分配",
			EstimatedSavings: summary.CostBreakdown[CostTypeMemory] * 0.3,
			ImplementationComplexity: "medium",
			ImpactAssessment: "需要应用调整",
			Confidence: 0.6,
			CreatedAt:  time.Now(),
			Status:     "pending",
		}
	}
	return nil
}

// generateStorageSuggestion 生成存储优化建议.
func (c *CostCalculator) generateStorageSuggestion(targetID string, summary *CostSummary) *CostOptimizationSuggestion {
	if summary.CostBreakdown[CostTypeStorage] > 100 {
		return &CostOptimizationSuggestion{
			ID:         "storage-opt-" + targetID,
			TargetID:   targetID,
			Type:       "optimize",
			Priority:   2,
			Title:      "存储分层优化",
			Description: "建议将冷数据迁移到低成本存储层",
			EstimatedSavings: summary.CostBreakdown[CostTypeStorage] * 0.4,
			ImplementationComplexity: "easy",
			ImpactAssessment: "对访问性能有影响",
			Confidence: 0.8,
			CreatedAt:  time.Now(),
			Status:     "pending",
		}
	}
	return nil
}

// CheckBudgetAlerts 检查预算告警.
func (c *CostCalculator) CheckBudgetAlerts(ctx context.Context) []BudgetAlert {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var alerts []BudgetAlert
	now := time.Now()

	for targetID, budget := range c.budgets {
		if !budget.Enabled {
			continue
		}

		// 计算当前周期成本
		periodStart, periodEnd := c.getBudgetPeriod(budget, now)
		summary, _ := c.CalculateTotalCost(ctx, targetID, periodStart, periodEnd)
		if summary == nil {
			continue
		}

		usagePercent := summary.BudgetUsagePercent

		// 检查告警阈值
		if usagePercent >= budget.AlertThreshold3 {
			alerts = append(alerts, BudgetAlert{
				BudgetID:      budget.ID,
				Level:         "critical",
				UsagePercent:  usagePercent,
				BudgetLimit:   budget.Limit,
				CurrentCost:   summary.TotalCost,
				TriggeredAt:   now,
				Message:       "预算已超限",
			})
		} else if usagePercent >= budget.AlertThreshold2 {
			alerts = append(alerts, BudgetAlert{
				BudgetID:      budget.ID,
				Level:         "warning",
				UsagePercent:  usagePercent,
				BudgetLimit:   budget.Limit,
				CurrentCost:   summary.TotalCost,
				TriggeredAt:   now,
				Message:       "预算即将用尽",
			})
		} else if usagePercent >= budget.AlertThreshold1 {
			alerts = append(alerts, BudgetAlert{
				BudgetID:      budget.ID,
				Level:         "info",
				UsagePercent:  usagePercent,
				BudgetLimit:   budget.Limit,
				CurrentCost:   summary.TotalCost,
				TriggeredAt:   now,
				Message:       "预算使用超过阈值",
			})
		}
	}

	return alerts
}

// getBudgetPeriod 获取预算周期.
func (c *CostCalculator) getBudgetPeriod(budget *Budget, now time.Time) (time.Time, time.Time) {
	switch budget.Period {
	case "daily":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end := start.Add(24 * time.Hour)
		return start, end
	case "weekly":
		weekday := int(now.Weekday())
		start := now.AddDate(0, 0, -weekday)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 0, 7)
		return start, end
	case "monthly":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 1, 0)
		return start, end
	default:
		// 默认月度
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		end := start.AddDate(0, 1, 0)
		return start, end
	}
}

// Cleanup 清理过期记录.
func (c *CostCalculator) Cleanup(retentionDays int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	var validRecords []CostRecord
	for _, r := range c.records {
		if r.PeriodEnd.After(cutoff) {
			validRecords = append(validRecords, r)
		}
	}
	c.records = validRecords
}