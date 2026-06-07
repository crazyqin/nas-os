package guidedalerts

import (
	"fmt"
	"sync"
	"time"
)

// AlertManager 告警管理器
type AlertManager struct {
	mu           sync.RWMutex
	alerts       map[string]*Alert
	history      []AlertHistory
	silenceRules map[string]*SilenceRule
	inhibRules   map[string]*InhibitionRule
	ruleEngine   *RuleEngine
	router       *AlertRouter
	listeners    []AlertListener

	// 升级检查
	escalationTicker *time.Ticker
	stopCh           chan struct{}
}

// AlertListener 告警监听器
type AlertListener interface {
	OnAlert(alert *Alert)
	OnResolve(alert *Alert)
	OnEscalate(alert *Alert, level int)
}

// NewAlertManager 创建告警管理器
func NewAlertManager() *AlertManager {
	am := &AlertManager{
		alerts:       make(map[string]*Alert),
		history:      make([]AlertHistory, 0),
		silenceRules: make(map[string]*SilenceRule),
		inhibRules:   make(map[string]*InhibitionRule),
		ruleEngine:   NewRuleEngine(),
		router:       NewAlertRouter(),
		stopCh:       make(chan struct{}),
	}

	// 加载内置规则
	for _, rule := range GetBuiltinRules() {
		am.ruleEngine.AddRule(rule)
	}

	// 加载默认通道和路由
	for _, ch := range GetDefaultChannels() {
		am.router.AddChannel(ch)
	}
	for _, rule := range GetDefaultRouteRules() {
		am.router.AddRouteRule(rule)
	}

	// 启动升级检查
	am.startEscalationChecker()

	return am
}

// CreateAlert 创建告警
func (am *AlertManager) CreateAlert(alert *Alert) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if alert.ID == "" {
		return fmt.Errorf("alert ID is required")
	}

	now := time.Now()

	// 检查是否已存在相同告警（聚合）
	if existing, ok := am.alerts[alert.ID]; ok {
		existing.Count++
		existing.LastSeen = now
		existing.Status = StatusActive
		am.addHistory(alert.ID, "updated", "", "alert count incremented")
		return nil
	}

	// 设置默认值
	if alert.FirstSeen.IsZero() {
		alert.FirstSeen = now
	}
	if alert.LastSeen.IsZero() {
		alert.LastSeen = now
	}
	alert.Status = StatusActive
	alert.Count = 1

	// 检查静默规则
	if am.isSilenced(alert) {
		alert.Status = StatusSilenced
	}

	// 检查抑制规则
	if am.isInhibited(alert) {
		alert.Status = StatusSilenced
	}

	am.alerts[alert.ID] = alert
	am.addHistory(alert.ID, "created", "", alert.Title)

	// 通知监听器
	if alert.Status != StatusSilenced {
		for _, listener := range am.listeners {
			go listener.OnAlert(alert)
		}
	}

	// 路由告警
	channels := am.router.Route(alert)
	for _, ch := range channels {
		am.addHistory(alert.ID, "routed", "", fmt.Sprintf("routed to %s", ch.Name))
	}

	return nil
}

// GetAlert 获取告警
func (am *AlertManager) GetAlert(id string) (*Alert, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	alert, ok := am.alerts[id]
	return alert, ok
}

// ListAlerts 列出告警
func (am *AlertManager) ListAlerts(opts AlertFilter) []*Alert {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*Alert
	for _, alert := range am.alerts {
		if opts.Severity >= 0 && alert.Severity != opts.Severity {
			continue
		}
		if opts.Category != "" && alert.Category != opts.Category {
			continue
		}
		if opts.Status != "" && alert.Status != opts.Status {
			continue
		}
		if opts.Source != "" && alert.Source != opts.Source {
			continue
		}
		if opts.UnresolvedOnly && alert.Status == StatusResolved {
			continue
		}
		result = append(result, alert)
	}
	return result
}

// AlertFilter 告警过滤条件
type AlertFilter struct {
	Severity       AlertSeverity
	Category       AlertCategory
	Status         AlertStatus
	Source         string
	UnresolvedOnly bool
}

// AcknowledgeAlert 确认告警
func (am *AlertManager) AcknowledgeAlert(id string, user string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, ok := am.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}

	alert.Status = StatusAcknowledged
	now := time.Now()
	alert.AckedAt = &now
	am.addHistory(id, "acknowledged", user, "")

	return nil
}

// ResolveAlert 解决告警
func (am *AlertManager) ResolveAlert(id string, user string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, ok := am.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}

	alert.Status = StatusResolved
	now := time.Now()
	alert.ResolvedAt = &now
	am.addHistory(id, "resolved", user, "")

	// 通知监听器
	for _, listener := range am.listeners {
		go listener.OnResolve(alert)
	}

	return nil
}

// SilenceAlert 静默告警
func (am *AlertManager) SilenceAlert(id string, duration time.Duration, user string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	alert, ok := am.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}

	alert.Status = StatusSilenced
	am.addHistory(id, "silenced", user, fmt.Sprintf("silenced for %v", duration))

	return nil
}

// GetAlertHistory 获取告警历史
func (am *AlertManager) GetAlertHistory(alertID string) []AlertHistory {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []AlertHistory
	for _, h := range am.history {
		if h.AlertID == alertID {
			result = append(result, h)
		}
	}
	return result
}

// GetStats 获取告警统计
func (am *AlertManager) GetStats() *AlertStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := &AlertStats{
		BySeverity: make(map[string]int),
		ByCategory: make(map[string]int),
		ByStatus:   make(map[string]int),
	}

	var totalResolveMin float64
	var resolveCount int

	for _, alert := range am.alerts {
		stats.Total++
		stats.BySeverity[alert.Severity.String()]++
		stats.ByCategory[string(alert.Category)]++
		stats.ByStatus[string(alert.Status)]++

		switch alert.Status {
		case StatusActive:
			stats.ActiveCount++
		case StatusSilenced:
			stats.SilencedCount++
		case StatusResolved:
			stats.ResolvedCount++
			if alert.ResolvedAt != nil {
				totalResolveMin += alert.ResolvedAt.Sub(alert.FirstSeen).Minutes()
				resolveCount++
			}
		}
	}

	if resolveCount > 0 {
		stats.AvgResolveMin = totalResolveMin / float64(resolveCount)
	}

	return stats
}

// SilenceRule 管理

// AddSilenceRule 添加静默规则
func (am *AlertManager) AddSilenceRule(rule *SilenceRule) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("silence rule ID is required")
	}

	am.silenceRules[rule.ID] = rule
	return nil
}

// RemoveSilenceRule 移除静默规则
func (am *AlertManager) RemoveSilenceRule(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.silenceRules[id]; !exists {
		return fmt.Errorf("silence rule %s not found", id)
	}

	delete(am.silenceRules, id)
	return nil
}

// ListSilenceRules 列出静默规则
func (am *AlertManager) ListSilenceRules() []*SilenceRule {
	am.mu.RLock()
	defer am.mu.RUnlock()

	rules := make([]*SilenceRule, 0, len(am.silenceRules))
	for _, rule := range am.silenceRules {
		rules = append(rules, rule)
	}
	return rules
}

// InhibitionRule 管理

// AddInhibitionRule 添加抑制规则
func (am *AlertManager) AddInhibitionRule(rule *InhibitionRule) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("inhibition rule ID is required")
	}

	am.inhibRules[rule.ID] = rule
	return nil
}

// RemoveInhibitionRule 移除抑制规则
func (am *AlertManager) RemoveInhibitionRule(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	if _, exists := am.inhibRules[id]; !exists {
		return fmt.Errorf("inhibition rule %s not found", id)
	}

	delete(am.inhibRules, id)
	return nil
}

// ListInhibitionRules 列出抑制规则
func (am *AlertManager) ListInhibitionRules() []*InhibitionRule {
	am.mu.RLock()
	defer am.mu.RUnlock()

	rules := make([]*InhibitionRule, 0, len(am.inhibRules))
	for _, rule := range am.inhibRules {
		rules = append(rules, rule)
	}
	return rules
}

// 内部方法

func (am *AlertManager) addHistory(alertID, action, user, details string) {
	am.history = append(am.history, AlertHistory{
		AlertID:   alertID,
		Action:    action,
		Timestamp: time.Now(),
		User:      user,
		Details:   details,
	})
}

func (am *AlertManager) isSilenced(alert *Alert) bool {
	now := time.Now()
	for _, rule := range am.silenceRules {
		if !rule.Enabled {
			continue
		}
		if now.Before(rule.StartsAt) || now.After(rule.EndsAt) {
			continue
		}
		if matchAlertMatchers(alert, rule.Matchers) {
			return true
		}
	}
	return false
}

func (am *AlertManager) isInhibited(alert *Alert) bool {
	for _, rule := range am.inhibRules {
		if !rule.Enabled {
			continue
		}
		// 检查是否有活跃的源告警匹配
		for _, srcAlert := range am.alerts {
			if srcAlert.ID == alert.ID || srcAlert.Status == StatusResolved {
				continue
			}
			if matchAlertMatchers(srcAlert, rule.SourceMatchers) {
				// 检查目标告警是否匹配
				if matchAlertMatchers(alert, rule.TargetMatchers) {
					// 检查 Equal 标签是否相等
					if allLabelsEqual(srcAlert, alert, rule.Equal) {
						return true
					}
				}
			}
		}
	}
	return false
}

func matchAlertMatchers(alert *Alert, matchers []LabelMatcher) bool {
	for _, m := range matchers {
		value := getAlertLabel(alert, m.Name)
		matched := false

		if m.IsRegex {
			// 简化：不做正则匹配
			matched = (value == m.Value)
		} else {
			matched = (value == m.Value)
		}

		if m.IsEqual && !matched {
			return false
		}
		if !m.IsEqual && matched {
			return false
		}
	}
	return true
}

func getAlertLabel(alert *Alert, name string) string {
	switch name {
	case "severity":
		return alert.Severity.String()
	case "category":
		return string(alert.Category)
	case "source":
		return alert.Source
	case "status":
		return string(alert.Status)
	default:
		if alert.Labels != nil {
			return alert.Labels[name]
		}
		return ""
	}
}

func allLabelsEqual(a, b *Alert, labels []string) bool {
	for _, label := range labels {
		if getAlertLabel(a, label) != getAlertLabel(b, label) {
			return false
		}
	}
	return true
}

// 升级检查

func (am *AlertManager) startEscalationChecker() {
	am.escalationTicker = time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-am.escalationTicker.C:
				am.checkEscalations()
			case <-am.stopCh:
				return
			}
		}
	}()
}

func (am *AlertManager) checkEscalations() {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	for _, alert := range am.alerts {
		if alert.Status != StatusActive || alert.Escalation == nil {
			continue
		}
		if !alert.Escalation.Enabled {
			continue
		}

		// 检查是否需要升级
		if alert.Escalation.NextEscalation != nil && now.After(*alert.Escalation.NextEscalation) {
			// 执行升级
			alert.Escalation.CurrentLevel++
			alert.Status = StatusEscalated

			// 计算下次升级时间
			nextTime := now.Add(alert.Escalation.Timeout)
			alert.Escalation.NextEscalation = &nextTime

			am.addHistory(alert.ID, "escalated", "", fmt.Sprintf("escalated to level %d", alert.Escalation.CurrentLevel))

			// 通知监听器
			for _, listener := range am.listeners {
				go listener.OnEscalate(alert, alert.Escalation.CurrentLevel)
			}
		}
	}
}

// AddListener 添加监听器
func (am *AlertManager) AddListener(listener AlertListener) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.listeners = append(am.listeners, listener)
}

// GetRuleEngine 获取规则引擎
func (am *AlertManager) GetRuleEngine() *RuleEngine {
	return am.ruleEngine
}

// GetRouter 获取路由器
func (am *AlertManager) GetRouter() *AlertRouter {
	return am.router
}

// Stop 停止管理器
func (am *AlertManager) Stop() {
	close(am.stopCh)
	if am.escalationTicker != nil {
		am.escalationTicker.Stop()
	}
}
