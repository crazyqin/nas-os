package ransomware

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewThreatIntelligence(t *testing.T) {
	ti := NewThreatIntelligence("")
	if ti == nil {
		t.Fatal("NewThreatIntelligence returned nil")
	}

	if len(ti.signatures) == 0 {
		t.Error("expected builtin signatures to be loaded")
	}
}

func TestThreatIntelligence_AddIOC(t *testing.T) {
	ti := NewThreatIntelligence("")

	ioc := IOC{
		Type:        IOCTypeHash,
		Value:       "abc123def456",
		ThreatLevel: ThreatLevelCritical,
		Confidence:  0.95,
		Source:      "test",
		Description: "Test IOC",
	}

	ti.AddIOC(ioc)

	iocs := ti.GetIOCs(IOCTypeHash)
	if len(iocs) == 0 {
		t.Error("expected IOC to be added")
	}

	found := false
	for _, i := range iocs {
		if i.Value == "abc123def456" {
			found = true
		}
	}
	if !found {
		t.Error("added IOC not found")
	}
}

func TestThreatIntelligence_MatchHash(t *testing.T) {
	ti := NewThreatIntelligence("")

	// 添加一个 IOC
	ti.AddIOC(IOC{
		Type:        IOCTypeHash,
		Value:       "malware_hash_123",
		ThreatLevel: ThreatLevelCritical,
		Confidence:  0.99,
		Source:      "test",
	})

	// 匹配
	result := ti.MatchHash("malware_hash_123")
	if !result.Matched {
		t.Error("expected hash to match")
	}

	if result.Confidence != 0.99 {
		t.Errorf("expected confidence 0.99, got %f", result.Confidence)
	}

	// 不匹配
	result2 := ti.MatchHash("clean_hash")
	if result2.Matched {
		t.Error("clean hash should not match")
	}
}

func TestThreatIntelligence_MatchExtension(t *testing.T) {
	ti := NewThreatIntelligence("")

	// WannaCry 扩展名
	result := ti.MatchExtension(".wncry")
	if !result.Matched {
		t.Error("expected .wncry to match WannaCry signature")
	}

	if result.Signature == nil {
		t.Fatal("expected signature to be set")
	}

	if result.Signature.Family != "WannaCry" {
		t.Errorf("expected WannaCry family, got %s", result.Signature.Family)
	}

	// 普通扩展名不匹配
	result2 := ti.MatchExtension(".txt")
	if result2.Matched {
		t.Error(".txt should not match any signature")
	}
}

func TestThreatIntelligence_MatchRansomNote(t *testing.T) {
	ti := NewThreatIntelligence("")

	result := ti.MatchRansomNote("RyukReadMe.txt")
	if !result.Matched {
		t.Error("expected ransom note to match Ryuk")
	}

	if result.Signature.Family != "Ryuk" {
		t.Errorf("expected Ryuk, got %s", result.Signature.Family)
	}
}

func TestThreatIntelligence_MatchIP(t *testing.T) {
	ti := NewThreatIntelligence("")

	ti.AddIOC(IOC{
		Type:        IOCTypeIP,
		Value:       "1.2.3.4",
		ThreatLevel: ThreatLevelHigh,
		Confidence:  0.8,
	})

	result := ti.MatchIP("1.2.3.4")
	if !result.Matched {
		t.Error("expected IP to match")
	}

	result2 := ti.MatchIP("5.6.7.8")
	if result2.Matched {
		t.Error("unknown IP should not match")
	}
}

func TestThreatIntelligence_MatchDomain(t *testing.T) {
	ti := NewThreatIntelligence("")

	ti.AddIOC(IOC{
		Type:        IOCTypeDomain,
		Value:       "evil.com",
		ThreatLevel: ThreatLevelHigh,
		Confidence:  0.9,
	})

	result := ti.MatchDomain("evil.com")
	if !result.Matched {
		t.Error("expected domain to match")
	}

	// 子域名也应该匹配
	result2 := ti.MatchDomain("sub.evil.com")
	if !result2.Matched {
		t.Error("subdomain should match parent domain IOC")
	}

	result3 := ti.MatchDomain("safe.com")
	if result3.Matched {
		t.Error("safe domain should not match")
	}
}

func TestThreatIntelligence_RemoveIOC(t *testing.T) {
	ti := NewThreatIntelligence("")

	ti.AddIOC(IOC{
		ID:    "test-remove",
		Type:  IOCTypeHash,
		Value: "remove_me",
	})

	ti.RemoveIOC("test-remove")

	iocs := ti.GetIOCs(IOCTypeHash)
	for _, ioc := range iocs {
		if ioc.ID == "test-remove" {
			t.Error("IOC should have been removed")
		}
	}
}

func TestThreatIntelligence_AddSignature(t *testing.T) {
	ti := NewThreatIntelligence("")

	sig := RansomwareSignature{
		ID:       "custom-sig",
		Family:   "TestRansom",
		Name:     "Test Ransomware",
		FileExts: []string{".testransom"},
		Severity: ThreatLevelCritical,
	}

	ti.AddSignature(sig)

	sigs := ti.GetSignatures()
	found := false
	for _, s := range sigs {
		if s.ID == "custom-sig" {
			found = true
		}
	}
	if !found {
		t.Error("custom signature not found")
	}

	// 扩展名匹配应该能找到
	result := ti.MatchExtension(".testransom")
	if !result.Matched {
		t.Error("expected .testransom to match custom signature")
	}
}

func TestThreatIntelligence_GetStats(t *testing.T) {
	ti := NewThreatIntelligence("")

	ti.AddIOC(IOC{Type: IOCTypeHash, Value: "hash1"})
	ti.AddIOC(IOC{Type: IOCTypeIP, Value: "1.1.1.1"})

	stats := ti.GetStats()
	if stats.TotalIOCs < 2 {
		t.Errorf("expected at least 2 IOCs, got %d", stats.TotalIOCs)
	}

	if stats.Signatures == 0 {
		t.Error("expected builtin signatures to be counted")
	}
}

func TestThreatIntelligence_GetFeeds(t *testing.T) {
	ti := NewThreatIntelligence("")

	feeds := ti.GetFeeds()
	if len(feeds) == 0 {
		t.Error("expected default feeds to be loaded")
	}
}

func TestThreatIntelligence_AddFeed(t *testing.T) {
	ti := NewThreatIntelligence("")

	feed := IntelFeed{
		Name:        "Custom Feed",
		URL:         "https://example.com/feed",
		Type:        "custom",
		Enabled:     true,
		IntervalMin: 30,
	}

	ti.AddFeed(feed)

	feeds := ti.GetFeeds()
	found := false
	for _, f := range feeds {
		if f.Name == "Custom Feed" {
			found = true
		}
	}
	if !found {
		t.Error("custom feed not found")
	}
}

func TestThreatIntelligence_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	ti := NewThreatIntelligence(tmpDir)

	ti.AddIOC(IOC{
		Type:  IOCTypeHash,
		Value: "persist_hash_123",
	})

	if err := ti.SaveLocalData(); err != nil {
		t.Fatalf("SaveLocalData failed: %v", err)
	}

	// 验证文件存在
	iocsFile := filepath.Join(tmpDir, "iocs.json")
	if _, err := os.Stat(iocsFile); os.IsNotExist(err) {
		t.Error("iocs.json should exist after save")
	}

	// 新实例应该能加载
	ti2 := NewThreatIntelligence(tmpDir)
	ti2.loadLocalData()

	iocs := ti2.GetIOCs(IOCTypeHash)
	found := false
	for _, ioc := range iocs {
		if ioc.Value == "persist_hash_123" {
			found = true
		}
	}
	if !found {
		t.Error("persisted IOC not loaded")
	}
}

func TestThreatIntelligence_ClearCache(t *testing.T) {
	ti := NewThreatIntelligence("")

	ti.AddIOC(IOC{Type: IOCTypeHash, Value: "cached_hash"})

	// 第一次查询
	ti.MatchHash("cached_hash")

	// 清除缓存
	ti.ClearCache()

	stats := ti.GetStats()
	if stats.CacheHits != 0 {
		t.Error("cache should be empty after ClearCache")
	}
}

func TestThreatIntelligence_BuiltinSignatures(t *testing.T) {
	ti := NewThreatIntelligence("")

	expectedFamilies := []string{
		"WannaCry", "Ryuk", "Conti", "LockBit",
		"REvil/Sodinokibi", "Maze", "BlackCat/ALPHV",
	}

	sigs := ti.GetSignatures()
	sigFamilies := make(map[string]bool)
	for _, sig := range sigs {
		sigFamilies[sig.Family] = true
	}

	for _, family := range expectedFamilies {
		if !sigFamilies[family] {
			t.Errorf("expected builtin signature for family: %s", family)
		}
	}
}

func TestIOC_Types(t *testing.T) {
	ti := NewThreatIntelligence("")

	types := []IOCType{IOCTypeHash, IOCTypeIP, IOCTypeDomain, IOCTypeURL, IOCTypeEmail}
	for _, iocType := range types {
		ti.AddIOC(IOC{Type: iocType, Value: "test_" + string(iocType)})
	}

	for _, iocType := range types {
		iocs := ti.GetIOCs(iocType)
		if len(iocs) == 0 {
			t.Errorf("expected IOC for type %s", iocType)
		}
	}
}

func TestIntelFeed_NeedsUpdate(t *testing.T) {
	feed := IntelFeed{
		Enabled:     true,
		IntervalMin: 30,
		LastFetch:   time.Now().Add(-1 * time.Hour),
	}

	if !feed.needsUpdate() {
		t.Error("feed should need update after interval")
	}

	feed.LastFetch = time.Now()
	if feed.needsUpdate() {
		t.Error("feed should not need update right after fetch")
	}

	feed.IntervalMin = 0
	if feed.needsUpdate() {
		t.Error("feed with interval 0 should never need update")
	}
}
