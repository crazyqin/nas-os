package threatdetect

import (
	"testing"
)

func TestNewThreatDetector(t *testing.T) {
	td := NewThreatDetector(nil)
	if td == nil {
		t.Fatal("expected non-nil detector")
	}
	if len(td.rules) != 5 {
		t.Errorf("expected 5 default rules, got %d", len(td.rules))
	}
}

func TestAddRule(t *testing.T) {
	td := NewThreatDetector(nil)
	rule := &DetectionRule{
		ID: "custom", Name: "自定义", EventType: EventAnomalousAccess,
		Level: LevelMedium, Enabled: true, Threshold: 10, WindowSec: 60, Action: "log",
	}
	if err := td.AddRule(rule); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := td.rules["custom"]; !ok {
		t.Error("rule not added")
	}
}

func TestAddRuleInvalid(t *testing.T) {
	td := NewThreatDetector(nil)
	if err := td.AddRule(nil); err == nil {
		t.Fatal("expected error")
	}
	if err := td.AddRule(&DetectionRule{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRemoveRule(t *testing.T) {
	td := NewThreatDetector(nil)
	if err := td.RemoveRule("builtin-brute-force"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := td.rules["builtin-brute-force"]; ok {
		t.Error("rule should be removed")
	}
}

func TestRemoveRuleNotFound(t *testing.T) {
	td := NewThreatDetector(nil)
	if err := td.RemoveRule("nonexistent"); err == nil {
		t.Fatal("expected error")
	}
}

func TestProcessEvent(t *testing.T) {
	td := NewThreatDetector(nil)
	event := &SecurityEvent{
		Type: EventBruteForce, Level: LevelHigh,
		Source: "test", User: "attacker", IP: "10.0.0.1", Description: "暴力破解",
	}
	if err := td.ProcessEvent(event); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if event.ID == "" {
		t.Error("expected event ID")
	}
	metrics := td.GetMetrics()
	if metrics.TotalEvents != 1 {
		t.Errorf("expected 1, got %d", metrics.TotalEvents)
	}
}

func TestProcessEventNil(t *testing.T) {
	td := NewThreatDetector(nil)
	if err := td.ProcessEvent(nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestDetectBruteForce(t *testing.T) {
	td := NewThreatDetector(nil)
	for i := 0; i < 4; i++ {
		if r := td.DetectBruteForce("10.0.0.1", false); r != nil {
			t.Fatalf("should not trigger at attempt %d", i+1)
		}
	}
	r := td.DetectBruteForce("10.0.0.1", false)
	if r == nil {
		t.Fatal("expected detection at 5th attempt")
	}
	if r.Level != LevelHigh {
		t.Errorf("expected high, got %v", r.Level)
	}
}

func TestDetectBruteForceSuccess(t *testing.T) {
	td := NewThreatDetector(nil)
	if r := td.DetectBruteForce("10.0.0.1", true); r != nil {
		t.Fatal("should not trigger on success")
	}
}

func TestDetectRansomware(t *testing.T) {
	td := NewThreatDetector(nil)
	if r := td.DetectRansomware("user1", 5); r != nil {
		t.Fatal("should not trigger for 5")
	}
	r := td.DetectRansomware("user1", 15)
	if r == nil {
		t.Fatal("expected detection")
	}
	if r.Level != LevelCritical {
		t.Errorf("expected critical, got %v", r.Level)
	}
}

func TestDetectBulkDelete(t *testing.T) {
	td := NewThreatDetector(nil)
	if r := td.DetectBulkDelete("user1", 30); r != nil {
		t.Fatal("should not trigger for 30")
	}
	r := td.DetectBulkDelete("user1", 100)
	if r == nil {
		t.Fatal("expected detection")
	}
}

func TestScanFile(t *testing.T) {
	td := NewThreatDetector(nil)
	if r := td.ScanFile("/data/photo.jpg", nil); r != nil {
		t.Fatal("should not detect normal file")
	}
	r := td.ScanFile("/data/file.encrypted", nil)
	if r == nil {
		t.Fatal("should detect .encrypted")
	}
	if r.Level != LevelCritical {
		t.Errorf("expected critical, got %v", r.Level)
	}
}

func TestQuarantineAndRelease(t *testing.T) {
	td := NewThreatDetector(nil)
	event := &SecurityEvent{
		Type: EventRansomware, Level: LevelCritical,
		Source: "test", Path: "/data/file.encrypted", Description: "test",
	}
	td.ProcessEvent(event)

	list := td.GetQuarantineList(false)
	if len(list) != 1 {
		t.Fatalf("expected 1, got %d", len(list))
	}

	if err := td.ReleaseQuarantine(list[0].ID, "admin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	active := td.GetQuarantineList(false)
	if len(active) != 0 {
		t.Errorf("expected 0 active, got %d", len(active))
	}
}

func TestResolveEvent(t *testing.T) {
	td := NewThreatDetector(nil)
	event := &SecurityEvent{
		Type: EventBruteForce, Level: LevelHigh,
		Source: "test", Description: "test",
	}
	td.ProcessEvent(event)

	if err := td.ResolveEvent(event.ID, "analyst", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !event.Resolved {
		t.Error("expected resolved")
	}
}

func TestResolveEventFalsePositive(t *testing.T) {
	td := NewThreatDetector(nil)
	event := &SecurityEvent{
		Type: EventAnomalousAccess, Level: LevelLow,
		Source: "test", Description: "test",
	}
	td.ProcessEvent(event)
	td.ResolveEvent(event.ID, "analyst", true)

	metrics := td.GetMetrics()
	if metrics.FalsePositives != 1 {
		t.Errorf("expected 1, got %d", metrics.FalsePositives)
	}
}

func TestGetEvents(t *testing.T) {
	td := NewThreatDetector(nil)
	td.ProcessEvent(&SecurityEvent{Type: EventAnomalousAccess, Level: LevelLow, Source: "t", Description: "l"})
	td.ProcessEvent(&SecurityEvent{Type: EventBruteForce, Level: LevelHigh, Source: "t", Description: "h"})
	td.ProcessEvent(&SecurityEvent{Type: EventRansomware, Level: LevelCritical, Source: "t", Description: "c"})

	high := td.GetEvents(LevelHigh, false, 10)
	if len(high) != 2 {
		t.Errorf("expected 2, got %d", len(high))
	}
}

func TestThreatLevelString(t *testing.T) {
	tests := []struct {
		l ThreatLevel; s string
	}{
		{LevelLow, "low"}, {LevelMedium, "medium"}, {LevelHigh, "high"},
		{LevelCritical, "critical"}, {ThreatLevel(99), "unknown"},
	}
	for _, tt := range tests {
		if tt.l.String() != tt.s {
			t.Errorf("expected %s, got %s", tt.s, tt.l.String())
		}
	}
}
