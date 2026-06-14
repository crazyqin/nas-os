// Package healthpredictor 异常检测引擎
package healthpredictor

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// AnomalyDetector 异常检测器
type AnomalyDetector struct {
	mu       sync.RWMutex
	config   HealthPredictorConfig
	history  map[MetricType][]AnomalyDetectionResult
	maxHist  int
}

// NewAnomalyDetector 创建异常检测器
func NewAnomalyDetector(config HealthPredictorConfig) *AnomalyDetector {
	return &AnomalyDetector{
		config:  config,
		history: make(map[MetricType][]AnomalyDetectionResult),
		maxHist: 1000,
	}
}

// Detect 对时间序列进行异常检测
func (d *AnomalyDetector) Detect(series *TimeSeries) *AnomalyDetectionResult {
	if series == nil || len(series.Points) < 10 {
		return &AnomalyDetectionResult{
			IsAnomaly:  false,
			Level:      AnomalyNone,
			MetricType: series.MetricType,
			Timestamp:  time.Now(),
		}
	}

	points := series.Points
	latest := points[len(points)-1]

	// 计算滑动窗口统计量
	windowSize := d.config.AnomalyWindow
	if windowSize > len(points)-1 {
		windowSize = len(points) - 1
	}

	window := points[len(points)-1-windowSize : len(points)-1]

	// 计算均值和标准差
	mean := calcMean(window)
	stdDev := calcStdDev(window, mean)

	// Z-score 异常检测
	deviation := 0.0
	if stdDev > 0 {
		deviation = (latest.Value - mean) / stdDev
	}

	isAnomaly := math.Abs(deviation) >= d.config.AnomalySigma

	// 确定异常等级
	level := AnomalyNone
	if isAnomaly {
		if math.Abs(deviation) >= d.config.AnomalySigma*1.5 {
			level = AnomalyCritical
		} else {
			level = AnomalyWarning
		}
	}

	// 检查阈值
	if level == AnomalyNone {
		level = d.checkThresholds(series.MetricType, latest.Value)
		isAnomaly = level != AnomalyNone
	}

	result := &AnomalyDetectionResult{
		IsAnomaly:  isAnomaly,
		Level:      level,
		MetricType: series.MetricType,
		Value:      latest.Value,
		Expected:   mean,
		StdDev:     stdDev,
		Deviation:  deviation,
		Timestamp:  latest.Timestamp,
		Description: d.describeAnomaly(series.MetricType, level, latest.Value, mean, deviation),
	}

	// 存储历史
	if isAnomaly {
		d.recordAnomaly(series.MetricType, result)
	}

	return result
}

// DetectAll 对所有时间序列进行异常检测
func (d *AnomalyDetector) DetectAll(allSeries map[MetricType]*TimeSeries) []AnomalyDetectionResult {
	var results []AnomalyDetectionResult

	for _, series := range allSeries {
		result := d.Detect(series)
		if result.IsAnomaly {
			results = append(results, *result)
		}
	}

	return results
}

// checkThresholds 检查静态阈值
func (d *AnomalyDetector) checkThresholds(mt MetricType, value float64) AnomalyLevel {
	th := d.config.Thresholds

	switch mt {
	case MetricCPUUsage:
		if value >= th.CPUCritical {
			return AnomalyCritical
		} else if value >= th.CPUWarning {
			return AnomalyWarning
		}
	case MetricMemoryUsage:
		if value >= th.MemCritical {
			return AnomalyCritical
		} else if value >= th.MemWarning {
			return AnomalyWarning
		}
	case MetricDiskUsage:
		if value >= th.DiskCritical {
			return AnomalyCritical
		} else if value >= th.DiskWarning {
			return AnomalyWarning
		}
	case MetricDiskTemp:
		if value >= th.DiskTempCritical {
			return AnomalyCritical
		} else if value >= th.DiskTempWarning {
			return AnomalyWarning
		}
	}

	return AnomalyNone
}

// describeAnomaly 生成异常描述
func (d *AnomalyDetector) describeAnomaly(mt MetricType, level AnomalyLevel, value, mean, deviation float64) string {
	if level == AnomalyNone {
		return ""
	}

	metricName := metricDisplayName(mt)
	absDev := math.Abs(deviation)
	_ = absDev

	if level == AnomalyCritical {
		return fmt.Sprintf(metricName+" 严重异常: 当前值 %.1f, 均值 %.1f, 偏离 %.1fσ", value, mean, deviation)
	}
	return fmt.Sprintf(metricName+" 异常: 当前值 %.1f, 均值 %.1f, 偏离 %.1fσ", value, mean, deviation)
}

// recordAnomaly 记录异常
func (d *AnomalyDetector) recordAnomaly(mt MetricType, result *AnomalyDetectionResult) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.history[mt] = append(d.history[mt], *result)
	if len(d.history[mt]) > d.maxHist {
		d.history[mt] = d.history[mt][1:]
	}

	log.Printf("[HealthPredictor] 检测到异常: %s, 级别=%s, 值=%.1f, 偏离=%.1fσ",
		metricDisplayName(mt), result.Level, result.Value, result.Deviation)
}

// GetAnomalyHistory 获取异常历史
func (d *AnomalyDetector) GetAnomalyHistory(mt MetricType, limit int) []AnomalyDetectionResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	history, ok := d.history[mt]
	if !ok {
		return nil
	}

	if limit <= 0 || limit > len(history) {
		limit = len(history)
	}

	start := len(history) - limit
	result := make([]AnomalyDetectionResult, limit)
	copy(result, history[start:])
	return result
}

// GetRecentAnomalies 获取最近的所有异常
func (d *AnomalyDetector) GetRecentAnomalies(since time.Time) []AnomalyDetectionResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var results []AnomalyDetectionResult
	for _, history := range d.history {
		for _, a := range history {
			if a.Timestamp.After(since) {
				results = append(results, a)
			}
		}
	}
	return results
}

// 统计函数

func calcMean(points []MetricPoint) float64 {
	if len(points) == 0 {
		return 0
	}
	sum := 0.0
	for _, p := range points {
		sum += p.Value
	}
	return sum / float64(len(points))
}

func calcStdDev(points []MetricPoint, mean float64) float64 {
	if len(points) < 2 {
		return 0
	}
	sumSq := 0.0
	for _, p := range points {
		diff := p.Value - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(points)-1))
}

// calcSlope 计算线性回归斜率
func calcSlope(points []MetricPoint) float64 {
	n := float64(len(points))
	if n < 2 {
		return 0
	}

	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0
	for i, p := range points {
		x := float64(i)
		y := p.Value
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

// metricDisplayName 指标显示名
func metricDisplayName(mt MetricType) string {
	switch mt {
	case MetricCPUUsage:
		return "CPU使用率"
	case MetricMemoryUsage:
		return "内存使用率"
	case MetricDiskUsage:
		return "磁盘使用率"
	case MetricDiskTemp:
		return "磁盘温度"
	case MetricNetworkIn:
		return "网络入站"
	case MetricNetworkOut:
		return "网络出站"
	case MetricLoadAvg1:
		return "1分钟负载"
	case MetricLoadAvg5:
		return "5分钟负载"
	case MetricLoadAvg15:
		return "15分钟负载"
	}
	return string(mt)
}
