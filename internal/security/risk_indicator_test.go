package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRiskIndicatorManager(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false // 禁用自动更新

	manager := NewRiskIndicatorManager(config)

	if manager == nil {
		t.Fatal("Expected manager to be created, got nil")
	}

	if manager.config.EPSSThreshold != 0.1 {
		t.Errorf("Expected EPSSThreshold 0.1, got %f", manager.config.EPSSThreshold)
	}

	if manager.config.EPSSPercentileThreshold != 0.9 {
		t.Errorf("Expected EPSSPercentileThreshold 0.9, got %f", manager.config.EPSSPercentileThreshold)
	}
}

func TestDefaultRiskScoreFactors(t *testing.T) {
	factors := DefaultRiskScoreFactors()

	// 验证权重总和约为 1.0
	total := factors.CVSSWeight + factors.EPSSWeight + factors.KEVWeight +
		factors.AgeWeight + factors.AssetWeight + factors.ExploitWeight + factors.RansomwareWeight

	if total < 0.99 || total > 1.01 {
		t.Errorf("Expected weight sum ~1.0, got %f", total)
	}
}

func TestScoreToRiskLevel(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	tests := []struct {
		score    float64
		expected string
	}{
		{95, "critical"},
		{85, "critical"},
		{75, "high"},
		{55, "medium"},
		{35, "low"},
		{15, "low"},
	}

	for _, tt := range tests {
		level := manager.scoreToRiskLevel(tt.score)
		if level != tt.expected {
			t.Errorf("Score %f: expected %s, got %s", tt.score, tt.expected, level)
		}
	}
}

func TestDeterminePriority(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	tests := []struct {
		indicator *RiskIndicator
		expected  int
	}{
		{&RiskIndicator{RiskScore: 95, Exploitability: "known"}, 1},
		{&RiskIndicator{RiskScore: 75, Exploitability: "likely"}, 2},
		{&RiskIndicator{RiskScore: 55, Exploitability: "potential"}, 3},
		{&RiskIndicator{RiskScore: 25, Exploitability: "none"}, 4},
	}

	for _, tt := range tests {
		priority := manager.determinePriority(tt.indicator)
		if priority != tt.expected {
			t.Errorf("Expected priority %d, got %d", tt.expected, priority)
		}
	}
}

func TestDetermineExploitability(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	tests := []struct {
		name      string
		indicator *RiskIndicator
		expected  string
	}{
		{"KEV", &RiskIndicator{IsInKEV: true, EPSSScore: 0, EPSSPercentile: 0}, "known"},
		{"High EPSS", &RiskIndicator{EPSSScore: 0.8, EPSSPercentile: 0.95, IsInKEV: false}, "likely"},
		{"High Percentile", &RiskIndicator{EPSSScore: 0.05, EPSSPercentile: 0.95, IsInKEV: false}, "likely"},
		{"Medium EPSS", &RiskIndicator{EPSSScore: 0.02, EPSSPercentile: 0.5, IsInKEV: false}, "potential"},
		{"Low EPSS", &RiskIndicator{EPSSScore: 0, EPSSPercentile: 0, IsInKEV: false}, "none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exploit := manager.determineExploitability(tt.indicator)
			if exploit != tt.expected {
				t.Errorf("Expected exploitability %s, got %s", tt.expected, exploit)
			}
		})
	}
}

func TestIsInKEV(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	// 没有加载数据时应返回 false
	if manager.IsInKEV("CVE-2024-0001") {
		t.Error("Expected false when no KEV data loaded")
	}

	// 加载模拟数据
	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", VendorProject: "Test", Product: "Product"},
		},
	}
	manager.mu.Unlock()

	if !manager.IsInKEV("CVE-2024-0001") {
		t.Error("Expected true for CVE in KEV")
	}

	if manager.IsInKEV("CVE-2024-9999") {
		t.Error("Expected false for CVE not in KEV")
	}
}

func TestGetKEVInfo(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	// 加载测试数据
	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{
				CVEID:                      "CVE-2024-0001",
				VendorProject:              "TestVendor",
				Product:                    "TestProduct",
				VulnerabilityName:          "Test Vulnerability",
				DateAdded:                  "2024-01-01",
				ShortDescription:           "Test description",
				RequiredAction:             "Apply patch",
				KnownRansomwareCampaignUse: "Known",
			},
		},
	}
	manager.mu.Unlock()

	info := manager.GetKEVInfo("CVE-2024-0001")
	if info == nil {
		t.Fatal("Expected KEV info, got nil")
	}

	if info.VendorProject != "TestVendor" {
		t.Errorf("Expected VendorProject TestVendor, got %s", info.VendorProject)
	}

	if info.KnownRansomwareCampaignUse != "Known" {
		t.Error("Expected ransomware association")
	}
}

func TestCalculateRiskScore(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	// 加载测试数据
	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", KnownRansomwareCampaignUse: "Known"},
		},
	}
	manager.epssCache = map[string]*EPSSData{
		"CVE-2024-0001": {EPSSScore: 0.9, Percentile: 0.95},
	}
	manager.mu.Unlock()

	indicator := manager.CalculateRiskScore("CVE-2024-0001", 9.8, 1.0, time.Now().AddDate(0, -1, 0))

	if indicator == nil {
		t.Fatal("Expected indicator, got nil")
	}

	// 验证风险等级
	if indicator.RiskLevel != "critical" {
		t.Errorf("Expected critical risk level, got %s", indicator.RiskLevel)
	}

	// 验证在 KEV 中
	if !indicator.IsInKEV {
		t.Error("Expected IsInKEV to be true")
	}

	// 验证可利用性
	if indicator.Exploitability != "known" {
		t.Errorf("Expected exploitability 'known', got %s", indicator.Exploitability)
	}
}

func TestGetRiskIndicators(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	// 加载测试数据
	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", KnownRansomwareCampaignUse: "Known"},
			{CVEID: "CVE-2024-0002", KnownRansomwareCampaignUse: "Unknown"},
		},
	}
	manager.epssCache = map[string]*EPSSData{
		"CVE-2024-0001": {EPSSScore: 0.9, Percentile: 0.95},
		"CVE-2024-0002": {EPSSScore: 0.5, Percentile: 0.7},
	}
	manager.mu.Unlock()

	vulns := []VulnerabilityItem{
		{CVEID: "CVE-2024-0001", CVSSScore: 9.8, Severity: CVSSCritical},
		{CVEID: "CVE-2024-0002", CVSSScore: 7.5, Severity: CVSSHigh},
		{CVEID: "CVE-2024-0003", CVSSScore: 5.0, Severity: CVSSMedium},
	}

	indicators := manager.GetRiskIndicators(vulns)

	if len(indicators) != 3 {
		t.Errorf("Expected 3 indicators, got %d", len(indicators))
	}

	// 验证第一个是关键风险
	if indicators[0].RiskLevel != "critical" {
		t.Errorf("Expected critical, got %s", indicators[0].RiskLevel)
	}
}

func TestGenerateRiskReport(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	// 加载测试数据
	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", KnownRansomwareCampaignUse: "Known"},
		},
	}
	manager.epssCache = map[string]*EPSSData{
		"CVE-2024-0001": {EPSSScore: 0.9, Percentile: 0.95},
	}
	manager.mu.Unlock()

	vulns := []VulnerabilityItem{
		{CVEID: "CVE-2024-0001", CVSSScore: 9.8, Severity: CVSSCritical},
		{CVEID: "CVE-2024-0002", CVSSScore: 7.5, Severity: CVSSHigh},
	}

	report := manager.GenerateRiskReport(vulns)

	if report == nil {
		t.Fatal("Expected report, got nil")
	}

	if report.TotalVulnerabilities != 2 {
		t.Errorf("Expected 2 total vulnerabilities, got %d", report.TotalVulnerabilities)
	}

	if report.KEVCount != 1 {
		t.Errorf("Expected KEV count 1, got %d", report.KEVCount)
	}
}

func TestGetStatistics(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	indicators := []*RiskIndicator{
		{RiskLevel: "critical", Exploitability: "known", Priority: 1, RiskScore: 95, IsInKEV: true, EPSSScore: 0.9},
		{RiskLevel: "high", Exploitability: "likely", Priority: 2, RiskScore: 75, IsInKEV: false, EPSSScore: 0.5},
		{RiskLevel: "medium", Exploitability: "potential", Priority: 3, RiskScore: 55, IsInKEV: false, EPSSScore: 0.3},
		{RiskLevel: "low", Exploitability: "none", Priority: 4, RiskScore: 25, IsInKEV: false, EPSSScore: 0.1},
	}

	stats := manager.GetStatistics(indicators)

	if stats.TotalIndicators != 4 {
		t.Errorf("Expected 4 indicators, got %d", stats.TotalIndicators)
	}

	if stats.ByRiskLevel["critical"] != 1 {
		t.Errorf("Expected 1 critical, got %d", stats.ByRiskLevel["critical"])
	}

	if stats.KEVPercentage != 25.0 {
		t.Errorf("Expected KEV percentage 25.0, got %f", stats.KEVPercentage)
	}
}

func TestFilterKEVByRansomware(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	// 加载测试数据
	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", KnownRansomwareCampaignUse: "Known"},
			{CVEID: "CVE-2024-0002", KnownRansomwareCampaignUse: "Unknown"},
			{CVEID: "CVE-2024-0003", KnownRansomwareCampaignUse: "Known"},
		},
	}
	manager.mu.Unlock()

	ransomware := manager.FilterKEVByRansomware()

	if len(ransomware) != 2 {
		t.Errorf("Expected 2 ransomware-related vulnerabilities, got %d", len(ransomware))
	}
}

func TestSearchKEV(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	// 加载测试数据
	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", VendorProject: "Microsoft", Product: "Windows", VulnerabilityName: "RCE"},
			{CVEID: "CVE-2024-0002", VendorProject: "Apple", Product: "iOS", VulnerabilityName: "XSS"},
		},
	}
	manager.mu.Unlock()

	results := manager.SearchKEV("Microsoft")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for Microsoft, got %d", len(results))
	}

	results = manager.SearchKEV("iOS")
	if len(results) != 1 {
		t.Errorf("Expected 1 result for iOS, got %d", len(results))
	}
}

func TestFetchEPSSData(t *testing.T) {
	// 创建模拟 EPSS 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"status": "OK",
			"data": []map[string]interface{}{
				{"cve": "CVE-2024-0001", "epss": 0.9, "percentile": 0.95},
				{"cve": "CVE-2024-0002", "epss": 0.5, "percentile": 0.7},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	config.EPSSSourceURL = server.URL
	manager := NewRiskIndicatorManager(config)

	ctx := context.Background()
	cveIDs := []string{"CVE-2024-0001", "CVE-2024-0002"}

	data, err := manager.FetchEPSSData(ctx, cveIDs)
	if err != nil {
		t.Fatalf("FetchEPSSData failed: %v", err)
	}

	if len(data) != 2 {
		t.Errorf("Expected 2 EPSS entries, got %d", len(data))
	}

	if data["CVE-2024-0001"].EPSSScore != 0.9 {
		t.Errorf("Expected EPSS score 0.9, got %f", data["CVE-2024-0001"].EPSSScore)
	}
}

func TestFetchKEVCatalog(t *testing.T) {
	// 创建临时缓存目录
	tmpDir := t.TempDir()

	// 创建模拟 KEV 服务器
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := KEVCatalog{
			Title:          "Test KEV Catalog",
			CatalogVersion: "2024.01",
			Count:          1,
			Vulnerabilities: []KEVEntry{
				{
					CVEID:             "CVE-2024-0001",
					VendorProject:     "Test",
					Product:           "TestProduct",
					VulnerabilityName: "Test Vuln",
					DateAdded:         "2024-01-01",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	config.KEVSourceURL = server.URL
	config.CacheDir = tmpDir
	manager := NewRiskIndicatorManager(config)

	ctx := context.Background()
	err := manager.FetchKEVCatalog(ctx)
	if err != nil {
		t.Fatalf("FetchKEVCatalog failed: %v", err)
	}

	if manager.kevCatalog == nil {
		t.Fatal("Expected KEV catalog to be loaded")
	}

	if manager.kevCatalog.Count != 1 {
		t.Errorf("Expected count 1, got %d", manager.kevCatalog.Count)
	}
}

func TestGetAllKEVVulnerabilities(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	// 没有数据时
	all := manager.GetAllKEVVulnerabilities()
	if len(all) != 0 {
		t.Errorf("Expected 0 entries when no data, got %d", len(all))
	}

	// 加载数据
	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001"},
			{CVEID: "CVE-2024-0002"},
		},
	}
	manager.mu.Unlock()

	all = manager.GetAllKEVVulnerabilities()
	if len(all) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(all))
	}
}

func TestFilterKEVByVendor(t *testing.T) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", VendorProject: "Microsoft"},
			{CVEID: "CVE-2024-0002", VendorProject: "Apple"},
			{CVEID: "CVE-2024-0003", VendorProject: "Microsoft"},
		},
	}
	manager.mu.Unlock()

	results := manager.FilterKEVByVendor("Microsoft")
	if len(results) != 2 {
		t.Errorf("Expected 2 Microsoft vulnerabilities, got %d", len(results))
	}
}

func TestRiskIndicatorJSON(t *testing.T) {
	now := time.Now()
	indicator := &RiskIndicator{
		ID:             "test-1",
		CVEID:          "CVE-2024-0001",
		Name:           "Test Vulnerability",
		RiskScore:      95.5,
		RiskLevel:      "critical",
		CVSSScore:      9.8,
		EPSSScore:      0.9,
		EPSSPercentile: 0.95,
		IsInKEV:        true,
		Exploitability: "known",
		Priority:       1,
		LastUpdated:    now,
	}

	// 序列化
	data, err := json.Marshal(indicator)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// 反序列化
	var parsed RiskIndicator
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if parsed.CVEID != indicator.CVEID {
		t.Errorf("CVEID mismatch: expected %s, got %s", indicator.CVEID, parsed.CVEID)
	}

	if parsed.RiskScore != indicator.RiskScore {
		t.Errorf("RiskScore mismatch: expected %f, got %f", indicator.RiskScore, parsed.RiskScore)
	}
}

// Benchmark 测试
func BenchmarkCalculateRiskScore(b *testing.B) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", KnownRansomwareCampaignUse: "Known"},
		},
	}
	manager.epssCache = map[string]*EPSSData{
		"CVE-2024-0001": {EPSSScore: 0.9, Percentile: 0.95},
	}
	manager.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.CalculateRiskScore("CVE-2024-0001", 9.8, 1.0, time.Now())
	}
}

func BenchmarkIsInKEV(b *testing.B) {
	config := DefaultRiskIndicatorConfig()
	config.AutoUpdate = false
	manager := NewRiskIndicatorManager(config)

	manager.mu.Lock()
	manager.kevCatalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001"},
		},
	}
	manager.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.IsInKEV("CVE-2024-0001")
	}
}
