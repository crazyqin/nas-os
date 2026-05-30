// Package antiRansomHoneypot 测试
package antiRansomHoneypot

import (
	"fmt"
	"testing"
)

func TestNewHoneypotManager(t *testing.T) {
	m := NewHoneypotManager()
	if m == nil {
		t.Fatal("NewHoneypotManager returned nil")
	}
	if m.stats == nil {
		t.Fatal("stats not initialized")
	}
}

func TestCreateHoneypot(t *testing.T) {
	m := NewHoneypotManager()

	hp := &HoneypotFile{
		Path:     "/shares/documents/decoy.docx",
		Type:     HoneypotTypeDocument,
		FileName: "decoy.docx",
		Enabled:  true,
		ShareName: "documents",
	}

	if err := m.CreateHoneypot(hp); err != nil {
		t.Fatalf("CreateHoneypot failed: %v", err)
	}
	if hp.ID == "" {
		t.Fatal("ID not generated")
	}
	if m.stats.TotalHoneypots != 1 {
		t.Fatalf("expected 1 honeypot, got %d", m.stats.TotalHoneypots)
	}
	if m.stats.ActiveHoneypots != 1 {
		t.Fatalf("expected 1 active, got %d", m.stats.ActiveHoneypots)
	}
}

func TestCreateHoneypotDuplicate(t *testing.T) {
	m := NewHoneypotManager()
	hp := &HoneypotFile{Path: "/test", Type: HoneypotTypeDocument, Enabled: true}
	m.CreateHoneypot(hp)

	if err := m.CreateHoneypot(hp); err != ErrHoneypotExists {
		t.Fatalf("expected ErrHoneypotExists, got %v", err)
	}
}

func TestCreateHoneypotInvalidPath(t *testing.T) {
	m := NewHoneypotManager()
	hp := &HoneypotFile{Path: "", Type: HoneypotTypeDocument}
	if err := m.CreateHoneypot(hp); err != ErrInvalidPath {
		t.Fatalf("expected ErrInvalidPath, got %v", err)
	}
}

func TestRemoveHoneypot(t *testing.T) {
	m := NewHoneypotManager()
	hp := &HoneypotFile{Path: "/test", Type: HoneypotTypeDocument, Enabled: true}
	m.CreateHoneypot(hp)

	if err := m.RemoveHoneypot(hp.ID); err != nil {
		t.Fatalf("RemoveHoneypot failed: %v", err)
	}
	if m.stats.TotalHoneypots != 0 {
		t.Fatalf("expected 0 honeypots, got %d", m.stats.TotalHoneypots)
	}
}

func TestRemoveHoneypotNotFound(t *testing.T) {
	m := NewHoneypotManager()
	if err := m.RemoveHoneypot("nonexistent"); err != ErrHoneypotNotFound {
		t.Fatalf("expected ErrHoneypotNotFound, got %v", err)
	}
}

func TestReportThreat(t *testing.T) {
	m := NewHoneypotManager()
	hp := &HoneypotFile{Path: "/test", Type: HoneypotTypeDocument, Enabled: true}
	m.CreateHoneypot(hp)

	event := &ThreatEvent{
		HoneypotID:    hp.ID,
		SourceIP:      "192.168.1.100",
		ProcessName:   "unknown.exe",
		Operation:     "encrypt",
		EntropyBefore: 4.5,
		EntropyAfter:  7.8,
	}

	result, err := m.ReportThreat(event)
	if err != nil {
		t.Fatalf("ReportThreat failed: %v", err)
	}
	if result.ID == "" {
		t.Fatal("event ID not generated")
	}
	if result.ThreatLevel != ThreatLevelCritical {
		t.Fatalf("expected critical threat, got %s", result.ThreatLevel)
	}
	if hp.TriggerCount != 1 {
		t.Fatalf("expected trigger count 1, got %d", hp.TriggerCount)
	}
}

func TestReportThreatNotFound(t *testing.T) {
	m := NewHoneypotManager()
	event := &ThreatEvent{HoneypotID: "nonexistent", Operation: "read"}
	if _, err := m.ReportThreat(event); err != ErrHoneypotNotFound {
		t.Fatalf("expected ErrHoneypotNotFound, got %v", err)
	}
}

func TestAnalyzeEntropy(t *testing.T) {
	// 均匀分布数据应该有高熵
	uniform := make([]byte, 256)
	for i := 0; i < 256; i++ {
		uniform[i] = byte(i)
	}
	entropy := AnalyzeEntropy(uniform)
	if entropy < 7.0 {
		t.Fatalf("expected high entropy for uniform data, got %f", entropy)
	}

	// 重复数据应该有低熵
	repeated := make([]byte, 256)
	for i := range repeated {
		repeated[i] = 0xAA
	}
	entropy = AnalyzeEntropy(repeated)
	if entropy > 0.1 {
		t.Fatalf("expected ~0 entropy for repeated data, got %f", entropy)
	}

	// 空数据
	entropy = AnalyzeEntropy(nil)
	if entropy != 0 {
		t.Fatalf("expected 0 for nil data, got %f", entropy)
	}
}

func TestSetProtectionPolicy(t *testing.T) {
	m := NewHoneypotManager()
	policy := &ProtectionPolicy{
		Name:             "默认防护",
		Enabled:          true,
		EntropyThreshold: 7.0,
		FileChangeRateMax: 10,
		DefaultAction:    ActionQuarantine,
		AutoResponse:     true,
	}

	if err := m.SetProtectionPolicy(policy); err != nil {
		t.Fatalf("SetProtectionPolicy failed: %v", err)
	}
	if policy.ID == "" {
		t.Fatal("policy ID not generated")
	}
}

func TestGetEvents(t *testing.T) {
	m := NewHoneypotManager()
	hp := &HoneypotFile{Path: "/test", Type: HoneypotTypeDocument, Enabled: true}
	m.CreateHoneypot(hp)

	for i := 0; i < 5; i++ {
		m.ReportThreat(&ThreatEvent{
			HoneypotID:  hp.ID,
			Operation:   "read",
			SourceIP:    "192.168.1.1",
			EntropyAfter: 3.0,
		})
	}

	events := m.GetEvents(3, "")
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
}

func TestGetStats(t *testing.T) {
	m := NewHoneypotManager()
	hp := &HoneypotFile{Path: "/test", Type: HoneypotTypeDocument, Enabled: true}
	m.CreateHoneypot(hp)

	m.ReportThreat(&ThreatEvent{
		HoneypotID: hp.ID,
		Operation:  "encrypt",
		SourceIP:   "10.0.0.1",
		EntropyBefore: 4.0,
		EntropyAfter:  7.5,
		ActionTaken:   ActionQuarantine,
	})

	stats := m.GetStats()
	if stats.TotalHoneypots != 1 {
		t.Fatalf("expected 1 honeypot, got %d", stats.TotalHoneypots)
	}
	if stats.TotalEvents != 1 {
		t.Fatalf("expected 1 event, got %d", stats.TotalEvents)
	}
	if stats.BlockedAttacks != 1 {
		t.Fatalf("expected 1 blocked, got %d", stats.BlockedAttacks)
	}
}

func TestListHoneypots(t *testing.T) {
	m := NewHoneypotManager()
	for i := 0; i < 3; i++ {
		m.CreateHoneypot(&HoneypotFile{
			Path:     fmt.Sprintf("/test%d", i),
			Type:     HoneypotTypeDocument,
			Enabled:  true,
		})
	}

	list := m.ListHoneypots()
	if len(list) != 3 {
		t.Fatalf("expected 3 honeypots, got %d", len(list))
	}
}

func TestExportEvents(t *testing.T) {
	m := NewHoneypotManager()
	hp := &HoneypotFile{Path: "/test", Type: HoneypotTypeDocument, Enabled: true}
	m.CreateHoneypot(hp)

	m.ReportThreat(&ThreatEvent{
		HoneypotID: hp.ID,
		Operation:  "read",
	})

	data, err := m.ExportEvents()
	if err != nil {
		t.Fatalf("ExportEvents failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("exported data is empty")
	}
}
