// Package anomalydetect 提供 NAS 系统的 AI 异常检测引擎
// 基于统计学方法（Z-Score、滑动窗口、线性回归）实现无外部 AI 库依赖的智能异常检测
package anomalydetect

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ==================== 类型定义 ====================

// MetricType 指标类型枚举.
type MetricType string

const (
	MetricCPU    MetricType = "cpu"         // CPU 使用率
	MetricMemory MetricType = "memory"      // 内存使用率
	MetricDisk   MetricType = "disk"        // 磁盘使用率
	MetricNet    MetricType = "network"     // 网络流量（MB/s）
	MetricTemp   MetricType = "temperature" // 温度（°C）
)

// AnomalyType 异常类型枚举.
type AnomalyType string

const (
	AnomalySpike   AnomalyType = "spike"   // 突增异常
	AnomalyDrop    AnomalyType = "drop"    // 突降异常
	AnomalyTrend   AnomalyType = "trend"   // 趋势异常（持续上升/下降）
	AnomalyPattern AnomalyType = "pattern" // 模式异常（分布偏移）
)

// Severity 严重程度枚举.
type Severity string

const (
	SeverityCritical Severity = "critical" // 严重
	SeverityWarning  Severity = "warning"  // 警告
	SeverityInfo     Severity = "info"     // 信息
)

// MetricDataPoint 指标数据点.
type MetricDataPoint struct {
	Timestamp time.Time  `json:"timestamp"` // 采样时间
	Value     float64    `json:"value"`     // 指标值
	Type      MetricType `json:"type"`      // 指标类型
	Source    string     `json:"source"`    // 数据来源
}

// AnomalyResult 异常检测结果.
type AnomalyResult struct {
	Timestamp   time.Time   `json:"timestamp"`    // 检测时间
	MetricType  MetricType  `json:"metric_type"`  // 指标类型
	AnomalyType AnomalyType `json:"anomaly_type"` // 异常类型
	Value       float64     `json:"value"`        // 当前值
	Threshold   float64     `json:"threshold"`    // 触发阈值
	ZScore      float64     `json:"z_score"`      // Z-Score 值
	Severity    Severity    `json:"severity"`     // 严重程度
	Message     string      `json:"message"`      // 描述信息
}

// AdaptiveThreshold 自适应阈值.
type AdaptiveThreshold struct {
	Mean        float64 `json:"mean"`         // 均值
	StdDev      float64 `json:"std_dev"`      // 标准差
	Upper       float64 `json:"upper"`        // 上界阈值
	Lower       float64 `json:"lower"`        // 下界阈值
	SampleCount int     `json:"sample_count"` // 样本数量
}

// ==================== 滑动窗口 ====================

// SlidingWindow 环形缓冲区滑动窗口.
type SlidingWindow struct {
	mu    sync.RWMutex
	data  []float64 // 环形缓冲区
	size  int       // 窗口大小
	head  int       // 头指针（下一个写入位置）
	count int       // 当前数据量
}

// NewSlidingWindow 创建指定大小的滑动窗口.
func NewSlidingWindow(size int) *SlidingWindow {
	return &SlidingWindow{
		data: make([]float64, size),
		size: size,
	}
}

// Add 添加数据点到滑动窗口.
func (sw *SlidingWindow) Add(value float64) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.data[sw.head] = value
	sw.head = (sw.head + 1) % sw.size
	if sw.count < sw.size {
		sw.count++
	}
}

// GetData 获取窗口内所有数据（按时间顺序从旧到新）.
func (sw *SlidingWindow) GetData() []float64 {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	if sw.count == 0 {
		return nil
	}
	result := make([]float64, sw.count)
	if sw.count < sw.size {
		// 窗口未满，数据从索引0开始
		copy(result, sw.data[:sw.count])
	} else {
		// 窗口已满，从最旧的数据开始
		for i := 0; i < sw.size; i++ {
			result[i] = sw.data[(sw.head+i)%sw.size]
		}
	}
	return result
}

// Count 返回当前数据量.
func (sw *SlidingWindow) Count() int {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	return sw.count
}

// Mean 计算窗口内数据均值.
func (sw *SlidingWindow) Mean() float64 {
	return calculateMean(sw.GetData())
}

// StdDev 计算窗口内数据标准差.
func (sw *SlidingWindow) StdDev() float64 {
	return calculateStdDev(sw.GetData())
}

// LatestValue 获取最新数据点.
func (sw *SlidingWindow) LatestValue() (float64, bool) {
	sw.mu.RLock()
	defer sw.mu.RUnlock()
	if sw.count == 0 {
		return 0, false
	}
	idx := (sw.head - 1 + sw.size) % sw.size
	return sw.data[idx], true
}

// ==================== 异常检测器 ====================

// AnomalyDetector 异常检测器核心结构体.
type AnomalyDetector struct {
	mu              sync.RWMutex
	windows         map[MetricType]*SlidingWindow     // 各指标的滑动窗口
	thresholds      map[MetricType]*AdaptiveThreshold // 自适应阈值
	history         map[MetricType][]float64          // 历史数据（用于自适应阈值）
	minDataPoints   int                               // 最少数据点数（低于此值不检测）
	zScoreThreshold float64                           // Z-Score 阈值（默认3.0）
	maxHistory      int                               // 历史数据最大长度
	windowSize      int                               // 窗口大小
}

// DetectorConfig 检测器配置.
type DetectorConfig struct {
	WindowSize      int     // 滑动窗口大小
	MinDataPoints   int     // 最少数据点数
	ZScoreThreshold float64 // Z-Score 阈值
	MaxHistory      int     // 历史数据最大长度
}

// DefaultDetectorConfig 返回默认检测器配置.
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		WindowSize:      60,
		MinDataPoints:   10,
		ZScoreThreshold: 3.0,
		MaxHistory:      1000,
	}
}

// NewAnomalyDetector 创建异常检测器实例.
func NewAnomalyDetector(config DetectorConfig) *AnomalyDetector {
	return &AnomalyDetector{
		windows:         make(map[MetricType]*SlidingWindow),
		thresholds:      make(map[MetricType]*AdaptiveThreshold),
		history:         make(map[MetricType][]float64),
		minDataPoints:   config.MinDataPoints,
		zScoreThreshold: config.ZScoreThreshold,
		maxHistory:      config.MaxHistory,
		windowSize:      config.WindowSize,
	}
}

// AddMetric 添加指标数据点并更新自适应阈值.
func (d *AnomalyDetector) AddMetric(metricType MetricType, value float64, source string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 确保滑动窗口存在
	win, ok := d.windows[metricType]
	if !ok {
		win = NewSlidingWindow(d.windowSize)
		d.windows[metricType] = win
	}

	// 写入滑动窗口
	win.Add(value)

	// 记录历史数据用于自适应阈值
	d.history[metricType] = append(d.history[metricType], value)
	if len(d.history[metricType]) > d.maxHistory {
		d.history[metricType] = d.history[metricType][1:]
	}

	// 更新自适应阈值
	d.updateThreshold(metricType)
}

// updateThreshold 更新指定指标的自适应阈值（内部方法，调用者需持写锁）.
func (d *AnomalyDetector) updateThreshold(metricType MetricType) {
	hist := d.history[metricType]
	if len(hist) < d.minDataPoints {
		return
	}
	mean := calculateMean(hist)
	stdDev := calculateStdDev(hist)
	d.thresholds[metricType] = &AdaptiveThreshold{
		Mean:        mean,
		StdDev:      stdDev,
		Upper:       mean + d.zScoreThreshold*stdDev,
		Lower:       mean - d.zScoreThreshold*stdDev,
		SampleCount: len(hist),
	}
}

// DetectAll 对所有已注册指标执行异常检测.
func (d *AnomalyDetector) DetectAll() []AnomalyResult {
	d.mu.RLock()
	metricTypes := make([]MetricType, 0, len(d.windows))
	for mt := range d.windows {
		metricTypes = append(metricTypes, mt)
	}
	d.mu.RUnlock()

	var results []AnomalyResult
	for _, mt := range metricTypes {
		results = append(results, d.Detect(mt)...)
	}
	return results
}

// Detect 对指定指标执行异常检测，返回所有类型的异常.
func (d *AnomalyDetector) Detect(metricType MetricType) []AnomalyResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	win, ok := d.windows[metricType]
	if !ok || win.Count() < d.minDataPoints {
		return nil
	}

	latest, hasValue := win.LatestValue()
	if !hasValue {
		return nil
	}

	data := win.GetData()
	mean := win.Mean()
	stdDev := win.StdDev()
	if stdDev < 1e-9 {
		stdDev = 1e-9 // 避免除零
	}

	var results []AnomalyResult

	// ---- 1. Z-Score 突增/突降检测 ----
	zScore := (latest - mean) / stdDev
	if math.Abs(zScore) >= d.zScoreThreshold {
		anomalyType := AnomalySpike
		if zScore < 0 {
			anomalyType = AnomalyDrop
		}
		severity := SeverityWarning
		if math.Abs(zScore) >= d.zScoreThreshold*1.5 {
			severity = SeverityCritical
		}
		threshold := mean + d.zScoreThreshold*stdDev
		if zScore < 0 {
			threshold = mean - d.zScoreThreshold*stdDev
		}
		results = append(results, AnomalyResult{
			Timestamp:   time.Now(),
			MetricType:  metricType,
			AnomalyType: anomalyType,
			Value:       latest,
			Threshold:   threshold,
			ZScore:      zScore,
			Severity:    severity,
			Message:     fmt.Sprintf("指标 %s 异常: 当前值 %.2f, 均值 %.2f, Z-Score %.2f", metricType, latest, mean, zScore),
		})
	}

	// ---- 2. 趋势异常检测 ----
	if trend := d.detectTrend(metricType, data); trend != nil {
		results = append(results, *trend)
	}

	// ---- 3. 模式异常检测 ----
	if pattern := d.detectPattern(metricType, data); pattern != nil {
		results = append(results, *pattern)
	}

	return results
}

// detectTrend 检测趋势异常（线性回归斜率分析）.
func (d *AnomalyDetector) detectTrend(metricType MetricType, data []float64) *AnomalyResult {
	if len(data) < 20 {
		return nil
	}
	// 取最近 20 个数据点
	recent := data[len(data)-20:]
	slope := calculateSlope(recent)

	// 斜率阈值：均值的 5%，最低 0.1
	mean := calculateMean(recent)
	threshold := mean * 0.05
	if threshold < 0.1 {
		threshold = 0.1
	}

	if math.Abs(slope) > threshold {
		severity := SeverityInfo
		if math.Abs(slope) > threshold*2 {
			severity = SeverityWarning
		}
		direction := "持续上升"
		if slope < 0 {
			direction = "持续下降"
		}
		return &AnomalyResult{
			Timestamp:   time.Now(),
			MetricType:  metricType,
			AnomalyType: AnomalyTrend,
			Value:       slope,
			Threshold:   threshold,
			ZScore:      0,
			Severity:    severity,
			Message:     fmt.Sprintf("指标 %s %s 趋势异常, 斜率 %.4f", metricType, direction, slope),
		}
	}
	return nil
}

// detectPattern 检测模式异常（前后半段均值偏移）.
func (d *AnomalyDetector) detectPattern(metricType MetricType, data []float64) *AnomalyResult {
	if len(data) < 20 {
		return nil
	}
	mid := len(data) / 2
	firstHalf := data[:mid]
	secondHalf := data[mid:]

	mean1 := calculateMean(firstHalf)
	mean2 := calculateMean(secondHalf)
	stdDev1 := calculateStdDev(firstHalf)
	if stdDev1 < 1e-9 {
		stdDev1 = 1e-9
	}

	shift := math.Abs(mean2 - mean1)
	if shift > 2*stdDev1 {
		severity := SeverityInfo
		if shift > 3*stdDev1 {
			severity = SeverityWarning
		}
		return &AnomalyResult{
			Timestamp:   time.Now(),
			MetricType:  metricType,
			AnomalyType: AnomalyPattern,
			Value:       mean2,
			Threshold:   mean1 + 2*stdDev1,
			ZScore:      0,
			Severity:    severity,
			Message:     fmt.Sprintf("指标 %s 模式异常: 均值从 %.2f 偏移到 %.2f", metricType, mean1, mean2),
		}
	}
	return nil
}

// GetThreshold 获取指定指标的自适应阈值.
func (d *AnomalyDetector) GetThreshold(metricType MetricType) *AdaptiveThreshold {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.thresholds[metricType]
}

// GetAllThresholds 获取所有指标的自适应阈值.
func (d *AnomalyDetector) GetAllThresholds() map[MetricType]*AdaptiveThreshold {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make(map[MetricType]*AdaptiveThreshold, len(d.thresholds))
	for k, v := range d.thresholds {
		result[k] = v
	}
	return result
}

// GetMetricsCount 获取各指标的滑动窗口数据点数量.
func (d *AnomalyDetector) GetMetricsCount() map[MetricType]int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make(map[MetricType]int, len(d.windows))
	for mt, win := range d.windows {
		result[mt] = win.Count()
	}
	return result
}

// ==================== 统计工具函数 ====================

// calculateMean 计算数据切片的算术平均值.
func calculateMean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// calculateStdDev 计算数据切片的样本标准差（Bessel 校正）.
func calculateStdDev(data []float64) float64 {
	n := len(data)
	if n < 2 {
		return 0
	}
	mean := calculateMean(data)
	sumSq := 0.0
	for _, v := range data {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(n-1))
}

// calculateSlope 计算线性回归斜率（最小二乘法）
// x 轴为数据索引 0,1,2,..., y 轴为数据值.
func calculateSlope(data []float64) float64 {
	n := float64(len(data))
	if n < 2 {
		return 0
	}
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0
	}
	return (n*sumXY - sumX*sumY) / denom
}
