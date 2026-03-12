// Package quota 提供存储配额管理功能
package quota

import (
	"sync"
	"time"
)

// Monitor 配额监控器
type Monitor struct {
	manager   *Manager
	config    AlertConfig
	stopChan  chan struct{}
	running   bool
	mu        sync.Mutex
	trendData map[string][]TrendDataPoint // quotaID -> 历史数据点
	trendMu   sync.RWMutex
}

// NewMonitor 创建监控器
func NewMonitor(manager *Manager, config AlertConfig) *Monitor {
	return &Monitor{
		manager:   manager,
		config:    config,
		stopChan:  make(chan struct{}),
		trendData: make(map[string][]TrendDataPoint),
	}
}

// Start 启动监控
func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	m.running = true
	m.stopChan = make(chan struct{})

	go m.run()
}

// Stop 停止监控
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopChan)
	m.running = false
}

// UpdateConfig 更新配置
func (m *Monitor) UpdateConfig(config AlertConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// run 监控循环
func (m *Monitor) run() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	// 首次立即检查
	m.checkAll()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.checkAll()
		}
	}
}

// checkAll 检查所有配额
func (m *Monitor) checkAll() {
	if !m.config.Enabled {
		return
	}

	usages, err := m.manager.GetAllUsage()
	if err != nil {
		return
	}

	for _, usage := range usages {
		m.checkUsage(usage)
	}
}

// checkUsage 检查单个配额使用情况
func (m *Monitor) checkUsage(usage *QuotaUsage) {
	// 记录趋势数据
	m.recordTrend(usage)

	// 检查是否超过硬限制
	if usage.IsOverHard {
		m.triggerAlert(usage, AlertTypeHardLimit)
		return
	}

	// 检查是否超过软限制
	if usage.IsOverSoft {
		m.triggerAlert(usage, AlertTypeSoftLimit)
		return
	}

	// 检查是否接近软限制阈值
	if m.config.SoftLimitThreshold > 0 {
		thresholdPercent := m.config.SoftLimitThreshold
		if usage.UsagePercent >= thresholdPercent {
			m.triggerAlert(usage, AlertTypeSoftLimit)
		}
	}
}

// triggerAlert 触发告警
func (m *Monitor) triggerAlert(usage *QuotaUsage, alertType AlertType) {
	m.manager.mu.Lock()
	defer m.manager.mu.Unlock()

	quota, exists := m.manager.quotas[usage.QuotaID]
	if !exists {
		return
	}

	// 检查是否已有相同类型的活跃告警
	for _, existingAlert := range m.manager.alerts {
		if existingAlert.QuotaID == usage.QuotaID &&
			existingAlert.Type == alertType &&
			existingAlert.Status == AlertStatusActive {
			// 更新现有告警
			existingAlert.UsedBytes = usage.UsedBytes
			existingAlert.UsagePercent = usage.UsagePercent
			return
		}
	}

	// 创建新告警
	alert := m.manager.createAlert(quota, usage, alertType)

	// 发送通知
	m.sendNotification(alert)
}

// sendNotification 发送告警通知
func (m *Monitor) sendNotification(alert *Alert) {
	// TODO: 实现邮件和 Webhook 通知
	// 这里可以扩展支持多种通知渠道
	if m.config.NotifyWebhook && m.config.WebhookURL != "" {
		go m.sendWebhook(alert)
	}
}

// sendWebhook 发送 Webhook 通知
func (m *Monitor) sendWebhook(alert *Alert) {
	// TODO: 实现 HTTP webhook 调用
	// 可以使用 http.Post 发送 JSON 格式的告警信息
}

// recordTrend 记录趋势数据
func (m *Monitor) recordTrend(usage *QuotaUsage) {
	m.trendMu.Lock()
	defer m.trendMu.Unlock()

	point := TrendDataPoint{
		Timestamp:    time.Now(),
		UsedBytes:    usage.UsedBytes,
		UsagePercent: usage.UsagePercent,
	}

	history := m.trendData[usage.QuotaID]
	history = append(history, point)

	// 只保留最近 24 小时的数据（假设每 5 分钟一次，最多 288 个点）
	maxPoints := 288
	if len(history) > maxPoints {
		history = history[len(history)-maxPoints:]
	}

	m.trendData[usage.QuotaID] = history
}

// GetTrend 获取配额趋势数据
func (m *Monitor) GetTrend(quotaID string, duration time.Duration) []TrendDataPoint {
	m.trendMu.RLock()
	defer m.trendMu.RUnlock()

	history := m.trendData[quotaID]
	if len(history) == 0 {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	result := make([]TrendDataPoint, 0)
	for _, point := range history {
		if point.Timestamp.After(cutoff) {
			result = append(result, point)
		}
	}

	return result
}

// CalculateGrowthRate 计算增长率
func (m *Monitor) CalculateGrowthRate(quotaID string) float64 {
	m.trendMu.RLock()
	defer m.trendMu.RUnlock()

	history := m.trendData[quotaID]
	if len(history) < 2 {
		return 0
	}

	// 使用最近的数据点计算增长率
	recent := history[len(history)-1]
	oldest := history[0]

	timeDiff := recent.Timestamp.Sub(oldest.Timestamp).Hours() / 24 // 天数
	if timeDiff == 0 {
		return 0
	}

	bytesDiff := float64(recent.UsedBytes) - float64(oldest.UsedBytes)
	return bytesDiff / timeDiff // 字节/天
}

// PredictFullTime 预测填满时间
func (m *Monitor) PredictFullTime(quotaID string, hardLimit uint64) int {
	growthRate := m.CalculateGrowthRate(quotaID)
	if growthRate <= 0 {
		return -1 // 无增长或负增长
	}

	// 获取当前使用量
	m.trendMu.RLock()
	history := m.trendData[quotaID]
	m.trendMu.RUnlock()

	if len(history) == 0 {
		return -1
	}

	currentUsage := history[len(history)-1].UsedBytes
	remaining := float64(hardLimit) - float64(currentUsage)

	if remaining <= 0 {
		return 0 // 已满
	}

	daysToFull := remaining / growthRate
	return int(daysToFull)
}

// GetMonitorStatus 获取监控状态
func (m *Monitor) GetMonitorStatus() map[string]interface{} {
	m.mu.Lock()
	running := m.running
	m.mu.Unlock()

	m.trendMu.RLock()
	trendCount := len(m.trendData)
	m.trendMu.RUnlock()

	return map[string]interface{}{
		"running":        running,
		"check_interval": m.config.CheckInterval.String(),
		"alert_enabled":  m.config.Enabled,
		"tracked_quotas": trendCount,
	}
}

// CleanupOldData 清理过期的趋势数据
func (m *Monitor) CleanupOldData(maxAge time.Duration) {
	m.trendMu.Lock()
	defer m.trendMu.Unlock()

	cutoff := time.Now().Add(-maxAge)

	for quotaID, history := range m.trendData {
		// 找到第一个不超过截止时间的数据点
		startIdx := 0
		for i, point := range history {
			if point.Timestamp.After(cutoff) {
				startIdx = i
				break
			}
		}

		if startIdx > 0 {
			m.trendData[quotaID] = history[startIdx:]
		}
	}
}
