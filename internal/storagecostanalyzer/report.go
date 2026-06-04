// Package storagecostanalyzer 存储成本分析器 - 报表生成
package storagecostanalyzer

import (
	"fmt"
	"time"
)

// ReportGenerator 报表生成器.
type ReportGenerator struct {
	manager *Manager
}

// NewReportGenerator 创建报表生成器.
func NewReportGenerator(manager *Manager) *ReportGenerator {
	return &ReportGenerator{manager: manager}
}

// GenerateMultiDimensionReport 生成多维度成本报表.
func (r *ReportGenerator) GenerateMultiDimensionReport(period string) (*MultiDimensionReport, error) {
	r.manager.mu.RLock()
	defer r.manager.mu.RUnlock()

	now := r.manager.nowFunc()
	var periodStart, periodEnd time.Time

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

	id := fmt.Sprintf("report-multi-%s-%d", period, now.UnixNano())

	// 收集数据
	totalCost := 0.0
	totalCapacity := 0.0
	totalUsed := 0.0
	tierCosts := make(map[StorageTier]float64)
	categoryCosts := make(map[CostCategory]float64)
	providerCosts := make(map[string]map[StorageTier]float64)

	for tier, ts := range r.manager.tiers {
		tierCost := 0.0
		for _, rec := range ts.records {
			if !rec.Timestamp.Before(periodStart) && !rec.Timestamp.After(periodEnd) {
				tierCost += rec.Amount
				categoryCosts[rec.Category] += rec.Amount

				if providerCosts[rec.Provider] == nil {
					providerCosts[rec.Provider] = make(map[StorageTier]float64)
				}
				providerCosts[rec.Provider][tier] += rec.Amount
			}
		}
		tierCosts[tier] = tierCost
		totalCost += tierCost
		totalCapacity += ts.config.CapacityTB
		totalUsed += ts.config.UsedTB
	}

	// 如果没有记录，使用配置估算
	if totalCost == 0 {
		for tier, ts := range r.manager.tiers {
			estimatedCost := ts.config.UsedTB * ts.config.CostPerTBMonth
			tierCosts[tier] = estimatedCost
			totalCost += estimatedCost
			categoryCosts[CategorySubscription] += estimatedCost
		}
	}

	// 计算利用率
	overallUtilization := 0.0
	if totalCapacity > 0 {
		overallUtilization = (totalUsed / totalCapacity) * 100
	}

	avgCostPerTB := 0.0
	if totalUsed > 0 {
		avgCostPerTB = totalCost / totalUsed
	}

	// 按层级分类
	var tierBreakdown []TierCostBreakdown
	for tier, ts := range r.manager.tiers {
		capacityTB := ts.config.CapacityTB
		usedTB := ts.config.UsedTB
		utilization := 0.0
		if capacityTB > 0 {
			utilization = (usedTB / capacityTB) * 100
		}
		cost := tierCosts[tier]
		costPerTB := ts.config.CostPerTBMonth
		costShare := 0.0
		if totalCost > 0 {
			costShare = (cost / totalCost) * 100
		}

		tierBreakdown = append(tierBreakdown, TierCostBreakdown{
			Tier:        tier,
			TierName:    ts.config.Name,
			CapacityTB:  capacityTB,
			UsedTB:      usedTB,
			Utilization: utilization,
			CostPerTB:   costPerTB,
			MonthlyCost: cost,
			CostShare:   costShare,
		})
	}

	// 按成本类别分类
	var categoryBreakdown []CategoryCostBreakdown
	categoryDescriptions := map[CostCategory]string{
		CategoryHardware:     "硬件采购与维护",
		CategoryPower:        "电力消耗",
		CategoryCooling:      "散热制冷",
		CategoryMaintenance:  "系统维护",
		CategorySubscription: "订阅服务",
		CategoryBandwidth:    "网络带宽",
		CategoryLabor:        "人力成本",
		CategoryDepreciation: "资产折旧",
	}

	for category, amount := range categoryCosts {
		percentage := 0.0
		if totalCost > 0 {
			percentage = (amount / totalCost) * 100
		}
		desc := categoryDescriptions[category]
		if desc == "" {
			desc = string(category)
		}
		categoryBreakdown = append(categoryBreakdown, CategoryCostBreakdown{
			Category:    category,
			Amount:      amount,
			Percentage:  percentage,
			Description: desc,
		})
	}

	// 按供应商分类
	var providerBreakdown []ProviderCostBreakdown
	for provider, tiers := range providerCosts {
		for tier, amount := range tiers {
			percentage := 0.0
			if totalCost > 0 {
				percentage = (amount / totalCost) * 100
			}
			providerBreakdown = append(providerBreakdown, ProviderCostBreakdown{
				Provider:   provider,
				Amount:     amount,
				Percentage: percentage,
				Tier:       tier,
			})
		}
	}

	// 时间序列数据（简化：单点）
	var timeSeries []TimeSeriesPoint
	timeSeries = append(timeSeries, TimeSeriesPoint{
		Date:        now,
		Cost:        totalCost,
		CapacityTB:  totalCapacity,
		UsedTB:      totalUsed,
		CostPerTB:   avgCostPerTB,
	})

	// 优化影响分析
	potentialSavings := 0.0
	for _, ts := range r.manager.tiers {
		utilization := 0.0
		if ts.config.CapacityTB > 0 {
			utilization = (ts.config.UsedTB / ts.config.CapacityTB) * 100
		}
		if utilization < 50 {
			unusedTB := ts.config.CapacityTB - ts.config.UsedTB
			potentialSavings += unusedTB * ts.config.CostPerTBMonth * 12
		}
	}

	summary := ReportSummary{
		TotalCost:          totalCost,
		AvgMonthlyCost:     totalCost,
		TotalCapacityTB:    totalCapacity,
		TotalUsedTB:        totalUsed,
		OverallUtilization: overallUtilization,
		AvgCostPerTB:       avgCostPerTB,
		CostChangePercent:  0,
	}

	return &MultiDimensionReport{
		ID:                 id,
		Title:              fmt.Sprintf("%s 多维度成本报表", period),
		GeneratedAt:        now,
		PeriodStart:        periodStart,
		PeriodEnd:          periodEnd,
		Summary:            summary,
		TierBreakdown:      tierBreakdown,
		CategoryBreakdown:  categoryBreakdown,
		ProviderBreakdown:  providerBreakdown,
		TimeSeries:         timeSeries,
		OptimizationImpact: OptimizationImpact{
			PotentialSavings: potentialSavings,
		},
	}, nil
}

// GenerateExecutiveSummary 生成高管摘要.
func (r *ReportGenerator) GenerateExecutiveSummary(period string) (*ExecutiveSummary, error) {
	report, err := r.GenerateMultiDimensionReport(period)
	if err != nil {
		return nil, err
	}

	r.manager.mu.RLock()
	defer r.manager.mu.RUnlock()

	// 识别关键洞察
	var keyInsights []string
	var recommendations []string

	// 检查利用率
	for _, tier := range report.TierBreakdown {
		if tier.Utilization > 85 {
			keyInsights = append(keyInsights,
				fmt.Sprintf("%s 层级利用率 %.1f%% 偏高，需要关注", tier.TierName, tier.Utilization))
			recommendations = append(recommendations,
				fmt.Sprintf("建议对 %s 层级进行扩容或数据迁移", tier.TierName))
		} else if tier.Utilization < 30 {
			keyInsights = append(keyInsights,
				fmt.Sprintf("%s 层级利用率仅 %.1f%%，存在资源浪费", tier.TierName, tier.Utilization))
			recommendations = append(recommendations,
				fmt.Sprintf("建议优化 %s 层级，考虑缩减容量或迁移数据", tier.TierName))
		}
	}

	// 检查成本占比
	for _, category := range report.CategoryBreakdown {
		if category.Percentage > 40 {
			keyInsights = append(keyInsights,
				fmt.Sprintf("%s 占总成本 %.1f%%，是主要成本驱动因素", category.Description, category.Percentage))
		}
	}

	// 添加通用建议
	if report.Summary.OverallUtilization < 50 {
		recommendations = append(recommendations, "整体利用率较低，建议评估存储整合机会")
	}
	if report.OptimizationImpact.PotentialSavings > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("通过优化可节省约 ¥%.0f/年", report.OptimizationImpact.PotentialSavings))
	}

	// 风险评估
	var risks []RiskItem
	for _, tier := range report.TierBreakdown {
		if tier.Utilization > 90 {
			risks = append(risks, RiskItem{
				Level:       "high",
				Description: fmt.Sprintf("%s 存储即将满容量", tier.TierName),
				Impact:      "可能导致服务中断",
				Mitigation:  "立即扩容或迁移数据",
			})
		}
	}

	return &ExecutiveSummary{
		GeneratedAt:        r.manager.nowFunc(),
		Period:             period,
		TotalCost:          report.Summary.TotalCost,
		TotalCapacityTB:    report.Summary.TotalCapacityTB,
		TotalUsedTB:        report.Summary.TotalUsedTB,
		OverallUtilization: report.Summary.OverallUtilization,
		KeyInsights:        keyInsights,
		Recommendations:    recommendations,
		Risks:              risks,
	}, nil
}

// ExecutiveSummary 高管摘要.
type ExecutiveSummary struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// Period 周期.
	Period string `json:"period"`
	// TotalCost 总成本.
	TotalCost float64 `json:"totalCost"`
	// TotalCapacityTB 总容量（TB）.
	TotalCapacityTB float64 `json:"totalCapacityTB"`
	// TotalUsedTB 总已用（TB）.
	TotalUsedTB float64 `json:"totalUsedTB"`
	// OverallUtilization 总体利用率（%）.
	OverallUtilization float64 `json:"overallUtilization"`
	// KeyInsights 关键洞察.
	KeyInsights []string `json:"keyInsights"`
	// Recommendations 建议.
	Recommendations []string `json:"recommendations"`
	// Risks 风险.
	Risks []RiskItem `json:"risks"`
}

// RiskItem 风险项.
type RiskItem struct {
	// Level 级别（low/medium/high/critical）.
	Level string `json:"level"`
	// Description 描述.
	Description string `json:"description"`
	// Impact 影响.
	Impact string `json:"impact"`
	// Mitigation 缓解措施.
	Mitigation string `json:"mitigation"`
}

// GenerateTrendReport 生成趋势报表.
func (r *ReportGenerator) GenerateTrendReport(months int) (*TrendReport, error) {
	r.manager.mu.RLock()
	defer r.manager.mu.RUnlock()

	if months <= 0 {
		months = 12
	}

	now := r.manager.nowFunc()

	// 生成月度趋势数据
	var monthlyTrends []MonthlyTrend
	for i := months; i >= 0; i-- {
		date := now.AddDate(0, -i, 0)
		monthStart := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
		monthEnd := monthStart.AddDate(0, 1, 0).Add(-time.Nanosecond)

		totalCost := 0.0
		totalCapacity := 0.0
		totalUsed := 0.0

		for _, ts := range r.manager.tiers {
			monthCost := 0.0
			for _, rec := range ts.records {
				if !rec.Timestamp.Before(monthStart) && !rec.Timestamp.After(monthEnd) {
					monthCost += rec.Amount
				}
			}
			if monthCost == 0 {
				monthCost = ts.config.UsedTB * ts.config.CostPerTBMonth
			}
			totalCost += monthCost
			totalCapacity += ts.config.CapacityTB
			totalUsed += ts.config.UsedTB
		}

		utilization := 0.0
		if totalCapacity > 0 {
			utilization = (totalUsed / totalCapacity) * 100
		}

		costPerTB := 0.0
		if totalUsed > 0 {
			costPerTB = totalCost / totalUsed
		}

		monthlyTrends = append(monthlyTrends, MonthlyTrend{
			Month:       monthStart,
			TotalCost:   totalCost,
			CapacityTB:  totalCapacity,
			UsedTB:      totalUsed,
			Utilization: utilization,
			CostPerTB:   costPerTB,
		})
	}

	// 计算趋势指标
	costTrend := "stable"
	if len(monthlyTrends) >= 2 {
		first := monthlyTrends[0].TotalCost
		last := monthlyTrends[len(monthlyTrends)-1].TotalCost
		if last > first*1.1 {
			costTrend = "increasing"
		} else if last < first*0.9 {
			costTrend = "decreasing"
		}
	}

	utilizationTrend := "stable"
	if len(monthlyTrends) >= 2 {
		first := monthlyTrends[0].Utilization
		last := monthlyTrends[len(monthlyTrends)-1].Utilization
		if last > first+5 {
			utilizationTrend = "increasing"
		} else if last < first-5 {
			utilizationTrend = "decreasing"
		}
	}

	// 计算月均增长率
	avgCostGrowthRate := 0.0
	avgUtilizationGrowthRate := 0.0
	if len(monthlyTrends) >= 2 {
		first := monthlyTrends[0]
		last := monthlyTrends[len(monthlyTrends)-1]
		monthDiff := float64(len(monthlyTrends) - 1)
		if first.TotalCost > 0 && monthDiff > 0 {
			avgCostGrowthRate = ((last.TotalCost/first.TotalCost - 1) / monthDiff) * 100
		}
		if monthDiff > 0 {
			avgUtilizationGrowthRate = (last.Utilization - first.Utilization) / monthDiff
		}
	}

	return &TrendReport{
		GeneratedAt:             now,
		AnalysisMonths:          months,
		MonthlyTrends:           monthlyTrends,
		CostTrend:               costTrend,
		UtilizationTrend:        utilizationTrend,
		AvgCostGrowthRate:       avgCostGrowthRate,
		AvgUtilizationGrowthRate: avgUtilizationGrowthRate,
	}, nil
}

// TrendReport 趋势报表.
type TrendReport struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// AnalysisMonths 分析月数.
	AnalysisMonths int `json:"analysisMonths"`
	// MonthlyTrends 月度趋势.
	MonthlyTrends []MonthlyTrend `json:"monthlyTrends"`
	// CostTrend 成本趋势（increasing/stable/decreasing）.
	CostTrend string `json:"costTrend"`
	// UtilizationTrend 利用率趋势.
	UtilizationTrend string `json:"utilizationTrend"`
	// AvgCostGrowthRate 月均成本增长率（%）.
	AvgCostGrowthRate float64 `json:"avgCostGrowthRate"`
	// AvgUtilizationGrowthRate 月均利用率增长率.
	AvgUtilizationGrowthRate float64 `json:"avgUtilizationGrowthRate"`
}

// MonthlyTrend 月度趋势.
type MonthlyTrend struct {
	// Month 月份.
	Month time.Time `json:"month"`
	// TotalCost 总成本.
	TotalCost float64 `json:"totalCost"`
	// CapacityTB 容量（TB）.
	CapacityTB float64 `json:"capacityTB"`
	// UsedTB 已用（TB）.
	UsedTB float64 `json:"usedTB"`
	// Utilization 利用率（%）.
	Utilization float64 `json:"utilization"`
	// CostPerTB 每TB成本.
	CostPerTB float64 `json:"costPerTB"`
}

// GenerateCostAllocationReport 生成成本分配报表.
func (r *ReportGenerator) GenerateCostAllocationReport() (*CostAllocationReport, error) {
	r.manager.mu.RLock()
	defer r.manager.mu.RUnlock()

	now := r.manager.nowFunc()
	totalCost := 0.0

	// 按层级分配
	var tierAllocations []TierAllocation
	for tier, ts := range r.manager.tiers {
		cost := ts.config.UsedTB * ts.config.CostPerTBMonth
		totalCost += cost

		tierAllocations = append(tierAllocations, TierAllocation{
			Tier:        tier,
			TierName:    ts.config.Name,
			CapacityTB:  ts.config.CapacityTB,
			UsedTB:      ts.config.UsedTB,
			MonthlyCost: cost,
			CostPerTB:   ts.config.CostPerTBMonth,
		})
	}

	// 计算占比
	for i := range tierAllocations {
		if totalCost > 0 {
			tierAllocations[i].CostShare = (tierAllocations[i].MonthlyCost / totalCost) * 100
		}
	}

	// 计算效率指标
	var efficiencyMetrics []EfficiencyMetric
	for _, alloc := range tierAllocations {
		utilization := 0.0
		if alloc.CapacityTB > 0 {
			utilization = (alloc.UsedTB / alloc.CapacityTB) * 100
		}
		costEfficiency := 0.0
		if alloc.UsedTB > 0 {
			costEfficiency = alloc.MonthlyCost / alloc.UsedTB
		}
		wasteCost := (alloc.CapacityTB - alloc.UsedTB) * alloc.CostPerTB

		efficiencyMetrics = append(efficiencyMetrics, EfficiencyMetric{
			Tier:           alloc.Tier,
			TierName:       alloc.TierName,
			Utilization:    utilization,
			CostEfficiency: costEfficiency,
			WasteCost:      wasteCost,
		})
	}

	return &CostAllocationReport{
		GeneratedAt:       now,
		TotalMonthlyCost:  totalCost,
		TierAllocations:   tierAllocations,
		EfficiencyMetrics: efficiencyMetrics,
	}, nil
}

// CostAllocationReport 成本分配报表.
type CostAllocationReport struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// TotalMonthlyCost 总月成本.
	TotalMonthlyCost float64 `json:"totalMonthlyCost"`
	// TierAllocations 按层级分配.
	TierAllocations []TierAllocation `json:"tierAllocations"`
	// EfficiencyMetrics 效率指标.
	EfficiencyMetrics []EfficiencyMetric `json:"efficiencyMetrics"`
}

// TierAllocation 层级分配.
type TierAllocation struct {
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// TierName 层级名称.
	TierName string `json:"tierName"`
	// CapacityTB 容量（TB）.
	CapacityTB float64 `json:"capacityTB"`
	// UsedTB 已用（TB）.
	UsedTB float64 `json:"usedTB"`
	// MonthlyCost 月成本.
	MonthlyCost float64 `json:"monthlyCost"`
	// CostPerTB 每TB成本.
	CostPerTB float64 `json:"costPerTB"`
	// CostShare 成本占比（%）.
	CostShare float64 `json:"costShare"`
}

// EfficiencyMetric 效率指标.
type EfficiencyMetric struct {
	// Tier 存储层级.
	Tier StorageTier `json:"tier"`
	// TierName 层级名称.
	TierName string `json:"tierName"`
	// Utilization 利用率（%）.
	Utilization float64 `json:"utilization"`
	// CostEfficiency 成本效率（元/TB）.
	CostEfficiency float64 `json:"costEfficiency"`
	// WasteCost 浪费成本.
	WasteCost float64 `json:"wasteCost"`
}
