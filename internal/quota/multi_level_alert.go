// Package quota 提供配额多级告警系统
// 实现存储配额的多级预警、自动升级和通知机制
package quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 多级告警系统 ==========

// MultiLevelAlertSystem 多级告警系统
type MultiLevelAlertSystem struct {
	config    *MultiLevelAlertConfig
	storage   AlertStorage
	notifier  AlertNotifier
	escalator AlertEscalator
	logger    interface{} // 使用interface避免依赖

	alerts     map[string]*ActiveAlert // alertID -> alert
	thresholds []*AlertThreshold
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// MultiLevelAlertConfig 多级告警配置
type MultiLevelAlertConfig struct {
	Enabled           bool              `json:"enabled"`
	CheckInterval     time.Duration     `json:"check_interval"`     // 检查间隔
	DefaultThresholds []*AlertThreshold `json:"default_thresholds"` // 默认阈值

	// 告警级别配置
	Levels          []MultiAlertLevelConfig `json:"levels"`           // 告警级别
	SilenceDuration time.Duration           `json:"silence_duration"` // 静默时长
	SilenceCooldown time.Duration           `json:"silence_cooldown"` // 静默冷却时间

	// 升级配置
	EscalationEnabled     bool            `json:"escalation_enabled"`      // 启用告警升级
	EscalationIntervals   []time.Duration `json:"escalation_intervals"`    // 各级别升级间隔
	MaxEscalationLevel    int             `json:"max_escalation_level"`    // 最大升级级别
	EscalationAutoResolve bool            `json:"escalation_auto_resolve"` // 自动解决后降级

	// 通知配置
	NotifyEmail       bool     `json:"notify_email"`        // 邮件通知
	NotifyWebhook     bool     `json:"notify_webhook"`      // Webhook通知
	NotifyPush        bool     `json:"notify_push"`         // 推送通知
	NotifySMS         bool     `json:"notify_sms"`          // 短信通知
	WebhookURLs       []string `json:"webhook_urls"`        // Webhook地址列表
	EmailRecipients   []string `json:"email_recipients"`    // 邮件接收人
	ExcludeEmailLevel []string `json:"exclude_email_level"` // 不发邮件的级别

	// 聚合配置
	AggregationEnabled   bool          `json:"aggregation_enabled"`    // 启用告警聚合
	AggregationWindow    time.Duration `json:"aggregation_window"`     // 聚合窗口
	AggregationMaxAlerts int           `json:"aggregation_max_alerts"` // 单次最大聚合数

	// 持久化配置
	PersistEnabled  bool          `json:"persist_enabled"`  // 启用持久化
	PersistPath     string        `json:"persist_path"`     // 持久化路径
	PersistInterval time.Duration `json:"persist_interval"` // 持久化间隔
}

// AlertThreshold 告警阈值
type AlertThreshold struct {
	Name       string        `json:"name"`        // 阈值名称
	Level      AlertSeverity `json:"level"`       // 告警级别
	Percentage float64       `json:"percentage"`  // 触发百分比
	Duration   time.Duration `json:"duration"`    // 持续时间要求（可选）
	Message    string        `json:"message"`     // 自定义消息
	AutoAction string        `json:"auto_action"` // 自动动作（可选）
}

// MultiAlertLevelConfig 多级告警级别配置
type MultiAlertLevelConfig struct {
	Name           string        `json:"name"`            // 级别名称
	Severity       AlertSeverity `json:"severity"`        // 严重程度
	Color          string        `json:"color"`           // 显示颜色
	Icon           string        `json:"icon"`            // 图标
	Priority       int           `json:"priority"`        // 优先级（数值越大越优先）
	NotifyChannels []string      `json:"notify_channels"` // 通知渠道
	SilenceAble    bool          `json:"silence_able"`    // 可静默
	AutoEscalate   bool          `json:"auto_escalate"`   // 自动升级
	EscalateAfter  time.Duration `json:"escalate_after"`  // 升级时间
}

// ActiveAlert 活动告警
type ActiveAlert struct {
	ID              string        `json:"id"`
	QuotaID         string        `json:"quota_id"`
	QuotaName       string        `json:"quota_name"`
	CurrentLevel    AlertSeverity `json:"current_level"`
	OriginalLevel   AlertSeverity `json:"original_level"`
	TriggeredAt     time.Time     `json:"triggered_at"`
	LastEscalatedAt *time.Time    `json:"last_escalated_at,omitempty"`
	EscalationLevel int           `json:"escalation_level"` // 当前升级层级
	Status          AlertStatus   `json:"status"`
	UsagePercent    float64       `json:"usage_percent"`
	Threshold       float64       `json:"threshold"`
	UsedBytes       uint64        `json:"used_bytes"`
	LimitBytes      uint64        `json:"limit_bytes"`
	Message         string        `json:"message"`
	NotifyCount     int           `json:"notify_count"` // 通知次数
	LastNotifyAt    *time.Time    `json:"last_notify_at,omitempty"`
	SilencedAt      *time.Time    `json:"silenced_at,omitempty"`
	SilenceBy       string        `json:"silence_by,omitempty"`
	ResolvedAt      *time.Time    `json:"resolved_at,omitempty"`
	ResolveBy       string        `json:"resolve_by,omitempty"`
	History         []AlertEvent  `json:"history"` // 告警历史事件
}

// AlertEvent 告警事件
type AlertEvent struct {
	Timestamp time.Time     `json:"timestamp"`
	Type      string        `json:"type"` // triggered, escalated, notified, silenced, resolved
	Level     AlertSeverity `json:"level,omitempty"`
	Message   string        `json:"message,omitempty"`
	By        string        `json:"by,omitempty"` // 操作人
}

// MultiAlertNotification 多级告警通知
type MultiAlertNotification struct {
	AlertID      string        `json:"alert_id"`
	Level        AlertSeverity `json:"level"`
	QuotaName    string        `json:"quota_name"`
	UsagePercent float64       `json:"usage_percent"`
	Threshold    float64       `json:"threshold"`
	Message      string        `json:"message"`
	Timestamp    time.Time     `json:"timestamp"`
	Channels     []string      `json:"channels"` // 通知渠道
}

// MultiAlertSummary 多级告警汇总
type MultiAlertSummary struct {
	TotalAlerts       int            `json:"total_alerts"`
	ActiveAlerts      int            `json:"active_alerts"`
	SilencedAlerts    int            `json:"silenced_alerts"`
	ResolvedAlerts    int            `json:"resolved_alerts"`
	ByLevel           map[string]int `json:"by_level"`
	ByQuota           map[string]int `json:"by_quota"`
	TopAlerts         []ActiveAlert  `json:"top_alerts"`
	RecentEscalations int            `json:"recent_escalations"`
	PendingActions    int            `json:"pending_actions"`
}

// AlertStorage 告警存储接口
type AlertStorage interface {
	Save(alert *ActiveAlert) error
	Load(alertID string) (*ActiveAlert, error)
	List(filter AlertFilter) ([]*ActiveAlert, error)
	Delete(alertID string) error
}

// AlertNotifier 告警通知接口
type AlertNotifier interface {
	SendEmail(recipients []string, notification *MultiAlertNotification) error
	SendWebhook(url string, notification *MultiAlertNotification) error
	SendPush(notification *MultiAlertNotification) error
	SendSMS(phone string, notification *MultiAlertNotification) error
}

// AlertEscalator 告警升级器接口
type AlertEscalator interface {
	Escalate(alert *ActiveAlert) error
	ShouldEscalate(alert *ActiveAlert, now time.Time) bool
}

// AlertFilter 告警过滤条件
type AlertFilter struct {
	QuotaID   string        `json:"quota_id,omitempty"`
	Level     AlertSeverity `json:"level,omitempty"`
	Status    AlertStatus   `json:"status,omitempty"`
	StartTime *time.Time    `json:"start_time,omitempty"`
	EndTime   *time.Time    `json:"end_time,omitempty"`
	MinUsage  float64       `json:"min_usage,omitempty"`
	MaxUsage  float64       `json:"max_usage,omitempty"`
}

// DefaultMultiLevelAlertConfig 默认配置
func DefaultMultiLevelAlertConfig() *MultiLevelAlertConfig {
	return &MultiLevelAlertConfig{
		Enabled:               true,
		CheckInterval:         5 * time.Minute,
		SilenceDuration:       24 * time.Hour,
		SilenceCooldown:       1 * time.Hour,
		EscalationEnabled:     true,
		MaxEscalationLevel:    3,
		EscalationAutoResolve: true,
		NotifyEmail:           true,
		NotifyWebhook:         true,
		AggregationEnabled:    true,
		AggregationWindow:     10 * time.Minute,
		AggregationMaxAlerts:  20,
		PersistEnabled:        true,
		PersistInterval:       30 * time.Minute,
		DefaultThresholds: []*AlertThreshold{
			{Name: "info", Level: AlertSeverityInfo, Percentage: 60, Message: "存储使用已达到 %.1f%%"},
			{Name: "warning", Level: AlertSeverityWarning, Percentage: 70, Message: "存储使用已达到 %.1f%%，请注意"},
			{Name: "critical", Level: AlertSeverityCritical, Percentage: 85, Message: "存储使用已达到 %.1f%%，请及时处理"},
			{Name: "emergency", Level: AlertSeverityEmergency, Percentage: 95, Message: "存储使用已达到 %.1f%%，即将超出限制"},
		},
		Levels: []MultiAlertLevelConfig{
			{Name: "info", Severity: AlertSeverityInfo, Color: "#3B82F6", Icon: "ℹ️", Priority: 1, NotifyChannels: []string{"push"}, SilenceAble: false, AutoEscalate: false},
			{Name: "warning", Severity: AlertSeverityWarning, Color: "#F59E0B", Icon: "⚠️", Priority: 2, NotifyChannels: []string{"push", "email"}, SilenceAble: true, AutoEscalate: true, EscalateAfter: 30 * time.Minute},
			{Name: "critical", Severity: AlertSeverityCritical, Color: "#EF4444", Icon: "🔴", Priority: 3, NotifyChannels: []string{"push", "email", "webhook"}, SilenceAble: true, AutoEscalate: true, EscalateAfter: 15 * time.Minute},
			{Name: "emergency", Severity: AlertSeverityEmergency, Color: "#7C3AED", Icon: "🚨", Priority: 4, NotifyChannels: []string{"push", "email", "webhook", "sms"}, SilenceAble: true, AutoEscalate: false},
		},
		EscalationIntervals: []time.Duration{
			30 * time.Minute, // Level 1 -> 2
			15 * time.Minute, // Level 2 -> 3
			5 * time.Minute,  // Level 3 -> 4
		},
	}
}

// NewMultiLevelAlertSystem 创建多级告警系统
func NewMultiLevelAlertSystem(config *MultiLevelAlertConfig) (*MultiLevelAlertSystem, error) {
	if config == nil {
		config = DefaultMultiLevelAlertConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	system := &MultiLevelAlertSystem{
		config:     config,
		alerts:     make(map[string]*ActiveAlert),
		thresholds: config.DefaultThresholds,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 加载持久化数据
	if config.PersistEnabled && config.PersistPath != "" {
		if err := system.loadPersistedAlerts(); err == nil {
			// 恢复活动告警状态
		}
	}

	// 启动检查循环
	if config.Enabled {
		system.wg.Add(1)
		go system.checkLoop()
	}

	// 启动升级检查
	if config.EscalationEnabled {
		system.wg.Add(1)
		go system.escalationLoop()
	}

	// 启动持久化
	if config.PersistEnabled {
		system.wg.Add(1)
		go system.persistLoop()
	}

	return system, nil
}

// checkLoop 检查循环
func (s *MultiLevelAlertSystem) checkLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkAllQuotas()
		}
	}
}

// checkAllQuotas 检查所有配额
func (s *MultiLevelAlertSystem) checkAllQuotas() {
	// 获取所有配额使用情况（通过存储接口）
	// 这里简化实现，实际应从quota manager获取
}

// CheckQuota 检查单个配额
func (s *MultiLevelAlertSystem) CheckQuota(quotaID, quotaName string, usedBytes, limitBytes uint64) (*ActiveAlert, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limitBytes == 0 {
		return nil, false
	}

	usagePercent := float64(usedBytes) / float64(limitBytes) * 100

	// 确定触发的阈值
	var triggeredThreshold *AlertThreshold
	for i := len(s.thresholds) - 1; i >= 0; i-- {
		th := s.thresholds[i]
		if usagePercent >= th.Percentage {
			triggeredThreshold = th
			break
		}
	}

	if triggeredThreshold == nil {
		// 检查是否有现有告警需要解决
		if alert, exists := s.alerts[quotaID]; exists {
			s.resolveAlert(alert, "usage_dropped")
		}
		return nil, false
	}

	// 检查是否已有告警
	if alert, exists := s.alerts[quotaID]; exists {
		// 更新告警
		alert.UsagePercent = usagePercent
		alert.UsedBytes = usedBytes
		alert.LimitBytes = limitBytes

		// 检查是否需要升级
		if triggeredThreshold.Level != alert.CurrentLevel &&
			s.getLevelPriority(triggeredThreshold.Level) > s.getLevelPriority(alert.CurrentLevel) {
			s.escalateAlert(alert, triggeredThreshold.Level)
		}

		return alert, false
	}

	// 创建新告警
	alert := s.createAlert(quotaID, quotaName, usagePercent, usedBytes, limitBytes, triggeredThreshold)
	s.alerts[quotaID] = alert

	// 发送通知
	s.notifyAlert(alert)

	return alert, true
}

// createAlert 创建告警
func (s *MultiLevelAlertSystem) createAlert(quotaID, quotaName string, usagePercent float64, usedBytes, limitBytes uint64, threshold *AlertThreshold) *ActiveAlert {
	alertID := generateAlertID(quotaID)
	message := fmt.Sprintf(threshold.Message, usagePercent)

	alert := &ActiveAlert{
		ID:              alertID,
		QuotaID:         quotaID,
		QuotaName:       quotaName,
		CurrentLevel:    threshold.Level,
		OriginalLevel:   threshold.Level,
		TriggeredAt:     time.Now(),
		Status:          AlertStatusActive,
		UsagePercent:    usagePercent,
		Threshold:       threshold.Percentage,
		UsedBytes:       usedBytes,
		LimitBytes:      limitBytes,
		Message:         message,
		EscalationLevel: 0,
		History: []AlertEvent{
			{Timestamp: time.Now(), Type: "triggered", Level: threshold.Level, Message: message},
		},
	}

	return alert
}

// escalateAlert 升级告警
func (s *MultiLevelAlertSystem) escalateAlert(alert *ActiveAlert, newLevel AlertSeverity) {
	now := time.Now()
	alert.CurrentLevel = newLevel
	alert.EscalationLevel++
	alert.LastEscalatedAt = &now

	levelConfig := s.getLevelConfig(newLevel)
	if levelConfig != nil {
		alert.Message = fmt.Sprintf("[升级] %s - 当前使用 %.1f%%", levelConfig.Name, alert.UsagePercent)
	}

	alert.History = append(alert.History, AlertEvent{
		Timestamp: now,
		Type:      "escalated",
		Level:     newLevel,
		Message:   fmt.Sprintf("告警升级至 %s 级别", newLevel),
	})

	// 发送升级通知
	s.notifyAlert(alert)
}

// resolveAlert 解决告警
func (s *MultiLevelAlertSystem) resolveAlert(alert *ActiveAlert, reason string) {
	now := time.Now()
	alert.Status = AlertStatusResolved
	alert.ResolvedAt = &now
	alert.ResolveBy = reason

	alert.History = append(alert.History, AlertEvent{
		Timestamp: now,
		Type:      "resolved",
		Message:   fmt.Sprintf("告警已解决: %s", reason),
	})

	// 发送解决通知
	if s.notifier != nil {
		notification := &MultiAlertNotification{
			AlertID:   alert.ID,
			Level:     alert.CurrentLevel,
			QuotaName: alert.QuotaName,
			Message:   fmt.Sprintf("告警已解决: %s", alert.QuotaName),
			Timestamp: now,
		}
		s.notifyChannels(alert, notification)
	}

	// 从活动告警中移除
	delete(s.alerts, alert.QuotaID)
}

// silenceAlert 静默告警
func (s *MultiLevelAlertSystem) silenceAlert(alertID, user string, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, alert := range s.alerts {
		if alert.ID == alertID {
			now := time.Now()
			alert.Status = AlertStatusSilenced
			alert.SilencedAt = &now
			alert.SilenceBy = user

			alert.History = append(alert.History, AlertEvent{
				Timestamp: now,
				Type:      "silenced",
				Message:   fmt.Sprintf("告警已静默 %v", duration),
				By:        user,
			})

			return nil
		}
	}

	return errors.New("告警不存在")
}

// notifyAlert 发送告警通知
func (s *MultiLevelAlertSystem) notifyAlert(alert *ActiveAlert) {
	if s.notifier == nil {
		return
	}

	now := time.Now()
	alert.NotifyCount++
	alert.LastNotifyAt = &now

	notification := &MultiAlertNotification{
		AlertID:      alert.ID,
		Level:        alert.CurrentLevel,
		QuotaName:    alert.QuotaName,
		UsagePercent: alert.UsagePercent,
		Threshold:    alert.Threshold,
		Message:      alert.Message,
		Timestamp:    now,
	}

	s.notifyChannels(alert, notification)

	alert.History = append(alert.History, AlertEvent{
		Timestamp: now,
		Type:      "notified",
		Message:   fmt.Sprintf("发送通知 (%s)", alert.CurrentLevel),
	})
}

// notifyChannels 通过渠道发送通知
func (s *MultiLevelAlertSystem) notifyChannels(alert *ActiveAlert, notification *MultiAlertNotification) {
	levelConfig := s.getLevelConfig(alert.CurrentLevel)
	if levelConfig == nil {
		return
	}

	for _, channel := range levelConfig.NotifyChannels {
		switch channel {
		case "email":
			if s.config.NotifyEmail && len(s.config.EmailRecipients) > 0 {
				s.notifier.SendEmail(s.config.EmailRecipients, notification)
			}
		case "webhook":
			if s.config.NotifyWebhook && len(s.config.WebhookURLs) > 0 {
				for _, url := range s.config.WebhookURLs {
					s.notifier.SendWebhook(url, notification)
				}
			}
		case "push":
			if s.config.NotifyPush {
				s.notifier.SendPush(notification)
			}
		case "sms":
			if s.config.NotifySMS {
				// 简化实现
			}
		}
	}
}

// escalationLoop 升级检查循环
func (s *MultiLevelAlertSystem) escalationLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkEscalations()
		}
	}
}

// checkEscalations 检查需要升级的告警
func (s *MultiLevelAlertSystem) checkEscalations() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for _, alert := range s.alerts {
		if alert.Status != AlertStatusActive {
			continue
		}

		levelConfig := s.getLevelConfig(alert.CurrentLevel)
		if levelConfig == nil || !levelConfig.AutoEscalate {
			continue
		}

		// 检查是否需要升级
		var escalateAfter time.Duration
		if alert.EscalationLevel < len(s.config.EscalationIntervals) {
			escalateAfter = s.config.EscalationIntervals[alert.EscalationLevel]
		}

		if escalateAfter > 0 {
			var referenceTime time.Time
			if alert.LastEscalatedAt != nil {
				referenceTime = *alert.LastEscalatedAt
			} else {
				referenceTime = alert.TriggeredAt
			}

			if now.Sub(referenceTime) >= escalateAfter {
				// 执行升级
				nextLevel := s.getNextLevel(alert.CurrentLevel)
				if nextLevel != "" && alert.EscalationLevel < s.config.MaxEscalationLevel {
					s.escalateAlert(alert, nextLevel)
				}
			}
		}
	}
}

// getLevelConfig 获取级别配置
func (s *MultiLevelAlertSystem) getLevelConfig(level AlertSeverity) *MultiAlertLevelConfig {
	for _, lc := range s.config.Levels {
		if lc.Severity == level {
			return &lc
		}
	}
	return nil
}

// getLevelPriority 获取级别优先级
func (s *MultiLevelAlertSystem) getLevelPriority(level AlertSeverity) int {
	lc := s.getLevelConfig(level)
	if lc != nil {
		return lc.Priority
	}
	return 0
}

// getNextLevel 获取下一级别
func (s *MultiLevelAlertSystem) getNextLevel(current AlertSeverity) AlertSeverity {
	priorities := []AlertSeverity{
		AlertSeverityInfo,
		AlertSeverityWarning,
		AlertSeverityCritical,
		AlertSeverityEmergency,
	}

	currentPriority := s.getLevelPriority(current)
	for _, p := range priorities {
		if s.getLevelPriority(p) > currentPriority {
			return p
		}
	}
	return ""
}

// persistLoop 持久化循环
func (s *MultiLevelAlertSystem) persistLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.PersistInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.persist()
			return
		case <-ticker.C:
			s.persist()
		}
	}
}

// persist 持久化告警
func (s *MultiLevelAlertSystem) persist() error {
	if !s.config.PersistEnabled || s.config.PersistPath == "" {
		return nil
	}

	s.mu.RLock()
	alerts := make([]*ActiveAlert, 0, len(s.alerts))
	for _, alert := range s.alerts {
		alerts = append(alerts, alert)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(alerts, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.config.PersistPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(s.config.PersistPath, data, 0644)
}

// loadPersistedAlerts 加载持久化告警
func (s *MultiLevelAlertSystem) loadPersistedAlerts() error {
	if s.config.PersistPath == "" {
		return nil
	}

	data, err := os.ReadFile(s.config.PersistPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var alerts []*ActiveAlert
	if err := json.Unmarshal(data, &alerts); err != nil {
		return err
	}

	s.mu.Lock()
	for _, alert := range alerts {
		// 只恢复活动状态的告警
		if alert.Status == AlertStatusActive {
			s.alerts[alert.QuotaID] = alert
		}
	}
	s.mu.Unlock()

	return nil
}

// GetAlerts 获取所有告警
func (s *MultiLevelAlertSystem) GetAlerts(filter AlertFilter) []*ActiveAlert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*ActiveAlert, 0)
	for _, alert := range s.alerts {
		if s.matchesFilter(alert, filter) {
			result = append(result, alert)
		}
	}
	return result
}

// GetSummary 获取告警汇总
func (s *MultiLevelAlertSystem) GetSummary() *MultiAlertSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := &MultiAlertSummary{
		TotalAlerts: len(s.alerts),
		ByLevel:     make(map[string]int),
		ByQuota:     make(map[string]int),
		TopAlerts:   make([]ActiveAlert, 0),
	}

	for _, alert := range s.alerts {
		switch alert.Status {
		case AlertStatusActive:
			summary.ActiveAlerts++
		case AlertStatusSilenced:
			summary.SilencedAlerts++
		case AlertStatusResolved:
			summary.ResolvedAlerts++
		}

		summary.ByLevel[string(alert.CurrentLevel)]++
		summary.ByQuota[alert.QuotaName]++

		if alert.EscalationLevel > 0 {
			summary.RecentEscalations++
		}
	}

	return summary
}

// matchesFilter 匹配过滤条件
func (s *MultiLevelAlertSystem) matchesFilter(alert *ActiveAlert, filter AlertFilter) bool {
	if filter.QuotaID != "" && alert.QuotaID != filter.QuotaID {
		return false
	}
	if filter.Level != "" && alert.CurrentLevel != filter.Level {
		return false
	}
	if filter.Status != "" && alert.Status != filter.Status {
		return false
	}
	if filter.MinUsage > 0 && alert.UsagePercent < filter.MinUsage {
		return false
	}
	if filter.MaxUsage > 0 && alert.UsagePercent > filter.MaxUsage {
		return false
	}
	if filter.StartTime != nil && alert.TriggeredAt.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && alert.TriggeredAt.After(*filter.EndTime) {
		return false
	}
	return true
}

// Close 关闭系统
func (s *MultiLevelAlertSystem) Close() error {
	s.cancel()
	s.wg.Wait()

	// 最终持久化
	if s.config.PersistEnabled {
		s.persist()
	}

	return nil
}

// SetNotifier 设置通知器
func (s *MultiLevelAlertSystem) SetNotifier(notifier AlertNotifier) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.notifier = notifier
}

// SetStorage 设置存储
func (s *MultiLevelAlertSystem) SetStorage(storage AlertStorage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.storage = storage
}

// AddThreshold 添加阈值
func (s *MultiLevelAlertSystem) AddThreshold(threshold AlertThreshold) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.thresholds = append(s.thresholds, &threshold)
	// 按百分比排序
	s.sortThresholds()
}

// sortThresholds 排序阈值
func (s *MultiLevelAlertSystem) sortThresholds() {
	for i := 0; i < len(s.thresholds)-1; i++ {
		for j := i + 1; j < len(s.thresholds); j++ {
			if s.thresholds[i].Percentage > s.thresholds[j].Percentage {
				s.thresholds[i], s.thresholds[j] = s.thresholds[j], s.thresholds[i]
			}
		}
	}
}

// generateAlertID 生成告警ID
func generateAlertID(quotaID string) string {
	return fmt.Sprintf("alert-%s-%d", quotaID, time.Now().UnixNano())
}
