// Package quota 提供存储配额管理功能
package quota

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Monitor 配额监控器
type Monitor struct {
	manager     *Manager
	config      AlertConfig
	stopChan    chan struct{}
	running     bool
	mu          sync.Mutex
	trendData   map[string][]TrendDataPoint // quotaID -> 历史数据点
	trendMu     sync.RWMutex
	alertLevels []AlertLevelConfig // 预警级别配置
}

// NewMonitor 创建监控器
func NewMonitor(manager *Manager, config AlertConfig) *Monitor {
	// 设置默认预警级别
	alertLevels := DefaultAlertLevels

	// 如果配置中指定了阈值，更新默认级别
	if config.WarningThreshold > 0 {
		alertLevels[1].Threshold = config.WarningThreshold
	}
	if config.CriticalThreshold > 0 {
		alertLevels[2].Threshold = config.CriticalThreshold
	}
	if config.EmergencyThreshold > 0 {
		alertLevels[3].Threshold = config.EmergencyThreshold
	}

	return &Monitor{
		manager:     manager,
		config:      config,
		stopChan:    make(chan struct{}),
		trendData:   make(map[string][]TrendDataPoint),
		alertLevels: alertLevels,
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
			// 检查告警升级
			if m.config.EscalationEnabled {
				m.checkEscalation()
			}
		}
	}
}

// checkEscalation 检查告警升级
func (m *Monitor) checkEscalation() {
	m.manager.mu.Lock()
	defer m.manager.mu.Unlock()

	now := time.Now()
	for _, alert := range m.manager.alerts {
		if alert.Status != AlertStatusActive {
			continue
		}

		// 检查是否需要升级
		timeSinceCreated := now.Sub(alert.CreatedAt)
		expectedLevel := int(timeSinceCreated / m.config.EscalationInterval)

		if expectedLevel > alert.EscalationLevel && expectedLevel <= m.config.MaxEscalationLevel {
			// 执行升级
			alert.EscalationLevel = expectedLevel
			now := time.Now()
			alert.EscalatedAt = &now
			alert.Status = AlertStatusEscalated

			// 发送升级通知
			m.sendEscalationNotification(alert)

			// 恢复活跃状态
			alert.Status = AlertStatusActive
		}
	}
}

// sendEscalationNotification 发送升级通知
func (m *Monitor) sendEscalationNotification(alert *Alert) {
	fmt.Printf("[quota] 告警升级：%s -> 级别 %d (已持续 %v)\n",
		alert.ID, alert.EscalationLevel, time.Since(alert.CreatedAt).Round(time.Minute))

	// 升级 Webhook 通知
	if m.config.EscalationWebhookURL != "" {
		go m.sendWebhookWithSeverity(alert, m.config.EscalationWebhookURL)
	}

	// 升级邮件通知
	if m.config.EscalationNotifyEmail {
		go m.sendEmail(alert)
	}
}

// sendWebhookWithSeverity 发送带严重级别的 Webhook
func (m *Monitor) sendWebhookWithSeverity(alert *Alert, webhookURL string) {
	if webhookURL == "" {
		return
	}

	// 获取配额信息
	m.manager.mu.RLock()
	quota, exists := m.manager.quotas[alert.QuotaID]
	m.manager.mu.RUnlock()

	quotaName := alert.QuotaID
	if exists {
		quotaName = quota.TargetName
	}

	payload := map[string]interface{}{
		"alert_id":         alert.ID,
		"type":             "quota_alert_escalation",
		"quota_id":         alert.QuotaID,
		"quota_name":       quotaName,
		"volume":           alert.VolumeName,
		"target_name":      alert.TargetName,
		"severity":         alert.Severity,
		"escalation_level": alert.EscalationLevel,
		"used_bytes":       alert.UsedBytes,
		"usage_percent":    alert.UsagePercent,
		"threshold":        alert.Threshold,
		"status":           "escalated",
		"created_at":       alert.CreatedAt,
		"escalated_at":     alert.EscalatedAt,
		"duration":         time.Since(alert.CreatedAt).String(),
		"message":          alert.Message,
	}

	jsonData, _ := json.Marshal(payload)
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("POST", webhookURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Alert-Type", "quota-escalation")
	req.Header.Set("X-Alert-Severity", string(alert.Severity))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[quota] 升级 webhook 发送失败：%v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("[quota] 升级 webhook 发送成功，状态：%d\n", resp.StatusCode)
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

	// 检查是否超过硬限制（最高优先级）
	if usage.IsOverHard {
		m.triggerMultiLevelAlert(usage, AlertSeverityEmergency, 100)
		return
	}

	// 按严重级别从高到低检查
	for i := len(m.alertLevels) - 1; i >= 0; i-- {
		level := m.alertLevels[i]
		if usage.UsagePercent >= level.Threshold {
			m.triggerMultiLevelAlert(usage, level.Severity, level.Threshold)
			return
		}
	}
}

// triggerMultiLevelAlert 触发多级预警
func (m *Monitor) triggerMultiLevelAlert(usage *QuotaUsage, severity AlertSeverity, threshold float64) {
	m.manager.mu.Lock()
	defer m.manager.mu.Unlock()

	quota, exists := m.manager.quotas[usage.QuotaID]
	if !exists {
		return
	}

	// 检查是否已有相同配额的活跃告警
	existingAlert, hasActive := m.findActiveAlert(usage.QuotaID)

	if hasActive {
		// 如果严重级别提升，更新告警
		if getSeverityLevel(existingAlert.Severity) < getSeverityLevel(severity) {
			existingAlert.Severity = severity
			existingAlert.UsagePercent = usage.UsagePercent
			existingAlert.UsedBytes = usage.UsedBytes
			existingAlert.Threshold = threshold
			existingAlert.Message = m.buildAlertMessage(usage, severity)

			// 重新发送通知
			m.sendNotification(existingAlert)
		} else {
			// 只更新使用量
			existingAlert.UsagePercent = usage.UsagePercent
			existingAlert.UsedBytes = usage.UsedBytes
		}
		return
	}

	// 创建新告警
	alert := &Alert{
		ID:           generateID(),
		QuotaID:      quota.ID,
		Type:         AlertTypeSoftLimit,
		Severity:     severity,
		Status:       AlertStatusActive,
		TargetID:     quota.TargetID,
		TargetName:   quota.TargetName,
		Path:         quota.Path,
		UsedBytes:    usage.UsedBytes,
		LimitBytes:   quota.HardLimit,
		UsagePercent: usage.UsagePercent,
		Threshold:    threshold,
		Message:      m.buildAlertMessage(usage, severity),
		CreatedAt:    time.Now(),
	}

	// 添加到管理器
	m.manager.mu.Lock()
	m.manager.alerts[alert.ID] = alert
	m.manager.mu.Unlock()

	// 发送通知
	m.sendNotification(alert)
}

// findActiveAlert 查找活跃告警
func (m *Monitor) findActiveAlert(quotaID string) (*Alert, bool) {
	for _, alert := range m.manager.alerts {
		if alert.QuotaID == quotaID && alert.Status == AlertStatusActive {
			return alert, true
		}
	}
	return nil, false
}

// buildAlertMessage 构建告警消息
func (m *Monitor) buildAlertMessage(usage *QuotaUsage, severity AlertSeverity) string {
	severityText := map[AlertSeverity]string{
		AlertSeverityInfo:      "提示",
		AlertSeverityWarning:   "警告",
		AlertSeverityCritical:  "严重",
		AlertSeverityEmergency: "紧急",
	}

	return fmt.Sprintf("[%s] %s 存储使用已达 %.1f%%",
		severityText[severity], usage.TargetName, usage.UsagePercent)
}

// getSeverityLevel 获取严重级别数值（用于比较）
func getSeverityLevel(severity AlertSeverity) int {
	switch severity {
	case AlertSeverityInfo:
		return 1
	case AlertSeverityWarning:
		return 2
	case AlertSeverityCritical:
		return 3
	case AlertSeverityEmergency:
		return 4
	default:
		return 0
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
	// 支持 Webhook 通知
	if m.config.NotifyWebhook && m.config.WebhookURL != "" {
		go m.sendWebhook(alert)
	}

	// 邮件通知预留接口（需要配置 notify 模块）
	if m.config.NotifyEmail {
		go m.sendEmail(alert)
	}
}

// sendEmail 发送邮件通知（预留实现）
func (m *Monitor) sendEmail(alert *Alert) {
	// 获取配额信息来构建更详细的消息
	m.manager.mu.RLock()
	quota, exists := m.manager.quotas[alert.QuotaID]
	m.manager.mu.RUnlock()

	quotaName := alert.QuotaID
	if exists {
		quotaName = quota.TargetName
	}

	subject := fmt.Sprintf("[NAS-OS] 存储配额告警 - %s", quotaName)
	_ = subject // 预留使用

	// 实际项目中会调用 notify.SendEmail(recipient, subject, body)
	fmt.Printf("[quota] 邮件告警通知：%s (类型：%s)\n", quotaName, alert.Type)
}

// sendWebhook 发送 Webhook 通知
func (m *Monitor) sendWebhook(alert *Alert) {
	if m.config.WebhookURL == "" {
		return
	}

	// 获取配额信息
	m.manager.mu.RLock()
	quota, exists := m.manager.quotas[alert.QuotaID]
	m.manager.mu.RUnlock()

	quotaName := alert.QuotaID
	targetType := ""
	if exists {
		quotaName = quota.TargetName
		targetType = string(quota.Type)
	}

	// 构建告警 payload
	payload := map[string]interface{}{
		"alert_id":      alert.ID,
		"type":          "quota_alert",
		"quota_id":      alert.QuotaID,
		"quota_name":    quotaName,
		"volume":        alert.VolumeName,
		"target_type":   targetType,
		"target_name":   alert.TargetName,
		"alert_type":    alert.Type,
		"used_bytes":    alert.UsedBytes,
		"limit_bytes":   alert.LimitBytes,
		"usage_percent": alert.UsagePercent,
		"status":        alert.Status,
		"created_at":    alert.CreatedAt,
		"message":       alert.Message,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("[quota] 序列化 webhook payload 失败：%v\n", err)
		return
	}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("POST", m.config.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("[quota] 创建 webhook 请求失败：%v\n", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Alert-Type", "quota")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[quota] 发送 webhook 失败：%v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Printf("[quota] webhook 返回异常状态：%d\n", resp.StatusCode)
		return
	}

	fmt.Printf("[quota] webhook 通知发送成功：%s\n", alert.ID)
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

// ========== 趋势统计增强 ==========

// GetTrendStats 获取趋势统计
func (m *Monitor) GetTrendStats(quotaID string, duration time.Duration) *TrendStats {
	m.trendMu.RLock()
	history := m.trendData[quotaID]
	m.trendMu.RUnlock()

	if len(history) == 0 {
		return nil
	}

	// 过滤时间范围内的数据
	cutoff := time.Now().Add(-duration)
	filtered := make([]TrendDataPoint, 0)
	for _, point := range history {
		if point.Timestamp.After(cutoff) {
			filtered = append(filtered, point)
		}
	}

	if len(filtered) == 0 {
		return nil
	}

	stats := &TrendStats{
		QuotaID:        quotaID,
		StartTime:      filtered[0].Timestamp,
		EndTime:        filtered[len(filtered)-1].Timestamp,
		DataPointCount: len(filtered),
	}

	// 计算统计数据
	var totalUsed, totalPercent float64
	stats.MinUsedBytes = filtered[0].UsedBytes
	stats.MaxUsedBytes = filtered[0].UsedBytes
	stats.MinUsagePercent = filtered[0].UsagePercent
	stats.MaxUsagePercent = filtered[0].UsagePercent

	for _, point := range filtered {
		totalUsed += float64(point.UsedBytes)
		totalPercent += point.UsagePercent

		if point.UsedBytes < stats.MinUsedBytes {
			stats.MinUsedBytes = point.UsedBytes
		}
		if point.UsedBytes > stats.MaxUsedBytes {
			stats.MaxUsedBytes = point.UsedBytes
		}
		if point.UsagePercent < stats.MinUsagePercent {
			stats.MinUsagePercent = point.UsagePercent
		}
		if point.UsagePercent > stats.MaxUsagePercent {
			stats.MaxUsagePercent = point.UsagePercent
		}
	}

	stats.AvgUsedBytes = totalUsed / float64(len(filtered))
	stats.AvgUsagePercent = totalPercent / float64(len(filtered))

	// 当前值
	latest := filtered[len(filtered)-1]
	stats.CurrentUsedBytes = latest.UsedBytes
	stats.CurrentUsagePercent = latest.UsagePercent

	// 峰值分析
	stats.PeakUsedBytes = stats.MaxUsedBytes
	stats.PeakUsagePercent = stats.MaxUsagePercent
	for _, point := range filtered {
		if point.UsedBytes == stats.MaxUsedBytes {
			stats.PeakTime = &point.Timestamp
			break
		}
	}

	// 增长率计算
	stats.GrowthRate = m.CalculateGrowthRate(quotaID)
	if stats.AvgUsedBytes > 0 {
		stats.GrowthPercent = (stats.GrowthRate / stats.AvgUsedBytes) * 100
	}

	// 获取配额信息来计算预测填满时间
	m.manager.mu.RLock()
	quota, exists := m.manager.quotas[quotaID]
	m.manager.mu.RUnlock()

	if exists {
		stats.ProjectedDaysToFull = m.PredictFullTime(quotaID, quota.HardLimit)
		if stats.ProjectedDaysToFull > 0 {
			projectedDate := time.Now().AddDate(0, 0, stats.ProjectedDaysToFull)
			stats.ProjectedFullDate = &projectedDate
		}
	}

	return stats
}

// GetAllTrendStats 获取所有配额的趋势统计
func (m *Monitor) GetAllTrendStats(duration time.Duration) []*TrendStats {
	m.trendMu.RLock()
	quotaIDs := make([]string, 0, len(m.trendData))
	for id := range m.trendData {
		quotaIDs = append(quotaIDs, id)
	}
	m.trendMu.RUnlock()

	stats := make([]*TrendStats, 0, len(quotaIDs))
	for _, id := range quotaIDs {
		if s := m.GetTrendStats(id, duration); s != nil {
			// 添加目标名称
			m.manager.mu.RLock()
			if quota, exists := m.manager.quotas[id]; exists {
				s.TargetName = quota.TargetName
			}
			m.manager.mu.RUnlock()
			stats = append(stats, s)
		}
	}

	return stats
}

// ExportTrendData 导出趋势数据（用于持久化）
func (m *Monitor) ExportTrendData() map[string][]TrendDataPoint {
	m.trendMu.RLock()
	defer m.trendMu.RUnlock()

	result := make(map[string][]TrendDataPoint)
	for id, data := range m.trendData {
		result[id] = append([]TrendDataPoint{}, data...)
	}
	return result
}

// ImportTrendData 导入趋势数据（用于恢复）
func (m *Monitor) ImportTrendData(data map[string][]TrendDataPoint) {
	m.trendMu.Lock()
	defer m.trendMu.Unlock()

	for id, points := range data {
		m.trendData[id] = append([]TrendDataPoint{}, points...)
	}
}
