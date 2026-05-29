// Package smartcapacityplanner 提供智能容量规划管理核心业务逻辑
package smartcapacityplanner

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// PlannerManager 智能容量规划管理器.
type PlannerManager struct {
	snapshots  map[string]*CapacitySnapshot
	forecasts  map[string]*ForecastResult
	trends     map[string]*GrowthTrend
	plans      map[string]*CapacityPlan
	alerts     map[string]*Alert
	history    []*CapacitySnapshot
	warningThreshold  float64
	criticalThreshold float64
	mu         sync.RWMutex
}

// NewPlannerManager 创建智能容量规划管理器.
func NewPlannerManager() *PlannerManager {
	return &PlannerManager{
		snapshots:  make(map[string]*CapacitySnapshot),
		forecasts:  make(map[string]*ForecastResult),
		trends:     make(map[string]*GrowthTrend),
		plans:      make(map[string]*CapacityPlan),
		alerts:     make(map[string]*Alert),
		history:    make([]*CapacitySnapshot, 0),
		warningThreshold:  0.80, // 80%
		criticalThreshold: 0.95, // 95%
	}
}

// ========== 快照管理 ==========

// RecordUsage 记录存储使用量快照.
func (m *PlannerManager) RecordUsage(req RecordUsageRequest) (*CapacitySnapshot, error) {
	if req.TotalBytes <= 0 {
		return nil, fmt.Errorf("total_bytes must be positive")
	}
	if req.UsedBytes < 0 {
		return nil, fmt.Errorf("used_bytes cannot be negative")
	}
	if req.UsedBytes > req.TotalBytes {
		return nil, fmt.Errorf("used_bytes cannot exceed total_bytes")
	}

	snapshot := &CapacitySnapshot{
		ID:         uuid.New().String(),
		TotalBytes: req.TotalBytes,
		UsedBytes:  req.UsedBytes,
		FreeBytes:  req.TotalBytes - req.UsedBytes,
		UsageRate:  float64(req.UsedBytes) / float64(req.TotalBytes),
		MountPoint: req.MountPoint,
		FileSystem: req.FileSystem,
		Timestamp:  time.Now(),
	}

	m.mu.Lock()
	m.snapshots[snapshot.ID] = snapshot
	m.history = append(m.history, snapshot)
	m.mu.Unlock()

	// 检查是否触发告警
	m.checkAndTriggerAlerts(snapshot)

	return snapshot, nil
}

// GetLatestSnapshot 获取最新快照.
func (m *PlannerManager) GetLatestSnapshot(mountPoint string) (*CapacitySnapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.history) == 0 {
		return nil, fmt.Errorf("no snapshots available")
	}

	// 如果指定了挂载点，查找该挂载点的最新快照
	if mountPoint != "" {
		for i := len(m.history) - 1; i >= 0; i-- {
			if m.history[i].MountPoint == mountPoint {
				cp := *m.history[i]
				return &cp, nil
			}
		}
		return nil, fmt.Errorf("no snapshots for mount point %q", mountPoint)
	}

	// 返回最新的快照
	cp := *m.history[len(m.history)-1]
	return &cp, nil
}

// ListSnapshots 列出所有快照.
func (m *PlannerManager) ListSnapshots(limit int) []*CapacitySnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*CapacitySnapshot, limit)
	for i, s := range m.history[start:] {
		cp := *s
		result[i] = &cp
	}
	return result
}

// ========== 预测功能 ==========

// ForecastCapacity 预测容量.
func (m *PlannerManager) ForecastCapacity(req ForecastRequest) (*ForecastResult, error) {
	m.mu.RLock()
	history := make([]*CapacitySnapshot, len(m.history))
	copy(history, m.history)
	m.mu.RUnlock()

	if len(history) < 2 {
		return nil, fmt.Errorf("need at least 2 snapshots for forecasting")
	}

	// 过滤挂载点
	if req.MountPoint != "" {
		filtered := make([]*CapacitySnapshot, 0)
		for _, s := range history {
			if s.MountPoint == req.MountPoint {
				filtered = append(filtered, s)
			}
		}
		history = filtered
		if len(history) < 2 {
			return nil, fmt.Errorf("need at least 2 snapshots for mount point %q", req.MountPoint)
		}
	}

	// 默认预测30天
	daysAhead := req.DaysAhead
	if daysAhead <= 0 {
		daysAhead = 30
	}

	// 排序历史记录
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.Before(history[j].Timestamp)
	})

	var predictedUsage float64
	var growthRate float64
	var confidence float64

	switch ForecastModel(req.ModelType) {
	case ModelLinear:
		predictedUsage, growthRate, confidence = m.forecastLinear(history, daysAhead)
	case ModelExponential:
		predictedUsage, growthRate, confidence = m.forecastExponential(history, daysAhead)
	case ModelSeasonal:
		predictedUsage, growthRate, confidence = m.forecastSeasonal(history, daysAhead)
	default:
		return nil, fmt.Errorf("unsupported model type: %s", req.ModelType)
	}

	// 确保预测值在合理范围内
	if predictedUsage > 1.0 {
		predictedUsage = 1.0
	}
	if predictedUsage < 0 {
		predictedUsage = 0
	}

	result := &ForecastResult{
		ID:             uuid.New().String(),
		ModelType:      req.ModelType,
		PredictedUsage: math.Round(predictedUsage*10000) / 10000,
		PredictedDate:  time.Now().AddDate(0, 0, daysAhead),
		Confidence:     math.Round(confidence*100) / 100,
		GrowthRate:     math.Round(growthRate*10000) / 10000,
		Timestamp:      time.Now(),
	}

	m.mu.Lock()
	m.forecasts[result.ID] = result
	m.mu.Unlock()

	return result, nil
}

// forecastLinear 线性预测.
func (m *PlannerManager) forecastLinear(history []*CapacitySnapshot, daysAhead int) (float64, float64, float64) {
	n := len(history)

	// 计算线性回归
	var sumX, sumY, sumXY, sumX2 float64
	for i, s := range history {
		x := float64(i)
		y := s.UsageRate
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	nFloat := float64(n)
	slope := (nFloat*sumXY - sumX*sumY) / (nFloat*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / nFloat

	// 计算平均间隔天数
	totalDays := history[n-1].Timestamp.Sub(history[0].Timestamp).Hours() / 24
	avgInterval := totalDays / float64(n-1)
	if avgInterval < 1 {
		avgInterval = 1
	}

	// 预测未来使用率
	futurePoints := float64(daysAhead) / avgInterval
	predictedUsage := intercept + slope*(float64(n-1)+futurePoints)

	// 计算置信度 (基于R²)
	var ssRes, ssTot float64
	meanY := sumY / nFloat
	for i, s := range history {
		predicted := intercept + slope*float64(i)
		ssRes += (s.UsageRate - predicted) * (s.UsageRate - predicted)
		ssTot += (s.UsageRate - meanY) * (s.UsageRate - meanY)
	}
	r2 := 1.0 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0
	}

	return predictedUsage, slope / avgInterval, r2
}

// forecastExponential 指数预测.
func (m *PlannerManager) forecastExponential(history []*CapacitySnapshot, daysAhead int) (float64, float64, float64) {
	n := len(history)

	// 对使用率取对数进行线性回归
	var sumX, sumY, sumXY, sumX2 float64
	validPoints := 0
	for i, s := range history {
		if s.UsageRate <= 0 {
			continue
		}
		x := float64(i)
		y := math.Log(s.UsageRate)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		validPoints++
	}

	if validPoints < 2 {
		// 回退到线性
		return m.forecastLinear(history, daysAhead)
	}

	nFloat := float64(validPoints)
	slope := (nFloat*sumXY - sumX*sumY) / (nFloat*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / nFloat

	// 计算平均间隔天数
	totalDays := history[n-1].Timestamp.Sub(history[0].Timestamp).Hours() / 24
	avgInterval := totalDays / float64(n-1)
	if avgInterval < 1 {
		avgInterval = 1
	}

	// 预测未来使用率
	futurePoints := float64(daysAhead) / avgInterval
	predictedLog := intercept + slope*(float64(n-1)+futurePoints)
	predictedUsage := math.Exp(predictedLog)

	// 计算置信度
	var ssRes, ssTot float64
	meanY := sumY / nFloat
	for i, s := range history {
		if s.UsageRate <= 0 {
			continue
		}
		predicted := intercept + slope*float64(i)
		actual := math.Log(s.UsageRate)
		ssRes += (actual - predicted) * (actual - predicted)
		ssTot += (actual - meanY) * (actual - meanY)
	}
	r2 := 1.0 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0
	}

	return predictedUsage, slope / avgInterval, r2
}

// forecastSeasonal 季节性预测.
func (m *PlannerManager) forecastSeasonal(history []*CapacitySnapshot, daysAhead int) (float64, float64, float64) {
	n := len(history)

	if n < 7 {
		// 数据不足，回退到线性
		return m.forecastLinear(history, daysAhead)
	}

	// 计算周期性成分 (假设7天周期)
	period := 7
	seasonal := make([]float64, period)
	seasonalCount := make([]int, period)

	for i, s := range history {
		idx := i % period
		seasonal[idx] += s.UsageRate
		seasonalCount[idx]++
	}

	for i := range seasonal {
		if seasonalCount[i] > 0 {
			seasonal[i] /= float64(seasonalCount[i])
		}
	}

	// 计算去季节性后的趋势
	deseasonalized := make([]float64, n)
	for i, s := range history {
		deseasonalized[i] = s.UsageRate - seasonal[i%period]
	}

	// 对去季节性数据进行线性回归
	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range deseasonalized {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	nFloat := float64(n)
	slope := (nFloat*sumXY - sumX*sumY) / (nFloat*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / nFloat

	// 计算平均间隔天数
	totalDays := history[n-1].Timestamp.Sub(history[0].Timestamp).Hours() / 24
	avgInterval := totalDays / float64(n-1)
	if avgInterval < 1 {
		avgInterval = 1
	}

	// 预测未来使用率
	futurePoints := float64(daysAhead) / avgInterval
	futureIndex := float64(n-1) + futurePoints
	futureSeasonalIdx := int(math.Round(futureIndex)) % period
	trendValue := intercept + slope*futureIndex
	predictedUsage := trendValue + seasonal[futureSeasonalIdx]

	// 计算置信度
	var ssRes, ssTot float64
	meanY := sumY / nFloat
	for i, y := range deseasonalized {
		predicted := intercept + slope*float64(i)
		ssRes += (y - predicted) * (y - predicted)
		ssTot += (y - meanY) * (y - meanY)
	}
	r2 := 1.0 - ssRes/ssTot
	if r2 < 0 {
		r2 = 0
	}

	return predictedUsage, slope / avgInterval, r2
}

// ========== 趋势分析 ==========

// GetGrowthTrend 获取增长趋势.
func (m *PlannerManager) GetGrowthTrend(mountPoint, period string) (*GrowthTrend, error) {
	m.mu.RLock()
	history := make([]*CapacitySnapshot, len(m.history))
	copy(history, m.history)
	m.mu.RUnlock()

	if len(history) < 2 {
		return nil, fmt.Errorf("need at least 2 snapshots for trend analysis")
	}

	// 过滤挂载点
	if mountPoint != "" {
		filtered := make([]*CapacitySnapshot, 0)
		for _, s := range history {
			if s.MountPoint == mountPoint {
				filtered = append(filtered, s)
			}
		}
		history = filtered
		if len(history) < 2 {
			return nil, fmt.Errorf("need at least 2 snapshots for mount point %q", mountPoint)
		}
	}

	// 排序
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.Before(history[j].Timestamp)
	})

	// 根据周期筛选数据
	var duration time.Duration
	switch period {
	case "daily":
		duration = 24 * time.Hour
	case "weekly":
		duration = 7 * 24 * time.Hour
	case "monthly":
		duration = 30 * 24 * time.Hour
	default:
		period = "daily"
		duration = 24 * time.Hour
	}

	// 获取最新周期的数据
	endTime := history[len(history)-1].Timestamp
	startTime := endTime.Add(-duration)

	var startSnapshot, endSnapshot *CapacitySnapshot
	for _, s := range history {
		if s.Timestamp.After(startTime) || s.Timestamp.Equal(startTime) {
			if startSnapshot == nil {
				startSnapshot = s
			}
			endSnapshot = s
		}
	}

	if startSnapshot == nil || endSnapshot == nil {
		return nil, fmt.Errorf("insufficient data for period %q", period)
	}

	growthBytes := endSnapshot.UsedBytes - startSnapshot.UsedBytes
	var growthRate float64
	if startSnapshot.UsedBytes > 0 {
		growthRate = float64(growthBytes) / float64(startSnapshot.UsedBytes)
	}

	trend := &GrowthTrend{
		ID:          uuid.New().String(),
		Period:      period,
		GrowthBytes: growthBytes,
		GrowthRate:  math.Round(growthRate*10000) / 10000,
		StartDate:   startSnapshot.Timestamp,
		EndDate:     endSnapshot.Timestamp,
		Timestamp:   time.Now(),
	}

	m.mu.Lock()
	m.trends[trend.ID] = trend
	m.mu.Unlock()

	return trend, nil
}

// ========== 规划建议 ==========

// GeneratePlan 生成容量规划建议.
func (m *PlannerManager) GeneratePlan(mountPoint string) (*CapacityPlan, error) {
	m.mu.RLock()
	history := make([]*CapacitySnapshot, len(m.history))
	copy(history, m.history)
	m.mu.RUnlock()

	if len(history) < 2 {
		return nil, fmt.Errorf("need at least 2 snapshots for planning")
	}

	// 过滤挂载点
	if mountPoint != "" {
		filtered := make([]*CapacitySnapshot, 0)
		for _, s := range history {
			if s.MountPoint == mountPoint {
				filtered = append(filtered, s)
			}
		}
		history = filtered
		if len(history) < 2 {
			return nil, fmt.Errorf("need at least 2 snapshots for mount point %q", mountPoint)
		}
	}

	// 排序
	sort.Slice(history, func(i, j int) bool {
		return history[i].Timestamp.Before(history[j].Timestamp)
	})

	latest := history[len(history)-1]

	// 计算增长率
	n := len(history)
	totalDays := history[n-1].Timestamp.Sub(history[0].Timestamp).Hours() / 24
	if totalDays < 1 {
		totalDays = 1
	}

	usageGrowth := latest.UsageRate - history[0].UsageRate
	dailyGrowthRate := usageGrowth / totalDays

	// 预计多少天后满
	var daysUntilFull int
	if dailyGrowthRate > 0 {
		remainingRate := 1.0 - latest.UsageRate
		daysUntilFull = int(remainingRate / dailyGrowthRate)
		if daysUntilFull <= 0 {
			daysUntilFull = 1
		}
	} else {
		daysUntilFull = 999999 // 不会满
	}

	// 确定优先级
	var priority Priority
	if daysUntilFull <= 30 {
		priority = PriorityHigh
	} else if daysUntilFull <= 90 {
		priority = PriorityMedium
	} else {
		priority = PriorityLow
	}

	// 生成建议
	recommendedAction := m.generateRecommendation(latest.UsageRate, daysUntilFull, priority)

	// 计算建议扩容大小 (建议扩容到使用率70%)
	var recommendedSize int64
	if latest.UsageRate > 0.7 {
		targetUsage := 0.7
		newTotal := int64(float64(latest.UsedBytes) / targetUsage)
		recommendedSize = newTotal - latest.TotalBytes
		if recommendedSize < 0 {
			recommendedSize = 0
		}
	}

	// 使用线性预测未来30天使用率
	predictedUsage := latest.UsageRate + dailyGrowthRate*30
	if predictedUsage > 1.0 {
		predictedUsage = 1.0
	}
	if predictedUsage < 0 {
		predictedUsage = 0
	}

	plan := &CapacityPlan{
		ID:                uuid.New().String(),
		CurrentUsage:      math.Round(latest.UsageRate*10000) / 10000,
		PredictedUsage:    math.Round(predictedUsage*10000) / 10000,
		DaysUntilFull:     daysUntilFull,
		RecommendedAction: recommendedAction,
		RecommendedSize:   recommendedSize,
		Priority:          string(priority),
		Timestamp:         time.Now(),
	}

	m.mu.Lock()
	m.plans[plan.ID] = plan
	m.mu.Unlock()

	return plan, nil
}

// generateRecommendation 生成建议.
func (m *PlannerManager) generateRecommendation(usageRate float64, daysUntilFull int, priority Priority) string {
	switch priority {
	case PriorityHigh:
		if daysUntilFull <= 7 {
			return "紧急扩容：存储空间将在一周内耗尽，建议立即扩容或清理数据"
		}
		return "尽快扩容：存储空间将在一个月内耗尽，建议规划扩容方案"
	case PriorityMedium:
		return "建议扩容：存储空间将在三个月内耗尽，建议开始规划扩容"
	default:
		if usageRate < 0.5 {
			return "存储空间充足，无需操作"
		}
		return "存储空间健康，建议持续监控"
	}
}

// ========== 告警管理 ==========

// checkAndTriggerAlerts 检查并触发告警.
func (m *PlannerManager) checkAndTriggerAlerts(snapshot *CapacitySnapshot) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if snapshot.UsageRate >= m.criticalThreshold {
		alert := &Alert{
			ID:        uuid.New().String(),
			Level:     string(AlertCritical),
			Message:   fmt.Sprintf("存储空间严重不足！使用率 %.1f%% 超过阈值 %.1f%%", snapshot.UsageRate*100, m.criticalThreshold*100),
			Threshold: m.criticalThreshold,
			Current:   snapshot.UsageRate,
			IsRead:    false,
			Timestamp: time.Now(),
		}
		m.alerts[alert.ID] = alert
	} else if snapshot.UsageRate >= m.warningThreshold {
		alert := &Alert{
			ID:        uuid.New().String(),
			Level:     string(AlertWarning),
			Message:   fmt.Sprintf("存储空间告警：使用率 %.1f%% 超过阈值 %.1f%%", snapshot.UsageRate*100, m.warningThreshold*100),
			Threshold: m.warningThreshold,
			Current:   snapshot.UsageRate,
			IsRead:    false,
			Timestamp: time.Now(),
		}
		m.alerts[alert.ID] = alert
	}
}

// TriggerAlert 手动触发告警检查.
func (m *PlannerManager) TriggerAlert(mountPoint string) ([]*Alert, error) {
	snapshot, err := m.GetLatestSnapshot(mountPoint)
	if err != nil {
		return nil, err
	}

	// 触发检查
	m.checkAndTriggerAlerts(snapshot)

	// 返回新生成的告警
	m.mu.RLock()
	defer m.mu.RUnlock()

	newAlerts := make([]*Alert, 0)
	for _, alert := range m.alerts {
		if !alert.IsRead {
			newAlerts = append(newAlerts, alert)
		}
	}

	return newAlerts, nil
}

// SetAlertThresholds 设置告警阈值.
func (m *PlannerManager) SetAlertThresholds(warning, critical float64) error {
	if warning <= 0 || warning >= 1 {
		return fmt.Errorf("warning threshold must be between 0 and 1")
	}
	if critical <= 0 || critical >= 1 {
		return fmt.Errorf("critical threshold must be between 0 and 1")
	}
	if warning >= critical {
		return fmt.Errorf("warning threshold must be less than critical threshold")
	}

	m.mu.Lock()
	m.warningThreshold = warning
	m.criticalThreshold = critical
	m.mu.Unlock()

	return nil
}

// GetAlertThresholds 获取告警阈值.
func (m *PlannerManager) GetAlertThresholds() (warning, critical float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.warningThreshold, m.criticalThreshold
}

// ListAlerts 列出告警.
func (m *PlannerManager) ListAlerts(unreadOnly bool) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*Alert, 0)
	for _, alert := range m.alerts {
		if unreadOnly && alert.IsRead {
			continue
		}
		cp := *alert
		alerts = append(alerts, &cp)
	}

	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Timestamp.After(alerts[j].Timestamp)
	})

	return alerts
}

// MarkAlertRead 标记告警为已读.
func (m *PlannerManager) MarkAlertRead(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return fmt.Errorf("alert %q not found", alertID)
	}

	alert.IsRead = true
	return nil
}

// ClearAlerts 清除所有告警.
func (m *PlannerManager) ClearAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alerts = make(map[string]*Alert)
}

// GetForecasts 获取所有预测结果.
func (m *PlannerManager) GetForecasts() []*ForecastResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	forecasts := make([]*ForecastResult, 0, len(m.forecasts))
	for _, f := range m.forecasts {
		cp := *f
		forecasts = append(forecasts, &cp)
	}

	sort.Slice(forecasts, func(i, j int) bool {
		return forecasts[i].Timestamp.After(forecasts[j].Timestamp)
	})

	return forecasts
}

// GetPlans 获取所有规划建议.
func (m *PlannerManager) GetPlans() []*CapacityPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*CapacityPlan, 0, len(m.plans))
	for _, p := range m.plans {
		cp := *p
		plans = append(plans, &cp)
	}

	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Timestamp.After(plans[j].Timestamp)
	})

	return plans
}

// ClearHistory 清除所有历史数据.
func (m *PlannerManager) ClearHistory() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.snapshots = make(map[string]*CapacitySnapshot)
	m.history = make([]*CapacitySnapshot, 0)
	m.forecasts = make(map[string]*ForecastResult)
	m.trends = make(map[string]*GrowthTrend)
	m.plans = make(map[string]*CapacityPlan)
}
