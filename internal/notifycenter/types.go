package notifycenter

import (
	"sync"
	"time"
)

// ChannelType 通知渠道类型.
type ChannelType string

const (
	ChannelEmail   ChannelType = "email"
	ChannelWebhook ChannelType = "webhook"
	ChannelWeChat  ChannelType = "wechat"
	ChannelDingTalk ChannelType = "dingtalk"
	ChannelTelegram ChannelType = "telegram"
	ChannelSMS     ChannelType = "sms"
	ChannelInApp   ChannelType = "in_app"
)

// Priority 通知优先级.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Notification 通知消息.
type Notification struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Priority  Priority  `json:"priority"`
	Channel   ChannelType `json:"channel"`
	Source    string    `json:"source"`
	Tags      []string  `json:"tags,omitempty"`
	Read      bool      `json:"read"`
	SentAt    time.Time `json:"sent_at"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
}

// Channel 通知渠道配置.
type Channel struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Type      ChannelType `json:"type"`
	Enabled   bool        `json:"enabled"`
	Config    map[string]string `json:"config"`
	// RateLimitPerMin 每分钟最大发送数.
	RateLimitPerMin int `json:"rate_limit_per_min"`
}

// Template 通知模板.
type Template struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Channel  ChannelType `json:"channel"`
	Subject  string      `json:"subject"`
	Body     string      `json:"body"`
	// Variables 模板变量列表.
	Variables []string `json:"variables,omitempty"`
}

// Preference 用户通知偏好.
type Preference struct {
	UserID           string          `json:"user_id"`
	EnabledChannels  []ChannelType   `json:"enabled_channels"`
	MinPriority      Priority        `json:"min_priority"`
	QuietHoursStart  string          `json:"quiet_hours_start"` // "22:00"
	QuietHoursEnd    string          `json:"quiet_hours_end"`   // "08:00"
	MutedSources     []string        `json:"muted_sources"`
}

// Manager 通知中心管理器.
type Manager struct {
	mu            sync.RWMutex
	notifications []Notification
	channels      map[string]*Channel
	templates     map[string]*Template
	preferences   map[string]*Preference
	maxNotifs     int
}

// NewManager 创建通知中心管理器.
func NewManager() *Manager {
	return &Manager{
		notifications: make([]Notification, 0, 1000),
		channels:      make(map[string]*Channel),
		templates:     make(map[string]*Template),
		preferences:   make(map[string]*Preference),
		maxNotifs:     50000,
	}
}

// Send 发送通知.
func (m *Manager) Send(notif *Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	notif.SentAt = time.Now()
	notif.Read = false
	m.notifications = append(m.notifications, *notif)

	// 清理旧通知
	if len(m.notifications) > m.maxNotifs {
		m.notifications = m.notifications[len(m.notifications)-m.maxNotifs:]
	}
	return nil
}

// List 获取通知列表.
func (m *Manager) List(opts ListOptions) []Notification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Notification, 0)
	for i := len(m.notifications) - 1; i >= 0; i-- {
		n := m.notifications[i]
		if opts.UnreadOnly && n.Read {
			continue
		}
		if opts.Channel != "" && n.Channel != opts.Channel {
			continue
		}
		if opts.Source != "" && n.Source != opts.Source {
			continue
		}
		result = append(result, n)
		if opts.Limit > 0 && len(result) >= opts.Limit {
			break
		}
	}
	return result
}

// ListOptions 通知列表选项.
type ListOptions struct {
	UnreadOnly bool
	Channel    ChannelType
	Source     string
	Limit      int
}

// MarkRead 标记已读.
func (m *Manager) MarkRead(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.notifications {
		if m.notifications[i].ID == id {
			now := time.Now()
			m.notifications[i].Read = true
			m.notifications[i].ReadAt = &now
			return true
		}
	}
	return false
}

// MarkAllRead 全部标记已读.
func (m *Manager) MarkAllRead() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	count := 0
	for i := range m.notifications {
		if !m.notifications[i].Read {
			m.notifications[i].Read = true
			m.notifications[i].ReadAt = &now
			count++
		}
	}
	return count
}

// UnreadCount 未读数量.
func (m *Manager) UnreadCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, n := range m.notifications {
		if !n.Read {
			count++
		}
	}
	return count
}

// DeleteNotification 删除通知.
func (m *Manager) DeleteNotification(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.notifications {
		if m.notifications[i].ID == id {
			m.notifications = append(m.notifications[:i], m.notifications[i+1:]...)
			return true
		}
	}
	return false
}

// AddChannel 添加渠道.
func (m *Manager) AddChannel(ch *Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[ch.ID] = ch
}

// GetChannel 获取渠道.
func (m *Manager) GetChannel(id string) (*Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch, ok := m.channels[id]
	return ch, ok
}

// ListChannels 列出渠道.
func (m *Manager) ListChannels() []*Channel {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chs := make([]*Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		chs = append(chs, ch)
	}
	return chs
}

// AddTemplate 添加模板.
func (m *Manager) AddTemplate(tmpl *Template) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.templates[tmpl.ID] = tmpl
}

// GetTemplate 获取模板.
func (m *Manager) GetTemplate(id string) (*Template, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.templates[id]
	return t, ok
}

// SetPreference 设置用户偏好.
func (m *Manager) SetPreference(pref *Preference) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.preferences[pref.UserID] = pref
}

// GetPreference 获取用户偏好.
func (m *Manager) GetPreference(userID string) (*Preference, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.preferences[userID]
	return p, ok
}

// GetStats 获取统计.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := len(m.notifications)
	unread := 0
	channelCounts := make(map[ChannelType]int)
	for _, n := range m.notifications {
		if !n.Read {
			unread++
		}
		channelCounts[n.Channel]++
	}
	return map[string]interface{}{
		"total":           total,
		"unread":          unread,
		"channels_count":  len(m.channels),
		"templates_count": len(m.templates),
		"by_channel":      channelCounts,
	}
}
