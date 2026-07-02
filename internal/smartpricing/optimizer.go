// Package smartpricing 提供智能存储定价分析功能
package smartpricing

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Optimizer 智能存储方案优化器.
type Optimizer struct {
	mu       sync.RWMutex
	analyzer *Analyzer
}

// NewOptimizer 创建智能优化器.
func NewOptimizer(analyzer *Analyzer) *Optimizer {
	return &Optimizer{
		analyzer: analyzer,
	}
}

// Optimize 智能推荐最优存储方案.
func (o *Optimizer) Optimize(req OptimizeRequest) (*OptimizeResult, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	o.mu.RLock()
	defer o.mu.RUnlock()

	// 获取所有可用方案
	plans := o.analyzer.GetPlans()

	// 过滤并评分所有方案
	var recommendations []Recommendation
	for _, plan := range plans {
		rec, ok := o.evaluatePlan(plan, req)
		if !ok {
			continue
		}
		recommendations = append(recommendations, rec)
	}

	if len(recommendations) == 0 {
		return &OptimizeResult{
			Request:         req,
			Recommendations: []Recommendation{},
			BestOption:      nil,
			GeneratedAt:     now(),
		}, nil
	}

	// 按评分排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Score > recommendations[j].Score
	})

	// 设置排名
	for i := range recommendations {
		recommendations[i].Rank = i + 1
	}

	result := &OptimizeResult{
		Request:         req,
		Recommendations: recommendations,
		BestOption:      &recommendations[0],
		GeneratedAt:     now(),
	}

	return result, nil
}

// evaluatePlan 评估单个方案.
func (o *Optimizer) evaluatePlan(plan PricingPlan, req OptimizeRequest) (Recommendation, bool) {
	rec := Recommendation{
		Plan: plan,
	}

	// 检查容量要求
	if req.CapacityGB < plan.MinCapacity {
		return rec, false
	}
	if plan.MaxCapacity > 0 && req.CapacityGB > plan.MaxCapacity {
		return rec, false
	}

	// 计算成本
	costAnalysis, err := o.analyzer.Analyze(req.CapacityGB, plan.Tier, plan.Replica, req.Workload)
	if err != nil {
		return rec, false
	}

	rec.TotalCost = costAnalysis.MonthlyCost.TotalCost
	rec.CostAnalysis = costAnalysis

	if req.CapacityGB > 0 {
		rec.CostPerGB = rec.TotalCost / float64(req.CapacityGB)
	}

	// 检查预算约束
	var reasons []string
	var warnings []string

	if req.MaxBudget > 0 && rec.TotalCost > req.MaxBudget {
		warnings = append(warnings, fmt.Sprintf("月度成本 %.2f 元超出预算 %.2f 元", rec.TotalCost, req.MaxBudget))
	}

	// 检查 IOPS 要求
	if req.MinIOPS > 0 && plan.IOPSLimit < req.MinIOPS {
		warnings = append(warnings, fmt.Sprintf("IOPS %d 低于要求的 %d", plan.IOPSLimit, req.MinIOPS))
	}

	// 检查延迟要求
	if req.MaxLatencyMs > 0 && plan.ReadLatencyMs > req.MaxLatencyMs {
		warnings = append(warnings, fmt.Sprintf("读延迟 %.2fms 超出要求的 %.2fms", plan.ReadLatencyMs, req.MaxLatencyMs))
	}

	// 计算综合评分
	score := o.calculateScore(plan, req, rec.TotalCost, len(warnings))
	rec.Score = score

	// 生成推荐理由
	reasons = o.generateReasons(plan, req, rec.TotalCost)
	rec.Reasons = reasons
	rec.Warnings = warnings

	return rec, true
}

// calculateScore 计算综合评分.
func (o *Optimizer) calculateScore(plan PricingPlan, req OptimizeRequest, totalCost float64, warningCount int) float64 {
	score := 100.0

	// 1. 成本评分（40%）
	costScore := 40.0
	if req.MaxBudget > 0 {
		costRatio := totalCost / req.MaxBudget
		if costRatio <= 0.5 {
			costScore = 40.0 // 极优
		} else if costRatio <= 0.8 {
			costScore = 35.0 // 良好
		} else if costRatio <= 1.0 {
			costScore = 25.0 // 一般
		} else {
			costScore = 10.0 // 超预算
		}
	} else {
		// 无预算限制，按性价比评分
		switch plan.Tier {
		case TierHDD:
			costScore = 40.0
		case TierHybrid:
			costScore = 35.0
		default:
			costScore = 25.0
		}
	}

	// 2. 性能评分（30%）
	perfScore := 30.0
	if req.MinIOPS > 0 {
		iopsRatio := float64(plan.IOPSLimit) / float64(req.MinIOPS)
		if iopsRatio >= 2.0 {
			perfScore = 30.0
		} else if iopsRatio >= 1.0 {
			perfScore = 25.0
		} else {
			perfScore = 10.0
		}
	}

	// 3. 延迟评分（20%）
	latencyScore := 20.0
	if req.MaxLatencyMs > 0 {
		latencyRatio := plan.ReadLatencyMs / req.MaxLatencyMs
		if latencyRatio <= 0.5 {
			latencyScore = 20.0
		} else if latencyRatio <= 1.0 {
			latencyScore = 15.0
		} else {
			latencyScore = 5.0
		}
	}

	// 4. 工作负载匹配度（10%）
	workloadScore := 10.0
	if o.isWorkloadMatch(plan.Tier, req.Workload) {
		workloadScore = 10.0
	} else {
		workloadScore = 5.0
	}

	// 减去警告扣分
	warningPenalty := float64(warningCount) * 5.0

	score = costScore + perfScore + latencyScore + workloadScore - warningPenalty
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return math.Round(score*10) / 10
}

// isWorkloadMatch 检查层级是否匹配工作负载.
func (o *Optimizer) isWorkloadMatch(tier StorageTier, workload WorkloadType) bool {
	switch workload {
	case WorkloadCold:
		return tier == TierHDD
	case WorkloadWarm:
		return tier == TierHybrid
	case WorkloadHot:
		return tier == TierSSD
	case WorkloadMixed:
		return tier == TierHybrid || tier == TierSSD
	default:
		return true
	}
}

// generateReasons 生成推荐理由.
func (o *Optimizer) generateReasons(plan PricingPlan, req OptimizeRequest, totalCost float64) []string {
	var reasons []string

	// 层级优势
	switch plan.Tier {
	case TierSSD:
		reasons = append(reasons, "高性能 SSD 存储，读写延迟极低")
	case TierHDD:
		reasons = append(reasons, "大容量 HDD 存储，单位成本最低")
	case TierHybrid:
		reasons = append(reasons, "混合存储方案，兼顾性能和成本")
	}

	// 副本策略
	switch plan.Replica {
	case ReplicaMirror:
		reasons = append(reasons, "双副本保护，数据安全性高")
	case ReplicaRAID5:
		reasons = append(reasons, "RAID5 容错，兼顾容量和可靠性")
	case ReplicaRAID6:
		reasons = append(reasons, "RAID6 双盘容错，适合大规模存储")
	case ReplicaTriple:
		reasons = append(reasons, "三副本保护，最高数据安全性")
	}

	// 性能指标
	if plan.IOPSLimit > 0 {
		reasons = append(reasons, fmt.Sprintf("IOPS 上限 %d，满足高并发需求", plan.IOPSLimit))
	}
	if plan.ReadLatencyMs < 1.0 {
		reasons = append(reasons, fmt.Sprintf("读延迟 %.1fms，响应极快", plan.ReadLatencyMs))
	}

	// 成本优势
	if req.MaxBudget > 0 && totalCost <= req.MaxBudget*0.7 {
		reasons = append(reasons, "月度成本远低于预算，性价比高")
	}

	return reasons
}

// now 返回当前时间（方便测试替换）.
var now = func() time.Time {
	return time.Now()
}
