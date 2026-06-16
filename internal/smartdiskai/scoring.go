// Package smartdiskai - 健康评分系统、故障预测、磁盘生命周期管理
package smartdiskai

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ============================================================
// HealthScorer - 健康评分系统 (0-100)
// ============================================================

// HealthScorer 健康评分系统
type HealthScorer struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	collector    *SMARTCollector
	scoreHistory map[string][]float64
	weights      map[SMARTAttributeID]float64
}

// NewHealthScorer 创建健康评分系统
func NewHealthScorer(logger *zap.Logger, collector *SMARTCollector) *HealthScorer {
	if logger == nil {
		logger = zap.NewNop()
	}
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
	return &HealthScorer{
		logger:       logger,
		collector:    collector,
		scoreHistory: make(map[string][]float64),
		weights:      weights,
	}
}

// Calculate 计算设备健康评分
func (h *HealthScorer) Calculate(device string) (*HealthScore, error) {
	data, err := h.collector.GetLatestData(device)
	if err != nil {
		return nil, err
	}

	score := &HealthScore{
		Device:       device,
		CalculatedAt: time.Now(),
	}

	// 获取上次评分
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

	// 计算加权平均分
	if totalWeight > 0 {
		score.Score = totalWeightedScore / totalWeight
	}

	// 限制在 0-100 范围
	score.Score = math.Max(0, math.Min(100, score.Score))
	score.Grade = ScoreToGrade(score.Score)
	score.Status = ScoreToStatus(score.Score)

	// 计算趋势
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

	// 记录评分历史
	h.mu.Lock()
	h.scoreHistory[device] = append(h.scoreHistory[device], score.Score)
	if len(h.scoreHistory[device]) > 365 {
		h.scoreHistory[device] = h.scoreHistory[device][len(h.scoreHistory[device])-365:]
	}
	h.mu.Unlock()

	// 按分数排序（最差的在前面）
	sort.Slice(score.AttributeScores, func(i, j int) bool {
		return score.AttributeScores[i].Score < score.AttributeScores[j].Score
	})

	h.logger.Debug("健康评分计算完成",
		zap.String("device", device),
		zap.Float64("score", score.Score),
		zap.String("grade", string(score.Grade)),
	)

	return score, nil
}

// scoreAttribute 为单个属性评分
func (h *HealthScorer) scoreAttribute(attrID SMARTAttributeID, value uint64, data *SMARTData) float64 {
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

// ============================================================
// FailurePredictor - 故障预测器（线性回归 + 阈值报警）
// ============================================================

// FailurePredictor 故障预测器
type FailurePredictor struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	collector *SMARTCollector
	scorer    *HealthScorer

	// 阈值配置
	reallocThreshold   uint64
	pendingThreshold   uint64
	uncorrectThreshold uint64
	tempWarnThreshold  uint64
	tempCritThreshold  uint64
}

// NewFailurePredictor 创建故障预测器
func NewFailurePredictor(logger *zap.Logger, collector *SMARTCollector, scorer *HealthScorer) *FailurePredictor {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FailurePredictor{
		logger:             logger,
		collector:          collector,
		scorer:             scorer,
		reallocThreshold:   10,
		pendingThreshold:   5,
		uncorrectThreshold: 5,
		tempWarnThreshold:  50,
		tempCritThreshold:  60,
	}
}

// Predict 对设备进行故障预测
func (f *FailurePredictor) Predict(device string) (*FailurePrediction, error) {
	healthScore, err := f.scorer.Calculate(device)
	if err != nil {
		return nil, fmt.Errorf("计算健康评分失败: %w", err)
	}

	analysis, err := f.collector.Analyze(device)
	if err != nil {
		return nil, fmt.Errorf("SMART 分析失败: %w", err)
	}

	data, err := f.collector.GetLatestData(device)
	if err != nil {
		return nil, err
	}

	prediction := &FailurePrediction{
		Device:      device,
		PredictedAt: time.Now(),
	}

	// 1. 基于健康评分计算基础故障概率
	baseProb := 1.0 - healthScore.Score/100.0

	// 2. 检查阈值违规
	violations := f.checkThresholdViolations(data)
	prediction.ThresholdViolations = violations

	// 根据违规调整概率
	violationProb := 0.0
	for _, v := range violations {
		if v.Severity == "critical" {
			violationProb += 0.15
		} else {
			violationProb += 0.05
		}
	}

	// 3. 基于趋势分析调整
	trendProb := 0.0
	switch analysis.OverallTrend {
	case TrendDeclining:
		trendProb = 0.15
	case TrendCritical:
		trendProb = 0.30
	}

	// 4. 综合故障概率
	failureProb := baseProb*0.5 + violationProb + trendProb
	failureProb = math.Min(1.0, math.Max(0.0, failureProb))
	prediction.FailureProbability = failureProb

	// 5. 确定风险等级
	prediction.RiskLevel = probabilityToRiskLevel(failureProb)

	// 6. 估算剩余寿命
	prediction.EstimatedLifeDays = f.estimateRemainingLife(data, healthScore.Score, analysis)

	// 7. 预计故障日期
	if prediction.EstimatedLifeDays > 0 {
		failDate := time.Now().AddDate(0, 0, prediction.EstimatedLifeDays)
		prediction.FailDateEstimate = &failDate
	}

	// 8. 计算置信度
	prediction.Confidence = f.calculateConfidence(analysis)

	// 9. 识别风险因素
	prediction.RiskFactors = f.identifyRiskFactors(data, healthScore, analysis)

	f.logger.Info("故障预测完成",
		zap.String("device", device),
		zap.Float64("probability", failureProb),
		zap.String("risk_level", string(prediction.RiskLevel)),
		zap.Int("estimated_life_days", prediction.EstimatedLifeDays),
	)

	return prediction, nil
}

// checkThresholdViolations 检查阈值违规
func (f *FailurePredictor) checkThresholdViolations(data *SMARTData) []ThresholdViolation {
	var violations []ThresholdViolation

	reallocated := getAttributeValue(data, SMARTIDReallocatedSectorCt)
	if reallocated > f.reallocThreshold {
		severity := "warning"
		if reallocated > f.reallocThreshold*5 {
			severity = "critical"
		}
		violations = append(violations, ThresholdViolation{
			AttributeID:   SMARTIDReallocatedSectorCt,
			AttributeName: GetAttributeName(SMARTIDReallocatedSectorCt),
			CurrentValue:  reallocated,
			Threshold:     f.reallocThreshold,
			Severity:      severity,
			Message:       fmt.Sprintf("重映射扇区 %d 超过阈值 %d", reallocated, f.reallocThreshold),
		})
	}

	pending := getAttributeValue(data, SMARTIDCurrentPendingSector)
	if pending > f.pendingThreshold {
		severity := "warning"
		if pending > f.pendingThreshold*5 {
			severity = "critical"
		}
		violations = append(violations, ThresholdViolation{
			AttributeID:   SMARTIDCurrentPendingSector,
			AttributeName: GetAttributeName(SMARTIDCurrentPendingSector),
			CurrentValue:  pending,
			Threshold:     f.pendingThreshold,
			Severity:      severity,
			Message:       fmt.Sprintf("待映射扇区 %d 超过阈值 %d", pending, f.pendingThreshold),
		})
	}

	uncorrectable := getAttributeValue(data, SMARTIDOfflineUncorrectable)
	if uncorrectable > f.uncorrectThreshold {
		violations = append(violations, ThresholdViolation{
			AttributeID:   SMARTIDOfflineUncorrectable,
			AttributeName: GetAttributeName(SMARTIDOfflineUncorrectable),
			CurrentValue:  uncorrectable,
			Threshold:     f.uncorrectThreshold,
			Severity:      "warning",
			Message:       fmt.Sprintf("不可修复扇区 %d 超过阈值 %d", uncorrectable, f.uncorrectThreshold),
		})
	}

	temp := getAttributeValue(data, SMARTIDTemperatureCelsius)
	if temp >= f.tempCritThreshold {
		violations = append(violations, ThresholdViolation{
			AttributeID:   SMARTIDTemperatureCelsius,
			AttributeName: GetAttributeName(SMARTIDTemperatureCelsius),
			CurrentValue:  temp,
			Threshold:     f.tempCritThreshold,
			Severity:      "critical",
			Message:       fmt.Sprintf("温度 %d℃ 超过临界阈值 %d℃", temp, f.tempCritThreshold),
		})
	} else if temp >= f.tempWarnThreshold {
		violations = append(violations, ThresholdViolation{
			AttributeID:   SMARTIDTemperatureCelsius,
			AttributeName: GetAttributeName(SMARTIDTemperatureCelsius),
			CurrentValue:  temp,
			Threshold:     f.tempWarnThreshold,
			Severity:      "warning",
			Message:       fmt.Sprintf("温度 %d℃ 超过警告阈值 %d℃", temp, f.tempWarnThreshold),
		})
	}

	return violations
}

// estimateRemainingLife 估算剩余寿命
func (f *FailurePredictor) estimateRemainingLife(data *SMARTData, score float64, analysis *SMARTAnalysisResult) int {
	// 基础寿命估算：评分 * 18.25 天
	days := int(score * 18.25)

	// SSD 特殊处理
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

	// 根据坏扇区调整
	reallocated := getAttributeValue(data, SMARTIDReallocatedSectorCt)
	if reallocated > 0 {
		days -= int(reallocated) * 30
	}

	pending := getAttributeValue(data, SMARTIDCurrentPendingSector)
	if pending > 0 {
		days -= int(pending) * 20
	}

	// 根据趋势调整
	if analysis != nil {
		for _, attr := range analysis.Attributes {
			if attr.Trend == TrendDeclining && (attr.AttributeID == SMARTIDReallocatedSectorCt ||
				attr.AttributeID == SMARTIDCurrentPendingSector) {
				days = int(float64(days) * 0.7) // 恶化趋势，减少30%
				break
			}
		}
	}

	if days < 0 {
		days = 0
	}
	return days
}

// calculateConfidence 计算置信度
func (f *FailurePredictor) calculateConfidence(analysis *SMARTAnalysisResult) float64 {
	confidence := 0.5

	historyLen := float64(len(f.collector.GetHistory(analysis.Device)))
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

	return math.Min(1.0, confidence)
}

// identifyRiskFactors 识别风险因素
func (f *FailurePredictor) identifyRiskFactors(data *SMARTData, score *HealthScore, analysis *SMARTAnalysisResult) []string {
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

	// 趋势风险
	if analysis != nil && analysis.OverallTrend == TrendDeclining {
		factors = append(factors, "SMART 属性呈恶化趋势")
	}

	return factors
}

// ============================================================
// LifecycleManager - 磁盘生命周期管理
// ============================================================

// LifecycleManager 磁盘生命周期管理
type LifecycleManager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	collector *SMARTCollector
	scorer    *HealthScorer

	// 保修信息存储
	warrantyInfo map[string]*WarrantyInfo
}

// WarrantyInfo 保修信息
type WarrantyInfo struct {
	Device         string     `json:"device"`
	ManufactureDate *time.Time `json:"manufacture_date,omitempty"`
	WarrantyStart   *time.Time `json:"warranty_start,omitempty"`
	WarrantyEnd     *time.Time `json:"warranty_end,omitempty"`
	WarrantyYears   int        `json:"warranty_years"`
}

// NewLifecycleManager 创建生命周期管理器
func NewLifecycleManager(logger *zap.Logger, collector *SMARTCollector, scorer *HealthScorer) *LifecycleManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &LifecycleManager{
		logger:       logger,
		collector:    collector,
		scorer:       scorer,
		warrantyInfo: make(map[string]*WarrantyInfo),
	}
}

// SetWarrantyInfo 设置磁盘保修信息
func (l *LifecycleManager) SetWarrantyInfo(info *WarrantyInfo) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warrantyInfo[info.Device] = info
}

// GetDiskLifecycle 获取磁盘生命周期信息
func (l *LifecycleManager) GetDiskLifecycle(device string) (*DiskLifecycle, error) {
	data, err := l.collector.GetLatestData(device)
	if err != nil {
		return nil, err
	}

	score, err := l.scorer.Calculate(device)
	if err != nil {
		return nil, err
	}

	lifecycle := &DiskLifecycle{
		Device:       device,
		Model:        data.Model,
		Serial:       data.Serial,
		PowerOnHours: data.PowerOnHours,
		IsSSD:        data.IsSSD,
		TotalWrites:  data.TotalLBAsWritten * 512, // LBA 转字节
		TotalReads:   data.TotalLBAsRead * 512,
		HealthScore:  score.Score,
		UpdatedAt:    time.Now(),
	}

	// 保修信息
	l.mu.RLock()
	warranty, ok := l.warrantyInfo[device]
	l.mu.RUnlock()

	if ok {
		lifecycle.ManufactureDate = warranty.ManufactureDate
		lifecycle.WarrantyStart = warranty.WarrantyStart
		lifecycle.WarrantyEnd = warranty.WarrantyEnd
		lifecycle.WarrantyYears = warranty.WarrantyYears

		if warranty.WarrantyEnd != nil {
			daysLeft := int(time.Until(*warranty.WarrantyEnd).Hours() / 24)
			lifecycle.WarrantyDaysLeft = daysLeft
			if daysLeft < 0 {
				lifecycle.WarrantyStatus = "expired"
			} else if daysLeft < 90 {
				lifecycle.WarrantyStatus = "expiring_soon"
			} else {
				lifecycle.WarrantyStatus = "active"
			}
		} else {
			lifecycle.WarrantyStatus = "unknown"
		}

		if warranty.ManufactureDate != nil {
			lifecycle.AgeDays = int(time.Since(*warranty.ManufactureDate).Hours() / 24)
		}
	} else {
		lifecycle.WarrantyStatus = "unknown"
		// 根据通电时间估算使用天数
		lifecycle.AgeDays = int(data.PowerOnHours / 24)
	}

	// SSD 磨损均衡信息
	if data.IsSSD {
		lifecycle.WearLevel = l.calculateWearLevel(data)
		if lifecycle.WearLevel != nil {
			lifecycle.RemainingLife = lifecycle.WearLevel.PercentRemaining
		}
	} else {
		// HDD 使用寿命估算
		expectedLifeHours := 40000.0
		lifecycle.RemainingLife = math.Max(0, 100*(1-float64(data.PowerOnHours)/expectedLifeHours))
	}

	return lifecycle, nil
}

// calculateWearLevel 计算 SSD 磨损均衡信息
func (l *LifecycleManager) calculateWearLevel(data *SMARTData) *WearLevelInfo {
	wearLevel := getAttributeValue(data, SMARTIDWearLevelingCount)
	nandWrites := getAttributeValue(data, SMARTIDNANDWrites)
	lifeLeft := getAttributeValue(data, SMARTIDSSDLifeLeft)

	if wearLevel == 0 && nandWrites == 0 && lifeLeft == 0 {
		return nil
	}

	info := &WearLevelInfo{
		WearLevelingCount: wearLevel,
	}

	// SSD Life Left 直接表示剩余百分比
	if lifeLeft > 0 {
		info.PercentRemaining = float64(lifeLeft)
		info.PercentUsed = 100 - float64(lifeLeft)
	}

	// NAND Writes 估算 TBW
	if nandWrites > 0 {
		info.CurrentTBW = float64(nandWrites) / 1e12
		info.EstimatedTBW = 600.0 // 假设 600 TBW 预期寿命
		info.TBWRatio = info.CurrentTBW / info.EstimatedTBW
		if lifeLeft == 0 {
			info.PercentUsed = info.TBWRatio * 100
			info.PercentRemaining = math.Max(0, 100-info.PercentUsed)
		}
	}

	return info
}

// GetLifecycleSummary 获取所有磁盘生命周期摘要
func (l *LifecycleManager) GetLifecycleSummary() ([]*DiskLifecycle, error) {
	devices := l.collector.GetDevices()
	if len(devices) == 0 {
		return nil, fmt.Errorf("无磁盘数据")
	}

	var lifecycles []*DiskLifecycle
	for _, device := range devices {
		lc, err := l.GetDiskLifecycle(device)
		if err != nil {
			continue
		}
		lifecycles = append(lifecycles, lc)
	}

	return lifecycles, nil
}

// ============================================================
// 辅助函数
// ============================================================

// ScoreToGrade 评分转等级
func ScoreToGrade(score float64) HealthGrade {
	switch {
	case score >= 90:
		return GradeExcellent
	case score >= 70:
		return GradeGood
	case score >= 50:
		return GradeFair
	case score >= 30:
		return GradePoor
	default:
		return GradeCritical
	}
}

// ScoreToStatus 评分转状态
func ScoreToStatus(score float64) DiskStatus {
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

// probabilityToRiskLevel 故障概率转风险等级
func probabilityToRiskLevel(prob float64) RiskLevel {
	switch {
	case prob >= 0.7:
		return RiskCritical
	case prob >= 0.4:
		return RiskHigh
	case prob >= 0.2:
		return RiskMedium
	default:
		return RiskLow
	}
}
