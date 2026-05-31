// Package notificationcenter 实现NAS系统的统一通知中心模块。
// 它提供通知的发送、查询、已读管理、多渠道分发、模板系统、
// 频率限制和通知聚合等功能。
package notificationcenter

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// NotificationLevel 表示通知的级别。
type NotificationLevel string

const (
	LevelInfo     NotificationLevel = "info"
	LevelWarning  NotificationLevel = "warning"
	LevelError    NotificationLevel = "error"
	LevelCritical NotificationLevel = "critical"
)

// NotificationCategory 表示通知的分类。
type NotificationCategory string

const (
	CategorySystem  NotificationCategory = "system"
	CategoryStorage NotificationCategory = "storage"
	CategorySecurity NotificationCategory = "security"
	CategoryApp     NotificationCategory = "app"
	CategoryBackup  NotificationCategory = "backup"
	CategoryNetwork NotificationCategory = "network"
	CategoryUser    NotificationCategory = "user"
)

// NotificationChannel 表示通知渠道类型。
type NotificationChannel string

const (
	ChannelEmail    NotificationChannel = "email"
	ChannelWebhook  NotificationChannel = "webhook"
	ChannelTelegram NotificationChannel = "telegram"
	ChannelDiscord  NotificationChannel = "discord"
	ChannelSMS      NotificationChannel = "sms"
	ChannelPush     NotificationChannel = "push"
)

// Notification 表示一条通知消息。
type Notification struct {
	// ID 是通知的唯一标识符。
	ID string `json:"id"`
	// Title 是通知标题。
	Title string `json:"title"`
	// Body 是通知正文内容。
	Body string `json:"body"`
	// Level 是通知级别。
	Level NotificationLevel `json:"level"`
	// Source 是通知来源（系统模块名）。
	Source string `json:"source"`
	// Timestamp 是通知创建时间。
	Timestamp time.Time `json:"timestamp"`
	// Read 表示通知是否已读。
	Read bool `json:"read"`
	// Action 是通知关联的操作链接或标识。
	Action string `json:"action,omitempty"`
	// Category 是通知分类。
	Category NotificationCategory `json:"category"`
	// Channels 是此通知分发的渠道列表。
	Channels []NotificationChannel `json:"channels,omitempty"`
	// Metadata 包含通知的附加键值对数据。
	Metadata map[string]string `json:"metadata,omitempty"`
	// GroupKey 用于通知分组和折叠。
	GroupKey string `json:"group_key,omitempty"`
}

// NotificationFilter 定义查询通知的过滤条件。
type NotificationFilter struct {
	// Levels 过滤指定级别的通知。
	Levels []NotificationLevel `json:"levels,omitempty"`
	// Categories 过滤指定分类的通知。
	Categories []NotificationCategory `json:"categories,omitempty"`
	// Sources 过滤指定来源的通知。
	Sources []string `json:"sources,omitempty"`
	// Read 过滤已读/未读状态（nil 表示不过滤）。
	Read *bool `json:"read,omitempty"`
	// StartTime 过滤在此时间之后的通知。
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime 过滤在此时间之前的通知。
	EndTime *time.Time `json:"end_time,omitempty"`
	// Keyword 匹配标题或正文中的关键词。
	Keyword string `json:"keyword,omitempty"`
	// GroupKey 过滤指定分组键的通知。
	GroupKey string `json:"group_key,omitempty"`
	// Limit 限制返回的通知数量（默认50）。
	Limit int `json:"limit,omitempty"`
	// Offset 用于分页的偏移量。
	Offset int `json:"offset,omitempty"`
}

// ChannelConfig 定义通知渠道的配置。
type ChannelConfig struct {
	// ID 是渠道的唯一标识符。
	ID string `json:"id"`
	// Type 是渠道类型。
	Type NotificationChannel `json:"type"`
	// Name 是渠道显示名称。
	Name string `json:"name"`
	// Enabled 表示渠道是否启用。
	Enabled bool `json:"enabled"`
	// Endpoint 是渠道的端点地址（邮箱、Webhook URL、Chat ID等）。
	Endpoint string `json:"endpoint"`
	// MinLevel 是通过此渠道发送的最低级别（低级别不发送）。
	MinLevel NotificationLevel `json:"min_level"`
	// Categories 是此渠道接收的通知分类（空表示全部）。
	Categories []NotificationCategory `json:"categories,omitempty"`
	// CreatedAt 是渠道创建时间。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 是渠道最后更新时间。
	UpdatedAt time.Time `json:"updated_at"`
}

// NotificationTemplate 定义通知模板。
type NotificationTemplate struct {
	// ID 是模板的唯一标识符。
	ID string `json:"id"`
	// Name 是模板名称。
	Name string `json:"name"`
	// TitleTmpl 是标题模板（支持 {{.变量}} 占位符）。
	TitleTmpl string `json:"title_tmpl"`
	// BodyTmpl 是正文模板。
	BodyTmpl string `json:"body_tmpl"`
	// Level 是模板默认级别。
	Level NotificationLevel `json:"level"`
	// Category 是模板默认分类。
	Category NotificationCategory `json:"category"`
	// Channels 是模板默认分发渠道。
	Channels []NotificationChannel `json:"channels,omitempty"`
	// CreatedAt 是模板创建时间。
	CreatedAt time.Time `json:"created_at"`
}

// RateLimitConfig 定义频率限制配置。
type RateLimitConfig struct {
	// MaxPerMinute 每分钟最大发送数量。
	MaxPerMinute int `json:"max_per_minute"`
	// MaxPerHour 每小时最大发送数量。
	MaxPerHour int `json:"max_per_hour"`
	// CooldownSeconds 相同 GroupKey 的冷却时间（秒）。
	CooldownSeconds int `json:"cooldown_seconds"`
}

// NotificationStats 包含通知的统计信息。
type NotificationStats struct {
	// Total 是通知总数。
	Total int `json:"total"`
	// Unread 是未读通知数。
	Unread int `json:"read"`
	// ByLevel 按级别统计的通知数量。
	ByLevel map[NotificationLevel]int `json:"by_level"`
	// ByCategory 按分类统计的通知数量。
	ByCategory map[NotificationCategory]int `json:"by_category"`
	// LastHourCount 最近一小时的通知数。
	LastHourCount int `json:"last_hour_count"`
	// GeneratedAt 是统计生成时间。
	GeneratedAt time.Time `json:"generated_at"`
}

// sendRecord 记录发送历史，用于频率限制。
type sendRecord struct {
	Timestamp time.Time
	GroupKey  string
}

// NotificationCenter 是通知中心的核心管理器。
type NotificationCenter struct {
	mu            sync.RWMutex
	notifications []Notification
	channels      map[string]*ChannelConfig
	templates     map[string]*NotificationTemplate
	config        RateLimitConfig
	sendHistory   []sendRecord
	idCounter     int64
}

// NewNotificationCenter 创建并返回一个新的通知中心实例。
// config 参数指定频率限制配置，如果为 nil 则使用默认配置。
func NewNotificationCenter(config *RateLimitConfig) *NotificationCenter {
	cfg := RateLimitConfig{
		MaxPerMinute:    30,
		MaxPerHour:      200,
		CooldownSeconds: 60,
	}
	if config != nil {
		cfg = *config
	}

	return &NotificationCenter{
		notifications: make([]Notification, 0, 1000),
		channels:      make(map[string]*ChannelConfig),
		templates:     make(map[string]*NotificationTemplate),
		config:        cfg,
		sendHistory:   make([]sendRecord, 0, 1000),
	}
}

// Send 发送一条通知到通知中心。
// 它会检查频率限制，自动生成 ID 和时间戳，并分发到配置的渠道。
// 返回创建的通知和可能的错误。
func (nc *NotificationCenter) Send(notification Notification) (Notification, error) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	// 验证必填字段
	if notification.Title == "" {
		return Notification{}, fmt.Errorf("title is required")
	}
	if notification.Source == "" {
		return Notification{}, fmt.Errorf("source is required")
	}

	// 检查频率限制
	if err := nc.checkRateLimit(notification.GroupKey); err != nil {
		return Notification{}, err
	}

	// 生成 ID 和时间戳
	nc.idCounter++
	notification.ID = fmt.Sprintf("notif_%d_%d", time.Now().UnixNano(), nc.idCounter)
	notification.Timestamp = time.Now()

	// 设置默认值
	if notification.Level == "" {
		notification.Level = LevelInfo
	}
	if notification.Category == "" {
		Category := NotificationCategory("system")
		notification.Category = Category
	}

	// 默认渠道
	if len(notification.Channels) == 0 {
		notification.Channels = nc.getDefaultChannels(notification)
	}

	// 记录发送历史
	nc.sendHistory = append(nc.sendHistory, sendRecord{
		Timestamp: notification.Timestamp,
		GroupKey:  notification.GroupKey,
	})

	// 添加到通知列表
	nc.notifications = append(nc.notifications, notification)

	// 异步分发到渠道
	go nc.dispatchNotification(notification)

	return notification, nil
}

// SendFromTemplate 使用模板发送通知。
// templateID 指定模板 ID，data 为模板变量。
func (nc *NotificationCenter) SendFromTemplate(templateID string, data map[string]string) (Notification, error) {
	nc.mu.RLock()
	tmpl, ok := nc.templates[templateID]
	nc.mu.RUnlock()

	if !ok {
		return Notification{}, fmt.Errorf("template %s not found", templateID)
	}

	notification := Notification{
		Title:    nc.renderTemplate(tmpl.TitleTmpl, data),
		Body:     nc.renderTemplate(tmpl.BodyTmpl, data),
		Level:    tmpl.Level,
		Category: tmpl.Category,
		Channels: tmpl.Channels,
	}

	return nc.Send(notification)
}

// Query 根据过滤条件查询通知。
// 返回匹配的通知列表和可能的错误。
func (nc *NotificationCenter) Query(filter NotificationFilter) ([]Notification, error) {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	var results []Notification

	for _, notif := range nc.notifications {
		if nc.matchesFilter(notif, filter) {
			results = append(results, notif)
		}
	}

	// 按时间倒序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// 应用分页
	offset := filter.Offset
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	if offset >= len(results) {
		return []Notification{}, nil
	}

	end := offset + limit
	if end > len(results) {
		end = len(results)
	}

	return results[offset:end], nil
}

// GetUnread 获取未读通知列表。
func (nc *NotificationCenter) GetUnread() []Notification {
	read := false
	filter := NotificationFilter{
		Read: &read,
		Limit: 100,
	}
	results, _ := nc.Query(filter)
	return results
}

// MarkAsRead 将指定通知标记为已读。
func (nc *NotificationCenter) MarkAsRead(notificationID string) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	for i, notif := range nc.notifications {
		if notif.ID == notificationID {
			nc.notifications[i].Read = true
			return nil
		}
	}

	return fmt.Errorf("notification %s not found", notificationID)
}

// MarkAllAsRead 将所有通知标记为已读。
func (nc *NotificationCenter) MarkAllAsRead() int {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	count := 0
	for i := range nc.notifications {
		if !nc.notifications[i].Read {
			nc.notifications[i].Read = true
			count++
		}
	}

	return count
}

// Delete 删除指定通知。
func (nc *NotificationCenter) Delete(notificationID string) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	for i, notif := range nc.notifications {
		if notif.ID == notificationID {
			nc.notifications = append(nc.notifications[:i], nc.notifications[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("notification %s not found", notificationID)
}

// AddChannel 添加或更新通知渠道配置。
func (nc *NotificationCenter) AddChannel(config ChannelConfig) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if config.CreatedAt.IsZero() {
		config.CreatedAt = time.Now()
	}
	config.UpdatedAt = time.Now()

	nc.channels[config.ID] = &config
}

// UpdateChannel 更新指定渠道的配置。
func (nc *NotificationCenter) UpdateChannel(channelID string, config ChannelConfig) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	existing, ok := nc.channels[channelID]
	if !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	config.ID = channelID
	config.CreatedAt = existing.CreatedAt
	config.UpdatedAt = time.Now()

	nc.channels[channelID] = &config
	return nil
}

// GetChannels 获取所有渠道配置列表。
func (nc *NotificationCenter) GetChannels() []ChannelConfig {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	channels := make([]ChannelConfig, 0, len(nc.channels))
	for _, ch := range nc.channels {
		channels = append(channels, *ch)
	}

	return channels
}

// GetChannel 获取指定渠道的配置。
func (nc *NotificationCenter) GetChannel(channelID string) (ChannelConfig, error) {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	ch, ok := nc.channels[channelID]
	if !ok {
		return ChannelConfig{}, fmt.Errorf("channel %s not found", channelID)
	}

	return *ch, nil
}

// AddTemplate 添加通知模板。
func (nc *NotificationCenter) AddTemplate(template NotificationTemplate) {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if template.CreatedAt.IsZero() {
		template.CreatedAt = time.Now()
	}

	nc.templates[template.ID] = &template
}

// GetStats 获取通知统计信息。
func (nc *NotificationCenter) GetStats() NotificationStats {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	stats := NotificationStats{
		Total:       len(nc.notifications),
		ByLevel:     make(map[NotificationLevel]int),
		ByCategory:  make(map[NotificationCategory]int),
		GeneratedAt: time.Now(),
	}

	oneHourAgo := time.Now().Add(-1 * time.Hour)

	for _, notif := range nc.notifications {
		if !notif.Read {
			stats.Unread++
		}

		stats.ByLevel[notif.Level]++
		stats.ByCategory[notif.Category]++

		if notif.Timestamp.After(oneHourAgo) {
			stats.LastHourCount++
		}
	}

	return stats
}

// GetUnreadCount 获取未读通知数量。
func (nc *NotificationCenter) GetUnreadCount() int {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	count := 0
	for _, notif := range nc.notifications {
		if !notif.Read {
			count++
		}
	}
	return count
}

// checkRateLimit 检查频率限制。
func (nc *NotificationCenter) checkRateLimit(groupKey string) error {
	now := time.Now()
	oneMinuteAgo := now.Add(-1 * time.Minute)
	oneHourAgo := now.Add(-1 * time.Hour)

	// 清理过期记录
	validRecords := make([]sendRecord, 0, len(nc.sendHistory))
	for _, record := range nc.sendHistory {
		if record.Timestamp.After(oneHourAgo) {
			validRecords = append(validRecords, record)
		}
	}
	nc.sendHistory = validRecords

	// 检查每分钟限制
	minuteCount := 0
	for _, record := range nc.sendHistory {
		if record.Timestamp.After(oneMinuteAgo) {
			minuteCount++
		}
	}
	if minuteCount >= nc.config.MaxPerMinute {
		return fmt.Errorf("rate limit exceeded: max %d per minute", nc.config.MaxPerMinute)
	}

	// 检查每小时限制
	if len(nc.sendHistory) >= nc.config.MaxPerHour {
		return fmt.Errorf("rate limit exceeded: max %d per hour", nc.config.MaxPerHour)
	}

	// 检查相同 GroupKey 的冷却时间
	if groupKey != "" {
		cooldown := time.Duration(nc.config.CooldownSeconds) * time.Second
		for _, record := range nc.sendHistory {
			if record.GroupKey == groupKey && now.Sub(record.Timestamp) < cooldown {
				return fmt.Errorf("group %s is in cooldown period", groupKey)
			}
		}
	}

	return nil
}

// getDefaultChannels 获取通知的默认分发渠道。
func (nc *NotificationCenter) getDefaultChannels(notification Notification) []NotificationChannel {
	var channels []NotificationChannel

	for _, ch := range nc.channels {
		if !ch.Enabled {
			continue
		}

		// 检查级别阈值
		if !nc.meetsLevelRequirement(notification.Level, ch.MinLevel) {
			continue
		}

		// 检查分类过滤
		if len(ch.Categories) > 0 {
			found := false
			for _, cat := range ch.Categories {
				if cat == notification.Category {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		channels = append(channels, ch.Type)
	}

	return channels
}

// meetsLevelRequirement 检查通知级别是否满足渠道的最低级别要求。
func (nc *NotificationCenter) meetsLevelRequirement(notifLevel, minLevel NotificationLevel) bool {
	levelOrder := map[NotificationLevel]int{
		LevelInfo:     0,
		LevelWarning:  1,
		LevelError:    2,
		LevelCritical: 3,
	}

	return levelOrder[notifLevel] >= levelOrder[minLevel]
}

// dispatchNotification 分发通知到各个渠道。
func (nc *NotificationCenter) dispatchNotification(notification Notification) {
	nc.mu.RLock()
	defer nc.mu.RUnlock()

	for _, channelType := range notification.Channels {
		// 查找对应的渠道配置
		for _, ch := range nc.channels {
			if ch.Type == channelType && ch.Enabled {
				go nc.sendToChannel(ch, notification)
			}
		}
	}
}

// sendToChannel 发送通知到指定渠道。
// 这是实际发送的接口，可以被替换为真实的发送实现。
func (nc *NotificationCenter) sendToChannel(channel *ChannelConfig, notification Notification) {
	// 实际实现会调用各渠道的 SDK/API
	// 这里仅做日志记录
	fmt.Printf("[通知中心] 发送通知到 %s 渠道: %s - %s\n", channel.Type, notification.Title, notification.Body)
}

// renderTemplate 渲染模板字符串。
func (nc *NotificationCenter) renderTemplate(tmpl string, data map[string]string) string {
	result := tmpl
	for key, value := range data {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

// matchesFilter 检查通知是否匹配过滤条件。
func (nc *NotificationCenter) matchesFilter(notif Notification, filter NotificationFilter) bool {
	// 级别过滤
	if len(filter.Levels) > 0 {
		found := false
		for _, l := range filter.Levels {
			if notif.Level == l {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 分类过滤
	if len(filter.Categories) > 0 {
		found := false
		for _, c := range filter.Categories {
			if notif.Category == c {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 来源过滤
	if len(filter.Sources) > 0 {
		found := false
		for _, s := range filter.Sources {
			if notif.Source == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 已读状态过滤
	if filter.Read != nil && notif.Read != *filter.Read {
		return false
	}

	// 时间过滤
	if filter.StartTime != nil && notif.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && notif.Timestamp.After(*filter.EndTime) {
		return false
	}

	// 关键词过滤
	if filter.Keyword != "" {
		keyword := strings.ToLower(filter.Keyword)
		if !strings.Contains(strings.ToLower(notif.Title), keyword) &&
			!strings.Contains(strings.ToLower(notif.Body), keyword) {
			return false
		}
	}

	// 分组键过滤
	if filter.GroupKey != "" && notif.GroupKey != filter.GroupKey {
		return false
	}

	return true
}
