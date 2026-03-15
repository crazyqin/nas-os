// Package budget 提供预算警报功能
package budget

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrAlertRuleNotFound    = errors.New("警报规则不存在")
	ErrAlertAlreadyActive   = errors.New("警报已处于活跃状态")
	ErrAlertAlreadyResolved = errors.New("警报已解决")
	ErrInvalidNotifierType  = errors.New("无效的通知类型")
	ErrNotificationFailed   = errors.New("通知发送失败")
)

// ========== 预算警报配置 ==========

// AlertManagerConfig 警报管理器配置
type AlertManagerConfig struct {
	// 默认阈值配置
	DefaultThresholds []ThresholdConfig `json:"default_thresholds"`

	// 通知配置
	NotifyEmail    bool     `json:"notify_email"`
	NotifyWebhook  bool     `json:"notify_webhook"`
	WebhookURL     string   `json:"webhook_url,omitempty"`
	NotifyChannels []string `json:"notify_channels,omitempty"`

	// 冷却时间（分钟）
	CooldownMinutes int `json:"cooldown_minutes"`

	// 升级配置
	EscalationEnabled bool                   `json:"escalation_enabled"`
	EscalationRules   []EscalationRuleConfig `json:"escalation_rules"`

	// 警报保留天数
	AlertRetentionDays int `json:"alert_retention_days"`

	// 通知重试次数
	MaxRetryCount int `json:"max_retry_count"`
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	Percentage float64 `json:"percentage"` // 预算使用百分比
	Level      string  `json:"level"`      // info, warning, critical, emergency
	Message    string  `json:"message"`    // 自定义消息
}

// EscalationRuleConfig 升级规则配置
type EscalationRuleConfig struct {
	AfterMinutes int      `json:"after_minutes"` // 多少分钟后升级
	ToLevel      string   `json:"to_level"`      // 升级到的级别
	NotifyUsers  []string `json:"notify_users"`  // 通知用户
}

// DefaultAlertManagerConfig 默认警报管理器配置
func DefaultAlertManagerConfig() AlertManagerConfig {
	return AlertManagerConfig{
		DefaultThresholds: []ThresholdConfig{
			{Percentage: 50, Level: "info", Message: "预算已使用 50%"},
			{Percentage: 75, Level: "warning", Message: "预算已使用 75%，请注意"},
			{Percentage: 90, Level: "critical", Message: "预算已使用 90%，请及时处理"},
			{Percentage: 100, Level: "emergency", Message: "预算已耗尽，请立即处理"},
		},
		NotifyEmail:       true,
		NotifyWebhook:     false,
		CooldownMinutes:   60,
		EscalationEnabled: true,
		EscalationRules: []EscalationRuleConfig{
			{AfterMinutes: 30, ToLevel: "warning", NotifyUsers: []string{"manager"}},
			{AfterMinutes: 60, ToLevel: "critical", NotifyUsers: []string{"admin", "manager"}},
		},
		AlertRetentionDays: 90,
		MaxRetryCount:      3,
	}
}

// AlertRule 警报规则
type AlertRule struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	BudgetID     string            `json:"budget_id"` // 空表示全局规则
	Enabled      bool              `json:"enabled"`
	Thresholds   []ThresholdConfig `json:"thresholds"`
	NotifyConfig NotifyConfig      `json:"notify_config"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// NotifyConfig 通知配置
type NotifyConfig struct {
	NotifyEmail     bool     `json:"notify_email"`
	EmailRecipients []string `json:"email_recipients"`
	NotifyWebhook   bool     `json:"notify_webhook"`
	WebhookURL      string   `json:"webhook_url"`
	NotifyChannels  []string `json:"notify_channels"`
	NotifyUsers     []string `json:"notify_users"`

	// 自定义通知模板
	EmailSubjectTemplate   string `json:"email_subject_template,omitempty"`
	EmailBodyTemplate      string `json:"email_body_template,omitempty"`
	WebhookPayloadTemplate string `json:"webhook_payload_template,omitempty"`
}

// AlertHistory 警报历史
type AlertHistory struct {
	AlertID   string    `json:"alert_id"`
	BudgetID  string    `json:"budget_id"`
	Action    string    `json:"action"` // triggered, acknowledged, resolved, escalated
	ActionAt  time.Time `json:"action_at"`
	ActionBy  string    `json:"action_by,omitempty"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	OldLevel  string    `json:"old_level,omitempty"`
	NewLevel  string    `json:"new_level,omitempty"`
	Message   string    `json:"message,omitempty"`
}

// AlertStats 警报统计
type AlertStats struct {
	TotalAlerts           int                `json:"total_alerts"`
	ActiveAlerts          int                `json:"active_alerts"`
	AcknowledgedAlerts    int                `json:"acknowledged_alerts"`
	ResolvedAlerts        int                `json:"resolved_alerts"`
	ByLevel               map[AlertLevel]int `json:"by_level"`
	ByBudget              map[string]int     `json:"by_budget"`
	AverageResolutionTime float64            `json:"average_resolution_time_minutes"`
}

// ========== 警报通知接口 ==========

// AlertNotifier 警报通知接口
type AlertNotifier interface {
	// 发送通知
	Send(ctx context.Context, alert *BudgetAlert, config *NotifyConfig) error

	// 获取通知类型
	Type() string
}

// ========== 警报管理器 ==========

// AlertManager 警报管理器
type AlertManager struct {
	mu        sync.RWMutex
	config    AlertManagerConfig
	notifiers map[string]AlertNotifier

	// 警报存储
	alerts    map[string]*BudgetAlert
	alertHist []AlertHistory
	rules     map[string]*AlertRule

	// 预算数据提供者
	budgetProvider BudgetDataProvider

	// 冷却跟踪
	lastTriggerTime map[string]time.Time
}

// BudgetDataProvider 预算数据提供者接口
type BudgetDataProvider interface {
	// 获取预算
	GetBudget(ctx context.Context, budgetID string) (*BudgetInfo, error)

	// 获取所有预算
	GetAllBudgets(ctx context.Context) ([]*BudgetInfo, error)

	// 更新预算状态
	UpdateBudgetStatus(ctx context.Context, budgetID string, status string) error
}

// BudgetInfo 预算信息
type BudgetInfo struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Amount       float64   `json:"amount"`
	UsedAmount   float64   `json:"used_amount"`
	Remaining    float64   `json:"remaining"`
	UsagePercent float64   `json:"usage_percent"`
	Status       string    `json:"status"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
}

// NewAlertManager 创建警报管理器
func NewAlertManager(config AlertManagerConfig, budgetProvider BudgetDataProvider) *AlertManager {
	return &AlertManager{
		config:          config,
		notifiers:       make(map[string]AlertNotifier),
		alerts:          make(map[string]*BudgetAlert),
		alertHist:       make([]AlertHistory, 0),
		rules:           make(map[string]*AlertRule),
		budgetProvider:  budgetProvider,
		lastTriggerTime: make(map[string]time.Time),
	}
}

// RegisterNotifier 注册通知器
func (m *AlertManager) RegisterNotifier(notifier AlertNotifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers[notifier.Type()] = notifier
}

// ========== 警报检查 ==========

// CheckBudgetAlerts 检查预算警报
func (m *AlertManager) CheckBudgetAlerts(ctx context.Context) ([]*BudgetAlert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取所有预算
	budgets, err := m.budgetProvider.GetAllBudgets(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取预算数据失败: %w", err)
	}

	var triggeredAlerts []*BudgetAlert

	for _, budget := range budgets {
		alerts := m.checkBudget(budget)
		triggeredAlerts = append(triggeredAlerts, alerts...)
	}

	return triggeredAlerts, nil
}

// CheckBudgetAlert 检查单个预算警报
func (m *AlertManager) CheckBudgetAlert(ctx context.Context, budgetID string) ([]*BudgetAlert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	budget, err := m.budgetProvider.GetBudget(ctx, budgetID)
	if err != nil {
		return nil, fmt.Errorf("获取预算数据失败: %w", err)
	}

	return m.checkBudget(budget), nil
}

// checkBudget 检查单个预算
func (m *AlertManager) checkBudget(budget *BudgetInfo) []*BudgetAlert {
	var alerts []*BudgetAlert

	// 获取适用的阈值
	thresholds := m.getThresholds(budget.ID)

	// 检查是否在冷却期
	if m.isInCooldown(budget.ID) {
		return alerts
	}

	for _, threshold := range thresholds {
		// 检查是否触发阈值
		if budget.UsagePercent >= threshold.Percentage {
			// 检查是否已有相同级别的活跃警报
			if m.hasActiveAlert(budget.ID, stringToAlertLevel(threshold.Level)) {
				continue
			}

			// 创建警报
			alert := m.createAlert(budget, threshold)
			alerts = append(alerts, alert)

			// 存储警报
			m.alerts[alert.ID] = alert

			// 记录历史
			m.recordHistory(alert, "triggered", "", "")

			// 更新冷却时间
			m.lastTriggerTime[budget.ID] = time.Now()
		}
	}

	return alerts
}

// getThresholds 获取阈值配置
func (m *AlertManager) getThresholds(budgetID string) []ThresholdConfig {
	// 先检查特定规则
	if rule, ok := m.rules[budgetID]; ok && rule.Enabled {
		return rule.Thresholds
	}

	// 检查全局规则
	if rule, ok := m.rules[""]; ok && rule.Enabled {
		return rule.Thresholds
	}

	// 使用默认阈值
	return m.config.DefaultThresholds
}

// isInCooldown 检查是否在冷却期
func (m *AlertManager) isInCooldown(budgetID string) bool {
	lastTrigger, ok := m.lastTriggerTime[budgetID]
	if !ok {
		return false
	}

	cooldown := time.Duration(m.config.CooldownMinutes) * time.Minute
	return time.Since(lastTrigger) < cooldown
}

// hasActiveAlert 检查是否有活跃警报
func (m *AlertManager) hasActiveAlert(budgetID string, level AlertLevel) bool {
	for _, alert := range m.alerts {
		if alert.BudgetID == budgetID &&
			alert.Level == level &&
			(alert.Status == AlertStatusActive || alert.Status == AlertStatusAcknowledged) {
			return true
		}
	}
	return false
}

// createAlert 创建警报
func (m *AlertManager) createAlert(budget *BudgetInfo, threshold ThresholdConfig) *BudgetAlert {
	now := time.Now()
	level := stringToAlertLevel(threshold.Level)

	message := threshold.Message
	if message == "" {
		message = fmt.Sprintf("预算 %s 已使用 %.1f%%", budget.Name, budget.UsagePercent)
	}

	return &BudgetAlert{
		ID:              generateAlertID(),
		BudgetID:        budget.ID,
		BudgetName:      budget.Name,
		TriggeredAt:     now,
		Level:           level,
		Threshold:       threshold.Percentage,
		CurrentPercent:  budget.UsagePercent,
		CurrentSpend:    budget.UsedAmount,
		BudgetAmount:    budget.Amount,
		RemainingAmount: budget.Remaining,
		Status:          AlertStatusActive,
		Message:         message,
		NotifySent:      false,
		NotifyChannels:  make([]string, 0),
		EscalationLevel: 0,
		Metadata:        make(map[string]interface{}),
	}
}

// ========== 警报操作 ==========

// AcknowledgeAlert 确认警报
func (m *AlertManager) AcknowledgeAlert(alertID, acknowledgedBy string) (*BudgetAlert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return nil, ErrAlertRuleNotFound
	}

	if alert.Status == AlertStatusResolved {
		return nil, ErrAlertAlreadyResolved
	}

	now := time.Now()
	oldStatus := string(alert.Status)

	alert.Status = AlertStatusAcknowledged
	alert.AcknowledgedAt = &now
	alert.AcknowledgedBy = acknowledgedBy

	m.recordHistory(alert, "acknowledged", oldStatus, string(AlertStatusAcknowledged))

	return alert, nil
}

// ResolveAlert 解决警报
func (m *AlertManager) ResolveAlert(alertID, resolvedBy string) (*BudgetAlert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return nil, ErrAlertRuleNotFound
	}

	if alert.Status == AlertStatusResolved {
		return nil, ErrAlertAlreadyResolved
	}

	now := time.Now()
	oldStatus := string(alert.Status)

	alert.Status = AlertStatusResolved
	alert.ResolvedAt = &now

	m.recordHistory(alert, "resolved", oldStatus, string(AlertStatusResolved))

	return alert, nil
}

// SuppressAlert 抑制警报
func (m *AlertManager) SuppressAlert(alertID, reason string) (*BudgetAlert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return nil, ErrAlertRuleNotFound
	}

	oldStatus := string(alert.Status)

	alert.Status = AlertStatusSuppressed
	alert.CustomMessage = reason

	m.recordHistory(alert, "suppressed", oldStatus, string(AlertStatusSuppressed))

	return alert, nil
}

// ========== 通知发送 ==========

// SendAlertNotifications 发送警报通知
func (m *AlertManager) SendAlertNotifications(ctx context.Context, alert *BudgetAlert) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取通知配置
	notifyConfig := m.getNotifyConfig(alert.BudgetID)

	var lastErr error
	var successChannels []string

	// 发送邮件通知
	if notifyConfig.NotifyEmail && m.config.NotifyEmail {
		if notifier, ok := m.notifiers["email"]; ok {
			if err := notifier.Send(ctx, alert, notifyConfig); err != nil {
				lastErr = err
			} else {
				successChannels = append(successChannels, "email")
			}
		}
	}

	// 发送Webhook通知
	if notifyConfig.NotifyWebhook && m.config.NotifyWebhook {
		if notifier, ok := m.notifiers["webhook"]; ok {
			if err := notifier.Send(ctx, alert, notifyConfig); err != nil {
				lastErr = err
			} else {
				successChannels = append(successChannels, "webhook")
			}
		}
	}

	// 发送到通知频道
	for _, channel := range notifyConfig.NotifyChannels {
		if notifier, ok := m.notifiers[channel]; ok {
			if err := notifier.Send(ctx, alert, notifyConfig); err != nil {
				lastErr = err
			} else {
				successChannels = append(successChannels, channel)
			}
		}
	}

	// 更新通知状态
	now := time.Now()
	alert.NotifySent = len(successChannels) > 0
	alert.NotifySentAt = &now
	alert.NotifyChannels = successChannels
	if lastErr != nil {
		alert.NotifyError = lastErr.Error()
	}

	return lastErr
}

// getNotifyConfig 获取通知配置
func (m *AlertManager) getNotifyConfig(budgetID string) *NotifyConfig {
	// 检查特定规则
	if rule, ok := m.rules[budgetID]; ok && rule.Enabled {
		return &rule.NotifyConfig
	}

	// 检查全局规则
	if rule, ok := m.rules[""]; ok && rule.Enabled {
		return &rule.NotifyConfig
	}

	// 返回默认配置
	return &NotifyConfig{
		NotifyEmail:    m.config.NotifyEmail,
		NotifyWebhook:  m.config.NotifyWebhook,
		WebhookURL:     m.config.WebhookURL,
		NotifyChannels: m.config.NotifyChannels,
	}
}

// ========== 警报升级 ==========

// CheckEscalations 检查警报升级
func (m *AlertManager) CheckEscalations(ctx context.Context) ([]*BudgetAlert, error) {
	if !m.config.EscalationEnabled {
		return nil, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var escalatedAlerts []*BudgetAlert
	now := time.Now()

	for _, alert := range m.alerts {
		if alert.Status != AlertStatusActive {
			continue
		}

		// 检查是否需要升级
		for _, rule := range m.config.EscalationRules {
			escalationTime := alert.TriggeredAt.Add(time.Duration(rule.AfterMinutes) * time.Minute)
			if now.After(escalationTime) && alert.EscalationLevel < getEscalationLevel(rule.ToLevel) {

				// 升级警报
				oldLevel := string(alert.Level)
				alert.Level = stringToAlertLevel(rule.ToLevel)
				alert.EscalationLevel = getEscalationLevel(rule.ToLevel)
				alert.LastEscalatedAt = &now

				// 记录历史
				m.recordHistory(alert, "escalated", oldLevel, string(alert.Level))

				// 发送升级通知
				if len(rule.NotifyUsers) > 0 {
					notifyConfig := &NotifyConfig{
						NotifyUsers: rule.NotifyUsers,
					}
					for _, notifier := range m.notifiers {
						notifier.Send(ctx, alert, notifyConfig)
					}
				}

				escalatedAlerts = append(escalatedAlerts, alert)
				break
			}
		}
	}

	return escalatedAlerts, nil
}

// ========== 警报查询 ==========

// GetAlert 获取警报
func (m *AlertManager) GetAlert(alertID string) (*BudgetAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.alerts[alertID]
	if !ok {
		return nil, ErrAlertRuleNotFound
	}

	return alert, nil
}

// ListAlerts 列出警报
func (m *AlertManager) ListAlerts(query AlertQuery) ([]*BudgetAlert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*BudgetAlert

	for _, alert := range m.alerts {
		// 应用过滤条件
		if len(query.BudgetIDs) > 0 {
			found := false
			for _, id := range query.BudgetIDs {
				if alert.BudgetID == id {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if len(query.Levels) > 0 {
			found := false
			for _, level := range query.Levels {
				if alert.Level == level {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if len(query.Statuses) > 0 {
			found := false
			for _, status := range query.Statuses {
				if alert.Status == status {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if query.StartTime != nil && alert.TriggeredAt.Before(*query.StartTime) {
			continue
		}

		if query.EndTime != nil && alert.TriggeredAt.After(*query.EndTime) {
			continue
		}

		result = append(result, alert)
	}

	// 分页
	if query.PageSize > 0 {
		start := query.Page * query.PageSize
		if start >= len(result) {
			return []*BudgetAlert{}, nil
		}
		end := start + query.PageSize
		if end > len(result) {
			end = len(result)
		}
		result = result[start:end]
	}

	return result, nil
}

// GetActiveAlerts 获取活跃警报
func (m *AlertManager) GetActiveAlerts() []*BudgetAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*BudgetAlert
	for _, alert := range m.alerts {
		if alert.Status == AlertStatusActive || alert.Status == AlertStatusAcknowledged {
			result = append(result, alert)
		}
	}
	return result
}

// GetAlertStats 获取警报统计
func (m *AlertManager) GetAlertStats() *AlertStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &AlertStats{
		ByLevel:  make(map[AlertLevel]int),
		ByBudget: make(map[string]int),
	}

	var totalResolutionTime float64
	var resolvedCount int

	for _, alert := range m.alerts {
		stats.TotalAlerts++

		switch alert.Status {
		case AlertStatusActive:
			stats.ActiveAlerts++
		case AlertStatusAcknowledged:
			stats.AcknowledgedAlerts++
		case AlertStatusResolved:
			stats.ResolvedAlerts++
			if alert.ResolvedAt != nil {
				resolutionTime := alert.ResolvedAt.Sub(alert.TriggeredAt).Minutes()
				totalResolutionTime += resolutionTime
				resolvedCount++
			}
		}

		stats.ByLevel[alert.Level]++
		stats.ByBudget[alert.BudgetID]++
	}

	if resolvedCount > 0 {
		stats.AverageResolutionTime = totalResolutionTime / float64(resolvedCount)
	}

	return stats
}

// GetAlertHistory 获取警报历史
func (m *AlertManager) GetAlertHistory(budgetID string, limit int) []AlertHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []AlertHistory
	for i := len(m.alertHist) - 1; i >= 0 && len(result) < limit; i-- {
		h := m.alertHist[i]
		if budgetID == "" || h.BudgetID == budgetID {
			result = append(result, h)
		}
	}
	return result
}

// ========== 规则管理 ==========

// CreateAlertRule 创建警报规则
func (m *AlertManager) CreateAlertRule(rule *AlertRule) (*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule

	return rule, nil
}

// UpdateAlertRule 更新警报规则
func (m *AlertManager) UpdateAlertRule(ruleID string, rule *AlertRule) (*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.rules[ruleID]
	if !ok {
		return nil, ErrAlertRuleNotFound
	}

	rule.ID = ruleID
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()

	m.rules[ruleID] = rule

	return rule, nil
}

// DeleteAlertRule 删除警报规则
func (m *AlertManager) DeleteAlertRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[ruleID]; !ok {
		return ErrAlertRuleNotFound
	}

	delete(m.rules, ruleID)
	return nil
}

// GetAlertRule 获取警报规则
func (m *AlertManager) GetAlertRule(ruleID string) (*AlertRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[ruleID]
	if !ok {
		return nil, ErrAlertRuleNotFound
	}

	return rule, nil
}

// ListAlertRules 列出警报规则
func (m *AlertManager) ListAlertRules(budgetID string) []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*AlertRule
	for _, rule := range m.rules {
		if budgetID == "" || rule.BudgetID == budgetID || rule.BudgetID == "" {
			result = append(result, rule)
		}
	}
	return result
}

// ========== 辅助方法 ==========

// recordHistory 记录历史
func (m *AlertManager) recordHistory(alert *BudgetAlert, action, oldStatus, newStatus string) {
	history := AlertHistory{
		AlertID:   alert.ID,
		BudgetID:  alert.BudgetID,
		Action:    action,
		ActionAt:  time.Now(),
		OldStatus: oldStatus,
		NewStatus: newStatus,
	}

	m.alertHist = append(m.alertHist, history)

	// 限制历史记录数量
	if len(m.alertHist) > 10000 {
		m.alertHist = m.alertHist[len(m.alertHist)-5000:]
	}
}

// stringToAlertLevel 字符串转警报级别
func stringToAlertLevel(s string) AlertLevel {
	switch s {
	case "info":
		return AlertLevelInfo
	case "warning":
		return AlertLevelWarning
	case "critical":
		return AlertLevelCritical
	case "emergency":
		return AlertLevelEmergency
	default:
		return AlertLevelInfo
	}
}

// getEscalationLevel 获取升级级别数值
func getEscalationLevel(level string) int {
	switch level {
	case "info":
		return 1
	case "warning":
		return 2
	case "critical":
		return 3
	case "emergency":
		return 4
	default:
		return 0
	}
}

// generateAlertID 生成警报ID
func generateAlertID() string {
	return fmt.Sprintf("alert-%d-%s", time.Now().UnixNano(), randomAlertString(6))
}

// generateRuleID 生成规则ID
func generateRuleID() string {
	return fmt.Sprintf("rule-%d-%s", time.Now().UnixNano(), randomAlertString(6))
}

// randomAlertString 生成随机字符串
func randomAlertString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().Nanosecond()%len(letters)]
	}
	return string(b)
}

// ========== 内置通知器实现 ==========

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	smtpHost     string
	smtpPort     int
	smtpUser     string
	smtpPassword string
	fromAddress  string
}

// NewEmailNotifier 创建邮件通知器
func NewEmailNotifier(smtpHost string, smtpPort int, user, password, from string) *EmailNotifier {
	return &EmailNotifier{
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUser:     user,
		smtpPassword: password,
		fromAddress:  from,
	}
}

// Send 发送邮件
func (n *EmailNotifier) Send(ctx context.Context, alert *BudgetAlert, config *NotifyConfig) error {
	// 邮件发送逻辑（需要实现实际的SMTP发送）
	// 这里是简化实现
	subject := fmt.Sprintf("[预算警报] %s - %s", alert.Level, alert.BudgetName)
	body := fmt.Sprintf(`
预算警报通知

预算名称: %s
警报级别: %s
触发时间: %s

当前支出: %.2f 元
预算金额: %.2f 元
使用比例: %.1f%%
剩余金额: %.2f 元

消息: %s

请及时处理。
`, alert.BudgetName, alert.Level, alert.TriggeredAt.Format(time.RFC3339),
		alert.CurrentSpend, alert.BudgetAmount, alert.CurrentPercent,
		alert.RemainingAmount, alert.Message)

	_ = subject
	_ = body
	// 实际发送邮件的逻辑...

	return nil
}

// Type 获取类型
func (n *EmailNotifier) Type() string {
	return "email"
}

// WebhookNotifier Webhook通知器
type WebhookNotifier struct {
	defaultURL string
}

// NewWebhookNotifier 创建Webhook通知器
func NewWebhookNotifier(defaultURL string) *WebhookNotifier {
	return &WebhookNotifier{defaultURL: defaultURL}
}

// Send 发送Webhook
func (n *WebhookNotifier) Send(ctx context.Context, alert *BudgetAlert, config *NotifyConfig) error {
	url := config.WebhookURL
	if url == "" {
		url = n.defaultURL
	}

	// Webhook发送逻辑（需要实现实际的HTTP请求）
	_ = url
	// 实际发送webhook的逻辑...

	return nil
}

// Type 获取类型
func (n *WebhookNotifier) Type() string {
	return "webhook"
}
