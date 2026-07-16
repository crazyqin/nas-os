// Package smartpricing 提供智能存储定价分析功能
package smartpricing

import (
	"fmt"
	"sync"
	"time"
)

// Analyzer 存储成本分析引擎.
type Analyzer struct {
	mu    sync.RWMutex
	plans []PricingPlan // 可用定价方案列表
}

// NewAnalyzer 创建成本分析引擎.
func NewAnalyzer() *Analyzer {
	a := &Analyzer{
		plans: defaultPlans(),
	}
	return a
}

// defaultPlans 返回默认定价方案.
func defaultPlans() []PricingPlan {
	return []PricingPlan{
		// SSD 方案
		{
			ID:             "ssd-basic",
			Name:           "SSD 基础版",
			Tier:           TierSSD,
			Replica:        ReplicaNone,
			UnitPrice:      1.2,
			MinCapacity:    10,
			MaxCapacity:    0,
			IOPSLimit:      10000,
			ThroughputMB:   500,
			ReadLatencyMs:  0.1,
			WriteLatencyMs: 0.2,
			MonthlyBaseFee: 0,
			TransferFee:    0.05,
			Description:    "高性能 SSD 存储，适合热数据和数据库",
		},
		{
			ID:             "ssd-ha",
			Name:           "SSD 高可用版",
			Tier:           TierSSD,
			Replica:        ReplicaMirror,
			UnitPrice:      1.2,
			MinCapacity:    50,
			MaxCapacity:    0,
			IOPSLimit:      8000,
			ThroughputMB:   400,
			ReadLatencyMs:  0.15,
			WriteLatencyMs: 0.3,
			MonthlyBaseFee: 50,
			TransferFee:    0.05,
			Description:    "双副本 SSD 存储，适合关键业务数据",
		},
		// HDD 方案
		{
			ID:             "hdd-basic",
			Name:           "HDD 基础版",
			Tier:           TierHDD,
			Replica:        ReplicaNone,
			UnitPrice:      0.25,
			MinCapacity:    100,
			MaxCapacity:    0,
			IOPSLimit:      200,
			ThroughputMB:   150,
			ReadLatencyMs:  5.0,
			WriteLatencyMs: 8.0,
			MonthlyBaseFee: 0,
			TransferFee:    0.03,
			Description:    "大容量 HDD 存储，适合冷数据和归档",
		},
		{
			ID:             "hdd-raid5",
			Name:           "HDD RAID5版",
			Tier:           TierHDD,
			Replica:        ReplicaRAID5,
			UnitPrice:      0.25,
			MinCapacity:    500,
			MaxCapacity:    0,
			IOPSLimit:      150,
			ThroughputMB:   120,
			ReadLatencyMs:  6.0,
			WriteLatencyMs: 10.0,
			MonthlyBaseFee: 30,
			TransferFee:    0.03,
			Description:    "RAID5 HDD 存储，兼顾容量和容错",
		},
		// 混合方案
		{
			ID:             "hybrid-standard",
			Name:           "混合存储标准版",
			Tier:           TierHybrid,
			Replica:        ReplicaNone,
			UnitPrice:      0.7,
			MinCapacity:    100,
			MaxCapacity:    0,
			IOPSLimit:      5000,
			ThroughputMB:   300,
			ReadLatencyMs:  0.5,
			WriteLatencyMs: 2.0,
			MonthlyBaseFee: 20,
			TransferFee:    0.04,
			Description:    "SSD 缓存 + HDD 存储，性价比最优",
		},
		{
			ID:             "hybrid-ha",
			Name:           "混合存储高可用版",
			Tier:           TierHybrid,
			Replica:        ReplicaRAID5,
			UnitPrice:      0.7,
			MinCapacity:    200,
			MaxCapacity:    0,
			IOPSLimit:      4000,
			ThroughputMB:   250,
			ReadLatencyMs:  0.6,
			WriteLatencyMs: 2.5,
			MonthlyBaseFee: 50,
			TransferFee:    0.04,
			Description:    "RAID5 混合存储，适合中型业务",
		},
		// 经济方案
		{
			ID:             "hdd-archive",
			Name:           "HDD 归档版",
			Tier:           TierHDD,
			Replica:        ReplicaNone,
			UnitPrice:      0.15,
			MinCapacity:    1000,
			MaxCapacity:    0,
			IOPSLimit:      50,
			ThroughputMB:   80,
			ReadLatencyMs:  10.0,
			WriteLatencyMs: 15.0,
			MonthlyBaseFee: 0,
			TransferFee:    0.02,
			Description:    "超低成本归档存储，适合备份和冷数据",
		},
	}
}

// AddPlan 添加自定义定价方案.
func (a *Analyzer) AddPlan(plan PricingPlan) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.plans = append(a.plans, plan)
}

// GetPlans 获取所有可用定价方案.
func (a *Analyzer) GetPlans() []PricingPlan {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]PricingPlan, len(a.plans))
	copy(result, a.plans)
	return result
}

// FindPlan 根据ID查找方案.
func (a *Analyzer) FindPlan(planID string) (*PricingPlan, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, p := range a.plans {
		if p.ID == planID {
			return &p, true
		}
	}
	return nil, false
}

// Analyze 执行成本分析.
func (a *Analyzer) Analyze(capacityGB int64, tier StorageTier, replica ReplicaPolicy, workload WorkloadType) (*CostAnalysis, error) {
	if capacityGB <= 0 {
		return nil, fmt.Errorf("capacity must be positive, got %d", capacityGB)
	}

	// 查找匹配的方案
	plan := a.findBestPlan(tier, replica, capacityGB)
	if plan == nil {
		return nil, fmt.Errorf("no matching plan found for tier=%s replica=%s capacity=%dGB", tier, replica, capacityGB)
	}

	// 计算副本开销
	replicaOverhead := GetReplicaOverhead(replica)
	usableRatio := GetUsableRatio(replica)

	// 计算月度成本
	monthlyCost := a.calculateCost(plan, capacityGB, replicaOverhead)

	// 年度成本（月度 * 12）
	annualCost := CostBreakdown{
		StorageCost:    monthlyCost.StorageCost * 12,
		ReplicaCost:    monthlyCost.ReplicaCost * 12,
		TransferCost:   monthlyCost.TransferCost * 12,
		BaseFee:        monthlyCost.BaseFee * 12,
		TotalCost:      monthlyCost.TotalCost * 12,
		EffectivePerGB: monthlyCost.EffectivePerGB,
	}

	// 三年 TCO
	threeYearCost := CostBreakdown{
		StorageCost:    monthlyCost.StorageCost * 36,
		ReplicaCost:    monthlyCost.ReplicaCost * 36,
		TransferCost:   monthlyCost.TransferCost * 36,
		BaseFee:        monthlyCost.BaseFee * 36,
		TotalCost:      monthlyCost.TotalCost * 36,
		EffectivePerGB: monthlyCost.EffectivePerGB,
	}

	analysis := &CostAnalysis{
		TotalCapacityGB:     capacityGB,
		Tier:                tier,
		Replica:             replica,
		Workload:            workload,
		MonthlyCost:         monthlyCost,
		AnnualCost:          annualCost,
		ThreeYearCost:       threeYearCost,
		EffectiveIOPS:       plan.IOPSLimit,
		EffectiveThroughput: plan.ThroughputMB,
		ReadLatencyMs:       plan.ReadLatencyMs,
		WriteLatencyMs:      plan.WriteLatencyMs,
		ReplicaOverhead:     replicaOverhead,
		UsableRatio:         usableRatio,
		AnalyzedAt:          time.Now(),
		PlanUsed:            plan.Name,
	}

	return analysis, nil
}

// calculateCost 计算成本明细.
func (a *Analyzer) calculateCost(plan *PricingPlan, capacityGB int64, replicaOverhead float64) CostBreakdown {
	// 存储费用 = 容量 * 单价
	storageCost := float64(capacityGB) * plan.UnitPrice

	// 副本费用 = 存储费用 * (副本系数 - 1)
	replicaCost := storageCost * (replicaOverhead - 1.0)

	// 基础费用
	baseFee := plan.MonthlyBaseFee

	// 传输费用（预估每月传输容量的 10%）
	estimatedTransferGB := float64(capacityGB) * 0.1
	transferCost := estimatedTransferGB * plan.TransferFee

	totalCost := storageCost + replicaCost + baseFee + transferCost

	// 有效单价
	effectivePerGB := 0.0
	if capacityGB > 0 {
		effectivePerGB = totalCost / float64(capacityGB)
	}

	return CostBreakdown{
		StorageCost:    storageCost,
		ReplicaCost:    replicaCost,
		TransferCost:   transferCost,
		BaseFee:        baseFee,
		TotalCost:      totalCost,
		EffectivePerGB: effectivePerGB,
	}
}

// findBestPlan 查找最佳匹配方案.
func (a *Analyzer) findBestPlan(tier StorageTier, replica ReplicaPolicy, capacityGB int64) *PricingPlan {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var best *PricingPlan
	for i, p := range a.plans {
		// 匹配层级和副本策略
		if p.Tier != tier || p.Replica != replica {
			continue
		}

		// 检查容量范围
		if capacityGB < p.MinCapacity {
			continue
		}
		if p.MaxCapacity > 0 && capacityGB > p.MaxCapacity {
			continue
		}

		// 选择价格最低的方案
		if best == nil || p.UnitPrice < best.UnitPrice {
			best = &a.plans[i]
		}
	}
	return best
}

// GetPlanByWorkload 根据工作负载推荐方案.
func (a *Analyzer) GetPlanByWorkload(workload WorkloadType, capacityGB int64) *PricingPlan {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var tier StorageTier
	switch workload {
	case WorkloadCold:
		tier = TierHDD
	case WorkloadWarm:
		tier = TierHybrid
	case WorkloadHot:
		tier = TierSSD
	case WorkloadMixed:
		tier = TierHybrid
	default:
		tier = TierHybrid
	}

	// 查找匹配的方案
	for i, p := range a.plans {
		if p.Tier == tier && (capacityGB >= p.MinCapacity) {
			if p.MaxCapacity == 0 || capacityGB <= p.MaxCapacity {
				return &a.plans[i]
			}
		}
	}
	return nil
}

// ComparePlans 比较多个方案的成本.
func (a *Analyzer) ComparePlans(capacityGB int64, planIDs []string) ([]*CostAnalysis, error) {
	if capacityGB <= 0 {
		return nil, fmt.Errorf("capacity must be positive")
	}

	var results []*CostAnalysis
	for _, planID := range planIDs {
		plan, ok := a.FindPlan(planID)
		if !ok {
			continue
		}

		replicaOverhead := GetReplicaOverhead(plan.Replica)
		cost := a.calculateCost(plan, capacityGB, replicaOverhead)

		analysis := &CostAnalysis{
			TotalCapacityGB: capacityGB,
			Tier:            plan.Tier,
			Replica:         plan.Replica,
			Workload:        WorkloadMixed,
			MonthlyCost:     cost,
			AnnualCost: CostBreakdown{
				StorageCost:    cost.StorageCost * 12,
				ReplicaCost:    cost.ReplicaCost * 12,
				TransferCost:   cost.TransferCost * 12,
				BaseFee:        cost.BaseFee * 12,
				TotalCost:      cost.TotalCost * 12,
				EffectivePerGB: cost.EffectivePerGB,
			},
			EffectiveIOPS:       plan.IOPSLimit,
			EffectiveThroughput: plan.ThroughputMB,
			ReadLatencyMs:       plan.ReadLatencyMs,
			WriteLatencyMs:      plan.WriteLatencyMs,
			ReplicaOverhead:     replicaOverhead,
			UsableRatio:         GetUsableRatio(plan.Replica),
			AnalyzedAt:          time.Now(),
			PlanUsed:            plan.Name,
		}
		results = append(results, analysis)
	}

	return results, nil
}
