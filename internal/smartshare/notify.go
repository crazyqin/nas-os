// Package smartshare 提供访问通知和异常告警功能
package smartshare

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Notifier 通知器.
type Notifier struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	config       *NotifyConfig
	events       []*NotifyEvent
	webhookQueue chan *NotifyEvent
	emailQueue   chan *NotifyEvent
	stopChan     chan struct{}
	running      bool
}

// NewNotifier 创建通知器.
func NewNotifier(logger *zap.Logger, config *NotifyConfig) *Notifier {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultNotifyConfig()
	}

	return &Notifier{
		logger:       logger,
		config:       config,
		events:       make([]*NotifyEvent, 0),
		webhookQueue: make(chan *NotifyEvent, 100),
		emailQueue:   make(chan *NotifyEvent, 100),
		stopChan:     make(chan struct{}),
	}
}

// Start 启动通知器.
func (n *Notifier) Start() {
	n.mu.Lock()
	if n.running {
		n.mu.Unlock()
		return
	}
	n.running = true
	n.mu.Unlock()

	go n.processWebhookQueue()
	go n.processEmailQueue()

	n.logger.Info("notifier started")
}

// Stop 停止通知器.
func (n *Notifier) Stop() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.running {
		return
	}

	close(n.stopChan)
	n.running = false
	n.logger.Info("notifier stopped")
}

// SendEvent 发送通知事件.
func (n *Notifier) SendEvent(event *NotifyEvent) {
	n.mu.Lock()
	n.events = append(n.events, event)
	n.mu.Unlock()

	// 检查是否超过每小时限制
	if n.isRateLimited() {
		n.logger.Warn("notification rate limited",
			zap.String("event_type", event.EventType))
		return
	}

	// 根据事件类型决定是否发送
	if n.shouldNotify(event) {
		// 发送到 webhook
		if n.config.WebhookURL != "" {
			select {
			case n.webhookQueue <- event:
			default:
				n.logger.Warn("webhook queue full, dropping event")
			}
		}

		// 发送邮件
		if n.config.EmailTo != "" {
			select {
			case n.emailQueue <- event:
			default:
				n.logger.Warn("email queue full, dropping event")
			}
		}
	}

	n.logger.Info("notification event recorded",
		zap.String("type", event.EventType),
		zap.String("level", string(event.Level)),
		zap.String("share_id", event.ShareID))
}

// shouldNotify 判断是否应该发送通知.
func (n *Notifier) shouldNotify(event *NotifyEvent) bool {
	switch event.EventType {
	case "view":
		return n.config.OnView
	case "download":
		return n.config.OnDownload
	case "first_access":
		return n.config.OnFirstAccess
	case "expired":
		return n.config.OnExpired
	case "anomaly":
		return n.config.OnAnomaly
	default:
		return true
	}
}

// isRateLimited 检查是否被速率限制.
func (n *Notifier) isRateLimited() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if n.config.MaxPerHour <= 0 {
		return false
	}

	// 统计最近一小时的事件数
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	count := 0
	for i := len(n.events) - 1; i >= 0; i-- {
		if n.events[i].Timestamp.After(oneHourAgo) {
			count++
		} else {
			break
		}
	}

	return count >= n.config.MaxPerHour
}

// processWebhookQueue 处理 webhook 队列.
func (n *Notifier) processWebhookQueue() {
	for {
		select {
		case <-n.stopChan:
			return
		case event := <-n.webhookQueue:
			n.sendWebhook(event)
		}
	}
}

// processEmailQueue 处理邮件队列.
func (n *Notifier) processEmailQueue() {
	for {
		select {
		case <-n.stopChan:
			return
		case event := <-n.emailQueue:
			n.sendEmail(event)
		}
	}
}

// sendWebhook 发送 webhook 通知.
func (n *Notifier) sendWebhook(event *NotifyEvent) {
	n.logger.Info("sending webhook notification",
		zap.String("url", n.config.WebhookURL),
		zap.String("event_type", event.EventType),
		zap.String("share_id", event.ShareID))

	// 实际实现需要发送 HTTP 请求到 webhook URL
	// 这里只是模拟
}

// sendEmail 发送邮件通知.
func (n *Notifier) sendEmail(event *NotifyEvent) {
	n.logger.Info("sending email notification",
		zap.String("to", n.config.EmailTo),
		zap.String("event_type", event.EventType),
		zap.String("share_id", event.ShareID))

	// 实际实现需要发送邮件
	// 这里只是模拟
}

// DetectAnomaly 检测异常访问.
func (n *Notifier) DetectAnomaly(log *AccessLog, shareLink *ShareLink) *NotifyEvent {
	// 检测异常情况

	// 1. 短时间内大量访问
	if n.detectRapidAccess(log.ShareID) {
		return &NotifyEvent{
			ID:        generateID(),
			ShareID:   log.ShareID,
			EventType: "anomaly",
			Level:     AlertLevelWarning,
			Title:     "异常访问检测",
			Message:   fmt.Sprintf("检测到短时间内大量访问，IP: %s", log.IPAddress),
			IPAddress: log.IPAddress,
			UserAgent: log.UserAgent,
			Timestamp: time.Now(),
		}
	}

	// 2. 非常规时间访问（凌晨2-6点）
	hour := log.Timestamp.Hour()
	if hour >= 2 && hour <= 6 {
		return &NotifyEvent{
			ID:        generateID(),
			ShareID:   log.ShareID,
			EventType: "anomaly",
			Level:     AlertLevelInfo,
			Title:     "非常规时间访问",
			Message:   fmt.Sprintf("在凌晨时段检测到访问，IP: %s，时间: %s", log.IPAddress, log.Timestamp.Format("15:04")),
			IPAddress: log.IPAddress,
			UserAgent: log.UserAgent,
			Timestamp: time.Now(),
		}
	}

	// 3. 检测可疑 User-Agent
	if n.detectSuspiciousUA(log.UserAgent) {
		return &NotifyEvent{
			ID:        generateID(),
			ShareID:   log.ShareID,
			EventType: "anomaly",
			Level:     AlertLevelWarning,
			Title:     "可疑 User-Agent",
			Message:   fmt.Sprintf("检测到可疑的 User-Agent: %s", log.UserAgent),
			IPAddress: log.IPAddress,
			UserAgent: log.UserAgent,
			Timestamp: time.Now(),
		}
	}

	return nil
}

// detectRapidAccess 检测短时间内大量访问.
func (n *Notifier) detectRapidAccess(shareID string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// 统计最近5分钟的访问次数
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	count := 0
	for i := len(n.events) - 1; i >= 0; i-- {
		event := n.events[i]
		if event.ShareID == shareID && event.Timestamp.After(fiveMinAgo) {
			count++
		}
	}

	// 超过20次视为异常
	return count > 20
}

// detectSuspiciousUA 检测可疑 User-Agent.
func (n *Notifier) detectSuspiciousUA(ua string) bool {
	suspiciousKeywords := []string{
		"curl", "wget", "python", "java", "go-http",
		"scanner", "crawler", "bot", "spider",
	}

	for _, keyword := range suspiciousKeywords {
		if contains(ua, keyword) {
			return true
		}
	}

	return false
}

// GetEvents 获取通知事件列表.
func (n *Notifier) GetEvents(limit int) []*NotifyEvent {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if limit <= 0 || limit > len(n.events) {
		limit = len(n.events)
	}

	events := make([]*NotifyEvent, limit)
	copy(events, n.events[len(n.events)-limit:])
	return events
}

// GetEventsByShareID 根据分享ID获取事件.
func (n *Notifier) GetEventsByShareID(shareID string, limit int) []*NotifyEvent {
	n.mu.RLock()
	defer n.mu.RUnlock()

	result := make([]*NotifyEvent, 0)
	for i := len(n.events) - 1; i >= 0; i-- {
		if n.events[i].ShareID == shareID {
			result = append(result, n.events[i])
			if len(result) >= limit {
				break
			}
		}
	}

	return result
}

// UpdateConfig 更新通知配置.
func (n *Notifier) UpdateConfig(config *NotifyConfig) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.config = config
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString 搜索子串.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// generateID 生成唯一 ID.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
