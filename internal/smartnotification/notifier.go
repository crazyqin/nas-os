// Package smartnotification 提供智能通知聚合功能。
// 支持多通道通知（邮件/WebSocket/Webhook/短信）、智能去重、
// 通知分组、静默规则、优先级队列和通知摘要。
// 对标群晖 Notification Center + TrueNAS Alert Service。
package smartnotification

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// NotifyChannel 通知渠道
type NotifyChannel string

const (
	ChannelEmail     NotifyChannel = "email"
	ChannelWebhook   NotifyChannel = "webhook"
	ChannelWebSocket NotifyChannel = "websocket"
	ChannelSMS       NotifyChannel = "sms"
	ChannelApp       NotifyChannel = "app" // 移动端推送
)

// Priority 通知优先级
type Priority int

const (
	PriorityLow    Priority = 0
	PriorityNormal Priority = 1
	PriorityHigh   Priority = 2
	PriorityUrgent Priority = 3
)

// NotifyStatus 通知状态
type NotifyStatus string

const (
	StatusPending   NotifyStatus = "pending"
	StatusSent      NotifyStatus = "sent"
	StatusDelivered NotifyStatus = "delivered"
	StatusFailed    NotifyStatus = "failed"
	StatusSilenced  NotifyStatus = "silenced"
	StatusDeduped   NotifyStatus = "deduped"
)

// Notification 通知消息
type Notification struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Source    string            `json:"source"`   // 来源模块
	Category  string            `json:"category"` // 分类
	Priority  Priority          `json:"priority"`
	Channels  []NotifyChannel   `json:"channels"`
	Status    NotifyStatus      `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	SentAt    *time.Time        `json:"sentAt,omitempty"`
	ReadAt    *time.Time        `json:"readAt,omitempty"`
	GroupKey  string            `json:"groupKey"` // 用于去重和分组
	TTL       time.Duration     `json:"ttl"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// SilencRule 静默规则
type SilencRule struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Source    string        `json:"source,omitempty"`      // 匹配来源
	Category  string        `json:"category,omitempty"`    // 匹配分类
	Channel   NotifyChannel `json:"channel,omitempty"`     // 匹配渠道
	Priority  Priority      `json:"minPriority,omitempty"` // 最低优先级
	StartTime time.Time     `json:"startTime"`
	EndTime   time.Time     `json:"endTime"`
	Enabled   bool          `json:"enabled"`
}

// NotifyConfig 通知配置
type NotifyConfig struct {
	MaxQueueSize    int           `json:"maxQueueSize"`
	MaxRetries      int           `json:"maxRetries"`
	RetryInterval   time.Duration `json:"retryInterval"`
	DedupWindow     time.Duration `json:"dedupWindow"`
	GroupWindow     time.Duration `json:"groupWindow"`
	SummaryInterval time.Duration `json:"summaryInterval"`
}

// DefaultConfig 默认配置
func DefaultConfig() NotifyConfig {
	return NotifyConfig{
		MaxQueueSize:    10000,
		MaxRetries:      3,
		RetryInterval:   time.Minute * 5,
		DedupWindow:     time.Minute * 10,
		GroupWindow:     time.Minute * 5,
		SummaryInterval: time.Hour,
	}
}

// NotifyResult 通知结果
type NotifyResult struct {
	NotificationID string        `json:"notificationId"`
	Channel        NotifyChannel `json:"channel"`
	Status         NotifyStatus  `json:"status"`
	Error          string        `json:"error,omitempty"`
	SentAt         time.Time     `json:"sentAt"`
}

// SummaryReport 通知摘要
type SummaryReport struct {
	Period        time.Duration         `json:"period"`
	TotalSent     int                   `json:"totalSent"`
	TotalFailed   int                   `json:"totalFailed"`
	TotalSilenced int                   `json:"totalSilenced"`
	TotalDeduped  int                   `json:"totalDeduped"`
	ByCategory    map[string]int        `json:"byCategory"`
	ByChannel     map[NotifyChannel]int `json:"byChannel"`
	TopSources    []SourceCount         `json:"topSources"`
	GeneratedAt   time.Time             `json:"generatedAt"`
}

// SourceCount 来源统计
type SourceCount struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

// SmartNotifier 智能通知引擎
type SmartNotifier struct {
	mu         sync.RWMutex
	config     NotifyConfig
	queue      []*Notification
	history    []*Notification
	rules      []SilencRule
	handlers   map[NotifyChannel]NotifyHandler
	dedupCache map[string]time.Time
	groupCache map[string][]*Notification
	stats      *notifyStats
}

// NotifyHandler 通知处理器接口
type NotifyHandler interface {
	Send(ctx context.Context, notification *Notification) error
}

type notifyStats struct {
	sent     int
	failed   int
	silenced int
	deduped  int
}

// NewSmartNotifier 创建智能通知引擎
func NewSmartNotifier(config NotifyConfig) *SmartNotifier {
	return &SmartNotifier{
		config:     config,
		queue:      make([]*Notification, 0, config.MaxQueueSize),
		history:    make([]*Notification, 0, 1000),
		rules:      make([]SilencRule, 0),
		handlers:   make(map[NotifyChannel]NotifyHandler),
		dedupCache: make(map[string]time.Time),
		groupCache: make(map[string][]*Notification),
		stats:      &notifyStats{},
	}
}

// RegisterHandler 注册通知处理器
func (sn *SmartNotifier) RegisterHandler(channel NotifyChannel, handler NotifyHandler) {
	sn.mu.Lock()
	defer sn.mu.Unlock()
	sn.handlers[channel] = handler
}

// AddSilencRule 添加静默规则
func (sn *SmartNotifier) AddSilencRule(rule SilencRule) {
	sn.mu.Lock()
	defer sn.mu.Unlock()
	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule-%d", time.Now().UnixNano())
	}
	sn.rules = append(sn.rules, rule)
	log.Printf("[SmartNotify] 添加静默规则: %s", rule.Name)
}

// Send 发送通知
func (sn *SmartNotifier) Send(ctx context.Context, notification *Notification) (*NotifyResult, error) {
	sn.mu.Lock()
	defer sn.mu.Unlock()

	// 设置默认值
	if notification.ID == "" {
		notification.ID = fmt.Sprintf("notif-%d", time.Now().UnixNano())
	}
	if notification.Timestamp.IsZero() {
		notification.Timestamp = time.Now()
	}
	if notification.TTL == 0 {
		notification.TTL = time.Hour * 24
	}

	// 检查去重
	if sn.isDuplicate(notification) {
		sn.stats.deduped++
		notification.Status = StatusDeduped
		return &NotifyResult{
			NotificationID: notification.ID,
			Status:         StatusDeduped,
			SentAt:         time.Now(),
		}, nil
	}

	// 检查静默规则
	if sn.isSilenced(notification) {
		sn.stats.silenced++
		notification.Status = StatusSilenced
		return &NotifyResult{
			NotificationID: notification.ID,
			Status:         StatusSilenced,
			SentAt:         time.Now(),
		}, nil
	}

	// 更新去重缓存
	sn.dedupCache[notification.GroupKey] = time.Now()

	// 分发到各渠道
	var lastErr error
	for _, channel := range notification.Channels {
		handler, exists := sn.handlers[channel]
		if !exists {
			log.Printf("[SmartNotify] 未注册的渠道: %s", channel)
			continue
		}

		if err := handler.Send(ctx, notification); err != nil {
			sn.stats.failed++
			lastErr = err
			log.Printf("[SmartNotify] 发送失败 [%s]: %v", channel, err)
		} else {
			sn.stats.sent++
			now := time.Now()
			notification.SentAt = &now
			notification.Status = StatusSent
		}
	}

	// 记录历史
	sn.history = append(sn.history, notification)
	if len(sn.history) > 1000 {
		sn.history = sn.history[1:]
	}

	return &NotifyResult{
		NotificationID: notification.ID,
		Channel:        notification.Channels[0],
		Status:         notification.Status,
		Error:          fmt.Sprintf("%v", lastErr),
		SentAt:         time.Now(),
	}, lastErr
}

func (sn *SmartNotifier) isDuplicate(notification *Notification) bool {
	if notification.GroupKey == "" {
		return false
	}

	lastSent, exists := sn.dedupCache[notification.GroupKey]
	if !exists {
		return false
	}

	return time.Since(lastSent) < sn.config.DedupWindow
}

func (sn *SmartNotifier) isSilenced(notification *Notification) bool {
	now := time.Now()

	for _, rule := range sn.rules {
		if !rule.Enabled {
			continue
		}

		// 检查时间范围
		if now.Before(rule.StartTime) || now.After(rule.EndTime) {
			continue
		}

		// 检查匹配条件
		match := true
		if rule.Source != "" && rule.Source != notification.Source {
			match = false
		}
		if rule.Category != "" && rule.Category != notification.Category {
			match = false
		}
		if rule.Priority > 0 && notification.Priority < rule.Priority {
			match = false
		}

		// 检查渠道
		if rule.Channel != "" {
			channelMatch := false
			for _, ch := range notification.Channels {
				if ch == rule.Channel {
					channelMatch = true
					break
				}
			}
			if !channelMatch {
				match = false
			}
		}

		if match {
			return true
		}
	}

	return false
}

// GenerateSummary 生成通知摘要
func (sn *SmartNotifier) GenerateSummary(period time.Duration) *SummaryReport {
	sn.mu.RLock()
	defer sn.mu.RUnlock()

	cutoff := time.Now().Add(-period)
	report := &SummaryReport{
		Period:      period,
		ByCategory:  make(map[string]int),
		ByChannel:   make(map[NotifyChannel]int),
		GeneratedAt: time.Now(),
	}

	sourceCounts := make(map[string]int)

	for _, n := range sn.history {
		if n.Timestamp.Before(cutoff) {
			continue
		}

		switch n.Status {
		case StatusSent, StatusDelivered:
			report.TotalSent++
		case StatusFailed:
			report.TotalFailed++
		case StatusSilenced:
			report.TotalSilenced++
		case StatusDeduped:
			report.TotalDeduped++
		}

		report.ByCategory[n.Category]++
		for _, ch := range n.Channels {
			report.ByChannel[ch]++
		}
		sourceCounts[n.Source]++
	}

	// 转换为TopSources
	for source, count := range sourceCounts {
		report.TopSources = append(report.TopSources, SourceCount{
			Source: source,
			Count:  count,
		})
	}

	return report
}

// GetPendingNotifications 获取待处理通知
func (sn *SmartNotifier) GetPendingNotifications(limit int) []*Notification {
	sn.mu.RLock()
	defer sn.mu.RUnlock()

	var pending []*Notification
	for i := len(sn.queue) - 1; i >= 0; i-- {
		if sn.queue[i].Status == StatusPending {
			pending = append(pending, sn.queue[i])
			if limit > 0 && len(pending) >= limit {
				break
			}
		}
	}
	return pending
}

// MarkAsRead 标记通知为已读
func (sn *SmartNotifier) MarkAsRead(notificationID string) error {
	sn.mu.Lock()
	defer sn.mu.Unlock()

	for _, n := range sn.history {
		if n.ID == notificationID {
			now := time.Now()
			n.ReadAt = &now
			return nil
		}
	}
	return fmt.Errorf("通知不存在: %s", notificationID)
}

// CleanupExpired 清理过期通知
func (sn *SmartNotifier) CleanupExpired() int {
	sn.mu.Lock()
	defer sn.mu.Unlock()

	now := time.Now()
	before := len(sn.history)
	var valid []*Notification

	for _, n := range sn.history {
		if n.TTL > 0 && now.Sub(n.Timestamp) < n.TTL {
			valid = append(valid, n)
		}
	}

	sn.history = valid

	// 清理去重缓存
	for key, lastSent := range sn.dedupCache {
		if now.Sub(lastSent) > sn.config.DedupWindow*10 {
			delete(sn.dedupCache, key)
		}
	}

	cleaned := before - len(valid)
	if cleaned > 0 {
		log.Printf("[SmartNotify] 清理过期通知: %d 条", cleaned)
	}
	return cleaned
}

// GetStats 获取统计信息
func (sn *SmartNotifier) GetStats() map[string]int {
	sn.mu.RLock()
	defer sn.mu.RUnlock()

	return map[string]int{
		"sent":        sn.stats.sent,
		"failed":      sn.stats.failed,
		"silenced":    sn.stats.silenced,
		"deduped":     sn.stats.deduped,
		"queueSize":   len(sn.queue),
		"historySize": len(sn.history),
	}
}

// QueueSize 返回队列大小
func (sn *SmartNotifier) QueueSize() int {
	sn.mu.RLock()
	defer sn.mu.RUnlock()
	return len(sn.queue)
}
