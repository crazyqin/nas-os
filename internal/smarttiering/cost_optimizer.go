package smarttiering

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// CostOptimizer 成本优化器
// 计算最优存储成本，提供迁移建议以降低总成本
type CostOptimizer struct {
	mu        sync.RWMutex
	config    CostOptimizerConfig
	logger    *zap.Logger
	predictor *Predictor
	tierCosts map[StorageTier]TierCostProfile
}

// NewCostOptimizer 创建成本优化器
func NewCostOptimizer(config CostOptimizerConfig, predictor *Predictor, tierCosts []TierCostProfile, logger *zap.Logger) *CostOptimizer {
	if logger == nil {
		logger = zap.NewNop()
	}
	costMap := make(map[StorageTier]TierCostProfile)
	for _, tc := range tierCosts {
		costMap[tc.Tier] = tc
	}
	return &CostOptimizer{
		config:    config,
		logger:    logger,
		predictor: predictor,
		tierCosts: costMap,
	}
}

// GenerateReport 生成成本报告
func (co *CostOptimizer) GenerateReport(ctx context.Context) (*CostReport, error) {
	co.mu.RLock()
	defer co.mu.RUnlock()

	files := co.predictor.GetAllFiles()
	if len(files) == 0 {
		return &CostReport{
			TierCosts:   make(map[StorageTier]float64),
			TierSizesGB: make(map[StorageTier]float64),
			GeneratedAt: time.Now(),
		}, nil
	}

	report := &CostReport{
		TierCosts:       make(map[StorageTier]float64),
		TierSizesGB:     make(map[StorageTier]float64),
		Recommendations: make([]CostRecommendation, 0),
		GeneratedAt:     time.Now(),
	}

	// 计算当前成本
	for _, meta := range files {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		sizeGB := float64(meta.Size) / (1024 * 1024 * 1024)
		report.TierSizesGB[meta.CurrentTier] += sizeGB

		costProfile, ok := co.tierCosts[meta.CurrentTier]
		if ok {
			cost := sizeGB * costProfile.CostPerGBMonth
			report.TierCosts[meta.CurrentTier] += cost
			report.TotalCostPerMonth += cost
		}
	}

	// 计算最优成本和建议
	optimalCost := 0.0
	for _, meta := range files {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		sizeGB := float64(meta.Size) / (1024 * 1024 * 1024)
		currentCostProfile, ok := co.tierCosts[meta.CurrentTier]
		if !ok {
			continue
		}

		// 根据热度推荐最优层级
		bestTier := co.recommendTier(meta)
		bestCostProfile, ok := co.tierCosts[bestTier]
		if !ok {
			continue
		}

		currentCost := sizeGB * currentCostProfile.CostPerGBMonth
		recommendedCost := sizeGB * bestCostProfile.CostPerGBMonth
		optimalCost += recommendedCost

		// 如果推荐层级不同，添加建议
		if bestTier != meta.CurrentTier {
			savings := currentCost - recommendedCost
			if savings > 0.01 { // 节省超过0.01元才建议
				reason := co.explainRecommendation(meta, bestTier)
				report.Recommendations = append(report.Recommendations, CostRecommendation{
					FilePath:        meta.Path,
					CurrentTier:     meta.CurrentTier,
					RecommendedTier: bestTier,
					CurrentCost:     math.Round(currentCost*100) / 100,
					RecommendedCost: math.Round(recommendedCost*100) / 100,
					Savings:         math.Round(savings*100) / 100,
					Reason:          reason,
				})
			}
		}
	}

	report.OptimalCostPerMonth = math.Round(optimalCost*100) / 100
	if report.TotalCostPerMonth > 0 {
		report.SavingsPercent = math.Round((1-optimalCost/report.TotalCostPerMonth)*10000) / 100
	}

	co.logger.Info("cost report generated",
		zap.Float64("current_cost", report.TotalCostPerMonth),
		zap.Float64("optimal_cost", report.OptimalCostPerMonth),
		zap.Int("recommendations", len(report.Recommendations)))

	return report, nil
}

// recommendTier 基于热度和成本推荐最优层级
func (co *CostOptimizer) recommendTier(meta *FileMetadata) StorageTier {
	// 基于热度评分选择层级
	switch {
	case meta.HeatScore >= 70:
		return TierHot
	case meta.HeatScore >= 40:
		return TierWarm
	case meta.HeatScore >= 15:
		return TierCold
	default:
		return TierArchive
	}
}

// explainRecommendation 解释推荐原因
func (co *CostOptimizer) explainRecommendation(meta *FileMetadata, targetTier StorageTier) string {
	currentCost := co.tierCosts[meta.CurrentTier]
	targetCost := co.tierCosts[targetTier]

	if targetTier > meta.CurrentTier {
		// 降级：数据冷了
		return fmt.Sprintf("heat score %.1f indicates cold data, downgrade from %s ($%.2f/GB/mo) to %s ($%.2f/GB/mo)",
			meta.HeatScore, meta.CurrentTier, currentCost.CostPerGBMonth, targetTier, targetCost.CostPerGBMonth)
	}
	// 升级：数据热了
	return fmt.Sprintf("heat score %.1f indicates hot data, upgrade from %s ($%.2f/GB/mo) to %s ($%.2f/GB/mo)",
		meta.HeatScore, meta.CurrentTier, currentCost.CostPerGBMonth, targetTier, targetCost.CostPerGBMonth)
}

// GetBudgetStatus 获取预算状态
func (co *CostOptimizer) GetBudgetStatus(ctx context.Context) (used float64, budget float64, remaining float64, err error) {
	report, err := co.GenerateReport(ctx)
	if err != nil {
		return 0, 0, 0, err
	}
	budget = co.config.BudgetPerMonth
	used = report.TotalCostPerMonth
	remaining = budget - used
	return
}

// EstimateMigrationCost 估算迁移成本
func (co *CostOptimizer) EstimateMigrationCost(ctx context.Context, migrations []CostRecommendation) (float64, error) {
	totalCost := 0.0
	for _, _ = range migrations {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		// 迁移本身的成本 = 数据量 * IO成本
		// 这里简化为固定成本
		totalCost += 0.01 // 每次迁移0.01元的IO成本
	}
	return totalCost, nil
}

// GetTierCosts 获取层级成本配置
func (co *CostOptimizer) GetTierCosts() []TierCostProfile {
	co.mu.RLock()
	defer co.mu.RUnlock()
	result := make([]TierCostProfile, 0, len(co.tierCosts))
	for _, v := range co.tierCosts {
		result = append(result, v)
	}
	return result
}

// UpdateConfig 更新配置
func (co *CostOptimizer) UpdateConfig(config CostOptimizerConfig) {
	co.mu.Lock()
	defer co.mu.Unlock()
	co.config = config
}

// GetConfig 获取配置
func (co *CostOptimizer) GetConfig() CostOptimizerConfig {
	co.mu.RLock()
	defer co.mu.RUnlock()
	return co.config
}
