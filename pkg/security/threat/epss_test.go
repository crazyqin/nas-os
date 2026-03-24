// Package threat - EPSS 数据库测试
package threat

import (
	"testing"
)

func TestEPSSDatabase_New(t *testing.T) {
	config := EPSSConfig{
		CachePath:   t.TempDir() + "/epss_test.json",
		OfflineMode: true,
	}

	edb := NewEPSSDatabase(config)
	if edb == nil {
		t.Fatal("NewEPSSDatabase returned nil")
	}
}

func TestEPSSDatabase_GetEPSS(t *testing.T) {
	config := EPSSConfig{
		CachePath:   t.TempDir() + "/epss_test.json",
		OfflineMode: true,
	}

	edb := NewEPSSDatabase(config)

	// 手动添加测试数据
	edb.mu.Lock()
	edb.cache["CVE-2024-0001"] = &EPSSData{
		CVEID:      "CVE-2024-0001",
		Score:      0.5,
		Percentile: 0.95,
	}
	edb.mu.Unlock()

	data := edb.GetEPSS("CVE-2024-0001")
	if data == nil {
		t.Fatal("Expected to get EPSS data")
	}

	if data.Score != 0.5 {
		t.Errorf("Expected score 0.5, got %f", data.Score)
	}

	if data.Percentile != 0.95 {
		t.Errorf("Expected percentile 0.95, got %f", data.Percentile)
	}
}

func TestEPSSDatabase_GetHighEPSS(t *testing.T) {
	config := EPSSConfig{
		CachePath:   t.TempDir() + "/epss_test.json",
		OfflineMode: true,
	}

	edb := NewEPSSDatabase(config)

	edb.mu.Lock()
	edb.cache["CVE-2024-0001"] = &EPSSData{CVEID: "CVE-2024-0001", Score: 0.5}
	edb.cache["CVE-2024-0002"] = &EPSSData{CVEID: "CVE-2024-0002", Score: 0.05}
	edb.cache["CVE-2024-0003"] = &EPSSData{CVEID: "CVE-2024-0003", Score: 0.8}
	edb.mu.Unlock()

	threshold := 0.1
	results := edb.GetHighEPSS(threshold)
	if len(results) != 2 {
		t.Errorf("Expected 2 high EPSS entries, got %d", len(results))
	}
}

func TestEPSSDatabase_GetHighPercentile(t *testing.T) {
	config := EPSSConfig{
		CachePath:   t.TempDir() + "/epss_test.json",
		OfflineMode: true,
	}

	edb := NewEPSSDatabase(config)

	edb.mu.Lock()
	edb.cache["CVE-2024-0001"] = &EPSSData{CVEID: "CVE-2024-0001", Percentile: 0.95}
	edb.cache["CVE-2024-0002"] = &EPSSData{CVEID: "CVE-2024-0002", Percentile: 0.85}
	edb.cache["CVE-2024-0003"] = &EPSSData{CVEID: "CVE-2024-0003", Percentile: 0.99}
	edb.mu.Unlock()

	threshold := 0.9
	results := edb.GetHighPercentile(threshold)
	if len(results) != 2 {
		t.Errorf("Expected 2 high percentile entries, got %d", len(results))
	}
}

func TestEPSSDatabase_GetEPSSRiskLevel(t *testing.T) {
	config := EPSSConfig{
		CachePath:   t.TempDir() + "/epss_test.json",
		OfflineMode: true,
	}

	edb := NewEPSSDatabase(config)

	tests := []struct {
		score    float64
		expected EPSSRiskLevel
	}{
		{0.6, EPSSRiskCritical},
		{0.3, EPSSRiskHigh},
		{0.05, EPSSRiskMedium},
		{0.005, EPSSRiskLow},
	}

	for _, tt := range tests {
		level := edb.GetEPSSRiskLevel(tt.score)
		if level != tt.expected {
			t.Errorf("Score %f: expected %s, got %s", tt.score, tt.expected, level)
		}
	}
}

func TestEPSSDatabase_ClassifyByRiskLevel(t *testing.T) {
	config := EPSSConfig{
		CachePath:   t.TempDir() + "/epss_test.json",
		OfflineMode: true,
	}

	edb := NewEPSSDatabase(config)

	edb.mu.Lock()
	edb.cache["CVE-2024-0001"] = &EPSSData{CVEID: "CVE-2024-0001", Score: 0.6}
	edb.cache["CVE-2024-0002"] = &EPSSData{CVEID: "CVE-2024-0002", Score: 0.3}
	edb.cache["CVE-2024-0003"] = &EPSSData{CVEID: "CVE-2024-0003", Score: 0.05}
	edb.cache["CVE-2024-0004"] = &EPSSData{CVEID: "CVE-2024-0004", Score: 0.005}
	edb.mu.Unlock()

	classified := edb.ClassifyByRiskLevel()

	if len(classified[EPSSRiskCritical]) != 1 {
		t.Errorf("Expected 1 critical, got %d", len(classified[EPSSRiskCritical]))
	}

	if len(classified[EPSSRiskHigh]) != 1 {
		t.Errorf("Expected 1 high, got %d", len(classified[EPSSRiskHigh]))
	}

	if len(classified[EPSSRiskMedium]) != 1 {
		t.Errorf("Expected 1 medium, got %d", len(classified[EPSSRiskMedium]))
	}

	if len(classified[EPSSRiskLow]) != 1 {
		t.Errorf("Expected 1 low, got %d", len(classified[EPSSRiskLow]))
	}
}

func TestEPSSDatabase_GetStatistics(t *testing.T) {
	config := EPSSConfig{
		CachePath:   t.TempDir() + "/epss_test.json",
		OfflineMode: true,
	}

	edb := NewEPSSDatabase(config)

	edb.mu.Lock()
	edb.cache["CVE-2024-0001"] = &EPSSData{CVEID: "CVE-2024-0001", Score: 0.5, Percentile: 0.9}
	edb.cache["CVE-2024-0002"] = &EPSSData{CVEID: "CVE-2024-0002", Score: 0.3, Percentile: 0.8}
	edb.mu.Unlock()

	stats := edb.GetStatistics()

	total := stats["total"].(int)
	if total != 2 {
		t.Errorf("Expected total 2, got %d", total)
	}

	avgScore := stats["average_score"].(float64)
	if avgScore != 0.4 {
		t.Errorf("Expected average score 0.4, got %f", avgScore)
	}
}