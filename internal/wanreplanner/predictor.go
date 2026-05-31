package wanreplanner

import (
	"math"
	"sort"
	"time"
)

// PredictBandwidth 基于历史数据预测带宽
// 使用简单线性回归 + 指数加权移动平均
func (p *WANPlanner) PredictBandwidth(linkID string) (*PredictionResult, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	samples := p.filterHistory(linkID)
	if len(samples) < 5 {
		return nil, ErrInsufficientData
	}

	// 计算指数加权移动平均 (EWMA)
	alpha := 0.3
	ewma := samples[0].Utilization
	for i := 1; i < len(samples); i++ {
		ewma = alpha*samples[i].Utilization + (1-alpha)*ewma
	}

	// 线性回归预测趋势
	slope, intercept := linearRegression(samples)
	n := float64(len(samples))
	nextX := n
	predictedUtil := slope*nextX + intercept
	if predictedUtil < 0 {
		predictedUtil = 0
	}
	if predictedUtil > 1 {
		predictedUtil = 1
	}

	// 结合 EWMA 和线性回归
	combined := ewma*0.6 + predictedUtil*0.4

	// 计算可用带宽
	link, exists := p.links[linkID]
	var estimatedBW int64
	if exists && link.Bandwidth > 0 {
		estimatedBW = int64(float64(link.Bandwidth) * (1.0 - combined))
	} else {
		// 使用历史数据估算
		var totalBW int64
		for _, s := range samples {
			totalBW += s.BytesIn + s.BytesOut
		}
		avgBW := totalBW / int64(len(samples))
		estimatedBW = avgBW
	}

	// 计算置信度
	confidence := calculateConfidence(samples)

	return &PredictionResult{
		LinkID:      linkID,
		EstimatedBW: estimatedBW,
		Confidence:  confidence,
		PredictedAt: time.Now(),
		ValidUntil:  time.Now().Add(p.config.PredictionWindow),
	}, nil
}

// PredictAllLinks 预测所有链路带宽
func (p *WANPlanner) PredictAllLinks() map[string]*PredictionResult {
	p.mu.RLock()
	linkIDs := make([]string, 0, len(p.links))
	for id := range p.links {
		linkIDs = append(linkIDs, id)
	}
	p.mu.RUnlock()

	results := make(map[string]*PredictionResult)
	for _, id := range linkIDs {
		result, err := p.PredictBandwidth(id)
		if err == nil {
			results[id] = result
		}
	}
	return results
}

// GetUtilizationTrend 获取链路利用率趋势
func (p *WANPlanner) GetUtilizationTrend(linkID string, duration time.Duration) []BandwidthSample {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	result := make([]BandwidthSample, 0)
	for _, s := range p.history {
		if s.LinkID == linkID && s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}

// GetPeakUsage 获取指定时间段内的峰值使用
func (p *WANPlanner) GetPeakUsage(linkID string, duration time.Duration) (peakUtil float64, peakTime time.Time) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	for _, s := range p.history {
		if s.LinkID == linkID && s.Timestamp.After(cutoff) {
			if s.Utilization > peakUtil {
				peakUtil = s.Utilization
				peakTime = s.Timestamp
			}
		}
	}
	return
}

// filterHistory 筛选指定链路的历史数据（调用者需持锁）
func (p *WANPlanner) filterHistory(linkID string) []BandwidthSample {
	result := make([]BandwidthSample, 0)
	for _, s := range p.history {
		if s.LinkID == linkID {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}

// linearRegression 简单线性回归
// x = index (0, 1, 2, ...), y = utilization
func linearRegression(samples []BandwidthSample) (slope, intercept float64) {
	n := float64(len(samples))
	if n == 0 {
		return 0, 0
	}
	var sumX, sumY, sumXY, sumX2 float64
	for i, s := range samples {
		x := float64(i)
		y := s.Utilization
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	denom := n*sumX2 - sumX*sumX
	if denom == 0 {
		return 0, sumY / n
	}
	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return
}

// calculateConfidence 计算预测置信度
func calculateConfidence(samples []BandwidthSample) float64 {
	if len(samples) < 2 {
		return 0.1
	}
	// 计算变异系数 (CV)
	var sum, sumSq float64
	for _, s := range samples {
		sum += s.Utilization
		sumSq += s.Utilization * s.Utilization
	}
	n := float64(len(samples))
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance < 0 {
		variance = 0
	}
	stddev := math.Sqrt(variance)
	cv := stddev / mean
	if math.IsNaN(cv) || math.IsInf(cv, 0) {
		return 0.1
	}
	// CV 越小，置信度越高
	confidence := 1.0 - cv
	if confidence < 0.1 {
		confidence = 0.1
	}
	if confidence > 0.99 {
		confidence = 0.99
	}
	// 样本数量影响置信度
	sizeFactor := math.Min(n/50.0, 1.0)
	return confidence * sizeFactor
}
