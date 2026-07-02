package capacityforecast

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"
)

// Manager 容量预测管理器.
type Manager struct {
	mu        sync.RWMutex
	config    ForecastConfig
	snapshots []CapacitySnapshot
	alerts    []CapacityAlert
	scenarios map[string]*WhatIfScenario
	stopCh    chan struct{}
}

// NewManager 创建容量预测管理器.
func NewManager(config ForecastConfig) *Manager {
	return &Manager{
		config:    config,
		snapshots: make([]CapacitySnapshot, 0),
		scenarios: make(map[string]*WhatIfScenario),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start(ctx context.Context) error {
	if !m.config.Enabled {
		return nil
	}
	go m.monitorLoop(ctx)
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	select {
	case <-m.stopCh:
	default:
		close(m.stopCh)
	}
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(config ForecastConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取当前配置.
func (m *Manager) GetConfig() ForecastConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// AddSnapshot 添加容量快照.
func (m *Manager) AddSnapshot(snapshot CapacitySnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if snapshot.Timestamp.IsZero() {
		snapshot.Timestamp = time.Now()
	}
	if snapshot.TotalBytes > 0 {
		snapshot.FreeBytes = snapshot.TotalBytes - snapshot.UsedBytes
		snapshot.UsedPercent = float64(snapshot.UsedBytes) / float64(snapshot.TotalBytes) * 100
	}

	m.snapshots = append(m.snapshots, snapshot)
	if len(m.snapshots) > m.config.MaxSnapshots {
		m.snapshots = m.snapshots[len(m.snapshots)-m.config.MaxSnapshots:]
	}

	m.checkAlerts(snapshot)
}

// GetSnapshots 获取快照列表.
func (m *Manager) GetSnapshots(duration time.Duration) []CapacitySnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var result []CapacitySnapshot
	for _, s := range m.snapshots {
		if s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}
	return result
}

// PredictCapacity 预测容量.
func (m *Manager) PredictCapacity(targetDays int) (*Forecast, error) {
	m.mu.RLock()
	snapshots := m.snapshots
	config := m.config
	m.mu.RUnlock()

	if len(snapshots) < config.MinDataPoints {
		return nil, fmt.Errorf("数据点不足，需要至少 %d 个快照", config.MinDataPoints)
	}

	// 线性回归预测
	linearRate, linearConf := m.linearRegression(snapshots)

	// 移动平均预测
	maRate, maConf := m.movingAverageForecast(snapshots)

	// 综合加权
	confidence := linearConf*0.6 + maConf*0.4
	if confidence > 1 {
		confidence = 1
	}
	if confidence < 0 {
		confidence = 0
	}

	growthRate := linearRate*0.6 + maRate*0.4
	latest := snapshots[len(snapshots)-1]

	// 计算预测值
	targetDate := time.Now().AddDate(0, 0, targetDays)
	predictedUsage := latest.UsedPercent + growthRate*float64(targetDays)
	if predictedUsage > 100 {
		predictedUsage = 100
	}
	if predictedUsage < 0 {
		predictedUsage = 0
	}

	predictedBytes := latest.UsedBytes + int64(growthRate*float64(latest.TotalBytes)/100)*int64(targetDays)
	if predictedBytes < 0 {
		predictedBytes = 0
	}

	// 计算满载天数
	var daysUntilFull int
	var estimatedFullDate *time.Time
	if growthRate > 0 && latest.UsedPercent < 100 {
		days := (100 - latest.UsedPercent) / growthRate
		daysUntilFull = int(math.Ceil(days))
		fullDate := time.Now().AddDate(0, 0, daysUntilFull)
		estimatedFullDate = &fullDate
	} else if growthRate <= 0 {
		daysUntilFull = -1
	}

	trend := m.determineTrend(growthRate)

	return &Forecast{
		ID:                fmt.Sprintf("forecast-%d", time.Now().Unix()),
		TargetDate:        targetDate,
		CurrentUsage:      latest.UsedPercent,
		PredictedUsage:    predictedUsage,
		PredictedBytes:    predictedBytes,
		Trend:             trend,
		Method:            MethodLinearRegression,
		Confidence:        confidence,
		DaysUntilFull:     daysUntilFull,
		EstimatedFullDate: estimatedFullDate,
		CreatedAt:         time.Now(),
	}, nil
}

// AnalyzeGrowth 分析增长率.
func (m *Manager) AnalyzeGrowth() (*GrowthAnalysis, error) {
	m.mu.RLock()
	snapshots := m.snapshots
	m.mu.RUnlock()

	if len(snapshots) < 2 {
		return nil, fmt.Errorf("需要至少 2 个快照进行增长分析")
	}

	// 按数据类型聚合
	typeData := make(map[DataType][]int64)
	for _, s := range snapshots {
		for dt, bytes := range s.ByType {
			typeData[dt] = append(typeData[dt], bytes)
		}
	}

	// 计算各类型增长率
	var growthRates []GrowthRate
	var totalDailyGrowth int64

	latest := snapshots[len(snapshots)-1]
	totalUsed := latest.UsedBytes

	for dt, values := range typeData {
		if len(values) < 2 {
			continue
		}

		first := values[0]
		last := values[len(values)-1]
		daysDiff := snapshots[len(snapshots)-1].Timestamp.Sub(snapshots[0].Timestamp).Hours() / 24
		if daysDiff <= 0 {
			daysDiff = 1
		}

		dailyGrowth := int64(float64(last-first) / daysDiff)
		growthPercent := 0.0
		if first > 0 {
			growthPercent = float64(last-first) / float64(first) * 100
		}

		share := 0.0
		if totalUsed > 0 {
			share = float64(last) / float64(totalUsed) * 100
		}

		trend := TrendStable
		if dailyGrowth > 0 {
			trend = TrendIncreasing
		} else if dailyGrowth < 0 {
			trend = TrendDecreasing
		}

		growthRates = append(growthRates, GrowthRate{
			DataType:           dt,
			DailyGrowthBytes:   dailyGrowth,
			WeeklyGrowthBytes:  dailyGrowth * 7,
			MonthlyGrowthBytes: dailyGrowth * 30,
			GrowthPercent:      growthPercent,
			Trend:              trend,
			CurrentSizeBytes:   last,
			TotalShare:         share,
		})

		totalDailyGrowth += dailyGrowth
	}

	overallRate := 0.0
	if totalUsed > 0 {
		overallRate = float64(totalDailyGrowth) / float64(totalUsed) * 100
	}

	return &GrowthAnalysis{
		OverallGrowthRate: overallRate,
		OverallDailyBytes: totalDailyGrowth,
		ByType:            growthRates,
		AnalysisTime:      time.Now(),
	}, nil
}

// CheckAlerts 获取当前告警.
func (m *Manager) CheckAlerts(dismissed bool) []CapacityAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []CapacityAlert
	for _, a := range m.alerts {
		if a.Dismissed == dismissed {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// DismissAlert 忽略告警.
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

// AcknowledgeAlert 确认告警.
func (m *Manager) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.alerts {
		if m.alerts[i].ID == alertID {
			m.alerts[i].Acknowledged = true
			return nil
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// SimulateWhatIf What-If 模拟.
func (m *Manager) SimulateWhatIf(scenario WhatIfScenario) (*WhatIfScenario, error) {
	m.mu.RLock()
	snapshots := m.snapshots
	config := m.config
	m.mu.RUnlock()

	if len(snapshots) < config.MinDataPoints {
		return nil, fmt.Errorf("数据不足，无法进行模拟")
	}

	latest := snapshots[len(snapshots)-1]

	// 应用修改
	simUsed := latest.UsedBytes
	simTotal := latest.TotalBytes

	for _, mod := range scenario.Modifications {
		switch mod.Type {
		case "add_data":
			simUsed += mod.AmountBytes
		case "remove_data":
			simUsed -= mod.AmountBytes
			if simUsed < 0 {
				simUsed = 0
			}
		case "add_capacity":
			simTotal += mod.AmountBytes
		}
	}

	// 计算模拟结果
	simFree := simTotal - simUsed
	if simFree < 0 {
		simFree = 0
	}
	simUsage := 0.0
	if simTotal > 0 {
		simUsage = float64(simUsed) / float64(simTotal) * 100
	}

	// 计算预测满载天数
	linearRate, _ := m.linearRegression(snapshots)
	var simDaysToFull int
	var estimatedFullDate *time.Time

	if linearRate > 0 && simUsage < 100 {
		days := (100 - simUsage) / linearRate
		simDaysToFull = int(math.Ceil(days))
		fullDate := time.Now().AddDate(0, 0, simDaysToFull)
		estimatedFullDate = &fullDate
	} else if linearRate <= 0 {
		simDaysToFull = -1
	}

	// 对比当前状态
	comparison := &Comparison{
		UsageChange:      simUsage - latest.UsedPercent,
		FreeBytesChange:  simFree - latest.FreeBytes,
		DaysToFullChange: 0,
	}

	// 计算当前满载天数用于对比
	if linearRate > 0 && latest.UsedPercent < 100 {
		currentDays := int(math.Ceil((100 - latest.UsedPercent) / linearRate))
		comparison.DaysToFullChange = simDaysToFull - currentDays
	}

	scenario.SimulatedResult = &SimulationResult{
		ProjectedTotalBytes:   simTotal,
		ProjectedUsedBytes:    simUsed,
		ProjectedFreeBytes:    simFree,
		ProjectedUsage:        simUsage,
		ProjectedDaysToFull:   simDaysToFull,
		ComparisonWithCurrent: comparison,
		EstimatedFullDate:     estimatedFullDate,
	}
	scenario.CreatedAt = time.Now()

	// 保存场景
	m.mu.Lock()
	scenario.ID = fmt.Sprintf("scenario-%d", time.Now().Unix())
	m.scenarios[scenario.ID] = &scenario
	m.mu.Unlock()

	return &scenario, nil
}

// GetScenario 获取模拟场景.
func (m *Manager) GetScenario(id string) (*WhatIfScenario, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scenario, exists := m.scenarios[id]
	if !exists {
		return nil, fmt.Errorf("场景 %s 不存在", id)
	}
	return scenario, nil
}

// ListScenarios 列出所有模拟场景.
func (m *Manager) ListScenarios() []*WhatIfScenario {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scenarios := make([]*WhatIfScenario, 0, len(m.scenarios))
	for _, s := range m.scenarios {
		scenarios = append(scenarios, s)
	}
	return scenarios
}

// RecommendExpansion 推荐扩容方案.
func (m *Manager) RecommendExpansion() (*ExpansionRecommendation, error) {
	m.mu.RLock()
	snapshots := m.snapshots
	config := m.config
	m.mu.RUnlock()

	if len(snapshots) < config.MinDataPoints {
		return nil, fmt.Errorf("数据不足，无法生成扩容建议")
	}

	latest := snapshots[len(snapshots)-1]
	linearRate, _ := m.linearRegression(snapshots)
	dailyGrowthBytes := int64(linearRate * float64(latest.TotalBytes) / 100)

	var daysUntilFull int
	if dailyGrowthBytes > 0 && latest.FreeBytes > 0 {
		daysUntilFull = int(math.Ceil(float64(latest.FreeBytes) / float64(dailyGrowthBytes)))
	} else if dailyGrowthBytes <= 0 {
		daysUntilFull = -1
	}

	// 生成扩容方案
	var plans []ExpansionPlan

	// 方案 1: 添加单块大容量硬盘
	plan1 := m.generateAddDiskPlan(latest, dailyGrowthBytes, config)
	plans = append(plans, plan1)

	// 方案 2: 添加多块小容量硬盘
	plan2 := m.generateMultiDiskPlan(latest, dailyGrowthBytes, config)
	plans = append(plans, plan2)

	// 方案 3: 云分层存储
	plan3 := m.generateCloudTierPlan(latest, dailyGrowthBytes, config)
	plans = append(plans, plan3)

	// 方案 4: 替换所有硬盘为更大容量
	plan4 := m.generateReplaceAllPlan(latest, dailyGrowthBytes, config)
	plans = append(plans, plan4)

	// 按成本效益排序
	for i := range plans {
		if plans[i].DaysSupported > 0 {
			plans[i].Rank = i + 1
		}
	}

	// 推荐最优方案（DaysSupported / Cost 比值最高的非云方案）
	var bestPlan *ExpansionPlan
	bestRatio := 0.0
	for i := range plans {
		if plans[i].Type == ExpansionCloudTier {
			continue
		}
		if plans[i].EstimatedCost > 0 {
			ratio := float64(plans[i].DaysSupported) / plans[i].EstimatedCost
			if ratio > bestRatio {
				bestRatio = ratio
				bestPlan = &plans[i]
			}
		}
	}

	return &ExpansionRecommendation{
		CurrentTotalBytes: latest.TotalBytes,
		CurrentUsedBytes:  latest.UsedBytes,
		DailyGrowthBytes:  dailyGrowthBytes,
		DaysUntilFull:     daysUntilFull,
		Plans:             plans,
		RecommendedPlan:   bestPlan,
		CreatedAt:         time.Now(),
	}, nil
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeAlerts := 0
	for _, a := range m.alerts {
		if !a.Dismissed {
			activeAlerts++
		}
	}

	return map[string]interface{}{
		"total_snapshots": len(m.snapshots),
		"total_alerts":    len(m.alerts),
		"active_alerts":   activeAlerts,
		"total_scenarios": len(m.scenarios),
	}
}

// --- 内部方法 ---

// checkAlerts 检查告警.
func (m *Manager) checkAlerts(snapshot CapacitySnapshot) {
	level := m.determineAlertLevel(snapshot.UsedPercent)
	if level == AlertInfo {
		return
	}

	// 检查重复告警
	for _, a := range m.alerts {
		if a.Level == level && !a.Dismissed {
			return
		}
	}

	alert := CapacityAlert{
		ID:        fmt.Sprintf("alert-%s-%d", level, time.Now().Unix()),
		Level:     level,
		Threshold: m.getThreshold(level),
		Current:   snapshot.UsedPercent,
		CreatedAt: time.Now(),
	}

	switch level {
	case AlertEmergency:
		alert.Message = fmt.Sprintf("⚠️ 存储使用率 %.1f%%，紧急！", snapshot.UsedPercent)
	case AlertCritical:
		alert.Message = fmt.Sprintf("存储使用率 %.1f%%，空间紧张", snapshot.UsedPercent)
	case AlertWarning:
		alert.Message = fmt.Sprintf("存储使用率 %.1f%%，请关注", snapshot.UsedPercent)
	}

	m.alerts = append(m.alerts, alert)
}

// linearRegression 线性回归.
func (m *Manager) linearRegression(snapshots []CapacitySnapshot) (float64, float64) {
	if len(snapshots) < 2 {
		return 0, 0
	}

	limit := 168
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

	// R²
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

	return slope * 24, confidence // 转换为每日
}

// movingAverageForecast 移动平均预测.
func (m *Manager) movingAverageForecast(snapshots []CapacitySnapshot) (float64, float64) {
	if len(snapshots) < m.config.MovingAverageWindow*2 {
		return 0, 0
	}

	window := m.config.MovingAverageWindow
	if window <= 0 {
		window = 7
	}

	// 按日聚合
	dailyBuckets := m.bucketByDay(snapshots)
	if len(dailyBuckets) < window*2 {
		return 0, 0
	}

	// 移动平均
	var windowMeans []float64
	for i := window; i <= len(dailyBuckets); i++ {
		sum := 0.0
		for _, v := range dailyBuckets[i-window : i] {
			sum += v
		}
		windowMeans = append(windowMeans, sum/float64(window))
	}

	if len(windowMeans) < 2 {
		return 0, 0
	}

	// 线性回归移动平均值
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

	// 置信度
	meanY := sumY / n
	var variance float64
	for _, y := range windowMeans {
		variance += (y - meanY) * (y - meanY)
	}
	variance /= n
	confidence := 1.0 / (1.0 + variance)
	if confidence < 0.3 {
		confidence = 0.3
	}

	return slope, confidence
}

// bucketByDay 按天分桶.
func (m *Manager) bucketByDay(snapshots []CapacitySnapshot) []float64 {
	if len(snapshots) == 0 {
		return nil
	}

	startDay := snapshots[0].Timestamp.Truncate(24 * time.Hour)
	endDay := snapshots[len(snapshots)-1].Timestamp.Truncate(24 * time.Hour)
	totalDays := int(endDay.Sub(startDay).Hours()/24) + 1

	buckets := make([]float64, totalDays)
	counts := make([]int, totalDays)

	for _, s := range snapshots {
		idx := int(s.Timestamp.Truncate(24*time.Hour).Sub(startDay).Hours() / 24)
		if idx >= 0 && idx < totalDays {
			buckets[idx] += s.UsedPercent
			counts[idx]++
		}
	}

	for i := 0; i < totalDays; i++ {
		if counts[i] > 0 {
			buckets[i] /= float64(counts[i])
		} else {
			// 线性插值
			prev, prevIdx := -1.0, -1
			next, nextIdx := -1.0, -1
			for j := i - 1; j >= 0; j-- {
				if counts[j] > 0 {
					prev = buckets[j]
					prevIdx = j
					break
				}
			}
			for j := i + 1; j < totalDays; j++ {
				if counts[j] > 0 {
					next = buckets[j]
					nextIdx = j
					break
				}
			}
			switch {
			case prevIdx >= 0 && nextIdx >= 0:
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

// determineTrend 确定趋势方向.
func (m *Manager) determineTrend(rate float64) TrendDirection {
	threshold := 0.01
	if rate > threshold {
		return TrendIncreasing
	} else if rate < -threshold {
		return TrendDecreasing
	}
	return TrendStable
}

// determineAlertLevel 确定告警级别.
func (m *Manager) determineAlertLevel(usage float64) AlertLevel {
	if usage >= m.config.EmergencyThreshold {
		return AlertEmergency
	}
	if usage >= m.config.CriticalThreshold {
		return AlertCritical
	}
	if usage >= m.config.WarningThreshold {
		return AlertWarning
	}
	return AlertInfo
}

// getThreshold 获取阈值.
func (m *Manager) getThreshold(level AlertLevel) float64 {
	switch level {
	case AlertEmergency:
		return m.config.EmergencyThreshold
	case AlertCritical:
		return m.config.CriticalThreshold
	case AlertWarning:
		return m.config.WarningThreshold
	default:
		return 0
	}
}

// generateAddDiskPlan 生成添加硬盘方案.
func (m *Manager) generateAddDiskPlan(latest CapacitySnapshot, dailyGrowth int64, config ForecastConfig) ExpansionPlan {
	neededBytes := dailyGrowth * int64(config.ExpansionTargetDays)
	bufferBytes := neededBytes / 10
	totalNeeded := neededBytes + bufferBytes

	// 向上取整到 TB
	tbCount := int64(math.Ceil(float64(totalNeeded) / 1099511627776))
	if tbCount < 1 {
		tbCount = 1
	}
	addBytes := tbCount * 1099511627776
	costPerTB := config.CostPerTBMonth * 12 // 年度成本

	urgency := "low"
	if dailyGrowth > 0 && latest.FreeBytes > 0 {
		days := latest.FreeBytes / dailyGrowth
		switch {
		case days <= 30:
			urgency = "critical"
		case days <= 90:
			urgency = "high"
		case days <= 180:
			urgency = "medium"
		}
	}

	return ExpansionPlan{
		ID:                 "add-single-disk",
		Type:               ExpansionAddDisk,
		Name:               "添加单块大容量硬盘",
		Description:        fmt.Sprintf("添加 %dTB 硬盘扩展存储池", tbCount),
		AddCapacityBytes:   addBytes,
		AddCapacityDisplay: FormatBytes(addBytes),
		EstimatedCost:      float64(tbCount) * costPerTB,
		CostCurrency:       config.CostCurrency,
		CostPerTB:          costPerTB,
		DaysSupported:      config.ExpansionTargetDays,
		Urgency:            urgency,
		Pros:               []string{"实施简单", "无需迁移数据", "即时生效"},
		Cons:               []string{"需要空闲硬盘位", "单点故障风险"},
	}
}

// generateMultiDiskPlan 生成多硬盘方案.
func (m *Manager) generateMultiDiskPlan(latest CapacitySnapshot, dailyGrowth int64, config ForecastConfig) ExpansionPlan {
	neededBytes := dailyGrowth * int64(config.ExpansionTargetDays)
	bufferBytes := neededBytes / 10
	totalNeeded := neededBytes + bufferBytes

	// 使用多块 4TB 硬盘
	diskSize := int64(4 * 1099511627776) // 4TB
	diskCount := int(math.Ceil(float64(totalNeeded) / float64(diskSize)))
	if diskCount < 2 {
		diskCount = 2
	}
	addBytes := int64(diskCount) * diskSize
	costPerTB := config.CostPerTBMonth * 12 * 0.9 // 批量折扣

	urgency := "low"
	if dailyGrowth > 0 && latest.FreeBytes > 0 {
		days := latest.FreeBytes / dailyGrowth
		switch {
		case days <= 30:
			urgency = "critical"
		case days <= 90:
			urgency = "high"
		case days <= 180:
			urgency = "medium"
		}
	}

	return ExpansionPlan{
		ID:                 "add-multi-disk",
		Type:               ExpansionAddDisk,
		Name:               "添加多块 4TB 硬盘",
		Description:        fmt.Sprintf("添加 %d 块 4TB 硬盘，提供冗余", diskCount),
		AddCapacityBytes:   addBytes,
		AddCapacityDisplay: FormatBytes(addBytes),
		EstimatedCost:      float64(diskCount) * 4 * costPerTB,
		CostCurrency:       config.CostCurrency,
		CostPerTB:          costPerTB,
		DaysSupported:      config.ExpansionTargetDays,
		Urgency:            urgency,
		Pros:               []string{"数据冗余", "灵活扩展", "成本较低"},
		Cons:               []string{"需要多个硬盘位", "RAID 重建时间"},
	}
}

// generateCloudTierPlan 生成云分层方案.
func (m *Manager) generateCloudTierPlan(latest CapacitySnapshot, dailyGrowth int64, config ForecastConfig) ExpansionPlan {
	neededBytes := dailyGrowth * int64(config.ExpansionTargetDays)
	tbNeeded := float64(neededBytes) / 1099511627776

	// 云存储成本更低
	cloudCostPerTBMonth := config.CostPerTBMonth * 0.3
	monthlyCost := tbNeeded * cloudCostPerTBMonth

	return ExpansionPlan{
		ID:                 "cloud-tier",
		Type:               ExpansionCloudTier,
		Name:               "云分层存储",
		Description:        "将冷数据迁移至云存储，释放本地空间",
		AddCapacityBytes:   neededBytes,
		AddCapacityDisplay: FormatBytes(neededBytes),
		EstimatedCost:      monthlyCost * 12,
		CostCurrency:       config.CostCurrency,
		CostPerTB:          cloudCostPerTBMonth * 12,
		DaysSupported:      config.ExpansionTargetDays,
		Urgency:            "low",
		Pros:               []string{"无需硬件投入", "弹性扩展", "按需付费"},
		Cons:               []string{"依赖网络带宽", "长期成本较高", "数据主权考量"},
	}
}

// generateReplaceAllPlan 生成全部替换方案.
func (m *Manager) generateReplaceAllPlan(latest CapacitySnapshot, dailyGrowth int64, config ForecastConfig) ExpansionPlan {
	// 假设当前是 4TB x 4 的配置
	currentDiskCount := 4
	newDiskSizeTB := 8 // 升级到 8TB
	oldDiskSizeTB := 4

	newTotal := int64(currentDiskCount) * int64(newDiskSizeTB) * 1099511627776
	addBytes := newTotal - latest.TotalBytes
	if addBytes < 0 {
		addBytes = int64(newDiskSizeTB-oldDiskSizeTB) * 1099511627776 * int64(currentDiskCount)
	}

	costPerTB := config.CostPerTBMonth * 12
	totalCost := float64(currentDiskCount*newDiskSizeTB) * costPerTB

	daysSupported := 0
	if dailyGrowth > 0 {
		additionalFree := addBytes + latest.FreeBytes
		daysSupported = int(float64(additionalFree) / float64(dailyGrowth))
	}

	return ExpansionPlan{
		ID:                 "replace-all",
		Type:               ExpansionReplaceAll,
		Name:               "替换所有硬盘",
		Description:        fmt.Sprintf("将所有硬盘升级为 %dTB", newDiskSizeTB),
		AddCapacityBytes:   addBytes,
		AddCapacityDisplay: FormatBytes(addBytes),
		EstimatedCost:      totalCost,
		CostCurrency:       config.CostCurrency,
		CostPerTB:          costPerTB,
		DaysSupported:      daysSupported,
		Urgency:            "low",
		Pros:               []string{"统一硬件规格", "最大化容量提升", "延长使用寿命"},
		Cons:               []string{"成本最高", "需要数据迁移", "停机时间较长"},
	}
}

// monitorLoop 监控循环.
func (m *Manager) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.SnapshotInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			// 周期性检查（快照由外部添加）
		}
	}
}

// FormatBytes 格式化字节数.
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
