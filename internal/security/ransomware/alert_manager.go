package ransomware

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// AlertManager 告警管理器.
type AlertManager struct {
	config     AlertConfig
	alerts     []*Alert
	alertMu    sync.RWMutex
	cooldown   map[string]time.Time // 告警冷却
	cooldownMu sync.RWMutex
	notifyCh   chan<- Alert
	stats      Statistics
	statsMu    sync.RWMutex
}

// NewAlertManager 创建告警管理器.
func NewAlertManager(config AlertConfig) *AlertManager {
	return &AlertManager{
		config:   config,
		alerts:   make([]*Alert, 0),
		cooldown: make(map[string]time.Time),
	}
}

// SetNotifyChannel 设置通知通道.
func (am *AlertManager) SetNotifyChannel(ch chan<- Alert) {
	am.notifyCh = ch
}

// CreateAlert 创建告警.
func (am *AlertManager) CreateAlert(result *DetectionResult) *Alert {
	if !am.config.Enabled {
		return nil
	}

	// 检查严重级别是否达到阈值
	if !am.meetsMinSeverity(result.ThreatLevel) {
		return nil
	}

	// 检查冷却
	alertKey := string(result.DetectionType) + ":" + result.FilePath
	if am.isInCooldown(alertKey) {
		return nil
	}

	alert := &Alert{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		Severity:      result.ThreatLevel,
		Type:          string(result.DetectionType),
		Title:         am.generateAlertTitle(result),
		Message:       am.generateAlertMessage(result),
		DetectionID:   result.ID,
		AffectedPath:  result.FilePath,
		AffectedFiles: result.AffectedFiles,
		SignatureName: result.SignatureName,
		Confidence:    result.Confidence,
		Status:        AlertStatusNew,
		ActionTaken:   []string{},
		Details:       result.Details,
	}

	if result.ProcessInfo != nil {
		alert.ProcessInfo = result.ProcessInfo
	}

	// 存储告警
	am.storeAlert(alert)

	// 设置冷却
	am.setCooldown(alertKey)

	// 发送通知
	am.sendNotification(alert)

	// 更新统计
	am.updateStats(alert)

	return alert
}

// generateAlertTitle 生成告警标题.
func (am *AlertManager) generateAlertTitle(result *DetectionResult) string {
	switch result.DetectionType {
	case DetectionTypeExtension:
		return "检测到勒索软件文件扩展名: " + result.SignatureName
	case DetectionTypeSignature:
		return "检测到勒索信: " + result.SignatureName
	case DetectionTypeBehavior:
		return "检测到异常加密行为: " + result.BehaviorName
	default:
		return "检测到潜在勒索软件威胁"
	}
}

// generateAlertMessage 生成告警消息.
func (am *AlertManager) generateAlertMessage(result *DetectionResult) string {
	msg := "威胁级别: " + string(result.ThreatLevel) + "\n"
	msg += "检测类型: " + string(result.DetectionType) + "\n"
	msg += "置信度: " + formatConfidence(result.Confidence) + "\n"
	msg += "受影响文件: " + result.FilePath + "\n"

	if result.SignatureName != "" {
		msg += "勒索软件家族: " + result.SignatureName + "\n"
	}

	if result.FileCount > 0 {
		msg += "受影响文件数: " + itoa(result.FileCount) + "\n"
	}

	msg += "\n建议操作: " + result.SuggestedAction

	return msg
}

// storeAlert 存储告警.
func (am *AlertManager) storeAlert(alert *Alert) {
	am.alertMu.Lock()
	defer am.alertMu.Unlock()

	// 限制最大告警数
	if len(am.alerts) >= am.config.MaxAlerts {
		am.alerts = am.alerts[1:]
	}

	am.alerts = append(am.alerts, alert)
}

// isInCooldown 检查是否在冷却期.
func (am *AlertManager) isInCooldown(key string) bool {
	am.cooldownMu.RLock()
	defer am.cooldownMu.RUnlock()

	cooldownTime, exists := am.cooldown[key]
	if !exists {
		return false
	}

	return time.Now().Before(cooldownTime)
}

// setCooldown 设置冷却.
func (am *AlertManager) setCooldown(key string) {
	am.cooldownMu.Lock()
	defer am.cooldownMu.Unlock()

	am.cooldown[key] = time.Now().Add(am.config.CooldownPeriod)
}

// sendNotification 发送通知.
func (am *AlertManager) sendNotification(alert *Alert) {
	if am.notifyCh != nil {
		select {
		case am.notifyCh <- *alert:
		default:
			// 通道阻塞，丢弃通知
		}
	}
}

// meetsMinSeverity 检查是否达到最低严重级别.
func (am *AlertManager) meetsMinSeverity(level ThreatLevel) bool {
	severityOrder := map[ThreatLevel]int{
		ThreatLevelNone:     0,
		ThreatLevelLow:      1,
		ThreatLevelMedium:   2,
		ThreatLevelHigh:     3,
		ThreatLevelCritical: 4,
	}

	return severityOrder[level] >= severityOrder[am.config.MinSeverity]
}

// GetAlert 获取指定告警.
func (am *AlertManager) GetAlert(id string) (*Alert, bool) {
	am.alertMu.RLock()
	defer am.alertMu.RUnlock()

	for _, alert := range am.alerts {
		if alert.ID == id {
			return alert, true
		}
	}
	return nil, false
}

// GetAlerts 获取告警列表.
func (am *AlertManager) GetAlerts(limit, offset int, severity *ThreatLevel, status *AlertStatus) []*Alert {
	am.alertMu.RLock()
	defer am.alertMu.RUnlock()

	var filtered []*Alert
	for _, alert := range am.alerts {
		if severity != nil && alert.Severity != *severity {
			continue
		}
		if status != nil && alert.Status != *status {
			continue
		}
		filtered = append(filtered, alert)
	}

	// 应用分页
	if offset >= len(filtered) {
		return []*Alert{}
	}

	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end]
}

// AcknowledgeAlert 确认告警.
func (am *AlertManager) AcknowledgeAlert(id, acknowledgedBy string) error {
	am.alertMu.Lock()
	defer am.alertMu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == id {
			alert.Status = AlertStatusAcknowledged
			alert.AcknowledgedBy = acknowledgedBy
			now := time.Now()
			alert.AcknowledgedAt = &now
			return nil
		}
	}

	return ErrAlertNotFound
}

// ResolveAlert 解决告警.
func (am *AlertManager) ResolveAlert(id string) error {
	am.alertMu.Lock()
	defer am.alertMu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == id {
			alert.Status = AlertStatusResolved
			now := time.Now()
			alert.ResolvedAt = &now
			return nil
		}
	}

	return ErrAlertNotFound
}

// MarkFalsePositive 标记为误报.
func (am *AlertManager) MarkFalsePositive(id string) error {
	am.alertMu.Lock()
	defer am.alertMu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == id {
			alert.Status = AlertStatusFalsePositive
			return nil
		}
	}

	return ErrAlertNotFound
}

// AddActionTaken 添加已执行的操作.
func (am *AlertManager) AddActionTaken(id, action string) error {
	am.alertMu.Lock()
	defer am.alertMu.Unlock()

	for _, alert := range am.alerts {
		if alert.ID == id {
			alert.ActionTaken = append(alert.ActionTaken, action)
			return nil
		}
	}

	return ErrAlertNotFound
}

// GetUnacknowledgedCount 获取未确认告警数量.
func (am *AlertManager) GetUnacknowledgedCount() int {
	am.alertMu.RLock()
	defer am.alertMu.RUnlock()

	var count int
	for _, alert := range am.alerts {
		if alert.Status == AlertStatusNew {
			count++
		}
	}
	return count
}

// GetStats 获取告警统计.
func (am *AlertManager) GetStats() map[string]interface{} {
	am.alertMu.RLock()
	defer am.alertMu.RUnlock()

	stats := map[string]interface{}{
		"total_alerts":   len(am.alerts),
		"by_severity":    make(map[ThreatLevel]int),
		"by_status":      make(map[AlertStatus]int),
		"unacknowledged": am.GetUnacknowledgedCount(),
	}

	severityStats, ok1 := stats["by_severity"].(map[ThreatLevel]int)
	statusStats, ok2 := stats["by_status"].(map[AlertStatus]int)
	if !ok1 || !ok2 {
		return stats
	}

	for _, alert := range am.alerts {
		severityStats[alert.Severity]++
		statusStats[alert.Status]++
	}

	return stats
}

// updateStats 更新统计.
func (am *AlertManager) updateStats(alert *Alert) {
	am.statsMu.Lock()
	defer am.statsMu.Unlock()

	am.stats.TotalAlerts++
	if am.stats.ByThreatLevel == nil {
		am.stats.ByThreatLevel = make(map[ThreatLevel]int64)
	}
	am.stats.ByThreatLevel[alert.Severity]++
}

// ClearOldAlerts 清除过期告警.
func (am *AlertManager) ClearOldAlerts(olderThan time.Duration) int {
	am.alertMu.Lock()
	defer am.alertMu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	var newAlerts []*Alert
	var cleared int

	for _, alert := range am.alerts {
		if alert.Timestamp.After(cutoff) {
			newAlerts = append(newAlerts, alert)
		} else {
			cleared++
		}
	}

	am.alerts = newAlerts
	return cleared
}

// Helper functions

func formatConfidence(c float64) string {
	return itoa(int(c*100)) + "%"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var negative bool
	if i < 0 {
		negative = true
		i = -i
	}

	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}

// ErrAlertNotFound indicates that an alert was not found.
var ErrAlertNotFound = &AlertError{Message: "告警不存在"}

// AlertError 告警错误.
type AlertError struct {
	Message string
}

func (e *AlertError) Error() string {
	return e.Message
}
