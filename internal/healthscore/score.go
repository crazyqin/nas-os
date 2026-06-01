package healthscore

import (
	"fmt"
)

// ScoreCalculator calculates the overall health score
type ScoreCalculator struct {
	hs *HealthScore
}

// NewScoreCalculator creates a new score calculator
func NewScoreCalculator(hs *HealthScore) *ScoreCalculator {
	return &ScoreCalculator{hs: hs}
}

// CalculateOverallScore calculates the overall health score from components
func (sc *ScoreCalculator) CalculateOverallScore(components []ComponentScore) float64 {
	sc.hs.mu.RLock()
	defer sc.hs.mu.RUnlock()

	totalWeight := 0.0
	weightedSum := 0.0

	for _, comp := range components {
		weight := sc.getWeight(comp.Type)
		weightedSum += comp.Score * weight
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0
	}

	return weightedSum / totalWeight
}

// DetermineStatus determines the health status from score
func (sc *ScoreCalculator) DetermineStatus(score float64) HealthStatus {
	switch {
	case score >= 90:
		return StatusExcellent
	case score >= 75:
		return StatusGood
	case score >= 60:
		return StatusFair
	case score >= 40:
		return StatusPoor
	default:
		return StatusCritical
	}
}

// DetermineTrend determines the trend from historical data
func (sc *ScoreCalculator) DetermineTrend(history []ScoreHistory) string {
	if len(history) < 3 {
		return "stable"
	}

	// Compare recent average with older average
	recent := history[len(history)-3:]
	older := history[len(history)-6 : len(history)-3]

	if len(older) == 0 {
		return "stable"
	}

	recentAvg := averageScores(recent)
	olderAvg := averageScores(older)

	diff := recentAvg - olderAvg
	if diff > 5 {
		return "improving"
	} else if diff < -5 {
		return "declining"
	}
	return "stable"
}

// GenerateRecommendations generates recommendations based on component scores
func (sc *ScoreCalculator) GenerateRecommendations(components []ComponentScore) []Recommendation {
	var recommendations []Recommendation

	for _, comp := range components {
		if comp.Score < 60 {
			rec := sc.generateComponentRecommendation(comp)
			recommendations = append(recommendations, rec)
		}
	}

	return recommendations
}

// getWeight returns the weight for a component type
func (sc *ScoreCalculator) getWeight(compType ComponentType) float64 {
	if weight, exists := sc.hs.weights[compType]; exists {
		return weight
	}
	if weight, exists := DefaultWeights[compType]; exists {
		return weight
	}
	return 0.1 // Default weight
}

// generateComponentRecommendation generates a recommendation for a component
func (sc *ScoreCalculator) generateComponentRecommendation(comp ComponentScore) Recommendation {
	priority := "low"
	if comp.Score < 40 {
		priority = "critical"
	} else if comp.Score < 50 {
		priority = "high"
	} else if comp.Score < 60 {
		priority = "medium"
	}

	title, description, action := getRecommendationDetails(comp.Type, comp.Score)

	return Recommendation{
		Priority:    priority,
		Component:   string(comp.Type),
		Title:       title,
		Description: description,
		Action:      action,
	}
}

// getRecommendationDetails returns recommendation details for a component
func getRecommendationDetails(compType ComponentType, score float64) (string, string, string) {
	switch compType {
	case ComponentDisk:
		return "磁盘空间不足",
			fmt.Sprintf("磁盘健康评分为 %.0f，低于建议阈值", score),
			"清理无用文件或扩展存储容量"
	case ComponentCPU:
		return "CPU 使用率过高",
			fmt.Sprintf("CPU 健康评分为 %.0f，系统负载较重", score),
			"检查高 CPU 进程或考虑升级硬件"
	case ComponentMemory:
		return "内存使用率过高",
			fmt.Sprintf("内存健康评分为 %.0f，可用内存不足", score),
			"关闭不必要的服务或增加内存"
	case ComponentNetwork:
		return "网络性能下降",
			fmt.Sprintf("网络健康评分为 %.0f，连接质量不佳", score),
			"检查网络连接和带宽使用情况"
	case ComponentRAID:
		return "RAID 状态异常",
			fmt.Sprintf("RAID 健康评分为 %.0f，可能存在数据风险", score),
			"立即检查 RAID 状态并修复问题"
	case ComponentService:
		return "服务状态异常",
			fmt.Sprintf("服务健康评分为 %.0f，部分服务未正常运行", score),
			"检查并重启异常服务"
	case ComponentTemperature:
		return "温度过高",
			fmt.Sprintf("温度健康评分为 %.0f，设备温度偏高", score),
			"检查散热系统或降低负载"
	default:
		return "需要关注",
			fmt.Sprintf("组件健康评分为 %.0f", score),
			"请检查相关组件状态"
	}
}

// averageScores calculates the average score from history
func averageScores(history []ScoreHistory) float64 {
	if len(history) == 0 {
		return 0
	}
	sum := 0.0
	for _, h := range history {
		sum += h.Score
	}
	return sum / float64(len(history))
}
