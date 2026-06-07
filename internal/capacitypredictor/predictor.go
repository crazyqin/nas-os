// Package capacitypredictor 提供智能存储容量预测功能
// 基于历史使用趋势预测存储容量耗尽时间，支持多维度分析
package capacitypredictor

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ==================== 数据类型 ====================

// UsageSnapshot 使用量快照
type UsageSnapshot struct {
	Timestamp  time.Time `json:"timestamp"`
	TotalBytes int64     `json:"totalBytes"`
	UsedBytes  int64     `json:"usedBytes"`
	FreeBytes  int64     `json:"freeBytes"`
	InodeTotal int64     `json:"inodeTotal"`
	InodeUsed  int64     `json:"inodeUsed"`
}

// PredictionResult 预测结果
type PredictionResult struct {
	Dataset         string      `json:"dataset"`
	CurrentUsage    float64     `json:"currentUsage"`    // 当前使用率 (0-100)
	GrowthRateDaily float64     `json:"growthRateDaily"` // 每日增长 bytes
	GrowthRatePct   float64     `json:"growthRatePct"`   // 每日增长百分比
	DaysRemaining   int         `json:"daysRemaining"`   // 预计剩余天数
	FullDate        *time.Time  `json:"fullDate"`        // 预计满盘日期
	Confidence      float64     `json:"confidence"`      // 预测置信度 (0-100)
	Trend           GrowthTrend `json:"trend"`           // 增长趋势
	Recommendations []string    `json:"recommendations"` // 建议
	GeneratedAt     time.Time   `json:"generatedAt"`
}

// GrowthTrend 增长趋势
type GrowthTrend string

const (
	TrendStable    GrowthTrend = "stable"    // 稳定
	TrendGrowing   GrowthTrend = "growing"   // 增长
	TrendSlowing   GrowthTrend = "slowing"   // 减速
	TrendSpiking   GrowthTrend = "spiking"   // 激增
	TrendDeclining GrowthTrend = "declining" // 下降
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
)

// CapacityAlert 容量告警
type CapacityAlert struct {
	Level     AlertLevel `json:"level"`
	Dataset   string     `json:"dataset"`
	Message   string     `json:"message"`
	Threshold float64    `json:"threshold"`
	Current   float64    `json:"current"`
	CreatedAt time.Time  `json:"createdAt"`
}

// CapacityReport 容量报告
type CapacityReport struct {
	GeneratedAt  time.Time           `json:"generatedAt"`
	Datasets     []*PredictionResult `json:"datasets"`
	Alerts       []*CapacityAlert    `json:"alerts"`
	TotalUsed    int64               `json:"totalUsed"`
	TotalFree    int64               `json:"totalFree"`
	OverallUsage float64             `json:"overallUsage"`
}

// ==================== 预测器 ====================

// Predictor 容量预测器
type Predictor struct {
	mu sync.RWMutex

	// 历史数据
	history map[string][]*UsageSnapshot // dataset -> snapshots

	// 告警阈值
	alertThresholds []float64 // [50, 70, 80, 90, 95]

	// 配置
	maxHistoryDays int
	minSamples     int
}

// NewPredictor 创建容量预测器
func NewPredictor() *Predictor {
	return &Predictor{
		history:         make(map[string][]*UsageSnapshot),
		alertThresholds: []float64{50, 70, 80, 90, 95},
		maxHistoryDays:  365,
		minSamples:      3,
	}
}

// ==================== 数据采集 ====================

// RecordSnapshot 记录使用量快照
func (p *Predictor) RecordSnapshot(dataset string, snap *UsageSnapshot) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if snap.Timestamp.IsZero() {
		snap.Timestamp = time.Now()
	}

	p.history[dataset] = append(p.history[dataset], snap)

	// 清理过期数据
	cutoff := time.Now().AddDate(0, 0, -p.maxHistoryDays)
	var filtered []*UsageSnapshot
	for _, s := range p.history[dataset] {
		if s.Timestamp.After(cutoff) {
			filtered = append(filtered, s)
		}
	}
	p.history[dataset] = filtered

	log.Printf("[容量预测] 记录快照: %s, 使用率: %.1f%%", dataset, float64(snap.UsedBytes)/float64(snap.TotalBytes)*100)
}

// ==================== 预测分析 ====================

// Predict 预测指定数据集的容量
func (p *Predictor) Predict(dataset string) (*PredictionResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snapshots, exists := p.history[dataset]
	if !exists || len(snapshots) < p.minSamples {
		return nil, fmt.Errorf("数据集 %s 样本不足，需要至少 %d 个", dataset, p.minSamples)
	}

	// 计算当前使用率
	latest := snapshots[len(snapshots)-1]
	currentUsage := float64(latest.UsedBytes) / float64(latest.TotalBytes) * 100

	// 线性回归计算增长趋势
	growthRate, confidence := p.calculateGrowthRate(snapshots)
	growthRateDaily := growthRate * 86400 // 转换为每日
	growthRatePct := 0.0
	if latest.TotalBytes > 0 {
		growthRatePct = growthRateDaily / float64(latest.TotalBytes) * 100
	}

	// 判断趋势
	trend := p.analyzeTrend(snapshots)

	// 计算剩余天数
	daysRemaining := -1
	var fullDate *time.Time
	if growthRateDaily > 0 {
		freeBytes := float64(latest.FreeBytes)
		days := int(freeBytes / growthRateDaily)
		if days > 0 {
			daysRemaining = days
			fd := time.Now().AddDate(0, 0, days)
			fullDate = &fd
		}
	}

	// 生成建议
	recommendations := p.generateRecommendations(currentUsage, trend, daysRemaining)

	result := &PredictionResult{
		Dataset:         dataset,
		CurrentUsage:    currentUsage,
		GrowthRateDaily: growthRateDaily,
		GrowthRatePct:   growthRatePct,
		DaysRemaining:   daysRemaining,
		FullDate:        fullDate,
		Confidence:      confidence,
		Trend:           trend,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
	}

	log.Printf("[容量预测] 预测完成: %s, 当前: %.1f%%, 趋势: %s, 剩余: %d天",
		dataset, currentUsage, trend, daysRemaining)

	return result, nil
}

// calculateGrowthRate 计算增长速率（线性回归）
func (p *Predictor) calculateGrowthRate(snapshots []*UsageSnapshot) (float64, float64) {
	n := len(snapshots)
	if n < 2 {
		return 0, 0
	}

	// 转换为相对时间（秒）
	baseTime := snapshots[0].Timestamp
	var xSum, ySum, xySum, x2Sum float64
	for _, s := range snapshots {
		x := s.Timestamp.Sub(baseTime).Seconds()
		y := float64(s.UsedBytes)
		xSum += x
		ySum += y
		xySum += x * y
		x2Sum += x * x
	}

	nf := float64(n)
	// 线性回归斜率
	denominator := nf*x2Sum - xSum*xSum
	if denominator == 0 {
		return 0, 0
	}
	slope := (nf*xySum - xSum*ySum) / denominator

	// 计算 R² 置信度
	meanY := ySum / nf
	var ssRes, ssTot float64
	for _, s := range snapshots {
		x := s.Timestamp.Sub(baseTime).Seconds()
		y := float64(s.UsedBytes)
		predicted := slope*x + (ySum-slope*xSum)/nf
		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}

	confidence := 0.0
	if ssTot > 0 {
		confidence = (1 - ssRes/ssTot) * 100
		if confidence < 0 {
			confidence = 0
		}
	}

	return slope, confidence
}

// analyzeTrend 分析增长趋势
func (p *Predictor) analyzeTrend(snapshots []*UsageSnapshot) GrowthTrend {
	n := len(snapshots)
	if n < 3 {
		return TrendStable
	}

	// 比较最近 1/3 和前 2/3 的增长速率
	mid := n * 2 / 3
	earlyRate := p.segmentGrowthRate(snapshots[:mid])
	recentRate := p.segmentGrowthRate(snapshots[mid:])

	if recentRate <= 0 && earlyRate <= 0 {
		return TrendStable
	}

	ratio := 0.0
	if earlyRate != 0 {
		ratio = recentRate / earlyRate
	}

	switch {
	case ratio > 2.0:
		return TrendSpiking
	case ratio > 1.2:
		return TrendGrowing
	case ratio < 0.5:
		return TrendDeclining
	case ratio < 0.8:
		return TrendSlowing
	default:
		return TrendStable
	}
}

// segmentGrowthRate 计算片段增长速率
func (p *Predictor) segmentGrowthRate(snapshots []*UsageSnapshot) float64 {
	n := len(snapshots)
	if n < 2 {
		return 0
	}
	duration := snapshots[n-1].Timestamp.Sub(snapshots[0].Timestamp).Seconds()
	if duration <= 0 {
		return 0
	}
	growth := float64(snapshots[n-1].UsedBytes - snapshots[0].UsedBytes)
	return growth / duration
}

// generateRecommendations 生成建议
func (p *Predictor) generateRecommendations(usage float64, trend GrowthTrend, daysRemaining int) []string {
	var recs []string

	if usage >= 95 {
		recs = append(recs, "紧急：存储使用率超过95%，请立即清理或扩容")
	} else if usage >= 90 {
		recs = append(recs, "警告：存储使用率超过90%，建议尽快清理")
	} else if usage >= 80 {
		recs = append(recs, "注意：存储使用率超过80%，请关注增长趋势")
	}

	if daysRemaining > 0 && daysRemaining <= 30 {
		recs = append(recs, fmt.Sprintf("预计 %d 天后存储将满，建议提前扩容", daysRemaining))
	}

	switch trend {
	case TrendSpiking:
		recs = append(recs, "检测到使用量激增，建议检查是否有异常写入")
	case TrendGrowing:
		if daysRemaining > 0 && daysRemaining <= 90 {
			recs = append(recs, "存储持续增长，建议规划扩容或启用数据分层")
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "存储状态健康")
	}

	return recs
}

// ==================== 告警管理 ====================

// CheckAlerts 检查容量告警
func (p *Predictor) CheckAlerts(dataset string) []*CapacityAlert {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snapshots, exists := p.history[dataset]
	if !exists || len(snapshots) == 0 {
		return nil
	}

	latest := snapshots[len(snapshots)-1]
	usage := float64(latest.UsedBytes) / float64(latest.TotalBytes) * 100

	var alerts []*CapacityAlert
	for _, threshold := range p.alertThresholds {
		if usage >= threshold {
			level := AlertInfo
			switch {
			case threshold >= 90:
				level = AlertCritical
			case threshold >= 70:
				level = AlertWarning
			}
			alerts = append(alerts, &CapacityAlert{
				Level:     level,
				Dataset:   dataset,
				Message:   fmt.Sprintf("存储使用率达到 %.0f%% (阈值: %.0f%%)", usage, threshold),
				Threshold: threshold,
				Current:   usage,
				CreatedAt: time.Now(),
			})
		}
	}

	return alerts
}

// ==================== 报告生成 ====================

// GenerateReport 生成容量报告
func (p *Predictor) GenerateReport() *CapacityReport {
	p.mu.RLock()
	datasets := make([]string, 0, len(p.history))
	for k := range p.history {
		datasets = append(datasets, k)
	}
	p.mu.RUnlock()

	report := &CapacityReport{
		GeneratedAt: time.Now(),
	}

	var totalUsed, totalFree int64

	for _, ds := range datasets {
		pred, err := p.Predict(ds)
		if err != nil {
			continue
		}
		report.Datasets = append(report.Datasets, pred)

		alerts := p.CheckAlerts(ds)
		report.Alerts = append(report.Alerts, alerts...)

		p.mu.RLock()
		snaps := p.history[ds]
		if len(snaps) > 0 {
			latest := snaps[len(snaps)-1]
			totalUsed += latest.UsedBytes
			totalFree += latest.FreeBytes
		}
		p.mu.RUnlock()
	}

	report.TotalUsed = totalUsed
	report.TotalFree = totalFree
	total := totalUsed + totalFree
	if total > 0 {
		report.OverallUsage = float64(totalUsed) / float64(total) * 100
	}

	log.Printf("[容量预测] 生成报告, 数据集: %d, 告警: %d", len(report.Datasets), len(report.Alerts))
	return report
}

// ==================== 配置 ====================

// SetAlertThresholds 设置告警阈值
func (p *Predictor) SetAlertThresholds(thresholds []float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.alertThresholds = thresholds
}

// GetHistory 获取历史数据
func (p *Predictor) GetHistory(dataset string, days int) []*UsageSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snapshots, exists := p.history[dataset]
	if !exists {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var result []*UsageSnapshot
	for _, s := range snapshots {
		if s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}
	return result
}

// GetDatasets 获取所有数据集列表
func (p *Predictor) GetDatasets() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	datasets := make([]string, 0, len(p.history))
	for k := range p.history {
		datasets = append(datasets, k)
	}
	return datasets
}

// CalculateLinearForecast 线性预测（用于自定义时间范围）
func (p *Predictor) CalculateLinearForecast(dataset string, daysAhead int) (float64, error) {
	pred, err := p.Predict(dataset)
	if err != nil {
		return 0, err
	}

	p.mu.RLock()
	snaps := p.history[dataset]
	latest := snaps[len(snaps)-1]
	p.mu.RUnlock()

	futureUsed := float64(latest.UsedBytes) + pred.GrowthRateDaily*float64(daysAhead)
	futureUsage := futureUsed / float64(latest.TotalBytes) * 100
	if futureUsage > 100 {
		futureUsage = 100
	}
	if futureUsage < 0 {
		futureUsage = 0
	}
	return futureUsage, nil
}

// EstimateRequiredCapacity 估算扩容需求
func (p *Predictor) EstimateRequiredCapacity(dataset string, targetDays int) (int64, error) {
	pred, err := p.Predict(dataset)
	if err != nil {
		return 0, err
	}

	if pred.GrowthRateDaily <= 0 {
		return 0, nil
	}

	required := int64(pred.GrowthRateDaily * float64(targetDays))
	return required, nil
}

// GetPeakUsage 获取峰值使用时间
func (p *Predictor) GetPeakUsage(dataset string) *UsageSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()

	snaps, exists := p.history[dataset]
	if !exists || len(snaps) == 0 {
		return nil
	}

	peak := snaps[0]
	for _, s := range snaps[1:] {
		if s.UsedBytes > peak.UsedBytes {
			peak = s
		}
	}
	return peak
}

// EstimateDaysUntilFull 计算满盘天数（简化接口）
func EstimateDaysUntilFull(currentFree int64, dailyGrowth float64) int {
	if dailyGrowth <= 0 {
		return math.MaxInt32
	}
	return int(float64(currentFree) / dailyGrowth)
}
