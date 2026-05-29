// Package storageforecast 提供存储趋势预测功能
// 对标群晖 Active Insight 存储趋势预警
// 特性：容量趋势分析、增长预测、告警阈值、智能建议
package storageforecast

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"
	AlertWarning  AlertLevel = "warning"
	AlertCritical AlertLevel = "critical"
	AlertFull     AlertLevel = "full"
)

// TrendDirection 趋势方向
type TrendDirection string

const (
	TrendIncreasing TrendDirection = "increasing"
	TrendDecreasing TrendDirection = "decreasing"
	TrendStable     TrendDirection = "stable"
	TrendUnknown    TrendDirection = "unknown"
)

// StoragePool 存储池
type StoragePool struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	TotalBytes  int64     `json:"total_bytes"`
	UsedBytes   int64     `json:"used_bytes"`
	FreeBytes   int64     `json:"free_bytes"`
	UsedPercent float64   `json:"used_percent"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// UsageSnapshot 使用量快照
type UsageSnapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	PoolID      string    `json:"pool_id"`
	TotalBytes  int64     `json:"total_bytes"`
	UsedBytes   int64     `json:"used_bytes"`
	FreeBytes   int64     `json:"free_bytes"`
	UsedPercent float64   `json:"used_percent"`
}

// ForecastResult 预测结果
type ForecastResult struct {
	PoolID           string         `json:"pool_id"`
	PoolName         string         `json:"pool_name"`
	CurrentUsage     float64        `json:"current_usage"`
	Trend            TrendDirection `json:"trend"`
	DailyGrowthBytes int64          `json:"daily_growth_bytes"`
	DailyGrowthRate  float64        `json:"daily_growth_rate"`
	DaysUntilFull    int            `json:"days_until_full"`
	EstimatedFullDate *time.Time    `json:"estimated_full_date,omitempty"`
	Confidence       float64        `json:"confidence"`
	AlertLevel       AlertLevel     `json:"alert_level"`
	Suggestions      []string       `json:"suggestions"`
}

// ForecastConfig 配置
type ForecastConfig struct {
	Enabled           bool    `json:"enabled"`
	WarningThreshold  float64 `json:"warning_threshold"`   // 80%
	CriticalThreshold float64 `json:"critical_threshold"`  // 90%
	FullThreshold     float64 `json:"full_threshold"`      // 95%
	ForecastDays      int     `json:"forecast_days"`       // 预测天数
	SnapshotInterval  time.Duration `json:"snapshot_interval"`
	MaxSnapshots      int     `json:"max_snapshots"`
	MinDataPoints     int     `json:"min_data_points"`     // 最少数据点
}

// DefaultConfig 返回默认配置
func DefaultConfig() ForecastConfig {
	return ForecastConfig{
		Enabled:           true,
		WarningThreshold:  80.0,
		CriticalThreshold: 90.0,
		FullThreshold:     95.0,
		ForecastDays:      90,
		SnapshotInterval:  1 * time.Hour,
		MaxSnapshots:      2160, // 90 天 * 24 小时
		MinDataPoints:     7,    // 至少 7 个数据点
	}
}

// Forecast 存储预测器
type Forecast struct {
	mu        sync.RWMutex
	config    ForecastConfig
	pools     map[string]*StoragePool
	snapshots map[string][]UsageSnapshot // poolID -> snapshots
	alerts    []Alert
	stopCh    chan struct{}
}

// Alert 告警
type Alert struct {
	ID        string     `json:"id"`
	PoolID    string     `json:"pool_id"`
	PoolName  string     `json:"pool_name"`
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	Threshold float64    `json:"threshold"`
	Current   float64    `json:"current"`
	CreatedAt time.Time  `json:"created_at"`
	Dismissed bool       `json:"dismissed"`
}

// NewForecast 创建存储预测器
func NewForecast(config ForecastConfig) *Forecast {
	return &Forecast{
		config:    config,
		pools:     make(map[string]*StoragePool),
		snapshots: make(map[string][]UsageSnapshot),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动预测器
func (f *Forecast) Start(ctx context.Context) error {
	if !f.config.Enabled {
		return nil
	}

	go f.snapshotLoop(ctx)
	go f.forecastLoop(ctx)

	return nil
}

// Stop 停止预测器
func (f *Forecast) Stop() {
	close(f.stopCh)
}

// RegisterPool 注册存储池
func (f *Forecast) RegisterPool(pool StoragePool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pool.UpdatedAt = time.Now()
	f.pools[pool.ID] = &pool
}

// UpdatePoolUsage 更新存储池使用量
func (f *Forecast) UpdatePoolUsage(poolID string, usedBytes int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	pool, exists := f.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	pool.UsedBytes = usedBytes
	pool.FreeBytes = pool.TotalBytes - usedBytes
	pool.UsedPercent = float64(usedBytes) / float64(pool.TotalBytes) * 100
	pool.UpdatedAt = time.Now()

	// 记录快照
	f.addSnapshot(UsageSnapshot{
		Timestamp:   time.Now(),
		PoolID:      poolID,
		TotalBytes:  pool.TotalBytes,
		UsedBytes:   usedBytes,
		FreeBytes:   pool.FreeBytes,
		UsedPercent: pool.UsedPercent,
	})

	// 检查告警
	f.checkAlerts(pool)

	return nil
}

// GetPool 获取存储池信息
func (f *Forecast) GetPool(poolID string) (*StoragePool, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	pool, exists := f.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	return pool, nil
}

// ListPools 列出所有存储池
func (f *Forecast) ListPools() []StoragePool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	pools := make([]StoragePool, 0, len(f.pools))
	for _, p := range f.pools {
		pools = append(pools, *p)
	}
	return pools
}

// GetForecast 获取预测结果
func (f *Forecast) GetForecast(poolID string) (*ForecastResult, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	pool, exists := f.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	snapshots := f.snapshots[poolID]
	if len(snapshots) < f.config.MinDataPoints {
		return &ForecastResult{
			PoolID:       poolID,
			PoolName:     pool.Name,
			CurrentUsage: pool.UsedPercent,
			Trend:        TrendUnknown,
			AlertLevel:   AlertInfo,
			Suggestions:  []string{"数据点不足，请等待更多采样数据"},
			Confidence:   0,
		}, nil
	}

	// 线性回归分析
	growthRate, confidence := f.analyzeTrend(snapshots)
	trend := f.determineTrend(growthRate)

	// 计算预测
	dailyGrowthBytes := int64(growthRate * float64(pool.TotalBytes) / 100)
	var daysUntilFull int
	var estimatedFullDate *time.Time

	if dailyGrowthBytes > 0 {
		if pool.FreeBytes > 0 {
			days := float64(pool.FreeBytes) / float64(dailyGrowthBytes)
			daysUntilFull = int(math.Ceil(days))
			fullDate := time.Now().AddDate(0, 0, daysUntilFull)
			estimatedFullDate = &fullDate
		} else {
			daysUntilFull = 0
		}
	} else if dailyGrowthBytes < 0 {
		daysUntilFull = -1 // 空间在释放
	}

	// 确定告警级别
	alertLevel := f.determineAlertLevel(pool.UsedPercent, daysUntilFull)

	// 生成建议
	suggestions := f.generateSuggestions(pool, alertLevel, daysUntilFull)

	return &ForecastResult{
		PoolID:            poolID,
		PoolName:          pool.Name,
		CurrentUsage:      pool.UsedPercent,
		Trend:             trend,
		DailyGrowthBytes:  dailyGrowthBytes,
		DailyGrowthRate:   growthRate,
		DaysUntilFull:     daysUntilFull,
		EstimatedFullDate: estimatedFullDate,
		Confidence:        confidence,
		AlertLevel:        alertLevel,
		Suggestions:       suggestions,
	}, nil
}

// GetAllForecasts 获取所有存储池的预测
func (f *Forecast) GetAllForecasts() []ForecastResult {
	f.mu.RLock()
	poolIDs := make([]string, 0, len(f.pools))
	for id := range f.pools {
		poolIDs = append(poolIDs, id)
	}
	f.mu.RUnlock()

	results := make([]ForecastResult, 0, len(poolIDs))
	for _, id := range poolIDs {
		result, err := f.GetForecast(id)
		if err == nil {
			results = append(results, *result)
		}
	}
	return results
}

// GetAlerts 获取告警
func (f *Forecast) GetAlerts(dismissed bool) []Alert {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var alerts []Alert
	for _, a := range f.alerts {
		if a.Dismissed == dismissed {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// DismissAlert 忽略告警
func (f *Forecast) DismissAlert(alertID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	for i := range f.alerts {
		if f.alerts[i].ID == alertID {
			f.alerts[i].Dismissed = true
			return nil
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// GetSnapshots 获取快照
func (f *Forecast) GetSnapshots(poolID string, duration time.Duration) []UsageSnapshot {
	f.mu.RLock()
	defer f.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	snapshots := f.snapshots[poolID]

	var result []UsageSnapshot
	for _, s := range snapshots {
		if s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}
	return result
}

// GetStats 获取统计信息
func (f *Forecast) GetStats() map[string]interface{} {
	f.mu.RLock()
	defer f.mu.RUnlock()

	totalSnapshots := 0
	for _, s := range f.snapshots {
		totalSnapshots += len(s)
	}

	activeAlerts := 0
	for _, a := range f.alerts {
		if !a.Dismissed {
			activeAlerts++
		}
	}

	return map[string]interface{}{
		"total_pools":     len(f.pools),
		"total_snapshots": totalSnapshots,
		"active_alerts":   activeAlerts,
		"total_alerts":    len(f.alerts),
	}
}

// addSnapshot 添加快照
func (f *Forecast) addSnapshot(snapshot UsageSnapshot) {
	snapshots := f.snapshots[snapshot.PoolID]
	snapshots = append(snapshots, snapshot)

	// 限制快照数量
	if len(snapshots) > f.config.MaxSnapshots {
		snapshots = snapshots[len(snapshots)-f.config.MaxSnapshots:]
	}

	f.snapshots[snapshot.PoolID] = snapshots
}

// analyzeTrend 分析趋势（线性回归）
func (f *Forecast) analyzeTrend(snapshots []UsageSnapshot) (float64, float64) {
	if len(snapshots) < 2 {
		return 0, 0
	}

	// 使用最近的数据点
	limit := 168 // 7 天 * 24 小时
	if len(snapshots) < limit {
		limit = len(snapshots)
	}
	recent := snapshots[len(snapshots)-limit:]

	// 线性回归
	n := float64(len(recent))
	var sumX, sumY, sumXY, sumX2 float64

	for i, s := range recent {
		x := float64(i)
		y := s.UsedPercent
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, 0
	}

	slope := (n*sumXY - sumX*sumY) / denominator

	// 计算 R²
	meanY := sumY / n
	var ssRes, ssTot float64
	for i, s := range recent {
		x := float64(i)
		predicted := slope*x + (sumY-slope*sumX)/n
		ssRes += (s.UsedPercent - predicted) * (s.UsedPercent - predicted)
		ssTot += (s.UsedPercent - meanY) * (s.UsedPercent - meanY)
	}

	confidence := 0.0
	if ssTot > 0 {
		confidence = 1 - ssRes/ssTot
		if confidence < 0 {
			confidence = 0
		}
	}

	// 转换为每日增长率（百分比/小时 -> 百分比/天）
	dailyRate := slope * 24

	return dailyRate, confidence
}

// determineTrend 确定趋势方向
func (f *Forecast) determineTrend(growthRate float64) TrendDirection {
	threshold := 0.01 // 0.01% 每天
	if growthRate > threshold {
		return TrendIncreasing
	} else if growthRate < -threshold {
		return TrendDecreasing
	}
	return TrendStable
}

// determineAlertLevel 确定告警级别
func (f *Forecast) determineAlertLevel(usage float64, daysUntilFull int) AlertLevel {
	if daysUntilFull >= 0 && daysUntilFull <= 7 {
		return AlertFull
	}
	if usage >= f.config.FullThreshold {
		return AlertFull
	}
	if usage >= f.config.CriticalThreshold || (daysUntilFull >= 0 && daysUntilFull <= 30) {
		return AlertCritical
	}
	if usage >= f.config.WarningThreshold || (daysUntilFull >= 0 && daysUntilFull <= 60) {
		return AlertWarning
	}
	return AlertInfo
}

// generateSuggestions 生成建议
func (f *Forecast) generateSuggestions(pool *StoragePool, level AlertLevel, daysUntilFull int) []string {
	var suggestions []string

	switch level {
	case AlertFull:
		suggestions = append(suggestions, "⚠️ 存储即将满载，请立即清理或扩容")
		suggestions = append(suggestions, "建议删除不需要的文件或归档旧数据")
		suggestions = append(suggestions, "考虑添加新硬盘扩展存储池")
	case AlertCritical:
		suggestions = append(suggestions, "存储使用率较高，建议规划扩容")
		if daysUntilFull > 0 {
			suggestions = append(suggestions, fmt.Sprintf("预计 %d 天后空间耗尽", daysUntilFull))
		}
		suggestions = append(suggestions, "检查并清理大文件和重复数据")
	case AlertWarning:
		suggestions = append(suggestions, "存储使用率超过阈值，请关注")
		suggestions = append(suggestions, "建议定期检查存储使用趋势")
	default:
		suggestions = append(suggestions, "存储状态良好")
	}

	if pool.UsedPercent > 50 {
		suggestions = append(suggestions, "可启用数据压缩节省空间")
		suggestions = append(suggestions, "考虑启用自动归档策略")
	}

	return suggestions
}

// checkAlerts 检查告警
func (f *Forecast) checkAlerts(pool *StoragePool) {
	level := f.determineAlertLevel(pool.UsedPercent, -1)

	if level == AlertInfo {
		return
	}

	// 检查是否已有相同告警
	for _, a := range f.alerts {
		if a.PoolID == pool.ID && a.Level == level && !a.Dismissed {
			return
		}
	}

	alert := Alert{
		ID:        fmt.Sprintf("%s-%s-%d", pool.ID, level, time.Now().Unix()),
		PoolID:    pool.ID,
		PoolName:  pool.Name,
		Level:     level,
		Threshold: f.getThreshold(level),
		Current:   pool.UsedPercent,
		CreatedAt: time.Now(),
	}

	switch level {
	case AlertFull:
		alert.Message = fmt.Sprintf("存储池 %s 使用率 %.1f%%，即将满载！", pool.Name, pool.UsedPercent)
	case AlertCritical:
		alert.Message = fmt.Sprintf("存储池 %s 使用率 %.1f%%，空间紧张", pool.Name, pool.UsedPercent)
	case AlertWarning:
		alert.Message = fmt.Sprintf("存储池 %s 使用率 %.1f%%，请关注", pool.Name, pool.UsedPercent)
	}

	f.alerts = append(f.alerts, alert)
}

// getThreshold 获取阈值
func (f *Forecast) getThreshold(level AlertLevel) float64 {
	switch level {
	case AlertFull:
		return f.config.FullThreshold
	case AlertCritical:
		return f.config.CriticalThreshold
	case AlertWarning:
		return f.config.WarningThreshold
	default:
		return 0
	}
}

// snapshotLoop 快照采集循环
func (f *Forecast) snapshotLoop(ctx context.Context) {
	ticker := time.NewTicker(f.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.collectSnapshots()
		}
	}
}

// forecastLoop 预测循环
func (f *Forecast) forecastLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopCh:
			return
		case <-ticker.C:
			f.runForecasts()
		}
	}
}

// collectSnapshots 采集快照
func (f *Forecast) collectSnapshots() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, pool := range f.pools {
		f.addSnapshot(UsageSnapshot{
			Timestamp:   time.Now(),
			PoolID:      pool.ID,
			TotalBytes:  pool.TotalBytes,
			UsedBytes:   pool.UsedBytes,
			FreeBytes:   pool.FreeBytes,
			UsedPercent: pool.UsedPercent,
		})
	}
}

// runForecasts 运行预测
func (f *Forecast) runForecasts() {
	// 触发预测检查
	f.mu.RLock()
	poolIDs := make([]string, 0, len(f.pools))
	for id := range f.pools {
		poolIDs = append(poolIDs, id)
	}
	f.mu.RUnlock()

	for _, id := range poolIDs {
		f.GetForecast(id)
	}
}
