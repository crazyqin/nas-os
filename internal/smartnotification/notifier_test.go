// smartnotification 单元测试
package smartnotification

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// mockHandler 模拟通知处理器
type mockHandler struct {
	sendCount int
	shouldFail bool
}

func (m *mockHandler) Send(ctx context.Context, notification *Notification) error {
	m.sendCount++
	if m.shouldFail {
		return fmt.Errorf("mock send failure")
	}
	return nil
}

func TestNewSmartNotifier(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	if notifier == nil {
		t.Fatal("NewSmartNotifier returned nil")
	}
	if notifier.config.MaxQueueSize != 10000 {
		t.Errorf("Expected MaxQueueSize=10000, got %d", notifier.config.MaxQueueSize)
	}
}

func TestSend(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	handler := &mockHandler{}
	notifier.RegisterHandler(ChannelWebhook, handler)

	notification := &Notification{
		Title:    "测试通知",
		Body:     "这是一条测试通知",
		Source:   "test",
		Category: "system",
		Priority: PriorityNormal,
		Channels: []NotifyChannel{ChannelWebhook},
	}

	ctx := context.Background()
	result, err := notifier.Send(ctx, notification)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if result.Status != StatusSent {
		t.Errorf("Expected status sent, got %s", result.Status)
	}
	if handler.sendCount != 1 {
		t.Errorf("Expected handler called once, got %d", handler.sendCount)
	}
}

func TestSendDeduplication(t *testing.T) {
	config := DefaultConfig()
	config.DedupWindow = time.Minute * 5
	notifier := NewSmartNotifier(config)

	handler := &mockHandler{}
	notifier.RegisterHandler(ChannelWebhook, handler)

	notification := &Notification{
		Title:    "重复通知",
		Body:     "重复内容",
		Source:   "test",
		Category: "system",
		Channels: []NotifyChannel{ChannelWebhook},
		GroupKey: "dedup-key-1",
	}

	ctx := context.Background()

	// 第一次发送
	result1, _ := notifier.Send(ctx, notification)
	if result1.Status != StatusSent {
		t.Errorf("First send should be sent, got %s", result1.Status)
	}

	// 第二次发送（应该被去重）
	notification2 := &Notification{
		Title:    "重复通知",
		Body:     "重复内容",
		Source:   "test",
		Category: "system",
		Channels: []NotifyChannel{ChannelWebhook},
		GroupKey: "dedup-key-1",
	}
	result2, _ := notifier.Send(ctx, notification2)
	if result2.Status != StatusDeduped {
		t.Errorf("Second send should be deduped, got %s", result2.Status)
	}

	if handler.sendCount != 1 {
		t.Errorf("Handler should be called once, got %d", handler.sendCount)
	}
}

func TestSilencRule(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	handler := &mockHandler{}
	notifier.RegisterHandler(ChannelWebhook, handler)

	// 添加静默规则
	notifier.AddSilencRule(SilencRule{
		Name:      "静默系统通知",
		Source:    "system",
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
		Enabled:   true,
	})

	// 发送匹配静默规则的通知
	notification := &Notification{
		Title:    "系统通知",
		Body:     "静默测试",
		Source:   "system",
		Category: "info",
		Channels: []NotifyChannel{ChannelWebhook},
	}

	ctx := context.Background()
	result, _ := notifier.Send(ctx, notification)
	if result.Status != StatusSilenced {
		t.Errorf("Expected silenced, got %s", result.Status)
	}

	if handler.sendCount != 0 {
		t.Errorf("Handler should not be called, got %d", handler.sendCount)
	}
}

func TestSilencRuleDisabled(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	handler := &mockHandler{}
	notifier.RegisterHandler(ChannelWebhook, handler)

	// 添加禁用的静默规则
	notifier.AddSilencRule(SilencRule{
		Name:      "禁用规则",
		Source:    "system",
		StartTime: time.Now().Add(-time.Hour),
		EndTime:   time.Now().Add(time.Hour),
		Enabled:   false,
	})

	notification := &Notification{
		Title:    "系统通知",
		Body:     "测试",
		Source:   "system",
		Channels: []NotifyChannel{ChannelWebhook},
	}

	ctx := context.Background()
	result, _ := notifier.Send(ctx, notification)
	if result.Status != StatusSent {
		t.Errorf("Expected sent (rule disabled), got %s", result.Status)
	}
}

func TestMultipleChannels(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	webhookHandler := &mockHandler{}
	emailHandler := &mockHandler{}
	notifier.RegisterHandler(ChannelWebhook, webhookHandler)
	notifier.RegisterHandler(ChannelEmail, emailHandler)

	notification := &Notification{
		Title:    "多渠道通知",
		Body:     "测试多渠道",
		Source:   "test",
		Channels: []NotifyChannel{ChannelWebhook, ChannelEmail},
	}

	ctx := context.Background()
	result, _ := notifier.Send(ctx, notification)
	if result.Status != StatusSent {
		t.Errorf("Expected sent, got %s", result.Status)
	}

	if webhookHandler.sendCount != 1 {
		t.Errorf("Webhook handler should be called once, got %d", webhookHandler.sendCount)
	}
	if emailHandler.sendCount != 1 {
		t.Errorf("Email handler should be called once, got %d", emailHandler.sendCount)
	}
}

func TestGenerateSummary(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	handler := &mockHandler{}
	notifier.RegisterHandler(ChannelWebhook, handler)

	ctx := context.Background()

	// 发送多条通知
	for i := 0; i < 5; i++ {
		notifier.Send(ctx, &Notification{
			Title:    fmt.Sprintf("通知 %d", i),
			Source:   "test",
			Category: "system",
			Channels: []NotifyChannel{ChannelWebhook},
		})
	}

	summary := notifier.GenerateSummary(time.Hour)
	if summary.TotalSent != 5 {
		t.Errorf("Expected 5 sent, got %d", summary.TotalSent)
	}
	if summary.ByCategory["system"] != 5 {
		t.Errorf("Expected 5 system notifications, got %d", summary.ByCategory["system"])
	}
}

func TestGetStats(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	handler := &mockHandler{}
	notifier.RegisterHandler(ChannelWebhook, handler)

	ctx := context.Background()
	notifier.Send(ctx, &Notification{
		Title:    "统计测试",
		Source:   "test",
		Channels: []NotifyChannel{ChannelWebhook},
	})

	stats := notifier.GetStats()
	if stats["sent"] != 1 {
		t.Errorf("Expected sent=1, got %d", stats["sent"])
	}
	if stats["historySize"] != 1 {
		t.Errorf("Expected historySize=1, got %d", stats["historySize"])
	}
}

func TestMarkAsRead(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	handler := &mockHandler{}
	notifier.RegisterHandler(ChannelWebhook, handler)

	ctx := context.Background()
	result, _ := notifier.Send(ctx, &Notification{
		ID:       "read-test-1",
		Title:    "已读测试",
		Source:   "test",
		Channels: []NotifyChannel{ChannelWebhook},
	})

	err := notifier.MarkAsRead(result.NotificationID)
	if err != nil {
		t.Fatalf("MarkAsRead failed: %v", err)
	}
}

func TestMarkAsReadNotFound(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	err := notifier.MarkAsRead("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent notification")
	}
}

func TestCleanupExpired(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	// 手动添加过期通知
	notifier.mu.Lock()
	notifier.history = append(notifier.history, &Notification{
		ID:        "expired-1",
		Title:     "过期通知",
		Timestamp: time.Now().Add(-time.Hour * 48),
		TTL:       time.Hour,
	})
	notifier.history = append(notifier.history, &Notification{
		ID:        "valid-1",
		Title:     "有效通知",
		Timestamp: time.Now(),
		TTL:       time.Hour * 24,
	})
	notifier.mu.Unlock()

	cleaned := notifier.CleanupExpired()
	if cleaned != 1 {
		t.Errorf("Expected 1 cleaned, got %d", cleaned)
	}

	if len(notifier.history) != 1 {
		t.Errorf("Expected 1 remaining, got %d", len(notifier.history))
	}
}

func TestQueueSize(t *testing.T) {
	config := DefaultConfig()
	notifier := NewSmartNotifier(config)

	if notifier.QueueSize() != 0 {
		t.Errorf("Expected queue size 0, got %d", notifier.QueueSize())
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxQueueSize != 10000 {
		t.Errorf("Expected MaxQueueSize=10000, got %d", config.MaxQueueSize)
	}
	if config.MaxRetries != 3 {
		t.Errorf("Expected MaxRetries=3, got %d", config.MaxRetries)
	}
	if config.DedupWindow != time.Minute*10 {
		t.Errorf("Expected DedupWindow=10m, got %v", config.DedupWindow)
	}
}
