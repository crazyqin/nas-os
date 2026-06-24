package p2pwebshare

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())
	if m == nil {
		t.Fatal("Expected non-nil manager")
	}
	if m.baseURL != "https://test.example.com" {
		t.Errorf("Expected baseURL=https://test.example.com, got %s", m.baseURL)
	}
}

func TestNewManager_DefaultURL(t *testing.T) {
	m := NewManager("", nil)
	if m.baseURL != "https://share.example.com" {
		t.Errorf("Expected default baseURL, got %s", m.baseURL)
	}
}

func TestManager_CreateShareLink(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{
		FilePath:     "/data/test.pdf",
		Name:         "测试文档",
		Password:     "secret123",
		ExpireHours:  24,
		MaxDownloads: 10,
		Note:         "仅限内部使用",
	}

	link, err := m.CreateShareLink(req, "testuser")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if link.ID == "" {
		t.Error("Expected non-empty ID")
	}
	if link.Name != "测试文档" {
		t.Errorf("Expected Name=测试文档, got %s", link.Name)
	}
	if link.FilePath != "/data/test.pdf" {
		t.Errorf("Expected FilePath=/data/test.pdf, got %s", link.FilePath)
	}
	if !link.HasPassword {
		t.Error("Expected HasPassword=true")
	}
	if link.MaxDownloads != 10 {
		t.Errorf("Expected MaxDownloads=10, got %d", link.MaxDownloads)
	}
	if link.Note != "仅限内部使用" {
		t.Errorf("Expected Note=仅限内部使用, got %s", link.Note)
	}
	if link.CreatedBy != "testuser" {
		t.Errorf("Expected CreatedBy=testuser, got %s", link.CreatedBy)
	}
	if !link.Enabled {
		t.Error("Expected Enabled=true")
	}
	if link.ShareURL == "" {
		t.Error("Expected non-empty ShareURL")
	}
}

func TestManager_CreateShareLink_EmptyPath(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	_, err := m.CreateShareLink(ShareRequest{}, "testuser")
	if err == nil {
		t.Error("Expected error for empty file path")
	}
}

func TestManager_CreateShareLink_Defaults(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{
		FilePath: "/data/test.txt",
	}

	link, err := m.CreateShareLink(req, "testuser")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 默认 72 小时过期
	expectedExpiry := time.Now().Add(72 * time.Hour)
	if link.ExpiresAt.Before(expectedExpiry.Add(-time.Minute)) {
		t.Error("Expected default expiry ~72 hours")
	}

	// 默认不限制下载
	if link.MaxDownloads != 0 {
		t.Errorf("Expected MaxDownloads=0, got %d", link.MaxDownloads)
	}

	// 无密码
	if link.HasPassword {
		t.Error("Expected HasPassword=false")
	}

	// 文件名从路径提取
	if link.Name != "test.txt" {
		t.Errorf("Expected Name=test.txt, got %s", link.Name)
	}
}

func TestManager_GetShareLink(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{FilePath: "/data/test.pdf"}
	link, _ := m.CreateShareLink(req, "testuser")

	// 获取存在的链接
	got, err := m.GetShareLink(link.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if got.ID != link.ID {
		t.Errorf("Expected ID=%s, got %s", link.ID, got.ID)
	}
}

func TestManager_GetShareLink_NotFound(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	_, err := m.GetShareLink("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent link")
	}
}

func TestManager_GetShareLink_Disabled(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{FilePath: "/data/test.pdf"}
	link, _ := m.CreateShareLink(req, "testuser")

	m.DisableShareLink(link.ID)

	_, err := m.GetShareLink(link.ID)
	if err == nil {
		t.Error("Expected error for disabled link")
	}
}

func TestManager_VerifyPassword(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{
		FilePath: "/data/test.pdf",
		Password: "secret",
	}
	link, _ := m.CreateShareLink(req, "testuser")

	// 正确密码
	if !m.VerifyPassword(link.ID, "secret") {
		t.Error("Expected password verification to succeed")
	}

	// 错误密码
	if m.VerifyPassword(link.ID, "wrong") {
		t.Error("Expected password verification to fail")
	}
}

func TestManager_VerifyPassword_NoPassword(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{FilePath: "/data/test.pdf"}
	link, _ := m.CreateShareLink(req, "testuser")

	// 无密码时任何密码都通过
	if !m.VerifyPassword(link.ID, "anything") {
		t.Error("Expected verification to pass when no password set")
	}
}

func TestManager_RecordDownload(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{
		FilePath:     "/data/test.pdf",
		MaxDownloads: 3,
	}
	link, _ := m.CreateShareLink(req, "testuser")

	// 记录下载
	err := m.RecordDownload(link.ID, "192.168.1.1", "Mozilla/5.0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 验证下载计数
	got, _ := m.GetShareLink(link.ID)
	if got.DownloadCount != 1 {
		t.Errorf("Expected DownloadCount=1, got %d", got.DownloadCount)
	}
}

func TestManager_RecordDownload_MaxExceeded(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{
		FilePath:     "/data/test.pdf",
		MaxDownloads: 1,
	}
	link, _ := m.CreateShareLink(req, "testuser")

	// 第一次下载成功
	m.RecordDownload(link.ID, "192.168.1.1", "Mozilla/5.0")

	// 第二次下载应失败
	err := m.RecordDownload(link.ID, "192.168.1.2", "Mozilla/5.0")
	if err == nil {
		t.Error("Expected error when max downloads exceeded")
	}
}

func TestManager_DeleteShareLink(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{FilePath: "/data/test.pdf"}
	link, _ := m.CreateShareLink(req, "testuser")

	err := m.DeleteShareLink(link.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	_, err = m.GetShareLink(link.ID)
	if err == nil {
		t.Error("Expected error for deleted link")
	}
}

func TestManager_DeleteShareLink_NotFound(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	err := m.DeleteShareLink("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent link")
	}
}

func TestManager_DisableShareLink(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{FilePath: "/data/test.pdf"}
	link, _ := m.CreateShareLink(req, "testuser")

	err := m.DisableShareLink(link.ID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// 链接仍然存在但被禁用
	m.mu.RLock()
	stored := m.links[link.ID]
	m.mu.RUnlock()

	if stored.Enabled {
		t.Error("Expected link to be disabled")
	}
}

func TestManager_ListShareLinks(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	m.CreateShareLink(ShareRequest{FilePath: "/a.pdf"}, "user1")
	m.CreateShareLink(ShareRequest{FilePath: "/b.pdf"}, "user2")
	m.CreateShareLink(ShareRequest{FilePath: "/c.pdf"}, "user1")

	// 列出所有
	all := m.ListShareLinks("")
	if len(all) != 3 {
		t.Errorf("Expected 3 links, got %d", len(all))
	}

	// 按用户过滤
	user1Links := m.ListShareLinks("user1")
	if len(user1Links) != 2 {
		t.Errorf("Expected 2 links for user1, got %d", len(user1Links))
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	m.CreateShareLink(ShareRequest{FilePath: "/a.pdf"}, "user1")
	m.CreateShareLink(ShareRequest{FilePath: "/b.pdf"}, "user2")

	stats := m.GetStats()
	if stats.TotalLinks != 2 {
		t.Errorf("Expected TotalLinks=2, got %d", stats.TotalLinks)
	}
	if stats.ActiveLinks != 2 {
		t.Errorf("Expected ActiveLinks=2, got %d", stats.ActiveLinks)
	}
	if stats.UpdatedAt.IsZero() {
		t.Error("Expected non-zero UpdatedAt")
	}
}

func TestManager_GetDownloadLogs(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	req := ShareRequest{FilePath: "/data/test.pdf"}
	link, _ := m.CreateShareLink(req, "testuser")

	m.RecordDownload(link.ID, "192.168.1.1", "Mozilla/5.0")
	m.RecordDownload(link.ID, "192.168.1.2", "Chrome/100")

	logs := m.GetDownloadLogs(10)
	if len(logs) != 2 {
		t.Errorf("Expected 2 logs, got %d", len(logs))
	}

	if logs[0].ClientIP != "192.168.1.1" {
		t.Errorf("Expected first log IP=192.168.1.1, got %s", logs[0].ClientIP)
	}
}

func TestManager_CleanupExpired(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	// 创建一个已过期的链接
	req := ShareRequest{FilePath: "/data/test.pdf", ExpireHours: -1}
	m.CreateShareLink(req, "testuser")

	// 创建一个未过期的链接
	req2 := ShareRequest{FilePath: "/data/test2.pdf", ExpireHours: 24}
	m.CreateShareLink(req2, "testuser")

	count := m.CleanupExpired()
	if count != 1 {
		t.Errorf("Expected 1 expired link cleaned, got %d", count)
	}

	stats := m.GetStats()
	if stats.TotalLinks != 1 {
		t.Errorf("Expected 1 link remaining, got %d", stats.TotalLinks)
	}
}

func TestManager_RegisterRoutes(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())
	mux := http.NewServeMux()
	m.RegisterRoutes(mux)

	// 验证路由存在
	routes := []string{
		"/api/v1/p2p/share",
		"/api/v1/p2p/share/list",
		"/api/v1/p2p/share/stats",
		"/api/v1/p2p/share/logs",
		"/api/v1/p2p/share/delete",
	}

	for _, route := range routes {
		req := httptest.NewRequest(http.MethodGet, route, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
	}
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"photo.jpg", "image"},
		{"video.mp4", "video"},
		{"song.mp3", "audio"},
		{"doc.pdf", "pdf"},
		{"report.docx", "document"},
		{"data.xlsx", "spreadsheet"},
		{"archive.zip", "archive"},
		{"unknown.xyz", "file"},
	}

	for _, tt := range tests {
		result := detectFileType(tt.path)
		if result != tt.expected {
			t.Errorf("detectFileType(%s): expected %s, got %s", tt.path, tt.expected, result)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id1, err := generateID(16)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(id1) != 16 {
		t.Errorf("Expected ID length 16, got %d", len(id1))
	}

	id2, _ := generateID(16)
	if id1 == id2 {
		t.Error("Expected unique IDs")
	}
}

func TestShareLink_Fields(t *testing.T) {
	link := ShareLink{
		ID:           "test123",
		Name:         "test.pdf",
		FilePath:     "/data/test.pdf",
		FileSize:     1024,
		FileType:     "pdf",
		ShareURL:     "https://share.example.com/s/test123",
		HasPassword:  true,
		MaxDownloads: 5,
		DownloadCount: 2,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
		CreatedBy:    "user1",
		Enabled:      true,
		Note:         "测试",
	}

	if link.ID != "test123" {
		t.Errorf("Expected ID=test123, got %s", link.ID)
	}
	if !link.HasPassword {
		t.Error("Expected HasPassword=true")
	}
	if link.MaxDownloads != 5 {
		t.Errorf("Expected MaxDownloads=5, got %d", link.MaxDownloads)
	}
}

func TestShareStats_Fields(t *testing.T) {
	stats := ShareStats{
		TotalLinks:      10,
		ActiveLinks:     7,
		ExpiredLinks:    3,
		TotalDownloads:  50,
		TotalDataShared: 5 * 1024 * 1024 * 1024,
		UpdatedAt:       time.Now(),
	}

	if stats.TotalLinks != 10 {
		t.Errorf("Expected TotalLinks=10, got %d", stats.TotalLinks)
	}
	if stats.ActiveLinks != 7 {
		t.Errorf("Expected ActiveLinks=7, got %d", stats.ActiveLinks)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	m := NewManager("https://test.example.com", slog.Default())

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			m.CreateShareLink(ShareRequest{FilePath: "/test.pdf"}, "user")
			m.GetStats()
			m.ListShareLinks("")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent access")
		}
	}
}
