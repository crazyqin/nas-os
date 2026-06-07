package mailserver

import (
	"testing"
)

func TestNewMailServer(t *testing.T) {
	config := &Config{
		SMTPHost:       "localhost",
		SMTPPort:       25,
		IMAPHost:       "localhost",
		IMAPPort:       143,
		MaxMailboxes:   100,
		MaxMessageSize: 10 * 1024 * 1024,
		EnableTLS:      false,
	}

	ms := NewMailServer(config)
	if ms == nil {
		t.Fatal("NewMailServer returned nil")
	}

	if ms.config != config {
		t.Error("config not set correctly")
	}
}

func TestAddDomain(t *testing.T) {
	config := &Config{
		SMTPHost: "localhost",
		SMTPPort: 25,
		IMAPHost: "localhost",
		IMAPPort: 143,
	}

	ms := NewMailServer(config)

	// 添加域名
	err := ms.AddDomain("example.com")
	if err != nil {
		t.Fatalf("AddDomain failed: %v", err)
	}

	// 验证域名已添加
	domains := ms.GetDomains()
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %d", len(domains))
	}

	if domains[0].Name != "example.com" {
		t.Errorf("expected domain name 'example.com', got '%s'", domains[0].Name)
	}
}

func TestAddDuplicateDomain(t *testing.T) {
	config := &Config{
		SMTPHost: "localhost",
		SMTPPort: 25,
		IMAPHost: "localhost",
		IMAPPort: 143,
	}

	ms := NewMailServer(config)

	// 添加域名
	err := ms.AddDomain("example.com")
	if err != nil {
		t.Fatalf("AddDomain failed: %v", err)
	}

	// 尝试添加重复域名
	err = ms.AddDomain("example.com")
	if err == nil {
		t.Error("expected error when adding duplicate domain")
	}
}

func TestRemoveDomain(t *testing.T) {
	config := &Config{
		SMTPHost: "localhost",
		SMTPPort: 25,
		IMAPHost: "localhost",
		IMAPPort: 143,
	}

	ms := NewMailServer(config)

	// 添加域名
	ms.AddDomain("example.com")

	// 删除域名
	err := ms.RemoveDomain("example.com")
	if err != nil {
		t.Fatalf("RemoveDomain failed: %v", err)
	}

	// 验证域名已删除
	domains := ms.GetDomains()
	if len(domains) != 0 {
		t.Fatalf("expected 0 domains, got %d", len(domains))
	}
}

func TestAddUser(t *testing.T) {
	config := &Config{
		SMTPHost: "localhost",
		SMTPPort: 25,
		IMAPHost: "localhost",
		IMAPPort: 143,
	}

	ms := NewMailServer(config)

	// 添加域名
	ms.AddDomain("example.com")

	// 添加用户
	err := ms.AddUser("user1", "example.com", "password123", 100*1024*1024)
	if err != nil {
		t.Fatalf("AddUser failed: %v", err)
	}

	// 验证用户已添加
	users := ms.GetUsers("example.com")
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}

	if users[0].Username != "user1" {
		t.Errorf("expected username 'user1', got '%s'", users[0].Username)
	}
}

func TestSendMessage(t *testing.T) {
	config := &Config{
		SMTPHost:       "localhost",
		SMTPPort:       25,
		IMAPHost:       "localhost",
		IMAPPort:       143,
		MaxMessageSize: 10 * 1024 * 1024,
	}

	ms := NewMailServer(config)

	// 添加域名和用户
	ms.AddDomain("example.com")
	ms.AddUser("sender", "example.com", "pass123", 100*1024*1024)
	ms.AddUser("receiver", "example.com", "pass456", 100*1024*1024)

	// 发送邮件
	msg, err := ms.SendMessage(
		"sender@example.com",
		[]string{"receiver@example.com"},
		"Test Subject",
		"Test Body",
		"",
		nil,
	)

	if err != nil {
		t.Fatalf("SendMessage failed: %v", err)
	}

	if msg.Subject != "Test Subject" {
		t.Errorf("expected subject 'Test Subject', got '%s'", msg.Subject)
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{
		SMTPHost:  "localhost",
		SMTPPort:  25,
		IMAPHost:  "localhost",
		IMAPPort:  143,
		EnableTLS: true,
	}

	ms := NewMailServer(config)

	// 添加数据
	ms.AddDomain("example.com")
	ms.AddUser("user1", "example.com", "pass123", 100*1024*1024)

	stats := ms.GetStats()

	if stats["domains"] != 1 {
		t.Errorf("expected 1 domain, got %v", stats["domains"])
	}

	if stats["users"] != 1 {
		t.Errorf("expected 1 user, got %v", stats["users"])
	}

	if stats["tls_enabled"] != true {
		t.Error("expected TLS enabled")
	}
}
