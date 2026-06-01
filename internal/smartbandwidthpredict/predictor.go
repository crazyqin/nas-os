package smartbandwidthpredict

import (
	"fmt"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Predictor 带宽预测器
type Predictor struct {
	mu     sync.RWMutex
	config *Config
	logger *zap.Logger
}

// NewPredictor 创建带宽预测器
func NewPredictor(config *Config, logger *zap.Logger) *Predictor {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Predictor{
		config: config,
		logger: logger,
	}
}

// Predict 预测带宽
func (p *Predictor) Predict(samples []*TrafficSample, horizonMinutes int) (*BandwidthPrediction, error) {
	if len(samples) == 0 {
		return nil, fmt.Errorf("采样数据为空")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// 提取带宽数据
	bandwidths := make([]float64, len(samples))
	for i, s := range samples {
		bandwidths[i] = s.InboundMbps + s.OutboundMbps
	}

	// 计算移动平均
	maValue := p.movingAverage(bandwidths, p.config.PredictionWindow)

	// 计算指数平滑
	esValue := p.exponentialSmoothing(bandwidths, p.config.SmoothingAlpha)

	// 检测周期性
	periodicity := p.detectPeriodicity(bandwidths)

	// 计算趋势
	trend := p.detectTrend(bandwidths)

	// 综合预测
	predictedValue := p.combinePrediction(maValue, esValue, periodicity, trend)

	// 计算置信区间
	lower, upper, confidence := p.calculateConfidenceInterval(bandwidths, predictedValue)

	prediction := &BandwidthPrediction{
		Timestamp:      time.Now().Add(time.Duration(horizonMinutes) * time.Minute),
		PredictedMbps:  predictedValue,
		LowerBound:     lower,
		UpperBound:     upper,
		Confidence:     confidence,
		Trend:          trend,
		HorizonMinutes: horizonMinutes,
	}

	p.logger.Debug("带宽预测完成",
		zap.Float64("predicted_mbps", predictedValue),
		zap.Float64("lower_bound", lower),
		zap.Float64("upper_bound", upper),
		zap.Float64("confidence", confidence),
		zap.String("trend", string(trend)),
	)

	return prediction, nil
}

// movingAverage 计算移动平均
func (p *Predictor) movingAverage(data []float64, window int) float64 {
	if len(data) == 0 {
		return 0
	}

	if window > len(data) {
		window = len(data)
	}

	sum := 0.0
	for i := len(data) - window; i < len(data); i++ {
		sum += data[i]
	}

	return sum / float64(window)
}

// exponentialSmoothing 计算指数平滑
func (p *Predictor) exponentialSmoothing(data []float64, alpha float64) float64 {
	if len(data) == 0 {
		return 0
	}

	if alpha <= 0 || alpha >= 1 {
		alpha = p.config.SmoothingAlpha
	}

	result := data[0]
	for i := 1; i < len(data); i++ {
		result = alpha*data[i] + (1-alpha)*result
	}

	return result
}

// detectPeriodicity 检测周期性
func (p *Predictor) detectPeriodicity(data []float64) float64 {
	if len(data) < 24 {
		return 0
	}

	// 尝试检测日周期（24小时，假设每小时一个采样点）
	// 这里简化处理，计算自相关
	maxLag := len(data) / 2
	if maxLag > 168 { // 最多检测一周
		maxLag = 168
	}

	bestCorr := 0.0
	bestLag := 0

	for lag := 1; lag <= maxLag; lag++ {
		corr := p.autocorrelation(data, lag)
		if corr > bestCorr {
			bestCorr = corr
			bestLag = lag
		}
	}

	if bestCorr > 0.7 && bestLag > 0 {
		// 检测到强周期性，预测下一周期的值
		if bestLag < len(data) {
			return data[len(data)-bestLag]
		}
	}

	return 0
}

// autocorrelation 计算自相关系数
func (p *Predictor) autocorrelation(data []float64, lag int) float64 {
	if len(data) <= lag {
		return 0
	}

	n := len(data) - lag
	if n <= 0 {
		return 0
	}

	// 计算均值
	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))

	// 计算自相关
	variance := 0.0
	covariance := 0.0

	for i := 0; i < n; i++ {
		deviation := data[i] - mean
		variance += deviation * deviation
		covariance += deviation * (data[i+lag] - mean)
	}

	if variance == 0 {
		return 0
	}

	return covariance / variance
}

// detectTrend 检测趋势
func (p *Predictor) detectTrend(data []float64) TrendType {
	if len(data) < 10 {
		return TrendStable
	}

	// 使用线性回归检测趋势
	n := float64(len(data))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 计算斜率
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return TrendStable
	}

	slope := (n*sumXY - sumX*sumY) / denominator

	// 计算平均值
	avgY := sumY / n

	// 判断趋势（斜率相对于平均值的比例）
	if avgY == 0 {
		return TrendStable
	}

	relativeSlope := slope / avgY * 100

	if relativeSlope > 5 {
		return TrendRising
	} else if relativeSlope < -5 {
		return TrendFalling
	}

	return TrendStable
}

// combinePrediction 综合预测
func (p *Predictor) combinePrediction(ma, es, periodic float64, trend TrendType) float64 {
	// 加权组合
	weights := map[string]float64{
		"ma":       0.4,
		"es":       0.3,
		"periodic": 0.2,
		"trend":    0.1,
	}

	result := ma*weights["ma"] + es*weights["es"]

	if periodic > 0 {
		result = result*0.8 + periodic*0.2
	}

	// 根据趋势调整
	switch trend {
	case TrendRising:
		result *= 1.1
	case TrendFalling:
		result *= 0.9
	}

	return result
}

// calculateConfidenceInterval 计算置信区间
func (p *Predictor) calculateConfidenceInterval(data []float64, predicted float64) (lower, upper, confidence float64) {
	if len(data) < 2 {
		return predicted * 0.8, predicted * 1.2, 0.5
	}

	// 计算标准差
	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(len(data))

	variance := 0.0
	for _, v := range data {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(data) - 1)
	stdDev := math.Sqrt(variance)

	// 计算标准误差
	stdErr := stdDev / math.Sqrt(float64(len(data)))

	// 使用t分布近似（简化处理）
	// 95%置信区间，z=1.96
	z := 1.96

	margin := z * stdErr

	lower = predicted - margin
	upper = predicted + margin

	// 确保下界不为负
	if lower < 0 {
		lower = 0
	}

	// 计算置信度（基于数据量和稳定性）
	dataPoints := float64(len(data))
	stability := 1 - (stdDev / mean) // 变异系数的补数

	confidence = math.Min(0.95, 0.5+dataPoints/200) * math.Max(0.5, stability)

	return lower, upper, confidence
}

// IsAnomaly 检测异常
func (p *Predictor) IsAnomaly(samples []*TrafficSample) bool {
	if len(samples) < 10 {
		return false
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// 提取带宽数据
	bandwidths := make([]float64, len(samples))
	for i, s := range samples {
		bandwidths[i] = s.InboundMbps + s.OutboundMbps
	}

	// 计算均值和标准差
	mean := 0.0
	for _, v := range bandwidths {
		mean += v
	}
	mean /= float64(len(bandwidths))

	variance := 0.0
	for _, v := range bandwidths {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(bandwidths) - 1)
	stdDev := math.Sqrt(variance)

	// 检测最新值是否异常
	latest := bandwidths[len(bandwidths)-1]
	threshold := p.config.AnomalyThreshold

	if math.Abs(latest-mean) > threshold*stdDev {
		p.logger.Warn("检测到异常流量",
			zap.Float64("latest", latest),
			zap.Float64("mean", mean),
			zap.Float64("std_dev", stdDev),
			zap.Float64("threshold", threshold),
		)
		return true
	}

	return false
}

// DetectDailyPattern 检测日模式
func (p *Predictor) DetectDailyPattern(samples []*TrafficSample) map[int]float64 {
	if len(samples) < 24 {
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// 按小时分组
	hourlyData := make(map[int][]float64)
	for _, sample := range samples {
		hour := sample.Timestamp.Hour()
		bandwidth := sample.InboundMbps + sample.OutboundMbps
		hourlyData[hour] = append(hourlyData[hour], bandwidth)
	}

	// 计算每小时平均
	pattern := make(map[int]float64)
	for hour, data := range hourlyData {
		sum := 0.0
		for _, v := range data {
			sum += v
		}
		pattern[hour] = sum / float64(len(data))
	}

	return pattern
}

// DetectWeeklyPattern 检测周模式
func (p *Predictor) DetectWeeklyPattern(samples []*TrafficSample) map[time.Weekday]float64 {
	if len(samples) < 168 { // 至少一周的数据
		return nil
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// 按星期几分组
	weekdayData := make(map[time.Weekday][]float64)
	for _, sample := range samples {
		weekday := sample.Timestamp.Weekday()
		bandwidth := sample.InboundMbps + sample.OutboundMbps
		weekdayData[weekday] = append(weekdayData[weekday], bandwidth)
	}

	// 计算每天平均
	pattern := make(map[time.Weekday]float64)
	for weekday, data := range weekdayData {
		sum := 0.0
		for _, v := range data {
			sum += v
		}
		pattern[weekday] = sum / float64(len(data))
	}

	return pattern
}

// UpdateConfig 更新配置
func (p *Predictor) UpdateConfig(config *Config) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if config != nil {
		p.config = config
	}
}

// GetConfig 获取配置
func (p *Predictor) GetConfig() *Config {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}
