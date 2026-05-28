// Package smartnotify 提供智能通知核心管理逻辑
package smartnotify

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 智能通知管理器
type Manager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	config        *SmartNotifyConfig
	notifications map[string]*Notification
	rules         map[string]*NotifyRule
	templates     map[string]*NotifyTemplate
	history       []*NotifyHistory
	dedupCache    map[string]time.Time
	stopChan      chan struct{}
	running       bool
}

// NewManager 创建智能通知管理器
func NewManager(logger *zap.Logger, config *SmartNotifyConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultSmartNotifyConfig()
	}

	m := &Manager{
		logger:        logger,
		config:        config,
		notifications: make(map[string]*Notification),
		rules:         make(map[string]*NotifyRule),
		templates:     make(map[string]*NotifyTemplate),
		history:       make([]*NotifyHistory, 0),
		dedupCache:    make(map[string]time.Time),
		stopChan:      make(chan struct{}),
	}

	// 初始化默认规则
	m.initDefaultRules()
	// 初始化默认模板
	m.initDefaultTemplates()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// initDefaultRules 初始化默认规则
func (m *Manager) initDefaultRules() {
	defaultRules := []*NotifyRule{
		{
			ID:          "rule-system-alert",
			Name:        "系统告警规则",
			Description: "系统级别的告警通知",
			Enabled:     true,
			Priority:    PriorityUrgent,
			Conditions: []RuleCondition{
				{Field: "source", Operator: OpEquals, Value: "system"},
				{Field: "priority", Operator: OpEquals, Value: "urgent"},
			},
			Channels: []NotifyChannel{ChannelEmail, ChannelSMS, ChannelPush},
			Escalation: EscalationConfig{
				Enabled:  true,
				Timeout:  10 * time.Minute,
				MaxLevel: 3,
				Channels: []NotifyChannel{ChannelDingTalk, ChannelWeChat},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "rule-security",
			Name:        "安全事件规则",
			Description: "安全相关事件通知",
			Enabled:     true,
			Priority:    PriorityImportant,
			Conditions: []RuleCondition{
				{Field: "tags.category", Operator: OpEquals, Value: "security"},
			},
			Channels: []NotifyChannel{ChannelEmail, ChannelPush},
			Silence: SilenceConfig{
				Enabled:   true,
				StartTime: "23:00",
				EndTime:   "07:00",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "rule-disk-space",
			Name:        "磁盘空间告警",
			Description: "磁盘空间不足通知",
			Enabled:     true,
			Priority:    PriorityImportant,
			Conditions: []RuleCondition{
				{Field: "tags.metric", Operator: OpEquals, Value: "disk_space"},
				{Field: "tags.value", Operator: OpLessThan, Value: "10"},
			},
			Channels: []NotifyChannel{ChannelEmail, ChannelWeChat},
			Aggregate: AggregateConfig{
				Type:   AggregateTime,
				Window: 30 * time.Minute,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:          "rule-service-down",
			Name:        "服务宕机规则",
			Description: "服务不可用通知",
			Enabled:     true,
			Priority:    PriorityUrgent,
			Conditions: []RuleCondition{
				{Field: "tags.status", Operator: OpEquals, Value: "down"},
			},
			Channels: []NotifyChannel{ChannelSMS, ChannelDingTalk, ChannelPush},
			Escalation: EscalationConfig{
				Enabled:  true,
				Timeout:  5 * time.Minute,
				MaxLevel: 5,
				Channels: []NotifyChannel{ChannelEmail, ChannelWeChat},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, r := range defaultRules {
		m.rules[r.ID] = r
	}
}

// initDefaultTemplates 初始化默认模板
func (m *Manager) initDefaultTemplates() {
	defaultTemplates := []*NotifyTemplate{
		{
			ID:        "tpl-alert-email",
			Name:      "告警邮件模板",
			Channel:   ChannelEmail,
			Title:     "[{{priority}}] {{title}}",
			Content:   "系统告警\n\n标题: {{title}}\n优先级: {{priority}}\n来源: {{source}}\n时间: {{time}}\n\n详情:\n{{content}}",
			Variables: []string{"priority", "title", "source", "time", "content"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "tpl-alert-sms",
			Name:      "告警短信模板",
			Channel:   ChannelSMS,
			Title:     "系统告警",
			Content:   "[{{priority}}] {{title}} - {{content}}",
			Variables: []string{"priority", "title", "content"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "tpl-alert-push",
			Name:      "推送通知模板",
			Channel:   ChannelPush,
			Title:     "{{title}}",
			Content:   "{{content}}",
			Variables: []string{"title", "content"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:        "tpl-webhook",
			Name:      "Webhook 通知模板",
			Channel:   ChannelWebhook,
			Title:     "notification",
			Content:   `{"title":"{{title}}","content":"{{content}}","priority":"{{priority}}","source":"{{source}}","time":"{{time}}"}`,
			Variables: []string{"title", "content", "priority", "source", "time"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	for _, t := range defaultTemplates {
		m.templates[t.ID] = t
	}
}

// SendNotification 发送通知
func (m *Manager) SendNotification(notify *Notification) error {
	if !m.config.Enabled {
		return fmt.Errorf("notification system is disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 设置默认值
	if notify.ID == "" {
		notify.ID = generateID()
	}
	if notify.Priority == 0 {
		notify.Priority = PriorityNormal
	}
	if len(notify.Channels) == 0 {
		notify.Channels = m.config.DefaultChannels
	}
	notify.Status = StatusPending
	notify.CreatedAt = time.Now()
	notify.UpdatedAt = time.Now()

	// 去重检查
	if m.config.Deduplication && m.isDuplicate(notify) {
		notify.Status = StatusSilenced
		m.logger.Info("notification deduplicated",
			zap.String("id", notify.ID),
			zap.String("title", notify.Title))
		m.notifications[notify.ID] = notify
		return nil
	}

	// 匹配规则
	rule := m.matchRule(notify)

	// 检查免打扰
	if rule != nil && m.isInSilencePeriod(rule) {
		if notify.Priority < PriorityUrgent {
			notify.Status = StatusSilenced
			m.logger.Info("notification silenced by rule",
				zap.String("id", notify.ID),
				zap.String("rule", rule.Name))
			m.notifications[notify.ID] = notify
			m.addHistory(notify, rule.ID, "", StatusSilenced)
			return nil
		}
	}

	// 更新去重缓存
	if m.config.Deduplication {
		m.updateDedupCache(notify)
	}

	// 模拟发送到各渠道
	notify.Status = StatusSending
	notify.UpdatedAt = time.Now()

	ruleID := ""
	if rule != nil {
		ruleID = rule.ID
	}

	for _, ch := range notify.Channels {
		// 模拟发送成功
		m.addHistory(notify, ruleID, ch, StatusSent)
	}

	notify.Status = StatusSent
	now := time.Now()
	notify.SentAt = &now
	notify.UpdatedAt = now

	m.notifications[notify.ID] = notify

	m.logger.Info("notification sent",
		zap.String("id", notify.ID),
		zap.Strings("channels", channelsToStrings(notify.Channels)))

	return nil
}

// isDuplicate 检查是否重复通知
func (m *Manager) isDuplicate(notify *Notification) bool {
	key := m.dedupKey(notify)
	if lastTime, ok := m.dedupCache[key]; ok {
		if time.Since(lastTime) < m.config.DedupWindow {
			return true
		}
	}
	return false
}

// dedupKey 生成去重 key
func (m *Manager) dedupKey(notify *Notification) string {
	return fmt.Sprintf("%s:%s:%s", notify.Title, notify.Content, notify.Source)
}

// updateDedupCache 更新去重缓存
func (m *Manager) updateDedupCache(notify *Notification) {
	key := m.dedupKey(notify)
	m.dedupCache[key] = time.Now()

	// 清理过期缓存
	for k, t := range m.dedupCache {
		if time.Since(t) > m.config.DedupWindow {
			delete(m.dedupCache, k)
		}
	}
}

// matchRule 匹配通知规则
func (m *Manager) matchRule(notify *Notification) *NotifyRule {
	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}
		if m.matchConditions(notify, rule.Conditions) {
			return rule
		}
	}
	return nil
}

// matchConditions 匹配规则条件
func (m *Manager) matchConditions(notify *Notification, conditions []RuleCondition) bool {
	for _, cond := range conditions {
		if !m.matchCondition(notify, cond) {
			return false
		}
	}
	return true
}

// matchCondition 匹配单个条件
func (m *Manager) matchCondition(notify *Notification, cond RuleCondition) bool {
	actualValue := m.getFieldValue(notify, cond.Field)

	switch cond.Operator {
	case OpEquals:
		return actualValue == cond.Value
	case OpNotEquals:
		return actualValue != cond.Value
	case OpContains:
		return strings.Contains(actualValue, cond.Value)
	case OpGreaterThan:
		return compareNumeric(actualValue, cond.Value) > 0
	case OpLessThan:
		return compareNumeric(actualValue, cond.Value) < 0
	case OpRegex:
		matched, _ := regexp.MatchString(cond.Value, actualValue)
		return matched
	default:
		return false
	}
}

// getFieldValue 获取通知字段值
func (m *Manager) getFieldValue(notify *Notification, field string) string {
	switch field {
	case "title":
		return notify.Title
	case "content":
		return notify.Content
	case "source":
		return notify.Source
	case "priority":
		return fmt.Sprintf("%d", notify.Priority)
	case "template_id":
		return notify.TemplateID
	default:
		// 支持 tags.xxx 格式
		if strings.HasPrefix(field, "tags.") {
			key := strings.TrimPrefix(field, "tags.")
			if val, ok := notify.Tags[key]; ok {
				return val
			}
		}
		return ""
	}
}

// isInSilencePeriod 检查是否在免打扰时段
func (m *Manager) isInSilencePeriod(rule *NotifyRule) bool {
	if !rule.Silence.Enabled {
		return false
	}

	now := time.Now()

	// 检查日期
	if len(rule.Silence.Days) > 0 {
		weekday := now.Weekday()
		found := false
		for _, d := range rule.Silence.Days {
			if d == weekday {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查时间
	if rule.Silence.StartTime != "" && rule.Silence.EndTime != "" {
		startTime, err1 := parseHHMM(rule.Silence.StartTime)
		endTime, err2 := parseHHMM(rule.Silence.EndTime)
		if err1 == nil && err2 == nil {
			currentMinutes := now.Hour()*60 + now.Minute()

			if startTime <= endTime {
				// 同一天内
				return currentMinutes >= startTime && currentMinutes < endTime
			}
			// 跨天
			return currentMinutes >= startTime || currentMinutes < endTime
		}
	}

	return false
}

// parseHHMM 解析 HH:MM 格式
func parseHHMM(s string) (int, error) {
	var h, m int
	_, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

// addHistory 添加历史记录
func (m *Manager) addHistory(notify *Notification, ruleID string, channel NotifyChannel, status NotifyStatus) {
	history := &NotifyHistory{
		ID:        generateID(),
		NotifyID:  notify.ID,
		RuleID:    ruleID,
		Title:     notify.Title,
		Content:   notify.Content,
		Priority:  notify.Priority,
		Channel:   channel,
		Status:    status,
		CreatedAt: time.Now(),
	}

	m.history = append(m.history, history)

	// 限制历史大小
	if len(m.history) > m.config.MaxHistory {
		m.history = m.history[len(m.history)-m.config.MaxHistory:]
	}
}

// GetNotification 获取通知详情
func (m *Manager) GetNotification(id string) (*Notification, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	notify, ok := m.notifications[id]
	if !ok {
		return nil, fmt.Errorf("notification not found: %s", id)
	}
	return notify, nil
}

// ListNotifications 列出通知
func (m *Manager) ListNotifications(limit int) []*Notification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	notifications := make([]*Notification, 0, len(m.notifications))
	for _, n := range m.notifications {
		notifications = append(notifications, n)
	}

	if limit > 0 && limit < len(notifications) {
		notifications = notifications[:limit]
	}

	return notifications
}

// CreateRule 创建通知规则
func (m *Manager) CreateRule(rule *NotifyRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID()
	}
	rule.Enabled = true
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	m.rules[rule.ID] = rule

	m.logger.Info("rule created",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name))

	return nil
}

// GetRule 获取规则
func (m *Manager) GetRule(id string) (*NotifyRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}
	return rule, nil
}

// ListRules 列出所有规则
func (m *Manager) ListRules() []*NotifyRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*NotifyRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// UpdateRule 更新规则
func (m *Manager) UpdateRule(id string, rule *NotifyRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.rules[id]
	if !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	m.rules[id] = rule

	m.logger.Info("rule updated",
		zap.String("id", id),
		zap.String("name", rule.Name))

	return nil
}

// DeleteRule 删除规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}
	delete(m.rules, id)

	m.logger.Info("rule deleted", zap.String("id", id))
	return nil
}

// ToggleRule 启用/禁用规则
func (m *Manager) ToggleRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	rule.Enabled = !rule.Enabled
	rule.UpdatedAt = time.Now()

	m.logger.Info("rule toggled",
		zap.String("id", id),
		zap.Bool("enabled", rule.Enabled))

	return nil
}

// CreateTemplate 创建通知模板
func (m *Manager) CreateTemplate(tpl *NotifyTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tpl.ID == "" {
		tpl.ID = generateID()
	}
	tpl.CreatedAt = time.Now()
	tpl.UpdatedAt = time.Now()

	m.templates[tpl.ID] = tpl

	m.logger.Info("template created",
		zap.String("id", tpl.ID),
		zap.String("name", tpl.Name))

	return nil
}

// GetTemplate 获取模板
func (m *Manager) GetTemplate(id string) (*NotifyTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpl, ok := m.templates[id]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return tpl, nil
}

// ListTemplates 列出所有模板
func (m *Manager) ListTemplates() []*NotifyTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*NotifyTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, t)
	}
	return templates
}

// UpdateTemplate 更新模板
func (m *Manager) UpdateTemplate(id string, tpl *NotifyTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.templates[id]
	if !ok {
		return fmt.Errorf("template not found: %s", id)
	}

	tpl.ID = id
	tpl.CreatedAt = existing.CreatedAt
	tpl.UpdatedAt = time.Now()
	m.templates[id] = tpl

	m.logger.Info("template updated",
		zap.String("id", id),
		zap.String("name", tpl.Name))

	return nil
}

// DeleteTemplate 删除模板
func (m *Manager) DeleteTemplate(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.templates[id]; !ok {
		return fmt.Errorf("template not found: %s", id)
	}
	delete(m.templates, id)

	m.logger.Info("template deleted", zap.String("id", id))
	return nil
}

// RenderTemplate 渲染模板
func (m *Manager) RenderTemplate(id string, variables map[string]string) (string, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpl, ok := m.templates[id]
	if !ok {
		return "", "", fmt.Errorf("template not found: %s", id)
	}

	title := tpl.Title
	content := tpl.Content

	for k, v := range variables {
		placeholder := fmt.Sprintf("{{%s}}", k)
		title = strings.ReplaceAll(title, placeholder, v)
		content = strings.ReplaceAll(content, placeholder, v)
	}

	return title, content, nil
}

// GetHistory 获取通知历史
func (m *Manager) GetHistory(limit int) []*NotifyHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	start := len(m.history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*NotifyHistory, limit)
	copy(result, m.history[start:])
	return result
}

// GetStats 获取通知统计
func (m *Manager) GetStats() *NotifyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &NotifyStats{
		ByChannel:  make(map[NotifyChannel]int),
		ByPriority: make(map[string]int),
	}

	for _, h := range m.history {
		switch h.Status {
		case StatusSent, StatusDelivered:
			stats.TotalSent++
		case StatusFailed:
			stats.TotalFailed++
		case StatusSilenced:
			stats.TotalSilenced++
		}

		stats.ByChannel[h.Channel]++
		stats.ByPriority[PriorityName(h.Priority)]++
	}

	return stats
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *SmartNotifyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *SmartNotifyConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// channelsToStrings 渠道列表转字符串
func channelsToStrings(channels []NotifyChannel) []string {
	result := make([]string, len(channels))
	for i, ch := range channels {
		result[i] = string(ch)
	}
	return result
}

// compareNumeric 比较数值
func compareNumeric(a, b string) int {
	var numA, numB float64
	fmt.Sscanf(a, "%f", &numA)
	fmt.Sscanf(b, "%f", &numB)

	if numA > numB {
		return 1
	}
	if numA < numB {
		return -1
	}
	return 0
}
