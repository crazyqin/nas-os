package websharepro

import (
	"testing"
	"time"
)

func TestNewWebShareManager(t *testing.T) {
	m := NewWebShareManager(nil)
	if m == nil {
		t.Fatal("NewWebShareManager returned nil")
	}
}

func TestCreateShareLink(t *testing.T) {
	m := NewWebShareManager(nil)

	link, err := m.CreateShareLink("/data/file.txt", "file.txt", "admin", PermReadOnly, 24, "")
	if err != nil {
		t.Fatalf("CreateShareLink failed: %v", err)
	}

	if link.Path != "/data/file.txt" {
		t.Errorf("expected path '/data/file.txt', got '%s'", link.Path)
	}
	if link.Permission != PermReadOnly {
		t.Errorf("expected readonly permission, got %s", link.Permission)
	}
	if link.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
	if !link.IsActive {
		t.Error("expected link to be active")
	}
}

func TestGetShareLinkByToken(t *testing.T) {
	m := NewWebShareManager(nil)
	link, _ := m.CreateShareLink("/data/file.txt", "file.txt", "admin", PermReadOnly, 24, "")

	found, ok := m.GetShareLinkByToken(link.Token)
	if !ok {
		t.Fatal("GetShareLinkByToken returned false")
	}
	if found.ID != link.ID {
		t.Errorf("expected ID '%s', got '%s'", link.ID, found.ID)
	}
}

func TestListShareLinks(t *testing.T) {
	m := NewWebShareManager(nil)
	m.CreateShareLink("/a.txt", "a.txt", "user1", PermReadOnly, 24, "")
	m.CreateShareLink("/b.txt", "b.txt", "user2", PermReadWrite, 24, "")
	m.CreateShareLink("/c.txt", "c.txt", "user1", PermReadOnly, 24, "")

	// 全部
	all := m.ListShareLinks("", false)
	if len(all) != 3 {
		t.Errorf("expected 3 links, got %d", len(all))
	}

	// 按用户
	user1 := m.ListShareLinks("user1", false)
	if len(user1) != 2 {
		t.Errorf("expected 2 links for user1, got %d", len(user1))
	}
}

func TestDeleteShareLink(t *testing.T) {
	m := NewWebShareManager(nil)
	link, _ := m.CreateShareLink("/data/file.txt", "file.txt", "admin", PermReadOnly, 24, "")

	err := m.DeleteShareLink(link.ID)
	if err != nil {
		t.Fatalf("DeleteShareLink failed: %v", err)
	}

	got, _ := m.GetShareLink(link.ID)
	if got.IsActive {
		t.Error("expected link to be inactive after deletion")
	}
}

func TestRecordAccess(t *testing.T) {
	m := NewWebShareManager(nil)
	link, _ := m.CreateShareLink("/data/file.txt", "file.txt", "admin", PermReadOnly, 24, "")

	m.RecordAccess(link.ID, "192.168.1.1", "Mozilla/5.0", "download")

	got, _ := m.GetShareLink(link.ID)
	if len(got.AccessLog) != 1 {
		t.Errorf("expected 1 access log entry, got %d", len(got.AccessLog))
	}
	if got.DownloadCount != 1 {
		t.Errorf("expected download count 1, got %d", got.DownloadCount)
	}
}

func TestGetStats(t *testing.T) {
	m := NewWebShareManager(nil)
	m.CreateShareLink("/a.txt", "a.txt", "user1", PermReadOnly, 24, "")
	m.CreateShareLink("/b.txt", "b.txt", "user1", PermReadOnly, 24, "")

	stats := m.GetStats()
	if stats.TotalLinks != 2 {
		t.Errorf("expected 2 total links, got %d", stats.TotalLinks)
	}
	if stats.ActiveLinks != 2 {
		t.Errorf("expected 2 active links, got %d", stats.ActiveLinks)
	}
}

func TestCleanupExpired(t *testing.T) {
	m := NewWebShareManager(nil)

	// 创建一个已过期的链接
	expired := time.Now().Add(-1 * time.Hour)
	link := &ShareLink{
		ID:        "expired-1",
		Path:      "/data/old.txt",
		Token:     "token123",
		IsActive:  true,
		ExpiresAt: &expired,
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	m.links[link.ID] = link

	count := m.CleanupExpired()
	if count != 1 {
		t.Errorf("expected 1 expired link cleaned, got %d", count)
	}

	got, _ := m.GetShareLink("expired-1")
	if got.IsActive {
		t.Error("expected expired link to be deactivated")
	}
}
