// Package resourcepredict 提供系统资源预测告警功能
// 基于历史数据趋势预测资源耗尽时间，并自动触发告警
// 支持预测：磁盘空间、内存使用、CPU负载、网络带宽、inode使用
package resourcepredict

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ResourceType 资源类型
type ResourceType string

const (
	ResourceDisk    ResourceType = "disk"
	ResourceMemory  ResourceType = "memory"
	ResourceCPU     ResourceType = "cpu"
	ResourceNetwork ResourceType = "network"
	ResourceInode   ResourceType = "inode"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
	AlertUrgent   AlertLevel = "urgent"
)

// Thresholds 预测告警阈值
type Thresholds struct {
	WarningDays   int     `json:"warningDays"`   // 跄尽前N天告警(Warning)
	CriticalDays  int     `json:"criticalDays"`  // 耗尽前N天告警(Critical)
	UrgentDays    int     `json:"urgentDays"`    // 耗尽前N天告警(Urgent)
	MinR2         float64 `json:"minR2"`         // 最低R²拟合度(低于此值不做预测)
	MinDataPoints int     `json:"minDataPoints"` // 最少数据点
}

// DefaultThresholds 默认阈值
func DefaultThresholds() Thresholds {
	return Thresholds{
		WarningDays:   30,
		CriticalDays:  14,
		UrgentDays:    7,
		MinR2:         0.5,
		MinDataPoints: 10,
	}
}

// DataPoint 资源使用数据点
type DataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"` // 使用率 (0-100) 或绝对值
	Total     float64   `json:"total"` // 总量(如磁盘总容量)
}

// ResourceMetric 资源指标
type ResourceMetric struct {
	Type         ResourceType `json:"type"`
	Name         string       `json:"name"` // 如 "/dev/sda1", "system-memory"
	Unit         string       `json:"unit"` // "bytes", "percent", "count"
	Points       []DataPoint  `json:"points"`
	CurrentValue float64      `json:"currentValue"`
	MaxValue     float64      `json:"maxValue"`
}

// Prediction 预测结果
type Prediction struct {
	ResourceType  ResourceType `json:"resourceType"`
	ResourceName  string       `json:"resourceName"`
	CurrentValue  float64      `json:"currentValue"`
	MaxValue      float64      `json:"maxValue"`
	UsagePercent  float64      `json:"usagePercent"`
	TrendSlope    float64      `json:"trendSlope"`    // 每天变化率
	TrendR2       float64      `json:"trendR2"`       // R²拟合度
	DaysUntilFull float64      `json:"daysUntilFull"` // 预计耗尽天数 (-1=不适用)
	PredictedDate string       `json:"predictedDate"` // 预计耗尽日期
	AlertLevel    AlertLevel   `json:"alertLevel"`
	AlertMessage  string       `json:"alertMessage"`
	Confidence    float64      `json:"confidence"` // 预测置信度 0-1
	IsIncreasing  bool         `json:"isIncreasing"`
}

// PredictionReport 预测报告
type PredictionReport struct {
	Timestamp   time.Time    `json:"timestamp"`
	Predictions []Prediction `json:"predictions"`
	Summary     string       `json:"summary"`
	MaxAlert    AlertLevel   `json:"maxAlert"`
}

// PredictorConfig 预测器配置
type PredictorConfig struct {
	Thresholds       Thresholds    `json:"thresholds"`
	RetentionDays    int           `json:"retentionDays"`    // 数据保留天数
	SamplingInterval time.Duration `json:"samplingInterval"` // 采样间隔
	MaxDataPoints    int           `json:"maxDataPoints"`    // 每资源最大数据点数
}

// DefaultPredictorConfig 默认预测器配置
func DefaultPredictorConfig() PredictorConfig {
	return PredictorConfig{
		Thresholds:       DefaultThresholds(),
		RetentionDays:    90,
		SamplingInterval: time.Hour,
		MaxDataPoints:    2160, // 90天 × 24次/天
	}
}

// ResourcePredictor 资源预测器
type ResourcePredictor struct {
	config      PredictorConfig
	metrics     map[ResourceType]*ResourceMetric
	predictions []Prediction
	mu          sync.RWMutex
	stopCh      chan struct{}
	running     bool

	// 回调
	onAlert func(prediction Prediction)
}

// NewResourcePredictor 创建资源预测器
func NewResourcePredictor(config PredictorConfig) *ResourcePredictor {
	return &ResourcePredictor{
		config:  config,
		metrics: make(map[ResourceType]*ResourceMetric),
		stopCh:  make(chan struct{}),
	}
}

// SetAlertCallback 设置告警回调
func (rp *ResourcePredictor) SetAlertCallback(fn func(prediction Prediction)) {
	rp.onAlert = fn
}

// RegisterResource 注册资源监控
func (rp *ResourcePredictor) RegisterResource(resType ResourceType, name, unit string, maxValue float64) {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	rp.metrics[resType] = &ResourceMetric{
		Type:     resType,
		Name:     name,
		Unit:     unit,
		MaxValue: maxValue,
		Points:   make([]DataPoint, 0, rp.config.MaxDataPoints),
	}
	log.Printf("[ResourcePredict] registered %s (%s), max=%.0f %s", resType, name, maxValue, unit)
}

// RecordValue 记录资源使用值
func (rp *ResourcePredictor) RecordValue(resType ResourceType, value float64) {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	metric, exists := rp.metrics[resType]
	if !exists {
		return
	}

	point := DataPoint{
		Timestamp: time.Now(),
		Value:     value,
		Total:     metric.MaxValue,
	}

	metric.Points = append(metric.Points, point)
	metric.CurrentValue = value

	// 保留最近的数据点
	if len(metric.Points) > rp.config.MaxDataPoints {
		metric.Points = metric.Points[len(metric.Points)-rp.config.MaxDataPoints:]
	}
}

// Start 启动预测循环
func (rp *ResourcePredictor) Start() {
	rp.mu.Lock()
	if rp.running {
		rp.mu.Unlock()
		return
	}
	rp.running = true
	rp.mu.Unlock()

	go rp.predictLoop()
	log.Printf("[ResourcePredict] predictor started, interval=%v", rp.config.SamplingInterval)
}

// Stop 停止预测
func (rp *ResourcePredictor) Stop() {
	rp.mu.Lock()
	defer rp.mu.Unlock()
	if !rp.running {
		return
	}
	close(rp.stopCh)
	rp.running = false
	log.Printf("[ResourcePredict] predictor stopped")
}

// PredictNow 立即执行预测
func (rp *ResourcePredictor) PredictNow() PredictionReport {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	report := PredictionReport{
		Timestamp:   time.Now(),
		Predictions: make([]Prediction, 0),
		MaxAlert:    AlertInfo,
	}

	for _, metric := range rp.metrics {
		prediction := rp.predictResource(metric)
		report.Predictions = append(report.Predictions, prediction)

		// 更新最高告警级别
		if alertPriority(prediction.AlertLevel) > alertPriority(report.MaxAlert) {
			report.MaxAlert = prediction.AlertLevel
		}

		// 触发回调
		if prediction.AlertLevel != AlertInfo && rp.onAlert != nil {
			rp.onAlert(prediction)
		}
	}

	report.Summary = rp.generateSummary(report.Predictions)
	rp.predictions = report.Predictions
	return report
}

// GetLatest 获取最新预测结果
func (rp *ResourcePredictor) GetLatest() []Prediction {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	result := make([]Prediction, len(rp.predictions))
	copy(result, rp.predictions)
	return result
}

// predictResource 预测单个资源
func (rp *ResourcePredictor) predictResource(metric *ResourceMetric) Prediction {
	prediction := Prediction{
		ResourceType:  metric.Type,
		ResourceName:  metric.Name,
		CurrentValue:  metric.CurrentValue,
		MaxValue:      metric.MaxValue,
		DaysUntilFull: -1,
		AlertLevel:    AlertInfo,
	}

	if metric.MaxValue > 0 {
		prediction.UsagePercent = (metric.CurrentValue / metric.MaxValue) * 100
	}

	// 线性回归预测趋势
	if len(metric.Points) < rp.config.Thresholds.MinDataPoints {
		prediction.AlertMessage = fmt.Sprintf("数据点不足(%d/%d)，无法预测", len(metric.Points), rp.config.Thresholds.MinDataPoints)
		return prediction
	}

	slope, r2 := rp.linearRegression(metric.Points)
	prediction.TrendSlope = slope
	prediction.TrendR2 = r2
	prediction.IsIncreasing = slope > 0

	// 检查拟合度
	if r2 < rp.config.Thresholds.MinR2 {
		prediction.Confidence = r2
		prediction.AlertMessage = fmt.Sprintf("趋势不明显(R²=%.2f)，无法可靠预测", r2)
		return prediction
	}

	// 计算预测置信度
	prediction.Confidence = rp.calculateConfidence(metric, slope, r2)

	// 计算预计耗尽天数
	if slope > 0 && metric.MaxValue > 0 {
		remaining := metric.MaxValue - metric.CurrentValue
		if remaining > 0 {
			daysUntilFull := remaining / (slope * 86400) // slope是每秒变化率，转换为天
			prediction.DaysUntilFull = daysUntilFull
			predictedDate := time.Now().AddDate(0, 0, int(math.Ceil(daysUntilFull)))
			prediction.PredictedDate = predictedDate.Format("2006-01-02")

			// 确定告警级别
			prediction.AlertLevel = rp.determineAlertLevel(daysUntilFull, prediction.UsagePercent)
			prediction.AlertMessage = rp.generateAlertMessage(prediction)
		}
	} else if slope < 0 {
		// 资源在减少，好消息
		prediction.AlertLevel = AlertInfo
		prediction.AlertMessage = fmt.Sprintf("资源使用呈下降趋势(%.2f%%/天)，无告警", math.Abs(slope*86400))
	}

	return prediction
}

// linearRegression 线性回归
func (rp *ResourcePredictor) linearRegression(points []DataPoint) (slope, r2 float64) {
	n := float64(len(points))
	if n < 2 {
		return 0, 0
	}

	// 将时间转换为相对秒数
	baseTime := points[0].Timestamp
	var sumX, sumY, sumXY, sumX2, sumY2 float64

	for _, p := range points {
		x := p.Timestamp.Sub(baseTime).Seconds()
		y := p.Value
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, 0
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n

	// 计算R²
	meanY := sumY / n
	var ssTot, ssRes float64
	for _, p := range points {
		x := p.Timestamp.Sub(baseTime).Seconds()
		y := p.Value
		predicted := slope*x + intercept
		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}

	if ssTot > 0 {
		r2 = 1 - ssRes/ssTot
	}

	return slope, r2
}

// calculateConfidence 计算预测置信度
func (rp *ResourcePredictor) calculateConfidence(metric *ResourceMetric, slope, r2 float64) float64 {
	// 基础置信度来自R²
	confidence := r2

	// 数据点越多，置信度越高
	dataPointBonus := math.Min(0.1, float64(len(metric.Points))/float64(rp.config.MaxDataPoints)*0.1)
	confidence += dataPointBonus

	// 趋势稳定性：最近数据点与整体趋势的一致性
	if len(metric.Points) >= 20 {
		recentPoints := metric.Points[len(metric.Points)-10:]
		recentSlope, _ := rp.linearRegression(recentPoints)
		if slope != 0 {
			consistency := 1 - math.Abs(recentSlope-slope)/math.Abs(slope)
			if consistency > 0 {
				confidence *= (0.7 + 0.3*consistency)
			}
		}
	}

	return math.Min(1.0, math.Max(0, confidence))
}

// determineAlertLevel 确定告警级别
func (rp *ResourcePredictor) determineAlertLevel(daysUntilFull, usagePercent float64) AlertLevel {
	thresholds := rp.config.Thresholds

	// 使用率超过95%直接紧急告警
	if usagePercent >= 95 {
		return AlertUrgent
	}

	switch {
	case daysUntilFull <= float64(thresholds.UrgentDays):
		return AlertUrgent
	case daysUntilFull <= float64(thresholds.CriticalDays):
		return AlertCritical
	case daysUntilFull <= float64(thresholds.WarningDays):
		return AlertWarning
	default:
		return AlertInfo
	}
}

// generateAlertMessage 生成告警消息
func (rp *ResourcePredictor) generateAlertMessage(p Prediction) string {
	resourceName := string(p.ResourceType)
	if p.ResourceName != "" {
		resourceName = p.ResourceName
	}

	switch p.AlertLevel {
	case AlertUrgent:
		return fmt.Sprintf("⚠️ 紧急：%s 将在 %.0f 天内耗尽(%s)，当前使用率 %.1f%%",
			resourceName, p.DaysUntilFull, p.PredictedDate, p.UsagePercent)
	case AlertCritical:
		return fmt.Sprintf("🔴 严重：%s 预计 %.0f 天后耗尽(%s)，当前使用率 %.1f%%",
			resourceName, p.DaysUntilFull, p.PredictedDate, p.UsagePercent)
	case AlertWarning:
		return fmt.Sprintf("🟡 警告：%s 预计 %.0f 天后耗尽(%s)，当前使用率 %.1f%%",
			resourceName, p.DaysUntilFull, p.PredictedDate, p.UsagePercent)
	default:
		return fmt.Sprintf("✅ 正常：%s 使用率 %.1f%%，趋势稳定", resourceName, p.UsagePercent)
	}
}

// generateSummary 生成报告摘要
func (rp *ResourcePredictor) generateSummary(predictions []Prediction) string {
	warnings := 0
	criticals := 0
	urgents := 0

	for _, p := range predictions {
		switch p.AlertLevel {
		case AlertWarning:
			warnings++
		case AlertCritical:
			criticals++
		case AlertUrgent:
			urgents++
		}
	}

	if urgents > 0 {
		return fmt.Sprintf("🚨 %d 项资源紧急告警，%d 项严重，%d 项警告", urgents, criticals, warnings)
	}
	if criticals > 0 {
		return fmt.Sprintf("🔴 %d 项资源严重告警，%d 项警告", criticals, warnings)
	}
	if warnings > 0 {
		return fmt.Sprintf("🟡 %d 项资源告警", warnings)
	}
	return "✅ 所有资源状态正常"
}

// predictLoop 预测主循环
func (rp *ResourcePredictor) predictLoop() {
	ticker := time.NewTicker(rp.config.SamplingInterval)
	defer ticker.Stop()

	// 启动时立即预测一次
	rp.PredictNow()

	for {
		select {
		case <-rp.stopCh:
			return
		case <-ticker.C:
			rp.PredictNow()
		}
	}
}

// GetMetrics 获取所有资源指标
func (rp *ResourcePredictor) GetMetrics() map[ResourceType]*ResourceMetric {
	rp.mu.RLock()
	defer rp.mu.RUnlock()
	result := make(map[ResourceType]*ResourceMetric)
	for k, v := range rp.metrics {
		cp := *v
		result[k] = &cp
	}
	return result
}

// alertPriority 告警优先级
func alertPriority(level AlertLevel) int {
	switch level {
	case AlertUrgent:
		return 4
	case AlertCritical:
		return 3
	case AlertWarning:
		return 2
	default:
		return 1
	}
}
