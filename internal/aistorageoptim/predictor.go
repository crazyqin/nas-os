package aistorageoptim

import (
	"math"
	"time"
)

// Predictor 访问模式预测器.
type Predictor struct {
	historySize int
}

// NewPredictor 创建预测器.
func NewPredictor(historySize int) *Predictor {
	if historySize <= 0 {
		historySize = 100
	}
	return &Predictor{
		historySize: historySize,
	}
}

// PredictAccessPattern 预测访问模式.
func (p *Predictor) PredictAccessPattern(stats *FileAccessStats, now time.Time) AccessPattern {
	if stats.AccessCount == 0 {
		return PatternArchive
	}

	// 计算最近访问的热度
	hotness := p.calculateHotness(stats, now)

	switch {
	case hotness >= 80:
		return PatternHot
	case hotness >= 50:
		return PatternWarm
	case hotness >= 20:
		return PatternCold
	default:
		return PatternArchive
	}
}

// calculateHotness 计算热度值 (0-100).
func (p *Predictor) calculateHotness(stats *FileAccessStats, now time.Time) float64 {
	if stats.AccessCount == 0 {
		return 0
	}

	// 因素1: 访问频率 (40%)
	frequencyScore := p.calculateFrequencyScore(stats)

	// 因素2: 最近访问时间 (30%)
	recencyScore := p.calculateRecencyScore(stats, now)

	// 因素3: 访问规律性 (20%)
	regularityScore := p.calculateRegularityScore(stats)

	// 因素4: 访问趋势 (10%)
	trendScore := p.calculateTrendScore(stats, now)

	// 当没有窗口数据时，增加频率和最近访问的权重
	if len(stats.Windows) < 2 {
		return frequencyScore*0.55 + recencyScore*0.45
	}

	return frequencyScore*0.4 + recencyScore*0.3 + regularityScore*0.2 + trendScore*0.1
}

// calculateFrequencyScore 计算频率分数.
func (p *Predictor) calculateFrequencyScore(stats *FileAccessStats) float64 {
	if stats.AccessFrequency <= 0 {
		return 0
	}
	// 对数归一化，每小时50次访问≈85分
	score := math.Log10(stats.AccessFrequency+1) * 50
	return math.Min(100, score)
}

// calculateRecencyScore 计算最近访问分数.
func (p *Predictor) calculateRecencyScore(stats *FileAccessStats, now time.Time) float64 {
	if stats.LastAccessTime.IsZero() {
		return 0
	}

	hoursSince := now.Sub(stats.LastAccessTime).Hours()

	// 指数衰减: 1小时内=100，24小时=50，7天=10
	switch {
	case hoursSince < 1:
		return 100
	case hoursSince < 24:
		return 100 - (hoursSince/24)*50
	case hoursSince < 168: // 7天
		return 50 - ((hoursSince-24)/144)*40
	default:
		return math.Max(0, 10-math.Log10(hoursSince/168)*10)
	}
}

// calculateRegularityScore 计算规律性分数.
func (p *Predictor) calculateRegularityScore(stats *FileAccessStats) float64 {
	if len(stats.Windows) < 2 {
		return 50 // 数据不足，返回中等分数
	}

	// 计算访问间隔的标准差
	var intervals []float64
	for i := 1; i < len(stats.Windows); i++ {
		interval := stats.Windows[i].Timestamp.Sub(stats.Windows[i-1].Timestamp).Hours()
		intervals = append(intervals, interval)
	}

	if len(intervals) == 0 {
		return 50
	}

	mean := meanValue(intervals)
	stddev := stdDev(intervals, mean)

	// 变异系数越小，规律性越强
	if mean == 0 {
		return 50
	}
	cv := stddev / mean

	// cv < 0.3 = 高规律性，cv > 2 = 无规律
	switch {
	case cv < 0.3:
		return 90
	case cv < 0.5:
		return 70
	case cv < 1.0:
		return 50
	case cv < 2.0:
		return 30
	default:
		return 10
	}
}

// calculateTrendScore 计算趋势分数.
func (p *Predictor) calculateTrendScore(stats *FileAccessStats, now time.Time) float64 {
	if len(stats.Windows) < 4 {
		return 50 // 数据不足
	}

	// 将窗口分为前半和后半，比较访问量
	mid := len(stats.Windows) / 2
	var firstHalf, secondHalf int64

	for i, w := range stats.Windows {
		if i < mid {
			firstHalf += w.Count
		} else {
			secondHalf += w.Count
		}
	}

	if firstHalf == 0 {
		if secondHalf > 0 {
			return 90 // 新文件，访问量上升
		}
		return 50
	}

	ratio := float64(secondHalf) / float64(firstHalf)

	switch {
	case ratio > 2.0:
		return 90 // 访问量大幅上升
	case ratio > 1.5:
		return 75
	case ratio > 1.0:
		return 60 // 略有上升
	case ratio > 0.5:
		return 40 // 略有下降
	default:
		return 20 // 访问量大幅下降
	}
}

// PredictNextAccessTime 预测下次访问时间.
func (p *Predictor) PredictNextAccessTime(stats *FileAccessStats, now time.Time) (time.Time, float64) {
	if len(stats.Windows) < 3 {
		return time.Time{}, 0
	}

	// 计算访问间隔
	var intervals []time.Duration
	for i := 1; i < len(stats.Windows); i++ {
		interval := stats.Windows[i].Timestamp.Sub(stats.Windows[i-1].Timestamp)
		intervals = append(intervals, interval)
	}

	if len(intervals) == 0 {
		return time.Time{}, 0
	}

	// 指数平滑预测 (α=0.3)
	alpha := 0.3
	smoothed := float64(intervals[0])
	for _, interval := range intervals[1:] {
		smoothed = alpha*float64(interval) + (1-alpha)*smoothed
	}

	predictedInterval := time.Duration(smoothed)
	predicted := stats.LastAccessTime.Add(predictedInterval)

	// 计算置信度
	var intervalFloats []float64
	for _, iv := range intervals {
		intervalFloats = append(intervalFloats, float64(iv))
	}
	mean := meanValue(intervalFloats)
	sd := stdDev(intervalFloats, mean)

	confidence := 1.0
	if mean > 0 {
		confidence = 1.0 - math.Min(1.0, sd/mean)
	}

	// 确保预测时间在未来
	if predicted.Before(now) {
		predicted = now.Add(predictedInterval)
	}

	return predicted, confidence
}

// DetectIOPattern 检测I/O模式.
func (p *Predictor) DetectIOPattern(stats *FileAccessStats) IOPattern {
	if len(stats.Windows) < 3 {
		return IOPatternRandom
	}

	// 计算窗口间访问量的变异系数
	var counts []float64
	for _, w := range stats.Windows {
		counts = append(counts, float64(w.Count))
	}

	mean := meanValue(counts)
	if mean == 0 {
		return IOPatternRandom
	}

	stddev := stdDev(counts, mean)
	cv := stddev / mean

	// 计算平均字节数
	var totalBytes int64
	for _, w := range stats.Windows {
		totalBytes += w.Bytes
	}
	avgBytes := float64(totalBytes) / float64(len(stats.Windows))

	switch {
	case cv < 0.3:
		// 低变异 -> 顺序/流式
		if avgBytes > 4*1024*1024 { // >4MB
			return IOPatternStreaming
		}
		return IOPatternSequential
	case cv > 1.5:
		// 高变异 -> 突发
		return IOPatternBurst
	default:
		return IOPatternRandom
	}
}

// UpdateAccessStats 更新访问统计.
func (p *Predictor) UpdateAccessStats(stats *FileAccessStats, bytesRead, bytesWritten int64, now time.Time) {
	stats.AccessCount++
	stats.TotalBytesRead += bytesRead
	stats.TotalBytesWrite += bytesWritten
	stats.LastAccessTime = now

	if stats.FirstAccessTime.IsZero() {
		stats.FirstAccessTime = now
	}

	// 更新访问窗口
	windowDuration := time.Hour // 1小时窗口
	if len(stats.Windows) == 0 || now.Sub(stats.Windows[len(stats.Windows)-1].Timestamp) >= windowDuration {
		stats.Windows = append(stats.Windows, AccessWindow{
			Timestamp: now,
			Count:     1,
			Bytes:     bytesRead + bytesWritten,
		})
		// 保持窗口数在限制内
		if len(stats.Windows) > p.historySize {
			stats.Windows = stats.Windows[len(stats.Windows)-p.historySize:]
		}
	} else {
		last := &stats.Windows[len(stats.Windows)-1]
		last.Count++
		last.Bytes += bytesRead + bytesWritten
	}

	// 更新访问频率 (每小时)
	if !stats.FirstAccessTime.IsZero() {
		hours := now.Sub(stats.FirstAccessTime).Hours()
		if hours > 0 {
			stats.AccessFrequency = float64(stats.AccessCount) / hours
		}
	}

	// 更新IO模式
	stats.IOPattern = p.DetectIOPattern(stats)
}

// meanValue 计算平均值.
func meanValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// stdDev 计算标准差.
func stdDev(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		diff := v - mean
		sum += diff * diff
	}
	return math.Sqrt(sum / float64(len(values)))
}
