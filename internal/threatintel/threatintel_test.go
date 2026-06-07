// Package threatintel - threatintel_test.go 威胁情报中心模块完整测试
package threatintel

import (
	"fmt"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	t.Run("returns valid default config", func(t *testing.T) {
		config := DefaultConfig()
		if config == nil {
			t.Fatal("DefaultConfig returned nil")
		}
		if !config.Enabled {
			t.Error("Expected Enabled to be true")
		}
		if !config.AutoScan {
			t.Error("Expected AutoScan to be true")
		}
		if config.BlockThreshold != 80 {
			t.Errorf("Expected BlockThreshold 80, got %d", config.BlockThreshold)
		}
		if config.MaxIOCs != 100000 {
			t.Errorf("Expected MaxIOCs 100000, got %d", config.MaxIOCs)
		}
		if config.IOCExpiryDays != 90 {
			t.Errorf("Expected IOCExpiryDays 90, got %d", config.IOCExpiryDays)
		}
	})
}

func TestNewEngine(t *testing.T) {
	t.Run("creates engine with default config", func(t *testing.T) {
		engine := NewEngine(nil)
		if engine == nil {
			t.Fatal("NewEngine returned nil")
		}
	})

	t.Run("creates engine with custom config", func(t *testing.T) {
		config := &ThreatIntelConfig{
			Enabled:        true,
			AutoScan:       false,
			AutoBlock:      true,
			BlockThreshold: 70,
			MaxIOCs:        1000,
		}
		engine := NewEngine(config)
		if engine == nil {
			t.Fatal("NewEngine returned nil")
		}
	})
}

func TestFeedManagement(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	t.Run("adds feed successfully", func(t *testing.T) {
		feed := &ThreatFeed{
			ID:         "feed-1",
			Name:       "Test Feed",
			Type:       FeedTypeOpenSource,
			URL:        "https://example.com/feed",
			TrustLevel: 80,
		}

		err := engine.AddFeed(feed)
		if err != nil {
			t.Fatalf("AddFeed failed: %v", err)
		}

		got, err := engine.GetFeed("feed-1")
		if err != nil {
			t.Fatalf("GetFeed failed: %v", err)
		}
		if got.Name != "Test Feed" {
			t.Errorf("Expected name 'Test Feed', got '%s'", got.Name)
		}
	})

	t.Run("rejects duplicate feed ID", func(t *testing.T) {
		feed := &ThreatFeed{ID: "feed-1", Name: "Duplicate"}
		err := engine.AddFeed(feed)
		if err == nil {
			t.Error("Expected error for duplicate feed ID")
		}
	})

	t.Run("lists all feeds", func(t *testing.T) {
		feeds := engine.ListFeeds()
		if len(feeds) != 1 {
			t.Errorf("Expected 1 feed, got %d", len(feeds))
		}
	})

	t.Run("removes feed", func(t *testing.T) {
		err := engine.RemoveFeed("feed-1")
		if err != nil {
			t.Fatalf("RemoveFeed failed: %v", err)
		}

		_, err = engine.GetFeed("feed-1")
		if err == nil {
			t.Error("Expected error after removing feed")
		}
	})

	t.Run("returns error for non-existent feed", func(t *testing.T) {
		_, err := engine.GetFeed("nonexistent")
		if err == nil {
			t.Error("Expected error for non-existent feed")
		}
	})
}

func TestIOCManagement(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	t.Run("adds IOC successfully", func(t *testing.T) {
		ioc := &IOC{
			ID:          "ioc-1",
			Type:        IOCTypeIP,
			Value:       "192.168.1.100",
			Severity:    SeverityHigh,
			Confidence:  90,
			ThreatScore: 85,
			Description: "恶意 IP 地址",
			SourceID:    "feed-1",
		}

		err := engine.AddIOC(ioc)
		if err != nil {
			t.Fatalf("AddIOC failed: %v", err)
		}
	})

	t.Run("looks up IOC by type and value", func(t *testing.T) {
		ioc := engine.LookupIOC(IOCTypeIP, "192.168.1.100")
		if ioc == nil {
			t.Fatal("LookupIOC returned nil")
		}
		if ioc.Severity != SeverityHigh {
			t.Errorf("Expected severity %s, got %s", SeverityHigh, ioc.Severity)
		}
	})

	t.Run("blocks IOC", func(t *testing.T) {
		err := engine.BlockIOC("ioc-1")
		if err != nil {
			t.Fatalf("BlockIOC failed: %v", err)
		}

		blocked := engine.GetBlockedIOCs()
		if len(blocked) != 1 {
			t.Errorf("Expected 1 blocked IOC, got %d", len(blocked))
		}
	})

	t.Run("unblocks IOC", func(t *testing.T) {
		err := engine.UnblockIOC("ioc-1")
		if err != nil {
			t.Fatalf("UnblockIOC failed: %v", err)
		}

		blocked := engine.GetBlockedIOCs()
		if len(blocked) != 0 {
			t.Errorf("Expected 0 blocked IOCs, got %d", len(blocked))
		}
	})

	t.Run("lists all IOCs", func(t *testing.T) {
		iocs := engine.ListIOCs()
		if len(iocs) != 1 {
			t.Errorf("Expected 1 IOC, got %d", len(iocs))
		}
	})

	t.Run("removes IOC", func(t *testing.T) {
		err := engine.RemoveIOC("ioc-1")
		if err != nil {
			t.Fatalf("RemoveIOC failed: %v", err)
		}

		_, err = engine.GetIOC("ioc-1")
		if err == nil {
			t.Error("Expected error after removing IOC")
		}
	})
}

func TestAlertManagement(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	t.Run("creates alert", func(t *testing.T) {
		alert := &Alert{
			ID:          "alert-1",
			Title:       "恶意 IP 检测",
			Description: "检测到来自已知恶意 IP 的连接尝试",
			Severity:    SeverityHigh,
			Score:       85,
			Action:      "建议阻断来源 IP",
		}

		err := engine.CreateAlert(alert)
		if err != nil {
			t.Fatalf("CreateAlert failed: %v", err)
		}

		if alert.Status != AlertStatusOpen {
			t.Errorf("Expected status %s, got %s", AlertStatusOpen, alert.Status)
		}
	})

	t.Run("acknowledges alert", func(t *testing.T) {
		err := engine.AcknowledgeAlert("alert-1")
		if err != nil {
			t.Fatalf("AcknowledgeAlert failed: %v", err)
		}

		alert, _ := engine.GetAlert("alert-1")
		if alert.Status != AlertStatusAcknowledged {
			t.Errorf("Expected %s, got %s", AlertStatusAcknowledged, alert.Status)
		}
	})

	t.Run("resolves alert", func(t *testing.T) {
		err := engine.ResolveAlert("alert-1")
		if err != nil {
			t.Fatalf("ResolveAlert failed: %v", err)
		}

		alert, _ := engine.GetAlert("alert-1")
		if alert.Status != AlertStatusResolved {
			t.Errorf("Expected %s, got %s", AlertStatusResolved, alert.Status)
		}
	})

	t.Run("marks alert as false positive", func(t *testing.T) {
		alert := &Alert{ID: "alert-fp", Title: "Test", Severity: SeverityLow}
		engine.CreateAlert(alert)

		err := engine.MarkFalsePositive("alert-fp")
		if err != nil {
			t.Fatalf("MarkFalsePositive failed: %v", err)
		}

		got, _ := engine.GetAlert("alert-fp")
		if got.Status != AlertStatusFalsePositive {
			t.Errorf("Expected %s, got %s", AlertStatusFalsePositive, got.Status)
		}
	})

	t.Run("lists open alerts", func(t *testing.T) {
		engine.CreateAlert(&Alert{ID: "alert-open", Title: "Open", Severity: SeverityMedium})
		openAlerts := engine.GetOpenAlerts()
		if len(openAlerts) < 1 {
			t.Errorf("Expected at least 1 open alert, got %d", len(openAlerts))
		}
	})
}

func TestScanResults(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	t.Run("saves and retrieves scan result", func(t *testing.T) {
		result := &ScanResult{
			ID:         "scan-1",
			ScanType:   "port",
			Status:     ScanStatusComplete,
			Target:     "192.168.1.1",
			StartTime:  time.Now().Add(-5 * time.Minute),
			EndTime:    time.Now(),
			OpenPorts:  5,
			TotalPorts: 20,
		}

		engine.SaveScanResult(result)

		got, err := engine.GetScanResult("scan-1")
		if err != nil {
			t.Fatalf("GetScanResult failed: %v", err)
		}
		if got.Target != "192.168.1.1" {
			t.Errorf("Expected target '192.168.1.1', got '%s'", got.Target)
		}
	})

	t.Run("lists all scan results", func(t *testing.T) {
		results := engine.ListScanResults()
		if len(results) < 1 {
			t.Errorf("Expected at least 1 scan result, got %d", len(results))
		}
	})
}

func TestThreatScore(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	t.Run("calculates threat score", func(t *testing.T) {
		// 添加一些 IOC
		engine.AddIOC(&IOC{
			ID: "ioc-high", Type: IOCTypeIP, Value: "10.0.0.1",
			Severity: SeverityCritical, ThreatScore: 95,
		})
		engine.AddIOC(&IOC{
			ID: "ioc-low", Type: IOCTypeDomain, Value: "example.com",
			Severity: SeverityLow, ThreatScore: 20,
		})

		score := engine.CalculateThreatScore()
		if score == nil {
			t.Fatal("CalculateThreatScore returned nil")
		}
		if score.Overall < 0 || score.Overall > 100 {
			t.Errorf("Expected overall score 0-100, got %d", score.Overall)
		}
		if score.Level == "" {
			t.Error("Expected level to be set")
		}
	})

	t.Run("gets current threat score", func(t *testing.T) {
		score := engine.GetThreatScore()
		if score == nil {
			t.Fatal("GetThreatScore returned nil")
		}
	})
}

func TestStats(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	// 设置一些数据
	engine.AddFeed(&ThreatFeed{ID: "f1", Name: "Feed 1", Status: FeedStatusActive})
	engine.AddIOC(&IOC{ID: "i1", Type: IOCTypeIP, Value: "1.2.3.4", Blocked: true})
	engine.CreateAlert(&Alert{ID: "a1", Title: "Test Alert", Severity: SeverityHigh})

	stats := engine.GetStats()
	if stats.TotalFeeds != 1 {
		t.Errorf("Expected 1 feed, got %d", stats.TotalFeeds)
	}
	if stats.ActiveFeeds != 1 {
		t.Errorf("Expected 1 active feed, got %d", stats.ActiveFeeds)
	}
	if stats.TotalIOCs != 1 {
		t.Errorf("Expected 1 IOC, got %d", stats.TotalIOCs)
	}
	if stats.BlockedIOCs != 1 {
		t.Errorf("Expected 1 blocked IOC, got %d", stats.BlockedIOCs)
	}
	if stats.TotalAlerts != 1 {
		t.Errorf("Expected 1 alert, got %d", stats.TotalAlerts)
	}
}

func TestScanManager(t *testing.T) {
	t.Run("concurrent scan protection", func(t *testing.T) {
		sm := NewScanManager()

		if !sm.TryStartScan() {
			t.Error("Expected first scan to start")
		}

		if sm.TryStartScan() {
			t.Error("Expected second scan to be rejected")
		}

		sm.FinishScan()

		if !sm.TryStartScan() {
			t.Error("Expected scan to start after finish")
		}
		sm.FinishScan()
	})
}

func TestIOCValidator(t *testing.T) {
	v := NewIOCValidator()

	t.Run("validates IP addresses", func(t *testing.T) {
		tests := []struct {
			ip    string
			valid bool
		}{
			{"192.168.1.1", true},
			{"10.0.0.1", true},
			{"::1", true},
			{"invalid-ip", false},
			{"", false},
		}

		for _, tt := range tests {
			if got := v.ValidateIP(tt.ip); got != tt.valid {
				t.Errorf("ValidateIP(%s) = %v, want %v", tt.ip, got, tt.valid)
			}
		}
	})

	t.Run("validates domains", func(t *testing.T) {
		tests := []struct {
			domain string
			valid  bool
		}{
			{"example.com", true},
			{"sub.example.com", true},
			{"test-site.org", true},
			{".invalid", false},
			{"", false},
		}

		for _, tt := range tests {
			if got := v.ValidateDomain(tt.domain); got != tt.valid {
				t.Errorf("ValidateDomain(%s) = %v, want %v", tt.domain, got, tt.valid)
			}
		}
	})

	t.Run("validates file hashes", func(t *testing.T) {
		tests := []struct {
			hash  string
			valid bool
		}{
			{"d41d8cd98f00b204e9800998ecf8427e", true},                                 // MD5
			{"da39a3ee5e6b4b0d3255bfef95601890afd80709", true},                         // SHA1
			{"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", true}, // SHA256
			{"invalid", false},
			{"", false},
		}

		for _, tt := range tests {
			if got := v.ValidateFileHash(tt.hash); got != tt.valid {
				t.Errorf("ValidateFileHash(%s) = %v, want %v", tt.hash, got, tt.valid)
			}
		}
	})

	t.Run("validates URLs", func(t *testing.T) {
		tests := []struct {
			url   string
			valid bool
		}{
			{"http://example.com", true},
			{"https://example.com/path", true},
			{"ftp://example.com", false},
			{"not-a-url", false},
		}

		for _, tt := range tests {
			if got := v.ValidateURL(tt.url); got != tt.valid {
				t.Errorf("ValidateURL(%s) = %v, want %v", tt.url, got, tt.valid)
			}
		}
	})

	t.Run("validates IOC objects", func(t *testing.T) {
		validIOC := &IOC{Type: IOCTypeIP, Value: "192.168.1.1"}
		if err := v.ValidateIOC(validIOC); err != nil {
			t.Errorf("Expected valid IOC, got error: %v", err)
		}

		invalidIOC := &IOC{Type: IOCTypeIP, Value: "invalid"}
		if err := v.ValidateIOC(invalidIOC); err == nil {
			t.Error("Expected error for invalid IOC")
		}
	})
}

func TestIOCMatcher(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	matcher := NewIOCMatcher(engine)

	// 添加测试 IOC
	engine.AddIOC(&IOC{ID: "ip-1", Type: IOCTypeIP, Value: "192.168.1.100", Severity: SeverityHigh})
	engine.AddIOC(&IOC{ID: "cidr-1", Type: IOCTypeCIDR, Value: "10.0.0.0/24", Severity: SeverityMedium})
	engine.AddIOC(&IOC{ID: "domain-1", Type: IOCTypeDomain, Value: "malware.com", Severity: SeverityCritical})
	engine.AddIOC(&IOC{ID: "hash-1", Type: IOCTypeFileHash, Value: "d41d8cd98f00b204e9800998ecf8427e", Severity: SeverityHigh})

	t.Run("matches IP", func(t *testing.T) {
		matches := matcher.MatchIP("192.168.1.100")
		if len(matches) != 1 {
			t.Errorf("Expected 1 match, got %d", len(matches))
		}
	})

	t.Run("matches IP in CIDR", func(t *testing.T) {
		matches := matcher.MatchIP("10.0.0.50")
		if len(matches) != 1 {
			t.Errorf("Expected 1 match for CIDR, got %d", len(matches))
		}
	})

	t.Run("matches domain", func(t *testing.T) {
		matches := matcher.MatchDomain("malware.com")
		if len(matches) != 1 {
			t.Errorf("Expected 1 match, got %d", len(matches))
		}
	})

	t.Run("matches subdomain", func(t *testing.T) {
		matches := matcher.MatchDomain("sub.malware.com")
		if len(matches) != 1 {
			t.Errorf("Expected 1 match for subdomain, got %d", len(matches))
		}
	})

	t.Run("matches file hash", func(t *testing.T) {
		matches := matcher.MatchFileHash("d41d8cd98f00b204e9800998ecf8427e")
		if len(matches) != 1 {
			t.Errorf("Expected 1 match, got %d", len(matches))
		}
	})

	t.Run("matches any type", func(t *testing.T) {
		matches := matcher.MatchAny("192.168.1.100")
		if len(matches) != 1 {
			t.Errorf("Expected 1 match, got %d", len(matches))
		}
	})
}

func TestBlockManager(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	bm := NewBlockManager(engine)

	t.Run("blocks and checks IP", func(t *testing.T) {
		bm.BlockIP("192.168.1.1", 1*time.Hour)
		if !bm.IsIPBlocked("192.168.1.1") {
			t.Error("Expected IP to be blocked")
		}
		if bm.IsIPBlocked("192.168.1.2") {
			t.Error("Expected IP to not be blocked")
		}
	})

	t.Run("unblocks IP", func(t *testing.T) {
		bm.UnblockIP("192.168.1.1")
		if bm.IsIPBlocked("192.168.1.1") {
			t.Error("Expected IP to be unblocked")
		}
	})

	t.Run("blocks and checks domain", func(t *testing.T) {
		bm.BlockDomain("evil.com", 1*time.Hour)
		if !bm.IsDomainBlocked("evil.com") {
			t.Error("Expected domain to be blocked")
		}
	})

	t.Run("unblocks domain", func(t *testing.T) {
		bm.UnblockDomain("evil.com")
		if bm.IsDomainBlocked("evil.com") {
			t.Error("Expected domain to be unblocked")
		}
	})

	t.Run("gets blocked IPs", func(t *testing.T) {
		bm.BlockIP("10.0.0.1", 1*time.Hour)
		blocked := bm.GetBlockedIPs()
		if len(blocked) != 1 {
			t.Errorf("Expected 1 blocked IP, got %d", len(blocked))
		}
	})

	t.Run("cleans up expired blocks", func(t *testing.T) {
		bm.BlockIP("10.0.0.2", -1*time.Hour) // 已过期
		cleaned := bm.CleanupExpired()
		if cleaned < 1 {
			t.Errorf("Expected at least 1 cleaned, got %d", cleaned)
		}
	})
}

func TestFeedManager(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	fm := NewFeedManager(engine)

	t.Run("subscribes and notifies", func(t *testing.T) {
		ch := fm.Subscribe("feed-1")

		ioc := &IOC{ID: "test-ioc", Type: IOCTypeIP, Value: "1.2.3.4"}
		fm.NotifySubscribers("feed-1", ioc)

		select {
		case received := <-ch:
			if received.ID != "test-ioc" {
				t.Errorf("Expected IOC ID 'test-ioc', got '%s'", received.ID)
			}
		case <-time.After(1 * time.Second):
			t.Error("Timeout waiting for notification")
		}
	})

	t.Run("stops manager", func(t *testing.T) {
		fm.Stop()
	})
}

func TestTrustedSourceManager(t *testing.T) {
	tsm := NewTrustedSourceManager()

	t.Run("has default sources", func(t *testing.T) {
		sources := tsm.List()
		if len(sources) < 3 {
			t.Errorf("Expected at least 3 default sources, got %d", len(sources))
		}
	})

	t.Run("adds custom source", func(t *testing.T) {
		src := &TrustedSource{
			ID:         "custom",
			Name:       "Custom Source",
			TrustLevel: 70,
			Category:   "community",
		}
		tsm.Add(src)

		got, exists := tsm.Get("custom")
		if !exists {
			t.Fatal("Expected custom source to exist")
		}
		if got.Name != "Custom Source" {
			t.Errorf("Expected name 'Custom Source', got '%s'", got.Name)
		}
	})

	t.Run("filters by trust level", func(t *testing.T) {
		highTrust := tsm.GetByTrustLevel(90)
		if len(highTrust) < 2 {
			t.Errorf("Expected at least 2 high-trust sources, got %d", len(highTrust))
		}
	})

	t.Run("gets verified sources", func(t *testing.T) {
		verified := tsm.GetVerified()
		if len(verified) < 3 {
			t.Errorf("Expected at least 3 verified sources, got %d", len(verified))
		}
	})

	t.Run("removes source", func(t *testing.T) {
		tsm.Remove("custom")
		_, exists := tsm.Get("custom")
		if exists {
			t.Error("Expected custom source to be removed")
		}
	})
}

func TestSharingManager(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	sm := NewSharingManager(nil, engine)

	engine.AddIOC(&IOC{ID: "share-1", Type: IOCTypeIP, Value: "1.2.3.4", Severity: SeverityHigh})
	engine.AddIOC(&IOC{ID: "share-2", Type: IOCTypeDomain, Value: "evil.com", Severity: SeverityCritical})

	t.Run("exports IOCs", func(t *testing.T) {
		exported := sm.ExportIOCs()
		if len(exported) != 2 {
			t.Errorf("Expected 2 exported IOCs, got %d", len(exported))
		}
	})

	t.Run("imports IOCs", func(t *testing.T) {
		newIOCs := []*IOC{
			{ID: "import-1", Type: IOCTypeIP, Value: "5.6.7.8", Severity: SeverityMedium},
			{ID: "import-2", Type: IOCTypeIP, Value: "9.10.11.12", Severity: SeverityLow},
		}
		imported, skipped := sm.ImportIOCs(newIOCs, "external-source")
		if imported != 2 {
			t.Errorf("Expected 2 imported, got %d", imported)
		}
		if skipped != 0 {
			t.Errorf("Expected 0 skipped, got %d", skipped)
		}
	})

	t.Run("gets export stats", func(t *testing.T) {
		stats := sm.GetExportStats()
		if stats["total"] < 2 {
			t.Errorf("Expected at least 2 total, got %d", stats["total"])
		}
	})
}

func TestUpdateScheduler(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	fm := NewFeedManager(engine)
	us := NewUpdateScheduler(engine, fm, 1*time.Second)

	t.Run("starts and stops", func(t *testing.T) {
		us.Start()
		if !us.IsRunning() {
			t.Error("Expected scheduler to be running")
		}
		us.Stop()
		time.Sleep(100 * time.Millisecond) // 等待 goroutine 退出
	})

	t.Run("gets update status", func(t *testing.T) {
		status := us.GetUpdateStatus()
		if status == nil {
			t.Error("Expected non-nil status")
		}
	})
}

func TestFormatAlertMessage(t *testing.T) {
	alert := &Alert{
		Severity:    SeverityHigh,
		Title:       "测试告警",
		Description: "这是测试告警描述",
		Score:       85,
	}
	msg := FormatAlertMessage(alert)
	if msg == "" {
		t.Error("Expected non-empty message")
	}
}

func TestCleanupExpiredIOCs(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	expired := time.Now().Add(-1 * time.Hour)
	engine.AddIOC(&IOC{
		ID:        "expired-ioc",
		Type:      IOCTypeIP,
		Value:     "1.2.3.4",
		ExpiresAt: &expired,
	})
	engine.AddIOC(&IOC{
		ID:    "valid-ioc",
		Type:  IOCTypeIP,
		Value: "5.6.7.8",
	})

	cleaned := engine.CleanupExpiredIOCs()
	if cleaned != 1 {
		t.Errorf("Expected 1 cleaned, got %d", cleaned)
	}

	remaining := engine.ListIOCs()
	if len(remaining) != 1 {
		t.Errorf("Expected 1 remaining, got %d", len(remaining))
	}
}

func TestCommonVulns(t *testing.T) {
	if len(CommonVulns) < 2 {
		t.Errorf("Expected at least 2 common vulnerabilities, got %d", len(CommonVulns))
	}

	for _, vuln := range CommonVulns {
		if vuln.CVE == "" {
			t.Error("Expected CVE to be set")
		}
		if vuln.CVSS <= 0 {
			t.Errorf("Expected positive CVSS for %s", vuln.CVE)
		}
	}
}

func TestThreatIntelError(t *testing.T) {
	t.Run("formats error without inner error", func(t *testing.T) {
		err := &ThreatIntelError{Code: "TEST", Message: "test message"}
		expected := "[TEST] test message"
		if err.Error() != expected {
			t.Errorf("Expected '%s', got '%s'", expected, err.Error())
		}
	})

	t.Run("formats error with inner error", func(t *testing.T) {
		inner := fmt.Errorf("inner")
		err := &ThreatIntelError{Code: "TEST", Message: "outer", Err: inner}
		if err.Unwrap() != inner {
			t.Error("Expected Unwrap to return inner error")
		}
	})
}
