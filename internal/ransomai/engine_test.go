package ransomai

import (
	"testing"
	"time"
)

func TestNewRansomAI(t *testing.T) {
	ra := NewRansomAI()
	if ra == nil {
		t.Fatal("NewRansomAI returned nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.EntropyThreshold != 7.5 {
		t.Errorf("expected entropy 7.5, got %f", cfg.EntropyThreshold)
	}
	if !cfg.HoneypotEnabled {
		t.Error("expected honeypot enabled")
	}
}

func TestRecordEventNormal(t *testing.T) {
	ra := NewRansomAI()
	alert := ra.RecordEvent(FileEvent{
		Path:      "/data/normal.txt",
		Operation: "create",
		Size:      1024,
		Entropy:   4.0,
		Process:   "editor",
		Timestamp: time.Now(),
	})
	if alert != nil {
		t.Error("normal event should not trigger alert")
	}
}

func TestRecordEventHighEntropy(t *testing.T) {
	ra := NewRansomAI()
	alert := ra.RecordEvent(FileEvent{
		Path:      "/data/encrypted.bin",
		Operation: "modify",
		Size:      4096,
		Entropy:   8.0,
		Process:   "suspicious",
		Timestamp: time.Now(),
	})
	if alert == nil {
		t.Fatal("high entropy should trigger alert")
	}
	if alert.Level < ThreatHigh {
		t.Errorf("expected high threat, got %s", alert.Level.String())
	}
}

func TestHoneypot(t *testing.T) {
	ra := NewRansomAI()
	ra.AddHoneypot("/traps/important.docx", ".docx")
	
	honeypots := ra.GetHoneypots()
	if len(honeypots) != 1 {
		t.Fatalf("expected 1 honeypot, got %d", len(honeypots))
	}
	
	// 触发蜜罐
	alert := ra.RecordEvent(FileEvent{
		Path:      "/traps/important.docx",
		Operation: "modify",
		Process:   "ransomware.exe",
		Timestamp: time.Now(),
	})
	if alert == nil {
		t.Fatal("honeypot trigger should generate alert")
	}
	if alert.Level != ThreatCritical {
		t.Errorf("expected critical threat, got %s", alert.Level.String())
	}
}

func TestGetAlerts(t *testing.T) {
	ra := NewRansomAI()
	ra.RecordEvent(FileEvent{
		Path:      "/data/test",
		Operation: "modify",
		Entropy:   8.5,
		Timestamp: time.Now(),
	})
	
	alerts := ra.GetAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
}

func TestGetConfig(t *testing.T) {
	ra := NewRansomAI()
	cfg := ra.GetConfig()
	if cfg.EntropyThreshold != 7.5 {
		t.Errorf("unexpected threshold: %f", cfg.EntropyThreshold)
	}
}
