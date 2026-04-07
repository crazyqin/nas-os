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
// 寿命预测模块
// ============================================================================

// LifePredictor NVMe寿命预测器
type LifePredictor struct {
	config      *PredictionConfig
	history     map[string][]*HistoryPoint
	models      map[string]*PredictionModel
	mu          sync.RWMutex
	minSamples  int
	optimalSamples int
}

// PredictionConfig 预测配置
type PredictionConfig struct {
	Enabled          bool    `json:"enabled"`
	MinSamples       int     `json:"minSamples"`       // 最小样本数 (默认10)
	OptimalSamples   int     `json:"optimalSamples"`   // 最优样本数 (默认1440)
	MaxConfidence    float64 `json:"maxConfidence"`    // 最大置信度 (默认0.85)
	PredictionWindow int     `json:"predictionWindow"` // 预测窗口天数
}

// DefaultPredictionConfig 默认预测配置
func DefaultPredictionConfig() *PredictionConfig {
	return &PredictionConfig{
		Enabled:          true,
		MinSamples:       10,
		OptimalSamples:   1440, // 30天 × 48次/天
		MaxConfidence:    0.85,
		PredictionWindow: 365,
	}
}

// HistoryPoint 历史数据点
type HistoryPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	PercentUsed     float64   `json:"percentUsed"`     // 寿命已用百分比
	AvailableSpare  float64   `json:"availableSpare"`  // 可用备用空间
	Temperature     int       `json:"temperature"`     // 温度
	TotalWrites     uint64    `json:"totalWrites"`     // 总写入量(字节)
	MediaErrors     uint64    `json:"mediaErrors"`     // 媒体错误
	HealthScore     float64   `json:"healthScore"`     // 健康评分
}

// PredictionModel 预测模型
type PredictionModel struct {
	Device           string    `json:"device"`
	WriteRatePerDay  uint64    `json:"writeRatePerDay"`  // 日均写入量
	LifeDecayRate    float64   `json:"lifeDecayRate"`    // 寿命衰减率(%/天)
	LastCalculated   time.Time `json:"lastCalculated"`
	SampleCount      int       `json:"sampleCount"`
}

// NewLifePredictor 创建寿命预测器
func NewLifePredictor(config *PredictionConfig) *LifePredictor {
	if config == nil {
		config = DefaultPredictionConfig()
	}
	return &LifePredictor{
		config:      config,
		history:     make(map[string][]*HistoryPoint),
		models:      make(map[string]*PredictionModel),
		minSamples:  config.MinSamples,
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

	// 计算健康评分
	point.HealthScore = CalculateHealthScore(health)

	history := p.history[device]
	history = append(history, point)

	// 限制历史数据量
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
			RemainingDays:    -1, // 未知
			Confidence:       0,
			Method:           "insufficient_data",
		}
	}

	// 计算写入速率
	writeRate := p.calculateWriteRate(history)

	// 计算寿命衰减速率
	lifeDecayRate := p.calculateLifeDecayRate(history)

	// 获取当前寿命状态
	lastPoint := history[len(history)-1]
	remainingPercent := 100.0 - lastPoint.PercentUsed

	// 预测剩余天数
	var remainingDays int
	if lifeDecayRate > 0 {
		remainingDaysFloat := remainingPercent / lifeDecayRate
		// 安全转换，防止溢出
		if remainingDaysFloat > 36500 {
			remainingDays = 36500 // 最大100年
		} else if remainingDaysFloat < 0 {
			remainingDays = 0
		} else {
			remainingDays = int(remainingDaysFloat)
		}
	} else if writeRate > 0 {
		// 如果无法计算衰减率，使用写入速率估算
		// 假设TBW为总写入量的某个倍数
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
		remainingDays = -1 // 无法预测
	}

	// 计算置信度
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

	// 计算总写入增量
	var totalWriteIncrease uint64
	for i := 1; i < len(history); i++ {
		if history[i].TotalWrites > history[i-1].TotalWrites {
			totalWriteIncrease += history[i].TotalWrites - history[i-1].TotalWrites
		}
	}

	// 计算时间跨度
	duration := history[len(history)-1].Timestamp.Sub(history[0].Timestamp)
	days := duration.Hours() / 24

	if days <= 0 {
		return 0
	}

	// 日均写入量 - 安全转换
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

	// 计算寿命消耗增量
	var totalLifeIncrease float64
	for i := 1; i < len(history); i++ {
		if history[i].PercentUsed > history[i-1].PercentUsed {
			totalLifeIncrease += history[i].PercentUsed - history[i-1].PercentUsed
		}
	}

	// 计算时间跨度
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

// AnomalyDetector 异常检测器
type AnomalyDetector struct {
	config       *AnomalyConfig
	baselines    map[string]*Baseline
	anomalies    map[string][]*Anomaly
	mu           sync.RWMutex
}

// AnomalyConfig 异常检测配置
type AnomalyConfig struct {
	WriteSpikeThreshold  float64 `json:"writeSpikeThreshold"`  // 写入突发阈值(倍数)
	TempSpikeThreshold   float64 `json:"tempSpikeThreshold"`   // 温度突发阈值(度)
	LifeJumpThreshold    float64 `json:"lifeJumpThreshold"`    // 寿命跳跃阈值(%)
	AnomalyWindow        int     `json:"anomalyWindow"`        // 异常检测窗口(分钟)
}

// DefaultAnomalyConfig 默认异常检测配置
func DefaultAnomalyConfig() *AnomalyConfig {
	return &AnomalyConfig{
		WriteSpikeThreshold:  3.0,  // 3倍基线
		TempSpikeThreshold:   10.0, // 10度跳跃
		LifeJumpThreshold:    1.0,  // 1%跳跃
		AnomalyWindow:        5,    // 5分钟窗口
	}
}

// Baseline 基线数据
type Baseline struct {
	Device          string    `json:"device"`
	AvgWriteRate    uint64    `json:"avgWriteRate"`    // 平均写入速率
	AvgTemperature  int       `json:"avgTemperature"`  // 平均温度
	AvgLifeDecay    float64   `json:"avgLifeDecay"`    // 平均寿命衰减
	LastUpdate      time.Time `json:"lastUpdate"`
}

// Anomaly 异常事件
type Anomaly struct {
	Device      string        `json:"device"`
	Type        AnomalyType   `json:"type"`
	Severity    AnomalyLevel  `json:"severity"`
	Value       interface{}   `json:"value"`
	Baseline    interface{}   `json:"baseline"`
	Description string        `json:"description"`
	Timestamp   time.Time     `json:"timestamp"`
}

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalyWriteSpike  AnomalyType = "write_spike"  // 写入突发
	AnomalyTempSpike   AnomalyType = "temp_spike"   // 温度突发
	AnomalyLifeJump    AnomalyType = "life_jump"    // 寿命跳跃
	AnomalyMediaError  AnomalyType = "media_error"  // 媒体错误
	AnomalySpareLow    AnomalyType = "spare_low"    // 备用空间低
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

	// 计算平均写入速率
	var totalWrites uint64
	for _, p := range history {
		totalWrites += p.TotalWrites
	}
	avgWrites := totalWrites / uint64(len(history))

	// 计算平均温度
	var totalTemp int
	for _, p := range history {
		totalTemp += p.Temperature
	}
	avgTemp := totalTemp / len(history)

	// 计算平均寿命衰减
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

	// 存储异常记录
	d.mu.Lock()
	deviceAnomalies := d.anomalies[device]
	deviceAnomalies = append(deviceAnomalies, anomalies...)
	// 保留最近100条
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

// HealthEvaluator 健康评估器
type HealthEvaluator struct {
	predictor    *LifePredictor
	detector     *AnomalyDetector
	config       *EvaluationConfig
}

// EvaluationConfig 评估配置
type EvaluationConfig struct {
	Thresholds   *ThresholdConfig   `json:"thresholds"`
	Prediction   *PredictionConfig  `json:"prediction"`
	Anomaly      *AnomalyConfig     `json:"anomaly"`
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	LifespanWarning   float64 `json:"lifespanWarning"`   // 寿命警告阈值
	LifespanCritical  float64 `json:"lifespanCritical"`  // 寿命严重阈值
	LifespanEmergency float64 `json:"lifespanEmergency"` // 寿命紧急阈值
	TemperatureWarning  int    `json:"temperatureWarning"`  // 温度警告阈值
	TemperatureCritical int    `json:"temperatureCritical"` // 温度严重阈值
	SpareWarning      float64 `json:"spareWarning"`      // 备用空间警告阈值
	SpareCritical     float64 `json:"spareCritical"`     // 备用空间严重阈值
	DaysWarning       int     `json:"daysWarning"`       // 剩余天数警告阈值
	DaysCritical      int     `json:"daysCritical"`      // 剩余天数严重阈值
}

// DefaultThresholdConfig 默认阈值配置
func DefaultThresholdConfig() *ThresholdConfig {
	return &ThresholdConfig{
		LifespanWarning:    80,
		LifespanCritical:   90,
		LifespanEmergency:  95,
		TemperatureWarning:  60,
		TemperatureCritical: 70,
		SpareWarning:       20,
		SpareCritical:      10,
		DaysWarning:        180,
		DaysCritical:       90,
	}
}

// ComprehensiveHealth 全面健康评估
type ComprehensiveHealth struct {
	Device          string           `json:"device"`
	HealthScore     float64          `json:"healthScore"`     // 综合健康评分
	Status          HealthStatusType `json:"status"`          // 状态
	AlertLevel      AlertLevel       `json:"alertLevel"`      // 告警级别
	AlertMessage    string           `json:"alertMessage"`    // 告警消息
	PredictedLife   *PredictedLife   `json:"predictedLife"`   // 寿命预测
	Anomalies      []*Anomaly       `json:"anomalies"`       // 异常事件
	RiskFactors     []string         `json:"riskFactors"`     // 风险因素
	Recommendations []string         `json:"recommendations"` // 建议
	Timestamp       time.Time        `json:"timestamp"`
}

// HealthStatusType 健康状态类型
type HealthStatusType string

const (
	HealthStatusHealthy   HealthStatusType = "healthy"
	HealthStatusWarning   HealthStatusType = "warning"
	HealthStatusCritical  HealthStatusType = "critical"
	HealthStatusEmergency HealthStatusType = "emergency"
	HealthStatusUnknown   HealthStatusType = "unknown"
)

// NewHealthEvaluator 创建健康评估器
func NewHealthEvaluator(config *EvaluationConfig) *HealthEvaluator {
	if config == nil {
		config = &EvaluationConfig{
			Thresholds:   DefaultThresholdConfig(),
			Prediction:   DefaultPredictionConfig(),
			Anomaly:      DefaultAnomalyConfig(),
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
	// 添加历史数据
	for _, p := range history {
		e.predictor.AddHistoryPoint(device, health)
	}

	// 更新基线
	e.detector.UpdateBaseline(device, history)

	// 计算健康评分
	healthScore := CalculateHealthScore(health)

	// 执行寿命预测
	predictedLife := e.predictor.Predict(device)

	// 检测异常
	anomalies := e.detector.Detect(device, health)

	// 评估告警级别
	alertLevel, alertMessage := e.evaluateAlertLevel(health, predictedLife, anomalies)

	// 确定状态
	status := e.determineStatus(alertLevel, healthScore)

	// 识别风险因素
	riskFactors := e.identifyRiskFactors(health, predictedLife, anomalies)

	// 生成建议
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

	// 检查紧急条件
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

	// 检查严重条件
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

	// 检查警告条件
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

	// 检查异常事件
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

	// 1. 寿命因子 (权重40%)
	lifeScore := (100.0 - health.PercentUsed) * 0.4

	// 2. 备用空间因子 (权重30%)
	spareScore := health.AvailableSpare * 0.3

	// 3. 温度因子 (权重15%)
	tempScore := calculateTempScore(health.Temperature) * 0.15

	// 4. 媒体错误因子 (权重10%)
	errorScore := calculateErrorScore(health.MediaErrors) * 0.10

	// 5. 警告标志因子 (权重5%)
	warningScore := float64(1-health.CriticalWarning/255) * 100 * 0.05

	score = lifeScore + spareScore + tempScore + errorScore + warningScore

	// 确保评分在0-100范围内
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
		return 100 - float64(temp-50)*2 // 50-60: 100-80
	}
	if temp <= 70 {
		return 80 - float64(temp-60)*4 // 60-70: 80-40
	}
	if temp <= 80 {
		return 40 - float64(temp-70)*4 // 70-80: 40-0
	}
	return 0
}

// calculateErrorScore 计算错误评分
func calculateErrorScore(errors uint64) float64 {
	if errors == 0 {
		return 100
	}
	if errors <= 5 {
		return 100 - float64(errors)*10 // 0-5: 100-50
	}
	if errors <= 10 {
		return 50 - float64(errors-5)*5 // 5-10: 50-25
	}
	return 25 - float64(errors-10)*2.5 // 10+: 递减
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