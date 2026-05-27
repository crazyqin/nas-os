package dlp

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:             true,
		ScanIntervalHours:   24,
		MaxFileSizeMB:       100,
		DefaultMinSeverity:  "medium",
		QuarantinePath:      "/quarantine",
		EnableRealTime:      false,
		ConfidenceThreshold: 0.8,
		RetentionDays:       90,
	}
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestCreateScan(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	scan := manager.CreateScan("Daily Scan", "/data/documents", true)
	if scan == nil {
		t.Fatal("CreateScan returned nil")
	}
	if scan.Status != ScanPending {
		t.Errorf("expected pending, got %s", scan.Status)
	}
}

func TestAddFinding(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	finding := &Finding{
		ScanID:      "scan-1",
		FilePath:    "/data/docs/ssn.txt",
		LineNumber:  5,
		Type:        FindingPII,
		Severity:    SeverityHigh,
		Category:    "Social Security Number",
		Description: "SSN pattern detected",
		Confidence:  0.95,
	}
	manager.AddFinding(finding)

	findings := manager.ListFindings()
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestResolveFinding(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	finding := &Finding{
		ScanID:   "scan-1",
		FilePath: "/test",
		Type:     FindingCredentials,
		Severity: SeverityCritical,
	}
	manager.AddFinding(finding)

	if err := manager.ResolveFinding(finding.ID, "admin"); err != nil {
		t.Fatalf("ResolveFinding failed: %v", err)
	}
	if !finding.Resolved {
		t.Error("expected finding to be resolved")
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{Enabled: true}
	manager := NewManager(config)

	stats := manager.GetStats()
	if stats.TotalScans != 0 {
		t.Errorf("expected 0 scans, got %d", stats.TotalScans)
	}
}
