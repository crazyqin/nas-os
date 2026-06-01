// Package zerotrustaccess 提供安全审计日志和合规检查
package zerotrustaccess

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ========== 审计日志系统 ==========

// AuditEvent 审计事件类型
type AuditEventType string

const (
	// 访问事件
	AuditEventAccessRequest  AuditEventType = "access_request"
	AuditEventAccessGranted  AuditEventType = "access_granted"
	AuditEventAccessDenied   AuditEventType = "access_denied"
	AuditEventAccessRevoked  AuditEventType = "access_revoked"

	// 身份事件
	AuditEventLogin          AuditEventType = "login"
	AuditEventLogout         AuditEventType = "logout"
	AuditEventLoginFailed    AuditEventType = "login_failed"
	AuditEventMFAChallenge   AuditEventType = "mfa_challenge"
	AuditEventMFASuccess     AuditEventType = "mfa_success"
	AuditEventMFAFailed      AuditEventType = "mfa_failed"

	// 设备事件
	AuditEventDeviceRegister AuditEventType = "device_register"
	AuditEventDeviceComply   AuditEventType = "device_compliance"
	AuditEventDeviceBlock    AuditEventType = "device_block"
	AuditEventDeviceTrust    AuditEventType = "device_trust_change"

	// 风险事件
	AuditEventRiskDetected   AuditEventType = "risk_detected"
	AuditEventRiskEscalated  AuditEventType = "risk_escalated"
	AuditEventAnomalyDetected AuditEventType = "anomaly_detected"

	// 系统事件
	AuditEventPolicyChange   AuditEventType = "policy_change"
	AuditEventConfigChange   AuditEventType = "config_change"
	AuditEventSystemAlert    AuditEventType = "system_alert"
)

// AuditSeverity 审计严重级别
type AuditSeverity string

const (
	SeverityInfo     AuditSeverity = "info"
	SeverityWarning  AuditSeverity = "warning"
	SeverityError    AuditSeverity = "error"
	SeverityCritical AuditSeverity = "critical"
)

// SecurityAuditLog 安全审计日志
type SecurityAuditLog struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	EventType   AuditEventType         `json:"event_type"`
	Severity    AuditSeverity          `json:"severity"`
	Actor       AuditActor             `json:"actor"`
	Resource    AuditResource          `json:"resource"`
	Action      string                 `json:"action"`
	Result      string                 `json:"result"`
	Details     map[string]interface{} `json:"details"`
	RiskScore   float64                `json:"risk_score"`
	IPAddress   string                 `json:"ip_address"`
	UserAgent   string                 `json:"user_agent"`
	Location    *GeoLocation           `json:"location,omitempty"`
	SessionID   string                 `json:"session_id"`
	TraceID     string                 `json:"trace_id"`
	Tags        []string               `json:"tags,omitempty"`
	Retention   time.Duration          `json:"retention"`
}

// AuditActor 审计主体
type AuditActor struct {
	Type     string `json:"type"` // "user", "service", "system", "device"
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	DeviceID string `json:"device_id,omitempty"`
}

// AuditResource 审计资源
type AuditResource struct {
	Type string `json:"type"` // "user", "device", "policy", "permission", "data", "system"
	ID   string `json:"id"`
	Name string `json:"name"`
}

// GeoLocation 地理位置
type GeoLocation struct {
	Country   string  `json:"country"`
	Region    string  `json:"region"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	ISP       string  `json:"isp,omitempty"`
}

// ========== 审计管理器 ==========

// AuditManager 审计管理器
type AuditManager struct {
	mu             sync.RWMutex
	logs           []*SecurityAuditLog
	maxLogs        int
	retentionDays  int
	alertThresholds map[AuditEventType]int
	alertHandlers  []AuditAlertHandler
}

// AuditAlertHandler 审计告警处理器
type AuditAlertHandler func(log *SecurityAuditLog)

// AuditSearchCriteria 审计搜索条件
type AuditSearchCriteria struct {
	StartTime  *time.Time       `json:"start_time,omitempty"`
	EndTime    *time.Time       `json:"end_time,omitempty"`
	EventTypes []AuditEventType `json:"event_types,omitempty"`
	Severity   []AuditSeverity  `json:"severity,omitempty"`
	ActorID    string           `json:"actor_id,omitempty"`
	ActorType  string           `json:"actor_type,omitempty"`
	ResourceType string         `json:"resource_type,omitempty"`
	ResourceID string           `json:"resource_id,omitempty"`
	RiskScoreMin *float64       `json:"risk_score_min,omitempty"`
	RiskScoreMax *float64       `json:"risk_score_max,omitempty"`
	IPAddress  string           `json:"ip_address,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
}

// AuditStatistics 审计统计
type AuditStatistics struct {
	TotalEvents     int                     `json:"total_events"`
	EventsByType    map[AuditEventType]int  `json:"events_by_type"`
	EventsBySeverity map[AuditSeverity]int  `json:"events_by_severity"`
	FailedAttempts  int                     `json:"failed_attempts"`
	HighRiskEvents  int                     `json:"high_risk_events"`
	AverageRiskScore float64                `json:"average_risk_score"`
	TopActors       []ActorStatistics       `json:"top_actors"`
	TopResources    []ResourceStatistics    `json:"top_resources"`
	TimeRange       TimeRange               `json:"time_range"`
}

// ActorStatistics 主体统计
type ActorStatistics struct {
	ActorID      string `json:"actor_id"`
	EventCount   int    `json:"event_count"`
	FailedCount  int    `json:"failed_count"`
	AvgRiskScore float64 `json:"avg_risk_score"`
}

// ResourceStatistics 资源统计
type ResourceStatistics struct {
	ResourceID   string `json:"resource_id"`
	AccessCount  int    `json:"access_count"`
	DeniedCount  int    `json:"denied_count"`
}

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// NewAuditManager 创建审计管理器
func NewAuditManager(maxLogs, retentionDays int) *AuditManager {
	return &AuditManager{
		logs:           make([]*SecurityAuditLog, 0, maxLogs),
		maxLogs:        maxLogs,
		retentionDays:  retentionDays,
		alertThresholds: make(map[AuditEventType]int),
		alertHandlers:  make([]AuditAlertHandler, 0),
	}
}

// LogEvent 记录审计事件
func (m *AuditManager) LogEvent(event *SecurityAuditLog) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	m.logs = append(m.logs, event)

	// 检查是否超过最大日志数
	if len(m.logs) > m.maxLogs {
		// 移除最旧的日志
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}

	// 检查告警
	m.checkAlerts(event)
}

// checkAlerts 检查告警条件
func (m *AuditManager) checkAlerts(event *SecurityAuditLog) {
	// 高风险事件告警
	if event.RiskScore >= 70 || event.Severity == SeverityCritical {
		for _, handler := range m.alertHandlers {
			handler(event)
		}
	}

	// 阈值告警
	if threshold, ok := m.alertThresholds[event.EventType]; ok {
		count := m.countRecentEvents(event.EventType, time.Hour)
		if count >= threshold {
			for _, handler := range m.alertHandlers {
				handler(event)
			}
		}
	}
}

// countRecentEvents 统计最近事件数
func (m *AuditManager) countRecentEvents(eventType AuditEventType, duration time.Duration) int {
	cutoff := time.Now().Add(-duration)
	count := 0
	for _, log := range m.logs {
		if log.EventType == eventType && log.Timestamp.After(cutoff) {
			count++
		}
	}
	return count
}

// SearchLogs 搜索审计日志
func (m *AuditManager) SearchLogs(criteria AuditSearchCriteria) []*SecurityAuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*SecurityAuditLog

	for _, log := range m.logs {
		if m.matchesCriteria(log, criteria) {
			results = append(results, log)
		}
	}

	// 分页
	if criteria.Offset >= len(results) {
		return nil
	}

	end := criteria.Offset + criteria.Limit
	if criteria.Limit <= 0 || end > len(results) {
		end = len(results)
	}

	return results[criteria.Offset:end]
}

// matchesCriteria 匹配条件
func (m *AuditManager) matchesCriteria(log *SecurityAuditLog, criteria AuditSearchCriteria) bool {
	// 时间范围
	if criteria.StartTime != nil && log.Timestamp.Before(*criteria.StartTime) {
		return false
	}
	if criteria.EndTime != nil && log.Timestamp.After(*criteria.EndTime) {
		return false
	}

	// 事件类型
	if len(criteria.EventTypes) > 0 {
		found := false
		for _, t := range criteria.EventTypes {
			if log.EventType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 严重级别
	if len(criteria.Severity) > 0 {
		found := false
		for _, s := range criteria.Severity {
			if log.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 主体
	if criteria.ActorID != "" && log.Actor.ID != criteria.ActorID {
		return false
	}
	if criteria.ActorType != "" && log.Actor.Type != criteria.ActorType {
		return false
	}

	// 资源
	if criteria.ResourceType != "" && log.Resource.Type != criteria.ResourceType {
		return false
	}
	if criteria.ResourceID != "" && log.Resource.ID != criteria.ResourceID {
		return false
	}

	// 风险分数
	if criteria.RiskScoreMin != nil && log.RiskScore < *criteria.RiskScoreMin {
		return false
	}
	if criteria.RiskScoreMax != nil && log.RiskScore > *criteria.RiskScoreMax {
		return false
	}

	// IP地址
	if criteria.IPAddress != "" && log.IPAddress != criteria.IPAddress {
		return false
	}

	// 标签
	if len(criteria.Tags) > 0 {
		for _, tag := range criteria.Tags {
			found := false
			for _, logTag := range log.Tags {
				if logTag == tag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	return true
}

// GetStatistics 获取审计统计
func (m *AuditManager) GetStatistics(start, end time.Time) *AuditStatistics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &AuditStatistics{
		EventsByType:     make(map[AuditEventType]int),
		EventsBySeverity: make(map[AuditSeverity]int),
		TimeRange:        TimeRange{Start: start, End: end},
	}

	var totalRisk float64
	actorStats := make(map[string]*ActorStatistics)
	resourceStats := make(map[string]*ResourceStatistics)

	for _, log := range m.logs {
		if log.Timestamp.Before(start) || log.Timestamp.After(end) {
			continue
		}

		stats.TotalEvents++
		stats.EventsByType[log.EventType]++
		stats.EventsBySeverity[log.Severity]++

		totalRisk += log.RiskScore

		if log.RiskScore >= 60 {
			stats.HighRiskEvents++
		}

		// 统计失败尝试
		if log.Result == "fail" || log.Result == "deny" {
			stats.FailedAttempts++
		}

		// 主体统计
		if log.Actor.ID != "" {
			if _, ok := actorStats[log.Actor.ID]; !ok {
				actorStats[log.Actor.ID] = &ActorStatistics{
					ActorID: log.Actor.ID,
				}
			}
			actorStats[log.Actor.ID].EventCount++
			if log.Result == "fail" {
				actorStats[log.Actor.ID].FailedCount++
			}
		}

		// 资源统计
		if log.Resource.ID != "" {
			if _, ok := resourceStats[log.Resource.ID]; !ok {
				resourceStats[log.Resource.ID] = &ResourceStatistics{
					ResourceID: log.Resource.ID,
				}
			}
			resourceStats[log.Resource.ID].AccessCount++
			if log.Result == "deny" {
				resourceStats[log.Resource.ID].DeniedCount++
			}
		}
	}

	if stats.TotalEvents > 0 {
		stats.AverageRiskScore = totalRisk / float64(stats.TotalEvents)
	}

	// 转换为列表
	for _, as := range actorStats {
		stats.TopActors = append(stats.TopActors, *as)
	}
	for _, rs := range resourceStats {
		stats.TopResources = append(stats.TopResources, *rs)
	}

	return stats
}

// AddAlertHandler 添加告警处理器
func (m *AuditManager) AddAlertHandler(handler AuditAlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alertHandlers = append(m.alertHandlers, handler)
}

// SetAlertThreshold 设置告警阈值
func (m *AuditManager) SetAlertThreshold(eventType AuditEventType, threshold int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alertThresholds[eventType] = threshold
}

// GetLogsByActor 获取指定主体的日志
func (m *AuditManager) GetLogsByActor(actorID string, limit int) []*SecurityAuditLog {
	return m.SearchLogs(AuditSearchCriteria{
		ActorID: actorID,
		Limit:   limit,
	})
}

// GetHighRiskLogs 获取高风险日志
func (m *AuditManager) GetHighRiskLogs(minScore float64, limit int) []*SecurityAuditLog {
	return m.SearchLogs(AuditSearchCriteria{
		RiskScoreMin: &minScore,
		Limit:        limit,
	})
}

// ExportLogs 导出日志为JSON
func (m *AuditManager) ExportLogs(criteria AuditSearchCriteria) ([]byte, error) {
	logs := m.SearchLogs(criteria)
	return json.MarshalIndent(logs, "", "  ")
}

// CleanupOldLogs 清理过期日志
func (m *AuditManager) CleanupOldLogs() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.retentionDays)
	var kept []*SecurityAuditLog

	for _, log := range m.logs {
		if log.Timestamp.After(cutoff) {
			kept = append(kept, log)
		}
	}

	removed := len(m.logs) - len(kept)
	m.logs = kept

	return removed
}

// GetLogCount 获取日志数量
func (m *AuditManager) GetLogCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.logs)
}
