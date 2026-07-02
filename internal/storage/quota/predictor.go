// Package quota 提供存储配额管理和告警功能
package quota

import (
	"errors"
	"math"
	"sync"
	"time"
)

// ========== 预测器错误 ==========

var (
	// ErrInsufficientData 数据不足.
	ErrInsufficientData = errors.New("历史数据不足以进行预测")
	// ErrInvalidConfig 无效配置.
	ErrInvalidConfig = errors.New("无效的预测配置")
)

// Predictor 容量预测器.
type Predictor struct {
	mu       sync.RWMutex
	config   ForecastConfig
	history  map[string][]UsageHistory // targetID -> 历史数据
	maxItems int                       // 每个目标最大历史记录数
}

// NewPredictor 创建预测器.
func NewPredictor(config ForecastConfig) *Predictor {
	if config.HistoryDays <= 0 {
		config.HistoryDays = 30
	}
	if config.MinDataPoints <= 0 {
		config.MinDataPoints = 7
	}

	return &Predictor{
		config:   config,
		history:  make(map[string][]UsageHistory),
		maxItems: config.HistoryDays * 24, // 最多保存 HistoryDays 天的每小时数据
	}
}

// RecordUsage 记录使用量.
func (p *Predictor) RecordUsage(targetID string, usedBytes int64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	record := UsageHistory{
		Timestamp: time.Now(),
		UsedBytes: usedBytes,
	}

	records := p.history[targetID]
	records = append(records, record)

	// 保留最近的记录
	if len(records) > p.maxItems {
		records = records[len(records)-p.maxItems:]
	}

	p.history[targetID] = records
}

// Predict 预测容量使用趋势.
func (p *Predictor) Predict(ruleID, targetID string, maxBytes int64) (*PredictionResult, error) {
	p.mu.RLock()
	records := p.history[targetID]
	p.mu.RUnlock()

	if len(records) < p.config.MinDataPoints {
		return nil, ErrInsufficientData
	}

	// 计算当前使用量和增长率
	currentUsage := records[len(records)-1].UsedBytes
	currentPercent := float64(currentUsage) / float64(maxBytes) * 100

	// 计算每日增长率
	growthRate, trend, confidence := p.calculateGrowthRate(records)

	// 预测满额时间
	daysUntilFull := 0.0
	estimatedFullDate := "N/A"

	if growthRate > 0 {
		remainingBytes := maxBytes - currentUsage
		daysUntilFull = float64(remainingBytes) / growthRate
		if daysUntilFull > 0 && daysUntilFull < 365*10 { // 最多预测10年
			estimatedFullDate = time.Now().AddDate(0, 0, int(daysUntilFull)).Format("2006-01-02")
		} else {
			daysUntilFull = -1 // 负数表示几乎已满或增长太快无法预测
			estimatedFullDate = "即将满额"
		}
	} else if growthRate < 0 {
		// 使用量在下降
		daysUntilFull = -1
		estimatedFullDate = "不会满额（使用量下降趋势）"
	} else {
		// 无增长
		daysUntilFull = -1
		estimatedFullDate = "使用量稳定"
	}

	// 确定警告级别
	warningLevel := GetWarningLevel(currentPercent)

	return &PredictionResult{
		RuleID:            ruleID,
		TargetID:          targetID,
		CurrentUsage:      currentUsage,
		MaxBytes:          maxBytes,
		CurrentPercent:    currentPercent,
		DaysUntilFull:     daysUntilFull,
		EstimatedFullDate: estimatedFullDate,
		DailyGrowthRate:   growthRate,
		Trend:             trend,
		Confidence:        confidence,
		WarningLevel:      warningLevel,
	}, nil
}

// calculateGrowthRate 计算增长率.
func (p *Predictor) calculateGrowthRate(records []UsageHistory) (dailyGrowth float64, trend string, confidence float64) {
	if len(records) < 2 {
		return 0, TrendStable, 0
	}

	// 使用线性回归计算增长率
	var sumX, sumY, sumXY, sumX2 float64
	n := float64(len(records))

	for i, record := range records {
		x := float64(i)
		y := float64(record.UsedBytes)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 计算斜率（bytes per data point）
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, TrendStable, 0
	}

	slope := (n*sumXY - sumX*sumY) / denominator

	// 转换为每日增长率（假设数据点间隔为小时）
	dailyGrowth = slope * 24 // 每小时增长 * 24 = 每日增长

	// 计算R²确定置信度
	meanY := sumY / n
	var ssTotal, ssResidual float64
	for i, record := range records {
		predictedY := (sumY-sumX*slope)/n + slope*float64(i)
		ssTotal += math.Pow(float64(record.UsedBytes)-meanY, 2)
		ssResidual += math.Pow(float64(record.UsedBytes)-predictedY, 2)
	}

	rSquared := 0.0
	if ssTotal > 0 {
		rSquared = 1 - (ssResidual / ssTotal)
	}
	confidence = math.Max(0, math.Min(1, rSquared))

	// 确定趋势
	if dailyGrowth > 1024*1024*10 { // 每天增长超过10MB认为在增长
		trend = TrendGrowing
	} else if dailyGrowth < -1024*1024*10 { // 每天减少超过10MB认为在下降
		trend = TrendDeclining
	} else {
		trend = TrendStable
	}

	return dailyGrowth, trend, confidence
}

// GetHistory 获取历史数据.
func (p *Predictor) GetHistory(targetID string) []UsageHistory {
	p.mu.RLock()
	defer p.mu.RUnlock()

	records := p.history[targetID]
	if records == nil {
		return []UsageHistory{}
	}

	// 返回副本
	result := make([]UsageHistory, len(records))
	copy(result, records)
	return result
}

// GetHistoryStats 获取历史统计.
func (p *Predictor) GetHistoryStats(targetID string) map[string]interface{} {
	p.mu.RLock()
	records := p.history[targetID]
	p.mu.RUnlock()

	if len(records) == 0 {
		return map[string]interface{}{
			"count":        0,
			"min_usage":    0,
			"max_usage":    0,
			"avg_usage":    0,
			"first_record": nil,
			"last_record":  nil,
		}
	}

	var minUsage, maxUsage, sumUsage int64 = records[0].UsedBytes, records[0].UsedBytes, 0
	for _, r := range records {
		if r.UsedBytes < minUsage {
			minUsage = r.UsedBytes
		}
		if r.UsedBytes > maxUsage {
			maxUsage = r.UsedBytes
		}
		sumUsage += r.UsedBytes
	}

	return map[string]interface{}{
		"count":        len(records),
		"min_usage":    minUsage,
		"max_usage":    maxUsage,
		"avg_usage":    sumUsage / int64(len(records)),
		"first_record": records[0].Timestamp,
		"last_record":  records[len(records)-1].Timestamp,
	}
}

// ClearHistory 清除历史数据.
func (p *Predictor) ClearHistory(targetID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.history, targetID)
}

// ClearAllHistory 清除所有历史数据.
func (p *Predictor) ClearAllHistory() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.history = make(map[string][]UsageHistory)
}

// PredictAll 预测所有规则.
func (p *Predictor) PredictAll(rules []*QuotaRule, getUsageFunc func(targetType, targetID string) (int64, error)) []*PredictionResult {
	results := make([]*PredictionResult, 0)

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// 获取当前使用量
		usedBytes, err := getUsageFunc(rule.TargetType, rule.TargetID)
		if err != nil {
			continue
		}

		// 记录使用量
		p.RecordUsage(rule.TargetID, usedBytes)

		// 进行预测
		result, err := p.Predict(rule.ID, rule.TargetID, rule.MaxBytes)
		if err != nil {
			continue
		}

		results = append(results, result)
	}

	return results
}

// SetConfig 设置预测配置.
func (p *Predictor) SetConfig(config ForecastConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if config.HistoryDays <= 0 {
		config.HistoryDays = 30
	}
	if config.MinDataPoints <= 0 {
		config.MinDataPoints = 7
	}

	p.config = config
	p.maxItems = config.HistoryDays * 24
}

// GetConfig 获取预测配置.
func (p *Predictor) GetConfig() ForecastConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}
