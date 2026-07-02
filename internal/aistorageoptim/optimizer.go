package aistorageoptim

import (
	"math"
	"time"
)

// Optimizer 存储优化器.
type Optimizer struct {
	policy TieringPolicy
}

// NewOptimizer 创建优化器.
func NewOptimizer(policy TieringPolicy) *Optimizer {
	return &Optimizer{
		policy: policy,
	}
}

// CalculateScore 计算文件优化评分.
func (o *Optimizer) CalculateScore(stats *FileAccessStats, now time.Time) OptimizationScore {
	score := OptimizationScore{
		FilePath:    stats.FilePath,
		CurrentTier: stats.CurrentTier,
	}

	// 计算各维度分数
	score.AccessFrequencyScore = o.calculateAccessFrequencyScore(stats)
	score.FileSizeScore = o.calculateFileSizeScore(stats)
	score.IOPatternScore = o.calculateIOPatternScore(stats)
	score.TimeDecayScore = o.calculateTimeDecayScore(stats, now)

	// 加权总分
	score.Score = score.AccessFrequencyScore*o.policy.AccessFrequencyWeight +
		score.FileSizeScore*o.policy.FileSizeWeight +
		score.IOPatternScore*o.policy.IOPatternWeight +
		score.TimeDecayScore*o.policy.TimeDecayWeight

	// 推荐层级
	score.RecommendedTier = o.recommendTier(score.Score)

	// 计算优先级 (1-10)
	score.Priority = o.calculatePriority(score.Score, stats.CurrentTier, score.RecommendedTier)

	// 生成原因
	score.Reason = o.generateReason(score, stats)

	return score
}

// calculateAccessFrequencyScore 计算访问频率分数 (0-100).
func (o *Optimizer) calculateAccessFrequencyScore(stats *FileAccessStats) float64 {
	// 使用对数函数平滑高频访问
	if stats.AccessFrequency <= 0 {
		return 0
	}
	// 每小时100次访问 = 100分
	score := math.Log10(stats.AccessFrequency+1) * 50
	return math.Min(100, score)
}

// calculateFileSizeScore 计算文件大小分数 (0-100)
// 小文件更适合高速存储，大文件适合大容量存储.
func (o *Optimizer) calculateFileSizeScore(stats *FileAccessStats) float64 {
	if stats.FileSize <= 0 {
		return 50 // 默认中等分数
	}

	// 小文件得分高（适合NVMe/SSD）
	if stats.FileSize <= o.policy.SmallFileThreshold {
		return 90
	}

	// 大文件得分低（适合HDD）
	if stats.FileSize >= o.policy.LargeFileThreshold {
		return 20
	}

	// 中等文件，线性插值
	ratio := float64(stats.FileSize-o.policy.SmallFileThreshold) /
		float64(o.policy.LargeFileThreshold-o.policy.SmallFileThreshold)
	return 90 - ratio*70
}

// calculateIOPatternScore 计算IO模式分数 (0-100).
func (o *Optimizer) calculateIOPatternScore(stats *FileAccessStats) float64 {
	switch stats.IOPattern {
	case IOPatternRandom:
		return 95 // 随机IO最受益于高速存储
	case IOPatternBurst:
		return 85 // 突发IO需要快速响应
	case IOPatternSequential:
		return 50 // 顺序IO对存储类型不太敏感
	case IOPatternStreaming:
		return 40 // 流式IO更依赖带宽
	default:
		return 60
	}
}

// calculateTimeDecayScore 计算时间衰减分数 (0-100)
// 最近访问的数据得分高.
func (o *Optimizer) calculateTimeDecayScore(stats *FileAccessStats, now time.Time) float64 {
	if stats.LastAccessTime.IsZero() {
		return 0
	}

	hoursSinceAccess := now.Sub(stats.LastAccessTime).Hours()

	// 指数衰减：24小时内=100，7天后=50，30天后=10
	decay := math.Exp(-hoursSinceAccess / 168) // 168小时 = 7天半衰期
	return decay * 100
}

// recommendTier 根据分数推荐存储层级.
func (o *Optimizer) recommendTier(score float64) StorageTier {
	if score >= o.policy.NVMePromoteThreshold {
		return TierNVMe
	}
	if score >= o.policy.SSDPromoteThreshold {
		return TierSSD
	}
	return TierHDD
}

// calculatePriority 计算迁移优先级.
func (o *Optimizer) calculatePriority(score float64, currentTier, recommendedTier StorageTier) int {
	if currentTier == recommendedTier {
		return 1 // 无需迁移
	}

	// 计算层级差距
	tierOrder := map[StorageTier]int{TierHDD: 0, TierSSD: 1, TierNVMe: 2}
	currentOrder := tierOrder[currentTier]
	recommendedOrder := tierOrder[recommendedTier]
	gap := int(math.Abs(float64(currentOrder - recommendedOrder)))

	// 分数越极端，优先级越高
	scoreFactor := 0.0
	if score > 80 || score < 20 {
		scoreFactor = 3
	} else if score > 60 || score < 40 {
		scoreFactor = 2
	} else {
		scoreFactor = 1
	}

	priority := int(scoreFactor) + gap*2
	if priority > 10 {
		priority = 10
	}
	if priority < 1 {
		priority = 1
	}
	return priority
}

// generateReason 生成优化原因.
func (o *Optimizer) generateReason(score OptimizationScore, stats *FileAccessStats) string {
	reason := ""

	if score.AccessFrequencyScore > 70 {
		reason += "高频访问; "
	}
	if score.FileSizeScore > 70 {
		reason += "小文件适合高速存储; "
	}
	if score.IOPatternScore > 70 {
		reason += string(stats.IOPattern) + "模式受益于高速存储; "
	}
	if score.TimeDecayScore > 70 {
		reason += "最近活跃; "
	}

	if reason == "" {
		if score.Score < 30 {
			reason = "低活跃度，适合低成本存储"
		} else {
			reason = "当前层级适合"
		}
	}

	return reason
}

// MakeDecision 生成优化决策.
func (o *Optimizer) MakeDecision(score OptimizationScore) OptimizationDecision {
	decision := OptimizationDecision{
		FilePath: score.FilePath,
		FromTier: score.CurrentTier,
		Score:    score.Score,
		Priority: score.Priority,
		Reason:   score.Reason,
	}

	if score.CurrentTier == score.RecommendedTier {
		decision.Action = "keep"
		decision.ToTier = score.CurrentTier
		decision.EstimatedBenefit = 0
	} else {
		tierOrder := map[StorageTier]int{TierHDD: 0, TierSSD: 1, TierNVMe: 2}
		if tierOrder[score.RecommendedTier] > tierOrder[score.CurrentTier] {
			decision.Action = "promote"
		} else {
			decision.Action = "demote"
		}
		decision.ToTier = score.RecommendedTier
		decision.EstimatedBenefit = o.estimateBenefit(score)
	}

	return decision
}

// estimateBenefit 估算性能提升.
func (o *Optimizer) estimateBenefit(score OptimizationScore) float64 {
	// 基于分数差估算性能提升
	scoreDiff := score.Score - o.getTierBaseScore(score.CurrentTier)
	if scoreDiff <= 0 {
		return 0
	}
	return math.Min(100, scoreDiff*1.5)
}

// getTierBaseScore 获取层级基础分数.
func (o *Optimizer) getTierBaseScore(tier StorageTier) float64 {
	switch tier {
	case TierNVMe:
		return o.policy.NVMePromoteThreshold
	case TierSSD:
		return o.policy.SSDPromoteThreshold
	case TierHDD:
		return o.policy.HDDDemoteThreshold
	default:
		return 50
	}
}

// BatchOptimize 批量优化分析.
func (o *Optimizer) BatchOptimize(statsList []*FileAccessStats, now time.Time) ([]OptimizationDecision, OptimizationStats) {
	var decisions []OptimizationDecision
	var totalScore float64
	optStats := OptimizationStats{
		TotalFiles: int64(len(statsList)),
	}

	for _, stats := range statsList {
		score := o.CalculateScore(stats, now)
		decision := o.MakeDecision(score)

		if decision.Action != "keep" {
			decisions = append(decisions, decision)
			optStats.TotalDecisions++

			switch decision.Action {
			case "promote":
				optStats.PromoteCount++
			case "demote":
				optStats.DemoteCount++
			}
		} else {
			optStats.KeepCount++
		}

		totalScore += score.Score
	}

	if len(statsList) > 0 {
		optStats.AvgScore = totalScore / float64(len(statsList))
	}
	optStats.LastAnalysisTime = now.Format(time.RFC3339)

	// 按优先级排序
	sortDecisions(decisions)

	return decisions, optStats
}

// sortDecisions 按优先级排序（高优先级在前）.
func sortDecisions(decisions []OptimizationDecision) {
	for i := 1; i < len(decisions); i++ {
		for j := i; j > 0 && decisions[j].Priority > decisions[j-1].Priority; j-- {
			decisions[j], decisions[j-1] = decisions[j-1], decisions[j]
		}
	}
}
