// Package quota 提供存储配额管理功能
package quota

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

// ========== 趋势数据增强 ==========

// TrendConfig 趋势配置.
type TrendConfig struct {
	// 数据采集配置
	CollectInterval time.Duration `json:"collect_interval"` // 采集间隔
	MaxDataPoints   int           `json:"max_data_points"`  // 最大数据点数
	MaxHistoryAge   time.Duration `json:"max_history_age"`  // 最大历史数据保留时间

	// 聚合配置
	AggregationLevels []AggregationLevel `json:"aggregation_levels"` // 聚合级别

	// 预测配置
	PredictionEnabled          bool   `json:"prediction_enabled"`             // 是否启用预测
	PredictionMethod           string `json:"prediction_method"`              // 预测方法：linear, exponential, moving_avg
	MovingAvgWindow            int    `json:"moving_avg_window"`              // 移动平均窗口
	MinDataPointsForPrediction int    `json:"min_data_points_for_prediction"` // 预测所需最小数据点数

	// 持久化配置
	PersistEnabled  bool          `json:"persist_enabled"`  // 是否持久化
	PersistPath     string        `json:"persist_path"`     // 持久化路径
	PersistInterval time.Duration `json:"persist_interval"` // 持久化间隔
}

// AggregationLevel 聚合级别.
type AggregationLevel struct {
	Name      string        `json:"name"`      // 级别名称：hourly, daily, weekly, monthly
	Duration  time.Duration `json:"duration"`  // 聚合时长
	Retention time.Duration `json:"retention"` // 保留时间
}

// DefaultTrendConfig 默认趋势配置.
func DefaultTrendConfig() TrendConfig {
	return TrendConfig{
		CollectInterval:            5 * time.Minute,
		MaxDataPoints:              2016, // 7天 * 24小时 * 12 (每5分钟一个点)
		MaxHistoryAge:              365 * 24 * time.Hour,
		PredictionEnabled:          true,
		PredictionMethod:           "linear",
		MovingAvgWindow:            12, // 1小时的移动平均
		MinDataPointsForPrediction: 24,
		PersistEnabled:             true,
		PersistInterval:            1 * time.Hour,
		AggregationLevels: []AggregationLevel{
			{Name: "hourly", Duration: time.Hour, Retention: 24 * time.Hour},
			{Name: "daily", Duration: 24 * time.Hour, Retention: 30 * 24 * time.Hour},
			{Name: "weekly", Duration: 7 * 24 * time.Hour, Retention: 90 * 24 * time.Hour},
			{Name: "monthly", Duration: 30 * 24 * time.Hour, Retention: 365 * 24 * time.Hour},
		},
	}
}

// TrendDataPointExtended 扩展的趋势数据点.
type TrendDataPointExtended struct {
	QuotaID        string    `json:"quota_id,omitempty"`
	Timestamp      time.Time `json:"timestamp"`
	UsedBytes      uint64    `json:"used_bytes"`
	UsagePercent   float64   `json:"usage_percent"`
	AvailableBytes uint64    `json:"available_bytes"`
	FileCount      int       `json:"file_count,omitempty"`
	DirCount       int       `json:"dir_count,omitempty"`
	WriteOps       int       `json:"write_ops,omitempty"`
	ReadOps        int       `json:"read_ops,omitempty"`
	WriteBytes     uint64    `json:"write_bytes,omitempty"`
	ReadBytes      uint64    `json:"read_bytes,omitempty"`
}

// AggregatedTrendPoint 聚合趋势数据点.
type AggregatedTrendPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	MinUsedBytes    uint64    `json:"min_used_bytes"`
	MaxUsedBytes    uint64    `json:"max_used_bytes"`
	AvgUsedBytes    float64   `json:"avg_used_bytes"`
	MinUsagePercent float64   `json:"min_usage_percent"`
	MaxUsagePercent float64   `json:"max_usage_percent"`
	AvgUsagePercent float64   `json:"avg_usage_percent"`
	DataPointCount  int       `json:"data_point_count"`
}

// TrendPrediction 趋势预测结果.
type TrendPrediction struct {
	QuotaID            string           `json:"quota_id"`
	TargetName         string           `json:"target_name"`
	Method             string           `json:"method"`
	PredictedAt        time.Time        `json:"predicted_at"`
	PredictionPoints   []PredictedPoint `json:"prediction_points"`
	GrowthRate         float64          `json:"growth_rate"`          // 字节/天
	GrowthPercentDaily float64          `json:"growth_percent_daily"` // 日增长百分比
	DaysToSoftLimit    int              `json:"days_to_soft_limit,omitempty"`
	DaysToHardLimit    int              `json:"days_to_hard_limit,omitempty"`
	ProjectedSoftDate  *time.Time       `json:"projected_soft_date,omitempty"`
	ProjectedHardDate  *time.Time       `json:"projected_hard_date,omitempty"`
	Confidence         float64          `json:"confidence"` // 预测置信度 0-1
	WarningMessage     string           `json:"warning_message,omitempty"`
}

// PredictedPoint 预测数据点.
type PredictedPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsedBytes    uint64    `json:"used_bytes"`
	UsagePercent float64   `json:"usage_percent"`
	IsEstimate   bool      `json:"is_estimate"`
}

// TrendAnalysisReport 趋势分析报告.
type TrendAnalysisReport struct {
	QuotaID         string                 `json:"quota_id"`
	TargetName      string                 `json:"target_name"`
	GeneratedAt     time.Time              `json:"generated_at"`
	PeriodStart     time.Time              `json:"period_start"`
	PeriodEnd       time.Time              `json:"period_end"`
	Duration        time.Duration          `json:"duration"`
	DataPointCount  int                    `json:"data_point_count"`
	CurrentUsage    TrendDataPointExtended `json:"current_usage"`
	Statistics      TrendStatistics        `json:"statistics"`
	Patterns        []TrendPattern         `json:"patterns"`
	Prediction      *TrendPrediction       `json:"prediction,omitempty"`
	Recommendations []TrendRecommendation  `json:"recommendations"`
}

// TrendStatistics 趋势统计.
type TrendStatistics struct {
	// 使用量统计
	MinUsedBytes    uint64  `json:"min_used_bytes"`
	MaxUsedBytes    uint64  `json:"max_used_bytes"`
	AvgUsedBytes    float64 `json:"avg_used_bytes"`
	StdDevUsedBytes float64 `json:"std_dev_used_bytes"`

	// 使用率统计
	MinUsagePercent    float64 `json:"min_usage_percent"`
	MaxUsagePercent    float64 `json:"max_usage_percent"`
	AvgUsagePercent    float64 `json:"avg_usage_percent"`
	StdDevUsagePercent float64 `json:"std_dev_usage_percent"`

	// 增长分析
	TotalGrowthBytes   uint64  `json:"total_growth_bytes"`
	TotalGrowthPercent float64 `json:"total_growth_percent"`
	DailyGrowthRate    float64 `json:"daily_growth_rate"`    // 字节/天
	DailyGrowthPercent float64 `json:"daily_growth_percent"` // 百分比/天

	// 变化统计
	IncreasingDays int     `json:"increasing_days"`
	DecreasingDays int     `json:"decreasing_days"`
	StableDays     int     `json:"stable_days"`
	Volatility     float64 `json:"volatility"` // 波动率
}

// TrendPattern 趋势模式.
type TrendPattern struct {
	Type        string    `json:"type"` // daily_peak, weekly_peak, steady_growth, fluctuation
	Name        string    `json:"name"`
	Description string    `json:"description"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	Value       float64   `json:"value,omitempty"`
	Confidence  float64   `json:"confidence"`
}

// TrendRecommendation 趋势建议.
type TrendRecommendation struct {
	Type        string `json:"type"`     // increase_quota, cleanup, monitor, warning
	Priority    string `json:"priority"` // low, medium, high, critical
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action,omitempty"`
	Impact      string `json:"impact,omitempty"`
}

// TrendDataManager 趋势数据管理器.
type TrendDataManager struct {
	mu             sync.RWMutex
	config         TrendConfig
	rawData        map[string][]TrendDataPointExtended          // quotaID -> 原始数据点
	aggregatedData map[string]map[string][]AggregatedTrendPoint // quotaID -> levelName -> 聚合数据
	quotaMgr       *Manager
	persistTicker  *time.Ticker
	stopChan       chan struct{}
}

// NewTrendDataManager 创建趋势数据管理器.
func NewTrendDataManager(quotaMgr *Manager, config TrendConfig) *TrendDataManager {
	return &TrendDataManager{
		config:         config,
		rawData:        make(map[string][]TrendDataPointExtended),
		aggregatedData: make(map[string]map[string][]AggregatedTrendPoint),
		quotaMgr:       quotaMgr,
		stopChan:       make(chan struct{}),
	}
}

// Start 启动趋势数据管理.
func (m *TrendDataManager) Start() {
	if m.config.PersistEnabled {
		m.persistTicker = time.NewTicker(m.config.PersistInterval)
		go m.runPersist()
	}
}

// Stop 停止趋势数据管理.
func (m *TrendDataManager) Stop() {
	close(m.stopChan)
	if m.persistTicker != nil {
		m.persistTicker.Stop()
	}
	// 最后一次持久化
	if m.config.PersistEnabled {
		m.persist()
	}
}

// runPersist 运行持久化.
func (m *TrendDataManager) runPersist() {
	for {
		select {
		case <-m.stopChan:
			return
		case <-m.persistTicker.C:
			m.persist()
		}
	}
}

// RecordDataPoint 记录数据点.
func (m *TrendDataManager) RecordDataPoint(quotaID string, point TrendDataPointExtended) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 添加原始数据
	data := m.rawData[quotaID]
	data = append(data, point)

	// 限制数据点数量
	if len(data) > m.config.MaxDataPoints {
		data = data[len(data)-m.config.MaxDataPoints:]
	}
	m.rawData[quotaID] = data

	// 更新聚合数据
	m.updateAggregations(quotaID, point)
}

// updateAggregations 更新聚合数据.
func (m *TrendDataManager) updateAggregations(quotaID string, point TrendDataPointExtended) {
	if m.aggregatedData[quotaID] == nil {
		m.aggregatedData[quotaID] = make(map[string][]AggregatedTrendPoint)
	}

	for _, level := range m.config.AggregationLevels {
		// 计算当前聚合时间窗口
		windowStart := point.Timestamp.Truncate(level.Duration)
		aggData := m.aggregatedData[quotaID][level.Name]

		// 查找或创建聚合点
		var aggPoint *AggregatedTrendPoint
		for i := range aggData {
			if aggData[i].Timestamp.Equal(windowStart) {
				aggPoint = &aggData[i]
				break
			}
		}

		if aggPoint == nil {
			aggPoint = &AggregatedTrendPoint{
				Timestamp:       windowStart,
				MinUsedBytes:    point.UsedBytes,
				MaxUsedBytes:    point.UsedBytes,
				MinUsagePercent: point.UsagePercent,
				MaxUsagePercent: point.UsagePercent,
				AvgUsedBytes:    float64(point.UsedBytes),
				AvgUsagePercent: point.UsagePercent,
				DataPointCount:  1,
			}
			aggData = append(aggData, *aggPoint)
		} else {
			// 更新聚合点
			aggPoint.DataPointCount++
			if point.UsedBytes < aggPoint.MinUsedBytes {
				aggPoint.MinUsedBytes = point.UsedBytes
			}
			if point.UsedBytes > aggPoint.MaxUsedBytes {
				aggPoint.MaxUsedBytes = point.UsedBytes
			}
			if point.UsagePercent < aggPoint.MinUsagePercent {
				aggPoint.MinUsagePercent = point.UsagePercent
			}
			if point.UsagePercent > aggPoint.MaxUsagePercent {
				aggPoint.MaxUsagePercent = point.UsagePercent
			}
			// 更新平均值
			aggPoint.AvgUsedBytes = (aggPoint.AvgUsedBytes*float64(aggPoint.DataPointCount-1) + float64(point.UsedBytes)) / float64(aggPoint.DataPointCount)
			aggPoint.AvgUsagePercent = (aggPoint.AvgUsagePercent*float64(aggPoint.DataPointCount-1) + point.UsagePercent) / float64(aggPoint.DataPointCount)
		}

		// 清理过期数据
		cutoff := time.Now().Add(-level.Retention)
		filtered := make([]AggregatedTrendPoint, 0)
		for _, p := range aggData {
			if p.Timestamp.After(cutoff) {
				filtered = append(filtered, p)
			}
		}
		m.aggregatedData[quotaID][level.Name] = filtered
	}
}

// GetRawData 获取原始数据.
func (m *TrendDataManager) GetRawData(quotaID string, duration time.Duration) []TrendDataPointExtended {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := m.rawData[quotaID]
	if len(data) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	result := make([]TrendDataPointExtended, 0)
	for _, p := range data {
		if p.Timestamp.After(cutoff) {
			result = append(result, p)
		}
	}

	return result
}

// GetAggregatedData 获取聚合数据.
func (m *TrendDataManager) GetAggregatedData(quotaID string, levelName string, duration time.Duration) []AggregatedTrendPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quotaData := m.aggregatedData[quotaID]
	if quotaData == nil {
		return nil
	}

	data := quotaData[levelName]
	if len(data) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	result := make([]AggregatedTrendPoint, 0)
	for _, p := range data {
		if p.Timestamp.After(cutoff) {
			result = append(result, p)
		}
	}

	return result
}

// GetTrendStats 获取趋势统计.
func (m *TrendDataManager) GetTrendStats(quotaID string, duration time.Duration) *TrendStatistics {
	data := m.GetRawData(quotaID, duration)
	if len(data) < 2 {
		return nil
	}

	stats := &TrendStatistics{}
	stats.MinUsedBytes = data[0].UsedBytes
	stats.MaxUsedBytes = data[0].UsedBytes
	stats.MinUsagePercent = data[0].UsagePercent
	stats.MaxUsagePercent = data[0].UsagePercent

	var sumUsed float64
	var sumPercent float64

	for _, p := range data {
		sumUsed += float64(p.UsedBytes)
		sumPercent += p.UsagePercent

		if p.UsedBytes < stats.MinUsedBytes {
			stats.MinUsedBytes = p.UsedBytes
		}
		if p.UsedBytes > stats.MaxUsedBytes {
			stats.MaxUsedBytes = p.UsedBytes
		}
		if p.UsagePercent < stats.MinUsagePercent {
			stats.MinUsagePercent = p.UsagePercent
		}
		if p.UsagePercent > stats.MaxUsagePercent {
			stats.MaxUsagePercent = p.UsagePercent
		}
	}

	stats.AvgUsedBytes = sumUsed / float64(len(data))
	stats.AvgUsagePercent = sumPercent / float64(len(data))

	// 计算标准差
	var varianceUsed, variancePercent float64
	for _, p := range data {
		diffUsed := float64(p.UsedBytes) - stats.AvgUsedBytes
		diffPercent := p.UsagePercent - stats.AvgUsagePercent
		varianceUsed += diffUsed * diffUsed
		variancePercent += diffPercent * diffPercent
	}
	stats.StdDevUsedBytes = math.Sqrt(varianceUsed / float64(len(data)))
	stats.StdDevUsagePercent = math.Sqrt(variancePercent / float64(len(data)))

	// 计算增长
	first := data[0]
	last := data[len(data)-1]
	stats.TotalGrowthBytes = last.UsedBytes - first.UsedBytes
	if first.UsedBytes > 0 {
		stats.TotalGrowthPercent = float64(stats.TotalGrowthBytes) / float64(first.UsedBytes) * 100
	}

	// 计算日增长率
	days := last.Timestamp.Sub(first.Timestamp).Hours() / 24
	if days > 0 {
		stats.DailyGrowthRate = float64(stats.TotalGrowthBytes) / days
		if stats.AvgUsedBytes > 0 {
			stats.DailyGrowthPercent = stats.DailyGrowthRate / stats.AvgUsedBytes * 100
		}
	}

	// 计算波动率
	if stats.AvgUsedBytes > 0 {
		stats.Volatility = stats.StdDevUsedBytes / stats.AvgUsedBytes * 100
	}

	// 分析增加/减少/稳定天数
	for i := 1; i < len(data); i++ {
		if data[i].UsedBytes > data[i-1].UsedBytes {
			stats.IncreasingDays++
		} else if data[i].UsedBytes < data[i-1].UsedBytes {
			stats.DecreasingDays++
		} else {
			stats.StableDays++
		}
	}

	return stats
}

// Predict 预测趋势.
func (m *TrendDataManager) Predict(quotaID string, predictDays int) *TrendPrediction {
	m.mu.RLock()
	data := m.rawData[quotaID]
	m.mu.RUnlock()

	if len(data) < m.config.MinDataPointsForPrediction {
		return nil
	}

	prediction := &TrendPrediction{
		QuotaID:          quotaID,
		Method:           m.config.PredictionMethod,
		PredictedAt:      time.Now(),
		PredictionPoints: make([]PredictedPoint, 0),
	}

	// 获取配额信息
	m.quotaMgr.mu.RLock()
	quota, exists := m.quotaMgr.quotas[quotaID]
	m.quotaMgr.mu.RUnlock()

	if !exists {
		return nil
	}

	prediction.TargetName = quota.TargetName

	// 计算增长率（线性回归）
	stats := m.GetTrendStats(quotaID, 24*time.Hour*7) // 使用最近7天数据
	if stats == nil {
		return nil
	}

	prediction.GrowthRate = stats.DailyGrowthRate
	prediction.GrowthPercentDaily = stats.DailyGrowthPercent

	// 获取最新数据点
	latest := data[len(data)-1]

	// 生成预测点
	for i := 1; i <= predictDays; i++ {
		predictedBytes := latest.UsedBytes + uint64(float64(i)*prediction.GrowthRate)
		predictedPercent := float64(predictedBytes) / float64(quota.HardLimit) * 100

		prediction.PredictionPoints = append(prediction.PredictionPoints, PredictedPoint{
			Timestamp:    latest.Timestamp.AddDate(0, 0, i),
			UsedBytes:    predictedBytes,
			UsagePercent: predictedPercent,
			IsEstimate:   true,
		})
	}

	// 计算达到软限制和硬限制的时间
	if prediction.GrowthRate > 0 {
		// 到达硬限制
		remainingToHard := float64(quota.HardLimit) - float64(latest.UsedBytes)
		prediction.DaysToHardLimit = int(remainingToHard / prediction.GrowthRate)
		if prediction.DaysToHardLimit > 0 {
			date := time.Now().AddDate(0, 0, prediction.DaysToHardLimit)
			prediction.ProjectedHardDate = &date
		}

		// 到达软限制
		if quota.SoftLimit > 0 && latest.UsedBytes < quota.SoftLimit {
			remainingToSoft := float64(quota.SoftLimit) - float64(latest.UsedBytes)
			prediction.DaysToSoftLimit = int(remainingToSoft / prediction.GrowthRate)
			if prediction.DaysToSoftLimit > 0 {
				date := time.Now().AddDate(0, 0, prediction.DaysToSoftLimit)
				prediction.ProjectedSoftDate = &date
			}
		}

		// 计算置信度
		prediction.Confidence = m.calculateConfidence(data, stats)

		// 生成警告消息
		if prediction.DaysToHardLimit > 0 && prediction.DaysToHardLimit <= 30 {
			prediction.WarningMessage = fmt.Sprintf("按当前增长率，预计 %d 天后将达到配额上限", prediction.DaysToHardLimit)
		}
	}

	return prediction
}

// calculateConfidence 计算预测置信度.
func (m *TrendDataManager) calculateConfidence(data []TrendDataPointExtended, stats *TrendStatistics) float64 {
	if len(data) < 10 {
		return 0.3
	}

	confidence := 1.0

	// 波动率影响
	if stats.Volatility > 20 {
		confidence *= 0.5
	} else if stats.Volatility > 10 {
		confidence *= 0.7
	}

	// 数据点数量影响
	if len(data) < 24 {
		confidence *= 0.6
	} else if len(data) < 72 {
		confidence *= 0.8
	}

	// 增长一致性影响
	consistency := float64(stats.IncreasingDays) / float64(len(data)-1)
	if consistency < 0.5 {
		confidence *= 0.7
	}

	return confidence
}

// GenerateReport 生成趋势分析报告.
func (m *TrendDataManager) GenerateReport(quotaID string, duration time.Duration) *TrendAnalysisReport {
	data := m.GetRawData(quotaID, duration)
	if len(data) == 0 {
		return nil
	}

	m.quotaMgr.mu.RLock()
	quota, exists := m.quotaMgr.quotas[quotaID]
	m.quotaMgr.mu.RUnlock()

	if !exists {
		return nil
	}

	report := &TrendAnalysisReport{
		QuotaID:         quotaID,
		TargetName:      quota.TargetName,
		GeneratedAt:     time.Now(),
		PeriodStart:     data[0].Timestamp,
		PeriodEnd:       data[len(data)-1].Timestamp,
		Duration:        duration,
		DataPointCount:  len(data),
		CurrentUsage:    data[len(data)-1],
		Recommendations: make([]TrendRecommendation, 0),
	}

	report.Statistics = *m.GetTrendStats(quotaID, duration)

	// 检测模式
	report.Patterns = m.detectPatterns(data)

	// 生成预测
	if m.config.PredictionEnabled && len(data) >= m.config.MinDataPointsForPrediction {
		report.Prediction = m.Predict(quotaID, 30)
	}

	// 生成建议
	report.Recommendations = m.generateRecommendations(report, quota)

	return report
}

// detectPatterns 检测趋势模式.
func (m *TrendDataManager) detectPatterns(data []TrendDataPointExtended) []TrendPattern {
	patterns := make([]TrendPattern, 0)

	if len(data) < 24 {
		return patterns
	}

	// 直接计算统计信息而不是调用 GetTrendStats
	// 计算增长和波动
	first := data[0]
	last := data[len(data)-1]
	days := last.Timestamp.Sub(first.Timestamp).Hours() / 24

	var totalGrowthBytes uint64
	var dailyGrowthPercent float64
	var increasingDays, decreasingDays int
	var sumUsed float64

	for i, p := range data {
		sumUsed += float64(p.UsedBytes)
		if i > 0 {
			if p.UsedBytes > data[i-1].UsedBytes {
				increasingDays++
			} else if p.UsedBytes < data[i-1].UsedBytes {
				decreasingDays++
			}
		}
	}

	totalGrowthBytes = last.UsedBytes - first.UsedBytes
	avgUsed := sumUsed / float64(len(data))
	if days > 0 && avgUsed > 0 {
		dailyGrowthPercent = (float64(totalGrowthBytes) / days) / avgUsed * 100
	}

	// 检测稳定增长模式
	if dailyGrowthPercent > 0.5 {
		consistency := float64(increasingDays) / float64(len(data)-1)
		if consistency > 0.7 {
			patterns = append(patterns, TrendPattern{
				Type:        "steady_growth",
				Name:        "稳定增长",
				Description: fmt.Sprintf("存储使用以 %.2f%%/天的速度稳定增长", dailyGrowthPercent),
				Value:       dailyGrowthPercent,
				Confidence:  consistency,
			})
		}
	}

	// 计算波动率
	var variance float64
	for _, p := range data {
		diff := float64(p.UsedBytes) - avgUsed
		variance += diff * diff
	}
	volatility := math.Sqrt(variance/float64(len(data))) / avgUsed * 100

	// 检测波动模式
	if volatility > 10 {
		patterns = append(patterns, TrendPattern{
			Type:        "fluctuation",
			Name:        "高波动",
			Description: fmt.Sprintf("存储使用波动较大 (%.1f%%)", volatility),
			Value:       volatility,
			Confidence:  0.8,
		})
	}

	// 检测每日峰值模式（简单检测）
	if len(data) >= 48 {
		hourlyAvg := make(map[int][]float64)
		for _, p := range data {
			hour := p.Timestamp.Hour()
			hourlyAvg[hour] = append(hourlyAvg[hour], p.UsagePercent)
		}

		var maxHour int
		var maxAvg float64
		for hour, vals := range hourlyAvg {
			var sum float64
			for _, v := range vals {
				sum += v
			}
			avg := sum / float64(len(vals))
			if avg > maxAvg {
				maxAvg = avg
				maxHour = hour
			}
		}

		if maxAvg > 0 {
			patterns = append(patterns, TrendPattern{
				Type:        "daily_peak",
				Name:        "日常峰值",
				Description: fmt.Sprintf("通常在 %d:00 左右达到使用高峰", maxHour),
				Value:       float64(maxHour),
				Confidence:  0.6,
			})
		}
	}

	return patterns
}

// generateRecommendations 生成趋势建议.
func (m *TrendDataManager) generateRecommendations(report *TrendAnalysisReport, quota *Quota) []TrendRecommendation {
	recs := make([]TrendRecommendation, 0)

	// 检查当前使用率
	current := report.CurrentUsage.UsagePercent
	if current >= 90 {
		recs = append(recs, TrendRecommendation{
			Type:        "warning",
			Priority:    "critical",
			Title:       "存储空间即将耗尽",
			Description: fmt.Sprintf("当前使用率已达 %.1f%%，建议立即处理", current),
			Action:      "清理不需要的文件或增加配额",
			Impact:      "可能导致写入失败",
		})
	} else if current >= 80 {
		recs = append(recs, TrendRecommendation{
			Type:        "warning",
			Priority:    "high",
			Title:       "存储空间紧张",
			Description: fmt.Sprintf("当前使用率已达 %.1f%%，建议尽快处理", current),
			Action:      "检查并清理大文件或过期文件",
			Impact:      "可能影响系统性能",
		})
	}

	// 检查预测
	if report.Prediction != nil {
		if report.Prediction.DaysToHardLimit > 0 && report.Prediction.DaysToHardLimit <= 30 {
			priority := "medium"
			if report.Prediction.DaysToHardLimit <= 7 {
				priority = "high"
			}
			recs = append(recs, TrendRecommendation{
				Type:        "increase_quota",
				Priority:    priority,
				Title:       "即将达到配额上限",
				Description: fmt.Sprintf("按当前趋势，预计 %d 天后将达到配额上限", report.Prediction.DaysToHardLimit),
				Action:      "增加配额限制或实施自动清理策略",
				Impact:      fmt.Sprintf("预计在 %s 左右填满", report.Prediction.ProjectedHardDate.Format("2006-01-02")),
			})
		}
	}

	// 检查波动
	if report.Statistics.Volatility > 20 {
		recs = append(recs, TrendRecommendation{
			Type:        "monitor",
			Priority:    "low",
			Title:       "使用模式不稳定",
			Description: "存储使用波动较大，建议关注使用模式",
			Action:      "设置更细粒度的监控",
			Impact:      "可能存在周期性大文件写入",
		})
	}

	return recs
}

// GetChartData 获取图表数据.
func (m *TrendDataManager) GetChartData(quotaID string, duration time.Duration, granularity string) map[string]interface{} {
	result := map[string]interface{}{
		"quota_id":  quotaID,
		"duration":  duration.String(),
		"timestamp": time.Now(),
	}

	// 获取原始数据
	rawData := m.GetRawData(quotaID, duration)
	if len(rawData) == 0 {
		return result
	}

	// 构建图表数据
	labels := make([]string, 0, len(rawData))
	usedBytes := make([]uint64, 0, len(rawData))
	usagePercent := make([]float64, 0, len(rawData))

	for _, p := range rawData {
		labels = append(labels, p.Timestamp.Format("2006-01-02 15:04"))
		usedBytes = append(usedBytes, p.UsedBytes)
		usagePercent = append(usagePercent, p.UsagePercent)
	}

	result["labels"] = labels
	result["used_bytes"] = usedBytes
	result["usage_percent"] = usagePercent

	// 获取聚合数据
	if granularity != "" && granularity != "raw" {
		aggData := m.GetAggregatedData(quotaID, granularity, duration)
		if len(aggData) > 0 {
			aggLabels := make([]string, 0, len(aggData))
			avgUsage := make([]float64, 0, len(aggData))
			minUsage := make([]float64, 0, len(aggData))
			maxUsage := make([]float64, 0, len(aggData))

			for _, p := range aggData {
				aggLabels = append(aggLabels, p.Timestamp.Format("2006-01-02 15:04"))
				avgUsage = append(avgUsage, p.AvgUsagePercent)
				minUsage = append(minUsage, p.MinUsagePercent)
				maxUsage = append(maxUsage, p.MaxUsagePercent)
			}

			result["aggregated"] = map[string]interface{}{
				"labels":    aggLabels,
				"avg_usage": avgUsage,
				"min_usage": minUsage,
				"max_usage": maxUsage,
			}
		}
	}

	// 添加统计信息
	result["statistics"] = m.GetTrendStats(quotaID, duration)

	// 添加预测
	if m.config.PredictionEnabled {
		result["prediction"] = m.Predict(quotaID, 7)
	}

	return result
}

// persist 持久化数据.
func (m *TrendDataManager) persist() {
	if m.config.PersistPath == "" {
		return
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	data := struct {
		RawData        map[string][]TrendDataPointExtended          `json:"raw_data"`
		AggregatedData map[string]map[string][]AggregatedTrendPoint `json:"aggregated_data"`
		SavedAt        time.Time                                    `json:"saved_at"`
	}{
		RawData:        m.rawData,
		AggregatedData: m.aggregatedData,
		SavedAt:        time.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	_ = os.WriteFile(m.config.PersistPath, jsonData, 0600)
}

// Load 加载数据.
func (m *TrendDataManager) Load() error {
	if m.config.PersistPath == "" {
		return nil
	}

	if _, err := os.Stat(m.config.PersistPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.config.PersistPath)
	if err != nil {
		return err
	}

	var loaded struct {
		RawData        map[string][]TrendDataPointExtended          `json:"raw_data"`
		AggregatedData map[string]map[string][]AggregatedTrendPoint `json:"aggregated_data"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.rawData = loaded.RawData
	m.aggregatedData = loaded.AggregatedData

	return nil
}

// CleanupOldData 清理过期数据.
func (m *TrendDataManager) CleanupOldData() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-m.config.MaxHistoryAge)

	for quotaID, data := range m.rawData {
		filtered := make([]TrendDataPointExtended, 0)
		for _, p := range data {
			if p.Timestamp.After(cutoff) {
				filtered = append(filtered, p)
			}
		}
		m.rawData[quotaID] = filtered
	}
}
