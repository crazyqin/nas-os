package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if len(m.notifiers) != 0 {
		t.Errorf("Expected empty notifiers list, got %d", len(m.notifiers))
	}
}

func TestManagerAddNotifier(t *testing.T) {
	m := NewManager()
	mock := &mockNotifier{}
	m.AddNotifier(mock)

	if len(m.notifiers) != 1 {
		t.Errorf("Expected 1 notifier, got %d", len(m.notifiers))
	}
}

func TestManagerSend(t *testing.T) {
	m := NewManager()
	mock := &mockNotifier{}
	m.AddNotifier(mock)

	notif := &Notification{
		Title:   "Test Alert",
		Message: "This is a test message",
		Level:   LevelInfo,
		Source:  "test",
	}

	err := m.Send(notif)
	if err != nil {
		t.Errorf("Send() returned error: %v", err)
	}

	if !mock.called {
		t.Error("Notifier.Send() was not called")
	}

	if notif.Timestamp.IsZero() {
		t.Error("Timestamp was not set")
	}
}

func TestManagerSendMultipleNotifiers(t *testing.T) {
	m := NewManager()
	mock1 := &mockNotifier{}
	mock2 := &mockNotifier{name: "notifier2"}
	m.AddNotifier(mock1)
	m.AddNotifier(mock2)

	notif := &Notification{
		Title:   "Test",
		Message: "Test message",
		Level:   LevelWarning,
	}

	_ = m.Send(notif)

	if !mock1.called || !mock2.called {
		t.Error("Not all notifiers were called")
	}
}

func TestEmailNotifierName(t *testing.T) {
	n := NewEmailNotifier("smtp.test.com", 587, "user", "pass", "from@test.com", []string{"to@test.com"})
	if n.Name() != "email" {
		t.Errorf("Expected name 'email', got '%s'", n.Name())
	}
}

func TestEmailNotifierGetLevelLabel(t *testing.T) {
	n := &EmailNotifier{}

	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{LevelInfo, "信息"},
		{LevelWarning, "警告"},
		{LevelCritical, "严重"},
		{AlertLevel("unknown"), "未知"},
	}

	for _, tt := range tests {
		result := n.getLevelLabel(tt.level)
		if result != tt.expected {
			t.Errorf("getLevelLabel(%s) = %s, expected %s", tt.level, result, tt.expected)
		}
	}
}

func TestWeChatNotifierName(t *testing.T) {
	n := NewWeChatNotifier("https://test.webhook.url")
	if n.Name() != "wechat" {
		t.Errorf("Expected name 'wechat', got '%s'", n.Name())
	}
}

func TestWeChatNotifierSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWeChatNotifier(server.URL)

	notif := &Notification{
		Title:     "Test",
		Message:   "Test message",
		Level:     LevelCritical,
		Timestamp: time.Now(),
		Source:    "test",
	}

	err := n.Send(notif)
	if err != nil {
		t.Errorf("Send() returned error: %v", err)
	}
}

func TestWeChatNotifierGetLevelColor(t *testing.T) {
	n := &WeChatNotifier{}

	tests := []struct {
		level    AlertLevel
		expected string
	}{
		{LevelInfo, "info"},
		{LevelWarning, "warning"},
		{LevelCritical, "error"},
		{AlertLevel("unknown"), "comment"},
	}

	for _, tt := range tests {
		result := n.getLevelColor(tt.level)
		if result != tt.expected {
			t.Errorf("getLevelColor(%s) = %s, expected %s", tt.level, result, tt.expected)
		}
	}
}

func TestWebhookNotifierName(t *testing.T) {
	n := NewWebhookNotifier("https://test.webhook.url")
	if n.Name() != "webhook" {
		t.Errorf("Expected name 'webhook', got '%s'", n.Name())
	}
}

func TestWebhookNotifierSend(t *testing.T) {
	var receivedNotif Notification
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedNotif); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	n := NewWebhookNotifier(server.URL)

	notif := &Notification{
		Title:     "Test Alert",
		Message:   "This is a test message",
		Level:     LevelWarning,
		Timestamp: time.Now(),
		Source:    "unit-test",
	}

	err := n.Send(notif)
	if err != nil {
		t.Errorf("Send() returned error: %v", err)
	}

	if receivedNotif.Title != notif.Title {
		t.Errorf("Received title = %s, expected %s", receivedNotif.Title, notif.Title)
	}
	if receivedNotif.Message != notif.Message {
		t.Errorf("Received message = %s, expected %s", receivedNotif.Message, notif.Message)
	}
}

func TestWebhookNotifierSendError(t *testing.T) {
	n := NewWebhookNotifier("http://invalid-url-that-does-not-exist.local")

	notif := &Notification{
		Title:   "Test",
		Message: "Test message",
		Level:   LevelInfo,
	}

	err := n.Send(notif)
	if err == nil {
		t.Error("Expected error for invalid URL, got nil")
	}
}

// mockNotifier 用于测试的模拟通知器
type mockNotifier struct {
	name   string
	called bool
	err    error
}

func (m *mockNotifier) Send(notif *Notification) error {
	m.called = true
	return m.err
}

func (m *mockNotifier) Name() string {
	if m.name == "" {
		return "mock"
	}
	return m.name
}