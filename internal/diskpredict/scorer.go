package diskpredict

import (
	"math"
	"time"
)

// Scorer 健康评分计算器
type Scorer struct {
	// 各维度权重
	attributeWeight   float64 // SMART属性权重
	temperatureWeight float64 // 温度权重
	powerOnWeight     float64 // 通电时间权重
	overallWeight     float64 // 综合评估权重
}

// NewScorer 创建评分器
func NewScorer() *Scorer {
	return &Scorer{
		attributeWeight:   0.5, // SMART属性占50%
		temperatureWeight: 0.2, // 温度占20%
		powerOnWeight:     0.2, // 通电时间占20%
		overallWeight:     0.1, // 综合评估占10%
	}
}

// CalculateHealthScore 计算综合健康评分（0-100）
func (s *Scorer) CalculateHealthScore(
	attributeAnalyses []AttributeAnalysis,
	temperature int,
	powerOnHours uint64,
) float64 {
	// 1. 计算SMART属性得分
	attrScore := s.calculateAttributeScore(attributeAnalyses)

	// 2. 计算温度得分
	tempScore, _ := NewAnalyzer().AnalyzeTemperature(temperature)

	// 3. 计算通电时间得分
	powerScore, _ := NewAnalyzer().AnalyzePowerOnHours(powerOnHours)

	// 4. 计算综合得分（加权平均）
	overallScore := attrScore*s.attributeWeight +
		tempScore*s.temperatureWeight +
		powerScore*s.powerOnWeight +
		(attrScore+tempScore+powerScore)/3.0*s.overallWeight

	// 确保在0-100范围内
	return math.Max(0, math.Min(100, overallScore))
}

// calculateAttributeScore 计算所有SMART属性的综合得分
func (s *Scorer) calculateAttributeScore(analyses []AttributeAnalysis) float64 {
	if len(analyses) == 0 {
		return 50.0 // 没有数据时返回中等分数
	}

	totalWeight := 0.0
	weightedSum := 0.0

	for _, analysis := range analyses {
		totalWeight += analysis.Weight
		weightedSum += analysis.WeightedScore
	}

	if totalWeight == 0 {
		return 50.0
	}

	return weightedSum / totalWeight
}

// DetermineStatus 根据健康评分确定状态
func (s *Scorer) DetermineStatus(score float64) DiskStatus {
	switch {
	case score >= 80:
		return StatusHealthy
	case score >= 60:
		return StatusWarning
	case score >= 40:
		return StatusCritical
	default:
		return StatusFailed
	}
}

// EstimateRemainingLifeDays 估算剩余寿命（天）
func (s *Scorer) EstimateRemainingLifeDays(
	score float64,
	status DiskStatus,
	riskFactors []string,
) int {
	// 基础寿命估算
	baseDays := s.baseEstimateFromScore(score)

	// 根据风险因素调整
	adjustment := s.adjustmentFromRiskFactors(riskFactors)

	// 确保至少有1天
	result := int(float64(baseDays) * adjustment)
	if result < 1 {
		result = 1
	}

	return result
}

// baseEstimateFromScore 基于评分的基础寿命估算
func (s *Scorer) baseEstimateFromScore(score float64) int {
	switch {
	case score >= 90:
		return 1095 // 3年
	case score >= 80:
		return 730 // 2年
	case score >= 70:
		return 548 // 1.5年
	case score >= 60:
		return 365 // 1年
	case score >= 50:
		return 274 // 9个月
	case score >= 40:
		return 182 // 6个月
	case score >= 30:
		return 120 // 4个月
	case score >= 20:
		return 90 // 3个月
	case score >= 10:
		return 60 // 2个月
	default:
		return 30 // 1个月
	}
}

// adjustmentFromRiskFactors 根据风险因素调整系数
func (s *Scorer) adjustmentFromRiskFactors(riskFactors []string) float64 {
	if len(riskFactors) == 0 {
		return 1.0 // 没有风险因素，不调整
	}

	// 每个风险因素降低10%寿命，最多降低50%
	adjustment := 1.0 - float64(len(riskFactors))*0.1
	if adjustment < 0.5 {
		adjustment = 0.5
	}

	return adjustment
}

// CalculateRiskLevel 计算风险等级
func (s *Scorer) CalculateRiskLevel(score float64, riskFactorCount int) string {
	// 综合考虑评分和风险因素数量
	riskScore := score - float64(riskFactorCount)*5

	switch {
	case riskScore >= 80:
		return "low"
	case riskScore >= 60:
		return "medium"
	case riskScore >= 40:
		return "high"
	default:
		return "critical"
	}
}

// EstimateFailureDate 估算故障日期
func (s *Scorer) EstimateFailureDate(lifeDays int) *time.Time {
	if lifeDays <= 0 {
		return nil
	}

	failDate := time.Now().AddDate(0, 0, lifeDays)
	return &failDate
}

// ScoreToGrade 评分转等级
func (s *Scorer) ScoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A" // 优秀
	case score >= 80:
		return "B" // 良好
	case score >= 70:
		return "C" // 中等
	case score >= 60:
		return "D" // 较差
	default:
		return "F" // 差
	}
}

// ScoreToEmoji 评分转Emoji
func (s *Scorer) ScoreToEmoji(score float64) string {
	switch {
	case score >= 90:
		return "😊" // 优秀
	case score >= 80:
		return "🙂" // 良好
	case score >= 70:
		return "😐" // 中等
	case score >= 60:
		return "😟" // 较差
	case score >= 40:
		return "😨" // 差
	default:
		return "💀" // 危险
	}
}

// ScoreToDescription 评分转描述
func (s *Scorer) ScoreToDescription(score float64) string {
	switch {
	case score >= 90:
		return "磁盘状态优秀，运行良好"
	case score >= 80:
		return "磁盘状态良好，可继续使用"
	case score >= 70:
		return "磁盘状态中等，建议关注"
	case score >= 60:
		return "磁盘状态较差，建议备份数据"
	case score >= 40:
		return "磁盘状态差，建议尽快更换"
	default:
		return "磁盘状态极差，立即更换！"
	}
}
