// Package smartshare 单元测试
package smartshare

import (
	"testing"
	"time"
)

func TestDefaultSharePolicy(t *testing.T) {
	policy := DefaultSharePolicy()

	if policy == nil {
		t.Fatal("DefaultSharePolicy returned nil")
	}

	if policy.ID != "policy-default" {
		t.Errorf("expected ID 'policy-default', got '%s'", policy.ID)
	}

	if policy.DefaultMode != ShareModePassword {
		t.Errorf("expected default mode 'password', got '%s'", policy.DefaultMode)
	}

	if policy.DefaultExpiration != 7*24*time.Hour {
		t.Errorf("expected default expiration 7 days, got %v", policy.DefaultExpiration)
	}

	if policy.MaxFileSize != 10*1024*1024*1024 {
		t.Errorf("expected max file size 10GB, got %d", policy.MaxFileSize)
	}
}

func TestDefaultWatermarkConfig(t *testing.T) {
	config := DefaultWatermarkConfig()

	if config == nil {
		t.Fatal("DefaultWatermarkConfig returned nil")
	}

	if config.FontSize != 14 {
		t.Errorf("expected font size 14, got %d", config.FontSize)
	}

	if config.Opacity != 0.3 {
		t.Errorf("expected opacity 0.3, got %f", config.Opacity)
	}

	if config.Position != WatermarkTiled {
		t.Errorf("expected position 'tiled', got '%s'", config.Position)
	}
}

func TestDefaultBrandingConfig(t *testing.T) {
	config := DefaultBrandingConfig()

	if config == nil {
		t.Fatal("DefaultBrandingConfig returned nil")
	}

	if config.CompanyName != "NAS-OS" {
		t.Errorf("expected company name 'NAS-OS', got '%s'", config.CompanyName)
	}

	if config.PrimaryColor != "#1890ff" {
		t.Errorf("expected primary color '#1890ff', got '%s'", config.PrimaryColor)
	}
}

func TestGenerator(t *testing.T) {
	gen := NewGenerator(nil)

	// Test token generation
	token1 := gen.GenerateToken()
	token2 := gen.GenerateToken()

	if token1 == "" {
		t.Error("GenerateToken returned empty string")
	}

	if token1 == token2 {
		t.Error("GenerateToken should generate unique tokens")
	}

	// Test short code generation
	code1 := gen.GenerateShortCode()
	code2 := gen.GenerateShortCode()

	if code1 == "" {
		t.Error("GenerateShortCode returned empty string")
	}

	if code1 == code2 {
		t.Error("GenerateShortCode should generate unique codes")
	}

	if len(code1) > 8 {
		t.Errorf("short code too long: %d chars", len(code1))
	}

	// Test QR code generation
	qr := gen.GenerateQRCode(token1)
	if qr == nil {
		t.Fatal("GenerateQRCode returned nil")
	}

	if qr.Token != token1 {
		t.Errorf("expected token '%s', got '%s'", token1, qr.Token)
	}

	if qr.ShortURL == "" {
		t.Error("QR code short URL is empty")
	}

	if qr.QRCodeData == "" {
		t.Error("QR code data is empty")
	}
}

func TestAccessController(t *testing.T) {
	ac := NewAccessController(nil)

	// Test with active link
	link := &ShareLink{
		ID:     "test-1",
		Status: ShareStatusActive,
		Mode:   ShareModePublic,
	}

	req := &AccessRequest{
		ShareID:   "test-1",
		Token:     "token-1",
		IPAddress: "192.168.1.100",
		Action:    "view",
	}

	result := ac.CheckAccess(req, link)
	if !result.Allowed {
		t.Errorf("expected access allowed, got denied: %s", result.Reason)
	}

	// Test with expired link
	expiredLink := &ShareLink{
		ID:        "test-2",
		Status:    ShareStatusActive,
		Mode:      ShareModePublic,
		ExpiresAt: timePtr(time.Now().Add(-1 * time.Hour)),
	}

	result = ac.CheckAccess(req, expiredLink)
	if result.Allowed {
		t.Error("expected access denied for expired link")
	}

	// Test with revoked link
	revokedLink := &ShareLink{
		ID:     "test-3",
		Status: ShareStatusRevoked,
		Mode:   ShareModePublic,
	}

	result = ac.CheckAccess(req, revokedLink)
	if result.Allowed {
		t.Error("expected access denied for revoked link")
	}

	// Test with password-protected link
	passwordLink := &ShareLink{
		ID:       "test-4",
		Status:   ShareStatusActive,
		Mode:     ShareModePassword,
		Password: "secret123",
	}

	// Without password
	result = ac.CheckAccess(req, passwordLink)
	if result.Allowed {
		t.Error("expected access denied without password")
	}

	// With wrong password
	req.Password = "wrong"
	result = ac.CheckAccess(req, passwordLink)
	if result.Allowed {
		t.Error("expected access denied with wrong password")
	}

	// With correct password
	req.Password = "secret123"
	result = ac.CheckAccess(req, passwordLink)
	if !result.Allowed {
		t.Errorf("expected access allowed with correct password, got denied: %s", result.Reason)
	}
}

func TestAccessControllerIPWhitelist(t *testing.T) {
	ac := NewAccessController(nil)

	link := &ShareLink{
		ID:          "test-ip",
		Status:      ShareStatusActive,
		Mode:        ShareModePublic,
		IPWhitelist: []string{"192.168.1.0/24", "10.0.0.1"},
	}

	// IP in CIDR range
	req := &AccessRequest{
		ShareID:   "test-ip",
		IPAddress: "192.168.1.100",
		Action:    "view",
	}
	result := ac.CheckAccess(req, link)
	if !result.Allowed {
		t.Errorf("expected access allowed for IP in CIDR range, got denied: %s", result.Reason)
	}

	// IP in exact match
	req.IPAddress = "10.0.0.1"
	result = ac.CheckAccess(req, link)
	if !result.Allowed {
		t.Errorf("expected access allowed for exact IP match, got denied: %s", result.Reason)
	}

	// IP not in whitelist
	req.IPAddress = "172.16.0.1"
	result = ac.CheckAccess(req, link)
	if result.Allowed {
		t.Error("expected access denied for IP not in whitelist")
	}
}

func TestAccessControllerDownloadLimit(t *testing.T) {
	ac := NewAccessController(nil)

	link := &ShareLink{
		ID:            "test-dl",
		Status:        ShareStatusActive,
		Mode:          ShareModePublic,
		MaxDownloads:  2,
		DownloadCount: 2,
	}

	req := &AccessRequest{
		ShareID:   "test-dl",
		IPAddress: "192.168.1.100",
		Action:    "download",
	}

	result := ac.CheckAccess(req, link)
	if result.Allowed {
		t.Error("expected access denied when download limit reached")
	}
}

func TestAnalyticsEngine(t *testing.T) {
	ae := NewAnalyticsEngine(nil)

	// Record some access logs
	logs := []*AccessLog{
		{
			ID:         "log-1",
			ShareID:    "share-1",
			IPAddress:  "192.168.1.1",
			DeviceType: DeviceDesktop,
			OS:         "Windows",
			Browser:    "Chrome",
			Country:    "China",
			Region:     "Shanghai",
			Action:     "view",
			Timestamp:  time.Now(),
			Duration:   10000,
		},
		{
			ID:         "log-2",
			ShareID:    "share-1",
			IPAddress:  "192.168.1.2",
			DeviceType: DeviceMobile,
			OS:         "Android",
			Browser:    "Chrome",
			Country:    "China",
			Region:     "Beijing",
			Action:     "download",
			Timestamp:  time.Now(),
			Duration:   5000,
		},
		{
			ID:         "log-3",
			ShareID:    "share-1",
			IPAddress:  "192.168.1.1",
			DeviceType: DeviceDesktop,
			OS:         "Windows",
			Browser:    "Chrome",
			Country:    "China",
			Region:     "Shanghai",
			Action:     "view",
			Timestamp:  time.Now(),
			Duration:   3000,
		},
	}

	for _, log := range logs {
		ae.RecordAccess(log)
	}

	analytics := ae.GetAnalytics("share-1")

	if analytics.TotalViews != 2 {
		t.Errorf("expected 2 total views, got %d", analytics.TotalViews)
	}

	if analytics.TotalDownloads != 1 {
		t.Errorf("expected 1 total download, got %d", analytics.TotalDownloads)
	}

	if analytics.UniqueVisitors != 2 {
		t.Errorf("expected 2 unique visitors, got %d", analytics.UniqueVisitors)
	}

	if analytics.UniqueDownloaders != 1 {
		t.Errorf("expected 1 unique downloader, got %d", analytics.UniqueDownloaders)
	}

	if analytics.DeviceBreakdown[DeviceDesktop] != 2 {
		t.Errorf("expected 2 desktop devices, got %d", analytics.DeviceBreakdown[DeviceDesktop])
	}

	if analytics.DeviceBreakdown[DeviceMobile] != 1 {
		t.Errorf("expected 1 mobile device, got %d", analytics.DeviceBreakdown[DeviceMobile])
	}

	if analytics.CountryBreakdown["China"] != 3 {
		t.Errorf("expected 3 visits from China, got %d", analytics.CountryBreakdown["China"])
	}
}

func TestDetectUserAgent(t *testing.T) {
	tests := []struct {
		name       string
		ua         string
		deviceType DeviceType
		os         string
		browser    string
	}{
		{
			name:       "Windows Chrome",
			ua:         "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			deviceType: DeviceDesktop,
			os:         "Windows",
			browser:    "Chrome",
		},
		{
			name:       "macOS Safari",
			ua:         "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.1.1 Safari/605.1.15",
			deviceType: DeviceDesktop,
			os:         "macOS",
			browser:    "Safari",
		},
		{
			name:       "Android Chrome",
			ua:         "Mozilla/5.0 (Linux; Android 11; Pixel 5) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/90.0.4430.91 Mobile Safari/537.36",
			deviceType: DeviceMobile,
			os:         "Android",
			browser:    "Chrome",
		},
		{
			name:       "iOS Safari",
			ua:         "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			deviceType: DeviceMobile,
			os:         "iOS",
			browser:    "Safari",
		},
		{
			name:       "Bot",
			ua:         "Googlebot/2.1 (+http://www.google.com/bot.html)",
			deviceType: DeviceBot,
			os:         "Unknown",
			browser:    "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deviceType, os, browser := DetectUserAgent(tt.ua)

			if deviceType != tt.deviceType {
				t.Errorf("expected device type '%s', got '%s'", tt.deviceType, deviceType)
			}

			if os != tt.os {
				t.Errorf("expected OS '%s', got '%s'", tt.os, os)
			}

			if browser != tt.browser {
				t.Errorf("expected browser '%s', got '%s'", tt.browser, browser)
			}
		})
	}
}

func TestWatermarkEngine(t *testing.T) {
	we := NewWatermarkEngine(nil)

	// Test with default config
	req := &WatermarkRequest{
		FilePath:  "/test/file.txt",
		Timestamp: true,
		UserInfo: &UserInfo{
			UserID:   "user-1",
			Username: "testuser",
			IP:       "192.168.1.100",
		},
	}

	result, err := we.AddDynamicWatermark(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("result is nil")
	}

	if result.Format != "png" {
		t.Errorf("expected format 'png', got '%s'", result.Format)
	}
}

func TestWatermarkEngineValidation(t *testing.T) {
	we := NewWatermarkEngine(nil)

	// Test with invalid font size
	req := &WatermarkRequest{
		FilePath: "/test/file.txt",
		Config: &WatermarkConfig{
			FontSize: 0,
			Opacity:  0.5,
			Position: WatermarkCenter,
		},
	}

	_, err := we.AddTextWatermark(req)
	if err == nil {
		t.Error("expected error for invalid font size")
	}

	// Test with invalid opacity
	req.Config.FontSize = 14
	req.Config.Opacity = 1.5

	_, err = we.AddTextWatermark(req)
	if err == nil {
		t.Error("expected error for invalid opacity")
	}
}

func TestPreviewEngine(t *testing.T) {
	pe := NewPreviewEngine(nil, nil)

	tests := []struct {
		name     string
		filePath string
		expected FileType
	}{
		{"Image", "photo.jpg", FileTypeImage},
		{"Video", "movie.mp4", FileTypeVideo},
		{"Audio", "song.mp3", FileTypeAudio},
		{"PDF", "document.pdf", FileTypePDF},
		{"Office", "report.docx", FileTypeOffice},
		{"Text", "readme.txt", FileTypeText},
		{"Code", "main.go", FileTypeCode},
		{"Unknown", "data.xyz", FileTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileType := pe.detectFileType(tt.filePath)
			if fileType != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, fileType)
			}
		})
	}
}

func TestPreviewEngineCreatePreview(t *testing.T) {
	pe := NewPreviewEngine(nil, nil)

	req := &PreviewRequest{
		FilePath:  "document.pdf",
		ShareID:   "share-1",
		IP:        "192.168.1.100",
		Quality:   80,
		Watermark: true,
	}

	resp, err := pe.CreatePreview(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if resp.FileType != FileTypePDF {
		t.Errorf("expected file type 'pdf', got '%s'", resp.FileType)
	}

	if resp.SecurityInfo == nil {
		t.Fatal("security info is nil")
	}

	// Default config should not allow download
	if resp.SecurityInfo.AllowDownload {
		t.Error("expected download not allowed by default")
	}
}

func TestNotifier(t *testing.T) {
	n := NewNotifier(nil, nil)

	// Test sending event
	event := &NotifyEvent{
		ID:        "event-1",
		ShareID:   "share-1",
		EventType: "download",
		Level:     AlertLevelInfo,
		Title:     "文件被下载",
		Message:   "文件 test.txt 被下载",
		Timestamp: time.Now(),
	}

	n.SendEvent(event)

	events := n.GetEvents(10)
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}

	if events[0].ID != "event-1" {
		t.Errorf("expected event ID 'event-1', got '%s'", events[0].ID)
	}
}

func TestNotifierAnomalyDetection(t *testing.T) {
	n := NewNotifier(nil, nil)

	// Test suspicious UA detection
	log := &AccessLog{
		ShareID:   "share-1",
		IPAddress: "192.168.1.100",
		UserAgent: "curl/7.68.0",
		Timestamp: time.Now(),
	}

	event := n.DetectAnomaly(log, nil)
	if event == nil {
		t.Error("expected anomaly event for curl user agent")
	}

	// Test normal UA
	log.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0"
	event = n.DetectAnomaly(log, nil)
	// Should not detect anomaly during normal hours
	// (depends on current time, may or may not trigger)
}

func TestBrandingEngine(t *testing.T) {
	be := NewBrandingEngine(nil)

	config := &BrandingConfig{
		CompanyName:     "Test Corp",
		PrimaryColor:    "#ff0000",
		SecondaryColor:  "#00ff00",
		BackgroundColor: "#ffffff",
		TextColor:       "#333333",
		FooterText:      "Test Footer",
	}

	// Create branding
	created := be.CreateBranding("brand-1", config)
	if created == nil {
		t.Fatal("CreateBranding returned nil")
	}

	// Get branding
	retrieved, err := be.GetBranding("brand-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.CompanyName != "Test Corp" {
		t.Errorf("expected company name 'Test Corp', got '%s'", retrieved.CompanyName)
	}

	// Generate branded page
	req := &BrandingRequest{
		ShareID: "share-1",
		Config:  config,
	}

	resp, err := be.GenerateBrandedPage(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("response is nil")
	}

	if resp.HTML == "" {
		t.Error("HTML is empty")
	}

	if resp.CSS == "" {
		t.Error("CSS is empty")
	}

	if resp.FullPage == "" {
		t.Error("FullPage is empty")
	}

	// Check that company name appears in HTML
	if !contains(resp.HTML, "Test Corp") {
		t.Error("company name not found in HTML")
	}

	// Delete branding
	err = be.DeleteBranding("brand-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Try to get deleted branding
	_, err = be.GetBranding("brand-1")
	if err == nil {
		t.Error("expected error for deleted branding")
	}
}

func TestManager(t *testing.T) {
	m := NewManager(nil, nil)

	// Create share link
	req := &CreateShareRequest{
		FilePath:      "/data/test.txt",
		FileName:      "test.txt",
		FileSize:      1024,
		FileType:      "text/plain",
		Mode:          ShareModePublic,
		CreatorID:     "user-1",
		CreatorName:   "Test User",
		Description:   "Test share",
		Tags:          []string{"test", "share"},
		EnablePreview: true,
	}

	link, err := m.CreateShareLink(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if link == nil {
		t.Fatal("link is nil")
	}

	if link.Status != ShareStatusActive {
		t.Errorf("expected status 'active', got '%s'", link.Status)
	}

	if link.FileName != "test.txt" {
		t.Errorf("expected file name 'test.txt', got '%s'", link.FileName)
	}

	// Get share link by ID
	retrieved, err := m.GetShareLink(link.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != link.ID {
		t.Errorf("expected ID '%s', got '%s'", link.ID, retrieved.ID)
	}

	// Get share link by token
	retrieved, err = m.GetShareLinkByToken(link.Token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if retrieved.ID != link.ID {
		t.Errorf("expected ID '%s', got '%s'", link.ID, retrieved.ID)
	}

	// List share links
	links := m.ListShareLinks(nil)
	if len(links) != 1 {
		t.Errorf("expected 1 link, got %d", len(links))
	}

	// Update share link
	newDesc := "Updated description"
	updateReq := &UpdateShareRequest{
		Description: &newDesc,
	}

	updated, err := m.UpdateShareLink(link.ID, updateReq)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if updated.Description != "Updated description" {
		t.Errorf("expected description 'Updated description', got '%s'", updated.Description)
	}

	// Revoke share link
	err = m.RevokeShareLink(link.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	retrieved, _ = m.GetShareLink(link.ID)
	if retrieved.Status != ShareStatusRevoked {
		t.Errorf("expected status 'revoked', got '%s'", retrieved.Status)
	}

	// Get stats
	stats := m.GetStats()
	if stats.TotalLinks != 1 {
		t.Errorf("expected 1 total link, got %d", stats.TotalLinks)
	}

	if stats.RevokedLinks != 1 {
		t.Errorf("expected 1 revoked link, got %d", stats.RevokedLinks)
	}

	// Delete share link
	err = m.DeleteShareLink(link.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = m.GetShareLink(link.ID)
	if err == nil {
		t.Error("expected error for deleted link")
	}
}

func TestManagerPasswordProtected(t *testing.T) {
	m := NewManager(nil, nil)

	req := &CreateShareRequest{
		FilePath:    "/data/secret.txt",
		FileName:    "secret.txt",
		FileSize:    512,
		FileType:    "text/plain",
		Mode:        ShareModePassword,
		Password:    "mypassword",
		CreatorID:   "user-1",
		CreatorName: "Test User",
	}

	link, err := m.CreateShareLink(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if link.Mode != ShareModePassword {
		t.Errorf("expected mode 'password', got '%s'", link.Mode)
	}

	if link.Password != "mypassword" {
		t.Error("password not set correctly")
	}
}

func TestManagerExpiration(t *testing.T) {
	m := NewManager(nil, nil)

	req := &CreateShareRequest{
		FilePath:    "/data/temp.txt",
		FileName:    "temp.txt",
		FileSize:    256,
		FileType:    "text/plain",
		Mode:        ShareModePublic,
		ExpiresIn:   1 * time.Hour,
		CreatorID:   "user-1",
		CreatorName: "Test User",
	}

	link, err := m.CreateShareLink(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if link.ExpiresAt == nil {
		t.Fatal("expected expiration time to be set")
	}

	if link.ExpiresAt.Before(time.Now()) {
		t.Error("expiration time should be in the future")
	}
}

func TestManagerBatchCreate(t *testing.T) {
	m := NewManager(nil, nil)

	req := &BatchShareRequest{
		FilePaths: []string{
			"/data/file1.txt",
			"/data/file2.txt",
			"/data/file3.txt",
		},
		Mode: ShareModePublic,
	}

	result := m.BatchCreateShareLinks(req)

	if result.Total != 3 {
		t.Errorf("expected 3 total, got %d", result.Total)
	}

	if len(result.Success) != 3 {
		t.Errorf("expected 3 successful, got %d", len(result.Success))
	}

	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %d", len(result.Failed))
	}
}

func TestShareModes(t *testing.T) {
	modes := []ShareMode{
		ShareModePublic,
		ShareModePassword,
		ShareModePrivate,
		ShareModeOnce,
	}

	expected := []string{"public", "password", "private", "once"}

	for i, mode := range modes {
		if string(mode) != expected[i] {
			t.Errorf("expected mode '%s', got '%s'", expected[i], mode)
		}
	}
}

func TestShareStatuses(t *testing.T) {
	statuses := []ShareStatus{
		ShareStatusActive,
		ShareStatusExpired,
		ShareStatusRevoked,
		ShareStatusExhausted,
	}

	expected := []string{"active", "expired", "revoked", "exhausted"}

	for i, status := range statuses {
		if string(status) != expected[i] {
			t.Errorf("expected status '%s', got '%s'", expected[i], status)
		}
	}
}

func TestDeviceTypes(t *testing.T) {
	devices := []DeviceType{
		DeviceDesktop,
		DeviceMobile,
		DeviceTablet,
		DeviceBot,
		DeviceUnknown,
	}

	expected := []string{"desktop", "mobile", "tablet", "bot", "unknown"}

	for i, device := range devices {
		if string(device) != expected[i] {
			t.Errorf("expected device '%s', got '%s'", expected[i], device)
		}
	}
}

func TestAlertLevels(t *testing.T) {
	levels := []AlertLevel{
		AlertLevelInfo,
		AlertLevelWarning,
		AlertLevelCritical,
	}

	expected := []string{"info", "warning", "critical"}

	for i, level := range levels {
		if string(level) != expected[i] {
			t.Errorf("expected level '%s', got '%s'", expected[i], level)
		}
	}
}

func TestWatermarkPositions(t *testing.T) {
	positions := []WatermarkPosition{
		WatermarkTopLeft,
		WatermarkTopRight,
		WatermarkBottomLeft,
		WatermarkBottomRight,
		WatermarkCenter,
		WatermarkTiled,
	}

	expected := []string{"top_left", "top_right", "bottom_left", "bottom_right", "center", "tiled"}

	for i, pos := range positions {
		if string(pos) != expected[i] {
			t.Errorf("expected position '%s', got '%s'", expected[i], pos)
		}
	}
}

// Helper function
func timePtr(t time.Time) *time.Time {
	return &t
}
