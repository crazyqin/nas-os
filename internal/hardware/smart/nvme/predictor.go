// Package nvme 提供NVMe健康预测和监控增强功能
// Version: v1.0.0 - 户部第189轮任务
package nvme

import (
	"context"
	"math"
	"sync"
	"time"
)

// ============================================================================
// 类型定义 - 本包独立类型
// ============================================================================

// HealthStatus NVMe健康状态 (从 internal/hardware/nvme 复制以避免循环导入)
type HealthStatus struct {
	Device          string  // 设备路径
	Temperature     int     // 温度 (摄氏度)
	PercentUsed     float64 // 已用寿命百分比
	AvailableSpare  float64 // 可用备用空间百分比
	CriticalWarning int     // 关键警告标志
	DataUnitsWrite  uint64  // 写入数据单位
	MediaErrors     uint64  // 媒体错误数
}

// PredictedLife 预测寿命结果
type PredictedLife struct {
	RemainingDays    int       `json:"remainingDays"`    // 预计剩余天数
	EstimatedEndDate time.Time `json:"estimatedEndDate"` // 预计失效日期
	WriteRatePerDay  uint64    `json:"writeRatePerDay"`  // 日均写入量
	LifeDecayRate    float64   `json:"lifeDecayRate"`    // 寿命衰减率(%/天)
	Confidence       float64   `json:"confidence"`       // 置信度 (0-1)
	Method           string    `json:"method"`           // 预测方法
}

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelNone      AlertLevel = "none"
	AlertLevelWarning   AlertLevel = "warning"
	AlertLevelCritical  AlertLevel = "critical"
	AlertLevelEmergency AlertLevel = "emergency"
)

// HealthStatusType 健康状态类型
type HealthStatusType string

const (
	HealthStatusHealthy   HealthStatusType = "healthy"
	HealthStatusWarning   HealthStatusType = "warning"
	HealthStatusCritical  HealthStatusType = "critical"
	HealthStatusEmergency HealthStatusType = "emergency"
	HealthStatusUnknown   HealthStatusType = "unknown"
)

// ============================================================================
// 寿命预测模块
// ============================================================================

// LifePredictor NVMe寿命预测器
type LifePredictor struct {
	config         *PredictionConfig
	history        map[string][]*HistoryPoint
	models         map[string]*PredictionModel
	mu             sync.RWMutex
	minSamples     int
	optimalSamples int
}

// PredictionConfig 预测配置
type PredictionConfig struct {
	Enabled          bool    `json:"enabled"`
	MinSamples       int     `json:"minSamples"`
	OptimalSamples   int     `json:"optimalSamples"`
	MaxConfidence    float64 `json:"maxConfidence"`
	PredictionWindow int     `json:"predictionWindow"`
}

// DefaultPredictionConfig 默认预测配置
func DefaultPredictionConfig() *PredictionConfig {
	return &PredictionConfig{
		Enabled:          true,
		MinSamples:       10,
		OptimalSamples:   1440,
		MaxConfidence:    0.85,
		PredictionWindow: 365,
	}
}

// HistoryPoint 历史数据点
type HistoryPoint struct {
	Timestamp      time.Time `json:"timestamp"`
	PercentUsed    float64   `json:"percentUsed"`
	AvailableSpare float64   `json:"availableSpare"`
	Temperature    int       `json:"temperature"`
	TotalWrites    uint64    `json:"totalWrites"`
	MediaErrors    uint64    `json:"mediaErrors"`
	HealthScore    float64   `json:"healthScore"`
}

// PredictionModel 预测模型
type PredictionModel struct {
	Device          string    `json:"device"`
	WriteRatePerDay uint64    `json:"writeRatePerDay"`
	LifeDecayRate   float64   `json:"lifeDecayRate"`
	LastCalculated  time.Time `json:"lastCalculated"`
	SampleCount     int       `json:"sampleCount"`
}

// NewLifePredictor 创建寿命预测器
func NewLifePredictor(config *PredictionConfig) *LifePredictor {
	if config == nil {
		config = DefaultPredictionConfig()
	}
	return &LifePredictor{
		config:         config,
		history:        make(map[string][]*HistoryPoint),
		models:         make(map[string]*PredictionModel),
		minSamples:     config.MinSamples,
		optimalSamples: config.OptimalSamples,
	}
}

// AddHistoryPoint 添加历史数据点
func (p *LifePredictor) AddHistoryPoint(device string, health *HealthStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()

	point := &HistoryPoint{
		Timestamp:      time.Now(),
		PercentUsed:    health.PercentUsed,
		AvailableSpare: health.AvailableSpare,
		Temperature:    health.Temperature,
		TotalWrites:    health.DataUnitsWrite,
		MediaErrors:    health.MediaErrors,
	}

	point.HealthScore = CalculateHealthScore(health)

	history := p.history[device]
	history = append(history, point)

	maxPoints := p.optimalSamples * 2
	if len(history) > maxPoints {
		history = history[len(history)-maxPoints:]
	}

	p.history[device] = history
}

// Predict 执行寿命预测
func (p *LifePredictor) Predict(device string) *PredictedLife {
	p.mu.RLock()
	history := p.history[device]
	p.mu.RUnlock()

	if len(history) < p.minSamples {
		return &PredictedLife{
			RemainingDays: -1,
			Confidence:    0,
			Method:        "insufficient_data",
		}
	}

	writeRate := p.calculateWriteRate(history)
	lifeDecayRate := p.calculateLifeDecayRate(history)

	lastPoint := history[len(history)-1]
	remainingPercent := 100.0 - lastPoint.PercentUsed

	var remainingDays int
	if lifeDecayRate > 0 {
		remainingDaysFloat := remainingPercent / lifeDecayRate
		if remainingDaysFloat > 36500 {
			remainingDays = 36500
		} else if remainingDaysFloat < 0 {
			remainingDays = 0
		} else {
			remainingDays = int(remainingDaysFloat)
		}
	} else if writeRate > 0 {
		estimatedTBW := float64(lastPoint.TotalWrites) / lastPoint.PercentUsed * 100
		remainingWrites := estimatedTBW - float64(lastPoint.TotalWrites)
		remainingDaysFloat := remainingWrites / float64(writeRate)
		if remainingDaysFloat > 36500 {
			remainingDays = 36500
		} else if remainingDaysFloat < 0 {
			remainingDays = 0
		} else {
			remainingDays = int(remainingDaysFloat)
		}
	} else {
		remainingDays = -1
	}

	confidence := p.calculateConfidence(len(history))

	return &PredictedLife{
		RemainingDays:    remainingDays,
		EstimatedEndDate: time.Now().AddDate(0, 0, remainingDays),
		WriteRatePerDay:  writeRate,
		LifeDecayRate:    lifeDecayRate,
		Confidence:       confidence,
		Method:           "write_rate_projection",
	}
}

// calculateWriteRate 计算写入速率
func (p *LifePredictor) calculateWriteRate(history []*HistoryPoint) uint64 {
	if len(history) < 2 {
		return 0
	}

	var totalWriteIncrease uint64
	for i := 1; i < len(history); i++ {
		if history[i].TotalWrites > history[i-1].TotalWrites {
			totalWriteIncrease += history[i].TotalWrites - history[i-1].TotalWrites
		}
	}

	duration := history[len(history)-1].Timestamp.Sub(history[0].Timestamp)
	days := duration.Hours() / 24

	if days <= 0 {
		return 0
	}

	writeRateFloat := float64(totalWriteIncrease) / days
	if writeRateFloat > float64(math.MaxUint64) {
		return math.MaxUint64
	}
	return uint64(writeRateFloat)
}

// calculateLifeDecayRate 计算寿命衰减速率(%/天)
func (p *LifePredictor) calculateLifeDecayRate(history []*HistoryPoint) float64 {
	if len(history) < 2 {
		return 0
	}

	var totalLifeIncrease float64
	for i := 1; i < len(history); i++ {
		if history[i].PercentUsed > history[i-1].PercentUsed {
			totalLifeIncrease += history[i].PercentUsed - history[i-1].PercentUsed
		}
	}

	duration := history[len(history)-1].Timestamp.Sub(history[0].Timestamp)
	days := duration.Hours() / 24

	if days <= 0 {
		return 0
	}

	return totalLifeIncrease / days
}

// calculateConfidence 计算预测置信度
func (p *LifePredictor) calculateConfidence(sampleCount int) float64 {
	ratio := float64(sampleCount) / float64(p.optimalSamples)
	if ratio > 1 {
		ratio = 1
	}
	return ratio * p.config.MaxConfidence
}

// GetHistory 获取历史数据
func (p *LifePredictor) GetHistory(device string) []*HistoryPoint {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.history[device]
}

// ============================================================================
// 异常检测模块
// ============================================================================

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalyWriteSpike AnomalyType = "write_spike"
	AnomalyTempSpike  AnomalyType = "temp_spike"
	AnomalyLifeJump   AnomalyType = "life_jump"
	AnomalyMediaError AnomalyType = "media_error"
	AnomalySpareLow   AnomalyType = "spare_low"
)

// AnomalyLevel 异常级别
type AnomalyLevel string

const (
	AnomalyNone   AnomalyLevel = "none"
	AnomalyLow    AnomalyLevel = "low"
	AnomalyMedium AnomalyLevel = "medium"
	AnomalyHigh   AnomalyLevel = "high"
	AnomalySevere AnomalyLevel = "severe"
)

// Anomaly 异常事件
type Anomaly struct {
	Device      string       `json:"device"`
	Type        AnomalyType  `json:"type"`
	Severity    AnomalyLevel `json:"severity"`
	Value       interface{}  `json:"value"`
	Baseline    interface{}  `json:"baseline"`
	Description string       `json:"description"`
	Timestamp   time.Time    `json:"timestamp"`
}

// AnomalyDetector 异常检测器
type AnomalyDetector struct {
	config    *AnomalyConfig
	baselines map[string]*Baseline
	anomalies map[string][]*Anomaly
	mu        sync.RWMutex
}

// AnomalyConfig 异常检测配置
type AnomalyConfig struct {
	WriteSpikeThreshold float64 `json:"writeSpikeThreshold"`
	TempSpikeThreshold  float64 `json:"tempSpikeThreshold"`
	LifeJumpThreshold   float64 `json:"lifeJumpThreshold"`
	AnomalyWindow       int     `json:"anomalyWindow"`
}

// DefaultAnomalyConfig 默认异常检测配置
func DefaultAnomalyConfig() *AnomalyConfig {
	return &AnomalyConfig{
		WriteSpikeThreshold: 3.0,
		TempSpikeThreshold:  10.0,
		LifeJumpThreshold:   1.0,
		AnomalyWindow:       5,
	}
}

// Baseline 基线数据
type Baseline struct {
	Device         string    `json:"device"`
	AvgWriteRate   uint64    `json:"avgWriteRate"`
	AvgTemperature int       `json:"avgTemperature"`
	AvgLifeDecay   float64   `json:"avgLifeDecay"`
	LastUpdate     time.Time `json:"lastUpdate"`
}

// NewAnomalyDetector 创建异常检测器
func NewAnomalyDetector(config *AnomalyConfig) *AnomalyDetector {
	if config == nil {
		config = DefaultAnomalyConfig()
	}
	return &AnomalyDetector{
		config:    config,
		baselines: make(map[string]*Baseline),
		anomalies: make(map[string][]*Anomaly),
	}
}

// UpdateBaseline 更新基线数据
func (d *AnomalyDetector) UpdateBaseline(device string, history []*HistoryPoint) {
	if len(history) < 10 {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	var totalWrites uint64
	for _, p := range history {
		totalWrites += p.TotalWrites
	}
	avgWrites := totalWrites / uint64(len(history))

	var totalTemp int
	for _, p := range history {
		totalTemp += p.Temperature
	}
	avgTemp := totalTemp / len(history)

	var totalLife float64
	for _, p := range history {
		totalLife += p.PercentUsed
	}
	avgLife := totalLife / float64(len(history))

	d.baselines[device] = &Baseline{
		Device:         device,
		AvgWriteRate:   avgWrites,
		AvgTemperature: avgTemp,
		AvgLifeDecay:   avgLife,
		LastUpdate:     time.Now(),
	}
}

// Detect 检测异常
func (d *AnomalyDetector) Detect(device string, health *HealthStatus) []*Anomaly {
	d.mu.RLock()
	baseline := d.baselines[device]
	d.mu.RUnlock()

	if baseline == nil {
		return nil
	}

	anomalies := []*Anomaly{}

	// 检测写入突发
	if baseline.AvgWriteRate > 0 {
		ratio := float64(health.DataUnitsWrite) / float64(baseline.AvgWriteRate)
		if ratio > d.config.WriteSpikeThreshold {
			severity := AnomalyMedium
			if ratio > 5 {
				severity = AnomalyHigh
			}
			if ratio > 10 {
				severity = AnomalySevere
			}
			anomalies = append(anomalies, &Anomaly{
				Device:      device,
				Type:        AnomalyWriteSpike,
				Severity:    severity,
				Value:       ratio,
				Baseline:    baseline.AvgWriteRate,
				Description: "写入速率异常高于基线",
				Timestamp:   time.Now(),
			})
		}
	}

	// 检测温度突发
	tempDiff := health.Temperature - baseline.AvgTemperature
	if tempDiff > int(d.config.TempSpikeThreshold) {
		severity := AnomalyMedium
		if tempDiff > 20 {
			severity = AnomalyHigh
		}
		anomalies = append(anomalies, &Anomaly{
			Device:      device,
			Type:        AnomalyTempSpike,
			Severity:    severity,
			Value:       health.Temperature,
			Baseline:    baseline.AvgTemperature,
			Description: "温度异常升高",
			Timestamp:   time.Now(),
		})
	}

	// 检测媒体错误
	if health.MediaErrors > 0 {
		severity := AnomalyLow
		if health.MediaErrors > 5 {
			severity = AnomalyMedium
		}
		if health.MediaErrors > 10 {
			severity = AnomalyHigh
		}
		anomalies = append(anomalies, &Anomaly{
			Device:      device,
			Type:        AnomalyMediaError,
			Severity:    severity,
			Value:       health.MediaErrors,
			Baseline:    0,
			Description: "检测到媒体错误",
			Timestamp:   time.Now(),
		})
	}

	// 检测备用空间低
	if health.AvailableSpare < 20 {
		severity := AnomalyLow
		if health.AvailableSpare < 10 {
			severity = AnomalyMedium
		}
		if health.AvailableSpare < 5 {
			severity = AnomalySevere
		}
		anomalies = append(anomalies, &Anomaly{
			Device:      device,
			Type:        AnomalySpareLow,
			Severity:    severity,
			Value:       health.AvailableSpare,
			Baseline:    100,
			Description: "可用备用空间不足",
			Timestamp:   time.Now(),
		})
	}

	d.mu.Lock()
	deviceAnomalies := d.anomalies[device]
	deviceAnomalies = append(deviceAnomalies, anomalies...)
	if len(deviceAnomalies) > 100 {
		deviceAnomalies = deviceAnomalies[len(deviceAnomalies)-100:]
	}
	d.anomalies[device] = deviceAnomalies
	d.mu.Unlock()

	return anomalies
}

// GetAnomalies 获取异常历史
func (d *AnomalyDetector) GetAnomalies(device string) []*Anomaly {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.anomalies[device]
}

// ============================================================================
// 综合健康评估模块
// ============================================================================

// ComprehensiveHealth 全面健康评估
type ComprehensiveHealth struct {
	Device          string           `json:"device"`
	HealthScore     float64          `json:"healthScore"`
	Status          HealthStatusType `json:"status"`
	AlertLevel      AlertLevel       `json:"alertLevel"`
	AlertMessage    string           `json:"alertMessage"`
	PredictedLife   *PredictedLife   `json:"predictedLife"`
	Anomalies       []*Anomaly       `json:"anomalies"`
	RiskFactors     []string         `json:"riskFactors"`
	Recommendations []string         `json:"recommendations"`
	Timestamp       time.Time        `json:"timestamp"`
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	LifespanWarning     float64 `json:"lifespanWarning"`
	LifespanCritical    float64 `json:"lifespanCritical"`
	LifespanEmergency   float64 `json:"lifespanEmergency"`
	TemperatureWarning  int     `json:"temperatureWarning"`
	TemperatureCritical int     `json:"temperatureCritical"`
	SpareWarning        float64 `json:"spareWarning"`
	SpareCritical       float64 `json:"spareCritical"`
	DaysWarning         int     `json:"daysWarning"`
	DaysCritical        int     `json:"daysCritical"`
}

// DefaultThresholdConfig 默认阈值配置
func DefaultThresholdConfig() *ThresholdConfig {
	return &ThresholdConfig{
		LifespanWarning:     80,
		LifespanCritical:    90,
		LifespanEmergency:   95,
		TemperatureWarning:  60,
		TemperatureCritical: 70,
		SpareWarning:        20,
		SpareCritical:       10,
		DaysWarning:         180,
		DaysCritical:        90,
	}
}

// EvaluationConfig 评估配置
type EvaluationConfig struct {
	Thresholds *ThresholdConfig  `json:"thresholds"`
	Prediction *PredictionConfig `json:"prediction"`
	Anomaly    *AnomalyConfig    `json:"anomaly"`
}

// HealthEvaluator 健康评估器
type HealthEvaluator struct {
	predictor *LifePredictor
	detector  *AnomalyDetector
	config    *EvaluationConfig
}

// NewHealthEvaluator 创建健康评估器
func NewHealthEvaluator(config *EvaluationConfig) *HealthEvaluator {
	if config == nil {
		config = &EvaluationConfig{
			Thresholds: DefaultThresholdConfig(),
			Prediction: DefaultPredictionConfig(),
			Anomaly:    DefaultAnomalyConfig(),
		}
	}
	return &HealthEvaluator{
		predictor: NewLifePredictor(config.Prediction),
		detector:  NewAnomalyDetector(config.Anomaly),
		config:    config,
	}
}

// Evaluate 执行全面健康评估
func (e *HealthEvaluator) Evaluate(device string, health *HealthStatus, history []*HistoryPoint) *ComprehensiveHealth {
	// 添加当前健康状态作为新数据点
	e.predictor.AddHistoryPoint(device, health)

	e.detector.UpdateBaseline(device, history)

	healthScore := CalculateHealthScore(health)
	predictedLife := e.predictor.Predict(device)
	anomalies := e.detector.Detect(device, health)

	alertLevel, alertMessage := e.evaluateAlertLevel(health, predictedLife, anomalies)
	status := e.determineStatus(alertLevel, healthScore)
	riskFactors := e.identifyRiskFactors(health, predictedLife, anomalies)
	recommendations := e.generateRecommendations(status, riskFactors)

	return &ComprehensiveHealth{
		Device:          device,
		HealthScore:     healthScore,
		Status:          status,
		AlertLevel:      alertLevel,
		AlertMessage:    alertMessage,
		PredictedLife:   predictedLife,
		Anomalies:       anomalies,
		RiskFactors:     riskFactors,
		Recommendations: recommendations,
		Timestamp:       time.Now(),
	}
}

// evaluateAlertLevel 评估告警级别
func (e *HealthEvaluator) evaluateAlertLevel(health *HealthStatus, predicted *PredictedLife, anomalies []*Anomaly) (AlertLevel, string) {
	th := e.config.Thresholds

	if health.PercentUsed >= th.LifespanEmergency {
		return AlertLevelEmergency, "NVMe寿命即将耗尽，请立即更换！"
	}
	if health.AvailableSpare <= 5 {
		return AlertLevelEmergency, "可用备用空间严重不足，请立即更换！"
	}
	if health.Temperature >= 80 {
		return AlertLevelEmergency, "NVMe温度过高，可能导致数据损坏！"
	}
	if predicted != nil && predicted.RemainingDays > 0 && predicted.RemainingDays < 30 {
		return AlertLevelEmergency, "预计30天内寿命耗尽，请立即更换！"
	}

	if health.PercentUsed >= th.LifespanCritical {
		return AlertLevelCritical, "NVMe寿命已使用超过90%，请尽快更换！"
	}
	if health.AvailableSpare <= th.SpareCritical {
		return AlertLevelCritical, "可用备用空间不足10%，请尽快更换！"
	}
	if health.Temperature >= th.TemperatureCritical {
		return AlertLevelCritical, "NVMe温度过高，需要降温措施！"
	}
	if predicted != nil && predicted.RemainingDays > 0 && predicted.RemainingDays < th.DaysCritical {
		return AlertLevelCritical, "预计90天内寿命耗尽，请尽快更换！"
	}

	if health.PercentUsed >= th.LifespanWarning {
		return AlertLevelWarning, "NVMe寿命已使用超过80%，建议准备更换！"
	}
	if health.AvailableSpare <= th.SpareWarning {
		return AlertLevelWarning, "可用备用空间不足20%，建议关注！"
	}
	if health.Temperature >= th.TemperatureWarning {
		return AlertLevelWarning, "NVMe温度偏高，建议检查散热！"
	}
	if predicted != nil && predicted.RemainingDays > 0 && predicted.RemainingDays < th.DaysWarning {
		return AlertLevelWarning, "预计180天内寿命耗尽，建议准备更换！"
	}

	for _, a := range anomalies {
		if a.Severity == AnomalySevere || a.Severity == AnomalyHigh {
			return AlertLevelWarning, a.Description
		}
	}

	return AlertLevelNone, "NVMe状态正常"
}

// determineStatus 确定健康状态
func (e *HealthEvaluator) determineStatus(alertLevel AlertLevel, healthScore float64) HealthStatusType {
	switch alertLevel {
	case AlertLevelEmergency:
		return HealthStatusEmergency
	case AlertLevelCritical:
		return HealthStatusCritical
	case AlertLevelWarning:
		return HealthStatusWarning
	default:
		if healthScore >= 80 {
			return HealthStatusHealthy
		} else if healthScore >= 60 {
			return HealthStatusWarning
		} else if healthScore >= 40 {
			return HealthStatusCritical
		}
		return HealthStatusEmergency
	}
}

// identifyRiskFactors 识别风险因素
func (e *HealthEvaluator) identifyRiskFactors(health *HealthStatus, predicted *PredictedLife, anomalies []*Anomaly) []string {
	factors := []string{}

	if health.PercentUsed >= 80 {
		factors = append(factors, "寿命已用超过80%")
	}
	if health.AvailableSpare <= 20 {
		factors = append(factors, "备用空间不足")
	}
	if health.Temperature >= 60 {
		factors = append(factors, "温度偏高")
	}
	if health.MediaErrors > 0 {
		factors = append(factors, "存在媒体错误")
	}
	if health.CriticalWarning > 0 {
		factors = append(factors, "存在严重警告标志")
	}
	if predicted != nil && predicted.RemainingDays > 0 && predicted.RemainingDays < 180 {
		factors = append(factors, "预计寿命不足180天")
	}
	if predicted != nil && predicted.LifeDecayRate > 0.1 {
		factors = append(factors, "寿命衰减速率较快")
	}

	for _, a := range anomalies {
		if a.Severity != AnomalyNone && a.Severity != AnomalyLow {
			factors = append(factors, a.Description)
		}
	}

	return factors
}

// generateRecommendations 生成建议
func (e *HealthEvaluator) generateRecommendations(status HealthStatusType, riskFactors []string) []string {
	recommendations := []string{}

	switch status {
	case HealthStatusEmergency:
		recommendations = append(recommendations, "立即备份数据")
		recommendations = append(recommendations, "立即更换硬盘")
		recommendations = append(recommendations, "停止写入操作")
	case HealthStatusCritical:
		recommendations = append(recommendations, "尽快备份数据")
		recommendations = append(recommendations, "尽快更换硬盘")
		recommendations = append(recommendations, "减少写入负载")
	case HealthStatusWarning:
		recommendations = append(recommendations, "准备备份数据")
		recommendations = append(recommendations, "准备替换硬盘")
		recommendations = append(recommendations, "监控健康状态")
	default:
		recommendations = append(recommendations, "继续保持监控")
	}

	for _, f := range riskFactors {
		if f == "温度偏高" {
			recommendations = append(recommendations, "检查散热系统")
		}
		if f == "寿命衰减速率较快" {
			recommendations = append(recommendations, "检查写入模式")
		}
	}

	return recommendations
}

// ============================================================================
// 辅助函数
// ============================================================================

// CalculateHealthScore 计算健康评分
func CalculateHealthScore(health *HealthStatus) float64 {
	score := 100.0

	lifeScore := (100.0 - health.PercentUsed) * 0.4
	spareScore := health.AvailableSpare * 0.3
	tempScore := calculateTempScore(health.Temperature) * 0.15
	errorScore := calculateErrorScore(health.MediaErrors) * 0.10
	warningScore := float64(1-health.CriticalWarning/255) * 100 * 0.05

	score = lifeScore + spareScore + tempScore + errorScore + warningScore

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// calculateTempScore 计算温度评分
func calculateTempScore(temp int) float64 {
	if temp <= 50 {
		return 100
	}
	if temp <= 60 {
		return 100 - float64(temp-50)*2
	}
	if temp <= 70 {
		return 80 - float64(temp-60)*4
	}
	if temp <= 80 {
		return 40 - float64(temp-70)*4
	}
	return 0
}

// calculateErrorScore 计算错误评分
func calculateErrorScore(errors uint64) float64 {
	if errors == 0 {
		return 100
	}
	if errors <= 5 {
		return 100 - float64(errors)*10
	}
	if errors <= 10 {
		return 50 - float64(errors-5)*5
	}
	return 25 - float64(errors-10)*2.5
}

// PredictWithContext 带上下文的预测
func (p *LifePredictor) PredictWithContext(ctx context.Context, device string) *PredictedLife {
	select {
	case <-ctx.Done():
		return nil
	default:
		return p.Predict(device)
	}
}

// DetectWithContext 带上下文的异常检测
func (d *AnomalyDetector) DetectWithContext(ctx context.Context, device string, health *HealthStatus) []*Anomaly {
	select {
	case <-ctx.Done():
		return nil
	default:
		return d.Detect(device, health)
	}
}

// EvaluateWithContext 带上下文的健康评估
func (e *HealthEvaluator) EvaluateWithContext(ctx context.Context, device string, health *HealthStatus, history []*HistoryPoint) *ComprehensiveHealth {
	select {
	case <-ctx.Done():
		return nil
	default:
		return e.Evaluate(device, health, history)
	}
}
