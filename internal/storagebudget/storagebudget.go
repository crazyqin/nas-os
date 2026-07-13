// Package storagebudget 提供存储预算智能分配器功能。
// 对标 Synology 存储效率分析和 TrueNAS 容量规划，
// 根据用户使用模式、容量趋势、预算约束智能分配存储资源。
package storagebudget

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// BudgetAdvisor 存储预算智能分配器。
type BudgetAdvisor struct{}

// NewAdvisor 创建分配器实例。
func NewAdvisor() *BudgetAdvisor {
	return &BudgetAdvisor{}
}

// ========== 选项与输入结构 ==========

// AllocateOptions 分配选项。
type AllocateOptions struct {
	TotalCapacityGB float64    // 总容量（GB）
	Shares          []ShareInfo // 共享文件夹列表
	Users           []UserInfo // 用户列表
	BudgetTier     string     // 预算层级: "economy", "standard", "premium"
}

// ShareInfo 共享文件夹信息。
type ShareInfo struct {
	Name           string  // 名称
	CurrentUsageGB float64 // 当前使用量（GB）
	GrowthRate     float64 // 月增长率（GB/月）
	Priority       int     // 优先级 1-5，5最高
	Type           string  // 类型: "media", "backup", "archive", "work", "home"
}

// UserInfo 用户信息。
type UserInfo struct {
	Name     string  // 用户名
	UsageGB  float64 // 当前使用量（GB）
	QuotaGB  float64 // 当前配额（GB）
}

// ========== 输出结构 ==========

// AllocationPlan 分配计划。
type AllocationPlan struct {
	ShareAllocations map[string]float64 // 共享文件夹分配量（GB）
	UserQuotas       map[string]float64 // 用户配额（GB）
	ReservedGB       float64            // 保留空间（GB）
	Reasoning        string             // 分配理由
	Warnings         []string           // 警告信息
}

// UsagePoint 历史使用量数据点。
type UsagePoint struct {
	Timestamp int64   // Unix 时间戳
	UsageGB   float64 // 使用量（GB）
}

// GrowthPrediction 容量增长预测。
type GrowthPrediction struct {
	Trend             string // 趋势: "growing", "stable", "shrinking"
	PredictedFullDate string // 预计满载日期
	MonthsUntilFull   int    // 距离满载月数
	RecommendedAction string // 建议操作
}

// ShareUsage 共享文件夹当前使用情况。
type ShareUsage struct {
	Name               string  // 名称
	UsageGB            float64 // 实际使用量（GB）
	AllocatedGB        float64 // 分配量（GB）
	UtilizationPercent float64 // 利用率百分比
}

// MisallocationReport 错配报告。
type MisallocationReport struct {
	Overallocated            []string            // 过度分配的共享
	Underallocated           []string            // 分配不足的共享
	ReallocationSuggestions map[string]float64  // 重新分配建议（GB）
	TotalWastedGB            float64            // 总浪费空间（GB）
}

// BudgetConstraints 预算约束。
type BudgetConstraints struct {
	TotalBudget   float64          // 总预算（GB 表示容量预算）
	Priorities    map[string]int   // 各项优先级
	MinReservedGB float64          // 最小保留空间（GB）
}

// BudgetOptimization 预算优化结果。
type BudgetOptimization struct {
	RecommendedAllocations map[string]float64 // 推荐分配（GB）
	EstimatedSavings       float64            // 预计节省（GB）
	RiskLevel             string             // 风险等级: "low", "medium", "high"
	Tradeoffs             []string           // 权衡说明
}

// ========== 核心方法 ==========

// Allocate 根据选项智能分配存储资源。
func (a *BudgetAdvisor) Allocate(opts AllocateOptions) (*AllocationPlan, error) {
	if opts.TotalCapacityGB <= 0 {
		return nil, fmt.Errorf("total capacity must be positive")
	}
	if len(opts.Shares) == 0 && len(opts.Users) == 0 {
		return nil, fmt.Errorf("at least one share or user must be specified")
	}

	plan := &AllocationPlan{
		ShareAllocations: make(map[string]float64),
		UserQuotas:       make(map[string]float64),
		Warnings:         []string{},
	}

	// 确定保留空间比例
	reserveRatio := 0.10 // 默认 10%
	switch opts.BudgetTier {
	case "economy":
		reserveRatio = 0.05
	case "standard":
		reserveRatio = 0.10
	case "premium":
		reserveRatio = 0.15
	}
	reserved := opts.TotalCapacityGB * reserveRatio
	plan.ReservedGB = reserved
	available := opts.TotalCapacityGB - reserved

	// 计算共享文件夹当前总使用量
	var totalShareUsage float64
	for _, s := range opts.Shares {
		totalShareUsage += s.CurrentUsageGB
	}

	// 计算用户当前总使用量
	var totalUserUsage float64
	for _, u := range opts.Users {
		totalUserUsage += u.UsageGB
	}

	// 检查超额
	totalDemand := totalShareUsage + totalUserUsage
	if totalDemand > available {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("当前总需求 %.2f GB 超过可用容量 %.2f GB，需要缩减分配", totalDemand, available))
	}

	// --- 分配共享文件夹 ---
	if len(opts.Shares) > 0 {
		// 按优先级加权分配
		type shareWeight struct {
			share  ShareInfo
			weight float64
		}
		var weights []shareWeight
		var totalWeight float64
		for _, s := range opts.Shares {
			w := float64(s.Priority)
			if w < 1 {
				w = 1
			}
			// 按类型调整权重
			switch s.Type {
			case "backup":
				w *= 1.3
			case "archive":
				w *= 0.7
			case "media":
				w *= 1.1
			case "work":
				w *= 1.2
			case "home":
				w *= 1.0
			}
			// 考虑增长率：增长快的多分配
			if s.GrowthRate > 0 {
				w *= (1.0 + math.Min(s.GrowthRate/10.0, 0.5))
			}
			weights = append(weights, shareWeight{share: s, weight: w})
			totalWeight += w
		}

		// 共享分配占总可用容量的一定比例（预留用户空间）
		sharePool := available
		if len(opts.Users) > 0 {
			// 按当前使用比例划分
			if totalDemand > 0 {
				sharePool = available * (totalShareUsage / totalDemand)
			} else {
				sharePool = available * 0.6
			}
		}

		for _, sw := range weights {
			alloc := sharePool * (sw.weight / totalWeight)
			// 不低于当前使用量
			if alloc < sw.share.CurrentUsageGB {
				alloc = sw.share.CurrentUsageGB
				plan.Warnings = append(plan.Warnings,
					fmt.Sprintf("共享 %s 分配量已下调至当前使用量 %.2f GB", sw.share.Name, alloc))
			}
			plan.ShareAllocations[sw.share.Name] = math.Round(alloc*100) / 100
		}
	}

	// --- 分配用户配额 ---
	if len(opts.Users) > 0 {
		userPool := available
		if len(opts.Shares) > 0 {
			if totalDemand > 0 {
				userPool = available * (totalUserUsage / totalDemand)
			} else {
				userPool = available * 0.4
			}
		}

		// 按当前使用量加权，但有最低保障
		var totalU float64
		for _, u := range opts.Users {
			totalU += u.UsageGB
		}

		if totalU == 0 {
			// 平均分配
			perUser := userPool / float64(len(opts.Users))
			for _, u := range opts.Users {
				plan.UserQuotas[u.Name] = math.Round(perUser*100) / 100
			}
		} else {
			for _, u := range opts.Users {
				quota := userPool * (u.UsageGB / totalU)
				// 不低于当前使用量
				if quota < u.UsageGB {
					quota = u.UsageGB
				}
				plan.UserQuotas[u.Name] = math.Round(quota*100) / 100
			}
		}
	}

	// --- 生成理由 ---
	plan.Reasoning = fmt.Sprintf(
		"总容量 %.2f GB，保留 %.2f GB（%.0f%%），可用 %.2f GB。"+
			"按优先级和类型加权分配 %d 个共享、%d 个用户。",
		opts.TotalCapacityGB, reserved, reserveRatio*100, available,
		len(opts.Shares), len(opts.Users))

	// 检查分配总量不超过可用
	var allocTotal float64
	for _, v := range plan.ShareAllocations {
		allocTotal += v
	}
	for _, v := range plan.UserQuotas {
		allocTotal += v
	}
	if allocTotal > available {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("分配总量 %.2f GB 超过可用容量 %.2f GB", allocTotal, available))
	}

	return plan, nil
}

// PredictGrowth 根据历史使用量数据预测容量增长趋势。
func (a *BudgetAdvisor) PredictGrowth(history []UsagePoint) (*GrowthPrediction, error) {
	if len(history) < 2 {
		return nil, fmt.Errorf("at least 2 data points required for prediction")
	}

	// 按时间排序
	sorted := make([]UsagePoint, len(history))
	copy(sorted, history)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp < sorted[j].Timestamp
	})

	// 线性回归: y = a + b*x，x 为月份偏移
	n := float64(len(sorted))
	firstTs := sorted[0].Timestamp
	var sumX, sumY, sumXY, sumX2 float64
	for _, p := range sorted {
		monthsElapsed := float64(p.Timestamp-firstTs) / (30 * 24 * 3600)
		sumX += monthsElapsed
		sumY += p.UsageGB
		sumXY += monthsElapsed * p.UsageGB
		sumX2 += monthsElapsed * monthsElapsed
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return &GrowthPrediction{
			Trend:             "stable",
			PredictedFullDate: "unknown",
			MonthsUntilFull:   -1,
			RecommendedAction: "数据点不足或无变化，继续观察",
		}, nil
	}

	slope := (n*sumXY - sumX*sumY) / denominator
	intercept := (sumY - slope*sumX) / n

	// 判断趋势
	var trend string
	if slope > 0.5 {
		trend = "growing"
	} else if slope < -0.5 {
		trend = "shrinking"
	} else {
		trend = "stable"
	}

	prediction := &GrowthPrediction{Trend: trend}

	if slope > 0 {
		// 预测满载时间：假设系统总容量为最后一个数据点的值加一个合理上限
		// 用斜率推断增长到某个阈值的时间
		currentUsage := sorted[len(sorted)-1].UsageGB
		// 假设满载容量为当前使用量的 2 倍作为参考（或增长 100%）
		fullCapacity := currentUsage * 2
		if fullCapacity <= currentUsage {
			fullCapacity = currentUsage + 100
		}
		monthsToFull := int(math.Ceil((fullCapacity - intercept) / slope))
		if monthsToFull < 0 {
			monthsToFull = 0
		}
		prediction.MonthsUntilFull = monthsToFull

		if monthsToFull > 0 {
			fullDate := time.Now().AddDate(0, monthsToFull, 0)
			prediction.PredictedFullDate = fullDate.Format("2006-01-02")
		} else {
			prediction.PredictedFullDate = "already full"
		}

		switch {
		case monthsToFull <= 3:
			prediction.RecommendedAction = "紧急：容量即将耗尽，立即扩容或清理"
		case monthsToFull <= 6:
			prediction.RecommendedAction = "警告：6个月内将满载，制定扩容计划"
		case monthsToFull <= 12:
			prediction.RecommendedAction = "关注：1年内将满载，开始规划"
		default:
			prediction.RecommendedAction = "正常：容量充足，持续监控"
		}
	} else {
		prediction.MonthsUntilFull = -1
		prediction.PredictedFullDate = "no growth predicted"
		if trend == "shrinking" {
			prediction.RecommendedAction = "使用量下降，可回收部分配额"
		} else {
			prediction.RecommendedAction = "使用量稳定，保持当前策略"
		}
	}

	return prediction, nil
}

// DetectMisallocation 检测过度分配和分配不足。
func (a *BudgetAdvisor) DetectMisallocation(current []ShareUsage) (*MisallocationReport, error) {
	if len(current) == 0 {
		return nil, fmt.Errorf("no share usage data provided")
	}

	report := &MisallocationReport{
		Overallocated:            []string{},
		Underallocated:           []string{},
		ReallocationSuggestions:  make(map[string]float64),
	}

	// 利用率阈值
	const overUtilThreshold = 20.0  // 利用率低于 20% 为过度分配
	const underUtilThreshold = 85.0 // 利用率高于 85% 为分配不足

	for _, s := range current {
		if s.AllocatedGB <= 0 {
			report.Underallocated = append(report.Underallocated, s.Name)
			// 建议至少分配当前使用量的 1.2 倍
			report.ReallocationSuggestions[s.Name] = s.UsageGB * 1.2
			continue
		}

		utilization := (s.UsageGB / s.AllocatedGB) * 100.0

		if utilization < overUtilThreshold {
			report.Overallocated = append(report.Overallocated, s.Name)
			// 建议回收到当前使用量的 1.5 倍
			suggested := s.UsageGB * 1.5
			if suggested < 0 {
				suggested = 0
			}
			report.ReallocationSuggestions[s.Name] = math.Round(suggested*100) / 100
			// 浪费的空间 = 原分配 - 建议分配
			wasted := s.AllocatedGB - suggested
			if wasted > 0 {
				report.TotalWastedGB += wasted
			}
		} else if utilization > underUtilThreshold {
			report.Underallocated = append(report.Underallocated, s.Name)
			// 建议增加到当前使用量的 1.3 倍
			suggested := s.UsageGB * 1.3
			report.ReallocationSuggestions[s.Name] = math.Round(suggested*100) / 100
		}
	}

	report.TotalWastedGB = math.Round(report.TotalWastedGB*100) / 100

	return report, nil
}

// OptimizeBudget 在给定约束下优化存储预算分配。
func (a *BudgetAdvisor) OptimizeBudget(constraints BudgetConstraints) (*BudgetOptimization, error) {
	if constraints.TotalBudget <= 0 {
		return nil, fmt.Errorf("total budget must be positive")
	}

	result := &BudgetOptimization{
		RecommendedAllocations: make(map[string]float64),
		Tradeoffs:             []string{},
	}

	// 保留空间
	reserved := constraints.MinReservedGB
	if reserved < 0 {
		reserved = 0
	}
	if reserved >= constraints.TotalBudget {
		return nil, fmt.Errorf("minimum reserved exceeds total budget")
	}

	available := constraints.TotalBudget - reserved

	// 按优先级排序
	type item struct {
		name     string
		priority int
	}
	var items []item
	var totalPriority int
	for name, pri := range constraints.Priorities {
		items = append(items, item{name: name, priority: pri})
		totalPriority += pri
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].priority > items[j].priority
	})

	if totalPriority == 0 {
		// 无优先级信息，平均分配
		perItem := available / float64(len(items))
		for _, it := range items {
			result.RecommendedAllocations[it.name] = math.Round(perItem*100) / 100
		}
	} else {
		for _, it := range items {
			alloc := available * (float64(it.priority) / float64(totalPriority))
			result.RecommendedAllocations[it.name] = math.Round(alloc*100) / 100
		}
	}

	// 估算节省：高优先项获得更多空间，减少低优先项浪费
	result.EstimatedSavings = math.Round(available*0.05*100) / 100 // 预估优化节省 5%

	// 风险评估
	highPriCount := 0
	for _, pri := range constraints.Priorities {
		if pri >= 4 {
			highPriCount++
		}
	}
	switch {
	case highPriCount > len(constraints.Priorities)/2:
		result.RiskLevel = "high" // 过多高优先项导致风险
	case highPriCount > 0:
		result.RiskLevel = "medium"
	default:
		result.RiskLevel = "low"
	}

	// 权衡说明
	result.Tradeoffs = append(result.Tradeoffs,
		fmt.Sprintf("保留 %.2f GB 作为缓冲，可用 %.2f GB", reserved, available))
	if result.RiskLevel == "high" {
		result.Tradeoffs = append(result.Tradeoffs,
			"高优先项较多，低优先项可能空间不足")
	}
	if len(items) > 0 {
		result.Tradeoffs = append(result.Tradeoffs,
			fmt.Sprintf("最高优先项 %s 获得最大分配 %s%.2f GB",
				items[0].name, "", result.RecommendedAllocations[items[0].name]))
	}
	result.Tradeoffs = append(result.Tradeoffs,
		"建议定期复查优先级，根据实际使用调整")

	return result, nil
}