package sharelinks

import (
	"testing"
	"time"
)

func TestNewLinkManager(t *testing.T) {
	m := NewLinkManager(nil)
	if m == nil {
		t.Fatal("NewLinkManager returned nil")
	}
	if m.config == nil {
		t.Fatal("config should not be nil")
	}
}

func TestCreateLink(t *testing.T) {
	m := NewLinkManager(nil)

	link, err := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if link.Path != "/data/file.txt" {
		t.Errorf("expected path '/data/file.txt', got '%s'", link.Path)
	}
	if link.Name != "file.txt" {
		t.Errorf("expected name 'file.txt', got '%s'", link.Name)
	}
	if link.Type != LinkTypePublic {
		t.Errorf("expected type 'public', got '%s'", link.Type)
	}
	if link.ShortCode == "" {
		t.Error("short code should not be empty")
	}
	if !link.IsActive {
		t.Error("expected link to be active")
	}
	if link.ExpiresAt == nil {
		t.Error("expected ExpiresAt to be set")
	}
}

func TestCreateLinkWithPassword(t *testing.T) {
	m := NewLinkManager(nil)

	link, err := m.CreateLink("/data/secret.txt", "secret.txt", "admin", LinkTypeEncrypted,
		WithPassword("mypassword"),
	)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if link.Type != LinkTypeEncrypted {
		t.Errorf("expected type 'encrypted', got '%s'", link.Type)
	}
	if link.Password == "" {
		t.Error("password hash should not be empty")
	}
	if link.Password == "mypassword" {
		t.Error("password should be hashed")
	}
}

func TestCreateLinkWithExpiry(t *testing.T) {
	m := NewLinkManager(nil)

	link, err := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic,
		WithExpiry(48),
	)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if link.ExpiresAt == nil {
		t.Fatal("ExpiresAt should not be nil")
	}

	expectedExpiry := time.Now().Add(48 * time.Hour)
	if link.ExpiresAt.Sub(expectedExpiry).Abs() > time.Minute {
		t.Errorf("expiry time mismatch: expected ~%v, got %v", expectedExpiry, link.ExpiresAt)
	}
}

func TestCreateLinkWithMaxDownloads(t *testing.T) {
	m := NewLinkManager(nil)

	link, err := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic,
		WithMaxDownloads(10),
	)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if link.MaxDownloads != 10 {
		t.Errorf("expected MaxDownloads 10, got %d", link.MaxDownloads)
	}
}

func TestCreateBatchLink(t *testing.T) {
	m := NewLinkManager(nil)

	paths := []string{"/file1.txt", "/file2.txt", "/file3.txt"}
	link, err := m.CreateLink("/data/batch", "batch.zip", "admin", LinkTypePublic,
		WithBatchPaths(paths),
	)
	if err != nil {
		t.Fatalf("CreateLink failed: %v", err)
	}

	if !link.IsBatch {
		t.Error("expected IsBatch to be true")
	}
	if len(link.BatchPaths) != 3 {
		t.Errorf("expected 3 batch paths, got %d", len(link.BatchPaths))
	}
}

func TestGetLink(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	got, ok := m.GetLink(link.ID)
	if !ok {
		t.Fatal("GetLink returned false")
	}
	if got.ID != link.ID {
		t.Errorf("expected ID '%s', got '%s'", link.ID, got.ID)
	}
}

func TestGetLinkByShortCode(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	got, ok := m.GetLinkByShortCode(link.ShortCode)
	if !ok {
		t.Fatal("GetLinkByShortCode returned false")
	}
	if got.ID != link.ID {
		t.Errorf("expected ID '%s', got '%s'", link.ID, got.ID)
	}
}

func TestGetLinkByToken(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	got, ok := m.GetLinkByToken(link.Token)
	if !ok {
		t.Fatal("GetLinkByToken returned false")
	}
	if got.ID != link.ID {
		t.Errorf("expected ID '%s', got '%s'", link.ID, got.ID)
	}
}

func TestListLinks(t *testing.T) {
	m := NewLinkManager(nil)
	m.CreateLink("/a.txt", "a.txt", "user1", LinkTypePublic)
	m.CreateLink("/b.txt", "b.txt", "user2", LinkTypePrivate)
	m.CreateLink("/c.txt", "c.txt", "user1", LinkTypePublic)

	// 全部
	all := m.ListLinks("", false)
	if len(all) != 3 {
		t.Errorf("expected 3 links, got %d", len(all))
	}

	// 按用户
	user1 := m.ListLinks("user1", false)
	if len(user1) != 2 {
		t.Errorf("expected 2 links for user1, got %d", len(user1))
	}
}

func TestUpdateLink(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	newName := "updated.txt"
	updated, err := m.UpdateLink(link.ID, func(l *ShareLink) { l.Name = newName })
	if err != nil {
		t.Fatalf("UpdateLink failed: %v", err)
	}

	if updated.Name != "updated.txt" {
		t.Errorf("expected name 'updated.txt', got '%s'", updated.Name)
	}
	if updated.UpdatedAt.Before(link.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt")
	}
}

func TestDisableEnableLink(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	// 禁用
	err := m.DisableLink(link.ID)
	if err != nil {
		t.Fatalf("DisableLink failed: %v", err)
	}

	got, _ := m.GetLink(link.ID)
	if got.IsActive {
		t.Error("expected link to be disabled")
	}

	// 启用
	err = m.EnableLink(link.ID)
	if err != nil {
		t.Fatalf("EnableLink failed: %v", err)
	}

	got, _ = m.GetLink(link.ID)
	if !got.IsActive {
		t.Error("expected link to be enabled")
	}
}

func TestDeleteLink(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)
	shortCode := link.ShortCode

	err := m.DeleteLink(link.ID)
	if err != nil {
		t.Fatalf("DeleteLink failed: %v", err)
	}

	_, ok := m.GetLink(link.ID)
	if ok {
		t.Error("expected link to be deleted")
	}

	_, ok = m.GetLinkByShortCode(shortCode)
	if ok {
		t.Error("expected short code mapping to be deleted")
	}
}

func TestValidateAccess(t *testing.T) {
	m := NewLinkManager(nil)

	// 公开链接
	publicLink, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	_, err := m.ValidateAccess(publicLink.ID, "", "")
	if err != nil {
		t.Errorf("public link should be accessible: %v", err)
	}

	// 加密链接
	encryptedLink, _ := m.CreateLink("/data/secret.txt", "secret.txt", "admin", LinkTypeEncrypted,
		WithPassword("mypassword"),
	)

	// 无密码
	_, err = m.ValidateAccess(encryptedLink.ID, "", "")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}

	// 错误密码
	_, err = m.ValidateAccess(encryptedLink.ID, "wrong", "")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}

	// 正确密码
	_, err = m.ValidateAccess(encryptedLink.ID, "mypassword", "")
	if err != nil {
		t.Errorf("correct password should work: %v", err)
	}
}

func TestValidateAccessExpired(t *testing.T) {
	m := NewLinkManager(nil)

	expired := time.Now().Add(-1 * time.Hour)
	link := &ShareLink{
		ID:        "expired-1",
		Path:      "/data/old.txt",
		Token:     "token123",
		ShortCode: "abc123",
		IsActive:  true,
		ExpiresAt: &expired,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		AccessLog: make([]AccessEntry, 0),
	}
	m.links[link.ID] = link
	m.shortCodes[link.ShortCode] = link

	_, err := m.ValidateAccess(link.ID, "", "")
	if err != ErrLinkExpired {
		t.Errorf("expected ErrLinkExpired, got %v", err)
	}
}

func TestValidateAccessDownloadLimit(t *testing.T) {
	m := NewLinkManager(nil)

	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic,
		WithMaxDownloads(2),
	)

	// 模拟2次下载
	link.DownloadCount = 2

	_, err := m.ValidateAccess(link.ID, "", "")
	if err != ErrDownloadLimit {
		t.Errorf("expected ErrDownloadLimit, got %v", err)
	}
}

func TestValidateAccessRefererWhitelist(t *testing.T) {
	m := NewLinkManager(nil)

	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic,
		WithRefererWhitelist([]string{"https://example.com", "https://trusted.org"}),
	)

	// 允许的Referer
	_, err := m.ValidateAccess(link.ID, "", "https://example.com/page")
	if err != nil {
		t.Errorf("allowed referer should work: %v", err)
	}

	// 禁止的Referer
	_, err = m.ValidateAccess(link.ID, "", "https://evil.com/attack")
	if err != ErrRefererDenied {
		t.Errorf("expected ErrRefererDenied, got %v", err)
	}

	// 无Referer（允许）
	_, err = m.ValidateAccess(link.ID, "", "")
	if err != nil {
		t.Errorf("empty referer should work: %v", err)
	}
}

func TestRecordAccess(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	m.RecordAccess(link.ID, "192.168.1.1", "Mozilla/5.0", "https://example.com", "download")

	got, _ := m.GetLink(link.ID)
	if len(got.AccessLog) != 1 {
		t.Errorf("expected 1 access log entry, got %d", len(got.AccessLog))
	}
	if got.DownloadCount != 1 {
		t.Errorf("expected download count 1, got %d", got.DownloadCount)
	}
	if got.UniqueVisitors != 1 {
		t.Errorf("expected 1 unique visitor, got %d", got.UniqueVisitors)
	}
	if got.LastAccessedAt == nil {
		t.Error("LastAccessedAt should be set")
	}
}

func TestRecordAccessMultipleIPs(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	m.RecordAccess(link.ID, "192.168.1.1", "Mozilla/5.0", "", "view")
	m.RecordAccess(link.ID, "192.168.1.2", "Chrome/90", "", "view")
	m.RecordAccess(link.ID, "192.168.1.1", "Mozilla/5.0", "", "download")

	got, _ := m.GetLink(link.ID)
	if got.UniqueVisitors != 2 {
		t.Errorf("expected 2 unique visitors, got %d", got.UniqueVisitors)
	}
	if got.DownloadCount != 1 {
		t.Errorf("expected download count 1, got %d", got.DownloadCount)
	}
}

func TestGetLinkStats(t *testing.T) {
	m := NewLinkManager(nil)
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	m.RecordAccess(link.ID, "192.168.1.1", "Mozilla/5.0", "", "view")
	m.RecordAccess(link.ID, "192.168.1.2", "Chrome/90", "", "download")

	stats, err := m.GetLinkStats(link.ID)
	if err != nil {
		t.Fatalf("GetLinkStats failed: %v", err)
	}

	if stats.TotalLinks != 1 {
		t.Errorf("expected 1 total link, got %d", stats.TotalLinks)
	}
	if stats.ActiveLinks != 1 {
		t.Errorf("expected 1 active link, got %d", stats.ActiveLinks)
	}
	if stats.TotalDownloads != 1 {
		t.Errorf("expected 1 download, got %d", stats.TotalDownloads)
	}
	if stats.TotalViews != 2 {
		t.Errorf("expected 2 views, got %d", stats.TotalViews)
	}
}

func TestGetGlobalStats(t *testing.T) {
	m := NewLinkManager(nil)
	m.CreateLink("/a.txt", "a.txt", "user1", LinkTypePublic)
	m.CreateLink("/b.txt", "b.txt", "user1", LinkTypePublic)
	m.CreateLink("/c.txt", "c.txt", "user2", LinkTypePrivate)

	stats := m.GetGlobalStats()
	if stats.TotalLinks != 3 {
		t.Errorf("expected 3 total links, got %d", stats.TotalLinks)
	}
	if stats.ActiveLinks != 3 {
		t.Errorf("expected 3 active links, got %d", stats.ActiveLinks)
	}
}

func TestCleanupExpired(t *testing.T) {
	m := NewLinkManager(nil)

	// 创建一个已过期的链接
	expired := time.Now().Add(-1 * time.Hour)
	link := &ShareLink{
		ID:        "expired-1",
		Path:      "/data/old.txt",
		Token:     "token123",
		ShortCode: "exp001",
		IsActive:  true,
		ExpiresAt: &expired,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		AccessLog: make([]AccessEntry, 0),
	}
	m.links[link.ID] = link
	m.shortCodes[link.ShortCode] = link

	// 创建一个未过期的链接
	m.CreateLink("/data/new.txt", "new.txt", "admin", LinkTypePublic)

	count := m.CleanupExpired()
	if count != 1 {
		t.Errorf("expected 1 expired link cleaned, got %d", count)
	}

	got, _ := m.GetLink("expired-1")
	if got.IsActive {
		t.Error("expected expired link to be deactivated")
	}
}

func TestGenerateQRCodeData(t *testing.T) {
	m := NewLinkManager(&LinkConfig{
		BaseURL: "https://nas.example.com",
	})
	link, _ := m.CreateLink("/data/file.txt", "file.txt", "admin", LinkTypePublic)

	data, err := m.GenerateQRCodeData(link.ID)
	if err != nil {
		t.Fatalf("GenerateQRCodeData failed: %v", err)
	}

	expected := "https://nas.example.com/s/" + link.ShortCode
	if data != expected {
		t.Errorf("expected '%s', got '%s'", expected, data)
	}
}

func TestPreviewTypeDetection(t *testing.T) {
	tests := []struct {
		path     string
		expected PreviewType
	}{
		{"image.jpg", PreviewTypeImage},
		{"photo.png", PreviewTypeImage},
		{"document.pdf", PreviewTypeDocument},
		{"text.txt", PreviewTypeDocument},
		{"video.mp4", PreviewTypeVideo},
		{"audio.mp3", PreviewTypeAudio},
		{"data.zip", PreviewTypeNone},
	}

	for _, tt := range tests {
		result := detectPreviewType(tt.path)
		if result != tt.expected {
			t.Errorf("path '%s': expected %s, got %s", tt.path, tt.expected, result)
		}
	}
}

func TestBase62Encoding(t *testing.T) {
	tests := []struct {
		input    []byte
		expected string
	}{
		{[]byte{0}, ""},
		{[]byte{1}, "1"},
		{[]byte{62}, "10"},
	}

	for _, tt := range tests {
		result, err := encodeBase62(tt.input)
		if err != nil {
			t.Errorf("encodeBase62(%v) error: %v", tt.input, err)
			continue
		}
		if tt.input[0] == 0 && result != "" {
			// 0输入应该返回空
			t.Errorf("expected empty string for zero input, got '%s'", result)
		}
	}
}
