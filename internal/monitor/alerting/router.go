package alerting

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ChannelType 通知渠道类型
type ChannelType string

const (
	ChannelEmail    ChannelType = "email"
	ChannelWebhook  ChannelType = "webhook"
	ChannelTelegram ChannelType = "telegram"
	ChannelDingTalk ChannelType = "dingtalk"
	ChannelWeChat   ChannelType = "wechat"
	ChannelSlack    ChannelType = "slack"
	ChannelSMS      ChannelType = "sms"
)

// Channel 通知渠道
type Channel struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     ChannelType `json:"type"`
	Target   string      `json:"target"`   // 邮件地址/webhook URL/chat ID等
	Template string      `json:"template"` // 关联的模板ID
	Enabled  bool        `json:"enabled"`
	Config   map[string]string `json:"config,omitempty"` // 额外配置
}

// RouteRule 路由规则
type RouteRule struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Priority     int               `json:"priority"`     // 优先级，数字越小优先级越高
	Enabled      bool              `json:"enabled"`

	// 匹配条件
	Levels       []AlertLevel      `json:"levels"`       // 匹配的告警级别
	ServiceTypes []string          `json:"serviceTypes"` // 匹配的服务类型
	HostLabels   map[string]string `json:"hostLabels"`   // 匹配的主机标签
	NamePatterns []string          `json:"namePatterns"` // 告警名称匹配模式

	// 通知目标
	Channels     []string          `json:"channels"`     // 通知渠道ID列表
	Template     string            `json:"template"`     // 使用的模板ID（覆盖渠道默认模板）

	// 抑制配置
	SuppressionWindow time.Duration `json:"suppressionWindow"` // 抑制时间窗口
}

// suppressionKey 抑制记录的key
type suppressionKey struct {
	ruleID  string
	alertID string
}

// Router 告警路由器
type Router struct {
	mu          sync.RWMutex
	channels    map[string]*Channel
	rules       []*RouteRule
	suppression map[suppressionKey]time.Time // 上次发送时间
	engine      *TemplateEngine
}

// NewRouter 创建告警路由器
func NewRouter(engine *TemplateEngine) *Router {
	return &Router{
		channels:    make(map[string]*Channel),
		rules:       make([]*RouteRule, 0),
		suppression: make(map[suppressionKey]time.Time),
		engine:      engine,
	}
}

// AddChannel 添加通知渠道
func (r *Router) AddChannel(ch *Channel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if ch.ID == "" {
		return fmt.Errorf("渠道ID不能为空")
	}
	if ch.Type == "" {
		return fmt.Errorf("渠道类型不能为空")
	}
	if ch.Target == "" {
		return fmt.Errorf("渠道目标不能为空")
	}

	r.channels[ch.ID] = ch
	return nil
}

// RemoveChannel 删除通知渠道
func (r *Router) RemoveChannel(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.channels[id]; !exists {
		return fmt.Errorf("渠道不存在: %s", id)
	}

	delete(r.channels, id)
	return nil
}

// GetChannel 获取渠道
func (r *Router) GetChannel(id string) (*Channel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, ok := r.channels[id]
	return ch, ok
}

// ListChannels 列出所有渠道
func (r *Router) ListChannels() []*Channel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		result = append(result, ch)
	}
	return result
}

// AddRule 添加路由规则
func (r *Router) AddRule(rule *RouteRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("规则ID不能为空")
	}
	if len(rule.Channels) == 0 {
		return fmt.Errorf("至少需要一个通知渠道")
	}

	r.rules = append(r.rules, rule)
	r.sortRules()
	return nil
}

// UpdateRule 更新路由规则
func (r *Router) UpdateRule(rule *RouteRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, existing := range r.rules {
		if existing.ID == rule.ID {
			r.rules[i] = rule
			r.sortRules()
			return nil
		}
	}

	return fmt.Errorf("规则不存在: %s", rule.ID)
}

// RemoveRule 删除路由规则
func (r *Router) RemoveRule(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, rule := range r.rules {
		if rule.ID == id {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("规则不存在: %s", id)
}

// GetRules 获取所有规则（按优先级排序）
func (r *Router) GetRules() []*RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*RouteRule, len(r.rules))
	copy(result, r.rules)
	return result
}

// sortRules 按优先级排序规则
func (r *Router) sortRules() {
	// 简单插入排序，规则数量一般不多
	for i := 1; i < len(r.rules); i++ {
		for j := i; j > 0 && r.rules[j].Priority < r.rules[j-1].Priority; j-- {
			r.rules[j], r.rules[j-1] = r.rules[j-1], r.rules[j]
		}
	}
}

// Route 路由告警到对应渠道
func (r *Router) Route(ctx context.Context, alertLevel AlertLevel, alertName, serviceType string, hostLabels map[string]string, vars *AlertVars) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var sentChannels []string

	for _, rule := range r.rules {
		if !rule.Enabled {
			continue
		}

		if !r.matchRule(rule, alertLevel, alertName, serviceType, hostLabels) {
			continue
		}

		// 检查抑制
		if r.isSuppressed(rule.ID, alertName, rule.SuppressionWindow) {
			continue
		}

		// 记录发送时间（用于后续抑制）
		sKey := suppressionKey{ruleID: rule.ID, alertID: alertName}
		r.suppression[sKey] = time.Now()

		// 路由到对应渠道
		for _, channelID := range rule.Channels {
			ch, ok := r.channels[channelID]
			if !ok || !ch.Enabled {
				continue
			}

			// 选择模板：规则 > 渠道 > 默认
			templateID := r.selectTemplate(rule, ch)

			// 发送通知
			if err := r.sendToChannel(ctx, ch, templateID, vars); err != nil {
				// 记录错误但继续发送到其他渠道
				fmt.Printf("[Router] 发送到渠道 %s (%s) 失败: %v\n", ch.Name, ch.ID, err)
				continue
			}

			sentChannels = append(sentChannels, channelID)
		}
	}

	return sentChannels, nil
}

// matchRule 检查告警是否匹配规则
func (r *Router) matchRule(rule *RouteRule, level AlertLevel, name, serviceType string, labels map[string]string) bool {
	// 检查级别
	if len(rule.Levels) > 0 {
		levelMatch := false
		for _, l := range rule.Levels {
			if l == level {
				levelMatch = true
				break
			}
		}
		if !levelMatch {
			return false
		}
	}

	// 检查服务类型
	if len(rule.ServiceTypes) > 0 {
		typeMatch := false
		for _, st := range rule.ServiceTypes {
			if st == serviceType || st == "*" {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			return false
		}
	}

	// 检查主机标签
	if len(rule.HostLabels) > 0 {
		for k, v := range rule.HostLabels {
			if labelVal, ok := labels[k]; !ok || labelVal != v {
				return false
			}
		}
	}

	// 检查告警名称模式
	if len(rule.NamePatterns) > 0 {
		nameMatch := false
		for _, pattern := range rule.NamePatterns {
			if matchPattern(pattern, name) {
				nameMatch = true
				break
			}
		}
		if !nameMatch {
			return false
		}
	}

	return true
}

// matchPattern 简单的模式匹配
func matchPattern(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	if pattern == name {
		return true
	}
	// 支持 * 通配符
	if len(pattern) > 0 && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(name) >= len(suffix) && name[len(name)-len(suffix):] == suffix
	}
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(name) >= len(prefix) && name[:len(prefix)] == prefix
	}
	return pattern == name
}

// isSuppressed 检查是否在抑制窗口内
func (r *Router) isSuppressed(ruleID, alertID string, window time.Duration) bool {
	if window <= 0 {
		return false
	}

	key := suppressionKey{ruleID: ruleID, alertID: alertID}
	lastSent, exists := r.suppression[key]
	if !exists {
		return false
	}

	return time.Since(lastSent) < window
}

// selectTemplate 选择模板
func (r *Router) selectTemplate(rule *RouteRule, ch *Channel) string {
	// 规则级模板优先
	if rule.Template != "" {
		return rule.Template
	}
	// 渠道级模板
	if ch.Template != "" {
		return ch.Template
	}
	// 默认模板
	return defaultTemplateForChannel(ch.Type)
}

// defaultTemplateForChannel 根据渠道类型返回默认模板
func defaultTemplateForChannel(chType ChannelType) string {
	switch chType {
	case ChannelEmail:
		return "email_html_default"
	case ChannelWebhook:
		return "webhook_default"
	case ChannelTelegram:
		return "telegram_default"
	case ChannelDingTalk:
		return "dingtalk_default"
	case ChannelWeChat:
		return "wechat_default"
	case ChannelSlack:
		return "slack_default"
	default:
		return "webhook_default"
	}
}

// sendToChannel 发送到指定渠道
func (r *Router) sendToChannel(ctx context.Context, ch *Channel, templateID string, vars *AlertVars) error {
	switch ch.Type {
	case ChannelEmail:
		subject, body, err := r.engine.Render(templateID, vars)
		if err != nil {
			return err
		}
		tmpl, _ := r.engine.GetTemplate(templateID)
		isHTML := false
		if tmpl != nil {
			isHTML = tmpl.IsHTML
		}
		return SendEmail(ctx, ch.Target, subject, body, isHTML)

	case ChannelWebhook, ChannelSlack:
		payload, err := r.engine.RenderToJSON(templateID, vars)
		if err != nil {
			return err
		}
		return SendWebhook(ctx, ch.Target, payload)

	case ChannelTelegram, ChannelDingTalk, ChannelWeChat:
		_, body, err := r.engine.Render(templateID, vars)
		if err != nil {
			return err
		}
		// 这些渠道通过webhook方式发送
		payload := map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"content": body},
		}
		return SendWebhook(ctx, ch.Target, payload)

	default:
		return fmt.Errorf("不支持的渠道类型: %s", ch.Type)
	}
}

// ClearSuppression 清除抑制记录
func (r *Router) ClearSuppression() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.suppression = make(map[suppressionKey]time.Time)
}

// CleanExpiredSuppressions 清理过期的抑制记录
func (r *Router) CleanExpiredSuppressions() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	maxWindow := time.Duration(0)
	for _, rule := range r.rules {
		if rule.SuppressionWindow > maxWindow {
			maxWindow = rule.SuppressionWindow
		}
	}

	for key, lastSent := range r.suppression {
		if now.Sub(lastSent) > maxWindow {
			delete(r.suppression, key)
		}
	}
}

// GetSuppressionCount 获取当前抑制中的告警数量
func (r *Router) GetSuppressionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.suppression)
}
