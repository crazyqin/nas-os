// Package storagecostanalyzer 存储成本分析器 - 容量规划
package storagecostanalyzer

import (
	"fmt"
	"math"
	"time"
)

// CapacityPlanner 容量规划器.
type CapacityPlanner struct {
	manager *Manager
}

// NewCapacityPlanner 创建容量规划器.
func NewCapacityPlanner(manager *Manager) *CapacityPlanner {
	return &CapacityPlanner{manager: manager}
}

// CapacityPlanningInput 容量规划输入.
type CapacityPlanningInput struct {
	// Tier 存储层级.
	Tier StorageTier
	// PlanningMonths 规划周期（月）.
	PlanningMonths int
	// GrowthModel 增长模型（linear/exponential）.
	GrowthModel string
	// TargetUtilization 目标利用率（%）.
	TargetUtilization float64
	// ExpansionCostPerTB 每TB扩容成本.
	ExpansionCostPerTB float64
	// IncludeBuffer 是否包含缓冲区.
	IncludeBuffer bool
	// BufferPercent 缓冲百分比.
	BufferPercent float64
}

// DefaultCapacityPlanningInput 默认容量规划输入.
func DefaultCapacityPlanningInput(tier StorageTier, months int) CapacityPlanningInput {
	return CapacityPlanningInput{
		Tier:               tier,
		PlanningMonths:     months,
		GrowthModel:        "linear",
		TargetUtilization:  70.0,
		ExpansionCostPerTB: 500.0,
		IncludeBuffer:      true,
		BufferPercent:      20.0,
	}
}

// GenerateCapacityPlan 生成容量规划.
func (p *CapacityPlanner) GenerateCapacityPlan(input CapacityPlanningInput) (*CapacityPlan, error) {
	p.manager.mu.RLock()
	defer p.manager.mu.RUnlock()

	ts, ok := p.manager.tiers[input.Tier]
	if !ok {
		return nil, ErrTierNotFound
	}

	if input.PlanningMonths <= 0 {
		input.PlanningMonths = 12
	}

	cfg := ts.config
	currentCapacityTB := cfg.CapacityTB
	currentUsedTB := cfg.UsedTB
	currentUtilization := 0.0
	if currentCapacityTB > 0 {
		currentUtilization = (currentUsedTB / currentCapacityTB) * 100
	}

	// 计算增长率
	growthRateMonthly := p.calculateGrowthRate(ts, input.GrowthModel)

	// 计算满容量时间
	monthsUntilFull := p.calculateMonthsUntilFull(currentUsedTB, currentCapacityTB, growthRateMonthly)
	var fullDate *time.Time
	if monthsUntilFull > 0 {
		date := p.manager.nowFunc().AddDate(0, monthsUntilFull, 0)
		fullDate = &date
	}

	// 确定建议操作
	recommendedAction, recommendedCapacity, urgency, rationale, timeline, steps := p.determineRecommendation(
		currentCapacityTB,
		currentUsedTB,
		currentUtilization,
		growthRateMonthly,
		monthsUntilFull,
		input.PlanningMonths,
		input.TargetUtilization,
		input.IncludeBuffer,
		input.BufferPercent,
	)

	// 计算扩容成本
	expansionTB := recommendedCapacity - currentCapacityTB
	if expansionTB < 0 {
		expansionTB = 0
	}
	totalExpansionCost := expansionTB * input.ExpansionCostPerTB

	return &CapacityPlan{
		GeneratedAt:           p.manager.nowFunc(),
		Tier:                  input.Tier,
		TierName:              cfg.Name,
		CurrentCapacityTB:     currentCapacityTB,
		CurrentUsedTB:         currentUsedTB,
		CurrentUtilization:    currentUtilization,
		GrowthRateMonthly:     growthRateMonthly,
		MonthsUntilFull:       monthsUntilFull,
		FullDate:              fullDate,
		RecommendedAction:     recommendedAction,
		RecommendedCapacityTB: recommendedCapacity,
		ExpansionCostTB:       input.ExpansionCostPerTB,
		TotalExpansionCost:    totalExpansionCost,
		Urgency:               urgency,
		Rationale:             rationale,
		Timeline:              timeline,
		Steps:                 steps,
	}, nil
}

// calculateGrowthRate 计算增长率.
func (p *CapacityPlanner) calculateGrowthRate(ts *tierState, model string) float64 {
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

	switch model {
	case "exponential":
		// 指数增长模型
		if first.Amount <= 0 {
			return ts.config.UsedTB * 0.05
		}
		growthRatio := last.Amount / first.Amount
		if growthRatio <= 1 {
			return 0
		}
		monthlyGrowthRate := math.Pow(growthRatio, 1/months) - 1
		return ts.config.UsedTB * monthlyGrowthRate
	default:
		// 线性增长模型
		costGrowth := last.Amount - first.Amount
		if costGrowth <= 0 {
			return 0
		}
		if first.Amount <= 0 {
			return 0
		}
		growthRatio := costGrowth / first.Amount
		currentUsed := ts.config.UsedTB
		estimatedGrowth := currentUsed * growthRatio / months
		return math.Max(0, estimatedGrowth)
	}
}

// calculateMonthsUntilFull 计算满容量月数.
func (p *CapacityPlanner) calculateMonthsUntilFull(currentUsed, capacity, growthRateMonthly float64) int {
	if growthRateMonthly <= 0 || currentUsed >= capacity {
		return 0
	}

	remaining := capacity - currentUsed
	months := int(math.Ceil(remaining / growthRateMonthly))
	return months
}

// determineRecommendation 确定建议.
func (p *CapacityPlanner) determineRecommendation(
	capacity, used, utilization, growthRate float64,
	monthsUntilFull, planningMonths int,
	targetUtilization float64,
	includeBuffer bool,
	bufferPercent float64,
) (action string, recommendedCapacity float64, urgency, rationale, timeline string, steps []string) {
	if utilization >= 90 {
		// 紧急扩容
		action = "expand"
		recommendedCapacity = capacity * 1.5 // 扩容50%
		urgency = "critical"
		rationale = fmt.Sprintf("利用率 %.1f%% 已超过 90%%，存储空间即将耗尽", utilization)
		timeline = "建议1个月内完成扩容"
		steps = []string{
			"评估当前存储需求",
			"采购新存储设备",
			"完成硬件安装和配置",
			"扩展存储池",
			"验证存储可用性",
		}
	} else if utilization >= 75 {
		// 计划扩容
		action = "expand"
		recommendedCapacity = capacity * 1.3 // 扩容30%
		urgency = "high"
		rationale = fmt.Sprintf("利用率 %.1f%% 接近警戒线，建议提前扩容", utilization)
		timeline = "建议3个月内完成扩容"
		steps = []string{
			"监控存储增长趋势",
			"制定扩容计划",
			"采购存储设备",
			"安排维护窗口进行扩容",
		}
	} else if monthsUntilFull > 0 && monthsUntilFull <= planningMonths {
		// 规划扩容
		action = "plan_expand"
		// 计算规划期结束时需要的容量
		projectedUsed := used + growthRate*float64(planningMonths)
		if includeBuffer {
			projectedUsed *= (1 + bufferPercent/100)
		}
		recommendedCapacity = projectedUsed / (targetUtilization / 100)
		urgency = "medium"
		rationale = fmt.Sprintf("按当前增长趋势，预计 %d 个月后满容量", monthsUntilFull)
		timeline = fmt.Sprintf("建议在 %d 个月内完成扩容准备", monthsUntilFull-1)
		steps = []string{
			"持续监控增长趋势",
			"评估扩容方案和成本",
			"提前采购存储设备",
			"在容量达到警戒线前完成扩容",
		}
	} else if utilization < 30 {
		// 优化建议
		action = "optimize"
		recommendedCapacity = capacity * 0.7 // 建议缩减30%
		urgency = "low"
		rationale = fmt.Sprintf("利用率仅 %.1f%%，存在资源浪费", utilization)
		timeline = "可随时进行优化"
		steps = []string{
			"分析数据访问模式",
			"识别可迁移的冷数据",
			"制定数据分层策略",
			"执行数据迁移",
			"缩减闲置存储容量",
		}
	} else {
		// 维持现状
		action = "maintain"
		recommendedCapacity = capacity
		urgency = "low"
		rationale = fmt.Sprintf("利用率 %.1f%% 处于健康范围", utilization)
		timeline = "无需立即行动"
		steps = []string{
			"继续监控存储使用情况",
			"定期审查容量规划",
		}
	}

	return action, recommendedCapacity, urgency, rationale, timeline, steps
}

// ForecastCapacity 预测容量使用.
func (p *CapacityPlanner) ForecastCapacity(input CapacityPlanningInput) (*CapacityForecast, error) {
	p.manager.mu.RLock()
	defer p.manager.mu.RUnlock()

	ts, ok := p.manager.tiers[input.Tier]
	if !ok {
		return nil, ErrTierNotFound
	}

	if input.PlanningMonths <= 0 {
		input.PlanningMonths = 12
	}

	cfg := ts.config
	currentUsedTB := cfg.UsedTB
	currentCapacityTB := cfg.CapacityTB
	growthRate := p.calculateGrowthRate(ts, input.GrowthModel)

	// 生成预测点
	projectedPoints := make([]ProjectedPoint, 0, input.PlanningMonths)
	for i := 1; i <= input.PlanningMonths; i++ {
		var projectedUsed float64
		switch input.GrowthModel {
		case "exponential":
			if growthRate > 0 && currentUsedTB > 0 {
				monthlyRate := growthRate / currentUsedTB
				projectedUsed = currentUsedTB * math.Pow(1+monthlyRate, float64(i))
			} else {
				projectedUsed = currentUsedTB + growthRate*float64(i)
			}
		default:
			projectedUsed = currentUsedTB + growthRate*float64(i)
		}

		if projectedUsed > currentCapacityTB {
			projectedUsed = currentCapacityTB
		}

		utilization := 0.0
		if currentCapacityTB > 0 {
			utilization = (projectedUsed / currentCapacityTB) * 100
		}

		// 置信区间（±10%）
		lowerBound := projectedUsed * 0.9
		upperBound := math.Min(projectedUsed*1.1, currentCapacityTB)

		projectedPoints = append(projectedPoints, ProjectedPoint{
			Date:                 p.manager.nowFunc().AddDate(0, i, 0),
			ProjectedUsedTB:      projectedUsed,
			ProjectedUtilization: utilization,
			LowerBound:           lowerBound,
			UpperBound:           upperBound,
		})
	}

	// 计算满容量时间
	monthsUntilFull := p.calculateMonthsUntilFull(currentUsedTB, currentCapacityTB, growthRate)
	var fullDate *time.Time
	if monthsUntilFull > 0 {
		date := p.manager.nowFunc().AddDate(0, monthsUntilFull, 0)
		fullDate = &date
	}

	// 是否需要扩容
	expansionNeeded := monthsUntilFull > 0 && monthsUntilFull <= input.PlanningMonths
	recommendedExpansion := 0.0
	if expansionNeeded {
		projectedUsed := currentUsedTB + growthRate*float64(input.PlanningMonths)
		if input.IncludeBuffer {
			projectedUsed *= (1 + input.BufferPercent/100)
		}
		targetCapacity := projectedUsed / (input.TargetUtilization / 100)
		recommendedExpansion = targetCapacity - currentCapacityTB
		if recommendedExpansion < 0 {
			recommendedExpansion = 0
		}
	}

	// 计算日增长量（MB/天）
	growthRateMBPerDay := growthRate * 1024 * 1024 / 30 // TB/月 -> MB/天

	return &CapacityForecast{
		GeneratedAt:            p.manager.nowFunc(),
		Tier:                   input.Tier,
		CurrentUsedTB:          currentUsedTB,
		CurrentCapacityTB:      currentCapacityTB,
		GrowthRateMBPerDay:     growthRateMBPerDay,
		GrowthModel:            input.GrowthModel,
		DaysUntilFull:          monthsUntilFull * 30,
		FullDate:               fullDate,
		ProjectedUsage:         projectedPoints,
		ExpansionNeeded:        expansionNeeded,
		RecommendedExpansionTB: recommendedExpansion,
	}, nil
}

// GenerateMultiTierPlan 生成多层级容量规划.
func (p *CapacityPlanner) GenerateMultiTierPlan(planningMonths int, targetUtilization float64) (*MultiTierCapacityPlan, error) {
	p.manager.mu.RLock()
	defer p.manager.mu.RUnlock()

	if planningMonths <= 0 {
		planningMonths = 12
	}
	if targetUtilization <= 0 || targetUtilization > 100 {
		targetUtilization = 70.0
	}

	var tierPlans []CapacityPlan
	totalCurrentCapacity := 0.0
	totalCurrentUsed := 0.0
	totalRecommendedCapacity := 0.0
	totalExpansionCost := 0.0

	for tier, ts := range p.manager.tiers {
		input := CapacityPlanningInput{
			Tier:               tier,
			PlanningMonths:     planningMonths,
			GrowthModel:        "linear",
			TargetUtilization:  targetUtilization,
			ExpansionCostPerTB: 500.0,
			IncludeBuffer:      true,
			BufferPercent:      20.0,
		}

		plan, err := p.generateTierPlan(ts, input)
		if err != nil {
			continue
		}

		tierPlans = append(tierPlans, *plan)
		totalCurrentCapacity += plan.CurrentCapacityTB
		totalCurrentUsed += plan.CurrentUsedTB
		totalRecommendedCapacity += plan.RecommendedCapacityTB
		totalExpansionCost += plan.TotalExpansionCost
	}

	overallUtilization := 0.0
	if totalCurrentCapacity > 0 {
		overallUtilization = (totalCurrentUsed / totalCurrentCapacity) * 100
	}

	// 确定整体紧急程度
	overallUrgency := "low"
	for _, plan := range tierPlans {
		if plan.Urgency == "critical" {
			overallUrgency = "critical"
			break
		} else if plan.Urgency == "high" && overallUrgency != "critical" {
			overallUrgency = "high"
		} else if plan.Urgency == "medium" && overallUrgency == "low" {
			overallUrgency = "medium"
		}
	}

	return &MultiTierCapacityPlan{
		GeneratedAt:            p.manager.nowFunc(),
		PlanningMonths:         planningMonths,
		TargetUtilization:      targetUtilization,
		TierPlans:              tierPlans,
		TotalCurrentCapacityTB: totalCurrentCapacity,
		TotalCurrentUsedTB:     totalCurrentUsed,
		OverallUtilization:     overallUtilization,
		TotalRecommendedTB:     totalRecommendedCapacity,
		TotalExpansionCost:     totalExpansionCost,
		OverallUrgency:         overallUrgency,
	}, nil
}

// generateTierPlan 生成单层级规划（内部方法，不加锁）.
func (p *CapacityPlanner) generateTierPlan(ts *tierState, input CapacityPlanningInput) (*CapacityPlan, error) {
	cfg := ts.config
	currentCapacityTB := cfg.CapacityTB
	currentUsedTB := cfg.UsedTB
	currentUtilization := 0.0
	if currentCapacityTB > 0 {
		currentUtilization = (currentUsedTB / currentCapacityTB) * 100
	}

	growthRateMonthly := p.calculateGrowthRate(ts, input.GrowthModel)
	monthsUntilFull := p.calculateMonthsUntilFull(currentUsedTB, currentCapacityTB, growthRateMonthly)
	var fullDate *time.Time
	if monthsUntilFull > 0 {
		date := p.manager.nowFunc().AddDate(0, monthsUntilFull, 0)
		fullDate = &date
	}

	recommendedAction, recommendedCapacity, urgency, rationale, timeline, steps := p.determineRecommendation(
		currentCapacityTB,
		currentUsedTB,
		currentUtilization,
		growthRateMonthly,
		monthsUntilFull,
		input.PlanningMonths,
		input.TargetUtilization,
		input.IncludeBuffer,
		input.BufferPercent,
	)

	expansionTB := recommendedCapacity - currentCapacityTB
	if expansionTB < 0 {
		expansionTB = 0
	}
	totalExpansionCost := expansionTB * input.ExpansionCostPerTB

	return &CapacityPlan{
		GeneratedAt:           p.manager.nowFunc(),
		Tier:                  input.Tier,
		TierName:              cfg.Name,
		CurrentCapacityTB:     currentCapacityTB,
		CurrentUsedTB:         currentUsedTB,
		CurrentUtilization:    currentUtilization,
		GrowthRateMonthly:     growthRateMonthly,
		MonthsUntilFull:       monthsUntilFull,
		FullDate:              fullDate,
		RecommendedAction:     recommendedAction,
		RecommendedCapacityTB: recommendedCapacity,
		ExpansionCostTB:       input.ExpansionCostPerTB,
		TotalExpansionCost:    totalExpansionCost,
		Urgency:               urgency,
		Rationale:             rationale,
		Timeline:              timeline,
		Steps:                 steps,
	}, nil
}

// MultiTierCapacityPlan 多层级容量规划.
type MultiTierCapacityPlan struct {
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generatedAt"`
	// PlanningMonths 规划周期（月）.
	PlanningMonths int `json:"planningMonths"`
	// TargetUtilization 目标利用率（%）.
	TargetUtilization float64 `json:"targetUtilization"`
	// TierPlans 各层级规划.
	TierPlans []CapacityPlan `json:"tierPlans"`
	// TotalCurrentCapacityTB 当前总容量（TB）.
	TotalCurrentCapacityTB float64 `json:"totalCurrentCapacityTB"`
	// TotalCurrentUsedTB 当前总已用（TB）.
	TotalCurrentUsedTB float64 `json:"totalCurrentUsedTB"`
	// OverallUtilization 总体利用率（%）.
	OverallUtilization float64 `json:"overallUtilization"`
	// TotalRecommendedTB 总建议容量（TB）.
	TotalRecommendedTB float64 `json:"totalRecommendedTB"`
	// TotalExpansionCost 总扩容成本.
	TotalExpansionCost float64 `json:"totalExpansionCost"`
	// OverallUrgency 整体紧急程度.
	OverallUrgency string `json:"overallUrgency"`
}
