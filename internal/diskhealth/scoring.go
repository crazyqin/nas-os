// Package diskhealthai2 - 健康评分系统、故障预测器、维护建议引擎、磁盘组管理
package diskhealth

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ============================================================
// HealthScoreSystem - 健康评分系统
// ============================================================

// HealthScoreSystem 健康评分系统
type HealthScoreSystem struct {
	mu           sync.RWMutex
	analyzer     *SMARTAnalyzer
	scoreHistory map[string][]float64
	weights      map[SMARTAttributeID]float64
}

// NewHealthScoreSystem 创建健康评分系统
func NewHealthScoreSystem(analyzer *SMARTAnalyzer) *HealthScoreSystem {
	weights := map[SMARTAttributeID]float64{
		SMARTIDReallocatedSectorCt:  0.15,
		SMARTIDCurrentPendingSector: 0.12,
		SMARTIDOfflineUncorrectable: 0.12,
		SMARTIDTemperatureCelsius:   0.08,
		SMARTIDPowerOnHours:         0.07,
		SMARTIDWearLevelingCount:    0.10,
		SMARTIDSSDLifeLeft:          0.10,
		SMARTIDSeekErrorRate:        0.05,
		SMARTIDSpinRetryCount:       0.04,
		SMARTIDUDMAErrorCount:       0.04,
		SMARTIDNANDWrites:           0.03,
		SMARTIDUnsafeShutdownCount:  0.03,
		SMARTIDLoadUnloadCycleCount: 0.02,
		SMARTIDMultiZoneErrorRate:   0.02,
		SMARTIDGSENSEErrorRate:      0.02,
		SMARTIDHardwareECCRecovered: 0.01,
	}
	return &HealthScoreSystem{
		analyzer:     analyzer,
		scoreHistory: make(map[string][]float64),
		weights:      weights,
	}
}

// Calculate 计算设备健康评分
func (h *HealthScoreSystem) Calculate(device string) (*HealthScore, error) {
	data, err := h.analyzer.GetLatestData(device)
	if err != nil {
		return nil, err
	}

	score := &HealthScore{
		Device:       device,
		CalculatedAt: time.Now(),
	}

	h.mu.RLock()
	previousScores := h.scoreHistory[device]
	if len(previousScores) > 0 {
		prev := previousScores[len(previousScores)-1]
		score.PreviousScore = &prev
	}
	h.mu.RUnlock()

	var totalWeightedScore, totalWeight float64

	for attrID, weight := range h.weights {
		attrValue := getAttributeValue(data, attrID)
		attrScore := h.scoreAttribute(attrID, attrValue, data)
		weightedScore := attrScore * weight
		totalWeightedScore += weightedScore
		totalWeight += weight

		status := "normal"
		if attrScore < 50 {
			status = "critical"
		} else if attrScore < 70 {
			status = "warning"
		}

		score.AttributeScores = append(score.AttributeScores, AttributeScore{
			AttributeID:   attrID,
			AttributeName: GetAttributeName(attrID),
			Score:         attrScore,
			Weight:        weight,
			WeightedScore: weightedScore,
			Status:        status,
		})
	}

	correlationPenalties := h.calculateCorrelationPenalties(data)
	score.CorrelationPenalty = correlationPenalties

	if totalWeight > 0 {
		score.Score = totalWeightedScore / totalWeight
	}

	for _, penalty := range correlationPenalties {
		score.Score -= penalty.Penalty
	}

	score.Score = math.Max(0, math.Min(100, score.Score))
	score.Grade = scoreToGrade(score.Score)
	score.Status = scoreToStatus(score.Score)

	if score.PreviousScore != nil {
		score.ScoreDelta = score.Score - *score.PreviousScore
		if score.ScoreDelta > 2 {
			score.Trend = TrendImproving
		} else if score.ScoreDelta < -2 {
			score.Trend = TrendDeclining
		} else {
			score.Trend = TrendStable
		}
	} else {
		score.Trend = TrendStable
	}

	h.mu.Lock()
	h.scoreHistory[device] = append(h.scoreHistory[device], score.Score)
	if len(h.scoreHistory[device]) > 365 {
		h.scoreHistory[device] = h.scoreHistory[device][len(h.scoreHistory[device])-365:]
	}
	h.mu.Unlock()

	sort.Slice(score.AttributeScores, func(i, j int) bool {
		return score.AttributeScores[i].Score < score.AttributeScores[j].Score
	})

	return score, nil
}

// GetHistory 获取评分历史
func (h *HealthScoreSystem) GetHistory(device string) []float64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.scoreHistory[device]
}

// scoreAttribute 为单个属性评分
func (h *HealthScoreSystem) scoreAttribute(attrID SMARTAttributeID, value uint64, data *SMARTData) float64 {
	switch attrID {
	case SMARTIDReallocatedSectorCt:
		if value == 0 {
			return 100
		}
		return math.Max(0, 100-float64(value)*10)

	case SMARTIDCurrentPendingSector, SMARTIDOfflineUncorrectable:
		if value == 0 {
			return 100
		}
		return math.Max(0, 100-float64(value)*15)

	case SMARTIDTemperatureCelsius:
		temp := float64(value)
		if temp >= 25 && temp <= 45 {
			return 100
		} else if temp < 25 {
			return 80 + temp*0.8
		} else if temp <= 55 {
			return 100 - (temp-45)*5
		}
		return math.Max(0, 50-(temp-55)*5)

	case SMARTIDPowerOnHours:
		hours := float64(value)
		expectedLife := 40000.0
		if data.IsSSD {
			expectedLife = 35000.0
		}
		return math.Max(0, 100*(1-hours/expectedLife))

	case SMARTIDWearLevelingCount, SMARTIDSSDLifeLeft:
		return math.Min(100, float64(value))

	case SMARTIDSeekErrorRate:
		if value == 0 {
			return 100
		}
		return math.Max(0, 100-float64(value)*20)

	case SMARTIDSpinRetryCount:
		if value == 0 {
			return 100
		}
		return math.Max(0, 100-float64(value)*25)

	case SMARTIDUDMAErrorCount:
		if value == 0 {
			return 100
		}
		return math.Max(0, 100-float64(value)*15)

	case SMARTIDNANDWrites:
		tbw := float64(value) / 1e12
		expectedTBW := 600.0
		return math.Max(0, 100*(1-tbw/expectedTBW))

	case SMARTIDUnsafeShutdownCount:
		if value == 0 {
			return 100
		}
		return math.Max(0, 100-float64(value)*5)

	case SMARTIDLoadUnloadCycleCount:
		if value == 0 {
			return 100
		}
		return math.Max(0, 100-float64(value)*0.01)

	default:
		return 80
	}
}

// getAttributeValue 从 SMART 数据中获取属性值
func getAttributeValue(data *SMARTData, attrID SMARTAttributeID) uint64 {
	for _, attr := range data.Attributes {
		if attr.ID == attrID {
			return attr.RawValue
		}
	}
	return 0
}

// calculateCorrelationPenalties 计算属性关联惩罚
func (h *HealthScoreSystem) calculateCorrelationPenalties(data *SMARTData) []CorrelationPenalty {
	var penalties []CorrelationPenalty

	reallocated := getAttributeValue(data, SMARTIDReallocatedSectorCt)
	pending := getAttributeValue(data, SMARTIDCurrentPendingSector)
	if reallocated > 0 && pending > 0 {
		penalty := math.Min(10, float64(reallocated+pending)*2)
		penalties = append(penalties, CorrelationPenalty{
			Attribute1ID: SMARTIDReallocatedSectorCt, Attribute1Name: "Reallocated_Sector_Ct",
			Attribute2ID: SMARTIDCurrentPendingSector, Attribute2Name: "Current_Pending_Sector",
			Penalty: penalty, Reason: "坏扇区与待映射扇区同时存在，表明磁盘介质严重退化",
		})
	}

	temp := getAttributeValue(data, SMARTIDTemperatureCelsius)
	poh := getAttributeValue(data, SMARTIDPowerOnHours)
	if temp > 50 && poh < 1000 {
		penalties = append(penalties, CorrelationPenalty{
			Attribute1ID: SMARTIDTemperatureCelsius, Attribute1Name: "Temperature_Celsius",
			Attribute2ID: SMARTIDPowerOnHours, Attribute2Name: "Power_On_Hours",
			Penalty: 5, Reason: "新盘高温，可能存在散热问题",
		})
	}

	wear := getAttributeValue(data, SMARTIDWearLevelingCount)
	nand := getAttributeValue(data, SMARTIDNANDWrites)
	if wear < 30 && nand > 500 {
		penalty := math.Min(8, float64(30-wear)*0.3)
		penalties = append(penalties, CorrelationPenalty{
			Attribute1ID: SMARTIDWearLevelingCount, Attribute1Name: "Wear_Leveling_Count",
			Attribute2ID: SMARTIDNANDWrites, Attribute2Name: "NAND_Writes",
			Penalty: penalty, Reason: "SSD 磨损严重且写入量大，需考虑更换",
		})
	}

	seekErr := getAttributeValue(data, SMARTIDSeekErrorRate)
	udmaErr := getAttributeValue(data, SMARTIDUDMAErrorCount)
	if seekErr > 10 && udmaErr > 5 {
		penalties = append(penalties, CorrelationPenalty{
			Attribute1ID: SMARTIDSeekErrorRate, Attribute1Name: "Seek_Error_Rate",
			Attribute2ID: SMARTIDUDMAErrorCount, Attribute2Name: "UDMA_Error_Count",
			Penalty: 7, Reason: "寻道错误和 UDMA 错误同时升高，可能磁头或控制器故障",
		})
	}

	return penalties
}

// scoreToGrade 评分转等级
func scoreToGrade(score float64) HealthGrade {
	switch {
	case score >= 90:
		return GradeA
	case score >= 70:
		return GradeB
	case score >= 50:
		return GradeC
	case score >= 30:
		return GradeD
	default:
		return GradeF
	}
}

// scoreToStatus 评分转状态
func scoreToStatus(score float64) DiskStatus {
	switch {
	case score >= 70:
		return StatusHealthy
	case score >= 50:
		return StatusWarning
	case score >= 30:
		return StatusCritical
	default:
		return StatusFailed
	}
}

// ============================================================
// FailurePredictor - 贝叶斯故障预测器
// ============================================================

// FailurePredictor 贝叶斯故障预测器
type FailurePredictor struct {
	mu               sync.RWMutex
	analyzer         *SMARTAnalyzer
	scoreSys         *HealthScoreSystem
	priorFailureRate float64
}

// NewFailurePredictor 创建故障预测器
func NewFailurePredictor(analyzer *SMARTAnalyzer, scoreSys *HealthScoreSystem) *FailurePredictor {
	return &FailurePredictor{
		analyzer:         analyzer,
		scoreSys:         scoreSys,
		priorFailureRate: 0.02,
	}
}

// Predict 对设备进行贝叶斯故障预测
func (f *FailurePredictor) Predict(device string) (*BayesianPrediction, error) {
	healthScore, err := f.scoreSys.Calculate(device)
	if err != nil {
		return nil, fmt.Errorf("计算健康评分失败: %w", err)
	}

	analysis, err := f.analyzer.Analyze(device)
	if err != nil {
		return nil, fmt.Errorf("SMART 分析失败: %w", err)
	}

	data, err := f.analyzer.GetLatestData(device)
	if err != nil {
		return nil, err
	}

	prediction := &BayesianPrediction{
		Device:      device,
		PredictedAt: time.Now(),
	}

	// 贝叶斯故障预测
	prior := f.calculatePrior(data)
	prediction.PriorProbability = prior

	likelihood := f.calculateLikelihood(healthScore, analysis)
	prediction.Likelihood = likelihood

	// P(Failure|Evidence) = P(Evidence|Failure) * P(Failure) / P(Evidence)
	posterior := likelihood * prior
	normalizer := posterior + (1-likelihood)*(1-prior)
	if normalizer > 0 {
		posterior = posterior / normalizer
	}
	prediction.PosteriorProbability = posterior
	prediction.FailureProbability = posterior

	prediction.EstimatedLifeDays = f.estimateRemainingLife(data, healthScore.Score)
	prediction.Confidence = f.calculateConfidence(analysis)

	if prediction.EstimatedLifeDays > 0 {
		failDate := time.Now().AddDate(0, 0, prediction.EstimatedLifeDays)
		prediction.FailDateEstimate = &failDate
	}

	prediction.RiskFactors = f.identifyRiskFactors(data, healthScore)

	return prediction, nil
}

// calculatePrior 计算先验概率（浴盆曲线模型）
func (f *FailurePredictor) calculatePrior(data *SMARTData) float64 {
	ageYears := float64(data.PowerOnHours) / 8760.0

	switch {
	case ageYears < 1:
		return 0.03 - 0.02*ageYears
	case ageYears < 3:
		return 0.01
	case ageYears < 5:
		return 0.01 + 0.02*(ageYears-3)
	default:
		return math.Min(0.20, 0.05+0.015*(ageYears-5))
	}
}

// calculateLikelihood 计算似然
func (f *FailurePredictor) calculateLikelihood(score *HealthScore, analysis *SMARTAnalysisResult) float64 {
	likelihood := 0.0

	scoreLikelihood := 1.0 - score.Score/100.0
	likelihood = scoreLikelihood * 0.4

	anomalyCount := float64(len(analysis.Anomalies))
	if anomalyCount > 0 {
		likelihood += math.Min(0.3, anomalyCount*0.1)
	}

	switch analysis.OverallTrend {
	case TrendDeclining:
		likelihood += 0.15
	case TrendCritical:
		likelihood += 0.3
	}

	for _, attr := range score.AttributeScores {
		if attr.Score < 30 {
			likelihood += 0.1
		}
	}

	return math.Min(1.0, math.Max(0.0, likelihood))
}

// estimateRemainingLife 估算剩余寿命
func (f *FailurePredictor) estimateRemainingLife(data *SMARTData, score float64) int {
	days := int(score * 18.25)

	if data.IsSSD {
		for _, attr := range data.Attributes {
			if attr.ID == SMARTIDSSDLifeLeft {
				ssdDays := int(float64(attr.RawValue) * 36.5)
				days = min(days, ssdDays)
				break
			}
			if attr.ID == SMARTIDWearLevelingCount {
				wearDays := int(float64(attr.RawValue) * 36.5)
				days = min(days, wearDays)
				break
			}
		}
	}

	reallocated := getAttributeValue(data, SMARTIDReallocatedSectorCt)
	if reallocated > 0 {
		days -= int(reallocated) * 30
	}

	pending := getAttributeValue(data, SMARTIDCurrentPendingSector)
	if pending > 0 {
		days -= int(pending) * 20
	}

	if days < 0 {
		days = 0
	}
	return days
}

// calculateConfidence 计算置信度
func (f *FailurePredictor) calculateConfidence(analysis *SMARTAnalysisResult) float64 {
	confidence := 0.5

	historyLen := float64(len(f.analyzer.GetHistory(analysis.Device)))
	if historyLen > 100 {
		confidence += 0.2
	} else if historyLen > 30 {
		confidence += 0.1
	}

	var avgRSquared float64
	rSquaredCount := 0
	for _, attr := range analysis.Attributes {
		if attr.Regression != nil {
			avgRSquared += attr.Regression.RSquared
			rSquaredCount++
		}
	}
	if rSquaredCount > 0 {
		avgRSquared /= float64(rSquaredCount)
		confidence += avgRSquared * 0.2
	}

	if len(analysis.Anomalies) > 0 {
		confidence += 0.1
	}

	return math.Min(1.0, confidence)
}

// identifyRiskFactors 识别风险因素
func (f *FailurePredictor) identifyRiskFactors(data *SMARTData, score *HealthScore) []string {
	var factors []string

	reallocated := getAttributeValue(data, SMARTIDReallocatedSectorCt)
	if reallocated > 0 {
		factors = append(factors, fmt.Sprintf("存在 %d 个重映射扇区", reallocated))
	}

	pending := getAttributeValue(data, SMARTIDCurrentPendingSector)
	if pending > 0 {
		factors = append(factors, fmt.Sprintf("存在 %d 个待映射扇区", pending))
	}

	uncorrectable := getAttributeValue(data, SMARTIDOfflineUncorrectable)
	if uncorrectable > 0 {
		factors = append(factors, fmt.Sprintf("存在 %d 个不可修复扇区", uncorrectable))
	}

	temp := getAttributeValue(data, SMARTIDTemperatureCelsius)
	if temp > 55 {
		factors = append(factors, fmt.Sprintf("温度过高: %d℃", temp))
	}

	poh := getAttributeValue(data, SMARTIDPowerOnHours)
	if poh > 35000 {
		factors = append(factors, fmt.Sprintf("通电时间过长: %d 小时", poh))
	}

	if data.IsSSD {
		wear := getAttributeValue(data, SMARTIDWearLevelingCount)
		if wear < 20 {
			factors = append(factors, fmt.Sprintf("SSD 磨损严重，剩余寿命 %d%%", wear))
		}
	}

	if score.Score < 50 {
		factors = append(factors, fmt.Sprintf("综合健康评分过低: %.1f", score.Score))
	}

	return factors
}
