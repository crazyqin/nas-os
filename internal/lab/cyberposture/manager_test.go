package cyberposture

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.config == nil {
		t.Error("config not initialized")
	}
	if m.threats == nil {
		t.Error("threats map not initialized")
	}
	if m.vulnerabilities == nil {
		t.Error("vulnerabilities map not initialized")
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	config := &Config{
		ScanInterval:   30 * time.Minute,
		AlertThreshold: ThreatHigh,
		AutoMitigate:   false,
		MaxEvents:      5000,
	}

	m := NewManager(config)
	if m.config.ScanInterval != 30*time.Minute {
		t.Errorf("expected scan interval 30m, got %v", m.config.ScanInterval)
	}
}

func TestScanThreats(t *testing.T) {
	m := NewManager(nil)

	ctx := context.Background()
	threats, err := m.ScanThreats(ctx)
	if err != nil {
		t.Fatalf("ScanThreats failed: %v", err)
	}

	if len(threats) == 0 {
		t.Error("expected threats to be detected")
	}
}

func TestScanVulnerabilities(t *testing.T) {
	m := NewManager(nil)

	ctx := context.Background()
	vulns, err := m.ScanVulnerabilities(ctx)
	if err != nil {
		t.Fatalf("ScanVulnerabilities failed: %v", err)
	}

	if len(vulns) == 0 {
		t.Error("expected vulnerabilities to be detected")
	}
}

func TestAnalyzeAttackSurface(t *testing.T) {
	m := NewManager(nil)

	ctx := context.Background()
	surface, err := m.AnalyzeAttackSurface(ctx)
	if err != nil {
		t.Fatalf("AnalyzeAttackSurface failed: %v", err)
	}

	if surface == nil {
		t.Fatal("surface is nil")
	}
	if len(surface.OpenPorts) == 0 {
		t.Error("expected open ports")
	}
	if len(surface.Services) == 0 {
		t.Error("expected services")
	}
	if surface.RiskScore == 0 {
		t.Error("risk score not set")
	}
}

func TestGetScore(t *testing.T) {
	m := NewManager(nil)

	ctx := context.Background()
	score, err := m.GetScore(ctx)
	if err != nil {
		t.Fatalf("GetScore failed: %v", err)
	}

	if score == nil {
		t.Fatal("score is nil")
	}
	if score.Overall == 0 {
		t.Error("overall score not set")
	}
	if score.Network == 0 {
		t.Error("network score not set")
	}
}

func TestAddEvent(t *testing.T) {
	m := NewManager(nil)

	event := &SecurityEvent{
		Type:    "login_failure",
		Level:   ThreatMedium,
		Source:  "192.168.1.100",
		Message: "Failed login attempt",
	}

	err := m.AddEvent(event)
	if err != nil {
		t.Fatalf("AddEvent failed: %v", err)
	}

	if event.ID == "" {
		t.Error("event ID not generated")
	}
	if event.Timestamp.IsZero() {
		t.Error("timestamp not set")
	}
}

func TestGetEvents(t *testing.T) {
	m := NewManager(nil)

	m.AddEvent(&SecurityEvent{Level: ThreatLow, Message: "Low event"})
	m.AddEvent(&SecurityEvent{Level: ThreatHigh, Message: "High event"})
	m.AddEvent(&SecurityEvent{Level: ThreatMedium, Message: "Medium event"})

	events := m.GetEvents("", 10)
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

func TestGetEventsFiltered(t *testing.T) {
	m := NewManager(nil)

	m.AddEvent(&SecurityEvent{Level: ThreatLow, Message: "Low event"})
	m.AddEvent(&SecurityEvent{Level: ThreatHigh, Message: "High event"})
	m.AddEvent(&SecurityEvent{Level: ThreatMedium, Message: "Medium event"})

	events := m.GetEvents(ThreatHigh, 10)
	if len(events) != 1 {
		t.Errorf("expected 1 high event, got %d", len(events))
	}
}

func TestGetEventsLimit(t *testing.T) {
	m := NewManager(nil)

	for i := 0; i < 10; i++ {
		m.AddEvent(&SecurityEvent{Level: ThreatLow, Message: "Event"})
	}

	events := m.GetEvents("", 5)
	if len(events) != 5 {
		t.Errorf("expected 5 events, got %d", len(events))
	}
}

func TestResolveThreat(t *testing.T) {
	m := NewManager(nil)

	m.threats["threat-1"] = &Threat{
		ID:     "threat-1",
		Name:   "Test Threat",
		Status: "active",
	}

	err := m.ResolveThreat("threat-1", "Blocked IP")
	if err != nil {
		t.Fatalf("ResolveThreat failed: %v", err)
	}

	threat := m.threats["threat-1"]
	if threat.Status != "resolved" {
		t.Errorf("expected status 'resolved', got '%s'", threat.Status)
	}
	if threat.Mitigation != "Blocked IP" {
		t.Errorf("expected mitigation 'Blocked IP', got '%s'", threat.Mitigation)
	}
}

func TestResolveThreatNotFound(t *testing.T) {
	m := NewManager(nil)

	err := m.ResolveThreat("nonexistent", "mitigation")
	if err == nil {
		t.Error("expected error for nonexistent threat")
	}
}

func TestFixVulnerability(t *testing.T) {
	m := NewManager(nil)

	m.vulnerabilities["vuln-1"] = &Vulnerability{
		ID:     "vuln-1",
		Title:  "Test Vuln",
		Status: "open",
	}

	err := m.FixVulnerability("vuln-1")
	if err != nil {
		t.Fatalf("FixVulnerability failed: %v", err)
	}

	vuln := m.vulnerabilities["vuln-1"]
	if vuln.Status != "fixed" {
		t.Errorf("expected status 'fixed', got '%s'", vuln.Status)
	}
}

func TestFixVulnerabilityNotFound(t *testing.T) {
	m := NewManager(nil)

	err := m.FixVulnerability("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent vulnerability")
	}
}

func TestGetThreats(t *testing.T) {
	m := NewManager(nil)

	m.threats["t1"] = &Threat{ID: "t1", Level: ThreatLow, Status: "active"}
	m.threats["t2"] = &Threat{ID: "t2", Level: ThreatHigh, Status: "active"}
	m.threats["t3"] = &Threat{ID: "t3", Level: ThreatLow, Status: "resolved"}

	threats := m.GetThreats(ThreatLow)
	if len(threats) != 2 {
		t.Errorf("expected 2 low threats, got %d", len(threats))
	}
}

func TestGetVulnerabilities(t *testing.T) {
	m := NewManager(nil)

	m.vulnerabilities["v1"] = &Vulnerability{ID: "v1", Status: "open"}
	m.vulnerabilities["v2"] = &Vulnerability{ID: "v2", Status: "fixed"}
	m.vulnerabilities["v3"] = &Vulnerability{ID: "v3", Status: "open"}

	vulns := m.GetVulnerabilities("open")
	if len(vulns) != 2 {
		t.Errorf("expected 2 open vulns, got %d", len(vulns))
	}
}

func TestGenerateReport(t *testing.T) {
	m := NewManager(nil)

	m.threats["t1"] = &Threat{ID: "t1", Status: "active"}
	m.threats["t2"] = &Threat{ID: "t2", Status: "resolved"}
	m.vulnerabilities["v1"] = &Vulnerability{ID: "v1", Status: "open"}

	ctx := context.Background()
	report, err := m.GenerateReport(ctx)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if report["generated_at"] == nil {
		t.Error("generated_at not set")
	}
	if report["score"] == nil {
		t.Error("score not set")
	}
}

func TestMaxEvents(t *testing.T) {
	config := &Config{
		MaxEvents: 5,
	}
	m := NewManager(config)

	for i := 0; i < 10; i++ {
		m.AddEvent(&SecurityEvent{Level: ThreatLow, Message: "Event"})
	}

	events := m.GetEvents("", 100)
	if len(events) > 5 {
		t.Errorf("expected max 5 events, got %d", len(events))
	}
}

func TestThreatLevels(t *testing.T) {
	levels := []ThreatLevel{ThreatLow, ThreatMedium, ThreatHigh, ThreatCritical}
	expected := []string{"low", "medium", "high", "critical"}

	for i, l := range levels {
		if string(l) != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], string(l))
		}
	}
}

func TestAttackSurfaceAnalysis(t *testing.T) {
	m := NewManager(nil)

	ctx := context.Background()
	surface, _ := m.AnalyzeAttackSurface(ctx)

	if len(surface.OpenPorts) != 4 {
		t.Errorf("expected 4 open ports, got %d", len(surface.OpenPorts))
	}

	if len(surface.Recommendations) == 0 {
		t.Error("expected recommendations")
	}
}

func TestPostureScoreBreakdown(t *testing.T) {
	m := NewManager(nil)

	ctx := context.Background()
	score, _ := m.GetScore(ctx)

	if score.Breakdown == nil {
		t.Error("breakdown not set")
	}
	if score.Breakdown["firewall"] == 0 {
		t.Error("firewall score not set")
	}
}

func TestEventTimestamps(t *testing.T) {
	m := NewManager(nil)

	before := time.Now()
	event := &SecurityEvent{
		Type:    "test",
		Level:   ThreatLow,
		Message: "Test event",
	}
	m.AddEvent(event)
	after := time.Now()

	if event.Timestamp.Before(before) || event.Timestamp.After(after) {
		t.Error("timestamp not set correctly")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager(nil)

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			m.AddEvent(&SecurityEvent{Level: ThreatLow, Message: "Concurrent event"})
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	events := m.GetEvents("", 100)
	if len(events) != 10 {
		t.Errorf("expected 10 events, got %d", len(events))
	}
}
