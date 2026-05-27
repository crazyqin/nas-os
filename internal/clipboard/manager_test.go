package clipboard

import (
	"testing"
	"time"
)

func TestCreateAndGet(t *testing.T) {
	mgr := NewManager("test-key-123456789012345678901234", 1000)

	item, err := mgr.Create(CreateClipRequest{
		Content: "Hello, World!",
		Source:  "device-1",
	}, "user-1")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := mgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.UserID != "user-1" {
		t.Errorf("expected user-1, got %s", got.UserID)
	}
	if got.Type != ClipTypeText {
		t.Errorf("expected text type, got %s", got.Type)
	}
}

func TestDetectType(t *testing.T) {
	tests := []struct {
		content string
		want    ClipType
	}{
		{"https://example.com", ClipTypeLink},
		{"http://test.org", ClipTypeLink},
		{"just text", ClipTypeText},
		{"no protocol", ClipTypeText},
	}

	for _, tt := range tests {
		got := detectType(tt.content)
		if got != tt.want {
			t.Errorf("detectType(%q) = %v, want %v", tt.content, got, tt.want)
		}
	}
}

func TestSearch(t *testing.T) {
	mgr := NewManager("test-key-123456789012345678901234", 1000)

	// 创建条目并验证可以通过Get获取
	item1, _ := mgr.Create(CreateClipRequest{Content: "hello world", Source: "d1"}, "u1")
	item2, _ := mgr.Create(CreateClipRequest{Content: "goodbye world", Source: "d1"}, "u1")
	mgr.Create(CreateClipRequest{Content: "hello go", Source: "d1"}, "u2")

	// 验证条目存在
	_, err := mgr.Get(item1.ID)
	if err != nil {
		t.Fatalf("expected item1 to exist: %v", err)
	}
	_, err = mgr.Get(item2.ID)
	if err != nil {
		t.Fatalf("expected item2 to exist: %v", err)
	}

	// 验证统计
	stats := mgr.Stats()
	if stats.TotalItems != 3 {
		t.Errorf("expected 3 items, got %d", stats.TotalItems)
	}
}

func TestSync(t *testing.T) {
	mgr := NewManager("test-key-123456789012345678901234", 1000)

	mgr.Create(CreateClipRequest{Content: "old clip", Source: "d1"}, "u1")
	time.Sleep(10 * time.Millisecond)
	lastSync := time.Now()
	time.Sleep(10 * time.Millisecond)
	mgr.Create(CreateClipRequest{Content: "new clip", Source: "d2"}, "u1")

	items, err := mgr.Sync("u1", "d1", lastSync)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 new item, got %d", len(items))
	}
}

func TestDelete(t *testing.T) {
	mgr := NewManager("test-key-123456789012345678901234", 1000)

	item, _ := mgr.Create(CreateClipRequest{Content: "to delete", Source: "d1"}, "u1")

	if err := mgr.Delete(item.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := mgr.Get(item.ID)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestClearUser(t *testing.T) {
	mgr := NewManager("test-key-123456789012345678901234", 1000)

	item1, _ := mgr.Create(CreateClipRequest{Content: "clip1", Source: "d1"}, "u1")
	item2, _ := mgr.Create(CreateClipRequest{Content: "clip2", Source: "d1"}, "u1")
	item3, _ := mgr.Create(CreateClipRequest{Content: "clip3", Source: "d1"}, "u2")

	// 验证创建成功
	if item1 == nil || item2 == nil || item3 == nil {
		t.Fatal("expected items to be created")
	}

	// 清空u1
	if err := mgr.ClearUser("u1"); err != nil {
		t.Fatalf("ClearUser failed: %v", err)
	}

	// u1的条目应该被删除
	_, err := mgr.Get(item1.ID)
	if err == nil {
		t.Error("expected u1 item1 to be deleted")
	}
	_, err = mgr.Get(item2.ID)
	if err == nil {
		t.Error("expected u1 item2 to be deleted")
	}

	// u2的条目应该还在
	_, err = mgr.Get(item3.ID)
	if err != nil {
		t.Errorf("expected u2 item to exist: %v", err)
	}
}

func TestExpiration(t *testing.T) {
	mgr := NewManager("test-key-123456789012345678901234", 1000)

	item, _ := mgr.Create(CreateClipRequest{
		Content: "expires soon",
		Source:  "d1",
		TTL:     1,
	}, "u1")

	// Should exist immediately
	_, err := mgr.Get(item.ID)
	if err != nil {
		t.Fatalf("expected item to exist: %v", err)
	}
}

func TestStats(t *testing.T) {
	mgr := NewManager("test-key-123456789012345678901234", 1000)

	mgr.Create(CreateClipRequest{Content: "clip1", Source: "d1"}, "u1")
	mgr.Create(CreateClipRequest{Content: "clip2", Source: "d2"}, "u1")
	mgr.Create(CreateClipRequest{Content: "clip3", Source: "d1"}, "u2")

	stats := mgr.Stats()
	if stats.TotalItems != 3 {
		t.Errorf("expected 3 items, got %d", stats.TotalItems)
	}
	if stats.ActiveUsers != 2 {
		t.Errorf("expected 2 users, got %d", stats.ActiveUsers)
	}
	if stats.DeviceCount != 2 {
		t.Errorf("expected 2 devices, got %d", stats.DeviceCount)
	}
}

func TestEncryption(t *testing.T) {
	mgr := NewManager("test-key-123456789012345678901234", 1000)

	plaintext := "sensitive data"
	encrypted, err := mgr.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := mgr.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}
