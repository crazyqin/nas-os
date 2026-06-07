package guidedalerts

import (
	"fmt"
	"regexp"
	"sync"
)

// AlertRouter 告警路由器
type AlertRouter struct {
	mu       sync.RWMutex
	channels map[string]*RouteChannel
	rules    []*RouteRule
}

// NewAlertRouter 创建告警路由器
func NewAlertRouter() *AlertRouter {
	return &AlertRouter{
		channels: make(map[string]*RouteChannel),
		rules:    make([]*RouteRule, 0),
	}
}

// AddChannel 添加路由通道
func (r *AlertRouter) AddChannel(channel *RouteChannel) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if channel.ID == "" {
		return fmt.Errorf("channel ID is required")
	}
	if _, exists := r.channels[channel.ID]; exists {
		return fmt.Errorf("channel %s already exists", channel.ID)
	}

	r.channels[channel.ID] = channel
	return nil
}

// RemoveChannel 移除路由通道
func (r *AlertRouter) RemoveChannel(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.channels[id]; !exists {
		return fmt.Errorf("channel %s not found", id)
	}

	delete(r.channels, id)
	return nil
}

// GetChannel 获取通道
func (r *AlertRouter) GetChannel(id string) (*RouteChannel, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.channels[id]
	return ch, ok
}

// ListChannels 列出所有通道
func (r *AlertRouter) ListChannels() []*RouteChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	channels := make([]*RouteChannel, 0, len(r.channels))
	for _, ch := range r.channels {
		channels = append(channels, ch)
	}
	return channels
}

// AddRouteRule 添加路由规则
func (r *AlertRouter) AddRouteRule(rule *RouteRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("route rule ID is required")
	}

	// 验证引用的通道是否存在
	for _, chID := range rule.Channels {
		if _, exists := r.channels[chID]; !exists {
			return fmt.Errorf("channel %s not found", chID)
		}
	}

	r.rules = append(r.rules, rule)
	return nil
}

// RemoveRouteRule 移除路由规则
func (r *AlertRouter) RemoveRouteRule(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, rule := range r.rules {
		if rule.ID == id {
			r.rules = append(r.rules[:i], r.rules[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("route rule %s not found", id)
}

// ListRouteRules 列出路由规则
func (r *AlertRouter) ListRouteRules() []*RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rules := make([]*RouteRule, len(r.rules))
	copy(rules, r.rules)
	return rules
}

// Route 路由告警到匹配的通道
func (r *AlertRouter) Route(alert *Alert) []*RouteChannel {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*RouteChannel

	for _, rule := range r.rules {
		if !rule.Enabled {
			continue
		}

		if r.matchAlert(alert, rule.Matchers) {
			// 收集匹配的通道
			for _, chID := range rule.Channels {
				if ch, ok := r.channels[chID]; ok && ch.Enabled {
					matched = append(matched, ch)
				}
			}

			// 如果规则不继续匹配，直接返回
			if !rule.Continue {
				break
			}
		}
	}

	return matched
}

// matchAlert 检查告警是否匹配路由规则的标签条件
func (r *AlertRouter) matchAlert(alert *Alert, matchers []LabelMatcher) bool {
	if len(matchers) == 0 {
		return true // 无匹配条件，默认匹配
	}

	for _, matcher := range matchers {
		matched := r.matchLabel(alert, matcher)
		if matcher.IsEqual && !matched {
			return false
		}
		if !matcher.IsEqual && matched {
			return false
		}
	}

	return true
}

// matchLabel 检查单个标签匹配
func (r *AlertRouter) matchLabel(alert *Alert, matcher LabelMatcher) bool {
	var value string

	// 根据标签名获取对应的值
	switch matcher.Name {
	case "severity":
		value = alert.Severity.String()
	case "category":
		value = string(alert.Category)
	case "source":
		value = alert.Source
	case "status":
		value = string(alert.Status)
	default:
		// 从 alert.Labels 中获取
		if alert.Labels != nil {
			value = alert.Labels[matcher.Name]
		}
	}

	if matcher.IsRegex {
		re, err := regexp.Compile(matcher.Value)
		if err != nil {
			return false
		}
		return re.MatchString(value)
	}

	return value == matcher.Value
}

// GetDefaultRouteRules 获取默认路由规则
func GetDefaultRouteRules() []*RouteRule {
	return []*RouteRule{
		{
			ID:   "critical-alerts",
			Name: "严重告警路由",
			Matchers: []LabelMatcher{
				{Name: "severity", Value: "critical", IsEqual: true},
			},
			Channels: []string{"webhook-default", "syslog"},
			Continue: false,
			Priority: 1,
			Enabled:  true,
		},
		{
			ID:   "storage-alerts",
			Name: "存储告警路由",
			Matchers: []LabelMatcher{
				{Name: "category", Value: "storage", IsEqual: true},
			},
			Channels: []string{"webhook-storage"},
			Continue: true,
			Priority: 2,
			Enabled:  true,
		},
		{
			ID:       "default-alerts",
			Name:     "默认告警路由",
			Matchers: []LabelMatcher{},
			Channels: []string{"syslog"},
			Continue: false,
			Priority: 100,
			Enabled:  true,
		},
	}
}

// GetDefaultChannels 获取默认通道配置
func GetDefaultChannels() []*RouteChannel {
	return []*RouteChannel{
		{
			ID:   "syslog",
			Name: "系统日志",
			Type: "syslog",
			Config: ChannelConfig{
				Endpoint: "/dev/log",
				Template: "{{.Severity}} [{{.Category}}] {{.Title}}: {{.Description}}",
			},
			Enabled: true,
		},
		{
			ID:   "webhook-default",
			Name: "默认 Webhook",
			Type: "webhook",
			Config: ChannelConfig{
				Endpoint: "/api/alerts/webhook",
				Headers:  map[string]string{"Content-Type": "application/json"},
				Timeout:  30,
			},
			Enabled: true,
		},
		{
			ID:   "webhook-storage",
			Name: "存储告警 Webhook",
			Type: "webhook",
			Config: ChannelConfig{
				Endpoint: "/api/alerts/storage/webhook",
				Headers:  map[string]string{"Content-Type": "application/json"},
				Timeout:  30,
			},
			Enabled: true,
		},
	}
}
