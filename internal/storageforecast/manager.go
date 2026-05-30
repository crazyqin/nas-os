package storageforecast

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Manager 存储预测管理器
type Manager struct {
	mu        sync.RWMutex
	config    ForecastConfig
	pools     map[string]*StoragePool
	snapshots map[string][]UsageSnapshot // poolID -> snapshots
	alerts    []Alert
	stopCh    chan struct{}
}

// NewManager 创建存储预测管理器
func NewManager(config ForecastConfig) *Manager {
	return &Manager{
		config:    config,
		pools:     make(map[string]*StoragePool),
		snapshots: make(map[string][]UsageSnapshot),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动管理器
func (m *Manager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		return nil
	}

	go m.snapshotLoop(ctx)
	go m.forecastLoop(ctx)

	return nil
}

// Stop 停止管理器
func (m *Manager) Stop() {
	select {
	case <-m.stopCh:
		// already closed
	default:
		close(m.stopCh)
	}
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config ForecastConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取当前配置
func (m *Manager) GetConfig() ForecastConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// RegisterPool 注册存储池
func (m *Manager) RegisterPool(pool StoragePool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool.UpdatedAt = time.Now()
	m.pools[pool.ID] = &pool
}

// UnregisterPool 注销存储池
func (m *Manager) UnregisterPool(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pools[poolID]; !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	delete(m.pools, poolID)
	delete(m.snapshots, poolID)
	return nil
}

// UpdatePoolUsage 更新存储池使用量
func (m *Manager) UpdatePoolUsage(poolID string, usedBytes int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("存储池 %s 不存在", poolID)
	}

	pool.UsedBytes = usedBytes
	pool.FreeBytes = pool.TotalBytes - usedBytes
	if pool.TotalBytes > 0 {
		pool.UsedPercent = float64(usedBytes) / float64(pool.TotalBytes) * 100
	}
	pool.UpdatedAt = time.Now()

	// 记录快照
	m.addSnapshot(UsageSnapshot{
		Timestamp:   time.Now(),
		PoolID:      poolID,
		TotalBytes:  pool.TotalBytes,
		UsedBytes:   usedBytes,
		FreeBytes:   pool.FreeBytes,
		UsedPercent: pool.UsedPercent,
	})

	// 检查告警
	m.checkAlerts(pool)

	return nil
}

// GetPool 获取存储池信息
func (m *Manager) GetPool(poolID string) (*StoragePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	return pool, nil
}

// ListPools 列出所有存储池
func (m *Manager) ListPools() []StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pools := make([]StoragePool, 0, len(m.pools))
	for _, p := range m.pools {
		pools = append(pools, *p)
	}
	return pools
}

// GetForecast 获取预测结果
func (m *Manager) GetForecast(poolID string) (*ForecastResult, error) {
	m.mu.RLock()
	pool, exists := m.pools[poolID]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	snapshots := m.snapshots[poolID]
	config := m.config
	m.mu.RUnlock()

	if len(snapshots) < config.MinDataPoints {
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
	linearRate, linearConfidence := m.linearRegression(snapshots)

	// 移动平均分析
	maRate, maConfidence := m.movingAverageForecast(snapshots)

	// 综合两种算法，加权平均
	// 线性回归权重 0.6，移动平均权重 0.4
	confidence := linearConfidence*0.6 + maConfidence*0.4
	growthRate := linearRate*0.6 + maRate*0.4

	if confidence > 1 {
		confidence = 1
	}
	if confidence < 0 {
		confidence = 0
	}

	trend := m.determineTrend(growthRate)

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
	alertLevel := m.determineAlertLevel(pool.UsedPercent, daysUntilFull)

	// 生成建议
	suggestions := m.generateSuggestions(pool, alertLevel, daysUntilFull)

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
func (m *Manager) GetAllForecasts() []ForecastResult {
	m.mu.RLock()
	poolIDs := make([]string, 0, len(m.pools))
	for id := range m.pools {
		poolIDs = append(poolIDs, id)
	}
	m.mu.RUnlock()

	results := make([]ForecastResult, 0, len(poolIDs))
	for _, id := range poolIDs {
		result, err := m.GetForecast(id)
		if err == nil {
			results = append(results, *result)
		}
	}
	return results
}

// GetTrendSeries 获取趋势序列（日/周/月）
func (m *Manager) GetTrendSeries(poolID string, granularity TimeGranularity, duration time.Duration) (*TrendSeries, error) {
	m.mu.RLock()
	pool, exists := m.pools[poolID]
	if !exists {
		m.mu.RUnlock()
		return nil, fmt.Errorf("存储池 %s 不存在", poolID)
	}

	cutoff := time.Now().Add(-duration)
	snapshots := m.snapshots[poolID]
	m.mu.RUnlock()

	// 按粒度聚合
	points := m.aggregateByGranularity(snapshots, granularity, cutoff)

	return &TrendSeries{
		PoolID:      poolID,
		PoolName:    pool.Name,
		Granularity: granularity,
		Points:      points,
	}, nil
}

// GetExpansionRecommendation 获取扩容建议
func (m *Manager) GetExpansionRecommendation(poolID string) (*ExpansionRecommendation, error) {
	forecast, err := m.GetForecast(poolID)
	if err != nil {
		return nil, err
	}

	pool, err := m.GetPool(poolID)
	if err != nil {
		return nil, err
	}

	config := m.GetConfig()

	var recommendedAddBytes int64
	var urgency string

	if forecast.DailyGrowthBytes <= 0 {
		// 无需扩容
		return &ExpansionRecommendation{
			PoolID:              poolID,
			PoolName:            pool.Name,
			CurrentTotalBytes:   pool.TotalBytes,
			CurrentUsedBytes:    pool.UsedBytes,
			DailyGrowthBytes:    forecast.DailyGrowthBytes,
			DaysUntilFull:       forecast.DaysUntilFull,
			RecommendedAddBytes: 0,
			RecommendedAddSize:  "无需扩容",
			TargetDays:          config.ExpansionTargetDays,
			EstimatedCost:       0,
			CostCurrency:        config.CostCurrency,
			Urgency:             "none",
			CreatedAt:           time.Now(),
		}, nil
	}

	// 计算需要的额外空间：目标天数 * 每日增长 + 10% 缓冲
	neededBytes := forecast.DailyGrowthBytes * int64(config.ExpansionTargetDays)
	bufferBytes := neededBytes / 10
	recommendedAddBytes = neededBytes + bufferBytes

	// 向上取整到 TB 或 GB
	if recommendedAddBytes >= 1099511627776 { // 1TB
		tbCount := (recommendedAddBytes + 1099511627775) / 1099511627776
		recommendedAddBytes = tbCount * 1099511627776
	} else {
		gbCount := (recommendedAddBytes + 1073741823) / 1073741824
		recommendedAddBytes = gbCount * 1073741824
	}

	// 成本估算（按 TB 计）
	costPerTBMonth := config.CostPerGBMonth * 1024
	totalTB := float64(recommendedAddBytes) / 1099511627776
	estimatedCost := totalTB * costPerTBMonth

	// 紧急程度
	switch {
	case forecast.DaysUntilFull <= 7:
		urgency = "critical"
	case forecast.DaysUntilFull <= 30:
		urgency = "high"
	case forecast.DaysUntilFull <= 90:
		urgency = "medium"
	default:
		urgency = "low"
	}

	return &ExpansionRecommendation{
		PoolID:              poolID,
		PoolName:            pool.Name,
		CurrentTotalBytes:   pool.TotalBytes,
		CurrentUsedBytes:    pool.UsedBytes,
		DailyGrowthBytes:    forecast.DailyGrowthBytes,
		DaysUntilFull:       forecast.DaysUntilFull,
		RecommendedAddBytes: recommendedAddBytes,
		RecommendedAddSize:  FormatBytes(recommendedAddBytes),
		TargetDays:          config.ExpansionTargetDays,
		EstimatedCost:       math.Round(estimatedCost*100) / 100,
		CostCurrency:        config.CostCurrency,
		Urgency:             urgency,
		CreatedAt:           time.Now(),
	}, nil
}

// GetCostEstimate 获取存储成本估算
func (m *Manager) GetCostEstimate(poolID string) (*StorageCostEstimate, error) {
	pool, err := m.GetPool(poolID)
	if err != nil {
		return nil, err
	}

	config := m.GetConfig()

	totalGB := float64(pool.TotalBytes) / 1073741824
	monthlyCost := totalGB * config.CostPerGBMonth

	// 预测未来成本
	forecast, _ := m.GetForecast(poolID)
	growthGBPerMonth := float64(forecast.DailyGrowthBytes) / 1073741824 * 30

	projected3M := (totalGB + growthGBPerMonth*3) * config.CostPerGBMonth
	projected6M := (totalGB + growthGBPerMonth*6) * config.CostPerGBMonth
	projected12M := (totalGB + growthGBPerMonth*12) * config.CostPerGBMonth

	return &StorageCostEstimate{
		PoolID:           poolID,
		PoolName:         pool.Name,
		TotalBytes:       pool.TotalBytes,
		UsedBytes:        pool.UsedBytes,
		CostPerGBMonth:   config.CostPerGBMonth,
		MonthlyCost:      math.Round(monthlyCost*100) / 100,
		ProjectedCost3M:  math.Round(projected3M*100) / 100,
		ProjectedCost6M:  math.Round(projected6M*100) / 100,
		ProjectedCost12M: math.Round(projected12M*100) / 100,
		Currency:         config.CostCurrency,
	}, nil
}

// GetAlerts 获取告警
func (m *Manager) GetAlerts(dismissed bool) []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []Alert
	for _, a := range m.alerts {
		if a.Dismissed == dismissed {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// DismissAlert 忽略告警
func (m *Manager) DismissAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.alerts {
		if m.alerts[i].ID == alertID {
			m.alerts[i].Dismissed = true
			return nil
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// GetSnapshots 获取快照
func (m *Manager) GetSnapshots(poolID string, duration time.Duration) []UsageSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	snapshots := m.snapshots[poolID]

	var result []UsageSnapshot
	for _, s := range snapshots {
		if s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}
	return result
}

// GetStats 获取统计信息
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalSnapshots := 0
	for _, s := range m.snapshots {
		totalSnapshots += len(s)
	}

	activeAlerts := 0
	for _, a := range m.alerts {
		if !a.Dismissed {
			activeAlerts++
		}
	}

	return map[string]interface{}{
		"total_pools":     len(m.pools),
		"total_snapshots": totalSnapshots,
		"active_alerts":   activeAlerts,
		"total_alerts":    len(m.alerts),
	}
}

// --- 内部方法 ---

// addSnapshot 添加快照
func (m *Manager) addSnapshot(snapshot UsageSnapshot) {
	snapshots := m.snapshots[snapshot.PoolID]
	snapshots = append(snapshots, snapshot)

	// 限制快照数量
	if len(snapshots) > m.config.MaxSnapshots {
		snapshots = snapshots[len(snapshots)-m.config.MaxSnapshots:]
	}

	m.snapshots[snapshot.PoolID] = snapshots
}

// linearRegression 线性回归分析
func (m *Manager) linearRegression(snapshots []UsageSnapshot) (float64, float64) {
	if len(snapshots) < 2 {
		return 0, 0
	}

	// 使用最近的数据点
	limit := 168 // 7 天 * 24 小时
	if len(snapshots) < limit {
		limit = len(snapshots)
	}
	recent := snapshots[len(snapshots)-limit:]

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

// movingAverageForecast 移动平均预测
func (m *Manager) movingAverageForecast(snapshots []UsageSnapshot) (float64, float64) {
	if len(snapshots) < m.config.MovingAverageWindow*2 {
		return 0, 0
	}

	window := m.config.MovingAverageWindow
	if window <= 0 {
		window = 7
	}

	// 计算每小时的移动平均增长
	// 将数据按小时分桶
	hourlyBuckets := m.bucketSnapshotsByHour(snapshots)
	if len(hourlyBuckets) < window*24 {
		return 0, 0
	}

	// 计算每个窗口的平均使用率
	var windowMeans []float64
	for i := window * 24; i <= len(hourlyBuckets); i++ {
		sum := 0.0
		for _, v := range hourlyBuckets[i-window*24 : i] {
			sum += v
		}
		windowMeans = append(windowMeans, sum/float64(window*24))
	}

	if len(windowMeans) < 2 {
		return 0, 0
	}

	// 计算移动平均的增长率
	// 使用最近的移动平均值差分
	n := float64(len(windowMeans))
	var sumX, sumY, sumXY, sumX2 float64

	for i, y := range windowMeans {
		x := float64(i)
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

	// 置信度：基于数据平滑度
	meanY := sumY / n
	var variance float64
	for _, y := range windowMeans {
		variance += (y - meanY) * (y - meanY)
	}
	variance /= n

	// 方差越小，置信度越高
	confidence := 1.0 / (1.0 + variance)
	if confidence < 0.3 {
		confidence = 0.3
	}

	// 转换为每日增长率（百分比/天 -> 每天百分比）
	// slope 是每小时窗口的增长率，需要乘以 24 得到每日
	dailyRate := slope * 24

	return dailyRate, confidence
}

// bucketSnapshotsByHour 按小时分桶
func (m *Manager) bucketSnapshotsByHour(snapshots []UsageSnapshot) []float64 {
	if len(snapshots) == 0 {
		return nil
	}

	// 按时间排序（假设已排序）
	startHour := snapshots[0].Timestamp.Truncate(time.Hour)
	endHour := snapshots[len(snapshots)-1].Timestamp.Truncate(time.Hour)

	totalHours := int(endHour.Sub(startHour).Hours()) + 1
	if totalHours <= 0 {
		return nil
	}

	buckets := make([]float64, totalHours)
	counts := make([]int, totalHours)

	for _, s := range snapshots {
		idx := int(s.Timestamp.Truncate(time.Hour).Sub(startHour).Hours())
		if idx >= 0 && idx < totalHours {
			buckets[idx] += s.UsedPercent
			counts[idx]++
		}
	}

	// 填充空桶（线性插值）
	for i := 0; i < totalHours; i++ {
		if counts[i] > 0 {
			buckets[i] /= float64(counts[i])
		} else {
			// 找前后最近的非空桶进行插值
			prev, prevIdx := -1.0, -1
			next, nextIdx := -1.0, -1
			for j := i - 1; j >= 0; j-- {
				if counts[j] > 0 {
					prev = buckets[j]
					prevIdx = j
					break
				}
			}
			for j := i + 1; j < totalHours; j++ {
				if counts[j] > 0 {
					next = buckets[j]
					nextIdx = j
					break
				}
			}

			switch {
			case prevIdx >= 0 && nextIdx >= 0:
				// 线性插值
				ratio := float64(i-prevIdx) / float64(nextIdx-prevIdx)
				buckets[i] = prev + (next-prev)*ratio
			case prevIdx >= 0:
				buckets[i] = prev
			case nextIdx >= 0:
				buckets[i] = next
			}
		}
	}

	return buckets
}

// aggregateByGranularity 按粒度聚合
func (m *Manager) aggregateByGranularity(snapshots []UsageSnapshot, granularity TimeGranularity, cutoff time.Time) []TrendPoint {
	if len(snapshots) == 0 {
		return nil
	}

	var bucketDuration time.Duration
	switch granularity {
	case GranularityDay:
		bucketDuration = 24 * time.Hour
	case GranularityWeek:
		bucketDuration = 7 * 24 * time.Hour
	case GranularityMonth:
		bucketDuration = 30 * 24 * time.Hour
	default:
		bucketDuration = 24 * time.Hour
	}

	// 按时间分桶
	type bucket struct {
		usedBytesSum int64
		usedPctSum   float64
		count        int
		timestamp    time.Time
	}

	buckets := make(map[int64]*bucket)

	for _, s := range snapshots {
		if s.Timestamp.Before(cutoff) {
			continue
		}

		bucketKey := s.Timestamp.UnixNano() / int64(bucketDuration)
		b, exists := buckets[bucketKey]
		if !exists {
			b = &bucket{
				timestamp: time.Unix(0, bucketKey*int64(bucketDuration)),
			}
			buckets[bucketKey] = b
		}
		b.usedBytesSum += s.UsedBytes
		b.usedPctSum += s.UsedPercent
		b.count++
	}

	// 转换为趋势点
	points := make([]TrendPoint, 0, len(buckets))
	for _, b := range buckets {
		if b.count > 0 {
			points = append(points, TrendPoint{
				Timestamp:   b.timestamp,
				UsedBytes:   b.usedBytesSum / int64(b.count),
				UsedPercent: b.usedPctSum / float64(b.count),
			})
		}
	}

	return points
}

// determineTrend 确定趋势方向
func (m *Manager) determineTrend(growthRate float64) TrendDirection {
	threshold := 0.01 // 0.01% 每天
	if growthRate > threshold {
		return TrendIncreasing
	} else if growthRate < -threshold {
		return TrendDecreasing
	}
	return TrendStable
}

// determineAlertLevel 确定告警级别
func (m *Manager) determineAlertLevel(usage float64, daysUntilFull int) AlertLevel {
	if daysUntilFull >= 0 && daysUntilFull <= 7 {
		return AlertFull
	}
	if usage >= m.config.FullThreshold {
		return AlertFull
	}
	if usage >= m.config.CriticalThreshold || (daysUntilFull >= 0 && daysUntilFull <= 30) {
		return AlertCritical
	}
	if usage >= m.config.WarningThreshold || (daysUntilFull >= 0 && daysUntilFull <= 60) {
		return AlertWarning
	}
	return AlertInfo
}

// generateSuggestions 生成建议
func (m *Manager) generateSuggestions(pool *StoragePool, level AlertLevel, daysUntilFull int) []string {
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
func (m *Manager) checkAlerts(pool *StoragePool) {
	level := m.determineAlertLevel(pool.UsedPercent, -1)

	if level == AlertInfo {
		return
	}

	// 检查是否已有相同告警
	for _, a := range m.alerts {
		if a.PoolID == pool.ID && a.Level == level && !a.Dismissed {
			return
		}
	}

	alert := Alert{
		ID:        fmt.Sprintf("%s-%s-%d", pool.ID, level, time.Now().Unix()),
		PoolID:    pool.ID,
		PoolName:  pool.Name,
		Level:     level,
		Threshold: m.getThreshold(level),
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

	m.alerts = append(m.alerts, alert)
}

// getThreshold 获取阈值
func (m *Manager) getThreshold(level AlertLevel) float64 {
	switch level {
	case AlertFull:
		return m.config.FullThreshold
	case AlertCritical:
		return m.config.CriticalThreshold
	case AlertWarning:
		return m.config.WarningThreshold
	default:
		return 0
	}
}

// snapshotLoop 快照采集循环
func (m *Manager) snapshotLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.collectSnapshots()
		}
	}
}

// forecastLoop 预测循环
func (m *Manager) forecastLoop(ctx context.Context) {
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runForecasts()
		}
	}
}

// collectSnapshots 采集快照
func (m *Manager) collectSnapshots() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, pool := range m.pools {
		m.addSnapshot(UsageSnapshot{
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
func (m *Manager) runForecasts() {
	m.mu.RLock()
	poolIDs := make([]string, 0, len(m.pools))
	for id := range m.pools {
		poolIDs = append(poolIDs, id)
	}
	m.mu.RUnlock()

	for _, id := range poolIDs {
		m.GetForecast(id)
	}
}

// FormatBytes 格式化字节数
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
