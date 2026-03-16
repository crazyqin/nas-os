// Package reports 提供报表生成和管理功能
package reports

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// ========== v2.76.0 资源报告增强辅助函数 ==========

// FormatBytes 格式化字节为人类可读格式
func FormatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
		PB = TB * 1024
	)

	switch {
	case bytes >= PB:
		return roundStr(float64(bytes)/float64(PB), 2) + " PB"
	case bytes >= TB:
		return roundStr(float64(bytes)/float64(TB), 2) + " TB"
	case bytes >= GB:
		return roundStr(float64(bytes)/float64(GB), 2) + " GB"
	case bytes >= MB:
		return roundStr(float64(bytes)/float64(MB), 2) + " MB"
	case bytes >= KB:
		return roundStr(float64(bytes)/float64(KB), 2) + " KB"
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatBytesShort 格式化字节为简短格式
func FormatBytesShort(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return roundStr(float64(bytes)/float64(TB), 1) + "T"
	case bytes >= GB:
		return roundStr(float64(bytes)/float64(GB), 1) + "G"
	case bytes >= MB:
		return roundStr(float64(bytes)/float64(MB), 1) + "M"
	case bytes >= KB:
		return roundStr(float64(bytes)/float64(KB), 1) + "K"
	default:
		return fmt.Sprintf("%d", bytes)
	}
}

// FormatPercent 格式化百分比
func FormatPercent(value float64) string {
	return roundStr(value, 2) + "%"
}

// FormatDuration 格式化持续时间
func FormatDuration(seconds int64) string {
	if seconds < 0 {
		return "-"
	}

	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	switch {
	case days > 365:
		years := days / 365
		remainDays := days % 365
		return fmt.Sprintf("%d年%d天", years, remainDays)
	case days > 0:
		return fmt.Sprintf("%d天%d小时", days, hours)
	case hours > 0:
		return fmt.Sprintf("%d小时%d分", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%d分%d秒", minutes, secs)
	default:
		return fmt.Sprintf("%d秒", secs)
	}
}

// FormatTimestamp 格式化时间戳
func FormatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// FormatDate 格式化日期
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// CalculateGrowthRate 计算增长率
func CalculateGrowthRate(oldValue, newValue uint64, days float64) float64 {
	if oldValue == 0 || days == 0 {
		return 0
	}
	return (float64(newValue) - float64(oldValue)) / float64(oldValue) / days * 100
}

// PredictLinear 线性预测
func PredictLinear(currentValue uint64, dailyGrowth float64, days int) uint64 {
	return uint64(float64(currentValue) + dailyGrowth*float64(days))
}

// PredictExponential 指数预测
func PredictExponential(currentValue uint64, dailyRate float64, days int) uint64 {
	factor := math.Pow(1+dailyRate, float64(days))
	return uint64(float64(currentValue) * factor)
}

// CalculateDaysToCapacity 计算到达容量的天数
func CalculateDaysToCapacity(currentUsed, totalCapacity uint64, dailyGrowth float64) int {
	if dailyGrowth <= 0 {
		return -1 // 无限
	}

	remaining := float64(totalCapacity - currentUsed)
	if remaining <= 0 {
		return 0 // 已满
	}

	days := remaining / dailyGrowth
	return int(math.Ceil(days))
}

// CalculateRecommendedExpansion 计算建议扩容量
func CalculateRecommendedExpansion(currentUsed, totalCapacity uint64, forecastMonths int, growthRate float64) uint64 {
	if growthRate <= 0 {
		return 0
	}

	// 预测未来使用量
	forecastUsed := uint64(float64(currentUsed) * math.Pow(1+growthRate/100.0, float64(forecastMonths)))

	// 当前剩余空间
	remaining := totalCapacity - currentUsed

	// 需要扩容量 = 预测使用量 - 当前剩余 - 安全缓冲(20%)
	needed := int64(forecastUsed) - int64(remaining) - int64(float64(forecastUsed)*0.2)

	if needed <= 0 {
		return 0
	}

	return uint64(needed)
}

// CalculateEfficiencyScore 计算效率评分
func CalculateEfficiencyScore(usagePercent float64) float64 {
	// 使用率在 60-80% 之间效率最高
	if usagePercent >= 60 && usagePercent <= 80 {
		return 100
	}

	// 过低使用率
	if usagePercent < 60 {
		return round(usagePercent/60*70, 2)
	}

	// 过高使用率
	return round((100-usagePercent)/20*50+50, 2)
}

// CalculateHealthScore 计算健康评分
func CalculateHealthScore(cpuUsage, memoryUsage, diskUsage float64) int {
	// CPU 评分
	cpuScore := 100 - int(cpuUsage/2)
	if cpuScore < 0 {
		cpuScore = 0
	}

	// 内存评分
	memScore := 100 - int(memoryUsage/2)
	if memScore < 0 {
		memScore = 0
	}

	// 磁盘评分
	diskScore := 100 - int(diskUsage/2)
	if diskScore < 0 {
		diskScore = 0
	}

	return (cpuScore + memScore + diskScore) / 3
}

// GenerateReportID 生成报告ID
func GenerateReportID(prefix string) string {
	return prefix + "_" + time.Now().Format("20060102150405")
}

// SafeDivide 安全除法
func SafeDivide(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// SafeDivideUint 安全除法（uint64）
func SafeDivideUint(a, b uint64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// Clamp 将值限制在范围内
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// MinFloat 返回最小值
func MinFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// MaxFloat 返回最大值
func MaxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// MinUint 返回最小值（uint64）
func MinUint(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// MaxUint 返回最大值（uint64）
func MaxUint(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

// ========== 统计计算函数 ==========

// CalculateAverage 计算平均值
func CalculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// CalculateMedian 计算中位数
func CalculateMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 复制切片以避免修改原数据
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// 使用标准库排序 O(n log n) 替代冒泡排序 O(n²)
	sort.Float64s(sorted)

	n := len(sorted)
	if n%2 == 0 {
		return (sorted[n/2-1] + sorted[n/2]) / 2
	}
	return sorted[n/2]
}

// CalculateStdDev 计算标准差
func CalculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	avg := CalculateAverage(values)
	var variance float64
	for _, v := range values {
		variance += (v - avg) * (v - avg)
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}

// CalculatePercentile 计算百分位数
func CalculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 复制并排序 - 使用标准库排序 O(n log n) 替代冒泡排序 O(n²)
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sort.Float64s(sorted)

	index := (percentile / 100) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	fraction := index - float64(lower)
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}

// ========== 趋势分析函数 ==========

// TrendDirection 趋势方向
type TrendDirection string

const (
	TrendUp      TrendDirection = "up"
	TrendDown    TrendDirection = "down"
	TrendStable  TrendDirection = "stable"
	TrendUnknown TrendDirection = "unknown"
)

// AnalyzeTrend 分析趋势
func AnalyzeTrend(values []float64) TrendDirection {
	if len(values) < 2 {
		return TrendUnknown
	}

	// 简单线性趋势分析
	n := len(values)
	firstHalf := values[:n/2]
	secondHalf := values[n/2:]

	firstAvg := CalculateAverage(firstHalf)
	secondAvg := CalculateAverage(secondHalf)

	// 变化超过 5% 认为有明显趋势
	change := (secondAvg - firstAvg) / firstAvg * 100

	if change > 5 {
		return TrendUp
	} else if change < -5 {
		return TrendDown
	}
	return TrendStable
}

// CalculateTrendSlope 计算趋势斜率（简单线性回归）
func CalculateTrendSlope(values []float64) float64 {
	n := float64(len(values))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denominator
}
