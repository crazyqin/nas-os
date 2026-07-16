// Package smartpricing 提供智能存储定价分析功能
package smartpricing

import (
	"fmt"
	"time"
)

// GenerateMonthlyReport 生成月度成本报告.
func GenerateMonthlyReport(analyzer *Analyzer, capacityGB, usedGB int64, tier StorageTier, replica ReplicaPolicy) (*CostReport, error) {
	if capacityGB <= 0 {
		return nil, fmt.Errorf("capacity must be positive")
	}

	// 计算成本分析
	analysis, err := analyzer.Analyze(capacityGB, tier, replica, WorkloadMixed)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	now := time.Now()
	periodStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	periodEnd := periodStart.AddDate(0, 1, -1)

	// 使用率
	usageRatio := 0.0
	if capacityGB > 0 {
		usageRatio = float64(usedGB) / float64(capacityGB)
	}

	// 生成优化建议
	suggestions := generateSuggestions(analysis, usageRatio)

	report := &CostReport{
		ReportID:        fmt.Sprintf("monthly-%d", now.UnixNano()),
		ReportType:      ReportMonthly,
		Title:           fmt.Sprintf("月度存储成本报告 (%s)", now.Format("2006年01月")),
		GeneratedAt:     now,
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		TotalCapacityGB: capacityGB,
		UsedCapacityGB:  usedGB,
		UsageRatio:      usageRatio,
		TotalCost:       analysis.MonthlyCost.TotalCost,
		StorageCost:     analysis.MonthlyCost.StorageCost,
		ReplicaCost:     analysis.MonthlyCost.ReplicaCost,
		TransferCost:    analysis.MonthlyCost.TransferCost,
		TierBreakdown: []TierCostSummary{
			{
				Tier:       tier,
				CapacityGB: capacityGB,
				Cost:       analysis.MonthlyCost.TotalCost,
				CostPerGB:  analysis.MonthlyCost.EffectivePerGB,
				UsageRatio: usageRatio,
			},
		},
		Suggestions: suggestions,
	}

	return report, nil
}

// GenerateAnnualReport 生成年度成本报告.
func GenerateAnnualReport(analyzer *Analyzer, capacityGB, usedGB int64, tier StorageTier, replica ReplicaPolicy) (*CostReport, error) {
	if capacityGB <= 0 {
		return nil, fmt.Errorf("capacity must be positive")
	}

	// 计算成本分析
	analysis, err := analyzer.Analyze(capacityGB, tier, replica, WorkloadMixed)
	if err != nil {
		return nil, fmt.Errorf("analysis failed: %w", err)
	}

	now := time.Now()
	periodStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
	periodEnd := time.Date(now.Year(), 12, 31, 23, 59, 59, 0, now.Location())

	// 使用率
	usageRatio := 0.0
	if capacityGB > 0 {
		usageRatio = float64(usedGB) / float64(capacityGB)
	}

	// 生成优化建议
	suggestions := generateSuggestions(analysis, usageRatio)

	report := &CostReport{
		ReportID:        fmt.Sprintf("annual-%d", now.UnixNano()),
		ReportType:      ReportAnnual,
		Title:           fmt.Sprintf("年度存储成本报告 (%d年)", now.Year()),
		GeneratedAt:     now,
		PeriodStart:     periodStart,
		PeriodEnd:       periodEnd,
		TotalCapacityGB: capacityGB,
		UsedCapacityGB:  usedGB,
		UsageRatio:      usageRatio,
		TotalCost:       analysis.AnnualCost.TotalCost,
		StorageCost:     analysis.AnnualCost.StorageCost,
		ReplicaCost:     analysis.AnnualCost.ReplicaCost,
		TransferCost:    analysis.AnnualCost.TransferCost,
		TierBreakdown: []TierCostSummary{
			{
				Tier:       tier,
				CapacityGB: capacityGB,
				Cost:       analysis.AnnualCost.TotalCost,
				CostPerGB:  analysis.AnnualCost.EffectivePerGB,
				UsageRatio: usageRatio,
			},
		},
		Suggestions: suggestions,
	}

	return report, nil
}

// generateSuggestions 生成优化建议.
func generateSuggestions(analysis *CostAnalysis, usageRatio float64) []string {
	var suggestions []string

	// 使用率相关建议
	if usageRatio < 0.3 {
		suggestions = append(suggestions, "存储使用率低于 30%，建议缩减容量以降低成本")
	} else if usageRatio > 0.85 {
		suggestions = append(suggestions, "存储使用率超过 85%，建议扩容以避免性能下降")
	}

	// 副本策略建议
	if analysis.Replica == ReplicaNone && analysis.TotalCapacityGB > 500 {
		suggestions = append(suggestions, "大容量存储建议启用副本保护，防止数据丢失")
	}

	// 成本优化建议
	if analysis.MonthlyCost.EffectivePerGB > 1.0 {
		suggestions = append(suggestions, "单位成本较高，可考虑切换到混合存储或 HDD 降低成本")
	}

	// TCO 建议
	if analysis.ThreeYearCost.TotalCost > 0 {
		suggestions = append(suggestions, fmt.Sprintf("三年 TCO 约 %.2f 元，建议定期评估存储策略", analysis.ThreeYearCost.TotalCost))
	}

	// 如果没有建议，添加默认建议
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前存储配置合理，建议定期监控使用情况")
	}

	return suggestions
}
