// Package threat - KEV 数据库测试
package threat

import (
	"testing"
	"time"
)

func TestKEVDatabase_New(t *testing.T) {
	config := KEVConfig{
		CachePath:   t.TempDir() + "/kev_test.json",
		OfflineMode: true,
	}

	kdb := NewKEVDatabase(config)
	if kdb == nil {
		t.Fatal("NewKEVDatabase returned nil")
	}
}

func TestKEVDatabase_IsInKEV(t *testing.T) {
	config := KEVConfig{
		CachePath:   t.TempDir() + "/kev_test.json",
		OfflineMode: true,
	}

	kdb := NewKEVDatabase(config)

	// 手动添加测试数据
	kdb.mu.Lock()
	kdb.catalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", VendorProject: "Test"},
		},
	}
	kdb.mu.Unlock()

	if !kdb.IsInKEV("CVE-2024-0001") {
		t.Error("Expected CVE-2024-0001 to be in KEV")
	}

	if kdb.IsInKEV("CVE-9999-9999") {
		t.Error("Expected CVE-9999-9999 to not be in KEV")
	}
}

func TestKEVDatabase_GetKEVEntry(t *testing.T) {
	config := KEVConfig{
		CachePath:   t.TempDir() + "/kev_test.json",
		OfflineMode: true,
	}

	kdb := NewKEVDatabase(config)

	kdb.mu.Lock()
	kdb.catalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{
				CVEID:         "CVE-2024-0001",
				VendorProject: "TestVendor",
				Product:       "TestProduct",
			},
		},
	}
	kdb.mu.Unlock()

	entry := kdb.GetKEVEntry("CVE-2024-0001")
	if entry == nil {
		t.Fatal("Expected to get KEV entry")
	}

	if entry.VendorProject != "TestVendor" {
		t.Errorf("Expected VendorProject 'TestVendor', got '%s'", entry.VendorProject)
	}
}

func TestKEVDatabase_GetRansomwareRelated(t *testing.T) {
	config := KEVConfig{
		CachePath:   t.TempDir() + "/kev_test.json",
		OfflineMode: true,
	}

	kdb := NewKEVDatabase(config)

	kdb.mu.Lock()
	kdb.catalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{
				CVEID:                      "CVE-2024-0001",
				KnownRansomwareCampaignUse: "Known",
			},
			{
				CVEID:                      "CVE-2024-0002",
				KnownRansomwareCampaignUse: "Unknown",
			},
		},
	}
	kdb.mu.Unlock()

	entries := kdb.GetRansomwareRelated()
	if len(entries) != 1 {
		t.Errorf("Expected 1 ransomware related entry, got %d", len(entries))
	}
}

func TestKEVDatabase_GetOverdue(t *testing.T) {
	config := KEVConfig{
		CachePath:   t.TempDir() + "/kev_test.json",
		OfflineMode: true,
	}

	kdb := NewKEVDatabase(config)

	pastDate := time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	futureDate := time.Now().AddDate(0, 0, 30).Format("2006-01-02")

	kdb.mu.Lock()
	kdb.catalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{
				CVEID:   "CVE-2024-0001",
				DueDate: pastDate,
			},
			{
				CVEID:   "CVE-2024-0002",
				DueDate: futureDate,
			},
		},
	}
	kdb.mu.Unlock()

	entries := kdb.GetOverdue()
	if len(entries) != 1 {
		t.Errorf("Expected 1 overdue entry, got %d", len(entries))
	}
}

func TestKEVDatabase_Search(t *testing.T) {
	config := KEVConfig{
		CachePath:   t.TempDir() + "/kev_test.json",
		OfflineMode: true,
	}

	kdb := NewKEVDatabase(config)

	kdb.mu.Lock()
	kdb.catalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{
				CVEID:             "CVE-2024-0001",
				VendorProject:     "Microsoft",
				VulnerabilityName: "Test Vulnerability",
			},
			{
				CVEID:             "CVE-2024-0002",
				VendorProject:     "Apple",
				VulnerabilityName: "Another Test",
			},
		},
	}
	kdb.mu.Unlock()

	entries := kdb.Search("Microsoft")
	if len(entries) != 1 {
		t.Errorf("Expected 1 search result, got %d", len(entries))
	}

	entries = kdb.Search("Test")
	if len(entries) != 2 {
		t.Errorf("Expected 2 search results, got %d", len(entries))
	}
}

func TestKEVDatabase_FilterByVendor(t *testing.T) {
	config := KEVConfig{
		CachePath:   t.TempDir() + "/kev_test.json",
		OfflineMode: true,
	}

	kdb := NewKEVDatabase(config)

	kdb.mu.Lock()
	kdb.catalog = &KEVCatalog{
		Vulnerabilities: []KEVEntry{
			{CVEID: "CVE-2024-0001", VendorProject: "Microsoft"},
			{CVEID: "CVE-2024-0002", VendorProject: "Apple"},
			{CVEID: "CVE-2024-0003", VendorProject: "Microsoft"},
		},
	}
	kdb.mu.Unlock()

	entries := kdb.FilterByVendor("Microsoft")
	if len(entries) != 2 {
		t.Errorf("Expected 2 Microsoft entries, got %d", len(entries))
	}
}